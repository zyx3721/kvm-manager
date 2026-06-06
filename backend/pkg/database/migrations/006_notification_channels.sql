CREATE TABLE IF NOT EXISTS notification_channels (
  id TEXT PRIMARY KEY,
  enabled BOOLEAN NOT NULL DEFAULT FALSE,
  config JSONB NOT NULL DEFAULT '{}',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO notification_channels(id, enabled, config)
VALUES
  ('platform', TRUE, '{}'),
  ('webhook', FALSE, '{}'),
  ('email', FALSE, '{}'),
  ('lark', FALSE, '{}'),
  ('wechat', FALSE, '{}'),
  ('dingtalk', FALSE, '{}')
ON CONFLICT (id) DO NOTHING;

ALTER TABLE alerts
  ADD COLUMN IF NOT EXISTS notification_sent_at TIMESTAMPTZ;
