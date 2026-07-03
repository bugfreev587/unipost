package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/platform"
)

const youtubeAnalyticsReadonlyScope = "https://www.googleapis.com/auth/yt-analytics.readonly"

var youtubeAnalyticsRequiredScopes = []string{youtubeAnalyticsReadonlyScope}

type youtubeAnalyticsRange struct {
	Start     time.Time
	End       time.Time
	StartDate string `json:"start_date"`
	EndDate   string `json:"end_date"`
}

type youtubeAnalyticsSummaryResponse struct {
	SocialAccountID string                           `json:"social_account_id"`
	Platform        string                           `json:"platform"`
	StartDate       string                           `json:"start_date"`
	EndDate         string                           `json:"end_date"`
	Metrics         platform.YouTubeAnalyticsMetrics `json:"metrics"`
	FetchedAt       time.Time                        `json:"fetched_at"`
	RequiredScopes  []string                         `json:"required_scopes"`
	GrantedScopes   []string                         `json:"granted_scopes,omitempty"`
}

type youtubeAnalyticsTrendResponse struct {
	SocialAccountID string                              `json:"social_account_id"`
	Platform        string                              `json:"platform"`
	StartDate       string                              `json:"start_date"`
	EndDate         string                              `json:"end_date"`
	Rows            []platform.YouTubeAnalyticsTrendRow `json:"rows"`
	FetchedAt       time.Time                           `json:"fetched_at"`
	RequiredScopes  []string                            `json:"required_scopes"`
	GrantedScopes   []string                            `json:"granted_scopes,omitempty"`
}

type youtubeAnalyticsVideosResponse struct {
	SocialAccountID string                              `json:"social_account_id"`
	Platform        string                              `json:"platform"`
	StartDate       string                              `json:"start_date"`
	EndDate         string                              `json:"end_date"`
	Videos          []platform.YouTubeAnalyticsVideoRow `json:"videos"`
	Limit           int                                 `json:"limit"`
	FetchedAt       time.Time                           `json:"fetched_at"`
	RequiredScopes  []string                            `json:"required_scopes"`
	GrantedScopes   []string                            `json:"granted_scopes,omitempty"`
}

func (h *SocialAccountHandler) YouTubeAnalyticsSummary(w http.ResponseWriter, r *http.Request) {
	acc, yt, accessToken, ok := h.loadYouTubeForAnalytics(w, r)
	if !ok {
		return
	}
	reportRange, ok := h.youtubeAnalyticsRangeFromRequest(w, r)
	if !ok {
		return
	}

	summary, err := yt.GetYouTubeAnalyticsSummary(r.Context(), accessToken, acc.ExternalAccountID, reportRange.Start, reportRange.End)
	if err != nil {
		slog.Warn("youtube analytics summary: upstream failed", "account_id", acc.ID, "error", err)
		writeYouTubeAnalyticsError(w, err)
		return
	}
	writeSuccess(w, youtubeAnalyticsSummaryResponse{
		SocialAccountID: acc.ID,
		Platform:        acc.Platform,
		StartDate:       reportRange.StartDate,
		EndDate:         reportRange.EndDate,
		Metrics:         summary.Metrics,
		FetchedAt:       time.Now().UTC(),
		RequiredScopes:  youtubeAnalyticsRequiredScopes,
		GrantedScopes:   acc.Scope,
	})
}

func (h *SocialAccountHandler) YouTubeAnalyticsTrend(w http.ResponseWriter, r *http.Request) {
	acc, yt, accessToken, ok := h.loadYouTubeForAnalytics(w, r)
	if !ok {
		return
	}
	reportRange, ok := h.youtubeAnalyticsRangeFromRequest(w, r)
	if !ok {
		return
	}

	rows, err := yt.GetYouTubeAnalyticsTrend(r.Context(), accessToken, acc.ExternalAccountID, reportRange.Start, reportRange.End)
	if err != nil {
		slog.Warn("youtube analytics trend: upstream failed", "account_id", acc.ID, "error", err)
		writeYouTubeAnalyticsError(w, err)
		return
	}
	writeSuccess(w, youtubeAnalyticsTrendResponse{
		SocialAccountID: acc.ID,
		Platform:        acc.Platform,
		StartDate:       reportRange.StartDate,
		EndDate:         reportRange.EndDate,
		Rows:            rows,
		FetchedAt:       time.Now().UTC(),
		RequiredScopes:  youtubeAnalyticsRequiredScopes,
		GrantedScopes:   acc.Scope,
	})
}

