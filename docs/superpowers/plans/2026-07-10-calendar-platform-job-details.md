# Calendar Platform Job Details Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make Calendar and List View render the same per-platform delivery details, retry controls, Queue Diagnostics, Time Metrics, and Submitted Settings, with Queue Diagnostics collapsed by default.

**Architecture:** Extract the complete List View platform-result implementation into a shared client component used by both surfaces. The shared component owns queue loading, per-result job partitioning, retry behavior, panels, and styles; List controls grid layout while Calendar controls stacked layout and parent-data refresh.

**Tech Stack:** Next.js 16 App Router, React 19, TypeScript, Clerk auth, existing UniPost API client, Node test runner, Playwright dashboard regression.

---

## File Structure

- Create `dashboard/src/components/posts/details/post-platform-results-model.ts`: pure queue state and per-result job selection helpers.
- Create `dashboard/src/components/posts/details/post-platform-results.tsx`: shared queue fetch, result cards, retry, diagnostics, submitted settings, and shared styles.
- Move `dashboard/src/components/posts/list/time-metrics.ts` to `dashboard/src/components/posts/details/time-metrics.ts`.
- Move `dashboard/src/components/posts/list/time-metrics-panel.tsx` to `dashboard/src/components/posts/details/time-metrics-panel.tsx`.
- Modify `dashboard/src/components/posts/list/posts-legacy-list-view.tsx`: replace its local results implementation.
- Modify `dashboard/src/components/posts/calendar/posts-calendar-view.tsx`: replace simplified Calendar cards and wire refresh.
- Modify `dashboard/tests/post-time-metrics.test.mts` and `dashboard/tests/post-time-metrics-source.test.mjs`.
- Create `dashboard/tests/post-platform-results-model.test.mts`.
- Create `dashboard/tests/calendar-platform-results-source.test.mjs`.

### Task 1: Model queue-panel states and per-result job ownership

**Files:**
- Create: `dashboard/src/components/posts/details/post-platform-results-model.ts`
- Create: `dashboard/tests/post-platform-results-model.test.mts`

- [ ] **Step 1: Write the failing pure-model tests**

Create tests using the TypeScript data-URL pattern from `post-time-metrics.test.mts`:

```ts
test("partitions jobs without leaking other platform jobs", () => {
  const jobs = [
    { id: "ig-1", social_post_result_id: "ig-result" },
    { id: "tt-1", social_post_result_id: "tt-result" },
  ];
  assert.deepEqual(getJobsForResult(jobs, "ig-result").map((job) => job.id), ["ig-1"]);
  assert.deepEqual(getJobsForResult(jobs, undefined), []);
});

test("describes every diagnostics state", () => {
  assert.deepEqual(getQueueDiagnosticsState([], true, null), {
    kind: "loading",
    label: "Queue diagnostics",
  });
  assert.deepEqual(getQueueDiagnosticsState([], false, "network failed"), {
    kind: "unavailable",
    label: "Queue diagnostics · Unavailable",
  });
  assert.deepEqual(getQueueDiagnosticsState([], false, null), {
    kind: "not_queued",
    label: "Queue diagnostics · Not queued yet",
  });
  assert.deepEqual(getQueueDiagnosticsState([{ id: "job-1" }], false, null), {
    kind: "ready",
    label: "Queue diagnostics (1)",
  });
});
```

- [ ] **Step 2: Run RED**

Run `cd dashboard && node --test tests/post-platform-results-model.test.mts`.

Expected: FAIL because the model file does not exist.

- [ ] **Step 3: Implement the model**

