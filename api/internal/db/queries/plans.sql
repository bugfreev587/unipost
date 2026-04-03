-- name: GetPlan :one
SELECT * FROM plans WHERE id = $1;

-- name: ListPlans :many
SELECT * FROM plans ORDER BY price_cents ASC;

-- name: GetPlanByStripePriceID :one
SELECT * FROM plans WHERE stripe_price_id = $1;
