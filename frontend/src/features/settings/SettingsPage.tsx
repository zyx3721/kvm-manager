import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { toast } from 'sonner';
import {
  BellRingIcon,
  CheckCircle2Icon,
  MegaphoneIcon,
  NetworkIcon,
  SlidersHorizontalIcon,
  SaveIcon,
  SettingsIcon,
  ToggleLeftIcon,
  ToggleRightIcon,
  Trash2Icon,
  UsersRoundIcon,
} from 'lucide-react';
import { BaseSettingsPanel } from './components/BaseSettingsPanel';
import { NotificationSettingsPanel } from './components/NotificationSettingsPanel';
import { UserSettingsPanel } from './components/UserSettingsPanel';
import {
  fetchAuthProviders,
  testAuthProvider,
  updateAuthProvider,
  type AuthProvider,
} from '../../lib/api';
import { can } from '../../lib/permissions';
import {
  ConfigField,
  EnableMediaToggle,
  SectionTitle,
  SettingsDetailHeader,
  SettingsDetailPanel,
  SettingsSplitLayout,
  displayValue,
  normalizeConfig,
  removeEmptyConfigValues,
  removeSecretPresenceMarkers,
  secretConfigured,
  type Field,
} from './components/SettingsFormPrimitives';

type SettingsTab = 'base' | 'users' | 'auth' | 'notifications';
type AuthProviderId = 'ldap';

const authProviderMeta: Record<
  AuthProviderId,
  {
    name: string;
    description: string;
    icon: React.ElementType;
    color: string;
    requiredFields: Field[];
    optionalFields: Field[];
  }
> = {
  ldap: {
    name: 'AD/LDAP',
    description: '通过企业目录服务实现统一身份认证，支持 AD/LDAP 登录。',
    icon: NetworkIcon,
    color: '#38bdf8',
    requiredFields: [
      { key: 'host', label: '服务器地址', placeholder: 'ldap.example.com', required: true },
      { key: 'port', label: '端口', placeholder: '389', required: true, inputMode: 'numeric' },
      { key: 'baseDN', label: 'Base DN', placeholder: 'dc=example,dc=com', required: true },
      {
        key: 'userFilter',
        label: '用户过滤器',
        placeholder: '(sAMAccountName={username})',
        required: true,
      },
      {
        key: 'bindDN',
        label: '绑定 DN',
        placeholder: 'cn=readonly,dc=example,dc=com',
        required: true,
      },
      {
        key: 'bindPassword',
        label: '绑定密码',
        placeholder: '请输入绑定账号密码',
        required: true,
        type: 'password',
      },
    ],
    optionalFields: [
      { key: 'useTLS', label: '启用 LDAPS', placeholder: '', type: 'checkbox' },
      { key: 'startTLS', label: '启用 STARTTLS', placeholder: '', type: 'checkbox' },
      { key: 'insecureSkipVerify', label: '跳过证书校验', placeholder: '', type: 'checkbox' },
      { key: 'timeoutSeconds', label: '超时时间', placeholder: '8', type: 'number' },
      { key: 'groupFilter', label: '用户组过滤器', placeholder: 'cn=ops,dc=example,dc=com' },
    ],
  },
};

const authProviderOrder: AuthProviderId[] = ['ldap'];

