import { useCallback, useEffect, useMemo, useState } from 'react';
import { toast } from 'sonner';
import {
  CheckCircle2Icon,
  MessageCircleIcon,
  NetworkIcon,
  SaveIcon,
  ToggleLeftIcon,
  ToggleRightIcon,
  Trash2Icon,
} from 'lucide-react';
import {
  fetchAuthProviders,
  testAuthProvider,
  updateAuthProvider,
  type AuthProvider,
} from '../../../lib/api';
import { WECOM_PROVIDERS_CHANGED_EVENT } from '../../../lib/auth';
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
} from './SettingsFormPrimitives';

type ProviderSelection = 'ldap' | 'wecom';
type WecomAuthMode = 'direct' | 'sso';

const ldapRequiredFields: Field[] = [
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
];

const ldapOptionalFields: Field[] = [
  { key: 'useTLS', label: '启用 LDAPS', placeholder: '', type: 'checkbox' },
  { key: 'startTLS', label: '启用 STARTTLS', placeholder: '', type: 'checkbox' },
  { key: 'insecureSkipVerify', label: '跳过证书校验', placeholder: '', type: 'checkbox' },
  { key: 'timeoutSeconds', label: '超时时间', placeholder: '8', type: 'number' },
  { key: 'groupFilter', label: '用户组过滤器', placeholder: 'cn=ops,dc=example,dc=com' },
];

const wecomDirectFields: Field[] = [
  { key: 'corpid', label: '企业 ID（corpid）', placeholder: 'ww********', required: true },
  {
    key: 'agentid',
    label: '应用 AgentID',
    placeholder: '1000002',
    required: true,
    inputMode: 'numeric',
  },
  {
    key: 'secret',
    label: '应用 Secret',
    type: 'password',
    placeholder: '请输入自建应用的应用密钥',
    required: true,
  },
  {
    key: 'redirectPrefix',
    label: '回调地址前缀',
    placeholder: '留空则按当前访问地址推断',
    helper: '企业微信服务器需能访问，如 https://kvm.example.com',
  },
];

const wecomSsoFields: Field[] = [
  {
    key: 'ssoBaseUrl',
    label: '认证中心地址',
    placeholder: 'https://auth.example.com',
    required: true,
    helper: '统一认证中心（wecom-auth-center）的外部访问地址',
  },
  {
    key: 'ssoAppID',
    label: '应用标识',
    placeholder: 'kvm',
    required: true,
    helper: '认证中心 config.yaml 中 apps 下的条目名',
  },
  {
    key: 'ssoAppSecret',
    label: '应用密钥',
    type: 'password',
    placeholder: '与认证中心 apps 配置的 app_secret 一致',
    required: true,
  },
];

const wecomDirectGuidance =
  '配置步骤：企业微信管理后台 →「应用管理」→ 自建应用（记录 AgentID 与 Secret）→ 在「网页授权及 JS-SDK」中把回调域名加入可信域名 → 在「企业可信 IP」中加入本服务出口 IP。扫码确认后企业微信会携带授权码跳转至「回调地址前缀 + /api/auth/wecom/callback」完成登录或绑定。';

const wecomSsoGuidance =
  '配置步骤：部署企业微信统一认证中心（wecom-auth-center）→ 在认证中心 config.yaml 的 apps 下为本系统新增条目：domain 填本系统外部访问地址、callback_path 填 /api/auth/wecom/sso/callback、app_secret 填 32 位以上随机密钥 → 在上方填写认证中心地址、应用标识与应用密钥（与认证中心保持一致）→ 重启认证中心使配置生效。登录时本系统跳转认证中心完成企微扫码，认证中心携带一次性 ticket 回跳本系统完成登录或绑定。';

const authModeOptions: Array<{ value: WecomAuthMode; label: string; description: string }> = [
  { value: 'direct', label: '直连企业微信', description: '本系统直接持有企微应用凭据并完成扫码' },
  { value: 'sso', label: '统一认证中心', description: '经由 wecom-auth-center 完成企微扫码后回跳' },
];

