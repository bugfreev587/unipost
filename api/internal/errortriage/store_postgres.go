package errortriage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresStore struct {
	pool *pgxpool.Pool
}

const maxFailuresPerRun = 1000

func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{pool: pool}
}

type RunSummary struct {
	RunRecord
	HealthStatus RunHealthStatus `json:"health_status"`
	ItemsTotal   int             `json:"items_total"`
	EmailDrafts  int             `json:"email_drafts"`
	BugPlans     int             `json:"bug_plans"`
	NeedsReview  int             `json:"needs_review"`
	Summary      string          `json:"summary,omitempty"`
	CompletedAt  *time.Time      `json:"completed_at,omitempty"`
	StartedAt    time.Time       `json:"started_at"`
	CreatedAt    time.Time       `json:"created_at"`
	ErrorMessage string          `json:"error_message,omitempty"`
}

type ItemRecord struct {
	ID                     string            `json:"id"`
	RunID                  string            `json:"run_id"`
	DedupeKey              string            `json:"dedupe_key"`
	Classification         Classification    `json:"classification"`
	ActionKind             ActionKind        `json:"action_kind"`
	WorkflowStatus         WorkflowStatus    `json:"workflow_status"`
	Confidence             float64           `json:"confidence"`
	Platform               string            `json:"platform,omitempty"`
	Source                 string            `json:"source,omitempty"`
	ErrorCode              string            `json:"error_code,omitempty"`
	PlatformErrorCode      string            `json:"platform_error_code,omitempty"`
	FailureStage           string            `json:"failure_stage,omitempty"`
	AffectedUserCount      int               `json:"affected_user_count"`
	AffectedWorkspaceCount int               `json:"affected_workspace_count"`
	AffectedPostCount      int               `json:"affected_post_count"`
	LatestFailureAt        *time.Time        `json:"latest_failure_at,omitempty"`
	EvidenceJSON           json.RawMessage   `json:"evidence_json"`
	AISummary              string            `json:"ai_summary,omitempty"`
	AdminNotes             string            `json:"admin_notes,omitempty"`
	BugPlanJSON            json.RawMessage   `json:"bug_plan_json,omitempty"`
	EmailDraftJSON         json.RawMessage   `json:"email_draft_json,omitempty"`
	CTAURL                 string            `json:"cta_url,omitempty"`
	DuplicateOfItemID      string            `json:"duplicate_of_item_id,omitempty"`
	CreatedAt              time.Time         `json:"created_at"`
	UpdatedAt              time.Time         `json:"updated_at"`
	Recipients             []RecipientRecord `json:"recipients,omitempty"`
}

type RecipientRecord struct {
	ID                  string          `json:"id"`
	ItemID              string          `json:"item_id"`
	RecipientScopeKey   string          `json:"recipient_scope_key"`
	WorkspaceID         string          `json:"workspace_id"`
	RecipientUserID     string          `json:"recipient_user_id"`
	EmailSnapshot       string          `json:"email_snapshot"`
	CurrentEmail        string          `json:"current_email,omitempty"`
	Status              RecipientStatus `json:"status"`
	LatestSendAttemptID string          `json:"latest_send_attempt_id,omitempty"`
	DismissReason       string          `json:"dismiss_reason,omitempty"`
	CreatedAt           time.Time       `json:"created_at"`
	UpdatedAt           time.Time       `json:"updated_at"`
}

type RunDetail struct {
	Run   RunSummary   `json:"run"`
	Items []ItemRecord `json:"items"`
}

