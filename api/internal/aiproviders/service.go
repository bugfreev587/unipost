package aiproviders

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/db"
)

type Store interface {
	GetAdminAIProviderKey(context.Context, string) (db.AdminAiProviderKey, error)
	ListAdminAIProviderKeys(context.Context) ([]db.AdminAiProviderKey, error)
	UpsertAdminAIProviderKey(context.Context, db.UpsertAdminAIProviderKeyParams) (db.AdminAiProviderKey, error)
	UpdateAdminAIProviderConfig(context.Context, db.UpdateAdminAIProviderConfigParams) (db.AdminAiProviderKey, error)
	UpdateAdminAIProviderValidation(context.Context, db.UpdateAdminAIProviderValidationParams) (db.AdminAiProviderKey, error)
	DisableAdminAIProviderKey(context.Context, db.DisableAdminAIProviderKeyParams) (db.AdminAiProviderKey, error)
	DeleteAISurfaceRoutesForProvider(context.Context, string) error
	GetAISurfaceRoute(context.Context, string) (db.AiSurfaceRouting, error)
	ListAISurfaceRoutes(context.Context) ([]db.AiSurfaceRouting, error)
	UpsertAISurfaceRoute(context.Context, db.UpsertAISurfaceRouteParams) (db.AiSurfaceRouting, error)
	DeleteAISurfaceRoute(context.Context, string) error
	CreateAdminAIProviderEvent(context.Context, db.CreateAdminAIProviderEventParams) (db.AdminAiProviderEvent, error)
	ListAdminAIProviderEvents(context.Context, db.ListAdminAIProviderEventsParams) ([]db.AdminAiProviderEvent, error)
}

type Service struct {
	store      Store
	cipher     Cipher
	env        EnvLookup
	httpClient HTTPDoer
}

type SaveProviderInput struct {
	Provider      Provider `json:"provider"`
	APIKey        string   `json:"api_key"`
	BaseURL       string   `json:"base_url"`
	ChatModel     string   `json:"chat_model"`
	MessagesModel string   `json:"messages_model"`
	Enabled       bool     `json:"enabled"`
	ActorAdminID  string   `json:"-"`
}

type RouteSurfaceInput struct {
	Surface       Surface    `json:"surface"`
	Provider      Provider   `json:"provider"`
	ClientKind    ClientKind `json:"client_kind"`
	ModelOverride string     `json:"model_override"`
	ActorAdminID  string     `json:"-"`
}

type TestProviderInput struct {
	Provider      Provider `json:"provider"`
	APIKey        string   `json:"api_key"`
	BaseURL       string   `json:"base_url"`
	ChatModel     string   `json:"chat_model"`
	MessagesModel string   `json:"messages_model"`
	ActorAdminID  string   `json:"-"`
}

type ValidationResult struct {
	Status  ValidationStatus `json:"status"`
	Message string           `json:"message"`
}

func NewService(store Store, cipher Cipher) *Service {
	return &Service{store: store, cipher: cipher, env: osEnv}
}

func (s *Service) WithEnv(env EnvLookup) *Service {
	if env != nil {
		s.env = env
	}
	return s
}

func (s *Service) WithHTTPClient(client HTTPDoer) *Service {
	if client != nil {
		s.httpClient = client
	}
	return s
}

