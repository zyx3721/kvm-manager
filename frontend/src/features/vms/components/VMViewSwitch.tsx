import { BoxesIcon, Layers3Icon } from 'lucide-react';
import type React from 'react';

export type VMViewMode = 'vms' | 'templates';

export function VMViewSwitch({
  value,
  vmCount,
  templateCount,
  onChange,
}: {
  value: VMViewMode;
  vmCount: number;
  templateCount: number;
  onChange: (value: VMViewMode) => void;
}) {
  return (
    <div
      className="inline-flex h-10 items-center rounded-xl border p-1"
      style={{ background: 'var(--kvm-card)', borderColor: 'var(--kvm-border)' }}
    >
      <SwitchButton
        active={value === 'vms'}
        label="虚拟机"
        count={vmCount}
        icon={<BoxesIcon size={14} />}
        onClick={() => onChange('vms')}
      />
      <SwitchButton
        active={value === 'templates'}
        label="模板"
        count={templateCount}
        icon={<Layers3Icon size={14} />}
        onClick={() => onChange('templates')}
      />
    </div>
  );
}

function SwitchButton({
  active,
  label,
  count,
  icon,
  onClick,
}: {
  active: boolean;
  label: string;
  count: number;
  icon: React.ReactNode;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className="kvm-action-button inline-flex h-8 min-w-24 items-center justify-center gap-2 rounded-lg px-3 text-xs font-semibold"
      style={{
        background: active ? 'rgba(59,130,246,0.15)' : 'transparent',
        border: active ? '1px solid rgba(59,130,246,0.34)' : '1px solid transparent',
        color: active ? 'var(--kvm-accent-text)' : 'var(--kvm-text-muted)',
      }}
      aria-pressed={active}
    >
      {icon}
      <span>{label}</span>
      <span className="font-mono">{count}</span>
    </button>
  );
}
