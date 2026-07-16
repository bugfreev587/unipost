# PRD: GIF-to-MP4 Conversion and Unified Media Processing Lifecycle

**Date:** 2026-07-16
**Owner areas:** Media API, Publishing, Dashboard, SDKs, Object Storage
**Status:** Product design approved; ready for implementation planning and review

## 1. Summary

UniPost will add an asynchronous GIF-to-MP4 conversion workflow for platforms
that do not accept an unchanged animated GIF through their supported publishing
APIs.

The feature accepts an already uploaded UniPost GIF Media ID, renders one
platform-independent MP4, returns a new normal `video/mp4` Media ID, and leaves
publishing as a separate explicit action. The same output may be used through
the existing Create Post API, SDKs, or Dashboard composer.

The release also closes the existing object-storage lifecycle gaps around
media-processing inputs and outputs. GIF conversion and the existing Audio
Overlay feature will use one generalized media-processing lifecycle so active
inputs cannot disappear during processing and unused outputs cannot remain in
R2 indefinitely.

V1 includes:

- REST create and status APIs
- asynchronous FFmpeg processing
- JavaScript, Python, Go, and Java SDK support
- Dashboard conversion, preview, replacement, and retry flow
- plan-based fair-use controls
- plan-based lifecycle management for GIFs, converted MP4 files, and Audio
  Overlay media
- API Reference and Guidance updates

V1 does not automatically publish after conversion.

## 2. Problem

UniPost can publish GIF files directly to X and Facebook Pages. Other supported
destinations either do not expose an unchanged GIF publishing path through
their supported APIs or are not yet connected to UniPost's available native
GIF path.

Today a user who uploads a GIF for Instagram, TikTok, Pinterest, YouTube, or
Bluesky reaches an unsupported media path. They must convert the file outside
UniPost, upload the MP4 separately, and then rebuild the post.

There are also object-storage lifecycle gaps:

- post-linked media is protected and later deleted through
  `media_post_usages`, but an uploaded file that is never attached to a post can
  remain indefinitely after it becomes `uploaded`;
- an Audio Overlay output is created as a normal uploaded Media row, but an
  output that is never published has no guaranteed terminal cleanup deadline;
- a media-processing input needs an explicit active hold while a job is queued
  or processing;
- `DELETE /v1/media/{id}` soft-deletes the database row, but the current due
  cleanup query excludes `status = 'deleted'`, leaving a cleanup gap;
- a Cloudflare R2 age-based lifecycle rule cannot safely solve these problems,
  because it cannot distinguish abandoned media from media needed by a draft,
  scheduled post, retry, or active processing job.

The product needs one user-facing conversion workflow and one business-owned
media lifecycle that understands both Post and Media Processing references.

## 3. Goals

1. Let a user convert an uploaded GIF to a broadly compatible MP4 without
   leaving UniPost.
2. Return a normal Media ID that works with the existing post validation and
   publishing APIs.
3. Preserve full animation cycles and provide deterministic output.
4. Support transparent GIFs through a predictable background-color rule.
5. Let Free users test the complete upload, convert, preview, and publish flow.
6. Protect the service from CPU, memory, disk, and decompression abuse.
7. Ensure active processing inputs cannot be deleted by retention cleanup.
8. Ensure unused GIFs, converted MP4 files, and Audio Overlay outputs eventually
   leave R2 according to the workspace Plan.
9. Preserve the existing direct GIF workflow for X and Facebook.
10. Keep conversion and publishing separate so users can inspect the rendered
    video before it is sent to a social platform.

## 4. Non-goals

V1 does not include:

- automatic publishing when conversion succeeds;
- a remote URL as the GIF conversion input;
- target-platform-specific rendering profiles;
- per-platform Media overrides in the Dashboard composer;
- native GIF integration for LinkedIn or Threads;
- conversion cancellation;
- conversion job listing;
- completion webhooks;
- conversion credits or usage-based conversion billing;
- customer-selected codec, frame rate, bitrate, resolution, duration, or audio;
- cross-request content-level output caching;
- upscaling small GIFs to satisfy a destination's minimum dimensions;
- editing, trimming, cropping, captions, watermarks, or audio addition;
- replacing platform validation with conversion-time platform validation;
- a Cloudflare R2 object-age lifecycle rule for user post media.

## 5. Platform product behavior

| Platform | Official publishing shape | UniPost V1 behavior |
| --- | --- | --- |
| X / Twitter | Direct animated GIF media | Keep direct GIF publishing |
| Facebook Page | GIF photo post | Keep direct GIF publishing |
| LinkedIn | Native GIF capability exists upstream | UniPost native support remains Coming soon |
| Threads | Provider-backed GIF attachment capability exists upstream | UniPost native support remains Coming soon |
| Instagram | No unchanged GIF publishing surface in the supported flow | Offer GIF-to-MP4 |
| TikTok | Video publishing flow | Offer GIF-to-MP4 |
| Pinterest | Supported organic animated-media path is video-based | Offer GIF-to-MP4 |
| YouTube | Video upload flow | Offer GIF-to-MP4 |
| Bluesky | UniPost uses the video publishing path for animated output | Offer GIF-to-MP4 |

The conversion API itself is platform-independent. An API caller may use the
generated MP4 with any destination that accepts the output and passes the
destination's normal validation.

Conversion success does not guarantee that the output satisfies every target
platform. For example, the no-upscale rule may produce a small MP4 that later
fails a platform's minimum-dimension rule. That failure belongs to Create Post
validation, not to the conversion API.

