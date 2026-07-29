package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"github.com/xiaoboyu/unipost-api/internal/observabilityreads"
)

type countingObservabilitySelector struct {
	enabled bool
	calls   int
}

func (s *countingObservabilitySelector) UseV2(context.Context) bool {
	s.calls++
	return s.enabled
}

func TestAdminLogsOptionalInjectionOffSelectsLegacyWithoutCallingV2(t *testing.T) {
	v2 := &fakeObservabilityLogReader{}
	selector := &countingObservabilitySelector{}
	h := NewAdminHandler(nil, nil, nil).SetObservabilityReads(selector, v2)
	if reader := h.observabilityLogReader(context.Background()); reader != nil {
		t.Fatalf("OFF selector chose v2 reader %T; want legacy path", reader)
	}
	if selector.calls != 1 {
		t.Fatalf("selector calls = %d, want 1", selector.calls)
	}
	if v2.adminCalls != 0 {
		t.Fatalf("v2 admin calls = %d, want 0 while selector OFF", v2.adminCalls)
	}
}

func TestAdminLogsTypedNilV2ReaderSelectsLegacy(t *testing.T) {
	var v2 observabilityreads.LogReader = (*fakeObservabilityLogReader)(nil)
	h := NewAdminHandler(nil, nil, nil).SetObservabilityReads(fakeObservabilitySelector{enabled: true}, v2)
	if reader := h.observabilityLogReader(context.Background()); reader != nil {
		t.Fatalf("typed-nil v2 reader selected as %T; want legacy path", reader)
	}
}

func TestAdminLogsV2DefaultsTo24Hours100AndCapsAt200(t *testing.T) {
	for _, tt := range []struct {
		name      string
		target    string
		wantLimit int
	}{
		{name: "defaults", target: "/v1/admin/logs", wantLimit: 100},
		{name: "caps", target: "/v1/admin/logs?limit=201", wantLimit: 200},
	} {
		t.Run(tt.name, func(t *testing.T) {
			v2 := &fakeObservabilityLogReader{}
			h := NewAdminHandler(nil, nil, nil).SetObservabilityReads(fakeObservabilitySelector{enabled: true}, v2)
			before := time.Now().UTC()
			w := httptest.NewRecorder()
			h.ListLogs(w, httptest.NewRequest(http.MethodGet, tt.target, nil))
			after := time.Now().UTC()
			if w.Code != http.StatusOK {
				t.Fatalf("status = %d: %s", w.Code, w.Body.String())
			}
			if v2.adminLimit != tt.wantLimit {
				t.Fatalf("limit = %d, want %d", v2.adminLimit, tt.wantLimit)
			}
			if v2.adminFilters.To.Before(before) || v2.adminFilters.To.After(after) {
				t.Fatalf("to = %v, want between %v and %v", v2.adminFilters.To, before, after)
			}
			wantFrom := v2.adminFilters.To.Add(-24 * time.Hour)
			if delta := v2.adminFilters.From.Sub(wantFrom); delta < -time.Second || delta > time.Second {
				t.Fatalf("from = %v, want approximately %v", v2.adminFilters.From, wantFrom)
			}
		})
	}
}

func TestAdminLogsV2SupportsGlobalListAndCursor(t *testing.T) {
	now := time.Now().UTC()
	next := observabilityreads.LogCursor{Timestamp: now, SourceKind: observabilityreads.LogSourceIntegration, SourceID: "9"}
	v2 := &fakeObservabilityLogReader{adminPage: observabilityreads.LogPage{
		Logs: []observabilityreads.LogProjection{{
			ID: "request:event-1", SourceKind: observabilityreads.LogSourceRequest, SourceID: "event-1",
			WorkspaceID: "ws_1", WorkspaceName: "Workspace", OwnerEmail: "owner@example.com", Timestamp: now,
		}},
		NextCursor: &next,
	}}
	h := NewAdminHandler(nil, nil, nil).SetObservabilityReads(fakeObservabilitySelector{enabled: true}, v2)
	w := httptest.NewRecorder()
	h.ListLogs(w, httptest.NewRequest(http.MethodGet, "/v1/admin/logs?workspace_id=ws_1&owner_email=owner&limit=25", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", w.Code, w.Body.String())
	}
	if v2.adminFilters.WorkspaceID != "ws_1" || v2.adminFilters.OwnerEmail != "owner" || v2.adminLimit != 25 {
		t.Fatalf("admin filters = %#v limit=%d", v2.adminFilters, v2.adminLimit)
	}
	var response struct {
		Data []observabilityreads.LogProjection `json:"data"`
		Meta struct {
			HasMore    bool   `json:"has_more"`
			NextCursor string `json:"next_cursor"`
		} `json:"meta"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Data) != 1 || response.Data[0].ID != "request:event-1" || !response.Meta.HasMore {
		t.Fatalf("response = %#v", response)
	}
	decoded, err := observabilityreads.DecodeLogCursor(response.Meta.NextCursor)
	if err != nil || decoded != next {
		t.Fatalf("cursor = %#v err=%v", decoded, err)
	}
}

func TestAdminLogsV2DetailSupportsQualifiedIDsAndNotFound(t *testing.T) {
	for _, tt := range []struct {
		name       string
		id         string
		detail     observabilityreads.LogDetail
		err        error
		wantStatus int
	}{
		{name: "found", id: "integration:12", detail: observabilityreads.LogDetail{LogProjection: observabilityreads.LogProjection{ID: "integration:12"}}, wantStatus: http.StatusOK},
		{name: "legacy numeric integration detail", id: "12", detail: observabilityreads.LogDetail{LogProjection: observabilityreads.LogProjection{ID: "integration:12"}}, wantStatus: http.StatusOK},
		{name: "noncanonical legacy numeric rejected", id: "012", err: pgx.ErrNoRows, wantStatus: http.StatusNotFound},
		{name: "unknown", id: "request:missing", err: pgx.ErrNoRows, wantStatus: http.StatusNotFound},
	} {
		t.Run(tt.name, func(t *testing.T) {
			v2 := &fakeObservabilityLogReader{getAdminDetail: tt.detail, getAdminErr: tt.err}
			h := NewAdminHandler(nil, nil, nil).SetObservabilityReads(fakeObservabilitySelector{enabled: true}, v2)
			r := httptest.NewRequest(http.MethodGet, "/v1/admin/logs/"+tt.id, nil)
			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("id", tt.id)
			r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
			w := httptest.NewRecorder()
			h.GetLog(w, r)
			if w.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d: %s", w.Code, tt.wantStatus, w.Body.String())
			}
			if v2.getAdminID != tt.id {
				t.Fatalf("admin detail ID = %q, want %q", v2.getAdminID, tt.id)
			}
		})
	}
}
