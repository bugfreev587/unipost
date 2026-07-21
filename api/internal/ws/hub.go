// Package ws provides a WebSocket hub for real-time inbox delivery.
// The Hub manages per-workspace connections and broadcasts new inbox
// items to all connected clients.

package ws

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"

	"github.com/xiaoboyu/unipost-api/internal/inboxaccess"
)

// Conn wraps a single WebSocket connection.
type Conn struct {
	ws   *websocket.Conn
	send chan []byte
}

// Subscription is a non-WebSocket consumer of a workspace's broadcast
// stream. SSE handlers use it to receive the same raw envelopes the
// WebSocket clients get, without opening a PostgreSQL LISTEN connection
// per client. The buffered channel decouples a slow reader from the
// broadcaster; messages are dropped (not blocked) when the buffer is
// full, matching the WebSocket slow-client behavior.
type Subscription struct {
	ch chan []byte
}

// scopeKey is the canonical realtime partition for Inbox traffic. It is kept
// separate from the legacy workspace-only key used by logs and SSE.
type scopeKey struct {
	workspaceID    string
	mode           inboxaccess.Mode
	externalUserID string
}

func keyForScope(scope inboxaccess.Scope) (scopeKey, bool) {
	if scope.WorkspaceID == "" || strings.TrimSpace(scope.WorkspaceID) != scope.WorkspaceID {
		return scopeKey{}, false
	}

	switch scope.Mode {
	case inboxaccess.ModeWorkspace:
		if scope.ExternalUserID != "" {
			return scopeKey{}, false
		}
	case inboxaccess.ModeManagedUser:
		if scope.ExternalUserID == "" || strings.TrimSpace(scope.ExternalUserID) != scope.ExternalUserID {
			return scopeKey{}, false
		}
	default:
		return scopeKey{}, false
	}

	return scopeKey{
		workspaceID:    scope.WorkspaceID,
		mode:           scope.Mode,
		externalUserID: scope.ExternalUserID,
	}, true
}

// C returns the receive channel. It is closed by Unsubscribe.
func (s *Subscription) C() <-chan []byte { return s.ch }

// Hub manages WebSocket connections grouped by workspace ID.
type Hub struct {
	mu          sync.RWMutex
	conns       map[string]map[*Conn]struct{}         // workspaceID -> set of legacy connections
	subs        map[string]map[*Subscription]struct{} // workspaceID -> set of legacy raw subscribers
	scopedConns map[scopeKey]map[*Conn]struct{}
	scopedSubs  map[scopeKey]map[*Subscription]struct{}
}

func NewHub() *Hub {
	return &Hub{
		conns:       make(map[string]map[*Conn]struct{}),
		subs:        make(map[string]map[*Subscription]struct{}),
		scopedConns: make(map[scopeKey]map[*Conn]struct{}),
		scopedSubs:  make(map[scopeKey]map[*Subscription]struct{}),
	}
}

// SubscribeScope registers an Inbox subscriber only when scope is canonical.
// Malformed scopes fail closed instead of being widened to workspace access.
func (h *Hub) SubscribeScope(scope inboxaccess.Scope) (*Subscription, bool) {
	key, ok := keyForScope(scope)
	if !ok {
		return nil, false
	}

	s := &Subscription{ch: make(chan []byte, 256)}
	h.mu.Lock()
	if h.scopedSubs[key] == nil {
		h.scopedSubs[key] = make(map[*Subscription]struct{})
	}
	h.scopedSubs[key][s] = struct{}{}
	h.mu.Unlock()
	return s, true
}

