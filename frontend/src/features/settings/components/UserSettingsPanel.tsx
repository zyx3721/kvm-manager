import React, { useCallback, useEffect, useMemo, useState } from "react";
import { createPortal } from "react-dom";
import { toast } from "sonner";
import {
  BanIcon,
  CheckIcon,
  CircleCheckIcon,
  EyeIcon,
  EyeOffIcon,
  PlusIcon,
  SaveIcon,
  SearchIcon,
  ShieldCheckIcon,
  Trash2Icon,
  ToggleLeftIcon,
  ToggleRightIcon,
  UserCogIcon,
  UsersRoundIcon,
  XIcon,
} from "lucide-react";
import {
  createManagedUser,
  createUserGroup,
  createUserRole,
  deleteManagedUser,
  deleteUserGroup,
  deleteUserRole,
  fetchManagedUsers,
  fetchUserGroups,
  fetchUserPermissions,
  fetchUserRoles,
  updateManagedUser,
  updateManagedUserDisabled,
  updateUserGroup,
  updateUserRole,
  type ApiUser,
  type UserGroup,
  type UserPermission,
  type UserRole,
} from "../../../lib/api";
import { KvmTooltip } from "../../../components/kvm/StatusBadge";
import {
  emailPattern,
  emptyGroupForm,
  emptyRoleForm,
  emptyUserForm,
  filterPermissionGroups,
  formToUser,
  formatDateTime,
  formatLastLogin,
  groupPermissions,
  groupSearchText,
  groupToForm,
  isDefaultAdminUser,
  roleNames,
  roleSearchText,
  roleToForm,
  type GroupForm,
  type RoleForm,
  type UserForm,
  upsertById,
  userSearchText,
  userToForm,
} from "./userSettingsUtils";
import { impliedReadPermissionsFrom, normalizeRolePermissions } from "./rolePermissionRules";

type UserConfigTab = "users" | "groups" | "roles";

const userConfigCards: Array<{ id: UserConfigTab; title: string; description: string; icon: React.ElementType; color: string }> = [
  { id: "users", title: "用户", description: "维护本地账号和允许 AD/LDAP 登录的用户", icon: UserCogIcon, color: "#38bdf8" },
  { id: "groups", title: "用户群组", description: "按团队聚合用户，并统一分配角色", icon: UsersRoundIcon, color: "#22c55e" },
  { id: "roles", title: "用户角色", description: "维护内置角色和自定义权限集合", icon: ShieldCheckIcon, color: "#f59e0b" },
];

