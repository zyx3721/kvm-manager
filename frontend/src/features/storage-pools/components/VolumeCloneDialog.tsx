import { useMemo, useState } from 'react';
import { LoaderCircleIcon, XIcon } from 'lucide-react';

import { DialogPortal } from '../../../components/kvm/DialogPortal';
import { SelectMenu } from '../../../components/kvm/SelectMenu';
import type { StorageVolume, StorageVolumeClonePayload } from '../../../lib/api';
import { buttonStyle, disabledFieldStyle, primaryButtonStyle } from './storagePoolStyles';

const cloneFormats = ['raw', 'qcow', 'qcow2', 'qed'];

export function VolumeCloneDialog({
  volume,
  busy,
  onClose,
  onSubmit,
}: {
  volume: StorageVolume;
  busy: boolean;
  onClose: () => void;
  onSubmit: (payload: StorageVolumeClonePayload) => void;
}) {
  const [name, setName] = useState('');
  const [convert, setConvert] = useState(false);
  const [format, setFormat] = useState('raw');
  const suffix = useMemo(() => {
    const targetFormat = convert ? format : volume.format;
    return targetExtension(targetFormat);
  }, [convert, format, volume.format]);
  const targetName = `${name.trim()}${suffix}`;
  const disabled = !name.trim() || (convert && !format);

  return (
    <DialogPortal>
      <div
        className="fixed inset-0 z-[70] flex items-center justify-center px-4"
        style={{ background: 'rgba(2,6,23,0.36)' }}
      >
        <div className="kvm-dialog-panel w-full max-w-lg overflow-hidden rounded-xl p-0">
          <header
            className="flex items-start justify-between gap-4 border-b px-5 py-4"
            style={{ borderColor: 'var(--kvm-border)' }}
          >
            <div className="min-w-0">
              <div className="flex flex-wrap items-center gap-2">
                <h3 className="text-base font-semibold" style={{ color: 'var(--kvm-text)' }}>
                  克隆镜像
                </h3>
                <span
                  className="max-w-[260px] truncate rounded-md border px-2.5 py-1 text-xs font-semibold"
                  style={{
                    background: 'rgba(59,130,246,0.12)',
                    borderColor: 'rgba(59,130,246,0.28)',
                    color: 'var(--kvm-accent-text)',
                  }}
                >
                  {volume.name}
                </span>
              </div>
              <p className="mt-1 text-xs" style={{ color: 'var(--kvm-text-muted)' }}>
                创建当前镜像的副本，可按需转换格式
              </p>
            </div>
            <button
              type="button"
              onClick={onClose}
              aria-label="关闭"
              className="kvm-action-button inline-flex h-8 w-8 shrink-0 items-center justify-center rounded-lg border"
              style={buttonStyle}
            >
              <XIcon size={15} />
            </button>
          </header>
          <div className="mx-auto max-w-md space-y-4 px-5 py-5">
            <label className="block">
              <span
                className="mb-2 block text-sm font-semibold"
                style={{ color: 'var(--kvm-text)' }}
              >
                名称
              </span>
              <div
                className="grid grid-cols-[1fr_auto] overflow-hidden rounded-lg border"
                style={{ borderColor: 'var(--kvm-border)', background: 'var(--kvm-control-bg)' }}
              >
                <input
                  value={name}
                  onChange={event => setName(event.target.value)}
                  placeholder="clone-disk"
                  className="h-10 min-w-0 bg-transparent px-3 text-sm outline-none"
                  style={{ color: 'var(--kvm-text)' }}
                />
                <span
                  className="flex h-10 items-center border-l px-3 font-mono text-sm font-semibold"
                  style={{
                    borderColor: 'var(--kvm-border)',
                    color: 'var(--kvm-text-muted)',
                    background: 'rgba(148,163,184,0.08)',
                  }}
                >
                  {suffix}
                </span>
              </div>
            </label>
            <label
              className="flex cursor-pointer items-center justify-between gap-3 rounded-lg border p-3 transition-colors hover:bg-[rgba(59,130,246,0.06)]"
              style={{ borderColor: 'var(--kvm-border)', background: 'var(--kvm-control-bg-soft)' }}
            >
              <span>
                <span className="block text-sm font-semibold" style={{ color: 'var(--kvm-text)' }}>
                  转换格式
                </span>
                <span className="mt-1 block text-xs" style={{ color: 'var(--kvm-text-muted)' }}>
                  {convert ? '克隆时会生成所选格式的新镜像' : '默认保持源镜像格式'}
                </span>
              </span>
              <input
                type="checkbox"
                checked={convert}
                onChange={event => setConvert(event.target.checked)}
                className="h-4 w-4 cursor-pointer accent-blue-500"
              />
            </label>
            <div className="block">
              <span
                className="mb-2 block text-sm font-semibold"
                style={{ color: 'var(--kvm-text)' }}
              >
                格式
              </span>
              {convert ? (
                <SelectMenu
                  value={format}
                  placeholder="请选择格式"
                  placement="bottom"
                  maxVisibleItems={3}
                  options={cloneFormats.map(item => ({ value: item, label: item }))}
                  onChange={setFormat}
                />
              ) : (
                <div
                  className="flex h-10 items-center rounded-lg px-3 text-sm font-semibold"
                  style={disabledFieldStyle}
                >
                  保持源格式
                </div>
              )}
            </div>
            <div
              className="rounded-lg border p-3 text-xs"
              style={{
                borderColor: 'var(--kvm-border)',
                background: 'var(--kvm-control-bg-soft)',
                color: 'var(--kvm-text-muted)',
              }}
            >
              {name.trim()
                ? `将克隆为 ${targetName}${convert ? `，并转换格式为 ${format}` : ''}`
                : '请输入新镜像名称'}
            </div>
          </div>
          <div
            className="flex justify-end gap-2 border-t px-6 py-5"
            style={{ borderColor: 'var(--kvm-border)' }}
          >
            <button
              type="button"
              disabled={busy}
              onClick={onClose}
              className="kvm-action-button h-9 rounded-lg border px-4 text-sm disabled:opacity-60"
              style={buttonStyle}
            >
              关闭
            </button>
            <button
              type="button"
              disabled={busy || disabled}
              onClick={() =>
                onSubmit({ name: targetName, sourceName: volume.name, format, convert })
              }
              className="kvm-action-button inline-flex h-9 items-center gap-2 rounded-lg border px-4 text-sm font-semibold disabled:opacity-60"
              style={primaryButtonStyle}
            >
              {busy && <LoaderCircleIcon size={14} className="animate-spin" />}克隆
            </button>
          </div>
        </div>
      </div>
    </DialogPortal>
  );
}

function targetExtension(format: string) {
  return format === 'qcow2' ? '.qcow2' : '.img';
}
