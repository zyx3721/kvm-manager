import { useEffect, useMemo, useState, type ReactNode } from 'react';
import {
  CopyIcon,
  Disc3Icon,
  HardDriveIcon,
  LoaderCircleIcon,
  SearchIcon,
  Trash2Icon,
  UploadIcon,
  XIcon,
} from 'lucide-react';
import { toast } from 'sonner';

import { DialogPortal } from '../../../components/kvm/DialogPortal';
import { SelectMenu } from '../../../components/kvm/SelectMenu';
import { KvmTooltip } from '../../../components/kvm/StatusBadge';
import {
  createStorageVolume,
  deleteStorageVolume,
  deleteStoragePool,
  fetchStorageVolumes,
  type StoragePool,
  type StorageVolume,
  type StorageVolumeClonePayload,
} from '../../../lib/api';
import { formatBytes, formatBytesAuto, formatBytesAutoFixed } from '../../../lib/format';
import { AutostartBadge, isPoolActive, StateBadge } from './PoolBadges';
import { ISOUploadDialog } from './ISOUploadDialog';
import { friendlyVolumeError, volumeNameExists } from './storagePoolErrors';
import { buttonStyle, primaryButtonStyle } from './storagePoolStyles';
import { VolumeCloneDialog } from './VolumeCloneDialog';
import { runStorageISOUpload, runStorageVolumeClone } from '../utils/uploadStorageISO';
import { storageUsageColor } from '../utils/storageUsage';

type StoragePoolDetailDialogProps = {
  agentId: string;
  hostName: string;
  item: StoragePool;
  canManage: boolean;
  onClose: () => void;
  onRefresh: () => Promise<void>;
  onUpdateState: (active: boolean) => Promise<unknown>;
  onUpdateAutostart: (autostart: boolean) => Promise<unknown>;
  onDeleted: () => Promise<void>;
};

