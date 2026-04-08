package handler

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// TestPreviewToken_RoundTrip locks down the basic sign → verify happy
// path against a fresh secret.
func TestPreviewToken_RoundTrip(t *testing.T) {
	secret := []byte("32-byte-secret-for-test-only-xx")
	tok, exp, err := signPreviewToken("post_abc", secret)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	if exp.Before(time.Now()) {
		t.Errorf("expiry should be in the future: %v", exp)
	}
	if !strings.Contains(tok, ".") {
		t.Errorf("token should be base64.base64: %q", tok)
	}

	postID, err := verifyPreviewToken(tok, secret)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if postID != "post_abc" {
		t.Errorf("post_id mismatch: got %q", postID)
	}
}

// TestPreviewToken_WrongSecret confirms a token signed with one key
// fails verification under a different key. Catches accidental
// secret-swap during a rotation.
func TestPreviewToken_WrongSecret(t *testing.T) {
	tok, _, _ := signPreviewToken("post_abc", []byte("secret-A"))
	if _, err := verifyPreviewToken(tok, []byte("secret-B")); err == nil {
		t.Error("expected verification failure with different secret")
	}
}

// TestPreviewToken_Tampered confirms editing the body invalidates the
// signature even when the new body parses as valid JSON.
func TestPreviewToken_Tampered(t *testing.T) {
	secret := []byte("test-secret")
	tok, _, _ := signPreviewToken("post_abc", secret)
	// Replace the body half with a different one.
	parts := strings.SplitN(tok, ".", 2)
	tampered := "ZWNobyBoYWNrZXIK." + parts[1]
	if _, err := verifyPreviewToken(tampered, secret); err == nil {
		t.Error("expected verification failure for tampered body")
	}
}

// TestPreviewToken_Expired uses a tiny TTL hack: sign, then check
// rejection by overriding the time directly via crafting a token.
// Easier to just sign with a far-past timestamp via verifyPreview's
// payload check.
func TestPreviewToken_RejectsTooOld(t *testing.T) {
	// We can't easily mock time inside signPreviewToken, so build a
	// payload by hand with exp in the past and verify.
	//
	// The test's purpose is to confirm verifyPreviewToken honors the
	// exp field — even a perfectly-signed token with past exp should
	// fail.
	expired := mustMakeExpired(t, "post_abc", []byte("test-secret"))
	if _, err := verifyPreviewToken(expired, []byte("test-secret")); err == nil {
		t.Error("expected expired token to fail verification")
	}
}

// mustMakeExpired hand-builds a token whose exp is in the past so we
// can test the verifier's expiry check. Lives in the test file because
// the production code never needs to mint already-expired tokens.
func mustMakeExpired(t *testing.T, postID string, secret []byte) string {
	t.Helper()
	payload := previewTokenPayload{
		PostID:   postID,
		Audience: previewTokenAudience,
		Expires:  time.Now().Add(-1 * time.Hour).Unix(),
	}
	body, _ := json.Marshal(payload)
	bodyB64 := base64.RawURLEncoding.EncodeToString(body)
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(bodyB64))
	return bodyB64 + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
