package runtimeenv

import (
	"os"
	"strings"
)

const EnvVar = "UNIPOST_ENV"

// Current returns the normalized backend runtime environment. Empty values
// are treated as development so feature work remains hidden from production
// only when production explicitly declares itself via UNIPOST_ENV.
func Current() string {
	env := strings.ToLower(strings.TrimSpace(os.Getenv(EnvVar)))
	if env == "" {
		return "development"
	}
	return env
}

func IsProduction() bool {
	switch Current() {
	case "production", "prod", "live":
		return true
	default:
		return false
	}
}

// TruthyEnv accepts the common boolean values we use for feature flags.
func TruthyEnv(name string) bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv(name)))
	return v == "1" || v == "true" || v == "yes" || v == "on"
}

// FeatureEnabled returns an explicit feature flag value when present, or the
// supplied default otherwise. Pass !IsProduction() as defaultEnabled for
// features that should be visible in dev/preview but hidden in production.
func FeatureEnabled(name string, defaultEnabled bool) bool {
	if _, ok := os.LookupEnv(name); ok {
		return TruthyEnv(name)
	}
	return defaultEnabled
}
