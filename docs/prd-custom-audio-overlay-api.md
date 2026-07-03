# PRD - Custom Audio Overlay API

**Status:** Planning
**Owner:** Product / API
**Target:** Media processing milestone
**Created:** 2026-07-03

---

## Problem

Customers want to combine their own uploaded audio with their own uploaded videos before publishing to platforms such as TikTok and Instagram.

The major social publishing APIs do not expose a general "attach this audio file to this post" operation:

- TikTok video publishing accepts a final video file. It does not let an API caller upload a separate audio track or select a specific TikTok sound.
- Instagram selected platform audio is a Reels-specific Meta API feature. It does not solve customer-owned audio overlays for arbitrary videos, and it does not preserve image/feed/carousel posts as images.

Today UniPost can accept media uploads and publish final media files, but it does not provide a server-side audio/video composition pipeline. Customers must pre-render the video themselves before uploading to UniPost. That creates friction for API customers who want UniPost to be the full media-preparation layer before cross-platform publishing.

There is one important prerequisite: the current media upload allowlist accepts image and video MIME types only. Audio MIME types must be added before customers can upload the audio input for this API.

There is also a media-upload ergonomics issue in the current public docs: `POST /v1/media` requires customers to provide `size_bytes`. That is too low-level for the common upload flow. UniPost can derive the authoritative object size from R2 after upload, so `size_bytes` should become optional as part of this milestone.

## Product Decision

Ship this as an independent **Custom Audio Overlay API**, not as an implicit option inside `POST /v1/posts`.

The first release should be a two-step flow:

1. Customer uploads a video and an audio file through the existing media upload flow.
2. Customer calls a new media-processing endpoint to create a processed video media asset.
3. Customer publishes the returned `output_media_id` through the existing `POST /v1/posts` API.

This is intentionally separate from platform-native music support. The output is a normal video file with audio already included; platforms will not treat it as a TikTok Sound, Instagram licensed audio, or any other platform music-library asset.

As part of the same API ergonomics milestone, make `size_bytes` optional on `POST /v1/media`. SDKs and dashboard flows should hide file-size handling from users entirely. Raw REST callers may still provide `size_bytes` to get early validation before upload.

## Goals

1. Let customers combine one uploaded video with one uploaded audio file into a new processed video media asset.
2. Keep processing independent from publishing so failures, retries, logs, and customer support are easy to reason about.
3. Reuse the existing `media` table, R2 storage, signed download URLs, and `media_ids` publish path.
4. Make the output compatible with existing TikTok and Instagram video/Reel publishing flows.
5. Support both replacing the original video audio and mixing uploaded audio with the original video audio.
6. Expose job status, processing errors, and output `media_id` through an API that can be polled.
7. Keep v1 narrow enough to ship without building a full video editor.
8. Make media upload easier by allowing `POST /v1/media` without customer-supplied `size_bytes`.

## Non-goals

- Adding TikTok or Instagram platform-library music.
- Selecting a specific TikTok Sound through the TikTok API.
- Preserving image or carousel post types while attaching audio.
- Turning image carousels into slideshow videos in v1.
- Multi-track timeline editing.
- Waveform editing, beat matching, ducking, fades, captions, stickers, or visual overlays.
- Copyright detection, licensing verification, or legal clearance for uploaded audio.
- Automatic publish-time audio processing inside `POST /v1/posts` in v1.
- Browser-based video editing UI in v1.
- Proxying large upload bytes through the UniPost API server. Uploads should continue to go directly to R2 through presigned URLs.

## Current Codebase Findings

### Already present

- `POST /v1/media`, `GET /v1/media/{id}`, and `DELETE /v1/media/{id}` already create and manage R2-backed media assets.
- The media table already stores `workspace_id`, `storage_key`, `content_type`, `size_bytes`, `status`, and video metadata columns.
- Go-native video metadata probing exists for uploaded video files and persists width, height, and duration when available.
- R2 `HEAD` hydration already records authoritative `size_bytes` after upload.
- Publish paths already resolve `media_ids` into signed or staged URLs before platform dispatch.
- Media cleanup infrastructure exists for abandoned and post-publish media lifecycle management.

### Missing or incomplete

