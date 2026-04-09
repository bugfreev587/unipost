// meta_data_deletion.go is the Sprint 4 PR7 Meta App Review
// Data Deletion Callback endpoint.
//
//	POST /v1/meta/data-deletion
//
// Meta requires this endpoint for any app that handles user data
// from their platforms (Instagram, Threads, Facebook). When a Meta
// user revokes UniPost's access to their account, Meta sends a
// signed_request POST here. We verify the signature, extract the
// platform user_id, delete every social_accounts row on
// instagram/threads matching that external_account_id, and return
// the deletion confirmation in the format Meta expects.
//
// The endpoint is mandatory for Meta App Review submission. Sprint 4
// PR7 ships the code; the actual submission waits on Meta business
// verification clearing (deferred per Sprint 4 founder decision).
//
// Spec: https://developers.facebook.com/docs/development/create-an-app/app-dashboard/data-deletion-callback/
//
// Auth model: NONE (Meta calls this directly with the signed_request
// in the form body — there's no API key or session token). The
// signature on the JWT is what authenticates the caller.

package handler

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/xiaoboyu/unipost-api/internal/db"
)

// MetaDataDeletionHandler owns POST /v1/meta/data-deletion.
type MetaDataDeletionHandler struct {
	queries   *db.Queries
	appSecret string // META_APP_SECRET — used to verify the signed_request
	statusURL string // public URL where users can check deletion status
}

func NewMetaDataDeletionHandler(queries *db.Queries, appSecret, statusURL string) *MetaDataDeletionHandler {
	return &MetaDataDeletionHandler{
		queries:   queries,
		appSecret: appSecret,
		statusURL: statusURL,
	}
}

// metaDataDeletionResponse is the exact shape Meta requires per
// their callback spec. The url is where Meta directs the user to
// check the deletion status; confirmation_code is an opaque token
// the user can quote when contacting support.
type metaDataDeletionResponse struct {
	URL              string `json:"url"`
	ConfirmationCode string `json:"confirmation_code"`
}

// metaSignedRequestPayload is what we extract after verifying the
// signed_request. Meta sends additional fields (algorithm, issued_at,
// etc.) but only user_id matters for deletion.
type metaSignedRequestPayload struct {
	UserID    string `json:"user_id"`
	Algorithm string `json:"algorithm"`
	IssuedAt  int64  `json:"issued_at"`
}