export function StoragePoolDetailDialog({ agentId, hostName, item, canManage, onClose, onRefresh, onUpdateState, onUpdateAutostart, onDeleted }: StoragePoolDetailDialogProps) {
  const [localItem, setLocalItem] = useState(item);
  const [busy, setBusy] = useState('');
  const [volumes, setVolumes] = useState<StorageVolume[]>([]);
  const [volumeLoading, setVolumeLoading] = useState(true);
  const [deleteName, setDeleteName] = useState('');
  const [confirmVolume, setConfirmVolume] = useState<StorageVolume | null>(null);
  const [confirmPoolDelete, setConfirmPoolDelete] = useState(false);
  const [uploadOpen, setUploadOpen] = useState(false);
  const [cloneVolume, setCloneVolume] = useState<StorageVolume | null>(null);
  const [volumeQuery, setVolumeQuery] = useState('');
  const active = isPoolActive(localItem.state);
  const usage = localItem.capacity > 0 ? Math.round((localItem.allocation * 100) / localItem.capacity) : 0;
  const volumeKind = isIsoPool(localItem) ? '光盘镜像' : '卷';
  const uploadText = isIsoPool(localItem) ? '上传ISO' : '添加镜像';
  const filteredVolumes = useMemo(() => {
    const query = volumeQuery.trim().toLowerCase();
    if (!query) return volumes;
    return volumes.filter(volume =>
      [volume.name, volume.path, volume.type, volume.format, formatBytesAutoFixed(volume.capacity)]
        .some(value => String(value || '').toLowerCase().includes(query))
    );
  }, [volumeQuery, volumes]);

  useEffect(() => {
    setLocalItem(item);
  }, [item]);

  async function loadVolumes() {
    if (!isPoolActive(localItem.state)) {
      setVolumes([]);
      setVolumeLoading(false);
      return;
    }
    setVolumeLoading(true);
    try {
      const body = await fetchStorageVolumes(agentId, localItem.name);
      setVolumes(body.items);
    } catch (error) {
      setVolumes([]);
      toast.error(error instanceof Error ? error.message : '读取存储卷失败');
    } finally {
      setVolumeLoading(false);
    }
  }

  async function run(action: string, task: () => Promise<unknown>) {
    setBusy(action);
    try {
      const result = await task();
      if (action === 'state') {
        const activeResult = Boolean((result as { active?: boolean } | undefined)?.active);
        setLocalItem(current => ({ ...current, state: activeResult ? 'running' : 'inactive' }));
      }
      if (action === 'autostart') {
        const autostartResult = Boolean((result as { autostart?: boolean } | undefined)?.autostart);
        setLocalItem(current => ({ ...current, autostart: autostartResult }));
      }
      toast.success('存储池配置已更新');
      await onRefresh();
    } catch (error) {
      toast.error(error instanceof Error ? error.message : '修改存储池失败');
    } finally {
      setBusy('');
    }
  }

  async function removeVolume(volume: StorageVolume) {
    setDeleteName(volume.name);
    try {
      await deleteStorageVolume(agentId, localItem.name, volume.name);
      toast.success(isIsoPool(localItem) ? '镜像已删除' : '存储卷已删除');
      await loadVolumes();
      await onRefresh();
    } catch (error) {
      toast.error(friendlyVolumeError(error, '删除存储卷失败'));
    } finally {
      setDeleteName('');
      setConfirmVolume(null);
    }
  }

  async function removePool() {
    setBusy('delete');
    try {
      await deleteStoragePool(agentId, localItem.name);
      toast.success('存储池已删除');
      await onDeleted();
      onClose();
    } catch (error) {
      toast.error(error instanceof Error ? error.message : '删除存储池失败');
    } finally {
      setBusy('');
      setConfirmPoolDelete(false);
    }
  }

  async function uploadISO(file: File, name: string) {
    const targetName = name.trim() || file.name;
    if (!targetName.toLowerCase().endsWith('.iso')) {
      toast.error('ISO 名称必须以 .iso 结尾');
      return;
    }
    if (volumeNameExists(volumes, targetName)) {
      toast.error('镜像名称已存在，请更换名称');
      return;
    }
    setUploadOpen(false);
    setBusy('upload');
    try {
      const completed = await runStorageISOUpload({ agentId, poolName: localItem.name, file, targetName });
      if (completed) {
        await loadVolumes();
        await onRefresh();
      }
    } finally {
      setBusy('');
    }
  }

  async function createVolume(payload: { name: string; format: string; capacityBytes: number; preallocMetadata?: boolean }) {
    setBusy('create-volume');
    try {
      await createStorageVolume(agentId, localItem.name, payload);
      toast.success('镜像已添加');
      setUploadOpen(false);
      await loadVolumes();
      await onRefresh();
    } catch (error) {
      toast.error(friendlyVolumeError(error, '添加镜像失败'));
    } finally {
      setBusy('');
    }
  }

  async function cloneVolumeTo(payload: StorageVolumeClonePayload) {
    if (volumeNameExists(volumes, payload.name)) {
      toast.error('镜像名称已存在，请更换名称');
      return;
    }
    setCloneVolume(null);
    setBusy('clone-volume');
    try {
      const completed = await runStorageVolumeClone({ agentId, poolName: localItem.name, payload });
      if (completed) {
        await loadVolumes();
        await onRefresh();
      }
    } finally {
      setBusy('');
    }
  }

  useEffect(() => {
    void loadVolumes();
  }, [agentId, localItem.name, localItem.state]);

  const detailRows = useMemo(
    () => [
      { label: '池名称', value: localItem.name },
      { label: '池类别', value: localItem.type || 'unknown' },
      { label: '池路径', value: localItem.path || '-' },
      {
        label: '容量',
        value: localItem.capacity > 0
          ? <CapacityValue capacity={localItem.capacity} available={localItem.available} allocation={localItem.allocation} />
          : '-',
      },
      { label: '用量', value: `${usage}%` },
      { label: '卷数量', value: `${volumes.length}` },
    ],
    [localItem, usage, volumes.length]
  );

  return (
    <DialogPortal>
    <div className="kvm-dialog-backdrop fixed inset-0 z-50 flex items-center justify-center px-3">
      <div className="kvm-dialog-panel max-h-[92vh] w-[min(94vw,980px)] overflow-hidden rounded-2xl">
        <header className="flex items-center justify-between border-b px-5 py-4" style={{ borderColor: 'var(--kvm-border)' }}>
          <div className="min-w-0">
            <h2 className="truncate text-base font-semibold" style={{ color: 'var(--kvm-text)' }}>{localItem.name}</h2>
            <p className="mt-1 text-xs" style={{ color: 'var(--kvm-text-muted)' }}>{hostName} · {localItem.type || 'unknown'}</p>
          </div>
          <button type="button" onClick={onClose} aria-label="关闭" className="kvm-action-button inline-flex h-8 w-8 items-center justify-center rounded-lg border" style={buttonStyle}><XIcon size={15} /></button>
        </header>

        <div className="kvm-hidden-scrollbar max-h-[calc(92vh-74px)] overflow-y-auto p-5">
          <section className="kvm-dialog-card rounded-xl p-4">
            <div className="kvm-detail-tile-grid grid gap-3 md:grid-cols-2 xl:grid-cols-3">
              {detailRows.map(row => <InfoTile key={row.label} label={row.label} value={row.value} />)}
              {canManage && <ActionTile
                label="状态"
                value={<StateBadge active={active} />}
                buttonLabel={active ? '停止' : '启动'}
                buttonTone={active ? 'danger' : 'success'}
                disabled={busy !== ''}
                onClick={() => void run('state', () => onUpdateState(!active))}
                extra={
                  <DeletePoolButton
                    active={active}
                    disabled={busy !== ''}
                    onClick={() => setConfirmPoolDelete(true)}
                  />
                }
              />}
              {canManage && <ActionTile
                label="随物理机同启"
                value={<AutostartBadge enabled={localItem.autostart} />}
                buttonLabel={localItem.autostart ? '禁用' : '启用'}
                buttonTone={localItem.autostart ? 'warning' : 'primary'}
                disabled={busy !== ''}
                onClick={() => void run('autostart', () => onUpdateAutostart(!localItem.autostart))}
              />}
            </div>
            <div className="mt-4 h-2 overflow-hidden rounded-full" style={{ background: 'rgba(148,163,184,0.16)' }}>
              <div className="h-full rounded-full" style={{ width: `${Math.min(100, usage)}%`, background: storageUsageColor(usage) }} />
            </div>
          </section>

          <section className="mt-4">
            <div className="mb-3 flex flex-wrap items-center justify-between gap-3">
              <div className="flex items-center gap-2">
                {isIsoPool(localItem) ? <Disc3Icon size={18} style={{ color: '#60a5fa' }} /> : <HardDriveIcon size={18} style={{ color: '#2dd4bf' }} />}
                <h3 className="text-lg font-semibold" style={{ color: 'var(--kvm-text)' }}>{volumeKind}</h3>
              </div>
              <div className="flex flex-wrap items-center justify-end gap-2">
                <div className="relative w-[min(260px,calc(94vw-40px))]">
                  <SearchIcon size={14} className="absolute left-3 top-1/2 -translate-y-1/2" style={{ color: 'var(--kvm-text-muted)' }} />
                  <input value={volumeQuery} onChange={event => setVolumeQuery(event.target.value)} placeholder={`搜索${volumeKind}`} className="h-9 w-full rounded-lg pl-9 pr-9 text-sm outline-none" style={{ background: 'var(--kvm-control-bg)', border: '1px solid var(--kvm-border)', color: 'var(--kvm-text)' }} />
                  {volumeQuery && (
                    <button type="button" onClick={() => setVolumeQuery('')} aria-label="清空搜索" className="kvm-action-button absolute right-2 top-1/2 inline-flex h-5 w-5 -translate-y-1/2 items-center justify-center rounded-md" style={{ color: 'var(--kvm-text-muted)' }}>
                      <XIcon size={13} />
                    </button>
                  )}
                </div>
                {canManage && (
                  <button type="button" disabled={!active || busy !== ''} onClick={() => setUploadOpen(true)} className="kvm-action-button inline-flex h-9 items-center gap-2 rounded-lg border px-3 text-sm font-semibold disabled:cursor-not-allowed disabled:opacity-60" style={primaryButtonStyle} aria-label={uploadText}>
                    <UploadIcon size={15} />{uploadText}
                  </button>
                )}
              </div>
            </div>
            <VolumeTable
              volumes={filteredVolumes}
              loading={volumeLoading}
              inactive={!active}
              iso={isIsoPool(localItem)}
              emptyText={volumeQuery.trim() ? '没有匹配的结果' : undefined}
              deleteName={deleteName}
              cloneName={busy === 'clone-volume' ? cloneVolume?.name || '' : ''}
              canManage={canManage}
              onClone={setCloneVolume}
              onDelete={setConfirmVolume}
            />
          </section>
        </div>
      </div>
      {confirmVolume && (
        <ConfirmDialog
          title="删除存储卷"
          message={`确认删除 ${confirmVolume.name}？该操作不可撤销`}
          busy={deleteName === confirmVolume.name}
          onClose={() => setConfirmVolume(null)}
          onConfirm={() => void removeVolume(confirmVolume)}
        />
      )}
      {confirmPoolDelete && (
        <ConfirmDialog
          title="删除存储池"
          message={`确认删除存储池 ${localItem.name}？请确认该池已停止且不再使用`}
          busy={busy === 'delete'}
          onClose={() => setConfirmPoolDelete(false)}
          onConfirm={() => void removePool()}
        />
      )}
      {uploadOpen && isIsoPool(localItem) && (
        <ISOUploadDialog
          busy={busy === 'upload'}
          onClose={() => setUploadOpen(false)}
          onSubmit={(file, name) => void uploadISO(file, name)}
        />
      )}
      {uploadOpen && !isIsoPool(localItem) && (
        <VolumeCreateDialog
          busy={busy === 'create-volume'}
          onClose={() => setUploadOpen(false)}
          onSubmit={payload => void createVolume(payload)}
        />
      )}
      {cloneVolume && (
        <VolumeCloneDialog
          volume={cloneVolume}
          busy={busy === 'clone-volume'}
          onClose={() => setCloneVolume(null)}
          onSubmit={payload => void cloneVolumeTo(payload)}
        />
      )}
    </div>
    </DialogPortal>
  );
}

