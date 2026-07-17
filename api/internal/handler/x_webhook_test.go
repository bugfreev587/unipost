package handler

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/xiaoboyu/unipost-api/internal/xinbox"
)

type fakeXWebhookSecrets struct {
	secrets map[string]string
}

func (f fakeXWebhookSecrets) ConsumerSecret(_ context.Context, appClientID string) (string, error) {
	secret, ok := f.secrets[appClientID]
	if !ok {
		return "", xinbox.ErrAppSecretNotFound
	}
	return secret, nil
}

type fakeXWebhookIngestor struct {
	appClientIDs []string
	events       []xinbox.ActivityEvent
}

func (f *fakeXWebhookIngestor) IngestActivityEvent(_ context.Context, appClientID string, event xinbox.ActivityEvent) error {
	f.appClientIDs = append(f.appClientIDs, appClientID)
	f.events = append(f.events, event)
	return nil
}

func xSignature(body []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(body)
	return "sha256=" + base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func xWebhookRequest(method, target, appClientID string, body []byte) *http.Request {
	req := httptest.NewRequest(method, target, bytes.NewReader(body))
	routeContext := chi.NewRouteContext()
	routeContext.URLParams.Add("webhook_route_key", appClientID)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeContext))
}

func TestXWebhookCRCUsesAppSpecificConsumerSecret(t *testing.T) {
	handler := NewXWebhookHandler(XWebhookConfig{
		Secrets: fakeXWebhookSecrets{secrets: map[string]string{"route-1": "consumer-secret"}},
	})
	req := xWebhookRequest(http.MethodGet, "/v1/webhooks/twitter/route-1?crc_token=challenge", "route-1", nil)
	rec := httptest.NewRecorder()
	handler.CRC(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	const want = `{"response_token":"sha256=2RUZDVKjSpEV/C/r9ivMsVZJ4DFPAawjJFQQzY+6ba4="}`
	if got := rec.Body.String(); got != want+"\n" {
		t.Fatalf("body = %q, want %q", got, want+"\\n")
	}
}

func TestXWebhookCRCAcceptsOpaqueProviderToken(t *testing.T) {
	handler := NewXWebhookHandler(XWebhookConfig{
		Secrets: fakeXWebhookSecrets{secrets: map[string]string{"route-1": "consumer-secret"}},
	})
	crcToken := strings.Repeat("a", 256) + ".+/=:"
	target := "/v1/webhooks/twitter/route-1?" + url.Values{"crc_token": {crcToken}}.Encode()
	req := xWebhookRequest(http.MethodGet, target, "route-1", nil)
	rec := httptest.NewRecorder()
	handler.CRC(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if got, want := rec.Body.String(), `{"response_token":"`+xSignature([]byte(crcToken), "consumer-secret")+`"}`+"\n"; got != want {
		t.Fatalf("body = %q, want %q", got, want)
	}
}

func TestXWebhookCRCIgnoresProviderMetadataQueryParameter(t *testing.T) {
	handler := NewXWebhookHandler(XWebhookConfig{
		Secrets: fakeXWebhookSecrets{secrets: map[string]string{"route-1": "consumer-secret"}},
	})
	const crcToken = "provider-challenge"
	target := "/v1/webhooks/twitter/route-1?" + url.Values{
		"crc_token":  {crcToken},
		"webhook_id": {"provider-webhook-1"},
	}.Encode()
	req := xWebhookRequest(http.MethodGet, target, "route-1", nil)
	rec := httptest.NewRecorder()
	handler.CRC(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if got, want := rec.Body.String(), `{"response_token":"`+xSignature([]byte(crcToken), "consumer-secret")+`"}`+"\n"; got != want {
		t.Fatalf("body = %q, want %q", got, want)
	}
}

func TestXWebhookPOSTVerifiesRawBodyBeforeParsing(t *testing.T) {
	body := []byte(`{"data":{"event_type":"dm.received","filter":{"user_id":"owner-1"},"tag":"unipost:x:dm:account-1","payload":{"id":"dm-1","dm_conversation_id":"c-1","sender_id":"sender-1","created_at":"2026-07-16T12:00:00Z","text":"private"}}}`)
	ingestor := &fakeXWebhookIngestor{}
	handler := NewXWebhookHandler(XWebhookConfig{
		Secrets:  fakeXWebhookSecrets{secrets: map[string]string{"client-1": "consumer-secret"}},
		Ingestor: ingestor,
		Now:      func() time.Time { return time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC) },
	})

	invalid := xWebhookRequest(http.MethodPost, "/v1/webhooks/twitter/client-1", "client-1", body)
	invalid.Header.Set("x-twitter-webhooks-signature", xSignature(body, "wrong-secret"))
	invalidRec := httptest.NewRecorder()
	handler.Handle(invalidRec, invalid)
	if invalidRec.Code != http.StatusUnauthorized {
		t.Fatalf("invalid signature status = %d", invalidRec.Code)
	}
	if len(ingestor.events) != 0 {
		t.Fatal("parsed/ingested payload before signature verification")
	}

	valid := xWebhookRequest(http.MethodPost, "/v1/webhooks/twitter/client-1", "client-1", body)
	valid.Header.Set("x-twitter-webhooks-signature", xSignature(body, "consumer-secret"))
	validRec := httptest.NewRecorder()
	handler.Handle(validRec, valid)
	if validRec.Code != http.StatusOK {
		t.Fatalf("valid status = %d body=%s", validRec.Code, validRec.Body.String())
	}
	if len(ingestor.events) != 1 || ingestor.appClientIDs[0] != "client-1" {
		t.Fatalf("ingested = %#v apps=%#v", ingestor.events, ingestor.appClientIDs)
	}
}

func TestXWebhookPOSTBoundsBodyAndDiscardsVerifiedStaleEvent(t *testing.T) {
	ingestor := &fakeXWebhookIngestor{}
	handler := NewXWebhookHandler(XWebhookConfig{
		Secrets:      fakeXWebhookSecrets{secrets: map[string]string{"client-1": "consumer-secret"}},
		Ingestor:     ingestor,
		MaxBodyBytes: 64,
		Now:          func() time.Time { return time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC) },
		MaxEventAge:  24 * time.Hour,
	})

	oversizeBody := bytes.Repeat([]byte("x"), 65)
	oversize := xWebhookRequest(http.MethodPost, "/v1/webhooks/twitter/client-1", "client-1", oversizeBody)
	oversize.Header.Set("x-twitter-webhooks-signature", xSignature(oversizeBody, "consumer-secret"))
	oversizeRec := httptest.NewRecorder()
	handler.Handle(oversizeRec, oversize)
	if oversizeRec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversize status = %d", oversizeRec.Code)
	}

	staleBody := []byte(`{"data":{"event_type":"dm.received","filter":{"user_id":"owner-1"},"tag":"unipost:x:dm:account-1","payload":{"id":"dm-old","created_at":"2026-07-01T12:00:00Z","sender_id":"sender-1","text":"old private"}}}`)
	staleHandler := NewXWebhookHandler(XWebhookConfig{
		Secrets:     fakeXWebhookSecrets{secrets: map[string]string{"client-1": "consumer-secret"}},
		Ingestor:    ingestor,
		Now:         func() time.Time { return time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC) },
		MaxEventAge: 24 * time.Hour,
	})
	stale := xWebhookRequest(http.MethodPost, "/v1/webhooks/twitter/client-1", "client-1", staleBody)
	stale.Header.Set("x-twitter-webhooks-signature", xSignature(staleBody, "consumer-secret"))
	staleRec := httptest.NewRecorder()
	staleHandler.Handle(staleRec, stale)
	if staleRec.Code != http.StatusOK {
		t.Fatalf("stale status = %d", staleRec.Code)
	}
	if len(ingestor.events) != 0 {
		t.Fatalf("stale event ingested = %#v", ingestor.events)
	}
}

