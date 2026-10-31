#!/usr/bin/env python3
"""FileFlow MCP operations and bounded, streaming server uploads (Python 3.10+)."""

from __future__ import annotations

import argparse
import http.client
import json
import mimetypes
import os
from pathlib import Path
import re
import ssl
import stat
import sys
from typing import Any, Callable
from urllib.parse import quote, urlsplit
import uuid


MAX_FILE_BYTES = 5 * 1024**3
CHUNK_BYTES = 1024**2
MAX_RESPONSE_BYTES = 4 * 1024**2
MAX_MCP_REQUEST_BYTES = 12 * 1024**2
SECRET_KEYS = {
    "token", "apitoken", "password", "passwordhash", "currentpassword",
    "newpassword", "accesskeyid", "secretaccesskey", "authorization",
}


class ClientError(Exception):
    def __init__(self, message: str, status: int = 0, data: Any = None):
        super().__init__(message)
        self.status = status
        self.data = data


def _has_controls(value: str) -> bool:
    return any(ord(character) < 32 or ord(character) == 127 for character in value)


def _secret_values(value: Any) -> list[str]:
    found: list[str] = []
    if isinstance(value, dict):
        for key, item in value.items():
            if key.lower() in SECRET_KEYS and isinstance(item, str) and item:
                found.append(item)
            found.extend(_secret_values(item))
    elif isinstance(value, list):
        for item in value:
            found.extend(_secret_values(item))
    return found


def redact(value: Any, secrets: list[str], redact_keys: bool = True) -> Any:
    if isinstance(value, dict):
        return {
            key: "[REDACTED]" if redact_keys and key.lower() in SECRET_KEYS else redact(item, secrets, redact_keys)
            for key, item in value.items()
        }
    if isinstance(value, list):
        return [redact(item, secrets, redact_keys) for item in value]
    if isinstance(value, str):
        for secret in sorted(set(secrets), key=len, reverse=True):
            if secret:
                value = value.replace(secret, "[REDACTED]")
    return value


