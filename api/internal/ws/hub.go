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

// Hub manages WebSocket connections grouped by workspace ID.
type Hub struct {
	mu    sync.RWMutex
	conns map[string]map[*Conn]struct{} // workspaceID -> set of connections
}

func NewHub() *Hub {
	return &Hub{conns: make(map[string]map[*Conn]struct{})}
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

// Broadcast sends a message to all connections for a workspace.
func (h *Hub) Broadcast(workspaceID string, msg []byte) {
	h.mu.RLock()
	conns := h.conns[workspaceID]
	h.mu.RUnlock()

	for c := range conns {
		select {
		case c.send <- msg:
		default:
			// Client is slow, skip this message.
			slog.Warn("ws: dropping message for slow client", "workspace_id", workspaceID)
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
