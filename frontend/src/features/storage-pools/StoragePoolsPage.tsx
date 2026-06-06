import { useEffect, useMemo, useState } from 'react';
import {
  ArchiveIcon,
  DatabaseIcon,
  FolderIcon,
  HardDriveIcon,
  InfoIcon,
  PlusIcon,
  RefreshCwIcon,
} from 'lucide-react';
import { toast } from 'sonner';

import { SelectMenu } from '../../components/kvm/SelectMenu';
import { KvmTooltip } from '../../components/kvm/StatusBadge';
import {
  createStoragePool,
  fetchHosts,
  fetchStoragePools,
  updateStoragePoolAutostart,
  updateStoragePoolState,
  type Host,
  type StoragePool,
} from '../../lib/api';
import { formatBytes } from '../../lib/format';
import { onKvmResourceEvent } from '../../lib/resourceEvents';
import { can } from '../../lib/permissions';
import { StoragePoolCreateDialog } from './components/StoragePoolCreateDialog';
import { StoragePoolDetailDialog } from './components/StoragePoolDetailDialog';
import { buttonStyle, primaryButtonStyle } from './components/storagePoolStyles';
import { storageUsageColor } from './utils/storageUsage';

export default function StoragePoolsPage() {
  const [hosts, setHosts] = useState<Host[]>([]);
  const [selectedHost, setSelectedHost] = useState('');
  const [items, setItems] = useState<StoragePool[]>([]);
  const [loading, setLoading] = useState(true);
  const [dialogOpen, setDialogOpen] = useState(false);
  const [detailItem, setDetailItem] = useState<StoragePool | null>(null);
  const canManageStorage = can('storage.manage');

  const host = hosts.find(item => item.id === selectedHost);

  async function load(nextHost = selectedHost) {
    setLoading(true);
    try {
      const hostBody = await fetchHosts();
      setHosts(hostBody.items);
      const target = nextHost || hostBody.items[0]?.id || '';
      setSelectedHost(target);
      if (!target) {
        setItems([]);
        return [];
      }
      const poolBody = await fetchStoragePools(target);
      setItems(poolBody.items);
      return poolBody.items;
    } catch (error) {
      toast.error(error instanceof Error ? error.message : '读取存储池失败');
      return [];
    } finally {
      setLoading(false);
    }
  }

  async function openDetail(item: StoragePool) {
    if (!selectedHost) {
      setDetailItem(item);
      return;
    }
    const latest = await load(selectedHost);
    setDetailItem(latest.find(next => next.name === item.name) ?? item);
  }

  useEffect(() => {
    void load('');
  }, []);

  useEffect(
    () =>
      onKvmResourceEvent('storage.pool.updated', event => {
        if (!selectedHost || event.agentId === selectedHost) void refreshAfterEvent();
      }),
    [selectedHost]
  );

  async function refreshAfterEvent() {
    const latest = await load(selectedHost);
    setDetailItem(current => current ? latest.find(next => next.name === current.name) ?? current : current);
  }

  const totals = useMemo(() => {
    return storagePoolSummaryTotals(items);
  }, [items]);

  return (
    <div data-cmp="StoragePoolsPage" className="space-y-5 p-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="text-lg font-semibold" style={{ color: 'var(--kvm-text)' }}>存储池</h1>
          <p className="mt-1 text-sm" style={{ color: 'var(--kvm-text-muted)' }}>
            按宿主机管理 libvirt 存储池与 ISO 镜像目录
          </p>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <HostPicker value={selectedHost} hosts={hosts} onChange={hostId => void load(hostId)} />
          <button type="button" onClick={() => void load()} disabled={loading} className="kvm-action-button inline-flex h-10 items-center gap-2 rounded-lg border px-3 text-sm disabled:opacity-60" style={buttonStyle}>
            <RefreshCwIcon size={15} className={loading ? 'animate-spin' : ''} />刷新
          </button>
          {canManageStorage && (
            <button type="button" disabled={!selectedHost} onClick={() => setDialogOpen(true)} className="kvm-action-button inline-flex h-10 items-center gap-2 rounded-lg border px-3 text-sm font-semibold disabled:opacity-60" style={primaryButtonStyle}>
              <PlusIcon size={15} />新建存储池
            </button>
          )}
        </div>
      </div>

      <section className="grid gap-3 md:grid-cols-3">
        <MetricCard icon={DatabaseIcon} label="存储池" value={`${items.length}`} />
        <MetricCard icon={HardDriveIcon} label="总容量" value={formatBytes(totals.capacity, 'GB', 1)} />
        <MetricCard icon={ArchiveIcon} label="已分配" value={formatBytes(totals.allocation, 'GB', 1)} />
      </section>

      <section className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
        {!loading && items.length === 0 && <Empty text={host ? '当前宿主机暂无存储池' : '暂无可用宿主机'} />}
        {items.map(item => <StoragePoolCard key={item.name} item={item} onDetail={item => void openDetail(item)} />)}
      </section>

      {dialogOpen && selectedHost && (
        <StoragePoolCreateDialog
          hostName={host?.name || selectedHost}
          onClose={() => setDialogOpen(false)}
          onSubmit={async payload => {
            if (items.some(item => item.name.toLowerCase() === payload.name.trim().toLowerCase())) {
              toast.error('名称已存在，请重新更换名称');
              return;
            }
            try {
              await createStoragePool(selectedHost, payload);
              toast.success('存储池已创建');
              setDialogOpen(false);
              await load(selectedHost);
            } catch (error) {
              toast.error(friendlyPoolError(error));
            }
          }}
        />
      )}
      {detailItem && selectedHost && (
        <StoragePoolDetailDialog
          agentId={selectedHost}
          hostName={host?.name || selectedHost}
          item={detailItem}
          canManage={canManageStorage}
          onClose={() => setDetailItem(null)}
          onRefresh={async () => {
            const latest = await load(selectedHost);
            setDetailItem(current => current ? latest.find(next => next.name === current.name) ?? current : current);
          }}
          onUpdateState={async active => updateStoragePoolState(selectedHost, detailItem.name, active)}
          onUpdateAutostart={async autostart => updateStoragePoolAutostart(selectedHost, detailItem.name, autostart)}
          onDeleted={async () => {
            await load(selectedHost);
          }}
        />
      )}
    </div>
  );
}

