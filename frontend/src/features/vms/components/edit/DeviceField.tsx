import type { ReactNode } from 'react';

export function DeviceField({ label, children }: { label: string; children: ReactNode }) {
  return (
    <label className="block min-w-0 space-y-1">
      <span
        className="block px-1 text-[11px] font-semibold"
        style={{ color: 'var(--kvm-text-muted)' }}
      >
        {label}
      </span>
      {children}
    </label>
  );
}
