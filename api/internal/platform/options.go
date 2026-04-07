package platform

import "fmt"

// optString returns opts[key] as a string, or "" if missing/wrong type.
func optString(opts map[string]any, key string) string {
	if opts == nil {
		return ""
	}
	v, _ := opts[key].(string)
	return v
}

// optBool returns opts[key] as a bool. Accepts both native bool and the
// strings "true" / "1" so JSON callers can pass either form.
func optBool(opts map[string]any, key string) bool {
	if opts == nil {
		return false
	}
	switch v := opts[key].(type) {
	case bool:
		return v
	case string:
		return v == "true" || v == "1"
	}
	return false
}

// validateEnum returns an error if value is non-empty and not in allowed.
// The error format is intended to surface back to the API caller as a vendor-agnostic message.
func validateEnum(platform, field, value string, allowed []string) error {
	if value == "" {
		return nil
	}
	for _, a := range allowed {
		if a == value {
			return nil
		}
	}
	return fmt.Errorf("%s: invalid %s %q, allowed values: %v", platform, field, value, allowed)
}
