import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const source = (path) => readFile(new URL(`../${path}`, import.meta.url), "utf8");

test("the repository observability gate always runs this contract test", async () => {
  const packageJSON = JSON.parse(await source("package.json"));
  const plan = await readFile(
    new URL("../../docs/superpowers/plans/2026-07-29-observability-reads-v2.md", import.meta.url),
    "utf8",
  );

  assert.match(packageJSON.scripts["test:docs-ai"], /log-observability-v2-source\.test\.mjs/);
  assert.equal(
    packageJSON.scripts["test:regression:dashboard:local"],
    "DASHBOARD_WEB_SERVER=1 NEXT_PUBLIC_APP_HOST=dev-app.localhost:3000 DASHBOARD_BASE_URL=http://dev-app.localhost:3000 npm run test:regression:dashboard",
  );
  assert.doesNotMatch(plan, /Run `npm test`/);
  assert.doesNotMatch(plan, /Run from `dashboard\/`: `npm test`/);
  assert.match(plan, /`npm run test:docs-ai`/);
  assert.match(plan, /`npm run test:hosted-connect`/);
  assert.match(plan, /`npm run test:regression:dashboard:local`/);
});

test("local regression uses two RFC localhost hosts without skipping landing coverage", async () => {
  const mobileRegression = await source("tests/regression/mobile-layout.spec.ts");
  assert.match(mobileRegression, /replace\(":\/\/dev-app\.", ":\/\/dev\."\)/);
  assert.match(mobileRegression, /landingBaseURL !== appBaseURL/);
  assert.doesNotMatch(mobileRegression, /!\/localhost\|127\\\.0\\\.0\\\.1\/\.test/);
});

test("public Logs API docs remain on the shipped numeric SDK contract while v2 is locked", async () => {
  const [overview, list, get, stream] = await Promise.all([
    source("src/app/docs/api/logs/page.tsx"),
    source("src/app/docs/api/logs/list/page.tsx"),
    source("src/app/docs/api/logs/get/page.tsx"),
    source("src/app/docs/api/logs/stream/page.tsx"),
  ]);
  const corpus = [overview, list, get].join("\n");

  assert.doesNotMatch(corpus, /opaque source-qualified/i);
  assert.doesNotMatch(corpus, /request:&lt;id&gt;|integration:&lt;id&gt;|integration:110966|integration:101/i);
  assert.doesNotMatch(corpus, /physical (?:table|partition|storage) key/i);
  assert.match(list, /data\[\]\.id[^\n]*type: "integer"/);
  assert.match(get, /name: "id", type: "integer"/);
  assert.match(list, /"id": 101/);
  assert.match(get, /logs\.get\(110966\)|Logs\.Get\(context\.Background\(\), 110966\)|logs\(\)\.get\(110966\)/);
  assert.match(get, /"id": 110966/);
  assert.match(stream, /always numeric integration-log ids/i);
});

test("Logs docs promise metadata-only lists and bounded failure-only detail lifecycle", async () => {
  const [overview, list, get] = await Promise.all([
    source("src/app/docs/api/logs/page.tsx"),
    source("src/app/docs/api/logs/list/page.tsx"),
    source("src/app/docs/api/logs/get/page.tsx"),
  ]);
  const corpus = [overview, list, get].join("\n");

  assert.match(list, /metadata-only/i);
  assert.match(list, /never includes request_payload or response_payload/i);
  assert.match(get, /failed request/i);
  assert.match(get, /bounded/i);
  assert.match(get, /detail_status/);
  assert.match(get, /expired/);
  assert.match(corpus, /global order/i);
  assert.match(corpus, /opaque cursor/i);
});

test("Workspace and Admin Logs show expired canonical request detail without assuming payloads", async () => {
  const [api, logSearch, workspace, admin] = await Promise.all([
    source("src/lib/api.ts"),
    source("src/lib/log-search.ts"),
    source("src/app/(dashboard)/projects/[id]/logs/page.tsx"),
    source("src/app/admin/logs/page.tsx"),
  ]);

  assert.match(api, /detail_status\?: "available" \| "expired"/);
  assert.match(api, /has_error_detail\?: boolean/);
  assert.match(api, /request_payload\?: unknown/);
  assert.match(api, /response_payload\?: unknown/);
  for (const page of [workspace, admin]) {
    assert.match(page, /logDetailUnavailableMessage\(selectedLog\)/);
  }
  assert.match(logSearch, /Detail payload expired/);
  assert.doesNotMatch(logSearch, /startsWith\(["']request:/);
});
