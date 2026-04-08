package auth

import (
	"context"
	"net/http"
	"os"
	"strings"

	"github.com/xiaoboyu/unipost-api/internal/db"
)

// AdminMiddleware gates a route on whether the authenticated Clerk user's
// email matches the ADMIN_EMAIL env var. It MUST be mounted inside a group
// that already runs ClerkSessionMiddleware so the userID is in context.
//
// On startup, ADMIN_EMAIL=foo@bar.com pins a single account; missing or
// empty ADMIN_EMAIL locks the route down (every request returns 403).
func AdminMiddleware(queries *db.Queries) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			adminEmail := strings.ToLower(strings.TrimSpace(os.Getenv("ADMIN_EMAIL")))
			if adminEmail == "" {
				writeAdminForbidden(w, "Admin access disabled")
				return
			}

			userID := GetUserID(r.Context())
			if userID == "" {
				writeAdminForbidden(w, "Not authenticated")
				return
			}

			user, err := queries.GetUser(r.Context(), userID)
			if err != nil {
				writeAdminForbidden(w, "Admin lookup failed")
				return
			}

			if !strings.EqualFold(strings.TrimSpace(user.Email), adminEmail) {
				writeAdminForbidden(w, "Not an admin")
				return
			}

			// Stash the resolved email for handlers that want to log it.
			ctx := context.WithValue(r.Context(), AdminEmailKey, user.Email)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

const AdminEmailKey contextKey = "adminEmail"

func writeAdminForbidden(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	_, _ = w.Write([]byte(`{"error":{"code":"FORBIDDEN","message":"` + msg + `"}}`))
}
