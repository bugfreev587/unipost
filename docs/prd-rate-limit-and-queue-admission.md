# UniPost — Rate Limit + Queue Admission Control PRD
**Protect publish APIs and background queue from bursty customer traffic**
Version 1.0 | April 2026

---

## 1. Background

### 1.1 The product risk

UniPost now supports:

- immediate async publish
- scheduled publish
- retries
- draft publish
- per-result queue jobs

That is the correct architecture for a social publishing API, but it also creates a new operational risk:

- one customer can enqueue a large burst of writes in a few seconds
- a buggy client can loop retries or cancels aggressively
- a campaign import can create a large number of queued deliveries at once
- the API can accept work much faster than the workers can drain it

This is exactly the class of issue customers describe as:

> "I schedule 30 at once, delete 30 at once, sometimes retry twice because not all calls go through."

If UniPost does not add explicit admission control, the likely failure mode is not a clean `429`. It is:

- DB write amplification
- growing `social_posts` / `social_post_results` / `post_delivery_jobs` backlog
- slow queue drain
- noisy retries
- platform-facing rate-limit pressure
- degraded experience for unrelated workspaces

### 1.2 Current codebase reality

UniPost already has several pieces we should preserve:

- async immediate publish and scheduled publish enqueue into:
  - [social_post_queue.go](/Users/xiaoboyu/unipost/api/internal/handler/social_post_queue.go)
- background workers claim queue jobs from:
  - [post_delivery.go](/Users/xiaoboyu/unipost/api/internal/worker/post_delivery.go)
  - [scheduler.go](/Users/xiaoboyu/unipost/api/internal/worker/scheduler.go)
- queue jobs live in:
  - [048_post_delivery_jobs.sql](/Users/xiaoboyu/unipost/api/internal/db/migrations/048_post_delivery_jobs.sql)
  - [post_delivery_jobs.sql](/Users/xiaoboyu/unipost/api/internal/db/queries/post_delivery_jobs.sql)
- monthly billing quota already exists:
  - [checker.go](/Users/xiaoboyu/unipost/api/internal/quota/checker.go)
- optional per-account monthly quota already exists:
  - [per_account.go](/Users/xiaoboyu/unipost/api/internal/quota/per_account.go)
- Redis was provisioned on Railway on 2026-04-28 and is reachable from the API service via the internal URL (`REDIS_URL`). The API service does **not** yet import a Redis client — wiring that in is part of this PRD.

What is missing is **runtime protection against burst traffic**.

### 1.3 What we do NOT have today

UniPost does **not** currently have:

- a general per-workspace API request limiter
- a per-end-user limiter
- enqueue throughput limiting
- queue depth caps
- rate-limit headers for normal publish APIs
- a Redis-backed shared limiter across replicas

There are only a few narrow public-surface IP limiters today:

- Bluesky connect form:
  - [connect_bluesky.go](/Users/xiaoboyu/unipost/api/internal/handler/connect_bluesky.go)
- connect callback brute-force protection:
  - [connect_callback.go](/Users/xiaoboyu/unipost/api/internal/handler/connect_callback.go)

These are not enough for publish-path protection.

---

## 2. Goal

Add a production-grade admission-control layer so UniPost can safely absorb burst traffic without letting one workspace degrade the entire service.

The system should:

1. reject abusive or accidental bursts early with clean API errors
2. protect queue depth from unbounded growth
3. differentiate between workspace-wide abuse and one runaway managed user
4. scale across multiple API replicas
5. fit the current async publish architecture without rewriting it

---

## 3. Non-goals

