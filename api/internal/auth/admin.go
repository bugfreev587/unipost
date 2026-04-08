package auth

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/xiaoboyu/unipost-api/internal/db"
)

// AdminChecker resolves whether a Clerk user ID is on the ADMIN_USERS
// allowlist that gates /v1/admin/* and the dashboard's Admin entry.
//
// ADMIN_USERS is intentionally separate from SUPER_ADMINS:
//
//   - SUPER_ADMINS routes a user to Stripe sandbox (internal QA billing).
//   - ADMIN_USERS grants access to the cross-tenant admin panel.
//
// The two often overlap in practice but the concerns are distinct, so
// they live in different env vars and can be set independently.
//
// Entries follow the same shape as SUPER_ADMINS: comma-separated, where
// each entry is either a Clerk user ID (matched directly, no DB hit) or
// an email address (resolved to a user ID via queries.GetUser the first
// time it's checked, then cached for the lifetime of the process). Mix
// and match in one list. Email matching is case-insensitive.
type AdminChecker struct {
	queries *db.Queries

	userIDs map[string]bool // direct Clerk user ID entries
	emails  map[string]bool // lower-cased email entries

	mu       sync.RWMutex
	resolved map[string]bool // cache of email-resolution results, by user ID
}

// NewAdminChecker reads ADMIN_USERS from the environment. Empty/missing
// locks the admin panel down — every IsAdmin call returns false.
func NewAdminChecker(queries *db.Queries) *AdminChecker {
	userIDs, emails := parseAdminUsers(os.Getenv("ADMIN_USERS"))
	slog.Info("admin checker configured",
		"admin_user_ids", len(userIDs),
		"admin_emails", len(emails))
	return &AdminChecker{
		queries:  queries,
		userIDs:  userIDs,
		emails:   emails,
		resolved: make(map[string]bool),
	}
}

// parseAdminUsers splits the ADMIN_USERS env var into direct Clerk user
// IDs and email addresses. The split is by the presence of "@". Stray
// brackets / quotes are stripped so a JSON-array-looking value still
// parses cleanly.
func parseAdminUsers(raw string) (userIDs map[string]bool, emails map[string]bool) {
	userIDs = make(map[string]bool)
	emails = make(map[string]bool)
	for _, s := range strings.Split(raw, ",") {
		s = strings.TrimSpace(s)
		s = strings.Trim(s, `[]"' `)
		if s == "" {
			continue
		}
		if strings.Contains(s, "@") {
			emails[strings.ToLower(s)] = true
		} else {
			userIDs[s] = true
		}
	}
	return userIDs, emails
}

// IsAdmin reports whether the given Clerk user ID is on ADMIN_USERS.
// Returns false on missing user / DB error so a transient hiccup never
// accidentally grants access.
func (c *AdminChecker) IsAdmin(ctx context.Context, userID string) bool {
	if c == nil || userID == "" {
		return false
	}

	// Direct user ID entry — fast path, no lock needed (the maps are
	// only written at construction).
	if c.userIDs[userID] {
		return true
	}

	if len(c.emails) == 0 {
		return false
	}

	c.mu.RLock()
	if v, ok := c.resolved[userID]; ok {
		c.mu.RUnlock()
		return v
	}
	c.mu.RUnlock()

	user, err := c.queries.GetUser(ctx, userID)
	if err != nil {
		slog.Warn("admin: user lookup failed", "user_id", userID, "error", err)
		return false
	}
	isAdmin := c.emails[strings.ToLower(strings.TrimSpace(user.Email))]

	c.mu.Lock()
	c.resolved[userID] = isAdmin
	c.mu.Unlock()

	if isAdmin {
		slog.Info("admin: resolved email entry", "user_id", userID, "email", user.Email)
	}
	return isAdmin
}

// AdminMiddleware gates a route on the ADMIN_USERS allowlist. It MUST
// be mounted inside a group that already runs ClerkSessionMiddleware so
// the userID is in context.
func AdminMiddleware(checker *AdminChecker) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID := GetUserID(r.Context())
			if userID == "" {
				writeAdminForbidden(w, "Not authenticated")
				return
			}
			if !checker.IsAdmin(r.Context(), userID) {
				writeAdminForbidden(w, "Not an admin")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func writeAdminForbidden(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	_, _ = w.Write([]byte(`{"error":{"code":"FORBIDDEN","message":"` + msg + `"}}`))
}
