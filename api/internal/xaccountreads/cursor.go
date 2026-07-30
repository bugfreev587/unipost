package xaccountreads

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"
)

const cursorLifetime = 7 * 24 * time.Hour

var cursorAAD = []byte("unipost-x-account-read-cursor-v1")

type CursorScope struct {
	WorkspaceID            string `json:"workspace_id"`
	AccountID              string `json:"account_id"`
	ExternalUserID         string `json:"external_user_id"`
	StartTime              string `json:"start_time,omitempty"`
	EndTime                string `json:"end_time,omitempty"`
	ExcludeReposts         bool   `json:"exclude_reposts"`
	ExcludeRepliesToOthers bool   `json:"exclude_replies_to_others"`
}

type cursorPayload struct {
	Version         int         `json:"version"`
	Scope           CursorScope `json:"scope"`
	PaginationToken string      `json:"pagination_token"`
	IssuedAt        time.Time   `json:"issued_at"`
	ExpiresAt       time.Time   `json:"expires_at"`
}

type CursorCodec struct {
	aead cipher.AEAD
}

func NewCursorCodec(secret []byte) (*CursorCodec, error) {
	if len(secret) < 16 {
		return nil, errors.New("X account-read cursor secret must be at least 16 bytes")
	}
	key := sha256.Sum256(append([]byte("unipost:x-account-read:cursor:"), secret...))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &CursorCodec{aead: aead}, nil
}

func (c *CursorCodec) Encode(scope CursorScope, paginationToken string, now time.Time) (string, time.Time, error) {
	if c == nil || c.aead == nil || scope.WorkspaceID == "" || scope.AccountID == "" ||
		scope.ExternalUserID == "" || paginationToken == "" {
		return "", time.Time{}, errors.New("invalid X account-read cursor input")
	}
	now = now.UTC()
	expiresAt := now.Add(cursorLifetime)
	body, err := json.Marshal(cursorPayload{
		Version: 1, Scope: scope, PaginationToken: paginationToken,
		IssuedAt: now, ExpiresAt: expiresAt,
	})
	if err != nil {
		return "", time.Time{}, err
	}
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", time.Time{}, err
	}
	sealed := c.aead.Seal(nil, nonce, body, cursorAAD)
	encoded := append([]byte{1}, nonce...)
	encoded = append(encoded, sealed...)
	return base64.RawURLEncoding.EncodeToString(encoded), expiresAt, nil
}

func (c *CursorCodec) Decode(encoded string, expected CursorScope, now time.Time) (string, time.Time, error) {
	if c == nil || c.aead == nil || encoded == "" {
		return "", time.Time{}, errors.New("invalid X account-read cursor")
	}
	raw, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil || len(raw) < 1+c.aead.NonceSize()+c.aead.Overhead() || raw[0] != 1 {
		return "", time.Time{}, errors.New("invalid X account-read cursor")
	}
	nonce := raw[1 : 1+c.aead.NonceSize()]
	plaintext, err := c.aead.Open(nil, nonce, raw[1+c.aead.NonceSize():], cursorAAD)
	if err != nil {
		return "", time.Time{}, errors.New("invalid X account-read cursor")
	}
	var payload cursorPayload
	if err := json.Unmarshal(plaintext, &payload); err != nil || payload.Version != 1 ||
		payload.PaginationToken == "" || payload.Scope != expected || payload.ExpiresAt.IsZero() ||
		!payload.ExpiresAt.After(now.UTC()) {
		return "", time.Time{}, errors.New("invalid X account-read cursor")
	}
	return payload.PaginationToken, payload.ExpiresAt.UTC(), nil
}

func (c *CursorCodec) Refresh(encoded string, scope CursorScope, now time.Time) (string, time.Time, error) {
	token, _, err := c.Decode(encoded, scope, now)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("refresh X account-read cursor: %w", err)
	}
	return c.Encode(scope, token, now)
}
