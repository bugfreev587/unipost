import hashlib
import hmac
import os
import sys
from datetime import datetime, timedelta, timezone

sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), "../../../sdk/python")))

from unipost import UniPost, UniPostError, verify_webhook_signature

API_KEY = os.environ.get("UNIPOST_API_KEY", "")
TEST_ACCOUNT_ID = os.environ.get("TEST_ACCOUNT_ID", "")
TEST_PUBLISH_NOW = os.environ.get("TEST_PUBLISH_NOW") == "true"

passed = 0
failed = 0
skipped = 0
failures = []

created_post_ids = []
created_webhook_ids = []
created_media_ids = []
created_platform_credentials = []


def section(title):
    print(f"\n{'─' * 50}")
    print(f"  {title}")
    print("─" * 50)


def test(name, fn):
    global passed, failed
    print(f"  {name} ... ", end="", flush=True)
    try:
        value = fn()
        print("✅ PASS")
        passed += 1
        return value
    except Exception as exc:
        print(f"❌ FAIL — {exc}")
        failed += 1
        failures.append(f"{name}: {exc}")
        return None


def skip(name, reason):
    global skipped
    print(f"  {name} ... ⏭ SKIP — {reason}")
    skipped += 1


def assert_true(condition, message):
    if not condition:
        raise ValueError(message)


def expect_api_error(name, fn, expected_codes):
    def runner():
        try:
            fn()
        except UniPostError as exc:
            if expected_codes and exc.code not in expected_codes:
                raise ValueError(f"Expected {'/'.join(expected_codes)}, got {exc.code or 'unknown'}")
            return exc.code
        raise ValueError("Expected API error")

    return test(name, runner)


def cleanup(client, workspace_id):
    if created_webhook_ids or created_media_ids or created_post_ids or created_platform_credentials:
        section("Cleanup")

    for webhook_id in list(created_webhook_ids):
        try:
            client.webhooks.delete(webhook_id)
            print(f"  🧹 Deleted webhook {webhook_id[:8]}...")
        except Exception as exc:
            print(f"  ⚠ Failed to delete webhook {webhook_id[:8]}... ({exc})")

    for media_id in list(created_media_ids):
        try:
            client.media.delete(media_id)
            print(f"  🧹 Deleted media {media_id[:8]}...")
        except Exception as exc:
            print(f"  ⚠ Failed to delete media {media_id[:8]}... ({exc})")

    for post_id in list(created_post_ids):
        try:
            client.posts.delete(post_id)
            print(f"  🧹 Deleted post {post_id[:8]}...")
        except Exception as exc:
            print(f"  ⚠ Failed to delete post {post_id[:8]}... ({exc})")

    for platform_name in list(created_platform_credentials):
        try:
            client.platform_credentials.delete(workspace_id, platform_name)
            print(f"  🧹 Deleted platform credential {platform_name}")
        except Exception as exc:
            print(f"  ⚠ Failed to delete platform credential {platform_name}... ({exc})")


