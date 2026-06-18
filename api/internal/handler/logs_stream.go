package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/xiaoboyu/unipost-api/internal/auth"
	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/ws"
)

const (
	// sseReplayLimit caps how many retained rows a single reconnect can
	// replay before entering live mode. Keeps a reconnect bounded.
	sseReplayLimit = 500
	// sseKeepalivePeriod matches the PRD's 25s keepalive comment cadence.
	sseKeepalivePeriod = 25 * time.Second
)

// logsStreamStore is the subset of *db.Queries the SSE handler needs
// for replay. An interface keeps the handler unit-testable.
type logsStreamStore interface {
	ListIntegrationLogsAfterID(context.Context, db.ListIntegrationLogsAfterIDParams) ([]db.IntegrationLog, error)
}

// LogsStreamHandler serves an API-key-authenticated SSE log stream. It
// reuses the shared in-process logs hub (fed by a single PostgreSQL
// LISTEN loop) rather than opening a listener connection per client.
type LogsStreamHandler struct {
	hub     *ws.Hub
	queries logsStreamStore
}

func NewLogsStreamHandler(hub *ws.Hub, queries logsStreamStore) *LogsStreamHandler {
	return &LogsStreamHandler{hub: hub, queries: queries}
}

var (
	validStreamCategory = map[string]bool{"publishing": true, "api_request": true, "oauth": true, "webhook": true, "system": true}
	validStreamStatus   = map[string]bool{"success": true, "warning": true, "error": true}
	validStreamLevel    = map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
)

type streamFilters struct {
	category        string
	status          string
	level           string
	platform        string
	profileID       string
	socialAccountID string
	postID          string
	requestID       string
	errorCode       string
}

// parseStreamFilters reads and validates filter params. It returns a
// non-empty message when a known enum filter is invalid.
func parseStreamFilters(q url.Values) (streamFilters, string) {
	f := streamFilters{
		category:        strings.TrimSpace(q.Get("category")),
		status:          strings.TrimSpace(q.Get("status")),
		level:           strings.TrimSpace(q.Get("level")),
		platform:        strings.TrimSpace(q.Get("platform")),
		profileID:       strings.TrimSpace(q.Get("profile_id")),
		socialAccountID: strings.TrimSpace(q.Get("social_account_id")),
		postID:          strings.TrimSpace(q.Get("post_id")),
		requestID:       strings.TrimSpace(q.Get("request_id")),
		errorCode:       strings.TrimSpace(q.Get("error_code")),
	}
	if f.category != "" && !validStreamCategory[f.category] {
		return f, "Invalid category filter"
	}
	if f.status != "" && !validStreamStatus[f.status] {
		return f, "Invalid status filter"
	}
	if f.level != "" && !validStreamLevel[f.level] {
		return f, "Invalid level filter"
	}
	return f, ""
}

func (f streamFilters) matches(obj map[string]any) bool {
	check := func(key, want string) bool {
		if want == "" {
			return true
		}
		v, _ := obj[key].(string)
		return v == want
	}
	return check("category", f.category) &&
		check("status", f.status) &&
		check("level", f.level) &&
		check("platform", f.platform) &&
		check("profile_id", f.profileID) &&
		check("social_account_id", f.socialAccountID) &&
		check("post_id", f.postID) &&
		check("request_id", f.requestID) &&
		check("error_code", f.errorCode)
}

func (f streamFilters) replayParams(workspaceID string, afterID int64) db.ListIntegrationLogsAfterIDParams {
	return db.ListIntegrationLogsAfterIDParams{
		WorkspaceID:     workspaceID,
		AfterID:         afterID,
		Category:        f.category,
		Status:          f.status,
		Level:           f.level,
		Platform:        f.platform,
		ProfileID:       f.profileID,
		SocialAccountID: f.socialAccountID,
		PostID:          f.postID,
		RequestID:       f.requestID,
		ErrorCode:       f.errorCode,
		Limit:           sseReplayLimit,
	}
}

// resolveReplayStart applies the precedence in the PRD: an explicit
// after_id query param wins over the Last-Event-ID reconnect header. It
// returns (afterID, doReplay, errMessage). doReplay is false when
// neither is present, so the stream enters live mode with no backfill.
func resolveReplayStart(afterIDRaw, lastEventID string) (int64, bool, string) {
	if v := strings.TrimSpace(afterIDRaw); v != "" {
		parsed, err := strconv.ParseInt(v, 10, 64)
		if err != nil || parsed < 0 {
			return 0, false, "Invalid after_id"
		}
		return parsed, true, ""
	}
	if v := strings.TrimSpace(lastEventID); v != "" {
		parsed, err := strconv.ParseInt(v, 10, 64)
		if err != nil || parsed < 0 {
			return 0, false, "Invalid Last-Event-ID"
		}
		return parsed, true, ""
	}
	return 0, false, ""
}