export default function SettingsPage() {
  const [tab, setTab] = useState<SettingsTab>('base');
  const [authProviders, setAuthProviders] = useState<Record<string, AuthProvider>>({});
  const [selectedAuth, setSelectedAuth] = useState<AuthProviderId>('ldap');
  const [authForm, setAuthForm] = useState<Record<string, unknown>>({});
  const [authName, setAuthName] = useState('AD/LDAP');
  const [authEnabled, setAuthEnabled] = useState(false);
  const [busy, setBusy] = useState('');
  const [error, setError] = useState('');
  const authMeta = authProviderMeta[selectedAuth];
  const AuthIcon = authMeta.icon;
  const canReadBase = can('settings.base.read');
  const canManageBase = can('settings.base.manage');
  const canReadUsers = can('settings.users.read');
  const canManageUsers = can('settings.users.manage');
  const canReadAuth = can('settings.auth.read');
  const canManageAuth = can('settings.auth.manage');
  const canReadNotifications = can('settings.notifications.read');
  const canManageNotifications = can('settings.notifications.manage');
  const visibleTabs = useMemo(
    () =>
      [
        {
          id: 'base' as const,
          icon: SlidersHorizontalIcon,
          label: '基础配置',
          visible: canReadBase || canManageBase,
        },
        {
          id: 'users' as const,
          icon: UsersRoundIcon,
          label: '用户配置',
          visible: canReadUsers || canManageUsers,
        },
        {
          id: 'auth' as const,
          icon: NetworkIcon,
          label: '认证配置',
          visible: canReadAuth || canManageAuth,
        },
        {
          id: 'notifications' as const,
          icon: BellRingIcon,
          label: '通知配置',
          visible: canReadNotifications || canManageNotifications,
        },
      ].filter(item => item.visible),
    [
      canManageAuth,
      canManageBase,
      canManageNotifications,
      canManageUsers,
      canReadAuth,
      canReadBase,
      canReadNotifications,
      canReadUsers,
    ]
  );

  const load = useCallback(async () => {
    setError('');
    try {
      const requests: Promise<void>[] = [];
      if (canReadAuth || canManageAuth) {
        requests.push(
          fetchAuthProviders().then(response => {
            setAuthProviders(Object.fromEntries(response.items.map(item => [item.id, item])));
          })
        );
      }
      await Promise.all(requests);
    } catch (err) {
      const message = err instanceof Error ? err.message : '读取系统配置失败';
      toast.error(message);
      setError(isPermissionMessage(message) ? '' : message);
    }
  }, [canManageAuth, canReadAuth]);

  useEffect(() => {
    void load();
  }, [load]);
  useEffect(() => {
    if (!visibleTabs.length) return;
    if (!visibleTabs.some(item => item.id === tab)) {
      setTab(visibleTabs[0].id);
    }
  }, [tab, visibleTabs]);
  useEffect(() => {
    const provider = authProviders[selectedAuth];
    setAuthEnabled(provider?.enabled ?? false);
    setAuthName(provider?.name ?? authProviderMeta[selectedAuth].name);
    setAuthForm(normalizeConfig(provider?.config));
  }, [authProviders, selectedAuth]);

  const authCards = useMemo(
    () =>
      authProviderOrder.map(id => ({
        id,
        meta: authProviderMeta[id],
        provider: authProviders[id],
      })),
    [authProviders]
  );

  const saveAuth = async () => {
    const nextAuthName = authName.trim();
    if (!nextAuthName) {
      toast.error('显示名称不能为空');
      return;
    }
    const { config, error: configError } = prepareAuthConfig(selectedAuth, authForm, authEnabled);
    if (configError) {
      toast.error(configError);
      return;
    }
    setBusy('save-auth');
    try {
      const saved = await updateAuthProvider(selectedAuth, {
        name: nextAuthName,
        enabled: authEnabled,
        config,
      });
      setAuthProviders(current => ({ ...current, [selectedAuth]: saved }));
      toast.success('认证配置已保存');
    } catch (err) {
      toast.error(err instanceof Error ? err.message : '保存认证配置失败');
    } finally {
      setBusy('');
    }
  };

  const clearAuthConfig = () => {
    setAuthEnabled(false);
    setAuthForm({});
  };

  const testAuth = async () => {
    setBusy('test-auth');
    try {
      const result = await testAuthProvider(selectedAuth);
      toast.success(`认证连接测试通过，成功匹配 ${result.matchedUsers} 个用户`);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : '认证连接测试失败');
    } finally {
      setBusy('');
    }
  };

  const updateAuthConfigField = (field: Field, value: unknown) => {
    setAuthForm(current => {
      const next = { ...current, [field.key]: value };
      if (field.key === 'useTLS' && value === true) {
        next.startTLS = false;
        next.port = 636;
      }
      if (field.key === 'startTLS' && value === true) {
        next.useTLS = false;
        next.port = 389;
      }
      return next;
    });
  };
  return (
    <SettingsPageFrame>
      <div className="flex items-center justify-between gap-4">
        <div>
          <h1 className="text-lg font-semibold" style={{ color: 'var(--kvm-text)' }}>
            系统配置
          </h1>
          <p className="mt-1 text-sm" style={{ color: 'var(--kvm-text-muted)' }}>
            管理平台用户、认证与通知媒介
          </p>
        </div>
        <div
          className="hidden items-center gap-2 rounded-lg px-3 py-2 text-sm md:flex"
          style={{
            color: 'var(--kvm-text-muted)',
            background: 'rgba(255,255,255,0.04)',
            border: '1px solid var(--kvm-border)',
          }}
        >
          <SettingsIcon size={15} />
          配置中心
        </div>
      </div>
      <section
        className="flex flex-wrap gap-2 rounded-xl p-3"
        style={{ background: 'var(--kvm-card)', border: '1px solid var(--kvm-border)' }}
      >
        {visibleTabs.map(item => (
          <SettingsTabButton
            key={item.id}
            active={tab === item.id}
            icon={item.icon}
            label={item.label}
            onClick={() => setTab(item.id)}
          />
        ))}
      </section>
      {error && (
        <div
          className="rounded-xl p-4 text-sm"
          style={{
            background: 'rgba(245,158,11,0.1)',
            border: '1px solid rgba(245,158,11,0.25)',
            color: '#f59e0b',
          }}
        >
          {error}
        </div>
      )}
      {visibleTabs.length === 0 && (
        <div
          className="kvm-empty-state rounded-xl p-8 text-center text-sm"
          style={{ color: 'var(--kvm-text-muted)' }}
        >
          暂无可查看的配置项
        </div>
      )}
      {tab === 'base' && (canReadBase || canManageBase) && (
        <SettingsConfigSection
          title="基础配置"
          description="维护站点名称、登录展示、控制台品牌和固定数字配置规划。"
          badge={<SettingsSectionBadge icon={SlidersHorizontalIcon} label="系统基础" />}
        >
          <BaseSettingsPanel canManage={canManageBase} />
        </SettingsConfigSection>
      )}
      {tab === 'users' && (canReadUsers || canManageUsers) && (
        <SettingsConfigSection
          title="用户配置"
          description="维护平台用户、用户群组和角色权限。启用 AD/LDAP 后，外部账号也必须先在用户中创建并启用。"
          badge={<SettingsSectionBadge icon={UsersRoundIcon} label="用户权限" />}
        >
          <UserSettingsPanel canManage={canManageUsers} />
        </SettingsConfigSection>
      )}
      {tab === 'notifications' && (canReadNotifications || canManageNotifications) && (
        <SettingsConfigSection
          title="通知媒介"
          description="站内告警通过右上角通知中心查看，外部媒介启用后会接收活跃告警、恢复通知推送；也可用于找回密码。"
          badge={<SettingsSectionBadge icon={MegaphoneIcon} label="告警通知" />}
        >
          <NotificationSettingsPanel canManage={canManageNotifications} />
        </SettingsConfigSection>
      )}
      {tab === 'auth' && (canReadAuth || canManageAuth) && (
        <SettingsConfigSection
          title="认证配置"
          description="启用外部认证后，登录界面会显示对应登录方式。"
          badge={<SettingsSectionBadge icon={NetworkIcon} label="身份认证" />}
        >
          <SettingsSplitLayout
            sidebarLabel="认证配置"
            sidebar={authCards.map(({ id, meta: itemMeta, provider }) => (
              <AuthProviderCard
                key={id}
                id={id}
                meta={itemMeta}
                active={selectedAuth === id}
                enabled={provider?.enabled ?? false}
                onSelect={() => setSelectedAuth(id)}
              />
            ))}
          >
            <SettingsDetailPanel
              header={
                <SettingsDetailHeader
                  icon={AuthIcon}
                  color={authMeta.color}
                  title={authMeta.name}
                  subtitle={authEnabled ? '已启用' : '未启用'}
                  active={authEnabled}
                />
              }
              actions={
                canManageAuth ? (
                  <>
                    <button
                      type="button"
                      onClick={clearAuthConfig}
                      disabled={busy !== ''}
                      className="kvm-action-button kvm-danger-button flex items-center gap-2 rounded-lg border px-3 py-2 text-sm disabled:opacity-50"
                      style={{
                        borderColor: 'rgba(239,68,68,0.34)',
                        color: '#f87171',
                        background: 'rgba(239,68,68,0.08)',
                      }}
                    >
                      <Trash2Icon size={14} />
                      清空配置
                    </button>
                    <button
                      type="button"
                      onClick={() => void saveAuth()}
                      disabled={busy !== ''}
                      className="kvm-action-button flex items-center gap-2 rounded-lg border px-3 py-2 text-sm disabled:opacity-50"
                      style={{
                        borderColor: 'rgba(59,130,246,0.38)',
                        color: 'var(--kvm-accent-text)',
                        background: 'rgba(59,130,246,0.1)',
                      }}
                    >
                      <SaveIcon size={14} />
                      保存
                    </button>
                    <button
                      type="button"
                      onClick={() => void testAuth()}
                      disabled={busy !== '' || !authEnabled}
                      className="kvm-action-button flex items-center gap-2 rounded-lg border px-3 py-2 text-sm disabled:cursor-not-allowed disabled:opacity-50"
                      style={{
                        borderColor: 'rgba(16,185,129,0.35)',
                        color: '#34d399',
                        background: 'rgba(16,185,129,0.08)',
                      }}
                    >
                      <CheckCircle2Icon size={14} />
                      测试
                    </button>
                  </>
                ) : null
              }
            >
              <p className="mb-4 text-sm leading-6" style={{ color: 'var(--kvm-text-muted)' }}>
                {authMeta.description}
              </p>
              <EnableMediaToggle
                enabled={authEnabled}
                disabled={!canManageAuth}
                onChange={setAuthEnabled}
                label="启用认证"
                enabledText="登录页将显示该认证方式"
                disabledText="关闭后不会显示在登录页"
              />
              <ConfigField
                field={{ key: 'name', label: '显示名称', placeholder: 'AD/LDAP', required: true }}
                value={authName}
                disabled={!canManageAuth}
                onChange={value => setAuthName(String(value ?? ''))}
              />
              <div className="mt-4 space-y-3">
                <SectionTitle title="必填配置" />
                {authMeta.requiredFields.map(field => (
                  <ConfigField
                    key={field.key}
                    field={field}
                    value={displayValue(field, authForm[field.key])}
                    secretConfigured={secretConfigured(field, authForm)}
                    disabled={!canManageAuth}
                    onChange={value => updateAuthConfigField(field, value)}
                  />
                ))}
              </div>
              <div className="mt-5 space-y-3">
                <SectionTitle title="可选配置" />
                {authMeta.optionalFields.map(field => (
                  <ConfigField
                    key={field.key}
                    field={field}
                    value={displayValue(field, authForm[field.key])}
                    secretConfigured={secretConfigured(field, authForm)}
                    disabled={!canManageAuth}
                    onChange={value => updateAuthConfigField(field, value)}
                  />
                ))}
              </div>
            </SettingsDetailPanel>
          </SettingsSplitLayout>
        </SettingsConfigSection>
      )}
    </SettingsPageFrame>
  );
}

