import assert from "node:assert/strict";
import test from "node:test";

import {
  PLAN_PLATFORM_PUBLISHING_RESTRICTED_MESSAGE,
  describePublishingRestrictionRetry,
  filterAllowedAccountIds,
  isPlanPlatformPublishingRestrictedError,
  isPlatformRestricted,
  type PublishingRestrictionProjection,
} from "./publishing-restrictions.ts";

const restriction: PublishingRestrictionProjection = {
  restricted: true,
  platform: "tiktok",
  plan_id: "free",
  reason_code: "platform_capacity_limit",
  code: "plan_platform_publishing_restricted",
  message: PLAN_PLATFORM_PUBLISHING_RESTRICTED_MESSAGE,
  next_action: "upgrade_or_wait_then_retry",
};

test("uses the approved customer message exactly", () => {
  assert.equal(
    PLAN_PLATFORM_PUBLISHING_RESTRICTED_MESSAGE,
    "TikTok publishing is temporarily unavailable on the Free plan due to platform capacity limits. We’re working with TikTok to increase capacity. Upgrade your plan or try again after the restriction is lifted.",
  );
});

test("restricts Free TikTok while allowing Paid and other platforms", () => {
  assert.equal(isPlatformRestricted([restriction], "tiktok", "free"), true);
  assert.equal(isPlatformRestricted([restriction], "tiktok", "basic"), false);
  assert.equal(isPlatformRestricted([restriction], "instagram", "free"), false);
});

test("excludes restricted TikTok accounts from preselection and Toggle All", () => {
  const accounts = [
    { id: "tt-1", platform: "tiktok" },
    { id: "ig-1", platform: "instagram" },
  ];

  assert.deepEqual(
    filterAllowedAccountIds(accounts, ["tt-1", "ig-1"], [restriction]),
    ["ig-1"],
  );
});

test("recognizes both API and normalized publishing restriction codes", () => {
  assert.equal(isPlanPlatformPublishingRestrictedError({ code: "PLAN_PLATFORM_PUBLISHING_RESTRICTED" }), true);
  assert.equal(isPlanPlatformPublishingRestrictedError({ code: "plan_platform_publishing_restricted" }), true);
  assert.equal(isPlanPlatformPublishingRestrictedError({ rawCode: "PLAN_PLATFORM_PUBLISHING_RESTRICTED" }), true);
  assert.equal(isPlanPlatformPublishingRestrictedError({ code: "quota_exceeded" }), false);
});

test("describes blocked, ready, and expired manual Retry states", () => {
  assert.deepEqual(
    describePublishingRestrictionRetry({
      retry_policy: {
        manual_retry_allowed: false,
        reason: "publishing_restriction_active",
      },
      media_retained_until: "2026-09-24T12:00:00Z",
    }, new Date("2026-07-26T12:00:00Z")),
    { state: "blocked", label: "Publishing restriction active" },
  );

  assert.deepEqual(
    describePublishingRestrictionRetry({
      retry_policy: { manual_retry_allowed: true },
      media_retained_until: "2026-09-24T12:00:00Z",
    }, new Date("2026-07-26T12:00:00Z")),
    { state: "ready", label: "Retry" },
  );

  assert.deepEqual(
    describePublishingRestrictionRetry({
      retry_policy: {
        manual_retry_allowed: false,
        reason: "media_reupload_required",
      },
      media_retained_until: "2026-07-25T12:00:00Z",
    }, new Date("2026-07-26T12:00:00Z")),
    { state: "expired", label: "Upload media again to retry" },
  );
});
