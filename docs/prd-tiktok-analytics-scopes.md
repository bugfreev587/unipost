# PRD - TikTok analytics scopes

**Status:** Planning
**Owner:** TBD
**Target:** TikTok App Review revision for analytics scopes
**Created:** 2026-05-14

---

## Problem

UniPost's TikTok integration is approved for `user.info.basic`, `video.publish`, and `video.upload`, and the product can already connect TikTok accounts and publish TikTok content.

The next App Review revision requests:

- `user.info.profile`
- `user.info.stats`
- `video.list`

These scopes are needed to support TikTok analytics in the dashboard and API, but the current codebase only partially supports the required behavior:

- `video.list` is already assumed by the TikTok post analytics adapter, but it is not requested during OAuth.
- `user.info.profile` fields are not fetched or exposed.
- `user.info.stats` fields are not fetched or exposed for TikTok account metrics.
- There is no first-class TikTok public video list endpoint or dashboard surface.

Without this work, newly approved scopes would not be demonstrable end to end in the product, and existing TikTok accounts would continue to lack the scopes needed for analytics until they reconnect.

## Goals

1. Request the full TikTok scope set in OAuth once App Review approves the new scopes.
2. Store the granted TikTok scope set on `social_accounts.scope`.
3. Fetch and expose TikTok profile fields unlocked by `user.info.profile`.
4. Fetch and expose TikTok account metrics unlocked by `user.info.stats`.
5. Fetch and expose a connected user's public TikTok videos unlocked by `video.list`.
6. Make TikTok post analytics work for newly reconnected accounts.
7. Provide clear reconnect messaging for TikTok accounts connected before the analytics scopes were granted.
8. Create a demo-ready dashboard flow that clearly shows all three new scopes.

## Non-goals

- TikTok Business / Ads analytics
- follower demographics
- profile views, reach, impressions, watch time, average watch time, completion rate, or traffic source breakdowns
- comment management or inbox support
- analytics for videos not owned by the connected TikTok account
- scraping TikTok pages outside official APIs
- silent scope upgrade for existing tokens; users must reconnect

## Current Codebase Findings

### Already present

- TikTok OAuth connect and callback flow exists in `api/internal/platform/tiktok.go` and `api/internal/handler/oauth.go`.
- TikTok publishing exists for video and photo paths.
- TikTok `creator_info` is exposed through `GET /v1/accounts/{id}/tiktok/creator-info` and dashboard-nested equivalent.
- TikTok post-level analytics is partially implemented through `TikTokAdapter.GetAnalytics`.
- The analytics worker and `GET /v1/posts/{id}/analytics` already call platform adapters that implement `AnalyticsAdapter`.
- A generic account metrics endpoint exists at `GET /v1/accounts/{id}/metrics`, but TikTok does not implement `AccountMetricsAdapter`.

### Missing or incomplete

- `DefaultOAuthConfig.Scopes` for TikTok currently requests only:

```go
[]string{"video.publish", "video.upload", "user.info.basic"}
```

- `GetAuthURL` hardcodes the same old scope string:

```text
video.publish,video.upload,user.info.basic
```

- `getUserInfo` only asks for:

```text
open_id,display_name,avatar_url
```

- No code reads these `user.info.profile` fields:

```text
profile_web_link, profile_deep_link, bio_description, is_verified, username
```

- No code reads these `user.info.stats` fields:

```text
follower_count, following_count, likes_count, video_count
```

- `video.list` is consumed indirectly by `GetAnalytics` via `/v2/video/query`, but the OAuth flow does not request it yet.
- There is no endpoint for listing the user's public TikTok videos.
- Existing connected accounts will not gain the new scopes until they disconnect/reconnect.

## Product Requirements

### 1. OAuth scope expansion

After TikTok App Review approves the new scopes, TikTok OAuth must request:

```text
user.info.basic,user.info.profile,user.info.stats,video.list,video.publish,video.upload
```

Implementation requirements:

- Update both `DefaultOAuthConfig.Scopes` and `GetAuthURL` in the TikTok adapter.
- Keep the slice and serialized query parameter in one source of truth if possible.
- Add a unit test that locks the exact TikTok scope set.
- Ensure scope ordering matches the App Review submission to make debugging easier.
- Persist the granted scopes to `social_accounts.scope` for new TikTok OAuth connections.

