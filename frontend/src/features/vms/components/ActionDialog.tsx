import { DialogFrame } from './DialogFrame';
import { ConsoleViewer } from './ConsoleViewer';
import { actionMeta, type PendingAction } from '../types';

export function ActionDialog({
  pending,
  confirmName,
  shutdownMode,
  rebootMode,
  deleteMode,
  busy,
  canForce,
  onConfirmNameChange,
  onShutdownModeChange,
  onRebootModeChange,
  onDeleteModeChange,
  onClose,
  onConfirm,
  consolePassword,
}: {
  pending: PendingAction;
  confirmName: string;
  shutdownMode: 'stop' | 'force-stop' | 'shutdown' | 'force-shutdown';
  rebootMode: 'reboot' | 'force-reboot';
  deleteMode: 'delete' | 'force-delete';
  busy: boolean;
  canForce: boolean;
  onConfirmNameChange: (value: string) => void;
  onShutdownModeChange: (value: 'stop' | 'force-stop' | 'shutdown' | 'force-shutdown') => void;
  onRebootModeChange: (value: 'reboot' | 'force-reboot') => void;
  onDeleteModeChange: (value: 'delete' | 'force-delete') => void;
  onClose: () => void;
  onConfirm: () => void;
  consolePassword?: string;
}) {
  const vm = pending.vm;
  if (pending.action === 'console') {
    return (
      <DialogFrame tone="normal" onClose={onClose} wide hideHeader>
        <ConsoleViewer vm={vm} password={consolePassword} />
      </DialogFrame>
    );
  }

  const meta = actionMeta[pending.action];
  const isDelete = pending.action === 'delete';
  const isStop = pending.action === 'stop' || pending.action === 'shutdown';
  const isReboot = pending.action === 'reboot';
  const stopChoices =
    pending.action === 'shutdown'
      ? { normal: 'shutdown' as const, force: 'force-shutdown' as const, normalLabel: '正常关机', forceLabel: '强制关机' }
      : { normal: 'stop' as const, force: 'force-stop' as const, normalLabel: '正常停止', forceLabel: '强制停止' };
  const submitMeta = isStop
    ? actionMeta[shutdownMode]
    : isReboot
      ? actionMeta[rebootMode]
      : isDelete
        ? actionMeta[deleteMode]
        : meta;
  const canConfirm = !isDelete || confirmName.trim() === vm.name;

  return (
    <DialogFrame title={`${meta.label}虚拟机`} tone={meta.tone} onClose={onClose}>
      <div className="space-y-4">
        <div
          className="rounded-lg p-4"
          style={{
            background: meta.tone === 'danger' ? 'rgba(239,68,68,0.08)' : 'rgba(59,130,246,0.08)',
            border:
              meta.tone === 'danger'
                ? '1px solid rgba(239,68,68,0.24)'
                : '1px solid rgba(59,130,246,0.22)',
          }}
        >
          <div className="text-sm font-semibold" style={{ color: 'var(--kvm-text)' }}>
            {vm.name}
          </div>
          <div className="mt-1 text-xs leading-5" style={{ color: 'var(--kvm-text-muted)' }}>
            {meta.description}
          </div>
        </div>
        {isStop && (
          <div className={`grid gap-2 ${canForce ? 'grid-cols-2' : 'grid-cols-1'}`}>
            <button
              type="button"
              disabled={busy}
              onClick={() => onShutdownModeChange(stopChoices.normal)}
              className="kvm-action-button rounded-lg border px-3 py-2 text-left text-sm disabled:opacity-50"
              style={{
                borderColor:
                  shutdownMode === stopChoices.normal ? 'rgba(59,130,246,0.55)' : 'rgba(76,103,150,0.28)',
                background:
                  shutdownMode === stopChoices.normal ? 'rgba(59,130,246,0.14)' : 'rgba(255,255,255,0.035)',
                color: 'var(--kvm-text)',
              }}
            >
              <span className="block font-semibold">{stopChoices.normalLabel}</span>
              <span className="mt-1 block text-xs" style={{ color: 'var(--kvm-text-muted)' }}>
                Normal Shutdown
              </span>
            </button>
            {canForce && (
              <button
                type="button"
                disabled={busy}
                onClick={() => onShutdownModeChange(stopChoices.force)}
                className="kvm-action-button kvm-danger-button rounded-lg border px-3 py-2 text-left text-sm disabled:opacity-50"
                style={{
                  borderColor:
                    shutdownMode === stopChoices.force ? 'rgba(239,68,68,0.55)' : 'rgba(239,68,68,0.28)',
                  background:
                    shutdownMode === stopChoices.force
                      ? 'rgba(239,68,68,0.14)'
                      : 'rgba(255,255,255,0.035)',
                  color: 'var(--kvm-text)',
                }}
              >
                <span className="block font-semibold">{stopChoices.forceLabel}</span>
                <span className="mt-1 block text-xs" style={{ color: 'var(--kvm-text-muted)' }}>
                  Force Shutdown
                </span>
              </button>
            )}
          </div>
        )}
        {isReboot && (
          <div className={`grid gap-2 ${canForce ? 'grid-cols-2' : 'grid-cols-1'}`}>
            <button
              type="button"
              disabled={busy}
              onClick={() => onRebootModeChange('reboot')}
              className="kvm-action-button rounded-lg border px-3 py-2 text-left text-sm disabled:opacity-50"
              style={{
                borderColor:
                  rebootMode === 'reboot' ? 'rgba(245,158,11,0.55)' : 'rgba(76,103,150,0.28)',
                background:
                  rebootMode === 'reboot' ? 'rgba(245,158,11,0.14)' : 'rgba(255,255,255,0.035)',
                color: 'var(--kvm-text)',
              }}
            >
              <span className="block font-semibold">正常重启</span>
              <span className="mt-1 block text-xs" style={{ color: 'var(--kvm-text-muted)' }}>
                Normal Reboot
              </span>
            </button>
            {canForce && (
              <button
                type="button"
                disabled={busy}
                onClick={() => onRebootModeChange('force-reboot')}
                className="kvm-action-button kvm-danger-button rounded-lg border px-3 py-2 text-left text-sm disabled:opacity-50"
                style={{
                  borderColor:
                    rebootMode === 'force-reboot' ? 'rgba(239,68,68,0.55)' : 'rgba(239,68,68,0.28)',
                  background:
                    rebootMode === 'force-reboot'
                      ? 'rgba(239,68,68,0.14)'
                      : 'rgba(255,255,255,0.035)',
                  color: 'var(--kvm-text)',
                }}
              >
                <span className="block font-semibold">强制重启</span>
                <span className="mt-1 block text-xs" style={{ color: 'var(--kvm-text-muted)' }}>
                  Force Reset
                </span>
              </button>
            )}
          </div>
        )}
        {isDelete && (
          <>
            <div className={`grid gap-2 ${canForce ? 'grid-cols-2' : 'grid-cols-1'}`}>
              <button
                type="button"
                disabled={busy}
                onClick={() => onDeleteModeChange('delete')}
                className="kvm-action-button rounded-lg border px-3 py-2 text-left text-sm disabled:opacity-50"
                style={{
                  borderColor:
                    deleteMode === 'delete' ? 'rgba(245,158,11,0.55)' : 'rgba(76,103,150,0.28)',
                  background:
                    deleteMode === 'delete' ? 'rgba(245,158,11,0.14)' : 'rgba(255,255,255,0.035)',
                  color: 'var(--kvm-text)',
                }}
              >
                <span className="block font-semibold">正常删除</span>
                <span className="mt-1 block text-xs" style={{ color: 'var(--kvm-text-muted)' }}>
                  仅已关机时删除
                </span>
              </button>
              {canForce && (
                <button
                  type="button"
                  disabled={busy}
                  onClick={() => onDeleteModeChange('force-delete')}
                  className="kvm-action-button kvm-danger-button rounded-lg border px-3 py-2 text-left text-sm disabled:opacity-50"
                  style={{
                    borderColor:
                      deleteMode === 'force-delete'
                        ? 'rgba(239,68,68,0.55)'
                        : 'rgba(239,68,68,0.28)',
                    background:
                      deleteMode === 'force-delete'
                        ? 'rgba(239,68,68,0.14)'
                        : 'rgba(255,255,255,0.035)',
                    color: 'var(--kvm-text)',
                  }}
                >
                  <span className="block font-semibold">强制删除</span>
                  <span className="mt-1 block text-xs" style={{ color: 'var(--kvm-text-muted)' }}>
                    先强制关机再删除
                  </span>
                </button>
              )}
            </div>
            <label className="block space-y-2 text-sm">
              <div style={{ color: 'var(--kvm-text-muted)' }}>输入虚拟机名称确认删除</div>
              <input
                type="text"
                value={confirmName}
                disabled={busy}
                onChange={event => onConfirmNameChange(event.target.value)}
                placeholder={vm.name}
                className="w-full rounded-lg px-3 py-2 outline-none disabled:opacity-60"
                style={{
                  background: 'rgba(255,255,255,0.04)',
                  border: '1px solid rgba(239,68,68,0.32)',
                  color: 'var(--kvm-text)',
                }}
              />
            </label>
          </>
        )}
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
            onClick={onConfirm}
            disabled={busy || !canConfirm}
            className={
              'kvm-action-button ' +
              (submitMeta.tone === 'danger' ? 'kvm-danger-button ' : '') +
              'rounded-lg border px-4 py-2 text-sm font-semibold disabled:cursor-not-allowed disabled:opacity-45'
            }
            style={{
              borderColor:
                submitMeta.tone === 'danger' ? 'rgba(239,68,68,0.42)' : 'rgba(59,130,246,0.45)',
              color: submitMeta.tone === 'danger' ? '#fca5a5' : '#93c5fd',
              background:
                submitMeta.tone === 'danger' ? 'rgba(239,68,68,0.1)' : 'rgba(59,130,246,0.12)',
            }}
          >
            {busy ? '执行中' : submitMeta.label}
          </button>
        </div>
      </div>
    </DialogFrame>
  );
}