## 6. User experience

### 6.1 REST and SDK flow

1. Upload a GIF through `POST /v1/media`.
2. Complete the presigned upload and obtain an uploaded GIF Media ID.
3. Create a GIF conversion job.
4. Poll the conversion job until it succeeds or fails.
5. Read `output_media_id`.
6. Optionally fetch the output through `GET /v1/media/{id}` for preview.
7. Publish the MP4 through the existing Create Post API.

Conversion never creates a Post and never publishes automatically.

### 6.2 Dashboard flow

1. The user uploads a GIF in the composer.
2. The Dashboard completes the existing Media upload and obtains
   `gif_media_id`.
3. If the selected destinations include Instagram, TikTok, Pinterest, YouTube,
   or Bluesky, the media card shows a blocking compatibility message and a
   **Convert to MP4** action.
4. The user may keep the default white background or open an advanced control
   to choose a six-digit hexadecimal background color.
5. The Dashboard creates the job and polls its status.
6. Publish is disabled while the required conversion is queued or processing.
7. When the job succeeds, the Dashboard globally replaces the GIF Media item
   with the returned MP4 Media item.
8. The composer shows a playable preview plus duration, dimensions, and file
   size.
9. The user reviews the result and separately clicks Publish.

If conversion fails, the original GIF remains in the composer and the user can
retry.

### 6.3 Mixed-platform Dashboard posts

The current composer shares one Media collection across all selected
destinations.

If a user selects X and Instagram together and converts the GIF, both
destinations receive the MP4. V1 does not keep the GIF for X while sending the
MP4 to Instagram.

REST and SDK callers can preserve the native GIF for X or Facebook and use the
converted MP4 elsewhere by providing different
`platform_posts[].media_ids` values.

Per-platform Media overrides in the Dashboard are a later feature.

### 6.4 Composer state and recovery

- Persist the conversion job ID in the composer draft.
- After a refresh, reopen the draft and continue polling the same job.
- Poll every two seconds while the page is active.
- Reduce polling frequency when the document is hidden.
- Do not display a fabricated percentage. Use the states `Queued` and
  `Converting`.
- Removing the GIF from the composer does not cancel the server job.
- If a removed job later succeeds, its unused output is handled by Media
  Processing retention.
- If the selected destinations change, recompute whether conversion is
  required.
- If an MP4 already replaced the GIF, adding another video-based destination
  reuses the current MP4.
- When only X and/or Facebook are selected, conversion is not required.
- LinkedIn and Threads continue to display native GIF support as Coming soon
  and are not part of the V1 automatic conversion recommendation.

## 7. Public API

### 7.1 Create a GIF conversion

```http
POST /v1/media/gif-conversions
Authorization: Bearer <api-key>
Idempotency-Key: <unique-key>
Content-Type: application/json
```

```json
{
  "gif_media_id": "med_gif_123",
  "background_color": "#FFFFFF"
}
```

Fields:

| Field | Required | Rules |
| --- | --- | --- |
| `gif_media_id` | Yes | Must belong to the current workspace and resolve to an uploaded `image/gif` object |
| `background_color` | No | Six-digit `#RRGGBB`; defaults to `#FFFFFF` |

The API does not accept a remote GIF URL in V1.

Successful creation returns `202 Accepted`:

```json
{
  "data": {
    "id": "mpj_123",
    "kind": "gif_to_mp4",
    "status": "queued",
    "gif_media_id": "med_gif_123",
    "background_color": "#FFFFFF",
    "output_profile": "universal_mp4_v1",
    "output_media_id": null,
    "created_at": "2026-07-16T18:00:00Z",
    "started_at": null,
    "completed_at": null,
    "error": null
  }
}
```

The ID is an opaque Media Processing Job ID. This PRD does not require changing
the existing database ID format.

### 7.2 Get a GIF conversion

```http
GET /v1/media/gif-conversions/{conversion_id}
Authorization: Bearer <api-key>
```

Successful response:

```json
{
  "data": {
    "id": "mpj_123",
    "kind": "gif_to_mp4",
    "status": "succeeded",
    "gif_media_id": "med_gif_123",
    "background_color": "#FFFFFF",
    "output_profile": "universal_mp4_v1",
    "output_media_id": "med_mp4_456",
    "created_at": "2026-07-16T18:00:00Z",
    "started_at": "2026-07-16T18:00:01Z",
    "completed_at": "2026-07-16T18:00:05Z",
    "error": null
  }
}
```

Failed response body:

```json
{
  "data": {
    "id": "mpj_123",
    "kind": "gif_to_mp4",
    "status": "failed",
    "gif_media_id": "med_gif_123",
    "background_color": "#FFFFFF",
    "output_profile": "universal_mp4_v1",
    "output_media_id": null,
    "created_at": "2026-07-16T18:00:00Z",
    "started_at": "2026-07-16T18:00:01Z",
    "completed_at": "2026-07-16T18:00:03Z",
    "error": {
      "code": "gif_frame_count_exceeded",
      "message": "The GIF contains more than 2,000 frames.",
      "retryable": false
    }
  }
}
```

### 7.3 Job states

```text
queued -> processing -> succeeded
                    \-> failed
```

The existing database may retain `cancelled` as a generic Media Processing
status, but V1 GIF conversion does not expose a cancel endpoint.