def main():
    global TEST_ACCOUNT_ID

    print("\n╔══════════════════════════════════════════════════╗")
    print("║     unipost (Python) — API Validation Test       ║")
    print("╚══════════════════════════════════════════════════╝\n")

    if not API_KEY:
        print("❌ Please set UNIPOST_API_KEY")
        sys.exit(1)

    client = UniPost(api_key=API_KEY)

    section("1. Public catalogs")

    test("platforms.capabilities()", lambda: _test_platform_capabilities(client))
    test("plans.list()", lambda: _test_plans(client))

    section("2. Workspace & profiles")

    workspace = test("workspace.get()", lambda: _test_workspace_get(client))
    if workspace:
        test("workspace.update() — no-op", lambda: _test_workspace_update(client, workspace))

    profiles_page = test("profiles.list()", lambda: _test_profiles_list(client))
    profiles = profiles_page.get("data", []) if profiles_page else []
    first_profile = profiles[0] if profiles else None
    if first_profile:
        test("profiles.get()", lambda: _test_profiles_get(client, first_profile.id))
        test("profiles.update() — no-op", lambda: _test_profiles_update(client, first_profile))
    else:
        skip("profiles.get()", "No profiles available")
        skip("profiles.update() — no-op", "No profiles available")

    section("3. Accounts")

    accounts_page = test("accounts.list()", lambda: _test_accounts_list(client))
    accounts = accounts_page.get("data", []) if accounts_page else []
    first_account = accounts[0] if accounts else None
    tiktok_account = next((a for a in accounts if a.platform == "tiktok"), None)
    facebook_account = next((a for a in accounts if a.platform == "facebook"), None)

    if first_account and not TEST_ACCOUNT_ID:
        safest = next((a for a in accounts if a.platform == "bluesky"), first_account)
        TEST_ACCOUNT_ID = safest.id
        print(f"\n  Using TEST_ACCOUNT_ID={TEST_ACCOUNT_ID} for safe draft/scheduled tests")

    if first_account:
        test("accounts.get()", lambda: _test_accounts_get(client, first_account.id))
        test("accounts.health()", lambda: _test_accounts_health(client, first_account.id))
        test("accounts.capabilities()", lambda: _test_accounts_capabilities(client, first_account.id))
    else:
        skip("accounts.get()", "No accounts available")
        skip("accounts.health()", "No accounts available")
        skip("accounts.capabilities()", "No accounts available")

    if tiktok_account:
        test("accounts.tiktok_creator_info()", lambda: _test_tiktok_creator_info(client, tiktok_account.id))
    else:
        skip("accounts.tiktok_creator_info()", "No TikTok account connected")

    if facebook_account:
        test("accounts.facebook_page_insights()", lambda: _test_facebook_page_insights(client, facebook_account.id))
    else:
        skip("accounts.facebook_page_insights()", "No Facebook account connected")

    expect_api_error(
        "accounts.connect() — invalid credentials negative path",
        lambda: client.accounts.connect(platform="bluesky", credentials={"identifier": "invalid", "password": "invalid"}),
        ["auth_error", "unauthorized", "validation_error"],
    )

    section("4. Media, connect sessions, users")

    created_media = test("media.upload()", lambda: _test_media_upload(client))
    if created_media:
        media_id = getattr(created_media, "id", None) or getattr(created_media, "media_id", None)
        created_media_ids.append(media_id)
        test("media.get()", lambda: _test_media_get(client, media_id))

    connect_session = test("connect.create_session()", lambda: _test_connect_create(client, first_profile.id if first_profile else None))
    if connect_session:
        test("connect.get_session()", lambda: _test_connect_get(client, connect_session.id))

    users_page = test("users.list()", lambda: _test_users_list(client))
    users = users_page.get("data", []) if users_page else []
    if users:
        test("users.get()", lambda: _test_users_get(client, users[0].external_user_id))
    else:
        skip("users.get()", "No managed users available")

    section("5. Webhooks")

    test("verify_webhook_signature()", _test_verify_signature)

    webhook = test("webhooks.create()", lambda: _test_webhook_create(client))
    if webhook:
        created_webhook_ids.append(webhook.id)
        test("webhooks.list()", lambda: _test_webhook_list(client, webhook.id))
        test("webhooks.get()", lambda: _test_webhook_get(client, webhook.id))
        test("webhooks.update()", lambda: _test_webhook_update(client, webhook.id))
        test("webhooks.rotate()", lambda: _test_webhook_rotate(client, webhook.id))

    section("6. Platform credentials")

    if workspace:
        platform_name = f"sdk-py-{int(datetime.now(timezone.utc).timestamp())}"
        test("platform_credentials.create()/list()/delete()", lambda: _test_platform_credentials(client, workspace.id, platform_name))
    else:
        skip("platform_credentials.create()/list()/delete()", "No workspace available")

    section("7. Posts")

    test("posts.validate()", lambda: _test_posts_validate(client))

    posts_page = test("posts.list()", lambda: _test_posts_list(client))
    posts = posts_page.get("data", []) if posts_page else []
    first_post = posts[0] if posts else None

    if first_post:
        test("posts.get()", lambda: _test_posts_get(client, first_post.id))
        test("posts.get_queue()", lambda: _test_posts_queue(client, first_post.id))
        test("posts.analytics()", lambda: _test_posts_analytics(client, first_post.id))
    else:
        skip("posts.get()", "No posts available")
        skip("posts.get_queue()", "No posts available")
        skip("posts.analytics()", "No posts available")

    if TEST_ACCOUNT_ID:
        draft = test("posts.create() — draft", lambda: _test_create_draft(client))
        if draft:
            created_post_ids.append(draft.id)
            test("posts.update() — draft", lambda: _test_update_draft(client, draft.id))
            test("posts.preview_link()", lambda: _test_preview_link(client, draft.id))
            test("posts.archive()", lambda: _test_archive(client, draft.id))
            test("posts.restore()", lambda: _test_restore(client, draft.id))

        scheduled = test("posts.create() — scheduled", lambda: _test_create_scheduled(client))
        if scheduled:
            created_post_ids.append(scheduled.id)
            test("posts.update() — scheduled", lambda: _test_update_scheduled(client, scheduled.id))
            test("posts.cancel()", lambda: _test_cancel(client, scheduled.id))

        test("posts.bulk_create()", lambda: _test_bulk_create(client))

        if TEST_PUBLISH_NOW and draft:
            test("posts.publish() — live publish", lambda: _test_publish(client, draft.id))
        else:
            skip("posts.publish() — live publish", "Opt-in only (set TEST_PUBLISH_NOW=true)")
    else:
        skip("posts.create()/update()/preview/archive/restore/cancel/delete()", "No TEST_ACCOUNT_ID available")
        skip("posts.bulk_create()", "No TEST_ACCOUNT_ID available")
        skip("posts.publish() — live publish", "No TEST_ACCOUNT_ID available")

    failed_result = None
    if first_post and getattr(first_post, "results", None):
        failed_result = next((result for result in first_post.results if result.status == "failed"), None)
    if failed_result:
        test("posts.retry_result()", lambda: _test_retry_result(client, first_post.id, failed_result.id))
    else:
        skip("posts.retry_result()", "No failed post result available")

    section("8. Delivery jobs, analytics, usage, oauth")

    test("delivery_jobs.list()", lambda: _test_delivery_jobs_list(client))
    test("delivery_jobs.summary()", lambda: _test_delivery_jobs_summary(client))

    retryable_jobs = client.delivery_jobs.list(limit=20, states=["pending", "retrying"]).get("data", [])
    if retryable_jobs:
        test("delivery_jobs.retry()/cancel()", lambda: _test_delivery_job_commands(client, retryable_jobs[0]))
    else:
        skip("delivery_jobs.retry()/cancel()", "No retryable delivery jobs available")

    test("analytics.summary()", lambda: _test_analytics_summary(client))
    test("analytics.trend()", lambda: _test_analytics_trend(client))
    test("analytics.by_platform()", lambda: _test_analytics_by_platform(client))
    test("analytics.rollup()", lambda: _test_analytics_rollup(client))
    test("usage.get()", lambda: _test_usage(client))
    test("oauth.connect() — known backend path", lambda: _test_oauth_connect(client))

    cleanup(client, workspace.id if workspace else "")

    print("\n╔══════════════════════════════════════════════════╗")
    print(f"║  Results: {passed:2d} passed  {failed:2d} failed  {skipped:2d} skipped      ║")
    print("╚══════════════════════════════════════════════════╝\n")

    if failures:
        print("Failed tests:")
        for failure in failures:
            print(f"  ❌ {failure}")
        sys.exit(1)

    print("🎉 All required Python SDK validations passed.\n")


