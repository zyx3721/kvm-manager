import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { createPortal } from 'react-dom';
import { toast } from 'sonner';
import {
  ArchiveIcon,
  CameraIcon,
  ChevronDownIcon,
  EyeIcon,
  FilterIcon,
  PencilIcon,
  PlusIcon,
  RefreshCwIcon,
  RotateCcwIcon,
  SearchIcon,
  Trash2Icon,
  XIcon,
} from 'lucide-react';
import {
  createSnapshot,
  fetchHosts,
  fetchSnapshots,
  fetchVMs,
  refreshSnapshots as refreshSnapshotsOnly,
  runSnapshotAction,
  updateSnapshotAnnotation,
  type Host,
  type Snapshot,
  type VirtualMachine,
} from '../../lib/api';
import { formatBytes, formatTimeAgo } from '../../lib/format';
import { DialogPortal } from '../../components/kvm/DialogPortal';
import { SelectMenu } from '../../components/kvm/SelectMenu';
import { StatusBadge } from '../../components/kvm/StatusBadge';
import { onKvmRefresh } from '../../lib/refresh';
import { can } from '../../lib/permissions';
import { isVMStopped } from '../vms/utils/vmStatus';

type DialogState =
  | { type: 'create' }
  | { type: 'detail'; snapshot: Snapshot }
  | { type: 'edit'; snapshot: Snapshot }
  | { type: 'confirm'; snapshot: Snapshot; action: 'revert' | 'delete' }
  | null;

type SnapshotStatusFilter = 'all' | Snapshot['status'];
type SnapshotStatusCounts = Record<SnapshotStatusFilter, number>;

const snapshotStatusFilters: Array<{ key: SnapshotStatusFilter; label: string }> = [
  { key: 'all', label: '全部' },
  { key: 'ready', label: '就绪' },
  { key: 'creating', label: '创建中' },
  { key: 'failed', label: '失败' },
];

