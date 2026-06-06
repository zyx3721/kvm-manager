import { useEffect, useMemo, useRef, useState, type ReactNode } from 'react';
import { createPortal } from 'react-dom';
import { CheckIcon, ChevronDownIcon } from 'lucide-react';

import { KvmTooltip } from './StatusBadge';

export type SelectMenuOption = {
  value: string;
  label: ReactNode;
  searchLabel?: string;
  tooltip?: ReactNode;
  disabled?: boolean;
};

export function SelectMenu({
  value,
  options,
  placeholder,
  disabled,
  className = '',
  buttonClassName = '',
  menuClassName = '',
  optionClassName = '',
  menuZIndex = 1200,
  maxVisibleItems,
  placement: preferredPlacement = 'auto',
  optionTooltipPlacement = 'left',
  onChange,
}: {
  value: string;
  options: SelectMenuOption[];
  placeholder: string;
  disabled?: boolean;
  className?: string;
  buttonClassName?: string;
  menuClassName?: string;
  optionClassName?: string;
  menuZIndex?: number;
  maxVisibleItems?: number;
  placement?: 'auto' | 'bottom' | 'top';
  optionTooltipPlacement?: 'top' | 'bottom' | 'left' | 'right';
  onChange: (value: string) => void;
}) {
  const [open, setOpen] = useState(false);
  const [menuRect, setMenuRect] = useState<{
    top: number;
    left: number;
    width: number;
    placement: 'bottom' | 'top';
  }>({ top: 0, left: 0, width: 0, placement: 'bottom' });
  const rootRef = useRef<HTMLDivElement | null>(null);
  const menuRef = useRef<HTMLDivElement | null>(null);
  const selected = useMemo(() => options.find(item => item.value === value), [options, value]);
  const closed = disabled || options.length === 0;

  useEffect(() => {
    if (!open) return;
    const root = rootRef.current;
    if (!root) return;

    const updateMenuRect = () => {
      const rect = root.getBoundingClientRect();
      const menuHeight = Math.min(maxVisibleItems ? maxVisibleItems * 40 + 8 : 256, options.length * 40 + 8);
      const spaceBelow = window.innerHeight - rect.bottom;
      const spaceAbove = rect.top;
      const nextPlacement =
        preferredPlacement === 'auto'
          ? spaceBelow < menuHeight + 16 && spaceAbove > spaceBelow
            ? 'top'
            : 'bottom'
          : preferredPlacement;
      setMenuRect({
        top: nextPlacement === 'top' ? Math.max(8, rect.top - menuHeight - 8) : rect.bottom + 8,
        left: rect.left,
        width: rect.width,
        placement: nextPlacement,
      });
    };

    updateMenuRect();
    const close = (event: MouseEvent) => {
      const target = event.target as Node;
      if (!root.contains(target) && !menuRef.current?.contains(target)) setOpen(false);
    };
    window.addEventListener('resize', updateMenuRect);
    window.addEventListener('scroll', updateMenuRect, true);
    window.addEventListener('mousedown', close);
    return () => {
      window.removeEventListener('resize', updateMenuRect);
      window.removeEventListener('scroll', updateMenuRect, true);
      window.removeEventListener('mousedown', close);
    };
  }, [maxVisibleItems, open, options.length, preferredPlacement]);

  return (
    <div ref={rootRef} className={'relative min-w-0 ' + className}>
      <button
        type="button"
        disabled={closed}
        onClick={() => setOpen(next => !next)}
        className={
          'kvm-action-button flex h-10 w-full cursor-pointer items-center justify-between gap-3 rounded-lg border px-3 text-left text-sm font-semibold transition disabled:cursor-not-allowed disabled:opacity-60 ' +
          buttonClassName
        }
        style={{
          background: 'var(--kvm-control-bg)',
          borderColor: open ? 'rgba(45,212,191,0.45)' : 'var(--kvm-border)',
          color: selected ? 'var(--kvm-text)' : 'var(--kvm-text-muted)',
          boxShadow: open
            ? '0 0 0 3px rgba(45,212,191,0.08)'
            : 'inset 0 1px 0 rgba(255,255,255,0.045)',
        }}
        aria-haspopup="listbox"
        aria-expanded={open}
      >
        <span className="min-w-0 flex-1 truncate">{selected?.label ?? placeholder}</span>
        <ChevronDownIcon
          size={15}
          className={
            open ? 'shrink-0 rotate-180 transition-transform' : 'shrink-0 transition-transform'
          }
          style={{ color: 'var(--kvm-text-muted)' }}
        />
      </button>
      {open && !closed && typeof document !== 'undefined' ? createPortal(
        <div
          ref={menuRef}
          className={
            'kvm-hidden-scrollbar fixed max-h-64 overflow-y-auto rounded-lg border p-1 shadow-2xl ' +
            menuClassName
          }
          role="listbox"
          style={{
            background: 'var(--kvm-menu-bg)',
            borderColor: 'var(--kvm-popover-border)',
            boxShadow: 'var(--kvm-menu-shadow)',
            left: menuRect.left,
            top: menuRect.top,
            width: menuRect.width,
            zIndex: menuZIndex,
            maxHeight: maxVisibleItems ? `${maxVisibleItems * 40 + 8}px` : undefined,
          }}
        >
          {options.map(item => {
            const active = item.value === value;
            return (
              <button
                key={item.value}
                type="button"
                role="option"
                aria-selected={active}
                disabled={item.disabled}
                onClick={() => {
                  onChange(item.value);
                  setOpen(false);
                }}
                className={
                  'group flex h-10 w-full cursor-pointer items-center justify-between gap-3 rounded-md px-3 text-left text-sm font-semibold transition-colors hover:bg-[rgba(45,212,191,0.1)] disabled:cursor-not-allowed disabled:opacity-50 ' +
                  optionClassName
                }
                style={{
                  background: active ? 'rgba(45,212,191,0.14)' : undefined,
                  color: active ? '#99f6e4' : 'var(--kvm-text)',
                }}
              >
                <KvmTooltip label={item.tooltip} placement={optionTooltipPlacement} align="center" disabled={!item.tooltip} zIndex={menuZIndex + 1} className="min-w-0 flex-1">
                  <span className="block min-w-0 truncate">{item.label}</span>
                </KvmTooltip>
                {active && <CheckIcon size={15} className="shrink-0" />}
              </button>
            );
          })}
        </div>,
        document.body
      ) : null}
    </div>
  );
}