func (s *Service) SaveProvider(ctx context.Context, input SaveProviderInput) (ProviderStatus, error) {
	if s == nil || s.store == nil || s.cipher == nil {
		return ProviderStatus{}, fmt.Errorf("AI provider service is not configured")
	}
	provider, err := normalizeProvider(input.Provider)
	if err != nil {
		return ProviderStatus{}, err
	}
	baseURL := firstNonEmpty(normalizeBaseURL(input.BaseURL), defaultBaseURL(provider))
	chatModel := strings.TrimSpace(input.ChatModel)
	messagesModel := strings.TrimSpace(input.MessagesModel)
	actor := textOrNull(input.ActorAdminID)

	_, err = s.store.GetAdminAIProviderKey(ctx, string(provider))
	exists := err == nil
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return ProviderStatus{}, err
	}

	apiKey := strings.TrimSpace(input.APIKey)
	if apiKey == "" && !exists {
		return ProviderStatus{}, ErrProviderKeyRequired
	}

	var row db.AdminAiProviderKey
	action := EventUpdated
	if apiKey != "" {
		ciphertext, err := s.cipher.Encrypt(apiKey)
		if err != nil {
			return ProviderStatus{}, err
		}
		if exists {
			action = EventRotated
		} else {
			action = EventCreated
		}
		row, err = s.store.UpsertAdminAIProviderKey(ctx, db.UpsertAdminAIProviderKeyParams{
			Provider:         string(provider),
			Enabled:          input.Enabled,
			ApiKeyCiphertext: ciphertext,
			KeyTail:          keyTail(apiKey),
			BaseUrl:          baseURL,
			ChatModel:        chatModel,
			MessagesModel:    messagesModel,
			LastRotatedAt:    timestamp(time.Now()),
			CreatedByAdminID: actor,
			UpdatedByAdminID: actor,
		})
		if err != nil {
			return ProviderStatus{}, err
		}
	} else {
		row, err = s.store.UpdateAdminAIProviderConfig(ctx, db.UpdateAdminAIProviderConfigParams{
			Provider:         string(provider),
			Enabled:          input.Enabled,
			BaseUrl:          baseURL,
			ChatModel:        chatModel,
			MessagesModel:    messagesModel,
			UpdatedByAdminID: actor,
		})
		if err != nil {
			return ProviderStatus{}, err
		}
	}
	_ = s.writeEvent(ctx, action, provider, "", input.ActorAdminID, map[string]any{
		"provider":       provider,
		"base_url":       baseURL,
		"chat_model":     chatModel,
		"messages_model": messagesModel,
		"key_tail":       row.KeyTail,
		"enabled":        row.Enabled,
	})
	return providerStatusFromRow(row), nil
}

func (s *Service) RouteSurface(ctx context.Context, input RouteSurfaceInput) (RouteStatus, error) {
	surface, err := normalizeSurface(input.Surface)
	if err != nil {
		return RouteStatus{}, err
	}
	provider, err := normalizeProvider(input.Provider)
	if err != nil {
		return RouteStatus{}, err
	}
	clientKind, err := normalizeClientKind(input.ClientKind)
	if err != nil {
		return RouteStatus{}, err
	}
	if !surfaceCompatible(surface, clientKind) {
		return RouteStatus{}, ErrSurfaceIncompatible
	}
	providerRow, err := s.store.GetAdminAIProviderKey(ctx, string(provider))
	if errors.Is(err, pgx.ErrNoRows) {
		return RouteStatus{}, ErrProviderNotConfigured
	}
	if err != nil {
		return RouteStatus{}, err
	}
	if !providerRow.Enabled {
		return RouteStatus{}, ErrProviderDisabled
	}

	modelOverride := strings.TrimSpace(input.ModelOverride)
	row, err := s.store.UpsertAISurfaceRoute(ctx, db.UpsertAISurfaceRouteParams{
		Surface:          string(surface),
		Provider:         string(provider),
		ClientKind:       string(clientKind),
		ModelOverride:    modelOverride,
		CreatedByAdminID: textOrNull(input.ActorAdminID),
		UpdatedByAdminID: textOrNull(input.ActorAdminID),
	})
	if err != nil {
		return RouteStatus{}, err
	}
	status := routeStatusFromRow(row, SourceAdmin, s.modelFor(surface, providerStatusFromRow(providerRow), clientKind, modelOverride))
	_ = s.writeEvent(ctx, EventActivated, provider, surface, input.ActorAdminID, map[string]any{
		"provider":       provider,
		"surface":        surface,
		"client_kind":    clientKind,
		"model_override": modelOverride,
	})
	return status, nil
}