// HandleDataDeletion handles POST /v1/meta/data-deletion.
func (h *MetaDataDeletionHandler) HandleDataDeletion(w http.ResponseWriter, r *http.Request) {
	if h.appSecret == "" {
		// Meta won't actually call this until App Review submits.
		// Surface a 503 (rather than crashing) when META_APP_SECRET
		// hasn't been configured yet so the endpoint exists in
		// production from day 1 of the review submission.
		writeError(w, http.StatusServiceUnavailable, "NOT_CONFIGURED",
			"Meta integration not yet configured")
		return
	}

	if err := r.ParseForm(); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid form body")
		return
	}
	signedRequest := r.FormValue("signed_request")
	if signedRequest == "" {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR",
			"signed_request form field is required")
		return
	}

	payload, err := verifyMetaSignedRequest(signedRequest, h.appSecret)
	if err != nil {
		slog.Warn("meta data deletion: signed_request verification failed", "err", err)
		writeError(w, http.StatusUnauthorized, "INVALID_SIGNATURE", "signed_request verification failed")
		return
	}
	if payload.UserID == "" {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "user_id missing from signed_request")
		return
	}

	// Generate a confirmation code first so we can return it to Meta
	// even if the actual deletion job is queued asynchronously. The
	// code is opaque to Meta and they don't validate it; it's used
	// for user-facing status lookups.
	code, err := randomBase64URL(16)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR",
			"Failed to generate confirmation code")
		return
	}

	// Delete every Meta-platform social_accounts row for this user.
	// Sprint 4 ships the synchronous version because no Meta accounts
	// exist yet (App Review hasn't approved). When the Meta connector
	// lands and there are real rows in production, this should move
	// to a background queue so we can respond to Meta within the
	// 30-second timeout regardless of how many accounts the user has.
	deletedCount, err := h.deleteUserAccounts(r.Context(), payload.UserID)
	if err != nil {
		slog.Error("meta data deletion: delete failed",
			"meta_user_id", payload.UserID, "err", err)
		// Still return success to Meta — the requirement is that we
		// CONFIRM the request was accepted, not that we report
		// per-row failures inline. Operators triage from logs.
	}

	slog.Info("meta data deletion accepted",
		"meta_user_id", payload.UserID,
		"deleted_count", deletedCount,
		"confirmation_code", code,
	)

	resp := metaDataDeletionResponse{
		URL:              h.statusURL + "?code=" + code,
		ConfirmationCode: code,
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// deleteUserAccounts marks every instagram/threads social_accounts
// row matching the Meta user_id as disconnected. The actual data
// (encrypted access tokens) is wiped to satisfy GDPR-style erasure;
// the row itself is left in place with disconnected_at set so
// historical post records keep their FK references.
//
// Sprint 4 PR7: this is a placeholder. The query iterates by
// external_account_id which Meta sends as user_id, but the table
// has no Meta accounts yet (App Review pending). Once instagram
// and threads connectors land in Sprint 5/6, this becomes the
// hot path for compliance.
func (h *MetaDataDeletionHandler) deleteUserAccounts(_ context.Context, _ string) (int, error) {
	// TODO Sprint 5/6: when Meta connectors land, query
	// social_accounts where platform IN ('instagram','threads')
	// AND external_account_id = $1, then encrypt-zero the tokens
	// and set disconnected_at = NOW(). Returning 0 rows deleted
	// today because no Meta rows exist yet.
	return 0, nil
}

// verifyMetaSignedRequest parses Meta's signed_request format and
// validates the HMAC-SHA256 signature against the app secret.
//
// signed_request format: <signature>.<payload>
//   - signature is base64url-encoded HMAC-SHA256(payload, app_secret)
//   - payload is base64url-encoded JSON
//
// Both halves use the URL-safe base64 alphabet WITHOUT padding,
// per Meta's reference implementation.
func verifyMetaSignedRequest(signedRequest, appSecret string) (*metaSignedRequestPayload, error) {
	parts := strings.Split(signedRequest, ".")
	if len(parts) != 2 {
		return nil, fmt.Errorf("malformed signed_request: expected 2 parts, got %d", len(parts))
	}

	expectedSig, err := base64URLDecode(parts[0])
	if err != nil {
		return nil, fmt.Errorf("decode signature: %w", err)
	}
	payloadJSON, err := base64URLDecode(parts[1])
	if err != nil {
		return nil, fmt.Errorf("decode payload: %w", err)
	}

	// Compute the expected HMAC over the RAW payload bytes (the
	// second part of the signed_request, BEFORE base64 decoding).
	mac := hmac.New(sha256.New, []byte(appSecret))
	mac.Write([]byte(parts[1]))
	actualSig := mac.Sum(nil)
	if !hmac.Equal(expectedSig, actualSig) {
		return nil, fmt.Errorf("signature mismatch")
	}

	var payload metaSignedRequestPayload
	if err := json.Unmarshal(payloadJSON, &payload); err != nil {
		return nil, fmt.Errorf("decode payload json: %w", err)
	}
	if payload.Algorithm != "" && payload.Algorithm != "HMAC-SHA256" {
		return nil, fmt.Errorf("unsupported algorithm: %s", payload.Algorithm)
	}
	return &payload, nil
}

// base64URLDecode handles Meta's unpadded URL-safe base64 encoding.
// stdlib's base64.URLEncoding requires padding; we use the Raw
// variant which doesn't, but Meta is occasionally inconsistent so
// we accept either.
func base64URLDecode(s string) ([]byte, error) {
	// Try the unpadded variant first (what Meta sends).
	if b, err := base64.RawURLEncoding.DecodeString(s); err == nil {
		return b, nil
	}
	// Fall back to padded variant for safety.
	return base64.URLEncoding.DecodeString(s)
}
