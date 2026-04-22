#!/usr/bin/env python3
"""
End-to-end validation of the UniPost white-label API flow against
LinkedIn in production.

What this script exercises (all via Workspace API key):
  1. API-key sanity check (GET /v1/profiles/<id>)
  2. Create a Connect session → print branded Connect URL
  3. Pause for you to complete OAuth in a browser
  4. Poll the session until it completes + discover the
     social_account_id Meta/LinkedIn minted for the external user
  5. Publish a text-only LinkedIn post
  6. Poll the post until dispatching clears
  7. Fetch the post's analytics snapshot

What this script does NOT do:
  - Upload white-label OAuth credentials (that endpoint is
    Clerk-auth only; do it in the Dashboard at
    /projects/<profile>/accounts/native before running this)
  - Set profile branding (same — Dashboard only today)
  - Clean up — the account + post remain on LinkedIn for
    manual inspection afterwards

Usage:
  export UNIPOST_API_KEY='<your workspace API key>'
  export UNIPOST_PROFILE_ID='<your profile UUID>'
  python3 scripts/test_whitelabel_linkedin.py

Optional env vars:
  UNIPOST_API_BASE          default https://api.unipost.dev
  UNIPOST_EXTERNAL_USER_ID  default wl-test-<timestamp>
  UNIPOST_RETURN_URL        default https://example.com/return
  UNIPOST_POST_TEXT         default "UniPost white-label test …"
  UNIPOST_POLL_TIMEOUT_SEC  default 300 (5 min window for OAuth)
  UNIPOST_POST_TIMEOUT_SEC  default 60  (LinkedIn publishes fast)
"""

import json
import os
import sys
import time
import urllib.error
import urllib.request
import uuid
from typing import Any, Optional


API_BASE = os.environ.get("UNIPOST_API_BASE", "https://api.unipost.dev").rstrip("/")
API_KEY = os.environ.get("UNIPOST_API_KEY", "").strip()
PROFILE_ID = os.environ.get("UNIPOST_PROFILE_ID", "").strip()
EXTERNAL_USER_ID = os.environ.get(
    "UNIPOST_EXTERNAL_USER_ID", f"wl-test-{int(time.time())}"
)
RETURN_URL = os.environ.get("UNIPOST_RETURN_URL", "https://example.com/return")
POST_TEXT = os.environ.get(
    "UNIPOST_POST_TEXT",
    f"UniPost white-label test — {uuid.uuid4().hex[:8]}",
)
OAUTH_TIMEOUT = int(os.environ.get("UNIPOST_POLL_TIMEOUT_SEC", "300"))
POST_TIMEOUT = int(os.environ.get("UNIPOST_POST_TIMEOUT_SEC", "60"))


class APIError(RuntimeError):
    def __init__(self, status: int, body: Any):
        self.status = status
        self.body = body
        super().__init__(f"API {status}: {body}")


def _request(method: str, path: str, payload: Optional[dict] = None) -> Any:
    """Minimal JSON HTTP client. Raises APIError on non-2xx with
    the full response body so failures are debuggable at a glance."""
    url = f"{API_BASE}{path}"
    data = None
    headers = {
        "Authorization": f"Bearer {API_KEY}",
        "Accept": "application/json",
        # Cloudflare's managed rules on api.unipost.dev block the
        # default Python-urllib User-Agent as a bot signature, so we
        # send a neutral desktop-looking UA. Change here if CF starts
        # blocking this string too.
        "User-Agent": "unipost-whitelabel-test/1.0",
    }
    if payload is not None:
        data = json.dumps(payload).encode("utf-8")
        headers["Content-Type"] = "application/json"
    req = urllib.request.Request(url, data=data, method=method, headers=headers)
    try:
        with urllib.request.urlopen(req, timeout=30) as resp:
            body = resp.read().decode("utf-8")
            return json.loads(body) if body else None
    except urllib.error.HTTPError as e:
        raw = e.read().decode("utf-8", errors="replace")
        try:
            parsed = json.loads(raw)
        except json.JSONDecodeError:
            parsed = raw
        raise APIError(e.code, parsed) from None


def step(n: int, title: str) -> None:
    print(f"\n── Step {n}: {title} " + "─" * (60 - len(title)))


def dump(label: str, obj: Any) -> None:
    print(f"  {label}:")
    text = json.dumps(obj, indent=2, default=str) if obj is not None else "None"
    for line in text.splitlines():
        print(f"    {line}")


def require_env() -> None:
    missing = [k for k, v in [("UNIPOST_API_KEY", API_KEY), ("UNIPOST_PROFILE_ID", PROFILE_ID)] if not v]
    if missing:
        sys.stderr.write(
            "Missing required env var(s): " + ", ".join(missing) + "\n"
            "  Set UNIPOST_API_KEY (create one in Dashboard → Settings → API keys)\n"
            "  Set UNIPOST_PROFILE_ID (the profile UUID from the URL when viewing\n"
            "    a project in the Dashboard)\n"
        )
        sys.exit(2)