function InfoTile({ label, value }: { label: string; value: ReactNode }) {
  return (
    <div className="rounded-lg border p-3" style={{ borderColor: 'var(--kvm-border)', background: 'var(--kvm-control-bg-soft)' }}>
      <div className="text-xs" style={{ color: 'var(--kvm-text-muted)' }}>{label}</div>
      <div className="mt-2 min-w-0 break-all font-mono text-sm font-semibold" style={{ color: 'var(--kvm-text)' }}>{value}</div>
    </div>
  );
}

function CapacityValue({ capacity, available, allocation }: { capacity: number; available: number; allocation: number }) {
  return (
    <span className="inline-flex flex-wrap items-center gap-1.5">
      <CapacityPart label="总容量" value={capacity} />
      <span style={{ color: 'var(--kvm-text-muted)' }}>/</span>
      <CapacityPart label="可用容量" value={available} />
      <span style={{ color: 'var(--kvm-text-muted)' }}>/</span>
      <CapacityPart label="已用容量" value={allocation} />
    </span>
  );
}

function CapacityPart({ label, value }: { label: string; value: number }) {
  const text = formatBytes(value, 'GB', 1);
  return (
    <KvmTooltip label={`${label}：${text}`} placement="top">
      <span className="cursor-help rounded px-1 transition-colors hover:bg-[rgba(59,130,246,0.08)]">
        {text}
      </span>
    </KvmTooltip>
  );
}