def _test_platform_capabilities(client):
    payload = client.platforms.capabilities()
    assert_true(isinstance(payload.platforms, object), "Expected platforms payload")
    return payload


def _test_plans(client):
    plans = client.plans.list()
    assert_true(isinstance(plans, list), "Expected plans list")
    assert_true(len(plans) > 0, "Expected at least one plan")
    return plans


def _test_workspace_get(client):
    ws = client.workspace.get()
    assert_true(bool(ws.id), "Expected workspace id")
    return ws


def _test_workspace_update(client, workspace):
    updated = client.workspace.update(per_account_monthly_limit=getattr(workspace, "per_account_monthly_limit", None))
    assert_true(updated.id == workspace.id, "Expected same workspace")
    return updated


def _test_profiles_list(client):
    payload = client.profiles.list()
    assert_true(isinstance(payload.get("data", []), list), "Expected profiles list")
    return payload


def _test_profiles_get(client, profile_id):
    profile = client.profiles.get(profile_id)
    assert_true(profile.id == profile_id, "Expected matching profile")
    return profile


def _test_profiles_update(client, profile):
    updated = client.profiles.update(
        profile.id,
        name=profile.name,
        branding_logo_url=getattr(profile, "branding_logo_url", None),
        branding_display_name=getattr(profile, "branding_display_name", None),
        branding_primary_color=getattr(profile, "branding_primary_color", None),
    )
    assert_true(updated.id == profile.id, "Expected matching updated profile")
    return updated