V1 has no conversion list endpoint and no completion webhook.

### 7.4 Idempotency

`Idempotency-Key` is optional but strongly recommended.

- The normalized request includes `gif_media_id`, the normalized uppercase
  `background_color`, and `output_profile`.
- Reusing a key with the same normalized request returns the existing job.
- Reusing a key with different content returns HTTP `409` with
  `idempotency_conflict`.
- An idempotent replay does not consume another fair-use unit.
- Worker retries do not consume another fair-use unit.

### 7.5 Creation-time errors

| HTTP | Public code | Meaning |
| ---: | --- | --- |
| 404 | `media_not_found` | Media does not exist or belongs to another workspace |
| 409 | `idempotency_conflict` | Key was already used with another request |
| 409 | `input_media_unavailable` | Media row exists, but upload is incomplete or the R2 object is missing |
| 422 | `gif_media_required` | Input is not an uploaded `image/gif` |
| 422 | `gif_size_exceeded` | Hydrated GIF size exceeds 50 MB |
| 422 | `invalid_background_color` | Color is not a valid six-digit Hex value |
| 429 | `media_processing_capacity_exceeded` | Workspace active-job limit is reached |
| 429 | `gif_conversion_rate_limit_exceeded` | Workspace rolling 24-hour conversion limit is reached |
| 503 | `media_processing_unavailable` | Required storage or processing infrastructure is unavailable |

Capacity errors include `Retry-After`. The rolling-window error also includes
`reset_at`.

### 7.6 Asynchronous processing errors

Stable public Job error codes:

| Code | Retryable | Meaning |
| --- | --- | --- |
| `gif_dimensions_exceeded` | No | Source dimension safety limit exceeded |
| `gif_frame_count_exceeded` | No | Source contains more than 2,000 frames |
| `gif_decode_budget_exceeded` | No | Total decoded-pixel budget exceeded |
| `gif_duration_exceeded` | No | One complete GIF cycle is longer than 60 seconds |
| `gif_probe_failed` | No | Source metadata cannot be safely read |
| `gif_decode_failed` | No | Source is malformed, truncated, or unsupported |
| `gif_conversion_failed` | No | FFmpeg could not render a valid output |
| `output_size_exceeded` | No | Output exceeds UniPost's global Media hard cap |
| `output_upload_failed` | Yes | R2 output upload failed |
| `processing_timeout` | Yes until retries exhaust | Processing exceeded five minutes |
| `media_processing_worker_lost` | No after retries exhaust | A stale job could not be recovered |

Do not return raw FFmpeg stderr, worker file paths, storage keys, signed URLs,
credentials, or internal commands to users. Internal logs may retain a
sanitized diagnostic summary correlated by job ID and request ID.

## 8. Conversion behavior

### 8.1 Input

V1 accepts:

- a UniPost Media ID;
- owned by the current workspace;
- uploaded and present in R2;
- actual hydrated content type `image/gif`;
- hydrated size no greater than 50 MB.

The conversion endpoint must use the current Media hydration path before
admission so it does not trust only the client's original declared size or
content type.

### 8.2 Universal output profile

`universal_mp4_v1` is fixed:

| Property | Value |
| --- | --- |
| Container | MP4 |
| Video codec | H.264 through `libx264` |
| Pixel format | `yuv420p` |
| Frame rate | Constant 30 FPS |
| Audio | None |
| Web playback | `-movflags +faststart` |
| Aspect ratio | Preserve source |
| Maximum longest edge | 1920 px |
| Upscaling | Never upscale |
| Dimension parity | Width and height must be even |
| Transparency | Composite over selected background |
| Default background | `#FFFFFF` |

The profile is not parameterized by destination platform.

### 8.3 Duration and looping

Let `D` be the duration of one complete GIF animation cycle.

- If `D > 60 seconds`, reject with `gif_duration_exceeded`.
- Never silently truncate a complete source cycle.
- If `D >= 5 seconds`, render one complete cycle.
- If `D < 5 seconds`, render
  `ceil(5 seconds / D)` complete cycles.
- The resulting duration is therefore at least five seconds and no more than
  60 seconds.
- A static single-frame GIF is treated as a valid animation source and rendered
  as a five-second MP4.
- Preserve source frame timing as closely as the fixed 30 FPS output permits.

### 8.4 Dimensions

- Preserve the source aspect ratio.
- If the longest source edge is greater than 1920 px, scale down so the longest
  output edge is 1920 px or less.
- Do not upscale when both source dimensions are already below the limit.
- Adjust computed width and height down to even integers when required by
  H.264.
- Do not distort the image to reach even dimensions.
- A later Create Post validation may reject an output that is below a target
  platform's minimum dimensions.

### 8.5 Transparency

- The default background is opaque white.
- A caller may provide an opaque `#RRGGBB` background.
- Alpha-channel output is not supported in the MP4 profile.
- The Dashboard explains that transparent areas will become the selected
  background color before conversion begins.

### 8.6 Resource-safety limits

| Limit | V1 value |
| --- | ---: |
| Compressed GIF size | 50 MB |
| Source width | 4096 px maximum |
| Source height | 4096 px maximum |
| Frames | 2,000 maximum |
| Total decoded pixels | 1.5 billion maximum |
| One animation cycle | 60 seconds maximum |
| Worker execution timeout | 5 minutes |
| Automatic attempts | 3 maximum |

