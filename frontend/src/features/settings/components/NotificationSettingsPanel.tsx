import React, { useEffect, useMemo, useState } from "react";
import { toast } from "sonner";
import {
  CheckCircle2Icon,
  InfoIcon,
  MailIcon,
  MessageSquareIcon,
  RefreshCwIcon,
  RadioTowerIcon,
  SaveIcon,
  SendIcon,
  ToggleLeftIcon,
  ToggleRightIcon,
  Trash2Icon,
  WebhookIcon,
  XIcon,
} from "lucide-react";
import {
  fetchNotificationChannels,
  previewNotificationChannel,
  testNotificationChannel,
  updateNotificationChannel,
  type NotificationChannel,
  type NotificationTemplatePreview,
} from "../../../lib/api";
import { DialogPortal } from "../../../components/kvm/DialogPortal";
import { SelectMenu, type SelectMenuOption } from "../../../components/kvm/SelectMenu";
import {
  ConfigField,
  EnableMediaToggle,
  SettingsDetailHeader,
  SettingsDetailPanel,
  SettingsSplitLayout,
  displayValue,
  normalizeConfig,
  removeEmptyConfigValues,
  removeSecretPresenceMarkers,
  secretConfigured,
  type Field,
} from "./SettingsFormPrimitives";

type ChannelId = "webhook" | "email" | "lark" | "wechat" | "dingtalk";
type TemplateTab = "problem" | "recovery";

const defaultProblemTemplate = "[{{alert.level}}] {{alert.title}}\n{{alert.message}}\n来源：{{alert.sourceType}}/{{alert.sourceId}}";
const defaultRecoveryTemplate = "[恢复] {{alert.title}}\n{{alert.message}}\n来源：{{alert.sourceType}}/{{alert.sourceId}}\n恢复时间：{{alert.resolvedAt}}\n持续时间：{{alert.duration}}";
const defaultProblemSubject = "{{alert.title}}";
const defaultRecoverySubject = "恢复：{{alert.title}}";
const defaultWebhookProblemPayload = `{"id":"{{alert.id}}","eventType":"{{event.type}}","level":"{{alert.level}}","title":"{{alert.title}}","message":"{{alert.message}}","sourceType":"{{alert.sourceType}}","sourceId":"{{alert.sourceId}}","lastSeenAt":"{{alert.lastSeenAt}}"}`;
const defaultWebhookRecoveryPayload = `{"id":"{{alert.id}}","eventType":"{{event.type}}","level":"{{alert.level}}","title":"{{alert.title}}","message":"{{alert.message}}","sourceType":"{{alert.sourceType}}","sourceId":"{{alert.sourceId}}","lastSeenAt":"{{alert.lastSeenAt}}","resolvedAt":"{{alert.resolvedAt}}","duration":"{{alert.duration}}"}`;

