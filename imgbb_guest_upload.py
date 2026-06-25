"""Upload images to ImgBB as a guest and return direct/delete links."""

from __future__ import annotations

import argparse
import http.cookiejar
import json
import mimetypes
import re
import time
import urllib.error
import urllib.parse
import urllib.request
from dataclasses import asdict, dataclass
from pathlib import Path
from typing import Iterable
from uuid import uuid4


IMGBB_HOME_URL = "https://imgbb.com/"
IMGBB_JSON_URL = "https://imgbb.com/json"
DEFAULT_USER_AGENT = (
    "Mozilla/5.0 (Windows NT 10.0; Win64; x64) "
    "AppleWebKit/537.36 (KHTML, like Gecko) "
    "Chrome/150.0.0.0 Safari/537.36"
)

IMGBB_EXPIRATIONS = {
    "none": "",
    "5m": "PT5M",
    "15m": "PT15M",
    "30m": "PT30M",
    "1h": "PT1H",
    "3h": "PT3H",
    "6h": "PT6H",
    "12h": "PT12H",
    "1d": "P1D",
    "2d": "P2D",
    "3d": "P3D",
    "4d": "P4D",
    "5d": "P5D",
    "6d": "P6D",
    "1w": "P1W",
    "2w": "P2W",
    "3w": "P3W",
    "1mo": "P1M",
    "2mo": "P2M",
    "3mo": "P3M",
    "4mo": "P4M",
    "5mo": "P5M",
    "6mo": "P6M",
}


@dataclass(frozen=True)
class GuestAuth:
    """ImgBB guest auth data bound to the same cookie opener."""

    auth_token: str
    phpsessid: str | None
    opener: urllib.request.OpenerDirector


@dataclass(frozen=True)
class UploadLinks:
    """Links returned after a successful ImgBB upload."""

    delete_url: str
    direct_url: str


class ImgbbUploadError(RuntimeError):
    """Raised when ImgBB rejects or cannot complete an upload."""


def get_imgbb_guest_auth(
    timeout: int = 15,
    user_agent: str = DEFAULT_USER_AGENT,
) -> GuestAuth:
    """Fetch a fresh guest auth token and its bound PHP session.

    Args:
        timeout: Request timeout in seconds.
        user_agent: User-Agent sent to ImgBB.

    Returns:
        GuestAuth: Auth token, PHPSESSID, and cookie-aware opener.

    Raises:
        ImgbbUploadError: If the homepage does not expose an auth token.
        urllib.error.URLError: If the request fails.
    """
    cookie_jar = http.cookiejar.CookieJar()
    opener = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(cookie_jar))
    request = urllib.request.Request(
        IMGBB_HOME_URL,
        headers={
            "Accept": "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
            "User-Agent": user_agent,
        },
    )

    with opener.open(request, timeout=timeout) as response:
        html = response.read().decode("utf-8", errors="replace")

    match = re.search(r'PF\.obj\.config\.auth_token\s*=\s*"([^"]+)"', html)
    if not match:
        raise ImgbbUploadError("ImgBB auth_token not found in homepage HTML.")

    return GuestAuth(
        auth_token=match.group(1),
        phpsessid=_get_cookie_value(cookie_jar, "PHPSESSID"),
        opener=opener,
    )


def upload_to_imgbb(
    source: str,
    expiration: str = "none",
    timeout: int = 60,
    user_agent: str = DEFAULT_USER_AGENT,
) -> UploadLinks:
    """Upload a local file or remote image URL and return delete/direct links.

    Each call creates a fresh ImgBB guest session, so every image upload uses
    a newly issued auth_token bound to its own PHPSESSID cookie.

    Args:
        source: Local image path or remote image URL.
        expiration: Expiration key from IMGBB_EXPIRATIONS or a raw ImgBB value.
        timeout: Upload request timeout in seconds.
        user_agent: User-Agent sent to ImgBB.

    Returns:
        UploadLinks: Delete URL and original direct image URL.

    Raises:
        FileNotFoundError: If a local file source does not exist.
        ImgbbUploadError: If ImgBB returns an error or expected links are missing.
        urllib.error.URLError: If the request fails.
    """
    guest = get_imgbb_guest_auth(timeout=timeout, user_agent=user_agent)
    expiration_value = IMGBB_EXPIRATIONS.get(expiration, expiration)
    fields = [
        ("type", "url" if _is_url(source) else "file"),
        ("action", "upload"),
        ("timestamp", str(int(time.time() * 1000))),
        ("auth_token", guest.auth_token),
    ]

    if expiration_value:
        fields.append(("expiration", expiration_value))

    if _is_url(source):
        fields.insert(0, ("source", source))
        files: list[tuple[str, str, bytes, str]] = []
    else:
        file_path = Path(source).expanduser().resolve()
        if not file_path.is_file():
            raise FileNotFoundError(f"Image file not found: {file_path}")
        content_type = mimetypes.guess_type(file_path.name)[0] or "application/octet-stream"
        files = [("source", file_path.name, file_path.read_bytes(), content_type)]

    body, content_type = _encode_multipart(fields=fields, files=files)
    request = urllib.request.Request(
        IMGBB_JSON_URL,
        data=body,
        method="POST",
        headers={
            "Accept": "application/json",
            "Content-Type": content_type,
            "Origin": IMGBB_HOME_URL.rstrip("/"),
            "Referer": IMGBB_HOME_URL,
            "User-Agent": user_agent,
        },
    )

    try:
        with guest.opener.open(request, timeout=timeout) as response:
            payload = _read_json_response(response)
    except urllib.error.HTTPError as err:
        payload = _read_json_error(err)
        message = _error_message(payload) or f"HTTP {err.code}"
        raise ImgbbUploadError(f"ImgBB upload failed: {message}") from err

    image = payload.get("image")
    if not isinstance(image, dict):
        raise ImgbbUploadError("ImgBB upload response does not contain image data.")

    delete_url = image.get("delete_url")
    direct_url = image.get("url")
    if not isinstance(delete_url, str) or not isinstance(direct_url, str):
        raise ImgbbUploadError("ImgBB upload response does not contain expected links.")

    return UploadLinks(delete_url=delete_url, direct_url=direct_url)


