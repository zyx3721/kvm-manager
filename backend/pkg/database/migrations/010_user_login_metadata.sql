ALTER TABLE users
  ADD COLUMN IF NOT EXISTS source TEXT NOT NULL DEFAULT 'local',
  ADD COLUMN IF NOT EXISTS last_login_at TIMESTAMPTZ;

UPDATE users SET source = 'local' WHERE source IS NULL OR source = '';

CREATE INDEX IF NOT EXISTS idx_users_source ON users(source);
