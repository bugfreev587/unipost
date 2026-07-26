import assert from "node:assert/strict";
import { existsSync, readFileSync } from "node:fs";
import test from "node:test";

const read = (path) => readFileSync(new URL(`../${path}`, import.meta.url), "utf8");
const pagePath = new URL("../src/app/admin/publishing-restrictions/page.tsx", import.meta.url);
const page = existsSync(pagePath) ? readFileSync(pagePath, "utf8") : "";

function functionSource(source, name) {
  const start = source.indexOf(`function ${name}`);
  if (start < 0) return "";
  const bodyStart = source.indexOf("{", start);
  let depth = 0;
  for (let index = bodyStart; index < source.length; index += 1) {
    if (source[index] === "{") depth += 1;
    if (source[index] === "}") depth -= 1;
    if (depth === 0) return source.slice(start, index + 1);
  }
  return source.slice(start);
}

test("Admin navigation and typed publishing restriction APIs exist", () => {
  const nav = read("src/app/admin/_components/admin-ui.tsx");
  const api = read("src/lib/api.ts");

  assert.match(nav, /Publishing Restrictions/);
  assert.match(nav, /\/admin\/publishing-restrictions/);
  assert.match(api, /interface\s+AdminPublishingRestriction/);
  assert.match(api, /listAdminPublishingRestrictions/);
  assert.match(api, /updateAdminPublishingRestriction/);
  assert.match(api, /expected_version:\s*number/);
  assert.match(api, /\/v1\/admin\/publishing-restrictions/);
});

test("generic Super Admin center renders operational state and all accessible states", () => {
  assert.ok(page, "admin publishing restrictions route should exist");
  assert.match(page, /requireSuperAdmin/);
  assert.match(page, /restriction\.platform/);
  assert.match(page, /restricted_plan_ids/);
  assert.match(page, /reason_code/);
  assert.match(page, /affected_workspace_count/);
  assert.match(page, /affected_account_count/);
  assert.match(page, /restriction\.affected_workspace_count\s*\?\?\s*0/);
  assert.match(page, /restriction\.affected_account_count\s*\?\?\s*0/);
  assert.match(page, /updated_by_user_id/);
  assert.match(page, /restriction\.version/);
  assert.match(page, /recent_events/);
  assert.match(page, /aria-busy=\{loading\}/);
  assert.match(page, /role="alert"/);
  assert.match(page, /No publishing restrictions configured/);
});

test("enable and disable confirmations state side effects and use optimistic versioning", () => {
  assert.match(page, /already in-flight calls are not canceled/i);
  assert.match(page, /No customer email is sent/);
  assert.match(page, /No post is automatically retried/);
  assert.match(page, /No recovery email is sent unless separately previewed and confirmed/);
  assert.match(page, /expected_version:\s*pendingChange\.restriction\.version/);
  assert.match(page, /status\s*===\s*409/);
  assert.match(page, /changed in another Admin session/i);
  assert.match(page, /await\s+load\(\)/);
  assert.doesNotMatch(page, /window\.confirm/);
});

test("campaign clients cover preview, irreversible send, progress, and failed-recipient retry", () => {
  const api = read("src/lib/api.ts");

  assert.match(api, /previewAdminPublishingRestrictionCampaign/);
  assert.match(api, /createAdminPublishingRestrictionCampaign/);
  assert.match(api, /listAdminPublishingRestrictionCampaigns/);
  assert.match(api, /retryFailedAdminPublishingRestrictionCampaign/);
  assert.match(api, /email-campaigns\/preview/);
  assert.match(api, /email-campaigns\/\$\{encodeURIComponent\(campaignId\)\}\/retry-failed/);
});

test("campaign UI is fixed to restriction and recovery notices with two-step confirmation", () => {
  assert.match(page, /restriction_notice/);
  assert.match(page, /recovery_notice/);
  assert.match(page, /preview\.recipient_count/);
  assert.match(page, /preview\.subject/);
  assert.match(page, /preview\.body/);
  assert.match(page, /irreversible/i);
  assert.match(page, /confirmation:\s*"SEND"/);
  assert.match(page, /previewed_count/);
  assert.match(page, /snapshotted_count/);
  assert.match(page, /pending_count/);
  assert.match(page, /sent_count/);
  assert.match(page, /failed_count/);
  assert.match(page, /skipped_count/);
  assert.match(page, /Retry failed recipients/);
  assert.match(page, /item\.cycle_id\s*===\s*restriction\.cycle_id/);
});

test("campaign polling never couples sends or retries to restriction toggles", () => {
  const toggleMutation = functionSource(page, "confirmPendingChange");
  assert.match(page, /queued/);
  assert.match(page, /running/);
  assert.match(page, /setTimeout/);
  assert.match(page, /Email campaigns are separate manual actions/);
  assert.match(toggleMutation, /updateAdminPublishingRestriction/);
  assert.doesNotMatch(toggleMutation, /createAdminPublishingRestrictionCampaign/);
  assert.doesNotMatch(toggleMutation, /retryFailedAdminPublishingRestrictionCampaign/);
});
