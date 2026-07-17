# GIF Media Processing Infrastructure Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Deliver Deployment A from the approved GIF-to-MP4 PRD: a kind-safe media worker, unified processing-media lifecycle records, retention/soft-delete enforcement, and a separately deployable Railway process—without exposing a GIF conversion API or allowing GIF jobs to be created yet.

**Architecture:** Extend the existing `media_processing_jobs` model so Audio Overlay and future GIF conversion jobs have explicit, mutually exclusive inputs. Add a processing-usage ledger that owns input/output retention while work is active or recently completed. Make workers claim one job kind only, move all media cleanup ownership into `MediaCleanupWorker`, and run media processing in an isolated `UNIPOST_PROCESS=media-worker` service while preserving a controlled API-process fallback during the rolling rollout.

**Tech Stack:** Go, pgx/sqlc, PostgreSQL migrations, Railway process modes, Go unit/integration tests.

---

## Scope and rollout invariant

Deployment A is infrastructure-only. It must satisfy all of the following before Deployment B begins:

- `POST /v1/media/gif-conversions` is absent and returns `404`.
- No application path can insert a `gif_to_mp4` job.
- The dedicated media worker is healthy and claims only explicitly configured job kinds.
- Old API instances using the generic claim query have drained before any GIF job kind can exist.
- API-process media workers can be disabled only after the dedicated media worker is verified healthy.
- Existing Audio Overlay creation, processing, output retention, and cleanup behavior remains functional.

## Task 1: Add the lifecycle migration and generated database model

**Files:**

- Create: `api/internal/db/migrations/117_media_processing_lifecycle.sql`
- Modify: `api/internal/db/migrate_test.go`
- Modify: `api/internal/db/queries/media_processing_jobs.sql`
- Create: `api/internal/db/queries/media_processing_usages.sql`
- Regenerate: `api/internal/db/generated/*`

- [x] Add a failing migration-source test asserting migration 117 contains:
  - nullable legacy audio input columns;
  - `input_media_id` for GIF jobs;
  - a named kind/input-shape constraint;
  - `media_processing_usages` with role, lifecycle status, and cleanup deadline;
  - backfill SQL for existing Audio Overlay jobs and unattached uploaded media.
- [x] Run `GOCACHE=/tmp/unipost-go-build go test ./internal/db -run 'Migration117' -count=1` from `api/` and confirm RED.
- [x] Create migration 117. Replace the old kind constraint with a named constraint equivalent to:

  ```sql
  CHECK (
    (kind = 'audio_overlay'
      AND input_video_media_id IS NOT NULL
      AND input_audio_media_id IS NOT NULL
      AND input_media_id IS NULL)
    OR
    (kind = 'gif_to_mp4'
      AND input_media_id IS NOT NULL
      AND input_video_media_id IS NULL
      AND input_audio_media_id IS NULL)
  )
  ```

- [x] Create `media_processing_usages` with one row per `(job_id, media_id, role)`, roles `input`/`output`, statuses `active`/`succeeded`/`failed`/`cancelled`, timestamps, and `cleanup_after_at`; add indexes for active-use checks and cleanup scans.
- [x] Backfill existing Audio Overlay input/output usage rows. Active jobs keep input usages active; terminal jobs receive plan-aware cleanup deadlines. Backfill a base plan-aware deadline for uploaded, unattached media that currently has none.
- [x] Add SQL queries needed to insert, transition, look up, and delete processing usage rows.
- [x] Run `/Users/xiaoboyu/go/bin/sqlc generate` from `api/`.
- [x] Re-run the focused database tests and confirm GREEN.
- [ ] Commit: `feat(media): add processing lifecycle schema`.

## Task 2: Make job claiming kind-specific

**Files:**

- Modify: `api/internal/db/queries/media_processing_jobs.sql`
- Modify: `api/internal/worker/media_audio_overlay.go`
- Modify: `api/internal/worker/media_audio_overlay_test.go`
- Regenerate: `api/internal/db/generated/*`