class FileFlowClient:
    def __init__(self, base_url: str, token: str, timeout: float = 300):
        if not base_url:
            raise ClientError("Set FILEFLOW_URL or pass --url with the FileFlow service URL.")
        if _has_controls(base_url) or any(character.isspace() for character in base_url):
            raise ClientError("The service URL cannot contain whitespace or control characters.")
        try:
            parsed = urlsplit(base_url)
            port = parsed.port
        except ValueError as error:
            raise ClientError("Invalid FileFlow service URL.") from error
        if parsed.scheme not in {"http", "https"} or not parsed.hostname or parsed.username is not None or parsed.password is not None or parsed.query or parsed.fragment:
            raise ClientError("The service URL must be HTTP(S) without credentials, query parameters or a fragment.")
        if not token or _has_controls(token):
            raise ClientError("Set FILEFLOW_TOKEN or FILEFLOW_MCP_TOKEN with a valid Bearer credential.")
        self.host = parsed.hostname
        self.port = port
        self.scheme = parsed.scheme
        self.prefix = quote(parsed.path.rstrip("/"), safe="/%:@-._~!$&'()*+,;=")
        self.token = token
        self.timeout = timeout

    def _connection(self) -> http.client.HTTPConnection:
        if self.scheme == "https":
            return http.client.HTTPSConnection(self.host, self.port, timeout=self.timeout, context=ssl.create_default_context())
        return http.client.HTTPConnection(self.host, self.port, timeout=self.timeout)

    def _response(self, connection: http.client.HTTPConnection) -> tuple[int, Any]:
        response = connection.getresponse()
        if 300 <= response.status < 400:
            raise ClientError("Redirect refused. Configure the service's final URL; credentials are never forwarded to redirects.", response.status)
        body = response.read(MAX_RESPONSE_BYTES + 1)
        if len(body) > MAX_RESPONSE_BYTES:
            raise ClientError("The server response exceeded the 4 MiB response limit.", response.status)
        try:
            data = json.loads(body.decode("utf-8"))
        except (ValueError, UnicodeDecodeError) as error:
            raise ClientError("The server returned non-JSON data. Check the service URL and deployed version.", response.status) from error
        if not 200 <= response.status < 300:
            message = data.get("error", "FileFlow rejected the request.") if isinstance(data, dict) else "FileFlow rejected the request."
            raise ClientError(str(message), response.status, data)
        return response.status, data

    def _request(self, path: str, content_type: str, length: int, send_body: Callable[[http.client.HTTPConnection], None], mcp: bool = False) -> tuple[int, Any]:
        connection = self._connection()
        try:
            connection.putrequest("POST", self.prefix + path)
            connection.putheader("Authorization", "Bearer " + self.token)
            connection.putheader("Content-Type", content_type)
            connection.putheader("Content-Length", str(length))
            connection.putheader("Accept", "application/json, text/event-stream" if mcp else "application/json")
            if mcp:
                connection.putheader("MCP-Protocol-Version", "2025-06-18")
            connection.endheaders()
            try:
                send_body(connection)
            except (OSError, http.client.HTTPException):
                # Servers may reject authentication or size before consuming a file.
                # Read an already-issued error when possible, without resending bytes.
                return self._response(connection)
            return self._response(connection)
        except ClientError:
            raise
        except (OSError, http.client.HTTPException) as error:
            raise ClientError("The request did not complete. Check its result before manually retrying; no automatic retry was made.") from error
        finally:
            connection.close()

    def rpc(self, method: str, params: dict[str, Any]) -> tuple[int, dict[str, Any]]:
        try:
            payload = json.dumps({"jsonrpc": "2.0", "id": 1, "method": method, "params": params}, ensure_ascii=False, separators=(",", ":")).encode("utf-8")
        except (ValueError, UnicodeEncodeError) as error:
            raise ClientError("Tool parameters must contain valid Unicode JSON values.") from error
        if len(payload) > MAX_MCP_REQUEST_BYTES:
            raise ClientError("MCP parameters exceed 12 MiB. Use the upload command for local file bytes.")
        status, data = self._request("/mcp", "application/json", len(payload), lambda connection: connection.send(payload), mcp=True)
        if not isinstance(data, dict):
            raise ClientError("Invalid MCP response envelope.", status)
        if "error" in data:
            rpc_error = data["error"]
            message = rpc_error.get("message", "MCP protocol error.") if isinstance(rpc_error, dict) else "MCP protocol error."
            raise ClientError(str(message), status, {"rpcError": rpc_error})
        result = data.get("result")
        if not isinstance(result, dict):
            raise ClientError("The MCP response did not contain a result.", status)
        return status, result

    def tools(self) -> dict[str, Any]:
        status, result = self.rpc("tools/list", {})
        return {"status": status, "data": result, "isError": False}

    def call(self, name: str, arguments: dict[str, Any]) -> dict[str, Any]:
        if not re.fullmatch(r"fileflow_[a-z0-9_]+", name):
            raise ClientError("Use a FileFlow tool name returned by the tools command.")
        http_status, result = self.rpc("tools/call", {"name": name, "arguments": arguments})
        structured = result.get("structuredContent")
        if not isinstance(structured, dict) or "status" not in structured or "data" not in structured:
            raise ClientError("The tool did not return FileFlow structuredContent.", http_status)
        status = structured["status"]
        data = structured["data"]
        failed = bool(result.get("isError")) or not isinstance(status, int) or not 200 <= status < 300
        if isinstance(data, dict) and isinstance(data.get("failCount"), (int, float)) and data["failCount"] > 0:
            failed = True
        return {"status": status, "data": data, "isError": failed}

    def upload(self, file_path: str, account: str | None, path: str | None, keep_name: bool, expiration_days: int, request_id: str) -> dict[str, Any]:
        local_path = Path(file_path).expanduser()
        filename = local_path.name
        if not filename or _has_controls(filename):
            raise ClientError("The local file name cannot contain control characters.")
        try:
            filename.encode("utf-8")
        except UnicodeEncodeError as error:
            raise ClientError("The local file name must be valid UTF-8 text.") from error
        if not request_id or len(request_id.encode("utf-8")) > 128 or _has_controls(request_id):
            raise ClientError("requestId must contain 1–128 UTF-8 bytes without control characters.")
        fields = {"randomFilename": "false" if keep_name else "true", "expirationDays": str(expiration_days), "requestId": request_id}
        if account:
            fields["idGroup"] = account
        if path:
            fields["path"] = path
        if any(_has_controls(value) for value in fields.values()):
            raise ClientError("Upload options cannot contain control characters.")
        boundary = "fileflow-" + uuid.uuid4().hex
        prefix = bytearray()
        for key, value in fields.items():
            prefix.extend(f'--{boundary}\r\nContent-Disposition: form-data; name="{key}"\r\n\r\n{value}\r\n'.encode("utf-8"))
        quoted_filename = filename.replace("\\", "\\\\").replace('"', '\\"')
        content_type = mimetypes.guess_type(filename)[0] or "application/octet-stream"
        prefix.extend(f'--{boundary}\r\nContent-Disposition: form-data; name="file"; filename="{quoted_filename}"\r\nContent-Type: {content_type}\r\n\r\n'.encode("utf-8"))
        suffix = f"\r\n--{boundary}--\r\n".encode("ascii")
        try:
            if not local_path.is_file():
                raise ClientError("Upload requires a regular local file.")
            with local_path.open("rb") as source:
                info = os.fstat(source.fileno())
                if not stat.S_ISREG(info.st_mode):
                    raise ClientError("Upload requires a regular local file.")
                if info.st_size > MAX_FILE_BYTES:
                    raise ClientError("The file exceeds FileFlow's 5 GiB upload limit.")
                if info.st_size < 0:
                    raise ClientError("Invalid local file size.")

                def send_file(connection: http.client.HTTPConnection) -> None:
                    connection.send(prefix)
                    remaining = info.st_size
                    while remaining:
                        chunk = source.read(min(CHUNK_BYTES, remaining))
                        if not chunk:
                            raise ClientError("The local file changed or became unreadable during upload.")
                        connection.send(chunk)
                        remaining -= len(chunk)
                    if source.read(1):
                        raise ClientError("The local file grew during upload; the request was stopped.")
                    after = os.fstat(source.fileno())
                    if after.st_size != info.st_size or after.st_mtime_ns != info.st_mtime_ns:
                        raise ClientError("The local file changed during upload; verify the server result before retrying.")
                    connection.send(suffix)

                status, data = self._request("/api/upload", "multipart/form-data; boundary=" + boundary, len(prefix) + info.st_size + len(suffix), send_file)
        except ClientError:
            raise
        except OSError as error:
            raise ClientError("Cannot read the local upload file.") from error
        return {"status": status, "data": data, "isError": False, "requestId": request_id}


