package socialconnections

import (
	"context"
	"encoding/hex"
	"encoding/json"
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
	ErrReconnectRequired       = errors.New("social connection must be reconnected before binding")
	ErrReconnectTargetConflict = errors.New("reconnect target does not match verified social identity")
)

type Ownership struct {
	ConnectionType    string
	ExternalUserID    string
	ExternalUserEmail string
}

type CredentialInput struct {
	WorkspaceID        string
	ProfileID          string
	Platform           string
	ProviderIdentity   string
	ExternalAccountID  string
	AccessToken        string
	RefreshToken       string
	AccountName        string
	AvatarURL          string
	Scopes             []string
	Metadata           []byte
	TokenExpiresAt     time.Time
	XAppMode           string
	ConnectSessionID   string
	ReconnectAccountID string
	Ownership          Ownership
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
	GetReconnectRequiredSocialAccountForUpdate(context.Context, db.GetReconnectRequiredSocialAccountForUpdateParams) (db.SocialAccount, error)
	RecoverReconnectRequiredSocialAccountBinding(context.Context, db.RecoverReconnectRequiredSocialAccountBindingParams) (db.SocialAccount, error)
	ResolveMigrationConflictsForRecoveredAccount(context.Context, db.ResolveMigrationConflictsForRecoveredAccountParams) error
	GetResolvedSocialAccountByIDAndWorkspace(context.Context, db.GetResolvedSocialAccountByIDAndWorkspaceParams) (db.GetResolvedSocialAccountByIDAndWorkspaceRow, error)
	GetSocialConnectionForUpdate(context.Context, db.GetSocialConnectionForUpdateParams) (db.SocialConnection, error)
	GetProfile(context.Context, string) (db.Profile, error)
	UnbindSocialAccountBinding(context.Context, db.UnbindSocialAccountBindingParams) (db.SocialAccount, error)
	DisconnectSocialConnection(context.Context, db.DisconnectSocialConnectionParams) (db.SocialConnection, error)
	DisconnectAllSocialAccountBindings(context.Context, pgtype.Text) ([]db.SocialAccount, error)
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

	var reconnectTarget db.SocialAccount
	if input.ReconnectAccountID != "" {
		reconnectTarget, err = queries.GetReconnectRequiredSocialAccountForUpdate(ctx, db.GetReconnectRequiredSocialAccountForUpdateParams{
			ReconnectAccountID: input.ReconnectAccountID,
			WorkspaceID:        input.WorkspaceID, ProfileID: input.ProfileID, Platform: input.Platform,
		})
		if errors.Is(err, pgx.ErrNoRows) {
			return db.SocialAccount{}, ErrReconnectTargetConflict
		}
		if err != nil {
			return db.SocialAccount{}, fmt.Errorf("lock reconnect target: %w", err)
		}
		if err := requireReconnectTargetMatches(reconnectTarget, input); err != nil {
			return db.SocialAccount{}, err
		}
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
			if input.ReconnectAccountID != "" && match.ID == input.ReconnectAccountID {
				continue
			}
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
		if mode == SaveDirectCreate && input.ReconnectAccountID == "" {
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

	var account db.SocialAccount
	if input.ReconnectAccountID != "" {
		account, err = queries.RecoverReconnectRequiredSocialAccountBinding(ctx, recoveryParams(connection.ID, input))
		if errors.Is(err, pgx.ErrNoRows) {
			return db.SocialAccount{}, ErrReconnectTargetConflict
		}
		if err != nil {
			return db.SocialAccount{}, fmt.Errorf("recover social account binding: %w", err)
		}
		if err := queries.ResolveMigrationConflictsForRecoveredAccount(ctx, db.ResolveMigrationConflictsForRecoveredAccountParams{
			ReconnectAccountID: input.ReconnectAccountID, ConnectionID: connection.ID,
			WorkspaceID: input.WorkspaceID, Platform: input.Platform,
		}); err != nil {
			return db.SocialAccount{}, fmt.Errorf("resolve social connection migration evidence: %w", err)
		}
	} else {
		account, err = queries.CreateOrReactivateSocialAccountBinding(ctx, bindingParams(connection.ID, input))
		if err != nil {
			return db.SocialAccount{}, fmt.Errorf("create social account binding: %w", err)
		}
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
	if connection.Status != "active" || connection.DisconnectedAt.Valid {
		return db.SocialAccount{}, ErrReconnectRequired
	}
	if connection.ConnectionType == "managed" {
		if !connection.ExternalUserID.Valid || strings.TrimSpace(connection.ExternalUserID.String) == "" ||
			selectedExternalUserID != strings.TrimSpace(connection.ExternalUserID.String) {
			return db.SocialAccount{}, ErrOwnershipConflict
		}
	} else if selectedExternalUserID != "" {
		return db.SocialAccount{}, ErrOwnershipConflict
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

func (s *PostgresStore) Unbind(ctx context.Context, workspaceID, accountID string) error {
	workspaceID = strings.TrimSpace(workspaceID)
	accountID = strings.TrimSpace(accountID)
	if workspaceID == "" || accountID == "" {
		return ErrInvalidCredentialInput
	}
	tx, err := s.beginTx(ctx)
	if err != nil {
		return fmt.Errorf("begin social account unbind: %w", err)
	}
	defer tx.Rollback(ctx)
	queries := s.queriesFor(tx)
	source, connectionID, err := lockBindingConnection(ctx, queries, workspaceID, accountID)
	if err != nil {
		return err
	}
	if source.BindingStatus != "active" {
		return ErrBindingNotFound
	}
	if _, err := queries.UnbindSocialAccountBinding(ctx, db.UnbindSocialAccountBindingParams{
		ID: accountID, WorkspaceID: workspaceID, ConnectionID: textValue(connectionID),
	}); errors.Is(err, pgx.ErrNoRows) {
		return ErrBindingNotFound
	} else if err != nil {
		return fmt.Errorf("unbind social account: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit social account unbind: %w", err)
	}
	return nil
}

func (s *PostgresStore) Disconnect(ctx context.Context, workspaceID, accountID string) ([]db.SocialAccount, error) {
	workspaceID = strings.TrimSpace(workspaceID)
	accountID = strings.TrimSpace(accountID)
	if workspaceID == "" || accountID == "" {
		return nil, ErrInvalidCredentialInput
	}
	tx, err := s.beginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin social connection disconnect: %w", err)
	}
	defer tx.Rollback(ctx)
	queries := s.queriesFor(tx)
	_, connectionID, err := lockBindingConnection(ctx, queries, workspaceID, accountID)
	if err != nil {
		return nil, err
	}
	if _, err := queries.DisconnectSocialConnection(ctx, db.DisconnectSocialConnectionParams{
		ID: connectionID, WorkspaceID: workspaceID,
	}); errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrBindingNotFound
	} else if err != nil {
		return nil, fmt.Errorf("disconnect social connection: %w", err)
	}
	affected, err := queries.DisconnectAllSocialAccountBindings(ctx, textValue(connectionID))
	if err != nil {
		return nil, fmt.Errorf("disconnect social account bindings: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit social connection disconnect: %w", err)
	}
	return affected, nil
}

func lockBindingConnection(
	ctx context.Context,
	queries connectionQueries,
	workspaceID string,
	accountID string,
) (db.GetResolvedSocialAccountByIDAndWorkspaceRow, string, error) {
	source, err := queries.GetResolvedSocialAccountByIDAndWorkspace(ctx, db.GetResolvedSocialAccountByIDAndWorkspaceParams{
		ID: accountID, WorkspaceID: workspaceID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return db.GetResolvedSocialAccountByIDAndWorkspaceRow{}, "", ErrBindingNotFound
	}
	if err != nil {
		return db.GetResolvedSocialAccountByIDAndWorkspaceRow{}, "", fmt.Errorf("resolve social account binding: %w", err)
	}
	if !source.ConnectionID.Valid || strings.TrimSpace(source.ConnectionID.String) == "" {
		return db.GetResolvedSocialAccountByIDAndWorkspaceRow{}, "", ErrLegacyBinding
	}
	connectionID := strings.TrimSpace(source.ConnectionID.String)
	if _, err := queries.GetSocialConnectionForUpdate(ctx, db.GetSocialConnectionForUpdateParams{
		ID: connectionID, WorkspaceID: workspaceID,
	}); errors.Is(err, pgx.ErrNoRows) {
		return db.GetResolvedSocialAccountByIDAndWorkspaceRow{}, "", ErrBindingNotFound
	} else if err != nil {
		return db.GetResolvedSocialAccountByIDAndWorkspaceRow{}, "", fmt.Errorf("lock social connection: %w", err)
	}
	return source, connectionID, nil
}

func normalizeCredentialInput(mode SaveMode, input CredentialInput) (CredentialInput, error) {
	input.WorkspaceID = strings.TrimSpace(input.WorkspaceID)
	input.ProfileID = strings.TrimSpace(input.ProfileID)
	input.Platform = strings.ToLower(strings.TrimSpace(input.Platform))
	input.ProviderIdentity = strings.TrimSpace(input.ProviderIdentity)
	input.ExternalAccountID = strings.TrimSpace(input.ExternalAccountID)
	input.ReconnectAccountID = strings.TrimSpace(input.ReconnectAccountID)
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

func requireReconnectTargetMatches(target db.SocialAccount, input CredentialInput) error {
	if target.ID != input.ReconnectAccountID || target.ProfileID != input.ProfileID ||
		target.Platform != input.Platform || target.ConnectionID.Valid ||
		target.Status != "reconnect_required" || target.BindingStatus != "active" ||
		target.ConnectionType != input.Ownership.ConnectionType {
		return ErrReconnectTargetConflict
	}
	if input.Ownership.ConnectionType == "managed" {
		if target.ExternalUserID.Valid && strings.TrimSpace(target.ExternalUserID.String) != "" &&
			strings.TrimSpace(target.ExternalUserID.String) != input.Ownership.ExternalUserID {
			return ErrReconnectTargetConflict
		}
	} else if target.ExternalUserID.Valid && strings.TrimSpace(target.ExternalUserID.String) != "" {
		return ErrReconnectTargetConflict
	}

	storedProviderIdentity := strings.TrimSpace(target.ExternalAccountID)
	if input.Platform == "instagram" {
		var metadata map[string]json.RawMessage
		if len(target.Metadata) != 0 && json.Unmarshal(target.Metadata, &metadata) != nil {
			return ErrReconnectTargetConflict
		}
		storedProviderIdentity = ""
		if raw, ok := metadata["instagram_webhook_user_id"]; ok {
			if err := json.Unmarshal(raw, &storedProviderIdentity); err != nil {
				return ErrReconnectTargetConflict
			}
			storedProviderIdentity = strings.TrimSpace(storedProviderIdentity)
		}
	}
	if storedProviderIdentity != "" && storedProviderIdentity != input.ProviderIdentity {
		return ErrReconnectTargetConflict
	}
	return nil
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

func recoveryParams(connectionID string, input CredentialInput) db.RecoverReconnectRequiredSocialAccountBindingParams {
	return db.RecoverReconnectRequiredSocialAccountBindingParams{
		ConnectionID: textValue(connectionID), LegacyAccessToken: input.AccessToken,
		LegacyRefreshToken: nullableText(input.RefreshToken), TokenExpiresAt: timestamp(input.TokenExpiresAt),
		ExternalAccountID: input.ExternalAccountID, AccountName: nullableText(input.AccountName),
		AccountAvatarUrl: nullableText(input.AvatarURL), Metadata: input.Metadata, Scope: input.Scopes,
		ConnectionType: input.Ownership.ConnectionType, ConnectSessionID: nullableText(input.ConnectSessionID),
		ExternalUserID:    nullableText(input.Ownership.ExternalUserID),
		ExternalUserEmail: nullableText(input.Ownership.ExternalUserEmail), XAppMode: nullableText(input.XAppMode),
		ReconnectAccountID: input.ReconnectAccountID, WorkspaceID: input.WorkspaceID,
		ProfileID: input.ProfileID, Platform: input.Platform,
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
