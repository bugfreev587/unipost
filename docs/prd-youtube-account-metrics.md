# PRD - YouTube account metrics

**Status:** V1 implementation + V2 implementation
**Owner:** Product
**Engineering reviewer:** Senior Developer
**Target:** YouTube account metrics V1 and YouTube Analytics API V2
**Created:** 2026-07-03

---

## Problem

`GET /v1/accounts/{account_id}/metrics` currently returns `501 NOT_SUPPORTED` for connected YouTube accounts because `YouTubeAdapter` does not implement `platform.AccountMetricsAdapter`.

Customers already expect the shared account metrics endpoint to work across connected platforms. For YouTube, this creates avoidable integration failures even though the connected account already has enough YouTube Data API permission to return basic channel statistics.

## Goals

1. Add YouTube support to the existing account metrics endpoint.
2. Treat YouTube Data API basic channel statistics as V1.
3. Treat richer YouTube Analytics API reports as V2.
4. Avoid new Google OAuth scopes for V1.
5. Define and implement the V2 scope and reconnect requirements.
6. Keep the existing normalized account metrics contract stable for customers.

## Non-goals

- No YouTube revenue, ad performance, or monetary metrics in V1.
- No YouTube Analytics API integration in V1.
- No new UniPost API-key permission model.
- No scraping YouTube Studio or public pages.
- No silent OAuth scope upgrade for existing tokens.
- No feature flag unless explicitly requested later.

## Pre-implementation Codebase Findings

### Already present

- YouTube OAuth is implemented in `api/internal/platform/youtube.go` and `api/internal/connect/youtube.go`.
- Before V2 scope wiring, the YouTube OAuth scope set was:

```text
https://www.googleapis.com/auth/youtube.upload
https://www.googleapis.com/auth/youtube.readonly
```

- YouTube post-level analytics already reads video statistics through the YouTube Data API `videos.list` endpoint in `YouTubeAdapter.GetAnalytics`.
- YouTube account connection already stores the YouTube channel ID as `external_account_id`.
- `social_accounts.scope` already exists and the connect callback infrastructure stores granted provider scopes. V2 should verify the YouTube paths populate it correctly rather than introduce new scope storage.
- The shared account metrics handler already calls `platform.AccountMetricsAdapter` implementations through `GET /v1/accounts/{account_id}/metrics`.

### Original gaps

- `YouTubeAdapter` does not implement `GetAccountMetrics`.
- The account metrics docs list only X, Instagram, Threads, and TikTok as supported platforms.
- The reconnect analytics scopes guide does not mention YouTube.
- The request logging layer classifies all `>=500` responses as `internal_error`, even when the response body is `NOT_SUPPORTED`. This is not blocking for YouTube metrics, but it makes unsupported-platform telemetry look more severe than it is.

## External API References

- YouTube Data API `channels.list`: `https://developers.google.com/youtube/v3/docs/channels/list`
- YouTube Data API channel resource statistics: `https://developers.google.com/youtube/v3/docs/channels`
- YouTube Data API OAuth scopes: `https://developers.google.com/youtube/v3/guides/auth/server-side-web-apps`
- YouTube Data API quota and compliance audits: `https://developers.google.com/youtube/v3/guides/quota_and_compliance_audits`
- YouTube Analytics API reference: `https://developers.google.com/youtube/analytics/reference/`
- YouTube Analytics API `reports.query`: `https://developers.google.com/youtube/analytics/reference/reports/query`
- YouTube Analytics API channel reports: `https://developers.google.com/youtube/analytics/channel_reports`

## V1 - YouTube Data API Basic Account Metrics

### Product scope

V1 adds basic YouTube channel statistics to the existing account metrics endpoint:

```http
GET /v1/accounts/{account_id}/metrics
```

For a YouTube account, UniPost should return the same normalized response shape used by other platforms:

```json
{
  "data": {
    "social_account_id": "sa_youtube_123",
    "platform": "youtube",
    "follower_count": 123000,
    "following_count": 0,
    "post_count": 42,
    "platform_specific": {
      "view_count": 9876543,
      "hidden_subscriber_count": false,
      "following_count_supported": false,
      "subscriber_count_rounded": true,
      "post_count_public_only": true
    },
    "fetched_at": "2026-07-03T18:00:00Z"
  }
}
```

### Metric mapping

