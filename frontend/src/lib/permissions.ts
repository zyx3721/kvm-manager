import { getStoredUser, userHasPermission } from './auth';

export function can(permission: string) {
  return userHasPermission(getStoredUser(), permission);
}

export function vmActionPermission(action: string) {
  switch (action) {
    case 'delete':
      return 'vms.delete';
    case 'force-stop':
    case 'force-shutdown':
    case 'force-reboot':
    case 'force-delete':
      return 'vms.force';
    default:
      return 'vms.power';
  }
}
