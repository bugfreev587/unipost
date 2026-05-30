# UniPost - Post Failure Reliability PRD
**Turn failed publish logs into concrete reliability, prevention, and recovery improvements**
Status: Planning
Owner: Publishing / Platform Adapters / Logs
Created: 2026-05-30

---

## 1. Background

### 1.1 Investigation scope

On 2026-05-30, UniPost reviewed production publishing failures from the previous week:

- Time window: 2026-05-23 00:00 PDT through 2026-05-30 14:50 PDT.
- Primary data sources:
  - `post_failures`
  - `post_delivery_jobs`
  - `social_post_results`
  - `integration_logs`
  - Railway application error logs
- Environment: production.

The goal of the investigation was to classify user post publish failures, understand whether failures were caused by user input, platform limits, or UniPost behavior, and identify product and engineering changes that can reduce recurrence.

### 1.2 Summary of the last-week logs

During the window, production recorded:

- `1,030` `social_posts`
- `280` platform result rows in `social_post_results`
- `206` platform result rows that ended as `published`
- `74` platform result rows that ended as `failed`
- `200` `post_failures` events

The `post_failures` count is higher than the final failed result count because it includes retry attempts. Those `200` failure events corresponded to `75` platform result rows: `74` remained failed and `1` recovered through retry.

At the end of the investigation, there were no active publish delivery jobs in `pending`, `running`, or `retrying` state.

### 1.3 Failure categories

| Category | Scope | Current symptom | Current result |
| --- | ---: | --- | --- |
| Instagram container processing | 151 failure events, 26 result rows, 25 still failed | `container processing failed` or `container processing timed out` | Mostly single MP4 posts; errors are too generic to tell whether the root cause is source media, source URL fetch, Meta transcoding, or a platform incident |
| YouTube upload quota | 30 result rows, 27 posts | `youtube upload init failed (429)` with Google quota exceeded for `Video Uploads per day` | Deterministic project-level quota exhaustion; UniPost kept attempting uploads after quota was already exhausted |
| TikTok media rejection | 18 result rows | `file_format_check_failed` and photo init `invalid_params` | Likely media format or Content Posting API parameter issues, but all are currently classified as generic `platform_error` |
| Threads user lookup | 1 result row | `failed to get Threads user ID` | The adapter drops response status/body, so the error cannot distinguish token expiry, missing permission, or platform response shape |

TikTok caveat: as of this investigation, the TikTok Content Posting API approval was still in progress. Some `file_format_check_failed` or `invalid_params` behavior may be sandbox/app-review-specific. The product should still improve diagnostics and validation, but should avoid overfitting irreversible product rules to sandbox artifacts.

Failures were highly concentrated:

- One anonymized workspace (`57066f9bd9`) accounted for the Instagram, TikTok, Threads, and most YouTube failures.
- One additional anonymized workspace (`eb0a26c5c0`) saw YouTube quota failures only.

### 1.4 Validation noise excluded from the main failure count

`integration_logs` also contained `610` `post.validate.failed` events. These were excluded from the main publish failure count because samples looked like automated regression or smoke-test traffic, concentrated in anonymized workspace `bb8ba6f10c`.

The observed validation error codes were:

- `account_not_in_workspace`
- `thread_positions_not_contiguous`
- `thread_mixed_with_single`
- `media_id_not_in_workspace`
- `threads_unsupported`
- `exceeds_max_length`

These are still useful for product quality, but they are not the same class of failure as a user post that entered the publish delivery pipeline and then failed at adapter/platform dispatch time.

### 1.5 Product problem

UniPost now has a solid async publish queue, retry model, and structured failure tables, but the current failure experience is still reactive:

- Some deterministic failures are discovered only after expensive adapter work starts.
- Some platform failures are grouped into broad error codes that do not tell users or operators what to do.
- Retry eligibility is already error-type-aware through `postfailures.Classify`, but the backoff schedule is uniform and some deterministic provider limits are not special-cased.
- Platform quota exhaustion is not converted into a clean local circuit breaker.
- Worker and persisted log source attribution can be misleading.
- Admins do not yet have a first-class failure trend view across platform, workspace, account, and error code.

The result is avoidable user-facing failed posts, noisy retries, and slow root-cause analysis.

### 1.6 Why now and how much

This dataset is small and concentrated: most final failures came from one power workspace over one week. That makes the data strong enough to justify cheap, high-leverage reliability work, but not enough to justify a broad analytics product or paging system immediately.

The first implementation should prioritize:

1. taxonomy and diagnostics that make future incidents cheaper to understand
2. a YouTube quota breaker because the failure is deterministic and repeated
3. Instagram timeout/container measurement because the current 60s poll window may be self-inflicting failures
4. Threads error wrapping because it is small and clearly incomplete

