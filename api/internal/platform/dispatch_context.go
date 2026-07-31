package platform

import (
	"context"
	"sync"
	"time"
)

const maxDispatchEvents = 16

type DispatchEvent struct {
	Name             string
	Status           string
	Environment      string
	BoardFingerprint string
	HTTPStatus       int
	ProviderCode     string
	ErrorCode        string
	Reason           string
	FailureStage     string
	Retriable        bool
	Duration         time.Duration
	MediaType        string
	MediaSizeBytes   int64
	CustomerInput    bool
}

type DispatchEventRecorder struct {
	mu     sync.Mutex
	events []DispatchEvent
}

func NewDispatchEventRecorder() *DispatchEventRecorder {
	return &DispatchEventRecorder{}
}

func (r *DispatchEventRecorder) Record(event DispatchEvent) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.events) >= maxDispatchEvents {
		return
	}
	r.events = append(r.events, event)
}

func (r *DispatchEventRecorder) Snapshot() []DispatchEvent {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	result := make([]DispatchEvent, len(r.events))
	copy(result, r.events)
	return result
}

// DispatchMetadata carries server-owned facts that adapters need at the
// last responsible moment. It is deliberately separate from public platform
// options so clients cannot forge account or provider-environment identity.
type DispatchMetadata struct {
	SocialAccountID string
	Environment     string
	Events          *DispatchEventRecorder
}

type dispatchMetadataContextKey struct{}

func WithDispatchMetadata(ctx context.Context, metadata DispatchMetadata) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, dispatchMetadataContextKey{}, metadata)
}

func DispatchMetadataFromContext(ctx context.Context) (DispatchMetadata, bool) {
	if ctx == nil {
		return DispatchMetadata{}, false
	}
	metadata, ok := ctx.Value(dispatchMetadataContextKey{}).(DispatchMetadata)
	return metadata, ok
}
