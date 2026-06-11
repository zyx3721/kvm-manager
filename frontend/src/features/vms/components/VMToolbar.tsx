import { useEffect, useRef, useState } from 'react';
import { createPortal } from 'react-dom';
import {
  CheckIcon,
  ChevronDownIcon,
  Columns3Icon,
  DownloadIcon,
  FilterIcon,
  SearchIcon,
} from 'lucide-react';

import { SelectMenu } from '../../../components/kvm/SelectMenu';
import { VMCreateButton } from './VMCreateButton';
import type { VMTableColumnKey } from './VMTable';

export type VMFilterCounts = {
  all: number;
  running: number;
  stopped: number;
  paused: number;
  error: number;
};

export type VMFilterOption = {
  key: keyof VMFilterCounts;
  label: string;
};

export type VMColumnOption = {
  key: VMTableColumnKey;
  label: string;
};

export function VMToolbar({
  search,
  filter,
  hostFilter,
  hostOptions,
  filters,
  counts,
  onSearchChange,
  onFilterChange,
  onHostFilterChange,
  onCreate,
  onExport,
  canCreate,
  columnOptions,
  visibleColumns,
  onVisibleColumnsChange,
  actionsSlot,
  children,
}: {
  search: string;
  filter: string;
  hostFilter: string;
  hostOptions: Array<{ value: string; label: string; tooltip?: string }>;
  filters: VMFilterOption[];
  counts: VMFilterCounts;
  onSearchChange: (value: string) => void;
  onFilterChange: (value: string) => void;
  onHostFilterChange: (value: string) => void;
  onCreate: () => void;
  onExport: () => void;
  canCreate: boolean;
  columnOptions: VMColumnOption[];
  visibleColumns: VMTableColumnKey[];
  onVisibleColumnsChange: (columns: VMTableColumnKey[]) => void;
  actionsSlot?: React.ReactNode;
  children?: React.ReactNode;
}) {
  return (
    <>
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="text-lg font-semibold" style={{ color: 'var(--kvm-text)' }}>
            虚拟机 / 模板
          </h1>
          <p className="mt-1 text-sm" style={{ color: 'var(--kvm-text-muted)' }}>
            生命周期管理、模板标记与远程 Agent 操作，共 {counts.all} 台
          </p>
        </div>
        <div className="flex flex-wrap items-center justify-end gap-2">
          {actionsSlot}
          <VMColumnPicker
            options={columnOptions}
            selected={visibleColumns}
            onChange={onVisibleColumnsChange}
          />
          <button
            type="button"
            onClick={onExport}
            className="kvm-action-button flex h-10 items-center gap-2 rounded-lg border px-3 text-sm"
            style={{
              borderColor: 'rgba(59,130,246,0.38)',
              color: 'var(--kvm-accent-text)',
              background: 'rgba(59,130,246,0.1)',
            }}
          >
            <DownloadIcon size={14} />
            导出
          </button>
          {canCreate && <VMCreateButton onClick={onCreate} />}
        </div>
      </div>

      <div className="flex flex-wrap items-center justify-between gap-3">
        <div
          className="flex w-fit max-w-full flex-wrap items-center justify-start gap-2 rounded-xl px-3 py-3"
          style={{ background: 'var(--kvm-card)', border: '1px solid var(--kvm-border)' }}
        >
          <div className="relative w-[min(380px,calc(100vw-96px))]">
            <SearchIcon
              size={14}
              className="absolute left-3 top-1/2 -translate-y-1/2"
              style={{ color: 'var(--kvm-text-muted)' }}
            />
            <input
              value={search}
              onChange={event => onSearchChange(event.target.value)}
              placeholder="搜索名称、IP、宿主机或系统"
              className="w-full rounded-lg py-2 pl-9 pr-3 text-sm outline-none"
              style={{
                background: 'var(--kvm-control-bg)',
                border: '1px solid var(--kvm-border)',
                color: 'var(--kvm-text)',
              }}
            />
          </div>
          <FilterIcon size={14} className="mx-1" style={{ color: 'var(--kvm-text-muted)' }} />
          <SelectMenu
            value={hostFilter}
            options={hostOptions}
            placeholder="选择宿主机"
            className="w-[140px]"
            maxVisibleItems={6}
            onChange={onHostFilterChange}
          />
          {filters.map(item => (
            <button
              key={item.key}
              type="button"
              onClick={() => onFilterChange(item.key)}
              className="kvm-action-button rounded-lg px-3 py-1.5 text-xs transition-colors"
              style={{
                background: filter === item.key ? 'rgba(59,130,246,0.15)' : 'transparent',
                color: filter === item.key ? '#3b82f6' : 'var(--kvm-text-muted)',
                border:
                  filter === item.key ? '1px solid rgba(59,130,246,0.3)' : '1px solid transparent',
              }}
            >
              {item.label} ({counts[item.key]})
            </button>
          ))}
        </div>
        {children && <div className="ml-auto flex justify-end">{children}</div>}
      </div>
    </>
  );
}

