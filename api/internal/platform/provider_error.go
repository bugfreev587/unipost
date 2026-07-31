package platform

import "strings"

type providerError struct {
	message string
	fields  map[string]any
}

type FailureContract struct {
	ErrorCode   string
	Stage       string
	IsRetriable bool
}

type providerFailureError struct {
	providerError
	failure FailureContract
}

func newProviderError(message string, fields map[string]any) error {
	return providerError{
		message: strings.TrimSpace(message),
		fields:  fields,
	}
}

func newProviderFailure(message string, fields map[string]any, failure FailureContract) error {
	return providerFailureError{
		providerError: providerError{
			message: strings.TrimSpace(message),
			fields:  fields,
		},
		failure: failure,
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

func (e providerFailureError) FailureContractFields() map[string]any {
	return map[string]any{
		"error_code":    strings.TrimSpace(e.failure.ErrorCode),
		"failure_stage": strings.TrimSpace(e.failure.Stage),
		"is_retriable":  e.failure.IsRetriable,
	}
}

func (e providerFailureError) FailureStage() string {
	return strings.TrimSpace(e.failure.Stage)
}
