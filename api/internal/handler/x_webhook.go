package handler

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/xiaoboyu/unipost-api/internal/xinbox"
)

const (
	defaultXWebhookMaxBodyBytes int64 = 1 << 20
	defaultXWebhookMaxEventAge        = 7 * 24 * time.Hour
	xWebhookFutureTolerance           = 5 * time.Minute
	xWebhookProcessingTimeout         = 9 * time.Second
)

type XWebhookSecretResolver interface {
	ConsumerSecret(context.Context, string) (string, error)
}

type XWebhookIngestor interface {
	IngestActivityEvent(context.Context, string, xinbox.ActivityEvent) error
}

type XWebhookConfig struct {
	Secrets            XWebhookSecretResolver
	Ingestor           XWebhookIngestor
	ManagedAppClientID string
	MaxBodyBytes       int64
	MaxEventAge        time.Duration
	Now                func() time.Time
}

type XWebhookHandler struct {
	secrets            XWebhookSecretResolver
	ingestor           XWebhookIngestor
	managedAppClientID string
	maxBodyBytes       int64
	maxEventAge        time.Duration
	now                func() time.Time
}

func NewXWebhookHandler(config XWebhookConfig) *XWebhookHandler {
	maxBodyBytes := config.MaxBodyBytes
	if maxBodyBytes <= 0 {
		maxBodyBytes = defaultXWebhookMaxBodyBytes
	}
	maxEventAge := config.MaxEventAge
	if maxEventAge <= 0 {
		maxEventAge = defaultXWebhookMaxEventAge
	}
	now := config.Now
	if now == nil {
		now = time.Now
	}
	return &XWebhookHandler{
		secrets:            config.Secrets,
		ingestor:           config.Ingestor,
		managedAppClientID: strings.TrimSpace(config.ManagedAppClientID),
		maxBodyBytes:       maxBodyBytes,
		maxEventAge:        maxEventAge,
		now:                now,
	}
}

func (h *XWebhookHandler) CRC(w http.ResponseWriter, r *http.Request) {
	requestCtx, cancel := context.WithTimeout(r.Context(), xWebhookProcessingTimeout)
	defer cancel()
	appClientID := h.appClientID(r)
	if appClientID == "" {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "X webhook app was not found")
		return
	}
	crcToken := r.URL.Query().Get("crc_token")
	if crcToken == "" {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "crc_token is required")
		return
	}
	secret, err := h.resolveSecret(requestCtx, appClientID)
	if err != nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "X webhook app was not found")
		return
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(crcToken))
	writeJSON(w, http.StatusOK, map[string]string{
		"response_token": "sha256=" + base64.StdEncoding.EncodeToString(mac.Sum(nil)),
	})
}

func (h *XWebhookHandler) Handle(w http.ResponseWriter, r *http.Request) {
	requestCtx, cancel := context.WithTimeout(r.Context(), xWebhookProcessingTimeout)
	defer cancel()
	appClientID := h.appClientID(r)
	if appClientID == "" {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "X webhook app was not found")
		return
	}
	secret, err := h.resolveSecret(requestCtx, appClientID)
	if err != nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "X webhook app was not found")
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, h.maxBodyBytes+1))
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Failed to read webhook body")
		return
	}
	if int64(len(body)) > h.maxBodyBytes {
		writeError(w, http.StatusRequestEntityTooLarge, "PAYLOAD_TOO_LARGE", "Webhook body exceeds the allowed size")
		return
	}
	if !verifyXWebhookSignature(body, r.Header.Get("x-twitter-webhooks-signature"), secret) {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid webhook signature")
		return
	}

	// X does not sign a timestamp header. Signature verification must happen
	// over the raw bytes first; only then can we parse authenticated event
	// timestamps. Verified stale events are acknowledged and discarded so X
	// does not amplify them through retries. Receipt dedupe remains the
	// primary replay defense for events inside this generous age window.
	events, err := xinbox.ParseActivityEvents(body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid X webhook payload")
		return
	}
	now := h.now().UTC()
	for _, event := range events {
		if !event.CreatedAt.IsZero() &&
			(now.Sub(event.CreatedAt) > h.maxEventAge || event.CreatedAt.Sub(now) > xWebhookFutureTolerance) {
			continue
		}
		if h.ingestor != nil {
			if err := h.ingestor.IngestActivityEvent(requestCtx, appClientID, event); err != nil {
				if errors.Is(err, xinbox.ErrInboxAccountNotFound) {
					continue
				}
				writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to process X webhook")
				return
			}
		}
	}
	w.WriteHeader(http.StatusOK)
}

func (h *XWebhookHandler) appClientID(r *http.Request) string {
	if appClientID := strings.TrimSpace(chi.URLParam(r, "app_client_id")); appClientID != "" {
		return appClientID
	}
	// The legacy base path is intentionally limited to the one configured
	// UniPost-managed app. Workspace apps always require their app-specific
	// path, so an ambiguous or unconfigured base route fails closed.
	return h.managedAppClientID
}

func (h *XWebhookHandler) resolveSecret(ctx context.Context, appClientID string) (string, error) {
	if h == nil || h.secrets == nil {
		return "", xinbox.ErrAppSecretNotFound
	}
	secret, err := h.secrets.ConsumerSecret(ctx, appClientID)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(secret) == "" {
		return "", xinbox.ErrAppSecretNotFound
	}
	return secret, nil
}

func verifyXWebhookSignature(body []byte, signatureHeader, secret string) bool {
	if signatureHeader == "" || secret == "" {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(body)
	expected := "sha256=" + base64.StdEncoding.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(signatureHeader))
}