export default function Snapshots() {
  const [snapshots, setSnapshots] = useState<Snapshot[]>([]);
  const [vms, setVMs] = useState<VirtualMachine[]>([]);
  const [hosts, setHosts] = useState<Host[]>([]);
  const [query, setQuery] = useState('');
  const [vmFilter, setVMFilter] = useState('all');
  const [hostFilter, setHostFilter] = useState('all');
  const [statusFilter, setStatusFilter] = useState<SnapshotStatusFilter>('all');
  const [dialog, setDialog] = useState<DialogState>(null);
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [actionId, setActionId] = useState('');
  const [error, setError] = useState('');
  const canCreateSnapshots = can('snapshots.create');
  const canUpdateSnapshots = can('snapshots.update');
  const canRevertSnapshots = can('snapshots.revert');
  const canDeleteSnapshots = can('snapshots.delete');
  const canRefreshSnapshots = can('snapshots.read');

  const loadSnapshots = useCallback(async () => {
    setLoading(true);
    setError('');
    try {
      const [snapshotResponse, vmResponse, hostResponse] = await Promise.all([
        fetchSnapshots(),
        fetchVMs(),
        fetchHosts(),
      ]);
      setSnapshots(snapshotResponse.items);
      setVMs(vmResponse.items);
      setHosts(hostResponse.items);
    } catch (err) {
      const message = err instanceof Error ? err.message : '读取快照列表失败';
      toast.error(message);
      setError(isPermissionMessage(message) ? '' : message);
    } finally {
      setLoading(false);
    }
  }, []);

  const refreshSnapshots = async () => {
    setRefreshing(true);
    setError('');
    try {
      await refreshSnapshotsOnly();
      await loadSnapshots();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : '创建快照刷新任务失败');
    } finally {
      setRefreshing(false);
    }
  };

  const handleSnapshotAction = async (snapshot: Snapshot, action: 'revert' | 'delete') => {
    setActionId(`${snapshot.id}:${action}`);
    setError('');
    try {
      await runSnapshotAction(snapshot.id, action);
      await loadSnapshots();
      setDialog(null);
      toast.success(`${snapshotTitle(snapshot)} ${action === 'revert' ? '恢复' : '删除'}完成`);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : '快照操作失败');
    } finally {
      setActionId('');
    }
  };

  const handleCreate = async (payload: {
    vmId: string;
    name: string;
    description: string;
    tags: string[];
  }) => {
    setActionId('create');
    try {
      await createSnapshot(payload);
      await loadSnapshots();
      setDialog(null);
      toast.success('快照已创建');
    } catch (err) {
      toast.error(err instanceof Error ? err.message : '创建快照失败');
    } finally {
      setActionId('');
    }
  };

  const handleSaveAnnotation = async (
    snapshot: Snapshot,
    payload: { displayName: string; description: string; tags: string[] }
  ) => {
    setActionId(`${snapshot.id}:edit`);
    try {
      const updated = await updateSnapshotAnnotation(snapshot.id, payload);
      setSnapshots(current => current.map(item => (item.id === updated.id ? updated : item)));
      setDialog(null);
      toast.success('快照备注已保存');
    } catch (err) {
      toast.error(err instanceof Error ? err.message : '保存快照备注失败');
    } finally {
      setActionId('');
    }
  };

  useEffect(() => {
    const timer = window.setTimeout(() => void loadSnapshots(), 0);
    const unsubscribe = onKvmRefresh(() => void loadSnapshots());
    return () => {
      window.clearTimeout(timer);
      unsubscribe();
    };
  }, [loadSnapshots]);

  const summary = useMemo(
    () => ({
      snapshots: snapshots.filter(item => item.type === 'snapshot').length,
      totalBytes: snapshots.reduce((total, item) => total + item.sizeBytes, 0),
    }),
    [snapshots]
  );
  const hostOptions = useMemo(
    () => [
      { value: 'all', label: '全部宿主机' },
      ...hosts.map(host => ({ value: host.id, label: host.name || host.address || host.id })),
    ],
    [hosts]
  );
  const visibleVMs = useMemo(
    () => (hostFilter === 'all' ? vms : vms.filter(vm => vm.hostId === hostFilter)),
    [hostFilter, vms]
  );
  const vmOptions = useMemo(
    () => [
      { value: 'all', label: '全部虚拟机' },
      ...visibleVMs.map(vm => ({ value: vm.id, label: vm.name })),
    ],
    [visibleVMs]
  );
  const statusCounts = useMemo<SnapshotStatusCounts>(
    () => ({
      all: snapshots.length,
      ready: snapshots.filter(item => item.status === 'ready').length,
      creating: snapshots.filter(item => item.status === 'creating').length,
      failed: snapshots.filter(item => item.status === 'failed').length,
    }),
    [snapshots]
  );
  const filteredSnapshots = useMemo(
    () =>
      snapshots.filter(item => {
        const keyword = query.trim().toLowerCase();
        const hostAddress = snapshotHostAddress(item, hosts);
        const matchesQuery =
          !keyword ||
          `${snapshotTitle(item)} ${item.name} ${item.description} ${item.vmName} ${item.hostName} ${hostAddress} ${(item.tags || []).join(' ')}`
            .toLowerCase()
            .includes(keyword);
        return (
          matchesQuery &&
          (hostFilter === 'all' || item.hostId === hostFilter) &&
          (vmFilter === 'all' || item.vmId === vmFilter) &&
          (statusFilter === 'all' || item.status === statusFilter)
        );
      }),
    [hostFilter, hosts, query, snapshots, statusFilter, vmFilter]
  );
  function handleHostFilterChange(nextHostId: string) {
    setHostFilter(nextHostId);
    if (
      nextHostId !== 'all' &&
      vmFilter !== 'all' &&
      !vms.some(vm => vm.id === vmFilter && vm.hostId === nextHostId)
    ) {
      setVMFilter('all');
    }
  }

  return (
    <div data-cmp="Snapshots" className="space-y-6 p-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="text-lg font-semibold" style={{ color: 'var(--kvm-text)' }}>
            快照
          </h1>
          <p className="mt-1 text-sm" style={{ color: 'var(--kvm-text-muted)' }}>
            创建、恢复、删除和备注 VM 还原点
          </p>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <SelectMenu
            value={hostFilter}
            options={hostOptions}
            placeholder="选择宿主机"
            className="w-[min(280px,calc(100vw-48px))] sm:w-52"
            maxVisibleItems={5}
            onChange={handleHostFilterChange}
          />
          {canRefreshSnapshots && (
            <button
              type="button"
              onClick={() => void refreshSnapshots()}
              disabled={refreshing}
              className="kvm-action-button inline-flex h-10 items-center gap-2 rounded-lg border px-3 text-sm disabled:opacity-60"
              style={buttonStyle}
            >
              <RefreshCwIcon size={15} className={refreshing ? 'animate-spin' : ''} />
              刷新
            </button>
          )}
          {canCreateSnapshots && (
            <button
              type="button"
              onClick={() => setDialog({ type: 'create' })}
              className="kvm-action-button inline-flex h-10 items-center gap-2 rounded-lg border px-3 text-sm font-semibold"
              style={primaryButtonStyle}
            >
              <PlusIcon size={15} />
              创建快照
            </button>
          )}
        </div>
      </div>

      <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
        <SummaryCard
          icon={CameraIcon}
          label="快照"
          value={`${summary.snapshots}`}
          color="#3b82f6"
        />
        <SummaryCard
          icon={ArchiveIcon}
          label="占用空间"
          value={formatBytes(summary.totalBytes, 'GB')}
          color="#f59e0b"
        />
      </div>
      {error && (
        <div
          className="rounded-xl p-4 text-sm"
          style={{
            background: 'rgba(245,158,11,0.1)',
            border: '1px solid rgba(245,158,11,0.25)',
            color: '#f59e0b',
          }}
        >
          {error}
        </div>
      )}
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
              value={query}
              onChange={event => setQuery(event.target.value)}
              placeholder="搜索快照、虚拟机、宿主机、IP 或标签"
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
            value={vmFilter}
            options={vmOptions}
            placeholder="选择虚拟机"
            className="w-[140px]"
            maxVisibleItems={6}
            onChange={setVMFilter}
          />
          {snapshotStatusFilters.map(item => (
            <button
              key={item.key}
              type="button"
              onClick={() => setStatusFilter(item.key)}
              className="kvm-action-button rounded-lg px-3 py-1.5 text-xs transition-colors"
              style={{
                background: statusFilter === item.key ? 'rgba(59,130,246,0.15)' : 'transparent',
                color: statusFilter === item.key ? '#3b82f6' : 'var(--kvm-text-muted)',
                border:
                  statusFilter === item.key
                    ? '1px solid rgba(59,130,246,0.3)'
                    : '1px solid transparent',
              }}
            >
              {item.label} ({statusCounts[item.key]})
            </button>
          ))}
        </div>
      </div>

      <SnapshotTable
        snapshots={filteredSnapshots}
        hosts={hosts}
        loading={loading}
        actionId={actionId}
        canUpdate={canUpdateSnapshots}
        canRevert={canRevertSnapshots}
        canDelete={canDeleteSnapshots}
        onDetail={snapshot => setDialog({ type: 'detail', snapshot })}
        onEdit={snapshot => setDialog({ type: 'edit', snapshot })}
        onConfirm={(snapshot, action) => setDialog({ type: 'confirm', snapshot, action })}
      />
      {dialog?.type === 'create' && (
        <CreateSnapshotDialog
          vms={vms}
          busy={actionId === 'create'}
          onClose={() => setDialog(null)}
          onSubmit={handleCreate}
        />
      )}
      {dialog?.type === 'detail' && (
        <SnapshotDetailDialog
          snapshot={dialog.snapshot}
          hosts={hosts}
          canUpdate={canUpdateSnapshots}
          onClose={() => setDialog(null)}
          onEdit={() => setDialog({ type: 'edit', snapshot: dialog.snapshot })}
        />
      )}
      {dialog?.type === 'edit' && (
        <EditSnapshotDialog
          snapshot={dialog.snapshot}
          busy={actionId === `${dialog.snapshot.id}:edit`}
          onClose={() => setDialog(null)}
          onSubmit={payload => void handleSaveAnnotation(dialog.snapshot, payload)}
        />
      )}
      {dialog?.type === 'confirm' && (
        <ConfirmDialog
          snapshot={dialog.snapshot}
          action={dialog.action}
          busy={actionId === `${dialog.snapshot.id}:${dialog.action}`}
          onClose={() => setDialog(null)}
          onConfirm={() => void handleSnapshotAction(dialog.snapshot, dialog.action)}
        />
      )}
    </div>
  );
}

