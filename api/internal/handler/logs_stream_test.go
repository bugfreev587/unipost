package handler

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/xiaoboyu/unipost-api/internal/auth"
	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/ws"
)

type fakeStreamStore struct {
	rows       []db.IntegrationLog
	lastParams db.ListIntegrationLogsAfterIDParams
}

func (f *fakeStreamStore) ListIntegrationLogsAfterID(ctx context.Context, arg db.ListIntegrationLogsAfterIDParams) ([]db.IntegrationLog, error) {
	f.lastParams = arg
	return f.rows, nil
}

func TestStream_MissingWorkspaceReturns401(t *testing.T) {
	h := NewLogsStreamHandler(ws.NewHub(), &fakeStreamStore{})
	r := httptest.NewRequest(http.MethodGet, "/v1/logs/stream", nil)
	w := httptest.NewRecorder()
	h.Stream(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestStream_InvalidAfterIDReturns422(t *testing.T) {
	h := NewLogsStreamHandler(ws.NewHub(), &fakeStreamStore{})
	r := httptest.NewRequest(http.MethodGet, "/v1/logs/stream?after_id=not-a-number", nil)
	r = r.WithContext(auth.SetWorkspaceID(r.Context(), "ws_1"))
	w := httptest.NewRecorder()
	h.Stream(w, r)
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", w.Code, w.Body.String())
	}
}

func TestStream_InvalidFilterReturns422(t *testing.T) {
	h := NewLogsStreamHandler(ws.NewHub(), &fakeStreamStore{})
	r := httptest.NewRequest(http.MethodGet, "/v1/logs/stream?level=loud", nil)
	r = r.WithContext(auth.SetWorkspaceID(r.Context(), "ws_1"))
	w := httptest.NewRecorder()
	h.Stream(w, r)
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", w.Code)
	}
}

func TestResolveReplayStart_Precedence(t *testing.T) {
	// after_id wins over Last-Event-ID.
	id, ok, msg := resolveReplayStart("10", "4")
	if msg != "" || !ok || id != 10 {
		t.Fatalf("after_id precedence failed: id=%d ok=%v msg=%q", id, ok, msg)
	}
	// Falls back to Last-Event-ID.
	id, ok, msg = resolveReplayStart("", "4")
	if msg != "" || !ok || id != 4 {
		t.Fatalf("last-event-id fallback failed: id=%d ok=%v msg=%q", id, ok, msg)
	}
	// Neither -> no replay.
	id, ok, msg = resolveReplayStart("", "")
	if msg != "" || ok || id != 0 {
		t.Fatalf("no-replay case failed: id=%d ok=%v msg=%q", id, ok, msg)
	}
	// Invalid.
	if _, _, msg = resolveReplayStart("-1", ""); msg == "" {
		t.Fatal("expected invalid after_id error")
	}
}

// streamIDs scans an SSE body and forwards each event id.
func streamIDs(body io.Reader, out chan<- int64) {
	sc := bufio.NewScanner(body)
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "id: ") {
			if id, err := strconv.ParseInt(strings.TrimPrefix(line, "id: "), 10, 64); err == nil {
				out <- id
			}
		}
	}
}

func broadcastLog(h *ws.Hub, workspaceID string, id int64) {
	row := sampleLog(id, workspaceID, time.Now())
	payload, _ := json.Marshal(ws.LogEnvelope(row))
	h.Broadcast(workspaceID, payload)
}

// TestStream_ReplayThenLive exercises the full no-gap path: replay of
// retained rows by after_id, precedence over Last-Event-ID, live
// delivery, cross-workspace isolation, and dedup of an already-sent id.
func TestStream_ReplayThenLive(t *testing.T) {
	hub := ws.NewHub()
	store := &fakeStreamStore{rows: []db.IntegrationLog{
		sampleLog(5, "ws_1", time.Now()),
		sampleLog(6, "ws_1", time.Now()),
	}}
	h := NewLogsStreamHandler(hub, store)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := auth.SetWorkspaceID(r.Context(), "ws_1")
		h.Stream(w, r.WithContext(ctx))
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// after_id=4 should win over the Last-Event-ID:2 header.
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/v1/logs/stream?after_id=4", nil)
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Last-Event-ID", "2")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	ids := make(chan int64, 16)
	go streamIDs(resp.Body, ids)

	// Replay rows arrive first.
	if got := waitID(t, ids); got != 5 {
		t.Fatalf("first replay id = %d, want 5", got)
	}
	if got := waitID(t, ids); got != 6 {
		t.Fatalf("second replay id = %d, want 6", got)
	}

	// after_id (4) must have won over Last-Event-ID (2).
	if store.lastParams.AfterID != 4 {
		t.Fatalf("replay queried after_id=%d, want 4", store.lastParams.AfterID)
	}

	// Now in live mode. A stale duplicate (id 6) must be deduped, a
	// cross-workspace event must be filtered, and id 7 must arrive.
	broadcastLog(hub, "ws_1", 6)  // already sent -> dropped
	broadcastLog(hub, "ws_2", 99) // other workspace -> never delivered
	broadcastLog(hub, "ws_1", 7)  // fresh -> delivered

	if got := waitID(t, ids); got != 7 {
		t.Fatalf("live id = %d, want 7 (dup/cross-workspace not filtered)", got)
	}
}

func TestStream_LastEventIDReplay(t *testing.T) {
	hub := ws.NewHub()
	store := &fakeStreamStore{rows: []db.IntegrationLog{sampleLog(8, "ws_1", time.Now())}}
	h := NewLogsStreamHandler(hub, store)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := auth.SetWorkspaceID(r.Context(), "ws_1")
		h.Stream(w, r.WithContext(ctx))
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/v1/logs/stream", nil)
	req.Header.Set("Last-Event-ID", "7")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	ids := make(chan int64, 4)
	go streamIDs(resp.Body, ids)

	if got := waitID(t, ids); got != 8 {
		t.Fatalf("replay id = %d, want 8", got)
	}
	if store.lastParams.AfterID != 7 {
		t.Fatalf("replay queried after_id=%d, want 7", store.lastParams.AfterID)
	}
}

func waitID(t *testing.T, ids <-chan int64) int64 {
	t.Helper()
	select {
	case id := <-ids:
		return id
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for an SSE event")
		return 0
	}
}
