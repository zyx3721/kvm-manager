import React, { useCallback, useEffect, useMemo, useState } from "react";
import { toast } from "sonner";
import {
  BellRingIcon,
  CheckCircle2Icon,
  EyeIcon,
  EyeOffIcon,
  MailIcon,
  MegaphoneIcon,
  MessageSquareIcon,
  NetworkIcon,
  SlidersHorizontalIcon,
  RadioTowerIcon,
  SaveIcon,
  SendIcon,
  SettingsIcon,
  ToggleLeftIcon,
  ToggleRightIcon,
  Trash2Icon,
  UsersRoundIcon,
  WebhookIcon,
} from "lucide-react";
import { BaseSettingsPanel } from "./components/BaseSettingsPanel";
import { UserSettingsPanel } from "./components/UserSettingsPanel";
import { KvmTooltip } from "../../components/kvm/StatusBadge";
import {
  fetchNotificationChannels,
  fetchAuthProviders,
  testAuthProvider,
  testNotificationChannel,
  updateAuthProvider,
  updateNotificationChannel,
  type AuthProvider,
  type NotificationChannel,
} from "../../lib/api";
import { can } from "../../lib/permissions";

type SettingsTab = "base" | "users" | "auth" | "notifications";
type ChannelId = "webhook" | "email" | "lark" | "wechat" | "dingtalk";
type AuthProviderId = "ldap";
type Field = {
  key: string;
  label: string;
  placeholder: string;
  required?: boolean;
  helper?: string;
  type?: "text" | "password" | "number" | "checkbox" | "textarea";
};

const channelMeta: Record<ChannelId, { name: string; description: string; icon: React.ElementType; color: string; fields: Field[] }> = {
  webhook: {
    name: "Webhook",
    description: "通过 HTTP JSON 回调推送告警事件和找回密码验证码到外部系统。",
    icon: WebhookIcon,
    color: "#06b6d4",
    fields: [
      { key: "url", label: "Webhook URL", placeholder: "https://example.com/alert", required: true },
      { key: "method", label: "请求方法", placeholder: "POST", helper: "支持 POST、PUT、PATCH" },
      { key: "headers", label: "请求头 JSON", placeholder: "{\"Authorization\":\"Bearer token\"}", helper: "可选，必须是 JSON 对象", type: "textarea" },
    ],
  },
  email: {
    name: "邮件通知",
    description: "通过 SMTP 发送关键告警，也可发送找回密码验证码。",
    icon: MailIcon,
    color: "#10b981",
    fields: [
      { key: "smtpHost", label: "SMTP 主机", placeholder: "smtp.example.com", required: true },
      { key: "smtpPort", label: "SMTP 端口", placeholder: "465", required: true, type: "number" },
      { key: "username", label: "用户名", placeholder: "alert@example.com", required: true },
      { key: "password", label: "密码", placeholder: "SMTP 授权码", required: true, type: "password" },
      { key: "from", label: "发件人", placeholder: "alert@example.com", required: true },
      { key: "fromName", label: "发件人名称", placeholder: "KVM Console" },
      { key: "to", label: "收件人", placeholder: "ops@example.com,admin@example.com", required: true, helper: "多个邮箱用英文逗号分隔" },
      { key: "useTLS", label: "启用 TLS/SSL", placeholder: "", type: "checkbox" },
      { key: "startTLS", label: "启用 STARTTLS", placeholder: "", type: "checkbox" },
    ],
  },
  lark: { name: "飞书", description: "推送到飞书群机器人或告警协作群，可用于找回密码验证码，支持签名密钥。", icon: SendIcon, color: "#8b5cf6", fields: [{ key: "webhookUrl", label: "机器人 Webhook", placeholder: "https://open.feishu.cn/open-apis/bot/v2/hook/...", required: true }, { key: "secret", label: "签名密钥", placeholder: "可选", type: "password" }] },
  wechat: { name: "企业微信", description: "对接企业微信群机器人接收告警和找回密码验证码。", icon: MessageSquareIcon, color: "#22c55e", fields: [{ key: "webhookUrl", label: "机器人 Webhook", placeholder: "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=...", required: true }] },
  dingtalk: { name: "钉钉", description: "通过钉钉机器人发送故障、恢复通知和找回密码验证码，支持加签密钥。", icon: RadioTowerIcon, color: "#f59e0b", fields: [{ key: "webhookUrl", label: "机器人 Webhook", placeholder: "https://oapi.dingtalk.com/robot/send?access_token=...", required: true }, { key: "secret", label: "加签密钥", placeholder: "可选", type: "password" }] },
};

