package aiproviders

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/db"
)

func TestSaveProviderCreateUpdateRotateSemantics(t *testing.T) {
	store := newFakeStore()
	cipher := fakeCipher{}
	service := NewService(store, cipher).WithEnv(mapEnv(nil))

	first, err := service.SaveProvider(context.Background(), SaveProviderInput{
		Provider:      ProviderTokenGate,
		APIKey:        "tokengate-first",
		BaseURL:       " https://gateway.mytokengate.com/v1/ ",
		ChatModel:     "gpt-4o",
		MessagesModel: "claude-sonnet-4-6",
		Enabled:       true,
		ActorAdminID:  "admin_1",
	})
	if err != nil {
		t.Fatalf("first save failed: %v", err)
	}
	if first.KeyTail != "irst" {
		t.Fatalf("expected key tail only, got %q", first.KeyTail)
	}
	if first.BaseURL != "https://gateway.mytokengate.com/v1" {
		t.Fatalf("base URL was not normalized: %q", first.BaseURL)
	}
	row := store.providers[string(ProviderTokenGate)]
	if row.ApiKeyCiphertext == "tokengate-first" || row.ApiKeyCiphertext == "" {
		t.Fatalf("stored key should be encrypted, got %q", row.ApiKeyCiphertext)
	}
	if got := store.eventActions(); len(got) != 1 || got[0] != EventCreated {
		t.Fatalf("expected CREATED event, got %#v", got)
	}

	ciphertext := row.ApiKeyCiphertext
	updated, err := service.SaveProvider(context.Background(), SaveProviderInput{
		Provider:      ProviderTokenGate,
		BaseURL:       "https://gateway.mytokengate.com/v1",
		ChatModel:     "gpt-4.1-mini",
		MessagesModel: "claude-sonnet-4-6",
		Enabled:       true,
		ActorAdminID:  "admin_1",
	})
	if err != nil {
		t.Fatalf("config update failed: %v", err)
	}
	if updated.ChatModel != "gpt-4.1-mini" {
		t.Fatalf("chat model was not updated: %q", updated.ChatModel)
	}
	if store.providers[string(ProviderTokenGate)].ApiKeyCiphertext != ciphertext {
		t.Fatalf("empty key update should keep existing ciphertext")
	}
	if got := store.eventActions(); len(got) != 2 || got[1] != EventUpdated {
		t.Fatalf("expected UPDATED event, got %#v", got)
	}

	rotated, err := service.SaveProvider(context.Background(), SaveProviderInput{
		Provider:      ProviderTokenGate,
		APIKey:        "tokengate-second",
		BaseURL:       "https://gateway.mytokengate.com/v1",
		ChatModel:     "gpt-4.1-mini",
		MessagesModel: "claude-sonnet-4-6",
		Enabled:       true,
		ActorAdminID:  "admin_1",
	})
	if err != nil {
		t.Fatalf("rotation failed: %v", err)
	}
	if rotated.KeyTail != "cond" {
		t.Fatalf("rotated tail mismatch: %q", rotated.KeyTail)
	}
	if store.providers[string(ProviderTokenGate)].ApiKeyCiphertext == ciphertext {
		t.Fatalf("rotation should replace ciphertext")
	}
	if got := store.eventActions(); len(got) != 3 || got[2] != EventRotated {
		t.Fatalf("expected ROTATED event, got %#v", got)
	}
}

func TestSaveProviderRequiresKeyOnFirstSave(t *testing.T) {
	service := NewService(newFakeStore(), fakeCipher{}).WithEnv(mapEnv(nil))

	_, err := service.SaveProvider(context.Background(), SaveProviderInput{
		Provider: ProviderOpenAI,
		BaseURL:  "https://api.openai.com/v1",
		Enabled:  true,
	})
	if !errors.Is(err, ErrProviderKeyRequired) {
		t.Fatalf("expected ErrProviderKeyRequired, got %v", err)
	}
}

