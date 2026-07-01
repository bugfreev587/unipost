-- +goose Up
DELETE FROM ai_surface_routing WHERE surface = 'app_review_ai';

ALTER TABLE ai_surface_routing
    DROP CONSTRAINT IF EXISTS ai_surface_routing_surface_check;
ALTER TABLE ai_surface_routing
    ADD CONSTRAINT ai_surface_routing_surface_check
    CHECK (surface IN ('post_assist', 'error_triage'));

-- +goose Down
ALTER TABLE ai_surface_routing
    DROP CONSTRAINT IF EXISTS ai_surface_routing_surface_check;
ALTER TABLE ai_surface_routing
    ADD CONSTRAINT ai_surface_routing_surface_check
    CHECK (surface IN ('post_assist', 'error_triage', 'app_review_ai'));