export function UserSettingsPanel({ canManage = true }: { canManage?: boolean }) {
  const [active, setActive] = useState<UserConfigTab>("users");
  const [users, setUsers] = useState<ApiUser[]>([]);
  const [roles, setRoles] = useState<UserRole[]>([]);
  const [groups, setGroups] = useState<UserGroup[]>([]);
  const [permissions, setPermissions] = useState<UserPermission[]>([]);
  const [userForm, setUserForm] = useState<UserForm>(emptyUserForm);
  const [roleForm, setRoleForm] = useState<RoleForm>(emptyRoleForm);
  const [groupForm, setGroupForm] = useState<GroupForm>(emptyGroupForm);
  const [dialog, setDialog] = useState<UserConfigTab | null>(null);
  const [busy, setBusy] = useState("");
  const [error, setError] = useState("");

  const load = useCallback(async () => {
    setError("");
    try {
      const [userResponse, roleResponse, groupResponse, permissionResponse] = await Promise.all([
        fetchManagedUsers(),
        fetchUserRoles(),
        fetchUserGroups(),
        fetchUserPermissions(),
      ]);
      setUsers(userResponse.items);
      setRoles(roleResponse.items);
      setGroups(groupResponse.items);
      setPermissions(permissionResponse.items);
    } catch (err) {
      const message = err instanceof Error ? err.message : "读取用户配置失败";
      toast.error(message);
      setError(isPermissionMessage(message) ? "" : message);
    }
  }, []);

  useEffect(() => { void load(); }, [load]);

  const selectedCard = userConfigCards.find((card) => card.id === active) ?? userConfigCards[0];
  const ActiveIcon = selectedCard.icon;
  const permissionGroups = useMemo(() => groupPermissions(permissions), [permissions]);
  const impliedReadPermissions = useMemo(() => impliedReadPermissionsFrom(permissions), [permissions]);

  const saveUser = async () => {
    const username = userForm.username.trim();
    if (!username) {
      toast.error("用户名不能为空");
      return;
    }
    const email = userForm.email.trim();
    if (!emailPattern.test(email)) {
      toast.error("请输入有效的邮箱地址");
      return;
    }
    if (!userForm.id && userForm.password.length < 6) {
      toast.error("密码至少 6 个字符");
      return;
    }
    setBusy("user");
    try {
      const payload = {
        username,
        password: userForm.password,
        email,
        displayName: userForm.displayName.trim() || username,
        roleKeys: userForm.roleKeys.length ? userForm.roleKeys : ["viewer"],
        disabled: userForm.disabled,
      };
      const saved = userForm.id ? await updateManagedUser(userForm.id, payload) : await createManagedUser(payload);
      setUsers((current) => upsertById(current, saved));
      setUserForm(userToForm(saved));
      setDialog(null);
      toast.success("用户配置已保存");
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "保存用户配置失败");
    } finally {
      setBusy("");
    }
  };

  const toggleUserDisabled = async (user: ApiUser) => {
    if (isDefaultAdminUser(user)) {
      toast.error("默认管理员不能禁用");
      return;
    }
    setBusy(`user-${user.id}`);
    try {
      const saved = await updateManagedUserDisabled(user.id, !user.disabled);
      setUsers((current) => upsertById(current, saved));
      if (userForm.id === saved.id) setUserForm(userToForm(saved));
      toast.success(saved.disabled ? "用户已禁用" : "用户已启用");
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "更新用户状态失败");
    } finally {
      setBusy("");
    }
  };

  const removeUser = async () => {
    if (!userForm.id) return;
    if (isDefaultAdminUser(userForm)) {
      toast.error("默认管理员不能删除");
      return;
    }
    setBusy("user");
    try {
      await deleteManagedUser(userForm.id);
      setUsers((current) => current.filter((user) => user.id !== userForm.id));
      setUserForm(emptyUserForm);
      setDialog(null);
      toast.success("用户已删除");
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "删除用户失败");
    } finally {
      setBusy("");
    }
  };

  const toggleGroupDisabled = async () => {
    if (!groupForm.id) return;
    setBusy("group");
    try {
      const saved = await updateUserGroup(groupForm.id, {
        name: groupForm.name.trim(),
        description: groupForm.description.trim(),
        disabled: !groupForm.disabled,
        memberIds: groupForm.memberIds,
        roleKeys: groupForm.roleKeys,
      });
      setGroups((current) => upsertById(current, saved));
      setGroupForm(groupToForm(saved));
      toast.success(saved.disabled ? "用户群组已禁用" : "用户群组已启用");
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "更新用户群组状态失败");
    } finally {
      setBusy("");
    }
  };

  const saveRole = async () => {
    if (roleForm.builtin) {
      toast.error("内置角色不可修改");
      return;
    }
    if (!roleForm.key.trim() || !roleForm.name.trim()) {
      toast.error("角色标识和名称不能为空");
      return;
    }
    setBusy("role");
    try {
      const payload = {
        key: roleForm.key.trim(),
        name: roleForm.name.trim(),
        description: roleForm.description.trim(),
        permissions: normalizeRolePermissions(roleForm.permissions, impliedReadPermissions),
      };
      const saved = roleForm.id ? await updateUserRole(roleForm.id, payload) : await createUserRole(payload);
      setRoles((current) => upsertById(current, saved));
      setRoleForm(roleToForm(saved));
      setDialog(null);
      toast.success("用户角色已保存");
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "保存用户角色失败");
    } finally {
      setBusy("");
    }
  };

  const removeRole = async () => {
    if (!roleForm.id || roleForm.builtin) return;
    setBusy("role");
    try {
      await deleteUserRole(roleForm.id);
      setRoles((current) => current.filter((role) => role.id !== roleForm.id));
      setRoleForm(emptyRoleForm);
      setDialog(null);
      toast.success("用户角色已删除");
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "删除用户角色失败");
    } finally {
      setBusy("");
    }
  };

  const saveGroup = async () => {
    if (!groupForm.name.trim()) {
      toast.error("用户群组名称不能为空");
      return;
    }
    setBusy("group");
    try {
      const payload = {
        name: groupForm.name.trim(),
        description: groupForm.description.trim(),
        disabled: groupForm.disabled,
        memberIds: groupForm.memberIds,
        roleKeys: groupForm.roleKeys,
      };
      const saved = groupForm.id ? await updateUserGroup(groupForm.id, payload) : await createUserGroup(payload);
      setGroups((current) => upsertById(current, saved));
      setGroupForm(groupToForm(saved));
      setDialog(null);
      toast.success("用户群组已保存");
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "保存用户群组失败");
    } finally {
      setBusy("");
    }
  };

  const removeGroup = async () => {
    if (!groupForm.id) return;
    setBusy("group");
    try {
      await deleteUserGroup(groupForm.id);
      setGroups((current) => current.filter((group) => group.id !== groupForm.id));
      setGroupForm(emptyGroupForm);
      setDialog(null);
      toast.success("用户群组已删除");
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "删除用户群组失败");
    } finally {
      setBusy("");
    }
  };

  return (
    <>
      <div className="grid min-h-0 flex-1 grid-cols-1 items-stretch gap-4 overflow-hidden xl:grid-cols-[430px_minmax(0,1fr)]">
        <nav className="kvm-hidden-scrollbar max-h-64 min-h-0 space-y-2 overflow-y-auto rounded-lg p-2 xl:max-h-none" style={{ background: "rgba(255,255,255,0.035)", border: "1px solid var(--kvm-border)" }} aria-label="用户配置">
          {userConfigCards.map((card) => (
            <UserConfigCard key={card.id} card={card} active={active === card.id} count={card.id === "users" ? users.length : card.id === "groups" ? groups.length : roles.length} onClick={() => setActive(card.id)} />
          ))}
        </nav>
        <aside className="flex min-h-0 flex-col overflow-hidden rounded-lg p-4" style={{ background: "rgba(255,255,255,0.035)", border: "1px solid var(--kvm-border)" }}>
          <div className="mb-4 flex items-center justify-between gap-3">
            <div className="flex min-w-0 items-center gap-3">
              <div className="flex h-10 w-10 items-center justify-center rounded-lg" style={{ color: selectedCard.color, background: "rgba(255,255,255,0.05)", border: "1px solid rgba(255,255,255,0.08)" }}><ActiveIcon size={19} /></div>
              <div className="min-w-0">
                <div className="truncate text-sm font-semibold" style={{ color: "var(--kvm-text)" }}>{selectedCard.title}</div>
                <div className="mt-0.5 truncate text-xs" style={{ color: "var(--kvm-text-muted)" }}>{selectedCard.description}</div>
              </div>
            </div>
          </div>
          {error && <div className="mb-4 rounded-lg p-3 text-sm" style={{ background: "rgba(245,158,11,0.1)", border: "1px solid rgba(245,158,11,0.25)", color: "#f59e0b" }}>{error}</div>}
          <div className="min-h-0 flex-1 overflow-hidden">
            {active === "users" && <UsersEditor canManage={canManage} users={users} roles={roles} form={userForm} onCreate={() => { setUserForm(emptyUserForm); setDialog("users"); }} onSelect={(form) => { setUserForm(form); setDialog("users"); }} />}
            {active === "groups" && <GroupsEditor canManage={canManage} groups={groups} form={groupForm} onCreate={() => { setGroupForm(emptyGroupForm); setDialog("groups"); }} onSelect={(form) => { setGroupForm(form); setDialog("groups"); }} />}
            {active === "roles" && <RolesEditor canManage={canManage} roles={roles} form={roleForm} onCreate={() => { setRoleForm(emptyRoleForm); setDialog("roles"); }} onSelect={(form) => { setRoleForm(form); setDialog("roles"); }} />}
          </div>
        </aside>
      </div>
      {dialog === "users" && <UserEditorDialog canManage={canManage} roles={roles} form={userForm} busy={busy} onClose={() => setDialog(null)} onFormChange={setUserForm} onSave={saveUser} onDelete={removeUser} onToggleDisabled={toggleUserDisabled} />}
      {dialog === "groups" && <GroupEditorDialog canManage={canManage} users={users} roles={roles} form={groupForm} busy={busy} onClose={() => setDialog(null)} onFormChange={setGroupForm} onSave={saveGroup} onDelete={removeGroup} onToggleDisabled={toggleGroupDisabled} />}
      {dialog === "roles" && <RoleEditorDialog canManage={canManage} permissionGroups={permissionGroups} form={roleForm} busy={busy} onClose={() => setDialog(null)} onFormChange={setRoleForm} onSave={saveRole} onDelete={removeRole} />}
    </>
  );
}

