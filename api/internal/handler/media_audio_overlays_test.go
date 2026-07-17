package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/auth"
	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/storage"
)

func TestCreateAudioOverlayJobDefaultsAndQueues(t *testing.T) {
	store := newFakeAudioOverlayQueries()
	objectStore := &fakeAudioOverlayObjectStore{heads: map[string]storage.HeadResult{
		"media/vid.mp4":   {Exists: true, ContentType: "video/mp4", SizeBytes: 40_000_000},
		"media/audio.mp3": {Exists: true, ContentType: "audio/mpeg", SizeBytes: 5_000_000},
	}}
	h := NewMediaAudioOverlayHandler(store, objectStore)

	req := audioOverlayRequest(t, `{
		"video_media_id": "med_video",
		"audio_media_id": "med_audio",
		"mode": "replace",
		"audio_volume": 80,
		"audio_start_ms": 250,
		"fit": "loop_to_video"
	}`)
	rr := httptest.NewRecorder()

	h.Create(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202; body: %s", rr.Code, rr.Body.String())
	}
	if len(store.createParams) != 1 {
		t.Fatalf("CreateMediaProcessingJob calls = %d, want 1", len(store.createParams))
	}
	params := store.createParams[0]
	if params.Kind != "audio_overlay" || params.Status != "queued" {
		t.Fatalf("kind/status = %q/%q, want audio_overlay/queued", params.Kind, params.Status)
	}
	if !params.InputVideoMediaID.Valid || params.InputVideoMediaID.String != "med_video" ||
		!params.InputAudioMediaID.Valid || params.InputAudioMediaID.String != "med_audio" {
		t.Fatalf("nullable media inputs = %#v/%#v, want valid video/audio ids", params.InputVideoMediaID, params.InputAudioMediaID)
	}
	if params.Mode != "replace" || params.Fit != "loop_to_video" {
		t.Fatalf("mode/fit = %q/%q", params.Mode, params.Fit)
	}
	if params.VideoVolume != 100 || params.AudioVolume != 80 || params.AudioStartMs != 250 {
		t.Fatalf("volumes/start = %d/%d/%d, want 100/80/250", params.VideoVolume, params.AudioVolume, params.AudioStartMs)
	}
	if params.IdempotencyKey.Valid {
		t.Fatalf("idempotency key should be null when header omitted: %#v", params.IdempotencyKey)
	}

	var got audioOverlaySuccessEnvelope
	if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.Data.ID == "" || got.Data.Status != "queued" || got.Data.OutputMediaID != nil {
		t.Fatalf("unexpected response data: %#v", got.Data)
	}
}

func TestCreateAudioOverlayJobIdempotencyReplaysExistingJob(t *testing.T) {
	store := newFakeAudioOverlayQueries()
	objectStore := &fakeAudioOverlayObjectStore{heads: map[string]storage.HeadResult{
		"media/vid.mp4":   {Exists: true, ContentType: "video/mp4", SizeBytes: 40_000_000},
		"media/audio.mp3": {Exists: true, ContentType: "audio/mpeg", SizeBytes: 5_000_000},
	}}
	h := NewMediaAudioOverlayHandler(store, objectStore)

	first := audioOverlayRequest(t, `{"video_media_id":"med_video","audio_media_id":"med_audio"}`)
	first.Header.Set("Idempotency-Key", "idem-overlay-1")
	firstRR := httptest.NewRecorder()
	h.Create(firstRR, first)
	if firstRR.Code != http.StatusAccepted {
		t.Fatalf("first status = %d, want 202; body: %s", firstRR.Code, firstRR.Body.String())
	}

	second := audioOverlayRequest(t, `{"video_media_id":"med_video","audio_media_id":"med_audio"}`)
	second.Header.Set("Idempotency-Key", "idem-overlay-1")
	secondRR := httptest.NewRecorder()
	h.Create(secondRR, second)
	if secondRR.Code != http.StatusAccepted {
		t.Fatalf("second status = %d, want 202; body: %s", secondRR.Code, secondRR.Body.String())
	}
	if len(store.createParams) != 1 {
		t.Fatalf("CreateMediaProcessingJob calls = %d, want replay without duplicate create", len(store.createParams))
	}
}

