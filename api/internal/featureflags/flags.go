package featureflags

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"

	"github.com/xiaoboyu/unipost-api/internal/runtimeenv"
)

type Flag string

const (
	TikTokAnalyticsScopes         Flag = "tiktok.analytics_scopes"
	AttributionUTMSignupBindingV1 Flag = "attribution.utm_signup_binding_v1"
	LoopsIntegrationV1            Flag = "email.loops_integration_v1"
	FreePlanHardPostQuota         Flag = "billing.free_plan_hard_post_quota"
	AppReviewAutopilotV1          Flag = "app_review.autopilot_v1"
	AppReviewAIAgentV1            Flag = "app_review.ai_agent_v1"
)

type Target struct {
	UserID        string
	UserEmail     string
	WorkspaceID   string
	SessionID     string
	RemoteAddress string
	Env           string
	Properties    map[string]string
}

type Provider interface {
	Enabled(ctx context.Context, flag Flag, target Target, fallback bool) bool
	Name() string
	Close() error
}

type Definition struct {
	Flag           Flag
	EnvVar         string
	LegacyEnvVars  []string
	Description    string
	DefaultEnabled func(Target) bool
}

type Evaluation struct {
	Flag     Flag   `json:"flag"`
	Enabled  bool   `json:"enabled"`
	Provider string `json:"provider"`
	Env      string `json:"env"`
}

var definitions = map[Flag]Definition{
	TikTokAnalyticsScopes: {
		Flag:          TikTokAnalyticsScopes,
		EnvVar:        "FEATURE_TIKTOK_ANALYTICS_SCOPES",
		LegacyEnvVars: []string{"TIKTOK_ANALYTICS_SCOPES_ENABLED"},
		Description:   "Requests TikTok analytics OAuth scopes: user.info.profile, user.info.stats, and video.list.",
		DefaultEnabled: func(target Target) bool {
			return !isProduction(target.Env)
		},
	},
	AttributionUTMSignupBindingV1: {
		Flag:        AttributionUTMSignupBindingV1,
		EnvVar:      "FEATURE_ATTRIBUTION_UTM_SIGNUP_BINDING_V1",
		Description: "Captures lightweight UTM attribution and binds landing sessions to signed-in users for Admin conversion reporting.",
		DefaultEnabled: func(target Target) bool {
			return !isProduction(target.Env)
		},
	},
	LoopsIntegrationV1: {
		Flag:        LoopsIntegrationV1,
		EnvVar:      "FEATURE_EMAIL_LOOPS_INTEGRATION_V1",
		Description: "Controls Loops contact sync, welcome lifecycle events, and transactional lifecycle notifications for UniPost dashboard users.",
		DefaultEnabled: func(target Target) bool {
			return !isProduction(target.Env)
		},
	},
	FreePlanHardPostQuota: {
		Flag:        FreePlanHardPostQuota,
		EnvVar:      "FEATURE_BILLING_FREE_PLAN_HARD_POST_QUOTA",
		Description: "Hard-blocks Free plan publish acceptance once the workspace would exceed its monthly post quota. Paid plans keep soft overage behavior.",
		DefaultEnabled: func(target Target) bool {
			return !isProduction(target.Env)
		},
	},
	AppReviewAutopilotV1: {
		Flag:        AppReviewAutopilotV1,
		EnvVar:      "FEATURE_APP_REVIEW_AUTOPILOT_V1",
		Description: "Controls the App Review Autopilot beta: review readiness, review kit/job APIs, signed review sessions, customer-domain review app surfaces, and review-agent orchestration.",
		DefaultEnabled: func(target Target) bool {
			return !isProduction(target.Env)
		},
	},
	AppReviewAIAgentV1: {
		Flag:        AppReviewAIAgentV1,
		EnvVar:      "FEATURE_APP_REVIEW_AI_AGENT_V1",
		Description: "Controls the AI-guided App Review Autopilot executor, server-side Anthropic orchestration, and evidence-gated browser actions.",
		DefaultEnabled: func(target Target) bool {
			return !isProduction(target.Env)
		},
	},
}

var (
	currentMu sync.RWMutex
	current   Provider = EnvProvider{}
)

func SetProvider(provider Provider) {
	if provider == nil {
		provider = EnvProvider{}
	}
	currentMu.Lock()
	current = provider
	currentMu.Unlock()
}

func ProviderName() string {
	currentMu.RLock()
	defer currentMu.RUnlock()
	return current.Name()
}

func Close() error {
	currentMu.RLock()
	provider := current
	currentMu.RUnlock()
	return provider.Close()
}

func Enabled(ctx context.Context, flag Flag, target Target) bool {
	return Evaluate(ctx, flag, target).Enabled
}

func Evaluate(ctx context.Context, flag Flag, target Target) Evaluation {
	target = normalizeTarget(target)
	def, ok := definitions[flag]
	if !ok {
		slog.Warn("feature flag evaluated without definition", "flag", flag)
		def = Definition{Flag: flag}
	}

	fallback := false
	if def.DefaultEnabled != nil {
		fallback = def.DefaultEnabled(target)
	}

	currentMu.RLock()
	provider := current
	currentMu.RUnlock()

	enabled := provider.Enabled(ctx, flag, target, fallback)
	return Evaluation{
		Flag:     flag,
		Enabled:  enabled,
		Provider: provider.Name(),
		Env:      target.Env,
	}
}

func DefinitionFor(flag Flag) (Definition, bool) {
	def, ok := definitions[flag]
	return def, ok
}

func Definitions() []Definition {
	items := make([]Definition, 0, len(definitions))
	for _, def := range definitions {
		items = append(items, def)
	}
	return items
}

func normalizeTarget(target Target) Target {
	if strings.TrimSpace(target.Env) == "" {
		target.Env = runtimeenv.Current()
	}
	return target
}

func isProduction(env string) bool {
	switch strings.ToLower(strings.TrimSpace(env)) {
	case "production", "prod", "live":
		return true
	default:
		return false
	}
}

type EnvProvider struct{}

func (EnvProvider) Name() string { return "env" }

func (EnvProvider) Close() error { return nil }

func (EnvProvider) Enabled(_ context.Context, flag Flag, _ Target, fallback bool) bool {
	def, ok := definitions[flag]
	if !ok {
		return fallback
	}
	for _, name := range append([]string{def.EnvVar}, def.LegacyEnvVars...) {
		if strings.TrimSpace(name) == "" {
			continue
		}
		if _, ok := os.LookupEnv(name); ok {
			return runtimeenv.TruthyEnv(name)
		}
	}
	return fallback
}

func NewProviderFromEnv() (Provider, error) {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("FEATURE_FLAGS_PROVIDER"))) {
	case "", "env":
		return EnvProvider{}, nil
	case "unleash":
		return NewUnleashProviderFromEnv()
	default:
		return nil, fmt.Errorf("unsupported FEATURE_FLAGS_PROVIDER %q", os.Getenv("FEATURE_FLAGS_PROVIDER"))
	}
}