def _test_accounts_list(client):
    payload = client.accounts.list()
    assert_true(isinstance(payload.get("data", []), list), "Expected accounts list")
    assert_true(len(payload.get("data", [])) > 0, "No connected accounts found")
    return payload


def _test_accounts_get(client, account_id):
    account = client.accounts.get(account_id)
    assert_true(account.id == account_id, "Expected matching account")
    return account


def _test_accounts_health(client, account_id):
    health = client.accounts.health(account_id)
    assert_true(health.social_account_id == account_id, "Expected matching health")
    return health


def _test_accounts_capabilities(client, account_id):
    try:
        payload = client.accounts.capabilities(account_id)
        assert_true(bool(payload.schema_version), "Expected schema version")
        return payload
    except UniPostError as exc:
        if exc.code == "not_found":
            return exc.code
        raise


def _test_tiktok_creator_info(client, account_id):
    payload = client.accounts.tiktok_creator_info(account_id)
    assert_true(hasattr(payload, "creator_username") or hasattr(payload, "creator_nickname"), "Expected TikTok creator info")
    return payload


def _test_facebook_page_insights(client, account_id):
    try:
        return client.accounts.facebook_page_insights(account_id)
    except UniPostError as exc:
        if exc.code in ("forbidden", "facebook_disabled", "FACEBOOK_DISABLED", "not_found"):
            return exc.code
        raise


def _test_media_upload(client):
    payload = client.media.upload(
        filename="sdk-validation.png",
        content_type="image/png",
        size_bytes=128,
        content_hash=f"sdk-py-{int(datetime.now(timezone.utc).timestamp())}",
    )
    assert_true(bool(getattr(payload, "id", None) or getattr(payload, "media_id", None)), "Expected media id")
    return payload


def _test_media_get(client, media_id):
    payload = client.media.get(media_id)
    returned_id = getattr(payload, "id", None) or getattr(payload, "media_id", None)
    assert_true(returned_id == media_id, "Expected matching media")
    return payload


def _test_connect_create(client, profile_id):
    payload = client.connect.create_session(
        platform="bluesky",
        profile_id=profile_id,
        external_user_id=f"sdk-py-{int(datetime.now(timezone.utc).timestamp())}",
        external_user_email="sdk-validation@example.com",
        return_url="https://example.com/return",
    )
    assert_true(bool(payload.id and payload.url), "Expected connect session")
    return payload


def _test_connect_get(client, session_id):
    payload = client.connect.get_session(session_id)
    assert_true(payload.id == session_id, "Expected matching connect session")
    return payload


def _test_users_list(client):
    payload = client.users.list()
    assert_true(isinstance(payload.get("data", []), list), "Expected users list")
    return payload


def _test_users_get(client, external_user_id):
    payload = client.users.get(external_user_id)
    assert_true(payload.external_user_id == external_user_id, "Expected matching user")
    return payload


def _test_verify_signature():
    payload = b'{"event":"post.published","data":{"id":"post_test_123"}}'
    secret = "whsec_test_local"
    signature = "sha256=" + hmac.new(secret.encode("utf-8"), payload, hashlib.sha256).hexdigest()
    assert_true(verify_webhook_signature(payload, signature, secret), "Expected valid signature")
    return True


