package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"github.com/xiaoboyu/unipost-api/internal/auth"
	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/platform"
	"github.com/xiaoboyu/unipost-api/internal/storage"
)

// TestPickExt covers the filename → extension fallback the media
// handler uses to give R2 a recognizable storage_key.
func TestPickExt(t *testing.T) {
	cases := []struct {
		filename, contentType, want string
	}{
		{"photo.jpg", "image/jpeg", ".jpg"},      // filename wins
		{"PHOTO.JPG", "image/jpeg", ".jpg"},      // case insensitive
		{"", "image/png", ".png"},                // fallback to content type
		{"", "video/mp4", ".mp4"},                // video fallback
		{"", "video/quicktime", ".mov"},          // mov fallback
		{"", "application/octet-stream", ".bin"}, // unknown
		{"unknown", "", ".bin"},                  // truly unknown
		{"a.tar.gz", "", ".gz"},                  // last ext wins (path.Ext semantics)
	}
	for _, c := range cases {
		got := pickExt(c.filename, c.contentType)
		if got != c.want {
			t.Errorf("pickExt(%q, %q) = %q, want %q", c.filename, c.contentType, got, c.want)
		}
	}
}

// TestPickContentType locks down the "trust R2 over the client" rule
// the hydration path uses. The client can lie at create time about
// content type, but R2 echoes back what was actually uploaded — that's
// the authoritative value.
func TestPickContentType(t *testing.T) {
	if got := pickContentType("image/png", "image/jpeg"); got != "image/png" {
		t.Errorf("R2 value should win: got %q", got)
	}
	if got := pickContentType("", "image/jpeg"); got != "image/jpeg" {
		t.Errorf("client fallback when R2 missing: got %q", got)
	}
}

// TestAllowedMimeTypes guards against accidentally narrowing the
// allowlist when refactoring. If you intentionally remove a type,
// update this test.
func TestAllowedMimeTypes(t *testing.T) {
	required := []string{
		"image/jpeg", "image/png", "image/webp", "image/gif",
		"video/mp4", "video/quicktime", "video/webm",
		"audio/mpeg", "audio/wav", "audio/x-wav", "audio/aac", "audio/mp4", "audio/x-m4a",
	}
	for _, m := range required {
		if !allowedMimeTypes[m] {
			t.Errorf("required mime %q not in allowlist", m)
		}
	}
}

func TestMediaCreateSizeBytesAllowsOmittedSize(t *testing.T) {
	got, ok := mediaCreateSizeBytes(nil)
	if !ok {
		t.Fatal("omitted size_bytes should be accepted")
	}
	if got != 0 {
		t.Fatalf("size = %d, want 0 until storage hydration", got)
	}
}

