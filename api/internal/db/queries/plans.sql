-- name: GetPlan :one
SELECT * FROM plans WHERE id = $1;

-- name: ListPlans :many
SELECT * FROM plans ORDER BY price_cents ASC;

-- name: GetPlanByStripePriceID :one
SELECT * FROM plans WHERE stripe_price_id = $1;

-- name: UpdatePlanStripePriceID :exec
UPDATE plans SET stripe_price_id = $2 WHERE id = $1;
