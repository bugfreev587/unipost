package metrics

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/xiaoboyu/unipost-api/internal/auth"
	"github.com/xiaoboyu/unipost-api/internal/db"
)

type recordingInserter struct {
	calls chan recordedMetric
}

type recordedMetric struct {
	ctxErr error
	arg    db.InsertAPIMetricParams
}

func newRecordingInserter() *recordingInserter {
	return &recordingInserter{calls: make(chan recordedMetric, 4)}
}

func (r *recordingInserter) InsertAPIMetric(ctx context.Context, arg db.InsertAPIMetricParams) error {
	r.calls <- recordedMetric{ctxErr: ctx.Err(), arg: arg}
	return nil
}

func TestRecorderRecordsAPIKeyTrafficWithRoutePatternAndDetachedContext(t *testing.T) {
	inserter := newRecordingInserter()
	rec := NewRecorder(inserter)
	cancelledErr := make(chan error, 1)

	router := chi.NewRouter()
	router.Use(testAuthContext("ws_1", "ak_1"))
	router.Use(rec.Middleware)
	router.Post("/v1/posts/{id}/publish", func(w http.ResponseWriter, r *http.Request) {
		cancelledErr <- r.Context().Err()
		w.WriteHeader(http.StatusAccepted)
	})

	reqCtx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodPost, "/v1/posts/42/publish?debug=true", nil).WithContext(reqCtx)
	rr := httptest.NewRecorder()

	cancel()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202", rr.Code)
	}
	if err := <-cancelledErr; !errors.Is(err, context.Canceled) {
		t.Fatalf("handler request context err = %v, want context.Canceled", err)
	}

	got := waitForMetric(t, inserter)
	if got.ctxErr != nil {
		t.Fatalf("insert context err = %v, want nil detached context", got.ctxErr)
	}
	if got.arg.WorkspaceID != "ws_1" || got.arg.ApiKeyID.String != "ak_1" || !got.arg.ApiKeyID.Valid {
		t.Fatalf("workspace/api key = %#v, want ws_1/ak_1", got.arg)
	}
	if got.arg.Method != http.MethodPost {
		t.Fatalf("method = %q, want POST", got.arg.Method)
	}
	if got.arg.Path != "/v1/posts/:id/publish" {
		t.Fatalf("path = %q, want normalized route pattern", got.arg.Path)
	}
	if got.arg.StatusCode != http.StatusAccepted {
		t.Fatalf("status code = %d, want 202", got.arg.StatusCode)
	}
}

func TestRecorderSkipsMetricsEndpointsAndClerkTraffic(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		apiKeyID string
	}{
		{name: "metrics endpoint", path: "/v1/api-metrics/summary", apiKeyID: "ak_1"},
		{name: "clerk traffic", path: "/v1/posts/42/publish", apiKeyID: ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			inserter := newRecordingInserter()
			rec := NewRecorder(inserter)
			router := chi.NewRouter()
			router.Use(testAuthContext("ws_1", tc.apiKeyID))
			router.Use(rec.Middleware)
			router.Get("/v1/api-metrics/summary", func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})
			router.Post("/v1/posts/{id}/publish", func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			req := httptest.NewRequest(methodForPath(tc.path), tc.path, nil)
			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)

			select {
			case got := <-inserter.calls:
				t.Fatalf("unexpected metric recorded: %#v", got.arg)
			case <-time.After(25 * time.Millisecond):
			}
		})
	}
}

func TestRecorderDoesNotSkipMediaTraffic(t *testing.T) {
	inserter := newRecordingInserter()
	rec := NewRecorder(inserter)
	router := chi.NewRouter()
	router.Use(testAuthContext("ws_1", "ak_1"))
	router.Use(rec.Middleware)
	router.Post("/v1/media", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/media", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201", rr.Code)
	}
	got := waitForMetric(t, inserter)
	if got.arg.Path != "/v1/media" {
		t.Fatalf("path = %q, want /v1/media", got.arg.Path)
	}
	if got.arg.StatusCode != http.StatusCreated {
		t.Fatalf("status code = %d, want 201", got.arg.StatusCode)
	}
}

func TestNormalizeRoutePatternKeepsResourceNamesAndNormalizesIDs(t *testing.T) {
	tests := []struct {
		name         string
		routePattern string
		rawPath      string
		want         string
	}{
		{
			name:         "keeps long resource name",
			routePattern: "/v1/social-posts",
			rawPath:      "/v1/social-posts",
			want:         "/v1/social-posts",
		},
		{
			name:         "normalizes short numeric id from route pattern",
			routePattern: "/v1/posts/{id}/publish",
			rawPath:      "/v1/posts/42/publish",
			want:         "/v1/posts/:id/publish",
		},
		{
			name:         "fallback strips uuid when no route pattern exists",
			routePattern: "",
			rawPath:      "/v1/posts/4fe26fc0-5f25-43a6-8544-6ce32dc2dc5c",
			want:         "/v1/posts/:id",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := normalizeRoutePattern(tc.routePattern, tc.rawPath); got != tc.want {
				t.Fatalf("normalizeRoutePattern(%q, %q) = %q, want %q", tc.routePattern, tc.rawPath, got, tc.want)
			}
		})
	}
}

func testAuthContext(workspaceID, apiKeyID string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := auth.SetWorkspaceID(r.Context(), workspaceID)
			if apiKeyID != "" {
				ctx = auth.SetAPIKeyID(ctx, apiKeyID)
			}
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func waitForMetric(t *testing.T, inserter *recordingInserter) recordedMetric {
	t.Helper()
	select {
	case got := <-inserter.calls:
		return got
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for metric insert")
		return recordedMetric{}
	}
}

func methodForPath(path string) string {
	if path == "/v1/api-metrics/summary" {
		return http.MethodGet
	}
	return http.MethodPost
}
