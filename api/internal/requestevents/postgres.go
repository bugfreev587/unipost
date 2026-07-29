package requestevents

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

type postgresDB interface {
	SendBatch(context.Context, *pgx.Batch) pgx.BatchResults
	Begin(context.Context) (pgx.Tx, error)
}

type PostgresStore struct {
	database postgresDB
}

func NewPostgresStore(database postgresDB) *PostgresStore {
	return &PostgresStore{database: database}
}

const insertEventSQL = `
INSERT INTO api_request_events (
  occurred_at,
  id,
  workspace_id,
  api_key_id,
  request_id,
  trace_id,
  method,
  route_pattern,
  status_code,
  duration_ms,
  outcome,
  error_code,
  profile_id,
  social_account_id,
  post_id,
  has_error_detail
) VALUES (
  $1, $2, $3, $4, $5, $6, $7, $8,
  $9, $10, $11, $12, $13, $14, $15, $16
)`

const insertDetailSQL = `
INSERT INTO api_request_error_details (
  occurred_at,
  event_id,
  workspace_id,
  request_headers,
  response_headers,
  query_keys,
  request_payload,
  response_payload,
  request_original_bytes,
  request_stored_bytes,
  response_original_bytes,
  response_stored_bytes,
  request_content_type,
  response_content_type,
  request_sha256,
  response_sha256,
  request_truncated,
  response_truncated,
  request_omission_reason,
  response_omission_reason,
  redaction_policy_version
) VALUES (
  $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11,
  $12, $13, $14, $15, $16, $17, $18, $19, $20, $21
)`

func (s *PostgresStore) InsertSuccessBatch(ctx context.Context, events []Event) error {
	if s == nil || s.database == nil {
		return errors.New("request event store is unavailable")
	}
	if len(events) == 0 {
		return nil
	}
	batch := &pgx.Batch{}
	for _, event := range events {
		if err := validateEvent(event); err != nil {
			return err
		}
		if event.StatusCode >= 400 || event.HasErrorDetail {
			return fmt.Errorf("success batch contains failure event %q", event.ID)
		}
		batch.Queue(insertEventSQL, eventArgs(event)...)
	}
	return s.database.SendBatch(ctx, batch).Close()
}

func (s *PostgresStore) InsertFailure(ctx context.Context, event Event, detail *Detail) error {
	if s == nil || s.database == nil {
		return errors.New("request event store is unavailable")
	}
	if err := validateEvent(event); err != nil {
		return err
	}
	if event.StatusCode < 400 {
		return fmt.Errorf("failure insert received successful event %q", event.ID)
	}
	if event.HasErrorDetail != (detail != nil) {
		return fmt.Errorf("event %q detail presence does not match has_error_detail", event.ID)
	}
	if detail != nil && (detail.EventID != event.ID || !detail.OccurredAt.Equal(event.OccurredAt) || detail.WorkspaceID != event.WorkspaceID) {
		return fmt.Errorf("event %q detail identity mismatch", event.ID)
	}

	transaction, err := s.database.Begin(ctx)
	if err != nil {
		return err
	}
	defer transaction.Rollback(ctx)
	if _, err := transaction.Exec(ctx, insertEventSQL, eventArgs(event)...); err != nil {
		return err
	}
	if detail != nil {
		args, err := detailArgs(*detail)
		if err != nil {
			return err
		}
		if _, err := transaction.Exec(ctx, insertDetailSQL, args...); err != nil {
			return err
		}
	}
	return transaction.Commit(ctx)
}

func validateEvent(event Event) error {
	if strings.TrimSpace(event.ID) == "" || strings.TrimSpace(event.WorkspaceID) == "" || strings.TrimSpace(event.APIKeyID) == "" || event.OccurredAt.IsZero() {
		return errors.New("request event identity is incomplete")
	}
	if event.StatusCode < 100 || event.StatusCode > 599 || event.DurationMS < 0 {
		return fmt.Errorf("request event %q has invalid status or duration", event.ID)
	}
	return nil
}

func eventArgs(event Event) []any {
	return []any{
		event.OccurredAt,
		event.ID,
		event.WorkspaceID,
		event.APIKeyID,
		nullableString(event.RequestID),
		nullableString(event.TraceID),
		event.Method,
		event.RoutePattern,
		event.StatusCode,
		event.DurationMS,
		string(event.Outcome),
		nullableString(event.ErrorCode),
		nullableString(event.ProfileID),
		nullableString(event.SocialAccountID),
		nullableString(event.PostID),
		event.HasErrorDetail,
	}
}

func detailArgs(detail Detail) ([]any, error) {
	requestHeaders, err := json.Marshal(nonNilMap(detail.RequestHeaders))
	if err != nil {
		return nil, err
	}
	responseHeaders, err := json.Marshal(nonNilMap(detail.ResponseHeaders))
	if err != nil {
		return nil, err
	}
	return []any{
		detail.OccurredAt,
		detail.EventID,
		detail.WorkspaceID,
		requestHeaders,
		responseHeaders,
		nonNilStrings(detail.QueryKeys),
		nullableString(detail.RequestPayload),
		nullableString(detail.ResponsePayload),
		detail.RequestOriginalBytes,
		detail.RequestStoredBytes,
		detail.ResponseOriginalBytes,
		detail.ResponseStoredBytes,
		nullableString(detail.RequestContentType),
		nullableString(detail.ResponseContentType),
		nullableString(detail.RequestSHA256),
		nullableString(detail.ResponseSHA256),
		detail.RequestTruncated,
		detail.ResponseTruncated,
		nullableString(detail.RequestOmissionReason),
		nullableString(detail.ResponseOmissionReason),
		detail.RedactionPolicyVersion,
	}, nil
}

func nullableString(value string) any {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return value
}

func nonNilMap(value map[string]string) map[string]string {
	if value == nil {
		return map[string]string{}
	}
	return value
}

func nonNilStrings(value []string) []string {
	if value == nil {
		return []string{}
	}
	return value
}