const channelMeta: Record<ChannelId, { name: string; description: string; icon: React.ElementType; color: string; fields: Field[] }> = {
  webhook: {
    name: "Webhook",
    description: "通过 HTTP JSON 回调推送告警事件、恢复事件和找回密码验证码到外部系统。",
    icon: WebhookIcon,
    color: "#06b6d4",
    fields: [
      { key: "url", label: "Webhook URL", placeholder: "https://example.com/alert", required: true },
      { key: "method", label: "请求方法", placeholder: "POST", helper: "支持 POST、PUT、PATCH" },
      { key: "headers", label: "请求头 JSON", placeholder: "{\"Authorization\":\"Bearer token\"}", helper: "可选，必须是 JSON 对象", type: "textarea" },
    ],
  },
  email: {
    name: "邮件",
    description: "通过 SMTP 发送关键告警、恢复通知，也可发送找回密码验证码。",
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
  wechat: { name: "企业微信", description: "对接企业微信群机器人接收告警、恢复通知和找回密码验证码。", icon: MessageSquareIcon, color: "#22c55e", fields: [{ key: "webhookUrl", label: "机器人 Webhook", placeholder: "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=...", required: true }] },
  dingtalk: { name: "钉钉", description: "通过钉钉机器人发送故障、恢复通知和找回密码验证码，支持加签密钥。", icon: RadioTowerIcon, color: "#f59e0b", fields: [{ key: "webhookUrl", label: "机器人 Webhook", placeholder: "https://oapi.dingtalk.com/robot/send?access_token=...", required: true }, { key: "secret", label: "加签密钥", placeholder: "可选", type: "password" }] },
};

const channelOrder: ChannelId[] = ["webhook", "email", "lark", "wechat", "dingtalk"];
const messageTypeOptions: Partial<Record<ChannelId, { key: string; label: string; options: SelectMenuOption[] }>> = {
  email: {
    key: "emailContentType",
    label: "内容类型",
    options: [
      { value: "text/plain", label: "纯文本", tooltip: "text/plain" },
      { value: "text/html", label: "HTML", tooltip: "text/html" },
    ],
  },
  lark: {
    key: "larkMessageType",
    label: "消息类型",
    options: [
      { value: "text", label: "文本", tooltip: "text" },
      { value: "post", label: "富文本", tooltip: "post" },
      { value: "interactive", label: "卡片", tooltip: "interactive" },
    ],
  },
  wechat: {
    key: "wechatMessageType",
    label: "消息类型",
    options: [
      { value: "text", label: "文本", tooltip: "text" },
      { value: "markdown", label: "Markdown", tooltip: "markdown" },
    ],
  },
  dingtalk: {
    key: "dingtalkMessageType",
    label: "消息类型",
    options: [
      { value: "text", label: "文本", tooltip: "text" },
      { value: "markdown", label: "Markdown", tooltip: "markdown" },
    ],
  },
};
const templateVariableGroups = [
  {
    title: "事件",
    variables: [
      { token: "{{event.type}}", desc: "事件类型，告警为 problem，恢复为 recovery" },
      { token: "{{event.statusText}}", desc: "中文事件状态，告警或恢复" },
    ],
  },
  {
    title: "告警",
    variables: [
      { token: "{{alert.id}}", desc: "告警 ID" },
      { token: "{{alert.level}}", desc: "原始级别，info、warning 或 critical" },
      { token: "{{alert.levelText}}", desc: "中文级别，信息、警告或严重" },
      { token: "{{alert.status}}", desc: "告警状态，active 或 resolved" },
      { token: "{{alert.title}}", desc: "告警标题" },
      { token: "{{alert.message}}", desc: "告警消息正文" },
      { token: "{{alert.sourceType}}", desc: "来源类型，如 agent、host、virtual_machine" },
      { token: "{{alert.sourceId}}", desc: "来源对象 ID" },
      { token: "{{alert.firstSeenAt}}", desc: "首次触发时间" },
      { token: "{{alert.lastSeenAt}}", desc: "最近触发时间" },
      { token: "{{alert.resolvedAt}}", desc: "恢复时间，仅恢复事件有值" },
      { token: "{{alert.duration}}", desc: "告警持续时间，仅恢复事件有值" },
    ],
  },
  {
    title: "元数据变量",
    variables: [
      { token: "{{metadata.agent}}", desc: "告警元数据中的 Agent 名称" },
      { token: "{{metadata.endpoint}}", desc: "Agent 离线告警中的 Agent 访问地址" },
      { token: "{{metadata.lastError}}", desc: "Agent 离线告警中的最近同步错误" },
      { token: "{{metadata.failureCount}}", desc: "Agent 离线告警中的连续失败次数" },
      { token: "{{metadata.vm}}", desc: "告警元数据中的虚拟机名称" },
      { token: "{{metadata.status}}", desc: "虚拟机状态异常告警中的虚拟机状态" },
      { token: "{{metadata.metric}}", desc: "资源阈值告警中的指标类型，如 cpu、memory、storage 或 disk" },
      { token: "{{metadata.value}}", desc: "资源阈值告警中的当前值" },
      { token: "{{metadata.limit}}", desc: "资源阈值告警中的阈值" },
      { token: "{{metadata.consecutive}}", desc: "资源阈值告警中的连续触发次数" },
    ],
  },
];

export function NotificationSettingsPanel({ canManage }: { canManage: boolean }) {
  const [channels, setChannels] = useState<Record<string, NotificationChannel>>({});
  const [selected, setSelected] = useState<ChannelId>("webhook");
  const [form, setForm] = useState<Record<string, unknown>>({});
  const [enabled, setEnabled] = useState(true);
  const [passwordResetEnabled, setPasswordResetEnabled] = useState(false);
  const [templateTab, setTemplateTab] = useState<TemplateTab>("problem");
  const [variableDialogOpen, setVariableDialogOpen] = useState(false);
  const [preview, setPreview] = useState<NotificationTemplatePreview | null>(null);
  const [busy, setBusy] = useState("");
  const meta = channelMeta[selected];
  const Icon = meta.icon;

  useEffect(() => {
    fetchNotificationChannels()
      .then((response) => setChannels(Object.fromEntries(response.items.map((item) => [item.id, item]))))
      .catch((err) => toast.error(err instanceof Error ? err.message : "读取通知配置失败"));
  }, []);

  useEffect(() => {
    const channel = channels[selected];
    setEnabled(channel?.enabled ?? false);
    setPasswordResetEnabled(channel?.passwordResetEnabled ?? false);
    setForm(normalizeConfig(channel?.config));
    setPreview(null);
  }, [channels, selected]);

  const cards = useMemo(() => channelOrder.map((id) => ({ id, meta: channelMeta[id], channel: channels[id] })), [channels]);

  const save = async () => {
    const { config, error } = prepareConfig(selected, form, enabled || passwordResetEnabled);
    if (error) {
      toast.error(error);
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

  const loadPreview = async () => {
    const { config, error } = prepareConfig(selected, form, false);
    if (error) {
      toast.error(error);
      return;
    }
    setBusy("preview");
    try {
      setPreview(await previewNotificationChannel(selected, { enabled, passwordResetEnabled, config }));
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "生成模板预览失败");
    } finally {
      setBusy("");
    }
  };

  const clearNotificationConfig = () => {
    setEnabled(false);
    setPasswordResetEnabled(false);
    setForm({});
  };

  const updateField = (field: Field, value: unknown) => {
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
    <SettingsSplitLayout
      sidebarLabel="通知媒介"
      sidebar={cards.map(({ id, meta: itemMeta, channel }) => <ChannelCard key={id} id={id} meta={itemMeta} active={selected === id} enabled={channel?.enabled ?? false} passwordResetEnabled={channel?.passwordResetEnabled ?? false} onSelect={() => setSelected(id)} />)}
    >
      <SettingsDetailPanel
        header={<SettingsDetailHeader icon={Icon} color={meta.color} title={meta.name} subtitle={notificationSubtitle(enabled, passwordResetEnabled, Boolean(form.sendRecovery))} active={enabled || passwordResetEnabled} />}
        actions={canManage ? <><button type="button" onClick={clearNotificationConfig} disabled={busy !== ""} className="kvm-action-button kvm-danger-button flex items-center gap-2 rounded-lg border px-3 py-2 text-sm disabled:opacity-50" style={{ borderColor: "rgba(239,68,68,0.34)", color: "#f87171", background: "rgba(239,68,68,0.08)" }}><Trash2Icon size={14} />清空配置</button><button type="button" onClick={() => void save()} disabled={busy !== ""} className="kvm-action-button flex items-center gap-2 rounded-lg border px-3 py-2 text-sm disabled:opacity-50" style={{ borderColor: "rgba(59,130,246,0.38)", color: "var(--kvm-accent-text)", background: "rgba(59,130,246,0.1)" }}><SaveIcon size={14} />保存</button><button type="button" onClick={() => void sendTest()} disabled={busy !== "" || !enabled} className="kvm-action-button flex items-center gap-2 rounded-lg border px-3 py-2 text-sm disabled:cursor-not-allowed disabled:opacity-50" style={{ borderColor: "rgba(16,185,129,0.35)", color: "#34d399", background: "rgba(16,185,129,0.08)" }}><CheckCircle2Icon size={14} />测试</button></> : null}
      >
        <p className="mb-4 text-sm leading-6" style={{ color: "var(--kvm-text-muted)" }}>{meta.description}</p>
        <div className="mb-4 grid gap-3 xl:grid-cols-3">
          <EnableMediaToggle enabled={enabled} disabled={!canManage} onChange={setEnabled} label="告警通知" enabledText="已开启活跃告警推送" disabledText="关闭后不发送告警通知" />
          <EnableMediaToggle enabled={Boolean(form.sendRecovery)} disabled={!canManage || !enabled} onChange={(value) => setForm((current) => ({ ...current, sendRecovery: value }))} label="恢复通知" enabledText="告警恢复后会推送恢复内容" disabledText="关闭后恢复不外发" />
          <EnableMediaToggle enabled={passwordResetEnabled} disabled={!canManage} onChange={setPasswordResetEnabled} label="找回密码" enabledText="可用于发送找回密码验证码" disabledText="关闭后不参与找回密码" />
        </div>
        <div className="space-y-3">{meta.fields.map((field) => <ConfigField key={field.key} field={field} value={displayValue(field, form[field.key])} secretConfigured={secretConfigured(field, form)} disabled={!canManage} onChange={(value) => updateField(field, value)} />)}</div>
        <TemplateEditor selected={selected} tab={templateTab} form={form} preview={preview} busy={busy === "preview"} disabled={!canManage} onPreview={() => void loadPreview()} onOpenVariables={() => setVariableDialogOpen(true)} onTabChange={setTemplateTab} onChange={(key, value) => setForm((current) => ({ ...current, [key]: value }))} />
        {variableDialogOpen && <TemplateVariablesDialog onClose={() => setVariableDialogOpen(false)} />}
      </SettingsDetailPanel>
    </SettingsSplitLayout>
  );
}

function TemplateEditor({ selected, tab, form, preview, busy, disabled, onPreview, onOpenVariables, onTabChange, onChange }: { selected: ChannelId; tab: TemplateTab; form: Record<string, unknown>; preview: NotificationTemplatePreview | null; busy: boolean; disabled: boolean; onPreview: () => void; onOpenVariables: () => void; onTabChange: (tab: TemplateTab) => void; onChange: (key: string, value: string) => void }) {
  const textKey = tab === "problem" ? "problemTemplate" : "recoveryTemplate";
  const subjectKey = tab === "problem" ? "problemSubjectTemplate" : "recoverySubjectTemplate";
  const webhookKey = tab === "problem" ? "webhookProblemPayload" : "webhookRecoveryPayload";
  const defaultText = tab === "problem" ? defaultProblemTemplate : defaultRecoveryTemplate;
  const defaultSubject = tab === "problem" ? defaultProblemSubject : defaultRecoverySubject;
  const defaultWebhook = tab === "problem" ? defaultWebhookProblemPayload : defaultWebhookRecoveryPayload;
  const messageType = messageTypeOptions[selected];
  return (
    <section className="mt-5 rounded-lg border p-3" style={{ borderColor: "var(--kvm-border)", background: "rgba(255,255,255,0.026)" }}>
      <div className="mb-3 flex flex-wrap items-center justify-between gap-3">
        <div>
          <div className="text-xs font-semibold" style={{ color: "var(--kvm-text)" }}>告警模板</div>
          <div className="mt-1 text-[11px]" style={{ color: "var(--kvm-text-muted)" }}>留空时使用系统默认模板，变量会在发送前替换</div>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <button type="button" onClick={onOpenVariables} className="kvm-action-button flex h-8 items-center gap-1.5 rounded-lg border px-2.5 text-xs" style={{ borderColor: "var(--kvm-border)", color: "var(--kvm-text-muted)", background: "rgba(255,255,255,0.03)" }}><InfoIcon size={13} />变量说明</button>
          <button type="button" onClick={onPreview} disabled={busy} className="kvm-action-button flex h-8 items-center gap-1.5 rounded-lg border px-2.5 text-xs disabled:opacity-60" style={{ borderColor: "rgba(59,130,246,0.34)", color: "var(--kvm-accent-text)", background: "rgba(59,130,246,0.08)" }}><RefreshCwIcon size={13} className={busy ? "animate-spin" : ""} />预览</button>
          <div className="flex rounded-lg border p-1" style={{ borderColor: "var(--kvm-border)", background: "rgba(255,255,255,0.03)" }}>
            <TemplateTabButton active={tab === "problem"} label="告警" onClick={() => onTabChange("problem")} />
            <TemplateTabButton active={tab === "recovery"} label="恢复" onClick={() => onTabChange("recovery")} />
          </div>
        </div>
      </div>
      {messageType && (
        <div className="mb-3 w-44 max-w-full">
          <div className="mb-1.5 text-xs" style={{ color: "var(--kvm-text-muted)" }}>{messageType.label}</div>
          <SelectMenu
            value={String(form[messageType.key] ?? messageType.options[0]?.value ?? "")}
            options={messageType.options}
            placeholder="选择发送内容类型"
            disabled={disabled}
            maxVisibleItems={4}
            buttonClassName="!font-normal"
            optionClassName="!font-normal"
            optionTooltipPlacement="right"
            onChange={(value) => onChange(messageType.key, value)}
          />
        </div>
      )}
      {selected === "email" && <TemplateTextarea label="邮件主题" value={String(form[subjectKey] ?? "")} placeholder={defaultSubject} disabled={disabled} rows={2} onChange={(value) => onChange(subjectKey, value)} />}
      <TemplateTextarea label="文本内容" value={String(form[textKey] ?? "")} placeholder={defaultText} disabled={disabled} onChange={(value) => onChange(textKey, value)} />
      {selected === "webhook" && <TemplateTextarea label="Webhook JSON 内容" value={String(form[webhookKey] ?? "")} placeholder={defaultWebhook} disabled={disabled} rows={5} onChange={(value) => onChange(webhookKey, value)} />}
      {preview && <TemplatePreviewPanel selected={selected} tab={tab} preview={preview} />}
    </section>
  );
}

function TemplatePreviewPanel({ selected, tab, preview }: { selected: ChannelId; tab: TemplateTab; preview: NotificationTemplatePreview }) {
  const subject = tab === "problem" ? preview.problemSubject : preview.recoverySubject;
  const text = tab === "problem" ? preview.problemText : preview.recoveryText;
  const webhook = tab === "problem" ? preview.problemWebhook : preview.recoveryWebhook;
  return (
    <div className="mt-4 rounded-lg border p-3" style={{ borderColor: "rgba(59,130,246,0.24)", background: "rgba(59,130,246,0.06)" }}>
      <div className="mb-2 text-xs font-semibold" style={{ color: "var(--kvm-text)" }}>预览结果</div>
      {preview.contentType && <PreviewBlock title="内容类型" value={preview.contentType} />}
      {preview.messageType && <PreviewBlock title="消息类型" value={preview.messageType} />}
      {selected === "email" && <PreviewBlock title="邮件主题" value={subject} />}
      <PreviewBlock title="文本内容" value={text} />
      {selected === "webhook" && webhook && <PreviewBlock title="Webhook JSON" value={JSON.stringify(webhook, null, 2)} />}
    </div>
  );
}

function PreviewBlock({ title, value }: { title: string; value: string }) {
  return <div className="mt-2"><div className="mb-1 text-[11px]" style={{ color: "var(--kvm-text-muted)" }}>{title}</div><pre className="max-h-44 overflow-auto whitespace-pre-wrap rounded-lg border p-2 font-mono text-xs leading-5" style={{ borderColor: "var(--kvm-border)", color: "var(--kvm-text)", background: "var(--kvm-control-bg)" }}>{value || "-"}</pre></div>;
}

function TemplateVariablesDialog({ onClose }: { onClose: () => void }) {
  return (
    <DialogPortal>
      <div className="kvm-dialog-backdrop fixed inset-0 z-[80] flex items-center justify-center px-4" role="dialog" aria-modal="true" onMouseDown={(event) => { if (event.target === event.currentTarget) onClose(); }}>
        <div className="kvm-dialog-panel flex max-h-[82vh] w-full max-w-3xl flex-col overflow-hidden rounded-xl border p-4 shadow-2xl" style={{ background: "var(--kvm-card)", borderColor: "var(--kvm-border)", boxShadow: "0 24px 80px rgba(0,0,0,0.32), inset 0 1px rgba(255,255,255,0.08)" }}>
          <div className="mb-3 flex items-center justify-between gap-3">
            <div><div className="text-sm font-semibold" style={{ color: "var(--kvm-text)" }}>模板变量说明</div><div className="mt-1 text-xs" style={{ color: "var(--kvm-text-muted)" }}>变量可用于文本模板、邮件主题和 Webhook JSON 模板</div></div>
            <button type="button" onClick={onClose} aria-label="关闭变量说明" className="kvm-action-button flex h-8 w-8 items-center justify-center rounded-lg border" style={{ borderColor: "var(--kvm-border)", color: "var(--kvm-text-muted)", background: "rgba(255,255,255,0.035)" }}><XIcon size={15} /></button>
          </div>
          <div className="kvm-hidden-scrollbar min-h-0 space-y-4 overflow-y-auto pr-1">
            {templateVariableGroups.map((group) => (
              <section key={group.title}>
                <div className="mb-2 text-xs font-semibold" style={{ color: "var(--kvm-text)" }}>{group.title}</div>
                <div className="grid gap-2 md:grid-cols-2">
                  {group.variables.map((item) => <div key={item.token} className="kvm-card-hover rounded-lg border p-3" style={{ borderColor: "var(--kvm-border)", background: "rgba(255,255,255,0.026)" }}><code className="text-xs" style={{ color: "var(--kvm-accent-text)" }}>{item.token}</code><div className="mt-1 text-xs leading-5" style={{ color: "var(--kvm-text-muted)" }}>{item.desc}</div></div>)}
                </div>
              </section>
            ))}
            <div className="rounded-lg border p-3 text-xs leading-5" style={{ borderColor: "rgba(59,130,246,0.26)", background: "rgba(59,130,246,0.07)", color: "var(--kvm-text-muted)" }}>
              元数据变量会随告警来源变化，也可以使用 <code style={{ color: "var(--kvm-accent-text)" }}>{"{{metadata.<字段名>}}"}</code> 引用后续新增的元数据字段
            </div>
          </div>
        </div>
      </div>
    </DialogPortal>
  );
}

function TemplateTextarea({ label, value, placeholder, disabled, rows = 4, onChange }: { label: string; value: string; placeholder: string; disabled: boolean; rows?: number; onChange: (value: string) => void }) {
  return <label className="mt-3 block space-y-1.5 text-xs" style={{ color: "var(--kvm-text-muted)" }}><span>{label}</span><textarea value={value} disabled={disabled} rows={rows} onChange={(event) => onChange(event.target.value)} placeholder={placeholder} className="w-full resize-y rounded-lg px-3 py-2 font-mono text-xs leading-5 outline-none disabled:opacity-60" style={{ background: "var(--kvm-control-bg)", border: "1px solid var(--kvm-border)", color: "var(--kvm-text)" }} /></label>;
}

function TemplateTabButton({ active, label, onClick }: { active: boolean; label: string; onClick: () => void }) {
  return <button type="button" onClick={onClick} className="h-7 rounded-md px-3 text-xs" style={{ color: active ? "var(--kvm-accent-hover)" : "var(--kvm-text-muted)", background: active ? "rgba(59,130,246,0.14)" : "transparent" }}>{label}</button>;
}

function ChannelCard({ id, meta, active, enabled, passwordResetEnabled, onSelect }: { id: ChannelId; meta: typeof channelMeta[ChannelId]; active: boolean; enabled: boolean; passwordResetEnabled: boolean; onSelect: () => void }) {
  const CardIcon = meta.icon;
  return <button type="button" onClick={onSelect} className="kvm-action-button flex w-full items-start gap-3 rounded-lg p-3 text-left" style={{ background: active ? "rgba(59,130,246,0.12)" : "transparent", border: active ? "1px solid rgba(96,165,250,0.56)" : "1px solid transparent", color: "var(--kvm-text)" }}><div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg" style={{ color: meta.color, background: "rgba(255,255,255,0.05)", border: "1px solid rgba(255,255,255,0.08)" }}><CardIcon size={19} /></div><div className="min-w-0 flex-1"><div className="flex items-center justify-between gap-3"><span className="truncate text-sm font-semibold">{meta.name}</span><span className="flex shrink-0 items-center gap-1"><UsageIcon enabled={enabled} label="告" /><UsageIcon enabled={passwordResetEnabled} label="密" /></span></div><p className="mt-1 line-clamp-2 text-xs leading-5" style={{ color: "var(--kvm-text-muted)" }}>{meta.description}</p></div></button>;
}

function UsageIcon({ enabled, label }: { enabled: boolean; label: string }) {
  return <span className="inline-flex h-5 w-5 items-center justify-center rounded-full border text-[10px] font-semibold" style={{ color: enabled ? "#86efac" : "var(--kvm-text-muted)", borderColor: enabled ? "rgba(134,239,172,0.48)" : "var(--kvm-border)", background: enabled ? "rgba(16,185,129,0.12)" : "rgba(148,163,184,0.08)" }}>{label}</span>;
}

function notificationSubtitle(alertEnabled: boolean, passwordEnabled: boolean, recoveryEnabled: boolean) {
  if (alertEnabled && recoveryEnabled && passwordEnabled) return "告警 / 恢复 / 找回密码已启用";
  if (alertEnabled && recoveryEnabled) return "告警 / 恢复已启用";
  if (alertEnabled && passwordEnabled) return "告警通知 / 找回密码已启用";
  if (alertEnabled) return "告警通知已启用";
  if (passwordEnabled) return "找回密码已启用";
  return "未启用";
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
  if (id === "email" && !["", "text/plain", "text/html"].includes(String(next.emailContentType ?? ""))) {
    return { config: {}, error: "邮件内容类型不正确" };
  }
  if (id === "lark" && !["", "text", "post", "interactive"].includes(String(next.larkMessageType ?? ""))) {
    return { config: {}, error: "飞书消息类型不正确" };
  }
  if (id === "wechat" && !["", "text", "markdown"].includes(String(next.wechatMessageType ?? ""))) {
    return { config: {}, error: "企业微信消息类型不正确" };
  }
  if (id === "dingtalk" && !["", "text", "markdown"].includes(String(next.dingtalkMessageType ?? ""))) {
    return { config: {}, error: "钉钉消息类型不正确" };
  }
  return { config: removeSecretPresenceMarkers(next), error: "" };
}
