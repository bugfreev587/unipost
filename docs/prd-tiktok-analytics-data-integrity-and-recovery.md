# UniPost - TikTok Analytics Data Integrity and Recovery PRD

**Fix misleading reconnect messaging, preserve TikTok video IDs exactly, and recover affected analytics**

**Status:** Review

**Owner:** Analytics / Platform Adapters / Dashboard

**Created:** 2026-07-17

**Target incident deployment:** Approximately 2 hours after implementation is approved and started

**Historical recovery target:** 12-24 hours after production deployment

---

## 1. Executive Summary

UniPost currently has two separate TikTok Analytics problems:

1. The TikTok Analytics page can show `Reconnect TikTok to enable analytics` even when the selected account is active and its current TikTok token and analytics permissions work.
2. UniPost-published TikTok posts can show zero views, likes, comments, and shares even when TikTok reports non-zero metrics.

The zero-metric issue has a confirmed backend root cause. TikTok returns public video IDs as 19-digit JSON numbers. UniPost decodes the publish-status response into `map[string]any`, which converts JSON numbers to `float64`. A 19-digit TikTok ID cannot be represented exactly as `float64`, so UniPost queries TikTok with a rounded ID. TikTok returns an empty video list, and UniPost currently stores that empty response as a successful all-zero analytics row.

The reconnect message is a separate error-modeling and UI-state problem. The page loads profile, account metrics, public videos, posts, and post analytics concurrently. Auth failures, missing analytics scopes, disconnected accounts, and transient upstream errors can all be reduced to a broad reconnect-style message. A prior request can also complete after the user changes accounts and overwrite the selected account's current state.

This PRD defines a narrow recovery:

- Preserve TikTok video IDs without numeric precision loss.
- Stop treating empty or unresolved TikTok analytics responses as successful zero metrics.
- Separate account disconnection, missing analytics permission, and temporary upstream errors.
- Prevent stale account requests from updating the currently selected account's page.
- Re-fetch affected TikTok analytics through the existing rate-limited worker.
- Do not change TikTok publishing behavior or require healthy accounts to reconnect.

No feature flag or new database table is required.

---

## 2. Incident Evidence

### 2.1 Customer-visible symptoms

The reported symptoms were:

- The TikTok Analytics page displayed:

```text
Reconnect TikTok to enable analytics.
```

- The selected account remained available in the account selector.
- UniPost-published TikTok post cards displayed zero views, likes, comments, and shares.
- Manually refreshing the page did not recover the metrics.

### 2.2 Account findings

At investigation time:

- `clipsworld566`
  - account status: `active`
  - connection type: `managed`
  - access token: present and current
  - refresh token: present
  - recorded scopes include:
    - `user.info.profile`
    - `user.info.stats`
    - `video.list`
  - direct TikTok checks for profile, account statistics, and public videos returned HTTP 200
  - no reconnect is currently required

- `clippingstudiox`
  - account status: `disconnected`
  - connection type: `managed`
  - TikTok currently returns `access_token_invalid`
  - reconnect is required before UniPost can retrieve new account or analytics data

These account states demonstrate that account connectivity and analytics availability must not be represented as one undifferentiated boolean.

### 2.3 Video-ID precision reproduction

For a known published post, TikTok returned this exact public video ID:

```text
7663542984343883021
```

The current generic JSON decoding path converted it to a floating-point value and then formatted it as:

```text
7663542984343883000
```

The two TikTok queries produced different results:

| Query ID | TikTok result |
| --- | --- |
| `7663542984343883021` | video found; 283 views and 6 likes at investigation time |
| `7663542984343883000` | HTTP 200 with an empty video list |

This confirms that the zero metrics are not caused by TikTok ingestion delay or a missing manual refresh trigger.

### 2.4 Production impact snapshot

At investigation time, production contained:

| Metric | Value |
| --- | ---: |
| TikTok `post_analytics` rows | 2,126 |
| Rows containing any non-zero TikTok metric | 0 |
| Rows containing `platform_specific.tiktok_video_id` | 0 |
| Rows with recorded refresh failures | 246 |

