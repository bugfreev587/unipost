package xinbox

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/xiaoboyu/unipost-api/internal/db"
)

type PostgresIngestionStore struct {
	queries                *db.Queries
	pool                   *pgxpool.Pool
	managedWebhookRouteKey string
}

var _ providerUserIngestionStore = (*PostgresIngestionStore)(nil)

func NewPostgresIngestionStore(
	queries *db.Queries,
	pool *pgxpool.Pool,
	managedWebhookRouteKey string,
) *PostgresIngestionStore {
	return &PostgresIngestionStore{
		queries:                queries,
		pool:                   pool,
		managedWebhookRouteKey: managedWebhookRouteKey,
	}
}

func (s *PostgresIngestionStore) AccountForApp(
	ctx context.Context,
	routeKey string,
	accountID string,
) (InboxAccount, error) {
	if s == nil || s.queries == nil {
		return InboxAccount{}, errors.New("X inbox database queries are not configured")
	}
	row, err := s.queries.FindXInboxAccountForApp(ctx, db.FindXInboxAccountForAppParams{
		AccountID:              accountID,
		WebhookRouteKey:        routeKey,
		ManagedWebhookRouteKey: s.managedWebhookRouteKey,
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

func (s *PostgresIngestionStore) AccountsForProviderUser(
	ctx context.Context,
	routeKey string,
	providerUserID string,
) ([]InboxAccount, error) {
	if s == nil || s.queries == nil {
		return nil, errors.New("X inbox database queries are not configured")
	}
	rows, err := s.queries.FindXInboxAccountsForProviderUserApp(
		ctx,
		db.FindXInboxAccountsForProviderUserAppParams{
			ProviderUserID:         providerUserID,
			WebhookRouteKey:        routeKey,
			ManagedWebhookRouteKey: s.managedWebhookRouteKey,
		},
	)
	if err != nil {
		return nil, err
	}
	providerRows := make([]providerUserAccountRow, 0, len(rows))
	for _, row := range rows {
		providerRows = append(providerRows, providerUserAccountRow{
			id: row.ID, workspaceID: row.WorkspaceID, externalUserID: row.ExternalUserID,
			externalAccountID: row.ExternalAccountID, accountName: row.AccountName,
			appMode: row.XAppMode, scopes: row.Scope, connectionType: row.ConnectionType,
			planID: row.PlanID, planAllowsInbox: row.PlanAllowsInbox,
		})
	}
	return inboxAccountsFromProviderRows(providerRows)
}

type providerUserAccountRow struct {
	id                string
	workspaceID       string
	externalUserID    pgtype.Text
	externalAccountID string
	accountName       string
	appMode           string
	scopes            []string
	connectionType    string
	planID            string
	planAllowsInbox   bool
}

func inboxAccountsFromProviderRows(rows []providerUserAccountRow) ([]InboxAccount, error) {
	accounts := make([]InboxAccount, 0, len(rows))
	for _, row := range rows {
		account, err := inboxAccountFromRow(
			row.id,
			row.workspaceID,
			row.externalUserID,
			row.externalAccountID,
			row.accountName,
			row.appMode,
			row.scopes,
			row.connectionType,
			row.planID,
			row.planAllowsInbox,
		)
		if err != nil {
			return nil, err
		}
		accounts = append(accounts, account)
	}
	return accounts, nil
}

func (s *PostgresIngestionStore) InsertInboxItem(
	ctx context.Context,
	item InboxItem,
) (InboxItem, bool, error) {
	if s == nil || s.pool == nil {
		return InboxItem{}, false, errors.New("X inbox database pool is not configured")
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return InboxItem{}, false, err
	}
	defer tx.Rollback(ctx)
	insertedItem, inserted, err := insertInboxItem(ctx, db.New(tx), item)
	if err != nil {
		return InboxItem{}, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return InboxItem{}, false, err
	}
	return insertedItem, inserted, nil
}

func (s *PostgresIngestionStore) InsertInboxItemTx(
	ctx context.Context,
	tx pgx.Tx,
	item InboxItem,
) (InboxItem, bool, error) {
	if tx == nil {
		return InboxItem{}, false, errors.New("X inbox transaction is not configured")
	}
	return insertInboxItem(ctx, db.New(tx), item)
}

func insertInboxItem(
	ctx context.Context,
	queries *db.Queries,
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
	row, err := queries.UpsertInboxItem(ctx, db.UpsertInboxItemParams{
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
		existing, loadErr := queries.GetInboxItemByExternalID(ctx, db.GetInboxItemByExternalIDParams{
			SocialAccountID: item.SocialAccountID,
			ExternalID:      item.ExternalID,
		})
		if loadErr != nil {
			return InboxItem{}, false, loadErr
		}
		existingItem := inboxItemFromDB(existing)
		if healErr := healXInboxOutboundFromWebhook(ctx, queries, existingItem); healErr != nil {
			return InboxItem{}, false, healErr
		}
		return existingItem, false, nil
	}
	if err != nil {
		return InboxItem{}, false, err
	}
	if err := healXInboxOutboundFromWebhook(ctx, queries, item); err != nil {
		return InboxItem{}, false, err
	}
	return inboxItemFromDB(row), true, nil
}

type xInboxOutboundWebhookCandidate struct {
	ID                     string
	InboxItemID            string
	PayloadHash            string
	SendStartedAt          time.Time
	ReconciliationDeadline time.Time
}

func xInboxOutboundWebhookPayloadHash(inboxItemID, source, body string) string {
	sum := sha256.Sum256([]byte(inboxItemID + "\x00" + source + "\x00" + body))
	return hex.EncodeToString(sum[:])
}

func xInboxOutboundWebhookBodyHash(body string) string {
	sum := sha256.Sum256([]byte(body))
	return hex.EncodeToString(sum[:])
}

func matchingXInboxOutboundWebhookCandidate(
	candidates []xInboxOutboundWebhookCandidate,
	source string,
	body string,
	eventAt time.Time,
) (string, bool) {
	if eventAt.IsZero() {
		return "", false
	}
	match := ""
	for _, candidate := range candidates {
		if candidate.SendStartedAt.IsZero() || candidate.ReconciliationDeadline.IsZero() ||
			eventAt.Before(candidate.SendStartedAt.Add(-5*time.Minute)) ||
			eventAt.After(candidate.ReconciliationDeadline) {
			continue
		}
		if candidate.PayloadHash != xInboxOutboundWebhookPayloadHash(candidate.InboxItemID, source, body) {
			continue
		}
		if match != "" {
			return "", false
		}
		match = candidate.ID
	}
	return match, match != ""
}

func healXInboxOutboundFromWebhook(
	ctx context.Context,
	queries *db.Queries,
	item InboxItem,
) error {
	if queries == nil || !item.IsOwn ||
		(item.Source != "x_reply" && item.Source != "x_dm") ||
		item.ExternalID == "" || item.Body == "" || item.ReceivedAt.IsZero() {
		return nil
	}
	rows, err := queries.ListXInboxOutboundWebhookCandidates(
		ctx,
		db.ListXInboxOutboundWebhookCandidatesParams{
			SocialAccountID:  item.SocialAccountID,
			Source:           item.Source,
			ParentExternalID: item.ParentExternalID,
			ThreadKey:        nullableText(item.ThreadKey),
			BodyHash:         nullableText(xInboxOutboundWebhookBodyHash(item.Body)),
			EventAt:          pgtype.Timestamptz{Time: item.ReceivedAt, Valid: true},
		},
	)
	if err != nil {
		return err
	}
	candidates := make([]xInboxOutboundWebhookCandidate, 0, len(rows))
	for _, row := range rows {
		candidates = append(candidates, xInboxOutboundWebhookCandidate{
			ID:                     row.OutboundRequestID,
			InboxItemID:            row.InboxItemID,
			PayloadHash:            row.PayloadHash,
			SendStartedAt:          row.SendStartedAt.Time,
			ReconciliationDeadline: row.ReconciliationDeadline.Time,
		})
	}
	requestID, matched := matchingXInboxOutboundWebhookCandidate(
		candidates, item.Source, item.Body, item.ReceivedAt,
	)
	if !matched {
		return nil
	}
	remoteURL := ""
	if item.Source == "x_reply" {
		remoteURL = xPostPermalink(item.ExternalID)
	}
	updated, err := queries.RecordXInboxOutboundRemoteSuccessFromWebhook(
		ctx,
		db.RecordXInboxOutboundRemoteSuccessFromWebhookParams{
			RemoteExternalID:     nullableText(item.ExternalID),
			RemoteConversationID: item.ThreadKey,
			RemoteUrl:            remoteURL,
			ID:                   requestID,
		},
	)
	if err != nil {
		return err
	}
	if updated == 1 {
		return nil
	}
	outbound, err := queries.GetXInboxOutboundRequestByID(ctx, requestID)
	if err != nil {
		return err
	}
	if (outbound.Status == "remote_succeeded" ||
		outbound.Status == "completed" ||
		outbound.Status == "succeeded") &&
		outbound.RemoteExternalID.Valid &&
		outbound.RemoteExternalID.String == item.ExternalID {
		return nil
	}
	return errors.New("X webhook could not persist the matched outbound remote outcome")
}

func (s *PostgresIngestionStore) EncryptedConsumerSecrets(
	ctx context.Context,
	routeKey string,
) ([]string, error) {
	rows, err := s.queries.ListTwitterConsumerSecretsByWebhookRouteKey(ctx, nullableText(routeKey))
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

func (s *PostgresIngestionStore) BackfillWebhookRouteKeys(
	ctx context.Context,
) error {
	rows, err := s.queries.ListTwitterCredentialsMissingWebhookRouteKey(ctx)
	if err != nil {
		return err
	}
	for _, row := range rows {
		routeKey, err := RandomWebhookRouteKey()
		if err != nil {
			return err
		}
		if err := s.queries.SetTwitterWebhookRouteKeyIfMissing(
			ctx,
			db.SetTwitterWebhookRouteKeyIfMissingParams{
				WorkspaceID:     row.WorkspaceID,
				ClientID:        row.ClientID,
				WebhookRouteKey: nullableText(routeKey),
			},
		); err != nil {
			return err
		}
	}
	return nil
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
	if err != nil {
		return InboxAccount{}, fmt.Errorf("normalize persisted X app mode for account %q: %w", id, err)
	}
	if mode != AppModeUniPostManaged && mode != AppModeWorkspace {
		return InboxAccount{}, fmt.Errorf("X inbox account %q has unsupported persisted app mode %q", id, mode)
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
