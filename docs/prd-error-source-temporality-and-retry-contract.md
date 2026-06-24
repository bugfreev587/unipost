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

- The response does not explicitly say whether the error source is UniPost, the official platform/provider, a worker, or the customer request.
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
    "code": "2",
    "subcode": null,
    "type": "OAuthException",
    "is_transient": true,
    "trace_id": "AJ4uhascsOC2cf1lq0bwhgJ"
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

### 6.2 `error_source` enum

| Value | Meaning | Examples |
| --- | --- | --- |
| `unipost` | UniPost rejected or failed the operation before or during local processing. | validation failure, quota gate, internal storage error |
| `platform` | The official provider/platform response caused the failure. | Meta OAuthException, TikTok invalid_params, YouTube quota response |
| `worker` | UniPost async worker execution failed or stalled independently of provider response. | stale running job recovery, worker timeout without provider response |
| `customer_request` | The submitted request or selected resource caused the failure. | invalid media ID, unsupported option, account not in workspace |
| `upstream_dependency` | A non-social-platform dependency failed. | storage provider, media fetch dependency, webhook target dependency |
| `unknown` | UniPost cannot safely attribute the source. | legacy rows, unparsed exceptions |

### 6.3 `error_temporality` enum

| Value | Meaning | Retry implication |
| --- | --- | --- |
| `temporary` | Same request may succeed later without customer changes. | Usually `is_retriable=true` |
| `permanent` | Same request should not be retried until input, account, permission, quota, or media changes. | Usually `is_retriable=false` |
| `unknown` | UniPost cannot safely determine whether retrying helps. | Default `is_retriable=false` unless an explicit retry signal exists |

### 6.4 `provider_error` object

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
| `is_transient` | boolean or null | Provider transient flag when available. |
| `trace_id` | string or null | Provider trace ID when safe, such as Meta `fbtrace_id`. |

Do not include:

- access tokens
- signed URLs
- full request payloads
- raw response bodies that might include secrets or user content
- debug curls

### 6.5 `retry_policy` object

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
- Legacy rows may return `retry_policy.retry_state=unknown`.

---

## 7. Classification Requirements

### 7.1 Mapping rules

| Signal | `error_source` | `error_temporality` | `error_code` | `is_retriable` | `retry_policy.will_retry` |
| --- | --- | --- | --- | --- | --- |
| Meta `is_transient=true` | `platform` | `temporary` | `temporary_platform_error` | true | true when delivery job attempts remain |
| Meta OAuthException `code=2` with retry wording | `platform` | `temporary` | `temporary_platform_error` | true | true when delivery job attempts remain |
| Provider 429 rate limit | `platform` | `temporary` | `rate_limit` | true | true when delivery job attempts remain |
| Worker stale running job recovery | `worker` | `temporary` | `worker_stalled` | true | true when delivery job attempts remain |
| Unsupported media format | `platform` or `customer_request` | `permanent` | `media_error` | false | false |
| Invalid platform options | `platform` or `customer_request` | `permanent` | `platform_request_invalid` | false | false |
| Expired token | `platform` | `permanent` | `auth_token_invalid` | false | false |
| Account reconnect required | `platform` | `permanent` | `account_reconnect_required` | false | false |
| Missing permission or scope | `platform` | `permanent` | `missing_permission` | false | false |
| UniPost plan or quota gate | `unipost` | `permanent` | `quota_exceeded` | false | false |
| Unknown provider failure | `platform` | `unknown` | `platform_error` | false by default | false by default |
| Unknown internal failure | `unipost` | `unknown` | `unknown_error` or `internal_error` | false by default | false by default |

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
- Permanent customer/request issue:
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
  - `error.source`
  - `error.temporality`
  - `error.is_retriable`
  - `error.next_action`
  - `error.provider_error`

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
4. Dashboard failed-result surfaces no longer require reading raw provider JSON to know source and retry behavior.
5. Public docs describe every new field and explicitly tell developers not to parse `error_message`.
6. SDK source types are updated and released after backend rollout.

---

## 13. Test Plan

Backend tests:

- Unit-test taxonomy mappings for Meta transient, Meta reconnect, rate limit, media rejection, validation, quota, and unknown provider errors.
- Unit-test provider error extraction for Meta `code`, Meta `error_subcode`, TikTok `invalid_params`, and Google quota reasons.
- Queue tests should prove retriable delivery failures create retry jobs while non-retriable failures go terminal.
- API response tests should prove `retry_policy.will_retry` reflects actual job state.

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

## 14. Rollout Plan

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

## 15. Open Questions

1. Should `provider_error.trace_id` be public for all platforms or only visible in dashboard/admin/support surfaces?
2. Should `error_source=customer_request` and `error_source=unipost` overlap for validation errors, or should validation always be customer-request sourced?
3. Should manual retry be blocked for all permanent errors, or allowed with a warning for users who have changed external account/media state?
4. Should legacy rows be best-effort backfilled for dashboard consistency, or only classified on read?
5. Should top-level API error envelopes use `error.source` and `error.temporality`, while post results use `error_source` and `error_temporality`, or should both surfaces share the same naming shape?
