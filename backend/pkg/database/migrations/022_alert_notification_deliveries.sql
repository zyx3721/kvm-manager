CREATE TABLE IF NOT EXISTS alert_notification_deliveries (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  alert_id UUID NOT NULL REFERENCES alerts(id) ON DELETE CASCADE,
  event_type TEXT NOT NULL CHECK (event_type IN ('problem', 'recovery')),
  channel_id TEXT NOT NULL REFERENCES notification_channels(id) ON DELETE CASCADE,
  status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'sent', 'failed')),
  payload JSONB NOT NULL DEFAULT '{}',
  error TEXT NOT NULL DEFAULT '',
  retry_count INTEGER NOT NULL DEFAULT 0,
  next_retry_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  last_attempt_at TIMESTAMPTZ,
  sent_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(alert_id, event_type, channel_id)
);

ALTER TABLE alert_notification_deliveries
  ADD COLUMN IF NOT EXISTS retry_count INTEGER NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS next_retry_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  ADD COLUMN IF NOT EXISTS last_attempt_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_alert_notification_deliveries_pending
  ON alert_notification_deliveries(next_retry_at, created_at)
  WHERE status = 'pending';

CREATE INDEX IF NOT EXISTS idx_alert_notification_deliveries_alert
  ON alert_notification_deliveries(alert_id, created_at);
