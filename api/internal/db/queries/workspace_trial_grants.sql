-- name: CreateWorkspaceTrialGrant :one
INSERT INTO workspace_trial_grants (
  workspace_id,
  kind,
  plan_id,
  duration_days,
  status,
  granted_by_user_id,
  stripe_mode,
  stripe_customer_id,
  stripe_subscription_id,
  granted_at
) VALUES (
  sqlc.arg(workspace_id),
  sqlc.arg(kind),
  sqlc.arg(plan_id),
  sqlc.arg(duration_days),
  sqlc.arg(status),
  sqlc.arg(granted_by_user_id),
  sqlc.narg(stripe_mode),
  sqlc.narg(stripe_customer_id),
  sqlc.narg(stripe_subscription_id),
  sqlc.arg(granted_at)
)
RETURNING *;

-- name: GetWorkspaceTrialGrant :one
SELECT *
FROM workspace_trial_grants
WHERE id = sqlc.arg(id);

-- name: GetOpenWorkspaceTrialGrant :one
SELECT *
FROM workspace_trial_grants
WHERE workspace_id = sqlc.arg(workspace_id)
  AND status IN ('provisioning', 'pending_activation', 'checkout_pending', 'scheduled', 'active');

-- name: GetWorkspaceTrialGrantByCheckoutSession :one
SELECT *
FROM workspace_trial_grants
WHERE stripe_checkout_session_id = sqlc.arg(stripe_checkout_session_id);

-- name: GetWorkspaceTrialGrantBySubscription :one
SELECT *
FROM workspace_trial_grants
WHERE stripe_subscription_id = sqlc.arg(stripe_subscription_id)
ORDER BY granted_at DESC
LIMIT 1;

-- name: GetWorkspaceTrialGrantBySchedule :one
SELECT *
FROM workspace_trial_grants
WHERE stripe_schedule_id = sqlc.arg(stripe_schedule_id)
ORDER BY granted_at DESC
LIMIT 1;

-- name: ClaimWorkspaceTrialGrantCheckout :one
UPDATE workspace_trial_grants
SET status = 'checkout_pending',
    updated_at = NOW()
WHERE id = sqlc.arg(id)
  AND workspace_id = sqlc.arg(workspace_id)
  AND plan_id = sqlc.arg(plan_id)
  AND status = 'pending_activation'
RETURNING *;

-- name: RecordWorkspaceTrialGrantCheckoutSession :one
UPDATE workspace_trial_grants
SET stripe_checkout_session_id = sqlc.arg(stripe_checkout_session_id),
    updated_at = NOW()
WHERE id = sqlc.arg(id)
  AND status = 'checkout_pending'
  AND stripe_checkout_session_id IS NULL
RETURNING *;

-- name: ReleaseExpiredWorkspaceTrialGrantCheckout :one
UPDATE workspace_trial_grants
SET status = 'pending_activation',
    stripe_checkout_session_id = NULL,
    updated_at = NOW()
WHERE id = sqlc.arg(id)
  AND status = 'checkout_pending'
  AND stripe_checkout_session_id = sqlc.arg(stripe_checkout_session_id)
  AND updated_at < sqlc.arg(expired_before)
RETURNING *;

-- name: ListWorkspaceTrialGrantHistory :many
SELECT *
FROM workspace_trial_grants
WHERE workspace_id = sqlc.arg(workspace_id)
ORDER BY granted_at DESC, id DESC;

-- name: TransitionWorkspaceTrialGrantStatus :one
UPDATE workspace_trial_grants
SET status = sqlc.arg(new_status),
    updated_at = NOW()
WHERE id = sqlc.arg(id)
  AND status = sqlc.arg(expected_status)
RETURNING *;

-- name: MarkWorkspaceTrialGrantScheduled :one
UPDATE workspace_trial_grants
SET status = 'scheduled',
    stripe_customer_id = sqlc.arg(stripe_customer_id),
    stripe_subscription_id = sqlc.narg(stripe_subscription_id),
    stripe_schedule_id = sqlc.arg(stripe_schedule_id),
    scheduled_start_at = sqlc.arg(scheduled_start_at),
    ends_at = sqlc.arg(ends_at),
    failure_code = NULL,
    failure_message = NULL,
    updated_at = NOW()
WHERE id = sqlc.arg(id)
  AND status = sqlc.arg(expected_status)
RETURNING *;

-- name: RecordWorkspaceTrialGrantProvisioningSchedule :one
UPDATE workspace_trial_grants
SET stripe_schedule_id = sqlc.arg(stripe_schedule_id),
    failure_code = sqlc.arg(failure_code),
    failure_message = sqlc.arg(failure_message),
    updated_at = NOW()
WHERE id = sqlc.arg(id)
  AND status = 'provisioning'
  AND (stripe_schedule_id IS NULL OR stripe_schedule_id = sqlc.arg(stripe_schedule_id))
RETURNING *;

-- name: MarkWorkspaceTrialGrantActive :one
UPDATE workspace_trial_grants
SET status = 'active',
    stripe_customer_id = sqlc.arg(stripe_customer_id),
    stripe_subscription_id = sqlc.arg(stripe_subscription_id),
    started_at = sqlc.arg(started_at),
    ends_at = sqlc.arg(ends_at),
    activated_at = sqlc.arg(activated_at),
    failure_code = NULL,
    failure_message = NULL,
    updated_at = NOW()
WHERE id = sqlc.arg(id)
  AND status = sqlc.arg(expected_status)
RETURNING *;

-- name: MarkWorkspaceTrialGrantCanceled :one
UPDATE workspace_trial_grants
SET status = 'canceled',
    canceled_at = sqlc.arg(canceled_at),
    updated_at = NOW()
WHERE id = sqlc.arg(id)
  AND status = sqlc.arg(expected_status)
RETURNING *;

-- name: MarkWorkspaceTrialGrantRevoked :one
UPDATE workspace_trial_grants
SET status = 'revoked',
    revoked_at = sqlc.arg(revoked_at),
    updated_at = NOW()
WHERE id = sqlc.arg(id)
  AND status = sqlc.arg(expected_status)
RETURNING *;

-- name: MarkWorkspaceTrialGrantSuperseded :one
UPDATE workspace_trial_grants
SET status = 'superseded',
    superseded_at = sqlc.arg(superseded_at),
    superseded_by_plan_id = sqlc.arg(superseded_by_plan_id),
    updated_at = NOW()
WHERE id = sqlc.arg(id)
  AND status = sqlc.arg(expected_status)
RETURNING *;

-- name: MarkWorkspaceTrialGrantCompleted :one
UPDATE workspace_trial_grants
SET status = 'completed',
    completed_at = sqlc.arg(completed_at),
    updated_at = NOW()
WHERE id = sqlc.arg(id)
  AND status = sqlc.arg(expected_status)
RETURNING *;

-- name: MarkWorkspaceTrialGrantFailed :one
UPDATE workspace_trial_grants
SET status = 'failed',
    failure_code = sqlc.arg(failure_code),
    failure_message = sqlc.arg(failure_message),
    updated_at = NOW()
WHERE id = sqlc.arg(id)
  AND status = sqlc.arg(expected_status)
RETURNING *;
