package xinbox

import (
	"fmt"
	"strings"
)

type AppMode string

const (
	AppModeUniPostManaged AppMode = "unipost_managed_app"
	AppModeWorkspace      AppMode = "workspace_x_app"
	AppModeLegacyUnknown  AppMode = "legacy_unknown"
)

const (
	DeliveryStatusPending         = "pending"
	DeliveryStatusActive          = "active"
	DeliveryStatusPausedCap       = "paused_cap"
	DeliveryStatusPausedAllowance = "paused_allowance"
	DeliveryStatusPausedPlan      = "paused_plan"
	DeliveryStatusError           = "error"
)

var (
	publishingScopes = []string{"tweet.read", "tweet.write", "users.read"}
	dmScopes         = []string{"dm.read", "dm.write"}
)

type AppCredentials struct {
	ClientIDConfigured       bool
	ClientSecretConfigured   bool
	AppBearerTokenConfigured bool
	ConsumerSecretConfigured bool
}

func (c AppCredentials) Missing() []string {
	missing := make([]string, 0, 4)
	if !c.ClientIDConfigured {
		missing = append(missing, "client_id")
	}
	if !c.ClientSecretConfigured {
		missing = append(missing, "client_secret")
	}
	if !c.AppBearerTokenConfigured {
		missing = append(missing, "app_bearer_token")
	}
	if !c.ConsumerSecretConfigured {
		missing = append(missing, "consumer_secret")
	}
	return missing
}

func (c AppCredentials) Complete() bool {
	return len(c.Missing()) == 0
}

type CapabilityInput struct {
	PlanAllowsInbox bool
	// DMsAvailable is nil for legacy callers and means enabled. A non-nil
	// false value hides the unfinished DM rollout without affecting comments.
	DMsAvailable   *bool
	AccountStatus  string
	Scopes         []string
	AppMode        AppMode
	AppCredentials AppCredentials
	DeliveryStatus string
}

type Capabilities struct {
	CommentsEnabled       bool     `json:"comments_enabled"`
	DMsEnabled            bool     `json:"dms_enabled"`
	MissingScopes         []string `json:"missing_scopes"`
	ReconnectRequired     bool     `json:"reconnect_required"`
	DeliveryStatus        string   `json:"delivery_status"`
	AppMode               AppMode  `json:"app_mode"`
	MissingAppCredentials []string `json:"missing_app_credentials"`
}

func RequiredInboxScopes() []string {
	scopes := append([]string{}, publishingScopes...)
	return append(scopes, dmScopes...)
}

func AppModeForManualConnection(platform string) (AppMode, bool) {
	if strings.EqualFold(strings.TrimSpace(platform), "twitter") {
		return AppModeWorkspace, true
	}
	return "", false
}

func ParseAppMode(raw string) (AppMode, error) {
	mode := AppMode(strings.TrimSpace(raw))
	switch mode {
	case AppModeUniPostManaged, AppModeWorkspace, AppModeLegacyUnknown:
		return mode, nil
	default:
		return "", fmt.Errorf("invalid persisted X app mode %q", raw)
	}
}

func NormalizePersistedAppMode(raw string) (AppMode, error) {
	if strings.TrimSpace(raw) == "" {
		return AppModeLegacyUnknown, nil
	}
	return ParseAppMode(raw)
}

func EvaluateCapabilities(input CapabilityInput) Capabilities {
	appMode, err := NormalizePersistedAppMode(string(input.AppMode))
	if err != nil {
		appMode = AppModeLegacyUnknown
	}
	result := Capabilities{
		MissingScopes:         []string{},
		DeliveryStatus:        input.DeliveryStatus,
		AppMode:               appMode,
		MissingAppCredentials: []string{},
	}
	if !input.PlanAllowsInbox {
		result.DeliveryStatus = DeliveryStatusPausedPlan
		return result
	}
	if result.DeliveryStatus == "" {
		result.DeliveryStatus = DeliveryStatusPending
	}
	if !strings.EqualFold(strings.TrimSpace(input.AccountStatus), "active") {
		return result
	}

	scopeSet := make(map[string]struct{}, len(input.Scopes))
	for _, scope := range input.Scopes {
		scope = strings.ToLower(strings.TrimSpace(scope))
		if scope != "" {
			scopeSet[scope] = struct{}{}
		}
	}
	hasPublishingScopes := hasAllScopes(scopeSet, publishingScopes)
	dmsAvailable := input.DMsAvailable == nil || *input.DMsAvailable
	requiredScopes := publishingScopes
	if dmsAvailable {
		requiredScopes = RequiredInboxScopes()
	}
	result.MissingScopes = missingScopes(scopeSet, requiredScopes)
	result.ReconnectRequired = len(result.MissingScopes) > 0

	appCredentialsComplete := false
	switch result.AppMode {
	case AppModeUniPostManaged:
		appCredentialsComplete = true
	case AppModeWorkspace:
		result.MissingAppCredentials = input.AppCredentials.Missing()
		appCredentialsComplete = len(result.MissingAppCredentials) == 0
	case AppModeLegacyUnknown:
		result.ReconnectRequired = true
	}
	result.CommentsEnabled = hasPublishingScopes && appCredentialsComplete
	result.DMsEnabled = dmsAvailable && result.CommentsEnabled && hasAllScopes(scopeSet, RequiredInboxScopes())
	return result
}

func hasAllScopes(have map[string]struct{}, required []string) bool {
	return len(missingScopes(have, required)) == 0
}

func missingScopes(have map[string]struct{}, required []string) []string {
	missing := make([]string, 0, len(required))
	for _, scope := range required {
		if _, ok := have[scope]; !ok {
			missing = append(missing, scope)
		}
	}
	return missing
}
