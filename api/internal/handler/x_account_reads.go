package handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/auth"
	"github.com/xiaoboyu/unipost-api/internal/connect"
	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/xaccountreads"
	"github.com/xiaoboyu/unipost-api/internal/xinbox"
)

type xAccountReadCipher interface {
	Encrypt(string) (string, error)
	Decrypt(string) (string, error)
}

type xAccountReadService interface {
	ReadProfile(context.Context, xaccountreads.ProfileRequest) (xaccountreads.ProfileResult, error)
	ReadPosts(context.Context, xaccountreads.PostsRequest) (xaccountreads.PostsResult, error)
}

type XAccountReadsHandler struct {
	queries   *db.Queries
	cipher    xAccountReadCipher
	service   xAccountReadService
	refresher xinbox.TokenRefresher
}

func NewXAccountReadsHandler(
	queries *db.Queries,
	cipher xAccountReadCipher,
	service xAccountReadService,
	refresher xinbox.TokenRefresher,
) *XAccountReadsHandler {
	return &XAccountReadsHandler{queries: queries, cipher: cipher, service: service, refresher: refresher}
}

func (h *XAccountReadsHandler) Profile(w http.ResponseWriter, r *http.Request) {
	account, externalUserID, idempotencyKey, ok := h.authorize(w, r, "users.read")
	if !ok {
		return
	}
	result, err := h.service.ReadProfile(r.Context(), xaccountreads.ProfileRequest{
		WorkspaceID: auth.GetWorkspaceID(r.Context()), AccountID: account.ID,
		ExternalUserID: externalUserID, ExternalAccountID: account.ExternalAccountID,
		AppMode: account.XAppMode.String, IdempotencyKey: idempotencyKey,
		Now: time.Now().UTC(),
	})
	if err != nil {
		h.writeReadError(w, err)
		return
	}
	writeXAccountReadSuccess(w, result.Data, result.Meta)
}

func (h *XAccountReadsHandler) Posts(w http.ResponseWriter, r *http.Request) {
	limit, err := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("limit")))
	if err != nil || limit < 5 || limit > 100 {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "limit is required and must be between 5 and 100")
		return
	}
	excludeReposts, ok := parseOptionalBool(w, r, "exclude_reposts")
	if !ok {
		return
	}
	excludeReplies, ok := parseOptionalBool(w, r, "exclude_replies_to_others")
	if !ok {
		return
	}
	start, ok := parseOptionalRFC3339(w, r, "start_time")
	if !ok {
		return
	}
	end, ok := parseOptionalRFC3339(w, r, "end_time")
	if !ok {
		return
	}
	if start != nil && end != nil && !end.After(*start) {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "end_time must be after start_time")
		return
	}

	account, externalUserID, idempotencyKey, authorized := h.authorize(w, r, "users.read", "tweet.read")
	if !authorized {
		return
	}
	result, err := h.service.ReadPosts(r.Context(), xaccountreads.PostsRequest{
		WorkspaceID: auth.GetWorkspaceID(r.Context()), AccountID: account.ID,
		ExternalUserID: externalUserID, ExternalAccountID: account.ExternalAccountID,
		AppMode: account.XAppMode.String, IdempotencyKey: idempotencyKey,
		Limit: limit, Cursor: strings.TrimSpace(r.URL.Query().Get("cursor")),
		StartTime: start, EndTime: end, ExcludeReposts: excludeReposts,
		ExcludeRepliesToOthers: excludeReplies, Now: time.Now().UTC(),
	})
	if err != nil {
		h.writeReadError(w, err)
		return
	}
	writeXAccountReadSuccess(w, result.Data, result.Meta)
}