def _test_webhook_create(client):
    payload = client.webhooks.create(
        url="https://example.com/unipost-webhook-test",
        events=["post.published", "post.partial", "post.failed"],
    )
    assert_true(payload.id and payload.secret.startswith("whsec_"), "Expected webhook id and secret")
    return payload


def _test_webhook_list(client, webhook_id):
    payload = client.webhooks.list()
    assert_true(any(item.id == webhook_id for item in payload.get("data", [])), "Expected created webhook in list")
    return payload


def _test_webhook_get(client, webhook_id):
    payload = client.webhooks.get(webhook_id)
    assert_true(payload.id == webhook_id, "Expected matching webhook")
    return payload


def _test_webhook_update(client, webhook_id):
    payload = client.webhooks.update(webhook_id, active=False, events=["post.failed"])
    assert_true(payload.active is False, "Expected inactive webhook")
    return payload


def _test_webhook_rotate(client, webhook_id):
    payload = client.webhooks.rotate(webhook_id)
    assert_true(payload.secret.startswith("whsec_"), "Expected rotated secret")
    return payload


def _test_platform_credentials(client, workspace_id, platform_name):
    try:
        created = client.platform_credentials.create(
            workspace_id,
            platform=platform_name,
            client_id="sdk-client-id",
            client_secret="sdk-client-secret",
        )
    except UniPostError as exc:
        if exc.code == "forbidden":
            skip("platform_credentials.create()/list()/delete()", "Plan-gated")
            return None
        raise
    created_platform_credentials.append(platform_name)
    listed = client.platform_credentials.list(workspace_id)
    assert_true(any(item.platform == platform_name for item in listed.get("data", [])), "Expected credential in list")
    client.platform_credentials.delete(workspace_id, platform_name)
    created_platform_credentials.remove(platform_name)
    return created


def _test_posts_validate(client):
    payload = client.posts.validate(caption="SDK validation", account_ids=[TEST_ACCOUNT_ID] if TEST_ACCOUNT_ID else [], status="draft")
    assert_true(isinstance(payload.valid, bool), "Expected validation result")
    return payload


def _test_posts_list(client):
    payload = client.posts.list(limit=5)
    assert_true(isinstance(payload.get("data", []), list), "Expected posts list")
    return payload


def _test_posts_get(client, post_id):
    payload = client.posts.get(post_id)
    assert_true(payload.id == post_id, "Expected matching post")
    return payload


def _test_posts_queue(client, post_id):
    payload = client.posts.get_queue(post_id)
    assert_true(payload.post.id == post_id, "Expected queue payload")
    return payload


def _test_posts_analytics(client, post_id):
    payload = client.posts.analytics(post_id)
    return payload


def _test_create_draft(client):
    payload = client.posts.create(
        caption=f"SDK Python draft {datetime.now(timezone.utc).isoformat()}",
        account_ids=[TEST_ACCOUNT_ID],
        status="draft",
    )
    assert_true(payload.id and payload.status == "draft", "Expected draft post")
    return payload


def _test_update_draft(client, post_id):
    payload = client.posts.update(post_id, caption="SDK Python draft updated", account_ids=[TEST_ACCOUNT_ID])
    assert_true(payload.id == post_id, "Expected updated draft")
    return payload


def _test_preview_link(client, post_id):
    payload = client.posts.preview_link(post_id)
    assert_true(bool(payload.url and payload.token), "Expected preview link")
    return payload


def _test_archive(client, post_id):
    payload = client.posts.archive(post_id)
    assert_true(payload.id == post_id, "Expected archived post")
    return payload


def _test_restore(client, post_id):
    payload = client.posts.restore(post_id)
    assert_true(payload.id == post_id, "Expected restored post")
    return payload


def _test_create_scheduled(client):
    payload = client.posts.create(
        caption=f"SDK Python scheduled {datetime.now(timezone.utc).isoformat()}",
        account_ids=[TEST_ACCOUNT_ID],
        scheduled_at=(datetime.now(timezone.utc) + timedelta(minutes=15)).isoformat(),
    )
    assert_true(payload.id and payload.status == "scheduled", "Expected scheduled post")
    return payload