func TestResolveEffectiveConfigUsesRouteThenEnvFallback(t *testing.T) {
	store := newFakeStore()
	service := NewService(store, fakeCipher{}).WithEnv(mapEnv(map[string]string{
		"OPENAI_API_KEY":              "env-openai",
		"OPENAI_MODEL":                "gpt-env-general",
		"OPENAI_ERROR_TRIAGE_MODEL":   "gpt-env-triage",
		"ANTHROPIC_API_KEY":           "env-anthropic",
		"ANTHROPIC_MODEL":             "claude-env",
		"OPENAI_ERROR_TRIAGE_URL":     "https://openai.example/v1/chat/completions",
		"TOKEN_SHOULD_NOT_BE_VISIBLE": "secret",
	}))

	_, err := service.SaveProvider(context.Background(), SaveProviderInput{
		Provider:  ProviderTokenGate,
		APIKey:    "tokengate-secret",
		BaseURL:   "https://gateway.example/v1/",
		ChatModel: "gpt-provider",
		Enabled:   true,
	})
	if err != nil {
		t.Fatalf("save provider failed: %v", err)
	}
	if _, err := service.RouteSurface(context.Background(), RouteSurfaceInput{
		Surface:       SurfaceErrorTriage,
		Provider:      ProviderTokenGate,
		ClientKind:    ClientKindChatCompletions,
		ModelOverride: "gpt-route",
	}); err != nil {
		t.Fatalf("route surface failed: %v", err)
	}

	cfg, err := service.Resolve(context.Background(), SurfaceErrorTriage)
	if err != nil {
		t.Fatalf("resolve routed config failed: %v", err)
	}
	if cfg.Source != SourceAdmin || cfg.Provider != ProviderTokenGate || cfg.Model != "gpt-route" {
		t.Fatalf("unexpected routed config: %#v", cfg)
	}
	if cfg.ChatCompletionsURL() != "https://gateway.example/v1/chat/completions" {
		t.Fatalf("unexpected chat URL: %q", cfg.ChatCompletionsURL())
	}

	if err := service.UnrouteSurface(context.Background(), SurfaceErrorTriage, "admin_1"); err != nil {
		t.Fatalf("unroute failed: %v", err)
	}
	cfg, err = service.Resolve(context.Background(), SurfaceErrorTriage)
	if err != nil {
		t.Fatalf("resolve env config failed: %v", err)
	}
	if cfg.Source != SourceEnv || cfg.Provider != ProviderOpenAI || cfg.Model != "gpt-env-triage" {
		t.Fatalf("unexpected env config: %#v", cfg)
	}
	if cfg.ChatCompletionsURL() != "https://openai.example/v1/chat/completions" {
		t.Fatalf("expected error triage URL fallback, got %q", cfg.ChatCompletionsURL())
	}
}

func TestMessagesHeadersDifferByProvider(t *testing.T) {
	native := EffectiveConfig{Provider: ProviderAnthropic, APIKey: "anthropic-key"}
	req, _ := http.NewRequest(http.MethodPost, "https://api.anthropic.com/v1/messages", nil)
	ApplyMessagesHeaders(req, native)
	if got := req.Header.Get("x-api-key"); got != "anthropic-key" {
		t.Fatalf("native anthropic x-api-key mismatch: %q", got)
	}
	if got := req.Header.Get("anthropic-version"); got == "" {
		t.Fatalf("native anthropic should include anthropic-version")
	}
	if got := req.Header.Get("Authorization"); got != "" {
		t.Fatalf("native anthropic should not use bearer auth, got %q", got)
	}

	tokengate := EffectiveConfig{Provider: ProviderTokenGate, APIKey: "tg-key"}
	req, _ = http.NewRequest(http.MethodPost, "https://gateway.mytokengate.com/v1/messages", nil)
	ApplyMessagesHeaders(req, tokengate)
	if got := req.Header.Get("Authorization"); got != "Bearer tg-key" {
		t.Fatalf("tokengate messages bearer mismatch: %q", got)
	}
	if got := req.Header.Get("x-api-key"); got != "" {
		t.Fatalf("tokengate messages should not use x-api-key, got %q", got)
	}
}