function HostPicker({ value, hosts, onChange }: { value: string; hosts: Host[]; onChange: (id: string) => void }) {
  return (
    <SelectMenu
      value={value}
      disabled={hosts.length === 0}
      maxVisibleItems={5}
      placeholder="暂无可用宿主机"
      options={hosts.map(item => ({ value: item.id, label: item.name }))}
      className="w-[min(280px,calc(100vw-48px))] sm:w-52"
      onChange={onChange}
    />
  );
}

function StoragePoolCard({ item, onDetail }: { item: StoragePool; onDetail: (item: StoragePool) => void }) {
  const used = item.capacity > 0 ? Math.round((item.allocation * 100) / item.capacity) : 0;
  return (
    <article className="kvm-surface-3d rounded-xl p-4">
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <h2 className="truncate text-base font-semibold" style={{ color: 'var(--kvm-text)' }}>{item.name}</h2>
          <p className="mt-1 text-xs uppercase" style={{ color: 'var(--kvm-text-muted)' }}>{item.type || 'unknown'} · {item.state || 'unknown'}</p>
        </div>
        <div className="flex items-center gap-2">
          <KvmTooltip label="查看详情" placement="top">
            <button type="button" aria-label="查看详情" onClick={() => onDetail(item)} className="kvm-action-button inline-flex h-8 w-8 items-center justify-center rounded-lg border" style={buttonStyle}>
              <InfoIcon size={15} />
            </button>
          </KvmTooltip>
          <FolderIcon size={18} style={{ color: '#2dd4bf' }} />
        </div>
      </div>
      <div className="mt-4 space-y-2 text-sm" style={{ color: 'var(--kvm-text-muted)' }}>
        <Row label="路径" value={item.path || '-'} />
        <Row label="容量" value={item.capacity > 0 ? formatBytes(item.capacity, 'GB', 1) : '-'} />
        <Row label="卷" value={`${item.volumeCount}`} />
      </div>
      <div className="mt-4 h-2 rounded-full" style={{ background: 'rgba(148,163,184,0.18)' }}>
        <div className="h-full rounded-full" style={{ width: `${Math.min(100, used)}%`, background: storageUsageColor(used) }} />
      </div>
      <div className="mt-2 text-xs" style={{ color: 'var(--kvm-text-muted)' }}>{used}% 已用</div>
    </article>
  );
}

function MetricCard({ icon: Icon, label, value }: { icon: typeof DatabaseIcon; label: string; value: string }) {
  return <div className="kvm-surface-3d rounded-xl p-4"><Icon size={17} style={{ color: '#3b82f6' }} /><div className="mt-3 text-xs" style={{ color: 'var(--kvm-text-muted)' }}>{label}</div><div className="mt-1 text-lg font-semibold" style={{ color: 'var(--kvm-text)' }}>{value}</div></div>;
}

function Empty({ text }: { text: string }) {
  return <div className="kvm-empty-state col-span-full rounded-xl p-8 text-center text-sm" style={{ color: 'var(--kvm-text-muted)' }}>{text}</div>;
}

function Row({ label, value }: { label: string; value: string }) {
  return <div className="flex justify-between gap-3"><span>{label}</span><span className="min-w-0 truncate font-mono" style={{ color: 'var(--kvm-text)' }}>{value}</span></div>;
}

function storagePoolSummaryTotals(items: StoragePool[]) {
  const seenSources = new Set<string>();
  return items.reduce((totals, item) => {
    const source = item.capacitySource?.trim();
    const key = source || `pool:${item.name}`;
    if (seenSources.has(key)) {
      return totals;
    }
    seenSources.add(key);
    return {
      capacity: totals.capacity + item.capacity,
      allocation: totals.allocation + item.allocation,
    };
  }, { capacity: 0, allocation: 0 });
}

function friendlyPoolError(error: unknown) {
  const raw = error instanceof Error ? error.message : '操作失败';
  const compact = raw.replace(/\s+/g, ' ').trim();
  const lower = compact.toLowerCase();
  if (lower.includes('already exists') || lower.includes('exists already')) return '名称已存在，请重新更换名称';
  if (lower.includes('name is required')) return '请填写名称';
  if (lower.includes('absolute path') || lower.includes('绝对路径')) return '请填写绝对路径';
  if (lower.includes('must be a directory') || lower.includes('路径必须是目录')) return '路径必须是目录';
  if (lower.includes('must be a block device') || lower.includes('块设备')) return 'LVM 设备路径必须是块设备';
  if (lower.includes('path is required') || lower.includes('target')) return '请填写正确的路径';
  if (lower.includes('device is required')) return '请填写设备路径';
  if (lower.includes('unsupported')) return '当前类型或参数不受支持';
  if (lower.includes('virsh') || lower.includes('error:')) return '宿主机命令执行失败，请检查名称、路径和权限';
  return compact.length > 120 ? `${compact.slice(0, 120)}...` : compact;
}