function UserConfigCard({ card, active, count, onClick }: { card: (typeof userConfigCards)[number]; active: boolean; count: number; onClick: () => void }) {
  const Icon = card.icon;
  return (
    <button type="button" onClick={onClick} className="kvm-action-button flex w-full items-start gap-3 rounded-lg p-3 text-left" style={{ background: active ? "rgba(59,130,246,0.12)" : "transparent", border: active ? "1px solid rgba(96,165,250,0.56)" : "1px solid transparent", color: "var(--kvm-text)" }}>
      <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg" style={{ color: card.color, background: "rgba(255,255,255,0.05)", border: "1px solid rgba(255,255,255,0.08)" }}><Icon size={19} /></div>
      <div className="min-w-0 flex-1">
        <div className="flex items-center justify-between gap-3"><span className="truncate text-sm font-semibold">{card.title}</span><span className="text-xs" style={{ color: "var(--kvm-text-muted)" }}>{count}</span></div>
        <p className="mt-1 line-clamp-2 text-xs leading-5" style={{ color: "var(--kvm-text-muted)" }}>{card.description}</p>
      </div>
    </button>
  );
}

function UsersEditor({ users, roles, form, canManage, onCreate, onSelect }: { users: ApiUser[]; roles: UserRole[]; form: UserForm; canManage: boolean; onCreate: () => void; onSelect: (form: UserForm) => void }) {
  const [query, setQuery] = useState("");
  const filtered = useMemo(() => users.filter((user) => userSearchText(user).includes(query.trim().toLowerCase())), [query, users]);
  return (
    <ManagementShell title="用户" count={users.length} query={query} queryPlaceholder="搜索用户名、显示名或角色" onQueryChange={setQuery} onCreate={onCreate} createLabel="新增用户" canCreate={canManage}>
      <ListPane emptyText="暂无用户" filteredEmptyText="没有匹配的用户" total={users.length} filtered={filtered.length}>
        {filtered.map((user) => <CompactRow key={user.id} active={form.id === user.id} title={user.displayName || user.username} meta={`${user.username} · ${directRoleName(user.role, roles)}`} extra={`创建时间：${formatDateTime(user.created_at)} · 最近登录：${formatLastLogin(user.lastLoginAt)}`} badge={user.disabled ? "禁用" : "启用"} enabled={!user.disabled} onClick={() => onSelect(userToForm(user))} />)}
      </ListPane>
    </ManagementShell>
  );
}

