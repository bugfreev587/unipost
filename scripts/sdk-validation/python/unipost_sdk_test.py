"""
unipost Python SDK Validation Test

Tests all supported API operations against the live UniPost API.

Setup:
  1. pip install -r requirements.txt
  2. Get an API key from https://app.unipost.dev → API Keys
  3. UNIPOST_API_KEY=up_live_xxx python unipost_sdk_test.py

Or with TEST_ACCOUNT_ID to test post creation:
  UNIPOST_API_KEY=up_live_xxx TEST_ACCOUNT_ID=<id> python unipost_sdk_test.py
"""

import os
import sys
import hmac
import hashlib
from datetime import datetime, timedelta, timezone
import requests as _requests

sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), "../../../sdk/python")))

from unipost import UniPost, verify_webhook_signature

# ─── Config ───────────────────────────────────────────────────────────────────

API_KEY = os.environ.get("UNIPOST_API_KEY", "")
API_URL = os.environ.get("UNIPOST_API_URL", "https://api.unipost.dev")
TEST_ACCOUNT_ID = os.environ.get("TEST_ACCOUNT_ID", "")

# Track post IDs created during the test run for cleanup.
created_post_ids = []
created_webhook_ids = []

# ─── Test runner ──────────────────────────────────────────────────────────────

passed = 0
failed = 0
failures = []


def test(name, fn):
    global passed, failed
    print(f"  {name} ... ", end="", flush=True)
    try:
        result = fn()
        print("✅ PASS")
        passed += 1
        return result
    except Exception as e:
        print(f"❌ FAIL — {e}")
        failed += 1
        failures.append((name, str(e)))
        return None


def section(title):
    print(f"\n{'─' * 50}")
    print(f"  {title}")
    print("─" * 50)


# ─── Main ─────────────────────────────────────────────────────────────────────