// UnsubscribeScope removes a canonical Inbox subscriber and closes its
// channel. It returns false for malformed scopes and unknown subscriptions.
func (h *Hub) UnsubscribeScope(scope inboxaccess.Scope, s *Subscription) bool {
	key, ok := keyForScope(scope)
	if !ok || s == nil {
		return false
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	set, ok := h.scopedSubs[key]
	if !ok {
		return false
	}
	if _, ok := set[s]; !ok {
		return false
	}
	delete(set, s)
	close(s.ch)
	if len(set) == 0 {
		delete(h.scopedSubs, key)
	}
	return true
}

// Subscribe registers a raw-byte subscriber for a workspace and returns
// it. Callers must Unsubscribe when done.
func (h *Hub) Subscribe(workspaceID string) *Subscription {
	s := &Subscription{ch: make(chan []byte, 256)}
	h.mu.Lock()
	if h.subs[workspaceID] == nil {
		h.subs[workspaceID] = make(map[*Subscription]struct{})
	}
	h.subs[workspaceID][s] = struct{}{}
	h.mu.Unlock()
	return s
}

// Unsubscribe removes a subscriber and closes its channel. Safe to call
// once per Subscription.
func (h *Hub) Unsubscribe(workspaceID string, s *Subscription) {
	h.mu.Lock()
	defer h.mu.Unlock()
	set, ok := h.subs[workspaceID]
	if !ok {
		return
	}
	if _, ok := set[s]; ok {
		delete(set, s)
		close(s.ch)
		if len(set) == 0 {
			delete(h.subs, workspaceID)
		}
	}
}

func (h *Hub) Register(workspaceID string, c *Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.conns[workspaceID] == nil {
		h.conns[workspaceID] = make(map[*Conn]struct{})
	}
	h.conns[workspaceID][c] = struct{}{}
	slog.Info("ws: client connected", "workspace_id", workspaceID, "total", len(h.conns[workspaceID]))
}

// RegisterScope registers an Inbox connection under an exact canonical scope.
func (h *Hub) RegisterScope(scope inboxaccess.Scope, c *Conn) bool {
	key, ok := keyForScope(scope)
	if !ok || c == nil || c.send == nil {
		return false
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	if h.scopedConns[key] == nil {
		h.scopedConns[key] = make(map[*Conn]struct{})
	}
	h.scopedConns[key][c] = struct{}{}
	slog.Info("ws: inbox client connected", "workspace_id", key.workspaceID, "scope_mode", key.mode, "total", len(h.scopedConns[key]))
	return true
}

// UnregisterScope removes a connection only from its exact Inbox partition.
func (h *Hub) UnregisterScope(scope inboxaccess.Scope, c *Conn) bool {
	key, ok := keyForScope(scope)
	if !ok || c == nil {
		return false
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	set, ok := h.scopedConns[key]
	if !ok {
		return false
	}
	if _, ok := set[c]; !ok {
		return false
	}
	delete(set, c)
	close(c.send)
	if len(set) == 0 {
		delete(h.scopedConns, key)
	}
	slog.Info("ws: inbox client disconnected", "workspace_id", key.workspaceID, "scope_mode", key.mode)
	return true
}

func (h *Hub) Unregister(workspaceID string, c *Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if set, ok := h.conns[workspaceID]; ok {
		delete(set, c)
		if len(set) == 0 {
			delete(h.conns, workspaceID)
		}
	}
	close(c.send)
	slog.Info("ws: client disconnected", "workspace_id", workspaceID)
}

// Broadcast sends a message to all WebSocket connections and raw
// subscribers for a workspace.
//
// The read lock is held for the whole fan-out. The sends are
// non-blocking (select/default), so holding the lock cannot stall, and
// it prevents Unregister/Unsubscribe from deleting an entry or closing
// a channel mid-iteration — which would otherwise race the map or panic
// on send-to-closed-channel.
func (h *Hub) Broadcast(workspaceID string, msg []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for c := range h.conns[workspaceID] {
		select {
		case c.send <- msg:
		default:
			// Client is slow, skip this message.
			slog.Warn("ws: dropping message for slow client", "workspace_id", workspaceID)
		}
	}

	for s := range h.subs[workspaceID] {
		select {
		case s.ch <- msg:
		default:
			// Subscriber is slow, skip this message.
			slog.Warn("ws: dropping message for slow subscriber", "workspace_id", workspaceID)
		}
	}
}

// BroadcastInbox delivers an Inbox event to the workspace aggregate and, when
// present, the exact managed-user partition in the same workspace. Empty
// externalUserID represents owner/BYO traffic and reaches only the aggregate.
// It intentionally never fans out through the legacy logs/SSE maps.
func (h *Hub) BroadcastInbox(workspaceID, externalUserID string, msg []byte) {
	if workspaceID == "" {
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	h.broadcastInboxKey(scopeKey{workspaceID: workspaceID, mode: inboxaccess.ModeWorkspace}, msg)
	if externalUserID != "" {
		h.broadcastInboxKey(scopeKey{
			workspaceID:    workspaceID,
			mode:           inboxaccess.ModeManagedUser,
			externalUserID: externalUserID,
		}, msg)
	}
}

func (h *Hub) broadcastInboxKey(key scopeKey, msg []byte) {
	for c := range h.scopedConns[key] {
		select {
		case c.send <- msg:
		default:
			slog.Warn("ws: dropping inbox message for slow client", "workspace_id", key.workspaceID, "scope_mode", key.mode)
		}
	}

	for s := range h.scopedSubs[key] {
		select {
		case s.ch <- msg:
		default:
			slog.Warn("ws: dropping inbox message for slow subscriber", "workspace_id", key.workspaceID, "scope_mode", key.mode)
		}
	}
}

// BroadcastInboxItem serializes an inbox item and broadcasts it.
func (h *Hub) BroadcastInboxItem(workspaceID, externalUserID string, item any) {
	msg, err := json.Marshal(map[string]any{
		"type": "inbox.new_item",
		"item": item,
	})
	if err != nil {
		return
	}
	h.BroadcastInbox(workspaceID, externalUserID, msg)
}

// ServeConn runs the read/write pumps for a connection. Blocks until
// the connection closes. Called as a goroutine from the HTTP handler.
func (h *Hub) ServeConn(parentCtx context.Context, workspaceID string, ws *websocket.Conn) {
	// Detach from the HTTP request context so the connection stays
	// alive after the handler returns. Use a cancel for cleanup.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	c := &Conn{ws: ws, send: make(chan []byte, 64)}
	h.Register(workspaceID, c)
	defer h.Unregister(workspaceID, c)

	// Write pump: drain send channel to the WebSocket.
	go func() {
		for msg := range c.send {
			writeCtx, writeCancel := context.WithTimeout(ctx, 5*time.Second)
			err := ws.Write(writeCtx, websocket.MessageText, msg)
			writeCancel()
			if err != nil {
				cancel()
				return
			}
		}
	}()

	// Read pump: keep alive, detect disconnect.
	for {
		_, _, err := ws.Read(ctx)
		if err != nil {
			return
		}
	}
}

// ServeScopedConn runs a WebSocket under an exact Inbox scope. Invalid scopes
// fail closed and are never registered in a workspace aggregate.
func (h *Hub) ServeScopedConn(parentCtx context.Context, scope inboxaccess.Scope, ws *websocket.Conn) {
	if _, ok := keyForScope(scope); !ok || ws == nil {
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	c := &Conn{ws: ws, send: make(chan []byte, 64)}
	if !h.RegisterScope(scope, c) {
		return
	}
	defer h.UnregisterScope(scope, c)

	go func() {
		for msg := range c.send {
			writeCtx, writeCancel := context.WithTimeout(ctx, 5*time.Second)
			err := ws.Write(writeCtx, websocket.MessageText, msg)
			writeCancel()
			if err != nil {
				cancel()
				return
			}
		}
	}()

	for {
		_, _, err := ws.Read(ctx)
		if err != nil {
			return
		}
	}
}
