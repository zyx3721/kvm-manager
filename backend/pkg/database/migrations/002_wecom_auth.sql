-- 企业微信认证提供方：单一 wecom 行，config.authMode 切换直连与统一认证中心两种接入方式。
INSERT INTO auth_providers(id, type, name, enabled, config)
VALUES ('wecom', 'wecom', '企业微信', FALSE, '{}')
ON CONFLICT (id) DO NOTHING;

-- OAuth 登录/绑定 state 一次性存储：回调时取出即删，防止重放；
-- purpose 区分 login/bind，bind 场景 user_id 记录发起绑定的用户。
CREATE TABLE IF NOT EXISTS auth_login_states (
    state TEXT PRIMARY KEY,
    provider TEXT NOT NULL,
    purpose TEXT NOT NULL DEFAULT 'login',
    user_id TEXT NOT NULL DEFAULT '',
    redirect TEXT NOT NULL DEFAULT '',
    remote_ip TEXT NOT NULL DEFAULT '',
    expires_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_auth_login_states_expires ON auth_login_states (expires_at);

-- 企业微信账号绑定：一个系统用户只能绑定一个企微账号，一个企微账号只能绑定一个用户。
CREATE TABLE IF NOT EXISTS auth_wecom_bindings (
    user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    wecom_userid TEXT NOT NULL UNIQUE,
    bound_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
