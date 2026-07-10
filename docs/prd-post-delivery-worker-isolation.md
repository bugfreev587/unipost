# UniPost - Post Delivery Worker Isolation + Fair Dispatch PRD
**Prevent slow publish workloads in one workspace from delaying unrelated users**
Version 1.0 | July 2026

---

## 1. Background

### 1.1 Incident that motivated this PRD

On 2026-07-10, post `87c63980-776b-4b7a-9562-bafec7ae97a4` stayed in `publishing` for an unexpectedly long time.

The post targeted only one Twitter account:

- post ID: `87c63980-776b-4b7a-9562-bafec7ae97a4`
- workspace ID: `ae267ee2-298d-4fa8-b6a0-c386000b17af`
- result ID: `19ba627f-bd78-4335-85ff-c486af5318be`
- delivery job ID: `7cdc5afa-a950-412b-98e6-0317b0ce7e06`

Database evidence showed:

- the parent `social_posts.status` was `publishing`
- the result row was still `pending`
- the delivery job stayed `pending` with `attempts=0` for about 14 minutes
- there were no `post_failures`
- there were no Twitter platform call logs
- there was only `post.publish.queued`

The issue was not a Twitter API delay. The job had not reached the Twitter adapter.

The job became eligible for claim immediately, but it was behind a batch of Instagram and TikTok delivery work owned by the same worker lease owner. When it was eventually claimed, it became `running` before `post.publish.platform_started` was emitted, meaning it was still waiting inside the worker process before actual platform dispatch.

### 1.2 Current codebase reality

The API process starts the publish workers in-process:

- `api/cmd/api/main.go`
  - `NewPostDispatchWorker(...)`
  - `NewPostRetryWorker(...)`

Railway starts only the API binary:

- `api/railway.toml`
  - `startCommand = "./bin/api"`

The delivery worker implementation is in:

- `api/internal/worker/post_delivery.go`

Key current constants and behavior:

- `claimBatchLimit = 20`
- `workspaceConcurrentDispatchCap = 30`
- one dispatch worker loop claims dispatch jobs
- one retry worker loop claims retry jobs
- claimed jobs are immediately marked `running` or `retrying`
- a heartbeat renews leases for every claimed job
- claimed jobs are processed concurrently inside the batch
- the worker still waits for the whole claimed batch to finish before claiming another batch

The claim query adds some protection:

- do not run two active jobs for the same `social_account_id`
- cap active jobs per workspace

But the worker pool itself is global:

- not per user
- not per workspace
- not per platform
- not a dedicated worker fleet

In the incident, active and recently claimed delivery jobs had the same `lease_owner`, showing the actual delivery capacity was a single API process worker owner.

### 1.3 Product problem

UniPost is a multi-tenant publishing API. A slow publish workload in one workspace must not make an unrelated user's one-account Twitter post appear stuck.

Today, one workspace can indirectly degrade another workspace through shared worker execution:

1. A large group of media-heavy Instagram or TikTok jobs is claimed.
2. Those jobs are marked active and kept leased.
3. Some jobs wait inside the process before actual platform dispatch.
4. The dispatch worker does not claim more work until the whole claimed batch finishes.
5. New jobs from unrelated workspaces can wait, even if they are fast and eligible.

The current code already starts a goroutine per claimed job, so the issue is not a purely serial `for` loop. The bottleneck is the batch lifecycle: `runOnce` calls `processClaimedPostDeliveryJobs`, and that function waits for every claimed job in the batch before the dispatch worker can return to the ticker and claim new work. A single long-running or pre-adapter-blocked job can therefore delay the next claim cycle for that worker owner.

This creates poor customer-facing behavior:

- posts remain `publishing` with no visible reason
- results may show `pending` or `running` before real platform execution starts
- support cannot quickly tell queue wait from platform wait
- one customer's workload can increase latency for another customer
- scaling the API service does not clearly scale publish execution as a separate operational concern

---

## 2. Goals

