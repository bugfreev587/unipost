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
  event_key = CASE WHEN email_send_attempts.status = 'sent' THEN email_send_attempts.event_key ELSE EXCLUDED.event_key END,
  recipient_user_id = CASE WHEN email_send_attempts.status = 'sent' THEN email_send_attempts.recipient_user_id ELSE EXCLUDED.recipient_user_id END,
  recipient_email = CASE WHEN email_send_attempts.status = 'sent' THEN email_send_attempts.recipient_email ELSE EXCLUDED.recipient_email END,
  workspace_id = CASE WHEN email_send_attempts.status = 'sent' THEN email_send_attempts.workspace_id ELSE EXCLUDED.workspace_id END,
  provider_template_id = CASE WHEN email_send_attempts.status = 'sent' THEN email_send_attempts.provider_template_id ELSE EXCLUDED.provider_template_id END,
  delivery_class = CASE WHEN email_send_attempts.status = 'sent' THEN email_send_attempts.delivery_class ELSE EXCLUDED.delivery_class END,
  status = CASE WHEN email_send_attempts.status = 'sent' THEN email_send_attempts.status ELSE 'pending' END,
  subject_snapshot = CASE WHEN email_send_attempts.status = 'sent' THEN email_send_attempts.subject_snapshot ELSE EXCLUDED.subject_snapshot END,
  data_variables_snapshot = CASE WHEN email_send_attempts.status = 'sent' THEN email_send_attempts.data_variables_snapshot ELSE EXCLUDED.data_variables_snapshot END,
  trigger_source = CASE WHEN email_send_attempts.status = 'sent' THEN email_send_attempts.trigger_source ELSE EXCLUDED.trigger_source END,
  trigger_reference_id = CASE WHEN email_send_attempts.status = 'sent' THEN email_send_attempts.trigger_reference_id ELSE EXCLUDED.trigger_reference_id END,
  attempt_count = CASE WHEN email_send_attempts.status = 'sent' THEN email_send_attempts.attempt_count ELSE email_send_attempts.attempt_count + 1 END,
  last_error = CASE WHEN email_send_attempts.status = 'sent' THEN email_send_attempts.last_error ELSE NULL END,
  attempted_at = CASE WHEN email_send_attempts.status = 'sent' THEN email_send_attempts.attempted_at ELSE NOW() END,
  sent_at = CASE WHEN email_send_attempts.status = 'sent' THEN email_send_attempts.sent_at ELSE NULL END,
  updated_at = CASE WHEN email_send_attempts.status = 'sent' THEN email_send_attempts.updated_at ELSE NOW() END
RETURNING *;

-- name: CreateSkippedEmailSendAttemptAudit :one
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
  'skipped',
  NULLIF(sqlc.arg(subject_snapshot), ''),
  COALESCE(sqlc.arg(data_variables_snapshot)::JSONB, '{}'::JSONB),
  NULLIF(sqlc.arg(trigger_source), ''),
  NULLIF(sqlc.arg(trigger_reference_id), ''),
  1,
  sqlc.arg(last_error),
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
  status = 'skipped',
  subject_snapshot = EXCLUDED.subject_snapshot,
  data_variables_snapshot = EXCLUDED.data_variables_snapshot,
  trigger_source = EXCLUDED.trigger_source,
  trigger_reference_id = EXCLUDED.trigger_reference_id,
  attempt_count = email_send_attempts.attempt_count + 1,
  last_error = EXCLUDED.last_error,
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
