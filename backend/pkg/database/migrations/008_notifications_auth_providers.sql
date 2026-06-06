ALTER TABLE alerts
  ADD COLUMN IF NOT EXISTS read_at TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS dismissed_at TIMESTAMPTZ;

DELETE FROM notification_channels WHERE id = 'platform';

CREATE TABLE IF NOT EXISTS auth_providers (
  id TEXT PRIMARY KEY,
  type TEXT NOT NULL,
  name TEXT NOT NULL,
  enabled BOOLEAN NOT NULL DEFAULT FALSE,
  config JSONB NOT NULL DEFAULT '{}',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO auth_providers(id, type, name, enabled, config)
VALUES
  ('ldap', 'ldap', 'AD/LDAP', FALSE, '{}')
ON CONFLICT (id) DO NOTHING;