- No media-processing job table exists.
- No FFmpeg or FFprobe worker pipeline exists for server-side composition.
- No endpoint creates derived media assets from existing media assets.
- No job status API exists for long-running media processing.
- No structured error taxonomy exists for media-processing failures.
- `POST /v1/media` does not currently accept audio MIME types.
- `POST /v1/media` currently requires `size_bytes`, even though the backend can derive the actual size after upload.
- `POST /v1/posts` should continue to reject audio-only media as publish media, even after audio uploads become valid processing inputs.

## Media Upload Prerequisites and Ergonomics

Extend the `POST /v1/media` MIME allowlist to support the initial audio input formats:

- `audio/mpeg`
- `audio/wav`
- `audio/x-wav`
- `audio/aac`
- `audio/mp4`
- `audio/x-m4a`

Audio media is valid only as a media-processing input in v1. If a customer passes an audio-only `media_id` directly to `POST /v1/posts`, validation should return a clear error such as `audio_media_not_publishable` instead of failing later in a platform adapter.

Make `size_bytes` optional on `POST /v1/media`:

- If `size_bytes` is provided, UniPost validates it before returning the presigned upload URL.
- If `size_bytes` is omitted, UniPost creates a pending media row and derives the actual size from R2 `HEAD` after upload.
- If `size_bytes` is present but less than or equal to zero, return a validation error.
- If the actual uploaded object exceeds the global media hard cap, hydration must reject it with `media_size_exceeded` and prevent the media from being published or used as a processing input.
- The SDKs should calculate or omit `size_bytes` automatically. End users should not manually compute file byte length.

The existing upload hard cap is 4 GB. The stricter audio-overlay limits below are enforced when creating the processing job, not when reserving the original upload.

## API Surface

### Reserve media upload

```http
POST /v1/media
```

Request without customer-supplied size:

```json
{
  "filename": "launch.mp4",
  "content_type": "video/mp4"
}
```

Optional early-validation request:

```json
{
  "filename": "launch.mp4",
  "content_type": "video/mp4",
  "size_bytes": 48293120
}
```

Response:

```json
{
  "data": {
    "id": "2d3ec946-8a6a-4b09-a59c-86f6f7d4cc8a",
    "status": "pending",
    "content_type": "video/mp4",
    "size_bytes": 0,
    "upload_url": "https://...",
    "expires_at": "2026-07-03T12:15:00Z",
    "created_at": "2026-07-03T12:00:00Z"
  },
  "request_id": "req_123"
}
```

When `size_bytes` is omitted, the pending response may return `size_bytes: 0`. After the customer uploads bytes to `upload_url`, `GET /v1/media/{id}` hydrates the row from R2 and returns the actual size.

### Create an audio overlay job

```http
POST /v1/media/audio-overlays
```

Response code: `202 Accepted`.

Request:

```json
{
  "video_media_id": "2d3ec946-8a6a-4b09-a59c-86f6f7d4cc8a",
  "audio_media_id": "8dd7a83b-07be-4bb1-b9ab-3d4983f3f613",
  "mode": "mix",
  "video_volume": 70,
  "audio_volume": 100,
  "audio_start_ms": 0,
  "fit": "trim_to_video"
}
```

Response:

```json
{
  "data": {
    "id": "3d97cd5d-fb08-4f35-a3c8-3d4939d97103",
    "status": "queued",
    "video_media_id": "2d3ec946-8a6a-4b09-a59c-86f6f7d4cc8a",
    "audio_media_id": "8dd7a83b-07be-4bb1-b9ab-3d4983f3f613",
    "output_media_id": null,
    "mode": "mix",
    "fit": "trim_to_video",
    "created_at": "2026-07-03T12:00:00Z",
    "started_at": null,
    "completed_at": null,
    "error": null
  },
  "request_id": "req_123"
}
```

### Get an audio overlay job

```http
GET /v1/media/audio-overlays/{job_id}
```

Successful job response:

