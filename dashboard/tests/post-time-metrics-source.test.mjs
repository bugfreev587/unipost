import assert from "node:assert/strict";
import { existsSync, readFileSync } from "node:fs";
import { resolve } from "node:path";
import test from "node:test";

const listPath = resolve("src/components/posts/list/posts-legacy-list-view.tsx");
const panelPath = resolve("src/components/posts/list/time-metrics-panel.tsx");
const list = readFileSync(listPath, "utf8");

test("Time Metrics panel is default-collapsed and exposes summary fields", () => {
  assert.ok(existsSync(panelPath), "Time Metrics panel component should exist");
  const panel = readFileSync(panelPath, "utf8");

  assert.match(panel, /const \[open, setOpen\] = useState\(false\)/);
  assert.match(panel, />Time Metrics</);
  assert.match(panel, />Total publishing time</);
  assert.match(panel, />Baseline</);
  assert.match(panel, />Retry count</);
});

test("every platform result renders Time Metrics immediately above Submitted Settings", () => {
  const renderedPanels = list.match(/<TimeMetricsPanel/g) || [];
  assert.equal(renderedPanels.length, 2);
  assert.match(list, /<TimeMetricsPanel[\s\S]*?<SubmittedSettingsPanel/);
});

test("expanded published tasks load queue timing data", () => {
  assert.match(list, /const shouldLoadQueue = results\.length > 0/);
  assert.match(list, /const resultQueueSignature = results/);
  assert.match(list, /resultQueueSignature,/);
  assert.doesNotMatch(list, /\n\s+results,\n\s+\]\);/);
});

test("queue failures do not report retry or job timing as recorded zeroes", () => {
  assert.ok(existsSync(panelPath), "Time Metrics panel component should exist");
  const panel = readFileSync(panelPath, "utf8");

  assert.match(panel, /error \? "Unavailable" : retryCount/);
  assert.match(panel, /jobTimingUnavailable/);
});
