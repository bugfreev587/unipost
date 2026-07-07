import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import test from "node:test";

function source(path) {
  return readFileSync(resolve(path), "utf8");
}

test("admin nav shows Object Storage immediately after Email", () => {
  const adminUI = source("src/app/admin/_components/admin-ui.tsx");
  const emailIndex = adminUI.indexOf('{ label: "Email", href: "/admin/email"');
  const objectStorageIndex = adminUI.indexOf('{ label: "Object Storage", href: "/admin/object-storage"');

  assert.notEqual(emailIndex, -1, "Email nav item should exist");
  assert.notEqual(objectStorageIndex, -1, "Object Storage nav item should exist");
  assert.ok(objectStorageIndex > emailIndex, "Object Storage should appear after Email");
  assert.match(adminUI.slice(emailIndex, objectStorageIndex), /icon: Mail/);
  assert.doesNotMatch(adminUI.slice(emailIndex, objectStorageIndex), /section: "Revenue"/, "Object Storage should stay in Overview before Revenue starts");
  assert.match(adminUI, /export const fmtBytes/);
});

test("admin object storage API client uses the shared admin endpoint", () => {
  const api = source("src/lib/api.ts");

  assert.match(api, /export type AdminObjectStoragePeriod/);
  assert.match(api, /export interface AdminObjectStorageResponse/);
  assert.match(api, /export async function getAdminObjectStorage/);
  assert.match(api, /\/v1\/admin\/object-storage/);
  assert.match(api, /period/);
  assert.match(api, /confirmed_tracked_bytes/);
  assert.match(api, /failed_object_count/);
  assert.match(api, /failed_run_count/);
  assert.match(api, /estimated_next_run_at/);
  assert.match(api, /active_run_started_at/);
  assert.match(api, /stale_running_runs/);
});

test("admin object storage page exposes PRD periods and labels", () => {
  const page = source("src/app/admin/object-storage/page.tsx");

  for (const period of ["yesterday", "last_7_days", "last_month", "this_week", "this_month", "this_year"]) {
    assert.match(page, new RegExp(period), `page should include ${period}`);
  }

  for (const label of [
    "Confirmed tracked size",
    "Estimated next run",
    "Active cleanup run",
    "Deleted in period",
    "Failed object count",
    "Failed run count",
    "Stale running runs",
    "Due cleanup",
    "Referenced",
    "No cleanup runs recorded yet",
  ]) {
    assert.match(page, new RegExp(label), `page should render ${label}`);
  }

  assert.match(page, /getAdminObjectStorage/);
  assert.match(page, /AdminShell title="Object Storage"/);
  assert.match(page, /fmtBytes/);
});
