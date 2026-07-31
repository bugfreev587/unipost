package socialconnections

import (
	"errors"
	"testing"
)

func TestProviderIdentity(t *testing.T) {
	tests := []struct {
		name     string
		platform string
		external string
		metadata map[string]any
		want     string
	}{
		{
			name:     "Instagram professional webhook identity",
			platform: "instagram",
			external: "app-scoped",
			metadata: map[string]any{"instagram_webhook_user_id": "professional"},
			want:     "professional",
		},
		{name: "Bluesky DID", platform: "bluesky", external: "did:plc:abc", want: "did:plc:abc"},
		{name: "Twitter provider ID", platform: "twitter", external: "42", want: "42"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ProviderIdentity(tt.platform, tt.external, tt.metadata)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Fatalf("ProviderIdentity() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestProviderIdentityRejectsUnverifiedIdentity(t *testing.T) {
	tests := []struct {
		name     string
		platform string
		external string
		metadata map[string]any
	}{
		{name: "Instagram missing webhook identity", platform: "instagram", external: "app-scoped"},
		{name: "Instagram non-string webhook identity", platform: "instagram", external: "app-scoped", metadata: map[string]any{"instagram_webhook_user_id": 42}},
		{name: "blank provider identity", platform: "twitter", external: "  "},
		{name: "blank platform", external: "42"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := ProviderIdentity(tt.platform, tt.external, tt.metadata); !errors.Is(err, ErrInvalidProviderIdentity) {
				t.Fatalf("ProviderIdentity() error = %v, want ErrInvalidProviderIdentity", err)
			}
		})
	}
}
