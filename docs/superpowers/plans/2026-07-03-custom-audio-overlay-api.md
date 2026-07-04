# Custom Audio Overlay API Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the Custom Audio Overlay API from `docs/prd-custom-audio-overlay-api.md`, including optional media upload sizes, audio media inputs, asynchronous overlay jobs, FFmpeg worker processing, docs, and SDK release gates.

**Architecture:** Keep media preparation separate from publishing. Customers upload video/audio media, create an async overlay job, poll until it produces a normal output `media_id`, then publish that output through the existing posts API. The API web process owns reservation/status endpoints; a separate media-worker process owns FFprobe/FFmpeg work.

**Tech Stack:** Go API, chi handlers, sqlc/Postgres migrations, R2 storage helpers, FFmpeg/FFprobe via `exec.CommandContext`, Railway web + worker services, dashboard docs, JS/Python/Go/Java SDKs.

---

## File Map

- `api/internal/handler/media.go`: make `size_bytes` optional, allow audio MIME types, hydrate oversized uploads safely.
- `api/internal/handler/media_test.go`: TDD coverage for optional size, audio MIME allowlist, non-positive size rejection, and oversized hydration.
- `api/internal/platform/adapter.go`: add `MediaKindAudio`.
- `api/internal/platform/validate.go`: reject audio-only media in publish requests.
- `api/internal/platform/validate_test.go`: TDD coverage for audio media publish rejection.
- `api/internal/db/migrations/096_media_processing_jobs.sql`: create audio overlay job table without media FKs.
- `api/internal/db/queries/media_processing_jobs.sql`: sqlc queries for create/get/claim/finish/fail/retry.
- `api/internal/db/queries/media.sql`: add helpers for input cleanup hold and oversized media failure handling if needed.
- `api/internal/handler/media_audio_overlays.go`: new `POST /v1/media/audio-overlays` and `GET /v1/media/audio-overlays/{id}` handlers.
- `api/internal/handler/media_audio_overlays_test.go`: handler tests for create, get, validation, idempotency replay/conflict.
- `api/internal/worker/media_audio_overlay.go`: worker loop, FFprobe/FFmpeg command construction, stale recovery, temp file cleanup.
- `api/internal/worker/media_audio_overlay_test.go`: worker unit tests and command-construction tests.
- `api/internal/storage/media.go`: add download/upload helpers for worker inputs/outputs if existing helpers are insufficient.
- `api/cmd/api/main.go`: register new routes in the API process and gate media worker startup to a worker mode.
- `api/railway.toml` or new worker deployment config: document/add media worker start command and FFmpeg installation path.
- `dashboard/src/app/docs/api/media/reserve/page.tsx`: document `size_bytes` optional and audio MIME inputs.
- `dashboard/src/app/docs/api/media/audio-overlays/page.tsx`: new endpoint docs.
- `docs/sdk-api-coverage-matrix.md` and SDK release docs: add coverage rows and release notes.
- SDK repositories under `/Users/xiaoboyu/unipost-dev/sdk-*`: add helpers after backend dev verification passes.

---

### Task 1: Media Reserve Ergonomics and Audio Inputs

**Files:**
- Modify: `api/internal/handler/media.go`
- Modify: `api/internal/handler/media_test.go`
- Modify: `api/internal/platform/adapter.go`
- Modify: `api/internal/platform/validate.go`
- Modify: `api/internal/platform/validate_test.go`

- [ ] **Step 1: Write failing tests for optional `size_bytes` and audio MIME types**

Add tests in `api/internal/handler/media_test.go`:

```go
func TestAllowedMimeTypesIncludesAudioInputs(t *testing.T) {
	required := []string{
		"audio/mpeg", "audio/wav", "audio/x-wav",
		"audio/aac", "audio/mp4", "audio/x-m4a",
	}
	for _, m := range required {
		if !allowedMimeTypes[m] {
			t.Errorf("required audio mime %q not in allowlist", m)
		}
	}
}

func TestCreateAllowsOmittedSizeBytes(t *testing.T) {
	h := NewMediaHandler(nil, &storage.Client{})
	body := strings.NewReader(`{"filename":"clip.mp4","content_type":"video/mp4"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/media", body)
	req = req.WithContext(auth.SetWorkspaceID(req.Context(), "ws_test"))
	rr := httptest.NewRecorder()

	h.Create(rr, req)

	if rr.Code == http.StatusUnprocessableEntity && strings.Contains(rr.Body.String(), "size_bytes") {
		t.Fatalf("omitted size_bytes should no longer be rejected: %s", rr.Body.String())
	}
}

