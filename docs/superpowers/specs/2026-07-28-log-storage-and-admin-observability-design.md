# UniPost Log Storage, Errors, and Logs Performance PRD

**Status:** Approved, scope locked, and authorized for implementation planning

**Date:** 2026-07-28

**Owner area:** API, Developer Logs, API Metrics, Admin Errors, Admin Logs

**Target flow:** task branch -> Preview Acceptance -> `dev` -> `staging` -> `main`

## 1. Executive summary

UniPost currently stores most API-key request traffic twice: once as a rich `integration_logs` row and again as a lean `api_metrics` row. Successful request logs also persist request and response payloads that are rarely needed. Separately, the cross-platform publishing debug recorder has embedded complete request bodies in `debug_curl`; the observed extreme rows came from TikTok binary uploads, but the unbounded request-body path is platform-independent.

These two patterns cause almost all current database growth and the observed Admin Logs and Admin Errors performance failures.

This PRD adopts a unified API request event model:

- one lean `api_request_events` record per API-key request;
- no request or response payload for successful requests;
- a separate, bounded, redacted detail record only for failed requests;
- API Metrics and Developer Logs read from the same canonical request event;
- non-API product events remain in PostgreSQL;
- internal runtime logs, stack traces, worker health, and infrastructure telemetry go to Better Stack;
- business state, audit records, outbox entries, delivery records, and idempotency receipts remain in PostgreSQL.

The target is to reduce the production database from approximately 7.22 GB to no more than 2.8 GB after log-only historical compaction, operate toward a 2.5 GB steady-state capacity objective, reduce daily physical growth by at least 75%, and bring cold Admin Logs load time below 1.5 seconds. The lower capacity target is intentionally secondary to product safety: this project does not rewrite or lock publishing, account-connection, authentication, billing, or other customer business tables to reclaim additional space.

## 2. Production evidence

The following figures were measured read-only against the production PostgreSQL database on 2026-07-28 America/Los_Angeles, corresponding to 2026-07-29 UTC.

### 2.1 Database composition

| Relation | Estimated rows | Total size | Share of database | Finding |
| --- | ---: | ---: | ---: | --- |
| `integration_logs` | 1.71 million | 5.71 GB | 79.16% | Dominant storage source |
| `api_metrics` | 1.64 million | 858 MB | 11.89% | Per-request duplicate with no retention worker |
| `social_post_results` | 13.2 thousand | 505 MB | 7.00% | 490 MB is large-field/TOAST storage |
| All other relations | — | About 145 MB | 1.95% | Not a current capacity concern |

The three largest relations account for 98.05% of the database.

### 2.2 `integration_logs`

The 5.71 GB relation consists of approximately:

| Component | Size |
| --- | ---: |
| Heap | 2.21 GB |
| TOAST and auxiliary table storage | 1.79 GB |
| Indexes | 1.71 GB |

A repeatable one-percent table sample found:

- 93.02% of rows were successful `api_request` events;
- request payloads were present in 93.91% of sampled rows;
- response payloads were present in 94.44% of sampled rows;
- successful API request rows averaged about 1.96 KB of request and response payload data;
- the sampled successful request payload footprint extrapolates to approximately 2.9 GiB.

The production Logs investigation also established that the Admin Logs query has no global `(ts, id)` index. Its default cross-workspace request scanned approximately 1.71 million rows and about 2.1 GB of database pages before returning 150 small records.

### 2.3 Duplicate API request storage

All workspace-scoped routes currently install both middleware in sequence:

1. `integrationlogs.Middleware(integrationLogger)`;
2. `apiMetricsRecorder.Middleware`.

For API-key traffic, both middleware write a raw per-request row. The production counts corroborate this path: the estimated number of successful API request integration logs closely matches the 1.64 million `api_metrics` rows.

### 2.4 `debug_curl`

The legacy debug recorder is not wholly unbounded. It already limits each response excerpt to 8 KB and each publish attempt to 16 failing entries. The actual unbounded component is the request path: `io.ReadAll(req.Body)` reads the complete request body and `buildCurl` renders all of it into `--data`. The fix therefore targets request capture while retaining or tightening the existing response and entry limits.

Production contained 1,119 non-empty `debug_curl` values totaling approximately 473 MB uncompressed:

- 20 rows larger than 1 MB accounted for approximately 472 MB;
- the remaining 1,099 rows accounted for only about 1.5 MB;
- the largest individual value was approximately 30 MB;
- all extreme values were recent and consistent with binary upload bodies being embedded in curl text;
- the extreme production samples were TikTok uploads, but the shared write path covers every publishing platform.

The Admin Errors list returned `debug_curl` for every result. A 100-item production response transferred approximately 73.6 MB and took approximately 15.6 seconds to complete.

### 2.5 Capacity impact model