func (s *Service) UnrouteSurface(ctx context.Context, surface Surface, actorAdminID string) error {
	normalized, err := normalizeSurface(surface)
	if err != nil {
		return err
	}
	if err := s.store.DeleteAISurfaceRoute(ctx, string(normalized)); err != nil {
		return err
	}
	_ = s.writeEvent(ctx, EventActivated, "", normalized, actorAdminID, map[string]any{
		"surface": string(normalized),
		"source":  SourceEnv,
	})
	return nil
}

func (s *Service) DisableProvider(ctx context.Context, provider Provider, actorAdminID string) (ProviderStatus, error) {
	normalized, err := normalizeProvider(provider)
	if err != nil {
		return ProviderStatus{}, err
	}
	if err := s.store.DeleteAISurfaceRoutesForProvider(ctx, string(normalized)); err != nil {
		return ProviderStatus{}, err
	}
	row, err := s.store.DisableAdminAIProviderKey(ctx, db.DisableAdminAIProviderKeyParams{
		Provider:         string(normalized),
		UpdatedByAdminID: textOrNull(actorAdminID),
	})
	if err != nil {
		return ProviderStatus{}, err
	}
	_ = s.writeEvent(ctx, EventDisabled, normalized, "", actorAdminID, map[string]any{
		"provider": normalized,
		"key_tail": row.KeyTail,
	})
	return providerStatusFromRow(row), nil
}

func (s *Service) Resolve(ctx context.Context, surface Surface) (EffectiveConfig, error) {
	normalized, err := normalizeSurface(surface)
	if err != nil {
		return EffectiveConfig{}, err
	}
	if route, err := s.store.GetAISurfaceRoute(ctx, string(normalized)); err == nil {
		if cfg, err := s.resolveAdminRoute(ctx, normalized, route); err == nil {
			return cfg, nil
		} else {
			slog.Warn("ai_provider_route_resolve_failed",
				"surface", normalized,
				"provider", route.Provider,
				"error", err)
			return EffectiveConfig{}, err
		}
	} else if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return EffectiveConfig{}, err
	}
	return s.resolveEnv(normalized)
}

func (s *Service) resolveAdminRoute(ctx context.Context, surface Surface, route db.AiSurfaceRouting) (EffectiveConfig, error) {
	clientKind, err := normalizeClientKind(ClientKind(route.ClientKind))
	if err != nil || !surfaceCompatible(surface, clientKind) {
		return EffectiveConfig{}, ErrSurfaceIncompatible
	}
	row, err := s.store.GetAdminAIProviderKey(ctx, route.Provider)
	if err != nil {
		return EffectiveConfig{}, err
	}
	if !row.Enabled {
		return EffectiveConfig{}, ErrProviderDisabled
	}
	provider, err := normalizeProvider(Provider(row.Provider))
	if err != nil {
		return EffectiveConfig{}, err
	}
	apiKey, err := s.cipher.Decrypt(row.ApiKeyCiphertext)
	if err != nil {
		return EffectiveConfig{}, err
	}
	status := providerStatusFromRow(row)
	return EffectiveConfig{
		Provider:   provider,
		Source:     SourceAdmin,
		ClientKind: clientKind,
		APIKey:     strings.TrimSpace(apiKey),
		BaseURL:    firstNonEmpty(normalizeBaseURL(row.BaseUrl), defaultBaseURL(provider)),
		Model:      s.modelFor(surface, status, clientKind, route.ModelOverride),
		Surface:    surface,
	}, nil
}

