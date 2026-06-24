# UniPost - Error Source, Temporality, and Retry Contract PRD
**Make publish failures explicitly attributable, actionable, and SDK-friendly**
Status: Planning
Owner: Publishing / API Platform / Dashboard / SDK
Created: 2026-06-24

---

## 1. Background

UniPost already stores and returns structured publish failure fields for per-platform post results:

- `error_message`
- `error_code`
- `failure_stage`
- `platform_error_code`
- `is_retriable`
- `next_action`

The async publish queue also uses `is_retriable` to decide whether failed delivery work should create or continue a retry job. In the normal delivery worker path, retriable failures create `kind=retry` jobs and retry up to the configured max attempts.

The recent Instagram investigation exposed a gap in the public and dashboard-facing contract. A Meta Graph API response returned:

```json
{
  "error": {
    "message": "An unexpected error has occurred. Please retry your request later.",
    "type": "OAuthException",
    "is_transient": true,
    "code": 2,
    "fbtrace_id": "AJ4uhascsOC2cf1lq0bwhgJ"
  }
}
```

Before the taxonomy fix, UniPost stored this as generic `platform_error` with `is_retriable=false`, so the job went terminal instead of retrying. The taxonomy now recognizes this Meta transient response as `temporary_platform_error` with `is_retriable=true`.

That fix solves one incident class, but the broader product contract is still incomplete:

- The response does not explicitly say whether the error source is UniPost, the official platform/provider, a worker, or unknown.
- The response does not explicitly say whether the error is temporary, permanent, or unknown.
- `is_retriable=true` means retrying may help, but it does not tell the caller whether UniPost has actually scheduled an automatic retry.
- `platform_error_code` is useful but incomplete. Some providers expose multiple fields, such as Meta `code`, `error_subcode`, `type`, and `fbtrace_id`.
- API users and dashboard users still need to infer too much from `error_code` and free-form `error_message`.

This PRD defines a small additive contract that makes failure source, temporality, and retry behavior explicit without breaking existing clients.

---

## 2. Problem

When a post result fails today, customers and internal operators cannot reliably answer these questions from the structured response alone:

1. Did UniPost reject or fail the request, or did the official platform/provider reject or fail it?
2. Is this a temporary condition, a permanent condition, or unknown?
3. Will UniPost automatically retry it, or does the customer need to take action?
4. If UniPost will retry it, when is the next retry and how many attempts remain?
5. Which provider error fields are safe and stable enough for support and automation?

Because these answers are not first-class fields, clients risk parsing strings or overloading `error_code`. That makes developer integrations fragile and makes dashboard copy less precise.

---

## 3. Goals

1. Add explicit failure attribution to publish result errors.
2. Add explicit temporary/permanent/unknown classification.
3. Separate "retrying might help" from "UniPost has scheduled an automatic retry."
4. Preserve all existing fields and current client compatibility.
5. Standardize provider error detail extraction for Meta, TikTok, YouTube, LinkedIn, Threads, Facebook, and other adapters over time.
6. Make dashboard failure copy precise without exposing unsafe raw provider payloads.
7. Update public API docs and SDK types so developers can branch on stable fields instead of message text.

---

## 4. Non-goals

- Do not replace the existing async publish queue.
- Do not remove or rename `error_code`, `platform_error_code`, `is_retriable`, or `next_action`.
- Do not expose raw access tokens, signed URLs, request headers, or full provider payloads in public responses.
- Do not guarantee that every provider error can be perfectly classified.
- Do not automatically retry permanent errors.
- Do not add a feature flag unless a later implementation explicitly needs staged API exposure. This is an additive API-layer change and should default to no flag.

---

## 5. Product Principles

### 5.1 Make the source explicit

Users should not need to guess whether an error came from UniPost or the official platform. Source is different from platform name:

- `platform=instagram` says where we tried to publish.
- `error_source=platform` says the failure originated from the official provider response.

### 5.2 Make retry semantics explicit

`is_retriable` should continue to mean "the same operation might succeed later." A separate retry policy should say whether UniPost has actually scheduled another attempt.

### 5.3 Keep unknown honest

When UniPost cannot safely classify a failure, it should return `unknown` rather than pretending the failure is permanent or temporary.

### 5.4 Keep old clients working

All new response fields must be optional and additive. Existing clients that only read `error_code`, `is_retriable`, and `next_action` must keep working.

### 5.5 Prefer explicit capture over string parsing

