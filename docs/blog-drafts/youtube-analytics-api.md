# YouTube Analytics API for Apps That Publish Video

Status: Draft for PM review

UniPost now gives product teams a cleaner way to show YouTube performance without building a separate Google reporting stack. The implementation has two layers because YouTube exposes two different kinds of data: basic channel statistics through the YouTube Data API, and richer owner-authorized reports through the YouTube Analytics API.

## The short version

V1 adds basic account metrics to the existing UniPost account metrics endpoint:

```bash
curl "https://api.unipost.dev/v1/accounts/{account_id}/metrics" \
  -H "Authorization: Bearer $UNIPOST_API_KEY"
```

For connected YouTube accounts, V1 returns subscriber count, public video count, and lifetime channel views. It uses `youtube.readonly`, so existing YouTube connections that already granted read access do not need the Analytics API scope just to show this basic channel snapshot.

V2 adds YouTube Analytics API reports for date-ranged performance:

```bash
curl "https://api.unipost.dev/v1/accounts/{account_id}/youtube/analytics/summary" \
  -H "Authorization: Bearer $UNIPOST_API_KEY"

curl "https://api.unipost.dev/v1/accounts/{account_id}/youtube/analytics/trend" \
  -H "Authorization: Bearer $UNIPOST_API_KEY"

curl "https://api.unipost.dev/v1/accounts/{account_id}/youtube/analytics/videos?limit=25" \
  -H "Authorization: Bearer $UNIPOST_API_KEY"
```

V2 requires `yt-analytics.readonly`. It does not include monetary or revenue metrics, and it does not require `yt-analytics-monetary.readonly`.

## Why split V1 and V2?

Basic channel metrics and Analytics reports answer different product questions.

V1 is for simple account cards: how many subscribers does this channel have, how many public videos are on it, and how many lifetime views has YouTube reported for the channel? That belongs in the same normalized account metrics contract UniPost already uses for other platforms.

V2 is for reporting screens: how many views happened during a date range, how much watch time did the channel receive, what was the daily trend, and which videos led performance? That belongs in a YouTube-specific analytics surface because it has date range semantics, YouTube Analytics API metric compatibility rules, and a separate OAuth permission.

## What the dashboard shows

The dashboard now has Analytics - Platforms - YouTube. It combines:

- Basic channel metrics from V1.
- Summary totals from V2.
- Daily trend rows from V2.
- Top video rows from V2.
- Reconnect guidance when an account has `youtube.readonly` but does not yet have `yt-analytics.readonly`.

This means customers can still see the basic YouTube channel snapshot even before they reconnect for richer Analytics reports.

## What developers should build

Start with V1 if your product only needs an account overview. Read `data.follower_count`, `data.post_count`, and `data.platform_specific.view_count`. YouTube may hide or round subscriber counts depending on channel settings, so the response includes platform-specific flags for that state.

Use V2 when you need time-based reporting. The summary endpoint is best for KPI cards, the trend endpoint is best for charts, and the videos endpoint is best for a top-content table. Existing accounts connected before `yt-analytics.readonly` was granted should reconnect before calling V2.

## FAQ

### Do I need a new UniPost API scope?

No. UniPost API keys do not need a separate API scope for YouTube Analytics. The requirement is on the Google OAuth permission stored with the connected YouTube account.

### Do existing YouTube accounts need to reconnect?

For V1, usually no, as long as the account already granted `youtube.readonly` and still resolves to the expected channel. For V2, accounts connected before `yt-analytics.readonly` was added need to reconnect.

### Does this include revenue, ad performance, or monetary reports?

No. V2 intentionally avoids monetary reports. It does not include monetary or revenue metrics and does not request `yt-analytics-monetary.readonly`.

### Is YouTube data real-time?

The basic channel snapshot is fetched live from the YouTube Data API. YouTube Analytics reports can have reporting delay, so date-ranged V2 responses should be treated as Analytics reports, not real-time counters.
