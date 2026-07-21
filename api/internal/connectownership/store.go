package connectownership

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/xiaoboyu/unipost-api/internal/db"
)

type ConflictClass string

const (
	ConflictOwnerBYO            ConflictClass = "owner_byo"
	ConflictManagedUserMismatch ConflictClass = "managed_user_mismatch"
	ConflictProfileMismatch     ConflictClass = "profile_mismatch"
	ConflictAmbiguousMatches    ConflictClass = "ambiguous_matches"
	ConflictAuthoritative       ConflictClass = "authoritative_conflict"
)

type OwnershipConflictError struct {
	ConflictClass ConflictClass
	MatchCount    int
}

func (*OwnershipConflictError) Error() string {
	return "ACCOUNT_OWNERSHIP_CONFLICT"
}

func (*OwnershipConflictError) Is(target error) bool {
	_, ok := target.(*OwnershipConflictError)
	return ok
}

var ErrOwnershipConflict = &OwnershipConflictError{}

type InvalidOwnershipRequestError struct{}

func (*InvalidOwnershipRequestError) Error() string {
	return "INVALID_ACCOUNT_OWNERSHIP_REQUEST"
}

var ErrInvalidOwnershipRequest = &InvalidOwnershipRequestError{}

type DecisionKind string

const (
	Create    DecisionKind = "create"
	Reconnect DecisionKind = "reconnect"
	Conflict  DecisionKind = "conflict"
)