Total decoded pixels are computed from the effective decoded canvas dimensions
multiplied by the decoded frame count. The worker must not rely on compressed
file size as its only decompression-bomb defense.

Validation and decode errors are not retried. Transient R2, database, process,
or worker-loss failures may be retried up to the attempt limit.

## 9. Fair-use and admission control

GIF conversion is available on every Plan and does not consume Post quota or a
separate conversion-credit balance.

| Plan | Active jobs per workspace | GIF conversions per rolling 24 hours |
| --- | ---: | ---: |
| Free | 1 | 10 |
| API | 2 | 50 |
| Basic | 2 | 100 |
| Growth | 4 | 300 |
| Team | 6 | 1,000 |
| Enterprise | Contract-configurable, default 6 | Contract-configurable, default 1,000 |

Rules:

- `queued` and `processing` jobs count as active.
- Active capacity is shared across GIF conversion and Audio Overlay so one
  workspace cannot bypass CPU protection by alternating job kinds.
- The rolling GIF limit counts newly created non-idempotent
  `kind = 'gif_to_mp4'` jobs.
- Idempotent replays and Worker retries do not add usage.
- No customer daily quota is added to Audio Overlay in this PRD.
- Plan limits live in a centralized Media Processing policy, not scattered
  handler conditionals.
- Enterprise overrides must be read through the existing entitlement or
  contract-configuration pattern.
- The API checks admission before inserting a new job, using an atomic or
  transactionally protected path so concurrent requests cannot exceed the
  active limit.
- Operational configuration may lower global Worker capacity without changing
  the public per-workspace Plan contract.

## 10. Backend architecture

### 10.1 Services

The API web service:

- authenticates and resolves workspace ownership;
- hydrates and validates input Media;
- enforces idempotency and fair-use;
- creates processing jobs and lifecycle references;
- serves job status;
- does not execute FFmpeg.

A dedicated Railway Media Worker:

- processes both `gif_to_mp4` and `audio_overlay`;
- downloads private inputs from R2;
- runs FFprobe and FFmpeg;
- uploads outputs;
- creates and hydrates output Media rows;
- updates processing lifecycle references;
- marks jobs terminal;
- performs stale-job recovery;
- removes local temporary files on every path.

Move the currently in-process Audio Overlay Worker out of the API runtime as
part of this release. The API and Media Worker may share an image, but their
process commands and resource allocation must be separate.

### 10.2 Worker concurrency and fairness

- Default Media Worker process concurrency is `1`.
- Raise concurrency only after observing CPU, memory, ephemeral disk, R2, and
  queue metrics in development and staging.
- GIF and Audio Overlay use the same global execution pool.
- Job claiming is kind-aware.
- With one available slot, alternate eligible kinds in round-robin order so a
  sustained stream of one kind cannot permanently starve the other.
- A kind-specific claim query accepts `kind` and uses
  `FOR UPDATE SKIP LOCKED`.
- The existing Audio Overlay loop must not claim a GIF job and then skip it.

### 10.3 Retry and stale recovery

- Increment `attempts` when a job is claimed.
- Retry only errors classified as transient.
- Maximum attempts: 3.
- A `processing` job older than five minutes is stale.
- If attempts remain, stale recovery moves it back to `queued`.
- If attempts are exhausted, mark it `failed` with
  `media_processing_worker_lost` or `processing_timeout`, according to the
  last known failure.
- A worker heartbeat is required for operational monitoring, but user-visible
  job correctness must not depend only on heartbeat state.

### 10.4 Temporary files

- Use a private per-job temporary directory.
- Never include a user filename in the local path.
- Estimate required ephemeral disk before starting when possible.
- Remove source, output, probe files, and directory contents after success,
  failure, timeout, or process cancellation.
- Do not log signed download URLs.

## 11. Data model

### 11.1 Generalize `media_processing_jobs`

The current table is Audio Overlay-specific. Additive migration requirements:

- extend `kind` to include `gif_to_mp4`;
- add nullable `input_media_id` for single-input processing;
- allow the existing `input_video_media_id` and `input_audio_media_id` columns
  to be nullable for non-Audio jobs;
- retain existing Audio Overlay settings and compatibility;
- retain `request`, `idempotency_key`, `request_hash`, error, retry, and
  timestamp fields;
- add or preserve an indexed claim path by `kind`, `status`, `created_at`, and
  `id`;
- preserve the unique workspace idempotency constraint.

Normalized GIF request JSON:

```json
{
  "gif_media_id": "med_gif_123",
  "background_color": "#FFFFFF",
  "output_profile": "universal_mp4_v1"
}
```

Do not add foreign keys from the job's historical input or output ID columns to
`media(id)`. Media cleanup must be able to hard-delete Media while job history
retains the original opaque IDs.

### 11.2 Add `media_processing_usages`

Add a business-owned lifecycle ledger:

```text
media_processing_usages
- id
- workspace_id
- media_id
- processing_job_id
- role
- status
- cleanup_after_at
- created_at
- updated_at
```

Constraints:

- `role IN ('input', 'output')`
- `status IN ('active', 'succeeded', 'failed')`
- one row per `(media_id, processing_job_id, role)`
- `media_id` may reference `media(id) ON DELETE CASCADE`
- `processing_job_id` may reference `media_processing_jobs(id) ON DELETE CASCADE`
- job history itself still retains plain input/output ID columns

Semantics:

- `active` always has `cleanup_after_at = NULL` and blocks deletion;
- `succeeded` has the Plan success deadline;
- `failed` has the Plan failure deadline;
- an output row is created only for an output that reached the uploaded Media
  state;
