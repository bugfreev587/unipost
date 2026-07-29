package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/xiaoboyu/unipost-api/internal/auth"
	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/integrationlogs"
	"github.com/xiaoboyu/unipost-api/internal/metrics"
	"github.com/xiaoboyu/unipost-api/internal/requestevents"
)

type dualWriteMetricStore struct {
	calls chan db.InsertAPIMetricParams
}

func (s *dualWriteMetricStore) InsertAPIMetric(_ context.Context, params db.InsertAPIMetricParams) error {
	s.calls <- params
	return nil
}

type dualWriteIntegrationLogStore struct {
	calls chan db.InsertIntegrationLogParams
}

func (s *dualWriteIntegrationLogStore) InsertIntegrationLog(_ context.Context, params db.InsertIntegrationLogParams) (db.IntegrationLog, error) {
	s.calls <- params
	return db.IntegrationLog{}, nil
}

type dualWriteCanonicalStore struct {
	failures chan requestevents.Event
}

func (s *dualWriteCanonicalStore) InsertSuccessBatch(context.Context, []requestevents.Event) error {
	return nil
}

func (s *dualWriteCanonicalStore) InsertFailure(_ context.Context, event requestevents.Event, _ *requestevents.Detail) error {
	s.failures <- event
	return nil
}

func TestAPIKeyRequestAttemptsLegacyAndCanonicalTelemetryExactlyOnce(t *testing.T) {
	metricStore := &dualWriteMetricStore{calls: make(chan db.InsertAPIMetricParams, 2)}
	legacyStore := &dualWriteIntegrationLogStore{calls: make(chan db.InsertIntegrationLogParams, 2)}
	canonicalStore := &dualWriteCanonicalStore{failures: make(chan requestevents.Event, 2)}
	legacyLogger := integrationlogs.NewLogger(legacyStore, nil)
	canonicalRecorder := requestevents.NewRecorder(canonicalStore, requestevents.Config{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go legacyLogger.Start(ctx)
	go canonicalRecorder.Start(ctx)

	router := chi.NewRouter()
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestCtx := auth.SetWorkspaceID(r.Context(), "workspace-1")
			requestCtx = auth.SetAPIKeyID(requestCtx, "api-key-1")
			next.ServeHTTP(w, r.WithContext(requestCtx))
		})
	})
	router.Use(integrationlogs.Middleware(legacyLogger))
	router.Use(metrics.NewRecorder(metricStore).Middleware)
	router.Use(requestevents.Middleware(canonicalRecorder))
	mutations := 0
	router.Post("/v1/posts/{id}/publish", func(w http.ResponseWriter, r *http.Request) {
		mutations++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"code":"PROVIDER_UNAVAILABLE"}`))
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/posts/post-1/publish", strings.NewReader(`{"title":"unchanged"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable || rec.Body.String() != `{"code":"PROVIDER_UNAVAILABLE"}` || mutations != 1 {
		t.Fatalf("business outcome changed: status=%d body=%q mutations=%d", rec.Code, rec.Body.String(), mutations)
	}
	metric := waitDualWrite(t, metricStore.calls)
	legacy := waitDualWrite(t, legacyStore.calls)
	canonical := waitDualWrite(t, canonicalStore.failures)
	if metric.WorkspaceID != "workspace-1" || metric.Path != "/v1/posts/:id/publish" || metric.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("legacy metric = %#v", metric)
	}
	if legacy.WorkspaceID != "workspace-1" || legacy.HTTPStatusCode.Int32 != http.StatusServiceUnavailable {
		t.Fatalf("legacy integration log = %#v", legacy)
	}
	if canonical.WorkspaceID != "workspace-1" || canonical.RoutePattern != "/v1/posts/:id/publish" || canonical.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("canonical event = %#v", canonical)
	}
	assertNoSecondDualWrite(t, metricStore.calls, "metric")
	assertNoSecondDualWrite(t, legacyStore.calls, "integration log")
	assertNoSecondDualWrite(t, canonicalStore.failures, "canonical event")
}

func waitDualWrite[T any](t *testing.T, calls <-chan T) T {
	t.Helper()
	select {
	case value := <-calls:
		return value
	case <-time.After(time.Second):
		var zero T
		t.Fatal("timed out waiting for telemetry write")
		return zero
	}
}

func assertNoSecondDualWrite[T any](t *testing.T, calls <-chan T, label string) {
	t.Helper()
	select {
	case extra := <-calls:
		t.Fatalf("unexpected second %s write: %#v", label, extra)
	case <-time.After(20 * time.Millisecond):
	}
}
