import assert from "node:assert/strict";
import { existsSync, readFileSync } from "node:fs";
import { join } from "node:path";
import test from "node:test";

const root = new URL("..", import.meta.url).pathname;

function read(relativePath) {
  return readFileSync(join(root, relativePath), "utf8");
}

function sectionBetween(source, start, end) {
  const startIndex = source.indexOf(start);
  assert.notEqual(startIndex, -1, `missing section start: ${start}`);
  const endIndex = source.indexOf(end, startIndex + start.length);
  assert.notEqual(endIndex, -1, `missing section end: ${end}`);
  return source.slice(startIndex, endIndex);
}

test("Admin Email client omits recipient and attempted-date parameters", () => {
  const api = read("src/lib/api.ts");
  const params = sectionBetween(
    api,
    "export interface AdminEmailNotificationListParams",
    "export interface AdminPaidQuotaFollowUpRow",
  );
  const request = sectionBetween(
    api,
    "export async function listAdminEmailNotifications",
    "export async function retryAdminPaidQuotaEmailNotification",
  );

  for (const unwanted of [
    "email?:",
    "start_at?:",
    "end_at?:",
    "AdminEmailNotificationFilterOptions",
  ]) {
    assert.equal(params.includes(unwanted), false, `params still contain ${unwanted}`);
  }
  for (const unwanted of [
    'qs.set("email"',
    'qs.set("start_at"',
    'qs.set("end_at"',
    "listAdminEmailNotificationFilterOptions",
    "/v1/admin/email-notifications/filter-options",
  ]) {
    assert.equal(request.includes(unwanted), false, `request still contains ${unwanted}`);
  }
});

test("Admin Email page removes only recipient and attempted-date controls", () => {
  const page = read("src/app/admin/email/page.tsx");

  for (const unwanted of [
    "listAdminEmailNotificationFilterOptions",
    "buildAttemptedDateRange",
    'aria-label="Filter by recipient email"',
    'aria-label="Attempted from"',
    'aria-label="Attempted through"',
    "loadFilterOptions",
    "filterOptionsError",
    "ae-primary-filters",
    "ae-date-filter",
  ]) {
    assert.equal(page.includes(unwanted), false, `page still contains ${unwanted}`);
  }

  for (const preserved of [
    "listAdminEmailNotifications",
    "retryAdminPaidQuotaEmailNotification",
    'fieldKey="admin.email.search"',
    'aria-label="Filter by email event key"',
    "STATUS_OPTIONS",
    "PROVIDER_OPTIONS",
    "THRESHOLD_OPTIONS",
    "currentPeriod()",
    "const requestGeneration = useRef(0)",
    "generation !== requestGeneration.current",
    "setOffset(0)",
  ]) {
    assert.equal(page.includes(preserved), true, `page lost ${preserved}`);
  }
});

test("Admin Posts client retains its independent absolute time filters", () => {
  const api = read("src/lib/api.ts");
  const listPosts = sectionBetween(
    api,
    "export async function listAdminPosts(",
    "export async function listAdminPostsAggregates(",
  );
  const aggregates = sectionBetween(
    api,
    "export async function listAdminPostsAggregates(",
    "export async function listAdminEmailNotifications(",
  );

  for (const [name, source] of [["list", listPosts], ["aggregates", aggregates]]) {
    assert.match(source, /qs\.set\("start_at", params\.start_at\)/, `${name} lost start_at`);
    assert.match(source, /qs\.set\("end_at", params\.end_at\)/, `${name} lost end_at`);
  }
});

test("recipient/date-only helper, test, and package script are absent", () => {
  assert.equal(existsSync(join(root, "src/app/admin/email/filters.ts")), false);
  assert.equal(existsSync(join(root, "tests/admin-email-filters.test.mjs")), false);
  const packageJson = JSON.parse(read("package.json"));
  assert.equal(Object.hasOwn(packageJson.scripts, "test:admin-email-filters"), false);
});
