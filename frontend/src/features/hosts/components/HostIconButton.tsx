import { type ReactNode, useRef, useState } from 'react';
import { createPortal } from 'react-dom';

export function HostIconButton({
  children,
  label,
  disabled,
  danger,
  variant = 'default',
  tooltipPlacement,
  onClick,
}: {
  children: ReactNode;
  label: string;
  disabled?: boolean;
  danger?: boolean;
  variant?: 'default' | 'test' | 'sync' | 'monitor';
  tooltipPlacement?: 'top' | 'bottom';
  onClick?: () => void;
}) {
  const triggerRef = useRef<HTMLButtonElement | null>(null);
  const [tooltip, setTooltip] = useState<{
    open: boolean;
    top: number;
    left: number;
    placement: 'top' | 'bottom';
  }>({ open: false, top: 0, left: 0, placement: 'top' });

  function showTooltip() {
    const trigger = triggerRef.current;
    if (!trigger) return;
    const rect = trigger.getBoundingClientRect();
    const estimatedHeight = 42;
    const gap = 8;
    const topSpace = rect.top;
    const bottomSpace = window.innerHeight - rect.bottom;
    const placement =
      tooltipPlacement ??
      (topSpace < estimatedHeight + gap && bottomSpace > topSpace ? 'bottom' : 'top');
    const left = Math.min(window.innerWidth - 72, Math.max(72, rect.left + rect.width / 2));
    const top = placement === 'top' ? rect.top - gap : rect.bottom + gap;
    setTooltip({ open: true, top, left, placement });
  }

  const color = danger
    ? '#ef4444'
    : variant === 'sync'
      ? '#10b981'
      : variant === 'test' || variant === 'monitor'
        ? 'var(--kvm-accent-text)'
        : 'var(--kvm-text-muted)';
  const borderColor = danger
    ? 'rgba(239,68,68,0.52)'
    : variant === 'sync'
      ? 'rgba(16,185,129,0.35)'
      : variant === 'test' || variant === 'monitor'
        ? 'rgba(59,130,246,0.35)'
        : 'rgba(76,103,150,0.22)';
  const background = danger
    ? 'rgba(239,68,68,0.08)'
    : variant === 'sync'
      ? 'rgba(16,185,129,0.06)'
      : variant === 'test' || variant === 'monitor'
        ? 'rgba(59,130,246,0.08)'
        : 'rgba(255,255,255,0.025)';

  return (
    <>
      <button
        ref={triggerRef}
        type="button"
        aria-label={label}
        disabled={disabled}
        onMouseEnter={showTooltip}
        onMouseLeave={() => setTooltip(current => ({ ...current, open: false }))}
        onFocus={showTooltip}
        onBlur={() => setTooltip(current => ({ ...current, open: false }))}
        onClick={onClick}
        className={`kvm-action-button ${danger ? 'kvm-danger-button ' : ''}flex h-7 w-7 items-center justify-center rounded-md border transition-colors disabled:cursor-not-allowed disabled:opacity-35`}
        style={{ borderColor, color, background }}
      >
        {children}
      </button>
      {tooltip.open &&
        createPortal(
          <div
            className="pointer-events-none fixed z-[1000] -translate-x-1/2 text-left shadow-2xl"
            style={{
              left: tooltip.left,
              top: tooltip.top,
              transform:
                tooltip.placement === 'top' ? 'translate(-50%, -100%)' : 'translate(-50%, 0)',
            }}
          >
            <div
              className="whitespace-nowrap rounded-lg border px-2.5 py-1.5 text-xs font-semibold"
              style={{
                background: 'var(--kvm-popover-bg)',
                borderColor: danger ? 'rgba(239, 68, 68, 0.46)' : 'var(--kvm-popover-border)',
                color: danger ? '#fca5a5' : 'var(--kvm-text)',
              }}
            >
              {label}
            </div>
          </div>,
          document.body
        )}
    </>
  );
}