```ts
export type QueueDiagnosticsKind = "loading" | "unavailable" | "not_queued" | "ready";

export function getJobsForResult<T extends { social_post_result_id: string }>(
  jobs: T[],
  resultId?: string,
): T[] {
  if (!resultId) return [];
  return jobs.filter((job) => job.social_post_result_id === resultId);
}

export function getQueueDiagnosticsState(
  jobs: Array<{ id: string }>,
  loading: boolean,
  error: string | null,
): { kind: QueueDiagnosticsKind; label: string } {
  if (loading && jobs.length === 0) return { kind: "loading", label: "Queue diagnostics" };
  if (error && jobs.length === 0) {
    return { kind: "unavailable", label: "Queue diagnostics · Unavailable" };
  }
  if (jobs.length === 0) {
    return { kind: "not_queued", label: "Queue diagnostics · Not queued yet" };
  }
  return { kind: "ready", label: `Queue diagnostics (${jobs.length})` };
}
```

- [ ] **Step 4: Run GREEN**

Run the Step 2 command. Expected: all model tests pass.

- [ ] **Step 5: Commit**

```bash
git add dashboard/src/components/posts/details/post-platform-results-model.ts dashboard/tests/post-platform-results-model.test.mts
git commit -m "test: define shared platform result states"
```

### Task 2: Extract the shared platform-results component

**Files:**
- Create: `dashboard/src/components/posts/details/post-platform-results.tsx`
- Move: `dashboard/src/components/posts/list/time-metrics.ts` → `dashboard/src/components/posts/details/time-metrics.ts`
- Move: `dashboard/src/components/posts/list/time-metrics-panel.tsx` → `dashboard/src/components/posts/details/time-metrics-panel.tsx`
- Modify: `dashboard/tests/post-time-metrics.test.mts`
- Modify: `dashboard/tests/post-time-metrics-source.test.mjs`

- [ ] **Step 1: Write failing shared-component source assertions**

```js
assert.match(shared, /export function PostPlatformResults/);
assert.match(shared, /layout: "grid" \| "stack"/);
assert.match(shared, /getSocialPostQueue/);
assert.match(shared, /retrySocialPostResult/);
assert.match(shared, /getJobsForResult/);
assert.match(shared, /<QueueDiagnostics/);
assert.match(shared, /<TimeMetricsPanel/);
assert.match(shared, /<SubmittedSettingsPanel/);
assert.match(shared, /const \[open, setOpen\] = useState\(false\)/);
```

Update timing test paths to `components/posts/details`.

- [ ] **Step 2: Run RED**

Run:

```bash
cd dashboard
node --test tests/post-time-metrics.test.mts tests/post-time-metrics-source.test.mjs
```

Expected: FAIL because the shared component and moved files do not exist.

- [ ] **Step 3: Move the timing files**

Use `git mv`, keep timing semantics unchanged, update the panel import to `./time-metrics`, and update tests to load the new paths.

- [ ] **Step 4: Implement the shared public component**

```tsx
type PostPlatformResultsProps = {
  post: SocialPost;
  workspaceId: string;
  layout: "grid" | "stack";
  onRetryComplete?: () => void | Promise<void>;
};

export function PostPlatformResults({
  post,
  workspaceId,
  layout,
  onRetryComplete,
}: PostPlatformResultsProps) {
  const { getToken } = useAuth();
  const [jobs, setJobs] = useState<PostDeliveryJob[] | null>(null);
  const [jobsLoading, setJobsLoading] = useState(false);
  const [jobsError, setJobsError] = useState<string | null>(null);
  const results = post.results || [];
  const resultQueueSignature = results
    .map((result) => `${result.id || result.social_account_id}:${result.status}:${result.published_at || ""}`)
    .join("|");

  const loadQueue = useCallback(async (isCancelled: () => boolean = () => false) => {
    if (results.length === 0) {
      if (!isCancelled()) {
        setJobs([]);
        setJobsError(null);
      }
      return;
    }
    if (!isCancelled()) setJobsLoading(true);
    try {
      const token = await getToken();
      if (!token || isCancelled()) return;
      const response = await getSocialPostQueue(token, post.id);
      if (isCancelled()) return;
      setJobs(response.data.jobs || []);
      setJobsError(null);
    } catch (error) {
      if (!isCancelled()) {
        setJobsError(error instanceof Error ? error.message : "Failed to load queue details");
      }
    } finally {
      if (!isCancelled()) setJobsLoading(false);
    }
  }, [getToken, post.id, resultQueueSignature]);

  useEffect(() => {
    let cancelled = false;
    void loadQueue(() => cancelled);
    return () => { cancelled = true; };
  }, [loadQueue]);

  const handleRetryComplete = useCallback(async () => {
    await loadQueue();
    await onRetryComplete?.();
  }, [loadQueue, onRetryComplete]);

  if (results.length === 0) {
    return <div className="posts-result-text">No platform results yet.</div>;
  }

  return (
    <div className={`posts-results-grid${layout === "stack" ? " is-stack" : ""}`}>
      {results.map((result, index) => (
        <PostPlatformResultCard
          key={result.id || result.social_account_id || `${result.platform || "platform"}-${index}`}
          post={post}
          result={result}
          workspaceId={workspaceId}
          jobs={getJobsForResult(jobs || [], result.id)}
          jobsLoading={jobsLoading}
          jobsError={jobsError}
          onRetryComplete={handleRetryComplete}
        />
      ))}
    </div>
  );
}
```

