-- 用户账号表：保存本地和外部认证用户的登录资料、角色入口和状态。
CREATE TABLE IF NOT EXISTS users (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  username TEXT NOT NULL UNIQUE,
  email TEXT NOT NULL DEFAULT '',
  password_hash TEXT NOT NULL,
  display_name TEXT NOT NULL,
  role TEXT NOT NULL DEFAULT 'admin',
  source TEXT NOT NULL DEFAULT 'local',
  disabled BOOLEAN NOT NULL DEFAULT FALSE,
  last_login_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 会话表：保存登录 Token 哈希、过期时间和最后访问时间。
CREATE TABLE IF NOT EXISTS sessions (
  token_hash TEXT PRIMARY KEY,
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  expires_at TIMESTAMPTZ NOT NULL,
  last_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Agent 表：保存 KVM 宿主机 Agent 的连接信息、令牌密文和同步状态。
CREATE TABLE IF NOT EXISTS agents (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name TEXT NOT NULL UNIQUE,
  endpoint TEXT NOT NULL,
  token_hash TEXT NOT NULL,
  token_ciphertext TEXT NOT NULL DEFAULT '',
  tls_insecure BOOLEAN NOT NULL DEFAULT FALSE,
  status TEXT NOT NULL DEFAULT 'unknown',
  version TEXT NOT NULL DEFAULT '',
  capabilities JSONB NOT NULL DEFAULT '[]',
  last_heartbeat_at TIMESTAMPTZ,
  last_error TEXT NOT NULL DEFAULT '',
  failure_count INT NOT NULL DEFAULT 0,
  last_sync_started_at TIMESTAMPTZ,
  last_sync_finished_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 任务表：保存刷新、虚拟机操作等后台任务的状态、目标和进度负载。
CREATE TABLE IF NOT EXISTS tasks (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  type TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'pending',
  target_type TEXT NOT NULL,
  target_id TEXT,
  payload JSONB NOT NULL DEFAULT '{}',
  error_message TEXT,
  created_by UUID REFERENCES users(id) ON DELETE SET NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  finished_at TIMESTAMPTZ
);

-- 审计日志表：记录用户触发的平台配置、资源操作和关键状态变更。
CREATE TABLE IF NOT EXISTS audit_logs (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID REFERENCES users(id) ON DELETE SET NULL,
  action TEXT NOT NULL,
  resource_type TEXT NOT NULL,
  resource_id TEXT,
  ip_address TEXT,
  metadata JSONB NOT NULL DEFAULT '{}',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 告警表：保存 Agent 离线、资源阈值等活跃或已恢复告警。
CREATE TABLE IF NOT EXISTS alerts (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  level TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'active',
  source_type TEXT NOT NULL,
  source_id TEXT NOT NULL,
  title TEXT NOT NULL,
  message TEXT NOT NULL,
  metadata JSONB NOT NULL DEFAULT '{}',
  read_at TIMESTAMPTZ,
  dismissed_at TIMESTAMPTZ,
  notification_sent_at TIMESTAMPTZ,
  first_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  last_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  resolved_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 宿主机指标样本表：保存按 Agent 采集的 CPU、内存、存储和 I/O 原始样本。
CREATE TABLE IF NOT EXISTS host_metric_samples (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
  host_name TEXT NOT NULL,
  cpu_usage INT NOT NULL,
  memory_usage INT NOT NULL,
  memory_bytes BIGINT NOT NULL,
  storage_usage INT NOT NULL,
  storage_bytes BIGINT NOT NULL,
  disk_read_bytes_per_second BIGINT NOT NULL DEFAULT 0,
  disk_write_bytes_per_second BIGINT NOT NULL DEFAULT 0,
  network_rx_bytes_per_second BIGINT NOT NULL DEFAULT 0,
  network_tx_bytes_per_second BIGINT NOT NULL DEFAULT 0,
  vm_count INT NOT NULL,
  collected_at TIMESTAMPTZ NOT NULL
);

-- 虚拟机指标样本表：保存虚拟机 CPU、内存、磁盘、网络和运行时长原始样本。
CREATE TABLE IF NOT EXISTS vm_metric_samples (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
  vm_id TEXT NOT NULL,
  vm_name TEXT NOT NULL,
  status TEXT NOT NULL,
  cpu_usage INT NOT NULL,
  cpu_usage_available BOOLEAN NOT NULL,
  memory_usage INT NOT NULL,
  memory_usage_available BOOLEAN NOT NULL,
  disk_usage INT NOT NULL,
  disk_usage_available BOOLEAN NOT NULL,
  disk_read_bytes_per_second BIGINT NOT NULL DEFAULT 0,
  disk_write_bytes_per_second BIGINT NOT NULL DEFAULT 0,
  network_rx_bytes_per_second BIGINT NOT NULL DEFAULT 0,
  network_tx_bytes_per_second BIGINT NOT NULL DEFAULT 0,
  uptime_seconds BIGINT NOT NULL,
  collected_at TIMESTAMPTZ NOT NULL
);

-- 宿主机指标聚合表：保存按时间桶聚合后的宿主机监控数据。
CREATE TABLE IF NOT EXISTS host_metric_rollups (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  bucket_size TEXT NOT NULL,
  bucket_at TIMESTAMPTZ NOT NULL,
  agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
  host_name TEXT NOT NULL,
  cpu_usage INT NOT NULL,
  memory_usage INT NOT NULL,
  storage_usage INT NOT NULL,
  disk_read_bytes_per_second BIGINT NOT NULL DEFAULT 0,
  disk_write_bytes_per_second BIGINT NOT NULL DEFAULT 0,
  network_rx_bytes_per_second BIGINT NOT NULL DEFAULT 0,
  network_tx_bytes_per_second BIGINT NOT NULL DEFAULT 0,
  vm_count INT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(bucket_size, bucket_at, agent_id)
);

-- 虚拟机指标聚合表：保存按时间桶聚合后的虚拟机监控数据。
CREATE TABLE IF NOT EXISTS vm_metric_rollups (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  bucket_size TEXT NOT NULL,
  bucket_at TIMESTAMPTZ NOT NULL,
  agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
  vm_id TEXT NOT NULL,
  vm_name TEXT NOT NULL,
  status TEXT NOT NULL,
  cpu_usage INT NOT NULL,
  memory_usage INT NOT NULL,
  disk_usage INT NOT NULL,
  disk_read_bytes_per_second BIGINT NOT NULL DEFAULT 0,
  disk_write_bytes_per_second BIGINT NOT NULL DEFAULT 0,
  network_rx_bytes_per_second BIGINT NOT NULL DEFAULT 0,
  network_tx_bytes_per_second BIGINT NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(bucket_size, bucket_at, vm_id)
);

-- 通知渠道表：保存 Webhook、邮件、机器人和应用通知的开关与配置。
CREATE TABLE IF NOT EXISTS notification_channels (
  id TEXT PRIMARY KEY,
  enabled BOOLEAN NOT NULL DEFAULT FALSE,
  password_reset_enabled BOOLEAN NOT NULL DEFAULT FALSE,
  config JSONB NOT NULL DEFAULT '{}',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO notification_channels(id, enabled, password_reset_enabled, config)
VALUES
  ('webhook', FALSE, FALSE, '{}'),
  ('email', FALSE, FALSE, '{}'),
  ('lark', FALSE, FALSE, '{}'),
  ('lark_app', FALSE, FALSE, '{}'),
  ('wechat', FALSE, FALSE, '{}'),
  ('wechat_app', FALSE, FALSE, '{}'),
  ('dingtalk', FALSE, FALSE, '{}'),
  ('dingtalk_app', FALSE, FALSE, '{}')
ON CONFLICT (id) DO NOTHING;

-- 外部认证提供方表：保存 AD/LDAP 等登录方式的启用状态与配置。
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

-- 角色表：保存内置角色和自定义角色的权限集合。
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

INSERT INTO roles(key, name, description, permissions, builtin)
VALUES
  ('admin', 'admin', '系统配置、Agent 管理、删除/强制操作、虚拟机与模板等所有资源操作', ARRAY[
    'dashboard.read','hosts.read','host.interfaces.read','host.interfaces.manage','agents.read','agents.manage',
    'vms.read','vms.create','vms.update','vms.power','vms.delete','vms.force','vms.console','vms.clone','vms.migrate',
    'snapshots.read','snapshots.create','snapshots.update','snapshots.revert','snapshots.delete',
    'storage.read','storage.manage','network.read','network.manage',
    'operations.read','alerts.manage',
    'settings.base.read','settings.base.manage','settings.users.read','settings.users.manage','settings.auth.read','settings.auth.manage','settings.notifications.read','settings.notifications.manage'
  ], TRUE),
  ('operator', 'operator', '虚拟机启停、编辑、模板创建/标记、快照、存储池/网络池日常操作，不能修改系统配置和 Agent', ARRAY[
    'dashboard.read','hosts.read','host.interfaces.read','host.interfaces.manage',
    'vms.read','vms.create','vms.update','vms.power','vms.console','vms.clone','vms.migrate',
    'snapshots.read','snapshots.create','snapshots.update','snapshots.revert',
    'storage.read','storage.manage','network.read','network.manage','operations.read'
  ], TRUE),
  ('viewer', 'viewer', '只读查看虚拟机、模板和其他资源，不能执行写操作', ARRAY[
    'dashboard.read','hosts.read','host.interfaces.read','agents.read','vms.read','snapshots.read','storage.read','network.read','operations.read'
  ], TRUE)
ON CONFLICT (key) DO UPDATE SET
  name = EXCLUDED.name,
  description = EXCLUDED.description,
  permissions = EXCLUDED.permissions,
  builtin = TRUE,
  updated_at = now();

-- 用户角色关联表：保存用户直接分配的角色。
CREATE TABLE IF NOT EXISTS user_roles (
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  role_id UUID NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (user_id, role_id)
);

-- 用户组表：保存用户分组及其启用状态。
CREATE TABLE IF NOT EXISTS user_groups (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name TEXT NOT NULL UNIQUE,
  description TEXT NOT NULL DEFAULT '',
  disabled BOOLEAN NOT NULL DEFAULT FALSE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 用户组成员表：保存用户与用户组的成员关系。
CREATE TABLE IF NOT EXISTS user_group_members (
  group_id UUID NOT NULL REFERENCES user_groups(id) ON DELETE CASCADE,
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (group_id, user_id)
);

-- 用户组角色关联表：保存用户组继承的角色。
CREATE TABLE IF NOT EXISTS user_group_roles (
  group_id UUID NOT NULL REFERENCES user_groups(id) ON DELETE CASCADE,
  role_id UUID NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (group_id, role_id)
);

INSERT INTO user_roles(user_id, role_id)
SELECT u.id, r.id
FROM users u
JOIN roles r ON r.key = COALESCE(NULLIF(u.role, ''), 'viewer')
ON CONFLICT DO NOTHING;

-- 快照标注表：保存平台侧维护的快照显示名、备注和标签。
CREATE TABLE IF NOT EXISTS snapshot_annotations (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
  vm_name TEXT NOT NULL,
  snapshot_name TEXT NOT NULL,
  display_name TEXT NOT NULL DEFAULT '',
  description TEXT NOT NULL DEFAULT '',
  tags JSONB NOT NULL DEFAULT '[]',
  created_by UUID REFERENCES users(id) ON DELETE SET NULL,
  updated_by UUID REFERENCES users(id) ON DELETE SET NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(agent_id, vm_name, snapshot_name)
);

-- 密码重置令牌表：保存找回密码验证码哈希、收件信息和使用状态。
CREATE TABLE IF NOT EXISTS password_reset_tokens (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  channel_id TEXT NOT NULL REFERENCES notification_channels(id) ON DELETE CASCADE,
  contact TEXT NOT NULL DEFAULT '',
  code_hash TEXT NOT NULL,
  request_ip TEXT NOT NULL DEFAULT '',
  expires_at TIMESTAMPTZ NOT NULL,
  used_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 系统设置表：保存基础配置、品牌、阈值和安全时效等全局配置。
CREATE TABLE IF NOT EXISTS system_settings (
  key TEXT PRIMARY KEY,
  value JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO system_settings(key, value)
VALUES (
  'base_config',
  '{"siteName":"KVM Manager","loginName":"KVM Manager","appName":"KVM Manager","appSubtitle":"VIRTUALIZATION OPS","iconData":"/favicon.svg","passwordResetCodeTtlMinutes":10,"passwordResetCaptchaTtlMinutes":1,"passwordResetSendCooldownMinutes":0.5,"passwordResetRateLimitMinutes":5,"resourceWarningThreshold":70,"resourceCriticalThreshold":85,"resourceAlertConsecutiveCount":3,"agentOfflineFailureCount":3,"alertNotificationTimeoutSeconds":8,"alertNotificationMaxRetryCount":6,"alertNotificationRetryBaseSeconds":30,"alertNotificationRetryMaxMinutes":15,"alertNotificationBatchSize":50}'::jsonb
)
ON CONFLICT (key) DO NOTHING;

-- 虚拟机模板标记表：保存平台侧对虚拟机模板的标记和说明。
CREATE TABLE IF NOT EXISTS vm_template_marks (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
  vm_uuid TEXT NOT NULL,
  name TEXT NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  created_by UUID REFERENCES users(id) ON DELETE SET NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(agent_id, vm_uuid)
);

-- 告警通知投递表：保存告警和恢复通知的投递状态、重试信息和负载。
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

CREATE INDEX IF NOT EXISTS idx_agents_status ON agents(status);
CREATE INDEX IF NOT EXISTS idx_tasks_created_at ON tasks(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_logs_created_at ON audit_logs(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_sessions_expires_at ON sessions(expires_at);
CREATE INDEX IF NOT EXISTS idx_sessions_last_seen_at ON sessions(last_seen_at);
CREATE UNIQUE INDEX IF NOT EXISTS idx_alerts_active_source ON alerts(source_type, source_id, title) WHERE status = 'active';
CREATE INDEX IF NOT EXISTS idx_alerts_status ON alerts(status);
CREATE INDEX IF NOT EXISTS idx_alerts_last_seen_at ON alerts(last_seen_at DESC);
CREATE INDEX IF NOT EXISTS idx_users_source ON users(source);
CREATE INDEX IF NOT EXISTS idx_host_metric_samples_agent_time ON host_metric_samples(agent_id, collected_at DESC);
CREATE INDEX IF NOT EXISTS idx_vm_metric_samples_vm_time ON vm_metric_samples(vm_id, collected_at DESC);
CREATE INDEX IF NOT EXISTS idx_host_metric_samples_time_brin ON host_metric_samples USING BRIN(collected_at);
CREATE INDEX IF NOT EXISTS idx_vm_metric_samples_time_brin ON vm_metric_samples USING BRIN(collected_at);
CREATE INDEX IF NOT EXISTS idx_host_metric_rollups_agent_time ON host_metric_rollups(agent_id, bucket_size, bucket_at DESC);
CREATE INDEX IF NOT EXISTS idx_vm_metric_rollups_vm_time ON vm_metric_rollups(vm_id, bucket_size, bucket_at DESC);
CREATE INDEX IF NOT EXISTS idx_user_roles_user_id ON user_roles(user_id);
CREATE INDEX IF NOT EXISTS idx_user_group_members_user_id ON user_group_members(user_id);
CREATE INDEX IF NOT EXISTS idx_password_reset_tokens_user_created_at ON password_reset_tokens(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_password_reset_tokens_expires_at ON password_reset_tokens(expires_at);
CREATE INDEX IF NOT EXISTS idx_vm_template_marks_agent ON vm_template_marks(agent_id);
CREATE INDEX IF NOT EXISTS idx_alert_notification_deliveries_pending
  ON alert_notification_deliveries(next_retry_at, created_at)
  WHERE status = 'pending';
CREATE INDEX IF NOT EXISTS idx_alert_notification_deliveries_alert
  ON alert_notification_deliveries(alert_id, created_at);
