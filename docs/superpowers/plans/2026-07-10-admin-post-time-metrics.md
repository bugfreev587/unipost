# Admin Post Time Metrics Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add scheduled time and all-platform duration to Admin Posts, plus a default-collapsed Time Metrics timeline for every dashboard platform result.

**Architecture:** Compute task duration in the Admin Posts SQL rollup so eligibility and multi-platform completion semantics have one backend authority. Build platform-result timing as pure TypeScript data transformations over the existing post, result, and queue-job payloads, then render it in a small client component. Reuse the worker timing fields already shipped on `origin/dev`; do not add an attempt-history table.

**Tech Stack:** Go, PostgreSQL/pgx, sqlc, Next.js 16, React 19, TypeScript, Node test runner, Playwright.

---

## File Structure

- Modify `api/internal/handler/admin.go`: expose and query `duration_seconds` for each admin post row.
- Modify `api/internal/handler/admin_test.go`: lock the all-results-published duration SQL/API contract.
- Modify `api/internal/db/queries/post_delivery_jobs.sql`: allow a captured completion timestamp to finish a successful job.
- Regenerate `api/internal/db/post_delivery_jobs.sql.go`: keep sqlc output synchronized.
- Modify `api/internal/db/post_delivery_jobs_contract_test.go`: lock explicit successful completion timestamp semantics.
- Modify `api/internal/handler/social_post_queue.go`: reuse one completion instant for synchronous result publication and job completion.
- Modify `api/internal/handler/social_post_queue_test.go`: verify the timestamp is passed through the success helper.
- Modify `dashboard/src/lib/api.ts`: type the new admin duration field.
- Modify `dashboard/src/app/admin/posts/timeline.ts`: format integer admin duration seconds.
- Modify `dashboard/src/app/admin/posts/page.tsx`: add Scheduled and Duration columns and horizontal overflow behavior.
- Modify `dashboard/tests/admin-post-timeline.test.mjs`: test duration formatting.
- Create `dashboard/src/components/posts/list/time-metrics.ts`: pure timing aggregation, retry counting, and formatting helpers.
- Create `dashboard/tests/post-time-metrics.test.mts`: test scheduled/immediate baselines, multi-job aggregation, missing fields, negative gaps, and retry counts.
- Create `dashboard/src/components/posts/list/time-metrics-panel.tsx`: render the collapsed/expanded platform timing panel.
- Modify `dashboard/src/components/posts/list/posts-legacy-list-view.tsx`: load queue data for published tasks, place Time Metrics above Submitted Settings, and add matching styles.
- Create `dashboard/tests/post-time-metrics-source.test.mjs`: lock panel placement, default state, and published queue loading.

### Task 1: Admin API Duration Contract

**Files:**
- Modify: `api/internal/handler/admin.go`
- Modify: `api/internal/handler/admin_test.go`

- [ ] **Step 1: Write the failing Admin Posts contract test**

Append a test that requires the response field and guarded SQL calculation:

```go
func TestAdminPostsSQLIncludesAllPublishedDurationSeconds(t *testing.T) {
	source, err := os.ReadFile("admin.go")
	if err != nil {
		t.Fatalf("read admin.go: %v", err)
	}
	body := string(source)

	for _, want := range []string{
		"DurationSeconds      *int64",
		"`json:\"duration_seconds,omitempty\"`",
		"COUNT(spr.id) > 0",
		"COUNT(*) FILTER (WHERE spr.status = 'published' AND spr.published_at IS NOT NULL) = COUNT(spr.id)",
		"MAX(spr.published_at) >= COALESCE(sp.scheduled_at, sp.created_at)",
		"EXTRACT(EPOCH FROM (MAX(spr.published_at) - COALESCE(sp.scheduled_at, sp.created_at)))",
		"AS duration_seconds",
		"&durationSeconds",
		"item.DurationSeconds = durationSeconds",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("admin post duration contract missing %q", want)
		}
	}
}
```

- [ ] **Step 2: Run the focused test and confirm RED**

Run:

```bash
cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/handler -run TestAdminPostsSQLIncludesAllPublishedDurationSeconds -count=1
```

Expected: FAIL because `duration_seconds` is absent.

- [ ] **Step 3: Implement the guarded SQL field and scan path**

Add `DurationSeconds *int64` to `adminPostRow`. In the `post_rollup` select, add:

```sql
CASE
  WHEN COUNT(spr.id) > 0
   AND COUNT(*) FILTER (
     WHERE spr.status = 'published' AND spr.published_at IS NOT NULL
   ) = COUNT(spr.id)
   AND MAX(spr.published_at) >= COALESCE(sp.scheduled_at, sp.created_at)
  THEN FLOOR(EXTRACT(EPOCH FROM (
    MAX(spr.published_at) - COALESCE(sp.scheduled_at, sp.created_at)
  )))::BIGINT
  ELSE NULL