function ActionTile({
  label,
  value,
  buttonLabel,
  buttonTone,
  disabled,
  onClick,
  extra,
}: {
  label: string;
  value: ReactNode;
  buttonLabel: string;
  buttonTone: 'primary' | 'success' | 'warning' | 'danger';
  disabled: boolean;
  onClick: () => void;
  extra?: ReactNode;
}) {
  return (
    <div className="rounded-lg border p-3" style={{ borderColor: actionColor(buttonTone).border, background: actionColor(buttonTone).background }}>
      <div className="text-xs" style={{ color: 'var(--kvm-text-muted)' }}>{label}</div>
      <div className="mt-2 flex items-center justify-between gap-3">
        {value}
        <div className="flex shrink-0 items-center gap-2">
          <button type="button" disabled={disabled} onClick={onClick} className="kvm-action-button h-8 rounded-lg border px-3 text-xs font-semibold disabled:opacity-60" style={{ borderColor: actionColor(buttonTone).border, background: actionColor(buttonTone).buttonBg, color: actionColor(buttonTone).text }}>
            {buttonLabel}
          </button>
          {extra}
        </div>
      </div>
    </div>
  );
}

function DeletePoolButton({ active, disabled, onClick }: { active: boolean; disabled: boolean; onClick: () => void }) {
  const button = (
    <button type="button" disabled={active || disabled} onClick={onClick} className="kvm-action-button kvm-danger-button inline-flex h-8 items-center gap-1.5 rounded-lg border px-2.5 text-xs font-semibold disabled:cursor-not-allowed disabled:opacity-50" style={{ borderColor: 'rgba(239,68,68,0.35)', color: '#f87171', background: 'rgba(239,68,68,0.1)' }}>
      <Trash2Icon size={13} />删除
    </button>
  );
  if (!active) return button;
  return <KvmTooltip label="运行中的存储池需要先停止后才能删除" placement="top">{button}</KvmTooltip>;
}