func (s *Service) resolveEnv(surface Surface) (EffectiveConfig, error) {
	switch surface {
	case SurfacePostAssist, SurfaceErrorTriage:
		apiKey := strings.TrimSpace(s.lookup("OPENAI_API_KEY"))
		if apiKey == "" {
			return EffectiveConfig{}, ErrProviderNotConfigured
		}
		baseURL := DefaultOpenAIBaseURL
		if surface == SurfaceErrorTriage {
			baseURL = firstNonEmpty(normalizeBaseURL(s.lookup("OPENAI_ERROR_TRIAGE_URL")), baseURL)
		}
		return EffectiveConfig{
			Provider:   ProviderOpenAI,
			Source:     SourceEnv,
			ClientKind: ClientKindChatCompletions,
			APIKey:     apiKey,
			BaseURL:    baseURL,
			Model:      s.envChatModel(surface),
			Surface:    surface,
		}, nil
	case SurfaceAppReviewAI:
		apiKey := strings.TrimSpace(s.lookup("ANTHROPIC_API_KEY"))
		if apiKey == "" {
			return EffectiveConfig{}, ErrProviderNotConfigured
		}
		return EffectiveConfig{
			Provider:   ProviderAnthropic,
			Source:     SourceEnv,
			ClientKind: ClientKindMessages,
			APIKey:     apiKey,
			BaseURL:    DefaultAnthropicBaseURL,
			Model:      s.envMessagesModel(surface),
			Surface:    surface,
		}, nil
	default:
		return EffectiveConfig{}, ErrSurfaceUnsupported
	}
}

func (s *Service) ListStatus(ctx context.Context) (StatusResponse, error) {
	rows, err := s.store.ListAdminAIProviderKeys(ctx)
	if err != nil {
		return StatusResponse{}, err
	}
	byProvider := map[Provider]ProviderStatus{}
	for _, provider := range []Provider{ProviderTokenGate, ProviderOpenAI, ProviderAnthropic} {
		byProvider[provider] = s.envProviderStatus(provider)
	}
	for _, row := range rows {
		status := providerStatusFromRow(row)
		byProvider[status.Provider] = status
	}

	providers := make([]ProviderStatus, 0, 3)
	for _, provider := range []Provider{ProviderTokenGate, ProviderOpenAI, ProviderAnthropic} {
		providers = append(providers, byProvider[provider])
	}

	routes := map[Surface]RouteStatus{}
	routeRows, err := s.store.ListAISurfaceRoutes(ctx)
	if err != nil {
		return StatusResponse{}, err
	}
	for _, row := range routeRows {
		providerStatus := byProvider[Provider(row.Provider)]
		surface := Surface(row.Surface)
		routes[surface] = routeStatusFromRow(row, SourceAdmin, s.modelFor(surface, providerStatus, ClientKind(row.ClientKind), row.ModelOverride))
	}

	effective := map[Surface]RouteStatus{}
	for _, surface := range []Surface{SurfacePostAssist, SurfaceErrorTriage, SurfaceAppReviewAI} {
		cfg, err := s.Resolve(ctx, surface)
		if err != nil {
			effective[surface] = RouteStatus{Surface: surface, Source: SourceNone}
			continue
		}
		effective[surface] = RouteStatus{
			Surface:    surface,
			Provider:   cfg.Provider,
			Source:     cfg.Source,
			ClientKind: cfg.ClientKind,
			Model:      cfg.Model,
		}
	}
	return StatusResponse{Providers: providers, Effective: effective, Routes: routes}, nil
}

func (s *Service) ListEvents(ctx context.Context, provider Provider, action string, beforeID int64, limit int32) ([]ProviderEvent, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	providerFilter := ""
	if provider != "" {
		normalized, err := normalizeProvider(provider)
		if err != nil {
			return nil, err
		}
		providerFilter = string(normalized)
	}
	rows, err := s.store.ListAdminAIProviderEvents(ctx, db.ListAdminAIProviderEventsParams{
		ProviderFilter: providerFilter,
		ActionFilter:   strings.TrimSpace(action),
		BeforeID:       beforeID,
		LimitRows:      limit,
	})
	if err != nil {
		return nil, err
	}
	out := make([]ProviderEvent, 0, len(rows))
	for _, row := range rows {
		out = append(out, providerEventFromRow(row))
	}
	return out, nil
}

