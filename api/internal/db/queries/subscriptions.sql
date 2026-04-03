-- name: CreateSubscription :one
INSERT INTO subscriptions (project_id, plan_id, stripe_customer_id, stripe_subscription_id, status)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (project_id) DO UPDATE
SET plan_id = EXCLUDED.plan_id,
    stripe_customer_id = EXCLUDED.stripe_customer_id,
    stripe_subscription_id = EXCLUDED.stripe_subscription_id,
    status = EXCLUDED.status,
    updated_at = NOW()
RETURNING *;

-- name: EnsureSubscription :exec
INSERT INTO subscriptions (project_id, plan_id, status)
VALUES ($1, 'free', 'active')
ON CONFLICT (project_id) DO NOTHING;

-- name: GetSubscriptionByProject :one
SELECT * FROM subscriptions WHERE project_id = $1;

-- name: GetSubscriptionByStripeCustomer :one
SELECT * FROM subscriptions WHERE stripe_customer_id = $1;

-- name: GetSubscriptionByStripeSubscription :one
SELECT * FROM subscriptions WHERE stripe_subscription_id = $1;

-- name: UpdateSubscriptionPlan :exec
UPDATE subscriptions
SET plan_id = $2, status = $3, updated_at = NOW()
WHERE project_id = $1;

-- name: UpdateSubscriptionStripe :exec
UPDATE subscriptions
SET stripe_customer_id = $2, stripe_subscription_id = $3, plan_id = $4, status = $5,
    current_period_start = $6, current_period_end = $7, updated_at = NOW()
WHERE project_id = $1;

-- name: UpdateSubscriptionStatus :exec
UPDATE subscriptions SET status = $2, updated_at = NOW() WHERE stripe_subscription_id = $1;

-- name: CancelSubscription :exec
UPDATE subscriptions
SET plan_id = 'free', status = 'active', stripe_subscription_id = NULL,
    cancel_at_period_end = FALSE, updated_at = NOW()
WHERE stripe_subscription_id = $1;
