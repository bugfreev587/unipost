# PRD — Facebook Reels publishing (Phase 3)

**Status:** Planning
**Owner:** TBD
**Target:** Phase 3 Facebook milestone
**Created:** 2026-04-24

---

## Problem

UniPost today publishes Facebook videos through `POST /{page_id}/videos` with `file_url=<public R2 URL>`. That endpoint does not support Reels — if the uploaded video is vertical (roughly 9:16 aspect ratio), Facebook silently reclassifies it as a Reel, assigns a `/reel/...` permalink, and leaves the upload stuck in `uploading/in_progress` forever because Reels require a different 3-phase endpoint that `file_url` pull cannot drive.

As of this PRD:

- Phase 1 mitigation has shipped: the validator accepts `platform_options.facebook.mediaType = "feed"` (default) and rejects `"reel"` with `facebook_reels_unsupported`. The adapter detects `/reel/` permalinks post-publish, cleans up the stuck video, and returns a clear error. The status worker fast-fails rows reclassified into the Reels pipeline after 10 minutes instead of waiting 12 hours.
- What's still missing: users genuinely cannot publish Reels through UniPost. This PRD covers wiring up the `/{page_id}/video_reels` 3-phase upload flow so `mediaType=reel` actually works.

## Goals

1. Ship `platform_options.facebook.mediaType = "reel"` as a fully working publish path.
2. Keep parity with the Feed-video flow (scheduling, inbox, analytics) wherever the Reels API allows.
3. Surface Reel progress in the dashboard with the same Uploading/Processing/Publishing phase breakdown users already see for Feed videos.
4. Leave the door open for future Reels-specific knobs (collaborators, content_tags, custom thumbnail offset) without another API break.

## Non-goals

