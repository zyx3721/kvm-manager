import { type ReactNode } from 'react';

import { type CpuIcon } from 'lucide-react';

export function SummaryCard({
  icon: Icon,
  label,
  value,
  color,
}: {
  icon: typeof CpuIcon;
  label: string;
  value: string;
  color: string;
}) {
  return (
    <div className="kvm-dialog-card kvm-card-hover rounded-xl p-3">
      <div className="flex items-center gap-2 text-xs" style={{ color: 'var(--kvm-text-muted)' }}>
        <Icon size={14} style={{ color }} />
        {label}
      </div>
      <div className="mt-2 truncate text-sm font-semibold" style={{ color: 'var(--kvm-text)' }}>
        {value}
      </div>
    </div>
  );
}

export function CardSection({ title, children }: { title: string; children: ReactNode }) {
  return (
    <section className="kvm-dialog-card kvm-card-hover rounded-xl p-4">
      <h3 className="mb-4 text-sm font-semibold" style={{ color: 'var(--kvm-text)' }}>
        {title}
      </h3>
      {children}
    </section>
  );
}

export function FormGrid({
  children,
  align = 'center',
}: {
  children: ReactNode;
  align?: 'center' | 'start';
}) {
  return (
    <div
      className={
        (align === 'start' ? 'md:items-start ' : 'md:items-center ') +
        'grid gap-3 md:grid-cols-[150px_1fr]'
      }
    >
      {children}
    </div>
  );
}

export function FieldText({
  children,
  className = '',
}: {
  children: ReactNode;
  className?: string;
}) {
  return (
    <div className={'text-right text-sm ' + className} style={{ color: 'var(--kvm-text)' }}>
      {children}
    </div>
  );
}

export function InlineNotice({
  children,
  tone,
}: {
  children: ReactNode;
  tone: 'info' | 'warning';
}) {
  const warning = tone === 'warning';
  return (
    <div
      className="mb-3 rounded-lg border px-3 py-2 text-xs"
      style={{
        background: warning ? 'rgba(245,158,11,0.08)' : 'rgba(59,130,246,0.08)',
        borderColor: warning ? 'rgba(245,158,11,0.28)' : 'rgba(59,130,246,0.26)',
        color: warning ? '#fbbf24' : 'var(--kvm-accent-text)',
      }}
    >
      {children}
    </div>
  );
}

export function InfoRows({ rows }: { rows: Array<[string, string]> }) {
  return (
    <div className="space-y-2.5">
      {rows.map(([label, value]) => (
        <div
          key={`${label}-${value}`}
          className="grid gap-3 rounded-lg px-3 py-2.5 md:grid-cols-[180px_1fr]"
          style={{
            background: 'var(--kvm-control-bg-soft)',
            border: '1px solid var(--kvm-border)',
          }}
        >
          <div className="text-right text-sm" style={{ color: 'var(--kvm-text)' }}>
            {label}
          </div>
          <div
            className="min-w-0 break-all font-mono text-sm"
            style={{ color: 'var(--kvm-text-muted)' }}
          >
            {value}
          </div>
        </div>
      ))}
    </div>
  );
}

export const inputClass = 'h-9 w-full rounded-lg px-3 text-sm outline-none';

export const fieldStyle = {
  background: 'var(--kvm-control-bg)',
  border: '1px solid var(--kvm-border)',
  color: 'var(--kvm-text)',
  boxShadow: 'inset 0 1px 0 rgba(255,255,255,0.045)',
};