func (h *SocialAccountHandler) YouTubeAnalyticsVideos(w http.ResponseWriter, r *http.Request) {
	acc, yt, accessToken, ok := h.loadYouTubeForAnalytics(w, r)
	if !ok {
		return
	}
	reportRange, ok := h.youtubeAnalyticsRangeFromRequest(w, r)
	if !ok {
		return
	}
	limit := parseClampedInt(r.URL.Query().Get("limit"), 25, 1, 200)

	videos, err := yt.GetYouTubeAnalyticsVideos(r.Context(), accessToken, acc.ExternalAccountID, reportRange.Start, reportRange.End, limit)
	if err != nil {
		slog.Warn("youtube analytics videos: upstream failed", "account_id", acc.ID, "error", err)
		writeYouTubeAnalyticsError(w, err)
		return
	}
	writeSuccess(w, youtubeAnalyticsVideosResponse{
		SocialAccountID: acc.ID,
		Platform:        acc.Platform,
		StartDate:       reportRange.StartDate,
		EndDate:         reportRange.EndDate,
		Videos:          videos,
		Limit:           limit,
		FetchedAt:       time.Now().UTC(),
		RequiredScopes:  youtubeAnalyticsRequiredScopes,
		GrantedScopes:   acc.Scope,
	})
}

func (h *SocialAccountHandler) youtubeAnalyticsRangeFromRequest(w http.ResponseWriter, r *http.Request) (youtubeAnalyticsRange, bool) {
	reportRange, err := parseYouTubeAnalyticsRange(r.URL.Query(), time.Now().UTC())
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", err.Error())
		return youtubeAnalyticsRange{}, false
	}
	return reportRange, true
}

func (h *SocialAccountHandler) loadYouTubeForAnalytics(w http.ResponseWriter, r *http.Request) (*db.SocialAccount, *platform.YouTubeAdapter, string, bool) {
	accountID := accountIDFromRequest(r)
	acc, ok := h.loadAccountForRequest(r, accountID)
	if !ok {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Account not found")
		return nil, nil, "", false
	}
	if acc.Platform != "youtube" {
		writeError(w, http.StatusConflict, "WRONG_PLATFORM", "Account is not a YouTube account")
		return nil, nil, "", false
	}
	if acc.DisconnectedAt.Valid {
		writeError(w, http.StatusConflict, "ACCOUNT_DISCONNECTED", "Account is disconnected — reconnect before fetching analytics")
		return nil, nil, "", false
	}
	if !youtubeAnalyticsHasRequiredScope(acc.Scope) {
		writeError(w, http.StatusConflict, "NEEDS_RECONNECT", "Reconnect YouTube to enable analytics.")
		return nil, nil, "", false
	}

	adapter, err := platform.Get("youtube")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "YouTube adapter unavailable")
		return nil, nil, "", false
	}
	yt, ok := adapter.(*platform.YouTubeAdapter)
	if !ok {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "YouTube adapter unavailable")
		return nil, nil, "", false
	}

	accessToken, err := h.encryptor.Decrypt(acc.AccessToken)
	if err != nil || strings.TrimSpace(accessToken) == "" {
		slog.Warn("youtube analytics: decrypt access token failed", "account_id", acc.ID, "error", err)
		writeError(w, http.StatusConflict, "NEEDS_RECONNECT", "Reconnect YouTube to enable analytics.")
		return nil, nil, "", false
	}

	if acc.TokenExpiresAt.Valid && acc.TokenExpiresAt.Time.Before(time.Now()) && acc.RefreshToken.Valid && acc.RefreshToken.String != "" {
		accessToken = h.refreshYouTubeAnalyticsAccessToken(r, acc, adapter, accessToken)
	}
	return acc, yt, accessToken, true
}

