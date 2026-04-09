package handler

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
)

// signedRequestForTest builds a Meta-format signed_request given a
// payload and app secret. Mirrors what Meta's reference impl emits.
func signedRequestForTest(t *testing.T, secret string, payload map[string]any) string {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	encodedPayload := base64.RawURLEncoding.EncodeToString(body)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(encodedPayload))
	encodedSig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return encodedSig + "." + encodedPayload
}

func TestVerifyMetaSignedRequest_HappyPath(t *testing.T) {
	secret := "test-app-secret-12345"
	signed := signedRequestForTest(t, secret, map[string]any{
		"user_id":   "1234567890",
		"algorithm": "HMAC-SHA256",
		"issued_at": int64(1700000000),
	})

	payload, err := verifyMetaSignedRequest(signed, secret)
	if err != nil {
		t.Fatalf("verify failed: %v", err)
	}
	if payload.UserID != "1234567890" {
		t.Errorf("user_id: got %q", payload.UserID)
	}
	if payload.Algorithm != "HMAC-SHA256" {
		t.Errorf("algorithm: got %q", payload.Algorithm)
	}
	if payload.IssuedAt != 1700000000 {
		t.Errorf("issued_at: got %d", payload.IssuedAt)
	}
}

func TestVerifyMetaSignedRequest_BadSignature(t *testing.T) {
	signed := signedRequestForTest(t, "right-secret", map[string]any{
		"user_id":   "1234567890",
		"algorithm": "HMAC-SHA256",
	})

	if _, err := verifyMetaSignedRequest(signed, "wrong-secret"); err == nil {
		t.Error("expected signature mismatch error, got nil")
	}
}

func TestVerifyMetaSignedRequest_MalformedFormat(t *testing.T) {
	cases := []string{
		"",                       // empty
		"only-one-part",          // missing dot
		"too.many.parts",         // too many dots
		"!!!.!!!",                // invalid base64
	}
	for _, c := range cases {
		if _, err := verifyMetaSignedRequest(c, "secret"); err == nil {
			t.Errorf("expected error for %q, got nil", c)
		}
	}
}

func TestVerifyMetaSignedRequest_WrongAlgorithm(t *testing.T) {
	secret := "test-secret"
	signed := signedRequestForTest(t, secret, map[string]any{
		"user_id":   "1234567890",
		"algorithm": "RS256", // we only accept HMAC-SHA256
	})

	if _, err := verifyMetaSignedRequest(signed, secret); err == nil {
		t.Error("expected unsupported algorithm error, got nil")
	}
}

func TestVerifyMetaSignedRequest_PayloadWithoutAlgorithm(t *testing.T) {
	// Some Meta callbacks omit algorithm — we should accept those
	// since the signature verification itself confirms HMAC-SHA256.
	secret := "test-secret"
	signed := signedRequestForTest(t, secret, map[string]any{
		"user_id": "1234567890",
	})

	payload, err := verifyMetaSignedRequest(signed, secret)
	if err != nil {
		t.Fatalf("verify failed: %v", err)
	}
	if payload.UserID != "1234567890" {
		t.Errorf("user_id: got %q", payload.UserID)
	}
}

func TestBase64URLDecode_AcceptsBothPaddingStyles(t *testing.T) {
	original := []byte("hello world")
	unpadded := base64.RawURLEncoding.EncodeToString(original)
	padded := base64.URLEncoding.EncodeToString(original)
	// Sanity: padded version should differ from unpadded.
	if !strings.HasSuffix(padded, "=") {
		t.Skip("test data doesn't actually have padding to test")
	}

	for _, in := range []string{unpadded, padded} {
		out, err := base64URLDecode(in)
		if err != nil {
			t.Errorf("decode %q failed: %v", in, err)
			continue
		}
		if string(out) != string(original) {
			t.Errorf("decode %q: got %q, want %q", in, string(out), string(original))
		}
	}
}
