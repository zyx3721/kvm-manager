import React, { useState } from "react";
import { EyeIcon, EyeOffIcon } from "lucide-react";
import { KvmTooltip } from "../../../components/kvm/StatusBadge";
import type { AuthProvider, NotificationChannel } from "../../../lib/api";

export type Field = {
  key: string;
  label: string;
  placeholder: string;
  required?: boolean;
  helper?: string;
  inputMode?: React.HTMLAttributes<HTMLInputElement>["inputMode"];
  type?: "text" | "password" | "number" | "checkbox" | "textarea";
};

const settingsNavColumnWidth = "430px";

export function SettingsSplitLayout({ sidebarLabel, sidebar, children }: { sidebarLabel: string; sidebar: React.ReactNode; children: React.ReactNode }) {
  return (
    <div className="grid min-h-0 flex-1 grid-cols-1 items-stretch gap-4 overflow-hidden xl:grid-cols-[var(--settings-nav-column)_minmax(0,1fr)]" style={{ "--settings-nav-column": settingsNavColumnWidth } as React.CSSProperties}>
      <nav className="kvm-hidden-scrollbar max-h-64 min-h-0 space-y-2 overflow-y-auto rounded-lg p-2 xl:max-h-none" style={{ background: "rgba(255,255,255,0.035)", border: "1px solid var(--kvm-border)" }} aria-label={sidebarLabel}>{sidebar}</nav>
      {children}
    </div>
  );
}

export function SettingsDetailPanel({ header, children, actions }: { header: React.ReactNode; children: React.ReactNode; actions: React.ReactNode }) {
  return (
    <aside className="flex min-h-0 flex-col overflow-hidden rounded-lg p-4" style={{ background: "rgba(255,255,255,0.035)", border: "1px solid var(--kvm-border)" }}>
      {header}
      <div className="kvm-hidden-scrollbar min-h-0 flex-1 overflow-y-auto pr-2">{children}</div>
      {actions && <div className="mt-5 flex shrink-0 justify-end gap-2">{actions}</div>}
    </aside>
  );
}

export function SettingsDetailHeader({ icon: Icon, color, title, subtitle, active }: { icon: React.ElementType; color: string; title: string; subtitle: string; active: boolean }) {
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

export function EnableMediaToggle({ enabled, disabled, onChange, label = "启用媒介", enabledText = "已开启外部通知推送", disabledText = "关闭后不会发送外部通知" }: { enabled: boolean; disabled: boolean; onChange: (value: boolean) => void; label?: string; enabledText?: string; disabledText?: string }) {
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

export function SectionTitle({ title }: { title: string }) {
  return <div className="text-xs font-semibold" style={{ color: "var(--kvm-text)" }}>{title}</div>;
}

export function ConfigField({ field, value, onChange, secretConfigured = false, disabled = false }: { field: Field; value: unknown; onChange: (value: unknown) => void; secretConfigured?: boolean; disabled?: boolean }) {
  const [passwordVisible, setPasswordVisible] = useState(false);
  if (field.type === "checkbox") return <label className={`flex items-start gap-2 text-sm ${disabled ? "cursor-not-allowed opacity-70" : "cursor-pointer"}`} style={{ color: "var(--kvm-text-muted)" }}><input type="checkbox" disabled={disabled} className="mt-1 cursor-pointer disabled:cursor-not-allowed" checked={Boolean(value)} onChange={(event) => onChange(event.target.checked)} /><span><span className="block">{field.label}</span>{field.helper && <span className="mt-0.5 block text-[11px] leading-4" style={{ color: "var(--kvm-text-muted)" }}>{field.helper}</span>}</span></label>;
  const commonStyle = { background: "var(--kvm-control-bg)", border: "1px solid var(--kvm-border)", color: "var(--kvm-text)" };
  const inputType = field.type === "password" ? (passwordVisible ? "text" : "password") : field.type === "number" ? "number" : "text";
  const textValue = String(value ?? "");
  const passwordCanReveal = field.type === "password" && textValue !== "";
  const placeholder = field.type === "password" && secretConfigured ? "已配置，留空表示不修改" : field.placeholder;
  return <label className="block space-y-1.5 text-xs" style={{ color: "var(--kvm-text-muted)" }}><span className="flex items-center gap-1">{field.label}{field.required && <span style={{ color: "#f87171" }}>*</span>}</span>{field.type === "textarea" ? <textarea value={textValue} disabled={disabled} onChange={(event) => onChange(event.target.value)} placeholder={field.placeholder} rows={3} className="w-full resize-y rounded-lg px-3 py-2 text-sm outline-none disabled:opacity-60" style={commonStyle} /> : <span className="relative block"><input value={textValue} disabled={disabled} onChange={(event) => onChange(field.type === "number" ? Number(event.target.value) : event.target.value)} placeholder={placeholder} type={inputType} inputMode={field.inputMode} className={`w-full rounded-lg px-3 py-2 text-sm outline-none disabled:opacity-60 ${field.type === "password" ? "pr-10" : ""} ${field.type === "number" ? "kvm-number-input" : ""}`} style={commonStyle} />{field.type === "password" && <KvmTooltip label={passwordVisible ? "隐藏密码" : "显示密码"} placement="top" align="center"><button type="button" disabled={disabled || !passwordCanReveal} className="kvm-action-button absolute right-2 top-1/2 flex h-7 w-7 -translate-y-1/2 items-center justify-center rounded-md disabled:cursor-not-allowed disabled:opacity-50" style={{ color: "var(--kvm-text-muted)", background: "transparent" }} onClick={(event) => { event.preventDefault(); setPasswordVisible((current) => !current); }} aria-label={passwordVisible ? "隐藏密码" : "显示密码"}>{passwordVisible ? <EyeOffIcon size={15} /> : <EyeIcon size={15} />}</button></KvmTooltip>}</span>}{field.helper && <span className="block text-[11px] leading-4" style={{ color: "var(--kvm-text-muted)" }}>{field.helper}</span>}</label>;
}

export function normalizeConfig(config: NotificationChannel["config"] | AuthProvider["config"] | undefined) {
  if (!config) return {};
  if (typeof config === "string") {
    try { return JSON.parse(config) as Record<string, unknown>; } catch { return {}; }
  }
  return config;
}

export function displayValue(field: Field, value: unknown) {
  if (field.key === "headers" && value && typeof value !== "string") return JSON.stringify(value, null, 2);
  if (field.key === "to" && Array.isArray(value)) return value.join(",");
  return value;
}

export function secretConfigured(field: Field, config: Record<string, unknown>) {
  if (field.type !== "password") return false;
  if (String(config[field.key] ?? "").trim() !== "") return true;
  return Boolean(config[secretPresenceKey(field.key)]);
}

export function removeEmptyConfigValues(config: Record<string, unknown>) {
  return Object.fromEntries(Object.entries(config).filter(([, value]) => {
    if (typeof value === "string") return value.trim() !== "";
    if (typeof value === "number") return Number.isFinite(value) && value > 0;
    if (Array.isArray(value)) return value.length > 0;
    if (value && typeof value === "object") return Object.keys(value).length > 0;
    return value !== undefined && value !== null;
  }));
}

export function removeSecretPresenceMarkers(config: Record<string, unknown>) {
  return Object.fromEntries(Object.entries(config).filter(([key]) => !/^has[A-Z]/.test(key)));
}

function secretPresenceKey(key: string) {
  return `has${key.charAt(0).toUpperCase()}${key.slice(1)}`;
}