Provider details should come from structured adapter/provider error objects for new failures. Parsing raw error strings is allowed only as a legacy or best-effort fallback.

---

## 6. Proposed API Contract

### 6.1 New fields on failed post results

Add these optional fields to `results[]` returned by public post APIs and dashboard post APIs:

```json
{
  "error_source": "platform",
  "error_temporality": "temporary",
  "provider_error": {
    "provider": "meta",
    "http_status": 500,
    "code": "2",
    "subcode": null,
    "type": "OAuthException",
    "reason": null,
    "domain": null,
    "is_transient": true
  },
  "retry_policy": {
    "is_retriable": true,
    "will_retry": true,
    "retry_state": "scheduled",
    "next_run_at": "2026-06-23T22:00:30Z",
    "attempts_made": 1,
    "max_attempts": 5,
    "attempts_remaining": 4,
    "manual_retry_allowed": false
  }
}
```

Existing fields remain present:

```json
{
  "error_message": "Instagram publish failed (500): ...",
  "error_code": "temporary_platform_error",
  "failure_stage": "publish",
  "platform_error_code": "2",
  "is_retriable": true,
  "next_action": "retry_later"
}
```

### 6.2 Field naming decision

Use the same flat snake_case field names on both public result objects and top-level API error envelopes.

Result objects:

```json
{
  "error_source": "platform",
  "error_temporality": "temporary",
  "provider_error": {},
  "retry_policy": {}
}
```

Top-level error envelopes:

```json
{
  "error": {
    "code": "PLATFORM_ERROR",
    "normalized_code": "platform_error",
    "message": "The platform returned an error.",
    "error_source": "platform",
    "error_temporality": "unknown",
    "provider_error": {},
    "retry_policy": {}
  },
  "request_id": "req_123"
}
```

Do not introduce `error.source` or `error.temporality` in v1. Keeping the same field names avoids separate SDK shapes for result failures and envelope failures.

### 6.3 `error_source` enum

v1 must use only values that UniPost can classify reliably from existing context plus explicit adapter/provider error capture:

| Value | Meaning | Examples |
| --- | --- | --- |
| `unipost` | UniPost rejected or failed the operation before or during local processing. | validation failure, quota gate, internal storage error |
| `platform` | The official provider/platform response caused the failure. | Meta OAuthException, TikTok invalid_params, YouTube quota response |
| `worker` | UniPost async worker execution failed or stalled independently of provider response. | stale running job recovery, worker timeout without provider response |
| `unknown` | UniPost cannot safely attribute the source. | legacy rows, unparsed exceptions |

Reserved future values:

- `customer_request`
- `upstream_dependency`

Do not emit reserved values until the backend has reliable source signals for them. In v1, customer-input failures should use `error_source=unipost` when UniPost rejects the request before platform dispatch, and `error_source=platform` when the official platform rejects the submitted payload.

### 6.4 `error_temporality` enum

| Value | Meaning | Retry implication |
| --- | --- | --- |
| `temporary` | Same request may succeed later without customer changes. | Usually `is_retriable=true` |
| `permanent` | Same request should not be retried until input, account, permission, quota, or media changes. | Usually `is_retriable=false` |
| `unknown` | UniPost cannot safely determine whether retrying helps. | Default `is_retriable=false` unless an explicit retry signal exists |

### 6.5 `provider_error` object

`provider_error` is a sanitized provider detail object. It should contain only safe, stable, non-secret fields.

Common fields:

| Field | Type | Notes |
| --- | --- | --- |
| `provider` | string | Provider family, such as `meta`, `tiktok`, `youtube`, `linkedin`, `x`, `bluesky`, or `pinterest`. |
| `http_status` | number | Provider HTTP status when available. |
| `code` | string | Provider primary code, such as Meta `code=2` or TikTok `invalid_params`. |
| `subcode` | string or null | Provider secondary code, such as Meta `error_subcode`. |
| `type` | string or null | Provider error class, such as `OAuthException`. |
| `reason` | string or null | Provider reason, such as Google `rateLimitExceeded`. |
| `domain` | string or null | Provider domain/category when available, such as Google API error domain. |
| `quota_limit` | string or null | Provider quota limit key when available. |
| `quota_location` | string or null | Provider quota scope/location when available. |
| `is_transient` | boolean or null | Provider transient flag when available. |

Do not include:

- access tokens
- signed URLs
- full request payloads
- raw response bodies that might include secrets or user content
- debug curls
- provider trace IDs in public responses by default

Internal/admin note:

- Persisted internal diagnostics may store provider trace IDs, such as Meta `fbtrace_id`, after sanitization.
- Public developer API responses should omit trace IDs in v1.
- Dashboard/admin/support surfaces may expose trace IDs only when the viewer is authorized for operational debugging.

### 6.6 Provider error source and sanitization strategy

v1 source of truth for new failures should be a structured provider error object captured at the adapter boundary.

Implementation requirements:

1. Define a shared internal provider error shape, for example `ProviderError`, with the allowlisted fields from section 6.5.
2. Update platform adapters as they are touched to wrap provider failures with both:
   - customer-safe message text
   - structured provider error details
3. Store the structured provider error in `post_failures.provider_error`.
4. Denormalize safe summary fields to `social_post_results` only when needed for fast reads.
5. Parse `post_failures.raw_error` only for:
   - legacy rows
   - adapters that have not yet been migrated
   - best-effort support diagnostics
6. Return `provider_error=null` when the provider payload cannot be parsed confidently or sanitized safely.

This makes the implementation cost explicit: reliable `provider_error` requires adapter-level changes, not only taxonomy string matching.

Initial v1 adapter coverage:

| Provider family | v1 requirement |
| --- | --- |
| Meta / Instagram / Facebook / Threads | Structured capture for `code`, `error_subcode`, `type`, `is_transient`, HTTP status; internal-only trace ID. |
| TikTok | Structured capture for `error.code`, `provider_error`, `fail_reason`, HTTP status. |
| YouTube / Google | Structured capture for `reason`, `domain`, `quota_limit`, `quota_location`, HTTP status. |
| Other providers | Preserve existing behavior; emit `provider_error=null` unless safe structured fields already exist. |

### 6.7 `retry_policy` object

`retry_policy` separates retry eligibility from actual queue state.

| Field | Type | Meaning |
| --- | --- | --- |
| `is_retriable` | boolean | Same meaning as the existing top-level `is_retriable`. Included here for grouping. |
| `will_retry` | boolean | UniPost has scheduled, or will schedule, another automatic attempt. |
| `retry_state` | enum | `not_retriable`, `scheduled`, `running`, `exhausted`, `blocked`, `manual_only`, or `unknown`. |
| `next_run_at` | timestamp or null | Next automatic retry time when scheduled. |
| `attempts_made` | number or null | Attempts already made for the delivery unit. |
| `max_attempts` | number or null | Maximum automatic attempts for this delivery unit. |
| `attempts_remaining` | number or null | Remaining automatic attempts. |
| `manual_retry_allowed` | boolean | Whether a user/API caller may trigger manual retry now. |
| `reason` | string or null | Short machine-readable reason when `will_retry=false`, such as `max_attempts_exhausted`, `requires_reconnect`, or `no_delivery_job`. |

Rules:

- `error_temporality=temporary` normally implies `is_retriable=true`.
- `is_retriable=true` does not always imply `will_retry=true`.
- `will_retry=true` requires an active or scheduled delivery job.
- `manual_retry_allowed=true` requires the result row to have `status="failed"` and no active delivery job for the same result.
- Legacy rows may return `retry_policy.retry_state=unknown`.

`retry_policy` is a best-effort snapshot. It reflects queue state at response generation time and may become stale before the client receives it. Clients should poll the post or queue endpoint before taking destructive or user-visible action.

When `next_run_at` is in the past and the job is still `pending`, the job is eligible to be claimed now. Return the timestamp as stored and let clients display it as "queued now" or "due now" rather than treating it as an error.

### 6.8 `post_delivery_jobs.state` to `retry_policy.retry_state` mapping

Derive retry state from the newest relevant delivery job for the result, with active jobs taking precedence over terminal historical jobs.

| Current condition | `retry_state` | `will_retry` | Notes |
| --- | --- | --- | --- |
| Result is not failed and has no active retry job | `not_retriable` | false | No retry action is relevant for a successful or still-processing result. |
| `is_retriable=false` and no active job | `not_retriable` | false | Permanent or non-retryable failure. |
| Newest active job state is `pending` | `scheduled` | true | Includes pending jobs whose `next_run_at` is already due. |
| Newest active job state is `running` or `retrying` | `running` | true | Worker has claimed the job. |
| Newest terminal retry job state is `dead` because attempts reached `max_attempts` | `exhausted` | false | Keep top-level `is_retriable=true`; attempts are exhausted, not the classification. |
| Newest terminal job state is `dead` for a non-retriable failure | `not_retriable` | false | Normal terminal permanent failure. |
| Newest terminal job state is `cancelled` | `manual_only` | false | Manual retry may be allowed only if the result is still `failed` and no active job exists. |
| Newest terminal job state is `failed` and a newer retry job exists | Use newer retry job state | depends | Failed dispatch jobs are historical after retry creation. |
| Newest terminal job state is `failed` and no newer job exists | `unknown` | false | This should be rare; avoid claiming retry is scheduled. |
| Queue admission, active-job conflict, or external blocker prevents enqueue | `blocked` | false | Include `reason`, such as `queue_job_active` or `admission_rejected`. |

