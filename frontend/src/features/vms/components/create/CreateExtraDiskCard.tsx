import { Trash2Icon } from 'lucide-react';

import { SelectMenu } from '../../../../components/kvm/SelectMenu';
import { diskTargetForBus } from '../../utils/createDisk';
import { Field, MetadataToggleRow, fieldStyle, inputClass } from './CreateFormShared';

export type CreateDiskDraft = {
  id: string;
  name: string;
  pool: string;
  format: string;
  bus: string;
  capacityGB: string;
  preallocMetadata: boolean;
  nameTouched: boolean;
};

const formats = ['qcow2', 'raw', 'qcow', 'qed'].map(value => ({ value, label: value }));
const buses = ['virtio', 'sata', 'scsi', 'ide'].map(value => ({ value, label: value }));

export function CreateExtraDiskCard({
  disk,
  index,
  disabled,
  storageOptions,
  metadataDisabled,
  onCapacityChange,
  onRemove,
}: {
  disk: CreateDiskDraft;
  index: number;
  disabled?: boolean;
  storageOptions: Array<{ value: string; label: string; tooltip?: string }>;
  metadataDisabled?: boolean;
  onCapacityChange: (capacityGB: string) => void;
  onRemove: () => void;
}) {
  const target = diskTargetForBus(disk.bus, index);
  return (
    <div
      className="rounded-xl border p-3"
      style={{ background: 'var(--kvm-control-bg-soft)', borderColor: 'var(--kvm-border)' }}
    >
      <div className="mb-3 flex items-center justify-between gap-3">
        <div>
          <div className="text-sm font-semibold" style={{ color: 'var(--kvm-text)' }}>
            数据盘 {index}
          </div>
          <div className="mt-0.5 text-xs" style={{ color: 'var(--kvm-text-muted)' }}>
            按添加顺序挂载，目标设备由 virt-install 分配
          </div>
        </div>
        <div className="flex items-center gap-2">
          <span
            className="rounded-md border px-2 py-0.5 font-mono text-xs"
            style={{
              background: 'var(--kvm-control-bg)',
              borderColor: 'var(--kvm-border)',
              color: 'var(--kvm-text-muted)',
            }}
          >
            {target}
          </span>
          <button
            type="button"
            disabled={disabled}
            onClick={onRemove}
            className="kvm-action-button inline-flex h-8 w-8 items-center justify-center rounded-lg border disabled:opacity-60"
            style={{
              background: 'var(--kvm-control-bg)',
              borderColor: 'var(--kvm-border)',
              color: 'var(--kvm-text-muted)',
            }}
            aria-label="移除数据盘"
          >
            <Trash2Icon size={14} />
          </button>
        </div>
      </div>
      <div className="grid gap-3 md:grid-cols-5">
        <Field label="存储池">
          <SelectMenu
            value={disk.pool}
            disabled
            options={storageOptions}
            placeholder="选择存储池"
            placement="top"
            maxVisibleItems={4}
            onChange={() => undefined}
          />
        </Field>
        <Field label="磁盘格式">
          <SelectMenu
            value={disk.format}
            disabled
            options={formats}
            placeholder="格式"
            placement="top"
            onChange={() => undefined}
          />
        </Field>
        <Field label="磁盘总线">
          <SelectMenu
            value={disk.bus}
            disabled
            options={buses}
            placeholder="总线"
            placement="top"
            onChange={() => undefined}
          />
        </Field>
        <Field label="容量 GB">
          <input
            value={disk.capacityGB}
            type="number"
            min="1"
            step="1"
            disabled={disabled}
            onChange={event => onCapacityChange(event.target.value)}
            className={inputClass}
            style={fieldStyle}
          />
        </Field>
        <Field label="卷名称">
          <input value={disk.name} disabled className={inputClass} style={fieldStyle} />
        </Field>
      </div>
      <MetadataToggleRow
        format={disk.format}
        checked={disk.preallocMetadata}
        disabled={disabled || metadataDisabled}
        onChange={() => undefined}
      />
    </div>
  );
}