func (s *Service) modelFor(surface Surface, provider ProviderStatus, clientKind ClientKind, override string) string {
	if model := strings.TrimSpace(override); model != "" {
		return model
	}
	if clientKind == ClientKindMessages {
		if provider.MessagesModel != "" {
			return provider.MessagesModel
		}
		return s.envMessagesModel(surface)
	}
	if provider.ChatModel != "" {
		return provider.ChatModel
	}
	return s.envChatModel(surface)
}

func (s *Service) envChatModel(surface Surface) string {
	if surface == SurfaceErrorTriage {
		if model := strings.TrimSpace(s.lookup("OPENAI_ERROR_TRIAGE_MODEL")); model != "" {
			return model
		}
	}
	if model := strings.TrimSpace(s.lookup("OPENAI_MODEL")); model != "" {
		return model
	}
	return DefaultOpenAIModel
}

func (s *Service) envMessagesModel(_ Surface) string {
	if model := strings.TrimSpace(s.lookup("ANTHROPIC_MODEL")); model != "" {
		return model
	}
	return DefaultAnthropicModel
}

func (s *Service) envProviderStatus(provider Provider) ProviderStatus {
	status := ProviderStatus{Provider: provider, Source: SourceNone, BaseURL: defaultBaseURL(provider)}
	switch provider {
	case ProviderOpenAI:
		if apiKey := strings.TrimSpace(s.lookup("OPENAI_API_KEY")); apiKey != "" {
			status.Configured = true
			status.Enabled = true
			status.Source = SourceEnv
			status.KeyTail = keyTail(apiKey)
			status.ChatModel = s.envChatModel(SurfacePostAssist)
		}
	case ProviderAnthropic:
		if apiKey := strings.TrimSpace(s.lookup("ANTHROPIC_API_KEY")); apiKey != "" {
			status.Configured = true
			status.Enabled = true
			status.Source = SourceEnv
			status.KeyTail = keyTail(apiKey)
			status.MessagesModel = s.envMessagesModel(SurfaceAppReviewAI)
		}
	case ProviderTokenGate:
	}
	return status
}

func (s *Service) lookup(key string) string {
	if s != nil && s.env != nil {
		return s.env(key)
	}
	return ""
}

func (s *Service) writeEvent(ctx context.Context, action string, provider Provider, surface Surface, actorAdminID string, metadata map[string]any) error {
	if s == nil || s.store == nil {
		return nil
	}
	raw, _ := json.Marshal(metadata)
	_, err := s.store.CreateAdminAIProviderEvent(ctx, db.CreateAdminAIProviderEventParams{
		Provider:     textOrNull(string(provider)),
		Surface:      textOrNull(string(surface)),
		Action:       action,
		Category:     "config",
		ActorAdminID: textOrNull(actorAdminID),
		Metadata:     raw,
	})
	return err
}

func normalizeProvider(provider Provider) (Provider, error) {
	switch Provider(strings.TrimSpace(string(provider))) {
	case ProviderTokenGate:
		return ProviderTokenGate, nil
	case ProviderOpenAI:
		return ProviderOpenAI, nil
	case ProviderAnthropic:
		return ProviderAnthropic, nil
	default:
		return "", ErrProviderUnsupported
	}
}

func normalizeSurface(surface Surface) (Surface, error) {
	switch Surface(strings.TrimSpace(string(surface))) {
	case SurfacePostAssist:
		return SurfacePostAssist, nil
	case SurfaceErrorTriage:
		return SurfaceErrorTriage, nil
	case SurfaceAppReviewAI:
		return SurfaceAppReviewAI, nil
	default:
		return "", ErrSurfaceUnsupported
	}
}