Manual retry eligibility must mirror the existing retry endpoint:

- `manual_retry_allowed=true` only when `social_post_results.status="failed"` and there is no active `pending`, `running`, or `retrying` job for the same result.
- `manual_retry_allowed=false` for all non-failed result statuses because `POST /v1/posts/{id}/results/{resultID}/retry` returns `RESULT_NOT_RETRYABLE`.
- Admission control may still reject a manual retry after the snapshot due to rate limits or queue depth. In that case the endpoint response is authoritative.

---

## 7. Classification Requirements

### 7.1 Mapping rules

The mapping must cover every existing `api/internal/postfailures/taxonomy.go` error code. Source may require context beyond `error_code`; for example, `quota_exceeded` can mean a UniPost plan quota or an official provider quota.

| Existing `error_code` | Primary signal | `error_source` | `error_temporality` | `is_retriable` | `retry_policy.will_retry` |
| --- | --- | --- | --- | --- | --- |
| `temporary_platform_error` | Meta `is_transient=true`, Meta OAuthException `code=2` with retry wording, provider temporary unavailable, container timeout | `platform` | `temporary` | true | true when delivery job attempts remain |
| `rate_limit` | Provider 429, `rate limit`, `too many requests` | `platform` | `temporary` | true | true when delivery job attempts remain |
| `worker_stalled` | UniPost delivery worker stale running job recovery | `worker` | `temporary` | true | true when delivery job attempts remain |
| `validation_error` | UniPost request validation or local publish-prep validation failed before provider dispatch | `unipost` | `permanent` | false | false |
| `platform_request_invalid` | Official provider rejected submitted platform options or metadata | `platform` | `permanent` | false | false |
| `media_error` | Official provider rejected media, or UniPost determined media cannot satisfy platform requirements | `platform` when provider returned rejection; otherwise `unipost` | `permanent` | false | false |
| `quota_exceeded` | UniPost plan/quota gate | `unipost` | `permanent` | false | false |
| `quota_exceeded` | Provider quota response, such as YouTube upload project quota | `platform` | `temporary` when reset is time-based; otherwise `permanent` | false in v1 unless a safe reset retry policy exists | false in v1 |
| `account_reconnect_required` | Provider account/token state requires reconnect | `platform` | `permanent` | false | false |
| `auth_token_invalid` | Expired or invalid provider token | `platform` | `permanent` | false | false |
| `missing_permission` | Provider permission or scope failure | `platform` | `permanent` | false | false |
| `target_not_found` | Provider target resource no longer exists or is not visible | `platform` | `permanent` | false | false |
| `platform_error` | Provider failure UniPost cannot classify more specifically | `platform` | `unknown` | false by default | false by default |
| `unknown_error` | Failure UniPost cannot classify or attribute safely | `unknown` | `unknown` | false by default | false by default |

### 7.2 Default behavior

Unknown errors should not automatically retry unless there is an explicit temporary signal. This prevents retry storms and avoids hiding permanent failures behind repeated attempts.

### 7.3 Provider code extraction

Expand provider code extraction so `platform_error_code` is populated more consistently:

- Meta:
  - `code`
  - `error_subcode`
  - `type`
  - `is_transient`
  - `fbtrace_id`
- TikTok:
  - `error.code`
  - `provider_error`
  - `fail_reason`
- YouTube/Google:
  - `reason`
  - `domain`
  - `quota_limit`
  - `quota_location`
- LinkedIn:
  - provider service error code
  - status
- Pinterest:
  - code
  - message type when available

`platform_error_code` should remain a single compact string for backward compatibility. Richer details belong in `provider_error`.

---

## 8. Dashboard Requirements

### 8.1 Users drawer and post details

Failed result rows should display source and retry behavior clearly:

- Temporary platform error with retry scheduled:
  - "Instagram had a temporary official-platform error. UniPost will retry automatically."
