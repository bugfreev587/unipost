# GIF-to-MP4 Deployment B Implementation Plan

> Execute with test-driven development from `dev-gif-to-mp4-backend`, based on `origin/dev` at `8b889f4f` or a later merged descendant. Deployment A is already live in development: migration 117 is applied, the dedicated `media-worker` is healthy, API-process media claiming is disabled, and a real Audio Overlay job completed through the dedicated worker.

**Goal:** Ship the GIF conversion backend, worker processing, stable public API, fair-use enforcement, SDK helpers, and API Reference without adding Dashboard conversion UI or automatic publishing.

**Architecture:** Reuse `media_processing_jobs` and `media_processing_usages`. A centralized admission service serializes per-workspace capacity checks and job creation. A single media-processing coordinator owns the worker execution slot and alternates eligible `audio_overlay` and `gif_to_mp4` jobs. GIF safety preflight parses bounded metadata before FFprobe/FFmpeg. Successful output is a normal hydrated `video/mp4` Media row protected by Plan-based processing usage.

**Scope gate:** Deployment B registers `POST/GET /v1/media/gif-conversions` and may insert `kind = 'gif_to_mp4'`. It does not change composer UI, replace GIFs in drafts, or publish automatically. No feature flag.

## Task 1: Lock the Deployment B contracts in tests

**Files:**

- Create: `api/internal/handler/media_gif_conversions_test.go`
- Create: `api/internal/worker/gif_preflight_test.go`
- Create: `api/internal/worker/media_processing_coordinator_test.go`
- Modify: `api/cmd/api/gif_deployment_gate_test.go`

- [ ] Replace the Deployment A route-absence assertion with a Deployment B route-registration contract.
- [ ] Add failing handler tests for request normalization, uppercase/default background, ownership-safe errors, hydration, idempotent replay/conflict, create/get response shape, and stable terminal errors.
- [ ] Add failing preflight tests for dimensions, frames, cycle duration, decoded-pixel budget, malformed/truncated data, parser bounds, static GIFs, and boundary values.
- [ ] Add failing coordinator tests proving one global slot, kind-aware claims, round-robin fairness, retry promotion, and stale recovery.
- [ ] Confirm RED with focused `go test` commands before production code.

## Task 2: Add centralized Media Processing Plan policy and atomic admission

**Files:**

- Create: `api/internal/mediaprocessing/policy.go`
- Create: `api/internal/mediaprocessing/policy_test.go`
- Modify: `api/internal/db/queries/media_processing_jobs.sql`
- Modify: `api/internal/db/media_processing_lifecycle_contract_test.go`
- Modify: `api/internal/handler/media_audio_overlays.go`
- Modify: `api/internal/handler/media_audio_overlays_test.go`

- [ ] Encode active and rolling-24-hour limits for Free/API/Basic/Growth/Team/Enterprise in one policy package; use the existing entitlement/contract override path for Enterprise.
- [ ] Add a transactionally serialized per-workspace admission path using a PostgreSQL transaction advisory lock.
- [ ] Count `queued`, `retry_wait`, and `processing` as active so backoff cannot bypass capacity.
- [ ] Count only newly created `gif_to_mp4` jobs toward the rolling GIF limit; exclude idempotent replay and worker retries.
- [ ] Return typed admission outcomes with `Retry-After` and rolling `reset_at` inputs.
- [ ] Apply the shared active cap to Audio Overlay creation without charging its rolling GIF limit.
- [ ] Add concurrency tests proving simultaneous requests cannot oversubscribe either limit.

## Task 3: Implement GIF create/get API and lifecycle insertion

**Files:**

- Create: `api/internal/handler/media_gif_conversions.go`
- Modify: `api/internal/db/queries/media_processing_jobs.sql`
- Modify: `api/cmd/api/main.go`
- Modify: relevant generated `api/internal/db/*.sql.go`

- [ ] Accept only owned uploaded `image/gif` Media, hydrate pending rows with R2 `HEAD`, enforce actual size `<= 50 MB`, and reject missing objects without decoding in the API process.
- [ ] Normalize `background_color` to uppercase six-digit `#RRGGBB`, default `#FFFFFF`, and include fixed `output_profile=universal_mp4_v1` in request JSON/hash.
- [ ] Preserve optional `Idempotency-Key` semantics and ownership-safe GET `404` behavior.
- [ ] Atomically insert the GIF job and its active input processing usage through the admission transaction.
- [ ] Map public errors exactly: media ownership/unavailability, validation, capacity, rolling limit, idempotency conflict, and infrastructure unavailable.
- [ ] Register POST before the generic `/v1/media/{id}` route and GET `/v1/media/gif-conversions/{id}`.
- [ ] Run sqlc and focused handler/database tests.