const authProviderMeta: Record<AuthProviderId, { name: string; description: string; icon: React.ElementType; color: string; requiredFields: Field[]; optionalFields: Field[] }> = {
  ldap: {
    name: "AD/LDAP",
    description: "通过企业目录服务实现统一身份认证，支持 AD/LDAP 登录。",
    icon: NetworkIcon,
    color: "#38bdf8",
    requiredFields: [
      { key: "host", label: "服务器地址", placeholder: "ldap.example.com", required: true },
      { key: "port", label: "端口", placeholder: "389", required: true, type: "number" },
      { key: "baseDN", label: "Base DN", placeholder: "dc=example,dc=com", required: true },
      { key: "userFilter", label: "用户过滤器", placeholder: "(sAMAccountName={username})", required: true },
      { key: "bindDN", label: "绑定 DN", placeholder: "cn=readonly,dc=example,dc=com", required: true },
      { key: "bindPassword", label: "绑定密码", placeholder: "请输入绑定账号密码", required: true, type: "password" },
    ],
    optionalFields: [
      { key: "useTLS", label: "启用 LDAPS", placeholder: "", type: "checkbox" },
      { key: "startTLS", label: "启用 STARTTLS", placeholder: "", type: "checkbox" },
      { key: "insecureSkipVerify", label: "跳过证书校验", placeholder: "", type: "checkbox" },
      { key: "timeoutSeconds", label: "超时时间", placeholder: "8", type: "number" },
      { key: "groupFilter", label: "用户组过滤器", placeholder: "cn=ops,dc=example,dc=com" },
    ],
  },
};

const channelOrder: ChannelId[] = ["webhook", "email", "lark", "wechat", "dingtalk"];
const authProviderOrder: AuthProviderId[] = ["ldap"];
const settingsNavColumnWidth = "430px";