def main():
    print("\n╔══════════════════════════════════════════════════╗")
    print("║     unipost (Python) — API Validation Test       ║")
    print("╚══════════════════════════════════════════════════╝\n")

    if not API_KEY:
        print("❌ Please set UNIPOST_API_KEY environment variable")
        print("   UNIPOST_API_KEY=up_live_xxx python unipost_sdk_test.py")
        sys.exit(1)

    client = UniPost(api_key=API_KEY)

    # ── 1. Accounts ────────────────────────────────────────────────────────────
    section("1. Accounts — list connected social accounts")

    accounts = test("accounts.list()", lambda: _test_list_accounts(client))

    if accounts:
        print(f"\n  Found {len(accounts)} connected accounts:")
        for a in accounts:
            name = a.account_name or a.id
            print(f"    • [{a.platform:<10}] {name}  (id: {a.id})")

        bluesky = next((a for a in accounts if a.platform == "bluesky"), None)
        if bluesky and not TEST_ACCOUNT_ID:
            print(f"\n  💡 Tip: Run with TEST_ACCOUNT_ID={bluesky.id} to test post creation")
        if not TEST_ACCOUNT_ID:
            candidate = bluesky or accounts[0]
            globals()["TEST_ACCOUNT_ID"] = candidate.id
            print(f"\n  Using TEST_ACCOUNT_ID={candidate.id} for safe draft/scheduled tests")

    # ── 2. Profiles — raw API smoke ────────────────────────────────────────────
    section("2. Profiles — list & filter accounts by profile")

    profiles = test("GET /v1/profiles", lambda: _test_profiles())
    if profiles:
        print(f"\n  Found {len(profiles)} profiles:")
        for p in profiles:
            print(f"    • {p['name']}  (id: {p['id']})")
        first = profiles[0]
        test(
            f"GET /v1/social-accounts?profile_id={first['id'][:8]}...",
            lambda: _test_profile_accounts(first["id"], first["name"]),
        )

    # ── 3. Webhooks — signature + CRUD ────────────────────────────────────────
    section("3. Webhooks — signature verification & subscription CRUD")

    test("verify_webhook_signature()", _test_verify_signature)

    webhook = test("webhooks.create()", lambda: client.webhooks.create(
        url="https://example.com/unipost-webhook-test",
        events=["post.published", "post.partial", "post.failed"],
    ))
    if webhook:
        created_webhook_ids.append(webhook.id)
        test("webhooks.list()", lambda: _test_list_webhooks(client, webhook.id))
        test(f'webhooks.get("{webhook.id[:8]}...")', lambda: _test_get_webhook(client, webhook.id))
        test("webhooks.update()", lambda: _test_update_webhook(client, webhook.id))
        test("webhooks.rotate()", lambda: _test_rotate_webhook(client, webhook.id))

    # ── 4. Posts — list & get ──────────────────────────────────────────────────
    section("4. Posts — list & get")

    posts = test("posts.list()", lambda: _test_list_posts(client))

    if posts and len(posts) > 0:
        first = posts[0]
        caption_preview = (first.caption or "")[:60]
        print(f'\n  First post: "{caption_preview}..."')
        print(f"  Status: {first.status}  |  Results: {len(first.results)}")

        test(f'posts.get("{first.id[:8]}...")', lambda: _test_get_post(client, first.id))
        test(f'posts.get_queue("{first.id[:8]}...")', lambda: _test_get_queue(client, first.id))
    else:
        print("\n  No posts yet — skipping posts.get() and posts.get_queue() tests")

    # ── 5. Posts — create ──────────────────────────────────────────────────────
    section("5. Posts — create (draft mode, no actual publishing)")

    if not TEST_ACCOUNT_ID:
        print("  ⏭  Skipped — set TEST_ACCOUNT_ID env var to run post creation tests")
    else:
        timestamp = datetime.now(timezone.utc).isoformat()
        caption = f"SDK validation test — {timestamp} [auto-generated]"

        draft = test("posts.create() — draft mode", lambda: _test_create_draft(client, caption))
        if draft:
            created_post_ids.append(draft.id)
            print(f"\n  Created draft post: {draft.id}")

        scheduled_at = (datetime.now(timezone.utc) + timedelta(minutes=10)).isoformat()
        scheduled = test(
            "posts.create() — scheduled mode",
            lambda: _test_create_scheduled(client, f"SDK scheduled — {timestamp}", scheduled_at),
        )
        if scheduled:
            created_post_ids.append(scheduled.id)
            print(f"  Created scheduled post: {scheduled.id}")
            test(
                f'posts.cancel("{scheduled.id[:8]}...")',
                lambda: _test_cancel(client, scheduled.id),
            )
            print("  Cancelled scheduled post ✓")

        publish_now = os.environ.get("TEST_PUBLISH_NOW") == "true"
        if publish_now:
            test(
                "posts.create() — publish NOW",
                lambda: _test_create_now(client, f"[SDK Test] Hello from unipost Python 🐍 {timestamp}"),
            )
        else:
            print("\n  ⏭  Real publish skipped (set TEST_PUBLISH_NOW=true to enable)")

    # ── 6. Analytics ───────────────────────────────────────────────────────────
    section("6. Analytics")

    now = datetime.now(timezone.utc)
    thirty_days_ago = now - timedelta(days=30)

    test(
        "analytics.rollup() — last 30 days",
        lambda: _test_analytics(client, thirty_days_ago.isoformat(), now.isoformat()),
    )

    # ── Cleanup ────────────────────────────────────────────────────────────────
    if created_webhook_ids or created_post_ids:
        section("7. Cleanup")
    for wid in created_webhook_ids:
        try:
            client.webhooks.delete(wid)
            print(f"  🧹 Deleted webhook {wid[:8]}...")
        except Exception as e:
            print(f"  ⚠  Failed to delete webhook {wid[:8]}... ({e})")
    if created_post_ids:
        for pid in created_post_ids:
            try:
                resp = _requests.delete(
                    f"{API_URL}/v1/social-posts/{pid}",
                    headers={"Authorization": f"Bearer {API_KEY}"},
                )
                resp.raise_for_status()
                print(f"  🗑  Deleted {pid[:8]}...")
            except Exception as e:
                print(f"  ⚠  Failed to delete {pid[:8]}... ({e})")

    # ── Summary ────────────────────────────────────────────────────────────────
    print(f"\n╔══════════════════════════════════════════════════╗")
    print(f"║  Results: {passed:2d} passed  {failed:2d} failed                    ║")
    print(f"╚══════════════════════════════════════════════════╝\n")

    if failed > 0:
        print("Failed tests:")
        for name, err in failures:
            print(f"  ❌ {name}: {err}")
        sys.exit(1)
    else:
        print("🎉 All tests passed! local unipost (Python) is working correctly.\n")


