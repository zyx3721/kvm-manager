import { type ReactNode, useRef, useState } from 'react';
import { createPortal } from 'react-dom';
import { cn } from '@/lib/utils';
import { metricUsageColor } from '@/lib/resourceThresholds';
import { useBaseConfig } from '@/lib/branding';
import type { SystemBaseConfig } from '@/lib/api';

type Status =
  | 'running'
  | 'stopped'
  | 'paused'
  | 'error'
  | 'online'
  | 'offline'
  | 'maintenance'
  | 'degraded'
  | 'unknown'
  | 'ready'
  | 'creating'
  | 'failed';

const statusMap: Record<Status, { label: string; className: string; dot: string }> = {
  running: { label: '运行中', className: 'kvm-badge-running', dot: '#10b981' },
  online: { label: '在线', className: 'kvm-badge-running', dot: '#10b981' },
  ready: { label: '就绪', className: 'kvm-badge-running', dot: '#10b981' },
  stopped: { label: '已停止', className: 'kvm-badge-stopped', dot: '#94a3b8' },
  offline: { label: '离线', className: 'kvm-badge-stopped', dot: '#94a3b8' },
  unknown: { label: '未知', className: 'kvm-badge-stopped', dot: '#94a3b8' },
  paused: { label: '已暂停', className: 'kvm-badge-paused', dot: '#f59e0b' },
  maintenance: { label: '维护中', className: 'kvm-badge-paused', dot: '#f59e0b' },
  degraded: { label: '降级', className: 'kvm-badge-paused', dot: '#f59e0b' },
  creating: { label: '创建中', className: 'kvm-badge-paused', dot: '#f59e0b' },
  error: { label: '异常', className: 'kvm-badge-error', dot: '#ef4444' },
  failed: { label: '失败', className: 'kvm-badge-error', dot: '#ef4444' },
};

export function StatusBadge({ status }: { status: string }) {
  const item = statusMap[(status as Status) in statusMap ? (status as Status) : 'unknown'];
  return (
    <span
      className={cn(
        'inline-flex shrink-0 items-center gap-1.5 whitespace-nowrap rounded-full px-2 py-0.5 text-xs',
        item.className
      )}
    >
      <span className="inline-block h-1.5 w-1.5 rounded-full" style={{ background: item.dot }} />
      {item.label}
    </span>
  );
}

type MetricBarThresholdConfig = Pick<SystemBaseConfig, 'resourceWarningThreshold' | 'resourceCriticalThreshold'>;

export function MetricBar({
  value,
  label,
  color,
  thresholdConfig,
}: {
  value: number;
  label?: string;
  color?: string;
  thresholdConfig?: MetricBarThresholdConfig;
}) {
  const baseConfig = useBaseConfig();
  const barColor = color ?? metricUsageColor(value, thresholdConfig ?? baseConfig);
  return (
    <div className="w-full space-y-1">
      {label && (
        <div className="flex items-center justify-between text-xs">
          <span style={{ color: 'var(--kvm-text-muted)' }}>{label}</span>
          <span className="font-mono tabular-nums" style={{ color: 'var(--kvm-text)' }}>
            {value}%
          </span>
        </div>
      )}
      <div
        className="h-1.5 w-full overflow-hidden rounded-full"
        style={{ background: 'rgba(56,78,120,0.35)' }}
      >
        <div
          className="h-full rounded-full transition-all"
          style={{ width: `${Math.min(100, Math.max(0, value))}%`, background: barColor }}
        />
      </div>
    </div>
  );
}

type TooltipPlacement = 'top' | 'bottom' | 'left' | 'right';
type TooltipAlign = 'start' | 'center' | 'end';

type KvmTooltipProps = {
  label?: ReactNode;
  children: ReactNode;
  className?: string;
  placement?: TooltipPlacement;
  align?: TooltipAlign;
  multiline?: boolean;
  disabled?: boolean;
  portalRoot?: HTMLElement | null;
  zIndex?: number;
};

export function KvmTooltip({
  label,
  children,
  className,
  placement,
  align = 'center',
  multiline,
  disabled,
  portalRoot,
  zIndex,
}: KvmTooltipProps) {
  const triggerRef = useRef<HTMLSpanElement | null>(null);
  const [tooltip, setTooltip] = useState<{
    open: boolean;
    top: number;
    left: number;
    placement: TooltipPlacement;
    align: TooltipAlign;
  }>({
    open: false,
    top: 0,
    left: 0,
    placement: 'top',
    align: 'center',
  });

  const active = !disabled && Boolean(label);

  function showTooltip() {
    if (!active) return;
    const trigger = triggerRef.current;
    if (!trigger) return;

    const rect = trigger.getBoundingClientRect();
    const gap = 8;
    const estimatedHeight = multiline ? 84 : 42;
    const topSpace = rect.top;
    const bottomSpace = window.innerHeight - rect.bottom;
    const nextPlacement =
      placement ?? (topSpace < estimatedHeight + gap && bottomSpace > topSpace ? 'bottom' : 'top');
    const minLeft = multiline ? 180 : 72;
    const maxLeft = window.innerWidth - minLeft;
    const centeredLeft = Math.min(maxLeft, Math.max(minLeft, rect.left + rect.width / 2));
    const edgeLeft = align === 'start' ? rect.left : align === 'end' ? rect.right : centeredLeft;
    const sideTop = rect.top + rect.height / 2;
    const left =
      nextPlacement === 'right'
        ? rect.right + gap
        : nextPlacement === 'left'
          ? rect.left - gap
          : edgeLeft;
    const top =
      nextPlacement === 'top'
        ? rect.top - gap
        : nextPlacement === 'bottom'
          ? rect.bottom + gap
          : sideTop;

    setTooltip({ open: true, top, left, placement: nextPlacement, align });
  }

  function hideTooltip() {
    setTooltip(current => ({ ...current, open: false }));
  }

  const horizontalTransform =
    tooltip.align === 'start' ? '0' : tooltip.align === 'end' ? '-100%' : '-50%';
  const transform =
    tooltip.placement === 'top'
      ? `translate(${horizontalTransform}, -100%)`
      : tooltip.placement === 'bottom'
        ? `translate(${horizontalTransform}, 0)`
        : tooltip.placement === 'right'
          ? 'translate(0, -50%)'
          : 'translate(-100%, -50%)';

  const bubble = active && tooltip.open && (
    <span
      className="pointer-events-none fixed text-left shadow-2xl"
      style={{ left: tooltip.left, top: tooltip.top, transform, zIndex: zIndex ?? 1000 }}
    >
      <span
        className={
          (multiline ? 'block max-w-[420px] whitespace-pre-wrap leading-5 ' : 'whitespace-nowrap ') +
          'rounded-lg border px-2.5 py-1.5 text-xs font-semibold'
        }
        style={{
          background: 'var(--kvm-popover-bg)',
          borderColor: 'var(--kvm-popover-border)',
          color: 'var(--kvm-text)',
        }}
      >
        {label}
      </span>
    </span>
  );

  const target = portalRoot ?? (typeof document === 'undefined' ? null : document.body);

  return (
    <span
      ref={triggerRef}
      className={className}
      onMouseEnter={showTooltip}
      onMouseLeave={hideTooltip}
      onFocus={showTooltip}
      onBlur={hideTooltip}
    >
      {children}
      {bubble && target ? createPortal(bubble, target) : null}
    </span>
  );
}