- partial or invalid output objects are compensating-deleted and do not become
  reusable output usages.

### 11.3 Repurpose `media.cleanup_after_at`

The column exists but its old fixed-size/fixed-two-hour policy was disabled.
Reuse it only as the Media row's base unattached deadline.

Rules:

- when a Media row becomes `uploaded`, set a base cleanup deadline using the
  current workspace Plan's success window;
- if bytes were uploaded but the row is never hydrated, it remains `pending`
  and continues through the existing seven-day abandoned-upload cleanup;
- `media.cleanup_after_at` does not override an active Post or Processing
  reference;
- never restore the old two-hour policy;
- a transition may extend a base deadline but must not shorten an existing
  future deadline;
- setting a usage-specific deadline does not require rewriting unrelated usage
  deadlines.

### 11.4 Plan retention windows

Use the existing Media retention matrix:

| Plan | Success / published | Failure / partial / cancelled |
| --- | ---: | ---: |
| Free | 1 day | 2 days |
| API | 2 days | 4 days |
| Basic | 4 days | 8 days |
| Growth | 15 days | 30 days |
| Team | 30 days | 60 days |
| Enterprise | 30 days | 60 days |

For processing:

- conversion or overlay success uses the success window;
- conversion or overlay failure uses the failure window for inputs;
- incomplete outputs are deleted immediately rather than retained as failed
  customer Media.

The workspace Plan is evaluated when a lifecycle transition writes its
deadline. Later Plan downgrade or upgrade does not retroactively recompute that
stored deadline. A later Post or Processing usage created under a new Plan gets
its own deadline from the then-current Plan.

## 12. Unified Media lifecycle

### 12.1 Lifecycle rules

| Event | Required behavior |
| --- | --- |
| Media upload reserved but never hydrated | Existing `pending` cleanup after 7 days |
| Media becomes uploaded but remains unattached | Base Plan success deadline starts |
| Processing job is queued | Input receives active processing usage |
| Processing job is running | Active input usage continues to block cleanup |
| Processing succeeds | Inputs and valid output receive succeeded usage deadlines |
| Processing fails | Inputs receive failed usage deadlines |
| Partial output exists | Delete R2 object and Media row through immediate compensation |
| Media is used by Draft, Scheduled, Queued, Publishing, or Processing Post | Active Post usage blocks cleanup |
| Post reaches a terminal status | Existing status/Plan Post deadline applies |
| User deletes unused Media | Mark for next cleanup sweep |
| User deletes referenced Media | Return `409 media_in_use` |
| Media has multiple references | Delete only after all references are terminal and due |

### 12.2 Cleanup eligibility

A Media object is eligible when:

1. at least one base or usage deadline is due, or the Media was explicitly
   soft-deleted; and
2. its base deadline is null or due; and
3. no Post usage has a null or future deadline; and
4. no Processing usage has a null or future deadline.

Conceptually:

```text
(base deadline due OR post usage due OR processing usage due OR explicitly deleted)
AND no future base deadline
AND no active/future post usage
AND no active/future processing usage
=> delete source R2 object, pull copy, and Media row
```

The actual SQL must evaluate blockers across both ledgers. A due reference must
never delete an object still protected by another reference.

The cleanup worker remains business-state-driven and runs once on startup and
then daily. Cloudflare R2 age-based lifecycle remains disabled for post Media.

### 12.3 Explicit deletion

`DELETE /v1/media/{id}` must:

- verify workspace ownership;
- return `404` if no active Media exists;
- return `409 media_in_use` when an active Post or Processing usage exists;
- otherwise mark the Media deleted and set it eligible for the next cleanup
  sweep;
- ensure the cleanup query includes soft-deleted rows;
- delete the R2 source object and deterministic pull copy before hard-deleting
  the database row.

### 12.4 Successful GIF conversion

When a GIF job succeeds:

- the source GIF row remains unchanged as customer Media;
- its active processing usage becomes `succeeded` with a success deadline;
- a new `video/mp4` Media row becomes `uploaded`;
- an output processing usage is created with the same success deadline;
- the job is marked `succeeded` only after the output is safely reusable;
- later Post usage of either Media ID can extend its effective lifetime by
  adding a blocking Post reference.

### 12.5 Audio Overlay lifecycle

Apply the same lifecycle to existing Audio Overlay jobs:

- video and audio inputs receive active usages at job creation;
- successful inputs and output receive Plan success deadlines;
- failed inputs receive Plan failure deadlines;
- incomplete output rows and objects are compensating-deleted;
- unused uploaded Audio Overlay output is no longer retained indefinitely.

## 13. Migration and backfill

The migration must be safe in environments containing existing Audio Overlay
jobs.

Required sequence:

1. Add or alter generalized job columns and constraints.
2. Add `media_processing_usages` and indexes.
3. Add the new claim indexes without removing the old path until code is ready.
4. Backfill input usages for existing Audio Overlay jobs.
5. Backfill output usages where `output_media_id` exists.
6. Use `active` with no deadline for existing `queued` or `processing` jobs.
7. Use the workspace Plan and job completion time for existing `succeeded`
   outputs and inputs.
8. Use the workspace Plan and job completion/update time for existing `failed`
   inputs.
9. If a terminal timestamp is unavailable, use the migration time
   conservatively rather than creating an already-expired deadline.
