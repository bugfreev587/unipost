package ws

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/clerk/clerk-sdk-go/v2"
	"github.com/clerk/clerk-sdk-go/v2/jwt"
	"github.com/clerk/clerk-sdk-go/v2/jwks"
	"github.com/go-chi/chi/v5"
	"github.com/coder/websocket"
)

// Handler upgrades an HTTP request to a WebSocket connection.
// Auth: Clerk JWT is passed as ?token=<jwt> query param since the
// browser WebSocket API doesn't support custom headers.
type Handler struct {
	Hub *Hub
}

func NewHandler(hub *Hub) *Handler {
	return &Handler{Hub: hub}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "workspaceID")
	token := r.URL.Query().Get("token")

	if token == "" {
		http.Error(w, `{"error":"missing token query param"}`, http.StatusUnauthorized)
		return
	}

	// Validate Clerk JWT.
	clerk.SetKey(os.Getenv("CLERK_SECRET_KEY"))
	client := jwks.NewClient(&clerk.ClientConfig{
		BackendConfig: clerk.BackendConfig{
			Key: clerk.String(os.Getenv("CLERK_SECRET_KEY")),
		},
	})
	_, err := jwt.Verify(r.Context(), &jwt.VerifyParams{
		Token:      token,
		JWKSClient: client,
	})
	if err != nil {
		slog.Warn("ws: auth failed", "err", err)
		http.Error(w, `{"error":"invalid token"}`, http.StatusUnauthorized)
		return
	}

	ws, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: []string{"app.unipost.dev", "localhost:3000", "unipost.dev"},
	})
	if err != nil {
		slog.Warn("ws: accept failed", "err", err)
		return
	}

	slog.Info("ws: upgrading", "workspace_id", workspaceID)
	h.Hub.ServeConn(r.Context(), workspaceID, ws)
}
