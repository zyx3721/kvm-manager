CREATE TABLE IF NOT EXISTS roles (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  key TEXT NOT NULL UNIQUE,
  name TEXT NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  permissions TEXT[] NOT NULL DEFAULT '{}',
  builtin BOOLEAN NOT NULL DEFAULT FALSE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS user_roles (
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  role_id UUID NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (user_id, role_id)
);

CREATE TABLE IF NOT EXISTS user_groups (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name TEXT NOT NULL UNIQUE,
  description TEXT NOT NULL DEFAULT '',
  disabled BOOLEAN NOT NULL DEFAULT FALSE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS user_group_members (
  group_id UUID NOT NULL REFERENCES user_groups(id) ON DELETE CASCADE,
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (group_id, user_id)
);

CREATE TABLE IF NOT EXISTS user_group_roles (
  group_id UUID NOT NULL REFERENCES user_groups(id) ON DELETE CASCADE,
  role_id UUID NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (group_id, role_id)
);

INSERT INTO roles(key, name, description, permissions, builtin)
VALUES
  ('admin', 'admin', '系统配置、Agent 管理、删除/强制操作、虚拟机与模板等所有资源操作', ARRAY[
    'dashboard.read','hosts.read','agents.read','agents.manage',
    'vms.read','vms.create','vms.update','vms.power','vms.delete','vms.force','vms.console','vms.clone','vms.migrate',
    'snapshots.read','snapshots.create','snapshots.update','snapshots.revert','snapshots.delete','storage.read','storage.manage','network.read','network.manage',
    'operations.read','alerts.manage',
    'settings.users.read','settings.users.manage','settings.auth.read','settings.auth.manage','settings.notifications.read','settings.notifications.manage'
  ], TRUE),
  ('operator', 'operator', '虚拟机启停、编辑、模板创建/标记、快照、存储池/网络池日常操作，不能修改系统配置和 Agent', ARRAY[
    'dashboard.read','hosts.read',
    'vms.read','vms.create','vms.update','vms.power','vms.console','vms.clone','vms.migrate',
    'snapshots.read','snapshots.create','snapshots.update','snapshots.revert','storage.read','storage.manage','network.read','network.manage','operations.read'
  ], TRUE),
  ('viewer', 'viewer', '只读查看虚拟机、模板和其他资源，不能执行写操作', ARRAY[
    'dashboard.read','hosts.read','agents.read','vms.read','snapshots.read','storage.read','network.read','operations.read'
  ], TRUE)
ON CONFLICT (key) DO UPDATE SET
  name = EXCLUDED.name,
  description = EXCLUDED.description,
  permissions = EXCLUDED.permissions,
  builtin = TRUE,
  updated_at = now();

INSERT INTO user_roles(user_id, role_id)
SELECT u.id, r.id
FROM users u
JOIN roles r ON r.key = COALESCE(NULLIF(u.role, ''), 'viewer')
ON CONFLICT DO NOTHING;

CREATE INDEX IF NOT EXISTS idx_user_roles_user_id ON user_roles(user_id);
CREATE INDEX IF NOT EXISTS idx_user_group_members_user_id ON user_group_members(user_id);