func TestCreateRejectsExplicitNonPositiveSizeBytes(t *testing.T) {
	h := NewMediaHandler(nil, &storage.Client{})
	body := strings.NewReader(`{"filename":"clip.mp4","content_type":"video/mp4","size_bytes":0}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/media", body)
	req = req.WithContext(auth.SetWorkspaceID(req.Context(), "ws_test"))
	rr := httptest.NewRecorder()

	h.Create(rr, req)

	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "size_bytes") {
		t.Fatalf("response should mention size_bytes, got %s", rr.Body.String())
	}
}
```

- [ ] **Step 2: Write failing publish-validation test for audio media**

Add in `api/internal/platform/validate_test.go`:

```go
func TestValidate_AudioMediaIDCannotBePublishedDirectly(t *testing.T) {
	res := ValidatePlatformPosts(ValidateOptions{
		Capabilities: stubCapabilities(),
		Accounts:     stubAccounts(),
		Media: map[string]ValidateMedia{
			"med_audio": {
				Status:      "uploaded",
				ContentType: "audio/mpeg",
				SizeBytes:   1234,
			},
		},
		Posts: []PlatformPostInput{
			{AccountID: "acc_twitter", Caption: "x", MediaIDs: []string{"med_audio"}},
		},
		Now: time.Date(2026, 7, 3, 12, 0, 0, 0, time.UTC),
	})
	hasError(t, res, 0, CodeAudioMediaNotPublishable)
}
```

- [ ] **Step 3: Run failing tests**

Run:

```bash
cd api
GOCACHE=/tmp/unipost-go-build go test ./internal/handler ./internal/platform
```

Expected: FAIL because audio MIME types, omitted `size_bytes`, and `CodeAudioMediaNotPublishable` are not implemented.

- [ ] **Step 4: Implement media reserve changes**

In `api/internal/handler/media.go`:

- Change request decoding to distinguish omitted `size_bytes` from explicit zero by using `*int64`.
- Add audio MIME types to `allowedMimeTypes`.
- Keep early cap validation when `size_bytes` is present.
- Create pending media rows with `size_bytes = 0` when omitted.
- Keep explicit non-positive `size_bytes` invalid.

- [ ] **Step 5: Implement audio media kind and publish validation**

In `api/internal/platform/adapter.go`, add:

```go
MediaKindAudio MediaKind = "audio"
```

Update `MediaFromContentType` to return `MediaKindAudio` for `audio/`.

In `api/internal/platform/validate.go`, add:

```go
CodeAudioMediaNotPublishable = "audio_media_not_publishable"
```

When validating loaded `media_ids`, if `MediaFromContentType(m.ContentType).Kind == MediaKindAudio`, append an error explaining audio media can be used for media processing but not published directly.

- [ ] **Step 6: Run tests green**

Run:

```bash
cd api
GOCACHE=/tmp/unipost-go-build go test ./internal/handler ./internal/platform
```

Expected: PASS.

- [ ] **Step 7: Commit Task 1**

```bash
git add api/internal/handler/media.go api/internal/handler/media_test.go api/internal/platform/adapter.go api/internal/platform/validate.go api/internal/platform/validate_test.go
git commit -m "feat: improve media upload inputs"
```

---

### Task 2: Media Processing Job Schema

**Files:**
- Create: `api/internal/db/migrations/096_media_processing_jobs.sql`
- Create: `api/internal/db/queries/media_processing_jobs.sql`
- Modify generated sqlc outputs after running sqlc.

- [ ] **Step 1: Write migration**

Create `096_media_processing_jobs.sql` with `media_processing_jobs`, no FK to `media(id)`, `idempotency_key`, `request_hash`, attempts, timestamps, and indexes matching the PRD.

- [ ] **Step 2: Write sqlc queries**

Create queries for:

- `CreateMediaProcessingJob`
- `GetMediaProcessingJobByIDAndWorkspace`
- `GetMediaProcessingJobByIdempotencyKey`
- `ClaimQueuedMediaProcessingJobs`
- `MarkMediaProcessingJobSucceeded`
- `MarkMediaProcessingJobFailed`
- `RequeueStaleMediaProcessingJobs`

- [ ] **Step 3: Run sqlc/generation**

Run the repo's existing sqlc generation command from `api/`.

- [ ] **Step 4: Run DB package tests**

```bash
cd api
GOCACHE=/tmp/unipost-go-build go test ./internal/db
```

- [ ] **Step 5: Commit Task 2**

```bash
git add api/internal/db
git commit -m "feat: add media processing job storage"
```

---

### Task 3: Audio Overlay API Handlers

**Files:**
- Create: `api/internal/handler/media_audio_overlays.go`
- Create: `api/internal/handler/media_audio_overlays_test.go`
- Modify: `api/cmd/api/main.go`

- [ ] **Step 1: Write handler tests**

Cover:

- missing auth returns 401
- invalid body returns 422
- missing video/audio IDs returns structured validation
- non-owned media returns 404 or validation
- idempotency replay returns original job
- idempotency conflict returns 409 `idempotency_conflict`
- success returns `202 Accepted` with `data` envelope

- [ ] **Step 2: Run failing handler tests**

```bash
cd api
GOCACHE=/tmp/unipost-go-build go test ./internal/handler -run AudioOverlay
```

- [ ] **Step 3: Implement handlers and routes**

Implement create/get response structs, validation, idempotency hash, input cleanup hold, and route registration under `/v1/media/audio-overlays`.

- [ ] **Step 4: Run handler tests green**

```bash
cd api
GOCACHE=/tmp/unipost-go-build go test ./internal/handler
```

- [ ] **Step 5: Commit Task 3**

```bash
git add api/internal/handler api/cmd/api/main.go
git commit -m "feat: add media audio overlay endpoints"
```

---

### Task 4: FFmpeg Worker

**Files:**
- Create: `api/internal/worker/media_audio_overlay.go`
- Create: `api/internal/worker/media_audio_overlay_test.go`
- Modify: `api/internal/storage/media.go`
- Modify: `api/cmd/api/main.go`

- [ ] **Step 1: Write command-builder tests**

Test replace/mix, no original audio branch, `trim_to_video`, `loop_to_video`, `normalize=0`, and no shell string construction.

- [ ] **Step 2: Write worker lifecycle tests**

Use fake storage and fake runner to prove claim -> processing -> uploaded output media -> succeeded, and timeout/failure -> failed.

- [ ] **Step 3: Run failing worker tests**

```bash
cd api
GOCACHE=/tmp/unipost-go-build go test ./internal/worker
```

- [ ] **Step 4: Implement worker**

Implement FFprobe metadata parsing, FFmpeg command creation, temp directory cleanup, R2 download/upload, output media hydration, retry policy, and stale processing recovery.

- [ ] **Step 5: Run worker tests green**

```bash
cd api
GOCACHE=/tmp/unipost-go-build go test ./internal/worker
```

- [ ] **Step 6: Commit Task 4**

```bash
git add api/internal/worker api/internal/storage api/cmd/api/main.go
git commit -m "feat: process media audio overlays"
```

---

### Task 5: Deployment and Docs

**Files:**
- Modify: `api/railway.toml` or add worker deployment docs/config
- Modify: `dashboard/src/app/docs/api/media/reserve/page.tsx`
- Create: `dashboard/src/app/docs/api/media/audio-overlays/page.tsx`
- Modify: `docs/sdk-api-coverage-matrix.md`

- [ ] **Step 1: Document worker deployment**

Pin FFmpeg/FFprobe install path and media worker start command.

- [ ] **Step 2: Update public docs**

Document optional `size_bytes`, audio MIME upload inputs, `POST /v1/media/audio-overlays`, job polling, and publish flow.

- [ ] **Step 3: Run docs build**

```bash
cd dashboard
npm run build
```

- [ ] **Step 4: Commit Task 5**

```bash
git add api/railway.toml dashboard/src/app/docs docs/sdk-api-coverage-matrix.md
git commit -m "docs: document audio overlay media flow"
```

---

### Task 6: Backend Validation and Dev Deployment

**Files:** no expected source edits unless tests reveal issues.

- [ ] **Step 1: Run backend tests**

```bash
cd api
GOCACHE=/tmp/unipost-go-build go test ./...
```

- [ ] **Step 2: Merge to local dev and rerun required checks**

Follow `AGENTS.md`: update local `dev`, merge this branch, rerun backend tests and dashboard build if docs changed.

- [ ] **Step 3: Push `origin/dev`**

Push only after tests pass on local `dev`.

- [ ] **Step 4: Monitor deployment and self-accept**

Wait for dev deployment, then verify:

- omitted `size_bytes` upload works
- audio upload works
- audio-only publish is rejected
- audio overlay job succeeds in dev
- output media publishes through a real dev social account

---

### Task 7: SDK Updates and Release

**Files:** SDK repos under `/Users/xiaoboyu/unipost-dev/sdk-js`, `sdk-python`, `sdk-go`, `sdk-java`.

- [ ] **Step 1: Add SDK helpers**

Add upload helpers that hide `size_bytes`, audio overlay create/get/wait helpers, and convenience upload-video + upload-audio + overlay flow.

- [ ] **Step 2: Run SDK source validation**

```bash
scripts/sdk-source-validation/run-suite.sh sdk-js
scripts/sdk-source-validation/run-suite.sh sdk-python
scripts/sdk-source-validation/run-suite.sh sdk-go
scripts/sdk-source-validation/run-suite.sh sdk-java
```

- [ ] **Step 3: Publish SDKs together**

Follow `docs/sdk-release.md` after backend dev verification passes.

