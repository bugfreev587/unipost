package handler

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
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
	routeContext.URLParams.Add("app_client_id", appClientID)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeContext))
}

func TestXWebhookCRCUsesAppSpecificConsumerSecret(t *testing.T) {
	handler := NewXWebhookHandler(XWebhookConfig{
		Secrets: fakeXWebhookSecrets{secrets: map[string]string{"client-1": "consumer-secret"}},
	})
	req := xWebhookRequest(http.MethodGet, "/v1/webhooks/twitter/client-1?crc_token=challenge", "client-1", nil)
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

func TestXWebhookPOSTVerifiesRawBodyBeforeParsing(t *testing.T) {
	body := []byte(`{"data":{"event_type":"dm.received","filter":{"user_id":"owner-1"},"tag":"unipost:x:dm:account-1","payload":{"id":"dm-1","dm_conversation_id":"c-1","sender_id":"sender-1","text":"private"}}}`)
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
