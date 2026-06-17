#!/usr/bin/env python3
"""
Create a UniPost Hosted Connect session URL for local OAuth testing.

Example:
  export UNIPOST_API_KEY="sk_live_..."
  python3 scripts/create_connect_session_url.py \
    --platform tiktok \
    --profile-id pr_123 \
    --external-user-id local-test-user \
    --allow-quickstart-creds

The script prints the hosted Connection session URL. Copy it into a
browser to complete the platform OAuth flow.
"""

import argparse
import json
import os
import sys
import time
import urllib.error
import urllib.request
from typing import Any, Callable, Mapping, Optional, TextIO


DEFAULT_API_BASE = "https://api.unipost.dev"
USER_AGENT = "unipost-connect-session-url/1.0"


class APIError(RuntimeError):
    def __init__(self, status: int, body: Any):
        self.status = status
        self.body = body
        super().__init__(f"API {status}: {body}")


RequestJSON = Callable[[str, str, dict[str, str], dict[str, Any]], Any]


def request_json(method: str, url: str, headers: dict[str, str], payload: dict[str, Any]) -> Any:
    data = json.dumps(payload).encode("utf-8")
    req = urllib.request.Request(url, data=data, method=method, headers=headers)
    try:
        with urllib.request.urlopen(req, timeout=30) as resp:
            body = resp.read().decode("utf-8")
            return json.loads(body) if body else None
    except urllib.error.HTTPError as err:
        raw = err.read().decode("utf-8", errors="replace")
        try:
            parsed = json.loads(raw)
        except json.JSONDecodeError:
            parsed = raw
        raise APIError(err.code, parsed) from None
    except urllib.error.URLError as err:
        raise RuntimeError(f"Network error: {err.reason}") from None


def env_bool(value: str) -> bool:
    return value.strip().lower() in {"1", "true", "yes", "y", "on"}


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(
        description="Create a UniPost Hosted Connect session URL for local OAuth testing.",
    )
    parser.add_argument(
        "--api-base",
        default=None,
        help=f"UniPost API base URL. Defaults to UNIPOST_API_BASE or {DEFAULT_API_BASE}.",
    )
    parser.add_argument(
        "--platform",
        default=None,
        help="Platform to connect, for example tiktok, linkedin, instagram, threads, youtube, pinterest, twitter, or facebook. Defaults to UNIPOST_PLATFORM.",
    )
    parser.add_argument(
        "--profile-id",
        default=None,
        help="Profile that should own the connected account. Defaults to UNIPOST_PROFILE_ID.",
    )
    parser.add_argument(
        "--external-user-id",
        default=None,
        help="Your local test user's stable identifier. Defaults to UNIPOST_EXTERNAL_USER_ID or a generated local-test-* value.",
    )
    parser.add_argument(
        "--external-user-email",
        default=None,
        help="Optional end-user email. Defaults to UNIPOST_EXTERNAL_USER_EMAIL.",
    )
    parser.add_argument(
        "--return-url",
        default=None,
        help="Optional post-OAuth landing URL. Defaults to UNIPOST_RETURN_URL.",
    )
    parser.add_argument(
        "--allow-quickstart-creds",
        action="store_true",
        help="Allow UniPost shared OAuth credentials when workspace platform credentials are missing. Can also be set with UNIPOST_ALLOW_QUICKSTART_CREDS=true.",
    )
    return parser


def pick(args_value: Optional[str], environ: Mapping[str, str], env_key: str, default: str = "") -> str:
    value = args_value if args_value is not None else environ.get(env_key, default)
    return value.strip()


