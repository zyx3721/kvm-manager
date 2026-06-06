import { SelectMenu } from '../../../components/kvm/SelectMenu';
import { actionMeta, type VMBulkAction } from '../types';

export function VMBulkActionBar({
  selectedCount,
  action,
  busy,
  canRun,
  onActionChange,
  onRun,
}: {
  selectedCount: number;
  action: VMBulkAction;
  busy: boolean;
  canRun: boolean;
  onActionChange: (action: VMBulkAction) => void;
  onRun: () => void;
}) {
  const options = (
    [
      'start',
      'pause',
      'stop',
      'force-stop',
      'delete',
      'force-delete',
      'reboot',
      'force-reboot',
      'shutdown',
      'force-shutdown',
    ] as VMBulkAction[]
  ).map(item => ({
    value: item,
    label: actionMeta[item].label,
    tooltip: actionMeta[item].description,
  }));

  return (
    <div
      className="flex w-[min(350px,calc(100vw-48px))] max-w-full flex-nowrap items-center gap-2 rounded-xl px-3 py-3"
      style={{ background: 'var(--kvm-card)', border: '1px solid var(--kvm-border)' }}
    >
      <span className="shrink-0 text-sm font-semibold" style={{ color: 'var(--kvm-text)' }}>
        已选择 {selectedCount} 台
      </span>
      <SelectMenu
        value={action}
        options={options}
        placeholder="选择操作"
        className="min-w-0 flex-1"
        maxVisibleItems={6}
        onChange={value => onActionChange(value as VMBulkAction)}
      />
      <button
        type="button"
        disabled={busy || !canRun}
        onClick={onRun}
        className="kvm-action-button h-10 w-20 shrink-0 rounded-lg border px-4 text-sm font-semibold disabled:opacity-60"
        style={{
          background:
            actionMeta[action].tone === 'danger'
              ? 'rgba(239,68,68,0.1)'
              : 'rgba(59,130,246,0.12)',
          borderColor:
            actionMeta[action].tone === 'danger'
              ? 'rgba(239,68,68,0.36)'
              : 'rgba(59,130,246,0.36)',
          color: actionMeta[action].tone === 'danger' ? '#fca5a5' : '#93c5fd',
        }}
      >
        {busy ? '执行中' : '执行'}
      </button>
    </div>
  );
}
