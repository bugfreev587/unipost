package platform

import "context"

// DispatchMetadata carries server-owned facts that adapters need at the
// last responsible moment. It is deliberately separate from public platform
// options so clients cannot forge account or provider-environment identity.
type DispatchMetadata struct {
	SocialAccountID string
	Environment     string
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