| Change level | Expected production size | Expected reduction |
| --- | ---: | ---: |
| Errors/Logs query fixes and bounded future `debug_curl` capture only | About 6.7 GB | 6–7% |
| Successful request payload removal, without removing duplicate rows | About 3.7–4.3 GB | 40–49% |
| Unified request event model plus log-only compaction | About 2.0–2.5 GB | 65–72% |

Sending internal runtime logs to Better Stack does not itself materially shrink the current database because structured `slog` output is already sent to Better Stack and is not the source of the three dominant PostgreSQL relations. The in-scope capacity benefit comes from removing duplicate per-request rows, removing successful payloads, bounding failure details, and physically compacting log-owned storage. The existing `social_post_results` footprint remains in the estimate because that customer publishing table is outside the allowed migration and compaction scope.

## 3. Goals

1. Replace duplicate raw writes with one canonical API request event representation.
2. Preserve successful requests as structured metadata without request or response payloads.
3. Preserve bounded, redacted payload detail only for failures.
4. Preserve existing customer Developer Logs retention entitlements.
5. Continue supporting Logs, API Metrics, Admin Logs, and Admin Errors without a customer-visible feature regression.
6. Reduce production database physical size to no more than 2.8 GB after migration and log-only compaction, with a 2.5 GB steady-state operating objective.
7. Reduce sustained daily physical growth by at least 75%.
8. Reduce Admin Logs cold-query time below 1.5 seconds.
9. Reduce Admin Errors and Admin Logs list responses below 250 KB.
10. Ensure Better Stack unavailability cannot affect customer API behavior or product log availability.
11. Preserve the behavior, response contracts, transaction boundaries, jobs, retries, and data writes of every customer-facing flow outside Logs, Metrics, and Admin observability.

## 4. Non-goals

- Do not use Better Stack as the source of truth for customer Developer Logs.
- Do not move audit records, outbox rows, delivery state, idempotency receipts, or billing events out of PostgreSQL.
- Do not change customer plan retention entitlements.
- Do not redesign the visual appearance of the Logs or Errors pages.
- Do not add customer-facing rollout flags; one temporary, internal Admin read-path kill-switch is explicitly in scope for Stage 2.
- Do not provide full-text search over request or response payloads.
- Do not preserve binary request bodies for replay.
- Do not change publishing, scheduling, delivery, retry, OAuth, Connect, account linking, token refresh, authentication, authorization, billing, quota, onboarding, webhook, or idempotency behavior.
- Do not rewrite, compact, change the schema of, or run blocking maintenance against customer business tables, including `social_posts`, `social_accounts`, `social_post_results`, `post_delivery_jobs`, outbox tables, and receipt/idempotency tables.
- Do not add a synchronous Better Stack call, log persistence dependency, or observability transaction to a customer request path.

### 4.1 Hard scope boundary

The implementation is limited to:

- log-, metrics-, debug-, rollup-, retention-, and audit-owned schemas and workers;
- telemetry middleware and recorders that observe a completed or already-determined request outcome;
- Logs, Metrics, Admin Logs, Admin Errors, and their detail endpoints;
- the internal Admin read-path feature flag and Better Stack telemetry plumbing;
- stopping future writes to the diagnostic-only `social_post_results.debug_curl` field, without rewriting its historical rows or changing any other `social_post_results` behavior.

Publishing and account-connection code may expose an already-determined outcome to an observability adapter, but the adapter must not participate in business decisions. It must not change request or job payloads, provider calls, token handling, database transactions, retry classification, timeout budgets, queueing, delivery state, response status, or response body.

Any implementation diff outside these surfaces requires a new PRD decision and explicit user approval. Discovery that the log work cannot be completed without changing a protected flow is a hard stop, not permission to expand scope.

## 5. Data classification and ownership

| Data class | System of record | Required behavior |
| --- | --- | --- |
| Runtime logs, panic traces, SQL failures, worker health, deployment telemetry | Better Stack | Internal-only, structured, alertable, no customer payloads |
| API-key request events | PostgreSQL `api_request_events` | Customer-visible, tenant-scoped, one row per request |
| API request failure details | PostgreSQL `api_request_error_details` | Failure-only, redacted, bounded, on-demand |
| Publishing, OAuth, Connect, webhook, and customer-actionable system events | PostgreSQL `integration_logs` | Product events; no new `api_request` rows |
| Optional non-API event failure details | PostgreSQL `integration_log_details` | Failure-only, bounded, on-demand |
| Publishing failure facts | PostgreSQL `post_failures` | Structured business and support record |
| Publishing HTTP diagnostic detail | PostgreSQL `post_failure_debug_details` | Bounded replacement for the legacy request-body-unbounded `debug_curl` path |
| Security-sensitive mutations | PostgreSQL `audit_log` | Durable, tenant-scoped audit evidence |
| Outbox, delivery, receipt, attempt, and idempotency state | Existing PostgreSQL business tables | Operational state, not disposable logs |

Better Stack must never become a dependency of the customer Logs API, the Metrics API, a publishing workflow, account connection, authentication, billing, or another product-critical request path. No log or debug record may be written in the same transaction as a customer business-state mutation.

