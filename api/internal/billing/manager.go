// Package billing wraps Stripe so the API can run two parallel modes
// (live and sandbox/test) keyed by which user owns the project being
// charged. We need this because superadmins should hit Stripe test mode
// for internal QA without affecting customer billing on the live account.
//
// Design sketch:
//
//	Manager
//	├── Live    *Mode  (always present once configured)
//	└── Sandbox *Mode  (optional; only present when STRIPE_SANDBOX_* env
//	                    vars are set)
//
// Each Mode owns its own *client.API, webhook secret, and price-ID map.
// Manager.For(userID) chooses which Mode to use:
//
//   - userID is in SUPER_ADMINS AND Sandbox is configured → Sandbox
//   - otherwise                                          → Live
//
// Webhook verification is symmetric: we don't know which mode the event
// came from, so VerifyWebhook tries both secrets and returns the matching
// Mode along with the parsed event.
package billing

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"

	"github.com/stripe/stripe-go/v82"
	"github.com/stripe/stripe-go/v82/client"
	"github.com/stripe/stripe-go/v82/webhook"
)

// UserLookupFunc returns the email address for a Clerk user ID. Used by
// Manager.IsSuperAdmin to resolve email entries in SUPER_ADMINS to the
// underlying user ID at request time. main.go wires this to a closure
// that calls the DB queries.GetUser helper.
type UserLookupFunc func(ctx context.Context, userID string) (email string, err error)

// Mode is one Stripe environment (live or sandbox/test) — its own API
// client, its own webhook signing secret, and its own price-ID map for
// the per-plan checkout flow.
//
// We deliberately don't store product IDs here. Stripe checkout only
// needs the price ID, and the product is resolved server-side from the
// price → product link. If a future flow ever needs an explicit product
// reference, fetch it lazily from the Stripe API rather than carrying
// dead env vars.
type Mode struct {
	Name          string // "live" or "sandbox" — used in logs only
	Client        *client.API
	WebhookSecret string
	priceIDs      map[string]string // plan ID → Stripe price ID
}

// PriceID returns the Stripe price ID for the given plan in this mode.
// Returns "" if the plan isn't configured for this mode (e.g. you set
// only the live env vars and a sandbox checkout slipped through).
func (m *Mode) PriceID(planID string) string {
	if m == nil || m.priceIDs == nil {
		return ""
	}
	return m.priceIDs[planID]
}

// Manager owns one Live mode and an optional Sandbox mode plus the
// SUPER_ADMINS allowlist that decides which one a given user gets.
//
// SUPER_ADMINS entries can be either Clerk user IDs (matched directly) or
// email addresses (resolved to a user ID via userLookup at request time
// and then cached). Mixing both formats in one comma-separated list is
// supported. Email matching is case-insensitive.
type Manager struct {
	Live    *Mode
	Sandbox *Mode

	// Direct Clerk user ID entries from SUPER_ADMINS — matched without
	// touching the DB.
	superAdminUserIDs map[string]bool

	// Lower-cased email entries from SUPER_ADMINS — resolved on demand.
	superAdminEmails map[string]bool

	// Cache populated by IsSuperAdmin after the first email-resolution
	// hit for a given userID. Stores both positive and negative results
	// so non-admins don't keep paying for repeated DB lookups.
	mu            sync.RWMutex
	resolvedCache map[string]bool

	// Optional user lookup. When nil, only direct user ID entries in
	// SUPER_ADMINS are honored — email entries silently degrade to a
	// no-op. main.go always wires this up after the DB pool is ready.
	userLookup UserLookupFunc
}

// planEnvNames maps internal plan IDs to the dollar-amount token used in
// env-var names. The full var name format is:
//
//	STRIPE_PRICE_ID_<token>          (live)
//	STRIPE_SANDBOX_PRICE_ID_<token>  (sandbox)
//
// Mirrors syncStripePriceIDs in cmd/api/main.go — keep in sync if you
// add a plan tier.
var planEnvNames = map[string]string{
	"p10":   "10",
	"p25":   "25",
	"p50":   "50",
	"p75":   "75",
	"p150":  "150",
	"p300":  "300",
	"p500":  "500",
	"p1000": "1000",
}

