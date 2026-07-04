package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/xiaoboyu/unipost-api/internal/auth"
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