- [ ] Extend the Audio Overlay worker query mock with a failing expectation that the claim call receives `audio_overlay`; confirm the current generic claim path fails the test.
- [ ] Replace `ClaimMediaProcessingJobs` with `ClaimMediaProcessingJobsByKind(kind, batch_limit)`. Keep `FOR UPDATE SKIP LOCKED`, retry scheduling, and ordering unchanged, but add `kind = $1` to the claim predicate.
- [ ] Regenerate sqlc and update the worker interface/call site to pass `audio_overlay` explicitly.
- [ ] Add a database query test or source assertion proving one worker kind cannot claim another kind.
- [ ] Run `GOCACHE=/tmp/unipost-go-build go test ./internal/worker ./internal/db -count=1` and confirm GREEN.
- [ ] Commit: `fix(media): isolate processing job claims by kind`.

## Task 3: Preserve Audio Overlay across nullable input fields

**Files:**

- Modify: `api/internal/handler/media_audio_overlays.go`
- Modify: `api/internal/handler/media_audio_overlays_test.go`
- Modify: `api/internal/worker/media_audio_overlay.go`
- Modify: `api/internal/worker/media_audio_overlay_test.go`
- Modify: `api/internal/db/queries/media_processing_jobs.sql`

- [ ] Add failing handler tests proving Audio Overlay jobs still write both required audio/video IDs after sqlc changes them to nullable values.
- [ ] Add failing worker tests proving malformed Audio Overlay rows fail terminally instead of panicking or remaining stuck in `processing`.
- [ ] Update handler inserts to use valid nullable pgx values and update worker reads to validate and unwrap both inputs before processing.
- [ ] Keep the public Audio Overlay API response and idempotency behavior unchanged.
- [ ] Run `GOCACHE=/tmp/unipost-go-build go test ./internal/handler ./internal/worker -run 'AudioOverlay' -count=1` and confirm GREEN.
- [ ] Commit: `fix(media): preserve audio overlay input compatibility`.

## Task 4: Make processing retention transitions atomic

**Files:**

- Modify: `api/internal/db/queries/media_processing_jobs.sql`
- Modify: `api/internal/db/queries/media_processing_usages.sql`
- Modify: `api/internal/handler/media_audio_overlays.go`
- Modify: `api/internal/handler/media_audio_overlays_test.go`
- Modify: `api/internal/worker/media_audio_overlay.go`
- Modify: `api/internal/worker/media_audio_overlay_test.go`
- Modify: `api/internal/mediaretention/policy.go` only if a shared helper is required

- [ ] Add failing handler tests asserting job creation also creates two active input-usage rows and cannot leave a job without its usage records.
- [ ] Implement one SQL statement/transaction boundary that creates the Audio Overlay job and both active input usages atomically. Preserve idempotency lookup before creation and the existing idempotency uniqueness constraint.
- [ ] Add failing worker tests for terminal success and failure transitions:
  - success transitions both inputs to `succeeded`, inserts/updates the output usage as `succeeded`, assigns plan-aware deadlines, and marks the job succeeded atomically;
  - terminal failure transitions inputs to `failed`, assigns plan-aware deadlines, and marks the job failed atomically;
  - retryable failure keeps input usage active.
- [ ] Implement terminal transition queries as single atomic SQL statements. The worker computes the plan-aware retention deadline using the existing media-retention policy and passes it into the transition.
- [ ] Ensure a worker crash before the terminal transaction leaves the job retryable and its inputs protected by active usages.
- [ ] Run focused handler/worker tests and confirm GREEN.
- [ ] Commit: `feat(media): track processing retention atomically`.

## Task 5: Assign a base cleanup deadline when uploads become usable

**Files:**

- Modify: `api/internal/db/queries/media.sql`
- Modify: `api/internal/db/migrate_test.go` or add a focused query source test
- Regenerate: `api/internal/db/generated/*`