function GroupsEditor({ groups, form, canManage, onCreate, onSelect }: { groups: UserGroup[]; form: GroupForm; canManage: boolean; onCreate: () => void; onSelect: (form: GroupForm) => void }) {
  const [query, setQuery] = useState("");
  const filtered = useMemo(() => groups.filter((group) => groupSearchText(group).includes(query.trim().toLowerCase())), [groups, query]);
  return (
    <ManagementShell title="用户群组" count={groups.length} query={query} queryPlaceholder="搜索群组、描述、成员或角色" onQueryChange={setQuery} onCreate={onCreate} createLabel="新增群组" canCreate={canManage}>
      <ListPane emptyText="暂无用户群组" filteredEmptyText="没有匹配的群组" total={groups.length} filtered={filtered.length}>
        {filtered.map((group) => <CompactRow key={group.id} active={form.id === group.id} title={group.name} meta={`${group.members?.length ?? 0} 个用户 · ${roleNames(group.roles)}`} extra={group.description || "未填写描述"} badge={`${group.roles?.length ?? 0} 角色`} enabled={!group.disabled} onClick={() => onSelect(groupToForm(group))} />)}
      </ListPane>
    </ManagementShell>
  );
}

function RolesEditor({ roles, form, canManage, onCreate, onSelect }: { roles: UserRole[]; form: RoleForm; canManage: boolean; onCreate: () => void; onSelect: (form: RoleForm) => void }) {
  const [query, setQuery] = useState("");
  const filteredRoles = useMemo(() => roles.filter((role) => roleSearchText(role).includes(query.trim().toLowerCase())), [query, roles]);
  return (
    <ManagementShell title="用户角色" count={roles.length} query={query} queryPlaceholder="搜索角色名称、标识或描述" onQueryChange={setQuery} onCreate={onCreate} createLabel="新增角色" canCreate={canManage}>
      <ListPane emptyText="暂无用户角色" filteredEmptyText="没有匹配的角色" total={roles.length} filtered={filteredRoles.length}>
        {filteredRoles.map((role) => <CompactRow key={role.id} active={form.id === role.id} title={role.name} meta={`${role.key} · ${role.permissions.length} 项权限`} extra={role.description || "未填写描述"} badge={role.builtin ? "内置" : "自定义"} onClick={() => onSelect(roleToForm(role))} />)}
      </ListPane>
    </ManagementShell>
  );
}

function ManagementShell({ title, count, query, queryPlaceholder, createLabel, canCreate, onQueryChange, onCreate, children }: { title: string; count: number; query: string; queryPlaceholder: string; createLabel: string; canCreate: boolean; onQueryChange: (value: string) => void; onCreate: () => void; children: React.ReactNode }) {
  return <div className="flex h-full min-h-0 flex-col overflow-hidden rounded-lg border" style={{ borderColor: "var(--kvm-border)", background: "rgba(255,255,255,0.018)" }}><div className="flex flex-wrap items-center justify-between gap-3 border-b p-3" style={{ borderColor: "var(--kvm-border)" }}><div className="flex items-baseline gap-2"><span className="text-sm font-semibold" style={{ color: "var(--kvm-text)" }}>{title}</span><span className="text-xs" style={{ color: "var(--kvm-text-muted)" }}>{count} 项</span></div><div className="flex min-w-0 flex-1 items-center justify-end gap-2"><SearchBox value={query} placeholder={queryPlaceholder} onChange={onQueryChange} />{canCreate && <button type="button" onClick={onCreate} className="kvm-action-button flex h-9 shrink-0 items-center gap-2 rounded-lg border px-3 text-sm" style={{ borderColor: "rgba(59,130,246,0.38)", color: "var(--kvm-accent-text)", background: "rgba(59,130,246,0.1)" }}><PlusIcon size={14} />{createLabel}</button>}</div></div>{children}</div>;
}

function SearchBox({ value, placeholder, onChange }: { value: string; placeholder: string; onChange: (value: string) => void }) {
  return <div className="relative min-w-[220px] max-w-sm flex-1"><SearchIcon size={14} className="absolute left-3 top-1/2 -translate-y-1/2" style={{ color: "var(--kvm-text-muted)" }} /><input value={value} onChange={(event) => onChange(event.target.value)} placeholder={placeholder} className="h-9 w-full rounded-lg pl-9 pr-3 text-sm outline-none" style={{ background: "var(--kvm-control-bg)", border: "1px solid var(--kvm-border)", color: "var(--kvm-text)" }} /></div>;
}