## 6. Target architecture

### 6.1 Write path

Each API-key request attempts one canonical request event write. Telemetry persistence remains outside the customer response contract, but the system must never intentionally write the same request into two raw event tables.

- A successful request creates one lean `api_request_events` row.
- A failed request creates one `api_request_events` row plus one atomic `api_request_error_details` row.
- The request does not create an `api_metrics` row.
- The request does not create an `integration_logs` row with `category='api_request'`.
- API Metrics aggregates the canonical request events and their rollups.
- Developer Logs projects the same canonical request events into its list and detail response contracts.

Non-API product events continue through the integration event writer. Runtime and infrastructure telemetry uses structured `slog` and Better Stack only.

All observability writes are best-effort from the perspective of customer behavior. Queue saturation, PostgreSQL telemetry errors, Better Stack errors, serialization errors, and redaction errors emit health signals but do not change the business response or commit/rollback decision. The recorder may observe immutable copies of identifiers and outcomes only after the business path has established them.

### 6.2 `api_request_events`

The event is deliberately narrow and contains only fields used for customer Logs, Metrics, correlation, filtering, and support:

- opaque time-sortable event ID;
- `workspace_id`;
- `api_key_id`;
- `occurred_at`;
- `request_id`;
- `trace_id`, when present;
- normalized `method`;
- normalized `route_pattern`;
- `status_code`;
- `duration_ms`;
- normalized `outcome`;
- normalized `error_code`, when present;
- optional `profile_id`, `social_account_id`, and `post_id` when the handler has an authoritative association.

The base event must not contain:

- request or response body;
- headers;
- query parameters;
- arbitrary JSON metadata;
- unnormalized resource URLs;
- provider error strings.

The route must come from the matched Chi route pattern. A conservative fallback may normalize a path only when no route pattern exists. Query strings and raw resource identifiers must never be stored as part of the route.

Outcome normalization is deterministic:

- `2xx` and `3xx`: success;
- `400`: client or validation error, using the structured application error code when available;
- `401`: authentication error;
- `402`: plan or quota gate;
- `403`: authorization or platform-policy error;
- `404`: not found;
- `409`: conflict;
- `429`: rate limited;
- `5xx`: server error.

Every status remains eligible for the lean event. A detail is permitted only for `4xx` and `5xx` outcomes. Plan/quota polling responses (`402`) remain metadata-only. Rate-limit responses may retain a bounded response body but never a request body.

The request body capture path uses a capped tee while the handler reads the request; it must not pre-read the entire body. The response writer begins bounded capture only after a failure status is known. Successful responses are not buffered for logging.

### 6.3 `api_request_error_details`

The detail table is one-to-one with a failed request event and stores:

- event identity and event timestamp needed to reference the partitioned parent;
- allowlisted request headers;
- allowlisted response headers;
- sanitized query summary, only when explicitly allowed;
- sanitized request payload;
- sanitized response payload;
- original and stored byte counts;
- content types;
- SHA-256 for omitted binary content only when the complete body fits within the 32 KB observation budget; oversized binary records `hash_omitted_size` instead of hashing the full body on the customer path;
- truncation flags and omission reasons;
- redaction policy version;
- creation timestamp.

Limits:

- request payload: 32 KB maximum;
- response payload: 32 KB maximum;
- combined serialized detail response: 70 KB maximum;
- binary and multipart bodies: zero stored body bytes.

The detail is never selected by list queries. It is loaded only through a detail endpoint after authorization against the parent event.

### 6.4 `integration_logs`

The existing physical name remains during the first implementation to reduce migration risk. Its responsibility narrows to non-API product events:

- publishing lifecycle;
- OAuth and Connect lifecycle;
- webhook delivery lifecycle;
- customer-visible worker events;
- customer-actionable system events.

New `category='api_request'` writes stop after cutover. The base row remains lean. Any request/response or provider payload needed for an error moves to an optional one-to-one `integration_log_details` record with the same redaction and size principles.

### 6.5 Publishing failures and Errors

`post_failures` remains the unchanged structured source for publishing failure history. `social_post_results` remains the unchanged current result state. Writes through the request-body-unbounded `social_post_results.debug_curl` path stop, but its existing schema and historical rows are not modified by this project. Existing response-body and failing-entry caps are preserved or tightened by the new structured representation.

`post_failure_debug_details` stores a bounded diagnostic representation:

- method and redacted URL;
- allowlisted headers;
- JSON/text request excerpt when safe;
- response status and bounded response excerpt;
- original size, stored size, hashes, and omission reasons;
- request duration and capture timestamp.

For supported JSON/text requests, the service may render a capped, safe curl command on demand. For binary or multipart requests, the rendered command contains a body omission explanation and never embeds the binary data. This capture occurs outside publishing business transactions and cannot alter publishing success, failure classification, retry, or latency budgets.