export default function SettingsPage() {
  const [tab, setTab] = useState<SettingsTab>("base");
  const [channels, setChannels] = useState<Record<string, NotificationChannel>>({});
  const [authProviders, setAuthProviders] = useState<Record<string, AuthProvider>>({});
  const [selected, setSelected] = useState<ChannelId>("webhook");
  const [selectedAuth, setSelectedAuth] = useState<AuthProviderId>("ldap");
  const [form, setForm] = useState<Record<string, unknown>>({});
  const [authForm, setAuthForm] = useState<Record<string, unknown>>({});
  const [authName, setAuthName] = useState("AD/LDAP");
  const [enabled, setEnabled] = useState(true);
  const [passwordResetEnabled, setPasswordResetEnabled] = useState(false);
  const [authEnabled, setAuthEnabled] = useState(false);
  const [busy, setBusy] = useState("");
  const [error, setError] = useState("");
  const meta = channelMeta[selected];
  const Icon = meta.icon;
  const authMeta = authProviderMeta[selectedAuth];
  const AuthIcon = authMeta.icon;
  const canReadBase = can("settings.base.read");
  const canManageBase = can("settings.base.manage");
  const canReadUsers = can("settings.users.read");
  const canManageUsers = can("settings.users.manage");
  const canReadAuth = can("settings.auth.read");
  const canManageAuth = can("settings.auth.manage");
  const canReadNotifications = can("settings.notifications.read");
  const canManageNotifications = can("settings.notifications.manage");
  const visibleTabs = useMemo(
    () =>
      [
        { id: "base" as const, icon: SlidersHorizontalIcon, label: "基础配置", visible: canReadBase || canManageBase },
        { id: "users" as const, icon: UsersRoundIcon, label: "用户配置", visible: canReadUsers || canManageUsers },
        { id: "auth" as const, icon: NetworkIcon, label: "认证配置", visible: canReadAuth || canManageAuth },
        { id: "notifications" as const, icon: BellRingIcon, label: "通知配置", visible: canReadNotifications || canManageNotifications },
      ].filter((item) => item.visible),
    [canManageAuth, canManageBase, canManageNotifications, canManageUsers, canReadAuth, canReadBase, canReadNotifications, canReadUsers]
  );

  const load = useCallback(async () => {
    setError("");
    try {
      const requests: Promise<void>[] = [];
      if (canReadNotifications || canManageNotifications) {
        requests.push(fetchNotificationChannels().then((response) => {
          setChannels(Object.fromEntries(response.items.map((item) => [item.id, item])));
        }));
      }
      if (canReadAuth || canManageAuth) {
        requests.push(fetchAuthProviders().then((response) => {
          setAuthProviders(Object.fromEntries(response.items.map((item) => [item.id, item])));
        }));
      }
      await Promise.all(requests);
    } catch (err) {
      const message = err instanceof Error ? err.message : "读取系统配置失败";
      toast.error(message);
      setError(isPermissionMessage(message) ? "" : message);
    }
  }, [canManageAuth, canManageNotifications, canReadAuth, canReadNotifications]);

  useEffect(() => { void load(); }, [load]);
  useEffect(() => {
    if (!visibleTabs.length) return;
    if (!visibleTabs.some((item) => item.id === tab)) {
      setTab(visibleTabs[0].id);
    }
  }, [tab, visibleTabs]);
  useEffect(() => {
    const channel = channels[selected];
    setEnabled(channel?.enabled ?? false);
    setPasswordResetEnabled(channel?.passwordResetEnabled ?? false);
    setForm(normalizeConfig(channel?.config));
  }, [channels, selected]);
  useEffect(() => {
    const provider = authProviders[selectedAuth];
    setAuthEnabled(provider?.enabled ?? false);
    setAuthName(provider?.name ?? authProviderMeta[selectedAuth].name);
    setAuthForm(normalizeConfig(provider?.config));
  }, [authProviders, selectedAuth]);

  const cards = useMemo(() => channelOrder.map((id) => ({ id, meta: channelMeta[id], channel: channels[id] })), [channels]);
  const authCards = useMemo(() => authProviderOrder.map((id) => ({ id, meta: authProviderMeta[id], provider: authProviders[id] })), [authProviders]);

  const save = async () => {
    const { config, error: configError } = prepareConfig(selected, form, enabled || passwordResetEnabled);
    if (configError) {
      toast.error(configError);
      return;
    }
    setBusy("save");
    try {
      const saved = await updateNotificationChannel(selected, { enabled, passwordResetEnabled, config });
      setChannels((current) => ({ ...current, [selected]: saved }));
      toast.success("通知配置已保存");
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "保存通知配置失败");
    } finally {
      setBusy("");
    }
  };

  const sendTest = async () => {
    setBusy("test");
    try {
      await testNotificationChannel(selected);
      toast.success("测试通知已发送");
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "发送测试通知失败");
    } finally {
      setBusy("");
    }
  };

  const saveAuth = async () => {
    const nextAuthName = authName.trim();
    if (!nextAuthName) {
      toast.error("显示名称不能为空");
      return;
    }
    const { config, error: configError } = prepareAuthConfig(selectedAuth, authForm, authEnabled);
    if (configError) {
      toast.error(configError);
      return;
    }
    setBusy("save-auth");
    try {
      const saved = await updateAuthProvider(selectedAuth, { name: nextAuthName, enabled: authEnabled, config });
      setAuthProviders((current) => ({ ...current, [selectedAuth]: saved }));
      toast.success("认证配置已保存");
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "保存认证配置失败");
    } finally {
      setBusy("");
    }
  };

  const clearNotificationConfig = () => {
    setEnabled(false);
    setPasswordResetEnabled(false);
    setForm({});
  };

  const clearAuthConfig = () => {
    setAuthEnabled(false);
    setAuthForm({});
  };

  const testAuth = async () => {
    setBusy("test-auth");
    try {
      const result = await testAuthProvider(selectedAuth);
      toast.success(`认证连接测试通过，成功匹配 ${result.matchedUsers} 个用户`);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "认证连接测试失败");
    } finally {
      setBusy("");
    }
  };

  const updateAuthConfigField = (field: Field, value: unknown) => {
    setAuthForm((current) => {
      const next = { ...current, [field.key]: value };
      if (field.key === "useTLS" && value === true) {
        next.startTLS = false;
        next.port = 636;
      }
      if (field.key === "startTLS" && value === true) {
        next.useTLS = false;
        next.port = 389;
      }
      return next;
    });
  };
  const updateNotificationConfigField = (field: Field, value: unknown) => {
    setForm((current) => {
      const next = { ...current, [field.key]: value };
      if (selected === "email" && field.key === "useTLS" && value === true) {
        next.startTLS = false;
        next.smtpPort = 465;
      }
      if (selected === "email" && field.key === "startTLS" && value === true) {
        next.useTLS = false;
        next.smtpPort = 587;
      }
      return next;
    });
  };

  return (
    <SettingsPageFrame>
      <div className="flex items-center justify-between gap-4">
        <div><h1 className="text-lg font-semibold" style={{ color: "var(--kvm-text)" }}>系统配置</h1><p className="mt-1 text-sm" style={{ color: "var(--kvm-text-muted)" }}>管理平台用户、认证与通知媒介</p></div>
        <div className="hidden items-center gap-2 rounded-lg px-3 py-2 text-sm md:flex" style={{ color: "var(--kvm-text-muted)", background: "rgba(255,255,255,0.04)", border: "1px solid var(--kvm-border)" }}><SettingsIcon size={15} />配置中心</div>
      </div>
      <section className="flex flex-wrap gap-2 rounded-xl p-3" style={{ background: "var(--kvm-card)", border: "1px solid var(--kvm-border)" }}>
        {visibleTabs.map((item) => <SettingsTabButton key={item.id} active={tab === item.id} icon={item.icon} label={item.label} onClick={() => setTab(item.id)} />)}
      </section>
      {error && <div className="rounded-xl p-4 text-sm" style={{ background: "rgba(245,158,11,0.1)", border: "1px solid rgba(245,158,11,0.25)", color: "#f59e0b" }}>{error}</div>}
      {visibleTabs.length === 0 && <div className="kvm-empty-state rounded-xl p-8 text-center text-sm" style={{ color: "var(--kvm-text-muted)" }}>暂无可查看的配置项</div>}
      {tab === "base" && (canReadBase || canManageBase) && <SettingsConfigSection title="基础配置" description="维护站点名称、登录展示、控制台品牌和固定数字配置规划。" badge={<SettingsSectionBadge icon={SlidersHorizontalIcon} label="系统基础" />}>
        <BaseSettingsPanel canManage={canManageBase} />
      </SettingsConfigSection>}
      {tab === "users" && (canReadUsers || canManageUsers) && <SettingsConfigSection title="用户配置" description="维护平台用户、用户群组和角色权限。启用 AD/LDAP 后，外部账号也必须先在用户中创建并启用。" badge={<SettingsSectionBadge icon={UsersRoundIcon} label="用户权限" />}>
        <UserSettingsPanel canManage={canManageUsers} />
      </SettingsConfigSection>}
      {tab === "notifications" && (canReadNotifications || canManageNotifications) && <SettingsConfigSection title="通知媒介" description="站内告警通过右上角通知中心查看，外部媒介启用后会接收活跃告警推送；也可用于找回密码。" badge={<SettingsSectionBadge icon={MegaphoneIcon} label="告警通知" />}>
        <SettingsSplitLayout
          sidebarLabel="通知媒介"
          sidebar={cards.map(({ id, meta: itemMeta, channel }) => <ChannelCard key={id} id={id} meta={itemMeta} active={selected === id} enabled={channel?.enabled ?? false} passwordResetEnabled={channel?.passwordResetEnabled ?? false} onSelect={() => setSelected(id)} />)}
        >
          <SettingsDetailPanel
            header={<SettingsDetailHeader icon={Icon} color={meta.color} title={meta.name} subtitle={notificationSubtitle(enabled, passwordResetEnabled)} active={enabled || passwordResetEnabled} />}
            actions={canManageNotifications ? <><button type="button" onClick={clearNotificationConfig} disabled={busy !== ""} className="kvm-action-button kvm-danger-button flex items-center gap-2 rounded-lg border px-3 py-2 text-sm disabled:opacity-50" style={{ borderColor: "rgba(239,68,68,0.34)", color: "#f87171", background: "rgba(239,68,68,0.08)" }}><Trash2Icon size={14} />清空配置</button><button type="button" onClick={() => void save()} disabled={busy !== ""} className="kvm-action-button flex items-center gap-2 rounded-lg border px-3 py-2 text-sm disabled:opacity-50" style={{ borderColor: "rgba(59,130,246,0.38)", color: "var(--kvm-accent-text)", background: "rgba(59,130,246,0.1)" }}><SaveIcon size={14} />保存</button><button type="button" onClick={() => void sendTest()} disabled={busy !== "" || !enabled} className="kvm-action-button flex items-center gap-2 rounded-lg border px-3 py-2 text-sm disabled:cursor-not-allowed disabled:opacity-50" style={{ borderColor: "rgba(16,185,129,0.35)", color: "#34d399", background: "rgba(16,185,129,0.08)" }}><CheckCircle2Icon size={14} />测试</button></> : null}
          >
              <p className="mb-4 text-sm leading-6" style={{ color: "var(--kvm-text-muted)" }}>{meta.description}</p>
              <div className="mb-4 grid gap-3 xl:grid-cols-2">
                <EnableMediaToggle enabled={enabled} disabled={!canManageNotifications} onChange={setEnabled} label="告警通知" enabledText="已开启活跃告警推送" disabledText="关闭后不发送告警通知" />
                <EnableMediaToggle enabled={passwordResetEnabled} disabled={!canManageNotifications} onChange={setPasswordResetEnabled} label="找回密码" enabledText="可用于发送找回密码验证码" disabledText="关闭后不参与找回密码" />
              </div>
              <div className="space-y-3">{meta.fields.map((field) => <ConfigField key={field.key} field={field} value={displayValue(field, form[field.key])} secretConfigured={secretConfigured(field, form)} disabled={!canManageNotifications} onChange={(value) => updateNotificationConfigField(field, value)} />)}</div>
          </SettingsDetailPanel>
        </SettingsSplitLayout>
      </SettingsConfigSection>}
      {tab === "auth" && (canReadAuth || canManageAuth) && <SettingsConfigSection title="认证配置" description="启用外部认证后，登录界面会显示对应登录方式。" badge={<SettingsSectionBadge icon={NetworkIcon} label="身份认证" />}>
        <SettingsSplitLayout
          sidebarLabel="认证配置"
          sidebar={authCards.map(({ id, meta: itemMeta, provider }) => <AuthProviderCard key={id} id={id} meta={itemMeta} active={selectedAuth === id} enabled={provider?.enabled ?? false} onSelect={() => setSelectedAuth(id)} />)}
        >
          <SettingsDetailPanel
            header={<SettingsDetailHeader icon={AuthIcon} color={authMeta.color} title={authMeta.name} subtitle={authEnabled ? "已启用" : "未启用"} active={authEnabled} />}
            actions={canManageAuth ? <><button type="button" onClick={clearAuthConfig} disabled={busy !== ""} className="kvm-action-button kvm-danger-button flex items-center gap-2 rounded-lg border px-3 py-2 text-sm disabled:opacity-50" style={{ borderColor: "rgba(239,68,68,0.34)", color: "#f87171", background: "rgba(239,68,68,0.08)" }}><Trash2Icon size={14} />清空配置</button><button type="button" onClick={() => void saveAuth()} disabled={busy !== ""} className="kvm-action-button flex items-center gap-2 rounded-lg border px-3 py-2 text-sm disabled:opacity-50" style={{ borderColor: "rgba(59,130,246,0.38)", color: "var(--kvm-accent-text)", background: "rgba(59,130,246,0.1)" }}><SaveIcon size={14} />保存</button><button type="button" onClick={() => void testAuth()} disabled={busy !== "" || !authEnabled} className="kvm-action-button flex items-center gap-2 rounded-lg border px-3 py-2 text-sm disabled:cursor-not-allowed disabled:opacity-50" style={{ borderColor: "rgba(16,185,129,0.35)", color: "#34d399", background: "rgba(16,185,129,0.08)" }}><CheckCircle2Icon size={14} />测试</button></> : null}
          >
              <p className="mb-4 text-sm leading-6" style={{ color: "var(--kvm-text-muted)" }}>{authMeta.description}</p>
              <EnableMediaToggle enabled={authEnabled} disabled={!canManageAuth} onChange={setAuthEnabled} label="启用认证" enabledText="登录页将显示该认证方式" disabledText="关闭后不会显示在登录页" />
              <ConfigField field={{ key: "name", label: "显示名称", placeholder: "AD/LDAP", required: true }} value={authName} disabled={!canManageAuth} onChange={(value) => setAuthName(String(value ?? ""))} />
              <div className="mt-4 space-y-3">
                <SectionTitle title="必填配置" />
                {authMeta.requiredFields.map((field) => <ConfigField key={field.key} field={field} value={displayValue(field, authForm[field.key])} secretConfigured={secretConfigured(field, authForm)} disabled={!canManageAuth} onChange={(value) => updateAuthConfigField(field, value)} />)}
              </div>
              <div className="mt-5 space-y-3">
                <SectionTitle title="可选配置" />
                {authMeta.optionalFields.map((field) => <ConfigField key={field.key} field={field} value={displayValue(field, authForm[field.key])} secretConfigured={secretConfigured(field, authForm)} disabled={!canManageAuth} onChange={(value) => updateAuthConfigField(field, value)} />)}
              </div>
          </SettingsDetailPanel>
        </SettingsSplitLayout>
      </SettingsConfigSection>}
    </SettingsPageFrame>
  );
}

