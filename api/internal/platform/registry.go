package platform

import "fmt"

var registry = map[string]PlatformAdapter{}

// AccountStateInvalidator lets adapters discard provider-resource state when
// an account reconnects or disconnects. Generic account handlers call the
// registry helper without knowing which adapters keep such state.
type AccountStateInvalidator interface {
	InvalidateAccountState(accountID string)
}

// Register adds a platform adapter to the global registry.
func Register(adapter PlatformAdapter) {
	registry[adapter.Platform()] = adapter
}

// Get returns the adapter for the given platform.
func Get(platform string) (PlatformAdapter, error) {
	a, ok := registry[platform]
	if !ok {
		return nil, fmt.Errorf("unsupported platform: %s", platform)
	}
	return a, nil
}

func InvalidateAccountState(platformName, accountID string) {
	adapter, err := Get(platformName)
	if err != nil {
		return
	}
	if invalidator, ok := adapter.(AccountStateInvalidator); ok {
		invalidator.InvalidateAccountState(accountID)
	}
}