1. Isolate delivery execution so one workspace's slow jobs do not block unrelated workspaces.
2. Make worker capacity independently scalable from HTTP API capacity.
3. Preserve per-account serialization so UniPost does not double-publish to the same connected account.
4. Preserve per-workspace concurrency caps, but make them part of a fair global scheduler.
5. Separate "claimed by worker" from "actually dispatching to platform" in both data and UI.
6. Decouple worker claim cadence from full-batch completion so one slow job cannot stall future claims by that worker owner.
7. Add enough telemetry to measure queue latency, worker wait latency, platform latency, and drain rate.
8. Reduce long `publishing` states caused by worker-side backlog.

---

## 3. Non-goals

- No replacement of `social_posts`, `social_post_results`, or `post_delivery_jobs`.
- No migration to a third-party queue system in v1.
- No exact modeling of every platform-native rate limit.
- No guarantee that slow platform processing becomes fast.
- No customer-facing priority tiers in v1.
- No per-user dedicated worker process in v1.
- No feature flag unless explicitly requested before implementation.

---

## 4. Product Principles

### 4.1 Shared infrastructure, fair scheduling

UniPost should not allocate a dedicated worker process per user or workspace by default. That would waste capacity for idle customers.

Instead, the product should keep a shared worker fleet, but make scheduling fair enough that one busy workspace cannot monopolize the fleet.

### 4.2 Tell the truth about state

`running` should mean a worker is actively executing or about to execute platform work, not merely that a row was selected in a batch.

If a job is reserved but not executing, the system should represent that as a distinct state or telemetry phase.

### 4.3 Preserve platform safety

Fairness must not create duplicate publishes or concurrent posts to the same account. The existing `social_account_id` serialization rule remains required.

### 4.4 Prefer operational clarity over cleverness

The first release should be easy to reason about:

- small, explicit concurrency controls
- clear ownership of worker services
- visible metrics
- deterministic claim behavior

---

## 5. Requirements

### 5.1 Dedicated post delivery worker service

Move post delivery execution out of the API service into a dedicated Railway worker service.

Requirements:

- Add a worker mode or separate command that starts post delivery workers without starting HTTP handlers.
- The API service may continue to run lightweight background workers that are tightly coupled to HTTP behavior, but post delivery dispatch and retry should run in the worker service.
- The worker service must support multiple replicas.
- Each replica must have a distinct `lease_owner`.
- `FOR UPDATE SKIP LOCKED` and lease expiry must remain the correctness mechanism for multi-replica claiming.
- The API service should not also run the same post delivery dispatch workers in production after the worker service is enabled.

Acceptance criteria:

- Production can run at least two post delivery worker replicas.
- Active delivery jobs show multiple `lease_owner` values over time when replicas are enabled.
- Restarting one worker replica does not stop queue drain.
- Scaling API replicas does not unintentionally duplicate post delivery workers.

### 5.2 Fair dispatch across workspaces

The worker claim logic must prevent one workspace from filling an entire global batch when other workspaces have eligible work.

Requirements:

- Keep per-workspace active cap.
- Add a per-claim fairness rule that only limits a busy workspace when other workspaces also have eligible work.
- Default v1 behavior should be simple and plan-agnostic.
- Preserve ordering within each `social_account_id`.
- Preserve `next_run_at` ordering for retries.
- Do not leave worker capacity idle merely because only one workspace currently has eligible work.

Recommended v1:

- Use round-robin workspace ranking inside the claim query:
  - first pass selects the oldest eligible job from each workspace
  - second pass selects the second oldest eligible job from each workspace
  - continue until the global batch is full
- Keep `claimBatchLimit` configurable.
- Add an optional `max_claim_per_workspace_when_contended` soft cap for overload protection, but only apply it when more than one workspace has eligible jobs.
- If only one workspace has eligible work, allow it to fill the remaining batch subject to per-account serialization and per-workspace active cap.

Acceptance criteria:

- If workspace A has 50 eligible Instagram jobs and workspace B has 1 eligible Twitter job, workspace B's job is claimed in the same or next claim cycle, not after workspace A drains.
- If only workspace A has eligible jobs, workspace A can still fill the batch up to configured global and workspace limits.
- No two active jobs for the same `social_account_id` run at once.
- Existing per-workspace active cap still applies.

### 5.3 Separate reserved, dispatching, and processing phases

The current model marks claimed jobs as `running` immediately. This is misleading when a job waits inside the worker before platform dispatch.

Requirements:

- Introduce a distinct pre-dispatch phase in data or telemetry.
- The system must let operators distinguish:
  1. queued in DB
  2. leased/reserved by a worker
  3. waiting inside worker before adapter call
  4. platform adapter started
  5. platform adapter completed or failed
- User-facing APIs should avoid presenting internal worker wait as platform execution.

Implementation options:

Option A - Add job states:

- `pending`
- `claimed`
- `dispatching`
- `retrying`
- terminal states

Option B - Keep states, add timestamps:

- `queued_at`
- `first_claimed_at`
- `last_attempt_at`
- `platform_started_at`
- `finished_at`

Recommended v1:

- Add timestamps first because it is lower risk and easier to backfill.
- Preserve `created_at` as the row creation timestamp.
- Add `first_claimed_at` for the first time any worker claims the job.
- Continue using `last_attempt_at` for the current or latest attempt claim timestamp.
- Use `next_run_at` to identify retry jobs waiting for their scheduled retry window.
- Continue using current state values, but derive display status from timestamps:
  - `pending` dispatch job with no `first_claimed_at`: queued
  - `pending` retry job with `next_run_at` in the future: waiting_retry
  - active with `last_attempt_at` and no `platform_started_at`: reserved
  - active with `platform_started_at`: dispatching

Acceptance criteria:

- For every delivery job, operators can calculate:
  - first queue wait: `first_claimed_at - created_at`
  - current attempt queue wait: `last_attempt_at - attempt_available_at`, where `attempt_available_at` is `created_at` for dispatch jobs and `next_run_at` for scheduled retry attempts
  - worker wait: `platform_started_at - last_attempt_at`
  - platform duration: `finished_at - platform_started_at`
- A post stuck before adapter execution is visibly different from a post stuck in platform execution.

### 5.4 Worker concurrency controls

Worker concurrency should be configurable and platform-aware enough to avoid slow media platforms starving fast text platforms.

Requirements:

- Decouple claiming from full-batch completion. The worker should be able to keep claiming eligible work while execution slots are available instead of blocking future claims behind one slow job in the previous batch.
- Configure global dispatch concurrency per worker replica.
- Configure optional per-platform concurrency caps.
- Keep per-account serialization in the database claim query.
- Make default values conservative.

Recommended v1 defaults:

- per worker replica global dispatch concurrency: 10
- per worker replica Instagram dispatch concurrency: 3
- per worker replica TikTok dispatch concurrency: 3
- per worker replica Twitter dispatch concurrency: 5

These numbers are starting points. The implementation should make them environment-configurable.

Acceptance criteria:

- Slow Instagram jobs cannot consume all local worker execution slots if Twitter jobs are waiting.
- Increasing worker replicas increases aggregate delivery throughput.
- A single worker process can report configured and active concurrency.

### 5.5 Database pool sizing for worker execution

The worker service must not rely on the default `pgxpool` connection count.

Requirements:

- Add explicit database pool configuration for API and worker process modes.
- Worker process pool size must be sized for configured worker concurrency plus heartbeat, logging, and support queries.
- API process pool size must be sized for HTTP traffic and non-delivery background tasks.
- Emit startup logs with pool configuration.

Acceptance criteria:

- Worker startup logs include DB pool max connections.
- Claimed jobs are not blocked from logging `platform_started` because all DB connections are saturated by unrelated API work.
- Pool exhaustion can be diagnosed from metrics or logs.

### 5.6 Queue and worker telemetry

Add operational metrics and logs that make worker capacity visible.

Requirements:

- Record timestamps:
  - `first_claimed_at`
  - `last_attempt_at`
  - `platform_started_at`
  - `finished_at`