func (h *XAccountReadsHandler) authorize(
	w http.ResponseWriter,
	r *http.Request,
	requiredScopes ...string,
) (db.SocialAccount, string, string, bool) {
	workspaceID := auth.GetWorkspaceID(r.Context())
	if workspaceID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing workspace context")
		return db.SocialAccount{}, "", "", false
	}
	externalUserID := strings.TrimSpace(r.URL.Query().Get("external_user_id"))
	if externalUserID == "" {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "external_user_id is required")
		return db.SocialAccount{}, "", "", false
	}
	idempotencyKey := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if idempotencyKey == "" {
		writeError(w, http.StatusBadRequest, "IDEMPOTENCY_KEY_REQUIRED", "Idempotency-Key is required")
		return db.SocialAccount{}, "", "", false
	}
	accountID := chi.URLParam(r, "id")
	account, err := h.queries.GetSocialAccountByIDAndWorkspace(r.Context(), db.GetSocialAccountByIDAndWorkspaceParams{
		ID: accountID, WorkspaceID: workspaceID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "ACCOUNT_NOT_FOUND", "Account not found")
		} else {
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load account")
		}
		return db.SocialAccount{}, "", "", false
	}
	if !strings.EqualFold(account.Platform, "twitter") {
		writeError(w, http.StatusBadRequest, "WRONG_PLATFORM", "This endpoint is available only for X accounts")
		return db.SocialAccount{}, "", "", false
	}
	if !account.ExternalUserID.Valid || account.ExternalUserID.String != externalUserID {
		writeError(w, http.StatusForbidden, "ACCOUNT_ACCESS_DENIED", "The account does not belong to the selected Managed User")
		return db.SocialAccount{}, "", "", false
	}
	if account.DisconnectedAt.Valid || !strings.EqualFold(account.Status, "active") ||
		!account.RefreshToken.Valid || strings.TrimSpace(account.RefreshToken.String) == "" ||
		!accountHasScopes(account.Scope, append(requiredScopes, "offline.access")...) {
		writeError(w, http.StatusUnprocessableEntity, "ACCOUNT_REAUTHORIZATION_REQUIRED", "Reconnect the X account with the required read permissions")
		return db.SocialAccount{}, "", "", false
	}
	if _, err := xinbox.NormalizePersistedAppMode(account.XAppMode.String); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "ACCOUNT_REAUTHORIZATION_REQUIRED", "Reconnect the X account to establish its X app identity")
		return db.SocialAccount{}, "", "", false
	}
	return account, externalUserID, idempotencyKey, true
}

func (h *XAccountReadsHandler) ResolveAccountReadToken(ctx context.Context, accountID string) (string, error) {
	account, err := h.queries.GetSocialAccount(ctx, accountID)
	if err != nil {
		return "", err
	}
	if account.DisconnectedAt.Valid || !strings.EqualFold(account.Platform, "twitter") || !strings.EqualFold(account.Status, "active") {
		return "", errors.New("X account is not readable")
	}
	return h.tokenForAccount(ctx, account)
}

func (h *XAccountReadsHandler) tokenForAccount(ctx context.Context, account db.SocialAccount) (string, error) {
	accessToken, err := h.cipher.Decrypt(account.AccessToken)
	if err != nil || strings.TrimSpace(accessToken) == "" {
		return "", errors.New("decrypt X access token")
	}
	if !account.TokenExpiresAt.Valid || account.TokenExpiresAt.Time.After(time.Now().Add(time.Minute)) {
		return accessToken, nil
	}
	if h.refresher == nil || !account.RefreshToken.Valid {
		return "", errors.New("X refresh token is unavailable")
	}
	refreshToken, err := h.cipher.Decrypt(account.RefreshToken.String)
	if err != nil || refreshToken == "" {
		return "", errors.New("decrypt X refresh token")
	}
	tokens, err := h.refresher.Refresh(ctx, account, refreshToken)
	if err != nil || tokens == nil || tokens.AccessToken == "" {
		return "", errors.New("refresh X access token")
	}
	if err := h.persistRefreshedTokens(ctx, account, tokens); err != nil {
		return "", err
	}
	return tokens.AccessToken, nil
}

