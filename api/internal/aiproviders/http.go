package aiproviders

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/db"
)

type chatCompletionsRequest struct {
	Model          string        `json:"model"`
	Messages       []ChatMessage `json:"messages"`
	ResponseFormat any           `json:"response_format,omitempty"`
	Temperature    float64       `json:"temperature,omitempty"`
}

type chatCompletionsResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

type providerModelsResponse struct {
	Data   []providerModel `json:"data"`
	Models []providerModel `json:"models"`
}

type providerModel struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Model string `json:"model"`
}

func (s *Service) ChatCompletionsJSON(ctx context.Context, surface Surface, messages []ChatMessage, responseFormat any, temperature float64, out any) (EffectiveConfig, error) {
	cfg, err := s.Resolve(ctx, surface)
	if err != nil {
		return EffectiveConfig{}, err
	}
	if cfg.ClientKind != ClientKindChatCompletions {
		return EffectiveConfig{}, ErrSurfaceIncompatible
	}
	reqBody := chatCompletionsRequest{
		Model:          cfg.Model,
		Messages:       messages,
		ResponseFormat: responseFormat,
		Temperature:    temperature,
	}
	raw, err := json.Marshal(reqBody)
	if err != nil {
		return cfg, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.ChatCompletionsURL(), bytes.NewReader(raw))
	if err != nil {
		return cfg, err
	}
	ApplyChatCompletionsHeaders(req, cfg)

	res, err := s.client().Do(req)
	if err != nil {
		return cfg, err
	}
	defer res.Body.Close()
	payload, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return cfg, err
	}
	if res.StatusCode >= 300 {
		var apiErr chatCompletionsResponse
		if json.Unmarshal(payload, &apiErr) == nil && apiErr.Error != nil && apiErr.Error.Message != "" {
			return cfg, errors.New(RedactProviderError(apiErr.Error.Message))
		}
		return cfg, fmt.Errorf("AI provider returned HTTP %d", res.StatusCode)
	}
	var parsed chatCompletionsResponse
	if err := json.Unmarshal(payload, &parsed); err != nil {
		return cfg, err
	}
	if parsed.Error != nil && parsed.Error.Message != "" {
		return cfg, errors.New(RedactProviderError(parsed.Error.Message))
	}
	if len(parsed.Choices) == 0 {
		return cfg, fmt.Errorf("AI provider returned no choices")
	}
	return cfg, json.Unmarshal([]byte(parsed.Choices[0].Message.Content), out)
}

func (s *Service) TestProvider(ctx context.Context, input TestProviderInput) (ValidationResult, error) {
	provider, err := normalizeProvider(input.Provider)
	if err != nil {
		return ValidationResult{Status: ValidationConfigFailed, Message: "Unsupported provider"}, err
	}
	inputBaseURL := normalizeBaseURL(input.BaseURL)
	baseURL := firstNonEmpty(inputBaseURL, defaultBaseURL(provider))
	apiKey := strings.TrimSpace(input.APIKey)
	chatModel := strings.TrimSpace(input.ChatModel)
	messagesModel := strings.TrimSpace(input.MessagesModel)
	persistResult := false
	if apiKey == "" {
		row, err := s.store.GetAdminAIProviderKey(ctx, string(provider))
		if errors.Is(err, pgx.ErrNoRows) {
			return ValidationResult{Status: ValidationConfigFailed, Message: "API key is required"}, ErrProviderKeyRequired
		}
		if err != nil {
			return ValidationResult{Status: ValidationProviderFailed, Message: "Failed to load provider config"}, err
		}
		apiKey, err = s.cipher.Decrypt(row.ApiKeyCiphertext)
		if err != nil {
			return ValidationResult{Status: ValidationProviderFailed, Message: "Failed to decrypt provider key"}, err
		}
		if inputBaseURL == "" {
			baseURL = firstNonEmpty(normalizeBaseURL(row.BaseUrl), defaultBaseURL(provider))
		}
		if chatModel == "" {
			chatModel = strings.TrimSpace(row.ChatModel)
		}
		if messagesModel == "" {
			messagesModel = strings.TrimSpace(row.MessagesModel)
		}
		persistResult = true
	}
	if strings.TrimSpace(apiKey) == "" {
		return ValidationResult{Status: ValidationConfigFailed, Message: "API key is required"}, ErrProviderKeyRequired
	}

	result := s.validateModels(ctx, EffectiveConfig{
		Provider: provider,
		APIKey:   apiKey,
		BaseURL:  baseURL,
	}, modelsForProviderValidation(provider, chatModel, messagesModel)...)
	if persistResult {
		_, _ = s.store.UpdateAdminAIProviderValidation(ctx, db.UpdateAdminAIProviderValidationParams{
			Provider:             string(provider),
			LastValidationStatus: pgtype.Text{String: string(result.Status), Valid: true},
			LastValidationError:  pgtype.Text{String: result.Message, Valid: result.Status != ValidationOK},
			UpdatedByAdminID:     textOrNull(input.ActorAdminID),
		})
	}
	_ = s.writeEvent(ctx, EventTested, provider, "", input.ActorAdminID, map[string]any{
		"provider":          provider,
		"base_url":          baseURL,
		"validation_status": result.Status,
		"validation_error":  result.Message,
	})
	if result.Status != ValidationOK {
		return result, nil
	}
	return result, nil
}

