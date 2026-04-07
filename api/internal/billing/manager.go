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
	"fmt"
	"os"
	"strings"

	"github.com/stripe/stripe-go/v82"
	"github.com/stripe/stripe-go/v82/client"
	"github.com/stripe/stripe-go/v82/webhook"
)

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
type Manager struct {
	Live        *Mode
	Sandbox     *Mode
	superAdmins map[string]bool
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
// optional. Returns an error if the live key is missing — without it the
// API can't create checkouts at all.
func NewManager() (*Manager, error) {
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

	mgr := &Manager{
		Live:        live,
		superAdmins: parseSuperAdmins(os.Getenv("SUPER_ADMINS")),
	}

	if sandboxKey := os.Getenv("STRIPE_SANDBOX_SECRET_KEY"); sandboxKey != "" {
		mgr.Sandbox = newMode("sandbox", sandboxKey, os.Getenv("STRIPE_SANDBOX_WEBHOOK_SECRET"), readPriceIDs("SANDBOX_"))
	}

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

func parseSuperAdmins(raw string) map[string]bool {
	out := make(map[string]bool)
	for _, s := range strings.Split(raw, ",") {
		s = strings.TrimSpace(s)
		if s != "" {
			out[s] = true
		}
	}
	return out
}

// IsSuperAdmin reports whether the given Clerk user ID is in the
// SUPER_ADMINS list. Used by the dashboard to gate sandbox-only UI.
func (m *Manager) IsSuperAdmin(userID string) bool {
	if m == nil {
		return false
	}
	return m.superAdmins[userID]
}

// For picks the Stripe mode to use for a given user. Sandbox wins when
// the user is on the SUPER_ADMINS list AND a sandbox mode was configured;
// otherwise everyone (including superadmins, when sandbox isn't set up)
// goes through live.
func (m *Manager) For(userID string) *Mode {
	if m == nil {
		return nil
	}
	if m.Sandbox != nil && m.IsSuperAdmin(userID) {
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