func TestValidationStatusFromHTTPStatus(t *testing.T) {
	cases := map[int]ValidationStatus{
		400: ValidationConfigFailed,
		401: ValidationAuthFailed,
		403: ValidationAuthFailed,
		429: ValidationRateLimited,
		503: ValidationProviderFailed,
		504: ValidationProviderFailed,
		500: ValidationProviderFailed,
	}
	for status, want := range cases {
		if got := ValidationStatusFromHTTPStatus(status); got != want {
			t.Fatalf("status %d: got %q want %q", status, got, want)
		}
	}
}

type fakeCipher struct{}

func (fakeCipher) Encrypt(value string) (string, error) {
	return "enc:" + value, nil
}

func (fakeCipher) Decrypt(value string) (string, error) {
	return strings.TrimPrefix(value, "enc:"), nil
}

type fakeStore struct {
	providers map[string]db.AdminAiProviderKey
	routes    map[string]db.AiSurfaceRouting
	events    []db.AdminAiProviderEvent
	nextID    int64
}

func newFakeStore() *fakeStore {
	return &fakeStore{
		providers: map[string]db.AdminAiProviderKey{},
		routes:    map[string]db.AiSurfaceRouting{},
		nextID:    1,
	}
}

func (s *fakeStore) GetAdminAIProviderKey(_ context.Context, provider string) (db.AdminAiProviderKey, error) {
	row, ok := s.providers[provider]
	if !ok {
		return db.AdminAiProviderKey{}, pgx.ErrNoRows
	}
	return row, nil
}

func (s *fakeStore) ListAdminAIProviderKeys(context.Context) ([]db.AdminAiProviderKey, error) {
	out := make([]db.AdminAiProviderKey, 0, len(s.providers))
	for _, row := range s.providers {
		out = append(out, row)
	}
	return out, nil
}

func (s *fakeStore) UpsertAdminAIProviderKey(_ context.Context, arg db.UpsertAdminAIProviderKeyParams) (db.AdminAiProviderKey, error) {
	existing, exists := s.providers[arg.Provider]
	row := db.AdminAiProviderKey{
		Provider:         arg.Provider,
		Enabled:          arg.Enabled,
		ApiKeyCiphertext: arg.ApiKeyCiphertext,
		KeyTail:          arg.KeyTail,
		BaseUrl:          arg.BaseUrl,
		ChatModel:        arg.ChatModel,
		MessagesModel:    arg.MessagesModel,
		LastRotatedAt:    arg.LastRotatedAt,
		CreatedByAdminID: arg.CreatedByAdminID,
		UpdatedByAdminID: arg.UpdatedByAdminID,
		CreatedAt:        pgTimestamp(),
		UpdatedAt:        pgTimestamp(),
	}
	if exists {
		row.CreatedByAdminID = existing.CreatedByAdminID
		row.CreatedAt = existing.CreatedAt
	}
	s.providers[arg.Provider] = row
	return row, nil
}

func (s *fakeStore) UpdateAdminAIProviderConfig(_ context.Context, arg db.UpdateAdminAIProviderConfigParams) (db.AdminAiProviderKey, error) {
	row, ok := s.providers[arg.Provider]
	if !ok {
		return db.AdminAiProviderKey{}, pgx.ErrNoRows
	}
	row.Enabled = arg.Enabled
	row.BaseUrl = arg.BaseUrl
	row.ChatModel = arg.ChatModel
	row.MessagesModel = arg.MessagesModel
	row.UpdatedByAdminID = arg.UpdatedByAdminID
	row.UpdatedAt = pgTimestamp()
	s.providers[arg.Provider] = row
	return row, nil
}