// NewManager reads STRIPE_*, STRIPE_SANDBOX_*, and SUPER_ADMINS from the
// environment and assembles both modes. Live is required; sandbox is
// optional. userLookup may be nil — in that case, email entries in
// SUPER_ADMINS are dropped (with a startup warning) and only direct
// Clerk user IDs are honored. Returns an error if the live key is
// missing.
func NewManager(userLookup UserLookupFunc) (*Manager, error) {
	liveKey := os.Getenv("STRIPE_SECRET_KEY")
	if liveKey == "" {
		return nil, fmt.Errorf("billing: STRIPE_SECRET_KEY is required")
	}

	live := newMode("live", liveKey, os.Getenv("STRIPE_WEBHOOK_SECRET"), readPriceIDs(""))

	// Set the global stripe.Key as a fallback so any code path that still
	// calls package-level stripe-go functions (e.g. legacy customer
	// helpers) keeps working in live mode. This is belt-and-suspenders —
	// the refactored handlers use the per-mode client.API directly.
	stripe.Key = liveKey

	userIDs, emails := parseSuperAdmins(os.Getenv("SUPER_ADMINS"))
	if len(emails) > 0 && userLookup == nil {
		slog.Warn("billing: SUPER_ADMINS contains email entries but no userLookup was provided; emails will be ignored",
			"email_count", len(emails))
	}

	mgr := &Manager{
		Live:              live,
		superAdminUserIDs: userIDs,
		superAdminEmails:  emails,
		resolvedCache:     make(map[string]bool),
		userLookup:        userLookup,
	}

	if sandboxKey := os.Getenv("STRIPE_SANDBOX_SECRET_KEY"); sandboxKey != "" {
		mgr.Sandbox = newMode("sandbox", sandboxKey, os.Getenv("STRIPE_SANDBOX_WEBHOOK_SECRET"), readPriceIDs("SANDBOX_"))
	}

	slog.Info("billing manager configured",
		"super_admin_user_ids", len(userIDs),
		"super_admin_emails", len(emails),
		"sandbox_enabled", mgr.Sandbox != nil)

	return mgr, nil
}

func newMode(name, key, webhookSecret string, priceIDs map[string]string) *Mode {
	c := &client.API{}
	c.Init(key, nil)
	return &Mode{
		Name:          name,
		Client:        c,
		WebhookSecret: webhookSecret,
		priceIDs:      priceIDs,
	}
}

// readPriceIDs walks planEnvNames and reads STRIPE_<prefix>PRICE_ID_<token>
// for each plan. The returned map only contains keys whose env var is
// non-empty. prefix is "" for live, "SANDBOX_" for sandbox.
func readPriceIDs(prefix string) map[string]string {
	out := make(map[string]string, len(planEnvNames))
	for planID, token := range planEnvNames {
		envVar := "STRIPE_" + prefix + "PRICE_ID_" + token
		if v := os.Getenv(envVar); v != "" {
			out[planID] = v
		}
	}
	return out
}

