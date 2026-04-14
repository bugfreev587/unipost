// meta_webhook.go handles inbound Meta platform webhooks for
// Instagram and Threads.
//
//	GET  /webhooks/meta   — subscription verification handshake
//	POST /webhooks/meta   — event delivery
//
// Meta sends the same webhook format for both Instagram and Threads
// because both products live under a single Meta App. The "object"
// field in the payload distinguishes Instagram ("instagram") from
// Threads (currently undocumented but expected to be "threads" or
// handled via the same app subscription).
//
// Verification handshake (GET):
//   Meta sends hub.mode=subscribe, hub.challenge=<string>, and
//   hub.verify_token=<token>. We check the verify token against
//   META_WEBHOOK_VERIFY_TOKEN and echo back the challenge as
//   plain text with 200 OK. Any mismatch → 403.
//
// Event delivery (POST):
//   Meta signs the raw body with HMAC-SHA256 using the app secret
//   and sends the signature in X-Hub-Signature-256 as "sha256=<hex>".
//   We verify before processing.
//
// Auth model: NONE (Meta calls this directly). Signature verification
// is the authentication mechanism.

package handler

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
)

// MetaWebhookHandler owns GET/POST /webhooks/meta.
type MetaWebhookHandler struct {
	appSecret   string // META_APP_SECRET — HMAC-SHA256 key for signature verification
	verifyToken string // META_WEBHOOK_VERIFY_TOKEN — shared secret for the subscribe handshake
}

func NewMetaWebhookHandler(appSecret, verifyToken string) *MetaWebhookHandler {
	return &MetaWebhookHandler{
		appSecret:   appSecret,
		verifyToken: verifyToken,
	}
}

// Verify handles the GET subscription verification handshake.
func (h *MetaWebhookHandler) Verify(w http.ResponseWriter, r *http.Request) {
	mode := r.URL.Query().Get("hub.mode")
	token := r.URL.Query().Get("hub.verify_token")
	challenge := r.URL.Query().Get("hub.challenge")

	if mode != "subscribe" {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "hub.mode must be 'subscribe'")
		return
	}
	if h.verifyToken == "" {
		writeError(w, http.StatusServiceUnavailable, "NOT_CONFIGURED",
			"META_WEBHOOK_VERIFY_TOKEN not configured")
		return
	}
	if token != h.verifyToken {
		slog.Warn("meta webhook verify: token mismatch")
		writeError(w, http.StatusForbidden, "FORBIDDEN", "verify_token mismatch")
		return
	}

	// Meta expects the challenge echoed back as plain text, not JSON.
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, challenge)
}

// Handle handles POST event delivery.
func (h *MetaWebhookHandler) Handle(w http.ResponseWriter, r *http.Request) {
	if h.appSecret == "" {
		writeError(w, http.StatusServiceUnavailable, "NOT_CONFIGURED",
			"META_APP_SECRET not configured")
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Failed to read body")
		return
	}

	// Verify X-Hub-Signature-256 header.
	sigHeader := r.Header.Get("X-Hub-Signature-256")
	if !verifyMetaWebhookSignature(body, sigHeader, h.appSecret) {
		slog.Warn("meta webhook: signature verification failed")
		writeError(w, http.StatusUnauthorized, "INVALID_SIGNATURE", "Signature verification failed")
		return
	}

	// Parse the top-level envelope to log what we received.
	var envelope struct {
		Object string            `json:"object"`
		Entry  []json.RawMessage `json:"entry"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid JSON payload")
		return
	}

	slog.Info("meta webhook received",
		"object", envelope.Object,
		"entries", len(envelope.Entry),
	)

	// TODO: route events to domain handlers based on envelope.Object
	// ("instagram", "page", etc.) and the specific change fields.
	// For now we acknowledge receipt — Meta retries on non-2xx so
	// returning 200 prevents retry storms while we build out
	// event processing.

	w.WriteHeader(http.StatusOK)
}

// verifyMetaWebhookSignature checks the X-Hub-Signature-256 header
// against an HMAC-SHA256 of the raw body using the app secret.
// The header format is "sha256=<hex>".
func verifyMetaWebhookSignature(body []byte, sigHeader, appSecret string) bool {
	if sigHeader == "" {
		return false
	}
	parts := strings.SplitN(sigHeader, "=", 2)
	if len(parts) != 2 || parts[0] != "sha256" {
		return false
	}
	expectedSig, err := hex.DecodeString(parts[1])
	if err != nil {
		return false
	}

	mac := hmac.New(sha256.New, []byte(appSecret))
	mac.Write(body)
	actualSig := mac.Sum(nil)

	return hmac.Equal(expectedSig, actualSig)
}
