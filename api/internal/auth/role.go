package auth

import (
	"net/http"
)

// RequireRole is a chi-style middleware that enforces a minimum
// workspace role on the wrapped handler. Roles use a numeric ladder
// (RoleLevel in unkey.go) so a single helper covers every gate:
//
//	r.With(auth.RequireRole(auth.RoleAdmin)).Post("/v1/members/invite", h.Invite)
//
// The middleware MUST run after the auth middleware that stamps the
// role into the request context (DualAuthMiddleware / ClerkSessionMiddleware).
// A missing role is treated as 403 with no log noise — the auth layer
// is responsible for catching unauthenticated requests with 401 first.
//
// Today every API-key-authenticated request carries owner role (see
// dualauth.go authenticateAPIKey). Clerk-authenticated requests carry
// the role from the user's active workspace membership. This shape
// will not change when the invite flow ships — we just gain non-owner
// memberships in the table.
func RequireRole(min string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			role := GetRole(r.Context())
			if RoleLevel(role) < RoleLevel(min) {
				writeJSON(w, http.StatusForbidden, map[string]any{
					"error": map[string]any{
						"code":    "INSUFFICIENT_ROLE",
						"message": "this action requires the " + min + " role or higher",
					},
				})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
