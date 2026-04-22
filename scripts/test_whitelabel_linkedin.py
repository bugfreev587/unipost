#!/usr/bin/env python3
"""
End-to-end validation of the UniPost white-label API flow against
LinkedIn in production.

Baseline steps (always run):
  1. API-key sanity check (GET /v1/profiles/<id>)
  2. Create a Connect session → print branded Connect URL
  3. Pause for you to complete OAuth in a browser
  4. Poll the session until it completes + discover the
     social_account_id LinkedIn minted for the external user
  5. Publish a text-only LinkedIn post
  6. Poll the post until dispatching clears
  7. Fetch the post's analytics snapshot once

Opt-in extras (via flags):
  --test-branding
      Before Step 2, PATCH the profile with test logo / display name /
      primary color so the Connect page in Step 3 renders with your
      branding. Original values are restored at the very end even on
      failure.

  --wait-analytics SECONDS
      After Step 7, sleep that long and re-fetch analytics so you can
      see whether the background refresh worker has populated data.
      LinkedIn's metrics commonly lag several minutes.

  --cleanup
      At the end, archive the published post via
      POST /v1/social-posts/<id>/archive. The account stays connected
      regardless — delete it from Dashboard if you want a clean slate.

Usage:
  export UNIPOST_API_KEY='<your workspace API key>'
  export UNIPOST_PROFILE_ID='<your profile UUID>'
  python3 scripts/test_whitelabel_linkedin.py --test-branding --wait-analytics 180 --cleanup
"""

import argparse
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

# Test-branding preset — obvious enough you can tell it's the test run
# rendering the Connect page, not whatever you've saved long-term.
TEST_BRANDING = {
    "branding_logo_url": "https://placehold.co/120x40/10b981/ffffff?text=WL-TEST",
    "branding_display_name": "White-Label Test Harness",
    "branding_primary_color": "#d97706",
}


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
    print(f"\n── Step {n}: {title} " + "─" * max(1, 60 - len(title)))


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


def fetch_profile() -> dict:
    resp = _request("GET", f"/v1/profiles/{PROFILE_ID}")
    return (resp or {}).get("data") or {}


def patch_branding(values: dict) -> dict:
    resp = _request("PATCH", f"/v1/profiles/{PROFILE_ID}", values)
    return (resp or {}).get("data") or {}


def check_api_key() -> dict:
    step(1, "Verify API key + profile")
    data = fetch_profile()
    dump("profile", data)
    if not data.get("id"):
        raise RuntimeError("Profile payload didn't include an id — check API key + profile id")
    return data


def snapshot_branding(profile: dict) -> dict:
    """Extract the three branding fields so we can put them back at the
    end. Missing values restore as empty strings (which the server
    handles by leaving the column unchanged via NULLIF / COALESCE).
    """
    return {
        "branding_logo_url": profile.get("branding_logo_url") or "",
        "branding_display_name": profile.get("branding_display_name") or "",
        "branding_primary_color": profile.get("branding_primary_color") or "",
    }


def apply_test_branding(original: dict) -> dict:
    step(2, "Apply test branding to profile")
    print("  original:")
    for k, v in original.items():
        print(f"    {k}: {v!r}")
    print("  new:")
    for k, v in TEST_BRANDING.items():
        print(f"    {k}: {v!r}")
    updated = patch_branding(TEST_BRANDING)
    dump("profile after PATCH", updated)
    return updated


def restore_branding(original: dict) -> None:
    step(99, "Restore original branding")
    try:
        _ = patch_branding(original)
        print("  restored")
    except APIError as e:
        print(f"  warning: could not restore branding (API {e.status}): {e.body}")


