import type { UserPermission } from '../../../lib/api';

export const fallbackImpliedReadPermissions: Record<string, string> = {
  'agents.manage': 'agents.read',
  'host.interfaces.manage': 'host.interfaces.read',
  'vms.create': 'vms.read',
  'vms.update': 'vms.read',
  'vms.power': 'vms.read',
  'vms.delete': 'vms.read',
  'vms.force': 'vms.read',
  'vms.console': 'vms.read',
  'vms.clone': 'vms.read',
  'vms.migrate': 'vms.read',
  'snapshots.create': 'snapshots.read',
  'snapshots.update': 'snapshots.read',
  'snapshots.revert': 'snapshots.read',
  'snapshots.delete': 'snapshots.read',
  'storage.manage': 'storage.read',
  'network.manage': 'network.read',
  'alerts.manage': 'operations.read',
  'settings.base.manage': 'settings.base.read',
  'settings.users.manage': 'settings.users.read',
  'settings.auth.manage': 'settings.auth.read',
  'settings.notifications.manage': 'settings.notifications.read',
};

export function impliedReadPermissionsFrom(items: UserPermission[]) {
  return items.reduce<Record<string, string>>(
    (rules, item) => {
      if (item.impliedReadPermission) {
        rules[item.key] = item.impliedReadPermission;
      }
      return rules;
    },
    { ...fallbackImpliedReadPermissions }
  );
}

export function normalizeRolePermissions(
  permissions: string[],
  impliedReadPermissions: Record<string, string> = fallbackImpliedReadPermissions
) {
  const next = new Set(permissions);
  permissions.forEach(permission => {
    const readPermission = impliedReadPermissions[permission];
    if (readPermission) next.add(readPermission);
  });
  return Array.from(next).sort();
}