- replacing the existing monthly billing quota model
- replacing `post_delivery_jobs` with Redis queues
- building a full distributed job system
- exact platform-native rate limit modeling
- per-platform adaptive throttling in v1
- customer-configurable rate limits in the first release
- batch-aware admission semantics for `POST /v1/posts/bulk` — bulk is currently undocumented (PostForMe parity gap, hidden from docs and MCP in commit `7810ee6`) and gets request-limiter-only coverage; per-batch reserve-or-reject is deferred until bulk becomes a publicly supported surface
- scheduler claim-time admission and per-workspace worker concurrency caps — these protect the worker domain from internal retry storms and are out of scope for v1 (see §13 Phase 3)

---

## 4. Core decision

UniPost should add **three separate controls**, not one:

1. **API request rate limiter**
2. **enqueue throughput limiter**
3. **queue depth limiter**

These solve different problems:

- request rate limiter protects HTTP + DB from sudden bursts
- enqueue limiter protects against "few requests, huge amount of work"
- queue depth limiter protects the system when workers are already backed up

### 4.1 Why one limiter is not enough

A plain request-per-minute limiter is insufficient because:

- one `POST /v1/posts` may enqueue multiple delivery jobs
- one `POST /v1/posts/bulk` may enqueue up to 50 posts
- one customer can stay under request limits but still flood the queue

Similarly, queue depth alone is insufficient because:

- the queue can still be slammed repeatedly before the depth check reacts
- retries/cancels/reschedules can create burst pressure without growing the queue immediately

### 4.2 Why Redis

This feature should use Redis, not only Postgres, because:

- rate limiting needs fast shared counters across API replicas
- sliding windows and token buckets are awkward and expensive in Postgres
- Redis was added to the Railway project on 2026-04-28 and is reachable from the API service via internal networking. The Go client and connection lifecycle do not exist yet — adding `github.com/redis/go-redis/v9` and a thin `internal/redis` package is part of this PRD's Phase 1.

UniPost reads the connection string from `REDIS_URL`. Do **not** hardcode Railway credentials in code, PRDs, or commits — rotate the password immediately if it has been pasted into a chat, screenshot, or ticket.

---

## 5. Limiting dimensions

### 5.1 Primary dimension: workspace

The primary protection boundary should be:

- `workspace_id`

Reason:

- API keys are workspace-scoped
- billing quota is workspace-scoped
- queue jobs are workspace-scoped
- system impact is naturally attributable to the workspace

### 5.2 Secondary dimension: managed end user

When the target accounts in the request all resolve to the same managed end user, UniPost should also apply a second limiter on:

- `workspace_id + external_user_id`

Reason:

- one end user inside a customer's app can otherwise consume a disproportionate amount of queue capacity
- this is especially useful for white-label / Connect-heavy customers

If the request spans multiple `external_user_id` values, the user cannot be determined cheaply, or `external_user_id` is NULL (BYO / non-Connect accounts), v1 should fall back to workspace-only protection. **Do not bucket NULL `external_user_id` requests into a shared `:none` key** — that would punish all BYO traffic in one workspace into a single tiny bucket.

### 5.3 Not a primary dimension in v1

Do not build first-release admission control primarily around:

- IP address
- Clerk user ID
- social account ID
- platform

These may be useful later for diagnostics or platform-aware throttling, but they are not the best first control boundary for UniPost's product model.

---

## 6. Control types

## 6.1 API request rate limiter

### Purpose

Protects:

- API server CPU
- request decoding / validation
- DB writes
- accidental retry storms

### Algorithm

Use a **token bucket** in Redis.

Why:

- allows short bursts
- simpler operationally than exact sliding logs
- good fit for "normal spiky SaaS traffic"

### Scope

Apply to write-heavy endpoints only.

Initial scope:

- `POST /v1/posts`
- `POST /v1/posts/{id}/publish`
- `POST /v1/posts/{id}/cancel`
- `POST /v1/posts/{id}/results/{resultID}/retry`
- `PATCH /v1/posts/{id}` (draft edit)
- draft publish / reschedule routes

`POST /v1/posts/bulk` gets request-limiter-only coverage as cheap insurance, but no batch-aware enqueue/depth admission in v1 — bulk is undocumented and will be revisited when it becomes a publicly supported surface.