- [ ] **Step 5: Move complete result-card behavior**

Move into the shared file: result card, Facebook phases, debug request panel, Queue Diagnostics, Submitted Settings, submitted-row formatting, long-date/duration/code formatting, inline status, failure guidance, and retry behavior.

Queue Diagnostics must always use one closed toggle:

```tsx
function QueueDiagnostics({ jobs, loading, error }: QueueDiagnosticsProps) {
  const [open, setOpen] = useState(false);
  const state = getQueueDiagnosticsState(jobs, loading, error);

  return (
    <div className="posts-queue-panel">
      <button
        type="button"
        onClick={() => setOpen((value) => !value)}
        className="posts-debug-toggle"
        aria-expanded={open}
      >
        {open ? <ChevronDown size={12} /> : <ChevronRight size={12} />}
        <span>{state.label}</span>
      </button>
      {open ? <QueueDiagnosticsBody state={state.kind} jobs={jobs} error={error} /> : null}
    </div>
  );
}
```

For `not_queued`, render: `This platform does not have a delivery job yet.`

- [ ] **Step 6: Move shared CSS ownership**

Place result card, retry, debug, queue, Time Metrics, Submitted Settings, and Facebook phase CSS in the shared component's global style block. Add:

```css
.posts-results-grid.is-stack {
  grid-template-columns: minmax(0, 1fr);
}
```

Preserve existing tokens and responsive rules.

- [ ] **Step 7: Verify and commit**

Run focused tests and `npm run build`, then:

```bash
git add dashboard/src/components/posts/details dashboard/tests/post-time-metrics.test.mts dashboard/tests/post-time-metrics-source.test.mjs
git commit -m "refactor: share platform result details"
```

### Task 3: Replace the List View local implementation

**Files:**
- Modify: `dashboard/src/components/posts/list/posts-legacy-list-view.tsx`
- Modify: `dashboard/tests/post-time-metrics-source.test.mjs`

- [ ] **Step 1: Write failing List ownership assertions**

```js
assert.match(list, /import \{ PostPlatformResults \} from "@\/components\/posts\/details\/post-platform-results"/);
assert.match(list, /<PostPlatformResults[\s\S]*layout="grid"/);
assert.doesNotMatch(list, /function PostResultsGrid\(/);
assert.doesNotMatch(list, /function QueueDiagnostics\(/);
```

- [ ] **Step 2: Run RED**

Run `cd dashboard && node --test tests/post-time-metrics-source.test.mjs`.

- [ ] **Step 3: Integrate the shared component**

Render:

```tsx
<PostPlatformResults
  post={post}
  workspaceId={workspaceId}
  layout="grid"
  onRetryComplete={loadData}
/>
```

Remove the local results grid, result cards, panels, helper functions, result-only imports, and CSS now owned by the shared component. Keep page-table, dialogs, task metadata, and tooltip code.

