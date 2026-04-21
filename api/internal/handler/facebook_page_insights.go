// facebook_page_insights.go fronts the Graph API's Page Insights
// endpoint for the analytics dashboard. One HTTP route, workspace-
// scoped, ACL-checked via the same loadAccountForRequest helper
// TikTok's creator_info route uses.
//
// The Graph API gates Page Insights behind a 100-like threshold —
// below that, every metric returns zero. FacebookAdapter's
// GetPageInsights translates Meta's 400-with-hint into a
// Below100LikesNotice flag; we surface that verbatim so the
// dashboard can render a friendly "keep growing!" state instead
// of an error.

package handler

import (
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/xiaoboyu/unipost-api/internal/platform"
)

type facebookPageInsightsResponse struct {
	Follows             int64  `json:"follows"`
	Impressions         int64  `json:"impressions"`
	PostEngagements     int64  `json:"post_engagements"`
	Below100LikesNotice bool   `json:"below_100_likes_notice"`
	Since               string `json:"since"`
	Until               string `json:"until"`
}

// FacebookPageInsights handles
//
//	GET .../social-accounts/{id}/facebook/page-insights?days=28
//
// `days` is clamped to [1, 92] (FB's documented max window).
// Returns workspace-scoped 404 when the caller doesn't own the
// account, 409 when the account isn't a Facebook Page, and a
// 200-ok body with zeros + below_100_likes_notice=true when FB
// refused the query due to the like threshold.
func (h *SocialAccountHandler) FacebookPageInsights(w http.ResponseWriter, r *http.Request) {
	accountID := chi.URLParam(r, "id")
	if accountID == "" {
		accountID = chi.URLParam(r, "accountID")
	}

	acc, ok := h.loadAccountForRequest(r, accountID)
	if !ok {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Account not found")
		return
	}
	if acc.Platform != "facebook" {
		writeError(w, http.StatusConflict, "WRONG_PLATFORM", "Account is not a Facebook Page")
		return
	}

	days := 28
	if d := r.URL.Query().Get("days"); d != "" {
		if parsed, err := strconv.Atoi(d); err == nil {
			days = parsed
		}
	}
	if days < 1 {
		days = 1
	}
	if days > 92 {
		days = 92
	}

	adapter, err := platform.Get("facebook")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Facebook adapter unavailable")
		return
	}
	fb, ok := adapter.(*platform.FacebookAdapter)
	if !ok {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Facebook adapter unavailable")
		return
	}
	accessToken, err := h.encryptor.Decrypt(acc.AccessToken)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to decrypt access token")
		return
	}

	until := time.Now().UTC()
	since := until.Add(-time.Duration(days) * 24 * time.Hour)

	stats, err := fb.GetPageInsights(r.Context(), accessToken, acc.ExternalAccountID, since, until)
	if err != nil {
		slog.Warn("facebook page insights: upstream failed",
			"account_id", acc.ID, "error", err)
		writeError(w, http.StatusBadGateway, "UPSTREAM_ERROR", err.Error())
		return
	}

	writeSuccess(w, facebookPageInsightsResponse{
		Follows:             stats.Follows,
		Impressions:         stats.Impressions,
		PostEngagements:     stats.PostEngagements,
		Below100LikesNotice: stats.Below100LikesNotice,
		Since:               since.Format(time.RFC3339),
		Until:               until.Format(time.RFC3339),
	})
}
