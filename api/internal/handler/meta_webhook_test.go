package handler

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMetaWebhookVerify(t *testing.T) {
	h := NewMetaWebhookHandler("test-secret", "my-verify-token")

	t.Run("valid subscribe", func(t *testing.T) {
		req := httptest.NewRequest("GET",
			"/webhooks/meta?hub.mode=subscribe&hub.verify_token=my-verify-token&hub.challenge=challenge123",
			nil)
		rr := httptest.NewRecorder()
		h.Verify(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rr.Code)
		}
		if body := rr.Body.String(); body != "challenge123" {
			t.Fatalf("expected challenge echoed back, got %q", body)
		}
	})

	t.Run("wrong verify token", func(t *testing.T) {
		req := httptest.NewRequest("GET",
			"/webhooks/meta?hub.mode=subscribe&hub.verify_token=wrong&hub.challenge=c",
			nil)
		rr := httptest.NewRecorder()
		h.Verify(rr, req)

		if rr.Code != http.StatusForbidden {
			t.Fatalf("expected 403, got %d", rr.Code)
		}
	})

	t.Run("wrong mode", func(t *testing.T) {
		req := httptest.NewRequest("GET",
			"/webhooks/meta?hub.mode=unsubscribe&hub.verify_token=my-verify-token&hub.challenge=c",
			nil)
		rr := httptest.NewRecorder()
		h.Verify(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rr.Code)
		}
	})
}

func TestMetaWebhookHandle(t *testing.T) {
	appSecret := "test-app-secret"
	h := NewMetaWebhookHandler(appSecret, "tok")

	sign := func(body string) string {
		mac := hmac.New(sha256.New, []byte(appSecret))
		mac.Write([]byte(body))
		return "sha256=" + hex.EncodeToString(mac.Sum(nil))
	}

	t.Run("valid payload", func(t *testing.T) {
		body := `{"object":"instagram","entry":[{"id":"123","time":1700000000}]}`
		req := httptest.NewRequest("POST", "/webhooks/meta", strings.NewReader(body))
		req.Header.Set("X-Hub-Signature-256", sign(body))
		rr := httptest.NewRecorder()
		h.Handle(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
		}
	})

	t.Run("bad signature", func(t *testing.T) {
		body := `{"object":"instagram","entry":[]}`
		req := httptest.NewRequest("POST", "/webhooks/meta", strings.NewReader(body))
		req.Header.Set("X-Hub-Signature-256", "sha256=0000000000000000000000000000000000000000000000000000000000000000")
		rr := httptest.NewRecorder()
		h.Handle(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rr.Code)
		}
	})

	t.Run("missing signature", func(t *testing.T) {
		body := `{"object":"instagram","entry":[]}`
		req := httptest.NewRequest("POST", "/webhooks/meta", strings.NewReader(body))
		rr := httptest.NewRecorder()
		h.Handle(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rr.Code)
		}
	})

	t.Run("not configured", func(t *testing.T) {
		unconfigured := NewMetaWebhookHandler("", "tok")
		body := `{"object":"instagram","entry":[]}`
		req := httptest.NewRequest("POST", "/webhooks/meta", strings.NewReader(body))
		rr := httptest.NewRecorder()
		unconfigured.Handle(rr, req)

		if rr.Code != http.StatusServiceUnavailable {
			t.Fatalf("expected 503, got %d", rr.Code)
		}
	})
}
