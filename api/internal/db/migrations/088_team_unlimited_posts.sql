-- +goose Up
--
-- Team is priced by collaboration and workflow value. Monthly publish
-- throughput is effectively zero marginal cost for UniPost, so Team uses the
-- existing -1 sentinel for unlimited posts.

UPDATE plans
SET post_limit = -1
WHERE id = 'team';

-- +goose Down

UPDATE plans
SET post_limit = 25000
WHERE id = 'team';
