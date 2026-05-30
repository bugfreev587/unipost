package ws

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/clerk/clerk-sdk-go/v2"
	"github.com/clerk/clerk-sdk-go/v2/jwks"
	"github.com/clerk/clerk-sdk-go/v2/jwt"
	"github.com/coder/websocket"

	"github.com/xiaoboyu/unipost-api/internal/db"
	appmw "github.com/xiaoboyu/unipost-api/internal/middleware"
)

type errorBody struct {
	Code           string `json:"code"`
	NormalizedCode string `json:"normalized_code,omitempty"`
	Message        string `json:"message"`
}

type errorResponse struct {
	Error     errorBody `json:"error"`
	RequestID string    `json:"request_id,omitempty"`
}

type inboxPlanChecker interface {
	PlanAllowsInbox(context.Context, string) bool
}

// Handler upgrades an HTTP request to a WebSocket connection.
// Auth: Clerk JWT is passed as ?token=<jwt> query param since the
// browser WebSocket API doesn't support custom headers. The workspace
// is resolved from the user's default workspace (single-workspace
// product surface), matching DualAuthMiddleware's Clerk path.
type Handler struct {
	Hub         *Hub
	queries     *db.Queries
	planChecker inboxPlanChecker
}

func NewHandler(hub *Hub, queries *db.Queries) *Handler {
	return &Handler{Hub: hub, queries: queries}
}

func (h *Handler) WithInboxPlanGate(checker inboxPlanChecker) *Handler {
	h.planChecker = checker
	return h
}

func writeWSError(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(errorResponse{
		Error: errorBody{
			Code:           code,
			NormalizedCode: strings.ToLower(code),
			Message:        message,
		},
		RequestID: appmw.GetRequestID(r.Context()),
	})
}

func (h *Handler) ensureInboxPlanAllowed(w http.ResponseWriter, r *http.Request, workspaceID string) bool {
	if h.planChecker != nil && !h.planChecker.PlanAllowsInbox(r.Context(), workspaceID) {
		writeWSError(w, r, http.StatusPaymentRequired, "PLAN_FEATURE_NOT_AVAILABLE",
			"Inbox requires the Basic plan or higher - upgrade at unipost.dev/pricing")
		return false
	}
	return true
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		writeWSError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Missing token query param")
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
		writeWSError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid token")
		return
	}

	workspace, err := h.queries.GetDefaultWorkspaceForUser(r.Context(), claims.Subject)
	if err != nil {
		slog.Warn("ws: no workspace for user", "user_id", claims.Subject, "err", err)
		writeWSError(w, r, http.StatusForbidden, "FORBIDDEN", "No workspace found for user")
		return
	}

	if !h.ensureInboxPlanAllowed(w, r, workspace.ID) {
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
