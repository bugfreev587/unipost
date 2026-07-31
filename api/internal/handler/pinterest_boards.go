package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/platform"
)

type pinterestBoardsResponse struct {
	Boards      []platform.PinterestBoard `json:"boards"`
	SandboxMode bool                      `json:"sandbox_mode"`
}

type pinterestBoardResponse struct {
	Board platform.PinterestBoard `json:"board"`
}

type pinterestBoardEndpointError struct {
	status    int
	code      string
	message   string
	reconnect bool
}

type pinterestFailureContractCarrier interface {
	FailureContractFields() map[string]any
}

func classifyPinterestBoardEndpointError(err error) pinterestBoardEndpointError {
	failureCode := ""
	if carrier, ok := err.(pinterestFailureContractCarrier); ok {
		if value, ok := carrier.FailureContractFields()["error_code"].(string); ok {
			failureCode = strings.TrimSpace(value)
		}
	}
	switch failureCode {
	case "auth_token_invalid", "account_reconnect_required":
		return pinterestBoardEndpointError{
			status: http.StatusConflict, code: "NEEDS_RECONNECT", reconnect: true,
			message: "Pinterest rejected this connection. Reconnect the account, then try again.",
		}
	case "missing_permission":
		return pinterestBoardEndpointError{
			status: http.StatusConflict, code: "MISSING_PERMISSION",
			message: "Pinterest denied the board permissions required by this connection. Reconnect the account and update its permissions.",
		}
	case "target_not_found":
		return pinterestBoardEndpointError{
			status: http.StatusConflict, code: "PINTEREST_BOARD_UNAVAILABLE",
			message: "The selected Pinterest board is no longer available for this account. Choose another board and publish again.",
		}
	case "rate_limit", "temporary_platform_error":
		return pinterestBoardEndpointError{
			status: http.StatusServiceUnavailable, code: "PINTEREST_TEMPORARY",
			message: "Pinterest is temporarily unavailable. Try again later.",
		}
	}
	if looksLikePinterestAuthError(err) {
		return pinterestBoardEndpointError{
			status: http.StatusConflict, code: "NEEDS_RECONNECT", reconnect: true,
			message: "Pinterest rejected this connection. Reconnect the account, then try again.",
		}
	}
	return pinterestBoardEndpointError{
		status: http.StatusBadGateway, code: "PINTEREST_ERROR",
		message: "Pinterest could not complete the board request. Try again later or contact support if the problem continues.",
	}
}

func writePinterestBoardEndpointError(w http.ResponseWriter, err error) {
	classified := classifyPinterestBoardEndpointError(err)
	writeError(w, classified.status, classified.code, classified.message)
}