- Temporary platform error but attempts exhausted:
  - "Instagram had a temporary official-platform error, but automatic retries are exhausted."
- Permanent request issue:
  - "This request needs a change before retrying."
- Account issue:
  - "Reconnect this account before retrying."
- Unknown:
  - "UniPost could not classify this failure. Contact support with the request ID."

The UI should not show raw JSON as the primary message. Raw details may remain available in debug/admin surfaces.

### 8.2 Queue UI

Queue views should expose:

- retry state
- next retry time
- attempts remaining
- manual retry availability

Manual retry buttons should respect `retry_policy.manual_retry_allowed`.

---

## 9. Data and Backend Requirements

### 9.1 Persistence

Persist normalized failure contract fields close to existing publish failure data:

- `post_failures.error_source`
- `post_failures.error_temporality`
- `post_failures.provider_error` as sanitized JSON
- optional denormalized summary fields on `social_post_results` for fast API reads:
  - `error_source`
  - `error_temporality`
  - `provider_error`

The implementation may derive `retry_policy` from `post_delivery_jobs` rather than storing it directly on `social_post_results`.

### 9.2 API response construction

Post result responses should combine:

- persisted classification fields from `social_post_results`
- most recent failure metadata from `post_failures` when needed
- current job state from `post_delivery_jobs` for `retry_policy`

### 9.3 Legacy rows

For historical failed rows that lack the new fields:

- derive best-effort values from existing `error_code` and `is_retriable`
- use `unknown` when derivation is unsafe
- do not backfill automatically in v1 unless needed for a dashboard migration

Legacy derivation must be implemented as a pure function, not as scattered serializer conditionals. Given the same persisted row and relevant job snapshot, the function must always return the same derived contract. Add snapshot tests that lock the derived output for representative legacy rows.

---

## 10. Public Documentation Requirements

Update API docs for:

- `GET /v1/posts/:post_id`
- `GET /v1/posts`
- `POST /v1/posts`
- `POST /v1/posts/:post_id/results/:result_id/retry`
- API Errors reference
- Queue / post delivery jobs reference where retry state is documented

Docs should state:

- Use `error_source` to distinguish UniPost from official platform failures.
- Use `error_temporality` to distinguish temporary, permanent, and unknown failures.
- Use `retry_policy.will_retry` to know whether UniPost has scheduled automatic retry.
- Treat `retry_policy` as a best-effort queue snapshot, not a strong consistency guarantee.
- Treat past `next_run_at` values on pending jobs as "due now".
- Do not parse `error_message` for branching.

---

## 11. SDK Release Decision

### 11.1 Is a new SDK version required?

Backend compatibility does not require a new SDK version because this PRD adds optional response fields. Existing SDK versions should continue to work.

However, a new SDK version should be released after the backend contract is implemented because the public SDKs expose typed post/result and error shapes:

- JavaScript/TypeScript has typed `PlatformResult`, `Post`, and SDK error classes.
- Python has dataclass response types for `PlatformResult`, `Post`, and `DeliveryJob`.
- Go has typed structs for `PlatformResult`, `Post`, `DeliveryJob`, and `APIError`.
- Java mostly returns `JsonNode` for posts and delivery jobs, so it is technically less affected, but releasing it with the same version keeps language packages aligned.

Recommendation:

- Do not block backend deploy on SDK release.
- Do release a new SDK patch/minor version once API docs and backend are live.
- Because this is an additive response contract, use a patch release if only types/docs/examples change.
- Use a minor release if SDKs add exported enum types or convenience helpers around retry policy.
- With the current SDK version at `0.4.0`, the preferred release target is:
  - `0.4.1` for type-only additive support
  - `0.5.0` if SDKs add new helper APIs, enum exports, or retry-policy convenience methods

### 11.2 SDK scope

SDK updates should include:

- `PlatformResult.error_source`
- `PlatformResult.error_temporality`
- `PlatformResult.provider_error`
- `PlatformResult.retry_policy`
- equivalent fields on delivery job objects if the backend exposes them there
- API error envelope support for:
  - `error.error_source`
  - `error.error_temporality`
  - `error.is_retriable`
  - `error.next_action`
  - `error.provider_error`
  - `error.retry_policy`

SDK examples should show:

- checking `result.retry_policy?.will_retry`
- checking `result.error_source === "platform"`
- avoiding string parsing of `error_message`

---

## 12. Acceptance Criteria

