import { useState } from 'react';
import { LoaderCircleIcon, XIcon } from 'lucide-react';

import { DialogPortal } from '../../../components/kvm/DialogPortal';
import { formatBytesAuto } from '../../../lib/format';
import { buttonStyle, primaryButtonStyle } from './storagePoolStyles';

export function ISOUploadDialog({
  busy,
  onClose,
  onSubmit,
}: {
  busy: boolean;
  onClose: () => void;
  onSubmit: (file: File, name: string) => void;
}) {
  const [file, setFile] = useState<File | null>(null);
  const [name, setName] = useState('');
  const displayName = name.trim() || file?.name || '';
  return (
    <DialogPortal>
    <div className="fixed inset-0 z-[70] flex items-center justify-center px-4" style={{ background: 'rgba(2,6,23,0.36)' }}>
      <div className="kvm-dialog-panel w-full max-w-md rounded-xl p-5">
        <DialogHeader title="上传 ISO" subtitle="选择本地 ISO 文件并上传到当前存储池" onClose={onClose} />
        <div className="mx-auto mt-5 max-w-sm space-y-4">
          <label className="block">
            <span className="mb-2 block text-sm font-semibold" style={{ color: 'var(--kvm-text)' }}>ISO 文件</span>
            <input type="file" accept=".iso,application/x-iso9660-image" onChange={event => setFile(event.target.files?.[0] ?? null)} className="block w-full cursor-pointer rounded-lg border p-2 text-sm" style={{ borderColor: 'var(--kvm-border)', background: 'var(--kvm-control-bg)', color: 'var(--kvm-text)' }} />
          </label>
          <label className="block">
            <span className="mb-2 block text-sm font-semibold" style={{ color: 'var(--kvm-text)' }}>名称</span>
            <input value={name} onChange={event => setName(event.target.value)} placeholder={file?.name || 'ubuntu-22.04.iso'} className="h-10 w-full rounded-lg px-3 text-sm outline-none" style={{ background: 'var(--kvm-control-bg)', border: '1px solid var(--kvm-border)', color: 'var(--kvm-text)' }} />
          </label>
          <div className="rounded-lg border p-3 text-xs" style={{ borderColor: 'var(--kvm-border)', background: 'var(--kvm-control-bg-soft)', color: 'var(--kvm-text-muted)' }}>
            {file ? `${displayName} · ${formatBytesAuto(file.size)}` : '尚未选择文件'}
          </div>
        </div>
        <DialogFooter busy={busy} confirmLabel="上传" disabled={!file || (displayName !== '' && !displayName.toLowerCase().endsWith('.iso'))} onClose={onClose} onConfirm={() => file && onSubmit(file, displayName)} />
      </div>
    </div>
    </DialogPortal>
  );
}

function DialogHeader({ title, subtitle, onClose }: { title: string; subtitle: string; onClose: () => void }) {
  return (
    <div className="flex items-start justify-between gap-4">
      <div>
        <h3 className="text-base font-semibold" style={{ color: 'var(--kvm-text)' }}>{title}</h3>
        <p className="mt-1 text-xs" style={{ color: 'var(--kvm-text-muted)' }}>{subtitle}</p>
      </div>
      <button type="button" onClick={onClose} aria-label="关闭" className="kvm-action-button inline-flex h-8 w-8 shrink-0 items-center justify-center rounded-lg border" style={buttonStyle}><XIcon size={15} /></button>
    </div>
  );
}

function DialogFooter({ busy, disabled, confirmLabel, onClose, onConfirm }: { busy: boolean; disabled: boolean; confirmLabel: string; onClose: () => void; onConfirm: () => void }) {
  return (
    <div className="mt-5 flex justify-end gap-2 border-t pt-4" style={{ borderColor: 'var(--kvm-border)' }}>
      <button type="button" disabled={busy} onClick={onClose} className="kvm-action-button h-9 rounded-lg border px-4 text-sm disabled:opacity-60" style={buttonStyle}>取消</button>
      <button type="button" disabled={busy || disabled} onClick={onConfirm} className="kvm-action-button inline-flex h-9 items-center gap-2 rounded-lg border px-4 text-sm font-semibold disabled:opacity-60" style={primaryButtonStyle}>
        {busy && <LoaderCircleIcon size={14} className="animate-spin" />}{confirmLabel}
      </button>
    </div>
  );
}