| UniPost field | YouTube source | Notes |
| --- | --- | --- |
| `follower_count` | `channel.statistics.subscriberCount` | YouTube rounds subscriber count to three significant figures. If subscriber count is hidden or absent, return `0` and expose `hidden_subscriber_count: true`. |
| `following_count` | Not exposed by YouTube Data API | Return `0` and expose `following_count_supported: false`. |
| `post_count` | `channel.statistics.videoCount` | YouTube returns public video count only, even for owners. |
| `platform_specific.view_count` | `channel.statistics.viewCount` | Lifetime channel video views from the Data API. |
| `platform_specific.hidden_subscriber_count` | `channel.statistics.hiddenSubscriberCount` | Indicates whether subscriber count is hidden. |
| `platform_specific.following_count_supported` | Constant `false` | YouTube does not expose a channel following count through this API. |
| `platform_specific.subscriber_count_rounded` | Constant `true` | Documents YouTube's rounded subscriber count policy in the machine-readable payload. |
| `platform_specific.post_count_public_only` | Constant `true` | Documents that `videoCount` reflects public videos only. |

### OAuth and scopes

V1 does not require a new Google OAuth scope.

The existing `https://www.googleapis.com/auth/youtube.readonly` scope is sufficient for reading the authenticated user's YouTube account and channel statistics. Existing connected YouTube accounts that already granted `youtube.readonly` should not need to reconnect for V1.

V1 does not require a new UniPost API key scope. UniPost API keys already resolve workspace access, and the endpoint remains workspace-scoped by social account ownership.

### Implementation requirements

1. Implement `GetAccountMetrics(ctx, accessToken, externalAccountID)` on `YouTubeAdapter`.
2. Call:

```http
GET https://www.googleapis.com/youtube/v3/channels?part=statistics&id={externalAccountID}
```

3. Parse `viewCount`, `subscriberCount`, `hiddenSubscriberCount`, and `videoCount` as integer or boolean fields.
4. Preserve the existing account ownership, disconnected-account, and workspace-scoping behavior in `SocialAccountHandler.AccountMetrics`.
5. If YouTube returns `401` or an auth/scope-related `403`, return a reconnect-oriented API error rather than a generic internal error.
6. If YouTube returns rate-limit, quota, or transient upstream failures, return `UPSTREAM_ERROR` with safe customer-facing messaging.
7. Rely on the existing token refresh workers for normal token freshness, then close the race where a token expires between worker ticks by adding the same inline expiry-check-and-refresh pattern used by the analytics refresh worker before the upstream metrics call.
8. Keep the endpoint live-fetching behavior. Do not introduce persistence or caching in V1.
9. Update docs to list YouTube as supported for basic account metrics.
10. Do not add another YouTube-specific string-sniffing branch to `social_account_metrics.go` if it can be avoided. Prefer a shared typed or sentinel error from platform adapters for reconnect-required cases, then have the handler map that class to a customer-facing reconnect response.

### V1 empty channel behavior

If `channels.list` returns `200` with zero items for the stored `external_account_id`, UniPost should treat the connected account as stale or no longer retrievable by the authorized user. The endpoint should return a reconnect-oriented `409 NEEDS_RECONNECT` response, not a successful zero metrics payload and not a generic `502`.

Implementation should reuse `ErrYouTubeNoChannel` if appropriate or introduce a more precise sentinel error such as `ErrYouTubeChannelNotFound`, then map it through the shared reconnect-required error path.

### V1 acceptance criteria

1. `GET /v1/accounts/{youtube_account_id}/metrics` returns `200` for an active connected YouTube account with valid `youtube.readonly` consent.
2. The response includes normalized `follower_count`, `following_count`, `post_count`, and YouTube-specific `view_count`, `hidden_subscriber_count`, `following_count_supported`, `subscriber_count_rounded`, and `post_count_public_only`.
3. Existing YouTube accounts with `youtube.readonly` do not need to reconnect.
4. Accounts missing required YouTube permission receive actionable reconnect messaging.
5. Disconnected accounts still return `409 ACCOUNT_DISCONNECTED`.
6. Non-YouTube unsupported platforms still return `501 NOT_SUPPORTED`.
7. An empty `channels.list` item set returns a reconnect-oriented `409 NEEDS_RECONNECT`.
8. Unit tests cover success, hidden subscriber count, empty channel response, auth failure, and upstream failure.
9. Docs and SDK-facing examples describe YouTube V1 metrics clearly and do not imply exact subscriber counts.

## V2 - YouTube Analytics API Reports

### Product scope

V2 adds richer owner-only YouTube Analytics API reports. V2 should be a separate platform-specific analytics surface, not a blocker for V1.

