"""Behavioral tests for the FileFlow helper; no live FileFlow/R2 data is used."""

from email.parser import BytesParser
from email.policy import default
import hashlib
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import importlib.util
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import threading
import unittest
from unittest import mock


SCRIPT = Path(__file__).with_name("fileflow.py")
SPEC = importlib.util.spec_from_file_location("fileflow_helper", SCRIPT)
helper = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(helper)
TEST_TOKEN = "fixture-token-not-live"


class StubServer:
    def __init__(self):
        self.requests = []
        self.mode = "success"
        self.upload_digest = None
        self.upload_size = 0
        self.fields = {}
        self.filename = None
        state = self

        class Handler(BaseHTTPRequestHandler):
            protocol_version = "HTTP/1.1"

            def log_message(self, *_):
                pass

            def do_POST(self):
                length = int(self.headers.get("Content-Length", "0"))
                body = self.rfile.read(length)
                state.requests.append({"path": self.path, "headers": dict(self.headers), "length": length})
                if state.mode == "redirect":
                    self.send_response(307)
                    self.send_header("Location", "http://127.0.0.1:1/steal")
                    self.send_header("Content-Length", "0")
                    self.end_headers()
                    return
                if state.mode in {"401", "429"}:
                    self.reply(int(state.mode), {"error": "denied", "code": "permission_denied" if state.mode == "401" else "quota_exceeded"})
                    return
                if self.path == "/api/upload":
                    mime = BytesParser(policy=default).parsebytes(b"Content-Type: " + self.headers["Content-Type"].encode() + b"\r\nMIME-Version: 1.0\r\n\r\n" + body)
                    for part in mime.iter_parts():
                        name = part.get_param("name", header="content-disposition")
                        content = part.get_payload(decode=True)
                        if name == "file":
                            state.upload_digest = hashlib.sha256(content).hexdigest()
                            state.upload_size = len(content)
                            state.filename = part.get_filename()
                        else:
                            state.fields[name] = content.decode("utf-8")
                    self.reply(200, {"id": "account-fixture", "key": "documents/" + state.filename, "size": state.upload_size, "url": "https://files.example/sample"})
                    return
                request = json.loads(body)
                if request["method"] == "tools/list":
                    result = {"tools": [{"name": "fileflow_create_user", "inputSchema": {"properties": {"password": {"type": "string"}}}}]}
                else:
                    tool = request["params"]["name"]
                    if tool == "fileflow_create_token":
                        structured = {"status": 201, "data": {"token": "created-secret", "name": "agent", "message": TEST_TOKEN}}
                    elif tool == "fileflow_batch_delete_files":
                        structured = {"status": 200, "data": {"failCount": 1, "results": [{"key": "x", "success": False}]}}
                    else:
                        structured = {"status": 403, "data": {"error": "permission denied"}}
                    result = {"structuredContent": structured, "isError": structured["status"] >= 400, "content": [{"type": "text", "text": json.dumps(structured)}]}
                self.reply(200, {"jsonrpc": "2.0", "id": 1, "result": result})

            def reply(self, status, value):
                body = json.dumps(value, ensure_ascii=False).encode("utf-8")
                self.send_response(status)
                self.send_header("Content-Type", "application/json")
                self.send_header("Content-Length", str(len(body)))
                self.end_headers()
                self.wfile.write(body)

        self.server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
        self.thread = threading.Thread(target=self.server.serve_forever, daemon=True)
        self.url = "http://127.0.0.1:" + str(self.server.server_port)

    def __enter__(self):
        self.thread.start()
        return self

    def __exit__(self, *_):
        self.server.shutdown()
        self.server.server_close()
        self.thread.join(timeout=2)


