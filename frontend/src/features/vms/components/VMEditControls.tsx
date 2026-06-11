import { useEffect, useRef, useState } from 'react';
import { CheckIcon, ChevronDownIcon } from 'lucide-react';

export function NumberSelect({
  value,
  values,
  disabled,
  onChange,
}: {
  value: string;
  values: number[];
  disabled?: boolean;
  onChange: (value: string) => void;
}) {
  const [open, setOpen] = useState(false);
  const [placement, setPlacement] = useState<'bottom' | 'top'>('bottom');
  const rootRef = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    if (!open) return;
    const root = rootRef.current;
    if (!root) return;
    const rect = root.getBoundingClientRect();
    const optionHeight = 36;
    const menuHeight = Math.min(224, values.length * optionHeight + 8);
    const viewportGap = 16;
    const spaceBelow = window.innerHeight - rect.bottom;
    const spaceAbove = rect.top;
    setPlacement(
      spaceBelow < menuHeight + viewportGap && spaceAbove > spaceBelow ? 'top' : 'bottom'
    );

    const card = root.closest<HTMLElement>('.kvm-dialog-card');
    const previousZIndex = card?.style.zIndex;
    if (card) card.style.zIndex = '80';

    const close = (event: MouseEvent) => {
      if (!root.contains(event.target as Node)) setOpen(false);
    };
    window.addEventListener('mousedown', close);
    return () => {
      window.removeEventListener('mousedown', close);
      if (card) card.style.zIndex = previousZIndex ?? '';
    };
  }, [open, values.length]);

  return (
    <div ref={rootRef} className={open ? 'relative z-[90]' : 'relative'}>
      <button
        type="button"
        disabled={disabled}
        onClick={() => setOpen(value => !value)}
        className="kvm-action-button flex h-9 w-full cursor-pointer items-center justify-between gap-3 rounded-lg border px-3 text-left text-sm font-semibold disabled:cursor-not-allowed disabled:opacity-60"
        style={{
          background: 'var(--kvm-control-bg)',
          borderColor: open ? 'rgba(45,212,191,0.45)' : 'var(--kvm-border)',
          color: 'var(--kvm-text)',
          boxShadow: open
            ? '0 0 0 3px rgba(45,212,191,0.08)'
            : 'inset 0 1px 0 rgba(255,255,255,0.045)',
        }}
        aria-haspopup="listbox"
        aria-expanded={open}
      >
        <span className="truncate">{value}</span>
        <ChevronDownIcon
          size={15}
          className={
            open ? 'shrink-0 rotate-180 transition-transform' : 'shrink-0 transition-transform'
          }
          style={{ color: 'var(--kvm-text-muted)' }}
        />
      </button>
      {open && (
        <div
          className={
            (placement === 'top' ? 'bottom-[calc(100%+8px)] ' : 'top-[calc(100%+8px)] ') +
            'kvm-hidden-scrollbar absolute left-0 z-[90] max-h-56 w-full overflow-y-auto rounded-lg border p-1 shadow-2xl'
          }
          role="listbox"
          style={{
            background: 'var(--kvm-menu-bg)',
            borderColor: 'var(--kvm-popover-border)',
            boxShadow: 'var(--kvm-menu-shadow)',
          }}
        >
          {values.map(item => {
            const itemValue = String(item);
            const active = itemValue === value;
            return (
              <button
                key={itemValue}
                type="button"
                role="option"
                aria-selected={active}
                onClick={() => {
                  onChange(itemValue);
                  setOpen(false);
                }}
                className="group flex h-9 w-full cursor-pointer items-center justify-between gap-3 rounded-md px-3 text-left text-sm font-semibold transition-colors hover:bg-[rgba(45,212,191,0.1)]"
                style={{
                  background: active ? 'rgba(45,212,191,0.14)' : undefined,
                  color: active ? '#99f6e4' : 'var(--kvm-text)',
                }}
              >
                <span>{itemValue}</span>
                {active && <CheckIcon size={15} className="shrink-0" />}
              </button>
            );
          })}
        </div>
      )}
    </div>
  );
}

export function AllocationControl({
  value,
  custom,
  values,
  disabled,
  onValueChange,
  onCustomChange,
}: {
  value: string;
  custom: boolean;
  values: number[];
  disabled?: boolean;
  onValueChange: (value: string) => void;
  onCustomChange: (value: boolean) => void;
}) {
  return (
    <div className="space-y-2">
      {custom ? (
        <input
          value={value}
          inputMode="numeric"
          disabled={disabled}
          onChange={event => onValueChange(event.target.value)}
          className={inputClass}
          style={fieldStyle}
        />
      ) : (
        <NumberSelect value={value} values={values} disabled={disabled} onChange={onValueChange} />
      )}
      <CheckToggle
        checked={custom}
        disabled={disabled}
        onChange={onCustomChange}
        label="自定义值"
      />
    </div>
  );
}

export function CheckToggle({
  checked,
  disabled,
  onChange,
  label,
}: {
  checked: boolean;
  disabled?: boolean;
  onChange: (value: boolean) => void;
  label: string;
}) {
  return (
    <label
      className={
        'kvm-action-button kvm-check-toggle inline-flex items-center gap-2 rounded-lg border px-2.5 py-1.5 text-xs font-semibold ' +
        (disabled ? 'cursor-not-allowed opacity-60' : 'cursor-pointer')
      }
      style={{
        background: checked ? 'rgba(45,212,191,0.12)' : 'var(--kvm-control-bg-soft)',
        borderColor: checked ? 'rgba(45,212,191,0.34)' : 'var(--kvm-border)',
        color: checked ? 'var(--kvm-check-toggle-active-text)' : 'var(--kvm-text-muted)',
      }}
    >
      <input
        type="checkbox"
        checked={checked}
        disabled={disabled}
        onChange={event => onChange(event.target.checked)}
        className="h-4 w-4 cursor-pointer accent-cyan-400 disabled:cursor-not-allowed"
      />
      {label}
    </label>
  );
}

export function PrimaryButton({
  label,
  compact,
  disabled,
  title,
  onClick,
}: {
  label: string;
  compact?: boolean;
  disabled?: boolean;
  title?: string;
  onClick?: () => void;
}) {
  return (
    <button
      type="button"
      disabled={disabled}
      title={title}
      onClick={onClick}
      className={
        (compact ? 'h-9 px-4 text-sm' : 'h-10 px-5 text-sm') +
        ' kvm-action-button rounded-lg border font-semibold disabled:opacity-60'
      }
      style={{
        background: 'rgba(59,130,246,0.15)',
        borderColor: 'rgba(59,130,246,0.42)',
        color: 'var(--kvm-accent-text)',
        boxShadow: '0 10px 20px rgba(37,99,235,0.14), inset 0 1px 0 rgba(255,255,255,0.1)',
      }}
    >
      {label}
    </button>
  );
}

const inputClass = 'h-9 w-full rounded-lg px-3 text-sm outline-none';
const fieldStyle = {
  background: 'var(--kvm-control-bg)',
  border: '1px solid var(--kvm-border)',
  color: 'var(--kvm-text)',
  boxShadow: 'inset 0 1px 0 rgba(255,255,255,0.045)',
};