function SettingsTabButton({ active, icon: Icon, label, onClick }: { active: boolean; icon: React.ElementType; label: string; onClick: () => void }) {
  return <button type="button" onClick={onClick} className="kvm-action-button flex h-10 items-center gap-2 rounded-lg px-4 text-sm font-medium" style={{ color: active ? "var(--kvm-accent-hover)" : "var(--kvm-text-muted)", background: active ? "rgba(59,130,246,0.14)" : "transparent", border: active ? "1px solid rgba(59,130,246,0.32)" : "1px solid transparent" }}><Icon size={16} />{label}</button>;
}

function SettingsPageFrame({ children }: { children: React.ReactNode }) {
  return <div data-cmp="SettingsPage" className="flex h-full min-h-0 flex-col gap-6 overflow-hidden p-6">{children}</div>;
}

function SettingsConfigSection({ title, description, badge, children }: { title: string; description: string; badge: React.ReactNode; children: React.ReactNode }) {
  return (
    <section className="flex min-h-0 flex-1 flex-col overflow-hidden rounded-xl p-5" style={{ background: "var(--kvm-card)", border: "1px solid var(--kvm-border)" }}>
      <div className="mb-5 flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
        <div>
          <h2 className="text-sm font-semibold" style={{ color: "var(--kvm-text)" }}>{title}</h2>
          <p className="mt-1 text-xs" style={{ color: "var(--kvm-text-muted)" }}>{description}</p>
        </div>
        {badge}
      </div>
      {children}
    </section>
  );
}

