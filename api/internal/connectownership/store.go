package connectownership

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/xiaoboyu/unipost-api/internal/db"
)

type OwnershipConflictError struct{}

func (*OwnershipConflictError) Error() string {
	return "ACCOUNT_OWNERSHIP_CONFLICT"
}

var ErrOwnershipConflict = &OwnershipConflictError{}

type DecisionKind string

const (
	Create    DecisionKind = "create"
	Reconnect DecisionKind = "reconnect"
	Conflict  DecisionKind = "conflict"
)

type Decision struct {
	Kind      DecisionKind
	AccountID string
}

type OwnershipKey struct {
	WorkspaceID      string
	ProfileID        string
	Platform         string
	ProviderIdentity string
	ExternalUserID   string
}

type SaveRequest struct {
	WorkspaceID      string
	ProfileID        string
	Platform         string
	ProviderIdentity string
	ExternalUserID   string
	Refresh          db.RefreshConnectedSocialAccountParams
	Upsert           db.UpsertManagedSocialAccountParams
	Create           db.CreateManagedSocialAccountParams
}

type ownershipQueries interface {
	ListActiveAccountsByWorkspaceProviderIdentity(context.Context, db.ListActiveAccountsByWorkspaceProviderIdentityParams) ([]db.SocialAccount, error)
	RefreshConnectedSocialAccount(context.Context, db.RefreshConnectedSocialAccountParams) (db.SocialAccount, error)
	UpsertManagedSocialAccount(context.Context, db.UpsertManagedSocialAccountParams) (db.SocialAccount, error)
	CreateManagedSocialAccount(context.Context, db.CreateManagedSocialAccountParams) (db.SocialAccount, error)
}

type ownershipTx interface {
	db.DBTX
	Commit(context.Context) error
	Rollback(context.Context) error
}

type Store struct {
	queries    ownershipQueries
	beginTx    func(context.Context) (ownershipTx, error)
	queriesFor func(db.DBTX) ownershipQueries
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{
		queries: db.New(pool),
		beginTx: func(ctx context.Context) (ownershipTx, error) {
			return pool.BeginTx(ctx, pgx.TxOptions{})
		},
		queriesFor: func(tx db.DBTX) ownershipQueries {
			return db.New(tx)
		},
	}
}

func (s *Store) Check(ctx context.Context, key OwnershipKey) (Decision, error) {
	matches, err := s.queries.ListActiveAccountsByWorkspaceProviderIdentity(ctx, ownershipLookupParams(
		key.WorkspaceID,
		key.Platform,
		key.ProviderIdentity,
	))
	if err != nil {
		return Decision{}, fmt.Errorf("check connect account ownership: %w", err)
	}
	return decide(matches, key.ProfileID, key.ExternalUserID), nil
}

func (s *Store) Save(ctx context.Context, request SaveRequest) (db.SocialAccount, error) {
	tx, err := s.beginTx(ctx)
	if err != nil {
		return db.SocialAccount{}, fmt.Errorf("begin connect account ownership save: %w", err)
	}
	defer tx.Rollback(ctx)

	lockValue := request.WorkspaceID + "\x00" + request.Platform + "\x00" + request.ProviderIdentity
	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1, 0))", lockValue); err != nil {
		return db.SocialAccount{}, fmt.Errorf("lock connect account ownership: %w", err)
	}

	queries := s.queriesFor(tx)
	matches, err := queries.ListActiveAccountsByWorkspaceProviderIdentity(ctx, ownershipLookupParams(
		request.WorkspaceID,
		request.Platform,
		request.ProviderIdentity,
	))
	if err != nil {
		return db.SocialAccount{}, fmt.Errorf("load connect account ownership: %w", err)
	}

	decision := decide(matches, request.ProfileID, request.ExternalUserID)
	account, err := applyDecision(ctx, queries, decision, request)
	if err != nil {
		return db.SocialAccount{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return db.SocialAccount{}, fmt.Errorf("commit connect account ownership: %w", err)
	}
	return account, nil
}

func ownershipLookupParams(workspaceID, platform, providerIdentity string) db.ListActiveAccountsByWorkspaceProviderIdentityParams {
	return db.ListActiveAccountsByWorkspaceProviderIdentityParams{
		WorkspaceID:      workspaceID,
		Platform:         platform,
		ProviderIdentity: providerIdentity,
	}
}

func decide(matches []db.SocialAccount, profileID, externalUserID string) Decision {
	if len(matches) == 0 {
		return Decision{Kind: Create}
	}
	if len(matches) != 1 {
		return Decision{Kind: Conflict}
	}

	match := matches[0]
	if match.ProfileID != profileID ||
		!match.ExternalUserID.Valid ||
		match.ExternalUserID.String == "" ||
		match.ExternalUserID.String != externalUserID {
		return Decision{Kind: Conflict}
	}

	return Decision{Kind: Reconnect, AccountID: match.ID}
}

func applyDecision(
	ctx context.Context,
	queries ownershipQueries,
	decision Decision,
	request SaveRequest,
) (db.SocialAccount, error) {
	switch decision.Kind {
	case Reconnect:
		request.Refresh.ID = decision.AccountID
		return queries.RefreshConnectedSocialAccount(ctx, request.Refresh)
	case Create:
		if request.Platform == "bluesky" {
			return queries.CreateManagedSocialAccount(ctx, request.Create)
		}
		return queries.UpsertManagedSocialAccount(ctx, request.Upsert)
	default:
		return db.SocialAccount{}, ErrOwnershipConflict
	}
}
