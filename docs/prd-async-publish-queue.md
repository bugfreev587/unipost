# UniPost — Async Publish + Queue Architecture PRD
**Move immediate publishing off the request path and introduce a real Queue page**
Version 1.1 | April 2026

---

## 1. Background

### 1.1 Current problem

Today, UniPost has two different publishing architectures:

- **Scheduled publish** is already asynchronous:
  - a `social_post` is stored as `scheduled`
  - a worker later claims and executes it
- **Immediate publish** is still synchronous:
  - the API request creates the post
  - the handler immediately loops through platform adapters
  - the caller waits until every platform attempt finishes

This is manageable for fast text posts, but becomes painful for:

- Facebook video uploads
- platforms that require `uploading -> processing -> publish`
- multi-platform fan-out where one platform is much slower than the others
- temporary provider/platform outages

The result is poor UX:

- users click `Publish` and must wait
- they do not know whether the post is still progressing or stuck
- transient failures are shown as final failures too early
- the existing `Posts > Queue` page is still a placeholder

### 1.2 Current codebase reality

Immediate publish currently blocks in:

- [social_posts.go](/Users/xiaoboyu/unipost/api/internal/handler/social_posts.go)

Specifically:

- `Create()` routes immediate posts to `createImmediatePost()`
- `createImmediatePost()` calls `executeImmediatePost()`
- `executeImmediatePost()` creates the parent `social_post`, then immediately runs `executePublishLoop()`

UniPost already has important foundations we should reuse:

- scheduled worker:
  - [scheduler.go](/Users/xiaoboyu/unipost/api/internal/worker/scheduler.go)
- per-platform result model:
  - [social_post_results.sql](/Users/xiaoboyu/unipost/api/internal/db/queries/social_post_results.sql)
- per-result manual retry:
  - [social_post_retry.go](/Users/xiaoboyu/unipost/api/internal/handler/social_post_retry.go)
- structured failure classification:
  - [taxonomy.go](/Users/xiaoboyu/unipost/api/internal/postfailures/taxonomy.go)
- structured failure persistence:
  - [044_post_failures.sql](/Users/xiaoboyu/unipost/api/internal/db/migrations/044_post_failures.sql)

### 1.3 Goal

Make immediate publishing asynchronous without rewriting UniPost's parent/child post model.

When a user clicks `Publish`:

1. UniPost should persist the post quickly
2. queue one delivery unit per target platform/account
3. return immediately
4. let workers execute those delivery units in the background
5. show live execution state in `Posts > Queue`
6. automatically retry only transient failures

---

## 2. Core product decision

### 2.1 Keep the existing post model

We should **not** replace `social_post` with a completely new "post unit" model.

The current hierarchy is already correct:

- `social_post`
  - the user-facing post object
- `social_post_result`
  - one platform/account delivery outcome

In practice, a `social_post_result` is already the "post unit" the user is asking for:

- one post
- one platform
- one target account

### 2.2 What needs to change

The missing piece is not the result model. The missing piece is the **execution model**.

Conceptually we need two queue layers:

1. **Dispatch queue**
   - handles the first execution attempt for immediate posts
   - replaces synchronous `executePublishLoop()` on the request path

2. **Retry queue**
   - handles limited automatic retries for transient failures
   - only for failures classified as retriable

In v1, both concepts should be implemented in one table:

- `post_delivery_jobs`

with:

- `kind='dispatch'`
- `kind='retry'`

This is the most pragmatic migration path off synchronous immediate publish.

---

## 3. User experience goals

### 3.1 Publish flow

When a user clicks `Publish`:

- the request should return quickly
- the UI should show `Queued`
- the user should not need to sit and wait for slow platforms

### 3.2 Queue visibility

Users should be able to open `Posts > Queue` and immediately answer:

- how many deliveries are pending?
- which platforms are slow?
- which posts are partially complete?
- what is the current state of each platform delivery?
- will UniPost retry automatically?
- can I manually trigger retry now?

### 3.3 Slow platform states

For platforms like Facebook video, Queue should show state transitions such as:

- `pending`
- `running`
- `processing`
- `retrying`
- `published`
- `dead`

The exact display may combine job state and adapter-reported progress text, but the user must never be left staring at a stuck button.

---

## 4. Success criteria

This project is successful if:

1. Immediate publish no longer blocks the user on slow platform execution
2. `Posts > Queue` shows real queued and in-flight platform deliveries
3. A `partial` post can visibly contain multiple per-platform queue items
4. Retriable failures are re-attempted automatically
5. Non-retriable failures are surfaced clearly and do not churn in queue
6. Users can manually trigger retry from the queue UI

---

## 5. Scope

### 5.1 In scope for v1

- async dispatch for immediate publish
- per-result execution units
- dispatch queue
- retry queue
- queue summary + queue list UI
- limited automatic retries based on failure type
- manual `Retry now` from Queue
- queue state surfaced in `All Posts`

### 5.2 Out of scope for v1

- inbox message retry queue
- media upload queue redesign
- global platform outage dashboard
- priority scheduling / SLA routing
- infinite retries
- full distributed queue service replacement
- `unknown_outcome` as a first-class new state

---

## 6. API contract

### 6.1 Immediate publish response

Immediate publish should become async, but v1 should avoid a hard breaking contract if possible.

Recommended response behavior:

- keep returning the parent post object
- keep compatibility fields such as `platform_results`
- allow those result fields to be empty or placeholder values on initial response
- return queue-aware metadata such as:
  - `status: "queued"`
  - `execution_mode: "async"`
  - `queued_results_count`
  - `queue_url` or equivalent

The caller should no longer expect final per-platform publish outcomes in the initial immediate-publish response.

### 6.2 Scheduled publish result creation timing

Scheduled posts should **not** create `social_post_results` at schedule-creation time.

Instead:

1. the parent post remains `scheduled`
2. when it becomes due, the scheduler claims it
3. only then are `social_post_results` created
4. only then are `post_delivery_jobs(kind='dispatch')` created

This avoids stale result rows and keeps scheduled execution aligned with the real account/platform state at run time.

---

## 7. Data model

### 7.1 Keep existing tables

Keep:

- `social_posts`
- `social_post_results`
- `post_failures`

### 7.2 New table: `post_delivery_jobs`

Add:

`post_delivery_jobs`

Suggested fields:

- `id`
- `post_id`
- `social_post_result_id`
- `workspace_id`
- `social_account_id`
- `platform`
- `kind`
- `state`
- `attempts`
- `max_attempts`
- `failure_stage`
- `error_code`
- `platform_error_code`
- `last_error`
- `next_run_at`
- `last_attempt_at`
- `created_at`
- `updated_at`

`kind` values:

- `dispatch`
- `retry`

`state` values:

- `pending`
- `running`
- `retrying`
- `succeeded`
- `failed`
- `dead`
- `cancelled`

Notes:

- `failed` means this job execution failed, but the delivery is not yet in a final `dead` or `cancelled` outcome.
- `retrying` should only be used for `kind='retry'`.

### 7.3 Uniqueness rule

For any given `social_post_result_id`, there may be at most one active job at a time.

Active includes:

- `pending`
- `running`
- `retrying`

This prevents:

- duplicate enqueue
- duplicate manual retry requests
- worker/user collisions

Recommended implementation:

- partial unique index on `social_post_result_id` where state is one of the active states

### 7.4 Retention

Suggested retention:

- `succeeded` jobs: keep for `14 days`, then clean up
- `dead` jobs: keep longer, for example `90 days`
- `cancelled` jobs: keep longer, for example `90 days`

Cleanup should be handled by a periodic worker/cron.

---

## 8. State model

### 8.1 Parent `social_post`

Persisted states:

- `draft`
- `scheduled`
- `published`
- `partial`
- `failed`

Derived UI-only states:

- `queued`
- `dispatching`
- `retrying`

For v1, only the persisted terminal/core states should be stored on the parent row. Intermediate display states should be derived from active delivery jobs.

### 8.2 Child `social_post_result`

Persisted states:

- `pending`
- `processing`
- `published`
- `failed`

V1 rule:

- keep `social_post_results.status` intentionally narrow
- do not add `retrying`, `dead`, or `queued_for_retry` to result status
- queue/execution progress should come from `post_delivery_jobs`

### 8.3 Job states

Delivery job states:

- `pending`
- `running`
- `retrying`
- `succeeded`
- `failed`
- `dead`
- `cancelled`

### 8.4 Parent status recomputation

The parent `social_post.status` should be recomputed with these rules:

- all results `published` -> parent `published`
- all terminal and all failed -> parent `failed`
- some published and some terminal failed -> parent `partial`
- scheduled parent remains `scheduled`
- draft parent remains `draft`

Other intermediate display states such as `queued`, `dispatching`, and `retrying` should be derived from active delivery jobs and should not require new persisted parent states in v1.

---

## 9. Retry eligibility and policy

### 9.1 Retry eligibility

Use the existing structured taxonomy in:

- [taxonomy.go](/Users/xiaoboyu/unipost/api/internal/postfailures/taxonomy.go)

Automatically retry only if `is_retriable = true`.

Auto-retry examples:

- `rate_limit`
- `temporary_platform_error`
- `timeout`
- transient upstream 5xx

Do not auto-retry:

- `validation_error`
- `missing_permission`
- `account_reconnect_required`
- `auth_token_invalid`
- `target_not_found`
- permanent content/media errors

### 9.2 Retry policy

Default max attempts:

- `5`

Default backoff:

- attempt 1: 2 min
- attempt 2: 10 min
- attempt 3: 30 min
- attempt 4: 2 hr
- attempt 5: 6 hr

After that:

- mark the retry job `dead`
- leave result as final failure

---

## 10. Worker model

### 10.1 Dispatch worker

Add:

- `PostDispatchWorker`

Responsibilities:

1. poll `post_delivery_jobs` where `kind='dispatch'`
2. claim `pending` jobs
3. execute one platform/account delivery
4. update `social_post_result`
5. mark dispatch job success/failure
6. if failed and retriable, enqueue `kind='retry'`
7. recompute parent post status

### 10.2 Retry worker

Add:

- `PostRetryWorker`

Responsibilities:

1. poll `post_delivery_jobs` where `kind='retry'`
2. claim jobs whose `next_run_at <= now`
3. execute retry for one result
4. update result and retry job
5. recompute parent post status

### 10.3 Claim mechanism

V1 should use database-native claim semantics:

- `FOR UPDATE SKIP LOCKED`

This is the simplest and safest implementation for the current codebase.

### 10.4 Default cadence

Dispatch worker default:

- poll every `2s`
- batch size `20`

Retry worker default:

- poll every `5s`
- batch size `20`

### 10.5 Same-account concurrency

V1 should treat one `social_account_id` as a serialized execution lane.

Meaning:

- do not run multiple delivery jobs concurrently for the same `social_account_id`

This reduces:

- ordering issues
- platform rate-limit pressure
- duplicate processing risks on slow media platforms

### 10.6 Shared retry executor

Refactor the existing logic in:

- [social_post_retry.go](/Users/xiaoboyu/unipost/api/internal/handler/social_post_retry.go)

into a shared executor used by:

- manual retry endpoint
- retry worker

The user-facing retry path should enqueue work, not directly execute adapters.

---

## 11. Failure windows and recovery limits

### 11.1 Success on platform, crash before local persistence

There is an unavoidable risk window:

- remote platform may already have accepted/published the content
- local worker may crash before persisting success

V1 should not introduce a first-class `unknown_outcome` state yet.

Instead, v1 should:

- record detailed attempt/debug information
- keep platform response context where possible
- minimize duplicate sends through idempotency where supported
- accept this as a known v1 operational risk

Future versions may add:

- reconcile logic
- `unknown_outcome`
- platform-specific recovery tooling

---

## 12. API changes

### 12.1 Queue endpoints

Queue endpoints:

- `GET /v1/workspaces/{workspaceID}/post-delivery-jobs`
- `GET /v1/workspaces/{workspaceID}/post-delivery-jobs/summary`
- `POST /v1/workspaces/{workspaceID}/post-delivery-jobs/{jobID}/retry-now`
- `POST /v1/workspaces/{workspaceID}/post-delivery-jobs/{jobID}/cancel`

Optional:

- `GET /v1/workspaces/{workspaceID}/social-posts/{id}/queue`

### 12.2 Post detail response

`GET /post` detail should include queue summary fields, at minimum:

- `active_job_count`
- `retrying_count`
- `dead_count`

This keeps UI composition simpler.

### 12.3 Retry now semantics

`Retry now` from UI should:

- not call adapters directly
- not bypass worker semantics
- simply move the job into immediately eligible state

The actual retry is still executed by the worker.