function SettingsSectionBadge({ icon: Icon, label }: { icon: React.ElementType; label: string }) {
  return <div className="flex items-center gap-2 rounded-lg px-3 py-2 text-xs" style={{ color: "var(--kvm-accent-text)", background: "rgba(59,130,246,0.1)", border: "1px solid rgba(59,130,246,0.24)" }}><Icon size={14} />{label}</div>;
}

function SettingsSplitLayout({ sidebarLabel, sidebar, children }: { sidebarLabel: string; sidebar: React.ReactNode; children: React.ReactNode }) {
  return (
    <div className="grid min-h-0 flex-1 grid-cols-1 items-stretch gap-4 overflow-hidden xl:grid-cols-[var(--settings-nav-column)_minmax(0,1fr)]" style={{ "--settings-nav-column": settingsNavColumnWidth } as React.CSSProperties}>
      <nav className="kvm-hidden-scrollbar max-h-64 min-h-0 space-y-2 overflow-y-auto rounded-lg p-2 xl:max-h-none" style={{ background: "rgba(255,255,255,0.035)", border: "1px solid var(--kvm-border)" }} aria-label={sidebarLabel}>{sidebar}</nav>
      {children}
    </div>
  );
}

function SettingsDetailPanel({ header, children, actions }: { header: React.ReactNode; children: React.ReactNode; actions: React.ReactNode }) {
  return (
    <aside className="flex min-h-0 flex-col overflow-hidden rounded-lg p-4" style={{ background: "rgba(255,255,255,0.035)", border: "1px solid var(--kvm-border)" }}>
      {header}
      <div className="kvm-hidden-scrollbar min-h-0 flex-1 overflow-y-auto pr-2">{children}</div>
      {actions && <div className="mt-5 flex shrink-0 justify-end gap-2">{actions}</div>}
    </aside>
  );
}