Recommended API surfaces:

```http
GET /v1/accounts/{account_id}/youtube/analytics/summary
GET /v1/accounts/{account_id}/youtube/analytics/trend
GET /v1/accounts/{account_id}/youtube/analytics/videos
```

The implemented endpoint shape follows the existing platform-specific analytics routing and docs conventions established for TikTok instead of inventing a disconnected route family. V2 does not overload the basic account metrics endpoint with date-ranged report semantics.

### V2 metrics

V2 should start with non-monetary channel reports:

- views
- likes
- comments
- shares, when available in the selected report
- subscribers gained
- subscribers lost
- estimated minutes watched
- average view duration
- average view percentage, when compatible with the chosen dimensions
- daily trend by `day`
- top videos by views or watch time

Revenue, ad performance, CPM, and partner-only content-owner reports are deferred.

### OAuth and scopes

V2 requires UniPost to request the Google OAuth scope:

```text
https://www.googleapis.com/auth/yt-analytics.readonly
```

The YouTube Analytics `reports.query` method also requires YouTube account read access. UniPost already requests:

```text
https://www.googleapis.com/auth/youtube.readonly
```

Therefore, the recommended V2 scope set is:

```text
https://www.googleapis.com/auth/youtube.upload
https://www.googleapis.com/auth/youtube.readonly
https://www.googleapis.com/auth/yt-analytics.readonly
```

Do not add `https://www.googleapis.com/auth/yt-analytics-monetary.readonly` in V2 unless revenue metrics become an explicit product requirement. That scope is for monetary and ad-performance reports and increases consent and review sensitivity.

Existing YouTube accounts must reconnect before V2 reports work because UniPost cannot silently add Google OAuth scopes to already-granted tokens.

### Google Cloud and review requirements

V2 scope readiness was confirmed on 2026-07-03. The UniPost Google Auth Platform Data Access screen already includes:

```text
https://www.googleapis.com/auth/yt-analytics.readonly
https://www.googleapis.com/auth/youtube.readonly
https://www.googleapis.com/auth/youtube.upload
```

The `yt-analytics.readonly` scope appears under non-sensitive scopes with the user-facing description "View YouTube Analytics reports for your YouTube content." Therefore, V2 is not blocked on a new Google OAuth scope application. Existing connected YouTube accounts still need reconnect if their stored `social_accounts.scope` does not include `yt-analytics.readonly`.

V2 deployment checklist:

1. Ensure the YouTube Analytics API is enabled in each Google Cloud project used by the target environment.
2. Add `yt-analytics.readonly` to both UniPost YouTube OAuth paths in code.
3. Update customer-facing Google verification and white-label credential docs.
4. Update reconnect guidance for existing YouTube accounts.
5. Verify at least one development YouTube account has reconnected with the V2 scope.

### V2 implementation requirements

1. Add `yt-analytics.readonly` to both YouTube OAuth paths:
   - `api/internal/platform/youtube.go`
   - `api/internal/connect/youtube.go`
2. Keep YouTube scope declarations in one source of truth where practical, or lock both paths with tests so they cannot drift.
3. Verify the YouTube OAuth and hosted Connect paths populate granted scopes in `social_accounts.scope`; add tests if either path can regress.
4. Add missing-scope detection in account health or the V2 handlers.
5. Query:

```http
GET https://youtubeanalytics.googleapis.com/v2/reports
```

6. Use `ids=channel=={externalAccountID}` or `ids=channel==MINE` only when the authenticated token owns that channel. UniPost should prefer the stored channel ID when possible so the response is tied to the requested account.
7. Require `start_date` and `end_date`, or provide a safe default such as the last 28 complete days.
8. Respect YouTube Analytics data availability delays. Do not promise same-day complete data.
9. Add quota and rate-limit handling distinct from provider auth failure.
10. Document that V2 is owner-authorized analytics, not public channel analytics.

### V2 acceptance criteria

1. Newly reconnected YouTube accounts store `yt-analytics.readonly` in `social_accounts.scope`.
2. Old YouTube accounts without `yt-analytics.readonly` receive `NEEDS_RECONNECT` or equivalent actionable messaging on V2 endpoints.
3. V2 summary returns non-monetary channel-level metrics for a date range.
4. V2 trend returns daily time-series rows for supported metrics.
5. V2 top videos returns video-level rows where YouTube Analytics supports the requested report.
6. V2 docs explain supported metrics, required scopes, reconnect behavior, data delay, and unavailable monetary reports.
7. Tests cover scope detection, report query construction, empty report responses, auth failures, and quota/upstream failures.

