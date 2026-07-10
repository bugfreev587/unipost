import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import test from "node:test";

const page = readFileSync(resolve("src/app/admin/posts/page.tsx"), "utf8");
const api = readFileSync(resolve("src/lib/api.ts"), "utf8");

test("admin posts expose Scheduled and Duration in the approved order", () => {
  const created = page.indexOf("<th>Created</th>");
  const scheduled = page.indexOf("<th>Scheduled</th>");
  const duration = page.indexOf("<th>Duration</th>");
  const publish = page.indexOf("<th>Publish Time</th>");

  assert.ok(created > -1, "Created header should be present");
  assert.ok(scheduled > -1, "Scheduled header should be present");
  assert.ok(duration > -1, "Duration header should be present");
  assert.ok(publish > -1, "Publish Time header should be present");
  assert.ok(created < scheduled && scheduled < duration && duration < publish);
});

test("admin posts preserve existing widths while narrowing only Post", () => {
  assert.match(api, /duration_seconds\?: number/);
  assert.match(page, /colSpan=\{11\}/);
  assert.match(page, /overflowX: "auto"/);
  assert.match(page, /minWidth: 1500/);
  assert.match(page, /minWidth: 210, width: 210, maxWidth: 210/);
});

test("Scheduled cells render the timestamp without a redundant status label", () => {
  assert.match(page, /fmtAdminPostTimelineDate\(post\.scheduled_at\)/);
  assert.doesNotMatch(page, /scheduled · \{fmtAdminPostTimelineDate\(post\.scheduled_at\)\}/);
});
