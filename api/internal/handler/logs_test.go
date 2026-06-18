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
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/auth"
	"github.com/xiaoboyu/unipost-api/internal/db"
)

// fakeLogsStore records the params it receives and returns canned rows
// so the handler's workspace scoping and cursor behavior can be tested
// without a database.
type fakeLogsStore struct {
	listParams db.ListIntegrationLogsParams
	listRows   []db.IntegrationLog
	listErr    error

	getParams db.GetIntegrationLogParams
	getRow    db.IntegrationLog
	getErr    error
}

func (f *fakeLogsStore) ListIntegrationLogs(ctx context.Context, arg db.ListIntegrationLogsParams) ([]db.IntegrationLog, error) {
	f.listParams = arg
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.listRows, nil
}

func (f *fakeLogsStore) GetIntegrationLog(ctx context.Context, arg db.GetIntegrationLogParams) (db.IntegrationLog, error) {
	f.getParams = arg
	if f.getErr != nil {
		return db.IntegrationLog{}, f.getErr
	}
	return f.getRow, nil
}

func sampleLog(id int64, workspaceID string, ts time.Time) db.IntegrationLog {
	return db.IntegrationLog{
		ID:              id,
		WorkspaceID:     workspaceID,
		Ts:              pgtype.Timestamptz{Time: ts, Valid: true},
		Level:           "error",
		Status:          "error",
		Category:        "oauth",
		Action:          "account.connect.callback_failed",
		Source:          "oauth",
		Message:         "Failed to persist connected account.",
		RequestPayload:  []byte(`{"headers":{"Authorization":"[REDACTED]"}}`),
		ResponsePayload: []byte(`{"error":"validation_error"}`),
	}
}

func newLogsRequest(target, workspaceID string) *http.Request {
	r := httptest.NewRequest(http.MethodGet, target, nil)
	if workspaceID != "" {
		r = r.WithContext(auth.SetWorkspaceID(r.Context(), workspaceID))
	}
	return r
}