```json
{
  "data": {
    "id": "3d97cd5d-fb08-4f35-a3c8-3d4939d97103",
    "status": "succeeded",
    "video_media_id": "2d3ec946-8a6a-4b09-a59c-86f6f7d4cc8a",
    "audio_media_id": "8dd7a83b-07be-4bb1-b9ab-3d4983f3f613",
    "output_media_id": "040db53a-687f-43f6-bf87-292db3ed2444",
    "mode": "mix",
    "fit": "trim_to_video",
    "created_at": "2026-07-03T12:00:00Z",
    "started_at": "2026-07-03T12:00:03Z",
    "completed_at": "2026-07-03T12:00:21Z",
    "error": null
  },
  "request_id": "req_123"
}
```

Failed job response:

```json
{
  "data": {
    "id": "3d97cd5d-fb08-4f35-a3c8-3d4939d97103",
    "status": "failed",
    "video_media_id": "2d3ec946-8a6a-4b09-a59c-86f6f7d4cc8a",
    "audio_media_id": "8dd7a83b-07be-4bb1-b9ab-3d4983f3f613",
    "output_media_id": null,
    "mode": "mix",
    "fit": "trim_to_video",
    "created_at": "2026-07-03T12:00:00Z",
    "started_at": "2026-07-03T12:00:03Z",
    "completed_at": "2026-07-03T12:00:08Z",
    "error": {
      "code": "audio_overlay_processing_failed",
      "message": "The uploaded audio could not be combined with the uploaded video.",
      "retryable": false
    }
  },
  "request_id": "req_123"
}
```

### Publish the processed video

Customers publish the output media through the existing post API:

```json
{
  "platform_posts": [
    {
      "account_id": "sa_tiktok_123",
      "caption": "Launch recap",
      "media_ids": ["040db53a-687f-43f6-bf87-292db3ed2444"],
      "platform_options": {
        "tiktok": {
          "privacy_level": "PUBLIC_TO_EVERYONE"
        }
      }
    }
  ]
}
```

## Request Fields

| Field | Required | Type | Behavior |
|---|---:|---|---|
| `video_media_id` | Yes | string | Existing uploaded media row owned by the workspace. Must be a video. |
| `audio_media_id` | Yes | string | Existing uploaded media row owned by the workspace. Must be audio or video-with-audio that FFmpeg can decode. |
| `mode` | No | enum | `mix` or `replace`. Default: `mix`. |
| `video_volume` | No | integer | Original video audio volume, 0-100. Used only in `mix`. Default: 100. |
| `audio_volume` | No | integer | Uploaded audio volume, 0-100. Default: 100. |
| `audio_start_ms` | No | integer | Offset into the uploaded audio before mixing. Must be less than the decoded audio duration. Default: 0. |
| `fit` | No | enum | `trim_to_video` or `loop_to_video`. Default: `trim_to_video`. |

### Mode behavior

- `replace`: output keeps the input video's visual stream and uses the uploaded audio as the only audio track.
- `mix`: output combines the input video's original audio with the uploaded audio.
- If `mix` is requested but the input video has no audio stream, UniPost treats the video audio as silence and proceeds.

### Fit behavior

- `trim_to_video`: output duration matches the input video duration. Uploaded audio is trimmed if longer. If uploaded audio is shorter, replace mode pads the remainder with silence, while mix mode lets the overlay fall silent and keeps the original video audio.
- `loop_to_video`: uploaded audio repeats until the video duration is filled, then trims at the video end.

## Job Statuses

| Status | Meaning |
|---|---|
| `queued` | Job has been accepted and is waiting for a worker. |
| `processing` | Worker is downloading inputs, running FFmpeg, or uploading output. |
| `succeeded` | Output media was created and is ready to publish. |
| `failed` | Processing failed and no output media is available. |

V1 does not need a cancel endpoint. A queued or processing job may be left to complete or fail. Cancellation can be added later if processing volume makes it valuable.

## Backend Requirements

### 1. Media reserve changes

Update `POST /v1/media` so `size_bytes` is optional:

- Decode `size_bytes` as an optional value, not a required positive integer.
- If `size_bytes` is omitted, create the pending media row with `size_bytes = 0`.
- If `size_bytes` is present and greater than zero, keep the current early validation against `MediaSizeHardCap`.
- If `size_bytes` is present and less than or equal to zero, return a validation error.
- Hydration must treat R2 `HEAD Content-Length` as authoritative and update `media.size_bytes`.
- If hydrated size exceeds `MediaSizeHardCap`, return `media_size_exceeded`, prevent the row from becoming publishable, and best-effort delete or mark the oversized object for cleanup.
- Job creation and publish-time media resolution must trigger hydration for pending rows before checking media-specific limits.