## Rollout Plan

### V1 rollout

1. Implement and test YouTube Data API account metrics.
2. Update account metrics docs, analytics scope guides, `dashboard/src/lib/docs-ai-search-index.ts`, and `docs/sdk-api-coverage-matrix.md`.
3. Deploy to development.
4. Verify in the development API with a connected YouTube account.
5. Confirm the customer-reported endpoint no longer returns `501` in development.
6. Push through normal `dev` deployment flow after local validation.

### V2 rollout

1. Confirm Google Cloud YouTube Analytics API enablement.
2. Implement V2 endpoints behind normal API auth.
3. Update docs and reconnect guidance.
4. Reconnect at least one test YouTube account with the new scope set.
5. Verify V2 report retrieval in development.
6. Validate scope-denied and old-token behavior.
7. Release after development environment validation passes.

## Customer Communication

For V1:

> UniPost now supports basic YouTube account metrics through the existing account metrics endpoint. You can read subscriber count, public video count, and lifetime channel views for connected YouTube accounts that granted YouTube read access.

For V2:

> Rich YouTube Analytics reports are available through separate YouTube Analytics endpoints after the account reconnects with the YouTube Analytics readonly permission. Basic YouTube account metrics continue to work through the shared account metrics endpoint.

## Product Risks

- YouTube subscriber count is rounded by platform policy, so customers must not treat it as an exact subscriber total.
- YouTube `videoCount` only reflects public videos.
- High-frequency polling can consume YouTube Data API quota because V1 remains live-fetched. `channels.list` costs 1 quota unit and the default YouTube Data API project quota is 10,000 units per day, so V1 does not need server-side caching at current scale, but customers should still avoid tight polling loops.
- V2 requires existing accounts to reconnect with the already-configured `yt-analytics.readonly` scope.
- Existing accounts need reconnect for V2.

## Senior Developer Review

### Review summary

The V1 design is technically small and low risk because it uses the existing YouTube Data API access already requested by UniPost and fits the current `AccountMetricsAdapter` abstraction. It directly fixes the observed `501 NOT_SUPPORTED` path without changing the public endpoint contract.

V2 is correctly separated from V1. YouTube Analytics API reports have different semantics: required date ranges, delayed availability, richer metric/dimension compatibility, and additional OAuth consent. Folding those into the basic account metrics endpoint would make the endpoint harder to reason about and would force a reconnect for a problem V1 can solve without reconnect.

### Engineering notes

- The implementation must update both YouTube OAuth paths if V2 adds scopes. Today the legacy platform adapter and hosted Connect connector each carry their own YouTube scope constants.
- The V1 account metrics endpoint must not require `yt-analytics.readonly`; YouTube Analytics consent is only required for the V2 report endpoints.
- Token freshness needs explicit handling. The token refresh workers cover normal refresh, but the account metrics handler currently decrypts `acc.AccessToken` and calls the adapter without an inline expiry guard. Add a local refresh helper or reuse the analytics refresh pattern so a token expiring between worker ticks does not cause avoidable metrics failures.
- `channels.list` should request `part=statistics` only for V1. Do not request broader parts such as `snippet` or `brandingSettings` from the metrics call unless the product needs them.
- Use the stored `external_account_id` channel ID rather than `mine=true` for V1 metrics. This keeps the response tied to the requested social account and avoids surprises if a Google user has multiple channel contexts.
- Handle `hiddenSubscriberCount` deliberately. A hidden or absent subscriber count must not be presented as an exact zero without the platform-specific flag.
- Treat a zero-item `channels.list` response as stale account state requiring reconnect.
- Prefer typed reconnect-required adapter errors over adding a third platform-specific auth/scope string matcher in the metrics handler.
- Preserve unsupported behavior for platforms that still lack `AccountMetricsAdapter`.
- Consider improving integration log error-code extraction separately so `NOT_SUPPORTED` responses are not logged as `internal_error`.

### Recommended implementation order

1. V1 adapter and unit tests.
2. V1 docs and deployed development verification.
3. Optional logging classification cleanup.
4. V2 OAuth scope wiring and scope persistence.
5. V2 endpoint implementation and development verification.

### Review verdict

Proceed with V1 first, then implement V2 in the same development flow now that `yt-analytics.readonly` is already available in Google Auth Platform Data Access. Treat V2 as a separate endpoint family because it changes consent, reconnect behavior, and API semantics.