- Emit structured logs for:
  - claim batch size
  - active worker slots
  - jobs waiting inside worker
  - per-platform active counts
  - per-workspace active counts
- Expose admin or logs queries for:
  - oldest pending job
  - oldest reserved-but-not-started job
  - p50/p95 queue wait by platform
  - p50/p95 worker wait by platform
  - p50/p95 platform duration by platform

Acceptance criteria:

- Support can answer whether a post is waiting for worker capacity or waiting on the platform.
- A dashboard or admin query can show top workspaces by active/pending delivery jobs.
- The incident class from 2026-07-10 is diagnosable without direct SQL spelunking.

### 5.7 User-facing status derivation

Post and result status should better reflect queue phase.

Requirements:

- Keep parent post terminal statuses unchanged:
  - `published`
  - `partial`
  - `failed`
  - `cancelled`
- Keep parent `publishing` while work is active, but expose richer derived sub-statuses through response fields.
- Result-level derived status should distinguish:
  - `queued`
  - `reserved`
  - `dispatching`
  - `processing`
  - `retrying`
  - `published`
  - `failed`
- Existing clients that only read `status` must not break.

Acceptance criteria:

- A user viewing a post can see that a delivery is queued behind worker capacity rather than stuck on Twitter.
- API docs explain that `publishing` is a parent rollup and that per-result delivery phase carries the detailed state.

---

## 6. Proposed Architecture

### 6.1 Services

Run two production process types:

1. API service
   - HTTP handlers
   - validation
   - enqueue
   - dashboard/API reads
   - lightweight background workers that are safe to keep in-process and do not execute post delivery jobs

2. Post delivery worker service
   - scheduler enqueue execution only if ownership is made explicit and leader-safe
   - dispatch worker
   - retry worker
   - delivery cleanup

The first implementation can use one binary with a mode variable:

- `UNIPOST_PROCESS=api`
- `UNIPOST_PROCESS=post-delivery-worker`

This avoids duplicating build artifacts while allowing separate Railway services and scaling rules.

Worker ownership must be explicit. Do not run scheduler, dispatch, retry, or delivery cleanup in both API and worker process modes unless the worker is proven idempotent and safe under multiple owners. Scheduler ownership is an open design question because running it in both services can duplicate enqueue attempts.

### 6.2 Claim flow

1. Worker wakes on ticker.
2. Worker recovers expired leases.
3. Worker claims eligible jobs with fairness constraints.
4. Worker marks claimed jobs with lease owner, `first_claimed_at` if unset, and `last_attempt_at` for the current attempt.
5. Worker queues claimed jobs into local execution slots.
6. Worker returns to claiming when slots are available instead of waiting for an entire previous batch to finish.
7. Worker writes `platform_started_at` immediately before adapter dispatch.
8. Worker writes terminal job state and refreshes parent post status after completion.

### 6.3 Fairness model

V1 fairness is workspace-round-robin through SQL ranking:

- partition pending jobs by workspace
- order each workspace's eligible jobs by account-safe queue order
- rank by workspace round, then by job age
- use job age as the tie-breaker within each workspace round
- apply global batch limit
- keep same-account exclusion
- keep per-workspace active cap
- fill unused batch capacity when there is no competing workspace

This is not perfect weighted fair queuing, but it fixes the incident class with limited complexity and avoids throttling a busy workspace when it is the only workspace with eligible work.

### 6.4 Status model

Persist minimal timing data:

- `created_at`: row creation timestamp and dispatch queued timestamp
- `next_run_at`: scheduled retry availability timestamp
- `first_claimed_at`: first worker claim timestamp
- `last_attempt_at`: current or most recent attempt claim timestamp
- `platform_started_at`: adapter start timestamp
- `finished_at`: terminal timestamp

Derived display:

| Job kind | DB state | Timestamp condition | Derived phase |
| --- | --- | --- | --- |
| `dispatch` | `pending` | no `first_claimed_at` | `queued` |
| `retry` | `pending` | `next_run_at` is in the future | `waiting_retry` |
| `retry` | `pending` | `next_run_at` is null or due | `queued_retry` |
| any | `running` / `retrying` | no `platform_started_at` | `reserved` |
| `dispatch` | `running` | `platform_started_at` set | `dispatching` |
| `retry` | `retrying` | `platform_started_at` set | `retrying` |
| any | `succeeded` | terminal | `published` or result status |
| any | `failed` / `dead` | terminal | `failed` |

---

## 7. API and Dashboard Changes

### 7.1 API response additions

Add optional fields to per-result or queue job responses:

- `delivery_phase`
- `queued_at`
- `first_claimed_at`
- `last_attempt_at`
- `platform_started_at`
- `finished_at`
- `queue_wait_ms`
- `worker_wait_ms`
- `platform_duration_ms`
- `worker_owner` for admin-only surfaces

Do not remove existing fields.

### 7.2 Dashboard

Posts and queue surfaces should show:

- Queued: waiting for worker
- Reserved: worker claimed it, waiting for execution slot
- Dispatching: platform call started
- Processing: platform accepted but still processing
- Retrying: automatic retry active
- Failed: terminal failure

The wording should avoid exposing internal worker IDs to normal users.

### 7.3 Admin and support surfaces

Admin queue/log views should expose:

- worker owner
- claim timestamp
- platform start timestamp
- age in each phase
- workspace and account identifiers

---

## 8. Rollout Plan

Phase 1 and Phase 2 improve observability and operational isolation. They do not fully fix the incident class by themselves. The user-visible fairness and latency fix depends on Phase 3 and Phase 4: fair workspace claiming plus a worker execution model that does not block future claims behind full-batch completion.

### Phase 1 - Observability and timestamps

- Add `first_claimed_at` to `post_delivery_jobs`.
- Add `platform_started_at` to `post_delivery_jobs`.
- Set `first_claimed_at` only once, and continue setting `last_attempt_at` on every attempt.
- Write timestamp before adapter dispatch.
- Add derived delivery phase in queue/API responses.
- Add query helpers for queue wait and worker wait.
- Add tests for status derivation.

### Phase 2 - Dedicated worker service

- Add process mode support.
- Disable post delivery workers in API process when running production API mode.
- Create Railway post delivery worker service.
- Run one worker replica first.
- Confirm delivery jobs drain with the new service.
- Do not claim this phase fixes cross-workspace head-of-line blocking; it creates the deployment boundary needed for scaling and safer rollout.

### Phase 3 - Fair workspace claim

- Add workspace round-robin claim ordering.
- Add optional `max_claim_per_workspace_when_contended`.
- Update dispatch and retry claim queries.
- Add tests proving an unrelated workspace job is not starved by a large backlog.
- Add tests proving a single busy workspace can still fill available batch capacity when no other workspace has eligible jobs.
- Deploy to development and verify with synthetic multi-workspace queue data.

### Phase 4 - Scalable concurrency

- Decouple claim cadence from full-batch completion.
- Add bounded configurable worker execution slots.
- Add per-platform caps.
- Tune defaults in development and staging.
- Scale worker replicas after telemetry confirms safe behavior.

### Phase 5 - Dashboard polish

- Surface delivery phases in Queue and post detail.
- Add admin/support queue latency summaries.
- Update API docs.

---

## 9. Testing Requirements

### 9.1 Unit tests

- Claim query fairness:
  - workspace A has 50 jobs
  - workspace B has 1 job
  - workspace B job is selected within the claim batch
- Claim query non-contention behavior:
  - workspace A has 50 jobs
  - no other workspace has eligible jobs
  - workspace A can fill the available batch subject to existing caps
- Same-account serialization still holds.
- Per-workspace active cap still holds.
- Delivery phase derivation from state and timestamps.
- Waiting retry delivery phase for `kind='retry'`, `state='pending'`, and future `next_run_at`.
- Parent post remains `publishing` while active jobs exist.

### 9.2 Integration tests