This keeps direct-to-R2 uploads intact while removing the need for users to manually compute file byte length.

### 2. Database

Add a `media_processing_jobs` table:

```sql
CREATE TABLE media_processing_jobs (
  id TEXT PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  kind TEXT NOT NULL,
  status TEXT NOT NULL,
  input_video_media_id TEXT NOT NULL,
  input_audio_media_id TEXT NOT NULL,
  output_media_id TEXT,
  request JSONB NOT NULL,
  idempotency_key TEXT,
  request_hash TEXT,
  error_code TEXT,
  error_message TEXT,
  retryable BOOLEAN NOT NULL DEFAULT false,
  attempts INTEGER NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  started_at TIMESTAMPTZ,
  completed_at TIMESTAMPTZ
);
```

Do not add foreign keys from `media_processing_jobs` to `media(id)`. The existing media cleanup worker hard-deletes media rows after post-publish cleanup windows. Plain text media IDs preserve job history without blocking cleanup deletes.

Indexes:

- `(workspace_id, created_at DESC)` for customer job history.
- `(status, created_at)` for worker dequeue.
- Partial index on `status IN ('queued', 'processing')` for active job monitoring.
- Unique partial index on `(workspace_id, idempotency_key)` where `idempotency_key IS NOT NULL`.

### 3. Media rows

The output of a successful job is a normal `media` row:

- `workspace_id`: same as input media.
- `content_type`: `video/mp4`.
- `status`: `uploaded`.
- `storage_key`: generated with the existing media key convention and a new media ID.
- `size_bytes`, `width`, `height`, and `duration_ms`: hydrated from output object metadata and FFprobe results.

The original video and audio media rows remain unchanged.

### 4. Input media retention

Large media rows may already have `cleanup_after_at` set because they were used in a previous publish. To avoid a worker race where an input file is deleted after job creation but before processing:

- Job creation must verify both input objects still exist.
- If an input media row has a non-null `cleanup_after_at`, job creation must extend it to at least `NOW() + processing_input_hold_window`.
- Suggested `processing_input_hold_window`: 24 hours.
- Small media rows with `cleanup_after_at = NULL` should remain NULL so they do not become newly scheduled for cleanup just because they were used as processing inputs.
- If the worker later discovers an input row or object is missing, mark the job failed with `input_media_unavailable`.

### 5. Worker

Add a media-processing worker that:

1. Dequeues one `queued` job with row-level locking and `FOR UPDATE SKIP LOCKED`.
2. Marks it `processing`.
3. Resolves and validates both input media rows.
4. Downloads input objects from R2 to a private temporary directory.
5. Runs FFprobe to validate streams and duration.
6. Runs FFmpeg with a bounded timeout.
7. Creates a pending output media row and target storage key.
8. Uploads the output MP4 to R2.
9. Hydrates the output media row to `uploaded`.
10. Marks the job `succeeded` with `output_media_id`, or `failed` with a structured error.
11. Deletes temporary files in all success and failure paths.

The API server should not run FFmpeg inside the request handler.

### 6. Worker deployment and recovery

Run audio overlay processing in a separate Railway worker service from the API web service.

Deployment requirements:

- Add FFmpeg and FFprobe to the worker image through a custom Dockerfile or pinned Nixpacks package configuration.
- Pin the FFmpeg major/minor version used in development and production.
- The API web process should create jobs and serve status requests, but should not spend CPU on transcoding.
- The media worker process should run only the media-processing worker loop.
- Default per-process media-processing concurrency should be `1`. Raise only after measuring CPU, memory, disk, and R2 throughput.
- Each worker must reserve enough ephemeral disk for both inputs, the output, and FFmpeg temporary files. A conservative estimate is `input_video_size + input_audio_size + output_estimate + 25%`.

Recovery requirements:

- Claim jobs with `FOR UPDATE SKIP LOCKED`, following the existing post-delivery job dequeue pattern.
- Increment `attempts` when a job is claimed.
- Retry retryable infrastructure failures up to 3 attempts.
- Treat validation and decode failures as non-retryable.
- A stale-processing reaper must move `processing` jobs whose `started_at` is older than the timeout window back to `queued` when `attempts < 3`, or mark them `failed` with `audio_overlay_worker_lost` when attempts are exhausted.
- If a worker crashes after creating a pending output media row but before marking it uploaded, the normal `GET /v1/media/{id}` hydration path may recover it if the object was uploaded. If not, the pending media row remains eligible for existing abandoned-media cleanup.

### 7. FFmpeg profile

V1 output profile:

- Container: MP4.
- Video: H.264. V1 always transcodes to H.264 for one predictable compatibility profile.
- Audio: AAC stereo.
- Fast start: `-movflags +faststart`.
- Duration: match the input video duration.

Example replace command shape:

```bash
ffmpeg -y -i input.mp4 -ss <audio_start_seconds> -i audio.mp3 \
  -filter_complex "[1:a]volume=1.0,apad,atrim=0:<video_duration_seconds>[a]" \
  -map 0:v:0 -map "[a]" \
  -c:v libx264 -preset veryfast -pix_fmt yuv420p \
  -c:a aac -b:a 128k \
  -t <video_duration_seconds> -movflags +faststart \
  output.mp4
```

Replace mode must not use `-shortest` without padding because that truncates the video when the uploaded audio is shorter than the video.

Example mix command shape:

```bash
ffmpeg -y -i input.mp4 -ss <audio_start_seconds> -i audio.mp3 \
  -filter_complex "[0:a]volume=0.7,apad,atrim=0:<video_duration_seconds>[base];[1:a]volume=1.0,apad,atrim=0:<video_duration_seconds>[music];[base][music]amix=inputs=2:duration=first:normalize=0[a]" \
  -map 0:v:0 -map "[a]" \
  -c:v libx264 -preset veryfast -pix_fmt yuv420p \
  -c:a aac -b:a 128k \
  -t <video_duration_seconds> -movflags +faststart \
  output.mp4
```

The worker must branch on FFprobe results:

- If the input video has no audio stream and `mode = mix`, inject a silent base track with `anullsrc` for the video duration instead of referencing `[0:a]`.
- Use `amix=normalize=0` so `video_volume` and `audio_volume` behave as direct gain controls.
- For `loop_to_video`, loop the uploaded audio input with `-stream_loop -1` or an equivalent audio filter before trimming to the video duration.

Implementation must use `exec.CommandContext` with explicit argument arrays. It must not build shell command strings from user input.

### 8. Validation

The create endpoint must reject:

- Media IDs not owned by the authenticated workspace.
- Media rows that are not `uploaded`.
- Deleted media.
- Missing video stream in `video_media_id`.
- Missing decodable audio stream in `audio_media_id`.
- Unsupported `mode`.
- Unsupported `fit`.
- `video_volume` or `audio_volume` outside 0-100.
- Negative `audio_start_ms`.
- `audio_start_ms` greater than or equal to the decoded audio duration.
- Present but non-positive `size_bytes` on `POST /v1/media`.
- Inputs that exceed service limits.

Initial service limits:

- Max input video duration: 10 minutes.
- Max input video size: 500 MB.
- Max input audio size: 100 MB.
- Max processing timeout: 15 minutes.
- Max active jobs per workspace: configurable by plan.

### 9. Error codes

Recommended validation errors:

- `video_media_id_required`
- `audio_media_id_required`
- `media_not_found`
- `media_not_uploaded`
- `media_not_owned`
- `video_stream_required`
- `audio_stream_required`
- `invalid_audio_overlay_mode`
- `invalid_audio_overlay_fit`
- `invalid_audio_overlay_volume`
- `invalid_audio_overlay_offset`
- `media_processing_limit_exceeded`
- `media_size_exceeded`
- `audio_media_not_publishable`
- `input_media_unavailable`

Recommended processing errors:

- `ffmpeg_unavailable`
- `ffprobe_failed`
- `audio_overlay_processing_failed`
- `audio_overlay_timeout`
- `audio_overlay_output_upload_failed`
- `audio_overlay_output_probe_failed`