10. Initialize base deadlines for existing unattached uploaded Media that have
    no Post or Processing usage, using migration time plus the current Plan
    success window.
11. Do not shorten a future deadline already recorded by any existing policy.
12. Update the cleanup query to include base deadlines, Post usages, Processing
    usages, and soft-deleted rows.

The backfill must be idempotent or protected so a retried deployment does not
create duplicate usages or shorten retention.

## 14. Worker success and failure ordering

### 14.1 Success

The worker completes in this order:

1. Download and validate the source.
2. Render a local MP4.
3. Probe and validate the local result.
4. Create a pending output Media row.
5. Assign the final `media/<id>.mp4` storage key.
6. Upload the MP4 to R2.
7. HEAD and probe the stored object.
8. Mark output Media `uploaded` with accurate metadata.
9. In one database transaction:
   - transition input processing usage to `succeeded`;
   - create output processing usage;
   - write Plan deadlines;
   - mark the job `succeeded` with `output_media_id`.
10. Remove local files.

The status API must never expose a successful `output_media_id` before the
Media row is uploaded and its processing lifecycle exists.

### 14.2 Failure

- Before output upload: mark the job failed and transition input usage.
- After creating an output row but before a valid uploaded output exists:
  best-effort delete the R2 object, then hard-delete the pending row.
- If compensation fails, record an internal cleanup error and leave a pending
  row that remains eligible for the existing abandoned cleanup.
- Never return a partial output Media ID to the customer.
- Remove temporary files.

## 15. Dashboard implementation contract

### 15.1 Media item state

The composer Media model needs to represent:

- local upload state;
- server-backed Media ID;
- original GIF preview;
- conversion job ID;
- conversion status;
- converted output Media ID;
- server-backed MP4 preview URL;
- output content type, dimensions, duration, and size;
- structured conversion error.

Create shared conversion orchestration used by both the Create Post drawer and
calendar/editor flows. Do not duplicate polling and replacement logic in each
composer.

### 15.2 Required UI states

- GIF ready and directly publishable
- GIF requires conversion for selected destinations
- conversion queued
- conversion processing
- conversion succeeded and MP4 preview ready
- conversion failed and retry available
- polling interrupted and recoverable
- conversion rate-limited with reset guidance

### 15.3 Accessibility and copy

- The action is labeled **Convert to MP4**.
- Explain that the result is published as a video, not as a native GIF.
- State when transparent areas will become white or the selected color.
- Use textual status in addition to color and animation.
- Conversion errors must remain readable without opening developer logs.
- Do not promise platform compatibility before Create Post validation runs.

## 16. SDK contract

Release JavaScript, Python, Go, and Java SDK methods:

```text
createGifConversion(...)
getGifConversion(conversionId)
waitForGifConversion(conversionId, options)
uploadAndConvertGif(file, options)
```

Example JavaScript shape:

```ts
const gif = await client.media.upload(file);

const conversion = await client.media.createGifConversion({
  gifMediaId: gif.id,
  backgroundColor: "#FFFFFF",
});

const result = await client.media.waitForGifConversion(conversion.id, {
  timeoutMs: 5 * 60 * 1000,
});

await client.posts.create({
  platform_posts: [{
    account_id: "sa_tiktok_123",
    caption: "Animated update",
    media_ids: [result.output_media_id],
  }],
});
```

SDK behavior:

- default polling interval: two seconds;
- support custom interval and timeout;
- support runtime-appropriate cancellation or abort for client polling;
- client cancellation does not cancel the server job;
- return only a succeeded result from `waitForGifConversion`;
- throw a typed error containing stable `code`, `message`, and `retryable` when
  the job fails;
- preserve lower-level create/get methods;
- `uploadAndConvertGif` wraps upload, conversion, and polling;
- no helper automatically publishes;
- SDKs may generate an idempotency key for the high-level helper while allowing
  the caller to provide one explicitly.

SDK publication follows backend and documentation deployment plus real
development-environment verification.

## 17. Post validation

The output enters the existing publishing system as normal `video/mp4` Media.

The Worker must write:

- `content_type`
- `size_bytes`
- `width`
- `height`
- `duration_ms`

Create Post validation remains authoritative for destination-specific:

- accepted video content types;
- minimum and maximum dimensions;
- aspect ratios;
- duration;
- file size;
- creator-specific TikTok limits;
- any platform or account capability.

Before launch, audit Instagram, TikTok, Pinterest, YouTube, and Bluesky
validation for missing minimum-dimension or duration constraints. Add missing
rules as part of this implementation so an incompatible output fails before
dispatch with an actionable normalized validation error.

Direct GIF validation for X and Facebook remains unchanged.

## 18. Documentation

Add:

- Create GIF Conversion API Reference
- Get GIF Conversion API Reference
- GIF conversion job/error reference content
- JavaScript, Python, Go, and Java examples

Update:

- Media upload API Reference
- Get Media API Reference
- Create Post API Reference
- Create Post validation documentation
- Publish GIFs to X and Facebook Guidance
- Video + Audio Overlay documentation where lifecycle behavior is relevant

Documentation must say:

- conversion renders a new MP4 Media asset;
- conversion and publishing are separate;
- the MP4 is published as video, not as platform-native GIF;
- the API accepts an uploaded UniPost GIF Media ID, not a remote URL;
- X and Facebook can still receive the original GIF;
- LinkedIn and Threads native GIF integration remains Coming soon;
- conversion success does not guarantee every platform's video validation will
  pass;