function SnapshotTable({
  snapshots,
  hosts,
  loading,
  actionId,
  canUpdate,
  canRevert,
  canDelete,
  onDetail,
  onEdit,
  onConfirm,
}: {
  snapshots: Snapshot[];
  hosts: Host[];
  loading: boolean;
  actionId: string;
  canUpdate: boolean;
  canRevert: boolean;
  canDelete: boolean;
  onDetail: (snapshot: Snapshot) => void;
  onEdit: (snapshot: Snapshot) => void;
  onConfirm: (snapshot: Snapshot, action: 'revert' | 'delete') => void;
}) {
  return (
    <div
      className="overflow-hidden rounded-xl"
      style={{ background: 'var(--kvm-card)', border: '1px solid var(--kvm-border)' }}
    >
      <div data-snapshot-table-scroll className="overflow-x-auto">
        <table className="w-full text-center text-sm">
          <thead>
            <tr style={{ borderBottom: '1px solid var(--kvm-border)' }}>
              {['名称', '类型', '虚拟机', '宿主机', '标签', '状态', '创建时间', '操作'].map(
                head => (
                  <th
                    key={head}
                    className="px-4 py-3 text-center text-xs font-semibold"
                    style={{ color: 'var(--kvm-text-muted)' }}
                  >
                    {head}
                  </th>
                )
              )}
            </tr>
          </thead>
          <tbody>
            {loading && snapshots.length === 0 && <Empty colSpan={8} text="正在加载快照..." />}
            {!loading && snapshots.length === 0 && <Empty colSpan={8} text="暂无快照" />}
            {snapshots.map(item => (
              <tr key={item.id} style={{ borderBottom: '1px solid rgba(56,78,120,0.16)' }}>
                <td className="px-4 py-3 text-center">
                  <div className="font-mono font-medium" style={{ color: 'var(--kvm-text)' }}>
                    {snapshotTitle(item)}
                  </div>
                </td>
                <td className="px-4 py-3 text-center">
                  <span
                    className="inline-flex items-center justify-center gap-1 text-xs"
                    style={{ color: item.type === 'snapshot' ? '#3b82f6' : '#06b6d4' }}
                  >
                    {item.type === 'snapshot' ? (
                      <CameraIcon size={13} />
                    ) : (
                      <ArchiveIcon size={13} />
                    )}
                    {item.type === 'snapshot' ? '快照' : '备份'}
                  </span>
                </td>
                <td className="px-4 py-3 text-center" style={{ color: 'var(--kvm-text-muted)' }}>
                  {item.vmName || '-'}
                </td>
                <td
                  className="px-4 py-3 text-center font-mono text-xs"
                  style={{ color: 'var(--kvm-text-muted)' }}
                >
                  {snapshotHostAddress(item, hosts)}
                </td>
                <td className="px-4 py-3 text-center">
                  <TagList tags={item.tags || []} />
                </td>
                <td className="px-4 py-3 text-center">
                  <StatusBadge status={item.status} />
                </td>
                <td
                  className="px-4 py-3 text-center text-xs"
                  style={{ color: 'var(--kvm-text-muted)' }}
                >
                  {formatTimeAgo(item.created_at)}
                </td>
                <td className="px-4 py-3 text-center">
                  <div className="flex items-center justify-center gap-1">
                    <IconButton label="详情" onClick={() => onDetail(item)}>
                      <EyeIcon size={13} />
                    </IconButton>
                    {canUpdate && (
                      <IconButton label="编辑备注" onClick={() => onEdit(item)}>
                        <PencilIcon size={13} />
                      </IconButton>
                    )}
                    {canRevert && (
                      <IconButton
                        label="恢复"
                        disabled={actionId !== ''}
                        onClick={() => onConfirm(item, 'revert')}
                      >
                        <RotateCcwIcon size={13} />
                      </IconButton>
                    )}
                    {canDelete && (
                      <IconButton
                        label="删除"
                        danger
                        disabled={actionId !== ''}
                        onClick={() => onConfirm(item, 'delete')}
                      >
                        <Trash2Icon size={13} />
                      </IconButton>
                    )}
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}
function Empty({ text, colSpan = 7 }: { text: string; colSpan?: number }) {
  return (
    <tr>
      <td
        colSpan={colSpan}
        className="px-4 py-10 text-center"
        style={{ color: 'var(--kvm-text-muted)' }}
      >
        {text}
      </td>
    </tr>
  );
}
function SummaryCard({
  icon: Icon,
  label,
  value,
  color,
}: {
  icon: React.ElementType;
  label: string;
  value: string;
  color: string;
}) {
  return (
    <div
      className="flex items-center justify-between rounded-xl p-5"
      style={{ background: 'var(--kvm-card)', border: '1px solid var(--kvm-border)' }}
    >
      <div>
        <div
          className="text-xs uppercase tracking-widest"
          style={{ color: 'var(--kvm-text-muted)' }}
        >
          {label}
        </div>
        <div className="mt-1 font-mono text-2xl font-semibold" style={{ color: 'var(--kvm-text)' }}>
          {value}
        </div>
      </div>
      <Icon size={30} style={{ color, opacity: 0.8 }} />
    </div>
  );
}
function TagList({ tags }: { tags: string[] }) {
  if (!tags.length) return <span style={{ color: 'var(--kvm-text-muted)' }}>-</span>;
  return (
    <div className="mx-auto flex max-w-[220px] flex-wrap justify-center gap-1">
      {tags.slice(0, 3).map(tag => (
        <span
          key={tag}
          className="rounded-md px-2 py-1 text-[11px]"
          style={{
            background: 'rgba(59,130,246,0.1)',
            border: '1px solid rgba(59,130,246,0.22)',
            color: '#93c5fd',
          }}
        >
          {tag}
        </span>
      ))}
      {tags.length > 3 && (
        <span className="text-xs" style={{ color: 'var(--kvm-text-muted)' }}>
          +{tags.length - 3}
        </span>
      )}
    </div>
  );
}

function IconButton({
  children,
  label,
  danger,
  disabled,
  onClick,
}: {
  children: React.ReactNode;
  label: string;
  danger?: boolean;
  disabled?: boolean;
  onClick?: () => void;
}) {
  const triggerRef = useRef<HTMLButtonElement | null>(null);
  const [tooltip, setTooltip] = useState({
    open: false,
    top: 0,
    left: 0,
    placement: 'top' as 'top' | 'bottom',
  });
  function showTooltip() {
    const trigger = triggerRef.current;
    if (!trigger) return;
    const rect = trigger.getBoundingClientRect();
    const scrollBox = trigger.closest('[data-snapshot-table-scroll]')?.getBoundingClientRect();
    const topLimit = scrollBox?.top ?? 0;
    const bottomLimit = scrollBox?.bottom ?? window.innerHeight;
    const estimatedHeight = 42;
    const gap = 8;
    const topSpace = rect.top - topLimit;
    const bottomSpace = bottomLimit - rect.bottom;
    const placement = topSpace < estimatedHeight + gap && bottomSpace > topSpace ? 'bottom' : 'top';
    const left = Math.min(window.innerWidth - 72, Math.max(72, rect.left + rect.width / 2));
    const top = placement === 'top' ? rect.top - gap : rect.bottom + gap;
    setTooltip({ open: true, top, left, placement });
  }
  const bubble = tooltip.open && (
    <div
      className="pointer-events-none fixed z-[1500] -translate-x-1/2 text-left shadow-2xl"
      style={{
        left: tooltip.left,
        top: tooltip.top,
        transform: tooltip.placement === 'top' ? 'translate(-50%, -100%)' : 'translate(-50%, 0)',
      }}
    >
      <div
        className="whitespace-nowrap rounded-lg border px-2.5 py-1.5 text-xs font-semibold"
        style={{
          background: 'var(--kvm-popover-bg)',
          borderColor: danger ? 'rgba(239,68,68,0.46)' : 'var(--kvm-popover-border)',
          color: danger ? '#fca5a5' : 'var(--kvm-text)',
        }}
      >
        {label}
      </div>
    </div>
  );
  return (
    <>
      <button
        ref={triggerRef}
        type="button"
        aria-label={label}
        disabled={disabled}
        onMouseEnter={showTooltip}
        onMouseLeave={() => setTooltip(current => ({ ...current, open: false }))}
        onFocus={showTooltip}
        onBlur={() => setTooltip(current => ({ ...current, open: false }))}
        onClick={onClick}
        className={`kvm-action-button ${danger ? 'kvm-danger-button' : ''} flex h-8 w-8 items-center justify-center rounded-md border disabled:cursor-not-allowed disabled:opacity-35`}
        style={{
          borderColor: danger ? 'rgba(239,68,68,0.35)' : 'var(--kvm-border)',
          color: danger ? '#ef4444' : 'var(--kvm-text-muted)',
          background: 'rgba(255,255,255,0.03)',
        }}
      >
        {children}
      </button>
      {bubble ? createPortal(bubble, document.body) : null}
    </>
  );
}

function DialogFrame({
  title,
  onClose,
  children,
}: {
  title: string;
  onClose: () => void;
  children: React.ReactNode;
}) {
  return (
    <DialogPortal>
      <div className="kvm-dialog-backdrop fixed inset-0 z-50 flex items-center justify-center px-4">
        <div className="kvm-dialog-panel w-full max-w-xl rounded-xl p-5 shadow-2xl">
          <div className="mb-4 flex items-center justify-between">
            <h3 className="text-base font-semibold" style={{ color: 'var(--kvm-text)' }}>
              {title}
            </h3>
            <button
              type="button"
              onClick={onClose}
              className="kvm-action-button flex h-8 w-8 items-center justify-center rounded-lg border"
              style={{
                borderColor: 'var(--kvm-border)',
                color: 'var(--kvm-text-muted)',
                background: 'var(--kvm-control-bg-soft)',
              }}
              aria-label="关闭"
            >
              <XIcon size={15} />
            </button>
          </div>
          {children}
        </div>
      </div>
    </DialogPortal>
  );
}
function CreateSnapshotDialog({
  vms,
  busy,
  onClose,
  onSubmit,
}: {
  vms: VirtualMachine[];
  busy: boolean;
  onClose: () => void;
  onSubmit: (payload: { vmId: string; name: string; description: string; tags: string[] }) => void;
}) {
  const stoppedVMs = vms.filter(vm => isVMStopped(vm.status));
  const initialVM = stoppedVMs[0];
  const [vmId, setVMId] = useState(initialVM?.id ?? '');
  const [name, setName] = useState(() => defaultSnapshotName(initialVM));
  const [description, setDescription] = useState('');
  const [tags, setTags] = useState('');
  const currentVM = vms.find(vm => vm.id === vmId);
  const canSubmit = Boolean(currentVM && isVMStopped(currentVM.status) && name.trim());
  function handleVMChange(nextVMId: string) {
    const nextVM = vms.find(vm => vm.id === nextVMId);
    setVMId(nextVMId);
    setName(defaultSnapshotName(nextVM));
  }
  return (
    <DialogFrame title="创建内部快照" onClose={onClose}>
      <div className="space-y-3">
        <div className="block space-y-1.5 text-xs" style={{ color: 'var(--kvm-text-muted)' }}>
          <span>虚拟机</span>
          <SnapshotVMSelect vms={vms} value={vmId} onChange={handleVMChange} onlyStopped />
        </div>
        <FieldLabel label="快照名称">
          <input
            value={name}
            onChange={event => setName(event.target.value)}
            className="h-10 w-full rounded-lg px-3 text-sm outline-none"
            style={fieldStyle}
          />
        </FieldLabel>
        <FieldLabel label="描述">
          <textarea
            value={description}
            onChange={event => setDescription(event.target.value)}
            rows={3}
            className="w-full rounded-lg px-3 py-2 text-sm outline-none"
            style={fieldStyle}
          />
        </FieldLabel>
        <FieldLabel label="标签">
          <input
            value={tags}
            onChange={event => setTags(event.target.value)}
            placeholder="生产,关键业务"
            className="h-10 w-full rounded-lg px-3 text-sm outline-none"
            style={fieldStyle}
          />
        </FieldLabel>
        <div
          className="rounded-lg border px-3 py-2 text-xs leading-5"
          style={{
            background: 'rgba(59,130,246,0.08)',
            borderColor: 'rgba(59,130,246,0.24)',
            color: 'var(--kvm-accent-text)',
          }}
        >
          仅已关机的虚拟机可以创建内部快照，恢复时虚拟机会回滚到该快照对应状态
        </div>
        {stoppedVMs.length === 0 && (
          <div
            className="rounded-lg border px-3 py-2 text-xs leading-5"
            style={{
              background: 'rgba(245,158,11,0.08)',
              borderColor: 'rgba(245,158,11,0.28)',
              color: '#fbbf24',
            }}
          >
            当前没有已关机的虚拟机，请先关闭虚拟机后再创建快照
          </div>
        )}
        <DialogActions
          busy={busy}
          disabled={!canSubmit}
          confirm="创建"
          busyConfirm="创建中"
          onClose={onClose}
          onConfirm={() => onSubmit({ vmId, name, description, tags: parseTags(tags) })}
        />
      </div>
    </DialogFrame>
  );
}
function SnapshotVMSelect({
  vms,
  value,
  onChange,
  onlyStopped,
}: {
  vms: VirtualMachine[];
  value: string;
  onChange: (value: string) => void;
  onlyStopped?: boolean;
}) {
  const [open, setOpen] = useState(false);
  const rootRef = useRef<HTMLDivElement | null>(null);
  const current = vms.find(vm => vm.id === value);
  useEffect(() => {
    const close = (event: MouseEvent) => {
      if (!rootRef.current?.contains(event.target as Node)) setOpen(false);
    };
    window.addEventListener('mousedown', close);
    return () => window.removeEventListener('mousedown', close);
  }, []);
  return (
    <div ref={rootRef} className="relative">
      <button
        type="button"
        onClick={() => setOpen(next => !next)}
        className="kvm-action-button flex h-10 w-full cursor-pointer items-center justify-between gap-3 rounded-lg border px-3 text-left"
        style={{
          background: 'rgba(255,255,255,0.045)',
          borderColor: open ? 'rgba(45,212,191,0.45)' : 'var(--kvm-border)',
          color: current ? 'var(--kvm-text)' : 'var(--kvm-text-muted)',
          boxShadow: open ? '0 0 0 3px rgba(45,212,191,0.08)' : 'none',
        }}
        aria-haspopup="listbox"
        aria-expanded={open}
      >
        <span className="truncate text-sm font-semibold">
          {current?.name || (vms.length ? '请选择虚拟机' : '暂无虚拟机')}
        </span>
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
          className="kvm-hidden-scrollbar absolute left-0 top-[calc(100%+8px)] z-[60] max-h-64 w-full overflow-y-auto rounded-lg border p-1 shadow-2xl"
          role="listbox"
          style={{
            background: 'var(--kvm-menu-bg)',
            borderColor: 'var(--kvm-popover-border)',
            boxShadow: 'var(--kvm-menu-shadow)',
          }}
        >
          <button
            type="button"
            role="option"
            aria-selected={value === ''}
            onClick={() => {
              onChange('');
              setOpen(false);
            }}
            className="group flex h-10 w-full cursor-pointer items-center justify-between gap-3 rounded-md px-3 text-left text-sm font-semibold transition-colors hover:bg-[rgba(45,212,191,0.1)]"
            style={{
              background: value === '' ? 'rgba(45,212,191,0.14)' : undefined,
              color: value === '' ? '#99f6e4' : 'var(--kvm-text-muted)',
            }}
          >
            <span>请选择虚拟机</span>
          </button>
          {vms.map(vm => {
            const active = vm.id === value;
            const disabled = Boolean(onlyStopped && !isVMStopped(vm.status));
            return (
              <button
                key={vm.id}
                type="button"
                role="option"
                aria-selected={active}
                disabled={disabled}
                onClick={() => {
                  if (disabled) return;
                  onChange(vm.id);
                  setOpen(false);
                }}
                className="group flex h-10 w-full cursor-pointer items-center justify-between gap-3 rounded-md px-3 text-left text-sm font-semibold transition-colors hover:bg-[rgba(45,212,191,0.1)] disabled:cursor-not-allowed disabled:opacity-45"
                style={{
                  background: active ? 'rgba(45,212,191,0.14)' : undefined,
                  color: active ? '#99f6e4' : 'var(--kvm-text)',
                }}
              >
                <span className="min-w-0 flex-1 truncate">{vm.name}</span>
                <span
                  className="shrink-0 text-[11px] font-medium"
                  style={{ color: vm.status === 'running' ? '#86efac' : 'var(--kvm-text-muted)' }}
                >
                  {labelVMStatus(vm.status)}
                </span>
              </button>
            );
          })}
        </div>
      )}
    </div>
  );
}
function EditSnapshotDialog({
  snapshot,
  busy,
  onClose,
  onSubmit,
}: {
  snapshot: Snapshot;
  busy: boolean;
  onClose: () => void;
  onSubmit: (payload: { displayName: string; description: string; tags: string[] }) => void;
}) {
  const [displayName, setDisplayName] = useState(snapshot.displayName || '');
  const [description, setDescription] = useState(snapshot.description || '');
  const [tags, setTags] = useState((snapshot.tags || []).join(','));
  return (
    <DialogFrame title="编辑快照备注" onClose={onClose}>
      <div className="space-y-3">
        <FieldLabel label="显示名称">
          <input
            value={displayName}
            onChange={event => setDisplayName(event.target.value)}
            placeholder={snapshot.name}
            className="h-10 w-full rounded-lg px-3 text-sm outline-none"
            style={fieldStyle}
          />
        </FieldLabel>
        <FieldLabel label="描述">
          <textarea
            value={description}
            onChange={event => setDescription(event.target.value)}
            rows={3}
            className="w-full rounded-lg px-3 py-2 text-sm outline-none"
            style={fieldStyle}
          />
        </FieldLabel>
        <FieldLabel label="标签">
          <input
            value={tags}
            onChange={event => setTags(event.target.value)}
            placeholder="生产,关键业务"
            className="h-10 w-full rounded-lg px-3 text-sm outline-none"
            style={fieldStyle}
          />
        </FieldLabel>
        <DialogActions
          busy={busy}
          confirm="保存"
          busyConfirm="保存中"
          onClose={onClose}
          onConfirm={() => onSubmit({ displayName, description, tags: parseTags(tags) })}
        />
      </div>
    </DialogFrame>
  );
}
function SnapshotDetailDialog({
  snapshot,
  hosts,
  canUpdate,
  onClose,
  onEdit,
}: {
  snapshot: Snapshot;
  hosts: Host[];
  canUpdate: boolean;
  onClose: () => void;
  onEdit: () => void;
}) {
  const rows = [
    ['名称', snapshotTitle(snapshot)],
    ['原始名称', snapshot.name],
    ['虚拟机', snapshot.vmName],
    ['宿主机', snapshotHostDetail(snapshot, hosts)],
    ['状态', snapshot.status],
    ['大小', formatBytes(snapshot.sizeBytes, 'GB')],
    ['创建时间', new Date(snapshot.created_at).toLocaleString()],
    ['描述', snapshot.description || '-'],
  ];
  return (
    <DialogFrame title="快照详情" onClose={onClose}>
      <div className="kvm-detail-tile-grid grid grid-cols-1 gap-3 md:grid-cols-2">
        {rows.map(([label, value]) => (
          <div
            key={label}
            className="rounded-lg border p-3"
            style={{ borderColor: 'var(--kvm-border)', background: 'var(--kvm-control-bg-soft)' }}
          >
            <div className="mb-2 text-xs" style={{ color: 'var(--kvm-text-muted)' }}>
              {label}
            </div>
            <div
              className="min-w-0 break-words font-mono text-sm font-semibold"
              style={{ color: 'var(--kvm-text)' }}
            >
              {value}
            </div>
          </div>
        ))}
      </div>
      <div className="mt-4">
        <TagList tags={snapshot.tags || []} />
      </div>
      {canUpdate && (
        <div className="mt-5 flex justify-end">
          <button
            type="button"
            onClick={onEdit}
            className="kvm-action-button rounded-lg border px-3 py-2 text-sm"
            style={{
              borderColor: 'var(--kvm-border)',
              color: 'var(--kvm-text)',
              background: 'var(--kvm-control-bg-soft)',
            }}
          >
            编辑备注
          </button>
        </div>
      )}
    </DialogFrame>
  );
}
function ConfirmDialog({
  snapshot,
  action,
  busy,
  onClose,
  onConfirm,
}: {
  snapshot: Snapshot;
  action: 'revert' | 'delete';
  busy: boolean;
  onClose: () => void;
  onConfirm: () => void;
}) {
  const danger = action === 'delete';
  return (
    <DialogFrame title={danger ? '删除快照' : '恢复快照'} onClose={onClose}>
      <p className="text-sm leading-6" style={{ color: 'var(--kvm-text-muted)' }}>
        {danger
          ? '删除后无法从平台恢复该快照。'
          : '恢复会将虚拟机回滚到该快照对应状态，请确认业务已经停止或可接受回滚。'}
      </p>
      <div
        className="kvm-dialog-card mt-4 rounded-lg p-3 text-sm"
        style={{ color: 'var(--kvm-text)' }}
      >
        {snapshotTitle(snapshot)} · {snapshot.vmName}
      </div>
      <DialogActions
        busy={busy}
        confirm={danger ? '删除' : '恢复'}
        busyConfirm={danger ? '删除中' : '恢复中'}
        danger={danger}
        onClose={onClose}
        onConfirm={onConfirm}
      />
    </DialogFrame>
  );
}
function FieldLabel({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <label className="block space-y-1.5 text-xs" style={{ color: 'var(--kvm-text-muted)' }}>
      <span>{label}</span>
      {children}
    </label>
  );
}
function DialogActions({
  busy,
  confirm,
  busyConfirm,
  danger,
  disabled,
  onClose,
  onConfirm,
}: {
  busy: boolean;
  confirm: string;
  busyConfirm?: string;
  danger?: boolean;
  disabled?: boolean;
  onClose: () => void;
  onConfirm: () => void;
}) {
  return (
    <div className="mt-5 flex justify-end gap-2">
      <button
        type="button"
        onClick={onClose}
        disabled={busy}
        className="kvm-action-button rounded-lg border px-3 py-2 text-sm disabled:cursor-not-allowed disabled:opacity-50"
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
        disabled={busy || disabled}
        onClick={onConfirm}
        className="kvm-action-button inline-flex min-w-20 items-center justify-center gap-2 rounded-lg border px-3 py-2 text-sm disabled:cursor-not-allowed disabled:opacity-50"
        style={{
          borderColor: danger ? 'rgba(239,68,68,0.38)' : 'rgba(59,130,246,0.38)',
          color: danger ? '#f87171' : 'var(--kvm-accent-text)',
          background: danger ? 'rgba(239,68,68,0.08)' : 'rgba(59,130,246,0.1)',
        }}
      >
        {busy && <RefreshCwIcon size={14} className="animate-spin" />}
        {busy ? busyConfirm || `${confirm}中` : confirm}
      </button>
    </div>
  );
}
const fieldStyle = {
  background: 'var(--kvm-control-bg)',
  border: '1px solid var(--kvm-border)',
  color: 'var(--kvm-text)',
};
const buttonStyle = {
  background: 'var(--kvm-control-bg)',
  borderColor: 'var(--kvm-border)',
  color: 'var(--kvm-text)',
};
const primaryButtonStyle = {
  background: 'rgba(59,130,246,0.15)',
  borderColor: 'rgba(59,130,246,0.42)',
  color: 'var(--kvm-accent-text)',
};
function defaultSnapshotName(vm?: VirtualMachine) {
  return `snap-${snapshotNamePart(vm?.name || 'vm')}-${localTimestamp()}`;
}
function snapshotNamePart(value: string) {
  return (
    value
      .trim()
      .replace(/[^A-Za-z0-9._:-]+/g, '-')
      .replace(/^-+|-+$/g, '') || 'vm'
  );
}
function localTimestamp() {
  const date = new Date();
  const pad = (value: number) => String(value).padStart(2, '0');
  return `${date.getFullYear()}${pad(date.getMonth() + 1)}${pad(date.getDate())}${pad(date.getHours())}${pad(date.getMinutes())}`;
}
function labelVMStatus(status: string) {
  return (
    { running: '运行中', paused: '已暂停', stopped: '已关机', error: '异常' }[status] ?? status
  );
}
function parseTags(value: string) {
  return value
    .split(',')
    .map(tag => tag.trim())
    .filter(Boolean);
}
function snapshotTitle(snapshot: Snapshot) {
  return snapshot.displayName || snapshot.name;
}
function snapshotHostAddress(snapshot: Snapshot, hosts: Host[]) {
  return hosts.find(host => host.id === snapshot.hostId)?.address || snapshot.hostName || '-';
}
function snapshotHostDetail(snapshot: Snapshot, hosts: Host[]) {
  const address = snapshotHostAddress(snapshot, hosts);
  return address && address !== snapshot.hostName
    ? `${snapshot.hostName || '-'} · ${address}`
    : snapshot.hostName || address || '-';
}
function isPermissionMessage(message: string) {
  return message.includes('当前用户无权执行此操作');
}