- [ ] **Step 4: Run GREEN, build, and commit**

Run the source tests and `npm run build`, then:

```bash
git add dashboard/src/components/posts/list/posts-legacy-list-view.tsx dashboard/tests/post-time-metrics-source.test.mjs
git commit -m "refactor: use shared post results in list view"
```

### Task 4: Replace Calendar cards and wire retry refresh

**Files:**
- Modify: `dashboard/src/components/posts/calendar/posts-calendar-view.tsx`
- Create: `dashboard/tests/calendar-platform-results-source.test.mjs`

- [ ] **Step 1: Write failing Calendar ownership assertions**

```js
assert.match(calendar, /import \{ PostPlatformResults \} from "@\/components\/posts\/details\/post-platform-results"/);
assert.match(calendar, /<PostPlatformResults[\s\S]*layout="stack"/);
assert.match(calendar, /onRetryComplete=\{onRetryComplete\}/);
assert.doesNotMatch(calendar, /function CalendarPostResultCard\(/);
assert.doesNotMatch(calendar, /function buildSubmittedRows\(/);
```

Also assert the parent passes `onRetryComplete={loadData}` into `EventPopover`.

- [ ] **Step 2: Run RED**

Run `cd dashboard && node --test tests/calendar-platform-results-source.test.mjs`.

- [ ] **Step 3: Wire refresh without losing selection**

Add `onRetryComplete: () => void | Promise<void>` to `EventPopover`, pass `loadData` from `PostsCalendarView`, and keep `selectedPostTarget.postId` unchanged so refreshed posts re-derive the open selection.

- [ ] **Step 4: Render the shared stack**

```tsx
<section className="posts-calendar-results">
  <div className="posts-calendar-results-label">Platform results</div>
  <PostPlatformResults
    post={post}
    workspaceId={profileId}
    layout="stack"
    onRetryComplete={onRetryComplete}
  />
</section>
```

Remove Calendar's local result card, submitted formatting, and obsolete result-card CSS. Keep Calendar section spacing and scrollable popover styles.

- [ ] **Step 5: Run GREEN, build, and commit**

Run all focused tests and `npm run build`, then:

```bash
git add dashboard/src/components/posts/calendar/posts-calendar-view.tsx dashboard/tests/calendar-platform-results-source.test.mjs
git commit -m "feat: show platform job details in calendar"
```

### Task 5: Verify and deliver

- [ ] **Step 1: Run full Dashboard regression**

Run `cd dashboard && npm run test:regression:dashboard`.

Expected: all configured tests pass; authenticated smoke may be skipped without credentials.

- [ ] **Step 2: Run final production build**

Run `cd dashboard && npm run build`.

Expected: exit 0 with no new warning beyond the existing Turbopack NFT trace warning.

- [ ] **Step 3: Audit the diff**

```bash
git diff --check origin/dev
git status --short
git diff --stat origin/dev
```

Expected: no whitespace errors and only scoped files.

- [ ] **Step 4: Review behavior**

Confirm no duplicate result-card implementation, no queue request while details are closed, cancelled async updates on popover unmount, stable Calendar selection after retry, closed Queue Diagnostics in every state, strict per-platform job partitioning, and one-column Calendar results.

- [ ] **Step 5: Merge and push dev**

Follow `AGENTS.md`: update local `dev`, merge `dev-calendar-job-details`, rerun Dashboard build/regression, and push local `dev` to `origin/dev`.

- [ ] **Step 6: Monitor and verify development deployment**

Wait for GitHub Actions, Vercel `unipost-dev`, and Railway `dev`. Verify in `https://dev-app.unipost.dev`:

- List Queue Diagnostics starts closed.
- Calendar published multi-platform task shows separate cards.
- Calendar scheduled/no-job state shows `Not queued yet`.
- Calendar Time Metrics exists and starts closed.
- Calendar failed result exposes Retry and inline feedback.
- Jobs, timing, and retry counts stay platform-specific.
