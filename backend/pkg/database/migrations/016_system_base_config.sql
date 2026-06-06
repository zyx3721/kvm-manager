CREATE TABLE IF NOT EXISTS system_settings (
  key TEXT PRIMARY KEY,
  value JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO system_settings(key, value)
VALUES (
  'base_config',
  '{"siteName":"KVM Manager","loginName":"KVM Manager","appName":"KVM Manager","appSubtitle":"VIRTUALIZATION OPS","iconData":"/favicon.svg","passwordResetCodeTtlMinutes":10,"passwordResetCaptchaTtlMinutes":1,"passwordResetSendCooldownMinutes":0.5,"passwordResetRateLimitMinutes":5,"resourceWarningThreshold":70,"resourceCriticalThreshold":85,"resourceAlertConsecutiveCount":3,"agentOfflineFailureCount":3}'::jsonb
)
ON CONFLICT (key) DO NOTHING;

UPDATE system_settings
SET value = value || '{"appSubtitle":"VIRTUALIZATION OPS"}'::jsonb,
    updated_at = now()
WHERE key = 'base_config'
  AND NOT (value ? 'appSubtitle');

UPDATE roles
SET permissions = (
  SELECT ARRAY(
    SELECT DISTINCT permission
    FROM unnest(permissions || ARRAY[
      CASE WHEN 'settings.base.manage' = ANY(permissions) THEN 'settings.base.read' END
    ]) AS permission
    WHERE permission IS NOT NULL AND permission <> ''
    ORDER BY permission
  )
)
WHERE builtin = FALSE
  AND 'settings.base.manage' = ANY(permissions);

UPDATE roles
SET permissions = ARRAY[
  'dashboard.read','hosts.read','host.interfaces.read','host.interfaces.manage','agents.read','agents.manage',
  'vms.read','vms.create','vms.update','vms.power','vms.delete','vms.force','vms.console','vms.clone','vms.migrate',
  'snapshots.read','snapshots.create','snapshots.update','snapshots.revert','snapshots.delete','storage.read','storage.manage','network.read','network.manage',
  'operations.read','alerts.manage',
  'settings.base.read','settings.base.manage','settings.users.read','settings.users.manage','settings.auth.read','settings.auth.manage','settings.notifications.read','settings.notifications.manage'
],
updated_at = now()
WHERE key = 'admin' AND builtin = TRUE;