Acceptance criteria:

- The TikTok auth URL contains all six scopes.
- The OAuth callback stores all six scopes when TikTok grants them.
- If TikTok returns a reduced scope set, the account metadata or health endpoint can surface the missing scopes.

### 2. TikTok profile information

UniPost must fetch and expose profile fields unlocked by `user.info.profile`.

Required TikTok fields:

- `username`
- `profile_web_link`
- `profile_deep_link`
- `bio_description`
- `is_verified`

Recommended API shape:

```http
GET /v1/accounts/{account_id}/tiktok/profile
GET /v1/profiles/{profile_id}/accounts/{account_id}/tiktok/profile
```

Example response:

```json
{
  "social_account_id": "sa_tiktok_123",
  "platform": "tiktok",
  "open_id": "tiktok_open_id",
  "display_name": "UniPost Demo",
  "avatar_url": "https://...",
  "username": "unipost_demo",
  "profile_web_link": "https://www.tiktok.com/@unipost_demo",
  "profile_deep_link": "snssdk1233://...",
  "bio_description": "Demo creator account",
  "is_verified": false,
  "fetched_at": "2026-05-14T12:00:00Z"
}
```

Implementation requirements:

- Extend the TikTok adapter with `FetchUserInfo` or equivalent.
- Use `/v2/user/info/` with the combined profile field list.
- Redact access tokens from logs.
- Return a reconnect-required error when TikTok rejects the token due to missing scopes.
- Store useful stable fields in `social_accounts.metadata` on new connections:
  - `open_id`
  - `display_name`
  - `username`
  - `profile_web_link`
  - `is_verified`
  - `granted_scopes`

Acceptance criteria:

- Dashboard can display username, bio, profile link, and verification status for a newly connected TikTok account.
- API returns a clear `NEEDS_RECONNECT` or `MISSING_SCOPE` error for old accounts.

### 3. TikTok account statistics

UniPost must expose account-level TikTok stats unlocked by `user.info.stats`.

Required TikTok fields:

- `follower_count`
- `following_count`
- `likes_count`
- `video_count`

Implementation requirements:

- Implement `platform.AccountMetricsAdapter` on `TikTokAdapter`.
- Map TikTok fields into the existing normalized shape:
  - `follower_count` -> `AccountMetrics.FollowerCount`
  - `following_count` -> `AccountMetrics.FollowingCount`
  - `video_count` -> `AccountMetrics.PostCount`
  - `likes_count` -> `PlatformSpecific.likes_count`
- Keep the existing `GET /v1/accounts/{id}/metrics` endpoint as the primary API surface.
- Add docs showing TikTok support on the account metrics endpoint.

Acceptance criteria:

- `GET /v1/accounts/{id}/metrics` returns TikTok follower, following, likes, and video counts.
- Existing old-scope accounts return actionable reconnect messaging.
- Dashboard account cards or analytics overview can show TikTok account stats.

### 4. TikTok public video list

UniPost must expose the connected user's public TikTok videos unlocked by `video.list`.

Recommended API shape:

```http
GET /v1/accounts/{account_id}/tiktok/videos?cursor={cursor}&limit=20
GET /v1/profiles/{profile_id}/accounts/{account_id}/tiktok/videos?cursor={cursor}&limit=20
```

Example response:

```json
{
  "videos": [
    {
      "id": "7350123456789012345",
      "title": "Launch demo",
      "cover_image_url": "https://...",
      "share_url": "https://www.tiktok.com/@unipost_demo/video/7350123456789012345",
      "create_time": 1712345678,
      "view_count": 1200,
      "like_count": 87,
      "comment_count": 9,
      "share_count": 4
    }
  ],
  "cursor": "next_cursor",
  "has_more": true,
  "fetched_at": "2026-05-14T12:00:00Z"
}
```

Implementation requirements:

- Add a TikTok adapter method for listing videos.
- Use official TikTok video list/query endpoints only.
- Support pagination cursor and bounded limit.
- Include only fields that TikTok exposes for `video.list`.
- Return clear reconnect/missing-scope errors for old accounts.
- Do not persist the full video list in phase 1 unless product needs caching.

