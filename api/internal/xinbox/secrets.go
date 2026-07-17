package xinbox

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"strings"
)

type EncryptedConsumerSecretStore interface {
	EncryptedConsumerSecrets(context.Context, string) ([]string, error)
}

type AppSecretResolverConfig struct {
	ManagedRouteKey string
	ManagedSecret   string
	Store           EncryptedConsumerSecretStore
	Decrypt         func(string) (string, error)
}

type AppSecretResolver struct {
	managedRouteKey string
	managedSecret   string
	store           EncryptedConsumerSecretStore
	decrypt         func(string) (string, error)
}

func NewAppSecretResolver(config AppSecretResolverConfig) *AppSecretResolver {
	return &AppSecretResolver{
		managedRouteKey: strings.TrimSpace(config.ManagedRouteKey),
		managedSecret:   strings.TrimSpace(config.ManagedSecret),
		store:           config.Store,
		decrypt:         config.Decrypt,
	}
}

func WebhookRouteKey(routeSecret, clientID string) string {
	routeSecret = strings.TrimSpace(routeSecret)
	clientID = strings.TrimSpace(clientID)
	if routeSecret == "" || clientID == "" {
		return ""
	}
	mac := hmac.New(sha256.New, []byte(routeSecret))
	_, _ = mac.Write([]byte("unipost:x:webhook-route:v1\x00"))
	_, _ = mac.Write([]byte(clientID))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func RandomWebhookRouteKey() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func (r *AppSecretResolver) ConsumerSecret(ctx context.Context, routeKey string) (string, error) {
	routeKey = strings.TrimSpace(routeKey)
	if r == nil || routeKey == "" {
		return "", ErrAppSecretNotFound
	}
	if routeKey == r.managedRouteKey {
		if r.managedSecret == "" {
			return "", ErrAppSecretNotFound
		}
		return r.managedSecret, nil
	}
	if r.store == nil || r.decrypt == nil {
		return "", ErrAppSecretNotFound
	}
	encrypted, err := r.store.EncryptedConsumerSecrets(ctx, routeKey)
	if err != nil {
		return "", err
	}
	var resolved string
	for _, ciphertext := range encrypted {
		if strings.TrimSpace(ciphertext) == "" {
			continue
		}
		secret, err := r.decrypt(ciphertext)
		if err != nil {
			return "", err
		}
		if secret == "" {
			continue
		}
		if resolved == "" {
			resolved = secret
			continue
		}
		if subtle.ConstantTimeCompare([]byte(resolved), []byte(secret)) != 1 {
			return "", errors.New("conflicting consumer secrets for X webhook route key")
		}
	}
	if resolved == "" {
		return "", ErrAppSecretNotFound
	}
	return resolved, nil
}
