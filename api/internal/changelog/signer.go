package changelog

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"
	"time"
)

type Signer struct {
	secret string
	now    func() time.Time
}

func NewSigner(secret string) *Signer {
	return &Signer{
		secret: strings.TrimSpace(secret),
		now:    time.Now,
	}
}

func (s *Signer) Sign(candidateID string, action Action, expires time.Time, sourceHash string) string {
	message := signatureMessage(candidateID, action, expires.Unix(), sourceHash)
	mac := hmac.New(sha256.New, []byte(s.secret))
	mac.Write([]byte(message))
	return hex.EncodeToString(mac.Sum(nil))
}

func (s *Signer) Verify(candidateID string, action Action, expiresUnix int64, sourceHash, signature string) error {
	if s == nil || s.secret == "" {
		return ErrInvalidSignature
	}
	if !ValidAction(action) {
		return ErrUnsupportedAction
	}
	now := time.Now
	if s.now != nil {
		now = s.now
	}
	if now().Unix() > expiresUnix {
		return ErrExpiredSignature
	}
	expected := s.Sign(candidateID, action, time.Unix(expiresUnix, 0).UTC(), sourceHash)
	got, err := hex.DecodeString(strings.TrimSpace(signature))
	if err != nil {
		return ErrInvalidSignature
	}
	want, err := hex.DecodeString(expected)
	if err != nil {
		return ErrInvalidSignature
	}
	if !hmac.Equal(got, want) {
		return ErrInvalidSignature
	}
	return nil
}

func signatureMessage(candidateID string, action Action, expiresUnix int64, sourceHash string) string {
	return strings.Join([]string{
		strings.TrimSpace(candidateID),
		string(action),
		strconv.FormatInt(expiresUnix, 10),
		strings.TrimSpace(sourceHash),
	}, "\n")
}
