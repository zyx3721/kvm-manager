import type { ApiUser, UserGroup, UserPermission, UserRole } from '../../../lib/api';

export type UserForm = {
  id: string;
  username: string;
  email: string;
  displayName: string;
  password: string;
  roleKeys: string[];
  disabled: boolean;
  createdAt: string;
};

export type RoleForm = {
  id: string;
  key: string;
  name: string;
  description: string;
  permissions: string[];
  builtin: boolean;
};

export type GroupForm = {
  id: string;
  name: string;
  description: string;
  disabled: boolean;
  memberIds: string[];
  roleKeys: string[];
};

export const emptyUserForm: UserForm = {
  id: '',
  username: '',
  email: '',
  displayName: '',
  password: '',
  roleKeys: ['viewer'],
  disabled: false,
  createdAt: '',
};
export const emptyRoleForm: RoleForm = {
  id: '',
  key: '',
  name: '',
  description: '',
  permissions: [],
  builtin: false,
};
export const emptyGroupForm: GroupForm = {
  id: '',
  name: '',
  description: '',
  disabled: false,
  memberIds: [],
  roleKeys: [],
};

export const emailPattern = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;

export function userToForm(user: ApiUser): UserForm {
  return {
    id: user.id,
    username: user.username,
    email: user.email ?? '',
    displayName: user.displayName,
    password: '',
    roleKeys: [user.role || 'viewer'],
    disabled: Boolean(user.disabled),
    createdAt: user.created_at ?? '',
  };
}

export function formToUser(form: UserForm): ApiUser {
  return {
    id: form.id,
    username: form.username,
    email: form.email,
    displayName: form.displayName,
    role: form.roleKeys[0] ?? 'viewer',
    disabled: form.disabled,
    created_at: form.createdAt,
  };
}

export function roleToForm(role: UserRole): RoleForm {
  return {
    id: role.id,
    key: role.key,
    name: role.name,
    description: role.description,
    permissions: role.permissions,
    builtin: role.builtin,
  };
}

export function groupToForm(group: UserGroup): GroupForm {
  return {
    id: group.id,
    name: group.name,
    description: group.description,
    disabled: group.disabled,
    memberIds: group.members?.map(user => user.id) ?? [],
    roleKeys: group.roles?.map(role => role.key) ?? [],
  };
}

export function roleNames(roles: UserRole[] | undefined) {
  if (!roles?.length) return '未分配角色';
  return roles.map(role => role.name || role.key).join('、');
}

export function userSearchText(user: ApiUser) {
  return `${user.username} ${user.email} ${user.displayName} ${roleNames(user.roles)} ${formatDateTime(user.created_at)} ${formatLastLogin(user.lastLoginAt)}`.toLowerCase();
}

export function groupSearchText(group: UserGroup) {
  return `${group.name} ${group.description} ${roleNames(group.roles)} ${(group.members ?? []).map(user => `${user.username} ${user.displayName}`).join(' ')}`.toLowerCase();
}

export function roleSearchText(role: UserRole) {
  return `${role.name} ${role.key} ${role.description} ${(role.permissions ?? []).join(' ')}`.toLowerCase();
}

export function groupPermissions(items: UserPermission[]) {
  return items.reduce<Record<string, UserPermission[]>>((groups, item) => {
    const category = item.category || '其他';
    groups[category] = [...(groups[category] ?? []), item];
    return groups;
  }, {});
}

export function filterPermissionGroups(groups: Record<string, UserPermission[]>, query: string) {
  const keyword = query.trim().toLowerCase();
  if (!keyword) return groups;
  return Object.entries(groups).reduce<Record<string, UserPermission[]>>(
    (result, [category, items]) => {
      const filtered = items.filter(item =>
        [item.key, item.name, item.description, item.category].some(value =>
          value.toLowerCase().includes(keyword)
        )
      );
      if (filtered.length) result[category] = filtered;
      return result;
    },
    {}
  );
}

export function formatLastLogin(value: string | undefined) {
  return formatDateTime(value, '从未登录');
}

export function formatDateTime(value: string | undefined, fallback = '-') {
  if (!value) return fallback;
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return fallback;
  return date.toLocaleString('zh-CN', { hour12: false });
}

export function isDefaultAdminUser(user: Pick<ApiUser, 'username'> | UserForm) {
  return user.username === 'admin';
}

export function upsertById<T extends { id: string }>(items: T[], item: T) {
  const exists = items.some(current => current.id === item.id);
  return exists
    ? items.map(current => (current.id === item.id ? item : current))
    : [...items, item];
}
