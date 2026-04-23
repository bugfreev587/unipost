package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/db"
)

func TestNormalizeErrorCode(t *testing.T) {
	tests := map[string]string{
		"VALIDATION_ERROR":       "validation_error",
		"UNAUTHORIZED":           "unauthorized",
		"NEEDS_RECONNECT":        "needs_reconnect",
		"QUEUE_JOB_ACTIVE":       "queue_job_active",
		"SOME_FUTURE_ERROR_CODE": "some_future_error_code",
	}

	for input, want := range tests {
		if got := normalizeErrorCode(input); got != want {
			t.Fatalf("normalizeErrorCode(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestWriteSuccessContract(t *testing.T) {
	rr := httptest.NewRecorder()
	rr.Header().Set("X-Request-Id", "req_success")

	writeSuccess(rr, map[string]any{"id": "acc_123"})

	if rr.Code != http.StatusOK {
		t.Fatalf("writeSuccess status = %d, want 200", rr.Code)
	}

	var got map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if got["request_id"] != "req_success" {
		t.Fatalf("request_id = %#v, want req_success", got["request_id"])
	}
	if _, ok := got["meta"]; ok {
		t.Fatalf("writeSuccess should omit meta, got %#v", got["meta"])
	}
}

func TestWriteSuccessWithListMetaContract(t *testing.T) {
	rr := httptest.NewRecorder()
	rr.Header().Set("X-Request-Id", "req_list")

	writeSuccessWithListMeta(rr, []string{"a", "b"}, 27, 10)

	if rr.Code != http.StatusOK {
		t.Fatalf("writeSuccessWithListMeta status = %d, want 200", rr.Code)
	}

	var got struct {
		Meta struct {
			Total float64 `json:"total"`
			Limit float64 `json:"limit"`
		} `json:"meta"`
		RequestID string `json:"request_id"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if got.Meta.Total != 27 || got.Meta.Limit != 10 {
		t.Fatalf("meta = %#v, want total=27 limit=10", got.Meta)
	}
	if got.RequestID != "req_list" {
		t.Fatalf("request_id = %q, want req_list", got.RequestID)
	}
}

func TestWriteSuccessWithCursorContract(t *testing.T) {
	rr := httptest.NewRecorder()
	rr.Header().Set("X-Request-Id", "req_cursor")

	writeSuccessWithCursor(rr, []string{"post_1"}, "cursor_2", true, 25)

	if rr.Code != http.StatusOK {
		t.Fatalf("writeSuccessWithCursor status = %d, want 200", rr.Code)
	}

	var got struct {
		Meta struct {
			Limit      float64 `json:"limit"`
			HasMore    bool    `json:"has_more"`
			NextCursor string  `json:"next_cursor"`
		} `json:"meta"`
		RequestID string `json:"request_id"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if got.Meta.Limit != 25 || !got.Meta.HasMore || got.Meta.NextCursor != "cursor_2" {
		t.Fatalf("meta = %#v, want limit=25 has_more=true next_cursor=cursor_2", got.Meta)
	}
	if got.RequestID != "req_cursor" {
		t.Fatalf("request_id = %q, want req_cursor", got.RequestID)
	}
}

func TestWriteSuccessWithLegacyCursorContract(t *testing.T) {
	rr := httptest.NewRecorder()
	rr.Header().Set("X-Request-Id", "req_legacy")

	writeSuccessWithLegacyCursor(rr, []string{"post_1"}, "cursor_2", true, 25)

	if rr.Code != http.StatusOK {
		t.Fatalf("writeSuccessWithLegacyCursor status = %d, want 200", rr.Code)
	}

	var got struct {
		Meta struct {
			NextCursor string `json:"next_cursor"`
		} `json:"meta"`
		NextCursor string `json:"next_cursor"`
		RequestID  string `json:"request_id"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if got.Meta.NextCursor != "cursor_2" || got.NextCursor != "cursor_2" {
		t.Fatalf("cursor fields = meta:%q top:%q, want both cursor_2", got.Meta.NextCursor, got.NextCursor)
	}
	if got.RequestID != "req_legacy" {
		t.Fatalf("request_id = %q, want req_legacy", got.RequestID)
	}
}

func TestWriteAcceptedContract(t *testing.T) {
	rr := httptest.NewRecorder()
	rr.Header().Set("X-Request-Id", "req_accepted")

	writeAccepted(rr, map[string]any{"id": "post_123"})

	if rr.Code != http.StatusAccepted {
		t.Fatalf("writeAccepted status = %d, want 202", rr.Code)
	}

	var got map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if got["request_id"] != "req_accepted" {
		t.Fatalf("request_id = %#v, want req_accepted", got["request_id"])
	}
}

func TestWriteErrorContract(t *testing.T) {
	rr := httptest.NewRecorder()
	rr.Header().Set("X-Request-Id", "req_error")

	writeError(rr, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "bad input")

	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("writeError status = %d, want 422", rr.Code)
	}

	var got struct {
		Error struct {
			Code           string `json:"code"`
			NormalizedCode string `json:"normalized_code"`
			Message        string `json:"message"`
		} `json:"error"`
		RequestID string `json:"request_id"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if got.Error.Code != "VALIDATION_ERROR" || got.Error.NormalizedCode != "validation_error" || got.Error.Message != "bad input" {
		t.Fatalf("error body = %#v, want validation error contract", got.Error)
	}
	if got.RequestID != "req_error" {
		t.Fatalf("request_id = %q, want req_error", got.RequestID)
	}
}

func TestSocialPostCreateStatusCode(t *testing.T) {
	tests := []struct {
		name string
		post db.SocialPost
		want int
	}{
		{
			name: "draft create stays 201",
			post: db.SocialPost{Status: "draft"},
			want: http.StatusCreated,
		},
		{
			name: "scheduled create stays 201",
			post: db.SocialPost{
				Status:      "scheduled",
				ScheduledAt: pgtype.Timestamptz{Valid: true},
			},
			want: http.StatusCreated,
		},
		{
			name: "immediate async create is 202",
			post: db.SocialPost{Status: "publishing"},
			want: http.StatusAccepted,
		},
		{
			name: "replayed published async create stays 202",
			post: db.SocialPost{Status: "published"},
			want: http.StatusAccepted,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := socialPostCreateStatusCode(tt.post); got != tt.want {
				t.Fatalf("socialPostCreateStatusCode(%+v) = %d, want %d", tt.post, got, tt.want)
			}
		})
	}
}