If the target result already has an active job:

- return `409`
- do not enqueue a duplicate job

### 12.4 Cancel semantics

`Cancel` means:

- stop future automatic processing for this delivery
- keep the delivery failed/dead from a user outcome perspective
- do not delete history

It does **not** mean:

- delete the failure
- revert the post
- pretend the delivery never existed

### 12.5 Manual retry endpoint

The user-facing manual retry path should be unified with queue semantics:

- retry should enqueue work
- retry should not directly execute adapters

V1 should avoid keeping a long-lived direct-execution bypass for normal product paths.

---

## 13. Dashboard UI

### 13.1 Navigation

Keep:

- `Posts`
  - `All Posts`
  - `Queue`

`Queue` becomes a real operational page.

### 13.2 Queue page

Page title:

- `Queue`

Subtitle:

- `Platform deliveries currently pending, running, processing, or retrying.`

### 13.3 Summary cards

Top summary bar:

- `Pending`
- `Running`
- `Retrying`
- `Dead`
- `Recovered today`

### 13.4 Queue list model

The queue list should be grouped by parent post, but each actionable row should be a **per-platform delivery unit** backed by one `post_delivery_job`.

Outer group shows:

- post image
- caption
- parent post status
- queued delivery count

Child rows show:

- platform icon
- account name
- queue status
- attempts
- next retry
- current execution stage
- latest error
- action buttons

This directly supports the user requirement:

- a partial failed post may contain multiple post-per-platform units
- users can manually retry a single unit

### 13.5 All Posts integration

`All Posts` should also expose queue awareness:

- `queued`
- `dispatching`
- `retrying`
- `2 deliveries queued`

These should be rendered as badges/secondary indicators in v1.

V1 should not add new top-level tabs such as `Queued` or `Processing`.

### 13.6 Queue refresh model

V1 should use polling, not websocket/SSE.

Suggested behavior:

- initial load
- poll every 5-10 seconds while page is visible
- pause/reduce frequency when tab is backgrounded

---

## 14. Notifications

For retriable failures:

- do not immediately treat them as final failed posts
- send failure notifications only when the retry path is exhausted (`dead`)

V1 rule:

- notify once on first transition into `dead`
- do not send a separate "recovered" notification when a later retry succeeds

Otherwise users will get noisy, misleading alerts during provider incidents.

---

## 15. Rollout plan

### Phase 1 — Async immediate publish foundation

1. add `post_delivery_jobs`
2. stop executing immediate publish loop on request path
3. create `kind='dispatch'` jobs instead
4. add dispatch worker

### Phase 2 — Retry recovery

1. wire structured retry eligibility
2. create `kind='retry'` jobs on retriable failure
3. add retry worker
4. share retry executor with manual retry endpoint

### Phase 3 — Queue UI

1. replace placeholder `Posts > Queue`
2. add summary cards
3. add grouped per-platform rows
4. add `Retry now` and `Cancel`
5. surface queue states in `All Posts`

### Phase 4 — Operational polish

1. queue-aware notifications
2. admin queue analytics
3. outage banners / provider degradation messaging
4. cleanup worker for historical jobs

---

## 16. Risks

1. If immediate publish and scheduled publish keep separate execution models, complexity will continue to grow
2. If queue actions bypass workers, state consistency will drift
3. If parent post state tries to represent too many child execution nuances directly, it will become confusing
4. If retries are not limited by structured failure types, the system may cause retry storms
5. If same-account jobs run concurrently, some platforms may see ordering and rate-limit issues

---

## 17. Recommendation

This architecture change should move forward.

It is not just a UX improvement. It is a structural improvement that better matches UniPost's actual domain:

- one user-visible post
- many platform-specific deliveries
- some platforms slow
- some failures transient
- some failures recoverable

The best v1 path is:

1. keep `social_post` as the parent object
2. treat `social_post_result` as the true per-platform execution unit
3. move immediate publish onto an async dispatch queue
4. use one `post_delivery_jobs` table with `kind=dispatch|retry`
5. add limited retry behavior for transient failures
6. turn `Posts > Queue` into a real operational dashboard for delivery state

This gives UniPost a publishing architecture that is much closer to reliable social infrastructure rather than a synchronous request-time fan-out service.

---

*UniPost Async Publish + Queue Architecture PRD v1.1*
