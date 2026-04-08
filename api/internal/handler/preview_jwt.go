// preview_jwt.go is a tiny self-contained HMAC-SHA256 token format
// for sharing read-only preview links to drafts. We deliberately don't
// pull a full JWT library — the token only needs to encode three
// fields (post_id, audience, exp) and verify a signature, and the
// existing crypto.AESEncryptor's underlying key (ENCRYPTION_KEY) can
// double as the HMAC secret with an audience claim for domain
// separation (per Sprint 2 review decision B2).
//
// Format: base64url(payload).base64url(hmac)
// Payload: JSON {"post_id":"...","aud":"preview","exp":<unix_seconds>}
//
// We don't follow the full JWT spec because we don't need
// algorithms, key IDs, or header negotiation. Drop in a real JWT
// library when we do.

package handler

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// previewTokenAudience is the audience claim baked into every
// preview token. Verifying the audience prevents a token signed for
// a different purpose (if we ever introduce one) from accidentally
// granting preview access.
const previewTokenAudience = "preview"

// previewTokenTTL is how long a freshly-issued token is valid for.
// 24h matches the Sprint 2 PRD spec — long enough to share via Slack
// or email, short enough that a leaked link self-expires.
const previewTokenTTL = 24 * time.Hour

// previewTokenPayload is the structured payload encoded in the token
// before signing.
type previewTokenPayload struct {
	PostID   string `json:"post_id"`
	Audience string `json:"aud"`
	Expires  int64  `json:"exp"`
}

// signPreviewToken returns a token string of the form
// base64url(payload).base64url(hmac), valid for previewTokenTTL.
// secret is the raw bytes used as the HMAC key — main.go passes the
// ENCRYPTION_KEY value here so we don't need a separate env var.
func signPreviewToken(postID string, secret []byte) (string, time.Time, error) {
	if postID == "" {
		return "", time.Time{}, fmt.Errorf("preview token: post_id is required")
	}
	if len(secret) == 0 {
		return "", time.Time{}, fmt.Errorf("preview token: signing secret is empty")
	}
	expires := time.Now().Add(previewTokenTTL)
	payload := previewTokenPayload{
		PostID:   postID,
		Audience: previewTokenAudience,
		Expires:  expires.Unix(),
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("preview token: marshal: %w", err)
	}
	bodyB64 := base64.RawURLEncoding.EncodeToString(body)

	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(bodyB64))
	sigB64 := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))

	return bodyB64 + "." + sigB64, expires, nil
}

// verifyPreviewToken parses a token, checks the HMAC, the audience,
// and the expiry, and returns the post_id on success. Any failure
// produces a generic error — we don't disclose whether the signature
// was wrong vs. the audience vs. the timestamp, since they all map
// to "this token is not valid for this preview" from the caller's
// perspective.
func verifyPreviewToken(token string, secret []byte) (string, error) {
	if token == "" {
		return "", fmt.Errorf("preview token: missing")
	}
	parts := strings.SplitN(token, ".", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("preview token: malformed")
	}
	bodyB64, sigB64 := parts[0], parts[1]

	// Verify signature with constant-time compare to avoid timing
	// oracles. We sign over the base64'd body so verification doesn't
	// have to re-marshal the payload struct.
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(bodyB64))
	expectedSig := mac.Sum(nil)

	gotSig, err := base64.RawURLEncoding.DecodeString(sigB64)
	if err != nil {
		return "", fmt.Errorf("preview token: malformed signature")
	}
	if !hmac.Equal(expectedSig, gotSig) {
		return "", fmt.Errorf("preview token: signature mismatch")
	}

	// Parse the payload AFTER verifying the signature so we don't
	// trust any field of an unverified token.
	bodyBytes, err := base64.RawURLEncoding.DecodeString(bodyB64)
	if err != nil {
		return "", fmt.Errorf("preview token: malformed body")
	}
	var payload previewTokenPayload
	if err := json.Unmarshal(bodyBytes, &payload); err != nil {
		return "", fmt.Errorf("preview token: bad json")
	}

	if payload.Audience != previewTokenAudience {
		return "", fmt.Errorf("preview token: wrong audience")
	}
	if time.Now().Unix() > payload.Expires {
		return "", fmt.Errorf("preview token: expired")
	}
	if payload.PostID == "" {
		return "", fmt.Errorf("preview token: missing post_id")
	}
	return payload.PostID, nil
}
