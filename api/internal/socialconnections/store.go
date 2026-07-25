package socialconnections

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/xiaoboyu/unipost-api/internal/db"
)

var (
	ErrAlreadyConnected        = errors.New("social account is already connected")
	ErrOwnershipConflict       = errors.New("social connection ownership conflict")
	ErrInvalidCredentialInput  = errors.New("invalid social connection input")
	ErrProfileNotInWorkspace   = errors.New("profile is not in workspace")
	ErrLegacyBinding           = errors.New("legacy social account has no shareable connection")
	ErrBindingNotFound         = errors.New("social account binding not found")
	ErrLifecycleNotImplemented = errors.New("social connection lifecycle is not implemented")
)

type Ownership struct {
	ConnectionType    string
	ExternalUserID    string
	ExternalUserEmail string
}

type CredentialInput struct {
	WorkspaceID       string
	ProfileID         string
	Platform          string
	ProviderIdentity  string
	ExternalAccountID string
	AccessToken       string
	RefreshToken      string
	AccountName       string
	AvatarURL         string
	Scopes            []string
	Metadata          []byte
	TokenExpiresAt    time.Time
	XAppMode          string
	ConnectSessionID  string
	Ownership         Ownership
}

type SaveMode int

const (
	SaveDirectCreate SaveMode = iota
	SaveOAuthReuse
	SaveManagedReuse
)

type Store interface {
	SaveVerified(context.Context, SaveMode, CredentialInput) (db.SocialAccount, error)
	BindExisting(context.Context, string, string, string, string) (db.SocialAccount, error)
	Unbind(context.Context, string, string) error
	Disconnect(context.Context, string, string) ([]db.SocialAccount, error)
}

type connectionQueries interface {
	FindCanonicalSocialConnectionForUpdate(context.Context, db.FindCanonicalSocialConnectionForUpdateParams) (db.SocialConnection, error)
	ListActiveAccountsByWorkspaceProviderIdentity(context.Context, db.ListActiveAccountsByWorkspaceProviderIdentityParams) ([]db.SocialAccount, error)
	CreateSocialConnection(context.Context, db.CreateSocialConnectionParams) (db.SocialConnection, error)
	RefreshSocialConnection(context.Context, db.RefreshSocialConnectionParams) (db.SocialConnection, error)
	CreateOrReactivateSocialAccountBinding(context.Context, db.CreateOrReactivateSocialAccountBindingParams) (db.SocialAccount, error)
	GetResolvedSocialAccountByIDAndWorkspace(context.Context, db.GetResolvedSocialAccountByIDAndWorkspaceParams) (db.GetResolvedSocialAccountByIDAndWorkspaceRow, error)
	GetSocialConnectionForUpdate(context.Context, db.GetSocialConnectionForUpdateParams) (db.SocialConnection, error)
	GetProfile(context.Context, string) (db.Profile, error)
}

type connectionTx interface {
	db.DBTX
	Commit(context.Context) error
	Rollback(context.Context) error
}

type PostgresStore struct {
	beginTx    func(context.Context) (connectionTx, error)
	queriesFor func(db.DBTX) connectionQueries
}

func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{
		beginTx: func(ctx context.Context) (connectionTx, error) {
			return pool.BeginTx(ctx, pgx.TxOptions{})
		},
		queriesFor: func(tx db.DBTX) connectionQueries { return db.New(tx) },
	}
}