func TestCreateAudioOverlayJobIdempotencyConflict(t *testing.T) {
	store := newFakeAudioOverlayQueries()
	store.existingByIdempotency = &db.MediaProcessingJob{
		ID:             "mpj_existing",
		WorkspaceID:    "ws_test",
		Kind:           "audio_overlay",
		Status:         "queued",
		RequestHash:    pgtype.Text{String: "different-request", Valid: true},
		IdempotencyKey: pgtype.Text{String: "idem-overlay-1", Valid: true},
		CreatedAt:      pgtype.Timestamptz{Time: time.Date(2026, 7, 3, 12, 0, 0, 0, time.UTC), Valid: true},
		UpdatedAt:      pgtype.Timestamptz{Time: time.Date(2026, 7, 3, 12, 0, 0, 0, time.UTC), Valid: true},
	}
	h := NewMediaAudioOverlayHandler(store, &fakeAudioOverlayObjectStore{})

	req := audioOverlayRequest(t, `{"video_media_id":"med_video","audio_media_id":"med_audio"}`)
	req.Header.Set("Idempotency-Key", "idem-overlay-1")
	rr := httptest.NewRecorder()

	h.Create(rr, req)

	if rr.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409; body: %s", rr.Code, rr.Body.String())
	}
	var got ErrorResponse
	if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if got.Error.NormalizedCode != "idempotency_conflict" {
		t.Fatalf("normalized code = %q, want idempotency_conflict", got.Error.NormalizedCode)
	}
}

func TestCreateAudioOverlayJobLazyHydratesPendingInputs(t *testing.T) {
	store := newFakeAudioOverlayQueries()
	store.media["med_video"] = overlayMediaWithStatus(store.media["med_video"], "pending")
	store.media["med_audio"] = overlayMediaWithStatus(store.media["med_audio"], "pending")
	objectStore := &fakeAudioOverlayObjectStore{
		heads: map[string]storage.HeadResult{
			"media/vid.mp4":   {Exists: true, ContentType: "video/mp4", SizeBytes: 41_000_000},
			"media/audio.mp3": {Exists: true, ContentType: "audio/mpeg", SizeBytes: 6_000_000},
		},
		videoMeta: map[string]storage.VideoMetadata{
			"media/vid.mp4": {Width: 1080, Height: 1920, DurationMS: 31_000},
		},
	}
	h := NewMediaAudioOverlayHandler(store, objectStore)

	rr := httptest.NewRecorder()
	h.Create(rr, audioOverlayRequest(t, `{"video_media_id":"med_video","audio_media_id":"med_audio"}`))

	if rr.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202; body: %s", rr.Code, rr.Body.String())
	}
	if len(store.markUploadedParams) != 2 {
		t.Fatalf("MarkMediaUploaded calls = %d, want 2", len(store.markUploadedParams))
	}
	if len(store.createParams) != 1 {
		t.Fatalf("CreateMediaProcessingJob calls = %d, want 1", len(store.createParams))
	}
	if got := store.media["med_video"]; got.Status != "uploaded" || got.SizeBytes != 41_000_000 || got.DurationMs.Int32 != 31_000 {
		t.Fatalf("hydrated video = %#v, want uploaded row with R2 metadata", got)
	}
	if got := store.media["med_audio"]; got.Status != "uploaded" || got.SizeBytes != 6_000_000 {
		t.Fatalf("hydrated audio = %#v, want uploaded row with R2 metadata", got)
	}
}

