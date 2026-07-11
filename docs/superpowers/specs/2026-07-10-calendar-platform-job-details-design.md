# Calendar Platform Job Details Design

**Date:** 2026-07-10
**Branch:** `dev-calendar-job-details`
**Status:** Approved for implementation

## Goal

Make Calendar post details and List View task details use the same platform-result behavior. A Calendar popover must show every platform result with the same queue diagnostics, time metrics, submitted settings, failure guidance, and retry controls available in List View.

The change also makes Queue Diagnostics collapsed by default everywhere.

## User Outcomes

1. Opening a Calendar post shows an independent result card for every target result, such as Instagram, TikTok, or YouTube.
2. Each result card shows only the delivery jobs associated with that result.
3. Calendar and List View both show, in this order:
   - platform result summary;
   - success or failure details;
   - retry controls when the failed result is retryable;
   - Queue Diagnostics;
   - Time Metrics;
   - Submitted Settings.
4. Queue Diagnostics, Time Metrics, and Submitted Settings are collapsed by default.
5. A scheduled result that does not yet have a delivery job is represented as `Not queued yet`, not as an error.
6. Calendar post content retains its existing editability rules. The Calendar popover may retry a failed platform result even when the post-content action says `View only`.

## Current State

List View owns the complete platform-result implementation inside `posts-legacy-list-view.tsx`. When a task is expanded, it fetches `GET /v1/social-posts/{postId}/queue`, partitions the returned jobs by `social_post_result_id`, and renders Queue Diagnostics, Time Metrics, Submitted Settings, failure guidance, and retry behavior.

Calendar maintains a second simplified result-card implementation inside `posts-calendar-view.tsx`. It renders result summaries and submitted settings but does not fetch queue jobs, does not show Time Metrics or Queue Diagnostics, and does not expose result retry.

This duplicated ownership caused the two views to drift.

## Chosen Architecture

### Shared platform-results component

Extract a shared `PostPlatformResults` component from the List View implementation. Both List View and Calendar use this component as the single owner of platform result behavior.

The component accepts:

- the `SocialPost`;
- the workspace/project ID;
- a layout mode (`grid` for List View, `stack` for Calendar);
- an optional refresh callback invoked after a successful retry.

The component owns:

- queue loading and request lifecycle;
- stable request dependencies based on the post ID and result signature;
- per-result job partitioning;
- platform result cards;
- retry state and errors;
- Queue Diagnostics;
- Time Metrics;
- Submitted Settings;
- loading, empty, unavailable, and not-yet-queued states.

The two surfaces differ only in the outer layout. List View retains its responsive grid. Calendar uses a single-column stack appropriate for the popover width.

### Shared result card

Each `PostPlatformResultCard` receives one result and the subset of jobs whose `social_post_result_id` matches that result ID. This prevents timing, retry count, or queue details from leaking between platforms in a multi-platform task.

The card preserves the current List View behavior for:

- original-post links;
- successful, processing, partial, and failed states;
- normalized failure guidance;
- retry eligibility and inline feedback;
- debug request information when present;
- Facebook processing phases;
- submitted platform settings.

Calendar no longer maintains a separate simplified result-card implementation.

## Data Flow

1. List View mounts `PostPlatformResults` only for an expanded task.
2. Calendar mounts `PostPlatformResults` only while the selected post popover is open.
3. The component fetches `GET /v1/social-posts/{postId}/queue` once for the mounted post state.
4. The response supplies the latest post plus all delivery jobs.
5. Jobs are grouped by `social_post_result_id` and passed to their matching result cards.
6. A result retry calls the existing retry endpoint.
7. On retry success, the shared component refreshes its queue data and invokes the surface refresh callback.
8. List View refreshes the posts list. Calendar reloads Calendar data while preserving selection by post ID, so the open popover stays attached to the same task.

No backend, database, or API contract change is required.

## Interaction Design

### Platform result order

Each result card presents:

1. platform icon, account, status, and original-post link;
2. platform name and publish time;
3. success message or failure explanation;
4. retry action and inline retry error when applicable;
5. Queue Diagnostics;
6. Time Metrics;
7. Submitted Settings.

### Queue Diagnostics

Queue Diagnostics uses a closed initial state on both surfaces.

- With jobs: the label is `Queue Diagnostics (N)`.
- While loading: the closed label remains visible; expanding shows the loading message.
- With no jobs for a scheduled or pending result: the label includes `Not queued yet`; expanding explains that no delivery job has been created for the platform.
- When the queue request fails: the label includes `Unavailable`; expanding shows the request error.

An empty job list is a valid state and must not be reported as a retry count of zero when the queue request itself failed.

### Time Metrics

Time Metrics remains closed initially.

- Published results show the baseline-to-published total.
- Unpublished results show `—` for the total.
- Created and Scheduled phases remain available even before a job exists.
- Missing job phases remain visible as not recorded.
- When queue data is unavailable, job-derived phases and retry count display `Unavailable`.

### Submitted Settings

Submitted Settings remains closed initially and continues to render the submitted per-platform values.

### Retry in Calendar

Calendar uses the same retry eligibility, request, disabled state, and inline error presentation as List View. Retrying a delivery result is independent of editing the post content. The existing Calendar `Edit` or `View only` action remains unchanged.

## Layout and Styling

The shared result component owns the CSS needed by its cards and collapsible panels so it renders correctly without depending on the List page being mounted.

- List View uses the current two-column desktop grid and one-column responsive fallback.
- Calendar uses a one-column result stack within the existing scrollable popover.
- Existing typography, border, status, and spacing tokens are preserved.
- No new visual language, animation system, or dependency is introduced.
- The popover remains usable at narrow widths and does not compress two platform cards side by side.

## Error and Edge States

- No results: show the existing no-platform-results empty state.
- Results but no queue jobs: keep result cards visible and show `Not queued yet` Queue Diagnostics.
- Queue request failure: keep result summaries, submitted settings, and post/result timing visible; mark job-derived data unavailable.
- Retry request failure: show the error inside the affected result card and keep the Calendar popover open.
- Retry success: refresh both queue and parent post data without changing the selected Calendar post.
- Result identity without an ID: render the card and submitted settings, but do not offer retry and do not associate unrelated jobs.

## Testing Strategy

### Component and source-contract tests

- Both Calendar and List View render the shared platform-results component.
- Calendar no longer contains a second platform result-card implementation.
- List and Calendar pass the correct grid/stack layout mode.
- Queue Diagnostics initializes with `aria-expanded=false` and no open body.
- Time Metrics and Submitted Settings remain collapsed initially.
- Calendar supplies a retry refresh callback that preserves the selected post ID.

### Behavior tests

- Queue jobs are partitioned by `social_post_result_id`.
- Scheduled result with no jobs renders `Not queued yet`.
- Queue failure renders `Unavailable` without a misleading zero retry count.
- Failed retryable result exposes Retry.
- Retry success refreshes data; retry failure remains inline.
- Existing Time Metrics duration, phase, and retry-count tests remain green.

### Validation

- Dashboard production build.
- Dashboard regression suite.
- Real development-environment verification for:
  - List View Queue Diagnostics default closed;
  - Calendar published multi-platform details;
  - Calendar scheduled result before queue creation;
  - Calendar failed result retry behavior;
  - Calendar Time Metrics and per-platform job separation.

## Non-Goals

- Changing queue scheduling or worker behavior.
- Adding per-attempt history beyond the existing Queue Diagnostics timeline.
- Changing Calendar post-content editing rules.
- Adding new backend endpoints or feature flags.
- Changing the task-level Admin Posts Duration definition.
