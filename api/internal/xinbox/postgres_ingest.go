package xinbox

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/db"
)

type PostgresIngestionStore struct {
	queries            *db.Queries
	managedAppClientID string
}

func NewPostgresIngestionStore(queries *db.Queries, managedAppClientID string) *PostgresIngestionStore {
	return &PostgresIngestionStore{
		queries:            queries,
		managedAppClientID: managedAppClientID,
	}
}

func (s *PostgresIngestionStore) AccountForApp(
	ctx context.Context,
	appClientID string,
	accountID string,
) (InboxAccount, error) {
	row, err := s.queries.FindXInboxAccountForApp(ctx, db.FindXInboxAccountForAppParams{
		AccountID:          accountID,
		AppClientID:        appClientID,
		ManagedAppClientID: s.managedAppClientID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return InboxAccount{}, ErrInboxAccountNotFound
	}
	if err != nil {
		return InboxAccount{}, err
	}
	return inboxAccountFromRow(
		row.ID,
		row.WorkspaceID,
		row.ExternalUserID,
		row.ExternalAccountID,
		row.AccountName,
		row.XAppMode,
		row.Scope,
		row.ConnectionType,
		row.PlanID,
		row.PlanAllowsInbox,
	)
}

func (s *PostgresIngestionStore) AccountsForExternalUser(
	ctx context.Context,
	appClientID string,
	externalUserID string,
) ([]InboxAccount, error) {
	rows, err := s.queries.FindXInboxAccountsForExternalUserApp(
		ctx,
		db.FindXInboxAccountsForExternalUserAppParams{
			ExternalUserID:     nullableText(externalUserID),
			AppClientID:        appClientID,
			ManagedAppClientID: s.managedAppClientID,
		},
	)
	if err != nil {
		return nil, err
	}
	accounts := make([]InboxAccount, 0, len(rows))
	for _, row := range rows {
		account, err := inboxAccountFromRow(
			row.ID,
			row.WorkspaceID,
			row.ExternalUserID,
			row.ExternalAccountID,
			row.AccountName,
			row.XAppMode,
			row.Scope,
			row.ConnectionType,
			row.PlanID,
			row.PlanAllowsInbox,
		)
		if err != nil {
			continue
		}
		accounts = append(accounts, account)
	}
	return accounts, nil
}

func (s *PostgresIngestionStore) InsertInboxItem(
	ctx context.Context,
	item InboxItem,
) (InboxItem, bool, error) {
	metadata := item.Metadata
	if metadata == nil {
		metadata = map[string]any{}
	}
	rawMetadata, err := json.Marshal(metadata)
	if err != nil {
		return InboxItem{}, false, err
	}
	row, err := s.queries.UpsertInboxItem(ctx, db.UpsertInboxItemParams{
		SocialAccountID:  item.SocialAccountID,
		WorkspaceID:      item.WorkspaceID,
		Source:           item.Source,
		ExternalID:       item.ExternalID,
		ParentExternalID: nullableText(item.ParentExternalID),
		AuthorName:       nullableText(item.AuthorName),
		AuthorID:         nullableText(item.AuthorID),
		AuthorAvatarUrl:  nullableText(item.AuthorAvatarURL),
		Body:             nullableText(item.Body),
		IsOwn:            item.IsOwn,
		ReceivedAt:       pgtype.Timestamptz{Time: item.ReceivedAt, Valid: !item.ReceivedAt.IsZero()},
		Metadata:         rawMetadata,
		ThreadKey:        item.ThreadKey,
		ThreadStatus:     firstNonEmptyString(item.ThreadStatus, "open"),
		AssignedTo:       nullableText(item.AssignedTo),
		LinkedPostID:     nullableText(item.LinkedPostID),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return InboxItem{}, false, nil
	}
	if err != nil {
		return InboxItem{}, false, err
	}
	return inboxItemFromDB(row), true, nil
}

func (s *PostgresIngestionStore) EncryptedConsumerSecrets(
	ctx context.Context,
	appClientID string,
) ([]string, error) {
	rows, err := s.queries.ListTwitterConsumerSecretsByClientID(ctx, appClientID)
	if err != nil {
		return nil, err
	}
	secrets := make([]string, 0, len(rows))
	for _, row := range rows {
		if row.Valid && row.String != "" {
			secrets = append(secrets, row.String)
		}
	}
	return secrets, nil
}

func inboxAccountFromRow(
	id string,
	workspaceID string,
	externalUserID pgtype.Text,
	externalAccountID string,
	accountName string,
	appMode string,
	scopes []string,
	connectionType string,
	planID string,
	planAllowsInbox bool,
) (InboxAccount, error) {
	mode, err := NormalizePersistedAppMode(appMode)
	if err != nil || (mode != AppModeUniPostManaged && mode != AppModeWorkspace) {
		return InboxAccount{}, ErrInboxAccountNotFound
	}
	return InboxAccount{
		ID:                id,
		WorkspaceID:       workspaceID,
		ExternalUserID:    externalUserID.String,
		ExternalAccountID: externalAccountID,
		AccountName:       accountName,
		AppMode:           mode,
		Scopes:            scopes,
		ConnectionType:    connectionType,
		PlanID:            planID,
		PlanAllowsInbox:   planAllowsInbox,
	}, nil
}

func inboxItemFromDB(row db.InboxItem) InboxItem {
	item := InboxItem{
		ID:               row.ID,
		SocialAccountID:  row.SocialAccountID,
		WorkspaceID:      row.WorkspaceID,
		Source:           row.Source,
		ExternalID:       row.ExternalID,
		Body:             row.Body.String,
		IsRead:           row.IsRead,
		IsOwn:            row.IsOwn,
		ThreadKey:        row.ThreadKey,
		ThreadStatus:     row.ThreadStatus,
		ReceivedAt:       row.ReceivedAt.Time,
		CreatedAt:        row.CreatedAt.Time,
		ParentExternalID: row.ParentExternalID.String,
		AuthorName:       row.AuthorName.String,
		AuthorID:         row.AuthorID.String,
		AuthorAvatarURL:  row.AuthorAvatarUrl.String,
		AssignedTo:       row.AssignedTo.String,
		LinkedPostID:     row.LinkedPostID.String,
	}
	if len(row.Metadata) > 0 {
		_ = json.Unmarshal(row.Metadata, &item.Metadata)
	}
	return item
}

func nullableText(value string) pgtype.Text {
	return pgtype.Text{String: value, Valid: value != ""}
}