function VMColumnPicker({
  options,
  selected,
  onChange,
}: {
  options: VMColumnOption[];
  selected: VMTableColumnKey[];
  onChange: (columns: VMTableColumnKey[]) => void;
}) {
  const [open, setOpen] = useState(false);
  const [menuRect, setMenuRect] = useState({
    top: 0,
    left: 0,
    width: 0,
    placement: 'bottom' as 'bottom' | 'top',
  });
  const rootRef = useRef<HTMLDivElement | null>(null);
  const menuRef = useRef<HTMLDivElement | null>(null);
  const allSelected = selected.length === options.length;

  useEffect(() => {
    if (!open) return;
    const root = rootRef.current;
    if (!root) return;
    const updateMenuRect = () => {
      const rect = root.getBoundingClientRect();
      const menuHeight = Math.min(360, options.length * 40 + 96);
      const spaceBelow = window.innerHeight - rect.bottom;
      const spaceAbove = rect.top;
      const placement = spaceBelow < menuHeight + 16 && spaceAbove > spaceBelow ? 'top' : 'bottom';
      setMenuRect({
        top: placement === 'top' ? Math.max(8, rect.top - menuHeight - 8) : rect.bottom + 8,
        left: rect.left,
        width: Math.max(240, rect.width),
        placement,
      });
    };
    const close = (event: MouseEvent) => {
      const target = event.target as Node;
      if (!root.contains(target) && !menuRef.current?.contains(target)) setOpen(false);
    };
    updateMenuRect();
    window.addEventListener('resize', updateMenuRect);
    window.addEventListener('scroll', updateMenuRect, true);
    window.addEventListener('mousedown', close);
    return () => {
      window.removeEventListener('resize', updateMenuRect);
      window.removeEventListener('scroll', updateMenuRect, true);
      window.removeEventListener('mousedown', close);
    };
  }, [open, options.length]);

  const toggleAll = () => {
    onChange(allSelected ? [] : options.map(option => option.key));
  };

  const toggleColumn = (key: VMTableColumnKey) => {
    onChange(selected.includes(key) ? selected.filter(item => item !== key) : [...selected, key]);
  };

  return (
    <div ref={rootRef} className="relative">
      <button
        type="button"
        onClick={() => setOpen(value => !value)}
        className="kvm-action-button flex h-10 items-center gap-2 rounded-lg border px-3 text-sm"
        style={{
          borderColor: open ? 'rgba(45,212,191,0.45)' : 'var(--kvm-border)',
          color: 'var(--kvm-text-muted)',
          background: 'var(--kvm-control-bg)',
          boxShadow: open
            ? '0 0 0 3px rgba(45,212,191,0.08)'
            : 'inset 0 1px 0 rgba(255,255,255,0.045)',
        }}
        aria-haspopup="listbox"
        aria-expanded={open}
      >
        <Columns3Icon size={14} />
        <span>列显示</span>
        <span className="text-xs" style={{ color: 'var(--kvm-text-muted)' }}>
          {selected.length}/{options.length}
        </span>
        <ChevronDownIcon
          size={14}
          className={open ? 'rotate-180 transition-transform' : 'transition-transform'}
        />
      </button>
      {open && typeof document !== 'undefined'
        ? createPortal(
            <div
              ref={menuRef}
              className="kvm-hidden-scrollbar fixed max-h-[360px] overflow-y-auto rounded-lg border p-2 shadow-2xl"
              role="listbox"
              aria-multiselectable="true"
              style={{
                background: 'var(--kvm-menu-bg)',
                borderColor: 'var(--kvm-popover-border)',
                boxShadow: 'var(--kvm-menu-shadow)',
                left: menuRect.left,
                top: menuRect.top,
                width: menuRect.width,
                zIndex: 1200,
              }}
            >
              <div
                className="flex items-center justify-between gap-2 border-b px-1 pb-2"
                style={{ borderColor: 'var(--kvm-border)' }}
              >
                <div className="text-xs font-semibold" style={{ color: 'var(--kvm-text)' }}>
                  当前视图列
                </div>
                <button
                  type="button"
                  onClick={toggleAll}
                  className="kvm-action-button rounded-md border px-2 py-1 text-xs font-semibold"
                  style={{
                    borderColor: 'var(--kvm-border)',
                    color: 'var(--kvm-text-muted)',
                    background: 'rgba(255,255,255,0.035)',
                  }}
                >
                  {allSelected ? '取消全选' : '全选'}
                </button>
              </div>
              <div className="pt-2">
                {options.map(option => {
                  const active = selected.includes(option.key);
                  return (
                    <button
                      key={option.key}
                      type="button"
                      role="option"
                      aria-selected={active}
                      onClick={() => toggleColumn(option.key)}
                      className="group flex h-10 w-full cursor-pointer items-center gap-3 rounded-md px-3 text-left text-sm font-semibold transition-colors hover:bg-[rgba(45,212,191,0.1)]"
                      style={{
                        background: active ? 'rgba(45,212,191,0.14)' : undefined,
                        color: active ? 'var(--kvm-text)' : 'var(--kvm-text-muted)',
                      }}
                    >
                      <span
                        className="flex h-4 w-4 shrink-0 items-center justify-center rounded border"
                        style={{
                          borderColor: active ? 'rgba(45,212,191,0.7)' : 'var(--kvm-border)',
                          background: active ? 'rgba(45,212,191,0.22)' : 'transparent',
                        }}
                      >
                        {active && <CheckIcon size={12} />}
                      </span>
                      <span className="min-w-0 truncate">{option.label}</span>
                    </button>
                  );
                })}
              </div>
            </div>,
            document.body
          )
        : null}
    </div>
  );
}