function SettingsDetailHeader({ icon: Icon, color, title, subtitle, active }: { icon: React.ElementType; color: string; title: string; subtitle: string; active: boolean }) {
  return (
    <div className="mb-4 flex items-center gap-3">
      <div className="flex h-10 w-10 items-center justify-center rounded-lg" style={{ color, background: "rgba(255,255,255,0.05)", border: "1px solid rgba(255,255,255,0.08)" }}><Icon size={19} /></div>
      <div>
        <div className="text-sm font-semibold" style={{ color: "var(--kvm-text)" }}>{title}</div>
        <div className="mt-0.5 text-xs" style={{ color: active ? "#86efac" : "var(--kvm-text-muted)" }}>{subtitle}</div>
      </div>
    </div>
  );
}

function EnableMediaToggle({ enabled, disabled, onChange, label = "启用媒介", enabledText = "已开启外部通知推送", disabledText = "关闭后不会发送外部通知" }: { enabled: boolean; disabled: boolean; onChange: (value: boolean) => void; label?: string; enabledText?: string; disabledText?: string }) {
  return (
    <label
      className={`group flex min-h-14 items-center justify-between gap-3 rounded-lg px-3 py-2 text-sm transition-all duration-200 ${disabled ? "cursor-not-allowed opacity-75" : "cursor-pointer hover:-translate-y-0.5 active:translate-y-0 active:scale-[0.99]"}`}
      style={{
        background: enabled ? "rgba(16,185,129,0.08)" : "rgba(255,255,255,0.035)",
        border: enabled ? "1px solid rgba(16,185,129,0.28)" : "1px solid var(--kvm-border)",
        boxShadow: enabled ? "0 10px 24px rgba(16,185,129,0.08)" : "none",
        color: "var(--kvm-text)",
      }}
    >
      <input type="checkbox" className="peer sr-only" checked={enabled} disabled={disabled} onChange={(event) => onChange(event.target.checked)} />
      <span className="min-w-0">
        <span className="block font-medium">{label}</span>
        <span className="mt-0.5 block text-xs" style={{ color: "var(--kvm-text-muted)" }}>{enabled ? enabledText : disabledText}</span>
      </span>
      <span
        aria-hidden="true"
        className="relative inline-flex h-6 w-11 shrink-0 rounded-full transition-all duration-200 peer-focus-visible:ring-2 peer-focus-visible:ring-emerald-300 peer-focus-visible:ring-offset-2 peer-focus-visible:ring-offset-transparent group-hover:shadow-[0_0_0_4px_rgba(16,185,129,0.08)]"
        style={{ background: enabled ? "rgba(16,185,129,0.95)" : "rgba(148,163,184,0.28)", border: enabled ? "1px solid rgba(134,239,172,0.4)" : "1px solid rgba(148,163,184,0.32)" }}
      >
        <span className={`absolute left-0.5 top-0.5 h-5 w-5 rounded-full bg-white shadow-sm transition-transform duration-200 ${enabled ? "translate-x-5" : "translate-x-0"}`} />
      </span>
    </label>
  );
}

function ChannelCard({ id, meta, active, enabled, passwordResetEnabled, onSelect }: { id: ChannelId; meta: typeof channelMeta[ChannelId]; active: boolean; enabled: boolean; passwordResetEnabled: boolean; onSelect: () => void }) {
  const CardIcon = meta.icon;
  return <button type="button" onClick={onSelect} className="kvm-action-button flex w-full items-start gap-3 rounded-lg p-3 text-left" style={{ background: active ? "rgba(59,130,246,0.12)" : "transparent", border: active ? "1px solid rgba(96,165,250,0.56)" : "1px solid transparent", color: "var(--kvm-text)" }}><div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg" style={{ color: meta.color, background: "rgba(255,255,255,0.05)", border: "1px solid rgba(255,255,255,0.08)" }}><CardIcon size={19} /></div><div className="min-w-0 flex-1"><div className="flex items-center justify-between gap-3"><span className="truncate text-sm font-semibold">{meta.name}</span><span className="flex shrink-0 items-center gap-1"><UsageIcon enabled={enabled} label="告" /><UsageIcon enabled={passwordResetEnabled} label="密" /></span></div><p className="mt-1 line-clamp-2 text-xs leading-5" style={{ color: "var(--kvm-text-muted)" }}>{meta.description}</p></div></button>;
}

