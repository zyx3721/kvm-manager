import { useEffect, useMemo, useState } from 'react';
import {
  ActivityIcon,
  InfoIcon,
  NetworkIcon,
  PlusIcon,
  RefreshCwIcon,
  RouterIcon,
  Trash2Icon,
  XIcon,
} from 'lucide-react';
import { toast } from 'sonner';

import { DialogPortal } from '../../components/kvm/DialogPortal';
import { SelectMenu } from '../../components/kvm/SelectMenu';
import { KvmTooltip } from '../../components/kvm/StatusBadge';
import { can } from '../../lib/permissions';
import { onKvmResourceEvent } from '../../lib/resourceEvents';
import {
  createHostInterface,
  deleteHostInterface,
  fetchHostInterfaces,
  fetchHosts,
  updateHostInterfaceState,
  type Host,
  type HostInterface,
  type HostInterfaceCreatePayload,
} from '../../lib/api';
import { HostInterfaceCreateDialog } from './components/HostInterfaceCreateDialog';
import { validateHostInterfaceAddressConflicts } from './ipAddressValidation';

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

export default function HostInterfacesPage() {
  const [hosts, setHosts] = useState<Host[]>([]);
  const [selectedHost, setSelectedHost] = useState('');
  const [items, setItems] = useState<HostInterface[]>([]);
  const [loading, setLoading] = useState(true);
  const [detailItem, setDetailItem] = useState<HostInterface | null>(null);
  const [dialogOpen, setDialogOpen] = useState(false);
  const canManageInterfaces = can('host.interfaces.manage');
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
      const body = await fetchHostInterfaces(target);
      setItems(body.items);
      return body.items;
    } catch (error) {
      toast.error(error instanceof Error ? error.message : '读取宿主机接口失败');
      return [];
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    void load('');
  }, []);

  useEffect(
    () =>
      onKvmResourceEvent('host.interface.updated', event => {
        if (!selectedHost || event.agentId === selectedHost) void refreshAfterEvent();
      }),
    [selectedHost]
  );

  async function openDetail(item: HostInterface) {
    if (!selectedHost) {
      setDetailItem(item);
      return;
    }
    const latest = await load(selectedHost);
    setDetailItem(latest.find(next => next.name === item.name) ?? item);
  }

  async function refreshAfterEvent() {
    const latest = await load(selectedHost);
    setDetailItem(current => current ? latest.find(next => next.name === current.name) ?? current : current);
  }

  const totals = useMemo(() => ({
    total: items.length,
    active: items.filter(item => item.status === 'up').length,
    bridges: items.filter(item => item.type === 'bridge').length,
  }), [items]);

  return (
    <div data-cmp="HostInterfacesPage" className="space-y-5 p-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="text-lg font-semibold" style={{ color: 'var(--kvm-text)' }}>接口</h1>
          <p className="mt-1 text-sm" style={{ color: 'var(--kvm-text-muted)' }}>
            按宿主机查看与创建物理网卡、桥接接口
          </p>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <HostPicker value={selectedHost} hosts={hosts} onChange={hostId => void load(hostId)} />
          <button type="button" onClick={() => void load()} disabled={loading} className="kvm-action-button inline-flex h-10 items-center gap-2 rounded-lg border px-3 text-sm disabled:opacity-60" style={buttonStyle}>
            <RefreshCwIcon size={15} className={loading ? 'animate-spin' : ''} />刷新
          </button>
          {canManageInterfaces && (
            <button type="button" disabled={!selectedHost} onClick={() => setDialogOpen(true)} className="kvm-action-button inline-flex h-10 items-center gap-2 rounded-lg border px-3 text-sm font-semibold disabled:opacity-60" style={primaryButtonStyle}>
              <PlusIcon size={15} />新增接口
            </button>
          )}
        </div>
      </div>

      <section className="grid gap-3 md:grid-cols-3">
        <MetricCard icon={NetworkIcon} label="接口" value={`${totals.total}`} />
        <MetricCard icon={ActivityIcon} label="在线接口" value={`${totals.active}`} />
        <MetricCard icon={RouterIcon} label="桥接接口" value={`${totals.bridges}`} />
      </section>

      <section className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
        {!loading && items.length === 0 && <Empty text={host ? '当前宿主机暂无接口' : '暂无可用宿主机'} />}
        {items.map(item => <HostInterfaceCard key={item.name} item={item} onDetail={item => void openDetail(item)} />)}
      </section>

      {detailItem && (
        <HostInterfaceDetailDialog
          agentId={selectedHost}
          hostName={host?.name || selectedHost}
          item={detailItem}
          canManage={canManageInterfaces}
          onClose={() => setDetailItem(null)}
          onRefresh={() => load(selectedHost)}
          onDeleted={async () => {
            setDetailItem(null);
            await load(selectedHost);
          }}
        />
      )}
      {dialogOpen && selectedHost && (
        <HostInterfaceCreateDialog
          agentId={selectedHost}
          hostName={host?.name || selectedHost}
          interfaces={items}
          onClose={() => setDialogOpen(false)}
          onSubmit={async payload => {
            const error = validatePayload(payload, items);
            if (error) {
              toast.error(error);
              return;
            }
            try {
              await createHostInterface(selectedHost, payload);
              toast.success('接口已创建');
              setDialogOpen(false);
              await load(selectedHost);
            } catch (error) {
              if (isAgentTimeoutError(error)) {
                const latest = await load(selectedHost);
                if (latest.some(item => item.name.toLowerCase() === payload.name.trim().toLowerCase())) {
                  toast.success('接口已创建');
                  setDialogOpen(false);
                  return;
                }
              }
              toast.error(friendlyInterfaceError(error));
            }
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

function HostInterfaceCard({ item, onDetail }: { item: HostInterface; onDetail: (item: HostInterface) => void }) {
  const active = item.status === 'up';
  return (
    <article className="kvm-surface-3d rounded-xl p-4">
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <h2 className="truncate text-base font-semibold" style={{ color: active ? 'var(--kvm-status-green-text)' : 'var(--kvm-status-red-text)' }}>{item.name}</h2>
          <p className="mt-1 text-xs uppercase" style={{ color: 'var(--kvm-text-muted)' }}>{item.type || 'unknown'} · {item.status || 'unknown'}</p>
        </div>
        <div className="flex items-center gap-2">
          <KvmTooltip label="查看详情" placement="top">
            <button type="button" aria-label="查看详情" onClick={() => onDetail(item)} className="kvm-action-button inline-flex h-8 w-8 items-center justify-center rounded-lg border" style={buttonStyle}>
              <InfoIcon size={15} />
            </button>
          </KvmTooltip>
          <NetworkIcon size={18} style={{ color: active ? '#2dd4bf' : '#f87171' }} />
        </div>
      </div>
      <div className="mt-4 space-y-2 text-sm" style={{ color: 'var(--kvm-text-muted)' }}>
        <Row label="类型" value={item.type || '-'} />
        <Row label="MAC" value={item.mac || '-'} />
        <Row label="IPv4" value={item.ipv4 || '-'} />
      </div>
    </article>
  );
}

function HostInterfaceDetailDialog({
  agentId,
  hostName,
  item,
  canManage,
  onClose,
  onRefresh,
  onDeleted,
}: {
  agentId: string;
  hostName: string;
  item: HostInterface;
  canManage: boolean;
  onClose: () => void;
  onRefresh: () => Promise<HostInterface[]>;
  onDeleted: () => Promise<void>;
}) {
  const [localItem, setLocalItem] = useState(item);
  const [busy, setBusy] = useState('');
  const [confirmDelete, setConfirmDelete] = useState(false);
  const active = localItem.status === 'up';

  useEffect(() => {
    setLocalItem(item);
  }, [item]);

  async function runState() {
    setBusy('state');
    try {
      const result = await updateHostInterfaceState(agentId, localItem.name, !active);
      setLocalItem(current => ({ ...current, status: result.active ? 'up' : 'down' }));
      toast.success('接口状态已更新');
      void onRefresh();
    } catch (error) {
      toast.error(friendlyInterfaceError(error));
    } finally {
      setBusy('');
    }
  }

  async function removeInterface() {
    setBusy('delete');
    try {
      await deleteHostInterface(agentId, localItem.name);
      toast.success('接口已删除');
      await onDeleted();
      onClose();
    } catch (error) {
      toast.error(friendlyInterfaceError(error));
    } finally {
      setBusy('');
      setConfirmDelete(false);
    }
  }

  return (
    <DialogPortal>
      <div className="kvm-dialog-backdrop fixed inset-0 z-50 flex items-center justify-center px-3">
        <div className="kvm-dialog-panel max-h-[82vh] w-[min(92vw,780px)] overflow-hidden rounded-2xl">
          <header className="flex items-center justify-between border-b px-5 py-4" style={{ borderColor: 'var(--kvm-border)' }}>
            <div className="min-w-0">
              <h2 className="truncate text-base font-semibold" style={{ color: 'var(--kvm-text)' }}>{item.name}</h2>
              <p className="mt-1 text-xs" style={{ color: 'var(--kvm-text-muted)' }}>{hostName} · {item.type || 'unknown'}</p>
            </div>
            <button type="button" onClick={onClose} aria-label="关闭" className="kvm-action-button inline-flex h-8 w-8 items-center justify-center rounded-lg border" style={buttonStyle}><XIcon size={15} /></button>
          </header>
          <div className="kvm-hidden-scrollbar max-h-[calc(82vh-74px)] overflow-y-auto p-5">
            <section className="kvm-dialog-card rounded-xl p-4">
              <div className="kvm-detail-tile-grid grid gap-3 md:grid-cols-2">
                <InfoTile label="接口" value={localItem.name} />
                <InfoTile label="IPv4" value={formatAddressMode(localItem.ipv4, localItem.ipv4Mode)} />
                <InfoTile label="IPv6" value={formatAddressMode(localItem.ipv6, localItem.ipv6Mode)} />
                <InfoTile label="MAC 地址" value={localItem.mac || '-'} />
                <InfoTile label="接口类型" value={localItem.type || '-'} />
                <InfoTile label="桥接设备" value={localItem.bridgeDevice || '-'} />
                <InfoTile label="启动模式" value={localItem.bootMode || '-'} />
                <InterfaceActionTile
                  active={active}
                  busy={busy !== ''}
                  canManage={canManage}
                  onState={() => void runState()}
                  onDelete={() => setConfirmDelete(true)}
                />
                <InfoTile label="STP" value={localItem.stp || '-'} />
                <InfoTile label="Delay" value={localItem.delay || '-'} />
              </div>
            </section>
          </div>
        </div>
        {confirmDelete && (
          <ConfirmDialog
            title="删除接口"
            message={`确认删除接口 ${localItem.name}？请确认该接口已停止且不再使用`}
            busy={busy === 'delete'}
            onClose={() => setConfirmDelete(false)}
            onConfirm={() => void removeInterface()}
          />
        )}
      </div>
    </DialogPortal>
  );
}

function validatePayload(payload: HostInterfaceCreatePayload, existing: HostInterface[]) {
  if (!payload.name.trim()) return '请填写名称';
  if (existing.some(item => item.name.toLowerCase() === payload.name.trim().toLowerCase())) return '接口名称已存在，请重新更换名称';
  if (!/^[A-Za-z0-9_.-]{1,15}$/.test(payload.name.trim())) return '接口名称只能包含字母、数字、点、短横线和下划线，长度不超过 15';
  if (!['bridge', 'ethernet'].includes(payload.type)) return '当前仅支持创建 bridge 或 ethernet 类型接口';
  if (payload.type === 'bridge' && (Number.isNaN(Number(payload.delay)) || Number(payload.delay) < 0)) return 'Delay 必须是大于等于 0 的数字';
  if (payload.ipv4Mode === 'static' && !payload.ipv4Address.trim()) return '请填写 IPv4 地址';
  if (payload.ipv6Mode === 'static' && !payload.ipv6Address.trim()) return '请填写 IPv6 地址';
  const usedBy = findInterfaceUsingDevice(payload.device, existing);
  if (usedBy) return `设备 ${payload.device.trim()} 已被接口 ${usedBy} 使用，请重新选择设备`;
  const addressConflict = validateHostInterfaceAddressConflicts(payload, existing);
  if (addressConflict) return addressConflict;
  return '';
}

function findInterfaceUsingDevice(device: string, existing: HostInterface[]) {
  const normalized = device.trim().toLowerCase();
  if (!normalized) return '';
  return existing.find(item => item.bridgeDevice.trim().toLowerCase() === normalized)?.name ?? '';
}

function friendlyInterfaceError(error: unknown) {
  const raw = error instanceof Error ? error.message : '操作失败';
  const compact = raw.replace(/\s+/g, ' ').trim();
  const lower = compact.toLowerCase();
  if (lower.includes('ipv4 address already exists')) return 'IPv4 地址已被其他接口使用，请更换后重试';
  if (lower.includes('ipv6 address already exists')) return 'IPv6 地址已被其他接口使用，请更换后重试';
  if (lower.includes('ipv4 subnet already exists')) return 'IPv4 子网已被其他接口使用，请避免同宿主机重复子网';
  if (lower.includes('ipv6 subnet already exists')) return 'IPv6 子网已被其他接口使用，请避免同宿主机重复子网';
  if (lower.includes('ipv4 gateway must be in the same subnet')) return 'IPv4 网关必须与地址处于同一子网';
  if (lower.includes('ipv6 gateway must be in the same subnet')) return 'IPv6 网关必须与地址处于同一子网';
  if (lower.includes('ipv4 gateway cannot equal address')) return 'IPv4 网关不能与地址相同';
  if (lower.includes('ipv6 gateway cannot equal address')) return 'IPv6 网关不能与地址相同';
  if (lower.includes('ipv4 gateway is invalid')) return 'IPv4 网关格式不正确';
  if (lower.includes('ipv6 gateway is invalid')) return 'IPv6 网关格式不正确';
  if (lower.includes('dns server is invalid')) return 'DNS 地址格式不正确';
  if (lower.includes('interface device already in use')) return '该设备已被其他接口使用，请重新选择设备';
  if (lower.includes('system interface config not found')) return '未找到宿主机系统网络配置，无法写入 DNS';
  if (lower.includes('write system interface dns failed') || lower.includes('backup system interface config failed')) return '写入宿主机 DNS 配置失败，请检查 Agent 权限';
  if (lower.includes('already exists')) return '接口名称已存在，请重新更换名称';
  if (lower.includes('interface not found') || lower.includes('not found')) return '接口不存在或已被删除，请刷新后重试';
  if (lower.includes('device does not exist')) return '绑定设备不存在，请重新选择设备';
  if (lower.includes('interface name is required')) return '请填写接口名称';
  if (lower.includes('interface name is invalid')) return '接口名称格式不正确';
  if (lower.includes('start mode is invalid')) return '启动模式不受支持，请选择 none、onboot 或 hotplug';
  if (lower.includes('unsupported interface type') || lower.includes('unsupported')) return '当前接口类型或参数不受支持';
  if (lower.includes('ipv4 address is required')) return '请填写 IPv4 地址';
  if (lower.includes('ipv6 address is required')) return '请填写 IPv6 地址';
  if (lower.includes('ipv4 address is invalid')) return 'IPv4 地址格式不正确，请使用 CIDR 格式';
  if (lower.includes('ipv6 address is invalid')) return 'IPv6 地址格式不正确，请使用 CIDR 格式';
  if (lower.includes('ipv4 mode is unsupported')) return 'IPv4 模式不受支持';
  if (lower.includes('ipv6 mode is unsupported')) return 'IPv6 模式不受支持';
  if (lower.includes('bridge delay is invalid')) return 'Delay 必须是有效数字';
  if (lower.includes('interface must be stopped before deletion')) return '请先停止接口后再删除';
  if (lower.includes('permission denied') || lower.includes('operation not permitted')) return '宿主机权限不足，无法创建接口';
  if (lower.includes('iface-define') || lower.includes('define interface') || lower.includes('failed to define')) return '接口定义失败，请检查接口类型、绑定设备和地址配置';
  if (lower.includes('iface-start') || lower.includes('start interface') || lower.includes('failed to start')) return '接口启动失败，请检查启动模式、绑定设备和宿主机网络配置';
  if (lower.includes('iface-destroy') || lower.includes('destroy interface') || lower.includes('failed to destroy')) return '接口停止失败，请检查接口当前状态和宿主机权限';
  if (lower.includes('iface-undefine') || lower.includes('undefine interface') || lower.includes('failed to undefine')) return '接口删除失败，请确认接口已停止且不再被使用';
  if (lower.includes('xml') || lower.includes('invalid argument') || lower.includes('unsupported configuration')) return '接口配置不被 libvirt 接受，请检查类型、设备和协议配置';
  if (lower.includes('ip link show')) return '读取绑定设备失败，请确认设备仍存在';
  if (lower.includes('ip ')) return '宿主机 ip 命令执行失败，请检查设备名称和权限';
  if (lower.includes('failed')) return `宿主机命令执行失败：${compact.length > 90 ? `${compact.slice(0, 90)}...` : compact}`;
  return compact.length > 120 ? `${compact.slice(0, 120)}...` : compact;
}

function isAgentTimeoutError(error: unknown) {
  const message = error instanceof Error ? error.message : String(error);
  const lower = message.toLowerCase();
  return message.includes('Agent 连接超时') || lower.includes('timeout');
}

function MetricCard({ icon: Icon, label, value }: { icon: typeof NetworkIcon; label: string; value: string }) {
  return <div className="kvm-surface-3d rounded-xl p-4"><Icon size={17} style={{ color: '#3b82f6' }} /><div className="mt-3 text-xs" style={{ color: 'var(--kvm-text-muted)' }}>{label}</div><div className="mt-1 text-lg font-semibold" style={{ color: 'var(--kvm-text)' }}>{value}</div></div>;
}

function Empty({ text }: { text: string }) {
  return <div className="kvm-empty-state col-span-full rounded-xl p-8 text-center text-sm" style={{ color: 'var(--kvm-text-muted)' }}>{text}</div>;
}

function Row({ label, value }: { label: string; value: string }) {
  return <div className="flex justify-between gap-3"><span>{label}</span><span className="min-w-0 truncate font-mono" style={{ color: 'var(--kvm-text)' }}>{value}</span></div>;
}

function InfoTile({ label, value }: { label: string; value: string }) {
  return <div className="rounded-lg border p-3" style={{ borderColor: 'var(--kvm-border)', background: 'var(--kvm-control-bg-soft)' }}><div className="text-xs" style={{ color: 'var(--kvm-text-muted)' }}>{label}</div><div className="mt-2 min-w-0 break-all font-mono text-sm font-semibold" style={{ color: 'var(--kvm-text)' }}>{value}</div></div>;
}

function InterfaceActionTile({ active, busy, canManage, onState, onDelete }: { active: boolean; busy: boolean; canManage: boolean; onState: () => void; onDelete: () => void }) {
  const stateColor = active
    ? { border: 'rgba(239,68,68,0.35)', background: 'rgba(239,68,68,0.08)', text: '#f87171' }
    : { border: 'rgba(16,185,129,0.34)', background: 'rgba(16,185,129,0.08)', text: 'var(--kvm-status-green-text)' };
  const deleteButton = <button type="button" disabled={active || busy} onClick={onDelete} className="kvm-action-button kvm-danger-button inline-flex h-8 items-center gap-1.5 rounded-lg border px-2.5 text-xs font-semibold disabled:cursor-not-allowed disabled:opacity-50" style={{ borderColor: 'rgba(239,68,68,0.35)', color: '#f87171', background: 'rgba(239,68,68,0.1)' }}><Trash2Icon size={13} />删除</button>;
  return (
    <div className="rounded-lg border p-3" style={{ borderColor: active ? 'rgba(16,185,129,0.34)' : 'rgba(239,68,68,0.34)', background: active ? 'rgba(16,185,129,0.08)' : 'rgba(239,68,68,0.08)' }}>
      <div className="text-xs" style={{ color: 'var(--kvm-text-muted)' }}>状态</div>
      <div className="mt-2 flex items-center justify-between gap-3">
        <span className="inline-flex h-7 items-center rounded-full border px-2.5 text-xs font-semibold" style={{ color: active ? 'var(--kvm-status-green-text)' : 'var(--kvm-status-red-text)', borderColor: active ? 'rgba(16,185,129,0.34)' : 'rgba(239,68,68,0.34)' }}>{active ? '运行' : '停止'}</span>
        {canManage && (
          <div className="flex shrink-0 items-center gap-2">
            <button type="button" disabled={busy} onClick={onState} className="kvm-action-button h-8 rounded-lg border px-3 text-xs font-semibold disabled:opacity-60" style={{ borderColor: stateColor.border, background: stateColor.background, color: stateColor.text }}>{active ? '停止' : '启动'}</button>
            {active ? <KvmTooltip label="运行中的接口需要先停止后才能删除" placement="top">{deleteButton}</KvmTooltip> : deleteButton}
          </div>
        )}
      </div>
    </div>
  );
}

function ConfirmDialog({ title, message, busy, onClose, onConfirm }: { title: string; message: string; busy: boolean; onClose: () => void; onConfirm: () => void }) {
  return (
    <div className="fixed inset-0 z-[60] flex items-center justify-center bg-slate-950/35 px-3">
      <div className="kvm-dialog-card w-[min(92vw,420px)] rounded-xl border p-5" style={{ borderColor: 'var(--kvm-border)' }}>
        <h3 className="text-base font-semibold" style={{ color: 'var(--kvm-text)' }}>{title}</h3>
        <p className="mt-2 text-sm" style={{ color: 'var(--kvm-text-muted)' }}>{message}</p>
        <div className="mt-5 flex justify-end gap-2">
          <button type="button" disabled={busy} onClick={onClose} className="kvm-action-button rounded-lg border px-4 py-2 text-sm disabled:opacity-60" style={buttonStyle}>取消</button>
          <button type="button" disabled={busy} onClick={onConfirm} className="kvm-action-button kvm-danger-button rounded-lg border px-4 py-2 text-sm font-semibold disabled:opacity-60" style={{ borderColor: 'rgba(239,68,68,0.35)', color: '#f87171', background: 'rgba(239,68,68,0.1)' }}>{busy ? '删除中' : '确认删除'}</button>
        </div>
      </div>
    </div>
  );
}

function StatusTile({ label, value, active }: { label: string; value: string; active: boolean }) {
  const color = active ? 'var(--kvm-status-green-text)' : 'var(--kvm-status-red-text)';
  return <div className="rounded-lg border p-3" style={{ borderColor: active ? 'rgba(16,185,129,0.34)' : 'rgba(239,68,68,0.34)', background: active ? 'rgba(16,185,129,0.08)' : 'rgba(239,68,68,0.08)' }}><div className="text-xs" style={{ color: 'var(--kvm-text-muted)' }}>{label}</div><div className="mt-2"><span className="inline-flex h-7 items-center rounded-full border px-2.5 text-xs font-semibold" style={{ color, borderColor: active ? 'rgba(16,185,129,0.34)' : 'rgba(239,68,68,0.34)' }}>{value}</span></div></div>;
}

function formatAddressMode(address: string, mode: string) {
  const label = mode || 'none';
  return address ? `${address} (${label})` : `- (${label})`;
}