## Task 4: Build bounded GIF metadata preflight

**Files:**

- Create: `api/internal/worker/gif_preflight.go`
- Expand: `api/internal/worker/gif_preflight_test.go`
- Add: small generated test fixtures under `api/internal/worker/testdata/gif/` only when source construction in tests is insufficient

- [ ] Parse GIF header, logical screen descriptor, color tables, extensions, frame descriptors, delays, and data sub-blocks without materializing decoded canvases.
- [ ] Enforce width/height `<=4096`, frames `<=2000`, decoded pixels `<=1.5B`, one complete cycle `<=60s`, compressed bytes `<=50 MB`, context/time checks, and bounded allocation.
- [ ] Treat a valid single-frame/static GIF as a five-second source; normalize zero delays conservatively and document the rule in code/tests.
- [ ] Return stable typed errors before FFprobe/FFmpeg can run.
- [ ] Test exact boundaries plus malformed, truncated, oversized-subblock, and decompression-bomb-shaped inputs.

## Task 5: Implement universal MP4 rendering and validation

**Files:**

- Create: `api/internal/worker/gif_processor.go`
- Create: `api/internal/worker/gif_processor_test.go`
- Modify: `api/internal/storage/r2.go`
- Modify: `api/internal/storage/r2_test.go`

- [ ] Download into a private per-job temp directory with a hard compressed-byte limit; never use user filenames or log signed URLs/paths.
- [ ] Run preflight before FFprobe/FFmpeg and enforce a five-minute context timeout.
- [ ] Build FFmpeg arguments for H.264/libx264, `yuv420p`, constant 30 FPS, no audio, faststart, no upscaling, longest edge `<=1920`, even dimensions, and opaque background compositing.
- [ ] Render complete cycles: one cycle for `D>=5s`, otherwise `ceil(5s/D)` cycles; never truncate a cycle.
- [ ] Probe the local result and reject wrong container/codec/pixel format/FPS/audio/dimensions/duration or global Media hard-cap overflow.
- [ ] Remove all temp artifacts on success, failure, timeout, and cancellation.
- [ ] Unit-test arguments/profile math; integration-test real FFmpeg fixtures when binaries are present.

## Task 6: Add GIF worker success/failure lifecycle

**Files:**

- Create: `api/internal/worker/media_gif_conversion.go`
- Create: `api/internal/worker/media_gif_conversion_test.go`
- Modify: `api/internal/db/queries/media_processing_usages.sql`
- Modify: `api/internal/db/queries/media_processing_jobs.sql`
- Modify generated sqlc files

- [ ] Claim only `gif_to_mp4` jobs and validate `input_media_id`/request fields defensively.
- [ ] Create pending output Media, assign `media/<id>.mp4`, upload, HEAD/probe, and hydrate accurate size/dimensions/duration/content type.
- [ ] In one terminal transaction transition input usage, add output usage, write Plan deadlines, and mark the job succeeded.
- [ ] Classify stable public errors and retry only transient R2/database/process timeout failures through existing bounded backoff.
- [ ] Compensating-delete incomplete output objects/rows; leave a pending row only when compensation itself fails so abandoned cleanup owns it.
- [ ] Never expose partial `output_media_id`.

## Task 7: Replace separate loops with one fair coordinator and stale recovery

**Files:**

- Create: `api/internal/worker/media_processing_coordinator.go`
- Expand: `api/internal/worker/media_processing_coordinator_test.go`
- Modify: `api/internal/worker/media_audio_overlay.go`
- Modify: `api/internal/worker/media_audio_overlay_test.go`
- Modify: `api/internal/db/queries/media_processing_jobs.sql`
- Modify: `api/cmd/api/main.go`

- [ ] Make Audio Overlay and GIF processors consume already-claimed jobs rather than owning independent claim batches.
- [ ] Default concurrency to one execution slot and alternate kind priority after each successful claim attempt.
- [ ] Promote due retries by kind and use `FOR UPDATE SKIP LOCKED` kind claims.
- [ ] Atomically recover `processing` jobs older than five minutes: requeue when attempts remain; terminally fail with `media_processing_worker_lost`/`processing_timeout` and release usage when exhausted.
- [ ] Emit structured logs/metrics for claims, queue wait, processing duration, outcome/error code, retries, stale recovery, bytes, and heartbeat without customer secrets.
- [ ] Preserve Audio Overlay real behavior and Deployment A process-mode isolation tests.

