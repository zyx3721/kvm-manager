-- 企业微信账号绑定：一个系统用户只能绑定一个企微账号，一个企微账号只能绑定一个用户。
CREATE TABLE IF NOT EXISTS auth_wecom_bindings (
    user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    wecom_userid TEXT NOT NULL UNIQUE,
    bound_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- OAuth state 区分用途（login/bind）并记录绑定发起者，取出即删语义不变。
ALTER TABLE auth_login_states ADD COLUMN IF NOT EXISTS purpose TEXT NOT NULL DEFAULT 'login';
ALTER TABLE auth_login_states ADD COLUMN IF NOT EXISTS user_id TEXT NOT NULL DEFAULT '';
