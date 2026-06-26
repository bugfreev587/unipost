-- name: CreateEmailSendAttemptAudit :one
INSERT INTO email_send_attempts (
  event_key,
  recipient_user_id,
  recipient_email,
  workspace_id,
  provider,
  provider_template_id,
  idempotency_key,
  delivery_class,
  status,
  subject_snapshot,
  data_variables_snapshot,
  trigger_source,
  trigger_reference_id,
  attempt_count,
  last_error,
  attempted_at,
  sent_at
)
VALUES (
  sqlc.arg(event_key),
  NULLIF(sqlc.arg(recipient_user_id), ''),
  sqlc.arg(recipient_email),
  NULLIF(sqlc.arg(workspace_id), ''),
  sqlc.arg(provider),
  NULLIF(sqlc.arg(provider_template_id), ''),
  sqlc.arg(idempotency_key),
  sqlc.arg(delivery_class),
  'pending',
  NULLIF(sqlc.arg(subject_snapshot), ''),
  COALESCE(sqlc.arg(data_variables_snapshot)::JSONB, '{}'::JSONB),
  NULLIF(sqlc.arg(trigger_source), ''),
  NULLIF(sqlc.arg(trigger_reference_id), ''),
  1,
  NULL,
  NOW(),
  NULL
)
ON CONFLICT (provider, idempotency_key) WHERE idempotency_key <> ''
DO UPDATE SET
  event_key = EXCLUDED.event_key,
  recipient_user_id = EXCLUDED.recipient_user_id,
  recipient_email = EXCLUDED.recipient_email,
  workspace_id = EXCLUDED.workspace_id,
  provider_template_id = EXCLUDED.provider_template_id,
  delivery_class = EXCLUDED.delivery_class,
  status = 'pending',
  subject_snapshot = EXCLUDED.subject_snapshot,
  data_variables_snapshot = EXCLUDED.data_variables_snapshot,
  trigger_source = EXCLUDED.trigger_source,
  trigger_reference_id = EXCLUDED.trigger_reference_id,
  attempt_count = email_send_attempts.attempt_count + 1,
  last_error = NULL,
  attempted_at = NOW(),
  sent_at = NULL,
  updated_at = NOW()
RETURNING *;

-- name: MarkEmailSendAttemptAuditSent :exec
UPDATE email_send_attempts
SET status = 'sent',
    last_error = NULL,
    sent_at = NOW(),
    updated_at = NOW()
WHERE id = $1;

-- name: MarkEmailSendAttemptAuditFailed :exec
UPDATE email_send_attempts
SET status = 'failed',
    last_error = $2,
    updated_at = NOW()
WHERE id = $1;
