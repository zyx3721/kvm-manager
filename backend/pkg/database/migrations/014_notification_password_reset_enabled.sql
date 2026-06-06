ALTER TABLE notification_channels
  ADD COLUMN IF NOT EXISTS password_reset_enabled BOOLEAN NOT NULL DEFAULT FALSE;