function SettingsTabButton({
  active,
  icon: Icon,
  label,
  onClick,
}: {
  active: boolean;
  icon: React.ElementType;
  label: string;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className="kvm-action-button flex h-10 items-center gap-2 rounded-lg px-4 text-sm font-medium"
      style={{
        color: active ? 'var(--kvm-accent-hover)' : 'var(--kvm-text-muted)',
        background: active ? 'rgba(59,130,246,0.14)' : 'transparent',
        border: active ? '1px solid rgba(59,130,246,0.32)' : '1px solid transparent',
      }}
    >
      <Icon size={16} />
      {label}
    </button>
  );
}

function SettingsPageFrame({ children }: { children: React.ReactNode }) {
  return (
    <div data-cmp="SettingsPage" className="flex h-full min-h-0 flex-col gap-6 overflow-hidden p-6">
      {children}
    </div>
  );
}

function SettingsConfigSection({
  title,
  description,
  badge,
  children,
}: {
  title: string;
  description: string;
  badge: React.ReactNode;
  children: React.ReactNode;
}) {
  return (
    <section
      className="flex min-h-0 flex-1 flex-col overflow-hidden rounded-xl p-5"
      style={{ background: 'var(--kvm-card)', border: '1px solid var(--kvm-border)' }}
    >
      <div className="mb-5 flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
        <div>
          <h2 className="text-sm font-semibold" style={{ color: 'var(--kvm-text)' }}>
            {title}
          </h2>
          <p className="mt-1 text-xs" style={{ color: 'var(--kvm-text-muted)' }}>
            {description}
          </p>
        </div>
        {badge}
      </div>
      {children}
    </section>
  );
}

