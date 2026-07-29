package featureflags

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fakeStore struct {
	enabled     map[string]bool
	owner       string
	globalCalls int
}

func (s *fakeStore) List(context.Context) ([]Flag, error) {
	flags := make([]Flag, 0, len(Definitions()))
	for _, definition := range Definitions() {
		flags = append(flags, Flag{
			Key:         definition.Key,
			Enabled:     s.enabled[definition.Key],
			Description: definition.Description,
		})
	}
	return flags, nil
}

func (s *fakeStore) Set(_ context.Context, key string, enabled bool, actor string) (Flag, error) {
	if _, ok := DefinitionFor(key); !ok {
		return Flag{}, ErrUnknownFlag
	}
	s.enabled[key] = enabled
	return Flag{Key: key, Enabled: enabled, UpdatedBy: actor, UpdatedAt: time.Now()}, nil
}

func (s *fakeStore) GlobalEnabled(_ context.Context, key string) (bool, error) {
	s.globalCalls++
	if _, ok := DefinitionFor(key); !ok {
		return false, ErrUnknownFlag
	}
	return s.enabled[key], nil
}

func TestEvaluatorPublicFailsClosedBeforeStoreForActivationLockedFlag(t *testing.T) {
	store := &fakeStore{enabled: map[string]bool{ObservabilityReadsV2: true}}
	evaluator := NewEvaluator(store, nil)

	enabled, err := evaluator.Public(context.Background(), ObservabilityReadsV2)
	if err != nil {
		t.Fatal(err)
	}
	if enabled {
		t.Fatal("activation-locked flag honored stale stored true value")
	}
	if store.globalCalls != 0 {
		t.Fatalf("GlobalEnabled calls = %d, want 0", store.globalCalls)
	}
}

func (s *fakeStore) WorkspaceOwner(context.Context, string) (string, error) {
	if s.owner == "" {
		return "", errors.New("missing workspace")
	}
	return s.owner, nil
}

type fakeSuperAdminChecker map[string]bool

func (c fakeSuperAdminChecker) IsSuperAdmin(_ context.Context, userID string) bool {
	return c[userID]
}

func TestDefinitionsAreAllowlistedAndDefaultOff(t *testing.T) {
	got := Definitions()
	if len(got) != 3 {
		t.Fatalf("Definitions() returned %d flags, want 3", len(got))
	}
	if got[0].Key != XDMSV1 || got[1].Key != XCreditsBillingV1 || got[2].Key != ObservabilityReadsV2 {
		t.Fatalf("unexpected keys: %#v", got)
	}
	for _, definition := range got {
		if definition.DefaultEnabled {
			t.Fatalf("%s must default off", definition.Key)
		}
	}
	if !got[0].ActivationReady || !got[1].ActivationReady || got[2].ActivationReady {
		t.Fatalf("activation readiness = %#v", got)
	}
}

func TestObservabilityReadFlagStaysLockedInternalAndDefaultOffUntilSDKsAreCompatible(t *testing.T) {
	definition, ok := DefinitionFor(ObservabilityReadsV2)
	if !ok {
		t.Fatal("observability read flag is not registered")
	}
	if definition.ActivationReady {
		t.Fatal("observability read flag must stay locked until every public SDK supports opaque log IDs")
	}
	if !definition.Internal {
		t.Fatal("observability read flag must remain internal")
	}
	if definition.DefaultEnabled {
		t.Fatal("observability read flag must remain default OFF")
	}
	if definition.OwnerArea != "API / Admin Observability" {
		t.Fatalf("owner area = %q, want API / Admin Observability", definition.OwnerArea)
	}
	if definition.Description != "Uses the canonical request-event model for Logs and API Metrics reads; activation is locked pending SDK, canonical live-publisher, historical Metrics coverage/parity, retention safety, and exact-SHA prerequisites." {
		t.Fatalf("description = %q", definition.Description)
	}
}

