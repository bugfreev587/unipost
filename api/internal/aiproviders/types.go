package aiproviders

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strings"
)

type Provider string

const (
	ProviderTokenGate Provider = "tokengate"
	ProviderOpenAI    Provider = "openai"
	ProviderAnthropic Provider = "anthropic"
)

type Surface string

const (
	SurfacePostAssist  Surface = "post_assist"
	SurfaceErrorTriage Surface = "error_triage"
	SurfaceAppReviewAI Surface = "app_review_ai"
)

type ClientKind string

const (
	ClientKindChatCompletions ClientKind = "chat_completions"
	ClientKindMessages        ClientKind = "messages"
)

type Source string

const (
	SourceAdmin Source = "admin"
	SourceEnv   Source = "env"
	SourceNone  Source = "none"
)

type ValidationStatus string

const (
	ValidationOK             ValidationStatus = "ok"
	ValidationAuthFailed     ValidationStatus = "auth_failed"
	ValidationModelFailed    ValidationStatus = "model_failed"
	ValidationRateLimited    ValidationStatus = "rate_limited"
	ValidationProviderFailed ValidationStatus = "provider_failed"
	ValidationConfigFailed   ValidationStatus = "config_failed"
)

const (
	EventCreated   = "AI_PROVIDER_KEY.CREATED"
	EventRotated   = "AI_PROVIDER_KEY.ROTATED"
	EventUpdated   = "AI_PROVIDER_KEY.UPDATED"
	EventTested    = "AI_PROVIDER_KEY.TESTED"
	EventActivated = "AI_PROVIDER_KEY.ACTIVATED"
	EventDisabled  = "AI_PROVIDER_KEY.DISABLED"
)

const (
	DefaultTokenGateBaseURL = "https://gateway.mytokengate.com/v1"
	DefaultOpenAIBaseURL    = "https://api.openai.com/v1"
	DefaultAnthropicBaseURL = "https://api.anthropic.com/v1"
	DefaultOpenAIModel      = "gpt-4.1-mini"
	DefaultAnthropicModel   = "claude-sonnet-4-20250514"
)

var (
	ErrProviderKeyRequired   = errors.New("AI_PROVIDER_KEY_REQUIRED")
	ErrProviderUnsupported   = errors.New("AI_PROVIDER_UNSUPPORTED")
	ErrSurfaceUnsupported    = errors.New("AI_SURFACE_UNSUPPORTED")
	ErrClientKindUnsupported = errors.New("AI_CLIENT_KIND_UNSUPPORTED")
	ErrSurfaceIncompatible   = errors.New("AI_SURFACE_CLIENT_KIND_INCOMPATIBLE")
	ErrProviderNotConfigured = errors.New("AI_PROVIDER_NOT_CONFIGURED")
	ErrProviderDisabled      = errors.New("AI_PROVIDER_DISABLED")
)

type EnvLookup func(string) string

func osEnv(key string) string {
	return os.Getenv(key)
}

func mapEnv(values map[string]string) EnvLookup {
	return func(key string) string {
		return values[key]
	}
}

type Cipher interface {
	Encrypt(string) (string, error)
	Decrypt(string) (string, error)
}

type HTTPDoer interface {
	Do(*http.Request) (*http.Response, error)
}

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type EffectiveConfig struct {
	Provider   Provider   `json:"provider"`
	Source     Source     `json:"source"`
	ClientKind ClientKind `json:"client_kind"`
	APIKey     string     `json:"-"`
	BaseURL    string     `json:"base_url"`
	Model      string     `json:"model"`
	Surface    Surface    `json:"surface"`
}

func (c EffectiveConfig) ChatCompletionsURL() string {
	base := normalizeBaseURL(c.BaseURL)
	if strings.HasSuffix(base, "/chat/completions") {
		return base
	}
	return base + "/chat/completions"
}

func (c EffectiveConfig) MessagesURL() string {
	base := normalizeBaseURL(c.BaseURL)
	if strings.HasSuffix(base, "/messages") {
		return base
	}
	return base + "/messages"
}

func (c EffectiveConfig) ModelName() string {
	if c.Provider == "" || c.Model == "" {
		return ""
	}
	return string(c.Provider) + ":" + c.Model
}

type ProviderStatus struct {
	Provider             Provider `json:"provider"`
	Configured           bool     `json:"configured"`
	Enabled              bool     `json:"enabled"`
	Source               Source   `json:"source"`
	KeyTail              string   `json:"key_tail"`
	BaseURL              string   `json:"base_url"`
	ChatModel            string   `json:"chat_model"`
	MessagesModel        string   `json:"messages_model"`
	LastValidatedAt      string   `json:"last_validated_at,omitempty"`
	LastValidationStatus string   `json:"last_validation_status,omitempty"`
	LastValidationError  string   `json:"last_validation_error,omitempty"`
	LastRotatedAt        string   `json:"last_rotated_at,omitempty"`
	UpdatedAt            string   `json:"updated_at,omitempty"`
}

type RouteStatus struct {
	Surface       Surface    `json:"surface"`
	Provider      Provider   `json:"provider"`
	Source        Source     `json:"source"`
	ClientKind    ClientKind `json:"client_kind"`
	Model         string     `json:"model"`
	ModelOverride string     `json:"model_override,omitempty"`
}

type StatusResponse struct {
	Providers []ProviderStatus        `json:"providers"`
	Effective map[Surface]RouteStatus `json:"effective"`
	Routes    map[Surface]RouteStatus `json:"routes"`
}

type ProviderEvent struct {
	ID           int64           `json:"id"`
	Provider     Provider        `json:"provider,omitempty"`
	Surface      Surface         `json:"surface,omitempty"`
	Action       string          `json:"action"`
	Category     string          `json:"category"`
	ActorAdminID string          `json:"actor_admin_id,omitempty"`
	Metadata     json.RawMessage `json:"metadata,omitempty"`
	CreatedAt    string          `json:"created_at,omitempty"`
}