func TestCreateAudioOverlayJobPendingInputWithoutObjectReturnsMediaNotUploaded(t *testing.T) {
	store := newFakeAudioOverlayQueries()
	store.media["med_audio"] = overlayMediaWithStatus(store.media["med_audio"], "pending")
	h := NewMediaAudioOverlayHandler(store, &fakeAudioOverlayObjectStore{heads: map[string]storage.HeadResult{
		"media/vid.mp4": {Exists: true, ContentType: "video/mp4", SizeBytes: 40_000_000},
	}})

	rr := httptest.NewRecorder()
	h.Create(rr, audioOverlayRequest(t, `{"video_media_id":"med_video","audio_media_id":"med_audio"}`))

	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422; body: %s", rr.Code, rr.Body.String())
	}
	if len(store.createParams) != 0 {
		t.Fatalf("CreateMediaProcessingJob calls = %d, want 0", len(store.createParams))
	}
	var got ErrorResponse
	if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if len(got.Error.Issues) != 1 {
		t.Fatalf("issues = %#v, want one media_not_uploaded issue", got.Error.Issues)
	}
	issue := got.Error.Issues[0]
	if issue.Code != "media_not_uploaded" || issue.Field != "audio_media_id" {
		t.Fatalf("issue = %#v, want audio media_not_uploaded", issue)
	}
	actual, ok := issue.Actual.(map[string]any)
	if !ok {
		t.Fatalf("issue actual = %#v, want structured details", issue.Actual)
	}
	if actual["media_id"] != "med_audio" || actual["media_status"] != "pending" {
		t.Fatalf("issue actual media details = %#v, want pending med_audio", actual)
	}
	if actual["next_step"] != "PUT bytes to upload_url, then poll GET /v1/media/{media_id} until status=uploaded" {
		t.Fatalf("next_step = %#v, want upload/poll guidance", actual["next_step"])
	}
	if actual["docs_url"] != "https://unipost.dev/docs/api/media/reserve" {
		t.Fatalf("docs_url = %#v, want reserve docs", actual["docs_url"])
	}
}

func TestCreateAudioOverlayJobRejectsInvalidInputs(t *testing.T) {
	tests := []struct {
		name string
		body string
		code string
	}{
		{name: "missing video", body: `{"audio_media_id":"med_audio"}`, code: "video_media_id_required"},
		{name: "missing audio", body: `{"video_media_id":"med_video"}`, code: "audio_media_id_required"},
		{name: "bad mode", body: `{"video_media_id":"med_video","audio_media_id":"med_audio","mode":"duck"}`, code: "invalid_audio_overlay_mode"},
		{name: "bad fit", body: `{"video_media_id":"med_video","audio_media_id":"med_audio","fit":"stretch"}`, code: "invalid_audio_overlay_fit"},
		{name: "bad volume", body: `{"video_media_id":"med_video","audio_media_id":"med_audio","audio_volume":101}`, code: "invalid_audio_overlay_volume"},
		{name: "bad offset", body: `{"video_media_id":"med_video","audio_media_id":"med_audio","audio_start_ms":-1}`, code: "invalid_audio_overlay_offset"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := NewMediaAudioOverlayHandler(newFakeAudioOverlayQueries(), &fakeAudioOverlayObjectStore{})
			rr := httptest.NewRecorder()

			h.Create(rr, audioOverlayRequest(t, tc.body))

			if rr.Code != http.StatusUnprocessableEntity {
				t.Fatalf("status = %d, want 422; body: %s", rr.Code, rr.Body.String())
			}
			var got ErrorResponse
			if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
				t.Fatalf("decode error: %v", err)
			}
			if len(got.Error.Issues) != 1 || got.Error.Issues[0].Code != tc.code {
				t.Fatalf("issues = %#v, want code %s", got.Error.Issues, tc.code)
			}
		})
	}
}