func TestXWebhookLegacyBaseFailsClosedWithoutSingleManagedApp(t *testing.T) {
	handler := NewXWebhookHandler(XWebhookConfig{
		Secrets: fakeXWebhookSecrets{secrets: map[string]string{}},
	})
	req := httptest.NewRequest(http.MethodGet, "/v1/webhooks/twitter?crc_token=challenge", nil)
	rec := httptest.NewRecorder()
	handler.CRC(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestXWebhookCRCRejectsSigningOracleInputsAndRateLimitsRouteIP(t *testing.T) {
	handler := NewXWebhookHandler(XWebhookConfig{
		Secrets:          fakeXWebhookSecrets{secrets: map[string]string{"route-1": "consumer-secret"}},
		CRCRateLimit:     2,
		CRCRateWindow:    time.Minute,
		MaxRateLimitKeys: 10,
	})
	for _, target := range []string{
		"/v1/webhooks/twitter/route-1?crc_token=%7B%22data%22%3A%7B%7D%7D",
		"/v1/webhooks/twitter/route-1?crc_token=one&crc_token=two",
		"/v1/webhooks/twitter/route-1?crc_token=",
	} {
		req := xWebhookRequest(http.MethodGet, target, "route-1", nil)
		req.RemoteAddr = "203.0.113.10:1234"
		rec := httptest.NewRecorder()
		handler.CRC(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("target %q status = %d body=%s", target, rec.Code, rec.Body.String())
		}
	}

	for i, want := range []int{http.StatusOK, http.StatusOK, http.StatusTooManyRequests} {
		req := xWebhookRequest(http.MethodGet, "/v1/webhooks/twitter/route-1?crc_token=challenge_"+string(rune('a'+i)), "route-1", nil)
		req.RemoteAddr = "203.0.113.10:1234"
		rec := httptest.NewRecorder()
		handler.CRC(rec, req)
		if rec.Code != want {
			t.Fatalf("rate request %d status = %d, want %d", i, rec.Code, want)
		}
	}
}

func TestXWebhookAuthenticatedMalformedDMReturnsNon2xx(t *testing.T) {
	body := []byte(`{"data":{"event_type":"dm.received","filter":{"user_id":"owner-1"},"tag":"unipost:x:dm:account-1","payload":{"sender_id":"sender-1","created_at":"2026-07-16T12:00:00Z"}}}`)
	handler := NewXWebhookHandler(XWebhookConfig{
		Secrets: fakeXWebhookSecrets{secrets: map[string]string{"route-1": "consumer-secret"}},
	})
	req := xWebhookRequest(http.MethodPost, "/v1/webhooks/twitter/route-1", "route-1", body)
	req.Header.Set("x-twitter-webhooks-signature", xSignature(body, "consumer-secret"))
	rec := httptest.NewRecorder()
	handler.Handle(rec, req)
	if rec.Code < 400 {
		t.Fatalf("status = %d, malformed recognized delivery was acknowledged", rec.Code)
	}
}