The background worker is running and updating `fetched_at`, but successful empty responses reset failure state and preserve false zero values.

The exact row count will change as users publish and the worker runs. Implementation and recovery reporting must calculate the live eligible count at execution time.

---

## 3. Root Causes

### 3.1 Root cause A: lossy JSON number decoding

Relevant code:

- `api/internal/platform/tiktok.go`
  - `CheckPublishStatus`
  - `GetAnalytics`
  - `tiktokExtractPublicPostID`

Current data flow:

```text
TikTok publish-status JSON
  -> json.Decoder into map[string]any
  -> public video ID becomes float64
  -> fmt.Sprintf("%.0f", value)
  -> rounded video ID
  -> TikTok video query returns no rows
  -> empty PostMetrics returned as success
  -> all-zero analytics row upserted
```

The first irreversible error occurs when the JSON number is decoded as `float64`. Formatting the value later cannot restore the lost digits.

### 3.2 Root cause B: empty data is treated as a successful zero

`TikTokAdapter.GetAnalytics` currently returns an empty `PostMetrics` with no error when:

- the publish status is not `PUBLISH_COMPLETE`, or
- `/v2/video/query/` returns an empty `videos` array.

The analytics handler and worker interpret a nil error as a successful fetch and call `UpsertPostAnalytics`. That:

- writes zero into all available metrics,
- sets `fetched_at = NOW()`,
- resets `consecutive_failures` to zero, and
- clears `last_failure_reason`.

The stored row therefore looks healthy even though no TikTok video metrics were retrieved.

### 3.3 Root cause C: reconnect and analytics availability are conflated

The TikTok Analytics page loads multiple resources:

- TikTok profile
- account metrics
- public videos
- workspace posts
- per-post analytics

The current backend and frontend error handling does not consistently distinguish:

1. The account is disconnected.
2. The account token is invalid or expired.
3. The token is valid for publishing but lacks one or more analytics scopes.
4. TikTok returned a temporary error, rate limit, or timeout.
5. A prior request belongs to an account that is no longer selected.

The database `social_accounts.scope` value is also not sufficient proof that TikTok granted every requested scope. Current connect flows can persist the requested scope set. Runtime TikTok responses remain authoritative for analytics access.

### 3.4 Root cause D: stale concurrent page requests can overwrite current state

`TikTokAnalyticsView` starts concurrent requests when the selected account changes. The current request is not cancelled or associated with a request generation. A slower request for the previous account can complete after a newer request and update:

- the error banner,
- profile state,
- metrics state,
- video state, or
- post rows.

This can display an error from one account while the selector shows another account.

---

## 4. Goals

1. Preserve every TikTok public video ID exactly from provider response through video query, storage, API response, and UI rendering.
2. Return real TikTok views, likes, comments, and shares for accessible UniPost-published videos.
3. Never store a missing TikTok video response as a successful all-zero analytics result.
4. Preserve previously fetched real metrics when a later TikTok refresh fails.
5. Show reconnect messaging only when the selected account or its analytics authorization actually requires user action.
6. Distinguish missing analytics scopes from a fully disconnected or invalid account.
7. Prevent stale requests for a previously selected account from updating the current account's page.
8. Re-fetch eligible affected historical TikTok rows without deleting posts or blocking publishing.
9. Complete the core incident deployment approximately two hours after implementation is approved and started.
10. Complete the supported historical recovery within 12-24 hours after production deployment.

---

## 5. Non-goals

- Do not change TikTok publish, upload, privacy, scheduling, or delivery behavior.
- Do not reconnect an account automatically.
- Do not request new TikTok scopes.
- Do not scrape TikTok public pages.
- Do not add TikTok Ads or Business analytics.
- Do not add historical account-level follower trend storage.
- Do not introduce a feature flag.
- Do not add a new database table.
- Do not backfill posts older than the existing 90-day analytics support window.
- Do not treat legitimate TikTok metric values of zero as errors when TikTok returns the matching video row successfully.
- Do not promise recovery for disconnected accounts until the user reconnects them.

---

## 6. Product Decisions

### 6.1 Selected implementation approach