- Two worker instances can claim from the same queue without duplicate jobs.
- Expired leases are recovered.
- A worker crash after claim but before platform start is retried safely.
- A worker crash after platform start preserves existing idempotent publish token behavior.
- Existing pre-publish and result-level duplicate guards continue to prevent duplicate platform calls.

### 9.3 Load tests

Create synthetic queue workloads:

1. One workspace with many slow Instagram jobs.
2. Another workspace with one fast Twitter job.
3. Mixed TikTok and Instagram media jobs.
4. Retry storm from temporary platform failures.

Success criteria:

- unrelated workspace Twitter job starts within a defined SLO
- no duplicate publishes
- queue drain rate increases when worker replicas increase

Recommended initial SLO:

- p95 queue wait for an eligible single-account text post should be under 60 seconds during normal load.

---

## 10. Operational Requirements

### 10.1 Metrics

Track:

- pending jobs by platform
- pending jobs by workspace
- active jobs by platform
- active jobs by workspace
- first queue wait p50/p95
- current attempt queue wait p50/p95
- worker wait p50/p95
- platform duration p50/p95
- delivery throughput per minute
- retry creation rate
- stale lease recovery count
- worker replica count
- DB pool acquire latency

### 10.2 Alerts

Initial alerts:

- oldest pending job older than 5 minutes
- oldest reserved-but-not-started job older than 2 minutes
- no jobs finished in 10 minutes while active jobs exist
- stale lease recovery spike
- DB pool saturation

### 10.3 Runbook

Create a runbook entry:

1. Check oldest pending and active delivery jobs.
2. Group active jobs by `lease_owner`.
3. Compare `created_at`, `first_claimed_at`, `last_attempt_at`, `platform_started_at`, and `finished_at`.
4. Determine whether bottleneck is queue wait, worker wait, platform wait, or DB pool.
5. Scale worker replicas or pause problematic workspace if needed.

---

## 11. Risks and Mitigations

### 11.1 Duplicate publish risk

Changing worker claim and execution phases can reintroduce duplicate publishes if leases are mishandled.

Mitigation:

- preserve idempotent publish tokens
- preserve pre-publish state checks
- preserve the result-level "already published" guard before retry dispatch
- keep same-account serialization
- test crash windows explicitly

### 11.2 Throughput regression

Fairness can reduce peak throughput for a single high-volume workspace.

Mitigation:

- use round-robin fairness that fills idle capacity when no other workspace is waiting
- make any per-workspace claim cap configurable and contention-aware
- monitor drain rate
- allow future plan-aware weighting

### 11.3 Operational complexity

Adding a worker service creates deployment and monitoring overhead.

Mitigation:

- use the same binary with process mode
- keep Railway service config simple
- add startup logs and health endpoints

### 11.4 Platform rate limits

More worker replicas can increase platform pressure.

Mitigation:

- per-platform concurrency caps
- keep per-account serialization
- expand platform-specific rate limiting later if needed

---

## 12. Open Questions

1. Should scheduled enqueue run in the API service, the post delivery worker service, or both with leader-safe claiming?
2. Should fair scheduling be plan-agnostic in v1, or should paid plans get higher workspace claim weights?
3. Should normal users see "Reserved" as a status, or should the UI phrase it as "Waiting for worker"?
4. What is the target p95 queue wait SLO for API customers on each plan?
5. Should support be able to manually pause a noisy workspace's delivery jobs without deleting them?

---

## 13. Acceptance Criteria

This project is complete when:

1. Post delivery workers run in a dedicated scalable service.
2. API service production mode no longer runs duplicate post delivery dispatch workers.
3. Worker claim logic is fair across workspaces.
4. A large slow backlog in one workspace does not prevent another workspace's eligible fast job from starting promptly.
5. Delivery jobs expose enough timestamps to distinguish queue wait, worker wait, and platform wait.
6. Queue and post detail responses include derived delivery phase without breaking existing clients.
7. Tests cover fairness, lease recovery, same-account serialization, and status derivation.
8. Development and staging verification demonstrate the original incident class is fixed.