Capture limits:

- request body: 32 KB;
- response body: 16 KB;
- failing calls per publish attempt: 8;
- total serialized debug detail: 64 KB.

## 7. Read paths and API behavior

### 7.1 Logs list

Customer Logs and Admin Logs read two indexed sources:

- `api_request_events`;
- non-API `integration_logs`.

Each source applies its own tenant, time, and structured filters before producing a bounded candidate set. The service merges the candidates and applies a stable global order:

1. event timestamp descending;
2. source kind as a deterministic tie-breaker;
3. source event ID descending.

The API uses an opaque cursor containing all three ordering components. OFFSET pagination is prohibited. The API requests `limit + 1` rows to determine whether a next cursor exists.

List projections contain base fields only. They must not join, select, deserialize, or transfer any detail payload.

Defaults and limits:

- Admin Logs default range: 24 hours;
- Admin Logs default page size: 100;
- maximum page size: 200;
- customer ranges remain bounded by plan retention;
- list response target: less than 250 KB.

### 7.2 Logs detail

The detail endpoint first resolves and authorizes the base event. It fetches a detail row only when the event indicates that one exists.

- Success events return base structured metadata and no payload object.
- Failed events may return bounded request and response detail.
- Expired details return the base event with `detail_status='expired'` rather than treating the event as missing.
- Cross-workspace, unauthorized, or unknown IDs return `404`.

### 7.3 Search

Search prioritizes exact structured fields:

- request ID;
- post ID;
- social account ID;
- profile ID;
- error code;
- route pattern;
- owner email in the Admin surface.

Substring search is allowed only over short base text fields and requires an explicit bounded time range. Payload search is not supported. A trigram or full-text index may be added only after production query evidence justifies its write and storage cost.

### 7.4 Errors list and detail

The Errors list:

- defaults to 50 records;
- allows at most 100 records;
- uses cursor pagination;
- never selects or returns `debug_curl` or debug detail;
- returns `has_debug_detail`, `debug_detail_size`, and `debug_detail_truncated`.

The debug detail is loaded from a dedicated endpoint when the user opens one failure. The frontend must not keep every failure detail in list state.

## 8. Index strategy

### 8.1 `api_request_events`

Required indexes:

- workspace and descending event time for customer Logs;
- descending event time for Admin Logs;
- workspace and request ID, partial for non-null request IDs;
- workspace, normalized route, and descending time for recent endpoint queries.

Status, outcome, API key, and every other optional filter must not each receive a large standalone index by default. Long-range metrics use rollups. Additional indexes require an observed production query plan and a measured benefit.

### 8.2 `integration_logs`

Required indexes:

- workspace and descending `(ts, id)`;
- descending global `(ts, id)`;
- confirmed exact lookup indexes for request, post, and social-account correlation.

The existing `(workspace_id, category, ts)`, `(workspace_id, status, ts)`, `(workspace_id, action, ts)`, `(workspace_id, platform, ts)`, and metadata GIN indexes enter a 14-day observation window. They must be evaluated as composite indexes against supported query predicates, not described or assessed as single-column indexes. The implementation records query plans and index usage, then removes only indexes proven redundant or unused by supported queries.

All new production indexes on populated tables must be created concurrently through a deployment-safe migration procedure.

## 9. API Metrics and rollups

### 9.1 Raw metrics

Recent metric calculations read `api_request_events`. No second raw metrics event is created.

### 9.2 Hourly rollup

`api_request_metric_rollups_hourly` stores:

- hour bucket;
- workspace;
- workspace-level dimensions required by the current Metrics product;
- normalized route;
- method;
- status code;
- request, error, server-error, validation-error, and rate-limit counts;
- duration sum, minimum, and maximum;
- fixed duration histogram bucket counts.

The histogram supports long-range approximate p50, p95, and p99 values without retaining every raw event solely for metrics. Recent ranges backed by raw events remain exact. Existing response fields remain compatible.

Per-API-key rollups are excluded from the first release because the current product does not expose per-key metrics and the dimension would substantially increase rollup cardinality. A future per-key metrics feature requires its own measured storage design.

The rollup has an idempotent uniqueness contract. The worker processes completed hours and recomputes the most recent 48 hours to incorporate late events and deployment interruptions.

The current maximum 90-day Metrics query range remains unchanged. Hourly rollups are retained for 90 days.

## 10. Retention and partitioning

Plan-based retention for the existing `integration_logs` table already exists through `IntegrationLogRetentionWorker` and `RetentionDaysForPlan`; this PRD does not claim it as a new capability. New work extends the same entitlement mapping to `api_request_events`, introduces a bounded raw-metric lifecycle where `api_metrics` previously had no retention, adds the missing audit/debug policies, and changes execution to partition-aware bounded cleanup.

### 10.1 Retention policy