func (s *PostgresStore) SaveVerified(ctx context.Context, mode SaveMode, input CredentialInput) (db.SocialAccount, error) {
	input, err := normalizeCredentialInput(mode, input)
	if err != nil {
		return db.SocialAccount{}, err
	}

	tx, err := s.beginTx(ctx)
	if err != nil {
		return db.SocialAccount{}, fmt.Errorf("begin social connection save: %w", err)
	}
	defer tx.Rollback(ctx)
	queries := s.queriesFor(tx)

	if err := requireProfileInWorkspace(ctx, queries, input.ProfileID, input.WorkspaceID); err != nil {
		return db.SocialAccount{}, err
	}
	if err := lockCanonicalIdentity(ctx, tx, input.WorkspaceID, input.Platform, input.ProviderIdentity); err != nil {
		return db.SocialAccount{}, err
	}

	connection, err := queries.FindCanonicalSocialConnectionForUpdate(ctx, db.FindCanonicalSocialConnectionForUpdateParams{
		WorkspaceID:      input.WorkspaceID,
		Platform:         input.Platform,
		ProviderIdentity: textValue(input.ProviderIdentity),
	})
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		legacyMatches, lookupErr := queries.ListActiveAccountsByWorkspaceProviderIdentity(ctx, db.ListActiveAccountsByWorkspaceProviderIdentityParams{
			WorkspaceID: input.WorkspaceID, Platform: input.Platform, ProviderIdentity: input.ProviderIdentity,
		})
		if lookupErr != nil {
			return db.SocialAccount{}, fmt.Errorf("check legacy social account identity: %w", lookupErr)
		}
		for _, match := range legacyMatches {
			if !match.ConnectionID.Valid || strings.TrimSpace(match.ConnectionID.String) == "" {
				return db.SocialAccount{}, ErrLegacyBinding
			}
		}
		if len(legacyMatches) != 0 {
			return db.SocialAccount{}, ErrOwnershipConflict
		}
		connection, err = queries.CreateSocialConnection(ctx, createConnectionParams(input))
		if err != nil {
			return db.SocialAccount{}, fmt.Errorf("create social connection: %w", err)
		}
	case err != nil:
		return db.SocialAccount{}, fmt.Errorf("find canonical social connection: %w", err)
	default:
		if mode == SaveDirectCreate {
			return db.SocialAccount{}, ErrAlreadyConnected
		}
		if err := requireCompatibleOwnership(connection, input.Ownership); err != nil {
			return db.SocialAccount{}, err
		}
		stableConnectionID := connection.ID
		connection, err = queries.RefreshSocialConnection(ctx, refreshConnectionParams(stableConnectionID, input))
		if err != nil {
			return db.SocialAccount{}, fmt.Errorf("refresh social connection: %w", err)
		}
		if connection.ID == "" {
			connection.ID = stableConnectionID
		}
	}

	account, err := queries.CreateOrReactivateSocialAccountBinding(ctx, bindingParams(connection.ID, input))
	if err != nil {
		return db.SocialAccount{}, fmt.Errorf("create social account binding: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return db.SocialAccount{}, fmt.Errorf("commit social connection save: %w", err)
	}
	return account, nil
}