END AS duration_seconds
```

Select `duration_seconds` from the CTE, scan into `var durationSeconds *int64`, and assign `item.DurationSeconds = durationSeconds`.

- [ ] **Step 4: Run focused and package tests**

Run:

```bash
cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/handler -run 'TestAdminPosts' -count=1
cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/handler -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit Task 1**

```bash
git add api/internal/handler/admin.go api/internal/handler/admin_test.go
git commit -m "feat: expose admin post duration seconds"
```

### Task 2: Admin Posts Scheduled and Duration Columns

**Files:**
- Modify: `dashboard/src/lib/api.ts`
- Modify: `dashboard/src/app/admin/posts/timeline.ts`
- Modify: `dashboard/src/app/admin/posts/page.tsx`
- Modify: `dashboard/tests/admin-post-timeline.test.mjs`
- Create: `dashboard/tests/admin-post-time-metrics-source.test.mjs`

- [ ] **Step 1: Write failing formatting and source tests**

Extend `admin-post-timeline.test.mjs` to load and test `formatAdminDurationSeconds`:

```js
const { getAdminPostPublishTimeline, formatAdminDurationSeconds } = await loadTimelineModule();

test("admin duration renders integer seconds and rejects invalid values", () => {
  assert.equal(formatAdminDurationSeconds(98), "98 s");
  assert.equal(formatAdminDurationSeconds(0), "0 s");
  assert.equal(formatAdminDurationSeconds(undefined), "—");
  assert.equal(formatAdminDurationSeconds(-1), "—");
  assert.equal(formatAdminDurationSeconds(Number.NaN), "—");
});
```

Create `admin-post-time-metrics-source.test.mjs` with assertions for `duration_seconds`, exact header order, `colSpan={11}`, no scheduled sub-label, narrowed Post width, and horizontal overflow:

```js
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
  assert.ok(created < scheduled && scheduled < duration && duration < publish);
  assert.match(api, /duration_seconds\?: number/);
  assert.match(page, /colSpan=\{11\}/);
  assert.match(page, /overflowX: "auto"/);
  assert.match(page, /minWidth: 210/);
  assert.doesNotMatch(page, /scheduled · \{fmtAdminPostTimelineDate\(post\.scheduled_at\)\}/);
});
```

- [ ] **Step 2: Run the dashboard tests and confirm RED**

Run:

```bash
cd dashboard && node --test tests/admin-post-timeline.test.mjs tests/admin-post-time-metrics-source.test.mjs
```

Expected: FAIL because the formatter, field, and columns are absent.

- [ ] **Step 3: Add the API field and duration formatter**

Add `duration_seconds?: number` to `AdminPostRow` and export:

```ts
export function formatAdminDurationSeconds(value?: number | null) {
  if (typeof value !== "number" || !Number.isFinite(value) || value < 0) return "—";
  return `${Math.floor(value)} s`;
}
```

- [ ] **Step 4: Render the two columns without compressing other columns**

Import `formatAdminDurationSeconds`. Change the wrapper/table opening to:

```tsx
<div className="ad-tbl-wrap ad-tbl-static" style={{ overflowX: "auto" }}>
  <table style={{ minWidth: 1500 }}>
```

Insert the headers after Created, update empty-state `colSpan` to 11, narrow only Post with `{ minWidth: 210, width: 210, maxWidth: 210 }`, and add:

```tsx
<td style={{ whiteSpace: "nowrap", minWidth: 132 }}>
  {post.scheduled_at ? fmtAdminPostTimelineDate(post.scheduled_at) : <span style={{ color: "var(--dmuted2)", fontSize: 11 }}>—</span>}
</td>
<td style={{ whiteSpace: "nowrap", minWidth: 88, fontFamily: "var(--font-geist-mono), monospace" }}>
  {formatAdminDurationSeconds(post.duration_seconds)}
</td>
```

- [ ] **Step 5: Run the dashboard tests and build**

Run:

```bash
cd dashboard && node --test tests/admin-post-timeline.test.mjs tests/admin-post-time-metrics-source.test.mjs
cd dashboard && npm run build
```

Expected: PASS.

- [ ] **Step 6: Commit Task 2**

```bash
git add dashboard/src/lib/api.ts dashboard/src/app/admin/posts/timeline.ts dashboard/src/app/admin/posts/page.tsx dashboard/tests/admin-post-timeline.test.mjs dashboard/tests/admin-post-time-metrics-source.test.mjs
git commit -m "feat: show scheduled time and duration in admin posts"
```

### Task 3: Coherent Successful Completion Timestamps

**Files:**
- Modify: `api/internal/db/queries/post_delivery_jobs.sql`
- Modify: `api/internal/db/post_delivery_jobs_contract_test.go`
- Regenerate: `api/internal/db/post_delivery_jobs.sql.go`
- Modify: `api/internal/handler/social_post_queue.go`
- Modify: `api/internal/handler/social_post_queue_test.go`

- [ ] **Step 1: Write failing DB and handler tests**

Require explicit `finished_at` input in the generated query contract:

```go
func TestPostDeliveryJobSuccessUsesCapturedFinishedAt(t *testing.T) {
	source, err := os.ReadFile("post_delivery_jobs.sql.go")
	if err != nil {
		t.Fatalf("read generated post delivery jobs: %v", err)
	}
	body := string(source)
	for _, want := range []string{
		"finished_at = $4",
		"FinishedAt    pgtype.Timestamptz",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("successful completion timestamp contract missing %q", want)
		}
	}
}
```

Add a handler unit test:

```go
func TestMarkDeliveryJobSucceededParamsUsesCapturedFinishedAt(t *testing.T) {
	finishedAt := time.Date(2026, 7, 10, 18, 30, 0, 125000000, time.UTC)
	job := db.PostDeliveryJob{
		ID: "job_1",
		LeaseOwner: pgtype.Text{String: "worker_1", Valid: true},
		LastAttemptAt: pgtype.Timestamptz{Time: finishedAt.Add(-time.Minute), Valid: true},
	}
	params := markDeliveryJobSucceededParams(job, finishedAt)
	if !params.FinishedAt.Valid || !params.FinishedAt.Time.Equal(finishedAt) {
		t.Fatalf("finished_at = %#v, want %s", params.FinishedAt, finishedAt)
	}
}
```

- [ ] **Step 2: Run focused tests and confirm RED**

Run:

```bash
cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/db -run TestPostDeliveryJobSuccessUsesCapturedFinishedAt -count=1
cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/handler -run TestMarkDeliveryJobSucceededParamsUsesCapturedFinishedAt -count=1
```

Expected: FAIL because success still uses database `NOW()` and the helper accepts no timestamp.

- [ ] **Step 3: Accept an explicit completion timestamp and regenerate sqlc**

Change the success query assignment to:

```sql
finished_at = sqlc.arg('finished_at')
```

Run:

```bash
cd api && sqlc generate
```

Expected: `MarkPostDeliveryJobSucceededParams` includes `FinishedAt pgtype.Timestamptz`.

- [ ] **Step 4: Use one captured instant for synchronous publication and job finish**

Change the helper signature to:

```go
func markDeliveryJobSucceededParams(job db.PostDeliveryJob, finishedAt time.Time) db.MarkPostDeliveryJobSucceededParams {
	return db.MarkPostDeliveryJobSucceededParams{
		ID:            job.ID,
		LeaseOwner:    job.LeaseOwner,
		LastAttemptAt: job.LastAttemptAt,
		FinishedAt:    pgtype.Timestamptz{Time: finishedAt, Valid: true},
	}
}
```

After `publishOneContext` returns successfully, capture `completedAt := time.Now()`. Use it for `result.published_at` when status is published and pass it to `markDeliveryJobSucceededParams`. For stale-recovery closure paths, pass `time.Now()`.