- unused source and output Media follow Plan-based retention.

## 19. Security, privacy, and abuse controls

- Enforce workspace ownership before reading input metadata or bytes.
- Use hydrated R2 metadata rather than trusting request values.
- Keep signed URLs and storage credentials out of application logs.
- Keep temporary input and output files private.
- Remove temporary files on all terminal paths.
- Enforce compressed size, dimensions, frame count, decoded-pixel budget,
  duration, timeout, and admission limits.
- Do not return raw decoder or FFmpeg output to customers.
- Treat both GIF source and generated MP4 as customer content.
- Preserve job history without retaining customer Media indefinitely.
- Do not make R2 objects public solely to perform conversion.
- Maintain normal Media download URLs as short-lived presigned URLs.

## 20. Observability

Metrics:

- queued, processing, succeeded, and failed jobs;
- counts by Plan and `kind`;
- counts by stable error code;
- queue wait p50, p95, p99;
- processing duration p50, p95, p99;
- input and output bytes;
- output/input byte ratio;
- source dimensions, frames, duration, and decoded-pixel buckets;
- retries, stale recoveries, and timeouts;
- R2 download and upload failures;
- Worker CPU, memory, and ephemeral disk;
- Worker heartbeat;
- active jobs by workspace and Plan;
- rolling fair-use rejections;
- Media objects and bytes protected by Processing usage;
- unattached Media due count and bytes;
- cleanup backlog, cleanup failures, deleted objects, and deleted bytes.

Recommended alerts:

- oldest queued job exceeds two minutes;
- five-minute processing failure rate exceeds 10%;
- no Worker heartbeat for five minutes;
- stale recovery rises continuously;
- Worker ephemeral disk exceeds 80%;
- consecutive R2 output upload failures;
- cleanup failures or due backlog rise across multiple sweeps.

Admin Object Storage must continue to reflect created objects, confirmed bytes,
due cleanup, and cleanup activity after processing outputs are introduced.

## 21. Rollout

No feature flag is added.

### Phase 1: Unified Media Processing infrastructure

- generalize `media_processing_jobs`;
- add kind-specific claims and shared Worker fairness;
- add `media_processing_usages`;
- repurpose the base Media deadline;
- fix unattached and soft-deleted Media cleanup;
- move Audio Overlay to the dedicated Media Worker;
- backfill Audio Overlay lifecycle references and output deadlines.

### Phase 2: GIF conversion backend

- add create/get endpoints;
- add GIF probing, limits, FFmpeg rendering, and output hydration;
- add Plan admission, retries, stale recovery, and metrics;
- add API Reference;
- add SDK methods and tests.

### Phase 3: Dashboard

- detect conversion-required destinations;
- add background selection;
- create, poll, preview, replace, and retry;
- persist conversion ID in drafts;
- update Guidance and publishing examples.

All three phases are one launch outcome. Do not announce general availability
until backend, SDK, Dashboard, retention cleanup, and real environment
acceptance have passed.

## 22. Rollback

- Stop accepting new GIF conversion jobs while keeping GET status available.
- Stop `gif_to_mp4` claims without stopping `audio_overlay`.
- Leave queued jobs and their active processing usages intact until processing
  resumes or an explicit operational recovery marks them terminal.
- Do not delete already succeeded output Media as part of rollback; normal
  retention manages them.
- Keep schema migrations additive so rollback does not discard job or Media
  history.
- If the new cleanup query is suspected, disable the cleanup Worker before
  changing stored deadlines.
- Audio Overlay must retain a viable independent claim path during deployment
  so the migration cannot strand existing jobs.

## 23. Testing

### 23.1 Backend and database

- request validation;
- workspace ownership isolation;
- hydration and missing-object behavior;
- idempotent replay and conflict;
- Plan active-job admission under concurrency;
- rolling 24-hour limits;
- kind-specific claim isolation;
- round-robin kind fairness;
- retry classification and attempt limits;
- stale processing recovery;
- migration and idempotent backfill;
- processing usage transitions;
- base unattached deadlines;
- soft-delete cleanup eligibility;
- shared Media across multiple Posts and Processing jobs;
- Plan transition deadline snapshots;
- no cleanup while any active or future reference exists;
- Audio Overlay regression.

### 23.2 Conversion

- opaque GIF;
- transparent GIF with default white background;
- transparent GIF with custom background;
- static single-frame GIF;
- short loop repeated to at least five seconds;
- loop exactly five seconds;
- loop between five and 60 seconds;
- loop exactly 60 seconds;
- loop over 60 seconds rejected;
- odd dimensions;
- small dimensions without upscaling;
- large dimensions scaled to longest edge 1920;
- 50 MB boundary;
- 2,000-frame boundary;
- decoded-pixel boundary;
- malformed and truncated files;
- decompression-bomb fixtures;
- no audio stream;
- H.264, `yuv420p`, 30 FPS, faststart metadata;
- output Media hydration;
- partial output compensation.

### 23.3 Dashboard

- X-only direct GIF;
- Facebook-only direct GIF;
- conversion-required single destination;
- mixed X plus conversion-required destination;
- transparent background notice and custom color;
- queued and processing status;
- successful global replacement;
- preview metadata;
- failed conversion retry;
- reload and draft recovery;
- removing Media during processing;
- changing selected destinations;
- rate-limit and capacity messaging;
- Publish disabled only when required conversion is incomplete.