def main() -> int:
    """Run the command-line uploader.

    Returns:
        int: Process exit code.
    """
    parser = argparse.ArgumentParser(
        description="Upload an image to ImgBB as a guest and print delete/direct links."
    )
    parser.add_argument("source", help="Local image path or remote image URL.")
    parser.add_argument(
        "-e",
        "--expiration",
        default="none",
        choices=tuple(IMGBB_EXPIRATIONS.keys()),
        help="Auto-delete time. Default: none.",
    )
    parser.add_argument(
        "--raw-expiration",
        default=None,
        help="Raw ImgBB expiration value, for example PT5M or P1D. Overrides --expiration.",
    )
    parser.add_argument(
        "--timeout",
        type=int,
        default=60,
        help="Request timeout in seconds. Default: 60.",
    )
    parser.add_argument(
        "--user-agent",
        default=DEFAULT_USER_AGENT,
        help="Custom User-Agent header.",
    )
    parser.add_argument(
        "--json",
        action="store_true",
        help="Print result as JSON.",
    )
    args = parser.parse_args()

    expiration = args.raw_expiration if args.raw_expiration is not None else args.expiration

    try:
        links = upload_to_imgbb(
            source=args.source,
            expiration=expiration,
            timeout=args.timeout,
            user_agent=args.user_agent,
        )
    except (FileNotFoundError, ImgbbUploadError, urllib.error.URLError) as err:
        print(f"error: {err}")
        return 1

    if args.json:
        print(json.dumps(asdict(links), ensure_ascii=False, indent=2))
    else:
        print(f"delete_url={links.delete_url}")
        print(f"direct_url={links.direct_url}")

    return 0


def _encode_multipart(
    fields: Iterable[tuple[str, str]],
    files: Iterable[tuple[str, str, bytes, str]],
) -> tuple[bytes, str]:
    """Encode multipart/form-data without requiring third-party packages.

    Args:
        fields: Text form fields.
        files: File fields as name, filename, data, content type.

    Returns:
        tuple[bytes, str]: Request body and Content-Type header value.
    """
    boundary = f"----ImgBBGuestUpload{uuid4().hex}"
    chunks: list[bytes] = []

    for name, value in fields:
        chunks.extend(
            [
                f"--{boundary}\r\n".encode("ascii"),
                f'Content-Disposition: form-data; name="{name}"\r\n\r\n'.encode("utf-8"),
                value.encode("utf-8"),
                b"\r\n",
            ]
        )

    for name, filename, data, content_type in files:
        chunks.extend(
            [
                f"--{boundary}\r\n".encode("ascii"),
                (
                    f'Content-Disposition: form-data; name="{name}"; '
                    f'filename="{filename}"\r\n'
                ).encode("utf-8"),
                f"Content-Type: {content_type}\r\n\r\n".encode("ascii"),
                data,
                b"\r\n",
            ]
        )

    chunks.append(f"--{boundary}--\r\n".encode("ascii"))
    return b"".join(chunks), f"multipart/form-data; boundary={boundary}"


def _read_json_response(response: object) -> dict:
    """Read and decode an ImgBB JSON response.

    Args:
        response: File-like HTTP response object.

    Returns:
        dict: Decoded JSON payload.

    Raises:
        ImgbbUploadError: If the body is not a JSON object.
    """
    raw = response.read().decode("utf-8", errors="replace")
    try:
        payload = json.loads(raw)
    except json.JSONDecodeError as err:
        raise ImgbbUploadError("ImgBB returned a non-JSON response.") from err

    if not isinstance(payload, dict):
        raise ImgbbUploadError("ImgBB returned an unexpected JSON response.")

    if payload.get("status_code") != 200:
        message = _error_message(payload) or "unknown error"
        raise ImgbbUploadError(f"ImgBB upload failed: {message}")

    return payload


def _read_json_error(err: urllib.error.HTTPError) -> dict | None:
    """Read a JSON error body from an HTTPError when available."""
    try:
        raw = err.read().decode("utf-8", errors="replace")
        payload = json.loads(raw)
    except json.JSONDecodeError:
        return None

    return payload if isinstance(payload, dict) else None


def _error_message(payload: dict | None) -> str | None:
    """Extract ImgBB error message from a JSON payload."""
    if not payload:
        return None

    error = payload.get("error")
    if isinstance(error, dict) and isinstance(error.get("message"), str):
        return error["message"]

    return None


def _get_cookie_value(cookie_jar: http.cookiejar.CookieJar, name: str) -> str | None:
    """Return a cookie value by name from a CookieJar."""
    for cookie in cookie_jar:
        if cookie.name == name:
            return cookie.value

    return None


def _is_url(value: str) -> bool:
    """Return whether a source value is an HTTP(S) URL."""
    parsed = urllib.parse.urlparse(value)
    return parsed.scheme in {"http", "https"} and bool(parsed.netloc)


if __name__ == "__main__":
    raise SystemExit(main())
