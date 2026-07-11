import assert from "node:assert/strict";
import { existsSync, readFileSync } from "node:fs";
import { resolve } from "node:path";
import test from "node:test";

const listPath = resolve("src/components/posts/list/posts-legacy-list-view.tsx");
const panelPath = resolve("src/components/posts/details/time-metrics-panel.tsx");
const sharedPath = resolve("src/components/posts/details/post-platform-results.tsx");
const list = readFileSync(listPath, "utf8");

test("list view renders the shared platform results grid", () => {
  assert.match(list, /import \{ PostPlatformResults \} from "\.\.\/details\/post-platform-results"/);
  assert.match(list, /<PostPlatformResults[\s\S]*?layout="grid"/);
  assert.doesNotMatch(list, /function PostResultsGrid\(/);
  assert.doesNotMatch(list, /function QueueDiagnostics\(/);
});

test("Time Metrics panel is default-collapsed and exposes summary fields", () => {
  assert.ok(existsSync(panelPath), "Time Metrics panel component should exist");
  const panel = readFileSync(panelPath, "utf8");

  assert.match(panel, /const \[open, setOpen\] = useState\(false\)/);
  assert.match(panel, />Time Metrics</);
  assert.match(panel, />Total publishing time</);
  assert.match(panel, />Baseline</);
  assert.match(panel, />Retry count</);
});

test("shared platform results own diagnostics, metrics, settings, and retry", () => {
  assert.ok(existsSync(sharedPath), "shared platform results component should exist");
  const shared = readFileSync(sharedPath, "utf8");

  assert.match(shared, /export function PostPlatformResults/);
  assert.match(shared, /layout: "grid" \| "stack"/);
  assert.match(shared, /getSocialPostQueue/);
  assert.match(shared, /retrySocialPostResult/);
  assert.match(shared, /getJobsForResult/);
  assert.match(shared, /<QueueDiagnostics/);
  assert.match(shared, /<TimeMetricsPanel[\s\S]*?<SubmittedSettingsPanel/);
});

test("expanded published tasks load queue timing data", () => {
  assert.ok(existsSync(sharedPath), "shared platform results component should exist");
  const shared = readFileSync(sharedPath, "utf8");

  assert.match(shared, /const shouldLoadQueue = results\.length > 0/);
  assert.match(shared, /const resultQueueSignature = results/);
  assert.match(shared, /resultQueueSignature,/);
  assert.doesNotMatch(shared, /\n\s+results,\n\s+\]\);/);
});

test("queue requests ignore stale responses after a post change or unmount", () => {
  const shared = readFileSync(sharedPath, "utf8");

  assert.match(shared, /const queueRequestRef = useRef\(0\)/);
  assert.match(shared, /if \(requestId !== queueRequestRef\.current\) return/);
  assert.match(shared, /return \(\) => \{\s*queueRequestRef\.current \+= 1;\s*\}/);
});

test("queue failures do not report retry or job timing as recorded zeroes", () => {
  assert.ok(existsSync(panelPath), "Time Metrics panel component should exist");
  const panel = readFileSync(panelPath, "utf8");

  assert.match(panel, /error \? "Unavailable" : retryCount/);
  assert.match(panel, /jobTimingUnavailable/);
});

test("Time Metrics dots align with phase titles and connect dot-to-dot", () => {
  const shared = readFileSync(sharedPath, "utf8");

  assert.match(shared, /\.posts-time-metrics-dot\{[^}]*align-self:start[^}]*margin-top:3px/);
  assert.match(shared, /\.posts-time-metrics-event:not\(:last-child\)::before\{[^}]*top:7px[^}]*bottom:-7px/);
  assert.doesNotMatch(shared, /\.posts-time-metrics-timeline::before/);
});
