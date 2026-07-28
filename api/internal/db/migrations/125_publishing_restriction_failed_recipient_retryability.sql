-- +goose Up

-- Migration 124 added retryable with DEFAULT TRUE, which also marked existing
-- failed recipients as eligible for automatic delivery. Converge databases
-- where 124 already ran and fresh databases on the safe terminal state.
UPDATE platform_publishing_restriction_email_recipients
SET retryable = FALSE
WHERE status = 'failed';

-- +goose Down

-- This retryability correction is irreversible: the pre-125 retryability of
-- failed recipients was not trustworthy and cannot be reconstructed. A Down
-- migration must not reactivate recipients that may already have been sent.
