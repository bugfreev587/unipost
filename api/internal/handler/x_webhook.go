package handler

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
	"net"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/xiaoboyu/unipost-api/internal/xinbox"
)

const (
	defaultXWebhookMaxBodyBytes int64 = 1 << 20
	defaultXWebhookMaxEventAge        = 7 * 24 * time.Hour
	xWebhookFutureTolerance           = 5 * time.Minute
	xWebhookProcessingTimeout         = 9 * time.Second
	defaultXCRCRateLimit              = 30
	defaultXCRCRateWindow             = time.Minute
	defaultXCRCMaxRateLimitKeys       = 10000
)

var xCRCTokenPattern = regexp.MustCompile(`^[A-Za-z0-9._~+/=:-]{1,512}$`)

type XWebhookSecretResolver interface {
	ConsumerSecret(context.Context, string) (string, error)
}

type XWebhookIngestor interface {
	IngestActivityEvent(context.Context, string, xinbox.ActivityEvent) error
}

type XWebhookConfig struct {
	Secrets          XWebhookSecretResolver
	Ingestor         XWebhookIngestor
	ManagedRouteKey  string
	MaxBodyBytes     int64
	MaxEventAge      time.Duration
	Now              func() time.Time
	CRCRateLimit     int
	CRCRateWindow    time.Duration
	MaxRateLimitKeys int
}

type XWebhookHandler struct {
	secrets         XWebhookSecretResolver
	ingestor        XWebhookIngestor
	managedRouteKey string
	maxBodyBytes    int64
	maxEventAge     time.Duration
	now             func() time.Time
	crcLimiter      *xCRCLimiter
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
	rateLimit := config.CRCRateLimit
	if rateLimit <= 0 {
		rateLimit = defaultXCRCRateLimit
	}
	rateWindow := config.CRCRateWindow
	if rateWindow <= 0 {
		rateWindow = defaultXCRCRateWindow
	}
	maxRateLimitKeys := config.MaxRateLimitKeys
	if maxRateLimitKeys <= 0 {
		maxRateLimitKeys = defaultXCRCMaxRateLimitKeys
	}
	return &XWebhookHandler{
		secrets:         config.Secrets,
		ingestor:        config.Ingestor,
		managedRouteKey: strings.TrimSpace(config.ManagedRouteKey),
		maxBodyBytes:    maxBodyBytes,
		maxEventAge:     maxEventAge,
		now:             now,
		crcLimiter: &xCRCLimiter{
			limit:      rateLimit,
			window:     rateWindow,
			maxEntries: maxRateLimitKeys,
			entries:    make(map[[32]byte]xCRCLimitEntry),
		},
	}
}

func (h *XWebhookHandler) CRC(w http.ResponseWriter, r *http.Request) {
	requestCtx, cancel := context.WithTimeout(r.Context(), xWebhookProcessingTimeout)
	defer cancel()
	routeKey := h.routeKey(r)
	if routeKey == "" {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "X webhook app was not found")
		return
	}
	values := r.URL.Query()
	crcTokens, ok := values["crc_token"]
	if !ok || len(values) != 1 || len(crcTokens) != 1 || !xCRCTokenPattern.MatchString(crcTokens[0]) {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "crc_token is required")
		return
	}
	crcToken := crcTokens[0]
	if !h.crcLimiter.Allow(routeKey, requestIP(r), h.now()) {
		writeError(w, http.StatusTooManyRequests, "RATE_LIMITED", "Too many CRC requests")
		return
	}
	secret, err := h.resolveSecret(requestCtx, routeKey)
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
	routeKey := h.routeKey(r)
	if routeKey == "" {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "X webhook app was not found")
		return
	}
	secret, err := h.resolveSecret(requestCtx, routeKey)
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
			if err := h.ingestor.IngestActivityEvent(requestCtx, routeKey, event); err != nil {
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

func (h *XWebhookHandler) routeKey(r *http.Request) string {
	if routeKey := strings.TrimSpace(chi.URLParam(r, "webhook_route_key")); routeKey != "" {
		return routeKey
	}
	// The legacy base path is intentionally limited to the one configured
	// UniPost-managed app. Workspace apps always require their app-specific
	// path, so an ambiguous or unconfigured base route fails closed.
	return h.managedRouteKey
}

func (h *XWebhookHandler) resolveSecret(ctx context.Context, routeKey string) (string, error) {
	if h == nil || h.secrets == nil {
		return "", xinbox.ErrAppSecretNotFound
	}
	secret, err := h.secrets.ConsumerSecret(ctx, routeKey)
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

type xCRCLimitEntry struct {
	windowStart time.Time
	count       int
}

type xCRCLimiter struct {
	mu         sync.Mutex
	limit      int
	window     time.Duration
	maxEntries int
	entries    map[[32]byte]xCRCLimitEntry
}

func (l *xCRCLimiter) Allow(routeKey, ip string, now time.Time) bool {
	if l == nil {
		return true
	}
	key := sha256.Sum256([]byte(routeKey + "\x00" + ip))
	now = now.UTC()
	l.mu.Lock()
	defer l.mu.Unlock()
	entry, exists := l.entries[key]
	if !exists || now.Sub(entry.windowStart) >= l.window || now.Before(entry.windowStart) {
		if !exists && len(l.entries) >= l.maxEntries {
			for existingKey, existing := range l.entries {
				if now.Sub(existing.windowStart) >= l.window {
					delete(l.entries, existingKey)
				}
			}
			if len(l.entries) >= l.maxEntries {
				return false
			}
		}
		l.entries[key] = xCRCLimitEntry{windowStart: now, count: 1}
		return true
	}
	if entry.count >= l.limit {
		return false
	}
	entry.count++
	l.entries[key] = entry
	return true
}

func requestIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err == nil {
		return host
	}
	return strings.TrimSpace(r.RemoteAddr)
}