// parseSuperAdmins splits the SUPER_ADMINS env var into two sets: direct
// Clerk user IDs and email addresses. The split is by the presence of an
// "@" — emails are lowercased so the runtime comparison is
// case-insensitive. Whitespace around entries is tolerated. Stray
// brackets / quotes are stripped so callers who accidentally paste a
// JSON-array-looking value (`["alice@example.com"]`) still get parsed
// correctly.
func parseSuperAdmins(raw string) (userIDs map[string]bool, emails map[string]bool) {
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

// SuperAdminAllowlist exposes the parsed SUPER_ADMINS entries as two
// slices: direct Clerk user IDs and lower-cased emails. Used by the
// admin dashboard to exclude internal test accounts from stats and
// user listings (super admins go through Stripe sandbox so their MRR
// is fake).
//
// The returned slices are fresh copies — callers may sort or mutate
// them without affecting the manager.
func (m *Manager) SuperAdminAllowlist() (userIDs []string, emails []string) {
	if m == nil {
		return nil, nil
	}
	userIDs = make([]string, 0, len(m.superAdminUserIDs))
	for id := range m.superAdminUserIDs {
		userIDs = append(userIDs, id)
	}
	emails = make([]string, 0, len(m.superAdminEmails))
	for e := range m.superAdminEmails {
		emails = append(emails, e)
	}
	return userIDs, emails
}

// IsSuperAdmin reports whether the given Clerk user ID is in the
// SUPER_ADMINS allowlist. Direct user-ID entries match without touching
// the DB. Email entries trigger a single user lookup the first time
// IsSuperAdmin is called for a given userID; the result (positive or
// negative) is cached for the lifetime of the process so repeat checks
// are O(1).
//
// Returns false on lookup error rather than propagating, so a transient
// DB hiccup doesn't accidentally upgrade or downgrade a user. The error
// is logged.
func (m *Manager) IsSuperAdmin(ctx context.Context, userID string) bool {
	if m == nil || userID == "" {
		return false
	}

	// Direct user ID entry — fast path, no lock needed (the maps are
	// only written at construction).
	if m.superAdminUserIDs[userID] {
		return true
	}

	// No email entries OR no way to resolve them → bail.
	if len(m.superAdminEmails) == 0 || m.userLookup == nil {
		return false
	}

	// Cached result from a previous lookup?
	m.mu.RLock()
	if v, ok := m.resolvedCache[userID]; ok {
		m.mu.RUnlock()
		return v
	}
	m.mu.RUnlock()

	// Resolve via DB.
	email, err := m.userLookup(ctx, userID)
	if err != nil {
		slog.Warn("billing: super-admin email lookup failed",
			"user_id", userID, "error", err)
		return false
	}
	isAdmin := m.superAdminEmails[strings.ToLower(strings.TrimSpace(email))]

	m.mu.Lock()
	m.resolvedCache[userID] = isAdmin
	m.mu.Unlock()

	if isAdmin {
		slog.Info("billing: resolved super-admin email", "user_id", userID, "email", email)
	}
	return isAdmin
}

// For picks the Stripe mode to use for a given user. Sandbox wins when
// the user is on the SUPER_ADMINS list AND a sandbox mode was configured;
// otherwise everyone (including superadmins, when sandbox isn't set up)
// goes through live.
func (m *Manager) For(ctx context.Context, userID string) *Mode {
	if m == nil {
		return nil
	}
	if m.Sandbox != nil && m.IsSuperAdmin(ctx, userID) {
		return m.Sandbox
	}
	return m.Live
}

// VerifyWebhook validates an incoming Stripe webhook against both
// configured secrets and returns the parsed event plus the mode whose
// secret accepted it. Returns the live error if neither secret matches,
// since that's the more common case.
func (m *Manager) VerifyWebhook(body []byte, signatureHeader string) (stripe.Event, *Mode, error) {
	if m == nil || m.Live == nil {
		return stripe.Event{}, nil, fmt.Errorf("billing: manager not configured")
	}

	opts := webhook.ConstructEventOptions{IgnoreAPIVersionMismatch: true}

	// Try live first — it's the more common case in production.
	if m.Live.WebhookSecret != "" {
		if event, err := webhook.ConstructEventWithOptions(body, signatureHeader, m.Live.WebhookSecret, opts); err == nil {
			return event, m.Live, nil
		}
	}

	// Fall back to sandbox if configured.
	if m.Sandbox != nil && m.Sandbox.WebhookSecret != "" {
		if event, err := webhook.ConstructEventWithOptions(body, signatureHeader, m.Sandbox.WebhookSecret, opts); err == nil {
			return event, m.Sandbox, nil
		}
	}

	return stripe.Event{}, nil, fmt.Errorf("billing: signature did not match any configured secret")
}
