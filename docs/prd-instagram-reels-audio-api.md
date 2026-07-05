# PRD - Instagram Reels Audio API

**Status:** Planning
**Owner:** Product / API
**Target:** Instagram Reels audio-selection milestone
**Created:** 2026-06-25

---

## Problem

UniPost can publish final Instagram videos and Reels when the uploaded video already contains its desired audio. Customers are now asking whether UniPost can add platform music automatically, especially for Instagram.

TikTok and Instagram differ in an important way:

- TikTok's normal Content Posting API does not expose a general music-library picker for uploaded videos. TikTok exposes `auto_add_music` only for photo-style posts.
- Meta now exposes an official **Instagram Audio API** that can search or retrieve audio assets and attach an `audio_id` to a Reel at creation time.

UniPost does not currently expose the Instagram Audio API. The Instagram adapter creates media containers with caption, media type, video URL, cover, and publish settings, but it does not search Instagram audio or send `audio_configuration` when creating Reels.

Without this work, UniPost's answer to Instagram music requests is weaker than the underlying Meta API allows. Customers who want authorized Instagram audio must either publish manually in Instagram or pre-render audio into the final video before using UniPost.

## Goals

1. Add first-class UniPost support for attaching Meta-authorized audio to Instagram Reels.
2. Let API customers search or retrieve Instagram audio assets before creating a post.
3. Let customers publish Instagram Reels with a selected `audio_id`, `audio_volume`, and `video_volume`.
4. Keep the feature scoped to Instagram Reels, where Meta supports `audio_configuration`.
5. Preserve the current final-video publishing flow for customers who already embed audio in their videos.
6. Make capability, validation, docs, and error messages explicit so customers do not confuse this with TikTok library music support.

## Non-goals

- TikTok music-library selection.
- TikTok video auto-music beyond the existing TikTok `auto_add_music` behavior for photo posts.
- Generic "auto-add music to all routes" behavior.
- Server-side audio/video muxing or video rendering.
- Uploading customer-owned audio files as Instagram audio assets.
- Attaching audio to Instagram feed images, feed carousels, or stories.
- Reel preview with attached audio before publishing; Meta documents that previews with attached audio are not supported.
- Ads audio replacement in the first release.
- Automatic music selection without a customer-selected or API-selected `audio_id`.

## Product Decision

Ship this as **Instagram Reels Audio**, not as a generic music feature.

The first release should support explicit audio selection:

1. Customer searches Instagram audio through UniPost.
2. Customer selects an `audio_id`.
3. Customer creates an Instagram Reel with `platform_options.instagram.audio`.

Do not add a broad `auto_add_music` option for Instagram in v1. Meta can return trending audio when no search query is provided, but UniPost should first make the selection explicit and auditable. A future phase can add a curated "use trending music" helper once eligibility, regional availability, and customer expectations are better understood.

## Official Platform Assumptions

This PRD assumes Meta's public documentation as of June 25, 2026:

- Instagram Audio API can search and retrieve audio assets, including original sounds and music.
- Audio search requires `audio_type` of `original_sound` or `music`.
- Audio can be attached to Reels at creation time through `audio_configuration`.
- The API returns audio authorized for third-party use, but available selection may differ from the native Instagram app.
- The API is available only for Instagram API with Facebook Login, not Instagram API with Instagram Login.
- Reel preview with attached audio is not supported.
- If no search query is provided, the audio search endpoint can return trending audio.

Primary source: https://developers.facebook.com/docs/instagram-platform/content-publishing/audio-api/

## Current Codebase Findings

### Already present

- Instagram OAuth and publishing are implemented in `api/internal/platform/instagram.go`.
- Instagram publishing supports feed, Reels, and stories through `platform_options.instagram.mediaType`.
- The dashboard composer exposes an Instagram media type selector in `dashboard/src/components/posts/create-post/platform-fields/instagram-fields.tsx`.
- The public API already accepts per-platform options through `platform_posts[].platform_options`.
- Media upload, validation, scheduling, queueing, and publish result tracking are already shared across platforms.

### Missing or incomplete

- No UniPost endpoint searches Instagram audio assets.
- No UniPost endpoint retrieves Instagram audio metadata by `audio_id`.
- The Instagram adapter does not send `audio_configuration` when creating Reel containers.
- The Instagram dashboard composer has no audio selector, audio search, volume controls, or warning about the lack of attached-audio preview.
- The capability map does not advertise Instagram Reels audio support.
- Existing Instagram OAuth appears to use the Instagram Login path; Meta documents Audio API support only for Instagram API with Facebook Login. Implementation must verify whether UniPost can use the existing connected accounts or needs a Facebook Login-based Instagram connection path.

## User Stories

