package xaccountreads

import (
	"strings"

	"github.com/xiaoboyu/unipost-api/internal/xcredits"
	"github.com/xiaoboyu/unipost-api/internal/xinbox"
)

type CapabilityCredits struct {
	AccountingEnabled           bool   `json:"accounting_enabled"`
	BillingMode                 string `json:"billing_mode"`
	BypassReason                string `json:"bypass_reason,omitempty"`
	Operation                   string `json:"operation"`
	CatalogCreditsPerResource   int64  `json:"catalog_credits_per_resource"`
	EffectiveCreditsPerResource int64  `json:"effective_credits_per_resource"`
}

type ReadCapability struct {
	Supported         bool              `json:"supported"`
	Authorized        bool              `json:"authorized"`
	ReconnectRequired bool              `json:"reconnect_required"`
	MinPageSize       int               `json:"min_page_size,omitempty"`
	MaxPageSize       int               `json:"max_page_size,omitempty"`
	Credits           CapabilityCredits `json:"credits"`
}

type Capabilities struct {
	ProfileRead        ReadCapability `json:"profile_read"`
	OwnPostHistoryRead ReadCapability `json:"own_post_history_read"`
}

type CapabilityInput struct {
	AccountStatus           string
	Disconnected            bool
	Scopes                  []string
	RefreshAvailable        bool
	AppCredentialsAvailable bool
	AppMode                 xinbox.AppMode
	BillingEnabled          bool
}

func EvaluateCapabilities(input CapabilityInput) Capabilities {
	profileAuthorized := accountReadBaseAuthorized(input) && hasScopes(input.Scopes, "users.read", "offline.access")
	postsAuthorized := accountReadBaseAuthorized(input) && hasScopes(input.Scopes, "users.read", "tweet.read", "offline.access")
	return Capabilities{
		ProfileRead: ReadCapability{
			Supported: true, Authorized: profileAuthorized, ReconnectRequired: !profileAuthorized,
			Credits: capabilityCredits(input, "user.read"),
		},
		OwnPostHistoryRead: ReadCapability{
			Supported: true, Authorized: postsAuthorized, ReconnectRequired: !postsAuthorized,
			MinPageSize: 5, MaxPageSize: 100, Credits: capabilityCredits(input, "post.read"),
		},
	}
}

func accountReadBaseAuthorized(input CapabilityInput) bool {
	if input.Disconnected || !strings.EqualFold(input.AccountStatus, "active") ||
		!input.RefreshAvailable || input.AppMode == xinbox.AppModeLegacyUnknown {
		return false
	}
	return input.AppMode != xinbox.AppModeWorkspace || input.AppCredentialsAvailable
}

func hasScopes(granted []string, required ...string) bool {
	set := make(map[string]struct{}, len(granted))
	for _, scope := range granted {
		set[strings.ToLower(strings.TrimSpace(scope))] = struct{}{}
	}
	for _, scope := range required {
		if _, ok := set[scope]; !ok {
			return false
		}
	}
	return true
}

func capabilityCredits(input CapabilityInput, operation string) CapabilityCredits {
	weight := xcredits.OperationWeight(operation)
	policy := CapabilityCredits{
		BillingMode: string(input.AppMode), Operation: operation,
		CatalogCreditsPerResource: weight,
	}
	if input.AppMode == xinbox.AppModeWorkspace {
		policy.BypassReason = xcredits.ExposureBypassCustomerXApp
		return policy
	}
	if !input.BillingEnabled {
		policy.BypassReason = xcredits.ExposureBypassFeatureDisabled
		return policy
	}
	policy.AccountingEnabled = true
	policy.EffectiveCreditsPerResource = weight
	return policy
}
