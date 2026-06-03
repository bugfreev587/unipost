package handler

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/apikey"
	"github.com/xiaoboyu/unipost-api/internal/auth"
	"github.com/xiaoboyu/unipost-api/internal/db"
)

const (
	cliSetupTokenTTL          = 10 * time.Minute
	cliSetupDefaultAPIBaseURL = "https://api.unipost.dev"
)

type CLISetupTokenStore interface {
	CreateCLISetupToken(context.Context, db.CreateCLISetupTokenParams) (db.CLISetupToken, error)
	GetCLISetupTokenByHash(context.Context, string) (db.CLISetupToken, error)
	MarkCLISetupTokenUsed(context.Context, db.MarkCLISetupTokenUsedParams) (db.CLISetupToken, error)
	CreateAPIKey(context.Context, db.CreateAPIKeyParams) (db.ApiKey, error)
}

type CLISetupTokenHandler struct {
	store          CLISetupTokenStore
	now            func() time.Time
	tokenGenerator func() (string, error)
	apiBaseURL     string
}

func NewCLISetupTokenHandler(store CLISetupTokenStore) *CLISetupTokenHandler {
	return &CLISetupTokenHandler{
		store:          store,
		now:            time.Now,
		tokenGenerator: generateCLISetupToken,
	}
}

func (h *CLISetupTokenHandler) WithNow(now func() time.Time) *CLISetupTokenHandler {
	h.now = now
	return h
}

func (h *CLISetupTokenHandler) WithTokenGenerator(generator func() (string, error)) *CLISetupTokenHandler {
	h.tokenGenerator = generator
	return h
}

func (h *CLISetupTokenHandler) WithAPIBaseURL(apiBaseURL string) *CLISetupTokenHandler {
	h.apiBaseURL = normalizeCLISetupBaseURL(apiBaseURL)
	return h
}

type cliSetupTokenIssueResponse struct {
	SetupToken        string    `json:"setup_token"`
	Client            string    `json:"client"`
	KeyName           string    `json:"key_name"`
	ExpiresAt         time.Time `json:"expires_at"`
	Command           string    `json:"command"`
	RecommendedPrompt string    `json:"recommended_prompt"`
}

type cliSetupTokenExchangeResponse struct {
	WorkspaceID string               `json:"workspace_id"`
	Client      string               `json:"client"`
	APIKey      apiKeyCreateResponse `json:"api_key"`
}

func (h *CLISetupTokenHandler) Issue(w http.ResponseWriter, r *http.Request) {
	workspaceID := auth.GetWorkspaceID(r.Context())
	userID := auth.GetUserID(r.Context())
	if workspaceID == "" || userID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing workspace or user context")
		return
	}

	var body struct {
		Client string `json:"client"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Invalid request body")
		return
	}
	client := normalizeCLIClient(body.Client)
	keyName := cliSetupKeyName(client)
	setupToken, err := h.tokenGenerator()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to generate setup token")
		return
	}
	now := h.now().UTC()
	expiresAt := now.Add(cliSetupTokenTTL)

	_, err = h.store.CreateCLISetupToken(r.Context(), db.CreateCLISetupTokenParams{
		ID:          uuid.New().String(),
		WorkspaceID: workspaceID,
		UserID:      userID,
		TokenHash:   apikey.Hash(setupToken),
		Client:      client,
		KeyName:     keyName,
		ExpiresAt:   pgtype.Timestamptz{Time: expiresAt, Valid: true},
		CreatedAt:   pgtype.Timestamptz{Time: now, Valid: true},
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to create setup token")
		return
	}

	apiBaseURL := h.cliSetupAPIBaseURL(r)
	writeCreated(w, cliSetupTokenIssueResponse{
		SetupToken:        setupToken,
		Client:            client,
		KeyName:           keyName,
		ExpiresAt:         expiresAt,
		Command:           cliSetupCommand(client, setupToken, apiBaseURL),
		RecommendedPrompt: cliSetupRecommendedPrompt(client, apiBaseURL),
	})
}

func (h *CLISetupTokenHandler) Exchange(w http.ResponseWriter, r *http.Request) {
	var body struct {
		SetupToken string `json:"setup_token"`
		Client     string `json:"client"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Invalid request body")
		return
	}
	body.SetupToken = strings.TrimSpace(body.SetupToken)
	if body.SetupToken == "" {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "setup_token is required")
		return
	}

	token, err := h.store.GetCLISetupTokenByHash(r.Context(), apikey.Hash(body.SetupToken))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusUnauthorized, "SETUP_TOKEN_INVALID", "Setup token is invalid")
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load setup token")
		return
	}
	now := h.now().UTC()
	if token.RevokedAt.Valid {
		writeError(w, http.StatusUnauthorized, "SETUP_TOKEN_INVALID", "Setup token is invalid")
		return
	}
	if token.UsedAt.Valid {
		writeError(w, http.StatusGone, "SETUP_TOKEN_USED", "Setup token has already been used")
		return
	}
	if token.ExpiresAt.Valid && !token.ExpiresAt.Time.After(now) {
		writeError(w, http.StatusGone, "SETUP_TOKEN_EXPIRED", "Setup token has expired")
		return
	}
	requestedClient := normalizeCLIClient(body.Client)
	if body.Client != "" && requestedClient != token.Client {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "client does not match setup token")
		return
	}

	_, err = h.store.MarkCLISetupTokenUsed(r.Context(), db.MarkCLISetupTokenUsedParams{
		ID:     token.ID,
		UsedAt: pgtype.Timestamptz{Time: now, Valid: true},
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusGone, "SETUP_TOKEN_USED", "Setup token has already been used")
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to consume setup token")
		return
	}

	plaintext, prefix, hash, err := apikey.Generate("production")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to generate API key")
		return
	}
	key, err := h.store.CreateAPIKey(r.Context(), db.CreateAPIKeyParams{
		ID:              uuid.New().String(),
		WorkspaceID:     token.WorkspaceID,
		Name:            token.KeyName,
		Prefix:          prefix,
		KeyHash:         hash,
		Environment:     "production",
		CreatedByUserID: token.UserID,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to create API key")
		return
	}

	writeCreated(w, cliSetupTokenExchangeResponse{
		WorkspaceID: token.WorkspaceID,
		Client:      token.Client,
		APIKey: apiKeyCreateResponse{
			ID:          key.ID,
			Name:        key.Name,
			Key:         plaintext,
			Prefix:      key.Prefix,
			Environment: key.Environment,
			CreatedAt:   key.CreatedAt.Time,
		},
	})
}

func generateCLISetupToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return "ust_" + base64.RawURLEncoding.EncodeToString(buf), nil
}

func normalizeCLIClient(client string) string {
	normalized := strings.ToLower(strings.TrimSpace(client))
	switch normalized {
	case "terminal", "codex", "claude-code", "cursor", "windsurf":
		return normalized
	default:
		return "codex"
	}
}

func cliSetupCommand(client, setupToken, apiBaseURL string) string {
	if client == "terminal" {
		return "unipost auth login --setup-token " + setupToken + " --client terminal --base-url " + apiBaseURL + " --json"
	}
	return "unipost agent bootstrap --client " + client + " --setup-token " + setupToken + " --base-url " + apiBaseURL + " --json"
}

func cliSetupRecommendedPrompt(client, apiBaseURL string) string {
	if client == "terminal" {
		return "Run the command once in your terminal, then run `unipost auth status --json` to confirm CLI auth."
	}
	return "Run the command once in the agent terminal, then rerun `unipost agent bootstrap --base-url " + apiBaseURL + " --json` for context."
}

func (h *CLISetupTokenHandler) cliSetupAPIBaseURL(r *http.Request) string {
	if h.apiBaseURL != "" {
		return h.apiBaseURL
	}
	if r == nil {
		return cliSetupDefaultAPIBaseURL
	}
	host := strings.TrimSpace(r.Host)
	if host == "" && r.URL != nil {
		host = strings.TrimSpace(r.URL.Host)
	}
	if host == "" {
		return cliSetupDefaultAPIBaseURL
	}
	scheme := cliSetupRequestScheme(r, host)
	return scheme + "://" + host
}

func cliSetupRequestScheme(r *http.Request, host string) string {
	for _, value := range strings.Split(r.Header.Get("X-Forwarded-Proto"), ",") {
		proto := strings.ToLower(strings.TrimSpace(value))
		if proto == "http" || proto == "https" {
			return proto
		}
	}
	if r.URL != nil {
		scheme := strings.ToLower(strings.TrimSpace(r.URL.Scheme))
		if scheme == "http" || scheme == "https" {
			return scheme
		}
	}
	if r.TLS != nil {
		return "https"
	}
	if strings.HasPrefix(host, "localhost") || strings.HasPrefix(host, "127.0.0.1") || strings.HasPrefix(host, "[::1]") {
		return "http"
	}
	return "https"
}

func normalizeCLISetupBaseURL(apiBaseURL string) string {
	baseURL := strings.TrimSpace(apiBaseURL)
	if baseURL == "" {
		return ""
	}
	return strings.TrimRight(baseURL, "/")
}

func cliSetupKeyName(client string) string {
	switch client {
	case "terminal":
		return "UniPost CLI"
	case "claude-code":
		return "Claude Code CLI"
	case "cursor":
		return "Cursor CLI"
	case "windsurf":
		return "Windsurf CLI"
	default:
		return "Codex CLI"
	}
}