func normalizeClientKind(clientKind ClientKind) (ClientKind, error) {
	switch ClientKind(strings.TrimSpace(string(clientKind))) {
	case ClientKindChatCompletions:
		return ClientKindChatCompletions, nil
	case ClientKindMessages:
		return ClientKindMessages, nil
	default:
		return "", ErrClientKindUnsupported
	}
}

func surfaceCompatible(surface Surface, clientKind ClientKind) bool {
	switch surface {
	case SurfacePostAssist, SurfaceErrorTriage:
		return clientKind == ClientKindChatCompletions
	case SurfaceAppReviewAI:
		return clientKind == ClientKindMessages
	default:
		return false
	}
}

func normalizeBaseURL(value string) string {
	return strings.TrimRight(strings.TrimSpace(value), "/")
}

func defaultBaseURL(provider Provider) string {
	switch provider {
	case ProviderTokenGate:
		return DefaultTokenGateBaseURL
	case ProviderAnthropic:
		return DefaultAnthropicBaseURL
	default:
		return DefaultOpenAIBaseURL
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func keyTail(value string) string {
	trimmed := strings.TrimSpace(value)
	if len(trimmed) <= 4 {
		return trimmed
	}
	return trimmed[len(trimmed)-4:]
}

func textOrNull(value string) pgtype.Text {
	trimmed := strings.TrimSpace(value)
	return pgtype.Text{String: trimmed, Valid: trimmed != ""}
}

func timestamp(value time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: value.UTC(), Valid: true}
}

func providerStatusFromRow(row db.AdminAiProviderKey) ProviderStatus {
	status := ProviderStatus{
		Provider:      Provider(row.Provider),
		Configured:    row.ApiKeyCiphertext != "",
		Enabled:       row.Enabled,
		Source:        SourceAdmin,
		KeyTail:       row.KeyTail,
		BaseURL:       normalizeBaseURL(row.BaseUrl),
		ChatModel:     row.ChatModel,
		MessagesModel: row.MessagesModel,
	}
	if row.LastValidatedAt.Valid {
		status.LastValidatedAt = row.LastValidatedAt.Time.UTC().Format(time.RFC3339)
	}
	if row.LastValidationStatus.Valid {
		status.LastValidationStatus = row.LastValidationStatus.String
	}
	if row.LastValidationError.Valid {
		status.LastValidationError = row.LastValidationError.String
	}
	if row.LastRotatedAt.Valid {
		status.LastRotatedAt = row.LastRotatedAt.Time.UTC().Format(time.RFC3339)
	}
	if row.UpdatedAt.Valid {
		status.UpdatedAt = row.UpdatedAt.Time.UTC().Format(time.RFC3339)
	}
	return status
}

func routeStatusFromRow(row db.AiSurfaceRouting, source Source, model string) RouteStatus {
	return RouteStatus{
		Surface:       Surface(row.Surface),
		Provider:      Provider(row.Provider),
		Source:        source,
		ClientKind:    ClientKind(row.ClientKind),
		Model:         model,
		ModelOverride: row.ModelOverride,
	}
}

func providerEventFromRow(row db.AdminAiProviderEvent) ProviderEvent {
	event := ProviderEvent{
		ID:       row.ID,
		Action:   row.Action,
		Category: row.Category,
	}
	if row.Provider.Valid {
		event.Provider = Provider(row.Provider.String)
	}
	if row.Surface.Valid {
		event.Surface = Surface(row.Surface.String)
	}
	if row.ActorAdminID.Valid {
		event.ActorAdminID = row.ActorAdminID.String
	}
	if len(row.Metadata) > 0 {
		event.Metadata = append(event.Metadata, row.Metadata...)
	}
	if row.CreatedAt.Valid {
		event.CreatedAt = row.CreatedAt.Time.UTC().Format(time.RFC3339)
	}
	return event
}