Full alerting, a polished admin trend dashboard, and deep TikTok codec probing should wait until there is a broader failure base or repeated demand from more workspaces.

### 1.7 Related PRDs and ownership

This PRD complements, rather than replaces, existing publishing and notification PRDs:

- `docs/prd-async-publish-queue.md` owns the async delivery job model, queue states, retries, and manual retry surface. This PRD only refines failure classification, diagnostics, and platform-specific retry decisions.
- `docs/prd-rate-limit-and-queue-admission.md` owns API admission control and queue-depth protection. This PRD only covers provider-specific limits discovered during platform dispatch, such as YouTube Data API upload quota.
- `docs/prd-email-failure-notifications.md` owns failure email templates and Logs AI Debug. This PRD owns the normalized cause/recovery-hint data that those surfaces can display.

---

## 2. Goals

1. Reduce preventable publish failures before platform dispatch.
2. Convert deterministic platform limits into clear local guardrails.
3. Improve failure taxonomy so users and operators see actionable error categories.
4. Preserve retries for genuinely transient cases while avoiding churn on terminal failures.
5. Improve stored diagnostics without exposing tokens, debug curls, or sensitive payloads to the wrong surface.
6. Define an operator reporting path that can later support spike detection by platform, workspace, account, and error code.
7. Give dashboard and API users clearer next steps after a post fails.

---

## 3. Non-goals

- No replacement of the async publish queue architecture.
- No replacement of `social_posts`, `social_post_results`, or `post_delivery_jobs`.
- No automatic OAuth reconnect on behalf of users.
- No guarantee that UniPost can make every platform accept every media file.
- No platform-specific quota marketplace or customer-owned platform app support in v1.
- No AI-generated automatic retry/fix action in v1.
- No bypassing the existing Unleash feature flag and rollout process.

---

## 4. Product Principles

### 4.1 Prevent before retrying

If UniPost can determine a failure before dispatching to a platform, it should fail fast with an actionable error instead of creating queue churn.

### 4.2 Retry only when the next attempt can plausibly succeed

Retries should be reserved for:

- temporary upstream failures
- timeouts
- platform processing delays
- network errors

Retries should not repeatedly execute for:

- project-level daily quota exhaustion
- media files known to violate platform constraints
- invalid request parameters
- disconnected or expired accounts

### 4.3 Preserve operator evidence

Stored logs should capture enough sanitized evidence to explain what happened later:

- platform
- account
- workspace
- post/result/job IDs
- adapter stage
- normalized error code
- provider HTTP status
- provider error code/reason/subcode
- retry decision
- current final status

### 4.4 Keep user-facing language actionable

Users should see what to do next, not raw provider JSON. Raw details belong in Logs and admin/debug views after redaction.

---

## 5. Requirements

### 5.1 Failure taxonomy improvements

Update `api/internal/postfailures/taxonomy.go` and publish failure mapping so common cases become actionable categories.

Requirements:

- Preserve existing top-level codes where they already exist. Do not add a second top-level quota code that overlaps with `quota_exceeded`.
- Map TikTok `file_format_check_failed` to `media_error`.
- Map TikTok `invalid_params` to `validation_error` or `platform_request_invalid`, depending on the final implementation convention.
- Map YouTube `RESOURCE_EXHAUSTED`, `rateLimitExceeded`, and upload quota 429s to `quota_exceeded`, with provider metadata that identifies the scope as Google project upload quota.
- Map Threads 401/invalid token responses to `account_reconnect_required`.
- Map Threads permission/scope errors to `missing_permission`.
- Keep `temporary_platform_error` for timeouts and transient 5xx cases.
- Store provider-specific codes where available:
  - Google `reason`
  - Google `quota_limit`
  - TikTok `error.code`
  - TikTok `fail_reason`
  - Meta `code` and `error_subcode`

Recommended mapping:

| Current signal | Current code | Target code | Additional metadata |
| --- | --- | --- | --- |
| YouTube `RESOURCE_EXHAUSTED` / `rateLimitExceeded` / upload 429 | `quota_exceeded` | `quota_exceeded` | `quota_scope=platform_project`, `provider=youtube`, `provider_reason=rateLimitExceeded`, `quota_limit=defaultVideoInsertPerDayPerProject` |
| TikTok `file_format_check_failed` | `platform_error` | `media_error` | `provider=tiktok`, `provider_reason=file_format_check_failed` |
| TikTok photo init `invalid_params` | `platform_error` | `validation_error` or `platform_request_invalid` | `provider=tiktok`, `provider_reason=invalid_params`, `media_kind=photo` |
| Threads invalid/expired token | `platform_error` | `account_reconnect_required` | `provider=threads`, `provider_http_status=401` when available |
| Threads missing permission/scope | `platform_error` | `missing_permission` | `provider=threads`, `provider_http_status=403` when available |
| Instagram container timeout | `temporary_platform_error` | `temporary_platform_error` | `provider=instagram`, `container_status=timeout`, poll metadata |
| Instagram container `ERROR` | `temporary_platform_error` today | `media_error` or `temporary_platform_error` based on Phase 1 measurement | `provider=instagram`, `container_status=ERROR`, poll metadata |

Acceptance criteria:

- A failed TikTok post with `file_format_check_failed` no longer appears as generic `platform_error`.
- A YouTube upload quota failure is distinguishable from customer account quota and UniPost billing quota.
- Threads user lookup failures contain enough structured data to determine auth vs permission vs provider issue.

### 5.2 YouTube upload quota circuit breaker

YouTube failures were deterministic project-level quota exhaustion. UniPost should stop trying uploads once the project quota is known to be exhausted.

Requirements:

- Detect YouTube upload 429 responses that indicate project-level `Video Uploads per day` exhaustion.
- Store a short-lived quota circuit breaker state for YouTube uploads.
- While the breaker is open:
  - do not download the source video
  - do not initialize a resumable upload
  - fail fast or defer according to the selected rollout mode
  - present a clear message that YouTube upload capacity is temporarily exhausted
- Use midnight Pacific Time as the reset heuristic for Google `Video Uploads per day` quota, and store the computed reset timestamp with the breaker.
- Add admin visibility for current breaker state.
- Do not conflate Google project quota with UniPost plan quota.

Recommended behavior for v1:

- Existing immediate publish attempts should fail fast with `quota_exceeded` plus `quota_scope=platform_project` while the breaker is open.
- Scheduled posts should be marked failed only when their execution window arrives and the breaker is still open.
- Manual retry should be disabled or warned while the breaker is open.

Acceptance criteria:

- After the first qualifying YouTube quota 429, subsequent YouTube publish attempts avoid video download and upload init until breaker reset.
- Users see a non-generic YouTube quota message.
- Operators can see when the breaker opened, why it opened, and when it is expected to clear.

### 5.3 Instagram container diagnostics and retry policy

Instagram had the largest failure volume. The current adapter reports only `container processing failed` or `container processing timed out`, which is not enough to separate media problems from platform processing issues.

Requirements:

- Treat measurement of the current 60s poll window as Phase 1 work, not a later nice-to-have:
  - capture elapsed container processing time for successful and failed Instagram publishes
  - separate timeout failures from explicit platform `ERROR`
  - determine whether successful MP4 posts commonly exceed 60s before changing user-facing media guidance
- Persist the Instagram creation container ID in failure diagnostics where safe.
- When polling container status, capture:
  - last HTTP status
  - last response body after redaction
  - last observed `status_code`
  - poll count
  - elapsed time
- Distinguish:
  - `ERROR` from the platform
  - timeout waiting for `FINISHED`
  - HTTP failures while polling
  - decode/empty response issues
- Keep timeout retriable.
- Treat explicit container `ERROR` as terminal unless evidence shows a fresh container usually succeeds for the same media.
- Add a media-oriented user message for explicit container `ERROR`.

Acceptance criteria:

- A future Instagram failed result can answer whether the container reached `ERROR`, never finished, or could not be polled.
- The team can answer, with production data, whether the current 60s polling window is too short.
- Retry policy differs between timeout and explicit terminal processing error.
- Dashboard and Logs present a clearer user action, such as replacing or re-encoding the video.

### 5.4 TikTok media preflight and photo publish hardening

TikTok failures were mostly media rejection. UniPost should catch obvious media issues before publish and produce better error categories when TikTok rejects content.

Requirements:

Phase 1 requirements:

- Add shallow TikTok media preflight for video posts using data UniPost already has or can cheaply infer:
  - file extension and content type
  - size
  - duration when metadata is available
  - exactly one video for video publish
- Add shallow TikTok photo preflight:
  - image count
  - image extension/content type
  - R2 media proxy availability
  - cover index bounds
- Normalize `file_format_check_failed` into a user-actionable media error.
- For photo `invalid_params`, reuse the same privacy fallback and error wrapping discipline already present in the video init path where applicable.
- Add sandbox/app-review context to operator diagnostics when a TikTok rejection may be affected by unapproved Content Posting API state.
- Do not log full source media URLs in user-visible messages.

Deferred requirements:

- Add deeper codec/container probing only after repeated TikTok media failures across more workspaces or after Content Posting API approval changes the baseline.
- Use uploaded media metadata first; avoid downloading arbitrary remote media solely for deep probing in v1.

Acceptance criteria:

- A media file that obviously violates TikTok constraints fails validation before queue dispatch.
- TikTok `file_format_check_failed` is visible as a media compatibility issue.
- Photo init failures tell operators which body field or fallback path was used, without leaking credentials.

### 5.5 Threads user lookup diagnostics

The single Threads failure was not diagnosable from the stored message.

Requirements:

- In `ThreadsAdapter.getUserID`, check non-2xx responses explicitly.
- Capture sanitized response body and HTTP status.
- Return typed or parseable errors for:
  - invalid/expired token
  - missing scope
  - permission denied
  - empty profile response
- Classify these errors through the same post failure taxonomy.

Acceptance criteria:

- A future `failed to get Threads user ID` case includes enough evidence to decide whether the user should reconnect.
- Generic `platform_error` is not used for known auth or permission failures.

### 5.6 Worker source attribution in integration logs

Worker-generated publishing logs are currently attributed as `dashboard` in production for `post.publish.platform_failed`. A source grouping query for the investigation window showed all persisted platform failures as `source='dashboard'`, even though they are emitted from async job processing.

The nuance: `integrationlogs.Normalize` defaults an empty source to `worker`, but `SocialPostHandler.logPublishingEvent` fills empty source first. If the context has no API key, it sets `SourceDashboard`. Worker contexts have no API key, so they are misclassified before normalization runs.

Requirements:

- Explicitly set `SourceWorker` for events emitted from background delivery workers.
- Preserve `SourceAPI` or `SourceDashboard` for user-initiated request-path validation and enqueue events.
- Consider changing `logPublishingEvent` inference so request-path callers opt into dashboard/API source, instead of treating every no-API-key context as dashboard.
- Keep actor fields empty for worker-only execution unless a safe original actor is explicitly stored and intended for display.

Acceptance criteria:

- `post.publish.platform_failed` records emitted from async job processing are stored with `source='worker'`.
- Dashboards and future reports do not misclassify worker failures as dashboard-originated failures.

### 5.7 Failure monitoring and admin reporting

The investigation required ad hoc SQL. UniPost should have an operator surface for this.

Phase 1 requirements:

- Define one supported internal query or super-admin endpoint that returns a failure summary over a time window:
  - platform
  - error code
  - failure stage
  - final current result status
  - event count
  - distinct posts
  - distinct accounts
  - distinct workspaces
  - first seen
  - last seen
- Separate:
  - validation failures
  - publish dispatch failures
  - retried-and-recovered failures
  - final terminal failures
- Include concentration fields so operators can see when one workspace or account accounts for most failures.

Deferred requirements:

- Add a polished admin UI only after the internal summary proves useful.
- Add alert thresholds only after there is a broader baseline. Candidate future alerts:
  - platform quota breaker opened
  - sudden spike in `media_error`
  - sudden spike in `temporary_platform_error`
  - retry exhaustion rate above threshold

Acceptance criteria:

- An operator can reproduce the summary in this PRD from an admin surface or one supported internal endpoint.
- Recovered retry events are not counted as final failed posts.
- Validation smoke-test noise can be filtered away from real publish failures.
- Phase 1 does not page humans or create noisy alerts from a one-week, low-n dataset.

### 5.8 User-facing recovery guidance

Failed publish rows should explain what the user can do next.

Requirements:

- Add normalized recovery hints for major error categories:
  - `quota_exceeded` with `quota_scope=platform_project`: wait until platform capacity resets; contact support if urgent
  - `media_error`: replace or re-encode media
  - `account_reconnect_required`: reconnect the account
  - `missing_permission`: reconnect with required scopes or contact workspace owner
  - `temporary_platform_error`: wait for automatic retry or retry manually after retries exhaust
- Show retry availability based on job state and retry policy.
- Avoid exposing provider JSON in the primary user message.
- Preserve raw details in Logs/admin debug views after existing redaction.

Acceptance criteria:

- A failed post detail can show both the short cause and the recommended next action.
- Manual retry controls are disabled or warned when retry cannot plausibly succeed.

---

## 6. Data Model and API Considerations

### 6.1 Reuse existing tables where possible

The first implementation should reuse:

- `post_failures`
- `post_delivery_jobs`
- `social_post_results`
- `integration_logs`

New columns should be added only when the existing JSON and text fields cannot support safe diagnostics.

### 6.2 Candidate additions

Potential additions:

- `post_failures.provider_http_status`
- `post_failures.provider_reason`
- `post_failures.quota_scope`
- `post_failures.recovery_hint`
- `post_delivery_jobs.retry_policy`
- `integration_logs.metadata.failure_group`
- a small platform circuit breaker table, or Redis keys backed by an admin-readable endpoint

Exact schema choices should be finalized during implementation planning.

### 6.3 Feature flags

Implementation decision on 2026-05-30: this reliability patch will not add new feature flags. The rollout will rely on targeted test coverage, development-environment validation, staging validation, and the existing release promotion flow.

Rationale:

- Taxonomy, diagnostics, and worker source attribution are low-risk correctness fixes.
- YouTube quota detection is deterministic and reduces repeated platform calls after the first observed quota exhaustion.
- Instagram and TikTok behavior changes must be covered by adapter-level tests because they affect retry and fallback behavior directly.

The existing Unleash-backed feature flag system documented in `docs/feature-flags-unleash.md` remains the default mechanism for future higher-risk rollout work, especially admin UI, dashboard recovery controls, or broader behavior changes.

---

## 7. Rollout Plan

### Phase 1 - Taxonomy and diagnostics

- Improve classification for YouTube, TikTok, Instagram, and Threads.
- Improve adapter diagnostics while preserving redaction.
- Fix worker source attribution.
- Measure whether Instagram's current 60s container polling window is too short for successful MP4 posts.
- Add tests for failure classification and adapter error wrapping.

### Phase 2 - Preventable failure guardrails

- Add YouTube quota circuit breaker.
- Add shallow TikTok media preflight.
- Adjust Instagram retry policy for timeout vs explicit `ERROR`, using Phase 1 measurement.
- Add API/dashboard recovery hints.

### Phase 3 - Operator visibility

- Add internal or super-admin failure summary.
- Add concentration and recovered-vs-terminal views.
- Defer alerting until there is enough baseline volume to avoid noisy thresholds.

### Phase 4 - Product polish

- Refine failed post detail copy.
- Link failure emails and logs to the right recovery action where applicable.
- Add support docs for common YouTube, Instagram, and TikTok failure causes.
- Revisit deeper TikTok codec/container probing after broader data or Content Posting API approval.

---

## 8. Validation Plan

Backend validation:

- Unit tests for `postfailures.Classify`.
- Unit tests for YouTube quota error parsing.
- Unit tests for TikTok media error parsing and preflight.
- Unit tests for Threads non-2xx user lookup errors.
- Integration-style tests for job retry policy where feasible.

Dashboard validation:

- Failed post detail shows normalized cause and recovery hint.
- Queue retry controls reflect retry eligibility.
- The internal or admin failure summary filters validation noise separately from publish failures.

Operational validation:

- Seed or replay representative failure records in a non-production database.
- Confirm the summary matches expected counts by platform and error category.
- Confirm no raw tokens or unredacted auth headers appear in user-facing surfaces.

---

## 9. Success Metrics

Baseline week: 2026-05-23 00:00 PDT through 2026-05-30 14:50 PDT.

Baseline counts:

- final failed platform result rows: 74
- YouTube quota failed result rows: 30
- Instagram container failed result rows: 25
- TikTok media/platform failed result rows: 18
- Threads generic user lookup failed result rows: 1
- known failures still classified as generic `platform_error`: 19
- recovered result rows after retry: 1

Targets for the first comparable 7-day production window after rollout:

- YouTube quota breaker: no more than 1 repeated YouTube upload-init attempt after the breaker opens.
- Known generic `platform_error`: 0 result rows for the known TikTok `file_format_check_failed` and Threads user lookup cases.
- Instagram diagnostics: 100% of Instagram container failures include container status, poll count, and elapsed time.
- Recovery hints: 100% of final failed result rows in the known categories have a recovery hint.
- Operator diagnosis: the weekly summary can be produced without ad hoc SQL.

Longer-term indicators, once volume is higher:

- Retry recovery rate by platform and error code.
- Final failed result rows by category, using absolute counts before percentages while `n` remains small.
- Number of admin investigations that can be completed without adding new one-off queries.

---

## 10. Open Questions

1. Should YouTube quota exhaustion fail fast or automatically defer scheduled jobs until the next quota window?
2. Which TikTok constraints can be validated from existing `media` metadata and URL/content-type inference without downloading arbitrary remote media?
3. Should Instagram explicit container `ERROR` ever retry automatically, or should retries be limited to timeout/poll failures only?
4. Should admin failure summaries expose raw workspace/account IDs to super admins, or anonymized references by default?
5. Should the deferred admin UI or future recovery controls be feature-flagged separately when they are implemented?
