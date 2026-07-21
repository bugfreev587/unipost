package handler

import (
	"log/slog"
	"net/http"

	"github.com/xiaoboyu/unipost-api/internal/auth"
	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/inboxaccess"
)

// RequireInboxAccessScope resolves the authenticated request's Inbox access
// boundary before any plan, provider, or business logic runs.
func RequireInboxAccessScope(queries *db.Queries) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			scope, failure := inboxaccess.Resolve(r, queries)
			if failure != nil {
				slog.Warn("Inbox scope rejected",
					"event", "inbox_scope_rejected",
					"reason", failure.Code,
					"workspace_id", auth.GetWorkspaceID(r.Context()),
				)
				writeError(w, failure.Status, failure.Code, failure.Message)
				return
			}
			next.ServeHTTP(w, r.WithContext(inboxaccess.WithContext(r.Context(), scope)))
		})
	}
}