export default function AuthSettingsPanel({ canManage }: { canManage: boolean }) {
  const [selected, setSelected] = useState<ProviderSelection>('ldap');
  const [providers, setProviders] = useState<Record<string, AuthProvider>>({});

  const load = useCallback(async () => {
    try {
      const response = await fetchAuthProviders();
      setProviders(Object.fromEntries(response.items.map(item => [item.id, item])));
    } catch (err) {
      toast.error(err instanceof Error ? err.message : '读取认证配置失败');
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  const handleChanged = useCallback(() => {
    void load();
    window.dispatchEvent(new Event(WECOM_PROVIDERS_CHANGED_EVENT));
  }, [load]);

  const providerCards = useMemo(
    () => [
      {
        id: 'ldap' as const,
        name: providers.ldap?.name || 'AD/LDAP',
        description: '通过企业目录服务实现统一身份认证，支持 AD/LDAP 登录',
        icon: NetworkIcon,
        color: '#38bdf8',
        enabled: providers.ldap?.enabled ?? false,
      },
      {
        id: 'wecom' as const,
        name: providers.wecom?.name || '企业微信',
        description: '企业微信扫码登录，支持在右上角绑定企微账号',
        icon: MessageCircleIcon,
        color: '#22d3ee',
        enabled: providers.wecom?.enabled ?? false,
      },
    ],
    [providers]
  );

  return (
    <SettingsSplitLayout
      sidebarLabel="认证配置"
      sidebar={providerCards.map(card => (
        <ProviderCard
          key={card.id}
          card={card}
          active={selected === card.id}
          onSelect={() => setSelected(card.id)}
        />
      ))}
    >
      {selected === 'ldap' ? (
        <LdapSettingsPanel
          provider={providers.ldap}
          canManage={canManage}
          onChanged={handleChanged}
        />
      ) : (
        <WecomSettingsPanel
          provider={providers.wecom}
          canManage={canManage}
          onChanged={handleChanged}
        />
      )}
    </SettingsSplitLayout>
  );
}

type ProviderCardMeta = {
  id: ProviderSelection;
  name: string;
  description: string;
  icon: React.ElementType;
  color: string;
  enabled: boolean;
};

function ProviderCard({
  card,
  active,
  onSelect,
}: {
  card: ProviderCardMeta;
  active: boolean;
  onSelect: () => void;
}) {
  const CardIcon = card.icon;
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
          color: card.color,
          background: 'rgba(255,255,255,0.05)',
          border: '1px solid rgba(255,255,255,0.08)',
        }}
      >
        <CardIcon size={19} />
      </div>
      <div className="min-w-0 flex-1">
        <div className="flex items-center justify-between gap-3">
          <span className="truncate text-sm font-semibold">{card.name}</span>
          {card.enabled ? (
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
          {card.description}
        </p>
      </div>
    </button>
  );
}

function LdapSettingsPanel({
  provider,
  canManage,
  onChanged,
}: {
  provider?: AuthProvider;
  canManage: boolean;
  onChanged: () => void;
}) {
  const [form, setForm] = useState<Record<string, unknown>>({});
  const [name, setName] = useState('AD/LDAP');
  const [enabled, setEnabled] = useState(false);
  const [busy, setBusy] = useState<'save' | 'test' | ''>('');

  useEffect(() => {
    setEnabled(provider?.enabled ?? false);
    setName(provider?.name || 'AD/LDAP');
    setForm(normalizeConfig(provider?.config));
  }, [provider]);

  const save = async () => {
    const nextName = name.trim();
    if (!nextName) {
      toast.error('显示名称不能为空');
      return;
    }
    const { config, error } = prepareLdapConfig(form, enabled);
    if (error) {
      toast.error(error);
      return;
    }
    setBusy('save');
    try {
      await updateAuthProvider('ldap', { name: nextName, enabled, config });
      toast.success('认证配置已保存');
      onChanged();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : '保存认证配置失败');
    } finally {
      setBusy('');
    }
  };

  const test = async () => {
    setBusy('test');
    try {
      const result = await testAuthProvider('ldap');
      toast.success(`认证连接测试通过，成功匹配 ${result.matchedUsers} 个用户`);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : '认证连接测试失败');
    } finally {
      setBusy('');
    }
  };

  const updateField = (field: Field, value: unknown) => {
    setForm(current => {
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
    <SettingsDetailPanel
      header={
        <SettingsDetailHeader
          icon={NetworkIcon}
          color="#38bdf8"
          title={name}
          subtitle={enabled ? '已启用' : '未启用'}
          active={enabled}
        />
      }
      actions={
        canManage ? (
          <>
            <button
              type="button"
              onClick={() => {
                setEnabled(false);
                setForm({});
              }}
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
              onClick={() => void save()}
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
              onClick={() => void test()}
              disabled={busy !== '' || !enabled}
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
        通过企业目录服务实现统一身份认证，支持 AD/LDAP 登录。
      </p>
      <EnableMediaToggle
        enabled={enabled}
        disabled={!canManage}
        onChange={setEnabled}
        label="启用认证"
        enabledText="登录页将显示 AD/LDAP 认证登录方式"
        disabledText="关闭后不会显示在登录页"
      />
      <ConfigField
        field={{ key: 'name', label: '显示名称', placeholder: 'AD/LDAP', required: true }}
        value={name}
        disabled={!canManage}
        onChange={value => setName(String(value ?? ''))}
      />
      <div className="mt-4 space-y-3">
        <SectionTitle title="必填配置" />
        {ldapRequiredFields.map(field => (
          <ConfigField
            key={field.key}
            field={field}
            value={displayValue(field, form[field.key])}
            secretConfigured={secretConfigured(field, form)}
            disabled={!canManage}
            onChange={value => updateField(field, value)}
          />
        ))}
      </div>
      <div className="mt-5 space-y-3">
        <SectionTitle title="可选配置" />
        {ldapOptionalFields.map(field => (
          <ConfigField
            key={field.key}
            field={field}
            value={displayValue(field, form[field.key])}
            secretConfigured={secretConfigured(field, form)}
            disabled={!canManage}
            onChange={value => updateField(field, value)}
          />
        ))}
      </div>
    </SettingsDetailPanel>
  );
}

function WecomSettingsPanel({
  provider,
  canManage,
  onChanged,
}: {
  provider?: AuthProvider;
  canManage: boolean;
  onChanged: () => void;
}) {
  const defaultForm = useMemo(
    () => ({
      authMode: 'direct',
      corpid: '',
      agentid: '',
      secret: '',
      redirectPrefix: '',
      ssoBaseUrl: '',
      ssoAppID: '',
      ssoAppSecret: '',
    }),
    []
  );
  const [form, setForm] = useState<Record<string, unknown>>(defaultForm);
  const [enabled, setEnabled] = useState(false);
  const [busy, setBusy] = useState<'save' | ''>('');

  useEffect(() => {
    setEnabled(provider?.enabled ?? false);
    setForm({ ...defaultForm, ...normalizeConfig(provider?.config) });
  }, [provider, defaultForm]);

  const authMode: WecomAuthMode = form.authMode === 'sso' ? 'sso' : 'direct';

  const save = async () => {
    const displayName = String(provider?.name || '企业微信');
    const { config, error } = prepareWecomConfig(form, enabled);
    if (error) {
      toast.error(error);
      return;
    }
    setBusy('save');
    try {
      await updateAuthProvider('wecom', { name: displayName, enabled, config });
      toast.success('企业微信认证配置已保存');
      onChanged();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : '企业微信认证配置保存失败');
    } finally {
      setBusy('');
    }
  };

  const clearConfig = () => {
    setEnabled(false);
    setForm(defaultForm);
  };

  const updateField = (field: Field, value: unknown) => {
    setForm(current => ({ ...current, [field.key]: value }));
  };

  const activeFields = authMode === 'sso' ? wecomSsoFields : wecomDirectFields;

  return (
    <SettingsDetailPanel
      header={
        <SettingsDetailHeader
          icon={MessageCircleIcon}
          color="#22d3ee"
          title={provider?.name || '企业微信'}
          subtitle={enabled ? '已启用' : '未启用'}
          active={enabled}
        />
      }
      actions={
        canManage ? (
          <>
            <button
              type="button"
              onClick={clearConfig}
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
              onClick={() => void save()}
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
          </>
        ) : null
      }
    >
      <p className="mb-4 text-sm leading-6" style={{ color: 'var(--kvm-text-muted)' }}>
        启用后登录页提供企业微信扫码登录，用户可在右上角菜单绑定与解绑企微账号。
      </p>
      <EnableMediaToggle
        enabled={enabled}
        disabled={!canManage}
        onChange={setEnabled}
        label="启用认证"
        enabledText="登录页将显示企业微信扫码登录方式"
        disabledText="关闭后不会显示在登录页"
      />
      <ConfigField
        field={{ key: 'name', label: '显示名称', placeholder: '企业微信' }}
        value={provider?.name || '企业微信'}
        disabled={true}
        onChange={() => undefined}
      />
      <div className="mt-4 space-y-3">
        <SectionTitle title="认证方式" />
        <AuthModeSwitch
          value={authMode}
          disabled={!canManage}
          onChange={value => setForm(current => ({ ...current, authMode: value }))}
        />
        <SectionTitle title="应用配置" />
        {activeFields.map(field => (
          <ConfigField
            key={field.key}
            field={field}
            value={displayValue(field, form[field.key])}
            secretConfigured={
              (field.key === 'secret' && Boolean(form.hasSecret)) ||
              (field.key === 'ssoAppSecret' && Boolean(form.hasSsoAppSecret))
            }
            disabled={!canManage}
            onChange={value => updateField(field, value)}
          />
        ))}
      </div>
      <p
        className="mt-5 rounded-lg p-3 text-xs leading-5"
        style={{
          border: '1px solid var(--kvm-border)',
          background: 'var(--kvm-control-bg-soft)',
          color: 'var(--kvm-text-muted)',
        }}
      >
        {authMode === 'sso' ? wecomSsoGuidance : wecomDirectGuidance}
      </p>
    </SettingsDetailPanel>
  );
}

function AuthModeSwitch({
  value,
  disabled,
  onChange,
}: {
  value: WecomAuthMode;
  disabled: boolean;
  onChange: (mode: WecomAuthMode) => void;
}) {
  return (
    <div className="grid grid-cols-1 gap-2 sm:grid-cols-2">
      {authModeOptions.map(option => {
        const active = option.value === value;
        return (
          <button
            key={option.value}
            type="button"
            disabled={disabled}
            onClick={() => onChange(option.value)}
            className={[
              'rounded-xl border p-3 text-left transition-all duration-300 ease-out',
              'hover:-translate-y-0.5 disabled:cursor-not-allowed disabled:opacity-60 disabled:hover:translate-y-0',
              active
                ? 'border-[var(--kvm-accent-text)] bg-[rgba(59,130,246,0.12)] hover:shadow-[0_6px_18px_rgba(37,99,235,0.16)]'
                : 'border-[var(--kvm-border)] bg-[var(--kvm-control-bg-soft)] hover:border-[var(--kvm-accent-text)] hover:bg-[rgba(59,130,246,0.06)]',
            ].join(' ')}
          >
            <span
              className="flex items-center gap-1.5 text-sm font-medium"
              style={{ color: 'var(--kvm-text)' }}
            >
              <span
                className="h-2 w-2 rounded-full"
                style={{ background: active ? 'var(--kvm-accent-text)' : 'var(--kvm-border)' }}
              />
              {option.label}
            </span>
            <span
              className="mt-1 block text-xs leading-4"
              style={{ color: 'var(--kvm-text-muted)' }}
            >
              {option.description}
            </span>
          </button>
        );
      })}
    </div>
  );
}

function prepareLdapConfig(form: Record<string, unknown>, enabled: boolean) {
  const next = { ...form };
  if (!enabled) {
    return { config: removeEmptyConfigValues(removeSecretPresenceMarkers(next)), error: '' };
  }
  if (Boolean(next.useTLS) && Boolean(next.startTLS)) {
    return { config: {}, error: 'LDAPS 与 StartTLS 不能同时启用' };
  }
  if (Boolean(next.useTLS)) next.port = 636;
  else if (Boolean(next.startTLS)) next.port = 389;
  else if (String(next.port ?? '').trim() === '') next.port = 389;
  else {
    const port = parsePort(next.port);
    if (!port) return { config: {}, error: '端口需为 1 到 65535 之间的整数' };
    next.port = port;
  }
  const missingField = ldapRequiredFields.find(field => {
    if (field.type === 'number') return !Number(next[field.key]);
    if (field.type === 'password' && secretConfigured(field, next)) return false;
    return !String(next[field.key] ?? '').trim();
  });
  if (missingField) return { config: {}, error: `${missingField.label}不能为空` };
  return { config: removeEmptyConfigValues(removeSecretPresenceMarkers(next)), error: '' };
}

function prepareWecomConfig(form: Record<string, unknown>, enabled: boolean) {
  const config = Object.fromEntries(
    Object.entries(form).filter(
      ([key, value]) =>
        key !== 'hasSecret' && key !== 'hasSsoAppSecret' && String(value ?? '').trim() !== ''
    )
  );
  if (!enabled) return { config, error: '' };
  if (form.authMode === 'sso') {
    const baseURL = String(config.ssoBaseUrl || '').trim();
    if (!baseURL) return { config: {}, error: '认证中心地址不能为空' };
    if (!/^https?:\/\//i.test(baseURL)) {
      return { config: {}, error: '认证中心地址需以 http:// 或 https:// 开头' };
    }
    if (!String(config.ssoAppID || '').trim()) return { config: {}, error: '应用标识不能为空' };
    if (!config.ssoAppSecret && !form.hasSsoAppSecret) {
      return { config: {}, error: '应用密钥不能为空' };
    }
    return { config, error: '' };
  }
  if (!String(config.corpid || '').trim())
    return { config: {}, error: '企业 ID（corpid）不能为空' };
  if (!Number(config.agentid)) return { config: {}, error: '应用 AgentID 不能为空' };
  if (!config.secret && !form.hasSecret) return { config: {}, error: '应用 Secret 不能为空' };
  return { config, error: '' };
}

function parsePort(value: unknown) {
  const text = String(value ?? '').trim();
  if (!/^\d+$/.test(text)) return 0;
  const port = Number(text);
  if (!Number.isInteger(port) || port < 1 || port > 65535) return 0;
  return port;
}