Acceptance criteria:

- Dashboard can show a TikTok Videos tab with public videos from the connected account.
- API users can list public videos for one TikTok social account.
- The App Review demo can clearly show `video.list` being used.

### 5. Post-level TikTok analytics

UniPost already has `TikTokAdapter.GetAnalytics`, but it depends on `video.list` and therefore only works after reconnect.

Implementation requirements:

- Keep the existing publish-status-to-video-id resolution flow.
- Confirm `/v2/video/query/` works with newly granted `video.list`.
- Include `tiktok_video_id` in `platform_specific`.
- Add tests for missing-scope error handling.
- Update UI copy when analytics refresh fails because the account needs reconnect.

Acceptance criteria:

- Newly reconnected TikTok accounts can refresh post analytics.
- Old accounts show a reconnect CTA instead of a generic analytics failure.

## Dashboard Requirements

### Analytics information architecture

TikTok analytics should be a platform-specific analytics drilldown, not a permanent top-level peer of the existing generic analytics pages.

Final dashboard navigation:

```text
Analytics
  Posts
  Platforms
  API
```

Route structure:

```text
/projects/{profile_id}/analytics
/projects/{profile_id}/analytics/platforms
/projects/{profile_id}/analytics/platforms/tiktok
/projects/{profile_id}/analytics/api
```

Rationale:

- `Posts` answers: "How are all published posts performing across platforms?"
- `Platforms` answers: "What platform-specific analytics and account data does each provider expose?"
- `API` answers: "How is my UniPost API usage performing?"

TikTok is not a separate analytics category; it is one implementation of platform-specific analytics. Keeping it under `Analytics -> Platforms -> TikTok` leaves room for future platform drilldowns such as Facebook Page Insights, YouTube channel/video stats, Instagram account insights, and X account metrics without expanding the sidebar into one item per platform.

### Relationship to generic analytics

The TikTok platform page must be integrated with, not separate from, the generic analytics system:

- `Analytics -> Posts` remains the source of truth for cross-platform post-level analytics.
- TikTok rows in `Posts` continue to show `video_views`, `likes`, `comments`, and `shares`.
- TikTok rows show `N/A` for metrics TikTok does not expose through the approved scopes, such as impressions, reach, saves, and clicks.
- The TikTok entry in `Analytics -> Platforms` links into the TikTok detail page.
- TikTok rows or the TikTok by-platform summary in `Posts` should include a "View TikTok analytics" drilldown link.
- `Analytics -> Platforms -> TikTok` embeds a "UniPost-published TikTok posts" section that reuses the same `post_analytics` data displayed in `Posts`.

### Platform index page

`Analytics -> Platforms` should list connected analytics-capable platforms as cards or rows.

Each platform entry should show:

- platform name and connected account count
- analytics availability state
- required scope readiness
- last successful refresh time
- key account-level metric preview when available
- drilldown link

For TikTok, the card should show:

- connected TikTok account name
- scope readiness for `user.info.profile`, `user.info.stats`, and `video.list`
- follower count
- video count
- last refresh time
- reconnect CTA when any required scope is missing

### TikTok platform detail page

`Analytics -> Platforms -> TikTok` should be the primary dashboard surface for the three new TikTok scopes.

The page should contain:

- connected account selector when multiple TikTok accounts exist
- scope readiness banner
- profile panel powered by `user.info.profile`
- account stats cards powered by `user.info.stats`
- public videos table powered by `video.list`
- UniPost-published TikTok posts section powered by existing post-level analytics
- reconnect CTA for old accounts missing one or more analytics scopes

Temporary local preview:

```text
/tools/tiktok-analytics
```

This public route exists only to preview and record the sample UI without Clerk authentication. It should not be treated as the final product location.

### Account / integration page

Show:

- connected TikTok username
- display name and avatar
- profile link
- bio
- verified badge/status
- follower count
- following count
- likes count
- video count
- granted scopes status

If any requested analytics scope is missing, show:

```text
Reconnect TikTok to enable analytics.
```

### TikTok platform analytics page

Show:

- TikTok account stats in the overview where applicable
- TikTok public videos in a video list/table
- TikTok post-level metrics for UniPost-published videos:
  - views
  - likes
  - comments
  - shares

Display `N/A` for metrics TikTok does not expose through the approved scopes.

### App Review demo readiness

The final UI must let a reviewer see:

1. OAuth requests the three new scopes.
2. `user.info.profile` data appears in UniPost.
3. `user.info.stats` data appears in UniPost.
4. `video.list` data appears in UniPost.
5. Existing posting scopes still work through the already approved flow.

## API and SDK Updates

### API

Add:

- `GET /v1/accounts/{id}/tiktok/profile`
- `GET /v1/accounts/{id}/tiktok/videos`
- dashboard-nested equivalents under `/v1/profiles/{profileID}/accounts/{accountID}/...`

Extend:

- `GET /v1/accounts/{id}/metrics` to support TikTok.
- `GET /v1/accounts/{id}/health` or account list responses with scope health if feasible.

### SDKs

Add methods to JS, Python, Go, and Java SDKs:

- `accounts.tiktokProfile(accountId)`
- `accounts.tiktokVideos(accountId, options)`
- ensure existing `accounts.metrics(accountId)` examples include TikTok

## Data Model

Phase 1 can avoid new tables.

Use:

- `social_accounts.scope` for granted scopes.
- `social_accounts.metadata` for stable profile snapshot fields.
- `post_analytics.platform_specific` for TikTok video IDs and raw per-video extras.

Add tables later only if UniPost needs historical account-level trend snapshots or cached TikTok video inventory.

## Rollout Plan

1. Keep current production OAuth scopes unchanged until TikTok approves the revision.
2. Merge code behind a feature flag or environment toggle:

```text
TIKTOK_ANALYTICS_SCOPES_ENABLED=true
```

3. Enable the expanded OAuth scope set only after App Review approval.
4. Ask internal/test TikTok accounts to disconnect and reconnect.
5. Validate profile, stats, video list, and post analytics against real approved tokens.
6. Update docs and dashboard messaging.
7. Announce that TikTok analytics requires reconnecting existing TikTok accounts.

## Test Plan

### Unit tests

- TikTok auth URL contains all approved scopes when the feature flag is enabled.
- TikTok auth URL keeps legacy scopes when the feature flag is disabled.
- TikTok user info parser handles profile and stats fields.
- TikTok account metrics maps `likes_count` to `platform_specific.likes_count`.
- TikTok video list parser handles pagination and empty lists.
- Missing-scope upstream errors normalize to reconnect/missing-scope errors.

### Integration tests

- OAuth connect stores granted scopes.
- `GET /v1/accounts/{id}/metrics` returns TikTok stats for an account with the new scopes.
- `GET /v1/accounts/{id}/tiktok/profile` returns profile fields.
- `GET /v1/accounts/{id}/tiktok/videos` returns public videos.
- `GET /v1/posts/{id}/analytics?refresh=true` returns TikTok post analytics after reconnect.

### Manual App Review test

- Record a demo from production domain.
- Connect or reconnect TikTok.
- Show profile fields.
- Show account stats.
- Show public video list.
- Show previously approved publish/upload flow via existing demo.

## Risks

- TikTok may reject expanded scopes in sandbox until App Review approval, so the rollout must keep old scopes available until production access is granted.
- Existing TikTok connections cannot be silently upgraded; reconnect is required.
- TikTok may expose fewer video fields than the UI wants; the UI must only claim fields that are actually returned by official APIs.
- Account stats are current snapshots, not historical trends, unless a future caching table is added.
- Reviewer may expect old and new scope demos in one upload; keep old approved demo videos and add the new analytics demo.

## Open Questions

1. Should TikTok analytics scope expansion be controlled by env flag or deployed only after approval?
2. Should the public video list be API-only in phase 1, or also visible in the dashboard for demo readiness?
3. Should account stats be cached to support historical trend charts, or fetched live only?
4. Should TikTok profile and account stats be returned as one endpoint to simplify the demo?
5. Should old TikTok accounts be proactively marked `reconnect_required` if `social_accounts.scope` lacks the new scopes?
