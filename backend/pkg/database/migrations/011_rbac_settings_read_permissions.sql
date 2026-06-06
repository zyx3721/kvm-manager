UPDATE roles
SET permissions = (
  SELECT ARRAY(
    SELECT DISTINCT permission
    FROM unnest(permissions || ARRAY[
      CASE WHEN 'settings.users.manage' = ANY(permissions) THEN 'settings.users.read' END,
      CASE WHEN 'settings.auth.manage' = ANY(permissions) THEN 'settings.auth.read' END,
      CASE WHEN 'settings.notifications.manage' = ANY(permissions) THEN 'settings.notifications.read' END,
      CASE WHEN 'alerts.manage' = ANY(permissions) THEN 'operations.read' END,
      CASE WHEN 'agents.manage' = ANY(permissions) THEN 'agents.read' END,
      CASE WHEN 'vms.create' = ANY(permissions) THEN 'vms.read' END,
      CASE WHEN 'vms.update' = ANY(permissions) THEN 'vms.read' END,
      CASE WHEN 'vms.power' = ANY(permissions) THEN 'vms.read' END,
      CASE WHEN 'vms.delete' = ANY(permissions) THEN 'vms.read' END,
      CASE WHEN 'vms.force' = ANY(permissions) THEN 'vms.read' END,
      CASE WHEN 'vms.console' = ANY(permissions) THEN 'vms.read' END,
      CASE WHEN 'vms.clone' = ANY(permissions) THEN 'vms.read' END,
      CASE WHEN 'vms.migrate' = ANY(permissions) THEN 'vms.read' END,
      CASE WHEN 'snapshots.manage' = ANY(permissions) THEN 'snapshots.read' END,
      CASE WHEN 'snapshots.create' = ANY(permissions) THEN 'snapshots.read' END,
      CASE WHEN 'snapshots.update' = ANY(permissions) THEN 'snapshots.read' END,
      CASE WHEN 'snapshots.revert' = ANY(permissions) THEN 'snapshots.read' END,
      CASE WHEN 'snapshots.delete' = ANY(permissions) THEN 'snapshots.read' END,
      CASE WHEN 'storage.manage' = ANY(permissions) THEN 'storage.read' END,
      CASE WHEN 'network.manage' = ANY(permissions) THEN 'network.read' END
    ]) AS permission
    WHERE permission IS NOT NULL AND permission <> ''
    ORDER BY permission
  )
)
WHERE builtin = FALSE
  AND (
    'settings.users.manage' = ANY(permissions)
    OR 'settings.auth.manage' = ANY(permissions)
    OR 'settings.notifications.manage' = ANY(permissions)
    OR 'alerts.manage' = ANY(permissions)
    OR 'agents.manage' = ANY(permissions)
    OR 'vms.create' = ANY(permissions)
    OR 'vms.update' = ANY(permissions)
    OR 'vms.power' = ANY(permissions)
    OR 'vms.delete' = ANY(permissions)
    OR 'vms.force' = ANY(permissions)
    OR 'vms.console' = ANY(permissions)
    OR 'vms.clone' = ANY(permissions)
    OR 'vms.migrate' = ANY(permissions)
    OR 'snapshots.manage' = ANY(permissions)
    OR 'snapshots.create' = ANY(permissions)
    OR 'snapshots.update' = ANY(permissions)
    OR 'snapshots.revert' = ANY(permissions)
    OR 'snapshots.delete' = ANY(permissions)
    OR 'storage.manage' = ANY(permissions)
    OR 'network.manage' = ANY(permissions)
  );

UPDATE roles
SET permissions = ARRAY[
  'dashboard.read','hosts.read','agents.read','agents.manage',
  'vms.read','vms.create','vms.update','vms.power','vms.delete','vms.force','vms.console','vms.clone','vms.migrate',
  'snapshots.read','snapshots.create','snapshots.update','snapshots.revert','snapshots.delete','storage.read','storage.manage','network.read','network.manage',
  'operations.read','alerts.manage',
  'settings.users.read','settings.users.manage','settings.auth.read','settings.auth.manage','settings.notifications.read','settings.notifications.manage'
],
updated_at = now()
WHERE key = 'admin' AND builtin = TRUE;