type Decision struct {
	Kind          DecisionKind
	AccountID     string
	ConflictClass ConflictClass
	MatchCount    int
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

type ownershipCheckQueries interface {
	CheckActiveAccountsByWorkspaceProviderIdentity(context.Context, db.CheckActiveAccountsByWorkspaceProviderIdentityParams) ([]db.SocialAccount, error)
}

type ownershipSaveQueries interface {
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
	queries    ownershipCheckQueries
	beginTx    func(context.Context) (ownershipTx, error)
	queriesFor func(db.DBTX) ownershipSaveQueries
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{
		queries: db.New(pool),
		beginTx: func(ctx context.Context) (ownershipTx, error) {
			return pool.BeginTx(ctx, pgx.TxOptions{})
		},
		queriesFor: func(tx db.DBTX) ownershipSaveQueries {
			return db.New(tx)
		},
	}
}

func (s *Store) Check(ctx context.Context, key OwnershipKey) (Decision, error) {
	matches, err := s.queries.CheckActiveAccountsByWorkspaceProviderIdentity(ctx, db.CheckActiveAccountsByWorkspaceProviderIdentityParams{
		WorkspaceID:      key.WorkspaceID,
		Platform:         key.Platform,
		ProviderIdentity: key.ProviderIdentity,
	})
	if err != nil {
		return Decision{}, fmt.Errorf("check connect account ownership: %w", err)
	}
	return decide(matches, key.ProfileID, key.ExternalUserID), nil
}

func (s *Store) Save(ctx context.Context, request SaveRequest) (db.SocialAccount, error) {
	normalized, err := normalizeSaveRequest(request)
	if err != nil {
		return db.SocialAccount{}, err
	}
	request = normalized

	tx, err := s.beginTx(ctx)
	if err != nil {
		return db.SocialAccount{}, fmt.Errorf("begin connect account ownership save: %w", err)
	}
	defer tx.Rollback(ctx)

	lockValue := connectOwnershipLockKey(request.WorkspaceID, request.Platform, request.ProviderIdentity)
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

func normalizeSaveRequest(request SaveRequest) (SaveRequest, error) {
	for _, required := range []string{
		request.WorkspaceID,
		request.ProfileID,
		request.Platform,
		request.ProviderIdentity,
		request.ExternalUserID,
	} {
		if strings.TrimSpace(required) == "" {
			return SaveRequest{}, ErrInvalidOwnershipRequest
		}
	}

	if !compatibleText(request.Refresh.ExternalUserID, request.ExternalUserID) ||
		(request.Refresh.ConnectionType != "" && request.Refresh.ConnectionType != "managed") {
		return SaveRequest{}, ErrInvalidOwnershipRequest
	}
	request.Refresh.ExternalUserID = managedUserID(request.ExternalUserID)
	request.Refresh.ConnectionType = "managed"
	if request.Platform == "instagram" {
		metadata, err := bindInstagramWebhookIdentity(request.Refresh.Metadata, request.ProviderIdentity)
		if err != nil {
			return SaveRequest{}, err
		}
		request.Refresh.Metadata = metadata
	} else {
		if !compatibleString(request.Refresh.ExternalAccountID, request.ProviderIdentity) {
			return SaveRequest{}, ErrInvalidOwnershipRequest
		}
		request.Refresh.ExternalAccountID = request.ProviderIdentity
	}

	if request.Platform == "bluesky" {
		if !compatibleString(request.Create.ProfileID, request.ProfileID) ||
			!compatibleString(request.Create.Platform, request.Platform) ||
			!compatibleString(request.Create.ExternalAccountID, request.ProviderIdentity) ||
			!compatibleText(request.Create.ExternalUserID, request.ExternalUserID) {
			return SaveRequest{}, ErrInvalidOwnershipRequest
		}
		request.Create.ProfileID = request.ProfileID
		request.Create.Platform = request.Platform
		request.Create.ExternalAccountID = request.ProviderIdentity
		request.Create.ExternalUserID = managedUserID(request.ExternalUserID)
		return request, nil
	}

	if !compatibleString(request.Upsert.ProfileID, request.ProfileID) ||
		!compatibleString(request.Upsert.Platform, request.Platform) ||
		!compatibleText(request.Upsert.ExternalUserID, request.ExternalUserID) {
		return SaveRequest{}, ErrInvalidOwnershipRequest
	}
	request.Upsert.ProfileID = request.ProfileID
	request.Upsert.Platform = request.Platform
	request.Upsert.ExternalUserID = managedUserID(request.ExternalUserID)
	if request.Platform == "instagram" {
		metadata, err := bindInstagramWebhookIdentity(request.Upsert.Metadata, request.ProviderIdentity)
		if err != nil {
			return SaveRequest{}, err
		}
		request.Upsert.Metadata = metadata
	} else {
		if !compatibleString(request.Upsert.ExternalAccountID, request.ProviderIdentity) {
			return SaveRequest{}, ErrInvalidOwnershipRequest
		}
		request.Upsert.ExternalAccountID = request.ProviderIdentity
	}
	return request, nil
}

func compatibleString(value, canonical string) bool {
	return value == "" || value == canonical
}

func compatibleText(value pgtype.Text, canonical string) bool {
	return !value.Valid || value.String == "" || value.String == canonical
}

func managedUserID(externalUserID string) pgtype.Text {
	return pgtype.Text{String: externalUserID, Valid: true}
}

func bindInstagramWebhookIdentity(metadata []byte, providerIdentity string) ([]byte, error) {
	object := make(map[string]json.RawMessage)
	if len(bytes.TrimSpace(metadata)) != 0 {
		if err := json.Unmarshal(metadata, &object); err != nil || object == nil {
			return nil, ErrInvalidOwnershipRequest
		}
	}

	const field = "instagram_webhook_user_id"
	if encoded, exists := object[field]; exists {
		var existing string
		if err := json.Unmarshal(encoded, &existing); err != nil || !compatibleString(existing, providerIdentity) {
			return nil, ErrInvalidOwnershipRequest
		}
	}
	encodedIdentity, err := json.Marshal(providerIdentity)
	if err != nil {
		return nil, ErrInvalidOwnershipRequest
	}
	object[field] = encodedIdentity
	normalized, err := json.Marshal(object)
	if err != nil {
		return nil, ErrInvalidOwnershipRequest
	}
	return normalized, nil
}

func connectOwnershipLockKey(workspaceID, platform, providerIdentity string) string {
	var encoded strings.Builder
	for _, component := range []string{workspaceID, platform, providerIdentity} {
		encoded.WriteString(strconv.Itoa(len(component)))
		encoded.WriteByte(':')
		encoded.WriteString(hex.EncodeToString([]byte(component)))
		encoded.WriteByte(';')
	}
	return encoded.String()
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
		return Decision{Kind: Conflict, ConflictClass: ConflictAmbiguousMatches, MatchCount: len(matches)}
	}

	match := matches[0]
	if match.ProfileID != profileID {
		return Decision{Kind: Conflict, ConflictClass: ConflictProfileMismatch, MatchCount: 1}
	}
	if !match.ExternalUserID.Valid || match.ExternalUserID.String == "" {
		return Decision{Kind: Conflict, ConflictClass: ConflictOwnerBYO, MatchCount: 1}
	}
	if match.ExternalUserID.String != externalUserID {
		return Decision{Kind: Conflict, ConflictClass: ConflictManagedUserMismatch, MatchCount: 1}
	}

	return Decision{Kind: Reconnect, AccountID: match.ID}
}

func applyDecision(
	ctx context.Context,
	queries ownershipSaveQueries,
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
		return db.SocialAccount{}, &OwnershipConflictError{
			ConflictClass: decision.ConflictClass,
			MatchCount:    decision.MatchCount,
		}
	}
}