function SettingsSectionBadge({ icon: Icon, label }: { icon: React.ElementType; label: string }) {
  return (
    <div
      className="flex items-center gap-2 rounded-lg px-3 py-2 text-xs"
      style={{
        color: 'var(--kvm-accent-text)',
        background: 'rgba(59,130,246,0.1)',
        border: '1px solid rgba(59,130,246,0.24)',
      }}
    >
      <Icon size={14} />
      {label}
    </div>
  );
}

function AuthProviderCard({
  id,
  meta,
  active,
  enabled,
  onSelect,
}: {
  id: AuthProviderId;
  meta: (typeof authProviderMeta)[AuthProviderId];
  active: boolean;
  enabled: boolean;
  onSelect: () => void;
}) {
  const CardIcon = meta.icon;
  return (
    <button
      type="button"
      onClick={onSelect}
      className="kvm-action-button flex w-full items-start gap-3 rounded-lg p-3 text-left"
      style={{
        background: active ? 'rgba(59,130,246,0.12)' : 'transparent',
        border: active ? '1px solid rgba(96,165,250,0.56)' : '1px solid transparent',
        color: 'var(--kvm-text)',
      }}
    >
      <div
        className="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg"
        style={{
          color: meta.color,
          background: 'rgba(255,255,255,0.05)',
          border: '1px solid rgba(255,255,255,0.08)',
        }}
      >
        <CardIcon size={19} />
      </div>
      <div className="min-w-0 flex-1">
        <div className="flex items-center justify-between gap-3">
          <span className="truncate text-sm font-semibold">{meta.name}</span>
          {enabled ? (
            <ToggleRightIcon size={18} className="shrink-0" style={{ color: '#86efac' }} />
          ) : (
            <ToggleLeftIcon
              size={18}
              className="shrink-0"
              style={{ color: 'var(--kvm-text-muted)' }}
            />
          )}
        </div>
        <p
          className="mt-1 line-clamp-2 text-xs leading-5"
          style={{ color: 'var(--kvm-text-muted)' }}
        >
          {meta.description}
        </p>
      </div>
    </button>
  );
}

