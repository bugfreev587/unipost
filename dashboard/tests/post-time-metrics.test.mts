import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import test from "node:test";
import ts from "typescript";

async function loadTimeMetricsModule() {
  const source = readFileSync(resolve("src/components/posts/list/time-metrics.ts"), "utf8");
  const compiled = ts.transpileModule(source, {
    compilerOptions: {
      module: ts.ModuleKind.ES2022,
      target: ts.ScriptTarget.ES2022,
    },
  });
  const dataUrl = `data:text/javascript;base64,${Buffer.from(compiled.outputText).toString("base64")}`;
  return import(dataUrl);
}

const {
  buildTimeMetricPhases,
  formatTimeMetricDuration,
  formatTimeMetricTimestamp,
  getPlatformPostTotalDurationMs,
  getRetryCount,
} = await loadTimeMetricsModule();

test("scheduled platform post total starts at scheduled_at", () => {
  assert.equal(
    getPlatformPostTotalDurationMs(
      { created_at: "2026-07-10T10:00:00Z", scheduled_at: "2026-07-10T10:30:00Z" },
      { published_at: "2026-07-10T10:31:38Z" },
    ),
    98_000,
  );
});

test("immediate platform post total starts at created_at", () => {
  assert.equal(
    getPlatformPostTotalDurationMs(
      { created_at: "2026-07-10T10:00:00Z" },
      { published_at: "2026-07-10T10:00:12Z" },
    ),
    12_000,
  );
});

test("platform post total rejects missing, invalid, and negative timestamps", () => {
  assert.equal(getPlatformPostTotalDurationMs({ created_at: "invalid" }, { published_at: "2026-07-10T10:00:12Z" }), null);
  assert.equal(getPlatformPostTotalDurationMs({ created_at: "2026-07-10T10:00:00Z" }, {}), null);
  assert.equal(
    getPlatformPostTotalDurationMs(
      { created_at: "2026-07-10T10:00:12Z" },
      { published_at: "2026-07-10T10:00:00Z" },
    ),
    null,
  );
});

test("retry count sums only executed retry attempts", () => {
  assert.equal(
    getRetryCount([
      { kind: "dispatch", attempts: 1 },
      { kind: "retry", attempts: 2 },
      { kind: "retry", attempts: 0 },
      { kind: "retry", attempts: -1 },
    ]),
    2,
  );
});

test("phase timeline aggregates the earliest starts and latest finish across jobs", () => {
  const phases = buildTimeMetricPhases(
    { created_at: "2026-07-10T10:00:00Z", scheduled_at: "2026-07-10T10:30:00Z" },
    { published_at: "2026-07-10T10:31:38Z" },
    [
      {
        kind: "dispatch",
        attempts: 1,
        created_at: "2026-07-10T10:30:01Z",
        first_claimed_at: "2026-07-10T10:30:02Z",
        platform_started_at: "2026-07-10T10:30:05Z",
        finished_at: "2026-07-10T10:30:20Z",
      },
      {
        kind: "retry",
        attempts: 1,
        created_at: "2026-07-10T10:30:30Z",
        first_claimed_at: "2026-07-10T10:30:35Z",
        platform_started_at: "2026-07-10T10:30:40Z",
        finished_at: "2026-07-10T10:31:36Z",
      },
    ],
  );

  assert.deepEqual(
    phases.map(({ key, at, durationFromPreviousMs }) => ({ key, at, durationFromPreviousMs })),
    [
      { key: "created", at: "2026-07-10T10:00:00Z", durationFromPreviousMs: null },
      { key: "scheduled", at: "2026-07-10T10:30:00Z", durationFromPreviousMs: 1_800_000 },
      { key: "queued", at: "2026-07-10T10:30:01Z", durationFromPreviousMs: 1_000 },
      { key: "claimed", at: "2026-07-10T10:30:02Z", durationFromPreviousMs: 1_000 },
      { key: "platform_started", at: "2026-07-10T10:30:05Z", durationFromPreviousMs: 3_000 },
      { key: "finished", at: "2026-07-10T10:31:36Z", durationFromPreviousMs: 91_000 },
      { key: "published", at: "2026-07-10T10:31:38Z", durationFromPreviousMs: 2_000 },
    ],
  );
});

test("missing phases stay visible and negative gaps are never rendered", () => {
  const phases = buildTimeMetricPhases(
    { created_at: "2026-07-10T10:00:00Z", scheduled_at: "2026-07-10T10:30:00Z" },
    { published_at: "2026-07-10T10:31:00Z" },
    [{ kind: "dispatch", attempts: 1, created_at: "2026-07-10T10:29:59Z", finished_at: "2026-07-10T10:30:58Z" }],
  );

  assert.equal(phases.find((phase) => phase.key === "claimed")?.at, null);
  assert.equal(phases.find((phase) => phase.key === "platform_started")?.at, null);
  assert.equal(phases.find((phase) => phase.key === "queued")?.durationFromPreviousMs, null);
  assert.equal(formatTimeMetricTimestamp(null), "Not recorded");
});

test("duration formatter uses seconds, then minutes and seconds", () => {
  assert.equal(formatTimeMetricDuration(null), "—");
  assert.equal(formatTimeMetricDuration(600), "0.6s");
  assert.equal(formatTimeMetricDuration(1_000), "1s");
  assert.equal(formatTimeMetricDuration(61_000), "1m 1s");
  assert.equal(formatTimeMetricDuration(98_000), "1m 38s");
});
