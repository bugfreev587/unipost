// Package connect implements UniPost Connect — the multi-tenant
// hosted OAuth flow that lets customers onboard *their* end users
// into managed social_accounts rows.
//
// Each platform has one Connector implementation:
//
//	twitter.go  — OAuth 2.0 PKCE
//	linkedin.go — OAuth 2.0 (no PKCE)
//
// Bluesky doesn't go through this package — it's an HTML form
// handled in internal/handler/connect_bluesky.go because it has no
// OAuth dance. Connectors here are only for the redirect-based flows.
//
// The handler layer (internal/handler/connect_callback.go) drives
// each Connector through three phases:
//
//	1. Authorize  — given a connect_session row, return the URL we
//	                redirect the end user to on the platform's auth
//	                server. PKCE challenge derivation lives here.
//
//	2. Exchange   — given an authorization code (from the callback
//	                redirect), POST to the platform's token endpoint
//	                and return the resulting tokens + profile metadata.
//
//	3. Refresh    — given a refresh token, mint a new access token.
//	                Used by the PR7 token refresh worker.

package connect

import (
	"context"
	"time"
)

// Profile is the minimal account identity we store after a
// successful connect. ExternalAccountID is the platform's stable
// user id (e.g. Twitter's numeric user id, LinkedIn's "sub" claim).
// Username is the human-readable handle / display name shown to the
// customer in their dashboard.
type Profile struct {
	ExternalAccountID string
	Username          string
	DisplayName       string
	AvatarURL         string
}

// TokenSet is what we get back from a token-endpoint round trip.
// RefreshToken can be empty when the platform doesn't issue one
// (e.g. LinkedIn doesn't rotate refresh tokens; the same value
// stays valid for the row's full lifetime).
type TokenSet struct {
	AccessToken  string
	RefreshToken string
	ExpiresAt    time.Time
	Scopes       []string
}

// Connector is the interface every OAuth Connect platform must
// implement. The handler dispatcher does NOT know which platform
// it's calling — it just routes by name into a registry.
type Connector interface {
	Platform() string

	// AuthorizeURL builds the URL to redirect the end user to. The
	// session carries oauth_state (the CSRF token / public lookup
	// bearer) and pkce_verifier (Twitter only). Implementations
	// derive the PKCE challenge from the verifier inline.
	AuthorizeURL(session SessionView) (string, error)

	// ExchangeCode trades the authorization_code we received on
	// the callback for an access + refresh token. The session is
	// passed in so PKCE-using implementations can read the verifier.
	ExchangeCode(ctx context.Context, session SessionView, code string) (*TokenSet, error)

	// FetchProfile reads the platform's "current user" endpoint to
	// get the stable account id + display name. Called immediately
	// after ExchangeCode to populate the social_accounts row.
	FetchProfile(ctx context.Context, accessToken string) (*Profile, error)

	// Refresh exchanges a refresh token for a new access token.
	// Used by the PR7 token refresh worker. Some platforms rotate
	// refresh tokens (Twitter); the returned TokenSet's RefreshToken
	// will be empty when the existing one should be kept.
	Refresh(ctx context.Context, refreshToken string) (*TokenSet, error)
}

// SessionView is the slice of connect_sessions a Connector needs.
// Pulling this into a struct rather than passing the raw db row
// keeps the connect/ package free of any sqlc generated types
// (which would be a circular import waiting to happen, since
// internal/handler imports internal/connect).
type SessionView struct {
	ID            string
	OAuthState    string
	PKCEVerifier  string // empty for non-PKCE platforms
	RedirectURI   string // the callback URL we registered with the platform
}

// Registry holds all available connectors keyed by Platform().
// Populated at process startup from cmd/api/main.go.
type Registry struct {
	connectors map[string]Connector
}

func NewRegistry(connectors ...Connector) *Registry {
	m := make(map[string]Connector, len(connectors))
	for _, c := range connectors {
		m[c.Platform()] = c
	}
	return &Registry{connectors: m}
}

// Get returns the connector for a platform. Second return is false
// when the platform isn't registered (Bluesky, or any unknown).
func (r *Registry) Get(platform string) (Connector, bool) {
	c, ok := r.connectors[platform]
	return c, ok
}