function ListPane({ children, emptyText, filteredEmptyText, total, filtered }: { children: React.ReactNode; emptyText: string; filteredEmptyText: string; total: number; filtered: number }) {
  return <div className="kvm-hidden-scrollbar min-h-0 flex-1 overflow-y-auto p-3">{total === 0 ? <EmptyState text={emptyText} /> : filtered === 0 ? <EmptyState text={filteredEmptyText} /> : <div className="space-y-2">{children}</div>}</div>;
}

function UserEditorDialog({ roles, form, busy, canManage, onClose, onFormChange, onSave, onDelete, onToggleDisabled }: { roles: UserRole[]; form: UserForm; busy: string; canManage: boolean; onClose: () => void; onFormChange: React.Dispatch<React.SetStateAction<UserForm>>; onSave: () => void; onDelete: () => void; onToggleDisabled: (user: ApiUser) => void }) {
  const defaultAdmin = isDefaultAdminUser(form);
  return (
    <DialogFrame title={form.id ? "编辑用户" : "新增用户"} subtitle={form.username || "配置平台账号和登录权限"} onClose={onClose} footer={canManage ? <>{form.id && <DangerButton busy={busy === "user"} disabled={!form.disabled || busy !== "" || defaultAdmin} onClick={onDelete} label="删除" />}<StateButton disabled={!form.id || busy !== "" || defaultAdmin} active={form.disabled} onClick={() => form.id && onToggleDisabled({ ...formToUser(form), disabled: form.disabled })} /><PrimaryButton busy={busy === "user"} onClick={onSave} label="保存" /></> : null}>
      <SectionTitle title="基础信息" />
      <Input label="用户名" required value={form.username} disabled={!canManage || defaultAdmin} onChange={(value) => onFormChange((current) => ({ ...current, username: value }))} placeholder="用户名" />
      <Input label="邮箱" required type="email" value={form.email} disabled={!canManage} onChange={(value) => onFormChange((current) => ({ ...current, email: value }))} placeholder="用于验证找回密码身份" />
      <Input label="显示名称" value={form.displayName} disabled={!canManage} onChange={(value) => onFormChange((current) => ({ ...current, displayName: value }))} placeholder="用户显示名称" />
      <Input label={form.id ? "新密码" : "密码"} required={!form.id} type="password" value={form.password} disabled={!canManage} onChange={(value) => onFormChange((current) => ({ ...current, password: value }))} placeholder={form.id ? "留空则不修改" : "至少 6 个字符"} />
      <SectionTitle title="授权与状态" />
      <CheckList compact single maxVisibleItems={3} hideActions title="角色" items={roles.map((role) => ({ key: role.key, label: role.name || role.key, helper: role.description }))} value={form.roleKeys} disabled={!canManage} onChange={(value) => onFormChange((current) => ({ ...current, roleKeys: value }))} />
    </DialogFrame>
  );
}

function GroupEditorDialog({ users, roles, form, busy, canManage, onClose, onFormChange, onSave, onDelete, onToggleDisabled }: { users: ApiUser[]; roles: UserRole[]; form: GroupForm; busy: string; canManage: boolean; onClose: () => void; onFormChange: React.Dispatch<React.SetStateAction<GroupForm>>; onSave: () => void; onDelete: () => void; onToggleDisabled: () => void }) {
  return (
    <DialogFrame title={form.id ? "编辑群组" : "新增群组"} subtitle={form.name || "批量组织成员和角色"} onClose={onClose} footer={canManage ? <>{form.id && <DangerButton busy={busy === "group"} onClick={onDelete} label="删除" />}<StateButton disabled={!form.id || busy !== ""} active={form.disabled} onClick={onToggleDisabled} /><PrimaryButton busy={busy === "group"} onClick={onSave} label="保存" /></> : null}>
      <SectionTitle title="基础信息" />
      <Input label="群组名称" required value={form.name} disabled={!canManage} onChange={(value) => onFormChange((current) => ({ ...current, name: value }))} placeholder="运维组" />
      <TextArea label="描述" value={form.description} disabled={!canManage} onChange={(value) => onFormChange((current) => ({ ...current, description: value }))} placeholder="群组用途" />
      <SectionTitle title="成员与角色" />
      <CheckList compact maxVisibleItems={3} title="群组成员" items={users.map((user) => ({ key: user.id, label: user.displayName || user.username, helper: user.username }))} value={form.memberIds} disabled={!canManage} onChange={(value) => onFormChange((current) => ({ ...current, memberIds: value }))} />
      <CheckList compact single maxVisibleItems={3} hideActions title="群组角色" items={roles.map((role) => ({ key: role.key, label: role.name || role.key, helper: role.description }))} value={form.roleKeys} disabled={!canManage} onChange={(value) => onFormChange((current) => ({ ...current, roleKeys: value }))} />
    </DialogFrame>
  );
}

