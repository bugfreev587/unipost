# UniPost - Threads Container Readiness and Retry PRD

**Replace fixed publish delays with observable container readiness and actionable failure handling**

**Status:** Review

**Owner:** Publishing / Platform Adapters / Reliability

**Created:** 2026-08-05

**Target:** Threads publishing reliability

**Code baseline:** `origin/dev`, not `origin/main`

> This PRD is written against `origin/dev`. Several primitives it depends on
> (`newProviderFailure`, `FailureContract`, `failureContractCarrier`,
> `inferDispatchFailureStage(err error)`, `DispatchEventRecorder`,
> `MediaItem.Origin`) exist on `dev` and have not yet been promoted to `main`.
> Reading this document against `main` will incorrectly suggest that the typed
> failure contract in section 8.4 does not exist. Branch the implementation
> from the latest `origin/dev`.

---

## 1. Summary

UniPost currently publishes Threads containers after fixed delays:

- 30 seconds when a post contains video;
- 3 seconds for text or image posts;
- 5 seconds before creating a carousel parent from child containers; and
- 3 seconds before publishing a Threads reply.

These delays do not prove that Meta has finished downloading and processing the container. A production video post failed after the 30-second delay with Meta OAuth error code `24`, subcode `4279009`, and `Media Not Found`, while the same media bytes published successfully in another post. The failure was then misclassified as a permanent `media_error`, so UniPost did not retry and incorrectly told the customer to fix valid media.

This PRD defines a narrow backend reliability change that:

1. polls the Threads container status endpoint until the container is ready;
2. publishes only after the status is `FINISHED`;
3. distinguishes permanent media validation failures from transient Meta processing failures;
4. treats Threads publish subcode `4279009` as a retriable readiness/propagation fallback;
5. preserves structured diagnostics without exposing credentials;
6. uses the existing delivery queue for automatic retry;
7. reuses the existing publish-token idempotency so those retries resume the container the post already created instead of publishing it twice; and
8. prevents long Threads polling from consuming all delivery-worker capacity.

No database migration, public API schema change, Dashboard change, or feature flag is required.

---

## 2. Incident and Evidence

### 2.1 Failed delivery

The motivating production delivery was:

```text
post_id=d0f1af65-5c78-4010-8ccb-cbe26fc540ae
post_failure_id=c325bd15-01c7-413c-bc06-e775c5188da9
social_post_result_id=0ea2ea8c-07e3-4257-b192-d35820e5db3c
platform=threads
source=api
post_status=failed
failure_stage=dispatch
error_code=media_error
platform_error_code=24
is_retriable=false
next_action=fix_media
```

Meta returned:

```json
{
  "error": {
    "message": "The requested resource does not exist",
    "type": "OAuthException",
    "code": 24,
    "error_subcode": 4279009,
    "is_transient": false,
    "error_user_title": "Media Not Found",
    "error_user_msg": "The media with id 18077125565432112 cannot be found."
  }
}
```

The referenced UniPost media was an uploaded `video/mp4` object:

```text
size_bytes=14573554
duration=15.1s
dimensions=1080x1920
```

The media was uploaded approximately one second before the post was created.

### 2.2 Account-level publishing pattern

The same Threads account had the following observed video results:

| Post created (UTC) | Result | Media bytes | Post created to terminal state |
| --- | --- | ---: | ---: |
| 2026-08-03 06:00:29 | failed (`4279009`) | 17,922,030 | unavailable |
| 2026-08-05 01:02:47 | published | 18,798,478 | 38s |
| 2026-08-05 01:04:20 | published | 14,573,554 | 38s |
| 2026-08-05 04:00:28 | published | 18,798,478 | 39s |
| 2026-08-05 04:01:37 | failed (`4279009`) | 14,573,554 | 31s |
| 2026-08-05 04:02:33 | published | 14,936,475 | 37s |