def create_connect_session() -> dict:
    step(3, "Create LinkedIn Connect session")
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
    step(4, "Wait for OAuth completion")
    print(
        "  Open the URL above in a browser (logged in as the end user you want\n"
        "  to connect), approve LinkedIn, then come back. Polling every 3s\n"
        f"  until the session flips to completed (timeout {OAUTH_TIMEOUT}s).\n"
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
    step(5, "Publish a text post to LinkedIn")
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
    step(6, "Wait for post to reach terminal state")
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


def fetch_analytics(post_id: str, label: str) -> Any:
    step(7, f"Fetch analytics snapshot ({label})")
    try:
        resp = _request("GET", f"/v1/social-posts/{post_id}/analytics")
        data = resp.get("data") if isinstance(resp, dict) else resp
        dump("analytics", data)
        return data
    except APIError as e:
        print(f"  analytics fetch non-fatal failure (API {e.status}): {e.body}")
        return None


def wait_and_refetch_analytics(post_id: str, seconds: int) -> None:
    step(8, f"Wait {seconds}s then re-fetch analytics")
    print(
        "  LinkedIn's analytics refresh worker runs on a tier-based TTL. If\n"
        "  the first fetch was empty (typical) this extra pass gives it room\n"
        "  to populate before you declare the feature broken."
    )
    for remaining in range(seconds, 0, -10):
        print(f"  sleeping… {remaining}s left")
        time.sleep(min(10, remaining))
    fetch_analytics(post_id, "after wait")


def archive_post(post_id: str) -> None:
    step(9, "Archive the published post (cleanup)")
    try:
        _ = _request("POST", f"/v1/social-posts/{post_id}/archive")
        print(f"  archived {post_id}")
    except APIError as e:
        print(f"  warning: archive failed (API {e.status}): {e.body}")


def main() -> int:
    parser = argparse.ArgumentParser(
        description="UniPost white-label API end-to-end test (LinkedIn)"
    )
    parser.add_argument(
        "--test-branding",
        action="store_true",
        help="Set test logo / display name / color on the profile so the "
             "Connect page renders branded, then restore the original "
             "values at the end.",
    )
    parser.add_argument(
        "--wait-analytics",
        type=int,
        default=0,
        metavar="SECONDS",
        help="After the first analytics fetch, sleep this long and fetch "
             "again. Useful when LinkedIn's numbers lag a few minutes.",
    )
    parser.add_argument(
        "--cleanup",
        action="store_true",
        help="Archive the published post at the end so it doesn't clutter "
             "the Dashboard's Posts list.",
    )
    args = parser.parse_args()

    require_env()
    print(f"API base : {API_BASE}")
    print(f"Profile  : {PROFILE_ID}")
    if args.test_branding:
        print("Flags    : --test-branding")
    if args.wait_analytics:
        print(f"Flags    : --wait-analytics {args.wait_analytics}")
    if args.cleanup:
        print("Flags    : --cleanup")

    original_branding: Optional[dict] = None
    try:
        profile = check_api_key()
        if args.test_branding:
            original_branding = snapshot_branding(profile)
            apply_test_branding(original_branding)

        session = create_connect_session()
        print(f"\n  >>> Open this URL in your browser to authorize LinkedIn:\n  {session['url']}\n")
        if args.test_branding:
            print(
                "      ^ The page should render with the test logo / color you "
                "just set.\n"
                "        If it still looks default, hard-refresh — there's no "
                "client-side cache, so it'll pick up the PATCH on next load."
            )

        completed = wait_for_oauth(session["id"])
        account_id = completed["completed_social_account_id"]
        post = publish_post(account_id)
        final = wait_for_post_terminal(post["id"])
        fetch_analytics(post["id"], "initial")
        if args.wait_analytics > 0:
            wait_and_refetch_analytics(post["id"], args.wait_analytics)
        if args.cleanup:
            archive_post(post["id"])

        print("\n── Summary " + "─" * 60)
        print(f"  profile_id      : {PROFILE_ID}")
        print(f"  external_user_id: {EXTERNAL_USER_ID}")
        print(f"  session_id      : {session['id']}")
        print(f"  account_id      : {account_id}")
        print(f"  post_id         : {post['id']}")
        print(f"  final status    : {final.get('status')}")
        for r in final.get("results") or []:
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
    finally:
        if args.test_branding and original_branding is not None:
            restore_branding(original_branding)


if __name__ == "__main__":
    sys.exit(main())