1. As an API user, I can search for Instagram music or original sounds available to a connected account.
2. As an API user, I can retrieve metadata for a known Instagram `audio_id`.
3. As an API user, I can publish an Instagram Reel with a selected audio asset.
4. As an API user, I can set music and original-video volume levels when attaching audio.
5. As a dashboard user, I can select an Instagram audio track when composing a Reel.
6. As a user, I receive a clear validation error if I try to attach Instagram audio to a non-Reel post.
7. As a user, I receive a clear error if my Instagram account connection path or permissions do not support the Audio API.
8. As a support operator, I can distinguish "Meta audio unavailable" from "UniPost validation rejected this request."

## Scope

### Phase 1: API foundation

- Add Instagram audio search endpoint.
- Add Instagram audio metadata endpoint.
- Add publish-time `audio_configuration` support for Instagram Reels.
- Add backend validation and docs.
- Add capability metadata indicating that Instagram supports Reels audio by selected `audio_id`.

### Phase 2: Dashboard composer

- Add an Instagram Reels audio picker to the composer.
- Support `music` and `original_sound` searches.
- Show audio title, artist/creator, duration, artwork when available, and preview link when available.
- Add audio and video volume controls.
- Warn that attached-audio preview is not supported before publish.

### Phase 3: Advanced helpers

- "Trending audio" mode that lists trending music or original sounds when no query is provided.
- Saved/recent audio selections.
- Ads audio-replacement discovery if UniPost later supports Instagram Reels ads workflows.

## API Surface

### Search Instagram audio

```http
GET /v1/accounts/{account_id}/instagram/audio?audio_type=music&search_query=birthday&limit=20
```

Parameters:

- `audio_type`: required enum, `music` or `original_sound`.
- `search_query`: optional string. When omitted, UniPost may return Meta's trending audio results.
- `limit`: optional integer, default 20.
- `cursor`: optional pagination cursor if Meta exposes paging for the result set.

Example response:

```json
{
  "account_id": "sa_instagram_123",
  "platform": "instagram",
  "audio_type": "music",
  "audio": [
    {
      "audio_id": "587784541076604",
      "audio_type": "music",
      "title": "Birthday Wish",
      "display_artist": "Shuba",
      "duration_in_ms": 153760,
      "cover_artwork_thumbnail_url": "https://scontent.example/cover.jpg",
      "download_url": "https://scontent.example/preview.mp3",
      "on_platform_audio_preview_link": "https://www.instagram.com/...",
      "is_ads_eligible": false
    }
  ],
  "next_cursor": null
}
```

### Get Instagram audio metadata

```http
GET /v1/accounts/{account_id}/instagram/audio/{audio_id}
```

Example response:

```json
{
  "audio_id": "587784541076604",
  "audio_type": "music",
  "title": "Birthday Wish",
  "display_artist": "Shuba",
  "duration_in_ms": 153760,
  "cover_artwork_thumbnail_url": "https://scontent.example/cover.jpg",
  "download_url": "https://scontent.example/preview.mp3",
  "on_platform_audio_preview_link": "https://www.instagram.com/...",
  "is_ads_eligible": false,
  "fetched_at": "2026-06-25T12:00:00Z"
}
```

### Publish an Instagram Reel with audio

Use the existing `POST /v1/posts` request shape.

```json
{
  "platform_posts": [
    {
      "account_id": "sa_instagram_123",
      "caption": "Launch week recap",
      "media_ids": ["media_reel_video_123"],
      "platform_options": {
        "instagram": {
          "mediaType": "reels",
          "audio": {
            "audio_id": "587784541076604",
            "audio_volume": 80,
            "video_volume": 50
          }
        }
      }
    }
  ]
}
```

Recommended request aliases:

- Accept `mediaType` and `media_type` for existing consistency.
- Prefer `audio` in UniPost request bodies.
- Map `platform_options.instagram.audio` to Meta's `audio_configuration`.

Field behavior:

- `audio.audio_id` is required when `audio` is present.
- `audio.audio_volume` is optional integer, 0 through 100, default 100.
- `audio.video_volume` is optional integer, 0 through 100, default 100.
- `audio` is valid only when `mediaType` resolves to `reels`.
- `audio` requires exactly one video media item.
- `audio` is rejected for Instagram feed images, feed carousels, stories, and non-Instagram platforms.

## Backend Requirements

### 1. Connection path compatibility

Before implementation, verify whether the currently connected Instagram accounts use a Meta API path that can call `/ig_audio`.

If the current Instagram Login flow cannot call Audio API:

- Add a clear product decision: either introduce a Facebook Login-based Instagram connection path, or mark Audio API as available only for compatible account connections.
- Return `unsupported_connection_type` or `reconnect_required` when an account cannot use Audio API.
- Document which connection method is required.

### 2. Adapter additions

Extend the Instagram adapter with:

- `SearchAudio(ctx, accessToken, igUserID, audioType, searchQuery, cursor, limit)`.
- `GetAudio(ctx, accessToken, igUserID, audioID)`.
- Publish-time mapping from `platform_options.instagram.audio` to Meta `audio_configuration`.