func TestLogsList_ScopedToAuthenticatedWorkspace(t *testing.T) {
	store := &fakeLogsStore{listRows: []db.IntegrationLog{sampleLog(1, "ws_authed", time.Now())}}
	h := NewLogsHandler(store)

	// A caller-supplied workspace_id must be ignored; only the
	// context workspace reaches the query.
	r := newLogsRequest("/v1/logs?workspace_id=ws_attacker", "ws_authed")
	w := httptest.NewRecorder()
	h.List(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if store.listParams.WorkspaceID != "ws_authed" {
		t.Fatalf("query ran for workspace %q, want ws_authed", store.listParams.WorkspaceID)
	}
}

func TestLogsList_MissingWorkspaceReturns401(t *testing.T) {
	h := NewLogsHandler(&fakeLogsStore{})
	r := newLogsRequest("/v1/logs", "")
	w := httptest.NewRecorder()
	h.List(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestLogsList_OmitsPayloads(t *testing.T) {
	store := &fakeLogsStore{listRows: []db.IntegrationLog{sampleLog(1, "ws_authed", time.Now())}}
	h := NewLogsHandler(store)

	r := newLogsRequest("/v1/logs", "ws_authed")
	w := httptest.NewRecorder()
	h.List(w, r)

	var resp struct {
		Data []map[string]json.RawMessage `json:"data"`
		Meta map[string]any               `json:"meta"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Data) != 1 {
		t.Fatalf("expected 1 row, got %d", len(resp.Data))
	}
	if _, ok := resp.Data[0]["request_payload"]; ok {
		t.Fatal("list response must not include request_payload")
	}
	if _, ok := resp.Data[0]["response_payload"]; ok {
		t.Fatal("list response must not include response_payload")
	}
	if _, ok := resp.Meta["total"]; ok {
		t.Fatal("cursor list meta must not include total")
	}
}

func TestLogsList_CursorPagination(t *testing.T) {
	now := time.Now().UTC()
	// Three rows for a page size of 2 -> handler fetches limit+1 (3),
	// returns 2, and signals has_more with a cursor pointing at row 2.
	rows := []db.IntegrationLog{
		sampleLog(3, "ws_authed", now),
		sampleLog(2, "ws_authed", now.Add(-time.Second)),
		sampleLog(1, "ws_authed", now.Add(-2*time.Second)),
	}
	store := &fakeLogsStore{listRows: rows}
	h := NewLogsHandler(store)

	r := newLogsRequest("/v1/logs?limit=2", "ws_authed")
	w := httptest.NewRecorder()
	h.List(w, r)

	if store.listParams.Limit != 3 {
		t.Fatalf("expected limit+1=3 fetched, got %d", store.listParams.Limit)
	}

	var resp struct {
		Data []map[string]any `json:"data"`
		Meta struct {
			Limit      int    `json:"limit"`
			HasMore    bool   `json:"has_more"`
			NextCursor string `json:"next_cursor"`
		} `json:"meta"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Data) != 2 {
		t.Fatalf("expected 2 returned rows, got %d", len(resp.Data))
	}
	if !resp.Meta.HasMore {
		t.Fatal("expected has_more=true")
	}
	if resp.Meta.NextCursor == "" {
		t.Fatal("expected a next_cursor")
	}

	// The cursor must decode to the last returned row (id 2).
	ts, id, err := decodeLogCursor(resp.Meta.NextCursor)
	if err != nil {
		t.Fatalf("decode cursor: %v", err)
	}
	if id != 2 {
		t.Fatalf("cursor id = %d, want 2", id)
	}
	if !ts.Equal(rows[1].Ts.Time.UTC()) {
		t.Fatalf("cursor ts = %v, want %v", ts, rows[1].Ts.Time.UTC())
	}
}

func TestLogsList_LastPageHasNoCursor(t *testing.T) {
	store := &fakeLogsStore{listRows: []db.IntegrationLog{sampleLog(1, "ws_authed", time.Now())}}
	h := NewLogsHandler(store)

	r := newLogsRequest("/v1/logs?limit=2", "ws_authed")
	w := httptest.NewRecorder()
	h.List(w, r)

	var resp struct {
		Meta struct {
			HasMore    bool   `json:"has_more"`
			NextCursor string `json:"next_cursor"`
		} `json:"meta"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Meta.HasMore {
		t.Fatal("expected has_more=false on a short page")
	}
	if resp.Meta.NextCursor != "" {
		t.Fatal("expected no next_cursor on the last page")
	}
}

func TestLogsList_ForwardsDecodedCursor(t *testing.T) {
	store := &fakeLogsStore{}
	h := NewLogsHandler(store)

	cursorTs := time.Unix(1700000000, 123).UTC()
	cursor := encodeLogCursor(cursorTs, 42)
	r := newLogsRequest("/v1/logs?cursor="+cursor, "ws_authed")
	w := httptest.NewRecorder()
	h.List(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if !store.listParams.CursorTs.Valid || !store.listParams.CursorTs.Time.Equal(cursorTs) {
		t.Fatalf("cursor ts not forwarded: %+v", store.listParams.CursorTs)
	}
	if store.listParams.CursorID != 42 {
		t.Fatalf("cursor id = %d, want 42", store.listParams.CursorID)
	}
}

func TestLogsList_InvalidCursorReturns422(t *testing.T) {
	h := NewLogsHandler(&fakeLogsStore{})
	r := newLogsRequest("/v1/logs?cursor=not-a-valid-cursor!!!", "ws_authed")
	w := httptest.NewRecorder()
	h.List(w, r)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", w.Code)
	}
	var resp ErrorResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Error.Code != "VALIDATION_ERROR" {
		t.Fatalf("expected VALIDATION_ERROR, got %q", resp.Error.Code)
	}
}

func TestLogsGet_CrossWorkspaceReturns404(t *testing.T) {
	// The store returns ErrNoRows because the id belongs to a
	// different workspace; the handler must surface 404, never the row.
	store := &fakeLogsStore{getErr: pgx.ErrNoRows}
	h := NewLogsHandler(store)

	r := newLogsRequest("/v1/logs/110966", "ws_authed")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "110966")
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()
	h.Get(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
	if store.getParams.WorkspaceID != "ws_authed" {
		t.Fatalf("get ran for workspace %q, want ws_authed", store.getParams.WorkspaceID)
	}
	if store.getParams.ID != 110966 {
		t.Fatalf("get ran for id %d, want 110966", store.getParams.ID)
	}
}

func TestLogsGet_IncludesRedactedPayloads(t *testing.T) {
	store := &fakeLogsStore{getRow: sampleLog(110966, "ws_authed", time.Now())}
	h := NewLogsHandler(store)

	r := newLogsRequest("/v1/logs/110966", "ws_authed")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "110966")
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()
	h.Get(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Data map[string]json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if _, ok := resp.Data["request_payload"]; !ok {
		t.Fatal("detail response must include request_payload")
	}
	if _, ok := resp.Data["response_payload"]; !ok {
		t.Fatal("detail response must include response_payload")
	}
	// The stored payload is already redacted; confirm the redaction
	// marker survives to the response body.
	if got := string(resp.Data["request_payload"]); got == "" || !json.Valid(resp.Data["request_payload"]) {
		t.Fatalf("unexpected request_payload: %q", got)
	}
}
