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

// Idempotent-publish option keys. The delivery worker injects these at
// dispatch time so an adapter can persist and resume its intermediate
// platform token (IG creation_id / TikTok publish_id) across retries
// instead of re-uploading and duplicating the post.
const (
	// OptResumePublishToken carries a token persisted by a prior attempt
	// (string). When present the adapter should resume from it rather than
	// initiating a fresh upload/publish.
	OptResumePublishToken = "resume_publish_token"
	// OptOnPublishToken carries a func(string) error the adapter calls the moment
	// it obtains an intermediate publish token, so the caller can persist it
	// before the (re)publish step. Returning an error aborts the provider flow.
	OptOnPublishToken = "on_publish_token"
)

// resumePublishToken returns the token a prior attempt persisted, or "".
func resumePublishToken(opts map[string]any) string {
	return optString(opts, OptResumePublishToken)
}

// persistPublishToken invokes the caller's persistence hook (if any) so a
// later retry can resume from token. A persistence failure must abort before
// the adapter continues with a non-durable provider token.
func persistPublishToken(opts map[string]any, token string) error {
	if token == "" || opts == nil {
		return nil
	}
	if fn, ok := opts[OptOnPublishToken].(func(string) error); ok {
		return fn(token)
	}
	return nil
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