func TestCreateRejectsExplicitNonPositiveSizeWithActionableIssue(t *testing.T) {
	h := NewMediaHandler(nil, &storage.Client{})
	body := strings.NewReader(`{"filename":"photo.jpg","content_type":"image/jpeg","size_bytes":0}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/media", body)
	req = req.WithContext(auth.SetWorkspaceID(req.Context(), "ws_test"))
	rr := httptest.NewRecorder()

	h.Create(rr, req)

	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", rr.Code)
	}

	var got ErrorResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if got.Error.Code != "VALIDATION_ERROR" || got.Error.NormalizedCode != "validation_error" {
		t.Fatalf("error identifiers = %#v, want validation error", got.Error)
	}
	if !strings.Contains(got.Error.Message, "public URL") {
		t.Fatalf("message = %q, want public URL guidance", got.Error.Message)
	}
	if len(got.Error.Issues) != 1 {
		t.Fatalf("issues = %#v, want one structured issue", got.Error.Issues)
	}
	issue := got.Error.Issues[0]
	if issue.Field != "size_bytes" || issue.Code != platform.CodeBelowMinLength {
		t.Fatalf("issue = %#v, want below_min_length size_bytes issue", issue)
	}
	if issue.Actual != float64(0) || issue.Limit != float64(1) {
		t.Fatalf("issue actual/limit = %#v/%#v, want 0/1", issue.Actual, issue.Limit)
	}
	if !strings.Contains(issue.Message, "omit size_bytes") {
		t.Fatalf("issue message = %q, want optional size guidance", issue.Message)
	}
}

func TestDeleteMediaReturnsNotFoundForMissingOrDeletedMedia(t *testing.T) {
	queries := &fakeMediaHandlerQueries{getErr: pgx.ErrNoRows}
	h := NewMediaHandler(queries, nil)
	rr := httptest.NewRecorder()

	h.Delete(rr, mediaDeleteRequest("med_missing"))

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body: %s", rr.Code, rr.Body.String())
	}
	if queries.softDeleteCalls != 0 {
		t.Fatalf("soft delete calls = %d, want 0", queries.softDeleteCalls)
	}
}

func TestDeleteMediaReturnsConflictWhenUsageBlocksDeletion(t *testing.T) {
	queries := &fakeMediaHandlerQueries{
		row:      db.Media{ID: "med_in_use", WorkspaceID: "ws_test", Status: "uploaded"},
		blocking: true,
	}
	h := NewMediaHandler(queries, nil)
	rr := httptest.NewRecorder()

	h.Delete(rr, mediaDeleteRequest("med_in_use"))

	if rr.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409; body: %s", rr.Code, rr.Body.String())
	}
	var got ErrorResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.Error.NormalizedCode != "media_in_use" {
		t.Fatalf("normalized code = %q, want media_in_use", got.Error.NormalizedCode)
	}
	if queries.softDeleteCalls != 0 {
		t.Fatalf("soft delete calls = %d, want 0", queries.softDeleteCalls)
	}
}

func TestDeleteMediaSchedulesUnusedMediaForCleanup(t *testing.T) {
	queries := &fakeMediaHandlerQueries{
		row: db.Media{ID: "med_unused", WorkspaceID: "ws_test", Status: "uploaded"},
	}
	h := NewMediaHandler(queries, nil)
	rr := httptest.NewRecorder()

	h.Delete(rr, mediaDeleteRequest("med_unused"))

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rr.Code, rr.Body.String())
	}
	if queries.softDeleteCalls != 1 || queries.softDeleteArg.ID != "med_unused" || queries.softDeleteArg.WorkspaceID != "ws_test" {
		t.Fatalf("soft delete calls/arg = %d/%#v", queries.softDeleteCalls, queries.softDeleteArg)
	}
}

func TestDeleteMediaReturnsConflictWhenUsageAppearsDuringTransition(t *testing.T) {
	queries := &fakeMediaHandlerQueries{
		row:                    db.Media{ID: "med_raced", WorkspaceID: "ws_test", Status: "uploaded"},
		blockSoftDeleteAtWrite: true,
	}
	h := NewMediaHandler(queries, nil)
	rr := httptest.NewRecorder()

	h.Delete(rr, mediaDeleteRequest("med_raced"))

	if rr.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409; body: %s", rr.Code, rr.Body.String())
	}
}

func mediaDeleteRequest(mediaID string) *http.Request {
	req := httptest.NewRequest(http.MethodDelete, "/v1/media/"+mediaID, nil)
	routeContext := chi.NewRouteContext()
	routeContext.URLParams.Add("id", mediaID)
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, routeContext)
	ctx = auth.SetWorkspaceID(ctx, "ws_test")
	return req.WithContext(ctx)
}

type fakeMediaHandlerQueries struct {
	row                    db.Media
	getErr                 error
	blocking               bool
	softDeleteCalls        int
	softDeleteArg          db.SoftDeleteMediaParams
	blockSoftDeleteAtWrite bool
}

func (f *fakeMediaHandlerQueries) GetMediaByIDAndWorkspace(context.Context, db.GetMediaByIDAndWorkspaceParams) (db.Media, error) {
	if f.getErr != nil {
		return db.Media{}, f.getErr
	}
	return f.row, nil
}

func (f *fakeMediaHandlerQueries) HasBlockingMediaUsage(context.Context, string) (bool, error) {
	return f.blocking, nil
}

func (f *fakeMediaHandlerQueries) SoftDeleteMedia(_ context.Context, arg db.SoftDeleteMediaParams) error {
	return nil
}

func (f *fakeMediaHandlerQueries) SoftDeleteUnusedMedia(_ context.Context, arg db.SoftDeleteUnusedMediaParams) (int64, error) {
	f.softDeleteCalls++
	f.softDeleteArg = db.SoftDeleteMediaParams{ID: arg.ID, WorkspaceID: arg.WorkspaceID}
	if f.blockSoftDeleteAtWrite {
		return 0, nil
	}
	return 1, nil
}

func (f *fakeMediaHandlerQueries) CreateMedia(context.Context, db.CreateMediaParams) (db.Media, error) {
	return db.Media{}, errUnexpectedMediaHandlerQuery
}

func (f *fakeMediaHandlerQueries) GetActiveMediaByHash(context.Context, db.GetActiveMediaByHashParams) (db.Media, error) {
	return db.Media{}, errUnexpectedMediaHandlerQuery
}

func (f *fakeMediaHandlerQueries) HardDeleteMedia(context.Context, string) error {
	return errUnexpectedMediaHandlerQuery
}

func (f *fakeMediaHandlerQueries) UpdateMediaStorageKey(context.Context, db.UpdateMediaStorageKeyParams) (db.Media, error) {
	return db.Media{}, errUnexpectedMediaHandlerQuery
}

func (f *fakeMediaHandlerQueries) MarkMediaUploaded(context.Context, db.MarkMediaUploadedParams) (db.Media, error) {
	return db.Media{}, errUnexpectedMediaHandlerQuery
}

var errUnexpectedMediaHandlerQuery = errors.New("unexpected media handler query")
