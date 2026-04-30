// facebook_webhook_admin.go is the diagnose-and-repair surface for
// the Facebook Page webhook subscription. Two endpoints, both auth'd
// the same way as the rest of /v1/accounts/{id}/* routes (workspace
// member can call them for accounts they own):
//
//	GET  /v1/accounts/{id}/facebook/webhook-status
//	     Reads /{page_id}/subscribed_apps from Meta and returns the
//	     full list along with a derived `subscribed: bool` saying
//	     whether OUR App is in there. This is the "is the connection
//	     working at the Meta-side level" check.
//
//	POST /v1/accounts/{id}/facebook/resubscribe-webhooks
//	     Re-runs the same SubscribePageToWebhooks call we make on
//	     OAuth finalize. The connect-time call swallows errors
//	     (logs only — see oauth_facebook.go's subscribePageToWebhooks),
//	     so a Page that silently fell off the subscription gets healed
//	     by this endpoint without forcing the user to disconnect and
//	     reconnect.
//
// Both endpoints are scoped to the workspace's Page access token, so
// they can never leak information about Pages the caller doesn't own.

package handler

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"

	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/platform"
)

type facebookWebhookStatusResponse struct {
	// PageID is echoed back so the caller can sanity-check we're
	// talking about the right Page when they have several connected.
	PageID string `json:"page_id"`
	// Subscribed reports whether OUR Meta App appears in the
	// /{page_id}/subscribed_apps list. False is the actionable case:
	// call POST resubscribe-webhooks to fix it.
	Subscribed bool `json:"subscribed"`
	// SubscribedFields is the list of webhook fields our App is
	// receiving for this Page (e.g. ["feed", "messages"]). Empty
	// when Subscribed is false. Useful to spot a partial subscription
	// where, say, "messages" got through but "feed" didn't.
	SubscribedFields []string `json:"subscribed_fields,omitempty"`
	// Apps is the raw list Meta returned, including names + categories.
	// Lets the caller see which OTHER apps are subscribed (typically
	// the Page admin's own dev apps) for context.
	Apps []platform.FacebookSubscribedApp `json:"apps"`
}

// FacebookWebhookStatus implements GET /v1/accounts/{id}/facebook/webhook-status.
func (h *SocialAccountHandler) FacebookWebhookStatus(w http.ResponseWriter, r *http.Request) {
	acc, fb, pageToken, ok := h.loadFacebookAccountAndToken(w, r)
	if !ok {
		return
	}

	apps, err := fb.FetchPageSubscribedApps(r.Context(), pageToken, acc.ExternalAccountID)
	if err != nil {
		slog.Warn("facebook webhook status: fetch subscribed_apps failed",
			"account_id", acc.ID, "page_id", acc.ExternalAccountID, "err", err)
		writeError(w, http.StatusBadGateway, "FACEBOOK_ERROR", err.Error())
		return
	}

	// Identify "our" app via the FACEBOOK_APP_ID env var (the same
	// one platform/facebook.go reads to construct OAuth URLs). When
	// it's unset we can't make a reliable match — fall back to "is
	// ANY app subscribed", which still tells the user whether their
	// Page has an active subscription at all.
	ourAppID := os.Getenv("FACEBOOK_APP_ID")
	subscribed := false
	subscribedFields := []string(nil)
	for _, app := range apps {
		if ourAppID != "" {
			if app.ID == ourAppID {
				subscribed = true
				subscribedFields = app.SubscribedFields
				break
			}
			continue
		}
		// FACEBOOK_APP_ID not configured — treat any subscribed app
		// as "subscribed" since we can't disambiguate.
		subscribed = true
		subscribedFields = app.SubscribedFields
	}

	writeSuccess(w, facebookWebhookStatusResponse{
		PageID:           acc.ExternalAccountID,
		Subscribed:       subscribed,
		SubscribedFields: subscribedFields,
		Apps:             apps,
	})
}