type VolumeTableProps = {
  volumes: StorageVolume[];
  loading: boolean;
  inactive: boolean;
  iso: boolean;
  emptyText?: string;
  deleteName: string;
  cloneName: string;
  canManage: boolean;
  onClone: (volume: StorageVolume) => void;
  onDelete: (volume: StorageVolume) => void;
};

function VolumeTable({ volumes, loading, inactive, iso, emptyText, deleteName, cloneName, canManage, onClone, onDelete }: VolumeTableProps) {
  return (
    <div className="overflow-hidden rounded-xl border" style={{ borderColor: 'var(--kvm-border)', background: 'var(--kvm-card)' }}>
      <div className={'kvm-hidden-scrollbar overflow-x-auto ' + (volumes.length > 5 ? 'overflow-y-auto' : '')} style={volumes.length > 5 ? { maxHeight: '342px' } : undefined}>
        <table className="w-full min-w-[720px] border-collapse text-sm">
          <thead style={{ background: 'var(--kvm-table-head-bg)', color: 'var(--kvm-text)' }}>
            <tr>
              {['#', '名称', '容量', '格式', '执行'].map(head => (
                <th key={head} className="border-b px-3 py-3 text-center font-semibold" style={{ borderColor: 'var(--kvm-border)' }}>{head}</th>
              ))}
            </tr>
          </thead>
          <tbody style={{ color: 'var(--kvm-text-muted)' }}>
            {loading && <tr><td colSpan={5} className="px-3 py-8 text-center"><LoaderCircleIcon size={18} className="mx-auto animate-spin" /></td></tr>}
            {!loading && volumes.length === 0 && <tr><td colSpan={5} className="px-3 py-8 text-center">{emptyText || (inactive ? '存储池已停止，暂无卷列表' : '暂无任何卷')}</td></tr>}
            {!loading && volumes.map((volume, index) => (
              <tr key={volume.name} style={{ borderTop: '1px solid var(--kvm-border)' }}>
                <td className="px-3 py-3 text-center">{index + 1}</td>
                <td className="max-w-[420px] truncate px-3 py-3 text-center font-semibold" style={{ color: 'var(--kvm-text)' }}>{volume.name}</td>
                <td className="px-3 py-3 text-center">{formatBytesAutoFixed(volume.capacity)}</td>
                <td className="px-3 py-3 text-center">{volume.format || volume.type || '-'}</td>
                <td className="px-3 py-3 text-center">
                  <div className="flex justify-center gap-2">
                    {canManage && !iso && (
                      <KvmTooltip label={volume.cloneSupported ? '克隆当前镜像' : '该格式暂不支持克隆'} placement="top">
                        <button type="button" disabled={!volume.cloneSupported || cloneName === volume.name} onClick={() => onClone(volume)} className="kvm-action-button inline-flex h-8 items-center gap-1.5 rounded-lg border px-2.5 text-xs disabled:opacity-50" style={{ borderColor: 'rgba(59,130,246,0.34)', color: 'var(--kvm-status-blue-text)', background: 'rgba(59,130,246,0.1)' }}>
                          {cloneName === volume.name ? <LoaderCircleIcon size={13} className="animate-spin" /> : <CopyIcon size={13} />}克隆
                        </button>
                      </KvmTooltip>
                    )}
                    {canManage && (
                      <button type="button" disabled={!volume.deleteSupported || deleteName === volume.name} onClick={() => void onDelete(volume)} className="kvm-action-button kvm-danger-button inline-flex h-8 items-center gap-1.5 rounded-lg border px-2.5 text-xs disabled:opacity-50" style={{ borderColor: 'rgba(239,68,68,0.35)', color: '#f87171', background: 'rgba(239,68,68,0.1)' }}>
                        {deleteName === volume.name ? <LoaderCircleIcon size={13} className="animate-spin" /> : <Trash2Icon size={13} />}删除
                      </button>
                    )}
                    {!canManage && <span className="text-xs" style={{ color: 'var(--kvm-text-muted)' }}>只读</span>}
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

function VolumeCreateDialog({ busy, onClose, onSubmit }: { busy: boolean; onClose: () => void; onSubmit: (payload: { name: string; format: string; capacityBytes: number; preallocMetadata: boolean }) => void }) {
  const [name, setName] = useState('');
  const [format, setFormat] = useState('qcow2');
  const [capacity, setCapacity] = useState('20');
  const [unit, setUnit] = useState('GB');
  const [preallocMetadata, setPreallocMetadata] = useState(false);
  const bytes = volumeBytes(Number(capacity), unit);
  const metadataEnabled = format === 'qcow2' && preallocMetadata;
  const extension = volumeExtension(format);
  const volumeName = `${trimVolumeExtension(name.trim())}${extension}`;
  return (
    <DialogPortal>
    <div className="fixed inset-0 z-[70] flex items-center justify-center px-4" style={{ background: 'rgba(2,6,23,0.36)' }}>
      <div className="kvm-dialog-panel w-full max-w-md rounded-xl p-5">
        <DialogHeader title="添加镜像" subtitle="创建新的存储卷镜像" onClose={onClose} />
        <div className="mx-auto mt-5 max-w-sm space-y-4">
          <FormField label="名称"><input value={name} onChange={event => setName(event.target.value)} placeholder="disk-01" className="h-10 w-full rounded-lg px-3 text-sm outline-none" style={{ background: 'var(--kvm-control-bg)', border: '1px solid var(--kvm-border)', color: 'var(--kvm-text)' }} /></FormField>
          <FormField label="格式"><SelectMenu value={format} placeholder="请选择格式" options={['qcow2', 'qcow', 'qed', 'raw'].map(item => ({ value: item, label: item }))} onChange={setFormat} /></FormField>
          {format === 'qcow2' && (
            <label className="flex cursor-pointer items-center justify-between gap-3 rounded-lg border p-3 transition-colors hover:bg-[rgba(59,130,246,0.06)]" style={{ borderColor: 'var(--kvm-border)', background: 'var(--kvm-control-bg-soft)' }}>
              <span>
                <span className="block text-sm font-semibold" style={{ color: 'var(--kvm-text)' }}>预分配元数据</span>
                <span className="mt-1 block text-xs" style={{ color: 'var(--kvm-text-muted)' }}>创建 qcow2 镜像时启用 metadata 预分配</span>
              </span>
              <input type="checkbox" checked={preallocMetadata} onChange={event => setPreallocMetadata(event.target.checked)} className="h-4 w-4 cursor-pointer accent-blue-500" />
            </label>
          )}
          <FormField label="容量">
            <div className="grid grid-cols-[1fr_104px] gap-2">
              <input value={capacity} onChange={event => setCapacity(event.target.value)} type="number" min="1" className="h-10 w-full rounded-lg px-3 text-sm outline-none" style={{ background: 'var(--kvm-control-bg)', border: '1px solid var(--kvm-border)', color: 'var(--kvm-text)' }} />
              <SelectMenu value={unit} placeholder="单位" options={['MB', 'GB', 'TB'].map(item => ({ value: item, label: item }))} onChange={setUnit} />
            </div>
          </FormField>
          <div className="rounded-lg border p-3 text-xs" style={{ borderColor: 'var(--kvm-border)', background: 'var(--kvm-control-bg-soft)', color: 'var(--kvm-text-muted)' }}>
            {Number(capacity) > 0 ? `将创建 ${volumeName}，容量 ${formatBytesAuto(bytes)}${metadataEnabled ? '，并预分配元数据' : ''}` : '请输入容量'}
          </div>
        </div>
        <DialogFooter busy={busy} confirmLabel="创建" disabled={!trimVolumeExtension(name.trim()) || bytes <= 0} onClose={onClose} onConfirm={() => onSubmit({ name: volumeName, format, capacityBytes: bytes, preallocMetadata: metadataEnabled })} />
      </div>
    </div>
    </DialogPortal>
  );
}

function trimVolumeExtension(name: string) { return name.replace(/\.(qcow2|qcow|qed|raw|img)$/i, ''); }

function volumeExtension(format: string) { return format === 'qcow2' ? '.qcow2' : '.img'; }

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

function FormField({ label, children }: { label: string; children: ReactNode }) {
  return <label className="block"><span className="mb-2 block text-sm font-semibold" style={{ color: 'var(--kvm-text)' }}>{label}</span>{children}</label>;
}

function volumeBytes(value: number, unit: string) {
  if (!Number.isFinite(value) || value <= 0) return 0;
  const powers: Record<string, number> = { MB: 2, GB: 3, TB: 4 };
  return Math.round(value * 1024 ** (powers[unit] ?? 3));
}

function ConfirmDialog({ title, message, busy, onClose, onConfirm }: { title: string; message: string; busy: boolean; onClose: () => void; onConfirm: () => void }) {
  return (
    <DialogPortal>
    <div className="fixed inset-0 z-[70] flex items-center justify-center px-4" style={{ background: 'rgba(2,6,23,0.36)' }}>
      <div className="kvm-dialog-panel w-full max-w-sm rounded-xl p-5" style={{ borderColor: 'rgba(239,68,68,0.34)' }}>
        <div className="flex items-start justify-between gap-3">
          <div>
            <h3 className="text-base font-semibold" style={{ color: 'var(--kvm-text)' }}>{title}</h3>
            <p className="mt-2 text-sm leading-6" style={{ color: 'var(--kvm-text-muted)' }}>{message}</p>
          </div>
          <button type="button" onClick={onClose} aria-label="关闭" className="kvm-action-button inline-flex h-8 w-8 shrink-0 items-center justify-center rounded-lg border" style={buttonStyle}><XIcon size={15} /></button>
        </div>
        <div className="mt-5 flex justify-end gap-2">
          <button type="button" disabled={busy} onClick={onClose} className="kvm-action-button h-9 rounded-lg border px-4 text-sm disabled:opacity-60" style={buttonStyle}>取消</button>
          <button type="button" disabled={busy} onClick={onConfirm} className="kvm-action-button kvm-danger-button inline-flex h-9 items-center gap-2 rounded-lg border px-4 text-sm font-semibold disabled:opacity-60" style={{ borderColor: 'rgba(239,68,68,0.35)', color: '#f87171', background: 'rgba(239,68,68,0.12)' }}>
            {busy && <LoaderCircleIcon size={14} className="animate-spin" />}确认删除
          </button>
        </div>
      </div>
    </div>
    </DialogPortal>
  );
}

function actionColor(tone: 'primary' | 'success' | 'warning' | 'danger') {
  switch (tone) {
    case 'success':
      return { background: 'rgba(16,185,129,0.08)', buttonBg: 'rgba(16,185,129,0.12)', border: 'rgba(16,185,129,0.34)', text: 'var(--kvm-status-green-text)' };
    case 'warning':
      return { background: 'rgba(245,158,11,0.08)', buttonBg: 'rgba(245,158,11,0.12)', border: 'rgba(245,158,11,0.34)', text: 'var(--kvm-status-yellow-text)' };
    case 'danger':
      return { background: 'rgba(239,68,68,0.08)', buttonBg: 'rgba(239,68,68,0.12)', border: 'rgba(239,68,68,0.34)', text: 'var(--kvm-status-red-text)' };
    default:
      return { background: 'rgba(59,130,246,0.08)', buttonBg: 'rgba(59,130,246,0.12)', border: 'rgba(59,130,246,0.34)', text: 'var(--kvm-status-blue-text)' };
  }
}

function isIsoPool(item: StoragePool) { return item.type.toLowerCase() === 'iso' || item.name.toLowerCase().includes('iso') || item.path.toLowerCase().includes('iso'); }
