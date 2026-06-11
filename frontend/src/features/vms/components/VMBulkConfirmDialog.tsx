import { DialogFrame } from './DialogFrame';
import { actionMeta, type VMBulkAction } from '../types';

export function VMBulkConfirmDialog({
  action,
  selectedNames,
  confirmText,
  busy,
  onConfirmTextChange,
  onClose,
  onConfirm,
}: {
  action: VMBulkAction;
  selectedNames: string[];
  confirmText: string;
  busy: boolean;
  onConfirmTextChange: (value: string) => void;
  onClose: () => void;
  onConfirm: () => void;
}) {
  const meta = actionMeta[action];
  const expected = '确认删除';
  const canConfirm = confirmText.trim() === expected;

  return (
    <DialogFrame title={`批量${meta.label}`} tone="danger" onClose={onClose}>
      <div className="space-y-4">
        <div
          className="rounded-lg p-4"
          style={{
            background: 'rgba(239,68,68,0.08)',
            border: '1px solid rgba(239,68,68,0.24)',
          }}
        >
          <div className="text-sm font-semibold" style={{ color: 'var(--kvm-text)' }}>
            将对 {selectedNames.length} 台虚拟机执行{meta.label}
          </div>
          <div
            className="mt-2 max-h-28 overflow-y-auto text-xs leading-5"
            style={{ color: 'var(--kvm-text-muted)' }}
          >
            {selectedNames.join('、')}
          </div>
        </div>
        <label className="block space-y-2 text-sm">
          <div style={{ color: 'var(--kvm-text-muted)' }}>输入“确认删除”继续</div>
          <input
            value={confirmText}
            disabled={busy}
            onChange={event => onConfirmTextChange(event.target.value)}
            className="w-full rounded-lg px-3 py-2 outline-none disabled:opacity-60"
            style={{
              background: 'rgba(255,255,255,0.04)',
              border: '1px solid rgba(239,68,68,0.32)',
              color: 'var(--kvm-text)',
            }}
          />
        </label>
        <div className="flex justify-end gap-2">
          <button
            type="button"
            onClick={onClose}
            disabled={busy}
            className="kvm-action-button rounded-lg border px-4 py-2 text-sm disabled:opacity-50"
            style={{
              borderColor: 'var(--kvm-border)',
              color: 'var(--kvm-text-muted)',
              background: 'rgba(255,255,255,0.03)',
            }}
          >
            取消
          </button>
          <button
            type="button"
            disabled={busy || !canConfirm}
            onClick={onConfirm}
            className="kvm-action-button kvm-danger-button rounded-lg border px-4 py-2 text-sm font-semibold disabled:cursor-not-allowed disabled:opacity-45"
            style={{
              borderColor: 'rgba(239,68,68,0.42)',
              color: '#fca5a5',
              background: 'rgba(239,68,68,0.1)',
            }}
          >
            {busy ? '执行中' : meta.label}
          </button>
        </div>
      </div>
    </DialogFrame>
  );
}