- [ ] Add a failing query-source or integration test proving `MarkMediaUploaded` assigns a plan-aware base `cleanup_after_at` when none exists and never shortens a later deadline.
- [ ] Update `MarkMediaUploaded` to derive the workspace's current plan and set the base success-retention deadline with `GREATEST(existing_deadline, plan_deadline)` semantics.
- [ ] Regenerate sqlc and run the focused database tests.
- [ ] Commit: `feat(media): set base upload retention deadline`.

## Task 6: Enforce unified cleanup and soft-delete rules

**Files:**

- Modify: `api/internal/db/queries/media.sql`
- Modify: `api/internal/db/queries/media_post_usages.sql`
- Modify: `api/internal/db/queries/media_cleanup_runs.sql`
- Modify: `api/internal/handler/media.go`
- Modify: `api/internal/handler/media_test.go`
- Modify: `api/internal/worker/media_cleanup.go`
- Modify: `api/internal/worker/media_cleanup_test.go`
- Regenerate: `api/internal/db/generated/*`

- [ ] Add failing cleanup query/source tests for all blockers:
  - base media deadline is due;
  - every post-usage deadline is due or absent;
  - no active processing usage exists;
  - every terminal processing-usage deadline is due or absent.
- [ ] Update cleanup-selection and admin backlog/deadline queries to use the same unified predicate, so operational reporting matches deletion behavior.
- [ ] Add failing DELETE handler tests: media referenced by an active post usage or active processing usage returns `409`; unreferenced media is soft-deleted.
- [ ] Add a database query that checks both active ledgers. Use it before soft delete, and change `SoftDeleteMedia` to set `cleanup_after_at = NOW()` without removing usage records.
- [ ] Add failing cleanup-worker tests proving a soft-deleted object is deleted only after all ledger blockers clear.
- [ ] Include a migration audit/backfill for historical soft-deleted rows that still have active references, preserving those objects until their ledgers clear.
- [ ] Regenerate sqlc; run handler, worker, and database tests.
- [ ] Commit: `feat(media): unify retention and deletion gates`.

## Task 7: Move abandoned-upload cleanup into MediaCleanupWorker

**Files:**

- Modify: `api/internal/worker/analytics_refresh.go`
- Modify: `api/internal/worker/media_cleanup.go`
- Modify: `api/internal/worker/media_cleanup_test.go`
- Modify: `api/cmd/api/main.go`
- Modify tests that construct either worker

- [ ] Add failing `MediaCleanupWorker` tests proving pending uploads older than seven days are swept on startup/hourly while normal retention cleanup remains daily.
- [ ] Move the `ListAbandonedMedia` sweep and storage deletion logic from `AnalyticsRefreshWorker` into `MediaCleanupWorker`.
- [ ] Give the cleanup worker separate hourly abandoned-upload and daily retention schedules; both run once at startup and respect context cancellation.
- [ ] Remove media-storage cleanup responsibilities and constructor dependencies from `AnalyticsRefreshWorker`.
- [ ] Update application wiring and all constructor tests.
- [ ] Run `GOCACHE=/tmp/unipost-go-build go test ./internal/worker ./cmd/api -count=1` and confirm GREEN.
- [ ] Commit: `refactor(media): centralize object cleanup ownership`.

## Task 8: Add the dedicated Railway media-worker process mode

**Files:**

- Modify: `api/cmd/api/main.go`
- Modify: `api/cmd/api/main_process_mode_test.go`
- Modify: `api/railway.toml`
- Modify: `api/README.md` or the existing process-mode operations document

- [ ] Add failing process-mode tests for:
  - normalization of `UNIPOST_PROCESS=media-worker`;
  - a dedicated database pool limit (`MEDIA_PROCESSING_WORKER_DATABASE_MAX_CONNS`);
  - media worker mode starting Audio Overlay and media cleanup workers but not the public API server or unrelated background workers;
  - API mode fallback being controllable by an environment variable and defaulting on for the rolling Deployment A transition;
  - `/health` remaining available in worker mode.
