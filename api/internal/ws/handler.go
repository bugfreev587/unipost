package ws

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/coder/websocket"

	"github.com/xiaoboyu/unipost-api/internal/apikey"
	"github.com/xiaoboyu/unipost-api/internal/auth"
	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/inboxaccess"
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

type tokenAuthenticator func(context.Context, *db.Queries, string) (context.Context, *auth.TokenAuthFailure)
type webSocketAcceptor func(http.ResponseWriter, *http.Request, *websocket.AcceptOptions) (*websocket.Conn, error)
type webSocketServer func(context.Context, string, *websocket.Conn)

// Handler upgrades an HTTP request to a WebSocket connection. Clerk sessions
// resolve their current active workspace membership exactly like
// DualAuthMiddleware. WithInboxScopeAuth additionally permits a workspace API
// key in the Authorization header and resolves a canonical Inbox scope.
type Handler struct {
	Hub                      *Hub
	queries                  *db.Queries
	planChecker              inboxPlanChecker
	scopedInboxAuth          bool
	clerkTokenAuthenticator  tokenAuthenticator
	apiKeyTokenAuthenticator tokenAuthenticator
	acceptWebSocket          webSocketAcceptor
	serveWebSocket           webSocketServer
}

func NewHandler(hub *Hub, queries *db.Queries) *Handler {
	return &Handler{
		Hub:                      hub,
		queries:                  queries,
		clerkTokenAuthenticator:  auth.AuthenticateClerkToken,
		apiKeyTokenAuthenticator: auth.AuthenticateAPIKeyToken,
		acceptWebSocket:          websocket.Accept,
		serveWebSocket:           hub.ServeConn,
	}
}

func (h *Handler) WithInboxPlanGate(checker inboxPlanChecker) *Handler {
	h.planChecker = checker
	return h
}

// WithInboxScopeAuth enables the Inbox-only handshake contract: browser
// Dashboard sessions use a Clerk JWT in the token query value, while customer
// backends use a workspace API key exclusively in the Authorization header.
func (h *Handler) WithInboxScopeAuth() *Handler {
	h.scopedInboxAuth = true
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
	if h.scopedInboxAuth {
		h.serveScopedInbox(w, r)
		return
	}
	h.serveClerkOnly(w, r)
}

func (h *Handler) serveScopedInbox(w http.ResponseWriter, r *http.Request) {
	query, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil || hasForbiddenURLCredential(query) {
		writeWSError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid WebSocket credentials")
		return
	}

	tokenValues, hasToken := query["token"]
	authorizationValues := r.Header.Values("Authorization")
	hasAuthorization := len(authorizationValues) > 0
	if hasToken == hasAuthorization {
		writeWSError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Exactly one WebSocket credential is required")
		return
	}

	var authenticated context.Context
	var failure *auth.TokenAuthFailure
	if hasToken {
		if len(tokenValues) != 1 || !validCredentialToken(tokenValues[0]) || isAPIKeyToken(tokenValues[0]) {
			writeWSError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid WebSocket credentials")
			return
		}
		authenticated, failure = h.clerkTokenAuthenticator(r.Context(), h.queries, tokenValues[0])
	} else {
		apiKeyToken, ok := parseAPIKeyAuthorization(authorizationValues)
		if !ok {
			writeWSError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid WebSocket credentials")
			return
		}
		authenticated, failure = h.apiKeyTokenAuthenticator(r.Context(), h.queries, apiKeyToken)
	}
	if failure != nil {
		writeWSError(w, r, failure.Status, failure.Code, failure.Message)
		return
	}

	authenticatedRequest := r.WithContext(authenticated)
	scope, scopeFailure := inboxaccess.Resolve(authenticatedRequest, h.queries)
	if scopeFailure != nil {
		writeWSError(w, authenticatedRequest, scopeFailure.Status, scopeFailure.Code, scopeFailure.Message)
		return
	}
	authenticatedRequest = authenticatedRequest.WithContext(inboxaccess.WithContext(authenticated, scope))
	h.acceptAndServe(w, authenticatedRequest, scope.WorkspaceID)
}

func (h *Handler) serveClerkOnly(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		writeWSError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Missing token query param")
		return
	}

	authenticated, failure := h.clerkTokenAuthenticator(r.Context(), h.queries, token)
	if failure != nil {
		writeWSError(w, r, failure.Status, failure.Code, failure.Message)
		return
	}
	authenticatedRequest := r.WithContext(authenticated)
	h.acceptAndServe(w, authenticatedRequest, auth.GetWorkspaceID(authenticated))
}

func (h *Handler) acceptAndServe(w http.ResponseWriter, r *http.Request, workspaceID string) {
	if !h.ensureInboxPlanAllowed(w, r, workspaceID) {
		return
	}

	connection, err := h.acceptWebSocket(w, r, &websocket.AcceptOptions{
		OriginPatterns: []string{"*.unipost.dev", "localhost:*", "*"},
	})
	if err != nil {
		slog.Warn("ws: accept failed")
		return
	}

	slog.Info("ws: upgrading", "workspace_id", workspaceID)
	h.serveWebSocket(r.Context(), workspaceID, connection)
}

func hasForbiddenURLCredential(query url.Values) bool {
	for name := range query {
		switch strings.ToLower(name) {
		case "api_key", "apikey", "api-key", "access_token", "authorization":
			return true
		}
	}
	return false
}

func validCredentialToken(token string) bool {
	return token != "" && !strings.ContainsAny(token, " \t\r\n,")
}

func isAPIKeyToken(token string) bool {
	return strings.HasPrefix(token, apikey.PrefixLive) || strings.HasPrefix(token, apikey.PrefixTest)
}

func parseAPIKeyAuthorization(values []string) (string, bool) {
	if len(values) != 1 {
		return "", false
	}
	value := values[0]
	if !strings.HasPrefix(value, "Bearer ") {
		return "", false
	}
	token := strings.TrimPrefix(value, "Bearer ")
	if !validCredentialToken(token) || !isAPIKeyToken(token) {
		return "", false
	}
	return token, true
}