## Task 8: Audit destination video validation

**Files:**

- Modify: `api/internal/platform/validate.go`
- Modify: platform validation tests for Instagram, TikTok, Pinterest, YouTube, and Bluesky

- [ ] Compare `universal_mp4_v1` metadata with every conversion-required destination's existing validation.
- [ ] Add only missing minimum dimension/duration/content constraints needed to reject incompatible outputs before dispatch.
- [ ] Keep X/Facebook direct GIF validation unchanged.
- [ ] Return actionable normalized validation issues.

## Task 9: Add API Reference and backend-facing Guidance updates

**Files:**

- Create: `dashboard/src/app/docs/api/media/gif-conversions/page.tsx`
- Modify: docs API index, docs shell, inline-link map, search index, Media/Get Media/Create Post references
- Modify: `dashboard/src/app/docs/guides/publish-gifs/page.tsx`
- Modify: Video + Audio Overlay docs for shared active capacity/lifecycle
- Create/modify source regression tests under `dashboard/tests/`

- [ ] Document create/get requests, job states, limits, stable errors, idempotency, polling, conversion/publishing separation, and Plan retention.
- [ ] Include JavaScript, Python, Go, and Java examples and links between upload, conversion, Media, and Create Post pages.
- [ ] Keep X and Facebook native GIF guidance; keep LinkedIn/Threads native support as Coming soon; describe MP4 conversion for unsupported native-GIF destinations.
- [ ] Document the new shared active cap for Audio Overlay.
- [ ] Build dashboard and run docs source tests.

## Task 10: Implement SDK methods in all four source repositories

**Repositories:**

- `/Users/xiaoboyu/unipost-dev/sdk-js`
- `/Users/xiaoboyu/unipost-dev/sdk-python`
- `/Users/xiaoboyu/unipost-dev/sdk-go`
- `/Users/xiaoboyu/unipost-dev/sdk-java`
- Update `scripts/sdk-source-validation/` in this repository

- [ ] Inspect and obey each SDK repository's `AGENTS.md` before changes; create its own task branch from its normal development base.
- [ ] Add typed create/get models and `createGifConversion`, `getGifConversion`, `waitForGifConversion`, and `uploadAndConvertGif` equivalents.
- [ ] Poll every two seconds by default, support custom interval/timeout and runtime cancellation, never cancel the server job, and raise typed terminal errors.
- [ ] Let callers supply idempotency keys; high-level helpers may generate one.
- [ ] Add unit tests and extend source-validation suites without publishing packages.

## Task 11: Validate, review, integrate, and pass the real Deployment B gate

- [ ] Run sqlc generation and `GOCACHE=/tmp/unipost-go-build go test ./...`, build, and vet from `api/`.
- [ ] Run dashboard docs tests, `npm run build`, and dashboard regression when browsers are installed.
- [ ] Run all four SDK unit/source validations.
- [ ] Perform independent Critical/Important code review and fix accepted findings.
- [ ] Update local `dev` from latest `origin/dev`, merge the task branch, rerun required checks, and push `dev` to `origin/dev`.
- [ ] Monitor GitHub, Railway API/post/media-worker, Vercel, and visible SDK checks until terminal success.
- [ ] In real dev, create opaque/transparent/static/short-loop/odd-dimension conversions; verify profile with FFprobe and stable rejection fixtures.
- [ ] Verify idempotency, ownership isolation, capacity/rate limits, stale recovery, retry classification, no Audio Overlay cross-claim, and Audio Overlay regression.
- [ ] Verify active usage blocks cleanup, Free/paid deadlines are correct, and a safely shortened dev-only deadline removes R2 object, Media row, and usage.
- [ ] Confirm Admin Object Storage metrics reflect creation/cleanup.
- [ ] Record that unauthenticated POST changed from Deployment A's generic-route `405` to the authenticated GIF conversion route; no Dashboard conversion UI is part of Deployment B.

## Rollback

- Stop accepting new GIF jobs while leaving GET status and existing worker processing available.
- If the GIF processor is unhealthy, stop GIF claims without stopping Audio Overlay claims.
- Do not delete queued jobs or terminal outputs during rollback; lifecycle retention remains authoritative.
- Re-enable API-process media claiming only if the dedicated worker itself becomes unhealthy.
