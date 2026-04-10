"""
unipost Python SDK Validation Test

Tests all supported API operations against the live UniPost API.

Setup:
  1. pip install unipost
  2. Get an API key from https://app.unipost.dev → API Keys
  3. UNIPOST_API_KEY=up_live_xxx python unipost_sdk_test.py

Or with TEST_ACCOUNT_ID to test post creation:
  UNIPOST_API_KEY=up_live_xxx TEST_ACCOUNT_ID=<id> python unipost_sdk_test.py
"""

import os
import sys
from datetime import datetime, timedelta, timezone

from unipost import UniPost

# ─── Config ───────────────────────────────────────────────────────────────────

API_KEY = os.environ.get("UNIPOST_API_KEY", "")
TEST_ACCOUNT_ID = os.environ.get("TEST_ACCOUNT_ID", "")

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

    # ── 2. Posts — list & get ──────────────────────────────────────────────────
    section("2. Posts — list & get")

    posts = test("posts.list()", lambda: _test_list_posts(client))

    if posts and len(posts) > 0:
        first = posts[0]
        caption_preview = (first.caption or "")[:60]
        print(f'\n  First post: "{caption_preview}..."')
        print(f"  Status: {first.status}  |  Results: {len(first.results)}")

        test(f'posts.get("{first.id[:8]}...")', lambda: _test_get_post(client, first.id))
    else:
        print("\n  No posts yet — skipping posts.get() test")

    # ── 3. Posts — create ──────────────────────────────────────────────────────
    section("3. Posts — create (draft mode, no actual publishing)")

    if not TEST_ACCOUNT_ID:
        print("  ⏭  Skipped — set TEST_ACCOUNT_ID env var to run post creation tests")
    else:
        timestamp = datetime.now(timezone.utc).isoformat()
        caption = f"SDK validation test — {timestamp} [auto-generated]"

        draft = test("posts.create() — draft mode", lambda: _test_create_draft(client, caption))
        if draft:
            print(f"\n  Created draft post: {draft.id}")

        scheduled_at = (datetime.now(timezone.utc) + timedelta(minutes=10)).isoformat()
        scheduled = test(
            "posts.create() — scheduled mode",
            lambda: _test_create_scheduled(client, f"SDK scheduled — {timestamp}", scheduled_at),
        )
        if scheduled:
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

    # ── 4. Analytics ───────────────────────────────────────────────────────────
    section("4. Analytics")

    now = datetime.now(timezone.utc)
    thirty_days_ago = now - timedelta(days=30)

    test(
        "analytics.rollup() — last 30 days",
        lambda: _test_analytics(client, thirty_days_ago.isoformat(), now.isoformat()),
    )

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
        print("🎉 All tests passed! unipost (Python) is working correctly.\n")


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


def _test_get_post(client, post_id):
    post = client.posts.get(post_id)
    if not post or not post.id:
        raise ValueError("Expected post with id")
    return post


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


if __name__ == "__main__":
    main()