Use exact-number decoding at the TikTok provider boundary, explicit analytics availability errors, selected-account request isolation, and the existing analytics worker for controlled recovery.

This approach:

- fixes the root cause at ingestion,
- keeps the data model unchanged,
- fits the incident deployment target,
- uses existing failure-preservation behavior in the analytics worker, and
- avoids a high-concurrency one-time recovery process.

### 6.2 Alternatives considered

#### Alternative A: Preserve JSON numbers with `Decoder.UseNumber` — selected

Decode TikTok publish-status responses with `json.Decoder.UseNumber` and handle `json.Number` as the exact decimal string.

Advantages:

- minimal code change,
- supports TikTok responses that contain numeric or string IDs,
- no schema change,
- easy to regression-test with the real response shape.

Trade-off:

- the status response remains a generic map.

#### Alternative B: Replace the generic map with a fully typed response

Define a typed publish-status response with a custom TikTok ID unmarshaler that accepts JSON strings and numbers.

Advantages:

- strongest compile-time contract,
- clearer provider error handling.

Trade-off:

- broader refactor for the incident window.

This can be a later cleanup, but it is not required for recovery.

#### Alternative C: Match posts against `/video/list/`

List the account's videos and infer which public video corresponds to each UniPost publish result.

Rejected because:

- matching is ambiguous,
- it adds pagination and rate-limit pressure,
- it is unnecessary when TikTok already returns the exact public post ID.

---

## 7. Backend Requirements

### 7.1 Preserve public video IDs exactly

Modify `api/internal/platform/tiktok.go`.

`CheckPublishStatus` must:

1. Decode JSON with `json.Decoder.UseNumber`.
2. Return a decode error when the response is malformed.
3. Check the HTTP status before treating the response as usable.
4. Check TikTok's `error.code` envelope and return an error when it is neither empty nor `ok`.
5. Never log or return access tokens.

`tiktokExtractPublicPostID` must:

- accept a non-empty string ID,
- accept `json.Number`,
- validate that a numeric ID is an unsigned base-10 integer,
- return the exact original decimal string,
- reject `float64` rather than constructing a known-lossy ID, and
- continue supporting TikTok's current typo and possible corrected field names:
  - `publicaly_available_post_id`
  - `publically_available_post_id`
  - `publicly_available_post_id`

Acceptance example:

```text
input JSON number: 7663542984343883021
extracted string:  7663542984343883021
```

### 7.2 Query metrics with the exact ID

`TikTokAdapter.GetAnalytics` must submit the exact extracted string to:

```http
POST /v2/video/query/?fields=id,like_count,comment_count,share_count,view_count
```

When TikTok returns a matching video row:

- `view_count` maps to `VideoViews` and the legacy `Views` alias,
- `like_count` maps to `Likes`,
- `comment_count` maps to `Comments`,
- `share_count` maps to `Shares`,
- `platform_specific.tiktok_video_id` stores the exact string ID.

### 7.3 Do not convert unavailable data into zero

For `PUBLISH_COMPLETE`:

- an empty TikTok `videos` array is an analytics fetch failure, not a successful zero result;
- the adapter must return an actionable error containing the operation and exact video ID;
- the worker must use the existing failure path that updates `fetched_at`, increments `consecutive_failures`, records `last_failure_reason`, and preserves previously stored real metrics.

For a TikTok video row that exists and contains zero metric values:

- zero is legitimate and must be stored as success;
- `platform_specific.tiktok_video_id` proves that a matching video was retrieved.

For an explicit non-complete publish status:

- do not upsert an all-zero success row;
- record or preserve an unavailable/pending reason;
- allow the normal tiered refresh policy to retry later.

The initial implementation may use a typed sentinel error or a classified error string, but tests must prove that no successful zero row is produced without a matched TikTok video.

### 7.4 Normalize TikTok analytics authorization errors

Backend responses must distinguish the following conditions:

| Condition | API code | `details.reason` | HTTP status | User action |
| --- | --- | --- | ---: | --- |
| `social_accounts.status=disconnected` or `disconnected_at` set | `ACCOUNT_DISCONNECTED` | `account_disconnected` | 409 | reconnect account |
| TikTok returns `access_token_invalid`, token expired, or equivalent | `NEEDS_RECONNECT` | `account_token_invalid` | 409 | reconnect account |
| TikTok returns `scope_not_authorized` or a required analytics scope is missing | `NEEDS_RECONNECT` | `analytics_scope_required` | 409 | reconnect to grant analytics access |
| TikTok returns 429 | `UPSTREAM_RATE_LIMITED` | `provider_rate_limited` | 429 | retry later |
| TikTok timeout, network error, or 5xx | `TIKTOK_TEMPORARY_ERROR` | `provider_temporary_error` | 502 | retry later |
| TikTok returns no matching video for a complete publish | `TIKTOK_ANALYTICS_UNAVAILABLE` | `video_not_found` | 502 for live fetch; failure metadata for worker | no reconnect claim |

Requirements:

- Preserve the existing top-level `ACCOUNT_DISCONNECTED` and `NEEDS_RECONNECT` codes for authorization failures so existing clients remain compatible.
- Add `details.reason` as the machine-readable discriminator used by the dashboard and future clients.
- Do not map rate limits, timeouts, 5xx responses, or missing video data to reconnect.
- Do not mark the entire social account `reconnect_required` solely because an otherwise valid publishing token lacks an analytics-only scope.
- A definitively invalid account token may reuse the existing account reconnect-required mechanism.
- Error response bodies must not expose TikTok tokens or provider secrets.

### 7.5 Keep account connectivity separate from analytics readiness

The account list remains the source of truth for connection inventory.

Analytics readiness must be determined from:

- account status,
- disconnected timestamp,
- stored scope hints, and
- authoritative runtime TikTok responses.

The API must not claim that stored requested scopes prove runtime authorization.

No data migration is required for `social_accounts.scope` in this incident. A later PRD may improve how granted scopes are persisted if TikTok exposes an authoritative granted-scope response.

---

## 8. Dashboard Requirements

### 8.1 Selected-account request isolation

Modify:

- `dashboard/src/components/analytics/tiktok-analytics-view.tsx`

Every `loadData` execution must be associated with the account ID and one of:

- an `AbortController`, or
- a monotonically increasing request-generation identifier.

Only the latest request for the currently selected account may update:

- `error`,
- `profile`,
- `metrics`,
- `videos`,
- `postRows`,
- `loading`, or
- `refreshing`.

Changing the selector must clear the prior account's error immediately.

### 8.2 Error-message behavior

The page must render messages from the normalized backend code and `details.reason`:

| API code and reason | Required UI message |
| --- | --- |
| `ACCOUNT_DISCONNECTED` + `account_disconnected` | `This TikTok account is disconnected. Reconnect it to continue.` |
| `NEEDS_RECONNECT` + `account_token_invalid` | `Your TikTok connection has expired. Reconnect the account.` |
| `NEEDS_RECONNECT` + `analytics_scope_required` | `Reconnect TikTok to grant the permissions required for analytics.` |
| `UPSTREAM_RATE_LIMITED` + `provider_rate_limited` | `TikTok is temporarily rate limiting analytics requests. Try again later.` |
| `TIKTOK_TEMPORARY_ERROR` + `provider_temporary_error` | `TikTok Analytics is temporarily unavailable. Try again.` |
| `TIKTOK_ANALYTICS_UNAVAILABLE` + `video_not_found` | `Analytics are not available for this video yet.` |

Requirements:

- Do not show `Reconnect TikTok to enable analytics` for a transient error.
- Do not show a reconnect CTA for an active account when current profile, metrics, and video-list authorization succeeds.
- A failure in one account must not display while another account is selected.
- Successful profile or video data may remain visible when a separate analytics request fails.

### 8.3 Partial page success

The page must not use one all-or-nothing `Promise.all` failure boundary for independent resources.

Profile, account metrics, public videos, and UniPost post analytics must settle independently. The page should:

- render successful sections,
- show a scoped error or unavailable state for failed sections, and
- reserve the page-level reconnect banner for account-wide authorization failures.

This avoids hiding all TikTok data because one optional analytics resource failed.

### 8.4 Metric display semantics

For UniPost-published TikTok posts:

- render returned values, including legitimate zero values, when a matching TikTok video was retrieved;
- render `N/A` or an unavailable state when no successful analytics row exists;
- do not convert missing data to zero;
- keep impressions, reach, saves, and clicks as `N/A` because the supported TikTok API does not provide them.

---

## 9. Historical Recovery Requirements

### 9.1 Eligible rows

Recovery includes `post_analytics` rows whose related records meet all of:

- `social_accounts.platform = 'tiktok'`
- `social_post_results.status = 'published'`
- `social_post_results.external_id IS NOT NULL`
- `social_post_results.published_at` is within the existing 90-day support window
- `social_posts.deleted_at IS NULL`
- `social_accounts.disconnected_at IS NULL`
- `social_accounts.status = 'active'`

Disconnected and reconnect-required accounts are excluded until the user restores access.

### 9.2 Recovery mechanism

After the fixed code is deployed:

1. Record the deployment timestamp.
2. Calculate the live eligible row count.
3. Mark eligible TikTok analytics rows as due by setting only their refresh scheduling fields, using the existing epoch-based refresh pattern.
4. Do not delete `post_analytics` rows.
5. Do not clear any real non-zero metrics before a successful replacement is available.
6. Let `AnalyticsRefreshWorker` process the rows through its existing controls:
   - maximum 200 due rows per cycle,
   - concurrency 5,
   - normal token refresh handling,
   - failure preservation.
7. Monitor each cycle until every eligible row has a post-deployment outcome.

The existing public refresh endpoint is not sufficient as the only global recovery mechanism because it caps requests and selects the newest matching rows. Recovery must use an operational query or one-time command that can mark the complete eligible set once.

### 9.3 Recovery completion definition

An eligible row is processed when its post-deployment state is one of:

1. Success:
   - `fetched_at` is after deployment,
   - `last_failure_reason IS NULL`,
   - `platform_specific.tiktok_video_id` contains the exact video ID.

2. Explicit failure/unavailable:
   - `fetched_at` is after deployment,
   - `last_failure_reason` is populated,
   - prior real metrics, if any, remain preserved.

Recovery is not measured by requiring every row to be non-zero. A real TikTok video can legitimately have zero engagement. Completion is measured by successful video matching or an explicit failure state.

### 9.4 Rate-limit protection

During recovery:

- keep worker concurrency at 5 or lower;
- keep batches at 200 rows or lower;
- do not run an unbounded parallel script;
- monitor TikTok 429 responses, timeouts, and 5xx errors;
- pause or reduce the recovery rate if provider failures materially increase;
- do not repeatedly reset already processed rows to due.

Analytics recovery must not intentionally consume publishing worker capacity. If TikTok applies an app-wide rate limit and publishing failures increase, analytics recovery must be paused before publishing is affected further.

### 9.5 Estimated duration

Using the investigation snapshot:

```text
2,126 rows / 200 rows per cycle = at least 11 worker cycles
```

The first cycle runs at application startup and later cycles run hourly. Other due platforms and provider retries can extend the duration.

Operational targets:

- new and recently due analytics begin recovering shortly after deployment;
- all eligible rows reach a post-deployment success or explicit failure state within 12-24 hours.

---

## 10. API Compatibility

No successful response fields are removed.

Additive changes:

- machine-readable `details.reason` values for existing authorization error codes,
- specific error codes for rate limits and temporary or unavailable analytics,
- exact `platform_specific.tiktok_video_id`,
- unavailable states instead of false successful zeros.

Existing API consumers that only inspect HTTP status will continue to receive non-2xx responses for authorization and upstream failures.

Existing consumers that inspect `ACCOUNT_DISCONNECTED` or `NEEDS_RECONNECT` continue to receive those top-level codes. The new dashboard must additionally inspect `details.reason` to distinguish a disconnected account, an invalid token, and a missing analytics scope.

When `details.reason` is absent, the dashboard must preserve legacy behavior:

- `ACCOUNT_DISCONNECTED` displays the disconnected-account message;
- `NEEDS_RECONNECT` displays the expired-connection reconnect message.

Contract tests must lock the top-level codes, reason values, HTTP statuses, and legacy fallback behavior.