type facebookResubscribeResponse struct {
	PageID string `json:"page_id"`
	// Before is the subscribed_apps state we read just before
	// re-subscribing. Useful as evidence that the operation actually
	// changed something (or that nothing was wrong to begin with).
	Before []platform.FacebookSubscribedApp `json:"before"`
	// SubscribeError is the error string from SubscribePageToWebhooks
	// itself, populated only when the call failed. Empty on success.
	SubscribeError string `json:"subscribe_error,omitempty"`
	// After is the subscribed_apps state after we re-ran subscribe.
	// On success this should now include our App with the
	// {feed, messages, messaging_postbacks} field set.
	After []platform.FacebookSubscribedApp `json:"after"`
}

// FacebookResubscribeWebhooks implements
// POST /v1/accounts/{id}/facebook/resubscribe-webhooks.
//
// Calls SubscribePageToWebhooks against Meta with the Page access
// token; surrounds it with before/after reads of /{page_id}/subscribed_apps
// so the response is self-describing — the caller doesn't need a
// separate verify step. Returns 200 even when the subscribe call
// errored (the body's subscribe_error reports the failure) because
// the before/after diff is still useful diagnostic information.
func (h *SocialAccountHandler) FacebookResubscribeWebhooks(w http.ResponseWriter, r *http.Request) {
	acc, fb, pageToken, ok := h.loadFacebookAccountAndToken(w, r)
	if !ok {
		return
	}

	resp := facebookResubscribeResponse{PageID: acc.ExternalAccountID}

	// Best-effort before snapshot; if Meta errors here we still try
	// the subscribe call and return what we got.
	if before, err := fb.FetchPageSubscribedApps(r.Context(), pageToken, acc.ExternalAccountID); err == nil {
		resp.Before = before
	}

	if err := fb.SubscribePageToWebhooks(r.Context(), pageToken, acc.ExternalAccountID); err != nil {
		slog.Warn("facebook resubscribe webhooks: subscribe failed",
			"account_id", acc.ID, "page_id", acc.ExternalAccountID, "err", err)
		resp.SubscribeError = err.Error()
	} else {
		slog.Info("facebook resubscribe webhooks: subscribe ok",
			"account_id", acc.ID, "page_id", acc.ExternalAccountID)
	}

	if after, err := fb.FetchPageSubscribedApps(r.Context(), pageToken, acc.ExternalAccountID); err == nil {
		resp.After = after
	}

	writeSuccess(w, resp)
}

// loadFacebookAccountAndToken centralizes the workspace-ACL check,
// platform check, and Page-token decryption for both endpoints in
// this file. Writes the appropriate HTTP error on failure and returns
// ok=false so the caller can early-return.
func (h *SocialAccountHandler) loadFacebookAccountAndToken(
	w http.ResponseWriter, r *http.Request,
) (*db.SocialAccount, *platform.FacebookAdapter, string, bool) {
	accountID := chi.URLParam(r, "id")
	if accountID == "" {
		accountID = chi.URLParam(r, "accountID")
	}

	loaded, found := h.loadAccountForRequest(r, accountID)
	if !found {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Account not found")
		return nil, nil, "", false
	}
	if loaded.Platform != "facebook" {
		writeError(w, http.StatusConflict, "WRONG_PLATFORM", "Account is not a Facebook account")
		return nil, nil, "", false
	}

	adapter, err := platform.Get("facebook")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Facebook adapter unavailable")
		return nil, nil, "", false
	}
	fbAdapter, castOK := adapter.(*platform.FacebookAdapter)
	if !castOK {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Facebook adapter unavailable")
		return nil, nil, "", false
	}

	token, err := h.encryptor.Decrypt(loaded.AccessToken)
	if err != nil || token == "" {
		writeError(w, http.StatusConflict, "NEEDS_RECONNECT",
			"Page access token unavailable. Reconnect the Facebook Page.")
		return nil, nil, "", false
	}

	return loaded, fbAdapter, token, true
}
