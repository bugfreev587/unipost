package xinbox

import (
	"context"
	"crypto/subtle"
	"errors"
	"strings"
)

type EncryptedConsumerSecretStore interface {
	EncryptedConsumerSecrets(context.Context, string) ([]string, error)
}

type AppSecretResolverConfig struct {
	ManagedAppClientID string
	ManagedSecret      string
	Store              EncryptedConsumerSecretStore
	Decrypt            func(string) (string, error)
}

type AppSecretResolver struct {
	managedAppClientID string
	managedSecret      string
	store              EncryptedConsumerSecretStore
	decrypt            func(string) (string, error)
}

func NewAppSecretResolver(config AppSecretResolverConfig) *AppSecretResolver {
	return &AppSecretResolver{
		managedAppClientID: strings.TrimSpace(config.ManagedAppClientID),
		managedSecret:      strings.TrimSpace(config.ManagedSecret),
		store:              config.Store,
		decrypt:            config.Decrypt,
	}
}

func (r *AppSecretResolver) ConsumerSecret(ctx context.Context, appClientID string) (string, error) {
	appClientID = strings.TrimSpace(appClientID)
	if r == nil || appClientID == "" {
		return "", ErrAppSecretNotFound
	}
	if appClientID == r.managedAppClientID {
		if r.managedSecret == "" {
			return "", ErrAppSecretNotFound
		}
		return r.managedSecret, nil
	}
	if r.store == nil || r.decrypt == nil {
		return "", ErrAppSecretNotFound
	}
	encrypted, err := r.store.EncryptedConsumerSecrets(ctx, appClientID)
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
			return "", errors.New("conflicting consumer secrets for X app client id")
		}
	}
	if resolved == "" {
		return "", ErrAppSecretNotFound
	}
	return resolved, nil
}