| Data | Retention |
| --- | --- |
| API request events | Free 1 day; API 7; Basic 14; Growth 30; Team 90; Enterprise 180 |
| API request error details | Same as parent event |
| Integration events | Existing plan policy: 1/7/14/30/90/180 days |
| Integration event details | Same as parent event |
| API Metrics hourly rollups | 90 days |
| Audit log, non-Team | 90 days |
| Audit log, Team/Enterprise | 365 days |
| Structured post failures | 365 days |
| Post failure debug details | 30 days |
| Better Stack runtime telemetry | 30 days by default; incident evidence is separately archived |

### 10.2 Partitioning

High-growth raw event tables use weekly range partitions on event timestamp. Weekly partitions keep individual relation and index sizes bounded at the observed 50,000–115,000 events per day.

Because customer retention differs by current plan, shorter-plan expiration still deletes in small batches within active partitions. Once a partition is older than the maximum 180-day raw-event retention, it can be detached and dropped after a boundary check.

Partitioned event identity and detail references must include the event timestamp required by PostgreSQL partition uniqueness rules. External APIs expose one opaque identifier and do not expose the physical partition key contract.

### 10.3 Retention execution

- Delete in bounded batches rather than one large workspace transaction.
- Continue processing other workspaces when one workspace fails.
- Record and alert on the oldest expired row that remains.
- Cascade detail deletion from its parent event during row-level deletion.
- Verify partition maximum timestamp and retention entitlement before dropping a partition.
- Tune autovacuum per active partition and monitor dead tuples and retention lag.

### 10.4 Parent and detail partition lifecycle

Parent event partitions and their one-to-one detail partitions use identical weekly boundaries and a recorded parent/child partition manifest. Row-level `ON DELETE CASCADE` does not run when a partition is dropped. The partition lifecycle operation must therefore:

1. verify both parent and detail partition bounds;
2. stop or reject writes into the closed interval;
3. detach the aligned detail partition and parent partition as one controlled operation;
4. validate that neither detached partition received an out-of-range row;
5. drop both detached partitions only after backup and retention checks;
6. alert and stop if one side is missing or has different bounds.

Partition lifecycle integration tests must prove that dropping an old parent partition cannot orphan a detail partition.

### 10.5 Short-plan retention inside long-lived partitions

Weekly time partitions cannot directly implement Free's one-day retention while Team and Enterprise events remain for 90 or 180 days. Free, API, Basic, and Growth expiration therefore remains row-level deletion inside active weekly partitions. This creates dead tuples until autovacuum reuses the space and does not immediately return storage to the Railway volume.

The capacity model depends on this cleanup remaining healthy. Active partitions require lower, measured autovacuum thresholds, bounded daily deletion, dead-tuple monitoring by plan, and a retention-lag alert. Whole-partition drop remains the physical reclamation mechanism only after the longest 180-day entitlement has expired.

## 11. Privacy and security

All PostgreSQL and Better Stack log paths use one versioned redaction library.

Rules:

- headers use an allowlist, not an expanding sensitive-header denylist;
- `Authorization`, cookies, API keys, tokens, secrets, passwords, and signatures are never stored;
- query parameters are omitted by default;
- JSON is recursively redacted by normalized key rules;
- arbitrary provider error strings are not assumed safe because they appear under a safe JSON key;
- non-JSON text is stored only for explicitly allowed content types;
- binary and multipart bodies are never read for diagnostic persistence;
- binary metadata contains content type, byte count, SHA-256, and omission reason only;
- Better Stack receives internal structured summaries and correlation IDs, not customer payloads;
- each detail records the redaction policy version.

Every customer event contains `workspace_id`. Workspace scope is enforced in SQL and detail authorization, not only in the frontend. Admin Logs, Admin Errors, and debug detail require Super Admin. Accessing publishing debug detail emits a security audit event.

## 12. Write reliability and failure handling

Customer API behavior must not depend on successful telemetry persistence.

The unified recorder uses bounded queues and batch inserts instead of spawning an unbounded goroutine per metrics event.

- Success events use the normal asynchronous batch queue.
- Failure events and details use a high-priority queue and one atomic database transaction.
- If the high-priority queue is full, the service attempts a direct short-timeout write.
- A full success queue may drop success telemetry, but every drop increments a counter and can trigger an alert.
- A failure event must never be silently dropped.
- Better Stack errors never affect customer responses.
- Telemetry persistence failure must never fail, roll back, delay, or reclassify a publishing, account-linking, authentication, billing, or other customer operation.

Exposed health signals:

- queue depth by priority;
- dropped success event count;
- failed failure-event write count;
- batch size and batch latency;
- event insert latency;
- rollup freshness;
- retention lag.

Rollup workers are idempotent. Recent Metrics may fall back to raw events when rollups are delayed. The API returns an explicit data-delay state rather than fabricated zeros when neither source is sufficiently fresh.

The acceptance window requires zero dropped success events. The exceptional drop behavior exists to protect the customer request path during overload; it is not considered normal successful operation.

## 13. Migration plan

