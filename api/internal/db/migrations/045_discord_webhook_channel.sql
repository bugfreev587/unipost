-- +goose Up

-- Add discord_webhook as a supported notification channel kind.
ALTER TABLE notification_channels
  DROP CONSTRAINT notification_channels_kind_check,
  ADD CONSTRAINT notification_channels_kind_check
    CHECK (kind IN ('email','slack_webhook','discord_webhook','sms','in_app'));

-- +goose Down

ALTER TABLE notification_channels
  DROP CONSTRAINT notification_channels_kind_check,
  ADD CONSTRAINT notification_channels_kind_check
    CHECK (kind IN ('email','slack_webhook','sms','in_app'));