func TestFlagWithDefinitionMetadataUsesAuthoritativeDescription(t *testing.T) {
	updatedAt := time.Date(2026, time.July, 29, 12, 0, 0, 0, time.UTC)
	persisted := Flag{
		Key:         ObservabilityReadsV2,
		Enabled:     true,
		Description: "Uses the canonical request-event model for Logs, Metrics, and Errors reads.",
		UpdatedBy:   "admin_1",
		UpdatedAt:   updatedAt,
	}

	got, ok := flagWithDefinitionMetadata(persisted)
	if !ok {
		t.Fatal("registered flag was rejected")
	}
	if got.Description != "Uses the canonical request-event model for Logs and API Metrics reads; activation is locked pending SDK, canonical live-publisher, historical Metrics coverage/parity, retention safety, and exact-SHA prerequisites." {
		t.Fatalf("description = %q", got.Description)
	}
	if got.Key != persisted.Key || got.Enabled != persisted.Enabled || got.UpdatedBy != persisted.UpdatedBy || !got.UpdatedAt.Equal(updatedAt) {
		t.Fatalf("persisted state changed: got %#v want %#v", got, persisted)
	}
	if _, ok := flagWithDefinitionMetadata(Flag{Key: "unknown", Description: "stale"}); ok {
		t.Fatal("unregistered database flag must remain hidden")
	}
}

func TestObservabilityReadFlagUsesGlobalEvaluatorWithoutSuperAdminBypass(t *testing.T) {
	store := &fakeStore{
		enabled: map[string]bool{ObservabilityReadsV2: false},
		owner:   "super-admin-owner",
	}
	evaluator := NewEvaluator(store, fakeSuperAdminChecker{"super-admin-owner": true})

	got, err := evaluator.Public(context.Background(), ObservabilityReadsV2)
	if err != nil {
		t.Fatal(err)
	}
	if got {
		t.Fatal("global observability read flag must remain OFF for Super Admin workspaces")
	}
}

func TestInternalObservabilityFlagIsNotExposedToCustomerFeatureMaps(t *testing.T) {
	store := &fakeStore{
		enabled: map[string]bool{ObservabilityReadsV2: true},
		owner:   "owner-1",
	}
	evaluator := NewEvaluator(store, fakeSuperAdminChecker{})

	publicFlags, err := evaluator.PublicFlags(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	workspaceFlags, err := evaluator.WorkspaceFlags(context.Background(), "workspace-1")
	if err != nil {
		t.Fatal(err)
	}
	if _, exposed := publicFlags[ObservabilityReadsV2]; exposed {
		t.Fatal("internal observability flag leaked through public features")
	}
	if _, exposed := workspaceFlags[ObservabilityReadsV2]; exposed {
		t.Fatal("internal observability flag leaked through workspace features")
	}
}

func TestEvaluatorForWorkspace(t *testing.T) {
	tests := []struct {
		name       string
		global     bool
		superAdmin bool
		want       bool
	}{
		{name: "regular workspace global off", global: false, want: false},
		{name: "regular workspace global on", global: true, want: true},
		{name: "super admin workspace global off", global: false, superAdmin: true, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &fakeStore{enabled: map[string]bool{XDMSV1: tt.global}, owner: "owner_1"}
			evaluator := NewEvaluator(store, fakeSuperAdminChecker{"owner_1": tt.superAdmin})
			got, err := evaluator.ForWorkspace(context.Background(), "workspace_1", XDMSV1)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Fatalf("ForWorkspace() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEvaluatorRejectsUnknownFlag(t *testing.T) {
	store := &fakeStore{enabled: map[string]bool{}, owner: "owner_1"}
	evaluator := NewEvaluator(store, fakeSuperAdminChecker{})
	if _, err := evaluator.ForWorkspace(context.Background(), "workspace_1", "unknown"); !errors.Is(err, ErrUnknownFlag) {
		t.Fatalf("ForWorkspace() error = %v, want ErrUnknownFlag", err)
	}
	if _, err := store.Set(context.Background(), "unknown", true, "actor_1"); !errors.Is(err, ErrUnknownFlag) {
		t.Fatalf("Set() error = %v, want ErrUnknownFlag", err)
	}
}