- Automatic aspect-ratio detection for videos submitted as `mediaType=feed`. If we want feed / reel auto-routing in a later phase, that's a separate Media-service change (ffprobe + persist dimensions on media rows).
- Live Reels. Only on-demand Reel uploads are in scope.
- Instagram Reels cross-posting. IG already has its own `platform_options.instagram.mediaType=reels` path.
- Scheduled Reels older than 75 days out (Meta's native scheduling cap).

## API surface (final shape — partially shipped in Phase 1 mitigation)

```json
{
  "platform_posts": [
    {
      "account_id": "sa_facebook_1",
      "caption": "Teaser for tomorrow's drop.",
      "media_ids": ["med_vertical_video_1"],
      "platform_options": {
        "facebook": {
          "mediaType": "reel",
          "title": "Optional Reel title",
          "content_tags": ["bakery", "launch"],
          "place": "1234567890",
          "thumb_offset_ms": 2500
        }
      }
    }
  ]
}
```

- `mediaType` default remains `"feed"` — backwards compatible.
- New optional fields only apply when `mediaType = "reel"`; validator rejects them on `feed`.
- Validator removes the `facebook_reels_unsupported` rejection once Phase 3 ships.

## Facebook API reference

**Endpoint:** `/{page_id}/video_reels`

**Phase 1 — `start`:**
Request: `POST /{page_id}/video_reels?upload_phase=start&access_token=<PAGE_TOKEN>`
Response: `{ "video_id": "...", "upload_url": "...", "end_offset": <int> }`

**Phase 2 — `transfer`:**
Two options; UniPost picks (b) for parity with the current Feed-video flow.
- (a) `POST upload_url` with raw bytes (resumable/chunked)
- (b) `POST upload_url?upload_phase=transfer&access_token=<PAGE_TOKEN>&file_url=<public R2 URL>` — Meta pulls the file asynchronously, identical pattern to `/videos?file_url`.

**Phase 3 — `finish`:**
Request: `POST /{page_id}/video_reels?upload_phase=finish&access_token=<PAGE_TOKEN>&video_id=<id>&video_state=PUBLISHED` (plus optional metadata).
Response: `{ "success": true }` once the finalize call is accepted. Actual publish is still async — Meta processes and then exposes `permalink_url` on `/{video_id}`.

**Terminal states** (via `GET /{video_id}?fields=status,permalink_url,post_id`): same shape as Feed videos.

## Implementation plan

### 1. Validator (`internal/platform/validate.go`)

- Drop the `facebook_reels_unsupported` rejection for `mediaType=reel`.
- Keep the `invalid_facebook_media_type` guard.
- Add new guards specific to Reel mode:
  - Exactly one video (no photos, no mixed media, no multi-video).
  - Reject `link` alongside media (same as Feed).
  - Reject caption-only Reels (media required).
  - Reject `scheduled_at` + `mediaType=reel` in the first cut — native Reel scheduling is a follow-up (see §Open questions).
  - Validate `thumb_offset_ms` is a non-negative integer ≤ 60_000.

### 2. Adapter (`internal/platform/facebook.go`)

New internal method `postVideoReel(ctx, token, pageID, text, mediaURL, opts)`:

1. **Start**: `POST /{page_id}/video_reels?upload_phase=start` → `video_id`, `upload_url`.
2. **Transfer**: `POST upload_url?upload_phase=transfer&file_url=<staged R2 URL>`. Use the same `mediaProxy.UploadFromURL` staging that `postVideo` already does so Meta's async pull sees a stable URL.
3. **Finish**: `POST /{page_id}/video_reels?upload_phase=finish&video_id=&video_state=PUBLISHED&description=&title=&...`.
4. Re-use `CheckVideoStatus` unchanged (same `GET /{video_id}?fields=status,post_id,permalink_url` works for both Feed and Reel videos).
5. Re-use the existing 60s inline poll loop — if Reel finishes in <60s, return "published"; otherwise return `Status="processing"` and let the worker flip it.

Dispatch in `Post()`:

```go
mediaType := optString(opts, "mediaType")
if mediaType == "" { mediaType = optString(opts, "media_type") }
switch {
case hasMedia && kind == MediaKindVideo && mediaType == "reel":
    return a.postVideoReel(...)
case hasMedia && kind == MediaKindVideo:
    return a.postVideo(...)  // existing feed path
...
}
```

Keep `isReelPermalink` / `tryDeleteVideo` / `phaseHasError` — they still help catch Feed-path reclassifications.

### 3. Worker (`internal/worker/facebook_video_status.go`)

No code change required. The worker already polls via `CheckVideoStatus` and flips `ready → published` / `error → failed`. The only gotcha is the Reel-reclassification fast-fail branch added in Phase 1; a legitimately-requested Reel has `permalink_url = "/reel/..."` **on purpose**, so that fast-fail must NOT fire for rows that the adapter itself put on the Reels path.

**Fix:** persist the chosen mediaType on the `social_post_results` row (new column `fb_media_type TEXT NULL`), and skip the reel-reclassified branch when `fb_media_type = 'reel'`.

### 4. Database

Add column `social_post_results.fb_media_type TEXT NULL` via a new migration.
- Backfill: leave NULL for existing rows — worker only consults this column; NULL preserves today's behavior.
- `social_posts.go` writes the value on create based on the post's `platform_options.facebook.mediaType`.
- `ListFacebookVideosAwaitingStatus` SELECTs the column so the worker can branch.

### 5. Dashboard

- Composer (`platform-fields/facebook-fields.tsx`):
  - Add a media-type toggle shown only when a video is attached: `Feed video` (default) / `Reel`.
  - Reel mode hides the `link` field (reject-on-submit path also exists).
  - Reel mode shows the optional extras: Title, Content Tags, Place ID, Thumbnail offset.
- Post detail page: label the post surface as "Reel" in the platform badge when `fb_media_type='reel'`.
- Inbox: Reels comments land on the same `fb_comment` source — no inbox change.

### 6. Documentation

Update `/docs/platforms/facebook`:
- Capabilities matrix: add a Reels row (Supported once Phase 3 ships).
- Field requirements: document `platform_options.facebook.mediaType`.
- API examples: add a Reel example with media_ids and the minimum metadata.

### 7. Tests

- `internal/platform/facebook_test.go`: mock Meta's `video_reels` endpoint for happy path + each phase-level failure.
- `internal/platform/validate_test.go`: accept `mediaType=reel` (drop the current reject test), exercise the new field-level guards.
- `internal/worker/facebook_video_status_test.go`: add a case where `permalink_url="/reel/..."` + `fb_media_type="reel"` does NOT trigger the fast-fail branch.
- End-to-end smoke: drop a known vertical MP4 into a Reel publish, assert it lands on a Page with a `/reel/...` permalink and a `post_id`.

## Rollout

1. Migration to add `fb_media_type` — behind a feature flag `FEATURE_FACEBOOK_REELS` off by default.
2. Deploy the adapter + worker code changes. Flag off = validator still rejects `mediaType=reel`; no behavior change for users.
3. Flip the flag on in a staging workspace, run the e2e smoke test.
4. Flip on in prod; announce in release notes and update the platform docs page.

## Open questions

- **Native Reel scheduling.** The Graph API supports `video_state=SCHEDULED` + `scheduled_publish_time`. Do we wire this in Phase 3, or leave scheduled Reels blocked alongside scheduled Feed videos (existing `facebook_scheduled_media_unsupported`) and unlock both together?
- **Content category for Reels.** Feed videos accept `content_category` to influence distribution. Reels have their own taxonomy (short-form content, music, etc.). Do we expose this as a knob or hide it?
- **Thumbnail upload vs thumb_offset.** Meta allows both a custom thumbnail file and a frame offset. File-upload requires a second multipart call; offset is just a query param. Start with offset-only; file-upload can ship in a Phase 3.5 if users ask.
- **Collaborator tags.** Meta supports `@mentions` of other Pages as Reels collaborators. Nice-to-have, not blocking for v1.

## What ships in Phase 1 mitigation (already done)

For reference — this PRD layers on top of these shipped items:

- `platform_options.facebook.mediaType` accepted on the API surface; `reel` rejected with `facebook_reels_unsupported`.
- Adapter detects `/reel/` permalinks mid-publish, deletes the stuck video, returns a clear error.
- Adapter handles `upload_failed` / `expired` and phase-level errors as terminal failures (no more 12h wait).
- Worker fast-fails Reel-reclassified rows at 10 minutes with a user-readable message.