Migration is split into independently deployable and reversible stages.

### Stage 0: Stop the immediate performance and growth failures

- remove `debug_curl` from Errors list queries and responses;
- add a dedicated debug detail endpoint;
- bound request capture and omit binary bodies;
- stop Logs list queries from selecting request and response payloads;
- create the Admin Logs global time index concurrently;
- verify production list response sizes and query plans.

Stage 0 may change only diagnostic capture and Admin/Logs projections. It must not change publishing requests, provider calls, account-connection callbacks, business transactions, retries, statuses, or customer-facing response contracts.

### Stage 1: Introduce the new model

- create weekly-partitioned request event and detail tables;
- create hourly metric rollups;
- introduce the unified recorder;
- keep existing writers temporarily for a bounded comparison window;
- add counters that compare old and new write paths;
- register the environment-global internal flag `observability_reads_v2`, seeded OFF, in the existing PostgreSQL feature flag control plane;
- display and audit this flag through `/admin/feature-flags` and the existing Super Admin API.

The flag is evaluated by the backend through the global evaluator, not the workspace evaluator, so the existing Super Admin workspace bypass cannot accidentally force new reads. It controls reads only and never gates writes, retention, backfill, or destructive migration.

Flag contract:

| Field | Value |
| --- | --- |
| Key | `observability_reads_v2` |
| Owner | API / Admin Observability |
| Production default | OFF |
| OFF behavior | Logs, Metrics, and Errors use the compatible legacy read projection |
| ON behavior | Logs, Metrics, and Errors use the unified request-event/detail projection |
| Rollback | Turn OFF in `/admin/feature-flags` while dual writes remain healthy |
| Third-party dependency | None |

### Stage 2: Switch reads

- turn `observability_reads_v2` ON in dev, then staging, then production after each environment's exact-SHA acceptance;
- when ON, switch Logs to the merged request-event and integration-event projection;
- when ON, switch Metrics to raw request events plus hourly rollups;
- when ON, switch Errors to list metadata plus on-demand debug detail;
- when OFF, immediately return all three surfaces to their compatible legacy reads without redeployment;
- compare dev and staging results against the old paths on the exact deployed SHA.

The kill-switch is valid only while old and new raw writers both remain current. Each ON/OFF transition is audited through `feature_flag_changes` and emits an internal Better Stack event.

### Stage 3: Stop duplicate writes

- require `observability_reads_v2` to remain continuously ON with successful production acceptance for at least 48 hours;
- stop `api_metrics` writes;
- stop `integration_logs` `api_request` writes;
- make the unified recorder the only API request event writer;
- monitor count parity, queue health, dropped events, and write failures for at least 48 hours.

Once old writes stop, OFF can no longer provide a current legacy view. Stage 3 therefore locks the flag ON and removes the toggle action from the Admin page. The flag is removed with the old read code and old relations in Stage 5.

### Stage 4: Backfill retained history

- backfill API request events still inside the applicable plan retention window;
- migrate successful requests as metadata only;
- migrate failed payloads only after applying current redaction and size limits;
- copy non-API integration events into the compact partitioned replacement;
- generate required rollups;
- validate counts by workspace, hour, route, method, status, and error code.

### Stage 5: Reclaim physical space

- delete the obsolete `api_metrics` relation after validation;
- swap the compact partitioned integration event relation in place of the 5.71 GB legacy relation;
- leave historical `social_post_results.debug_curl` values and the physical `social_post_results` relation untouched;
- remove old relations only after backup and post-cutover observation.

Production does not expose `pg_repack` in `pg_available_extensions`, and `social_post_results` is referenced by multiple foreign keys. Both an unverified shadow-table swap and a blocking `VACUUM FULL social_post_results` are rejected by this PRD because they could affect normal publishing. Reclaiming its historical diagnostic TOAST footprint requires a separate, explicitly approved maintenance design.

Plain `DELETE` and ordinary `VACUUM` are insufficient acceptance evidence because they usually make space reusable without returning it to the Railway volume. Acceptance uses `pg_total_relation_size` and `pg_database_size` after the physical compaction stage.

## 14. Migration safety and rollback

Before any destructive or space-intensive stage:

- Railway backup gate must succeed;
- staging must complete a production-scale rehearsal;
- the database volume must have at least 3 GB of temporary free capacity, or the migration must use an alternative bounded-copy procedure;
- long-running transactions and blocked autovacuum must be checked;
- exact source commits and changed files must pass promotion audit.

Rollback behavior:

- before destructive cleanup, old tables remain readable;
- during Stage 2, a failed read cutover first turns `observability_reads_v2` OFF; application rollback remains available while both schemas remain compatible;
- old raw writers remain available only for the bounded comparison stage;
- obsolete tables are retained for at least 48 hours after successful cutover when volume capacity permits;
- after physical cleanup, recovery relies on the verified Railway backup;
- database downgrade migrations are not required for application rollback.

### 14.1 High-risk implementation controls

