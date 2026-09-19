-- 新增企业微信认证提供方：直连企业微信与统一认证中心两种接入方式。
INSERT INTO auth_providers(id, type, name, enabled, config)
VALUES
    ('wecom', 'wecom', '企业微信·直连', FALSE, '{}'),
    ('wecom_center', 'wecom_center', '企业微信·统一认证中心', FALSE, '{}')
ON CONFLICT (id) DO NOTHING;

-- OAuth 登录 state 一次性存储：回调时取出即删，防止重放与跨提供方复用。
CREATE TABLE IF NOT EXISTS auth_login_states (
    state TEXT PRIMARY KEY,
    provider TEXT NOT NULL,
    redirect TEXT NOT NULL DEFAULT '',
    remote_ip TEXT NOT NULL DEFAULT '',
    expires_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_auth_login_states_expires ON auth_login_states (expires_at);
