-- 用户登录会话表：jti 与签发的 JWT 一一对应，注销删除行、过期登录时惰性清理。
CREATE TABLE IF NOT EXISTS user_sessions (
  jti TEXT PRIMARY KEY,
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  username TEXT NOT NULL,
  source TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  expires_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_user_sessions_expires_at ON user_sessions (expires_at);

-- 旧 token_hash 会话表废弃：令牌格式改为 JWT，历史会话全部失效。
DROP TABLE IF EXISTS sessions;