func (s *Service) validateModels(ctx context.Context, cfg EffectiveConfig, expectedModels ...string) ValidationResult {
	url := normalizeBaseURL(cfg.BaseURL) + "/models"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return ValidationResult{Status: ValidationConfigFailed, Message: "Invalid provider base URL"}
	}
	req.Header.Set("Content-Type", "application/json")
	if cfg.Provider == ProviderAnthropic {
		ApplyMessagesHeaders(req, cfg)
	} else {
		req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	}
	res, err := s.client().Do(req)
	if err != nil {
		return ValidationResult{Status: ValidationProviderFailed, Message: RedactProviderError(err.Error())}
	}
	defer res.Body.Close()
	payload, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if res.StatusCode >= 200 && res.StatusCode <= 299 {
		if missing := missingConfiguredModels(payload, expectedModels); len(missing) > 0 {
			return ValidationResult{Status: ValidationModelFailed, Message: "Configured model unavailable: " + strings.Join(missing, ", ")}
		}
		return ValidationResult{Status: ValidationOK, Message: "Provider reachable"}
	}
	status := ValidationStatusFromHTTPStatus(res.StatusCode)
	return ValidationResult{Status: status, Message: validationMessage(status, res.StatusCode)}
}

func modelsForProviderValidation(provider Provider, chatModel, messagesModel string) []string {
	var models []string
	switch provider {
	case ProviderOpenAI:
		models = appendModel(models, chatModel)
	case ProviderAnthropic:
		models = appendModel(models, messagesModel)
	case ProviderTokenGate:
		models = appendModel(models, chatModel)
		models = appendModel(models, messagesModel)
	}
	return models
}

func appendModel(models []string, model string) []string {
	model = strings.TrimSpace(model)
	if model == "" {
		return models
	}
	for _, existing := range models {
		if existing == model {
			return models
		}
	}
	return append(models, model)
}

func missingConfiguredModels(payload []byte, expectedModels []string) []string {
	expected := make([]string, 0, len(expectedModels))
	for _, model := range expectedModels {
		model = strings.TrimSpace(model)
		if model != "" {
			expected = append(expected, model)
		}
	}
	if len(expected) == 0 {
		return nil
	}

	available := map[string]bool{}
	var parsed providerModelsResponse
	if err := json.Unmarshal(payload, &parsed); err == nil {
		for _, model := range parsed.Data {
			addProviderModelIDs(available, model)
		}
		for _, model := range parsed.Models {
			addProviderModelIDs(available, model)
		}
	}
	if len(available) == 0 {
		return expected
	}

	var missing []string
	for _, model := range expected {
		if !available[model] {
			missing = append(missing, model)
		}
	}
	return missing
}

func addProviderModelIDs(ids map[string]bool, model providerModel) {
	for _, value := range []string{model.ID, model.Name, model.Model} {
		value = strings.TrimSpace(value)
		if value != "" {
			ids[value] = true
		}
	}
}

func ValidationStatusFromHTTPStatus(status int) ValidationStatus {
	switch status {
	case http.StatusBadRequest:
		return ValidationConfigFailed
	case http.StatusUnauthorized, http.StatusForbidden:
		return ValidationAuthFailed
	case http.StatusTooManyRequests:
		return ValidationRateLimited
	case http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return ValidationProviderFailed
	default:
		if status >= 500 {
			return ValidationProviderFailed
		}
		return ValidationModelFailed
	}
}

func validationMessage(status ValidationStatus, httpStatus int) string {
	switch status {
	case ValidationAuthFailed:
		return "Provider rejected the API key"
	case ValidationRateLimited:
		return "Provider rate limit exceeded"
	case ValidationProviderFailed:
		return fmt.Sprintf("Provider returned HTTP %d", httpStatus)
	case ValidationConfigFailed:
		return "Provider configuration is invalid"
	case ValidationModelFailed:
		return fmt.Sprintf("Provider returned HTTP %d", httpStatus)
	default:
		return "Provider reachable"
	}
}

func ApplyChatCompletionsHeaders(req *http.Request, cfg EffectiveConfig) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
}

func ApplyMessagesHeaders(req *http.Request, cfg EffectiveConfig) {
	req.Header.Set("Content-Type", "application/json")
	if cfg.Provider == ProviderAnthropic {
		req.Header.Set("x-api-key", cfg.APIKey)
		req.Header.Set("anthropic-version", "2023-06-01")
		return
	}
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
}

func RedactProviderError(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	redacted := redactBearerTokens(trimmed)
	redacted = redactSecretPrefixes(redacted)
	if len(redacted) > 500 {
		redacted = redacted[:500]
	}
	return redacted
}

func redactBearerTokens(value string) string {
	fields := strings.Fields(value)
	for i, field := range fields {
		if strings.EqualFold(field, "Bearer") && i+1 < len(fields) {
			fields[i+1] = "[redacted]"
		}
		if strings.HasPrefix(strings.ToLower(field), "authorization:") {
			fields[i] = "Authorization:[redacted]"
		}
	}
	return strings.Join(fields, " ")
}

func redactSecretPrefixes(value string) string {
	parts := strings.Fields(value)
	for i, part := range parts {
		lower := strings.ToLower(part)
		if strings.HasPrefix(lower, "sk-") || strings.HasPrefix(lower, "tg-") || strings.HasPrefix(lower, "tokengate-") {
			parts[i] = "[redacted]"
		}
	}
	return strings.Join(parts, " ")
}

func (s *Service) client() HTTPDoer {
	if s != nil && s.httpClient != nil {
		return s.httpClient
	}
	return &http.Client{Timeout: 30 * time.Second}
}