func (h *XAccountReadsHandler) persistRefreshedTokens(ctx context.Context, account db.SocialAccount, tokens *connect.TokenSet) error {
	accessToken, err := h.cipher.Encrypt(tokens.AccessToken)
	if err != nil {
		return err
	}
	refreshToken := account.RefreshToken
	if tokens.RefreshToken != "" {
		encrypted, err := h.cipher.Encrypt(tokens.RefreshToken)
		if err != nil {
			return err
		}
		refreshToken = pgtype.Text{String: encrypted, Valid: true}
	}
	return h.queries.UpdateSocialAccountTokens(ctx, db.UpdateSocialAccountTokensParams{
		ID: account.ID, AccessToken: accessToken, RefreshToken: refreshToken,
		TokenExpiresAt: pgtype.Timestamptz{Time: tokens.ExpiresAt.UTC(), Valid: !tokens.ExpiresAt.IsZero()},
	})
}

func (h *XAccountReadsHandler) writeReadError(w http.ResponseWriter, err error) {
	var operationErr *xaccountreads.OperationError
	if errors.As(err, &operationErr) {
		retriable := operationErr.Code == xaccountreads.CodeReadInProgress ||
			operationErr.Code == xaccountreads.CodeReadSettlementPending ||
			operationErr.Code == xaccountreads.CodeRateLimited
		if operationErr.RetryAfter > 0 {
			w.Header().Set("Retry-After", strconv.Itoa(int(operationErr.RetryAfter.Seconds())))
		}
		details := ErrorDetails{IsRetriable: &retriable}
		if operationErr.RetryCursor != "" {
			details.Details = map[string]any{
				"retry_cursor":            operationErr.RetryCursor,
				"retry_cursor_expires_at": operationErr.RetryCursorExpiresAt.UTC().Format(time.RFC3339),
			}
		}
		switch operationErr.Code {
		case xaccountreads.CodeInsufficientCredits:
			details.Details = map[string]any{
				"estimated_credits":    operationErr.EstimatedCredits,
				"available_credits":    operationErr.AvailableCredits,
				"max_affordable_limit": operationErr.MaxAffordableLimit,
			}
			writeErrorWithDetails(w, http.StatusPaymentRequired, operationErr.Code, "The Workspace does not have enough X Credits for this request", details)
		case xaccountreads.CodeIdempotencyConflict, xaccountreads.CodeReadInProgress, xaccountreads.CodeReadSettlementPending:
			writeErrorWithDetails(w, http.StatusConflict, operationErr.Code, "The X account read cannot start in the current idempotency state", details)
		case xaccountreads.CodeRateLimited:
			writeErrorWithDetails(w, http.StatusTooManyRequests, operationErr.Code, "X temporarily limited this account read", details)
		case xaccountreads.CodeReauthorization:
			writeErrorWithDetails(w, http.StatusUnprocessableEntity, operationErr.Code, "Reconnect the X account before retrying", details)
		default:
			writeErrorWithDetails(w, http.StatusBadGateway, xaccountreads.CodeXUpstream, "X could not complete the account read", details)
		}
		return
	}
	if strings.Contains(strings.ToLower(err.Error()), "cursor") {
		writeError(w, http.StatusBadRequest, "INVALID_CURSOR", "The pagination cursor is invalid or expired")
		return
	}
	writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to complete X account read")
}

func writeXAccountReadSuccess(w http.ResponseWriter, data, meta any) {
	writeJSON(w, http.StatusOK, struct {
		Data      any    `json:"data"`
		Meta      any    `json:"meta"`
		RequestID string `json:"request_id,omitempty"`
	}{Data: data, Meta: meta, RequestID: requestIDFromResponse(w)})
}

func parseOptionalBool(w http.ResponseWriter, r *http.Request, name string) (bool, bool) {
	raw := strings.TrimSpace(r.URL.Query().Get(name))
	if raw == "" {
		return false, true
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", name+" must be true or false")
		return false, false
	}
	return value, true
}

func parseOptionalRFC3339(w http.ResponseWriter, r *http.Request, name string) (*time.Time, bool) {
	raw := strings.TrimSpace(r.URL.Query().Get(name))
	if raw == "" {
		return nil, true
	}
	value, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", name+" must be an RFC 3339 timestamp")
		return nil, false
	}
	value = value.UTC()
	return &value, true
}

func accountHasScopes(granted []string, required ...string) bool {
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