func (s *PostgresStore) CreateRun(ctx context.Context, params CreateRunParams) (RunRecord, bool, error) {
	if s == nil || s.pool == nil {
		return RunRecord{}, false, errors.New("postgres store is not configured")
	}
	if params.RunType == RunTypeScheduled {
		row := s.pool.QueryRow(ctx, `
INSERT INTO error_triage_runs (run_type, window_start, window_end, created_by_admin_id)
VALUES ('scheduled', $1, $2, $3)
ON CONFLICT (window_start) WHERE run_type = 'scheduled' DO NOTHING
RETURNING id, run_type, status, window_start, window_end, failures_analyzed
`, params.WindowStart, params.WindowEnd, params.AdminUserID)
		run, err := scanRunRecord(row)
		if err == nil {
			return run, true, nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return RunRecord{}, false, err
		}
		existing, err := scanRunRecord(s.pool.QueryRow(ctx, `
SELECT id, run_type, status, window_start, window_end, failures_analyzed
FROM error_triage_runs
WHERE run_type = 'scheduled' AND window_start = $1
`, params.WindowStart))
		return existing, false, err
	}

	run, err := scanRunRecord(s.pool.QueryRow(ctx, `
INSERT INTO error_triage_runs (run_type, window_start, window_end, supersedes_run_id, created_by_admin_id)
VALUES ('manual', $1, $2, NULLIF($3, ''), $4)
RETURNING id, run_type, status, window_start, window_end, failures_analyzed
`, params.WindowStart, params.WindowEnd, params.SupersedesRunID, params.AdminUserID))
	return run, err == nil, err
}

func (s *PostgresStore) TryScheduledRunLock(ctx context.Context, windowStart time.Time) (func(context.Context) error, bool, error) {
	if s == nil || s.pool == nil {
		return nil, false, errors.New("postgres store is not configured")
	}
	conn, err := s.pool.Acquire(ctx)
	if err != nil {
		return nil, false, err
	}
	lockKey := "error_triage:scheduled:" + windowStart.UTC().Format(time.RFC3339)
	var acquired bool
	if err := conn.QueryRow(ctx, `SELECT pg_try_advisory_lock(hashtext($1)::bigint)`, lockKey).Scan(&acquired); err != nil {
		conn.Release()
		return nil, false, err
	}
	if !acquired {
		conn.Release()
		return nil, false, nil
	}
	unlock := func(ctx context.Context) error {
		defer conn.Release()
		var released bool
		if err := conn.QueryRow(ctx, `SELECT pg_advisory_unlock(hashtext($1)::bigint)`, lockKey).Scan(&released); err != nil {
			return err
		}
		if !released {
			return errors.New("scheduled run advisory lock was not held")
		}
		return nil
	}
	return unlock, true, nil
}

func (s *PostgresStore) CompleteRun(ctx context.Context, runID string, params CompleteRunParams) (RunRecord, error) {
	return scanRunRecord(s.pool.QueryRow(ctx, `
UPDATE error_triage_runs
SET status = 'completed',
    completed_at = NOW(),
    model = $2,
    prompt_version = $3,
    failures_analyzed = $4,
    affected_users = $5,
    affected_workspaces = $6,
    summary = $7,
    error_message = NULL,
    updated_at = NOW()
WHERE id = $1
RETURNING id, run_type, status, window_start, window_end, failures_analyzed
`, runID, params.Model, params.PromptVersion, params.FailuresAnalyzed, params.AffectedUsers, params.AffectedWorkspaces, params.Summary))
}

func (s *PostgresStore) FailRun(ctx context.Context, runID string, message string) error {
	_, err := s.pool.Exec(ctx, `
UPDATE error_triage_runs
SET status = 'failed', completed_at = NOW(), error_message = $2, updated_at = NOW()
WHERE id = $1
`, runID, message)
	return err
}