Read endpoints should remain much looser in v1.

### Error behavior

On limit breach:

- return `429 Too Many Requests`
- include `Retry-After` (seconds until the bucket has at least one token; computed inside the Lua script and returned alongside the deny decision — see §9.4)
- include normalized error code:
  - `rate_limited`

Recommended response shape:

```json
{
  "error": {
    "code": "RATE_LIMITED",
    "normalized_code": "rate_limited",
    "message": "Too many requests for this workspace. Please retry shortly."
  },
  "request_id": "req_123"
}
```

---

## 6.2 Enqueue throughput limiter

### Purpose

Protects against large bursts of accepted work even when request count looks small.

Examples:

- 30 simultaneous create-post calls
- multiple bulk batches in a few seconds
- one campaign import generating many queued deliveries

### Algorithm

Use a **sliding window** in Redis.

Why sliding window here:

- enqueue pressure is about recent accepted work volume
- the queue evolves over time
- this is easier to reason about than a request count bucket

### What to count

The counted unit should be:

- **post count** in v1

Not just request count.

Rules:

- single create post = `1`
- bulk create = `len(posts)`
- draft publish = `1`
- retry-now = `1`

Future refinement:

- count delivery jobs instead of posts when we want better accuracy for multi-account fan-out

### Dimensions

Apply to:

- workspace
- managed end user when resolvable

### Error behavior

On breach:

- `429 Too Many Requests`
- normalized code:
  - `enqueue_rate_limited`

Message example:

```json
{
  "error": {
    "code": "RATE_LIMITED",
    "normalized_code": "enqueue_rate_limited",
    "message": "This workspace is creating posts too quickly. Please slow down and retry."
  }
}
```

---

## 6.3 Queue depth limiter

### Purpose

Protects against unbounded backlog when workers cannot drain fast enough.

This is the system-level safety belt.

### Definition

Depth should count active jobs in:

- `pending`
- `running`
- `retrying`

using `post_delivery_jobs`.

That aligns directly with the existing state machine in:

- [post_delivery_jobs.sql](/Users/xiaoboyu/unipost/api/internal/db/queries/post_delivery_jobs.sql)

### Algorithm

For v1:

- use Postgres as the source of truth
- query active counts before enqueue
- wrap the count + insert pair in a workspace-scoped advisory lock (`pg_try_advisory_xact_lock(hash('admit:'+workspace_id))`) so two replicas cannot both observe `199/200` and each insert past the cap. Cross-workspace traffic is unaffected.
- ship a partial index on the active states alongside the v1 migration, since the existing `(workspace_id, created_at DESC)` index does not satisfy `WHERE state IN ('pending','running','retrying')` cheaply:

  ```sql
  CREATE INDEX post_delivery_jobs_workspace_active_idx
    ON post_delivery_jobs(workspace_id)
    WHERE state IN ('pending', 'running', 'retrying');
  ```

This is acceptable because:

- the count query is simpler than exact request-window accounting
- correctness matters more than micro-latency for queue depth

For v2:

- optionally mirror active depth in Redis for cheaper hot-path checks
- keep periodic reconciliation against Postgres

### Dimensions

Apply to:

- workspace active queue depth
- managed end user active queue depth, when resolvable

### Error behavior

On breach:

- `429 Too Many Requests`
- normalized code:
  - `queue_depth_exceeded`
  - `user_queue_depth_exceeded`

Message example:

```json
{
  "error": {
    "code": "RATE_LIMITED",
    "normalized_code": "queue_depth_exceeded",
    "message": "This workspace already has too many queued deliveries. Wait for the queue to drain before creating more posts."
  }
}
```

---

## 7. Per-plan defaults

These limits are **runtime safety limits**, not billing quota.

They should scale by paid tier, but remain conservative enough to protect the system.

## 7.1 Suggested v1 defaults