def _common_arguments(parser: argparse.ArgumentParser, suppress: bool = False) -> None:
    default = argparse.SUPPRESS if suppress else None
    parser.add_argument("--url", default=default, help="FileFlow base URL (default: FILEFLOW_URL)")
    parser.add_argument("--timeout", type=float, default=argparse.SUPPRESS if suppress else 300, help="Socket timeout in seconds, 1-3600 (default: 300)")
    parser.add_argument("--output", default=default, help="Write the full result to a NEW private JSON file; refuse overwriting")
    parser.add_argument("--show-secrets", action="store_true", default=argparse.SUPPRESS if suppress else False, help="Explicitly allow credential values in stdout")


def parser() -> argparse.ArgumentParser:
    root = argparse.ArgumentParser(description=__doc__)
    _common_arguments(root)
    commands = root.add_subparsers(dest="command", required=True)
    tools = commands.add_parser("tools", help="List currently authorized MCP tools")
    _common_arguments(tools, suppress=True)
    call = commands.add_parser("call", help="Call a discovered FileFlow MCP tool")
    _common_arguments(call, suppress=True)
    call.add_argument("tool")
    arguments = call.add_mutually_exclusive_group()
    arguments.add_argument("--args", default="{}", help="JSON object of tool arguments")
    arguments.add_argument("--args-file", help="Local UTF-8 JSON argument file")
    upload = commands.add_parser("upload", help="Stream a local file through FileFlow, bypassing MCP Base64 limits")
    _common_arguments(upload, suppress=True)
    upload.add_argument("file")
    upload.add_argument("--account", help="FileFlow storage account ID; omit for automatic selection")
    upload.add_argument("--path", help="Destination directory prefix")
    upload.add_argument("--keep-name", action="store_true", help="Preserve the local name; the server refuses overwrite")
    upload.add_argument("--expiration-days", type=int, default=-1, help="-1: server default, 0: permanent, 1-3650: days")
    upload.add_argument("--request-id", help="Stable ID for manual retries within the server's 24-hour deduplication window")
    return root