- [ ] **Step 5: Run focused and package tests**

Run:

```bash
cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/db ./internal/handler -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit Task 3**

```bash
git add api/internal/db/queries/post_delivery_jobs.sql api/internal/db/post_delivery_jobs.sql.go api/internal/db/post_delivery_jobs_contract_test.go api/internal/handler/social_post_queue.go api/internal/handler/social_post_queue_test.go
git commit -m "fix: align post result and delivery completion times"
```

### Task 4: Pure Platform Time Metrics Model

**Files:**
- Create: `dashboard/src/components/posts/list/time-metrics.ts`
- Create: `dashboard/tests/post-time-metrics.test.mts`

- [ ] **Step 1: Write failing pure model tests**

Create tests covering:

```ts
assert.equal(getPlatformPostTotalDurationMs(
  { created_at: "2026-07-10T10:00:00Z", scheduled_at: "2026-07-10T10:30:00Z" },
  { published_at: "2026-07-10T10:31:38Z" },
), 98_000);

assert.equal(getPlatformPostTotalDurationMs(
  { created_at: "2026-07-10T10:00:00Z" },
  { published_at: "2026-07-10T10:00:12Z" },
), 12_000);

assert.equal(getRetryCount([
  { kind: "dispatch", attempts: 1 },
  { kind: "retry", attempts: 2 },
  { kind: "retry", attempts: 0 },
]), 2);
```

Also assert that phase aggregation uses the earliest job created/claimed/platform-start times, the latest job finish, includes a `Not recorded` phase for missing timestamps, and returns no duration for negative intervals.

- [ ] **Step 2: Run the model test and confirm RED**

Run:

```bash
cd dashboard && node --experimental-strip-types --test tests/post-time-metrics.test.mts
```

Expected: FAIL because the model module does not exist.

- [ ] **Step 3: Implement the pure model**

Export these functions and types:

```ts
export type TimeMetricPhase = {
  key: "created" | "scheduled" | "queued" | "claimed" | "platform_started" | "finished" | "published";
  label: string;
  at: string | null;
  durationFromPreviousMs: number | null;
};