### Free

- write requests: `30/min`, burst `10`
- enqueue posts: `50/min`, `100/5min`
- active queue depth per workspace: `200`
- active queue depth per managed user: `25`

### p10

- write requests: `60/min`, burst `20`
- enqueue posts: `200/min`, `500/5min`
- active queue depth per workspace: `1000`
- active queue depth per managed user: `50`

### p25

- write requests: `120/min`, burst `40`
- enqueue posts: `500/min`, `1500/5min`
- active queue depth per workspace: `3000`
- active queue depth per managed user: `100`

### p50 / p75

- write requests: `240/min`, burst `60`
- enqueue posts: `1000/min`, `3000/5min`
- active queue depth per workspace: `10000`
- active queue depth per managed user: `250`

### p150+ (p150, p300, p500, p1000)

These tiers share runtime safety values. Runtime caps protect against operational burst, not monthly throughput — the monthly post quota already scales linearly across these tiers (20k → 200k posts/month). A workspace at p1000 has 10× the monthly headroom of p150 but does not need 10× the per-second burst tolerance; legitimate sustained throughput at the high end is handled via the per-tenant override hook below.

- write requests: `480/min`, burst `120`
- enqueue posts: `3000/min`, `10000/5min`
- active queue depth per workspace: `50000`
- active queue depth per managed user: `1000`

### Enterprise / custom override

Enterprise customers may eventually need overrides, but v1 can keep a static map in code (`internal/ratelimit/plans.go`). Per-workspace DB-backed overrides are Phase 3.

---

## 8. Endpoint policy matrix

### 8.1 Create / publish / retry

Apply all three:

- request limiter
- enqueue throughput limiter
- queue depth limiter

Endpoints:

- `POST /v1/posts` (create / immediate publish / scheduled)
- `POST /v1/posts/{id}/publish` (publish draft)
- `POST /v1/posts/{id}/results/{resultID}/retry` (retry now)

`POST /v1/posts/bulk` gets request limiter only (see §6.1 and §3 non-goals).

### 8.2 Cancel / reschedule / update

Apply request limiter only in v1.

Reason:

- they are write-heavy and can still flood DB
- they usually do not create new queue volume

Future:

- if cancel/retry storms become common, add separate cancel throughput controls

### 8.3 Read APIs

Use only very loose request limiting or leave unprotected in v1 except for public endpoints.

---

## 9. Redis design

## 9.1 Connection

Add a Redis client to the API service and initialize it in:

- [main.go](/Users/xiaoboyu/unipost/api/cmd/api/main.go)

Use environment configuration:

- `REDIS_URL`

If `REDIS_URL` is missing:

- admission control should fail open in local dev
- production should log loudly at startup

## 9.2 Key design

Suggested v1 keys:

- request limiter (workspace-aggregate, no per-route key — splitting by route would let traffic spread across endpoints to bypass the cap; keep `route` as a metric label, not a key dimension):
  - `rl:req:ws:{workspace_id}`
  - `rl:req:user:{workspace_id}:{external_user_id}` (only when `external_user_id` is non-NULL and resolved cheaply)

- enqueue sliding window:
  - `rl:enqueue:ws:{workspace_id}`
  - `rl:enqueue:user:{workspace_id}:{external_user_id}`

- optional future depth mirrors:
  - `rl:depth:ws:{workspace_id}`
  - `rl:depth:user:{workspace_id}:{external_user_id}`

## 9.3 TTLs

- request limiter keys: bounded by bucket refill horizon
- enqueue window keys: `5-10 min`
- depth mirror keys: no TTL or refreshed on write with reconciliation job

## 9.4 Lua scripts

Use Lua for:

- atomic token bucket updates
- atomic sliding-window check + insert

This avoids race conditions across replicas.

Both scripts return a 2-tuple `{allowed, retry_after_seconds}`:

- token bucket: `retry_after_seconds = ceil((1 - tokens) / refill_rate)` when denied
- sliding window: `retry_after_seconds = oldest_in_window_ts + window - now`

The Go caller writes `retry_after_seconds` into the `Retry-After` HTTP header on deny, so the client can honor a real backoff instead of guessing.

---

## 10. Code structure

## 10.1 New package

Add:

- `api/internal/redis` — owns the shared `*redis.Client`, `REDIS_URL` parsing, startup ping. Decoupled so future Redis consumers (idempotency cache, hot-path lookup cache, etc.) reuse the same client.
- `api/internal/ratelimit` — the Limiter package itself.

Suggested structure for `internal/ratelimit`:

- `limiter.go` — `Limiter` interface
- `redis_token_bucket.go` — request limiter (Lua-backed)
- `redis_sliding_window.go` — enqueue limiter (Lua-backed)
- `pg_depth.go` — queue depth limiter (Postgres + advisory lock)
- `circuit.go` — circuit breaker around Redis ops, used by both Redis-backed limiters
- `noop.go` — disabled-mode fallback for local dev when `REDIS_URL` is absent
- `plans.go` — static per-plan threshold map keyed on `plans.id`
- `errors.go`

### Interface

```go
type Limiter interface {
    AllowRequest(ctx context.Context, scope RequestScope) (Decision, error)
    AllowEnqueue(ctx context.Context, scope EnqueueScope, units int) (Decision, error)
    CheckQueueDepth(ctx context.Context, scope QueueScope, units int) (Decision, error)
}
```

## 10.2 Handler integration points

Apply checks in:

- [social_posts.go](/Users/xiaoboyu/unipost/api/internal/handler/social_posts.go) — `Create`, `PublishDraft`, `CancelPost`, `UpdateDraft`
- [social_post_retry.go](/Users/xiaoboyu/unipost/api/internal/handler/social_post_retry.go) — `RetryResult`
- [social_posts_drafts.go](/Users/xiaoboyu/unipost/api/internal/handler/social_posts_drafts.go) — draft routes (request limiter only)
- [social_posts_bulk.go](/Users/xiaoboyu/unipost/api/internal/handler/social_posts_bulk.go) — request limiter only (see §6.1)

Flow for create/publish/retry endpoints:

1. authenticate workspace (existing API-key middleware)
2. parse request
3. run request limiter (cheapest, fail-fast)
4. resolve target accounts and managed-user scope (existing `loadValidateAccounts` already does this for accounts; reuse its result for `external_user_id` resolution when all targets share one user)
5. run enqueue limiter with `units = number of posts being accepted`
6. acquire workspace advisory lock, run queue depth limiter, insert `social_posts` / `post_delivery_jobs`, release lock

The advisory lock is held for steps 6 only, not the whole request.

## 10.3 Queue depth queries

Add new sqlc queries for:

- active queue count by workspace
- active queue count by workspace + external_user_id

The second query will need to join:

- `post_delivery_jobs`
- `social_accounts`

through `social_account_id`.

---

## 11. Headers and observability

## 11.1 Response headers

Add optional headers on protected write endpoints:

- `X-UniPost-RateLimit-Limit`
- `X-UniPost-RateLimit-Remaining`
- `X-UniPost-RateLimit-Reset`
- `X-UniPost-QueueDepth`

These should supplement, not replace:

- `X-UniPost-Usage`
- `X-UniPost-Warning`

## 11.2 Metrics

Record:

- request limiter allow / deny counts
- enqueue limiter allow / deny counts
- queue depth limiter allow / deny counts
- top offending workspaces
- top offending external users

## 11.3 Logs

Log structured events on denial:

- workspace id
- route
- scope type
- current plan
- limiter type
- requested units
- current depth if relevant

---

## 12. Failure behavior

### 12.1 Redis down

Decision:

- a circuit breaker wraps every Redis call. After N consecutive timeouts/errors (default `5` within `10s`) the breaker opens for `30s`, during which the limiter switches to a **per-replica in-memory token bucket** sized at `plan_limit / replica_count` (rounded down, never zero).
- this is a degraded mode, not naïve fail-open: a single workspace can still burst but cannot bypass protection entirely just because Redis blips.
- log loudly on breaker open / close transitions and emit a metric so on-call sees it.
- after the breaker half-opens, one canary call probes Redis; success closes the breaker.

Reason:

- naïve fail-open lets one Redis hiccup become a free-traffic window for noisy workspaces.
- naïve fail-closed lets one Redis outage hard-break all publish APIs.

Queue depth checks that use Postgres are unaffected by Redis state.

### 12.2 Partial resolution of managed user

If managed-user scope cannot be determined cheaply or confidently:

- fall back to workspace-only checks

Do not reject the request just because `external_user_id` resolution is ambiguous.

---

## 13. Rollout plan

### Phase 1 (this PRD's main delivery)

- add `internal/redis` package + `REDIS_URL` config + startup ping
- add `internal/ratelimit` package with Limiter interface, Lua scripts, circuit breaker, noop fallback
- add migration for `post_delivery_jobs_workspace_active_idx`
- add sqlc query for active depth by workspace
- add request limiter on create / publish / retry / draft / bulk
- add enqueue sliding-window limiter on create / publish / retry
- add workspace queue-depth check on create / publish / retry, under advisory lock
- ship `Retry-After` and `RATE_LIMITED` normalized error code

### Phase 2

- add managed-user dimension (request limiter, enqueue limiter, depth check)
- add `X-UniPost-RateLimit-*` and `X-UniPost-QueueDepth` response headers
- add metrics and dashboard panels for allow/deny counters and top offenders
- tune thresholds against real production traffic

### Phase 3

- optional Redis-mirrored active depth (cache the `count` result)
- optional per-workspace plan overrides in DB or admin UI
- per-workspace concurrent-worker cap inside the claim queries (worker-domain protection — the missing layer for internal retry storms)
- scheduler claim-time admission for very large pre-scheduled batches
- optional platform-aware throttling

---

## 14. Open questions

1. ~~Should bulk create remain private-only while this ships, so we reduce the public burst surface first?~~ **Decided 2026-04-28: bulk stays as-is, undocumented; v1 covers it with request limiter only and revisits batch-aware admission once bulk has a public surface.**
2. **Decided: enqueue units count posts in v1**, not delivery jobs. Re-evaluate after Phase 2 metrics show whether multi-account fan-out is a real pressure source.
3. **Decided: cancel and update share the request limiter only in v1.** Revisit if metrics show cancel storms are an actual incident shape.
4. Should free-plan over-limit workspaces continue to publish indefinitely once runtime admission control exists, or should monthly quota eventually become a hard gate? — **Open**, not a v1 blocker.

---

## 15. Recommendation

Implement v1 as:

- new `internal/redis` package owning the shared `*redis.Client` (`REDIS_URL`, startup ping, reused by future Redis consumers)
- Redis-backed request limiter (token bucket, Lua, workspace-aggregate key, no per-route split)
- Redis-backed enqueue sliding-window limiter (Lua, units = post count, returns `Retry-After`)
- Postgres-backed queue-depth limiter under per-workspace advisory lock, with a new partial index on active states
- circuit breaker + per-replica degraded fallback (no naïve fail-open)
- workspace primary dimension; managed-user secondary dimension shipped in Phase 2
- bulk path gets request limiter only; batch-aware admission deferred
- static per-plan thresholds in code, covering all 9 plan tiers (`free`, `p10`, `p25`, `p50`, `p75`, `p150+`)

This is the smallest design that meaningfully reduces outage risk without rewriting UniPost's current publish architecture, and it stays compatible with the worker-domain and scheduler-domain protections that Phase 3 will add.
