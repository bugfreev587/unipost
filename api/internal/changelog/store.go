package changelog

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresStore struct {
	pool *pgxpool.Pool
}

func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{pool: pool}
}

func (s *PostgresStore) CreateCandidate(ctx context.Context, input CreateCandidateInput) (CandidateRecord, bool, error) {
	if s == nil || s.pool == nil {
		return CandidateRecord{}, false, errors.New("changelog postgres store is not configured")
	}
	if err := ValidateCandidate(input.Payload); err != nil {
		return CandidateRecord{}, false, err
	}
	payload, err := json.Marshal(input.Payload)
	if err != nil {
		return CandidateRecord{}, false, err
	}
	candidateID := input.Payload.Candidate.ID
	row := s.pool.QueryRow(ctx, `
INSERT INTO changelog_candidates (
  id, source_hash, status, payload_json, window_start, window_end, discord_message_id
)
VALUES ($1, $2, 'pending', $3, $4, $5, NULLIF($6, ''))
ON CONFLICT (source_hash) DO NOTHING
RETURNING id, source_hash, status, payload_json, window_start, window_end, COALESCE(discord_message_id, ''), COALESCE(action_request_id, ''), COALESCE(workflow_run_url, ''), COALESCE(acted_by_admin_id, ''), COALESCE(error_message, ''), created_at, updated_at
`, candidateID, input.SourceHash, payload, input.WindowStart, input.WindowEnd, input.DiscordMessageID)
	record, err := scanCandidateRecord(row)
	if err == nil {
		return record, true, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return CandidateRecord{}, false, err
	}
	existing, err := scanCandidateRecord(s.pool.QueryRow(ctx, `
SELECT id, source_hash, status, payload_json, window_start, window_end, COALESCE(discord_message_id, ''), COALESCE(action_request_id, ''), COALESCE(workflow_run_url, ''), COALESCE(acted_by_admin_id, ''), COALESCE(error_message, ''), created_at, updated_at
FROM changelog_candidates
WHERE source_hash = $1
`, input.SourceHash))
	return existing, false, err
}

func (s *PostgresStore) GetCandidate(ctx context.Context, id string) (CandidateRecord, error) {
	if s == nil || s.pool == nil {
		return CandidateRecord{}, errors.New("changelog postgres store is not configured")
	}
	record, err := scanCandidateRecord(s.pool.QueryRow(ctx, `
SELECT id, source_hash, status, payload_json, window_start, window_end, COALESCE(discord_message_id, ''), COALESCE(action_request_id, ''), COALESCE(workflow_run_url, ''), COALESCE(acted_by_admin_id, ''), COALESCE(error_message, ''), created_at, updated_at
FROM changelog_candidates
WHERE id = $1
`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return CandidateRecord{}, ErrCandidateNotFound
	}
	return record, err
}

func (s *PostgresStore) ListCandidatesByStatus(ctx context.Context, status CandidateStatus, limit int) ([]CandidateRecord, error) {
	if s == nil || s.pool == nil {
		return nil, errors.New("changelog postgres store is not configured")
	}
	if limit <= 0 {
		limit = 10
	}
	rows, err := s.pool.Query(ctx, `
SELECT id, source_hash, status, payload_json, window_start, window_end, COALESCE(discord_message_id, ''), COALESCE(action_request_id, ''), COALESCE(workflow_run_url, ''), COALESCE(acted_by_admin_id, ''), COALESCE(error_message, ''), created_at, updated_at
FROM changelog_candidates
WHERE status = $1
ORDER BY updated_at DESC, created_at DESC
LIMIT $2
`, string(status), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var records []CandidateRecord
	for rows.Next() {
		record, err := scanCandidateRecord(rows)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, rows.Err()
}

func (s *PostgresStore) ClaimCandidate(ctx context.Context, id string, from []CandidateStatus, to CandidateStatus, actor string) (CandidateRecord, error) {
	if s == nil || s.pool == nil {
		return CandidateRecord{}, errors.New("changelog postgres store is not configured")
	}
	if len(from) == 0 {
		return CandidateRecord{}, ErrCandidateAlreadyHandled
	}
	statuses := make([]string, 0, len(from))
	for _, status := range from {
		statuses = append(statuses, string(status))
	}
	record, err := scanCandidateRecord(s.pool.QueryRow(ctx, `
UPDATE changelog_candidates
SET status = $2,
    acted_by_admin_id = NULLIF($3, ''),
    acted_at = NOW(),
    updated_at = NOW()
WHERE id = $1
  AND status = ANY($4)
RETURNING id, source_hash, status, payload_json, window_start, window_end, COALESCE(discord_message_id, ''), COALESCE(action_request_id, ''), COALESCE(workflow_run_url, ''), COALESCE(acted_by_admin_id, ''), COALESCE(error_message, ''), created_at, updated_at
`, id, string(to), actor, statuses))
	if err == nil {
		return record, nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		if _, getErr := s.GetCandidate(ctx, id); errors.Is(getErr, ErrCandidateNotFound) {
			return CandidateRecord{}, ErrCandidateNotFound
		}
		return CandidateRecord{}, ErrCandidateAlreadyHandled
	}
	return CandidateRecord{}, err
}

func (s *PostgresStore) MarkCandidateFailed(ctx context.Context, id string, message string) error {
	_, err := s.pool.Exec(ctx, `
UPDATE changelog_candidates
SET status = 'failed', error_message = $2, updated_at = NOW()
WHERE id = $1
`, id, message)
	return err
}

func (s *PostgresStore) SetDispatchMetadata(ctx context.Context, id string, requestID string, workflowURL string) error {
	_, err := s.pool.Exec(ctx, `
UPDATE changelog_candidates
SET action_request_id = NULLIF($2, ''),
    workflow_run_url = NULLIF($3, ''),
    updated_at = NOW()
WHERE id = $1
`, id, requestID, workflowURL)
	return err
}

func scanCandidateRecord(row pgx.Row) (CandidateRecord, error) {
	var record CandidateRecord
	var status string
	var payload json.RawMessage
	err := row.Scan(
		&record.ID,
		&record.SourceHash,
		&status,
		&payload,
		&record.WindowStart,
		&record.WindowEnd,
		&record.DiscordMessageID,
		&record.ActionRequestID,
		&record.WorkflowRunURL,
		&record.ActedByAdminID,
		&record.ErrorMessage,
		&record.CreatedAt,
		&record.UpdatedAt,
	)
	if err != nil {
		return CandidateRecord{}, err
	}
	record.Status = CandidateStatus(status)
	record.PayloadJSON = payload
	if len(payload) > 0 {
		if err := json.Unmarshal(payload, &record.Payload); err != nil {
			return CandidateRecord{}, err
		}
	}
	return record, nil
}

func NormalizeWindow(start, end time.Time) (time.Time, time.Time) {
	return start.UTC(), end.UTC()
}
