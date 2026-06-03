package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/apikey"
	"github.com/xiaoboyu/unipost-api/internal/auth"
	"github.com/xiaoboyu/unipost-api/internal/db"
)

func TestCLISetupTokenIssueStoresOnlyHashAndReturnsAgentCommand(t *testing.T) {
	now := time.Date(2026, 6, 3, 12, 0, 0, 0, time.UTC)
	store := &cliSetupTokenFakeStore{}
	h := NewCLISetupTokenHandler(store).
		WithNow(func() time.Time { return now }).
		WithTokenGenerator(func() (string, error) { return "ust_test_issue_token", nil })

	req := httptest.NewRequest(http.MethodPost, "https://dev-api.unipost.dev/v1/cli/setup-tokens", strings.NewReader(`{"client":"codex"}`))
	ctx := auth.SetWorkspaceID(req.Context(), "ws_setup")
	ctx = context.WithValue(ctx, auth.UserIDKey, "user_admin")
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	h.Issue(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var env struct {
		Data struct {
			SetupToken string    `json:"setup_token"`
			Client     string    `json:"client"`
			KeyName    string    `json:"key_name"`
			ExpiresAt  time.Time `json:"expires_at"`
			Command    string    `json:"command"`
			Prompt     string    `json:"recommended_prompt"`
		} `json:"data"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&env); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if env.Data.SetupToken != "ust_test_issue_token" {
		t.Fatalf("setup token = %q", env.Data.SetupToken)
	}
	if env.Data.Client != "codex" || env.Data.KeyName != "Codex CLI" {
		t.Fatalf("unexpected client/key name: %+v", env.Data)
	}
	if env.Data.ExpiresAt.Sub(now.Add(10*time.Minute)) != 0 {
		t.Fatalf("expires_at = %s", env.Data.ExpiresAt)
	}
	wantCommand := "npx -y @unipost/cli agent bootstrap --client codex --setup-token ust_test_issue_token --base-url https://dev-api.unipost.dev --json"
	if env.Data.Command != wantCommand {
		t.Fatalf("command = %q, want %q", env.Data.Command, wantCommand)
	}
	if !strings.Contains(env.Data.Prompt, "npx -y @unipost/cli agent bootstrap --base-url https://dev-api.unipost.dev --json") {
		t.Fatalf("recommended prompt should use npx command, got %q", env.Data.Prompt)
	}
	if store.createdSetup.WorkspaceID != "ws_setup" || store.createdSetup.UserID != "user_admin" {
		t.Fatalf("created setup params = %+v", store.createdSetup)
	}
	if store.createdSetup.TokenHash == "" || store.createdSetup.TokenHash == "ust_test_issue_token" {
		t.Fatalf("setup token should be stored as a hash, got %q", store.createdSetup.TokenHash)
	}
}

func TestCLISetupTokenExchangeCreatesNamedAPIKeyAndConsumesToken(t *testing.T) {
	now := time.Date(2026, 6, 3, 12, 0, 0, 0, time.UTC)
	setupToken := "ust_test_exchange_token"
	store := &cliSetupTokenFakeStore{
		tokensByHash: map[string]db.CLISetupToken{
			apikey.Hash(setupToken): {
				ID:          "setup_1",
				WorkspaceID: "ws_exchange",
				UserID:      "user_admin",
				TokenHash:   apikey.Hash(setupToken),
				Client:      "claude-code",
				KeyName:     "Claude Code CLI",
				ExpiresAt:   pgtype.Timestamptz{Time: now.Add(10 * time.Minute), Valid: true},
				CreatedAt:   pgtype.Timestamptz{Time: now, Valid: true},
			},
		},
	}
	h := NewCLISetupTokenHandler(store).WithNow(func() time.Time { return now })

	req := httptest.NewRequest(http.MethodPost, "/v1/cli/setup-tokens/exchange", strings.NewReader(`{"setup_token":"ust_test_exchange_token","client":"claude-code"}`))
	rec := httptest.NewRecorder()

	h.Exchange(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var env struct {
		Data struct {
			WorkspaceID string `json:"workspace_id"`
			Client      string `json:"client"`
			APIKey      struct {
				ID          string `json:"id"`
				Name        string `json:"name"`
				Key         string `json:"key"`
				Prefix      string `json:"prefix"`
				Environment string `json:"environment"`
			} `json:"api_key"`
		} `json:"data"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&env); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if env.Data.WorkspaceID != "ws_exchange" || env.Data.Client != "claude-code" {
		t.Fatalf("unexpected exchange data: %+v", env.Data)
	}
	if env.Data.APIKey.Name != "Claude Code CLI" || !strings.HasPrefix(env.Data.APIKey.Key, "up_live_") {
		t.Fatalf("unexpected api key response: %+v", env.Data.APIKey)
	}
	if len(store.markedUsed) != 1 || store.markedUsed[0].ID != "setup_1" {
		t.Fatalf("token was not consumed: %+v", store.markedUsed)
	}
	if len(store.createdKeys) != 1 {
		t.Fatalf("created key count = %d", len(store.createdKeys))
	}
	created := store.createdKeys[0]
	if created.WorkspaceID != "ws_exchange" || created.Name != "Claude Code CLI" || created.CreatedByUserID != "user_admin" {
		t.Fatalf("created key params = %+v", created)
	}
	if created.KeyHash == "" || created.KeyHash == env.Data.APIKey.Key {
		t.Fatalf("api key hash should not be plaintext, got %q", created.KeyHash)
	}
}

func TestCLISetupTokenExchangeRejectsExpiredToken(t *testing.T) {
	now := time.Date(2026, 6, 3, 12, 0, 0, 0, time.UTC)
	setupToken := "ust_test_expired_token"
	store := &cliSetupTokenFakeStore{
		tokensByHash: map[string]db.CLISetupToken{
			apikey.Hash(setupToken): {
				ID:          "setup_expired",
				WorkspaceID: "ws_exchange",
				UserID:      "user_admin",
				TokenHash:   apikey.Hash(setupToken),
				Client:      "codex",
				KeyName:     "Codex CLI",
				ExpiresAt:   pgtype.Timestamptz{Time: now.Add(-1 * time.Minute), Valid: true},
				CreatedAt:   pgtype.Timestamptz{Time: now.Add(-11 * time.Minute), Valid: true},
			},
		},
	}
	h := NewCLISetupTokenHandler(store).WithNow(func() time.Time { return now })

	req := httptest.NewRequest(http.MethodPost, "/v1/cli/setup-tokens/exchange", strings.NewReader(`{"setup_token":"ust_test_expired_token","client":"codex"}`))
	rec := httptest.NewRecorder()

	h.Exchange(rec, req)

	if rec.Code != http.StatusGone {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"normalized_code":"setup_token_expired"`) {
		t.Fatalf("expected setup_token_expired body, got %s", rec.Body.String())
	}
	if len(store.createdKeys) != 0 || len(store.markedUsed) != 0 {
		t.Fatalf("expired token should not be consumed or create a key: keys=%d used=%d", len(store.createdKeys), len(store.markedUsed))
	}
}

type cliSetupTokenFakeStore struct {
	createdSetup db.CreateCLISetupTokenParams
	tokensByHash map[string]db.CLISetupToken
	markedUsed   []db.MarkCLISetupTokenUsedParams
	createdKeys  []db.CreateAPIKeyParams
}

func (s *cliSetupTokenFakeStore) CreateCLISetupToken(_ context.Context, arg db.CreateCLISetupTokenParams) (db.CLISetupToken, error) {
	s.createdSetup = arg
	token := db.CLISetupToken{
		ID:          arg.ID,
		WorkspaceID: arg.WorkspaceID,
		UserID:      arg.UserID,
		TokenHash:   arg.TokenHash,
		Client:      arg.Client,
		KeyName:     arg.KeyName,
		ExpiresAt:   arg.ExpiresAt,
		CreatedAt:   arg.CreatedAt,
	}
	if s.tokensByHash == nil {
		s.tokensByHash = map[string]db.CLISetupToken{}
	}
	s.tokensByHash[arg.TokenHash] = token
	return token, nil
}

func (s *cliSetupTokenFakeStore) GetCLISetupTokenByHash(_ context.Context, tokenHash string) (db.CLISetupToken, error) {
	if s.tokensByHash == nil {
		return db.CLISetupToken{}, pgx.ErrNoRows
	}
	token, ok := s.tokensByHash[tokenHash]
	if !ok {
		return db.CLISetupToken{}, pgx.ErrNoRows
	}
	return token, nil
}

func (s *cliSetupTokenFakeStore) MarkCLISetupTokenUsed(_ context.Context, arg db.MarkCLISetupTokenUsedParams) (db.CLISetupToken, error) {
	s.markedUsed = append(s.markedUsed, arg)
	for hash, token := range s.tokensByHash {
		if token.ID == arg.ID {
			token.UsedAt = arg.UsedAt
			s.tokensByHash[hash] = token
			return token, nil
		}
	}
	return db.CLISetupToken{}, pgx.ErrNoRows
}

func (s *cliSetupTokenFakeStore) CreateAPIKey(_ context.Context, arg db.CreateAPIKeyParams) (db.ApiKey, error) {
	s.createdKeys = append(s.createdKeys, arg)
	return db.ApiKey{
		ID:              arg.ID,
		WorkspaceID:     arg.WorkspaceID,
		Name:            arg.Name,
		Prefix:          arg.Prefix,
		KeyHash:         arg.KeyHash,
		Environment:     arg.Environment,
		ExpiresAt:       arg.ExpiresAt,
		CreatedByUserID: arg.CreatedByUserID,
		CreatedAt:       pgtype.Timestamptz{Time: time.Date(2026, 6, 3, 12, 1, 0, 0, time.UTC), Valid: true},
	}, nil
}