Logs must include job IDs and media IDs, but not signed URLs, raw FFmpeg command lines containing signed paths, access tokens, or user-provided filenames when they may contain sensitive text.

### 10. Idempotency

Accept an optional `Idempotency-Key` header on `POST /v1/media/audio-overlays`.

Repeated creates with the same workspace, endpoint, and idempotency key return the original job instead of starting duplicate processing. Requests without an idempotency key may create duplicate jobs.

If the same idempotency key is replayed with a different effective request body, return `409 Conflict` with error code `idempotency_conflict`.

## Publishing Behavior

The publish API does not need to know that a video was generated by audio overlay processing. It receives a normal `media_id`.

This keeps behavior predictable:

- Processing failure does not create a failed social post.
- Publishing failure does not rerun media processing.
- One processed video can be reused across TikTok, Instagram, Facebook, YouTube, LinkedIn, or any future video-capable platform.
- Customers can inspect and test the output before publishing.

## Dashboard Requirements

V1 is API-first. Dashboard support is optional but should use the same backend API if added.

If dashboard support is included later:

- Show a media-processing job state separate from post publishing state.
- Let users choose replace vs mix.
- Expose audio and video volume controls.
- Show a generated output video preview before publishing.
- Make it clear that the result is a rendered video, not platform-native music.

## Documentation Requirements

Update public docs for:

- Media uploads, including `size_bytes` as optional rather than required.
- New Custom Audio Overlay API endpoint.
- Media processing job statuses.
- Create Post examples that publish the processed `output_media_id`.
- Platform limitation notes for TikTok and Instagram music APIs.

Docs must explicitly state:

- This feature renders a new video file.
- It does not attach platform-library music.
- It does not let callers choose TikTok Sounds.
- It does not preserve image or carousel post types.
- Customers are responsible for having rights to uploaded audio.
- Users do not need to manually compute media byte size. SDKs and dashboard flows handle it automatically, and raw REST callers may omit `size_bytes`.

## SDK Release Strategy

Release updated SDKs after the backend API, docs, and real development-environment verification pass.

SDK requirements:

- Add high-level upload helpers that accept a local file, stream, or browser `File` object and do not require callers to provide `size_bytes` manually.
- When a runtime can read file size cheaply, the SDK may send `size_bytes` for early validation. When it cannot, the SDK should omit it and rely on server-side R2 hydration.
- Add media-processing methods for creating and retrieving audio overlay jobs.
- Add a convenience helper that can upload video, upload audio, create the overlay job, poll for completion, and return the processed `output_media_id`.
- Keep lower-level raw methods available for advanced callers.

Example SDK-level shape:

```ts
const video = await client.media.upload(videoFile);
const audio = await client.media.upload(audioFile);
const job = await client.media.createAudioOverlay({
  videoMediaId: video.id,
  audioMediaId: audio.id,
  mode: "mix",
  fit: "loop_to_video",
  videoVolume: 70,
  audioVolume: 100
});
const processed = await client.media.waitForAudioOverlay(job.id);
await client.posts.create({
  platform_posts: [{
    account_id: "sa_tiktok_123",
    caption: "Launch recap",
    media_ids: [processed.output_media_id]
  }]
});
```

SDK release gates:

- Backend deployed to development.
- Audio MIME uploads verified in development.
- `size_bytes` omitted upload verified in development.
- Audio overlay job verified in development with shorter, longer, and equal-length audio.
- Processed output published successfully through at least one real development social account.
- Source validation suites pass for JS, Python, Go, and Java SDKs.
- Published-package regression plan is updated for the new media helpers before broad announcement.

## Security, Privacy, and Abuse Controls

- Store temporary files in a private worker directory with per-job names that do not include user-provided filenames.
- Remove temporary files after every job.
- Enforce workspace ownership before reading any input media.
- Cap file sizes, duration, active jobs, and total processing time.
- Extend cleanup windows for large input media while a job is queued or processing.
- Avoid returning internal FFmpeg stderr directly to users.
- Redact signed URLs from logs.
- Treat uploaded audio as customer content subject to existing media retention and cleanup rules.

## Rollout