func TestGetAudioOverlayJob(t *testing.T) {
	store := newFakeAudioOverlayQueries()
	store.jobByID = overlayJobFromParams(db.CreateMediaProcessingJobParams{
		WorkspaceID:       "ws_test",
		Kind:              "audio_overlay",
		Status:            "succeeded",
		InputVideoMediaID: pgtype.Text{String: "med_video", Valid: true},
		InputAudioMediaID: pgtype.Text{String: "med_audio", Valid: true},
		OutputMediaID:     pgtype.Text{String: "med_output", Valid: true},
		Mode:              "mix",
		Fit:               "trim_to_video",
		VideoVolume:       70,
		AudioVolume:       100,
		AudioStartMs:      0,
		RequestHash:       pgtype.Text{String: "hash", Valid: true},
		RequestJson:       []byte(`{}`),
	})
	h := NewMediaAudioOverlayHandler(store, &fakeAudioOverlayObjectStore{})
	req := httptest.NewRequest(http.MethodGet, "/v1/media/audio-overlays/mpj_done", nil)
	req = req.WithContext(auth.SetWorkspaceID(req.Context(), "ws_test"))
	rr := httptest.NewRecorder()

	h.Get(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rr.Code, rr.Body.String())
	}
	var got audioOverlaySuccessEnvelope
	if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.Data.Status != "succeeded" || got.Data.OutputMediaID == nil || *got.Data.OutputMediaID != "med_output" {
		t.Fatalf("unexpected response data: %#v", got.Data)
	}
}

type audioOverlaySuccessEnvelope struct {
	Data struct {
		ID            string  `json:"id"`
		Status        string  `json:"status"`
		VideoMediaID  string  `json:"video_media_id"`
		AudioMediaID  string  `json:"audio_media_id"`
		OutputMediaID *string `json:"output_media_id"`
		Mode          string  `json:"mode"`
		Fit           string  `json:"fit"`
	} `json:"data"`
}

func audioOverlayRequest(t *testing.T, body string) *http.Request {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/v1/media/audio-overlays", strings.NewReader(body))
	return req.WithContext(auth.SetWorkspaceID(req.Context(), "ws_test"))
}

type fakeAudioOverlayQueries struct {
	media                 map[string]db.Media
	createParams          []db.CreateMediaProcessingJobParams
	markUploadedParams    []db.MarkMediaUploadedParams
	existingByIdempotency *db.MediaProcessingJob
	jobByID               db.MediaProcessingJob
}

func newFakeAudioOverlayQueries() *fakeAudioOverlayQueries {
	return &fakeAudioOverlayQueries{media: map[string]db.Media{
		"med_video": {
			ID:          "med_video",
			WorkspaceID: "ws_test",
			StorageKey:  "media/vid.mp4",
			ContentType: "video/mp4",
			SizeBytes:   40_000_000,
			Status:      "uploaded",
			DurationMs:  pgtype.Int4{Int32: 30_000, Valid: true},
		},
		"med_audio": {
			ID:          "med_audio",
			WorkspaceID: "ws_test",
			StorageKey:  "media/audio.mp3",
			ContentType: "audio/mpeg",
			SizeBytes:   5_000_000,
			Status:      "uploaded",
			DurationMs:  pgtype.Int4{Int32: 30_000, Valid: true},
		},
	}}
}

func (f *fakeAudioOverlayQueries) GetMediaByIDAndWorkspace(_ context.Context, arg db.GetMediaByIDAndWorkspaceParams) (db.Media, error) {
	m, ok := f.media[arg.ID]
	if !ok || arg.WorkspaceID != "ws_test" {
		return db.Media{}, pgx.ErrNoRows
	}
	return m, nil
}

func (f *fakeAudioOverlayQueries) GetMediaProcessingJobByIdempotencyKey(_ context.Context, _ db.GetMediaProcessingJobByIdempotencyKeyParams) (db.MediaProcessingJob, error) {
	if f.existingByIdempotency == nil {
		return db.MediaProcessingJob{}, pgx.ErrNoRows
	}
	return *f.existingByIdempotency, nil
}