---

## 11. Observability

Add or preserve structured logs for:

- TikTok publish ID,
- exact extracted public video ID,
- analytics operation,
- TikTok HTTP status,
- TikTok error code,
- social post result ID,
- social account ID,
- recovery batch size,
- success/failure counts,
- 429 count, and
- empty-video result count.

Do not log:

- access tokens,
- refresh tokens,
- authorization headers,
- client secret, or
- unredacted provider credentials.

Recommended operational counters:

- TikTok analytics attempted
- TikTok analytics matched video
- TikTok analytics unavailable
- TikTok analytics authorization failure
- TikTok analytics rate limited
- TikTok analytics recovered after deployment

---

## 12. Validation Plan

### 12.1 Unit tests

Add tests in `api/internal/platform/tiktok_test.go`.

Required coverage:

1. Numeric 19-digit public ID:

```json
{
  "data": {
    "status": "PUBLISH_COMPLETE",
    "publicaly_available_post_id": [7663542984343883021]
  },
  "error": {
    "code": "ok"
  }
}
```

Assert:

- extracted ID is exactly `7663542984343883021`,
- video-query request body contains exactly that string,
- no rounded variant appears.

2. String ID response remains supported.
3. Current and corrected TikTok field spellings remain supported.
4. `float64` input is rejected rather than formatted.
5. Matching video with zero metrics is stored as a successful zero with `tiktok_video_id`.
6. Empty video list returns an unavailable error, not empty successful metrics.
7. TikTok non-OK error envelope returns an error.
8. Non-2xx publish-status response returns an error.
9. Token and scope errors both preserve the compatible `NEEDS_RECONNECT` code while returning distinct `details.reason` values.

### 12.2 Worker and handler tests

Required coverage:

1. A successful TikTok fetch upserts exact metrics and resets prior failure state.
2. An empty video result increments failure state and preserves prior real metrics.
3. A pending or failed publish status does not overwrite prior metrics with zero.
4. An invalid token returns `NEEDS_RECONNECT` with `details.reason=account_token_invalid`.
5. A missing analytics scope returns `NEEDS_RECONNECT` with `details.reason=analytics_scope_required` without marking a publishing-capable account globally disconnected.
6. A rate limit or provider 5xx does not return a reconnect message.

### 12.3 Dashboard tests

Required coverage:

1. Switching from account A to account B prevents account A's late error from appearing on account B.
2. A profile failure does not discard successful public-video or post data.
3. A transient TikTok error shows retry guidance, not reconnect guidance.
4. A disconnected account shows reconnect guidance.
5. A missing analytics scope shows analytics-permission reconnect guidance.
6. Missing post analytics render as `N/A`.
7. A legitimate matched zero renders as `0`.

### 12.4 Local CI-equivalent validation

Backend/API:

```bash
cd api
GOCACHE=/tmp/unipost-go-build go test ./...
```

Dashboard:

```bash
cd dashboard
npm run build
npm run test:regression:dashboard
```

The regression test may be skipped only when Playwright browsers are unavailable, and the skipped check and reason must be reported before promotion.

### 12.5 Development environment acceptance

After the implementation reaches `origin/dev`, wait for all development deployments and checks to complete.

Use only:

- `https://dev-api.unipost.dev`
- `https://dev-app.unipost.dev`

Verify with development-owned TikTok accounts:

1. A healthy account loads profile, stats, and public videos without a reconnect banner.
2. A known published video's post analytics match TikTok's current values.
3. The exact TikTok video ID returned by publish status appears in `platform_specific`.
4. A disconnected test account shows account reconnect guidance.
5. A token missing an analytics scope shows analytics-scope guidance.
6. A simulated or controlled temporary upstream failure shows retry guidance.
7. Switching accounts quickly cannot show stale data or stale errors.
8. TikTok publishing still works for a healthy account.

### 12.6 Production acceptance

After production deployment:

1. Confirm production health before starting the backfill.
2. Verify the known precision reproduction no longer rounds the video ID.
3. Verify at least one known customer video returns non-zero real metrics when TikTok currently reports non-zero values.
4. Confirm healthy `clipsworld566` does not require reconnect.
5. Confirm disconnected `clippingstudiox` remains excluded until user reconnection.
6. Mark eligible rows due and monitor the controlled recovery.
7. Confirm every eligible row reaches a post-deployment success or explicit failure outcome within 12-24 hours.
8. Confirm TikTok publish success and failure rates do not regress during recovery.

---

## 13. Acceptance Criteria

1. A 19-digit numeric TikTok public video ID survives JSON decoding exactly.
2. TikTok video query receives the exact provider ID.
3. A matching TikTok video returns and stores its real views, likes, comments, and shares.
4. A successful TikTok analytics row contains `platform_specific.tiktok_video_id`.
5. An empty TikTok video list is not persisted as a successful all-zero result.
6. A failed refresh preserves previously stored real metrics.
7. Missing analytics data renders as `N/A` or unavailable, not false zero.
8. Legitimate zero metrics from a matched TikTok video still render as `0`.
9. A disconnected account receives account reconnect guidance.
10. An invalid or expired account token receives account reconnect guidance.
11. An analytics-scope failure receives analytics-permission guidance without claiming the whole account is disconnected.
12. Rate limits, timeouts, network errors, provider 5xx responses, and missing videos do not produce reconnect messaging.
13. A stale request for a previously selected account cannot update the current account's state.
14. Successful page sections remain visible when an independent section fails.
15. Existing TikTok publishing behavior remains unchanged.
16. All eligible historical rows are reprocessed through controlled batches.
17. Historical recovery completes within 12-24 hours after production deployment or an exact provider/credential blocker is reported.
18. No access token, refresh token, authorization header, or client secret is exposed in logs, persisted errors, API responses, or UI.

---

## 14. Rollout Plan

1. Implement on a short-lived branch based on the latest `origin/dev`.
2. Run backend and dashboard validation.
3. Merge into local `dev`, rerun validation, and push `origin/dev`.
4. Wait for all development checks and deployments.
5. Perform real development-environment acceptance.
6. Promote through the approved staging and production release path when production release is authorized.
7. Target production incident deployment approximately two hours after implementation is approved and started.
8. Verify production health and one known TikTok metric before marking historical rows due.
9. Start the controlled historical recovery.
10. Monitor provider errors, publish health, recovery progress, and customer-visible metrics.
11. Send the customer completion update only after the deployment is healthy and the historical recovery has reached its completion criteria.

No feature flag is required. Rollback is a code rollback plus stopping the recovery schedule. Existing rows are not deleted, so stopping recovery does not lose historical post data.

---

## 15. Customer Communication Requirements

The reconnect and analytics incidents must be communicated separately.

An incident acknowledgement with the deployment and recovery estimates may be sent before the fix is deployed. A completion update must wait until the relevant deployment or historical recovery completion criteria have been verified.

### 15.1 Reconnect communication

State:

- which account is currently healthy,
- which account is disconnected,
- whether user action is required,
- that account inventory and analytics readiness are separate states, and
- that UniPost is improving the accuracy of the reconnect message.

Do not tell a user to reconnect a currently healthy account.

### 15.2 Analytics communication

State:

- the root cause is TikTok video-ID precision loss in UniPost,
- manual refresh cannot correct the issue before deployment,
- UniPost will preserve exact IDs and re-fetch affected history,
- deployment is expected in approximately two hours after implementation starts, and
- historical data will recover progressively within approximately 12-24 hours after deployment.

Do not claim historical recovery is complete until the operational completion query confirms it.

---

## 16. Final Decisions

1. The reconnect-message issue and zero-metric issue are separate failure modes and must remain separate in code, UI, tests, and customer communication.
2. Exact-number decoding at the TikTok boundary is the incident fix.
3. Missing TikTok data must not be represented as successful zero data.
4. Analytics-only scope failures must not automatically disable an otherwise publishing-capable TikTok account.
5. Historical recovery uses the existing worker's bounded batches and concurrency.
6. The supported recovery window is the existing 90-day analytics window.
7. There is no feature flag and no new database table.
8. Implementation does not begin until this PRD is reviewed and approved.