func (s *PostgresStore) LoadFailures(ctx context.Context, start, end time.Time) ([]Failure, error) {
	rows, err := s.pool.Query(ctx, `
SELECT
  pf.id,
  pf.post_id,
  COALESCE(pf.social_post_result_id, ''),
  pf.workspace_id,
  w.name,
  w.user_id,
  u.email,
  pf.platform,
  sp.source,
  pf.error_code,
  COALESCE(pf.platform_error_code, ''),
  pf.failure_stage,
  pf.message,
  COALESCE(pf.raw_error, ''),
  COALESCE(spr.debug_curl, ''),
  pf.is_retriable,
  pf.created_at
FROM post_failures pf
JOIN social_posts sp ON sp.id = pf.post_id
JOIN workspaces w ON w.id = pf.workspace_id
JOIN users u ON u.id = w.user_id
LEFT JOIN social_post_results spr ON spr.id = pf.social_post_result_id
WHERE pf.created_at >= $1
  AND pf.created_at < $2
  AND sp.deleted_at IS NULL
ORDER BY pf.created_at DESC
LIMIT 1001
`, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Failure{}
	for rows.Next() {
		var f Failure
		if err := rows.Scan(
			&f.PostFailureID,
			&f.PostID,
			&f.SocialPostResultID,
			&f.WorkspaceID,
			&f.WorkspaceName,
			&f.UserID,
			&f.UserEmail,
			&f.Platform,
			&f.Source,
			&f.ErrorCode,
			&f.PlatformErrorCode,
			&f.FailureStage,
			&f.Message,
			&f.RawError,
			&f.DebugCurl,
			&f.IsRetriable,
			&f.CreatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(out) > maxFailuresPerRun {
		slog.Warn("error triage: failure load truncated",
			"window_start", start,
			"window_end", end,
			"limit", maxFailuresPerRun,
			"loaded", len(out),
		)
		out = out[:maxFailuresPerRun]
	}
	return out, nil
}

func (s *PostgresStore) FindPreviousItem(ctx context.Context, dedupeKey, runID string) (string, error) {
	var id string
	err := s.pool.QueryRow(ctx, `
SELECT id
FROM error_triage_items
WHERE dedupe_key = $1
  AND run_id <> $2
  AND workflow_status NOT IN ('dismissed')
  AND run_id <> COALESCE((SELECT supersedes_run_id FROM error_triage_runs WHERE id = $2), '00000000-0000-0000-0000-000000000000'::uuid)
ORDER BY created_at DESC
LIMIT 1
`, dedupeKey, runID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	return id, err
}

func (s *PostgresStore) InsertItem(ctx context.Context, runID string, draft ItemDraft, duplicateID string) (string, error) {
	evidence := draft.EvidenceJSON()
	bugPlan, _ := json.Marshal(draft.BugPlan)
	emailDraft, _ := json.Marshal(draft.EmailDraft)
	var id string
	err := s.pool.QueryRow(ctx, `
INSERT INTO error_triage_items (
  run_id, dedupe_key, classification, action_kind, workflow_status, confidence,
  platform, source, error_code, platform_error_code, failure_stage,
  affected_user_count, affected_workspace_count, affected_post_count, latest_failure_at,
  evidence_json, ai_summary, bug_plan_json, email_draft_json, cta_url, duplicate_of_item_id
)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,NULLIF($18, '{}')::jsonb,NULLIF($19, '{}')::jsonb,$20,NULLIF($21, ''))
RETURNING id
`,
		runID, draft.DedupeKey, draft.Classification, draft.ActionKind, draft.WorkflowStatus, draft.Confidence,
		nullEmpty(draft.Platform), nullEmpty(draft.Source), nullEmpty(draft.ErrorCode), nullEmpty(draft.PlatformErrorCode), nullEmpty(draft.FailureStage),
		draft.AffectedUserCount, draft.AffectedWorkspaceCount, draft.AffectedPostCount, nullableTime(draft.LatestFailureAt),
		evidence, draft.Summary, string(bugPlan), string(emailDraft), nullEmpty(draft.CTAURL), duplicateID,
	).Scan(&id)
	return id, err
}

func (s *PostgresStore) InsertItemFailure(ctx context.Context, itemID string, failure Failure) error {
	_, err := s.pool.Exec(ctx, `
INSERT INTO error_triage_item_failures (
  item_id, post_id, social_post_result_id, post_failure_id, workspace_id, user_id, user_email, platform, created_at
)
VALUES ($1,$2,NULLIF($3,''),NULLIF($4,''),$5,$6,$7,$8,$9)
`, itemID, failure.PostID, failure.SocialPostResultID, failure.PostFailureID, failure.WorkspaceID, failure.UserID, failure.UserEmail, failure.Platform, failure.CreatedAt)
	return err
}

func (s *PostgresStore) InsertRecipient(ctx context.Context, itemID string, recipient RecipientCandidate) error {
	_, err := s.pool.Exec(ctx, `
INSERT INTO error_triage_item_recipients (
  item_id, recipient_scope_key, workspace_id, recipient_user_id, email_snapshot
)
VALUES ($1,$2,$3,$4,$5)
ON CONFLICT (item_id, recipient_scope_key) DO NOTHING
`, itemID, recipient.ScopeKey, recipient.WorkspaceID, recipient.UserID, recipient.Email)
	return err
}

func (s *PostgresStore) ListRuns(ctx context.Context, limit int) ([]RunSummary, error) {
	if limit <= 0 || limit > 100 {
		limit = 30
	}
	rows, err := s.pool.Query(ctx, runsSelectSQL+`
GROUP BY r.id
ORDER BY r.window_start DESC, r.created_at DESC
LIMIT $1
`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []RunSummary{}
	for rows.Next() {
		run, err := scanRunSummary(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, run)
	}
	return out, rows.Err()
}

func (s *PostgresStore) GetRunDetail(ctx context.Context, runID string) (RunDetail, error) {
	run, err := scanRunSummary(s.pool.QueryRow(ctx, runsSelectSQL+`WHERE r.id = $1 GROUP BY r.id`, runID))
	if err != nil {
		return RunDetail{}, err
	}
	items, err := s.listItems(ctx, runID)
	if err != nil {
		return RunDetail{}, err
	}
	return RunDetail{Run: run, Items: items}, nil
}

func (s *PostgresStore) UpdateItem(ctx context.Context, itemID, workflowStatus, adminNotes string) error {
	_, err := s.pool.Exec(ctx, `
UPDATE error_triage_items
SET workflow_status = COALESCE(NULLIF($2, ''), workflow_status),
    admin_notes = COALESCE($3, admin_notes),
    updated_at = NOW()
WHERE id = $1
`, itemID, workflowStatus, nullableString(adminNotes))
	return err
}

func (s *PostgresStore) DismissRecipient(ctx context.Context, recipientID, adminID, reason string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var itemID string
	if err := tx.QueryRow(ctx, `
UPDATE error_triage_item_recipients
SET status = 'dismissed',
    dismissed_by_admin_id = $2,
    dismissed_at = NOW(),
    dismiss_reason = $3,
    updated_at = NOW()
WHERE id = $1
RETURNING item_id
`, recipientID, adminID, reason).Scan(&itemID); err != nil {
		return err
	}
	if err := refreshItemEmailWorkflowStatus(ctx, tx, itemID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *PostgresStore) LoadEmailSendContext(ctx context.Context, itemID, recipientID string) (EmailSendContext, error) {
	var out EmailSendContext
	var classification, actionKind, workflowStatus, recipientStatus string
	var draftRaw []byte
	err := s.pool.QueryRow(ctx, `
SELECT
  i.id,
  r.id,
  r.recipient_scope_key,
  r.recipient_user_id,
  COALESCE(u.email, ''),
  i.classification,
  i.action_kind,
  i.workflow_status,
  r.status,
  COALESCE(i.email_draft_json, '{}'::jsonb),
  COALESCE(i.cta_url, '')
FROM error_triage_items i
JOIN error_triage_item_recipients r ON r.item_id = i.id
LEFT JOIN users u ON u.id = r.recipient_user_id
WHERE i.id = $1 AND r.id = $2
`, itemID, recipientID).Scan(
		&out.ItemID,
		&out.RecipientID,
		&out.RecipientScopeKey,
		&out.RecipientUserID,
		&out.CurrentEmail,
		&classification,
		&actionKind,
		&workflowStatus,
		&recipientStatus,
		&draftRaw,
		&out.CTAURL,
	)
	if err != nil {
		return EmailSendContext{}, err
	}
	if err := json.Unmarshal(draftRaw, &out.Draft); err != nil {
		return EmailSendContext{}, err
	}
	out.Item = ItemState{
		Classification: Classification(classification),
		ActionKind:     ActionKind(actionKind),
		WorkflowStatus: WorkflowStatus(workflowStatus),
	}
	out.Recipient = RecipientState{Status: RecipientStatus(recipientStatus)}
	out.DraftVersion = 1
	return out, nil
}

func (s *PostgresStore) CreateEmailSendAttempt(ctx context.Context, params CreateEmailSendAttemptParams) (EmailSendAttempt, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return EmailSendAttempt{}, err
	}
	defer tx.Rollback(ctx)
	var status string
	if err := tx.QueryRow(ctx, `
SELECT status
FROM error_triage_item_recipients
WHERE id = $1 AND item_id = $2
FOR UPDATE
`, params.RecipientID, params.ItemID).Scan(&status); err != nil {
		return EmailSendAttempt{}, err
	}
	if status == string(RecipientStatusSent) || status == string(RecipientStatusDismissed) {
		return EmailSendAttempt{}, errors.New("recipient_already_final")
	}
	var out EmailSendAttempt
	err = tx.QueryRow(ctx, `
WITH next_attempt AS (
  SELECT COALESCE(MAX(attempt_number), 0) + 1 AS attempt_number
  FROM error_triage_email_sends
  WHERE recipient_id = $2
)
INSERT INTO error_triage_email_sends (
  item_id, recipient_id, recipient_scope_key, recipient_user_id, recipient_email,
  attempt_number, loops_transactional_id, idempotency_key, subject_snapshot,
  body_snapshot, sent_by_admin_id
)
SELECT $1,$2,$3,$4,$5,attempt_number,$6,$7,$8,$9,$10
FROM next_attempt
RETURNING id, attempt_number
`,
		params.ItemID,
		params.RecipientID,
		params.RecipientScopeKey,
		params.RecipientUserID,
		params.RecipientEmail,
		params.TransactionalID,
		params.IdempotencyKey,
		params.Subject,
		params.Body,
		params.SentByAdminID,
	).Scan(&out.ID, &out.AttemptNumber)
	if err != nil {
		return EmailSendAttempt{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return EmailSendAttempt{}, err
	}
	return out, nil
}

func (s *PostgresStore) MarkEmailSendSucceeded(ctx context.Context, attemptID, recipientID string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var itemID string
	if err := tx.QueryRow(ctx, `
UPDATE error_triage_email_sends
SET provider_status = 'succeeded', sent_at = NOW(), provider_error = NULL
WHERE id = $1 AND recipient_id = $2
RETURNING item_id
`, attemptID, recipientID).Scan(&itemID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
UPDATE error_triage_item_recipients
SET status = 'sent', latest_send_attempt_id = $1, updated_at = NOW()
WHERE id = $2
`, attemptID, recipientID); err != nil {
		return err
	}
	if err := refreshItemEmailWorkflowStatus(ctx, tx, itemID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *PostgresStore) MarkEmailSendFailed(ctx context.Context, attemptID, recipientID, message string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var itemID string
	if err := tx.QueryRow(ctx, `
UPDATE error_triage_email_sends
SET provider_status = 'failed', provider_error = $3
WHERE id = $1 AND recipient_id = $2
RETURNING item_id
`, attemptID, recipientID, message).Scan(&itemID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
UPDATE error_triage_item_recipients
SET status = 'send_failed', latest_send_attempt_id = $1, updated_at = NOW()
WHERE id = $2
`, attemptID, recipientID); err != nil {
		return err
	}
	if err := refreshItemEmailWorkflowStatus(ctx, tx, itemID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func refreshItemEmailWorkflowStatus(ctx context.Context, tx pgx.Tx, itemID string) error {
	_, err := tx.Exec(ctx, `
UPDATE error_triage_items i
SET workflow_status = CASE
    WHEN NOT EXISTS (
      SELECT 1
      FROM error_triage_item_recipients r
      WHERE r.item_id = i.id AND r.status IN ('pending', 'send_failed')
    ) THEN 'completed'
    WHEN EXISTS (
      SELECT 1
      FROM error_triage_item_recipients r
      WHERE r.item_id = i.id AND r.status IN ('sent', 'dismissed')
    ) THEN 'partially_completed'
    ELSE i.workflow_status
  END,
  updated_at = NOW()
WHERE i.id = $1 AND i.action_kind = 'email'
`, itemID)
	return err
}

func scanRunRecord(row pgx.Row) (RunRecord, error) {
	var run RunRecord
	var runType, status string
	if err := row.Scan(&run.ID, &runType, &status, &run.WindowStart, &run.WindowEnd, &run.FailuresAnalyzed); err != nil {
		return RunRecord{}, err
	}
	run.RunType = RunType(runType)
	run.Status = RunStatus(status)
	return run, nil
}

const runsSelectSQL = `
SELECT
  r.id,
  r.run_type,
  r.status,
  r.window_start,
  r.window_end,
  r.failures_analyzed,
  COALESCE(r.summary, ''),
  COALESCE(r.error_message, ''),
  r.started_at,
  r.completed_at,
  r.created_at,
  COALESCE(COUNT(i.id), 0)::INTEGER AS items_total,
  COALESCE(COUNT(i.id) FILTER (WHERE i.action_kind = 'email'), 0)::INTEGER AS email_drafts,
  COALESCE(COUNT(i.id) FILTER (WHERE i.action_kind = 'bug_plan'), 0)::INTEGER AS bug_plans,
  COALESCE(COUNT(i.id) FILTER (WHERE i.workflow_status = 'pending_review' OR i.classification = 'needs_human_review'), 0)::INTEGER AS needs_review,
  CASE
    WHEN r.status = 'failed' THEN 'needs_review'
    WHEN COALESCE(COUNT(i.id) FILTER (WHERE i.workflow_status = 'pending_review' OR i.classification = 'needs_human_review'), 0) > 0 THEN 'needs_review'
    WHEN COALESCE(COUNT(i.id) FILTER (WHERE i.action_kind <> 'none' AND i.workflow_status NOT IN ('completed','dismissed')), 0) > 0 THEN 'actionable_items'
    ELSE 'no_actionable_issues'
  END AS health_status
FROM error_triage_runs r
LEFT JOIN error_triage_items i ON i.run_id = r.id
`

func scanRunSummary(row pgx.Row) (RunSummary, error) {
	var run RunSummary
	var runType, status, health string
	var completedAt *time.Time
	if err := row.Scan(
		&run.ID,
		&runType,
		&status,
		&run.WindowStart,
		&run.WindowEnd,
		&run.FailuresAnalyzed,
		&run.Summary,
		&run.ErrorMessage,
		&run.StartedAt,
		&completedAt,
		&run.CreatedAt,
		&run.ItemsTotal,
		&run.EmailDrafts,
		&run.BugPlans,
		&run.NeedsReview,
		&health,
	); err != nil {
		return RunSummary{}, err
	}
	run.RunType = RunType(runType)
	run.Status = RunStatus(status)
	run.HealthStatus = RunHealthStatus(health)
	run.CompletedAt = completedAt
	return run, nil
}

func (s *PostgresStore) listItems(ctx context.Context, runID string) ([]ItemRecord, error) {
	rows, err := s.pool.Query(ctx, `
SELECT
  id, run_id, dedupe_key, classification, action_kind, workflow_status,
  confidence::FLOAT8, COALESCE(platform, ''), COALESCE(source, ''), COALESCE(error_code, ''),
  COALESCE(platform_error_code, ''), COALESCE(failure_stage, ''),
  affected_user_count, affected_workspace_count, affected_post_count,
  latest_failure_at, evidence_json, COALESCE(ai_summary, ''), COALESCE(admin_notes, ''),
  COALESCE(bug_plan_json, '{}'::jsonb), COALESCE(email_draft_json, '{}'::jsonb),
  COALESCE(cta_url, ''), COALESCE(duplicate_of_item_id, ''), created_at, updated_at
FROM error_triage_items
WHERE run_id = $1
ORDER BY created_at ASC
`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []ItemRecord{}
	for rows.Next() {
		item, err := scanItemRecord(rows)
		if err != nil {
			return nil, err
		}
		recipients, err := s.listRecipients(ctx, item.ID)
		if err != nil {
			return nil, err
		}
		item.Recipients = recipients
		items = append(items, item)
	}
	return items, rows.Err()
}

func scanItemRecord(row pgx.Row) (ItemRecord, error) {
	var item ItemRecord
	var classification, actionKind, workflowStatus string
	var latest *time.Time
	if err := row.Scan(
		&item.ID,
		&item.RunID,
		&item.DedupeKey,
		&classification,
		&actionKind,
		&workflowStatus,
		&item.Confidence,
		&item.Platform,
		&item.Source,
		&item.ErrorCode,
		&item.PlatformErrorCode,
		&item.FailureStage,
		&item.AffectedUserCount,
		&item.AffectedWorkspaceCount,
		&item.AffectedPostCount,
		&latest,
		&item.EvidenceJSON,
		&item.AISummary,
		&item.AdminNotes,
		&item.BugPlanJSON,
		&item.EmailDraftJSON,
		&item.CTAURL,
		&item.DuplicateOfItemID,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		return ItemRecord{}, err
	}
	item.Classification = Classification(classification)
	item.ActionKind = ActionKind(actionKind)
	item.WorkflowStatus = WorkflowStatus(workflowStatus)
	item.LatestFailureAt = latest
	return item, nil
}

func (s *PostgresStore) listRecipients(ctx context.Context, itemID string) ([]RecipientRecord, error) {
	rows, err := s.pool.Query(ctx, `
SELECT
  r.id, r.item_id, r.recipient_scope_key, r.workspace_id, r.recipient_user_id,
  r.email_snapshot, COALESCE(u.email, ''), r.status, COALESCE(r.latest_send_attempt_id, ''),
  COALESCE(r.dismiss_reason, ''), r.created_at, r.updated_at
FROM error_triage_item_recipients r
LEFT JOIN users u ON u.id = r.recipient_user_id
WHERE r.item_id = $1
ORDER BY r.created_at ASC
`, itemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []RecipientRecord{}
	for rows.Next() {
		var r RecipientRecord
		var status string
		if err := rows.Scan(
			&r.ID,
			&r.ItemID,
			&r.RecipientScopeKey,
			&r.WorkspaceID,
			&r.RecipientUserID,
			&r.EmailSnapshot,
			&r.CurrentEmail,
			&status,
			&r.LatestSendAttemptID,
			&r.DismissReason,
			&r.CreatedAt,
			&r.UpdatedAt,
		); err != nil {
			return nil, err
		}
		r.Status = RecipientStatus(status)
		out = append(out, r)
	}
	return out, rows.Err()
}

func nullEmpty(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func nullableString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func nullableTime(value time.Time) any {
	if value.IsZero() {
		return nil
	}
	return value
}

func (s *PostgresStore) String() string {
	return fmt.Sprintf("PostgresStore(%p)", s.pool)
}