def check_api_key() -> dict:
    step(1, "Verify API key + profile")
    resp = _request("GET", f"/v1/profiles/{PROFILE_ID}")
    data = resp.get("data") if isinstance(resp, dict) else None
    dump("profile", data)
    if not data or not data.get("id"):
        raise RuntimeError("Profile payload didn't include an id — check API key + profile id")
    return data


def create_connect_session() -> dict:
    step(2, "Create LinkedIn Connect session")
    body = {
        "platform": "linkedin",
        "profile_id": PROFILE_ID,
        "external_user_id": EXTERNAL_USER_ID,
        "return_url": RETURN_URL,
    }
    print(f"  external_user_id: {EXTERNAL_USER_ID}")
    resp = _request("POST", "/v1/connect/sessions", body)
    data = resp.get("data") if isinstance(resp, dict) else None
    dump("session", data)
    if not data or not data.get("url"):
        raise RuntimeError("Connect session response missing hosted URL")
    return data


def wait_for_oauth(session_id: str) -> dict:
    step(3, "Wait for OAuth completion")
    print(
        "  Open the URL above in a browser (logged in as the end user you want\n"
        "  to connect), approve LinkedIn, then come back. Polling every 3s\n"
        f"  until Meta marks the session complete (timeout {OAUTH_TIMEOUT}s).\n"
    )
    deadline = time.time() + OAUTH_TIMEOUT
    last_status = None
    while time.time() < deadline:
        resp = _request("GET", f"/v1/connect/sessions/{session_id}")
        data = resp.get("data") if isinstance(resp, dict) else None
        status = (data or {}).get("status")
        if status != last_status:
            print(f"  status={status}")
            last_status = status
        if status == "completed":
            dump("completed session", data)
            account_id = data.get("completed_social_account_id")
            if not account_id:
                raise RuntimeError("Session completed but no social_account_id — internal bug?")
            return data
        if status in {"expired", "failed"}:
            raise RuntimeError(f"Session ended with status={status}: {data}")
        time.sleep(3)
    raise RuntimeError(f"Timed out after {OAUTH_TIMEOUT}s waiting for OAuth")


def publish_post(account_id: str) -> dict:
    step(4, "Publish a text post to LinkedIn")
    # Legacy shape: caption + account_ids. Server fans this out into
    # one social_post_result per account.
    body = {
        "caption": POST_TEXT,
        "account_ids": [account_id],
        "idempotency_key": f"wl-test-{uuid.uuid4()}",
    }
    print(f"  caption: {POST_TEXT!r}")
    resp = _request("POST", "/v1/social-posts", body)
    data = resp.get("data") if isinstance(resp, dict) else None
    dump("post", data)
    if not data or not data.get("id"):
        raise RuntimeError("Publish response missing post id")
    return data


def wait_for_post_terminal(post_id: str) -> dict:
    step(5, "Wait for post to reach terminal state")
    deadline = time.time() + POST_TIMEOUT
    last_status = None
    while time.time() < deadline:
        resp = _request("GET", f"/v1/social-posts/{post_id}")
        data = resp.get("data") if isinstance(resp, dict) else None
        status = (data or {}).get("status")
        if status != last_status:
            print(f"  status={status}")
            last_status = status
        if status in {"published", "failed", "partial"}:
            dump("final post", data)
            return data
        time.sleep(2)
    raise RuntimeError(f"Post did not terminate within {POST_TIMEOUT}s — last status={last_status}")


def fetch_analytics(post_id: str) -> None:
    step(6, "Fetch analytics snapshot (may be empty on brand-new posts)")
    try:
        resp = _request("GET", f"/v1/social-posts/{post_id}/analytics")
        dump("analytics", resp.get("data") if isinstance(resp, dict) else resp)
    except APIError as e:
        print(f"  analytics fetch non-fatal failure (API {e.status}): {e.body}")


def main() -> int:
    require_env()
    print(f"API base : {API_BASE}")
    print(f"Profile  : {PROFILE_ID}")
    try:
        check_api_key()
        session = create_connect_session()
        print(f"\n  >>> Open this URL in your browser to authorize LinkedIn:\n  {session['url']}\n")
        completed = wait_for_oauth(session["id"])
        account_id = completed["completed_social_account_id"]
        post = publish_post(account_id)
        final = wait_for_post_terminal(post["id"])
        fetch_analytics(post["id"])

        print("\n── Summary " + "─" * 60)
        print(f"  profile_id      : {PROFILE_ID}")
        print(f"  external_user_id: {EXTERNAL_USER_ID}")
        print(f"  session_id      : {session['id']}")
        print(f"  account_id      : {account_id}")
        print(f"  post_id         : {post['id']}")
        print(f"  final status    : {final.get('status')}")
        results = final.get("results") or []
        for r in results:
            print(
                f"    result: platform={r.get('platform')} status={r.get('status')} "
                f"url={r.get('url')} error={r.get('error_message')}"
            )
        return 0
    except APIError as e:
        print(f"\n❌ API error {e.status}:")
        dump("body", e.body)
        return 1
    except Exception as e:  # noqa: BLE001
        print(f"\n❌ {type(e).__name__}: {e}")
        return 1


if __name__ == "__main__":
    sys.exit(main())
