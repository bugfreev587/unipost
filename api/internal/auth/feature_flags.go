// feature_flags.go holds the server-side kill-switches for features
// that are gradually rolled out. Each flag is a simple bool read from
// an env var at process startup; toggling requires a redeploy. For
// per-user rollout (like the dashboard Inbox tab's
// NEXT_PUBLIC_FEATURE_INBOX) the frontend owns the gate separately.
//
// Backend flags exist so an accidentally-leaked frontend flag flip
// can't let unreviewed adapter calls reach a third-party API —
// notably Facebook, where Meta's App Review will flag unexpected
// traffic against scopes that haven't been approved yet.

package auth

import (
	"log/slog"
	"net/http"
	"os"
	"strings"
)

// FacebookEnabled reports whether ENABLE_FACEBOOK_PAGES is set to a
// truthy value. Anything other than "true" / "1" / "yes" keeps the
// flag OFF — explicit opt-in only.
func FacebookEnabled() bool {
	return isTruthyFlag(os.Getenv("ENABLE_FACEBOOK_PAGES"))
}

func isTruthyFlag(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "true", "1", "yes", "on":
		return true
	}
	return false
}

// RequireFacebookEnabled is a chi-compatible middleware that rejects
// requests when the Facebook feature flag is off. Attach it to FB-
// specific routes (OAuth connect, callback, pending-connection
// finalize) so neither the dashboard nor a curious API caller can
// drive traffic through Meta while we're still in audit.
func RequireFacebookEnabled(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !FacebookEnabled() {
			slog.Info("facebook flag off; rejecting request",
				"path", r.URL.Path, "method", r.Method)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"error":{"code":"FACEBOOK_DISABLED","message":"Facebook integration is not enabled"}}`))
			return
		}
		next.ServeHTTP(w, r)
	})
}