func (s *PostgresStore) BindExisting(
	ctx context.Context,
	workspaceID string,
	sourceAccountID string,
	targetProfileID string,
	selectedExternalUserID string,
) (db.SocialAccount, error) {
	workspaceID = strings.TrimSpace(workspaceID)
	sourceAccountID = strings.TrimSpace(sourceAccountID)
	targetProfileID = strings.TrimSpace(targetProfileID)
	selectedExternalUserID = strings.TrimSpace(selectedExternalUserID)
	if workspaceID == "" || sourceAccountID == "" || targetProfileID == "" {
		return db.SocialAccount{}, ErrInvalidCredentialInput
	}

	tx, err := s.beginTx(ctx)
	if err != nil {
		return db.SocialAccount{}, fmt.Errorf("begin social connection bind: %w", err)
	}
	defer tx.Rollback(ctx)
	queries := s.queriesFor(tx)

	source, err := queries.GetResolvedSocialAccountByIDAndWorkspace(ctx, db.GetResolvedSocialAccountByIDAndWorkspaceParams{
		ID: sourceAccountID, WorkspaceID: workspaceID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return db.SocialAccount{}, ErrBindingNotFound
	}
	if err != nil {
		return db.SocialAccount{}, fmt.Errorf("resolve source social account: %w", err)
	}
	if source.BindingStatus != "active" {
		return db.SocialAccount{}, ErrBindingNotFound
	}
	if !source.ConnectionID.Valid || strings.TrimSpace(source.ConnectionID.String) == "" {
		return db.SocialAccount{}, ErrLegacyBinding
	}

	connection, err := queries.GetSocialConnectionForUpdate(ctx, db.GetSocialConnectionForUpdateParams{
		ID: source.ConnectionID.String, WorkspaceID: workspaceID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return db.SocialAccount{}, ErrBindingNotFound
	}
	if err != nil {
		return db.SocialAccount{}, fmt.Errorf("lock source social connection: %w", err)
	}
	if err := requireSelectedOwner(connection, selectedExternalUserID); err != nil {
		return db.SocialAccount{}, err
	}
	if err := requireProfileInWorkspace(ctx, queries, targetProfileID, workspaceID); err != nil {
		return db.SocialAccount{}, err
	}

	input := CredentialInput{
		WorkspaceID: workspaceID, ProfileID: targetProfileID, Platform: source.Platform,
		ProviderIdentity: connection.ProviderIdentity.String, ExternalAccountID: source.ExternalAccountID,
		AccessToken: connection.AccessToken, RefreshToken: connection.RefreshToken.String,
		AccountName: connection.AccountName.String, AvatarURL: connection.AccountAvatarUrl.String,
		Scopes: connection.Scope, Metadata: connection.Metadata,
		TokenExpiresAt: timestampValue(connection.TokenExpiresAt), XAppMode: connection.XAppMode.String,
		Ownership: Ownership{
			ConnectionType:    connection.ConnectionType,
			ExternalUserID:    connection.ExternalUserID.String,
			ExternalUserEmail: connection.ExternalUserEmail.String,
		},
	}
	account, err := queries.CreateOrReactivateSocialAccountBinding(ctx, bindingParams(connection.ID, input))
	if err != nil {
		return db.SocialAccount{}, fmt.Errorf("create sibling social account binding: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return db.SocialAccount{}, fmt.Errorf("commit social connection bind: %w", err)
	}
	return account, nil
}

func (*PostgresStore) Unbind(context.Context, string, string) error {
	return ErrLifecycleNotImplemented
}

func (*PostgresStore) Disconnect(context.Context, string, string) ([]db.SocialAccount, error) {
	return nil, ErrLifecycleNotImplemented
}

func normalizeCredentialInput(mode SaveMode, input CredentialInput) (CredentialInput, error) {
	input.WorkspaceID = strings.TrimSpace(input.WorkspaceID)
	input.ProfileID = strings.TrimSpace(input.ProfileID)
	input.Platform = strings.ToLower(strings.TrimSpace(input.Platform))
	input.ProviderIdentity = strings.TrimSpace(input.ProviderIdentity)
	input.ExternalAccountID = strings.TrimSpace(input.ExternalAccountID)
	input.Ownership.ConnectionType = strings.ToLower(strings.TrimSpace(input.Ownership.ConnectionType))
	input.Ownership.ExternalUserID = strings.TrimSpace(input.Ownership.ExternalUserID)
	input.Ownership.ExternalUserEmail = strings.TrimSpace(input.Ownership.ExternalUserEmail)
	if input.ExternalAccountID == "" && input.Platform != "instagram" {
		input.ExternalAccountID = input.ProviderIdentity
	}

	if input.WorkspaceID == "" || input.ProfileID == "" || input.Platform == "" ||
		input.ProviderIdentity == "" || input.ExternalAccountID == "" || input.AccessToken == "" {
		return CredentialInput{}, ErrInvalidCredentialInput
	}
	if mode < SaveDirectCreate || mode > SaveManagedReuse {
		return CredentialInput{}, ErrInvalidCredentialInput
	}
	if input.Ownership.ConnectionType == "managed" {
		if input.Ownership.ExternalUserID == "" || mode != SaveManagedReuse {
			return CredentialInput{}, ErrInvalidCredentialInput
		}
	} else if input.Ownership.ConnectionType == "byo" {
		if input.Ownership.ExternalUserID != "" || mode == SaveManagedReuse {
			return CredentialInput{}, ErrInvalidCredentialInput
		}
	} else {
		return CredentialInput{}, ErrInvalidCredentialInput
	}
	return input, nil
}

func requireCompatibleOwnership(connection db.SocialConnection, ownership Ownership) error {
	if connection.ConnectionType != ownership.ConnectionType {
		return ErrOwnershipConflict
	}
	if ownership.ConnectionType == "managed" {
		if !connection.ExternalUserID.Valid || connection.ExternalUserID.String != ownership.ExternalUserID {
			return ErrOwnershipConflict
		}
	} else if connection.ExternalUserID.Valid && strings.TrimSpace(connection.ExternalUserID.String) != "" {
		return ErrOwnershipConflict
	}
	return nil
}

func requireSelectedOwner(connection db.SocialConnection, selectedExternalUserID string) error {
	if connection.ConnectionType == "managed" {
		if !connection.ExternalUserID.Valid || connection.ExternalUserID.String == "" ||
			connection.ExternalUserID.String != selectedExternalUserID {
			return ErrOwnershipConflict
		}
		return nil
	}
	if selectedExternalUserID != "" {
		return ErrOwnershipConflict
	}
	return nil
}

func requireProfileInWorkspace(ctx context.Context, queries connectionQueries, profileID, workspaceID string) error {
	profile, err := queries.GetProfile(ctx, profileID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrProfileNotInWorkspace
	}
	if err != nil {
		return fmt.Errorf("load social connection profile: %w", err)
	}
	if profile.WorkspaceID != workspaceID {
		return ErrProfileNotInWorkspace
	}
	return nil
}

func lockCanonicalIdentity(ctx context.Context, tx connectionTx, workspaceID, platform, providerIdentity string) error {
	if _, err := tx.Exec(ctx,
		"SELECT pg_advisory_xact_lock(hashtextextended($1, 0))",
		canonicalLockKey(workspaceID, platform, providerIdentity),
	); err != nil {
		return fmt.Errorf("lock canonical social connection: %w", err)
	}
	return nil
}

func canonicalLockKey(components ...string) string {
	var encoded strings.Builder
	for _, component := range components {
		encoded.WriteString(strconv.Itoa(len(component)))
		encoded.WriteByte(':')
		encoded.WriteString(hex.EncodeToString([]byte(component)))
		encoded.WriteByte(';')
	}
	return encoded.String()
}

func createConnectionParams(input CredentialInput) db.CreateSocialConnectionParams {
	return db.CreateSocialConnectionParams{
		WorkspaceID: input.WorkspaceID, Platform: input.Platform,
		ProviderIdentity: textValue(input.ProviderIdentity), AccessToken: input.AccessToken,
		RefreshToken: nullableText(input.RefreshToken), TokenExpiresAt: timestamp(input.TokenExpiresAt),
		AccountName: nullableText(input.AccountName), AccountAvatarUrl: nullableText(input.AvatarURL),
		Metadata: input.Metadata, Scope: input.Scopes, ConnectionType: input.Ownership.ConnectionType,
		ExternalUserID:    nullableText(input.Ownership.ExternalUserID),
		ExternalUserEmail: nullableText(input.Ownership.ExternalUserEmail), XAppMode: nullableText(input.XAppMode),
	}
}

func refreshConnectionParams(connectionID string, input CredentialInput) db.RefreshSocialConnectionParams {
	return db.RefreshSocialConnectionParams{
		AccessToken: input.AccessToken, RefreshToken: nullableText(input.RefreshToken),
		TokenExpiresAt: timestamp(input.TokenExpiresAt), AccountName: nullableText(input.AccountName),
		AccountAvatarUrl: nullableText(input.AvatarURL), Metadata: input.Metadata, Scope: input.Scopes,
		ExternalUserEmail: nullableText(input.Ownership.ExternalUserEmail), XAppMode: nullableText(input.XAppMode),
		ID: connectionID, WorkspaceID: input.WorkspaceID,
	}
}

func bindingParams(connectionID string, input CredentialInput) db.CreateOrReactivateSocialAccountBindingParams {
	return db.CreateOrReactivateSocialAccountBindingParams{
		ProfileID: input.ProfileID, Platform: input.Platform, LegacyAccessToken: input.AccessToken,
		LegacyRefreshToken: nullableText(input.RefreshToken), TokenExpiresAt: timestamp(input.TokenExpiresAt),
		ExternalAccountID: input.ExternalAccountID, AccountName: nullableText(input.AccountName),
		AccountAvatarUrl: nullableText(input.AvatarURL), Metadata: input.Metadata, Scope: input.Scopes,
		ConnectionType: input.Ownership.ConnectionType, ConnectSessionID: nullableText(input.ConnectSessionID),
		ExternalUserID:    nullableText(input.Ownership.ExternalUserID),
		ExternalUserEmail: nullableText(input.Ownership.ExternalUserEmail), XAppMode: nullableText(input.XAppMode),
		ConnectionID: textValue(connectionID),
	}
}

func nullableText(value string) pgtype.Text {
	value = strings.TrimSpace(value)
	return pgtype.Text{String: value, Valid: value != ""}
}

func textValue(value string) pgtype.Text {
	return pgtype.Text{String: value, Valid: true}
}

func timestamp(value time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: value, Valid: !value.IsZero()}
}

func timestampValue(value pgtype.Timestamptz) time.Time {
	if !value.Valid {
		return time.Time{}
	}
	return value.Time
}