1. A Meta transient Instagram failure returns:
   - `error_source=platform`
   - `error_temporality=temporary`
   - `error_code=temporary_platform_error`
   - `is_retriable=true`
   - `provider_error.http_status=500`
   - `provider_error.code=2`
   - `provider_error.is_transient=true`
   - `retry_policy.will_retry=true` while attempts remain
2. A permanent media rejection returns:
   - `error_temporality=permanent`
   - `is_retriable=false`
   - `retry_policy.will_retry=false`
3. A failure with `is_retriable=true` but exhausted attempts returns:
   - `error_temporality=temporary`
   - `is_retriable=true`
   - `retry_policy.will_retry=false`
   - `retry_policy.retry_state=exhausted`
4. A manually retryable failed row returns `manual_retry_allowed=true` only when the result has `status="failed"` and no active delivery job exists for that result.
5. Dashboard failed-result surfaces no longer require reading raw provider JSON to know source and retry behavior.
6. Public docs describe every new field and explicitly tell developers not to parse `error_message`.
7. SDK source types are updated and released after backend rollout.

---

## 13. Test Plan

Backend tests:

- Unit-test taxonomy mappings for Meta transient, Meta reconnect, rate limit, media rejection, validation, quota, and unknown provider errors.
- Unit-test provider error extraction for Meta `code`, Meta `error_subcode`, TikTok `invalid_params`, and Google quota reasons.
- Queue tests should prove retriable delivery failures create retry jobs while non-retriable failures go terminal.
- API response tests should prove `retry_policy.will_retry` reflects actual job state.
- API response tests should prove exhausted retry attempts keep `is_retriable=true` while setting `retry_policy.will_retry=false` and `retry_policy.retry_state=exhausted`.
- Manual retry policy tests should mirror `POST /v1/posts/{id}/results/{resultID}/retry`: failed row plus no active job is allowed; non-failed rows or active jobs are not allowed.
- Legacy derivation tests should snapshot output from the pure derivation function for old rows without `error_source`, `error_temporality`, or `provider_error`.
- Provider error tests should verify low-confidence raw-error parsing returns `provider_error=null`.

Dashboard tests:

- Render failed result with automatic retry scheduled.
- Render exhausted temporary failure.
- Render permanent account reconnect failure.
- Render unknown failure.

SDK validation:

- Type-level tests compile for JS/TS and Go.
- Python dataclass parsing preserves new fields.
- Java examples can read new fields from `JsonNode`.
- Live SDK validation should tolerate both older responses without the fields and newer responses with them.

---

## 14. Observability and Quality Metrics

Track classification coverage and retry safety after rollout:

- percentage of failed results with `error_source=unknown`
- percentage of failed results with `error_temporality=unknown`
- percentage of platform failures with non-null `provider_error`
- retry recovery rate by `error_code`, `error_source`, and platform
- exhausted retry rate by `error_code`, `error_source`, and platform
- count of automatic retries created from `error_temporality=unknown`

Alerting requirements:

- Alert if automatic retries are created for `error_temporality=unknown`; the default should be no automatic retry.
- Alert if retry job volume for one platform spikes above a configured threshold, to catch retry storms.
- Alert if `error_source=unknown` exceeds a target threshold for new failures after the initial rollout window.

These metrics are the feedback loop for whether the new contract is actually reducing ambiguity.

---

## 15. Rollout Plan

1. Add backend taxonomy model fields and tests.
2. Add DB migration for persisted classification fields.
3. Populate new fields for new failures.
4. Update public API response serializers.
5. Update dashboard failure displays.
6. Update public docs.
7. Deploy to development and verify with seeded or forced transient/permanent failures.
8. Promote through staging and production using the standard release flow.
9. Update SDK types and examples after production API contract is live.
10. Release SDK `0.4.1` for type-only support, or `0.5.0` if helper APIs are included.

---

## 16. Resolved Decisions

1. Public API responses omit provider trace IDs in v1. Store trace IDs only in sanitized internal/admin diagnostics.
2. v1 `error_source` values are `unipost`, `platform`, `worker`, and `unknown`. `customer_request` and `upstream_dependency` are reserved for future versions.
3. Manual retry eligibility mirrors the existing retry endpoint: result status must be `failed`, and no active delivery job may exist for that result.
4. Legacy rows are derived on read in v1. No automatic backfill is required.
5. Top-level error envelopes and result objects both use flat snake_case fields: `error_source`, `error_temporality`, `provider_error`, and `retry_policy`.
