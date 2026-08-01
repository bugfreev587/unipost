package socialconnections

import (
	"errors"
	"strings"
)

var ErrInvalidProviderIdentity = errors.New("invalid verified provider identity")

// ProviderIdentity derives the canonical physical provider identity only from
// fields returned by a verified provider flow. Instagram's application-scoped
// account ID is intentionally not accepted as a substitute for its webhook
// professional-user ID.
func ProviderIdentity(platform, externalAccountID string, metadata map[string]any) (string, error) {
	platform = strings.ToLower(strings.TrimSpace(platform))
	if platform == "" {
		return "", ErrInvalidProviderIdentity
	}

	identity := strings.TrimSpace(externalAccountID)
	if platform == "instagram" {
		value, ok := metadata["instagram_webhook_user_id"].(string)
		if !ok {
			return "", ErrInvalidProviderIdentity
		}
		identity = strings.TrimSpace(value)
	}
	if identity == "" {
		return "", ErrInvalidProviderIdentity
	}
	return identity, nil
}