func (f *fakeAudioOverlayQueries) CreateMediaProcessingJob(_ context.Context, arg db.CreateMediaProcessingJobParams) (db.MediaProcessingJob, error) {
	f.createParams = append(f.createParams, arg)
	job := overlayJobFromParams(arg)
	f.existingByIdempotency = &job
	f.jobByID = job
	return job, nil
}

func (f *fakeAudioOverlayQueries) GetMediaProcessingJobByIDAndWorkspace(_ context.Context, arg db.GetMediaProcessingJobByIDAndWorkspaceParams) (db.MediaProcessingJob, error) {
	if f.jobByID.ID == "" || arg.ID != f.jobByID.ID || arg.WorkspaceID != f.jobByID.WorkspaceID {
		return db.MediaProcessingJob{}, pgx.ErrNoRows
	}
	return f.jobByID, nil
}

func (f *fakeAudioOverlayQueries) MarkMediaUploaded(_ context.Context, arg db.MarkMediaUploadedParams) (db.Media, error) {
	f.markUploadedParams = append(f.markUploadedParams, arg)
	row, ok := f.media[arg.ID]
	if !ok {
		return db.Media{}, pgx.ErrNoRows
	}
	row.Status = "uploaded"
	row.SizeBytes = arg.SizeBytes
	row.ContentType = arg.ContentType
	row.Width = arg.Width
	row.Height = arg.Height
	row.DurationMs = arg.DurationMs
	row.UploadedAt = pgtype.Timestamptz{Time: time.Date(2026, 7, 3, 12, 0, 0, 0, time.UTC), Valid: true}
	f.media[arg.ID] = row
	return row, nil
}

type fakeAudioOverlayObjectStore struct {
	heads     map[string]storage.HeadResult
	videoMeta map[string]storage.VideoMetadata
	err       error
}

func (f *fakeAudioOverlayObjectStore) Head(_ context.Context, key string) (storage.HeadResult, error) {
	if f.err != nil {
		return storage.HeadResult{}, f.err
	}
	head, ok := f.heads[key]
	if !ok {
		return storage.HeadResult{}, nil
	}
	return head, nil
}

func (f *fakeAudioOverlayObjectStore) ProbeVideo(_ context.Context, key string) (storage.VideoMetadata, error) {
	if f.err != nil {
		return storage.VideoMetadata{}, f.err
	}
	return f.videoMeta[key], nil
}

func overlayMediaWithStatus(row db.Media, status string) db.Media {
	row.Status = status
	if status == "pending" {
		row.UploadedAt = pgtype.Timestamptz{}
	}
	return row
}

func overlayJobFromParams(arg db.CreateMediaProcessingJobParams) db.MediaProcessingJob {
	now := pgtype.Timestamptz{Time: time.Date(2026, 7, 3, 12, 0, 0, 0, time.UTC), Valid: true}
	id := "mpj_1"
	if arg.Status == "succeeded" {
		id = "mpj_done"
	}
	return db.MediaProcessingJob{
		ID:                id,
		WorkspaceID:       arg.WorkspaceID,
		Kind:              arg.Kind,
		Status:            arg.Status,
		InputVideoMediaID: arg.InputVideoMediaID,
		InputAudioMediaID: arg.InputAudioMediaID,
		OutputMediaID:     arg.OutputMediaID,
		Mode:              arg.Mode,
		Fit:               arg.Fit,
		VideoVolume:       arg.VideoVolume,
		AudioVolume:       arg.AudioVolume,
		AudioStartMs:      arg.AudioStartMs,
		Request:           arg.RequestJson,
		IdempotencyKey:    arg.IdempotencyKey,
		RequestHash:       arg.RequestHash,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
}

var errUnexpectedAudioOverlayCall = errors.New("unexpected audio overlay query")