func (s *fakeStore) UpdateAdminAIProviderValidation(_ context.Context, arg db.UpdateAdminAIProviderValidationParams) (db.AdminAiProviderKey, error) {
	row, ok := s.providers[arg.Provider]
	if !ok {
		return db.AdminAiProviderKey{}, pgx.ErrNoRows
	}
	row.LastValidatedAt = pgTimestamp()
	row.LastValidationStatus = arg.LastValidationStatus
	row.LastValidationError = arg.LastValidationError
	row.UpdatedByAdminID = arg.UpdatedByAdminID
	row.UpdatedAt = pgTimestamp()
	s.providers[arg.Provider] = row
	return row, nil
}

func (s *fakeStore) DisableAdminAIProviderKey(_ context.Context, arg db.DisableAdminAIProviderKeyParams) (db.AdminAiProviderKey, error) {
	row, ok := s.providers[arg.Provider]
	if !ok {
		return db.AdminAiProviderKey{}, pgx.ErrNoRows
	}
	row.Enabled = false
	row.UpdatedByAdminID = arg.UpdatedByAdminID
	s.providers[arg.Provider] = row
	return row, nil
}

func (s *fakeStore) DeleteAISurfaceRoutesForProvider(_ context.Context, provider string) error {
	for surface, route := range s.routes {
		if route.Provider == provider {
			delete(s.routes, surface)
		}
	}
	return nil
}

func (s *fakeStore) GetAISurfaceRoute(_ context.Context, surface string) (db.AiSurfaceRouting, error) {
	route, ok := s.routes[surface]
	if !ok {
		return db.AiSurfaceRouting{}, pgx.ErrNoRows
	}
	return route, nil
}

func (s *fakeStore) ListAISurfaceRoutes(context.Context) ([]db.AiSurfaceRouting, error) {
	out := make([]db.AiSurfaceRouting, 0, len(s.routes))
	for _, route := range s.routes {
		out = append(out, route)
	}
	return out, nil
}

func (s *fakeStore) UpsertAISurfaceRoute(_ context.Context, arg db.UpsertAISurfaceRouteParams) (db.AiSurfaceRouting, error) {
	route := db.AiSurfaceRouting{
		Surface:          arg.Surface,
		Provider:         arg.Provider,
		ClientKind:       arg.ClientKind,
		ModelOverride:    arg.ModelOverride,
		CreatedByAdminID: arg.CreatedByAdminID,
		UpdatedByAdminID: arg.UpdatedByAdminID,
		CreatedAt:        pgTimestamp(),
		UpdatedAt:        pgTimestamp(),
	}
	s.routes[arg.Surface] = route
	return route, nil
}

func (s *fakeStore) DeleteAISurfaceRoute(_ context.Context, surface string) error {
	delete(s.routes, surface)
	return nil
}

func (s *fakeStore) CreateAdminAIProviderEvent(_ context.Context, arg db.CreateAdminAIProviderEventParams) (db.AdminAiProviderEvent, error) {
	event := db.AdminAiProviderEvent{
		ID:           s.nextID,
		Provider:     arg.Provider,
		Surface:      arg.Surface,
		Action:       arg.Action,
		Category:     arg.Category,
		ActorAdminID: arg.ActorAdminID,
		Metadata:     arg.Metadata,
		CreatedAt:    pgTimestamp(),
	}
	s.nextID++
	s.events = append(s.events, event)
	return event, nil
}

func (s *fakeStore) ListAdminAIProviderEvents(_ context.Context, _ db.ListAdminAIProviderEventsParams) ([]db.AdminAiProviderEvent, error) {
	return s.events, nil
}

func (s *fakeStore) eventActions() []string {
	out := make([]string, 0, len(s.events))
	for _, event := range s.events {
		out = append(out, event.Action)
	}
	return out
}

func pgTimestamp() pgtype.Timestamptz {
	return pgtype.Timestamptz{Valid: true}
}