### 23.4 SDKs

- serialization and response types;
- idempotency behavior;
- polling success;
- typed terminal failure;
- timeout;
- cancellation/abort;
- high-level upload-and-convert helper;
- no automatic publish.

## 24. Local and CI validation

Required before integration:

- from `api/`:
  `GOCACHE=/tmp/unipost-go-build go test ./...`
- from `dashboard/`:
  `npm run build`
- from `dashboard/`, when Playwright browsers are installed:
  `npm run test:regression:dashboard`
- JavaScript SDK validation
- Python SDK validation
- Go SDK validation
- Java SDK validation

## 25. Development-environment acceptance

After pushing to `origin/dev`:

1. Wait for GitHub Actions, Vercel, Railway API, and Railway Media Worker
   deployments to finish successfully.
2. Open the real development Dashboard at `https://dev-app.unipost.dev`.
3. Upload an opaque GIF and a transparent GIF.
4. Confirm a conversion-required destination presents **Convert to MP4**.
5. Confirm default white and custom Hex backgrounds.
6. Confirm success automatically replaces the GIF with a playable MP4.
7. Confirm the user must still click Publish.
8. Probe the stored output and verify:
   - MP4 container;
   - H.264 video;
   - `yuv420p`;
   - constant 30 FPS;
   - no audio;
   - even dimensions;
   - duration follows complete-loop rules.
9. Publish a generated MP4 through at least one real development account for a
   conversion-required platform.
10. Confirm X and Facebook direct GIF publishing still works with real
    development accounts.
11. Verify an active Processing usage prevents input cleanup.
12. Verify an unused source/output pair receives correct Plan deadlines.
13. Exercise or safely shorten a development-only deadline to verify the
    cleanup Worker removes the R2 object, pull copy, Media row, and usage rows.
14. Confirm the Admin Object Storage page reflects object creation and cleanup.
15. Run one Audio Overlay job and publish its output after the Worker migration.

The task is not complete until the real development environment matches this
expected outcome.

## 26. Acceptance criteria

The product is accepted when:

1. Every Plan can create GIF conversions within its fair-use limits.
2. The API accepts only an uploaded owned GIF Media ID.
3. A valid job returns a new uploaded `video/mp4` Media ID.
4. The output conforms to `universal_mp4_v1`.
5. Complete GIF cycles are preserved and never silently truncated.
6. Transparent GIFs use white by default and honor a valid custom Hex color.
7. Resource and decompression limits reject unsafe input with stable errors.
8. Conversion and publishing remain separate.
9. Dashboard conversion automatically replaces the GIF only after success.
10. X and Facebook direct GIF behavior is preserved.
11. LinkedIn and Threads remain documented as native GIF Coming soon.
12. Create Post validation rejects destination-incompatible MP4 output before
    dispatch.
13. Active Post and Processing references prevent Media cleanup.
14. Unused uploaded Media and processing outputs receive Plan-based deadlines.
15. Soft-deleted unused Media is actually removed by cleanup.
16. Audio Overlay uses the same lifecycle and continues to work.
17. SDKs expose create, get, wait, and upload-and-convert helpers.
18. Documentation explains direct GIF versus converted-video behavior.
19. Required local, CI, deployment, and real development checks pass.

## 27. References

Internal:

- [Publish GIFs to X and Facebook Guidance](../../../dashboard/src/app/docs/guides/publish-gifs/page.tsx)
- [Existing Media retention policy](../../../api/internal/mediaretention/policy.go)
- [Existing Post Media usage ledger](../../../api/internal/db/queries/media_post_usages.sql)
- [Existing Media cleanup Worker](../../../api/internal/worker/media_cleanup.go)
- [Existing Audio Overlay handler](../../../api/internal/handler/media_audio_overlays.go)
- [Existing Audio Overlay Worker](../../../api/internal/worker/media_audio_overlay.go)
- [R2 Media Retention PRD](../../prd-r2-media-retention-and-free-scheduled-cap.md)

Official platform references to revalidate during implementation:

- [TikTok Content Posting media transfer guide](https://developers.tiktok.com/doc/content-posting-api-media-transfer-guide)
- [Instagram content publishing](https://developers.facebook.com/docs/instagram-platform/content-publishing)
- [Pinterest API documentation](https://developers.pinterest.com/docs/api/v5/)
- [YouTube recommended upload encoding settings](https://support.google.com/youtube/answer/1722171)
- [Bluesky video upload tutorial](https://docs.bsky.app/docs/tutorials/video)

## 28. Approved decisions

The following product decisions were explicitly approved:

- asynchronous explicit conversion before publishing;
- uploaded UniPost GIF Media ID input only;
- one universal MP4 profile;
- full-cycle looping to at least five seconds, maximum 60 seconds;
- reject a source cycle longer than 60 seconds;
- API, SDK, and Dashboard at launch;
- white default transparency background with optional Hex color;
- 50 MB conversion input cap;
- all Plans with fair-use instead of conversion credits;
- polling without webhook;
- longest edge 1920, no upscaling, even dimensions;
- Dashboard automatically replaces the GIF but never auto-publishes;
- dedicated GIF endpoint backed by generalized Media Processing;
- unified lifecycle for GIF conversion and Audio Overlay;
- the stated Plan concurrency and rolling 24-hour limits;
- dedicated shared Media Worker;
- the stated SDK, error, validation, rollout, monitoring, and acceptance design.