The publish adapter should send `audio_configuration` only on Reel container creation.

### 3. Validation

Add validation rules for `platform_options.instagram.audio`:

- `audio_id` must be non-empty.
- `audio_volume` and `video_volume` must be integers from 0 to 100.
- `audio` requires `mediaType=reels`.
- `audio` requires exactly one video.
- `audio` cannot be used with images, stories, feed posts, or carousel posts.

Recommended error codes:

- `instagram_audio_requires_reel`
- `instagram_audio_requires_video`
- `instagram_audio_id_required`
- `invalid_instagram_audio_volume`
- `instagram_audio_connection_unsupported`
- `instagram_audio_unavailable`

### 4. Capability map

Add an Instagram capability detail such as:

```json
{
  "instagram": {
    "audio": {
      "reels": {
        "supported": true,
        "selection": "audio_id",
        "search_endpoint": "/v1/accounts/:account_id/instagram/audio",
        "supports_auto_pick": false,
        "supports_preview_before_publish": false,
        "requires_connection_type": "facebook_login"
      }
    }
  }
}
```

If the existing capability model cannot represent nested audio features, add this information to docs first and make a follow-up capability-schema PRD.

### 5. Docs

Update public docs for:

- Create Post endpoint.
- Instagram platform page.
- Account-specific Instagram audio endpoints.
- Platform requirements matrix.

Docs must explicitly state:

- This is for Instagram Reels only.
- Audio availability may differ from the native Instagram app.
- UniPost cannot preview the final Reel with attached audio before publish.
- TikTok library music selection remains unsupported because TikTok does not expose that through the normal posting API.

## Dashboard Requirements

Phase 2 should add a Reels-only audio section to the Instagram composer.

Behavior:

- Hidden unless Instagram is selected and `mediaType=reels`.
- Search input with audio type selector: `Music` / `Original sound`.
- Result list with title, artist or creator, duration, artwork, and preview link when Meta returns one.
- Selected audio chip/card with remove action.
- Audio volume and original video volume sliders.
- Notice: "Instagram does not support previewing the final Reel with attached audio before publish."
- Clear disabled state when the account connection does not support Audio API.

Do not show this control for TikTok, YouTube, Facebook, Instagram stories, or Instagram feed posts.

## Error Handling

Map Meta errors into actionable UniPost errors:

- Missing permission or unsupported connection path -> reconnect or connection-type guidance.
- Audio no longer available -> ask user to choose another track.
- Region or account restriction -> explain that Meta controls audio availability.
- Meta transient error -> retryable platform error.
- Invalid `audio_id` -> validation or provider error depending on when detected.

Logs must not include access tokens, preview download URLs with sensitive signatures, or full provider responses if they contain signed URLs.

## Rollout

1. Ship backend audio search and metadata endpoints behind internal capability/docs.
2. Add publish-time `audio_configuration` support for API requests.
3. Test with a compatible Instagram account using Facebook Login-based access.
4. Update docs and support messaging.
5. Add dashboard composer support.
6. Announce as "Instagram Reels audio selection" rather than "auto music."

## Acceptance Criteria

Phase 1 is complete when:

- `GET /v1/accounts/{id}/instagram/audio` returns Meta-authorized audio results for a compatible account.
- `GET /v1/accounts/{id}/instagram/audio/{audio_id}` returns metadata for a compatible account.
- `POST /v1/posts` can publish an Instagram Reel with `platform_options.instagram.audio.audio_id`.
- Invalid audio usage returns structured validation errors.
- Unsupported account connection paths return actionable errors.
- Public docs include examples and limitations.
- Local backend tests cover search, metadata, validation, and publish payload mapping.

Phase 2 is complete when:

- Dashboard users can search, select, remove, and configure audio for Instagram Reels.
- Dashboard hides or disables the audio selector for unsupported routes.
- Dashboard surfaces preview and availability limitations clearly.

## Open Questions

1. Does UniPost's current Instagram OAuth path support Audio API calls, or do we need a Facebook Login-based Instagram connection path first?
2. Should the initial API expose trending audio by allowing an omitted `search_query`, or should v1 require explicit search text?
3. Should UniPost cache audio search results briefly, or always proxy live Meta responses because availability can vary by account and region?
4. Should dashboard audio search be limited to `music` in v1, or include `original_sound` immediately?
5. Should `audio_name` for original Reels audio be exposed separately from `audio_configuration`, or kept out of v1 to avoid confusion?
6. Do any paid customers require ads audio-replacement flows, or can that stay out of scope?

## Customer-Facing Positioning

Suggested support language:

> For Instagram, Meta now exposes an Audio API for Reels. UniPost can add support for searching authorized Instagram audio and attaching a selected `audio_id` when publishing a Reel. This is different from TikTok: TikTok's normal posting API does not expose library music selection for uploaded videos. For Instagram, this is a UniPost roadmap item on top of a Meta-supported API.
