import React, { useRef, useState } from 'react';
import { createPortal } from 'react-dom';

export function ActionButton({
  children,
  label,
  disabled,
  danger,
  variant,
  onClick,
}: {
  children: React.ReactNode;
  label: string;
  disabled?: boolean;
  danger?: boolean;
  variant?: 'console' | 'clone';
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
    const scrollBox = trigger.closest('[data-vm-table-scroll]')?.getBoundingClientRect();
    const topLimit = scrollBox?.top ?? 0;
    const bottomLimit = scrollBox?.bottom ?? window.innerHeight;
    const estimatedHeight = 42;
    const gap = 8;
    const topSpace = rect.top - topLimit;
    const bottomSpace = bottomLimit - rect.bottom;
    const placement = topSpace < estimatedHeight + gap && bottomSpace > topSpace ? 'bottom' : 'top';
    const left = Math.min(window.innerWidth - 72, Math.max(72, rect.left + rect.width / 2));
    const top = placement === 'top' ? rect.top - gap : rect.bottom + gap;
    setTooltip({ open: true, top, left, placement });
  }

  const bubble = tooltip.open && (
    <div
      className="pointer-events-none fixed z-50 -translate-x-1/2 text-left shadow-2xl"
      style={{
        left: tooltip.left,
        top: tooltip.top,
        transform: tooltip.placement === 'top' ? 'translate(-50%, -100%)' : 'translate(-50%, 0)',
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
    </div>
  );

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
        className={
          'kvm-action-button ' +
          (danger ? 'kvm-danger-button ' : '') +
          'flex h-7 w-7 items-center justify-center rounded-lg border transition-colors disabled:cursor-not-allowed disabled:opacity-35'
        }
        style={{
          borderColor:
            variant === 'console'
              ? 'rgba(99,102,241,0.75)'
              : variant === 'clone'
                ? 'rgba(34,197,94,0.62)'
                : danger
                  ? 'rgba(239,68,68,0.52)'
                  : 'rgba(76,103,150,0.22)',
          color:
            variant === 'console'
              ? '#818cf8'
              : variant === 'clone'
                ? 'var(--kvm-status-green-text)'
                : danger
                  ? '#ef4444'
                  : 'var(--kvm-text-muted)',
          background:
            variant === 'console'
              ? 'rgba(79,70,229,0.12)'
              : variant === 'clone'
                ? 'rgba(34,197,94,0.1)'
                : danger
                  ? 'rgba(239,68,68,0.08)'
                  : 'rgba(255,255,255,0.025)',
        }}
      >
        {children}
      </button>
      {bubble ? createPortal(bubble, document.body) : null}
    </>
  );
}