function UsageIcon({ enabled, label }: { enabled: boolean; label: string }) {
  return <span className="inline-flex h-5 w-5 items-center justify-center rounded-full border text-[10px] font-semibold" style={{ color: enabled ? "#86efac" : "var(--kvm-text-muted)", borderColor: enabled ? "rgba(134,239,172,0.48)" : "var(--kvm-border)", background: enabled ? "rgba(16,185,129,0.12)" : "rgba(148,163,184,0.08)" }}>{label}</span>;
}

function notificationSubtitle(alertEnabled: boolean, passwordEnabled: boolean) {
  if (alertEnabled && passwordEnabled) return "告警通知 / 找回密码已启用";
  if (alertEnabled) return "告警通知已启用";
  if (passwordEnabled) return "找回密码已启用";
  return "未启用";
}

function AuthProviderCard({ id, meta, active, enabled, onSelect }: { id: AuthProviderId; meta: typeof authProviderMeta[AuthProviderId]; active: boolean; enabled: boolean; onSelect: () => void }) {
  const CardIcon = meta.icon;
  return <button type="button" onClick={onSelect} className="kvm-action-button flex w-full items-start gap-3 rounded-lg p-3 text-left" style={{ background: active ? "rgba(59,130,246,0.12)" : "transparent", border: active ? "1px solid rgba(96,165,250,0.56)" : "1px solid transparent", color: "var(--kvm-text)" }}><div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg" style={{ color: meta.color, background: "rgba(255,255,255,0.05)", border: "1px solid rgba(255,255,255,0.08)" }}><CardIcon size={19} /></div><div className="min-w-0 flex-1"><div className="flex items-center justify-between gap-3"><span className="truncate text-sm font-semibold">{meta.name}</span>{enabled ? <ToggleRightIcon size={18} className="shrink-0" style={{ color: "#86efac" }} /> : <ToggleLeftIcon size={18} className="shrink-0" style={{ color: "var(--kvm-text-muted)" }} />}</div><p className="mt-1 line-clamp-2 text-xs leading-5" style={{ color: "var(--kvm-text-muted)" }}>{meta.description}</p></div></button>;
}

function SectionTitle({ title }: { title: string }) {
  return <div className="text-xs font-semibold" style={{ color: "var(--kvm-text)" }}>{title}</div>;
}

function ConfigField({ field, value, onChange, secretConfigured = false, disabled = false }: { field: Field; value: unknown; onChange: (value: unknown) => void; secretConfigured?: boolean; disabled?: boolean }) {
  const [passwordVisible, setPasswordVisible] = useState(false);
  if (field.type === "checkbox") return <label className={`flex items-start gap-2 text-sm ${disabled ? "cursor-not-allowed opacity-70" : "cursor-pointer"}`} style={{ color: "var(--kvm-text-muted)" }}><input type="checkbox" disabled={disabled} className="mt-1 cursor-pointer disabled:cursor-not-allowed" checked={Boolean(value)} onChange={(event) => onChange(event.target.checked)} /><span><span className="block">{field.label}</span>{field.helper && <span className="mt-0.5 block text-[11px] leading-4" style={{ color: "var(--kvm-text-muted)" }}>{field.helper}</span>}</span></label>;
  const commonStyle = { background: "var(--kvm-control-bg)", border: "1px solid var(--kvm-border)", color: "var(--kvm-text)" };
  const inputType = field.type === "password" ? (passwordVisible ? "text" : "password") : field.type === "number" ? "number" : "text";
  const textValue = String(value ?? "");
  const passwordCanReveal = field.type === "password" && textValue !== "";
  const placeholder = field.type === "password" && secretConfigured ? "已配置，留空表示不修改" : field.placeholder;
  return <label className="block space-y-1.5 text-xs" style={{ color: "var(--kvm-text-muted)" }}><span className="flex items-center gap-1">{field.label}{field.required && <span style={{ color: "#f87171" }}>*</span>}</span>{field.type === "textarea" ? <textarea value={textValue} disabled={disabled} onChange={(event) => onChange(event.target.value)} placeholder={field.placeholder} rows={3} className="w-full resize-y rounded-lg px-3 py-2 text-sm outline-none disabled:opacity-60" style={commonStyle} /> : <span className="relative block"><input value={textValue} disabled={disabled} onChange={(event) => onChange(field.type === "number" ? Number(event.target.value) : event.target.value)} placeholder={placeholder} type={inputType} className={`w-full rounded-lg px-3 py-2 text-sm outline-none disabled:opacity-60 ${field.type === "password" ? "pr-10" : ""}`} style={commonStyle} />{field.type === "password" && <KvmTooltip label={passwordVisible ? "隐藏密码" : "显示密码"} placement="top" align="center"><button type="button" disabled={disabled || !passwordCanReveal} className="kvm-action-button absolute right-2 top-1/2 flex h-7 w-7 -translate-y-1/2 items-center justify-center rounded-md disabled:cursor-not-allowed disabled:opacity-50" style={{ color: "var(--kvm-text-muted)", background: "transparent" }} onClick={(event) => { event.preventDefault(); setPasswordVisible((current) => !current); }} aria-label={passwordVisible ? "隐藏密码" : "显示密码"}>{passwordVisible ? <EyeOffIcon size={15} /> : <EyeIcon size={15} />}</button></KvmTooltip>}</span>}{field.helper && <span className="block text-[11px] leading-4" style={{ color: "var(--kvm-text-muted)" }}>{field.helper}</span>}</label>;
}