def _load_arguments(options: argparse.Namespace) -> dict[str, Any]:
    try:
        if options.args_file:
            with Path(options.args_file).expanduser().open("rb") as source:
                encoded = source.read(MAX_MCP_REQUEST_BYTES + 1)
            if len(encoded) > MAX_MCP_REQUEST_BYTES:
                raise ClientError("The tool argument file exceeds 12 MiB.")
            arguments = json.loads(encoded.decode("utf-8"))
        else:
            arguments = json.loads(options.args)
    except (OSError, ValueError, UnicodeDecodeError) as error:
        raise ClientError("Tool arguments must be a valid UTF-8 JSON object.") from error
    if not isinstance(arguments, dict):
        raise ClientError("Tool arguments must be a JSON object.")
    return arguments


def _private_output(path: str):
    try:
        descriptor = os.open(Path(path).expanduser(), os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
        return os.fdopen(descriptor, "w", encoding="utf-8")
    except OSError as error:
        raise ClientError("Cannot write --output. Use a new file path; existing files are never overwritten.") from error


def main(argv: list[str] | None = None) -> int:
    options = parser().parse_args(argv)
    token = os.environ.get("FILEFLOW_TOKEN") or os.environ.get("FILEFLOW_MCP_TOKEN", "")
    secrets = [token] if token else []
    request_id = (options.request_id or str(uuid.uuid4())) if options.command == "upload" else None
    output = None
    try:
        if not 1 <= options.timeout <= 3600:
            raise ClientError("--timeout must be between 1 and 3600 seconds.")
        client = FileFlowClient(options.url or os.environ.get("FILEFLOW_URL", ""), token, options.timeout)
        if options.output:
            output = _private_output(options.output)
        if options.command == "tools":
            result = client.tools()
        elif options.command == "call":
            arguments = _load_arguments(options)
            secrets.extend(_secret_values(arguments))
            result = client.call(options.tool, arguments)
        else:
            if not -1 <= options.expiration_days <= 3650:
                raise ClientError("--expiration-days must be -1, 0, or 1–3650.")
            result = client.upload(options.file, options.account, options.path, options.keep_name, options.expiration_days, request_id)
    except ClientError as error:
        result = {"status": error.status, "error": str(error), "isError": True}
        if error.data is not None:
            result["data"] = error.data
        if request_id:
            result["requestId"] = request_id
    if output is not None:
        try:
            json.dump(result, output, ensure_ascii=True, indent=2)
            output.write("\n")
            output.close()
            result["outputFile"] = str(Path(options.output).expanduser())
        except (OSError, ValueError):
            result["error"] = "Cannot finish writing --output. Inspect the private file and operation result before retrying."
            result["isError"] = True
            output.close()
    if not options.show_secrets:
        result = redact(result, secrets, redact_keys=options.command != "tools")
    # Explicit UTF-8 bytes avoid Windows console encodings corrupting file names.
    encoded = (json.dumps(result, ensure_ascii=False, separators=(",", ":")) + "\n").encode("utf-8", errors="backslashreplace")
    if hasattr(sys.stdout, "buffer"):
        sys.stdout.buffer.write(encoded)
    else:
        sys.stdout.write(encoded.decode("utf-8"))
    return 1 if result.get("isError") else 0


if __name__ == "__main__":
    raise SystemExit(main())
