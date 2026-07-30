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

var (
	cursorAADV1 = []byte("unipost-x-account-read-cursor-v1")
	cursorAADV2 = []byte("unipost-x-account-read-cursor-v2")
)

const cursorKeyIDSize = 8

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
	primary cursorCipher
	byID    map[[cursorKeyIDSize]byte]cipher.AEAD
	all     []cipher.AEAD
}

type cursorCipher struct {
	id   [cursorKeyIDSize]byte
	aead cipher.AEAD
}

func newCursorCipher(secret []byte) (cursorCipher, error) {
	if len(secret) < 16 {
		return cursorCipher{}, errors.New("X account-read cursor secret must be at least 16 bytes")
	}
	keyMaterial := append([]byte("unipost:x-account-read:cursor:"), secret...)
	key := sha256.Sum256(keyMaterial)
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return cursorCipher{}, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return cursorCipher{}, err
	}
	idDigest := sha256.Sum256(append([]byte("unipost:x-account-read:cursor-kid:"), key[:]...))
	var id [cursorKeyIDSize]byte
	copy(id[:], idDigest[:cursorKeyIDSize])
	return cursorCipher{id: id, aead: aead}, nil
}

func NewCursorCodec(secret []byte, previousSecrets ...[]byte) (*CursorCodec, error) {
	current, err := newCursorCipher(secret)
	if err != nil {
		return nil, err
	}
	codec := &CursorCodec{
		primary: current,
		byID:    map[[cursorKeyIDSize]byte]cipher.AEAD{current.id: current.aead},
		all:     []cipher.AEAD{current.aead},
	}
	for _, previousSecret := range previousSecrets {
		previous, err := newCursorCipher(previousSecret)
		if err != nil {
			return nil, err
		}
		if _, duplicate := codec.byID[previous.id]; duplicate {
			continue
		}
		codec.byID[previous.id] = previous.aead
		codec.all = append(codec.all, previous.aead)
	}
	return codec, nil
}

func (c *CursorCodec) Encode(scope CursorScope, paginationToken string, now time.Time) (string, time.Time, error) {
	if c == nil || c.primary.aead == nil || scope.WorkspaceID == "" || scope.AccountID == "" ||
		scope.ExternalUserID == "" || paginationToken == "" {
		return "", time.Time{}, errors.New("invalid X account-read cursor input")
	}
	now = now.UTC()
	expiresAt := now.Add(cursorLifetime)
	body, err := json.Marshal(cursorPayload{
		Version: 2, Scope: scope, PaginationToken: paginationToken,
		IssuedAt: now, ExpiresAt: expiresAt,
	})
	if err != nil {
		return "", time.Time{}, err
	}
	nonce := make([]byte, c.primary.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", time.Time{}, err
	}
	aad := append(append([]byte(nil), cursorAADV2...), c.primary.id[:]...)
	sealed := c.primary.aead.Seal(nil, nonce, body, aad)
	encoded := append([]byte{2}, c.primary.id[:]...)
	encoded = append(encoded, nonce...)
	encoded = append(encoded, sealed...)
	return base64.RawURLEncoding.EncodeToString(encoded), expiresAt, nil
}

func (c *CursorCodec) Decode(encoded string, expected CursorScope, now time.Time) (string, time.Time, error) {
	if c == nil || c.primary.aead == nil || encoded == "" {
		return "", time.Time{}, errors.New("invalid X account-read cursor")
	}
	raw, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil || len(raw) < 1 {
		return "", time.Time{}, errors.New("invalid X account-read cursor")
	}
	var plaintext []byte
	switch raw[0] {
	case 2:
		if len(raw) < 1+cursorKeyIDSize {
			return "", time.Time{}, errors.New("invalid X account-read cursor")
		}
		var keyID [cursorKeyIDSize]byte
		copy(keyID[:], raw[1:1+cursorKeyIDSize])
		aead := c.byID[keyID]
		if aead == nil || len(raw) < 1+cursorKeyIDSize+aead.NonceSize()+aead.Overhead() {
			return "", time.Time{}, errors.New("invalid X account-read cursor")
		}
		nonceStart := 1 + cursorKeyIDSize
		nonce := raw[nonceStart : nonceStart+aead.NonceSize()]
		aad := append(append([]byte(nil), cursorAADV2...), keyID[:]...)
		plaintext, err = aead.Open(nil, nonce, raw[nonceStart+aead.NonceSize():], aad)
	case 1:
		for _, aead := range c.all {
			if len(raw) < 1+aead.NonceSize()+aead.Overhead() {
				continue
			}
			nonce := raw[1 : 1+aead.NonceSize()]
			plaintext, err = aead.Open(nil, nonce, raw[1+aead.NonceSize():], cursorAADV1)
			if err == nil {
				break
			}
		}
	default:
		err = errors.New("unsupported cursor version")
	}
	if err != nil || plaintext == nil {
		return "", time.Time{}, errors.New("invalid X account-read cursor")
	}
	var payload cursorPayload
	if err := json.Unmarshal(plaintext, &payload); err != nil || payload.Version != int(raw[0]) ||
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