func (h *LogsStreamHandler) Stream(w http.ResponseWriter, r *http.Request) {
	workspaceID := auth.GetWorkspaceID(r.Context())
	if workspaceID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing workspace context")
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Streaming unsupported")
		return
	}

	q := r.URL.Query()
	filters, verr := parseStreamFilters(q)
	if verr != "" {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", verr)
		return
	}

	afterID, doReplay, verr := resolveReplayStart(q.Get("after_id"), r.Header.Get("Last-Event-ID"))
	if verr != "" {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", verr)
		return
	}

	// Subscribe BEFORE querying replay rows so any log created during
	// replay is buffered and not dropped in the replay-to-live handoff.
	sub := h.hub.Subscribe(workspaceID)
	defer h.hub.Unsubscribe(workspaceID, sub)

	ctx := r.Context()
	lastID := afterID

	// Fetch the first replay page before committing headers so a replay
	// query failure can still surface as a 500 instead of a silently
	// truncated 200 stream.
	var firstPage []db.IntegrationLog
	if doReplay {
		rows, err := h.queries.ListIntegrationLogsAfterID(ctx, filters.replayParams(workspaceID, afterID))
		if err != nil {
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load logs for replay")
			return
		}
		firstPage = rows
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	if doReplay {
		// Page through retained rows until exhausted. Rows are ordered by
		// id ASC, so the last row's id is the next page cursor. lastID
		// advances for every row seen (matching or not) so the cursor
		// cannot stall and later-arriving live events are deduped.
		page := firstPage
		for {
			for _, row := range page {
				obj := logRowToStreamObj(row)
				if filters.matches(obj) {
					if err := writeLogEvent(w, flusher, row.ID, obj); err != nil {
						return
					}
				}
				if row.ID > lastID {
					lastID = row.ID
				}
			}
			if len(page) < sseReplayLimit {
				break // last page reached
			}
			next, err := h.queries.ListIntegrationLogsAfterID(ctx, filters.replayParams(workspaceID, lastID))
			if err != nil {
				// Headers are already committed, so signal the failure as
				// an SSE error event and close rather than pretend the
				// client is caught up.
				writeStreamError(w, flusher, "replay_failed", "Failed to load additional logs for replay")
				return
			}
			if len(next) == 0 {
				break
			}
			page = next
		}
	}

	keepalive := time.NewTicker(sseKeepalivePeriod)
	defer keepalive.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-keepalive.C:
			if _, err := fmt.Fprint(w, ": keepalive\n\n"); err != nil {
				return
			}
			flusher.Flush()
		case msg, open := <-sub.C():
			if !open {
				return
			}
			envWorkspace, obj, id, valid := decodeLiveEnvelope(msg)
			// Defense in depth: the subscription is workspace-scoped,
			// but never emit a row for a different workspace.
			if !valid || envWorkspace != workspaceID {
				continue
			}
			if id <= lastID {
				continue // already replayed or already sent
			}
			if !filters.matches(obj) {
				continue
			}
			if err := writeLogEvent(w, flusher, id, obj); err != nil {
				return
			}
			lastID = id
		}
	}
}

// logRowToStreamObj reuses the shared envelope shape so replayed rows
// are byte-identical to live broadcast rows.
func logRowToStreamObj(row db.IntegrationLog) map[string]any {
	env := ws.LogEnvelope(row)
	if obj, ok := env["log"].(map[string]any); ok {
		return obj
	}
	return map[string]any{}
}

// decodeLiveEnvelope parses a broadcast envelope, keeping numeric ids
// exact via UseNumber so the SSE id line is an integer.
func decodeLiveEnvelope(msg []byte) (workspaceID string, logObj map[string]any, id int64, ok bool) {
	dec := json.NewDecoder(bytes.NewReader(msg))
	dec.UseNumber()
	var env struct {
		Type        string         `json:"type"`
		WorkspaceID string         `json:"workspace_id"`
		Log         map[string]any `json:"log"`
	}
	if err := dec.Decode(&env); err != nil {
		return "", nil, 0, false
	}
	if env.Type != "logs.new" || env.Log == nil {
		return "", nil, 0, false
	}
	num, ok := env.Log["id"].(json.Number)
	if !ok {
		return "", nil, 0, false
	}
	parsed, err := num.Int64()
	if err != nil {
		return "", nil, 0, false
	}
	return env.WorkspaceID, env.Log, parsed, true
}

// writeStreamError emits a terminal SSE error event. Used when a
// failure happens after the 200 headers are already committed.
func writeStreamError(w http.ResponseWriter, flusher http.Flusher, code, message string) {
	data, err := json.Marshal(map[string]string{"code": code, "message": message})
	if err != nil {
		return
	}
	if _, err := fmt.Fprintf(w, "event: error\ndata: %s\n\n", data); err != nil {
		return
	}
	flusher.Flush()
}

func writeLogEvent(w http.ResponseWriter, flusher http.Flusher, id int64, logObj map[string]any) error {
	data, err := json.Marshal(logObj)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "event: log.created\nid: %d\ndata: %s\n\n", id, data); err != nil {
		return err
	}
	flusher.Flush()
	return nil
}
