// feature_flags.go holds the server-side kill-switches for features
// that are gradually rolled out. Each flag gates on the caller's
// identity — typically "is this user on the SUPER_ADMINS internal
// allowlist". Toggling requires an env-var redeploy of SUPER_ADMINS.
//
// Facebook Pages lives here because Meta's App Review watches for
// traffic against scopes the app hasn't been approved for yet — an
// accidentally-leaked frontend flag flip could send unreviewed
// requests to production Meta. Routing the gate through the same
// SUPER_ADMINS allowlist the billing sandbox uses keeps internal
// testing isolated while we wait for audit approval.

package auth

import (
	"log/slog"
	"net/http"
)

// RequireFacebookSuperAdmin is a chi-compatible middleware that
// rejects requests when the authenticated Clerk user isn't on
// SUPER_ADMINS. MUST be mounted inside a group that already runs
// ClerkSessionMiddleware so the user ID is in context.
//
// Returns 403 FACEBOOK_DISABLED so neither the dashboard nor a
// curious direct caller can drive traffic through Meta while we're
// still in App Review.
func RequireFacebookSuperAdmin(checker *SuperAdminChecker) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID := GetUserID(r.Context())
			if userID == "" || !checker.IsSuperAdmin(r.Context(), userID) {
				slog.Info("facebook super-admin gate: request rejected",
					"path", r.URL.Path, "method", r.Method, "user_id", userID)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				_, _ = w.Write([]byte(`{"error":{"code":"FACEBOOK_DISABLED","message":"Facebook integration is not enabled for your account"}}`))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
