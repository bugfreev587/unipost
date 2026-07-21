package inboxaccess

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/auth"
	"github.com/xiaoboyu/unipost-api/internal/db"
)

type Mode string

const (
	ModeWorkspace   Mode = "workspace"
	ModeManagedUser Mode = "managed_user"
)

type Scope struct {
	WorkspaceID    string
	Mode           Mode
	ExternalUserID string
}

func (s Scope) WorkspaceWide() bool {
	return s.Mode == ModeWorkspace
}

type Failure struct {
	Status  int
	Code    string
	Message string
}

type scopeContextKey struct{}

func FromContext(ctx context.Context) (Scope, bool) {
	scope, ok := ctx.Value(scopeContextKey{}).(Scope)
	return scope, ok
}

func WithContext(ctx context.Context, scope Scope) context.Context {
	return context.WithValue(ctx, scopeContextKey{}, scope)
}

func Resolve(r *http.Request, queries *db.Queries) (Scope, *Failure) {
	if auth.GetWorkspaceID(r.Context()) == "" {
		return Scope{}, failure(http.StatusUnauthorized, "UNAUTHORIZED", "Authenticated workspace is required")
	}

	query, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		return Scope{}, failure(http.StatusBadRequest, "INBOX_QUERY_INVALID", "Inbox query parameters are malformed")
	}
	return ResolveQuery(r, queries, query)
}

// ResolveQuery resolves an already strictly parsed query. It lets transports
// such as the Inbox WebSocket handshake validate their route-specific query
// contract without reparsing RawQuery and potentially accepting a different
// partial interpretation.
func ResolveQuery(r *http.Request, queries *db.Queries, query url.Values) (Scope, *Failure) {
	workspaceID := auth.GetWorkspaceID(r.Context())
	if workspaceID == "" {
		return Scope{}, failure(http.StatusUnauthorized, "UNAUTHORIZED", "Authenticated workspace is required")
	}

	isAPIKey := auth.GetAPIKeyID(r.Context()) != ""
	mode, resolveFailure := resolveMode(query["inbox_scope"], isAPIKey)
	if resolveFailure != nil {
		return Scope{}, resolveFailure
	}

	if !isAPIKey && auth.RoleLevel(auth.GetRole(r.Context())) < auth.RoleLevel(auth.RoleAdmin) {
		return Scope{}, insufficientRoleFailure()
	}

	switch mode {
	case ModeWorkspace:
		if _, present := query["external_user_id"]; present {
			return Scope{}, failure(http.StatusBadRequest, "EXTERNAL_USER_ID_NOT_ALLOWED", "external_user_id is not allowed for workspace scope")
		}
		if auth.RoleLevel(auth.GetRole(r.Context())) < auth.RoleLevel(auth.RoleAdmin) {
			return Scope{}, insufficientRoleFailure()
		}
		if isAPIKey && !auth.GetAPIKeyCreatorBound(r.Context()) {
			return Scope{}, failure(http.StatusForbidden, "API_KEY_CREATOR_REQUIRED", "Workspace scope requires a creator-bound API key")
		}
		return Scope{WorkspaceID: workspaceID, Mode: ModeWorkspace}, nil

	case ModeManagedUser:
		externalUserID, externalFailure := resolveExternalUserID(query["external_user_id"])
		if externalFailure != nil {
			return Scope{}, externalFailure
		}
		if queries == nil {
			return Scope{}, managedUserLookupFailure()
		}
		exists, err := queries.InboxManagedUserExists(r.Context(), db.InboxManagedUserExistsParams{
			WorkspaceID: workspaceID,
			ExternalUserID: pgtype.Text{
				String: externalUserID,
				Valid:  true,
			},
		})
		if err != nil {
			return Scope{}, managedUserLookupFailure()
		}
		if !exists {
			return Scope{}, failure(http.StatusNotFound, "MANAGED_USER_NOT_FOUND", "Managed user was not found")
		}
		return Scope{WorkspaceID: workspaceID, Mode: ModeManagedUser, ExternalUserID: externalUserID}, nil

	default:
		return Scope{}, failure(http.StatusBadRequest, "INBOX_SCOPE_INVALID", "inbox_scope is invalid")
	}
}

func resolveMode(values []string, required bool) (Mode, *Failure) {
	if len(values) == 0 {
		if required {
			return "", failure(http.StatusBadRequest, "INBOX_SCOPE_REQUIRED", "inbox_scope is required for API-key requests")
		}
		return ModeWorkspace, nil
	}
	if len(values) != 1 {
		return "", failure(http.StatusBadRequest, "INBOX_SCOPE_DUPLICATE", "inbox_scope must be provided exactly once")
	}

	mode := Mode(strings.TrimSpace(values[0]))
	if mode != ModeManagedUser && mode != ModeWorkspace {
		return "", failure(http.StatusBadRequest, "INBOX_SCOPE_INVALID", "inbox_scope must be managed_user or workspace")
	}
	return mode, nil
}

func resolveExternalUserID(values []string) (string, *Failure) {
	if len(values) == 0 {
		return "", failure(http.StatusBadRequest, "EXTERNAL_USER_ID_REQUIRED", "external_user_id is required for managed-user scope")
	}
	if len(values) != 1 {
		return "", failure(http.StatusBadRequest, "EXTERNAL_USER_ID_DUPLICATE", "external_user_id must be provided exactly once")
	}

	externalUserID := strings.TrimSpace(values[0])
	if externalUserID == "" {
		return "", failure(http.StatusBadRequest, "EXTERNAL_USER_ID_REQUIRED", "external_user_id is required for managed-user scope")
	}
	return externalUserID, nil
}

func insufficientRoleFailure() *Failure {
	return failure(http.StatusForbidden, "INSUFFICIENT_ROLE", "Inbox access requires the admin role or higher")
}

func managedUserLookupFailure() *Failure {
	return failure(http.StatusInternalServerError, "INBOX_SCOPE_LOOKUP_FAILED", "Unable to resolve Inbox access scope")
}

func failure(status int, code, message string) *Failure {
	return &Failure{Status: status, Code: code, Message: message}
}
