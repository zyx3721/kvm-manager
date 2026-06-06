import { EyeIcon, EyeOffIcon } from 'lucide-react';

import { fieldStyle, inputClass } from '../edit/EditShared';
import { CheckToggle, PrimaryButton } from '../VMEditControls';

export { fieldStyle, inputClass, CheckToggle, PrimaryButton };

export function Panel({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <section
      className="rounded-xl border p-4"
      style={{ background: 'var(--kvm-control-bg-soft)', borderColor: 'var(--kvm-border)' }}
    >
      <h3 className="mb-3 text-sm font-semibold" style={{ color: 'var(--kvm-text)' }}>
        {title}
      </h3>
      {children}
    </section>
  );
}

export function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <label className="min-w-0">
      <div className="mb-1.5 text-xs font-semibold" style={{ color: 'var(--kvm-text-muted)' }}>
        {label}
      </div>
      {children}
    </label>
  );
}

export function NumberField({
  label,
  value,
  onChange,
}: {
  label: string;
  value: number;
  onChange: (value: number) => void;
}) {
  return (
    <Field label={label}>
      <input
        type="number"
        min={1}
        value={value}
        onChange={event => onChange(Number(event.target.value))}
        className={inputClass}
        style={fieldStyle}
      />
    </Field>
  );
}

export function PasswordInput({
  value,
  disabled,
  visible,
  onVisibleChange,
  onChange,
}: {
  value: string;
  disabled?: boolean;
  visible: boolean;
  onVisibleChange: (visible: boolean) => void;
  onChange: (value: string) => void;
}) {
  const Icon = visible ? EyeOffIcon : EyeIcon;
  return (
    <div className="relative">
      <input
        value={value}
        disabled={disabled}
        type={visible ? 'text' : 'password'}
        autoComplete="new-password"
        onChange={event => onChange(event.target.value)}
        className={inputClass + ' pr-10'}
        style={fieldStyle}
      />
      <button
        type="button"
        disabled={disabled}
        aria-label={visible ? '隐藏控制台密码' : '显示控制台密码'}
        onClick={() => onVisibleChange(!visible)}
        className="kvm-action-button absolute right-1.5 top-1/2 flex h-6 w-6 -translate-y-1/2 items-center justify-center rounded-md disabled:cursor-not-allowed disabled:opacity-50"
        style={{ color: 'var(--kvm-text-muted)' }}
      >
        <Icon size={15} />
      </button>
    </div>
  );
}

export function Toggle({
  checked,
  disabled,
  label,
  onChange,
}: {
  checked: boolean;
  disabled?: boolean;
  label: string;
  onChange: (value: boolean) => void;
}) {
  return (
    <button
      type="button"
      disabled={disabled}
      onClick={() => onChange(!checked)}
      className="kvm-action-button h-9 rounded-lg border px-3 text-sm font-semibold disabled:opacity-60"
      style={{
        background: checked ? 'rgba(45,212,191,0.12)' : 'var(--kvm-control-bg)',
        borderColor: checked ? 'rgba(45,212,191,0.34)' : 'var(--kvm-border)',
        color: checked ? 'var(--kvm-check-toggle-active-text)' : 'var(--kvm-text-muted)',
      }}
    >
      {label}
    </button>
  );
}

export function MetadataToggleRow({
  format,
  checked,
  disabled,
  onChange,
}: {
  format: string;
  checked: boolean;
  disabled?: boolean;
  onChange: (value: boolean) => void;
}) {
  return (
    <div className="mt-3 flex min-h-9 flex-wrap items-center gap-3">
      {format === 'qcow2' ? (
        <CheckToggle
          checked={checked}
          disabled={disabled}
          onChange={onChange}
          label="预分配 Metadata"
        />
      ) : (
        <span className="text-xs" style={{ color: 'var(--kvm-text-muted)' }}>
          非 qcow2 格式的卷名称统一使用 .img 扩展名
        </span>
      )}
    </div>
  );
}