function prepareAuthConfig(
  id: AuthProviderId,
  form: Record<string, unknown>,
  enabled: boolean
): { config: Record<string, unknown>; error: string } {
  const next = { ...form };
  if (!enabled)
    return { config: removeEmptyConfigValues(removeSecretPresenceMarkers(next)), error: '' };
  if (id === 'ldap') {
    if (Boolean(next.useTLS) && Boolean(next.startTLS))
      return { config: {}, error: 'LDAPS 与 StartTLS 不能同时启用' };
    if (Boolean(next.useTLS)) next.port = 636;
    else if (Boolean(next.startTLS)) next.port = 389;
    else if (String(next.port ?? '').trim() === '') next.port = 389;
    else {
      const port = parsePort(next.port);
      if (!port) return { config: {}, error: '端口需为 1 到 65535 之间的整数' };
      next.port = port;
    }
    const missingField = authProviderMeta[id].requiredFields.find(field => {
      if (field.type === 'number') return !Number(next[field.key]);
      if (field.type === 'password' && secretConfigured(field, next)) return false;
      return !String(next[field.key] ?? '').trim();
    });
    if (missingField) return { config: {}, error: `${missingField.label}不能为空` };
  }
  return { config: removeEmptyConfigValues(removeSecretPresenceMarkers(next)), error: '' };
}

function parsePort(value: unknown) {
  const text = String(value ?? '').trim();
  if (!/^\d+$/.test(text)) return 0;
  const port = Number(text);
  if (!Number.isInteger(port) || port < 1 || port > 65535) return 0;
  return port;
}

function isPermissionMessage(message: string) {
  return message.includes('当前用户无权执行此操作');
}
