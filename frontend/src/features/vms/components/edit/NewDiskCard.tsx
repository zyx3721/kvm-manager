import { Trash2Icon } from 'lucide-react';
import { type ReactNode } from 'react';

import { KvmTooltip } from '../../../../components/kvm/StatusBadge';
import { SelectMenu } from '../../../../components/kvm/SelectMenu';
import { type StoragePool } from '../../../../lib/api';
import { CheckToggle } from '../VMEditControls';
import { fieldStyle, inputClass } from './EditShared';

export type NewDiskDraft = {
  id: string;
  target: string;
  name: string;
  pool: string;
  bus: string;
  format: string;
  capacityGB: string;
  preallocMetadata: boolean;
};

const diskFormatOptions = ['qcow2', 'raw', 'qcow', 'qed'].map(value => ({ value, label: value }));
const diskBusOptions = ['virtio', 'sata', 'scsi', 'ide'].map(value => ({ value, label: value }));

export function NewDiskCard({
  disk,
  disabled,
  storagePools,
  onChange,
  onRemove,
}: {
  disk: NewDiskDraft;
  disabled?: boolean;
  storagePools: StoragePool[];
  onChange: (disk: Partial<NewDiskDraft>) => void;
  onRemove: () => void;
}) {
  const extension = volumeExtensionForFormat(disk.format);
  const extensionMatched = disk.name.trim().toLowerCase().endsWith(extension);

  return (
    <div
      className="rounded-xl border p-4"
      style={{
        background:
          'linear-gradient(135deg, rgba(59,130,246,0.08), transparent 42%), var(--kvm-control-bg-soft)',
        borderColor: 'var(--kvm-border)',
        boxShadow: 'inset 0 1px 0 rgba(255,255,255,0.06)',
      }}
    >
      <div className="mb-4 flex items-start justify-between gap-3">
        <div className="min-w-0">
          <div className="flex flex-wrap items-center gap-2">
            <span className="text-sm font-semibold" style={{ color: 'var(--kvm-text)' }}>
              新增磁盘
            </span>
            <span
              className="rounded-md border px-2 py-0.5 font-mono text-xs"
              style={{
                background: 'var(--kvm-control-bg)',
                borderColor: 'var(--kvm-border)',
                color: 'var(--kvm-text-muted)',
              }}
            >
              {disk.target || '未设置目标'}
            </span>
          </div>
          <div className="mt-1 text-xs" style={{ color: 'var(--kvm-text-muted)' }}>
            {disk.name || `卷名需以 ${extension} 结尾`}
          </div>
        </div>
        <KvmTooltip label="移除新增磁盘" placement="top">
          <button
            type="button"
            disabled={disabled}
            onClick={onRemove}
            aria-label="移除新增磁盘"
            className="kvm-action-button inline-flex h-9 w-9 shrink-0 items-center justify-center rounded-lg border disabled:opacity-60"
            style={{
              background: 'var(--kvm-control-bg)',
              borderColor: 'var(--kvm-border)',
              color: 'var(--kvm-text-muted)',
            }}
          >
            <Trash2Icon size={15} />
          </button>
        </KvmTooltip>
      </div>

      <div className="grid gap-3 lg:grid-cols-[minmax(190px,1.35fr)_minmax(160px,1fr)_minmax(110px,0.58fr)_minmax(110px,0.58fr)_minmax(110px,0.64fr)]">
        <LabeledField label="卷名称">
          <input
            value={disk.name}
            disabled={disabled}
            onChange={event => onChange({ name: event.target.value })}
            className={inputClass + ' disabled:opacity-60'}
            style={{
              ...fieldStyle,
              borderColor: extensionMatched ? 'var(--kvm-border)' : 'rgba(245,158,11,0.46)',
            }}
            aria-label="新增磁盘卷名称"
          />
        </LabeledField>
        <LabeledField label="存储池">
          <SelectMenu
            value={disk.pool}
            disabled={disabled || storagePools.length === 0}
            placeholder="选择存储池"
            placement="top"
            maxVisibleItems={4}
            options={storagePools.map(pool => ({
              value: pool.name,
              label: pool.name,
              tooltip: pool.path || '-',
            }))}
            onChange={pool => onChange({ pool })}
          />
        </LabeledField>
        <LabeledField label="总线">
          <SelectMenu
            value={disk.bus}
            disabled={disabled}
            placeholder="总线"
            placement="top"
            options={diskBusOptions}
            onChange={bus => onChange({ bus })}
          />
        </LabeledField>
        <LabeledField label="目标">
          <KvmTooltip
            label="目标是虚拟机内识别到的磁盘设备名，必须与已有磁盘不同"
            placement="bottom"
            multiline
          >
            <input
              value={disk.target}
              disabled={disabled}
              onChange={event => onChange({ target: event.target.value })}
              className={inputClass + ' disabled:opacity-60'}
              style={fieldStyle}
              aria-label="新增磁盘目标设备"
            />
          </KvmTooltip>
        </LabeledField>
        <LabeledField label="容量">
          <div className="relative">
            <input
              value={disk.capacityGB}
              type="number"
              min="1"
              step="1"
              disabled={disabled}
              onChange={event => onChange({ capacityGB: event.target.value })}
              className={inputClass + ' pr-10 disabled:opacity-60'}
              style={fieldStyle}
              aria-label="新增磁盘容量"
            />
            <span
              className="pointer-events-none absolute right-3 top-1/2 -translate-y-1/2 text-xs"
              style={{ color: 'var(--kvm-text-muted)' }}
            >
              GB
            </span>
          </div>
        </LabeledField>
      </div>

      <div className="mt-3 grid gap-3 sm:grid-cols-[minmax(110px,0.16fr)_minmax(0,1fr)] sm:items-end">
        <LabeledField label="格式">
          <SelectMenu
            value={disk.format}
            disabled={disabled}
            placeholder="格式"
            placement="top"
            options={diskFormatOptions}
            onChange={format =>
              onChange({
                format,
                name: replaceDiskExtension(disk.name, format),
                preallocMetadata: format === 'qcow2' ? disk.preallocMetadata : false,
              })
            }
          />
        </LabeledField>
        <div className="flex min-h-10 flex-wrap items-center gap-3">
          {disk.format === 'qcow2' ? (
            <CheckToggle
              checked={disk.preallocMetadata}
              disabled={disabled}
              onChange={preallocMetadata => onChange({ preallocMetadata })}
              label="预分配 Metadata"
            />
          ) : (
            <span className="text-xs" style={{ color: 'var(--kvm-text-muted)' }}>
              非 qcow2 格式的卷名称统一使用 .img 扩展名
            </span>
          )}
        </div>
      </div>
    </div>
  );
}

function LabeledField({
  label,
  hint,
  children,
}: {
  label: string;
  hint?: string;
  children: ReactNode;
}) {
  return (
    <label className="min-w-0 space-y-1.5 text-xs" style={{ color: 'var(--kvm-text-muted)' }}>
      <span className="flex min-h-4 items-center justify-between gap-2">
        <span>{label}</span>
        {hint && <span className="truncate font-normal opacity-80">{hint}</span>}
      </span>
      {children}
    </label>
  );
}

export function volumeExtensionForFormat(format: string) {
  return format.trim().toLowerCase() === 'qcow2' ? '.qcow2' : '.img';
}

export function replaceDiskExtension(name: string, format: string) {
  const clean = name.trim();
  const extension = volumeExtensionForFormat(format);
  if (!clean) return clean;
  const index = clean.lastIndexOf('.');
  if (index <= 0) return `${clean}${extension}`;
  return `${clean.slice(0, index)}${extension}`;
}