- [ ] Add `media-worker` to the existing process-mode switch and share the existing signal/shutdown/health-server pattern used by `post-delivery-worker`.
- [ ] Add `shouldStartMediaProcessingWorkers`. In API mode, honor a single documented fallback environment variable; in media-worker mode always start the media-processing and cleanup workers.
- [ ] Document the Railway service command/process configuration, health endpoint, database pool variable, and rollout order in `api/railway.toml` comments and the operations document.
- [ ] Run `GOCACHE=/tmp/unipost-go-build go test ./cmd/api ./internal/worker -count=1` and confirm GREEN.
- [ ] Commit: `feat(media): add dedicated processing worker mode`.

## Task 9: Prove Deployment A cannot expose GIF conversion

**Files:**

- Modify: `api/cmd/api/main_test.go` or add a focused route-registration test
- Modify: `api/internal/db/migrate_test.go` if needed

- [ ] Add a test that constructs the API router and proves `POST /v1/media/gif-conversions` returns `404` in Deployment A.
- [ ] Add a source/query assertion that production Go code has no callable insert path using `kind = 'gif_to_mp4'` yet; the migration may recognize the future kind, but no handler may create it.
- [ ] Run the focused tests and confirm GREEN.
- [ ] Commit: `test(media): lock deployment a gif gate`.

## Task 10: Validate the task branch

- [ ] Run `/Users/xiaoboyu/go/bin/sqlc generate` from `api/` and confirm `git diff --exit-code` after generated changes are committed.
- [ ] Run `GOCACHE=/tmp/unipost-go-build go test ./...` from `api/`.
- [ ] Run any repository CI-equivalent lint/build command that covers the API if configured.
- [ ] Review the complete diff against the Deployment A scope. Confirm there is no GIF conversion handler, route, UI, SDK method, or insertion path.
- [ ] Use `superpowers:verification-before-completion` before claiming local readiness.

## Task 11: Integrate into dev and pass the real Deployment A gate

- [ ] Fetch `origin`; update local `dev` from the latest `origin/dev`; merge the task branch into local `dev` without including unrelated untracked files.
- [ ] Re-run `GOCACHE=/tmp/unipost-go-build go test ./...` from `api/` on the merged local `dev`.
- [ ] Push local `dev` to `origin/dev`.
- [ ] Monitor every triggered GitHub, Railway, and Vercel check/deployment until terminal success. Inspect and fix any in-scope failures before proceeding.
- [ ] Configure/verify the Railway development `media-worker` service with `UNIPOST_PROCESS=media-worker`, its health check, and its dedicated DB pool limit.
- [ ] Wait until all pre-Deployment-A API instances have drained. Verify the dedicated worker is healthy and logs kind-aware Audio Overlay claim activity.
- [ ] Disable the API-process media-worker fallback in development only after the dedicated worker is healthy; redeploy and monitor again.
- [ ] In the real development environment verify:
  - `https://dev-api.unipost.dev/health` is healthy;
  - the worker health endpoint is healthy through Railway;
  - a real Audio Overlay request completes and its input/output processing usages transition correctly;
  - `POST https://dev-api.unipost.dev/v1/media/gif-conversions` returns `404`;
  - cleanup/admin queries expose no unexpected overdue active rows;
  - no old generic media-job claimant remains deployed.
- [ ] Only after every check passes, mark Deployment A complete and begin a fresh Deployment B plan from the then-current `origin/dev` database/query signatures.

## Rollback

- Re-enable the API-process media-worker fallback first if the dedicated worker becomes unhealthy.
- Do not insert GIF jobs during rollback; Deployment A contains no such application path.
- Migration 117 is additive except for replacing the job CHECK and relaxing legacy input nullability. Roll back application processes without dropping lifecycle data; repair worker/query behavior forward.
- Never remove processing-usage rows to force cleanup. If retention is blocked, investigate the owning job and ledger state.
