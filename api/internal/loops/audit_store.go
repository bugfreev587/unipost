package loops

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/db"
)

type emailAuditQueries interface {
	CreateEmailSendAttemptAudit(ctx context.Context, arg db.CreateEmailSendAttemptAuditParams) (db.EmailSendAttempt, error)
	MarkEmailSendAttemptAuditSent(ctx context.Context, id string) error
	MarkEmailSendAttemptAuditFailed(ctx context.Context, arg db.MarkEmailSendAttemptAuditFailedParams) error
}

type PostgresEmailAuditStore struct {
	queries emailAuditQueries
}

func NewPostgresEmailAuditStore(queries emailAuditQueries) *PostgresEmailAuditStore {
	return &PostgresEmailAuditStore{queries: queries}
}

func (s *PostgresEmailAuditStore) CreateEmailSendAttempt(ctx context.Context, attempt EmailSendAttempt) (EmailSendAttemptRecord, error) {
	if s == nil || s.queries == nil {
		return EmailSendAttemptRecord{}, fmt.Errorf("email audit store is not configured")
	}
	vars := attempt.DataVariables
	if vars == nil {
		vars = map[string]any{}
	}
	raw, err := json.Marshal(vars)
	if err != nil {
		return EmailSendAttemptRecord{}, fmt.Errorf("marshal email audit variables: %w", err)
	}
	row, err := s.queries.CreateEmailSendAttemptAudit(ctx, db.CreateEmailSendAttemptAuditParams{
		EventKey:              attempt.EventKey,
		RecipientUserID:       attempt.RecipientUserID,
		RecipientEmail:        attempt.RecipientEmail,
		WorkspaceID:           attempt.WorkspaceID,
		Provider:              firstNonEmpty(attempt.Provider, "loops"),
		ProviderTemplateID:    attempt.ProviderTemplateID,
		IdempotencyKey:        attempt.IdempotencyKey,
		DeliveryClass:         attempt.DeliveryClass,
		SubjectSnapshot:       attempt.SubjectSnapshot,
		DataVariablesSnapshot: raw,
		TriggerSource:         attempt.TriggerSource,
		TriggerReferenceID:    attempt.TriggerReferenceID,
	})
	if err != nil {
		return EmailSendAttemptRecord{}, err
	}
	return EmailSendAttemptRecord{ID: row.ID}, nil
}

func (s *PostgresEmailAuditStore) MarkEmailSendAttemptSent(ctx context.Context, id string) error {
	if s == nil || s.queries == nil {
		return fmt.Errorf("email audit store is not configured")
	}
	return s.queries.MarkEmailSendAttemptAuditSent(ctx, id)
}

func (s *PostgresEmailAuditStore) MarkEmailSendAttemptFailed(ctx context.Context, id, reason string) error {
	if s == nil || s.queries == nil {
		return fmt.Errorf("email audit store is not configured")
	}
	return s.queries.MarkEmailSendAttemptAuditFailed(ctx, db.MarkEmailSendAttemptAuditFailedParams{
		ID:        id,
		LastError: pgtype.Text{String: reason, Valid: true},
	})
}