1. Make `size_bytes` optional on `POST /v1/media`.
2. Extend `POST /v1/media` to accept the v1 audio MIME types.
3. Add `POST /v1/posts` validation that rejects audio-only `media_ids` with `audio_media_not_publishable`.
4. Add the database table and generated query layer.
5. Add job create and job get endpoints using `202 Accepted` and the standard `data` response envelope.
6. Add input cleanup hold behavior for large media rows that already have `cleanup_after_at`.
7. Add a separate Railway media-worker deployment with pinned FFmpeg and FFprobe.
8. Add worker support with FFprobe validation, FFmpeg processing, stale-processing recovery, and per-process concurrency control.
9. Add pending output media creation, R2 upload, FFprobe hydration, and success/failure job transitions.
10. Add API docs and examples.
11. Run local integration tests with representative MP4, MOV, MP3, WAV, AAC, M4A, video-without-audio, audio-shorter-than-video, and audio-longer-than-video inputs.
12. Deploy to development, process a real sample video in the dev environment, and publish the output media through an existing TikTok or Instagram test account.
13. Update JS, Python, Go, and Java SDKs with upload and audio-overlay helpers after backend development verification passes.
14. Run SDK source validation against the development API.
15. Publish the updated SDKs together, then update changelog and SDK docs.
16. Monitor processing time, output size, failure rates, retry rates, disk pressure, SDK helper errors, and publish acceptance rates before broad customer announcement.

## Acceptance Criteria

The feature is complete when:

- `POST /v1/media/audio-overlays` creates an async processing job for uploaded video and audio media.
- `POST /v1/media` accepts omitted `size_bytes` and hydrates the actual size after upload.
- `POST /v1/media` rejects present but non-positive `size_bytes`.
- Oversized uploads discovered during hydration return `media_size_exceeded` and cannot be published or used as processing inputs.
- `POST /v1/media` accepts the documented audio MIME types.
- `POST /v1/posts` rejects audio-only media IDs with a clear validation error.
- `GET /v1/media/audio-overlays/{job_id}` returns queued, processing, succeeded, and failed states.
- A succeeded job creates an `output_media_id` with `content_type = video/mp4` and `status = uploaded`.
- The output media can be published through existing `POST /v1/posts` without publish API changes.
- `mix` and `replace` modes work with volume controls.
- `trim_to_video` and `loop_to_video` fit modes work.
- Replace mode does not truncate the video when uploaded audio is shorter than the video.
- Mix mode works when the input video has no original audio stream.
- Invalid media ownership, status, type, and option values return structured validation errors.
- FFmpeg failures and timeouts produce structured job errors.
- Stale `processing` jobs are retried or marked failed after worker crashes.
- Existing media cleanup hard deletes are not blocked by media-processing job history.
- Temporary files are deleted after success and failure.
- The media worker runs separately from the API web process with FFmpeg/FFprobe available.
- Backend tests cover validation, job lifecycle, FFmpeg command construction, worker success, worker failure, and output media creation.
- SDK helpers hide `size_bytes` from normal upload callers and expose audio overlay creation/status methods.
- JS, Python, Go, and Java SDK source validation passes before SDK release.
- Public docs explain the feature and platform limitations.

## Open Questions

1. Should later versions add stream-copy optimization for already-compatible H.264 inputs to reduce processing cost and quality loss?
2. Should processed output media inherit the source video's cleanup schedule, or should it be treated like a new uploaded media asset with its own cleanup lifecycle?
3. Should API customers be able to list processing jobs, or is direct lookup by job ID sufficient for v1?
4. Should plan limits count processing by job count, input duration, output duration, or compute seconds?
5. Should v1 include a generated preview URL in the job response, or rely on the existing `GET /v1/media/{output_media_id}` endpoint?
6. Should media-processing completion emit a webhook event through the existing webhook delivery infrastructure, or is polling sufficient for v1?

## Customer-Facing Positioning

Suggested support language:

> UniPost can combine your uploaded audio with your uploaded video before publishing. This creates a new video file that you can publish through the normal UniPost API. It is different from TikTok or Instagram platform music: the platform receives a regular video file with audio already included, not a selected TikTok Sound or Instagram licensed audio asset.