func (h *SocialAccountHandler) refreshYouTubeAnalyticsAccessToken(r *http.Request, acc *db.SocialAccount, adapter platform.PlatformAdapter, currentAccessToken string) string {
	refreshTok, decErr := h.encryptor.Decrypt(acc.RefreshToken.String)
	if decErr != nil {
		slog.Warn("youtube analytics: decrypt refresh token failed", "account_id", acc.ID, "error", decErr)
		return currentAccessToken
	}
	newAccess, newRefresh, expiresAt, refErr := adapter.RefreshToken(r.Context(), refreshTok)
	if refErr != nil || newAccess == "" {
		slog.Warn("youtube analytics: token refresh failed", "account_id", acc.ID, "error", refErr)
		return currentAccessToken
	}
	encAccess, encErr := h.encryptor.Encrypt(newAccess)
	var encRefresh string
	var encRefreshErr error
	if newRefresh != "" {
		encRefresh, encRefreshErr = h.encryptor.Encrypt(newRefresh)
	}
	if encErr != nil || encRefreshErr != nil {
		slog.Error("youtube analytics: encrypt refreshed tokens failed", "account_id", acc.ID, "access_err", encErr, "refresh_err", encRefreshErr)
		return currentAccessToken
	}
	if updateErr := h.queries.UpdateSocialAccountTokens(r.Context(), db.UpdateSocialAccountTokensParams{
		ID:             acc.ID,
		AccessToken:    encAccess,
		RefreshToken:   accountMetricsRefreshTokenForUpdate(acc.RefreshToken, encRefresh),
		TokenExpiresAt: pgtype.Timestamptz{Time: expiresAt, Valid: !expiresAt.IsZero()},
	}); updateErr != nil {
		slog.Error("youtube analytics: update refreshed tokens failed", "account_id", acc.ID, "error", updateErr)
	}
	return newAccess
}

func youtubeAnalyticsHasRequiredScope(scopes []string) bool {
	for _, scope := range scopes {
		if strings.TrimSpace(scope) == youtubeAnalyticsReadonlyScope {
			return true
		}
	}
	return false
}

func parseYouTubeAnalyticsRange(values url.Values, now time.Time) (youtubeAnalyticsRange, error) {
	now = now.UTC()
	end := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC).AddDate(0, 0, -1)
	start := end.AddDate(0, 0, -27)

	startRaw := firstYouTubeAnalyticsQueryValue(values, "start_date", "startDate", "from")
	endRaw := firstYouTubeAnalyticsQueryValue(values, "end_date", "endDate", "to")
	var err error
	if startRaw != "" {
		start, err = parseYouTubeAnalyticsDate(startRaw, "start_date")
		if err != nil {
			return youtubeAnalyticsRange{}, err
		}
	}
	if endRaw != "" {
		end, err = parseYouTubeAnalyticsDate(endRaw, "end_date")
		if err != nil {
			return youtubeAnalyticsRange{}, err
		}
	}
	if start.After(end) {
		return youtubeAnalyticsRange{}, errors.New("start_date must be on or before end_date")
	}
	return youtubeAnalyticsRange{
		Start:     start,
		End:       end,
		StartDate: start.Format("2006-01-02"),
		EndDate:   end.Format("2006-01-02"),
	}, nil
}

func parseYouTubeAnalyticsDate(raw, field string) (time.Time, error) {
	parsed, err := time.Parse("2006-01-02", strings.TrimSpace(raw))
	if err != nil {
		return time.Time{}, errors.New(field + " must be YYYY-MM-DD")
	}
	return parsed, nil
}

func firstYouTubeAnalyticsQueryValue(values url.Values, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(values.Get(key)); value != "" {
			return value
		}
	}
	return ""
}

func writeYouTubeAnalyticsError(w http.ResponseWriter, err error) {
	if status, code, message, ok := youtubeAnalyticsErrorResponse(err); ok {
		writeError(w, status, code, message)
		return
	}
	writeError(w, http.StatusBadGateway, "UPSTREAM_ERROR", "Failed to fetch YouTube analytics.")
}

func youtubeAnalyticsErrorResponse(err error) (int, string, string, bool) {
	if err == nil {
		return 0, "", "", false
	}
	if errors.Is(err, platform.ErrNeedsReconnect) || errors.Is(err, platform.ErrYouTubeNoChannel) {
		return http.StatusConflict, "NEEDS_RECONNECT", "Reconnect YouTube to enable analytics.", true
	}
	return 0, "", "", false
}
