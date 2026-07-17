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
	"github.com/xiaoboyu/unipost-api/internal/mediaprocessing"
	"github.com/xiaoboyu/unipost-api/internal/storage"
)

func TestCreateGIFConversionNormalizesDefaultsAndQueues(t *testing.T) {
	queries := &fakeGIFConversionQueries{media: map[string]db.Media{
		"med_gif": gifTestMedia("med_gif", "uploaded", "image/gif", 1234),
	}}
	admitter := &fakeGIFAdmitter{result: mediaprocessing.AdmissionResult{
		Decision: mediaprocessing.AdmissionDecision{Code: mediaprocessing.AdmissionAccepted},
		Job:      gifTestJob("job_1", "queued", []byte(`{"gif_media_id":"med_gif","background_color":"#FFFFFF","output_profile":"universal_mp4_v1"}`)),
	}}
	h := NewMediaGIFConversionHandler(queries, &fakeGIFObjectStore{head: storage.HeadResult{Exists: true, ContentType: "image/gif", SizeBytes: 1234}}, admitter)
	recorder := performGIFRequest(t, h.Create, http.MethodPost, "/v1/media/gif-conversions", `{"gif_media_id":" med_gif "}`)
	if recorder.Code != http.StatusAccepted {
		t.Fatalf("status = %d body=%s", recorder.Code, recorder.Body.String())
	}
	if len(admitter.requests) != 1 || admitter.requests[0].InputMediaID != "med_gif" || !strings.Contains(string(admitter.requests[0].RequestJSON), `"background_color":"#FFFFFF"`) {
		t.Fatalf("admission requests = %#v", admitter.requests)
	}
	var payload struct {
		Data mediaGIFConversionResponse `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Data.Kind != "gif_to_mp4" || payload.Data.BackgroundColor != "#FFFFFF" || payload.Data.OutputProfile != "universal_mp4_v1" || payload.Data.Status != "queued" {
		t.Fatalf("response = %#v", payload.Data)
	}
}

func TestCreateGIFConversionRejectsInvalidInputBeforeAdmission(t *testing.T) {
	tests := []struct {
		name   string
		body   string
		media  db.Media
		head   storage.HeadResult
		status int
		code   string
	}{
		{name: "missing id", body: `{}`, status: 422, code: "gif_media_required"},
		{name: "background", body: `{"gif_media_id":"med_gif","background_color":"white"}`, status: 422, code: "invalid_background_color"},
		{name: "wrong type", body: `{"gif_media_id":"med_gif"}`, media: gifTestMedia("med_gif", "uploaded", "image/png", 12), head: storage.HeadResult{Exists: true, ContentType: "image/png", SizeBytes: 12}, status: 422, code: "gif_media_required"},
		{name: "too large", body: `{"gif_media_id":"med_gif"}`, media: gifTestMedia("med_gif", "uploaded", "image/gif", gifConversionMaxBytes+1), head: storage.HeadResult{Exists: true, ContentType: "image/gif", SizeBytes: gifConversionMaxBytes + 1}, status: 422, code: "gif_size_exceeded"},
		{name: "object missing", body: `{"gif_media_id":"med_gif"}`, media: gifTestMedia("med_gif", "uploaded", "image/gif", 12), head: storage.HeadResult{}, status: 409, code: "input_media_unavailable"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			queries := &fakeGIFConversionQueries{media: map[string]db.Media{}}
			if tt.media.ID != "" {
				queries.media[tt.media.ID] = tt.media
			}
			admitter := &fakeGIFAdmitter{}
			h := NewMediaGIFConversionHandler(queries, &fakeGIFObjectStore{head: tt.head}, admitter)
			recorder := performGIFRequest(t, h.Create, http.MethodPost, "/v1/media/gif-conversions", tt.body)
			assertGIFError(t, recorder, tt.status, tt.code)
			if len(admitter.requests) != 0 {
				t.Fatalf("admission called: %#v", admitter.requests)
			}
		})
	}
}

func TestCreateGIFConversionUsesOwnershipSafeNotFound(t *testing.T) {
	h := NewMediaGIFConversionHandler(&fakeGIFConversionQueries{media: map[string]db.Media{}}, &fakeGIFObjectStore{}, &fakeGIFAdmitter{})
	recorder := performGIFRequest(t, h.Create, http.MethodPost, "/v1/media/gif-conversions", `{"gif_media_id":"other_workspace"}`)
	assertGIFError(t, recorder, 404, "media_not_found")
}

func TestCreateGIFConversionIdempotentReplayDoesNotRevalidateOrReadmit(t *testing.T) {
	normalized := normalizedGIFConversionRequest{GIFMediaID: "med_gif", BackgroundColor: "#FFFFFF", OutputProfile: "universal_mp4_v1"}
	requestJSON, _ := json.Marshal(normalized)
	_, requestHash, err := gifConversionRequestHash(normalized)
	if err != nil {
		t.Fatal(err)
	}
	existing := gifTestJob("job_existing", "succeeded", requestJSON)
	existing.RequestHash = pgtype.Text{String: requestHash, Valid: true}
	queries := &fakeGIFConversionQueries{idempotency: map[string]db.MediaProcessingJob{"key_1": existing}}
	admitter := &fakeGIFAdmitter{}
	h := NewMediaGIFConversionHandler(queries, &fakeGIFObjectStore{err: errors.New("must not HEAD")}, admitter)
	req := httptest.NewRequest(http.MethodPost, "/v1/media/gif-conversions", strings.NewReader(`{"gif_media_id":"med_gif"}`))
	req.Header.Set("Idempotency-Key", "key_1")
	req = req.WithContext(auth.SetWorkspaceID(req.Context(), "ws_1"))
	recorder := httptest.NewRecorder()
	h.Create(recorder, req)
	if recorder.Code != http.StatusAccepted || len(admitter.requests) != 0 {
		t.Fatalf("status=%d body=%s admissions=%d", recorder.Code, recorder.Body.String(), len(admitter.requests))
	}
}

func TestCreateGIFConversionMapsAdmissionDecisions(t *testing.T) {
	now := time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC)
	for _, tt := range []struct {
		name     string
		decision mediaprocessing.AdmissionDecision
		status   int
		code     string
	}{
		{name: "conflict", decision: mediaprocessing.AdmissionDecision{Code: mediaprocessing.AdmissionIdempotentConflict}, status: 409, code: "idempotency_conflict"},
		{name: "capacity", decision: mediaprocessing.AdmissionDecision{Code: mediaprocessing.AdmissionCapacityExceeded, RetryAfter: 30 * time.Second}, status: 429, code: "media_processing_capacity_exceeded"},
		{name: "rolling", decision: mediaprocessing.AdmissionDecision{Code: mediaprocessing.AdmissionGIFRateExceeded, RetryAfter: time.Hour, ResetAt: now.Add(time.Hour)}, status: 429, code: "gif_conversion_rate_limit_exceeded"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			queries := &fakeGIFConversionQueries{media: map[string]db.Media{"med_gif": gifTestMedia("med_gif", "uploaded", "image/gif", 12)}}
			h := NewMediaGIFConversionHandler(queries, &fakeGIFObjectStore{head: storage.HeadResult{Exists: true, ContentType: "image/gif", SizeBytes: 12}}, &fakeGIFAdmitter{result: mediaprocessing.AdmissionResult{Decision: tt.decision}})
			recorder := performGIFRequest(t, h.Create, http.MethodPost, "/v1/media/gif-conversions", `{"gif_media_id":"med_gif"}`)
			assertGIFError(t, recorder, tt.status, tt.code)
			if tt.status == 429 && recorder.Header().Get("Retry-After") == "" {
				t.Fatal("Retry-After missing")
			}
		})
	}
}

func TestGetGIFConversionHidesOtherKindsAndWorkspaces(t *testing.T) {
	queries := &fakeGIFConversionQueries{jobs: map[string]db.MediaProcessingJob{
		"gif":   gifTestJob("gif", "succeeded", []byte(`{"gif_media_id":"med_gif","background_color":"#FFFFFF","output_profile":"universal_mp4_v1"}`)),
		"audio": {ID: "audio", Kind: "audio_overlay"},
	}}
	h := NewMediaGIFConversionHandler(queries, &fakeGIFObjectStore{}, &fakeGIFAdmitter{})
	ok := performGIFRequest(t, h.Get, http.MethodGet, "/v1/media/gif-conversions/gif", "")
	if ok.Code != 200 || !strings.Contains(ok.Body.String(), `"output_profile":"universal_mp4_v1"`) {
		t.Fatalf("status=%d body=%s", ok.Code, ok.Body.String())
	}
	missing := performGIFRequest(t, h.Get, http.MethodGet, "/v1/media/gif-conversions/audio", "")
	assertGIFError(t, missing, 404, "not_found")
}

func performGIFRequest(t *testing.T, fn http.HandlerFunc, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req = req.WithContext(auth.SetWorkspaceID(req.Context(), "ws_1"))
	recorder := httptest.NewRecorder()
	fn(recorder, req)
	return recorder
}

func assertGIFError(t *testing.T, recorder *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	if recorder.Code != status || !strings.Contains(recorder.Body.String(), `"code":"`+code+`"`) {
		t.Fatalf("status=%d body=%s want status=%d code=%s", recorder.Code, recorder.Body.String(), status, code)
	}
}

func gifTestMedia(id, status, contentType string, size int64) db.Media {
	return db.Media{ID: id, WorkspaceID: "ws_1", StorageKey: "media/" + id + ".gif", Status: status, ContentType: contentType, SizeBytes: size}
}

func gifTestJob(id, status string, request []byte) db.MediaProcessingJob {
	return db.MediaProcessingJob{ID: id, WorkspaceID: "ws_1", Kind: "gif_to_mp4", Status: status, InputMediaID: pgtype.Text{String: "med_gif", Valid: true}, Request: request, CreatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true}}
}

type fakeGIFConversionQueries struct {
	media       map[string]db.Media
	jobs        map[string]db.MediaProcessingJob
	idempotency map[string]db.MediaProcessingJob
}

func (f *fakeGIFConversionQueries) GetMediaProcessingJobByIdempotencyKey(_ context.Context, arg db.GetMediaProcessingJobByIdempotencyKeyParams) (db.MediaProcessingJob, error) {
	row, ok := f.idempotency[arg.IdempotencyKey.String]
	if !ok || row.WorkspaceID != arg.WorkspaceID {
		return db.MediaProcessingJob{}, pgx.ErrNoRows
	}
	return row, nil
}

func (f *fakeGIFConversionQueries) GetMediaByIDAndWorkspace(_ context.Context, arg db.GetMediaByIDAndWorkspaceParams) (db.Media, error) {
	row, ok := f.media[arg.ID]
	if !ok || row.WorkspaceID != arg.WorkspaceID {
		return db.Media{}, pgx.ErrNoRows
	}
	return row, nil
}

func (f *fakeGIFConversionQueries) MarkMediaUploaded(_ context.Context, arg db.MarkMediaUploadedParams) (db.Media, error) {
	row, ok := f.media[arg.ID]
	if !ok {
		return db.Media{}, pgx.ErrNoRows
	}
	row.Status = "uploaded"
	row.ContentType = arg.ContentType
	row.SizeBytes = arg.SizeBytes
	f.media[arg.ID] = row
	return row, nil
}

func (f *fakeGIFConversionQueries) GetMediaProcessingJobByIDAndWorkspace(_ context.Context, arg db.GetMediaProcessingJobByIDAndWorkspaceParams) (db.MediaProcessingJob, error) {
	row, ok := f.jobs[arg.ID]
	if !ok || row.WorkspaceID != arg.WorkspaceID {
		return db.MediaProcessingJob{}, pgx.ErrNoRows
	}
	return row, nil
}

type fakeGIFObjectStore struct {
	head storage.HeadResult
	err  error
}

func (f *fakeGIFObjectStore) Head(context.Context, string) (storage.HeadResult, error) {
	return f.head, f.err
}

type fakeGIFAdmitter struct {
	requests []mediaprocessing.GIFAdmissionRequest
	result   mediaprocessing.AdmissionResult
	err      error
}

func (f *fakeGIFAdmitter) AdmitGIF(_ context.Context, req mediaprocessing.GIFAdmissionRequest) (mediaprocessing.AdmissionResult, error) {
	f.requests = append(f.requests, req)
	return f.result, f.err
}
