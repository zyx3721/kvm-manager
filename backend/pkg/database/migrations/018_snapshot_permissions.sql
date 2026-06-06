UPDATE roles
SET permissions = (
  SELECT ARRAY(
    SELECT DISTINCT permission
    FROM unnest(array_remove(permissions, 'snapshots.manage') || ARRAY[
      CASE WHEN 'snapshots.manage' = ANY(permissions) THEN 'snapshots.read' END,
      CASE WHEN 'snapshots.manage' = ANY(permissions) THEN 'snapshots.create' END,
      CASE WHEN 'snapshots.manage' = ANY(permissions) THEN 'snapshots.update' END,
      CASE WHEN 'snapshots.manage' = ANY(permissions) THEN 'snapshots.revert' END,
      CASE WHEN 'snapshots.manage' = ANY(permissions) THEN 'snapshots.delete' END,
      CASE WHEN 'snapshots.create' = ANY(permissions) THEN 'snapshots.read' END,
      CASE WHEN 'snapshots.update' = ANY(permissions) THEN 'snapshots.read' END,
      CASE WHEN 'snapshots.revert' = ANY(permissions) THEN 'snapshots.read' END,
      CASE WHEN 'snapshots.delete' = ANY(permissions) THEN 'snapshots.read' END
    ]) AS permission
    WHERE permission IS NOT NULL AND permission <> ''
    ORDER BY permission
  )
)
WHERE 'snapshots.manage' = ANY(permissions)
   OR 'snapshots.create' = ANY(permissions)
   OR 'snapshots.update' = ANY(permissions)
   OR 'snapshots.revert' = ANY(permissions)
   OR 'snapshots.delete' = ANY(permissions);

UPDATE roles
SET permissions = ARRAY[
  'dashboard.read','hosts.read','host.interfaces.read','host.interfaces.manage','agents.read','agents.manage',
  'vms.read','vms.create','vms.update','vms.power','vms.delete','vms.force','vms.console','vms.clone','vms.migrate',
  'snapshots.read','snapshots.create','snapshots.update','snapshots.revert','snapshots.delete',
  'storage.read','storage.manage','network.read','network.manage',
  'operations.read','alerts.manage',
  'settings.base.read','settings.base.manage','settings.users.read','settings.users.manage','settings.auth.read','settings.auth.manage','settings.notifications.read','settings.notifications.manage'
],
updated_at = now()
WHERE key = 'admin' AND builtin = TRUE;

UPDATE roles
SET permissions = ARRAY[
  'dashboard.read','hosts.read','host.interfaces.read','host.interfaces.manage',
  'vms.read','vms.create','vms.update','vms.power','vms.console','vms.clone','vms.migrate',
  'snapshots.read','snapshots.create','snapshots.update','snapshots.revert',
  'storage.read','storage.manage','network.read','network.manage','operations.read'
],
updated_at = now()
WHERE key = 'operator' AND builtin = TRUE;

UPDATE roles
SET permissions = ARRAY[
  'dashboard.read','hosts.read','host.interfaces.read','agents.read','vms.read','snapshots.read','storage.read','network.read','operations.read'
],
updated_at = now()
WHERE key = 'viewer' AND builtin = TRUE;
