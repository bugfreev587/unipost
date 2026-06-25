package platform

import "strings"

type providerError struct {
	message string
	fields  map[string]any
}

func newProviderError(message string, fields map[string]any) error {
	return providerError{
		message: strings.TrimSpace(message),
		fields:  fields,
	}
}

func (e providerError) Error() string {
	return e.message
}

func (e providerError) ProviderErrorFields() map[string]any {
	out := make(map[string]any, len(e.fields))
	for k, v := range e.fields {
		if strings.TrimSpace(k) == "" || v == nil {
			continue
		}
		if s, ok := v.(string); ok && strings.TrimSpace(s) == "" {
			continue
		}
		out[k] = v
	}
	return out
}