function RoleEditorDialog({ permissionGroups, form, busy, canManage, onClose, onFormChange, onSave, onDelete }: { permissionGroups: Record<string, UserPermission[]>; form: RoleForm; busy: string; canManage: boolean; onClose: () => void; onFormChange: React.Dispatch<React.SetStateAction<RoleForm>>; onSave: () => void; onDelete: () => void }) {
  const [permissionQuery, setPermissionQuery] = useState("");
  const filteredGroups = useMemo(() => filterPermissionGroups(permissionGroups, permissionQuery), [permissionGroups, permissionQuery]);
  const readonly = form.builtin || !canManage;
  return (
    <DialogFrame title={form.id ? "编辑角色" : "新增角色"} subtitle={form.builtin ? "内置角色只读" : form.key || "配置角色可用权限"} onClose={onClose} footer={canManage ? <>{form.id && !form.builtin && <DangerButton busy={busy === "role"} onClick={onDelete} label="删除" />}<PrimaryButton busy={busy === "role"} disabled={form.builtin} onClick={onSave} label="保存角色" /></> : null}>
      <SectionTitle title="角色信息" />
      <Input label="角色标识" required value={form.key} disabled={readonly} onChange={(value) => onFormChange((current) => ({ ...current, key: value }))} placeholder="custom-ops" />
      <Input label="角色名称" required value={form.name} disabled={readonly} onChange={(value) => onFormChange((current) => ({ ...current, name: value }))} placeholder="自定义运维" />
      <TextArea label="描述" value={form.description} disabled={readonly} onChange={(value) => onFormChange((current) => ({ ...current, description: value }))} placeholder="说明该角色可执行的操作" />
      <SectionTitle title="权限集合" />
      <Input label="搜索权限" value={permissionQuery} onChange={setPermissionQuery} placeholder="按名称、标识或描述搜索" disabled={form.builtin} />
      {Object.entries(filteredGroups).map(([category, items]) => <CheckList compact key={category} title={category} items={items.map((permission) => ({ key: permission.key, label: permission.name, helper: `${permission.key} · ${permission.description}` }))} value={form.permissions} disabled={readonly} onChange={(value) => onFormChange((current) => ({ ...current, permissions: value }))} />)}
      {Object.keys(filteredGroups).length === 0 && <div className="rounded-lg border p-3 text-sm" style={{ borderColor: "var(--kvm-border)", color: "var(--kvm-text-muted)", background: "rgba(255,255,255,0.03)" }}>没有匹配的权限</div>}
    </DialogFrame>
  );
}

function DialogFrame({ title, subtitle, footer, onClose, children }: { title: string; subtitle: string; footer: React.ReactNode; onClose: () => void; children: React.ReactNode }) {
  const node = <div className="kvm-dialog-backdrop fixed inset-0 z-[1500] flex items-center justify-center px-3 py-5" role="presentation"><div className="kvm-dialog-panel flex max-h-[88vh] w-[min(92vw,720px)] flex-col overflow-hidden rounded-2xl shadow-2xl" role="dialog" aria-modal="true" aria-label={title}><div className="flex items-start justify-between gap-4 border-b p-5" style={{ borderColor: "var(--kvm-border)" }}><div className="min-w-0"><h3 className="truncate text-base font-semibold" style={{ color: "var(--kvm-text)" }}>{title}</h3><p className="mt-1 truncate text-xs" style={{ color: "var(--kvm-text-muted)" }}>{subtitle}</p></div><button type="button" onClick={onClose} className="kvm-action-button flex h-8 w-8 shrink-0 items-center justify-center rounded-lg border" style={{ borderColor: "var(--kvm-border)", color: "var(--kvm-text-muted)", background: "rgba(255,255,255,0.04)" }} aria-label="关闭"><XIcon size={15} /></button></div><div className="kvm-hidden-scrollbar min-h-0 flex-1 space-y-3 overflow-y-auto p-5">{children}</div>{footer && <div className="flex justify-end gap-2 border-t p-4" style={{ borderColor: "var(--kvm-border)", background: "rgba(255,255,255,0.018)" }}>{footer}</div>}</div></div>;
  return typeof document === "undefined" ? node : createPortal(node, document.body);
}