The final column is `social_posts.created_at` to `published_at` (or to the
failure row's `created_at`). It includes delivery-queue pickup latency and is
therefore an upper bound on adapter time, not a measurement of it.

Two of six video posts failed with the same Meta subcode. The exact `14,573,554`-byte video succeeded in one post and failed in another. The account's observed text and image posts succeeded, and subcode `4279009` appeared only on video publishing.

### 2.3 What the evidence proves

The evidence rules out a deterministic media-format defect. A byte-identical file cannot be both permanently invalid and valid under the same platform specification.

The timing is consistent with a readiness race, though it is weaker evidence
than the byte-identical outcome split because the measured durations include
queue latency:

- the failure returned almost immediately after UniPost's fixed 30-second delay;
- successful video deliveries completed in approximately 38-39 seconds end to
  end, versus 31 seconds for the failing one; and
- the Threads API exposes an explicit asynchronous container status, which
  means readiness is observable and UniPost is choosing not to observe it.

### 2.4 What the evidence does not prove

UniPost did not record the failed container's status before calling `threads_publish`. The incident therefore cannot prove that this specific container was `IN_PROGRESS` at publish time. A transient Meta download or processing failure remains possible.

The product conclusion is consequently:

> The incident is a high-confidence Threads container-readiness or Meta-processing race, not a permanent customer-media defect.

Polling the authoritative status resolves both sides of that uncertainty: it prevents early publish and records a real processing error when Meta reports one.

---

## 3. Current Behavior

### 3.1 Threads adapter

`api/internal/platform/threads.go` currently follows this sequence:

1. resolve the Threads user ID;
2. create a text, media, or carousel container;
3. inspect the local media list for video;
4. sleep for 30 seconds when video is present, otherwise sleep for 3 seconds;
5. call `POST /{threads-user-id}/threads_publish`; and
6. return the published Threads media ID and permalink.

The adapter does not query the container's actual processing status before publish.

Additional fixed waits exist in the same adapter:

- carousel children are given a fixed 5-second delay before the parent carousel container is created; and
- text replies are given a fixed 3-second delay before publish.

### 3.2 Failure classification

`api/internal/postfailures/taxonomy.go` prefers the top-level Meta `code` for `platform_error_code`. For this incident that correctly stores `24`; Meta subcode `4279009` remains available in the structured `provider_error.subcode` field.

The classifier then reaches the generic branch:

```go
case strings.Contains(s, "media"):
    c.ErrorCode = "media_error"
```

Because `Media Not Found` contains `media`, the result becomes:

```text
error_code=media_error
error_temporality=permanent
is_retriable=false
next_action=fix_media
```

### 3.3 Retry consequence

The delivery worker schedules another attempt only when `failure.IsRetriable` is true. The incident therefore moved directly to a terminal failed state.

When a failure is retriable, UniPost's existing queue behavior is sufficient:

- the original dispatch failure creates a retry job;
- the result remains `processing` while another attempt is scheduled;
- retries use the standard backoff, beginning at two minutes; and
- the retry job uses the configured maximum of five attempts.

This PRD does not add an adapter-local immediate publish retry loop. Queue-owned retries preserve audit history, admission control, worker leases, and the existing duplicate-publish guards.

### 3.4 Execution context

Immediate API posts no longer execute adapters in the request path. The API persists the post, creates delivery jobs, and returns `202 Accepted`. The fixed Threads delay therefore blocks a background delivery-worker slot, not the API HTTP request.

This distinction matters because a five-minute polling budget is safe for API response latency but can reduce worker throughput.

---

## 4. Problem

UniPost makes three incorrect assumptions in the current Threads publishing flow:

1. **Elapsed time is treated as readiness.** A fixed 30-second delay cannot prove that a video container is ready.
2. **Provider wording is treated as customer-media evidence.** `Media Not Found` is classified as permanent even when it represents a Threads readiness or propagation race.
3. **All processing failures share one retry meaning.** A true format rejection such as `INVALID_DURATION` must not behave like a transient `FAILED_PROCESSING_VIDEO` or poll timeout.

The result is avoidable customer-visible failure, incorrect remediation guidance, missing diagnostics, and no automatic recovery for a failure that frequently succeeds on a later attempt.

---

## 5. Goals

1. Publish Threads containers only after Meta reports `status=FINISHED`, or after the bounded grace in section 8.2 when a container type reports no recognizable status at all.
2. Remove blind fixed waits from the Threads post, carousel-parent, and reply publish paths.
3. Persist actionable container diagnostics for failures and timeouts.
4. Classify permanent Threads media validation errors as `media_error` with `next_action=fix_media`.
5. Classify transient Threads download, processing, timeout, expiry, and propagation failures as `temporary_platform_error` with automatic retry.
6. Treat Meta code `24`, subcode `4279009` from the Threads publish boundary as retriable even when Meta returns `is_transient=false`.
7. Reuse the existing publish-token idempotency so a retry resumes the container this post already created instead of creating and publishing a second one.
8. Preserve top-level Meta code `24` in `platform_error_code` and preserve `4279009` in `provider_error.subcode`.
9. Respect context cancellation during polling and publish requests.
10. Prevent long-running Threads polls from exhausting all global delivery-worker slots.
11. Add deterministic tests for the status state machine and failure taxonomy.

---

## 6. Non-goals

- Do not change customer media specifications or validation limits.
- Do not modify, transcode, or re-upload the customer's media as part of this fix.
- Do not add a new queue system or retry table.
- Do not add an in-adapter retry loop around `threads_publish`.
- Do not change the public post request or result schema.
- Do not backfill historical failure rows.
- Do not automatically retry arbitrary Meta `Media Not Found` errors from Instagram or Facebook.
- Do not expose raw access tokens, signed media URLs, request headers, or unsanitized provider payloads.
- Do not add a feature flag. This is a backend correctness fix and must follow UniPost's default no-flag rule.
- Do not merge or promote the change until Preview Acceptance and deployed regression checks pass on the exact PR head SHA.

---

## 7. Product Principles

### 7.1 Observe state instead of guessing time

Meta's container status is authoritative. UniPost must never use a local sleep duration as evidence that a container is publishable.

### 7.2 Separate invalid input from temporary processing

The customer should change media only when Meta reports a stable validation defect. Download failures, provider processing failures, propagation races, and timeouts should recover through the queue first.

### 7.3 Prefer typed adapter errors

New Threads container failures must use the typed provider/failure contract that already exists on `dev` so `error_code`, `failure_stage`, and `is_retriable` do not depend on incidental message wording. String taxonomy remains a fallback for raw Meta publish errors and for historical rows written before this change.

### 7.4 Keep ambiguous writes out of automatic retry

If Meta reports that a container is already `PUBLISHED`, UniPost must not call `threads_publish` again and must not automatically create a new post. That state requires reconciliation because a blind retry can duplicate content.

Publish-token resume (section 8.13) is what gives this principle teeth. Without it every attempt creates a brand-new container, a brand-new container is never `PUBLISHED`, and the `PUBLISHED` branch in section 8.2 would be unreachable defensive code. With resume, a retry polls the container the previous attempt created, so a `threads_publish` that succeeded server-side while UniPost failed to observe the response is detected as `PUBLISHED` and reconciled instead of publishing the post a second time.

### 7.5 Keep retries queue-owned

The delivery queue already owns backoff, maximum attempts, job state, leases, and failure history. The adapter should report whether a failure is retriable, then return control to the worker.

---

## 8. Proposed Design

### 8.1 Shared Threads container waiter

Add a private helper to `api/internal/platform/threads.go`:

```go
func (a *ThreadsAdapter) waitForContainer(
    ctx context.Context,
    accessToken string,
    containerID string,
) error
```

The helper queries:

```text
GET https://graph.threads.net/v1.0/{threads-container-id}
    ?fields=id,status,error_message
    &access_token={access-token}
```

Polling configuration:

```text
initial status request: immediate
poll interval: 5 seconds
maximum elapsed time: 5 minutes
```

The interval and timeout must be package variables or injectable test configuration so unit tests can run without real delays. They are not deployment environment variables and are not feature flags.

Every wait between polls must use a timer selected against `ctx.Done()`. The helper must return promptly when the delivery worker is shutting down or the caller cancels the operation.

**Budget invariant.** `resolveMediaIDsToURLs` mints a 15-minute presigned R2 GET URL at the start of each dispatch, and Meta downloads the media from that URL during container processing. The 5-minute poll budget plus container creation must always stay comfortably inside that 15-minute window. Anyone raising the poll budget must raise the presigned URL TTL in the same change.

### 8.2 State machine

| Threads status | Adapter behavior | Error code | Retriable | Next action |
| --- | --- | --- | --- | --- |
| `IN_PROGRESS` | Continue polling within budget. | none | n/a | none |
| `FINISHED` | Return success; caller may publish. | none | n/a | none |
| `ERROR` | Stop polling and classify `error_message`. | mapped below | mapped below | mapped below |
| `EXPIRED` | Stop; a queue retry may create a fresh container. | `temporary_platform_error` | true | `retry_later` |
| `PUBLISHED` | Do not publish again; return an ambiguous/already-published terminal failure for reconciliation. | `platform_error` | false | `contact_support` |
| missing or unknown | Bounded grace, then treat as ready. See below. | `temporary_platform_error` at timeout | true | `retry_later` |

Only `FINISHED`, or the bounded-grace resolution below, authorizes `threads_publish`.

**Bounded grace for unrecognized status.** A container that returns HTTP 200 with no `status` field, or with a value outside the documented set, must not consume the full 5-minute budget. After `unknownStatusGraceAttempts` consecutive 200 responses in which no recognized status appears (default 3), the waiter records an `unknown_status_assumed_ready` diagnostic and proceeds to publish.

The reason is a regression asymmetry. Today only video posts fail; text and image posts on this account succeed consistently, and section 8.7 extends polling to those container types. If TEXT or IMAGE containers do not expose a recognizable `status`, a strict never-publish-without-`FINISHED` rule would stall every text and image post for 5 minutes and then fail it, converting a video-only defect into a platform-wide outage. Publishing after a bounded grace restores exactly today's behavior for those containers while preserving strict gating for video, where `IN_PROGRESS` is actually observed.

This branch is a safety net, not an expected path. Section 16.2 requires verifying the real status shape for all four container types in the development environment before merge; if all four report `FINISHED`, the grace path should never execute in production and its diagnostic counter should stay at zero.

### 8.3 `ERROR` mapping

Threads `error_message` values must be normalized case-insensitively.

Permanent customer-media validation errors:

| Threads `error_message` | UniPost classification |
| --- | --- |
| `INVALID_ASPEC_RATIO` | `media_error`, non-retriable |
| `INVALID_ASPECT_RATIO` | `media_error`, non-retriable; defensive spelling support |
| `INVALID_BIT_RATE` | `media_error`, non-retriable |
| `INVALID_DURATION` | `media_error`, non-retriable |
| `INVALID_FRAME_RATE` | `media_error`, non-retriable |
| `INVALID_AUDIO_CHANNELS` | `media_error`, non-retriable |
| `INVALID_AUDIO_CHANNEL_LAYOUT` | `media_error`, non-retriable |

Temporary provider/media-processing errors:

| Threads `error_message` | UniPost classification |
| --- | --- |
| `FAILED_DOWNLOADING_VIDEO` | `temporary_platform_error`, retriable |
| `FAILED_PROCESSING_AUDIO` | `temporary_platform_error`, retriable |
| `FAILED_PROCESSING_VIDEO` | `temporary_platform_error`, retriable |
| `UNKNOWN` | `temporary_platform_error`, retriable |
| missing/unrecognized value | `temporary_platform_error`, retriable, with raw diagnostic value retained |

`FAILED_DOWNLOADING_VIDEO` must not immediately tell the customer to replace media. Meta fetch availability, signed-URL timing, provider network behavior, and temporary storage delivery can all cause a download failure. The standard retry budget should be exhausted before a terminal user-visible failure.

### 8.4 Typed failure contract

Container `ERROR`, `EXPIRED`, `PUBLISHED`, and timeout results must use the typed failure helper that Pinterest already uses on `dev` (`api/internal/platform/provider_error.go`):

```go
newProviderFailure(
    message,                       // sanitized diagnostics, see 8.6
    map[string]any{                // ProviderError fields
        "provider": "meta",
        "reason":   normalizedStatusOrErrorMessage,
    },
    FailureContract{
        ErrorCode:   mappedUniPostErrorCode,
        Stage:       "container_processing",
        IsRetriable: mappedRetryDecision,
    },
)
```

How each half reaches the persisted row:

- `FailureContract.ErrorCode` and `FailureContract.IsRetriable` are read by `classifyError` in `api/internal/postfailures/contract.go` through the `failureContractCarrier` interface, using `errors.As` rather than a type assertion, and **override** whatever `Classify(raw)` derived from the message string.
- `FailureContract.Stage` is read by `inferDispatchFailureStage(err error)` in `api/internal/handler/social_post_queue.go` through the `FailureStage() string` carrier, which falls back to string matching only when the carrier is absent.
- The `map[string]any` half populates `provider_error` and is unrelated to classification.

Two consequences the implementation must rely on:

1. Because `ErrorCode` overrides string classification, the pre-existing generic `container processing failed` / `container processing timed out` taxonomy rule (which returns retriable `temporary_platform_error`) **cannot** hijack a typed permanent `media_error` such as `INVALID_DURATION`. No new ordering-sensitive string rule is required for container failures.
2. `FailureContract` does not carry `platform_error_code`. That field is still populated by `enrichClassification`, which backfills `FirstNonEmpty(ProviderError.Code, ProviderError.Subcode, ProviderError.Reason)` when it is otherwise empty. See section 9.2.

The error message should include sanitized diagnostics, but the typed contract is authoritative for classification.

### 8.5 Poll transport handling

For each status request:

- `200` with valid JSON: evaluate the state machine.
- `429` or `5xx`: retain the response and continue polling within the remaining budget.
- Meta authentication errors, including code `190`: stop immediately and return the provider response so existing reconnect taxonomy applies.
- other `4xx`: stop immediately as a non-retriable `platform_error` unless an existing taxonomy rule provides a more specific contract.
- malformed `200` response: retain the body and continue polling; if no valid response arrives before the budget, return a retriable timeout.
- transport error: retain the last transport error and continue while context and budget remain.

Each status request must be created with `http.NewRequestWithContext`.

### 8.6 Diagnostics

The waiter must retain:

```text
container_id
poll_count
elapsed_ms
last_http_status
status
error_message
response_body
last_transport_error
media_origin
```

`media_origin` is `external` or `managed`, taken from `MediaItem.Origin` (added on `dev`). Threads containers built from `media_ids` resolve to `managed`; containers built from request-supplied `media_urls` are `external`. This distinction is the difference between "Meta could not download our own R2 object" and "Meta could not download the customer's URL", which is the single most useful discriminator when triaging `FAILED_DOWNLOADING_VIDEO`.

Requirements:

- truncate `response_body` to at most 1,000 characters;
- never include `access_token` or the complete request URL;
- sanitize invalid UTF-8 and null bytes through existing delivery error sanitization;
- include diagnostics in timeout and terminal processing errors;
- preserve Meta provider code and subcode when the response contains them.

Example timeout message:

```text
threads container processing timed out:
container_id=18077125565432112
poll_count=60
elapsed_ms=300000
last_http_status=200
status=IN_PROGRESS
response_body={"id":"...","status":"IN_PROGRESS"}
```

### 8.7 Main post flow

Replace the fixed 30/3-second sleep in `ThreadsAdapter.Post`:

```text
create top-level container
        |
        v
waitForContainer(container_id)
        |
        +-- FINISHED --> threads_publish
        |
        +-- terminal/timeout --> return typed failure
```

The waiter applies to every top-level container type:

- text;
- image;
- video; and
- carousel.

The adapter must not decide wait behavior by locally sniffing whether a post contains video.

### 8.8 Carousel flow

The current fixed 5-second wait before creating the carousel parent must be removed.

Use the provider's documented sequence:

1. create every image/video item container with `is_carousel_item=true`;
2. create the parent carousel container with the returned child IDs;
3. poll the parent carousel container through `waitForContainer`; and
4. publish only after the parent reports `FINISHED`.

This PRD does not require individually polling every child container. The parent container is the publishable object and its status is the authoritative readiness gate for the assembled carousel. If production evidence later shows parent creation rejecting in-progress children, child batch polling requires a separate follow-up with an explicit combined timeout budget.

### 8.9 Reply flow

Replace the fixed 3-second sleep in `ReplyToComment` with the same waiter:

1. create the reply container;
2. wait for `FINISHED`; and
3. call `threads_publish` using a request bound to `ctx`.

### 8.10 Context-aware publish

Both main-post and reply `threads_publish` requests currently use `http.Client.Post`, which is not bound to the provided context. Replace them with `http.NewRequestWithContext` followed by `a.client.Do`.

The existing client-level 60-second timeout remains a per-request upper bound. Context cancellation may end the request sooner.

### 8.11 Threads-specific `4279009` handling

There are two layers here. The typed adapter contract is primary; the taxonomy string rule is a fallback.

**Primary: type the publish error in the adapter.** When `threads_publish` returns HTTP 400 with Meta code `24` and subcode `4279009`, `ThreadsAdapter.Post` must return `newProviderFailure` with `FailureContract{ErrorCode: "temporary_platform_error", Stage: "dispatch", IsRetriable: true}` and the parsed Meta fields in the `ProviderError` half. Because the typed contract overrides string classification (section 8.4), this alone produces the correct row and does not depend on message wording at all.

**Fallback: taxonomy string rule.** Add a rule before the generic `media` branch in `api/internal/postfailures/taxonomy.go`:

```text
boundary identifies a Threads publish failure
AND Meta code = 24
AND Meta subcode = 4279009
```

This rule exists for paths that do not carry the typed contract: `DeriveLegacyContract` reclassification of historical rows, and any future Threads call site that returns a bare error. It must not be the mechanism the new code depends on.

Classification (identical from either layer):

```text
error_code=temporary_platform_error
is_retriable=true
error_source=platform
error_temporality=temporary
next_action=retry_later
platform_error_code=24
provider_error.provider=meta
provider_error.code=24
provider_error.subcode=4279009
```

The implementation must inspect `extractMetaSubcode(raw)` or `provider_error.subcode`. It must not compare `PlatformErrorCode` to `4279009`, because the existing contract intentionally prefers top-level Meta code `24` for that field.

The fallback rule must be Threads-specific. Today `ThreadsAdapter.Post` formats publish errors as `publish failed (%d): %s`, which contains no platform token at all — the only Meta signals are `oauthexception` and `fbtrace_id`, which Instagram and Facebook share. The adapter must therefore also adopt a `threads publish failed` prefix so the fallback rule can identify the boundary. Historical rows keep the old wording, so the negative test in section 14.2 must not assume the prefix applies retroactively.

Meta's `is_transient=false` does not override this product classification. In this exact Threads publish boundary, production evidence and the asynchronous container protocol show that the operation can succeed after provider readiness catches up. The exact subcode is narrower and more actionable than the generic transient flag.

### 8.12 Worker concurrency guardrail

A failed status poll may occupy a delivery slot for up to five minutes. Threads currently has no platform-specific delivery concurrency cap, so a burst of stuck Threads containers can consume all default global worker slots.

Add a Threads platform cap consistent with the existing Instagram and TikTok defaults:

```text
POST_DELIVERY_PLATFORM_CAP_THREADS default=3
```

The worker lease heartbeat already renews a running job every 30 seconds with a 90-second lease. A live five-minute poll therefore remains owned and is not eligible for stale recovery. The existing five-minute stale-attempt threshold still protects abandoned jobs after their lease expires.

---

### 8.13 Publish-token resume

Decided in section 19.1. This PRD both makes publish failures retriable and stretches the in-flight window from a fixed 30 seconds to as much as 5 minutes, so a worker that dies mid-publish becomes materially more likely. Without resume, every retry creates a brand-new container, and a `threads_publish` that succeeded server-side but whose response we never observed would publish the post twice.

The mechanism already exists and is already wired. `attachPublishTokenResume` (`api/internal/handler/social_post_queue.go`) injects `OptResumePublishToken` and `OptOnPublishToken` into `PlatformOptions` for every delivery job on every platform; Instagram and TikTok consume them. `ThreadsAdapter.Post` currently discards them — its body opens with `_ = opts`. No migration and no new plumbing is required.

**Persist.** Immediately after the top-level container is created — text, image, video, or carousel parent — call `OptOnPublishToken` with the container ID, before any polling begins. The token must be durable before the first opportunity to lose the worker, otherwise resume cannot help.

**Resume.** When `OptResumePublishToken` is present, do not create a container. Poll the resumed container first and branch on its status:

| Resumed container status | Action |
| --- | --- |
| `FINISHED` | Publish it. This is the normal recovery path. |
| `IN_PROGRESS` | Keep polling within the remaining budget, exactly as a fresh container. |
| `PUBLISHED` | Already delivered. Do **not** publish again. Return the existing media ID as success and reconcile, per section 7.4. |
| `EXPIRED` | The 24-hour window closed. Discard the token, create a fresh container, and persist the new one. |
| `ERROR` | Classify per section 8.3 and do not reuse. If the mapped classification is retriable, discard the token so the next attempt starts clean. |
| missing / unknown | Treat as unusable: discard the token and create a fresh container rather than risk publishing an unknown object. |

This is what makes the `PUBLISHED` row in section 8.2 real protection rather than defensive dead code — without resume a freshly created container can never be `PUBLISHED`, so that branch would be unreachable.

**Reply and carousel.** Replies persist their own reply container ID under the same token. Carousel posts persist only the **parent** container ID; children are cheap to recreate and are not individually resumable, so an expired or unusable parent means recreating the whole set.

---

## 9. Failure Contract

### 9.1 Readiness timeout

```json
{
  "error_code": "temporary_platform_error",
  "failure_stage": "container_processing",
  "platform_error_code": null,
  "is_retriable": true,
  "error_source": "platform",
  "error_temporality": "temporary",
  "next_action": "retry_later"
}
```

To keep `platform_error_code` null here, the timeout failure must **not** set `reason` in its `ProviderError` fields. The last observed status (`IN_PROGRESS`) belongs in the diagnostics message described in section 8.6, not in `provider_error.reason`, because any non-empty `reason` would be backfilled into `platform_error_code`. `provider=meta` may still be set; it carries no code, subcode, or reason and so does not trigger the backfill.

### 9.2 Permanent media validation

```json
{
  "error_code": "media_error",
  "failure_stage": "container_processing",
  "platform_error_code": "INVALID_DURATION",
  "is_retriable": false,
  "error_source": "platform",
  "error_temporality": "permanent",
  "next_action": "fix_media",
  "provider_error": {
    "provider": "meta",
    "reason": "INVALID_DURATION"
  }
}
```

`platform_error_code` is **not** null here. Meta returns no numeric code for a container `ERROR`, so `enrichClassification` backfills the field from `FirstNonEmpty(ProviderError.Code, ProviderError.Subcode, ProviderError.Reason)`, which resolves to the normalized `reason`. This is the desired outcome — `INVALID_DURATION` is the most actionable value available — but it is produced by existing backfill behavior rather than set explicitly, so tests must assert it rather than assert null.

Section 9.1 differs because a readiness timeout carries no provider code, subcode, or reason, leaving the field genuinely empty.

### 9.3 Threads publish propagation fallback

```json
{
  "error_code": "temporary_platform_error",
  "failure_stage": "dispatch",
  "platform_error_code": "24",
  "is_retriable": true,
  "error_source": "platform",
  "error_temporality": "temporary",
  "next_action": "retry_later",
  "provider_error": {
    "provider": "meta",
    "http_status": 400,
    "code": "24",
    "subcode": "4279009",
    "type": "OAuthException",
    "is_transient": false
  }
}
```

The publish fallback keeps `failure_stage=dispatch` because the container reached `FINISHED` and the failure came from `threads_publish`. Container status failures use `failure_stage=container_processing`.

---

## 10. API and Dashboard Behavior

No response schema changes are required.

### 10.1 During automatic retry

For a retriable initial dispatch failure:

- the per-platform result remains `processing`;
- `retry_policy.will_retry=true`;
- the next retry job is visible through existing queue surfaces; and
- the customer is not told to modify valid media.

### 10.2 After retry exhaustion

When temporary failures exhaust the standard retry budget:

- the result becomes `failed`;
- the last structured failure remains `temporary_platform_error`;
- `retry_policy.will_retry=false`;
- manual retry remains available while the media is retained; and
- support can inspect container diagnostics and provider fields.

### 10.3 Permanent media validation

When Meta explicitly reports a permanent invalid-media reason:

- no automatic retry is scheduled;
- the result becomes `failed`;
- `next_action=fix_media`; and
- Dashboard/API guidance may correctly ask the customer to change or re-upload the media.

---

## 11. Media Retention and Manual Retry

The existing manual retry endpoint is sufficient:

```text
POST /v1/posts/{post-id}/results/{result-id}/retry
```

Before creating the retry job, UniPost atomically verifies that every referenced media row:

- belongs to the workspace;
- still exists;
- has `status=uploaded`; and
- can be reactivated for publishing retention.

If the retained media is unavailable, the endpoint returns:

```text
HTTP 409
code=MEDIA_REUPLOAD_REQUIRED
```

Therefore the motivating post can be retried without modifying the video while its media is retained. After cleanup, the caller must upload the media again.

This PRD does not perform an automatic historical retry. Incident remediation remains an explicit operator or customer action so UniPost does not publish old content without authorization.

---

## 12. Observability

### 12.1 Transport: reuse `DispatchEventRecorder`

Do not invent a new logging convention. `dev` added `api/internal/platform/dispatch_context.go`, and `publishOneContext` already installs a `DispatchEventRecorder` into the dispatch context and snapshots it onto `publishOneOutcome.dispatchEvents`. Adapters read it via `DispatchMetadataFromContext`.

`DispatchEvent` already carries almost exactly the fields this section requires: `Name`, `Status`, `ErrorCode`, `Reason`, `FailureStage`, `Retriable`, `Duration`, `HTTPStatus`, `ProviderCode`, `MediaType`, `MediaSizeBytes`, `CustomerInput`.

Three consequences:

1. **The 16-event cap is a hard constraint, not a style preference.** `maxDispatchEvents = 16`, and `Record` silently drops events beyond it. A 5-minute wait at a 5-second interval produces up to 60 polls, so per-poll events would overflow the buffer and discard the terminal event — the one that matters. Section 12.4's "one summary, one terminal" rule is what keeps the wait inside the cap.
2. **Section 13's redaction requirements become structural.** `DispatchEvent` has no field capable of holding a URL or a token, so token leakage through this surface is prevented by the type rather than by review discipline. Free-text diagnostics still go through the sanitizer.
3. **`poll_count` has no home in `DispatchEvent`.** Record it in the sanitized diagnostics message described in section 8.6, and use `Duration` for elapsed time. Do not add a high-cardinality field to `DispatchEvent` for it.

Use `CustomerInput` to carry the `media_origin` distinction from section 8.6: `true` for `MediaOriginExternal`, `false` for `MediaOriginManaged`.

### 12.2 Required structured fields

Every terminal Threads container wait must make the following queryable through existing post failure, dispatch event, and integration-log surfaces:

- Threads container ID;
- final status;
- normalized `error_message`;
- poll count;
- elapsed milliseconds;
- last HTTP status;
- retry decision;
- failure stage;
- Meta code and subcode when present; and
- delivery job ID through existing worker metadata.

### 12.3 Operational measures

The following measures must be derivable from existing structured integration logs, dispatch events, and post-failure records:

```text
threads_container_wait_total{outcome}
threads_container_wait_duration_ms{outcome}
threads_container_poll_count{outcome}
threads_publish_failure_total{code,subcode,retriable}
```

Expected `outcome` values:

```text
finished
error
expired
already_published
timeout
cancelled
poll_http_error
poll_decode_error
unknown_status_assumed_ready
```

`unknown_status_assumed_ready` is expected to stay at zero in production. A non-zero value means a container type is not reporting a documented status and the section 8.2 grace path is carrying traffic; treat it as an alert, not as normal operation.

This PRD does not introduce a new metrics subsystem. If these measures are later exported as counters or histograms, metric labels must not contain post IDs, container IDs, user IDs, workspace IDs, captions, media URLs, or other high-cardinality/customer-content values.

### 12.4 Logs

Normal `IN_PROGRESS` polls must not emit one log entry or one dispatch event per request. Emit one summarized success event and one terminal failure event per container wait. This avoids log volume proportional to poll count and keeps the wait inside the 16-event dispatch buffer described in section 12.1.

---

## 13. Security and Privacy

1. Do not include `access_token` in error strings, diagnostics, metrics, or integration logs.
2. Do not include complete status-request URLs because the token is a query parameter.
3. Continue using the delivery error sanitizer before persistence.
4. Truncate provider response bodies to 1,000 characters.
5. Treat provider response text as untrusted input; normalize UTF-8 and remove null bytes.
6. Do not expose the customer's caption, email, workspace name, or media URL in new telemetry.
7. Do not add a new public endpoint for container status.

---

## 14. Test Requirements

### 14.1 Threads adapter unit tests

Add deterministic fake-transport tests to `api/internal/platform/threads_test.go`:

1. `IN_PROGRESS -> FINISHED` calls `threads_publish` only after `FINISHED`.
2. Immediate `FINISHED` publishes without a fixed 30-second delay.
3. `ERROR + INVALID_DURATION` returns typed non-retriable `media_error`.
4. `ERROR + FAILED_DOWNLOADING_VIDEO` returns typed retriable `temporary_platform_error`.
5. unknown/missing `error_message` remains retriable and includes diagnostics.
6. `EXPIRED` is retriable and does not call publish.
7. `PUBLISHED` is non-retriable and does not call publish again.
8. repeated `IN_PROGRESS` reaches the configured timeout with poll diagnostics.
9. `429` and `5xx` poll responses are retried within budget.
10. OAuth code `190` poll response remains reconnect-required through the existing taxonomy.
11. context cancellation stops polling promptly.
12. malformed JSON is diagnosable and does not leak the access token.
13. the main publish request is bound to context.
14. the reply publish request uses the waiter and is bound to context.
15. carousel publishing waits on the parent container and no longer depends on the fixed five-second sleep.
16. a container returning HTTP 200 with no recognizable `status` publishes after the bounded grace instead of consuming the full budget, and records `unknown_status_assumed_ready`.
17. a wait that polls to the full budget records at most two dispatch events, so the terminal event survives the 16-event cap.
18. `MediaOriginManaged` and `MediaOriginExternal` containers set `CustomerInput` correspondingly on the terminal dispatch event.
19. the top-level container ID is persisted through `OptOnPublishToken` immediately after creation, before polling starts.
20. a resumed `FINISHED` container publishes without creating a second container.
21. a resumed `PUBLISHED` container returns the existing media ID and issues **no** `threads_publish` call — the duplicate-publish guard.
22. a resumed `EXPIRED` container discards the token, creates a fresh container, and persists the new token.
23. a resumed `IN_PROGRESS` container continues polling within the remaining budget.
24. a resumed container with a missing or unrecognized status creates a fresh container rather than publishing an unknown object.
25. carousel resume reuses the parent container only; children are recreated.

### 14.2 Taxonomy tests

Add the complete motivating error payload to `api/internal/postfailures/taxonomy_test.go` and assert:

```text
error_code=temporary_platform_error
platform_error_code=24
provider_error.code=24
provider_error.subcode=4279009
is_retriable=true
error_source=platform
error_temporality=temporary
next_action=retry_later
```

Add a negative test proving that an unrelated Meta `Media Not Found` message without the Threads publish boundary does not inherit this override.

Add explicit container-error classification tests proving that:

- Threads `INVALID_DURATION` does not fall through to the generic retriable `container processing failed` rule; and
- Threads `FAILED_PROCESSING_VIDEO` does not fall through to permanent generic `media_error` wording.

Both must be asserted through the typed contract path (`classifyError` with a `newProviderFailure` error), not only through `Classify(raw)`, because the typed `ErrorCode` override is what makes them correct. Also assert that `INVALID_DURATION` yields `platform_error_code="INVALID_DURATION"` via `enrichClassification` backfill, and that the readiness timeout yields a null `platform_error_code` because it sets no `reason`.

### 14.3 Worker tests

Add or extend worker configuration tests to assert:

```text
POST_DELIVERY_PLATFORM_CAP_THREADS default=3
```

Existing lease-heartbeat tests must continue to pass without modification to lease semantics.

### 14.4 Full backend validation

From `api/`:

```bash
GOCACHE=/tmp/unipost-go-build go test ./...
```

---

## 15. Acceptance Criteria

The change is accepted only when all of the following are true on the exact PR head SHA:

1. A fake Threads video container that remains `IN_PROGRESS` for longer than 30 seconds is not published early.
2. The same container publishes after it reaches `FINISHED`.
3. No production code path in Threads main-post, carousel-parent, or reply publishing relies on the existing fixed 30/5/3-second sleeps.
4. A Threads `INVALID_DURATION` container error is permanent `media_error` with `fix_media`.
5. A Threads `FAILED_DOWNLOADING_VIDEO` container error is temporary and retriable.
6. The exact code `24` / subcode `4279009` payload is temporary and retriable.
7. An unrelated Meta media-not-found error is not reclassified by the Threads-specific fallback.
8. `platform_error_code` remains `24`, while `provider_error.subcode` is `4279009`.
9. Context cancellation ends container polling without waiting for the full timeout.
10. Poll diagnostics contain status, error message, count, elapsed time, media origin, and last response but no access token.
11. Threads delivery concurrency defaults to three active jobs per worker process.
12. TEXT, IMAGE, VIDEO, and CAROUSEL containers have each been observed in the development environment and their real `status` values recorded in the PR, per section 16.2.
13. A container with an unrecognized status publishes after the bounded grace rather than stalling for the full budget.
14. A retry never publishes a post twice: a resumed `PUBLISHED` container issues no second `threads_publish`, and a resumed `FINISHED` container creates no second container.
15. The full backend test suite passes.
16. Preview Acceptance, deployed regression, and Codex browser acceptance pass against the exact Draft PR head SHA before merge to `dev`.
17. After merge, the development deployment completes and the real development environment shows the expected retry contract and successful Threads video publish behavior.

---

## 16. Rollout

### 16.1 Branch and Preview

Follow the normal UniPost task flow:

```text
dev-<task-slug>
    -> Draft PR to dev
    -> Railway PR Environment
    -> Vercel Preview wired to the PR API
    -> local CI
    -> deployed regression
    -> Codex browser acceptance
```

Do not merge until every required gate succeeds on the exact head SHA.

### 16.2 Development acceptance

**Before merge, on the PR Preview environment**, verify the container status shape for every container type this change now polls:

1. publish a TEXT-only Threads post and record the observed `status` value;
2. publish a single-IMAGE post and record it;
3. publish a single-VIDEO post and record it; and
4. publish a CAROUSEL post and record the parent container's value.

Paste the four observed values into the PR. If any type does not return a documented `status`, the bounded-grace path in section 8.2 is load-bearing rather than defensive, and that must be stated explicitly before merge. This gate exists because sections 8.7 and 8.8 extend polling to container types that currently publish successfully with a fixed sleep, and a wrong assumption there would regress working traffic.

**After merge to `dev`:**

5. wait for the Railway development API deployment;
6. confirm the worker reports the Threads platform cap;
7. publish a development Threads video through `https://dev-api.unipost.dev`;
8. verify that container diagnostics show one or more polls and `FINISHED` before publish;
9. verify the result becomes `published`;
10. verify `unknown_status_assumed_ready` did not fire for any of the four container types; and
11. verify no access token appears in logs or failure details.

If a safe development test account cannot force `ERROR` or `4279009`, the deterministic adapter and taxonomy tests remain the acceptance evidence for those branches. Do not induce malformed customer media on a real account without explicit authorization.

### 16.3 Production monitoring

After a separately authorized production release, monitor for at least 24 hours:

- Threads video publish success rate;
- `4279009` count and retry recovery rate;
- container wait p50/p95/p99 duration;
- container timeout count;
- Threads worker concurrency saturation; and
- queue age for Threads delivery jobs.

### 16.4 Rollback

Rollback is a code rollback through the normal environment promotion flow. There is no feature flag.

Rollback triggers include:

- material increase in duplicate Threads posts;
- Threads queue age exceeding the delivery SLO because of the new poll budget;
- unexpected authentication failures from the status endpoint;
- access-token leakage in any new diagnostic surface; or
- broad Threads publish regression relative to the pre-release baseline.

Do not roll back only because average video delivery time increases from the fixed 30-second floor to the actual provider readiness time; that latency is expected and is preferable to a false terminal failure.

---

## 17. Implementation Surface

Branch from the latest `origin/dev`. Expected files:

```text
api/internal/platform/threads.go
api/internal/platform/threads_test.go
api/internal/postfailures/taxonomy.go
api/internal/postfailures/taxonomy_test.go
api/internal/worker/post_delivery.go
api/internal/worker/post_delivery_worker_test.go
```

Files this change reads but must **not** need to modify. If any of them requires an edit, the typed contract is being bypassed and the design should be revisited:

```text
api/internal/platform/provider_error.go     # newProviderFailure, FailureContract
api/internal/platform/dispatch_context.go   # DispatchEventRecorder, DispatchMetadata
api/internal/postfailures/contract.go       # failureContractCarrier, enrichClassification
api/internal/handler/social_post_queue.go   # inferDispatchFailureStage(err error)
```

No migration, generated database code, Dashboard code, public OpenAPI schema, or SDK change is expected.

If implementation reveals that a schema or public contract change is necessary, stop and revise this PRD before expanding scope.

---

## 18. Resolved Decisions

| Question | Decision |
| --- | --- |
| Fixed delay or status polling? | Poll authoritative Threads container status. |
| Poll only videos? | No. Poll every top-level publishable container. |
| Poll budget? | Immediate first check, then every 5 seconds, maximum 5 minutes. |
| Publish while `IN_PROGRESS`? | Never. |
| Automatic retry owner? | Existing delivery queue, not adapter-local loops. |
| `4279009` retry decision? | Retriable only at the Threads publish boundary. |
| Store subcode in `platform_error_code`? | No. Preserve top-level `24`; store subcode in `provider_error.subcode`. |
| Treat all Threads `ERROR` states as media defects? | No. Map validation failures separately from download/processing failures. |
| Feature flag? | No. |
| Historical auto-retry? | No. Manual/operator action only. |
| Threads worker cap? | Add default cap of three. |
| Code baseline? | `origin/dev`. The typed failure contract does not exist on `main`. |
| Classification mechanism? | Typed `FailureContract` is primary; taxonomy string rules are fallback only. |
| Unrecognized container status? | Bounded grace, then publish. Never stall working container types for the full budget. |
| Observability transport? | Existing `DispatchEventRecorder`, within its 16-event cap. |
| Publish-token resume for Threads? | Adopt. Reuses the existing per-result token; no migration. See 8.13 and 19.1. |

---

## 19. Decision Record

### 19.1 Reuse the existing publish-token idempotency for Threads

**Status: resolved — adopt. Signed off by the owner 2026-08-06.** The implementation contract is section 8.13; this section records why.

An earlier draft of this PRD listed "do not implement crash-resume from a persisted Threads creation ID" as a non-goal. That non-goal was removed because the review found that the mechanism it declines already exists and is already wired for every platform:

- `attachPublishTokenResume` (`api/internal/handler/social_post_queue.go`) unconditionally injects `OptResumePublishToken` and `OptOnPublishToken` into `PlatformOptions` for **every** delivery job, on every platform.
- Instagram and TikTok consume it to resume from an already-created container or publish ID instead of duplicating a post.
- `ThreadsAdapter.Post` discards it: the function body opens with `_ = opts`.
- The storage column (`social_post_results.publish_token`) and its lease-checked writer already exist. Adopting it for Threads requires no migration and no new plumbing.

Arguments for adopting it in this change:

- This PRD makes Threads publish failures newly retriable and extends the in-flight window from a fixed 30 seconds to as much as 5 minutes. Both changes increase the number of retries and the size of the window in which a worker can die mid-publish.
- UniPost shipped a production duplicate-publish incident fix in July 2026. Declining an existing, proven idempotency guard while increasing retry pressure moves against that work.
- Without it, the `PUBLISHED` row in section 8.2 and the principle in section 7.4 are unreachable: a freshly created container is never `PUBLISHED`. Adopting resume is what makes that branch real protection rather than defensive dead code.

Arguments for deferring it:

- It widens this PRD beyond a readiness fix, and the Threads container semantics for resume are not yet validated the way Instagram's and TikTok's are.
- Threads containers expire after 24 hours, so a resumed token has a bounded useful life and needs an expiry check that Instagram and TikTok may not model identically.
- The specific incident that motivated this PRD (`4279009` at the publish boundary) is a definitive HTTP 400 in which the publish provably did not happen, so retrying it is safe without resume.

**Outcome:** adopt, scoped as recommended — persist the top-level container ID via `OptOnPublishToken`, and on resume poll that container's status first rather than blindly re-publishing. The deferral arguments are answered rather than dismissed: the 24-hour expiry concern is handled explicitly by treating `EXPIRED` as "create a fresh container" (section 8.13), and the observation that `4279009` alone is safe to retry without resume is true but does not cover the broader window this PRD opens.

Implementation contract: section 8.13.

---

## 20. References

First-party documentation (authoritative for container status values, the `error_message` vocabulary, and the "once per minute, no more than 5 minutes" polling guidance):

- Meta Threads troubleshooting: <https://developers.facebook.com/docs/threads/troubleshooting/>
- Meta Threads Posts documentation: <https://developers.facebook.com/docs/threads/posts/>

Secondary references:

- Meta Threads API, Check Container's Publishing Status: <https://www.postman.com/meta/threads/request/34203612-72c20362-5b0c-4f14-b9cd-4315ff91cd85>
- Meta Threads API, Carousel Threads Posts: <https://www.postman.com/meta/threads/documentation/dht3nzz/threads-api>

Code (all paths as they exist on `origin/dev`):

- Existing Instagram container waiter: `api/internal/platform/instagram.go`
- Typed failure contract: `api/internal/platform/provider_error.go`
- Pinterest reference implementation of the typed contract: `api/internal/platform/pinterest.go`
- Dispatch event recorder: `api/internal/platform/dispatch_context.go`
- Media origin (`external` / `managed`): `api/internal/platform/adapter.go`
- Existing failure taxonomy and contract: `api/internal/postfailures/taxonomy.go`, `api/internal/postfailures/contract.go`
- Existing delivery retry flow and stage inference: `api/internal/handler/social_post_queue.go`
- Existing publish-token idempotency: `api/internal/platform/options.go`
- Existing manual result retry: `api/internal/handler/social_post_retry.go`
- Existing error contract PRD: `docs/prd-error-source-temporality-and-retry-contract.md`
- Existing async publish queue PRD: `docs/prd-async-publish-queue.md`