# ─── Test implementations ─────────────────────────────────────────────────────

def _test_list_accounts(client):
    res = client.accounts.list()
    data = res.get("data", [])
    if not isinstance(data, list):
        raise ValueError("Expected data array")
    return data


def _test_list_posts(client):
    res = client.posts.list(limit=5)
    data = res.get("data", [])
    if not isinstance(data, list):
        raise ValueError("Expected data array")
    return data


def _test_profiles():
    resp = _requests.get(
        f"{API_URL}/v1/profiles",
        headers={"Authorization": f"Bearer {API_KEY}"},
        timeout=30,
    )
    resp.raise_for_status()
    return resp.json().get("data", [])


def _test_profile_accounts(profile_id, profile_name):
    resp = _requests.get(
        f"{API_URL}/v1/social-accounts",
        params={"profile_id": profile_id},
        headers={"Authorization": f"Bearer {API_KEY}"},
        timeout=30,
    )
    resp.raise_for_status()
    data = resp.json().get("data", [])
    print(f'    → {len(data)} accounts in profile "{profile_name}"')
    return data


def _test_get_post(client, post_id):
    post = client.posts.get(post_id)
    if not post or not post.id:
        raise ValueError("Expected post with id")
    return post


def _test_get_queue(client, post_id):
    queue = client.posts.get_queue(post_id)
    if not queue or not queue.post or not queue.post.id:
        raise ValueError("Expected queue snapshot")
    return queue


def _test_create_draft(client, caption):
    post = client.posts.create(
        caption=caption,
        account_ids=[TEST_ACCOUNT_ID],
        status="draft",
    )
    if not post or not post.id:
        raise ValueError("Expected post with id")
    if post.status != "draft":
        raise ValueError(f"Expected status=draft, got {post.status}")
    return post


def _test_create_scheduled(client, caption, scheduled_at):
    post = client.posts.create(
        caption=caption,
        account_ids=[TEST_ACCOUNT_ID],
        scheduled_at=scheduled_at,
    )
    if not post or not post.id:
        raise ValueError("Expected post with id")
    if post.status != "scheduled":
        raise ValueError(f"Expected status=scheduled, got {post.status}")
    return post


def _test_cancel(client, post_id):
    post = client.posts.cancel(post_id)
    if not post:
        raise ValueError("Expected response")
    return post


def _test_create_now(client, caption):
    post = client.posts.create(
        caption=caption,
        account_ids=[TEST_ACCOUNT_ID],
    )
    if not post or not post.id:
        raise ValueError("Expected post with id")
    return post


def _test_analytics(client, from_date, to_date):
    rollup = client.analytics.rollup(
        from_date=from_date,
        to_date=to_date,
        granularity="day",
    )
    if not rollup:
        raise ValueError("Expected rollup data")
    return rollup


def _test_verify_signature():
    payload = b'{"event":"post.published","data":{"id":"post_test_123"}}'
    secret = "whsec_test_local"
    signature = "sha256=" + hmac.new(secret.encode("utf-8"), payload, hashlib.sha256).hexdigest()
    if not verify_webhook_signature(payload, signature, secret):
        raise ValueError("Expected signature to verify")
    return True


def _test_list_webhooks(client, webhook_id):
    payload = client.webhooks.list()
    data = payload.get("data", [])
    if not any(item.id == webhook_id for item in data):
        raise ValueError("Created webhook not found in list")
    return data


def _test_get_webhook(client, webhook_id):
    item = client.webhooks.get(webhook_id)
    if item.id != webhook_id:
        raise ValueError("Wrong webhook returned")
    if hasattr(item, "secret"):
        raise ValueError("Read response should not contain plaintext secret")
    return item


def _test_update_webhook(client, webhook_id):
    item = client.webhooks.update(webhook_id, active=False, events=["post.failed"])
    if item.active is not False:
        raise ValueError("Expected active=False")
    if item.events != ["post.failed"]:
        raise ValueError("Expected updated events")
    return item


def _test_rotate_webhook(client, webhook_id):
    item = client.webhooks.rotate(webhook_id)
    if not item.secret.startswith("whsec_"):
        raise ValueError("Expected rotated secret")
    return item


if __name__ == "__main__":
    main()