func (h *SocialAccountHandler) PinterestBoards(w http.ResponseWriter, r *http.Request) {
	accountID := chi.URLParam(r, "id")
	if accountID == "" {
		accountID = chi.URLParam(r, "accountID")
	}

	acc, ok := h.loadAccountForRequest(r, accountID)
	if !ok {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Account not found")
		return
	}
	if acc.Platform != "pinterest" {
		writeError(w, http.StatusConflict, "WRONG_PLATFORM", "Account is not a Pinterest account")
		return
	}

	adapter, err := platform.Get("pinterest")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Pinterest adapter unavailable")
		return
	}
	pinterestAdapter, ok := adapter.(*platform.PinterestAdapter)
	if !ok {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Pinterest adapter unavailable")
		return
	}

	accessToken, err := h.encryptor.Decrypt(acc.AccessToken)
	if err != nil {
		slog.Error("pinterest boards: decrypt access token failed", "account_id", acc.ID, "error", err)
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to decrypt access token")
		return
	}
	if accessToken == "" {
		writeError(w, http.StatusConflict, "NEEDS_RECONNECT", "Your Pinterest connection has expired. Please reconnect the account.")
		return
	}

	if acc.TokenExpiresAt.Valid && acc.TokenExpiresAt.Time.Before(time.Now()) && acc.RefreshToken.Valid {
		refreshTok, decErr := h.encryptor.Decrypt(acc.RefreshToken.String)
		if decErr != nil {
			slog.Warn("pinterest boards: decrypt refresh token failed", "account_id", acc.ID, "error", decErr)
			writeError(w, http.StatusConflict, "NEEDS_RECONNECT", "Your Pinterest connection has expired. Please reconnect the account.")
			return
		}
		newAccess, newRefresh, expiresAt, refErr := pinterestAdapter.RefreshToken(r.Context(), refreshTok)
		if refErr != nil || newAccess == "" {
			slog.Warn("pinterest boards: refresh failed", "account_id", acc.ID, "error", refErr)
			writeError(w, http.StatusConflict, "NEEDS_RECONNECT", "Your Pinterest connection has expired. Please reconnect the account.")
			return
		}
		encAccess, encErr := h.encryptor.Encrypt(newAccess)
		encRefresh, encErr2 := h.encryptor.Encrypt(newRefresh)
		if encErr != nil || encErr2 != nil {
			slog.Error("pinterest boards: encrypt refreshed tokens failed", "account_id", acc.ID, "access_err", encErr, "refresh_err", encErr2)
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to persist refreshed tokens")
			return
		}
		accessToken = newAccess
		if updateErr := h.queries.UpdateSocialAccountTokens(r.Context(), db.UpdateSocialAccountTokensParams{
			ID:             acc.ID,
			AccessToken:    encAccess,
			RefreshToken:   pgtype.Text{String: encRefresh, Valid: true},
			TokenExpiresAt: pgtype.Timestamptz{Time: expiresAt, Valid: true},
		}); updateErr != nil {
			slog.Error("pinterest boards: update tokens failed", "account_id", acc.ID, "error", updateErr)
		}
	}

	boardCtx := platform.WithDispatchMetadata(r.Context(), platform.DispatchMetadata{
		SocialAccountID: acc.ID,
		Environment:     platform.PinterestEnvironment(),
	})
	boards, err := pinterestAdapter.FetchBoards(boardCtx, accessToken)
	if err != nil {
		slog.Warn("pinterest boards: upstream fetch failed", "account_id", acc.ID, "error", err)
		writePinterestBoardEndpointError(w, err)
		return
	}

	writeSuccess(w, pinterestBoardsResponse{
		Boards:      boards,
		SandboxMode: platform.PinterestUsesSandbox(),
	})
}

func (h *SocialAccountHandler) CreatePinterestBoard(w http.ResponseWriter, r *http.Request) {
	accountID := chi.URLParam(r, "id")
	if accountID == "" {
		accountID = chi.URLParam(r, "accountID")
	}

	acc, ok := h.loadAccountForRequest(r, accountID)
	if !ok {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Account not found")
		return
	}
	if acc.Platform != "pinterest" {
		writeError(w, http.StatusConflict, "WRONG_PLATFORM", "Account is not a Pinterest account")
		return
	}

	var body struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Invalid request body")
		return
	}
	body.Name = strings.TrimSpace(body.Name)
	if body.Name == "" {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Board name is required")
		return
	}

	adapter, err := platform.Get("pinterest")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Pinterest adapter unavailable")
		return
	}
	pinterestAdapter, ok := adapter.(*platform.PinterestAdapter)
	if !ok {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Pinterest adapter unavailable")
		return
	}

	accessToken, err := h.encryptor.Decrypt(acc.AccessToken)
	if err != nil {
		slog.Error("pinterest create board: decrypt access token failed", "account_id", acc.ID, "error", err)
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to decrypt access token")
		return
	}
	if accessToken == "" {
		writeError(w, http.StatusConflict, "NEEDS_RECONNECT", "Your Pinterest connection has expired. Please reconnect the account.")
		return
	}

	board, err := pinterestAdapter.CreateBoard(r.Context(), accessToken, body.Name)
	if err != nil {
		slog.Warn("pinterest create board: upstream create failed", "account_id", acc.ID, "error", err)
		writePinterestBoardEndpointError(w, err)
		return
	}
	pinterestAdapter.InvalidateBoardCache(acc.ID)

	writeCreated(w, pinterestBoardResponse{Board: *board})
}

func looksLikePinterestAuthError(err error) bool {
	if err == nil {
		return false
	}
	if carrier, ok := err.(pinterestFailureContractCarrier); ok {
		if value, ok := carrier.FailureContractFields()["error_code"].(string); ok {
			switch strings.TrimSpace(value) {
			case "auth_token_invalid", "account_reconnect_required":
				return true
			case "":
			default:
				return false
			}
		}
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unauthorized") ||
		strings.Contains(msg, "(401)") ||
		strings.Contains(msg, "invalid_token") ||
		strings.Contains(msg, "invalid access token") ||
		strings.Contains(msg, "expired")
}
