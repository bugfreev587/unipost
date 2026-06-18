// Package ws provides a WebSocket hub for real-time inbox delivery.
// The Hub manages per-workspace connections and broadcasts new inbox
// items to all connected clients.

package ws

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/coder/websocket"
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

// C returns the receive channel. It is closed by Unsubscribe.
func (s *Subscription) C() <-chan []byte { return s.ch }

// Hub manages WebSocket connections grouped by workspace ID.
type Hub struct {
	mu    sync.RWMutex
	conns map[string]map[*Conn]struct{}         // workspaceID -> set of connections
	subs  map[string]map[*Subscription]struct{} // workspaceID -> set of raw subscribers
}

func NewHub() *Hub {
	return &Hub{
		conns: make(map[string]map[*Conn]struct{}),
		subs:  make(map[string]map[*Subscription]struct{}),
	}
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
func (h *Hub) Broadcast(workspaceID string, msg []byte) {
	h.mu.RLock()
	conns := h.conns[workspaceID]
	subs := h.subs[workspaceID]
	h.mu.RUnlock()

	for c := range conns {
		select {
		case c.send <- msg:
		default:
			// Client is slow, skip this message.
			slog.Warn("ws: dropping message for slow client", "workspace_id", workspaceID)
		}
	}

	for s := range subs {
		select {
		case s.ch <- msg:
		default:
			// Subscriber is slow, skip this message.
			slog.Warn("ws: dropping message for slow subscriber", "workspace_id", workspaceID)
		}
	}
}

// BroadcastInboxItem serializes an inbox item and broadcasts it.
func (h *Hub) BroadcastInboxItem(workspaceID string, item any) {
	msg, err := json.Marshal(map[string]any{
		"type": "inbox.new_item",
		"item": item,
	})
	if err != nil {
		return
	}
	h.Broadcast(workspaceID, msg)
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