def build_payload(args: argparse.Namespace, environ: Mapping[str, str]) -> dict[str, Any]:
    platform = pick(args.platform, environ, "UNIPOST_PLATFORM").lower()
    if not platform:
        raise ValueError("Missing --platform or UNIPOST_PLATFORM")

    external_user_id = pick(args.external_user_id, environ, "UNIPOST_EXTERNAL_USER_ID")
    if not external_user_id:
        external_user_id = f"local-test-{int(time.time())}"

    payload: dict[str, Any] = {
        "platform": platform,
        "external_user_id": external_user_id,
    }

    profile_id = pick(args.profile_id, environ, "UNIPOST_PROFILE_ID")
    if profile_id:
        payload["profile_id"] = profile_id

    external_user_email = pick(args.external_user_email, environ, "UNIPOST_EXTERNAL_USER_EMAIL")
    if external_user_email:
        payload["external_user_email"] = external_user_email

    return_url = pick(args.return_url, environ, "UNIPOST_RETURN_URL")
    if return_url:
        payload["return_url"] = return_url

    allow_quickstart = args.allow_quickstart_creds or env_bool(environ.get("UNIPOST_ALLOW_QUICKSTART_CREDS", ""))
    if allow_quickstart:
        payload["allow_quickstart_creds"] = True

    return payload


def print_api_error(err: APIError, stderr: TextIO) -> None:
    print(f"API error {err.status} while creating the Connect session:", file=stderr)
    if isinstance(err.body, (dict, list)):
        print(json.dumps(err.body, indent=2), file=stderr)
    else:
        print(err.body, file=stderr)
    print("", file=stderr)
    print("Common fixes:", file=stderr)
    print("- Check UNIPOST_API_KEY is a workspace API key for the target account.", file=stderr)
    print("- If the workspace has no platform credentials, retry with --allow-quickstart-creds.", file=stderr)
    print("- If the workspace has multiple profiles, pass --profile-id.", file=stderr)


def main(
    argv: Optional[list[str]] = None,
    *,
    environ: Optional[Mapping[str, str]] = None,
    request_json: RequestJSON = request_json,
    stdout: TextIO = sys.stdout,
    stderr: TextIO = sys.stderr,
) -> int:
    if environ is None:
        environ = os.environ

    parser = build_parser()
    args = parser.parse_args(argv)

    api_key = environ.get("UNIPOST_API_KEY", "").strip()
    if not api_key:
        print("Missing UNIPOST_API_KEY.", file=stderr)
        print("Create or copy a workspace API key, then export UNIPOST_API_KEY before running this script.", file=stderr)
        return 2

    api_base = pick(args.api_base, environ, "UNIPOST_API_BASE", DEFAULT_API_BASE).rstrip("/")
    if not api_base:
        api_base = DEFAULT_API_BASE

    try:
        payload = build_payload(args, environ)
    except ValueError as err:
        print(str(err), file=stderr)
        return 2

    headers = {
        "Authorization": f"Bearer {api_key}",
        "Accept": "application/json",
        "Content-Type": "application/json",
        "User-Agent": USER_AGENT,
    }

    url = f"{api_base}/v1/connect/sessions"
    try:
        resp = request_json("POST", url, headers, payload)
    except APIError as err:
        print_api_error(err, stderr)
        return 1
    except Exception as err:  # noqa: BLE001 - command-line helper should print useful failures.
        print(f"Failed to create Connect session: {err}", file=stderr)
        return 1

    data = resp.get("data") if isinstance(resp, dict) else None
    session_url = data.get("url") if isinstance(data, dict) else ""
    if not session_url:
        print("Connect session response did not include data.url.", file=stderr)
        if resp is not None:
            print(json.dumps(resp, indent=2, default=str), file=stderr)
        return 1

    session_id = data.get("id", "") if isinstance(data, dict) else ""
    platform = data.get("platform", payload["platform"]) if isinstance(data, dict) else payload["platform"]
    status = data.get("status", "") if isinstance(data, dict) else ""
    expires_at = data.get("expires_at", "") if isinstance(data, dict) else ""

    if session_id:
        summary = f"Created Connect session {session_id}"
        if platform:
            summary += f" for {platform}"
        if status:
            summary += f" (status: {status})"
        print(summary + ".", file=stdout)

    print("", file=stdout)
    print("Connection session URL:", file=stdout)
    print(session_url, file=stdout)
    print("", file=stdout)
    print("Copy this URL into your browser to complete OAuth.", file=stdout)
    if expires_at:
        print(f"Expires at: {expires_at}", file=stdout)

    return 0


if __name__ == "__main__":
    sys.exit(main())