function CompactRow({ title, meta, extra, badge, enabled, active, onClick }: { title: string; meta: string; extra: string; badge: string; enabled?: boolean; active: boolean; onClick: () => void }) {
  return <button type="button" onClick={onClick} className="kvm-action-button group grid min-h-[74px] w-full grid-cols-[3px_minmax(0,1fr)_auto] items-center gap-3 rounded-lg border p-0 pr-3 text-left" style={{ background: active ? "rgba(59,130,246,0.11)" : "rgba(255,255,255,0.026)", borderColor: active ? "rgba(96,165,250,0.52)" : "var(--kvm-border)", color: "var(--kvm-text)" }}><span className="h-full rounded-l-lg" style={{ background: active ? "#60a5fa" : "transparent" }} /><span className="min-w-0 py-2"><span className="flex min-w-0 items-center gap-2"><span className="truncate text-sm font-semibold">{title}</span><span className="shrink-0 rounded-md border px-1.5 py-0.5 text-[11px]" style={{ color: "var(--kvm-text-muted)", borderColor: "var(--kvm-border)", background: "rgba(255,255,255,0.028)" }}>{badge}</span></span><span className="mt-1 block truncate text-xs" style={{ color: "var(--kvm-text-muted)" }}>{meta}</span><span className="mt-0.5 block truncate text-xs" style={{ color: "var(--kvm-text-muted)" }}>{extra}</span></span>{typeof enabled === "boolean" ? enabled ? <ToggleRightIcon size={18} style={{ color: "#86efac" }} /> : <ToggleLeftIcon size={18} style={{ color: "var(--kvm-text-muted)" }} /> : <span className="h-2 w-2 rounded-full" style={{ background: active ? "#60a5fa" : "var(--kvm-border)" }} />}</button>;
}

function EmptyState({ text }: { text: string }) {
  return <div className="flex min-h-48 items-center justify-center rounded-lg border text-sm" style={{ borderColor: "var(--kvm-border)", color: "var(--kvm-text-muted)", background: "rgba(255,255,255,0.02)" }}>{text}</div>;
}

function SectionTitle({ title }: { title: string }) {
  return <div className="pt-1 text-sm font-semibold" style={{ color: "var(--kvm-text)" }}>{title}</div>;
}

function Input({ label, value, onChange, placeholder, type = "text", required = false, disabled = false }: { label: string; value: string; onChange: (value: string) => void; placeholder: string; type?: string; required?: boolean; disabled?: boolean }) {
  const [passwordVisible, setPasswordVisible] = useState(false);
  const isPassword = type === "password";
  const inputType = isPassword && passwordVisible ? "text" : type;
  return <label className="grid grid-cols-1 gap-1.5 text-xs sm:grid-cols-[64px_minmax(0,1fr)] sm:items-center sm:gap-3" style={{ color: "var(--kvm-text-muted)" }}><span className="sm:text-right">{label}{required && <span style={{ color: "#f87171" }}>*</span>}</span><span className="relative block min-w-0"><input type={inputType} value={value} disabled={disabled} onChange={(event) => onChange(event.target.value)} placeholder={placeholder} className={`w-full rounded-lg px-3 py-2 text-sm outline-none disabled:opacity-60 ${isPassword ? "pr-10" : ""}`} style={{ background: "var(--kvm-control-bg)", border: "1px solid var(--kvm-border)", color: "var(--kvm-text)" }} />{isPassword && <KvmTooltip label={passwordVisible ? "隐藏密码" : "显示密码"} placement="top" align="center"><button type="button" disabled={disabled} onClick={(event) => { event.preventDefault(); setPasswordVisible((current) => !current); }} className="kvm-action-button absolute right-2 top-1/2 flex h-7 w-7 -translate-y-1/2 items-center justify-center rounded-md disabled:cursor-not-allowed disabled:opacity-50" style={{ color: "var(--kvm-text-muted)", background: "rgba(255,255,255,0.035)" }} aria-label={passwordVisible ? "隐藏密码" : "显示密码"}>{passwordVisible ? <EyeOffIcon size={15} /> : <EyeIcon size={15} />}</button></KvmTooltip>}</span></label>;
}

function TextArea({ label, value, onChange, placeholder, disabled = false }: { label: string; value: string; onChange: (value: string) => void; placeholder: string; disabled?: boolean }) {
  return <label className="grid grid-cols-1 gap-1.5 text-xs sm:grid-cols-[64px_minmax(0,1fr)] sm:items-start sm:gap-3" style={{ color: "var(--kvm-text-muted)" }}><span className="sm:pt-2 sm:text-right">{label}</span><textarea value={value} disabled={disabled} onChange={(event) => onChange(event.target.value)} placeholder={placeholder} rows={3} className="w-full resize-y rounded-lg px-3 py-2 text-sm outline-none disabled:opacity-60" style={{ background: "var(--kvm-control-bg)", border: "1px solid var(--kvm-border)", color: "var(--kvm-text)" }} /></label>;
}

