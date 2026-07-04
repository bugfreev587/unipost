import assert from "node:assert/strict";
import { existsSync, readFileSync } from "node:fs";
import { resolve } from "node:path";
import test from "node:test";

const toolsPage = readFileSync(resolve("src/app/tools/page.tsx"), "utf8");
const analyticsTool = readFileSync(resolve("src/app/tools/_components/public-analytics-tool.tsx"), "utf8");
const routePath = resolve("src/app/tools/youtube-analytics/page.tsx");

test("YouTube Analytics is listed as a public tools entry", () => {
  assert.match(toolsPage, /name:\s*"YouTube Analytics"/);
  assert.match(toolsPage, /href:\s*"\/tools\/youtube-analytics"/);
});

test("YouTube Analytics has a public tool route", () => {
  assert.equal(existsSync(routePath), true);
});

test("YouTube Analytics tool documents V1 and V2 analytics scopes", () => {
  assert.match(analyticsTool, /youtube:\s*{/);
  assert.match(analyticsTool, /youtube\.readonly/);
  assert.match(analyticsTool, /yt-analytics\.readonly/);
  assert.match(analyticsTool, /\/docs\/api\/analytics\/youtube/);
});
