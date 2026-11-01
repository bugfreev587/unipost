// feature_flags.go holds server-side gates for features that are
// gradually rolled out to the SUPER_ADMINS internal allowlist.

package auth

import (
	"log/slog"
	"net/http"
)

func RequireSuperAdmin(checker *SuperAdminChecker, code, message string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID := GetUserID(r.Context())
			if userID == "" || !checker.IsSuperAdmin(r.Context(), userID) {
				slog.Info("super-admin gate: request rejected",
					"path", r.URL.Path, "method", r.Method, "user_id", userID)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				_, _ = w.Write([]byte(`{"error":{"code":"` + code + `","message":"` + message + `"}}`))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
