// super_admin.go exposes the SUPER_ADMINS env var as a user allowlist
// auth-package consumers can check. Parallel to AdminChecker (which
// guards the admin panel via ADMIN_USERS) — the two lists overlap in
// practice but SUPER_ADMINS signals "internal team member, route
// billing to Stripe sandbox + allow in-development features".
//
// Parsing mirrors parseAdminUsers exactly: comma-separated, entries
// are either Clerk user IDs (no '@') or case-insensitive emails, with
// whitespace + stray brackets/quotes tolerated. Email entries resolve
// to Clerk user IDs lazily on first hit and get cached for the
// lifetime of the process.

package auth

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"sync"

	"github.com/xiaoboyu/unipost-api/internal/db"
)

type SuperAdminChecker struct {
	queries *db.Queries

	userIDs map[string]bool
	emails  map[string]bool

	mu       sync.RWMutex
	resolved map[string]bool
}

// NewSuperAdminChecker reads SUPER_ADMINS. When both maps are empty
// every IsSuperAdmin call returns false — which is the intent: a
// missing env var should lock super-admin-gated features to nobody.
func NewSuperAdminChecker(queries *db.Queries) *SuperAdminChecker {
	userIDs, emails := parseSuperAdminList(os.Getenv("SUPER_ADMINS"))
	slog.Info("super admin checker configured",
		"super_admin_user_ids", len(userIDs),
		"super_admin_emails", len(emails))
	return &SuperAdminChecker{
		queries:  queries,
		userIDs:  userIDs,
		emails:   emails,
		resolved: make(map[string]bool),
	}
}

func parseSuperAdminList(raw string) (userIDs map[string]bool, emails map[string]bool) {
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

// IsSuperAdmin reports whether the given Clerk user ID is on
// SUPER_ADMINS. Returns false on empty input / DB error so a
// transient hiccup never accidentally grants access.
func (c *SuperAdminChecker) IsSuperAdmin(ctx context.Context, userID string) bool {
	if c == nil || userID == "" {
		return false
	}
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
		slog.Warn("super admin: user lookup failed", "user_id", userID, "error", err)
		return false
	}
	isSuperAdmin := c.emails[strings.ToLower(strings.TrimSpace(user.Email))]

	c.mu.Lock()
	c.resolved[userID] = isSuperAdmin
	c.mu.Unlock()
	if isSuperAdmin {
		slog.Info("super admin: resolved email entry", "user_id", userID, "email", user.Email)
	}
	return isSuperAdmin
}

// IsSuperAdminByUser is the no-DB variant used from the OAuth
// callback, where we've already loaded the db.User row via
// profile → workspace lookups. Avoids a redundant GetUser call.
func (c *SuperAdminChecker) IsSuperAdminByUser(userID, email string) bool {
	if c == nil {
		return false
	}
	if userID != "" && c.userIDs[userID] {
		return true
	}
	return email != "" && c.emails[strings.ToLower(strings.TrimSpace(email))]
}