The following controls are release blockers, not optional implementation notes:

1. **Dual-source cursor:** the global order is `(timestamp DESC, source_kind ASC, source_id DESC)`. The next-page predicate is `timestamp < cursor.timestamp`, or equal timestamp with a later source kind, or equal timestamp and kind with a smaller source-local ID. IDs are compared only within the same source kind. Boundary tests cover identical timestamps, empty sources, deletion between pages, and page-size transitions.
2. **Aligned partition drop:** parent and detail partitions must share boundaries and be detached/dropped together; row-level cascade is not relied upon for partition removal.
3. **Short retention:** dead tuples, autovacuum progress, and per-plan retention lag must remain within measured thresholds before capacity acceptance.
4. **Protected business tables:** no migration, rewrite, compaction, new lock, retention job, or schema change may target `social_post_results` or another customer business table. Any such need is a hard stop requiring a separate approved design.
5. **Read kill-switch lifetime:** `observability_reads_v2` is reversible only during dual write, is locked ON before old writes stop, and is removed during legacy cleanup.
6. **Customer-path isolation:** telemetry code cannot participate in business transactions or decisions. Its failure, timeout, queue saturation, or third-party outage must leave status codes, bodies, jobs, retries, provider calls, and committed business rows unchanged.

## 15. Monitoring and alerts

### 15.1 Capacity dashboard

Track:

- database total size;
- heap, TOAST, and index size for each event relation;
- event rows per day;
- average success event size;
- average and maximum failure detail size;
- details exceeding 32 KB and 64 KB thresholds;
- partition count and size;
- dead tuples;
- retention lag.

### 15.2 Product and query dashboard

Track:

- Admin and customer Logs p50/p95/p99 latency;
- Errors list and detail p95 latency;
- list and detail response sizes;
- database rows and buffers read per query;
- recorder queue depth, drops, and failures;
- rollup freshness;
- Better Stack ingestion failures.

Alert when:

- any stored payload exceeds its hard limit;
- any failure event cannot be persisted;
- success event drops continue for five minutes;
- retention lag exceeds 48 hours;
- rollup freshness exceeds two hours;
- post-migration physical growth exceeds 50 MB per day on a sustained basis;
- the post-migration database exceeds 3.3 GB.

## 16. Implementation decomposition

This PRD is intentionally an umbrella design. It must not be implemented as one large pull request or one indivisible migration. After PRD approval, create a separate implementation plan and review cycle for each workstream:

1. **Immediate Errors and Logs performance containment:** remove list payloads, bound `debug_curl`, add detail reads, and add the Admin Logs global index.
2. **Canonical request event write model:** schema, partition creation, redaction library, bounded capture, queueing, and shadow comparison.
3. **Logs and Metrics read migration:** merged cursor contract, Metrics rollups, dashboard/API compatibility, and old/new parity checks.
4. **Retention and observability:** plan-aware batch retention, audit retention, partition lifecycle, database capacity dashboard, and alerts.
5. **Historical migration and physical compaction:** backfill, relation swap, `api_metrics` retirement, legacy curl cleanup, and Railway volume reclamation.

Each workstream receives its own task branch, Preview Acceptance, environment verification, promotion audit, rollback point, and production acceptance evidence. A destructive workstream cannot begin merely because a preceding code workstream merged.

Every workstream also receives an explicit file and relation allowlist. A changed file or database relation outside the approved observability surface blocks the PR until it is removed or separately approved.

## 17. Test plan

### 17.1 Unit tests

- a successful request creates one event and no detail;
- each supported failure creates one event and one detail;
- JSON redaction is recursive;
- header allowlisting excludes every credential class;
- query values are omitted by default;
- binary and multipart bodies are not read or persisted;
- request, response, and total detail limits are enforced;
- route normalization does not expose resource IDs;
- rollup recomputation is idempotent;
- plan retention mappings remain unchanged.
- logging failures do not change handler status, response body, business transaction outcome, queued jobs, retry decisions, or provider call count;
- the publishing and account-connection adapters expose outcomes without accepting control-flow decisions back from observability code.

### 17.2 PostgreSQL integration tests

- one API-key request does not write duplicate raw rows;
- failure event and detail persist atomically;
- workspace isolation is enforced;
- merged cursor pagination has no duplicate or missing records;
- request and integration events have stable global ordering;
- parent deletion cascades to details;
- weekly partition routing and expiration are correct;
- batch retention does not cross a plan entitlement boundary;
- Metrics rollup counts match raw event counts.
- log migrations acquire no blocking maintenance lock on protected business tables;
- protected business-table row mutations are identical with observability enabled, disabled, saturated, and unavailable.

### 17.3 Security tests

Use payloads containing bearer tokens, cookies, API keys, OAuth tokens, client secrets, password fields, signed query parameters, multipart video, and nested provider responses. Assert that secrets and binary bodies do not appear in PostgreSQL, API responses, Better Stack, test logs, emails, or support bundles.