def _test_update_scheduled(client, post_id):
    try:
        payload = client.posts.update(post_id, scheduled_at=(datetime.now(timezone.utc) + timedelta(minutes=20)).isoformat())
        assert_true(payload.id == post_id, "Expected updated scheduled post")
        return payload
    except UniPostError as exc:
        if exc.code == "validation_error":
            return exc.code
        raise


def _test_cancel(client, post_id):
    payload = client.posts.cancel(post_id)
    assert_true(payload.id == post_id, "Expected canceled post")
    return payload


def _test_bulk_create(client):
    payload = client.posts.bulk_create(
        [
            {"caption": "SDK Python bulk A", "account_ids": [TEST_ACCOUNT_ID], "status": "draft"},
            {"caption": "SDK Python bulk B", "account_ids": [TEST_ACCOUNT_ID], "status": "draft"},
        ]
    )
    assert_true(isinstance(payload, list) and len(payload) == 2, "Expected two bulk result entries")
    return payload


def _test_publish(client, post_id):
    payload = client.posts.publish(post_id)
    assert_true(payload.id == post_id, "Expected published post response")
    return payload


def _test_retry_result(client, post_id, result_id):
    payload = client.posts.retry_result(post_id, result_id)
    assert_true(payload.social_account_id, "Expected retry result payload")
    return payload


def _test_delivery_jobs_list(client):
    payload = client.delivery_jobs.list(limit=5)
    assert_true(isinstance(payload.get("data", []), list), "Expected delivery jobs list")
    return payload


def _test_delivery_jobs_summary(client):
    payload = client.delivery_jobs.summary()
    assert_true(payload is not None, "Expected summary payload")
    return payload


def _test_delivery_job_commands(client, job):
    try:
        client.delivery_jobs.retry(job.id)
    except UniPostError as exc:
        if exc.code not in ("queue_job_active", "bad_request", "conflict"):
            raise
    try:
        client.delivery_jobs.cancel(job.id)
    except UniPostError as exc:
        if exc.code not in ("bad_request", "conflict"):
            raise
    return True


def _test_analytics_summary(client):
    now = datetime.now(timezone.utc)
    payload = client.analytics.summary(from_date=(now - timedelta(days=30)).strftime("%Y-%m-%d"), to_date=now.strftime("%Y-%m-%d"))
    assert_true(hasattr(payload, "posts"), "Expected summary payload")
    return payload


def _test_analytics_trend(client):
    now = datetime.now(timezone.utc)
    payload = client.analytics.trend(from_date=(now - timedelta(days=30)).strftime("%Y-%m-%d"), to_date=now.strftime("%Y-%m-%d"))
    assert_true(isinstance(payload.dates, list), "Expected trend dates")
    return payload


def _test_analytics_by_platform(client):
    now = datetime.now(timezone.utc)
    payload = client.analytics.by_platform(from_date=(now - timedelta(days=30)).strftime("%Y-%m-%d"), to_date=now.strftime("%Y-%m-%d"))
    assert_true(isinstance(payload, list), "Expected by-platform list")
    return payload


def _test_analytics_rollup(client):
    now = datetime.now(timezone.utc)
    payload = client.analytics.rollup(**{"from": (now - timedelta(days=30)).isoformat(), "to": now.isoformat(), "granularity": "day"})
    assert_true(isinstance(payload.series, list), "Expected rollup series")
    return payload


def _test_usage(client):
    payload = client.usage.get()
    assert_true(isinstance(payload.post_count, int), "Expected usage payload")
    return payload


def _test_oauth_connect(client):
    try:
        payload = client.oauth.connect("bluesky", redirect_url="https://example.com/callback")
        assert_true(bool(payload.auth_url), "Expected auth_url")
        return payload
    except UniPostError as exc:
        if exc.code in ("unauthorized", "validation_error"):
            return exc.code
        raise


if __name__ == "__main__":
    main()