function CheckList({ title, items, value, onChange, disabled = false, compact = false, single = false, hideActions = false, maxVisibleItems }: { title: string; items: Array<{ key: string; label: string; helper?: string }>; value: string[]; onChange: (value: string[]) => void; disabled?: boolean; compact?: boolean; single?: boolean; hideActions?: boolean; maxVisibleItems?: number }) {
  const selected = new Set(value);
  const itemKeys = items.map((item) => item.key);
  const allSelected = itemKeys.length > 0 && itemKeys.every((key) => selected.has(key));
  const selectAll = () => onChange(Array.from(new Set([...value, ...itemKeys])));
  const clearAll = () => onChange(value.filter((key) => !itemKeys.includes(key)));
  const listMaxHeight = maxVisibleItems ? maxVisibleItems * 74 + Math.max(0, maxVisibleItems - 1) * 6 : undefined;
  return (
    <div className="space-y-2">
      <div className="flex items-center justify-between gap-2">
        <div className="text-sm font-semibold" style={{ color: "var(--kvm-text)" }}>{title}</div>
        {!hideActions && <div className="flex gap-1">
          <button type="button" disabled={disabled || itemKeys.length === 0 || allSelected} onClick={selectAll} className="kvm-action-button cursor-pointer rounded-md border px-2.5 py-1 text-[11px] transition disabled:cursor-not-allowed disabled:opacity-50" style={{ borderColor: "rgba(59,130,246,0.32)", color: "var(--kvm-accent-text)", background: "rgba(59,130,246,0.08)" }}>全选</button>
          <button type="button" disabled={disabled || itemKeys.length === 0 || !itemKeys.some((key) => selected.has(key))} onClick={clearAll} className="kvm-action-button cursor-pointer rounded-md border px-2.5 py-1 text-[11px] transition disabled:cursor-not-allowed disabled:opacity-50" style={{ borderColor: "var(--kvm-border)", color: "var(--kvm-text-muted)", background: "rgba(255,255,255,0.04)" }}>清空</button>
        </div>}
      </div>
      <div className={`${maxVisibleItems ? "" : compact ? "max-h-48" : "max-h-52"} kvm-hidden-scrollbar grid gap-1.5 overflow-y-auto pr-1`} style={{ maxHeight: listMaxHeight }}>
        {items.map((item) => {
          const checked = selected.has(item.key);
          return <label key={item.key} className={`flex items-start gap-2 rounded-lg border ${compact ? "p-2" : "p-2"} text-sm ${disabled ? "cursor-not-allowed opacity-70" : "cursor-pointer"}`} style={{ background: checked ? "rgba(59,130,246,0.1)" : "rgba(255,255,255,0.026)", borderColor: checked ? "rgba(96,165,250,0.45)" : "var(--kvm-border)", color: "var(--kvm-text)" }}><input type={single ? "radio" : "checkbox"} disabled={disabled} checked={checked} onChange={(event) => onChange(single ? event.target.checked ? [item.key] : value : event.target.checked ? [...value.filter((key) => key !== item.key), item.key] : value.filter((key) => key !== item.key))} className="mt-1" /><span className="min-w-0"><span className="block truncate">{item.label}</span>{item.helper && <span className="mt-0.5 block text-xs leading-4" style={{ color: "var(--kvm-text-muted)" }}>{item.helper}</span>}</span>{checked && <CheckIcon className="ml-auto shrink-0" size={15} style={{ color: "#93c5fd" }} />}</label>;
        })}
      </div>
    </div>
  );
}

function PrimaryButton({ label, busy, onClick, disabled = false }: { label: string; busy: boolean; onClick: () => void; disabled?: boolean }) {
  return <button type="button" onClick={() => void onClick()} disabled={busy || disabled} className="kvm-action-button flex items-center gap-2 rounded-lg border px-3 py-2 text-sm disabled:cursor-not-allowed disabled:opacity-50" style={{ borderColor: "rgba(59,130,246,0.38)", color: "var(--kvm-accent-text)", background: "rgba(59,130,246,0.1)" }}><SaveIcon size={14} />{busy ? "保存中" : label}</button>;
}

function StateButton({ active, disabled, onClick }: { active: boolean; disabled: boolean; onClick: () => void }) {
  const label = active ? "启用" : "禁用";
  const Icon = active ? CircleCheckIcon : BanIcon;
  return <button type="button" onClick={() => void onClick()} disabled={disabled} className="kvm-action-button flex items-center gap-2 rounded-lg border px-3 py-2 text-sm disabled:cursor-not-allowed disabled:opacity-50" style={{ borderColor: active ? "rgba(34,197,94,0.42)" : "rgba(245,158,11,0.36)", color: active ? "#86efac" : "#fbbf24", background: active ? "rgba(34,197,94,0.1)" : "rgba(245,158,11,0.09)" }}><Icon size={14} />{label}</button>;
}

function DangerButton({ label, busy, onClick, disabled = false }: { label: string; busy: boolean; onClick: () => void; disabled?: boolean }) {
  return <button type="button" onClick={() => void onClick()} disabled={busy || disabled} className="kvm-action-button flex items-center gap-2 rounded-lg border px-3 py-2 text-sm disabled:cursor-not-allowed disabled:opacity-50" style={{ borderColor: "rgba(239,68,68,0.34)", color: "#f87171", background: "rgba(239,68,68,0.08)" }}><Trash2Icon size={14} />{label}</button>;
}

function isPermissionMessage(message: string) {
  return message.includes("当前用户无权执行此操作");
}

function directRoleName(roleKey: string | undefined, roles: UserRole[]) {
  if (!roleKey) return "未分配角色";
  const role = roles.find((item) => item.key === roleKey);
  return role?.name || roleKey;
}
