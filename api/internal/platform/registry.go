package platform

import "fmt"

var registry = map[string]PlatformAdapter{}

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