export function getPlatformPostTotalDurationMs(post: TimeMetricPost, result: TimeMetricResult): number | null;
export function getRetryCount(jobs: TimeMetricJob[]): number;
export function buildTimeMetricPhases(post: TimeMetricPost, result: TimeMetricResult, jobs: TimeMetricJob[]): TimeMetricPhase[];
export function formatTimeMetricDuration(ms: number | null): string;
export function formatTimeMetricTimestamp(iso: string | null): string;
```

Use safe parsing helpers, `Math.min` for queued/claimed/platform start, `Math.max` for finished, and only compute non-negative durations.

- [ ] **Step 4: Run the model tests**

Run:

```bash
cd dashboard && node --experimental-strip-types --test tests/post-time-metrics.test.mts
```

Expected: PASS.

- [ ] **Step 5: Commit Task 4**

```bash
git add dashboard/src/components/posts/list/time-metrics.ts dashboard/tests/post-time-metrics.test.mts
git commit -m "feat: model platform post time metrics"
```

### Task 5: Time Metrics Panel and Queue Loading

**Files:**
- Create: `dashboard/src/components/posts/list/time-metrics-panel.tsx`
- Modify: `dashboard/src/components/posts/list/posts-legacy-list-view.tsx`
- Create: `dashboard/tests/post-time-metrics-source.test.mjs`

- [ ] **Step 1: Write the failing integration source test**

Create assertions that require:

```js
assert.match(list, /<TimeMetricsPanel/);
assert.ok(list.indexOf("<TimeMetricsPanel") < list.indexOf("<SubmittedSettingsPanel"));
assert.match(panel, /const \[open, setOpen\] = useState\(false\)/);
assert.match(panel, />Time Metrics</);
assert.match(panel, />Retry count</);
assert.match(list, /const shouldLoadQueue = results\.length > 0/);
```

- [ ] **Step 2: Run source tests and confirm RED**

Run:

```bash
cd dashboard && node --test tests/post-time-metrics-source.test.mjs
```

Expected: FAIL because the component and published-task queue loading are absent.

- [ ] **Step 3: Implement the focused panel**

Create a client component with props:

```ts
type TimeMetricsPanelProps = {
  post: SocialPost;
  result: NonNullable<SocialPost["results"]>[number];
  jobs: PostDeliveryJob[];
  loading: boolean;
  error: string | null;
};
```

Initialize `open` to false. The header renders `Time Metrics` plus formatted total. The expanded body renders Total publishing time, Baseline, Retry count, and each phase with timestamp plus `durationFromPreviousMs`. Loading/error copy stays inline in the panel body.

- [ ] **Step 4: Integrate the panel and approved styles**

Import `TimeMetricsPanel`, set `const shouldLoadQueue = results.length > 0`, and render:

```tsx
<QueueDiagnostics jobs={jobs} loading={jobsLoading} error={jobsError} />
<TimeMetricsPanel post={post} result={result} jobs={jobs} loading={jobsLoading} error={jobsError} />
{result.submitted ? (
  <SubmittedSettingsPanel platform={result.platform || ""} submitted={result.submitted} />
) : null}
```

Add CSS classes that reuse the Submitted Settings border/background and create a compact two-column summary plus vertical phase list. Keep typography within the existing Geist/Geist Mono dashboard system.

- [ ] **Step 5: Run focused tests and dashboard build**

Run:

```bash
cd dashboard && node --experimental-strip-types --test tests/post-time-metrics.test.mts
cd dashboard && node --test tests/admin-post-timeline.test.mjs tests/admin-post-time-metrics-source.test.mjs tests/post-time-metrics-source.test.mjs
cd dashboard && npm run build
```

Expected: PASS.

- [ ] **Step 6: Commit Task 5**

```bash
git add dashboard/src/components/posts/list/time-metrics-panel.tsx dashboard/src/components/posts/list/posts-legacy-list-view.tsx dashboard/tests/post-time-metrics-source.test.mjs
git commit -m "feat: show platform post time metrics"
```

### Task 6: Full Validation and Integration

**Files:**
- Verify all files changed in Tasks 1–5.

- [ ] **Step 1: Run backend CI-equivalent tests**

```bash
cd api && GOCACHE=/tmp/unipost-go-build go test ./...
```

Expected: PASS.

- [ ] **Step 2: Run dashboard unit/source tests and production build**

```bash
cd dashboard && node --experimental-strip-types --test tests/post-time-metrics.test.mts
cd dashboard && node --test tests/admin-post-timeline.test.mjs tests/admin-post-time-metrics-source.test.mjs tests/post-time-metrics-source.test.mjs
cd dashboard && npm run build
```

Expected: PASS.

- [ ] **Step 3: Run dashboard regression tests when browsers are installed**

```bash
cd dashboard && npm run test:regression:dashboard
```

Expected: PASS. If browsers are unavailable, record the exact Playwright installation error before proceeding.

- [ ] **Step 4: Inspect the final diff and worktree**

```bash
git diff --check origin/dev...HEAD
git status --short
git log --oneline origin/dev..HEAD
```

Expected: only focused commits and unrelated pre-existing untracked `.superpowers/`, `artifacts/`, and plan files remain uncommitted.

- [ ] **Step 5: Merge into updated local dev and rerun validation**

```bash
git fetch origin
git switch dev
git pull --ff-only origin dev
git merge --no-ff dev-admin-post-duration-metrics
cd api && GOCACHE=/tmp/unipost-go-build go test ./...
cd ../dashboard && npm run build
```

Expected: merge succeeds and both checks pass.

- [ ] **Step 6: Push dev and monitor all triggered checks/deployments**

```bash
git push origin dev
```

Wait until GitHub Actions, Railway development, and Vercel `unipost-dev` checks finish successfully.

- [ ] **Step 7: Verify the real development environment**

Use `https://dev-app.unipost.dev/admin/posts` to verify Scheduled, Duration, column widths, and horizontal scrolling. Expand a published multi-platform task in the development dashboard and verify Time Metrics is collapsed by default, precedes Submitted Settings, shows the correct longest task/result duration, phase gaps, missing historical values, and retry count.

- [ ] **Step 8: Record completion**

Report the commits, validation commands, deployment results, and real dev acceptance evidence.