### 17.4 Performance tests

Use at least two million production-shaped request events.

- Admin Logs cold query: less than 1.5 seconds;
- Admin Logs warm query: less than 500 milliseconds;
- customer Logs p95: less than 750 milliseconds;
- Errors list p95: less than 750 milliseconds;
- Logs and Errors list responses: less than 250 KB;
- detail response: less than 70 KB;
- query plans use expected time indexes;
- list plans do not read detail tables or payload TOAST data.

### 17.5 Migration validation

Compare old and new paths by workspace, UTC hour, route, method, status code, outcome, API key, and error code.

- total request count difference is zero except for explicitly enumerated historical drops;
- every migrated failure event has the expected detail state;
- Metrics count and error-rate values match;
- recent raw percentiles match exactly;
- rollup percentile error remains within the defined histogram bucket tolerance;
- sample at least 100 workspaces and every supported platform;
- no retained detail exceeds limits or contains a known credential.

### 17.6 Customer core-flow non-regression

Run the existing end-to-end and deployed regression coverage for publishing, scheduling, delivery retries, connected-account listing, OAuth/Connect callback validation, authentication, quota/billing gates, and idempotency. Compare the accepted baseline and candidate on the exact deployed SHA.

Except for the explicitly changed Logs, Metrics, Admin Logs, Admin Errors, and debug-detail surfaces, the candidate must produce:

- identical HTTP status and response contracts;
- identical business-table mutations and transaction boundaries;
- identical jobs, retry classifications, and provider call counts;
- no synchronous call to Better Stack or an observability datastore;
- user-path p95 latency increase no greater than the larger of 5 milliseconds or 2%;
- successful customer operations when Better Stack is unreachable and when the telemetry queue or database writer is forced to fail.

## 18. Release gates

Each migration stage uses a separate pull request and the standard environment promotion flow.

Before moving to the next environment or stage:

- relevant local API and database integration tests pass;
- Preview Acceptance passes on the exact PR head SHA;
- GitHub, Railway, Vercel, and deployed regression checks finish successfully;
- real dev acceptance passes;
- real staging acceptance and migration rehearsal pass;
- promotion content audit finds no unrelated or unidentified commit;
- destructive operations have a successful backup gate;
- the actually deployed SHA matches the audited SHA.
- changed files and database relations are confined to the explicit observability allowlist;
- the customer core-flow non-regression suite passes with healthy, saturated, and unavailable telemetry dependencies;
- no migration plan or execution plan takes a blocking lock on a protected business table.

Any failed, timed-out, skipped, cancelled, missing, or wrong-SHA required check is a hard stop.

## 19. Acceptance criteria

The project is complete only when all conditions hold in production:

1. The physical production database is no larger than 2.8 GB immediately after log-only compaction; 2.5 GB remains the steady-state operating objective and 3.3 GB the capacity alert threshold. Missing this capacity goal cannot justify maintenance on a protected business table.
2. Every successfully persisted API-key request has exactly one canonical request event and no duplicate raw event; the production acceptance window has zero dropped success events.
3. `integration_logs` receives no new `api_request` event.
4. `api_metrics` receives no new row and is removed after migration validation.
5. Successful API request events contain no request or response payload.
6. Only failures may have a detail record.
7. No detail exceeds its hard size limit or contains binary body content.
8. Admin Errors list responses are below 250 KB.
9. Admin Logs cold load completes in less than 1.5 seconds.
10. Sustained daily physical growth falls by at least 75% from the measured baseline.
11. Existing customer log retention entitlements remain unchanged.
12. Logs, Metrics, and Errors return equivalent supported information after migration.
13. Better Stack failure does not affect customer API behavior or product log availability.
14. Production monitoring, retention, rollup, and capacity alerts are active.
15. Publishing, scheduling, delivery, retries, account connection, authentication, billing/quota, onboarding, webhooks, and idempotency retain their existing behavior and response contracts.
16. No protected business table is rewritten, compacted, schema-changed, or exposed to a new blocking migration lock.
17. Forced Better Stack, telemetry queue, redaction, serialization, and telemetry database failures do not change any customer operation result.

## 20. Decision record

- Better Stack is internal observability only and is not the Developer Logs source of truth.
- The unified API request event model is selected over incremental dual-table optimization.
- Successful API requests retain structured metadata only.
- Full bounded payload detail is retained only for failures.
- Existing plan-based Developer Logs retention remains unchanged.
- Stage 2 uses the explicitly authorized internal `observability_reads_v2` kill-switch on `/admin/feature-flags`; it is not customer-facing and is removed after legacy cleanup.
- Customer core flows are protected scope: this project may observe their finalized outcomes for logging, but cannot change their control flow, persistence, external calls, retries, or response contracts.
- `social_post_results` compaction and historical `debug_curl` cleanup are excluded because the capacity benefit does not justify publishing risk without a separate approved maintenance design.