function normalizeConfig(config: NotificationChannel["config"] | AuthProvider["config"] | undefined) {
  if (!config) return {};
  if (typeof config === "string") {
    try { return JSON.parse(config) as Record<string, unknown>; } catch { return {}; }
  }
  return config;
}

function displayValue(field: Field, value: unknown) {
  if (field.key === "headers" && value && typeof value !== "string") return JSON.stringify(value, null, 2);
  if (field.key === "to" && Array.isArray(value)) return value.join(",");
  return value;
}

function secretConfigured(field: Field, config: Record<string, unknown>) {
  if (field.type !== "password") return false;
  if (String(config[field.key] ?? "").trim() !== "") return true;
  return Boolean(config[secretPresenceKey(field.key)]);
}

function secretPresenceKey(key: string) {
  return `has${key.charAt(0).toUpperCase()}${key.slice(1)}`;
}

function prepareConfig(id: ChannelId, form: Record<string, unknown>, enabled: boolean): { config: Record<string, unknown>; error: string } {
  const next = { ...form };
  if (!enabled) return { config: removeEmptyConfigValues(removeSecretPresenceMarkers(next)), error: "" };
  if (id === "webhook") {
    next.method = String(next.method || "").trim().toUpperCase();
    if (typeof next.headers === "string") {
      try { next.headers = next.headers.trim() ? JSON.parse(next.headers) : ""; } catch { return { config: {}, error: "请求头 JSON 格式不正确" }; }
    }
    return { config: removeEmptyConfigValues(next), error: "" };
  }
  if (id === "email" && typeof next.smtpPort === "string") next.smtpPort = Number(next.smtpPort);
  if (id === "email") {
    if (Boolean(next.useTLS) && Boolean(next.startTLS)) return { config: {}, error: "TLS 与 STARTTLS 不能同时启用" };
    if (Boolean(next.useTLS)) next.smtpPort = 465;
    else if (Boolean(next.startTLS)) next.smtpPort = 587;
    const missingField = channelMeta[id].fields.find((field) => {
      if (!field.required) return false;
      if (field.type === "number") return !Number(next[field.key]);
      if (field.type === "password" && secretConfigured(field, next)) return false;
      return !String(next[field.key] ?? "").trim();
    });
    if (missingField) return { config: {}, error: `${missingField.label}不能为空` };
  }
  return { config: removeSecretPresenceMarkers(next), error: "" };
}

function prepareAuthConfig(id: AuthProviderId, form: Record<string, unknown>, enabled: boolean): { config: Record<string, unknown>; error: string } {
  const next = { ...form };
  if (!enabled) return { config: removeEmptyConfigValues(removeSecretPresenceMarkers(next)), error: "" };
  if (id === "ldap") {
    if (Boolean(next.useTLS) && Boolean(next.startTLS)) return { config: {}, error: "LDAPS 与 StartTLS 不能同时启用" };
    if (Boolean(next.useTLS)) next.port = 636;
    else if (Boolean(next.startTLS)) next.port = 389;
    else if (!Number(next.port)) next.port = 389;
    const missingField = authProviderMeta[id].requiredFields.find((field) => {
      if (field.type === "number") return !Number(next[field.key]);
      if (field.type === "password" && secretConfigured(field, next)) return false;
      return !String(next[field.key] ?? "").trim();
    });
    if (missingField) return { config: {}, error: `${missingField.label}不能为空` };
  }
  return { config: removeEmptyConfigValues(removeSecretPresenceMarkers(next)), error: "" };
}

function removeEmptyConfigValues(config: Record<string, unknown>) {
  return Object.fromEntries(Object.entries(config).filter(([, value]) => {
    if (typeof value === "string") return value.trim() !== "";
    if (typeof value === "number") return Number.isFinite(value) && value > 0;
    if (Array.isArray(value)) return value.length > 0;
    if (value && typeof value === "object") return Object.keys(value).length > 0;
    return value !== undefined && value !== null;
  }));
}

function removeSecretPresenceMarkers(config: Record<string, unknown>) {
  return Object.fromEntries(Object.entries(config).filter(([key]) => !/^has[A-Z]/.test(key)));
}

function isPermissionMessage(message: string) {
  return message.includes("当前用户无权执行此操作");
}
