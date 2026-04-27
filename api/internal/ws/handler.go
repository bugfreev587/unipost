package ws

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/clerk/clerk-sdk-go/v2"
	"github.com/clerk/clerk-sdk-go/v2/jwt"
	"github.com/clerk/clerk-sdk-go/v2/jwks"
	"github.com/coder/websocket"

	"github.com/xiaoboyu/unipost-api/internal/db"
)

// Handler upgrades an HTTP request to a WebSocket connection.
// Auth: Clerk JWT is passed as ?token=<jwt> query param since the
// browser WebSocket API doesn't support custom headers. The workspace
// is resolved from the user's default workspace (single-workspace
// product surface), matching DualAuthMiddleware's Clerk path.
type Handler struct {
	Hub     *Hub
	queries *db.Queries
}

func NewHandler(hub *Hub, queries *db.Queries) *Handler {
	return &Handler{Hub: hub, queries: queries}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		http.Error(w, `{"error":"missing token query param"}`, http.StatusUnauthorized)
		return
	}

	clerk.SetKey(os.Getenv("CLERK_SECRET_KEY"))
	client := jwks.NewClient(&clerk.ClientConfig{
		BackendConfig: clerk.BackendConfig{
			Key: clerk.String(os.Getenv("CLERK_SECRET_KEY")),
		},
	})
	claims, err := jwt.Verify(r.Context(), &jwt.VerifyParams{
		Token:      token,
		JWKSClient: client,
	})
	if err != nil {
		slog.Warn("ws: auth failed", "err", err)
		http.Error(w, `{"error":"invalid token"}`, http.StatusUnauthorized)
		return
	}

	workspace, err := h.queries.GetDefaultWorkspaceForUser(r.Context(), claims.Subject)
	if err != nil {
		slog.Warn("ws: no workspace for user", "user_id", claims.Subject, "err", err)
		http.Error(w, `{"error":"no workspace"}`, http.StatusForbidden)
		return
	}

	ws, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: []string{"*.unipost.dev", "localhost:*", "*"},
	})
	if err != nil {
		slog.Warn("ws: accept failed", "err", err)
		return
	}

	slog.Info("ws: upgrading", "workspace_id", workspace.ID)
	h.Hub.ServeConn(r.Context(), workspace.ID, ws)
}
