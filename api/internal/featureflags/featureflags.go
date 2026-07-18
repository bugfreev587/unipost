package featureflags

import (
	"context"
	"errors"
	"fmt"
	"time"
)

const (
	XDMSV1            = "x_dms_v1"
	XCreditsBillingV1 = "x_credits_billing_v1"
)

var ErrUnknownFlag = errors.New("unknown feature flag")

type Definition struct {
	Key            string `json:"key"`
	Label          string `json:"label"`
	Description    string `json:"description"`
	OwnerArea      string `json:"owner_area"`
	DefaultEnabled bool   `json:"default_enabled"`
}

var definitions = []Definition{
	{
		Key:         XDMSV1,
		Label:       "X DMs",
		Description: "Makes X direct messages available to regular users.",
		OwnerArea:   "X Inbox",
	},
	{
		Key:         XCreditsBillingV1,
		Label:       "X Credits billing",
		Description: "Counts managed X API operations against customer X Credits.",
		OwnerArea:   "Billing",
	},
}

func Definitions() []Definition {
	result := make([]Definition, len(definitions))
	copy(result, definitions)
	return result
}

func DefinitionFor(key string) (Definition, bool) {
	for _, definition := range definitions {
		if definition.Key == key {
			return definition, true
		}
	}
	return Definition{}, false
}

type Flag struct {
	Key         string    `json:"key"`
	Enabled     bool      `json:"enabled"`
	Description string    `json:"description"`
	UpdatedBy   string    `json:"updated_by"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Store interface {
	List(context.Context) ([]Flag, error)
	Set(context.Context, string, bool, string) (Flag, error)
	GlobalEnabled(context.Context, string) (bool, error)
	WorkspaceOwner(context.Context, string) (string, error)
}

type SuperAdminChecker interface {
	IsSuperAdmin(context.Context, string) bool
}

type Evaluator struct {
	store       Store
	superAdmins SuperAdminChecker
}

func NewEvaluator(store Store, superAdmins SuperAdminChecker) *Evaluator {
	return &Evaluator{store: store, superAdmins: superAdmins}
}

func (e *Evaluator) ForWorkspace(ctx context.Context, workspaceID, key string) (bool, error) {
	if _, ok := DefinitionFor(key); !ok {
		return false, fmt.Errorf("%w: %s", ErrUnknownFlag, key)
	}
	if e == nil || e.store == nil {
		return false, errors.New("feature flag evaluator is not configured")
	}
	enabled, err := e.store.GlobalEnabled(ctx, key)
	if err != nil {
		return false, err
	}
	if enabled {
		return true, nil
	}
	ownerID, err := e.store.WorkspaceOwner(ctx, workspaceID)
	if err != nil {
		return false, err
	}
	return e.superAdmins != nil && e.superAdmins.IsSuperAdmin(ctx, ownerID), nil
}

func (e *Evaluator) Public(ctx context.Context, key string) (bool, error) {
	if _, ok := DefinitionFor(key); !ok {
		return false, fmt.Errorf("%w: %s", ErrUnknownFlag, key)
	}
	if e == nil || e.store == nil {
		return false, errors.New("feature flag evaluator is not configured")
	}
	return e.store.GlobalEnabled(ctx, key)
}

func (e *Evaluator) WorkspaceFlags(ctx context.Context, workspaceID string) (map[string]bool, error) {
	flags := make(map[string]bool, len(definitions))
	for _, definition := range definitions {
		enabled, err := e.ForWorkspace(ctx, workspaceID, definition.Key)
		if err != nil {
			return nil, err
		}
		flags[definition.Key] = enabled
	}
	return flags, nil
}

func (e *Evaluator) PublicFlags(ctx context.Context) (map[string]bool, error) {
	flags := make(map[string]bool, len(definitions))
	for _, definition := range definitions {
		enabled, err := e.Public(ctx, definition.Key)
		if err != nil {
			return nil, err
		}
		flags[definition.Key] = enabled
	}
	return flags, nil
}
