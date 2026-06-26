-- +goose Up

ALTER TABLE notification_deliveries
  DROP CONSTRAINT IF EXISTS notification_deliveries_status_check;

ALTER TABLE notification_deliveries
  ADD CONSTRAINT notification_deliveries_status_check
  CHECK (status IN ('pending','sent','failed','dead','skipped'));

-- +goose Down

UPDATE notification_deliveries
SET status = 'sent'
WHERE status = 'skipped';

ALTER TABLE notification_deliveries
  DROP CONSTRAINT IF EXISTS notification_deliveries_status_check;

ALTER TABLE notification_deliveries
  ADD CONSTRAINT notification_deliveries_status_check
  CHECK (status IN ('pending','sent','failed','dead'));