class HelperTests(unittest.TestCase):
    def cli(self, server, *arguments, extra_env=None):
        environment = {**os.environ, "FILEFLOW_URL": server.url, "FILEFLOW_TOKEN": TEST_TOKEN}
        environment.update(extra_env or {})
        result = subprocess.run([sys.executable, str(SCRIPT), *map(str, arguments)], capture_output=True, env=environment, timeout=15)
        self.assertEqual(result.stderr, b"", result.stderr)
        return result.returncode, json.loads(result.stdout.decode("utf-8"))

    def test_streaming_upload_exceeds_mcp_limit_and_preserves_unicode_fields(self):
        with tempfile.TemporaryDirectory() as directory, StubServer() as server:
            local = Path(directory, "团队报告.txt")
            payload = b"large-local-file\n" * ((9 * 1024**2) // 17 + 1)
            local.write_bytes(payload)
            code, result = self.cli(server, "upload", local, "--account", "account-fixture", "--path", "documents", "--keep-name", "--expiration-days", "7", "--request-id", "stable-local-file")
            self.assertEqual(code, 0, result)
            self.assertGreater(server.upload_size, 8 * 1024**2)
            self.assertEqual(server.upload_size, len(payload))
            self.assertEqual(server.upload_digest, hashlib.sha256(payload).hexdigest())
            self.assertEqual(server.filename, "团队报告.txt")
            self.assertEqual(server.fields, {"idGroup": "account-fixture", "path": "documents", "randomFilename": "false", "expirationDays": "7", "requestId": "stable-local-file"})
            self.assertEqual(server.requests[0]["headers"]["Authorization"], "Bearer " + TEST_TOKEN)
            self.assertEqual(server.requests[0]["path"], "/api/upload")
            self.assertNotIn(TEST_TOKEN, json.dumps(result))
            self.assertEqual(result["requestId"], "stable-local-file")

    def test_upload_send_chunks_are_bounded(self):
        class RecordingConnection:
            def __init__(self):
                self.sizes = []

            def send(self, content):
                self.sizes.append(len(content))

        class Client(helper.FileFlowClient):
            def _request(self, path, content_type, length, send_body, mcp=False):
                connection = RecordingConnection()
                send_body(connection)
                self.chunk_sizes = connection.sizes
                self.length = length
                return 200, {"size": length}

        with tempfile.TemporaryDirectory() as directory:
            local = Path(directory, "large.bin")
            with local.open("wb") as output:
                output.truncate(9 * 1024**2)
            client = Client("http://127.0.0.1:1", TEST_TOKEN)
            client.upload(str(local), None, None, False, -1, "bounded")
            self.assertLessEqual(max(client.chunk_sizes), helper.CHUNK_BYTES)
            self.assertEqual(sum(client.chunk_sizes), client.length)

    def test_above_five_gib_rejected_without_http_request(self):
        with tempfile.TemporaryDirectory() as directory, StubServer() as server:
            local = Path(directory, "oversize.bin")
            local.write_bytes(b"tiny")
            info = type("FileInfo", (), {"st_size": helper.MAX_FILE_BYTES + 1, "st_mode": 0o100600})()
            client = helper.FileFlowClient(server.url, TEST_TOKEN)
            with mock.patch.object(helper.os, "fstat", return_value=info):
                with self.assertRaisesRegex(helper.ClientError, "5 GiB"):
                    client.upload(str(local), None, None, False, -1, "too-big")
            self.assertEqual(server.requests, [])

    def test_server_errors_preserve_request_id_without_retry(self):
        with tempfile.TemporaryDirectory() as directory, StubServer() as server:
            local = Path(directory, "sample.txt")
            local.write_bytes(b"hello")
            for status in ("401", "429"):
                server.mode = status
                previous = len(server.requests)
                code, result = self.cli(server, "upload", local, "--request-id", "retry-only-manually")
                self.assertEqual(code, 1)
                self.assertEqual(result["status"], int(status))
                self.assertEqual(result["requestId"], "retry-only-manually")
                self.assertEqual(len(server.requests), previous + 1)

    def test_redirect_not_followed(self):
        with StubServer() as server:
            server.mode = "redirect"
            code, result = self.cli(server, "tools")
            self.assertEqual(code, 1)
            self.assertEqual(result["status"], 307)
            self.assertEqual(len(server.requests), 1)
            self.assertIn("Redirect refused", result["error"])

    def test_tool_catalog_schema_not_redacted_and_mcp_permission_error(self):
        with StubServer() as server:
            code, result = self.cli(server, "tools", "--url", server.url)
            self.assertEqual(code, 0)
            self.assertEqual(result["data"]["tools"][0]["inputSchema"]["properties"]["password"], {"type": "string"})
            code, result = self.cli(server, "call", "fileflow_update_settings", "--args", "{}")
            self.assertEqual(code, 1)
            self.assertEqual(result["status"], 403)
            self.assertTrue(result["isError"])

    def test_partial_batch_result_has_nonzero_exit(self):
        with StubServer() as server:
            code, result = self.cli(server, "call", "fileflow_batch_delete_files", "--args", '{"accountId":"x","keys":["x"]}')
            self.assertEqual(code, 1)
            self.assertEqual(result["data"]["failCount"], 1)
            self.assertTrue(result["isError"])

    def test_secrets_redacted_and_private_output_contains_full_result(self):
        with tempfile.TemporaryDirectory() as directory, StubServer() as server:
            output = Path(directory, "created-token.json")
            code, result = self.cli(server, "--output", output, "call", "fileflow_create_token", "--args", '{"name":"agent","permissions":["read"]}')
            self.assertEqual(code, 0)
            self.assertEqual(result["data"]["token"], "[REDACTED]")
            self.assertEqual(result["data"]["message"], "[REDACTED]")
            saved = json.loads(output.read_text(encoding="utf-8"))
            self.assertEqual(saved["data"]["token"], "created-secret")
            if os.name != "nt":
                self.assertEqual(output.stat().st_mode & 0o777, 0o600)
            request_count = len(server.requests)
            code, result = self.cli(server, "call", "fileflow_create_token", "--output", output)
            self.assertEqual(code, 1)
            self.assertEqual(len(server.requests), request_count)
            self.assertEqual(json.loads(output.read_text(encoding="utf-8")), saved)
            code, result = self.cli(server, "call", "fileflow_create_token", "--show-secrets")
            self.assertEqual(code, 0)
            self.assertEqual(result["data"]["token"], "created-secret")

    def test_invalid_url_and_missing_credential_do_not_send_requests(self):
        for address in ("https://user:secret@example.com", "https://example.com/?token=secret", "file:///tmp/file", "https://example.com/#fragment"):
            with self.assertRaises(helper.ClientError):
                helper.FileFlowClient(address, TEST_TOKEN)
        with self.assertRaises(helper.ClientError):
            helper.FileFlowClient("https://example.com", "")

    def test_generated_request_id_on_local_failure(self):
        with StubServer() as server:
            code, result = self.cli(server, "upload", "/this-file-does-not-exist")
            self.assertEqual(code, 1)
            self.assertRegex(result["requestId"], r"^[0-9a-f-]{36}$")
            self.assertEqual(server.requests, [])

    def test_response_size_is_bounded(self):
        class Response:
            status = 200

            def read(self, size):
                self.read_limit = size
                return b"x" * size

        response = Response()
        connection = mock.Mock()
        connection.getresponse.return_value = response
        client = helper.FileFlowClient("https://example.com", TEST_TOKEN)
        with self.assertRaisesRegex(helper.ClientError, "4 MiB"):
            client._response(connection)
        self.assertEqual(response.read_limit, helper.MAX_RESPONSE_BYTES + 1)

    def test_invalid_argument_unicode_is_rejected_without_network(self):
        client = helper.FileFlowClient("https://example.com", TEST_TOKEN)
        with self.assertRaisesRegex(helper.ClientError, "Unicode"):
            client.call("fileflow_create_user", {"displayName": "\ud800"})


if __name__ == "__main__":
    unittest.main()
