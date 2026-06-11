import { FileCode2Icon, Layers3Icon, SlidersHorizontalIcon, type LucideIcon } from 'lucide-react';

import { KvmTooltip } from '../../../components/kvm/StatusBadge';

export type VMDialogTab = 'form' | 'template' | 'xml';

const tabs: Array<{ key: VMDialogTab; label: string; tooltip: string; Icon: LucideIcon }> = [
  { key: 'form', label: '常规', tooltip: '现有从空磁盘创建', Icon: SlidersHorizontalIcon },
  {
    key: 'template',
    label: '模板',
    tooltip: '从已有虚拟机模板克隆系统盘后创建',
    Icon: Layers3Icon,
  },
  { key: 'xml', label: 'XML', tooltip: '现有 XML 创建', Icon: FileCode2Icon },
];

export function DialogTabs({
  active,
  disabled,
  onChange,
}: {
  active: VMDialogTab;
  disabled?: boolean;
  onChange: (tab: VMDialogTab) => void;
}) {
  return (
    <div className="flex shrink-0 border-b px-4 pt-3" style={{ borderColor: 'var(--kvm-border)' }}>
      <div className="flex min-w-0 gap-1">
        {tabs.map(({ key, label, tooltip, Icon }) => {
          const selected = key === active;
          return (
            <KvmTooltip key={key} label={tooltip} placement="bottom" align="center">
              <button
                type="button"
                disabled={disabled}
                onClick={() => onChange(key)}
                className="kvm-action-button flex h-9 items-center gap-2 rounded-t-lg border border-b-0 px-3 text-xs font-semibold disabled:cursor-not-allowed disabled:opacity-60"
                style={{
                  background: selected ? 'var(--kvm-control-bg-soft)' : 'var(--kvm-control-bg)',
                  borderColor: selected ? 'rgba(59,130,246,0.38)' : 'var(--kvm-border)',
                  color: selected ? 'var(--kvm-accent-text)' : 'var(--kvm-text-muted)',
                }}
                aria-label={`${label}：${tooltip}`}
                aria-pressed={selected}
              >
                <Icon size={14} />
                {label}
              </button>
            </KvmTooltip>
          );
        })}
      </div>
    </div>
  );
}
