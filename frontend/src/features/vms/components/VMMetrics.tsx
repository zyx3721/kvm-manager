import { useRef, useState } from 'react';
import { createPortal } from 'react-dom';
import type { VirtualMachine } from '../../../lib/api';
import { formatBytes } from '../../../lib/format';
import { MetricBar } from '../../../components/kvm/StatusBadge';

export function VMMetricCell({
  value,
  available,
  prefix,
}: {
  value: number;
  available?: boolean;
  prefix: string;
}) {
  const normalizedValue = Math.min(100, Math.max(0, value || 0));
  const display = available === false && normalizedValue !== 0 ? '-' : `${normalizedValue}%`;
  return (
    <div className="mx-auto max-w-40 space-y-1.5 text-center">
      <div className="flex items-center justify-center gap-1.5 text-xs">
        <span className="font-semibold" style={{ color: 'var(--kvm-text)' }}>
          {prefix}
        </span>
        <span
          className="font-mono tabular-nums"
          style={{ color: available === false ? 'var(--kvm-text-muted)' : 'var(--kvm-text)' }}
        >
          {display}
        </span>
      </div>
      <MetricBar value={normalizedValue} />
    </div>
  );
}

export function DiskMetricCell({ vm }: { vm: VirtualMachine }) {
  const triggerRef = useRef<HTMLDivElement | null>(null);
  const [tooltip, setTooltip] = useState<{
    open: boolean;
    top: number;
    left: number;
    placement: 'left' | 'right';
  }>({ open: false, top: 0, left: 0, placement: 'right' });
  const disks =
    vm.disks && vm.disks.length > 0
      ? vm.disks
      : [{ name: 'disk', usedBytes: vm.diskUsedBytes, bytes: vm.diskBytes, path: '' }];

  function showTooltip() {
    const trigger = triggerRef.current;
    if (!trigger) return;
    const rect = trigger.getBoundingClientRect();
    const tooltipWidth = 256;
    const estimatedHeight = 74 + disks.length * 46;
    const gap = 10;
    const viewportGap = 12;
    const placement = rect.right + gap + tooltipWidth <= window.innerWidth - viewportGap
      ? 'right'
      : 'left';
    const rawLeft = placement === 'right' ? rect.right + gap : rect.left - gap;
    const rawTop = rect.top + rect.height / 2;
    const top = Math.min(
      window.innerHeight - viewportGap - estimatedHeight / 2,
      Math.max(viewportGap + estimatedHeight / 2, rawTop)
    );
    const left = placement === 'right'
      ? Math.min(window.innerWidth - viewportGap, rawLeft)
      : Math.max(viewportGap, rawLeft);
    setTooltip({ open: true, top, left, placement });
  }

  const tooltipNode = tooltip.open ? (
    <div
      className="kvm-disk-tooltip pointer-events-none fixed z-50 w-64 text-left opacity-100 shadow-2xl"
      style={{
        left: tooltip.left,
        top: tooltip.top,
        transform:
          tooltip.placement === 'right' ? 'translate(0, -50%)' : 'translate(-100%, -50%)',
      }}
    >
      <div
        className="rounded-lg border p-3"
        style={{
          background: 'var(--kvm-popover-bg)',
          borderColor: 'var(--kvm-popover-border)',
          color: 'var(--kvm-text)',
        }}
      >
        <div className="mb-2 flex items-center justify-between gap-3 text-xs">
          <span className="font-semibold">磁盘明细</span>
          <span className="font-mono" style={{ color: 'var(--kvm-text-muted)' }}>
            {formatBytes(vm.diskUsedBytes, 'GB')} / {formatBytes(vm.diskBytes, 'GB')}
          </span>
        </div>
        <div className="space-y-2">
          {disks.map(disk => (
            <div
              key={`${disk.name}-${disk.path}`}
              className="rounded-md px-2 py-1.5"
              style={{
                background: 'rgba(255,255,255,0.04)',
                border: '1px solid rgba(76,103,150,0.24)',
              }}
            >
              <div className="flex items-center justify-between gap-2 text-xs">
                <span className="font-mono" style={{ color: '#93c5fd' }}>
                  {disk.name || 'disk'}
                </span>
                <span className="font-mono" style={{ color: 'var(--kvm-text-muted)' }}>
                  {formatBytes(disk.usedBytes, 'GB')} / {formatBytes(disk.bytes, 'GB')}
                </span>
              </div>
            </div>
          ))}
        </div>
      </div>
    </div>
  ) : null;

  return (
    <div
      ref={triggerRef}
      onMouseEnter={showTooltip}
      onMouseLeave={() => setTooltip(current => ({ ...current, open: false }))}
      className="kvm-disk-metric mx-auto max-w-40 space-y-1.5 text-center"
    >
      <VMMetricCell
        value={vm.diskUsage}
        available={vm.diskUsageAvailable}
        prefix={formatBytes(vm.diskBytes, 'GB')}
      />
      {tooltipNode ? createPortal(tooltipNode, document.body) : null}
    </div>
  );
}
