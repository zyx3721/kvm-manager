import { useEffect, useState, type ReactNode } from 'react';
import {
  CheckIcon,
  Globe2Icon,
  InfoIcon,
  LoaderCircleIcon,
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
import { AutostartBadge, isPoolActive, StateBadge } from '../storage-pools/components/PoolBadges';
import {
  createNetworkPool,
  deleteNetworkPool,
  fetchHosts,
  fetchNetworkPools,
  updateNetworkPoolAutostart,
  updateNetworkPoolState,
  type Host,
  type NetworkPool,
  type NetworkFixedAddress,
  type NetworkPoolCreatePayload,
} from '../../lib/api';

const networkTypes = ['NAT', 'ROUTE', 'ISOLATE', 'BRIDGE'];

export default function NetworkPoolsPage() {
  const [hosts, setHosts] = useState<Host[]>([]);
  const [selectedHost, setSelectedHost] = useState('');
  const [items, setItems] = useState<NetworkPool[]>([]);
  const [loading, setLoading] = useState(true);
  const [dialogOpen, setDialogOpen] = useState(false);
  const [detailItem, setDetailItem] = useState<NetworkPool | null>(null);
  const canManageNetwork = can('network.manage');
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
      const body = await fetchNetworkPools(target);
      setItems(body.items);
      return body.items;
    } catch (error) {
      toast.error(error instanceof Error ? error.message : '读取网络池失败');
      return [];
    } finally {
      setLoading(false);
    }
  }

  async function openDetail(item: NetworkPool) {
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
      onKvmResourceEvent('network.pool.updated', event => {
        if (!selectedHost || event.agentId === selectedHost) void refreshAfterEvent();
      }),
    [selectedHost]
  );

  async function refreshAfterEvent() {
    const latest = await load(selectedHost);
    setDetailItem(current =>
      current ? (latest.find(next => next.name === current.name) ?? current) : current
    );
  }

  return (
    <div data-cmp="NetworkPoolsPage" className="space-y-5 p-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="text-lg font-semibold" style={{ color: 'var(--kvm-text)' }}>
            网络池
          </h1>
          <p className="mt-1 text-sm" style={{ color: 'var(--kvm-text-muted)' }}>
            按宿主机管理 libvirt 网络、桥接和转发模式
          </p>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <HostPicker value={selectedHost} hosts={hosts} onChange={hostId => void load(hostId)} />
          <button
            type="button"
            onClick={() => void load()}
            disabled={loading}
            className="kvm-action-button inline-flex h-10 items-center gap-2 rounded-lg border px-3 text-sm disabled:opacity-60"
            style={buttonStyle}
          >
            <RefreshCwIcon size={15} className={loading ? 'animate-spin' : ''} />
            刷新
          </button>
          {canManageNetwork && (
            <button
              type="button"
              disabled={!selectedHost}
              onClick={() => setDialogOpen(true)}
              className="kvm-action-button inline-flex h-10 items-center gap-2 rounded-lg border px-3 text-sm font-semibold disabled:opacity-60"
              style={primaryButtonStyle}
            >
              <PlusIcon size={15} />
              新建网络池
            </button>
          )}
        </div>
      </div>

      <section className="grid gap-3 md:grid-cols-3">
        <MetricCard icon={NetworkIcon} label="网络池" value={`${items.length}`} />
        <MetricCard
          icon={RouterIcon}
          label="桥接网络"
          value={`${items.filter(item => item.forward === 'bridge').length}`}
        />
        <MetricCard
          icon={Globe2Icon}
          label="DHCP"
          value={`${items.filter(item => item.dhcp).length}`}
        />
      </section>

      <section className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
        {!loading && items.length === 0 && (
          <Empty text={host ? '当前宿主机暂无网络池' : '暂无可用宿主机'} />
        )}
        {items.map(item => (
          <NetworkPoolCard key={item.name} item={item} onDetail={item => void openDetail(item)} />
        ))}
      </section>

      {dialogOpen && selectedHost && (
        <NetworkPoolDialog
          hostName={host?.name || selectedHost}
          existing={items}
          onClose={() => setDialogOpen(false)}
          onSubmit={async payload => {
            const error = validateNetworkPayload(payload, items);
            if (error) {
              toast.error(error);
              return;
            }
            try {
              await createNetworkPool(selectedHost, payload);
              toast.success('网络池已创建');
              setDialogOpen(false);
              await load(selectedHost);
            } catch (error) {
              toast.error(friendlyPoolError(error));
            }
          }}
        />
      )}
      {detailItem && selectedHost && (
        <NetworkPoolDetailDialog
          agentId={selectedHost}
          hostName={host?.name || selectedHost}
          item={detailItem}
          canManage={canManageNetwork}
          onClose={() => setDetailItem(null)}
          onRefresh={async () => {
            const latest = await load(selectedHost);
            setDetailItem(current =>
              current ? (latest.find(next => next.name === current.name) ?? current) : current
            );
          }}
          onUpdateState={async active =>
            updateNetworkPoolState(selectedHost, detailItem.name, active)
          }
          onUpdateAutostart={async autostart =>
            updateNetworkPoolAutostart(selectedHost, detailItem.name, autostart)
          }
          onDeleted={async () => {
            await load(selectedHost);
          }}
        />
      )}
    </div>
  );
}

function HostPicker({
  value,
  hosts,
  onChange,
}: {
  value: string;
  hosts: Host[];
  onChange: (id: string) => void;
}) {
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

function NetworkPoolCard({
  item,
  onDetail,
}: {
  item: NetworkPool;
  onDetail: (item: NetworkPool) => void;
}) {
  return (
    <article className="kvm-surface-3d rounded-xl p-4">
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <h2 className="truncate text-base font-semibold" style={{ color: 'var(--kvm-text)' }}>
            {item.name}
          </h2>
          <p className="mt-1 text-xs uppercase" style={{ color: 'var(--kvm-text-muted)' }}>
            {item.state || 'unknown'} · {item.autostart ? 'autostart' : 'manual'}
          </p>
        </div>
        <div className="flex items-center gap-2">
          <KvmTooltip label="查看详情" placement="top">
            <button
              type="button"
              aria-label="查看详情"
              onClick={() => onDetail(item)}
              className="kvm-action-button inline-flex h-8 w-8 items-center justify-center rounded-lg border"
              style={buttonStyle}
            >
              <InfoIcon size={15} />
            </button>
          </KvmTooltip>
          <NetworkIcon size={18} style={{ color: '#2dd4bf' }} />
        </div>
      </div>
      <div className="mt-4 space-y-2 text-sm" style={{ color: 'var(--kvm-text-muted)' }}>
        <Row label="设备" value={item.bridge || '-'} />
        <Row label="转发" value={(item.forward || 'isolate').toUpperCase()} />
        <Row label="子网" value={item.subnet || '-'} />
        <Row label="DHCP" value={item.dhcp ? '启用' : '关闭'} />
      </div>
    </article>
  );
}

function NetworkPoolDetailDialog({
  agentId,
  hostName,
  item,
  canManage,
  onClose,
  onRefresh,
  onUpdateState,
  onUpdateAutostart,
  onDeleted,
}: {
  agentId: string;
  hostName: string;
  item: NetworkPool;
  canManage: boolean;
  onClose: () => void;
  onRefresh: () => Promise<void>;
  onUpdateState: (active: boolean) => Promise<unknown>;
  onUpdateAutostart: (autostart: boolean) => Promise<unknown>;
  onDeleted: () => Promise<void>;
}) {
  const [localItem, setLocalItem] = useState(item);
  const [busy, setBusy] = useState('');
  const [confirmDelete, setConfirmDelete] = useState(false);
  const active = isPoolActive(localItem.state);

  useEffect(() => {
    setLocalItem(item);
  }, [item]);

  async function run(action: string, task: () => Promise<unknown>) {
    setBusy(action);
    try {
      const result = await task();
      if (action === 'state') {
        const activeResult = Boolean((result as { active?: boolean } | undefined)?.active);
        setLocalItem(current => ({ ...current, state: activeResult ? 'yes' : 'no' }));
      }
      if (action === 'autostart') {
        const autostartResult = Boolean((result as { autostart?: boolean } | undefined)?.autostart);
        setLocalItem(current => ({ ...current, autostart: autostartResult }));
      }
      toast.success('网络池配置已更新');
      await onRefresh();
    } catch (error) {
      toast.error(error instanceof Error ? error.message : '修改网络池失败');
    } finally {
      setBusy('');
    }
  }

  async function removePool() {
    setBusy('delete');
    try {
      await deleteNetworkPool(agentId, localItem.name);
      toast.success('网络池已删除');
      await onDeleted();
      onClose();
    } catch (error) {
      toast.error(error instanceof Error ? error.message : '删除网络池失败');
    } finally {
      setBusy('');
      setConfirmDelete(false);
    }
  }

  return (
    <DialogPortal>
      <div className="kvm-dialog-backdrop fixed inset-0 z-50 flex items-center justify-center px-3">
        <div className="kvm-dialog-panel max-h-[82vh] w-[min(92vw,800px)] overflow-hidden rounded-2xl">
          <header
            className="flex items-center justify-between border-b px-5 py-4"
            style={{ borderColor: 'var(--kvm-border)' }}
          >
            <div className="min-w-0">
              <h2 className="truncate text-base font-semibold" style={{ color: 'var(--kvm-text)' }}>
                {localItem.name}
              </h2>
              <p className="mt-1 text-xs" style={{ color: 'var(--kvm-text-muted)' }}>
                {hostName} · {(localItem.forward || 'isolate').toUpperCase()}
              </p>
            </div>
            <button
              type="button"
              onClick={onClose}
              aria-label="关闭"
              className="kvm-action-button inline-flex h-8 w-8 items-center justify-center rounded-lg border"
              style={buttonStyle}
            >
              <XIcon size={15} />
            </button>
          </header>
          <div className="kvm-hidden-scrollbar max-h-[calc(82vh-74px)] overflow-y-auto p-5">
            <section className="kvm-dialog-card rounded-xl p-4">
              <div className="kvm-detail-tile-grid grid gap-3 md:grid-cols-2">
                <InfoTile label="网络池" value={localItem.name} />
                <InfoTile label="桥接设备" value={localItem.bridge || '-'} />
                <InfoTile label="转发模式" value={(localItem.forward || 'isolate').toUpperCase()} />
                <InfoTile label="子网池" value={localItem.subnet || '-'} />
                <InfoTile
                  label="DHCP 范围"
                  value={
                    localItem.dhcpStart && localItem.dhcpEnd
                      ? `${localItem.dhcpStart} - ${localItem.dhcpEnd}`
                      : '-'
                  }
                />
                <ValueTile
                  label="Open vSwitch"
                  value={localItem.openVSwitch ? '启用' : '关闭'}
                  enabled={localItem.openVSwitch}
                />
                <ValueTile
                  label="DHCP"
                  value={localItem.dhcp ? '启用' : '关闭'}
                  enabled={localItem.dhcp}
                />
                <ActionTile
                  label="状态"
                  value={<StateBadge active={active} />}
                  buttonLabel={active ? '停止' : '启动'}
                  tone={active ? 'danger' : 'success'}
                  disabled={busy !== ''}
                  canManage={canManage}
                  onClick={() => void run('state', () => onUpdateState(!active))}
                  extra={
                    <DeletePoolButton
                      active={active}
                      disabled={busy !== ''}
                      onClick={() => setConfirmDelete(true)}
                    />
                  }
                />
                <ActionTile
                  label="随物理机同启"
                  value={<AutostartBadge enabled={localItem.autostart} />}
                  buttonLabel={localItem.autostart ? '禁用' : '启用'}
                  tone={localItem.autostart ? 'warning' : 'primary'}
                  disabled={busy !== ''}
                  canManage={canManage}
                  onClick={() =>
                    void run('autostart', () => onUpdateAutostart(!localItem.autostart))
                  }
                />
              </div>
            </section>
            {(localItem.fixedAddresses || []).length > 0 && (
              <FixedAddressTable items={localItem.fixedAddresses || []} />
            )}
          </div>
        </div>
        {confirmDelete && (
          <ConfirmDialog
            title="删除网络池"
            message={`确认删除网络池 ${localItem.name}？请确认该网络池已停止且不再使用`}
            busy={busy === 'delete'}
            onClose={() => setConfirmDelete(false)}
            onConfirm={() => void removePool()}
          />
        )}
      </div>
    </DialogPortal>
  );
}

function InfoTile({ label, value }: { label: string; value: string }) {
  return (
    <div
      className="rounded-lg border p-3"
      style={{ borderColor: 'var(--kvm-border)', background: 'var(--kvm-control-bg-soft)' }}
    >
      <div className="text-xs" style={{ color: 'var(--kvm-text-muted)' }}>
        {label}
      </div>
      <div
        className="mt-2 min-w-0 break-all font-mono text-sm font-semibold"
        style={{ color: 'var(--kvm-text)' }}
      >
        {value}
      </div>
    </div>
  );
}

function ValueTile({ label, value, enabled }: { label: string; value: string; enabled: boolean }) {
  const color = enabled ? actionColor('success') : actionColor('danger');
  return (
    <div
      className="rounded-lg border p-3"
      style={{ borderColor: color.border, background: color.background }}
    >
      <div className="text-xs" style={{ color: 'var(--kvm-text-muted)' }}>
        {label}
      </div>
      <div className="mt-2">
        <span
          className="inline-flex h-7 items-center rounded-full border px-2.5 text-xs font-semibold"
          style={{ background: color.buttonBg, borderColor: color.border, color: color.text }}
        >
          {value}
        </span>
      </div>
    </div>
  );
}

function ActionTile({
  label,
  value,
  buttonLabel,
  tone,
  disabled,
  canManage,
  onClick,
  extra,
}: {
  label: string;
  value: ReactNode;
  buttonLabel: string;
  tone: 'primary' | 'success' | 'warning' | 'danger';
  disabled: boolean;
  canManage: boolean;
  onClick: () => void;
  extra?: ReactNode;
}) {
  const color = actionColor(tone);
  return (
    <div
      className="rounded-lg border p-3"
      style={{ borderColor: color.border, background: color.background }}
    >
      <div className="text-xs" style={{ color: 'var(--kvm-text-muted)' }}>
        {label}
      </div>
      <div className="mt-2 flex items-center justify-between gap-3">
        {value}
        {canManage && (
          <div className="flex shrink-0 items-center gap-2">
            <button
              type="button"
              disabled={disabled}
              onClick={onClick}
              className="kvm-action-button h-8 rounded-lg border px-3 text-xs font-semibold disabled:opacity-60"
              style={{ borderColor: color.border, background: color.buttonBg, color: color.text }}
            >
              {buttonLabel}
            </button>
            {extra}
          </div>
        )}
      </div>
    </div>
  );
}

function DeletePoolButton({
  active,
  disabled,
  onClick,
}: {
  active: boolean;
  disabled: boolean;
  onClick: () => void;
}) {
  const button = (
    <button
      type="button"
      disabled={active || disabled}
      onClick={onClick}
      className="kvm-action-button kvm-danger-button inline-flex h-8 items-center gap-1.5 rounded-lg border px-2.5 text-xs font-semibold disabled:cursor-not-allowed disabled:opacity-50"
      style={{
        borderColor: 'rgba(239,68,68,0.35)',
        color: '#f87171',
        background: 'rgba(239,68,68,0.1)',
      }}
    >
      <Trash2Icon size={13} />
      删除
    </button>
  );
  if (!active) return button;
  return (
    <KvmTooltip label="运行中的网络池需要先停止后才能删除" placement="top">
      {button}
    </KvmTooltip>
  );
}

function FixedAddressTable({ items }: { items: NetworkFixedAddress[] }) {
  const [query, setQuery] = useState('');
  const filtered = items.filter(item =>
    `${item.address} ${item.mac}`.toLowerCase().includes(query.trim().toLowerCase())
  );
  return (
    <section
      className="mt-4 rounded-xl border p-4"
      style={{ borderColor: 'var(--kvm-border)', background: 'var(--kvm-card)' }}
    >
      <div className="mb-3 flex flex-wrap items-center justify-between gap-3">
        <h3 className="text-base font-semibold" style={{ color: 'var(--kvm-text)' }}>
          固定地址
        </h3>
        <div className="flex items-center gap-2">
          <input
            value={query}
            onChange={event => setQuery(event.target.value)}
            placeholder="搜索 IP 或 MAC"
            className="h-9 w-48 rounded-lg px-3 text-sm outline-none"
            style={fieldStyle}
          />
          <button
            type="button"
            onClick={() => setQuery('')}
            className="kvm-action-button h-9 rounded-lg border px-3 text-sm"
            style={buttonStyle}
          >
            清空
          </button>
        </div>
      </div>
      <div
        className="overflow-hidden rounded-lg border"
        style={{ borderColor: 'var(--kvm-border)' }}
      >
        <div className="kvm-hidden-scrollbar max-h-56 overflow-y-auto">
          <table className="w-full min-w-[420px] text-sm">
            <thead
              className="sticky top-0 z-10"
              style={{ background: 'var(--kvm-table-head-bg)', color: 'var(--kvm-text)' }}
            >
              <tr>
                <th className="px-3 py-3 text-center">地址</th>
                <th className="px-3 py-3 text-center">MAC</th>
              </tr>
            </thead>
            <tbody style={{ color: 'var(--kvm-text-muted)' }}>
              {filtered.length === 0 && (
                <tr>
                  <td colSpan={2} className="px-3 py-6 text-center">
                    暂无匹配地址
                  </td>
                </tr>
              )}
              {filtered.map(item => (
                <tr
                  key={`${item.address}-${item.mac}`}
                  style={{ borderTop: '1px solid var(--kvm-border)' }}
                >
                  <td
                    className="px-3 py-3 text-center font-mono"
                    style={{ color: 'var(--kvm-text)' }}
                  >
                    {item.address}
                  </td>
                  <td className="px-3 py-3 text-center font-mono">{item.mac}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>
    </section>
  );
}

function NetworkPoolDialog({
  hostName,
  existing,
  onClose,
  onSubmit,
}: {
  hostName: string;
  existing: NetworkPool[];
  onClose: () => void;
  onSubmit: (payload: NetworkPoolCreatePayload) => Promise<void>;
}) {
  const [name, setName] = useState('');
  const [subnet, setSubnet] = useState('');
  const [dhcp, setDhcp] = useState(true);
  const [fixedAddress, setFixedAddress] = useState(false);
  const [type, setType] = useState('BRIDGE');
  const [bridge, setBridge] = useState('');
  const [openVSwitch, setOpenVSwitch] = useState(false);
  const bridgeMode = type === 'BRIDGE';
  const payload = {
    name,
    subnet,
    dhcp,
    fixedAddress,
    type: type.toLowerCase(),
    bridge,
    openVSwitch,
  };

  return (
    <DialogPortal>
      <div className="kvm-dialog-backdrop fixed inset-0 z-50 flex items-center justify-center px-3">
        <div className="kvm-dialog-panel w-[min(92vw,620px)] overflow-hidden rounded-2xl">
          <header
            className="flex items-center justify-between border-b px-5 py-4"
            style={{ borderColor: 'var(--kvm-border)' }}
          >
            <div>
              <h2 className="text-base font-semibold" style={{ color: 'var(--kvm-text)' }}>
                新增网络池
              </h2>
              <p className="mt-1 text-xs" style={{ color: 'var(--kvm-text-muted)' }}>
                {hostName}
              </p>
            </div>
            <button
              type="button"
              onClick={onClose}
              aria-label="关闭"
              className="kvm-action-button inline-flex h-8 w-8 items-center justify-center rounded-lg border"
              style={buttonStyle}
            >
              <XIcon size={15} />
            </button>
          </header>
          <div className="mx-auto max-w-[500px] space-y-4 p-5">
            <Field label="名称">
              <input
                value={name}
                onChange={event => setName(event.target.value)}
                placeholder="default"
                className="h-10 w-full rounded-lg px-3 text-sm outline-none"
                style={fieldStyle}
              />
            </Field>
            <Field label="网络类型">
              <SelectMenu
                value={type}
                placeholder="请选择网络类型"
                options={networkTypes.map(item => ({ value: item, label: item }))}
                placement="bottom"
                onChange={setType}
              />
            </Field>
            {!bridgeMode && (
              <>
                <Field label="子网池">
                  <input
                    value={subnet}
                    onChange={event => setSubnet(event.target.value)}
                    placeholder="192.168.100.0/24"
                    className="h-10 w-full rounded-lg px-3 text-sm outline-none"
                    style={fieldStyle}
                  />
                </Field>
                <Field label="DHCP">
                  <RoundedCheckbox checked={dhcp} label="启用 DHCP" onChange={setDhcp} />
                </Field>
                <Field label={<FixedAddressLabel />}>
                  <RoundedCheckbox
                    checked={fixedAddress}
                    label="启用 Fixed Address"
                    onChange={setFixedAddress}
                  />
                </Field>
              </>
            )}
            {bridgeMode && (
              <>
                <Field label="桥接名称">
                  <input
                    value={bridge}
                    onChange={event => setBridge(event.target.value)}
                    placeholder="br0"
                    className="h-10 w-full rounded-lg px-3 text-sm outline-none"
                    style={fieldStyle}
                  />
                </Field>
                <Field label={<OpenVSwitchLabel />}>
                  <RoundedCheckbox
                    checked={openVSwitch}
                    label="启用 Open vSwitch"
                    onChange={setOpenVSwitch}
                  />
                </Field>
              </>
            )}
          </div>
          <footer
            className="flex justify-end gap-2 border-t px-5 py-4"
            style={{ borderColor: 'var(--kvm-border)' }}
          >
            <button
              type="button"
              onClick={onClose}
              className="kvm-action-button rounded-lg border px-4 py-2 text-sm"
              style={buttonStyle}
            >
              关闭
            </button>
            <button
              type="button"
              onClick={() => void onSubmit(payload)}
              className="kvm-action-button rounded-lg border px-4 py-2 text-sm font-semibold"
              style={primaryButtonStyle}
            >
              创建
            </button>
          </footer>
        </div>
      </div>
    </DialogPortal>
  );
}

function RoundedCheckbox({
  checked,
  label,
  onChange,
}: {
  checked: boolean;
  label: string;
  onChange: (checked: boolean) => void;
}) {
  return (
    <label className="inline-flex min-h-10 cursor-pointer items-center gap-2.5">
      <input
        type="checkbox"
        checked={checked}
        onChange={event => onChange(event.target.checked)}
        className="peer sr-only"
      />
      <span
        className="inline-flex h-5 w-5 shrink-0 items-center justify-center rounded-md border transition"
        style={{
          background: checked ? 'rgba(45,212,191,0.18)' : 'var(--kvm-control-bg)',
          borderColor: checked ? 'rgba(45,212,191,0.62)' : 'var(--kvm-border)',
          boxShadow: checked
            ? 'inset 0 1px 0 rgba(255,255,255,0.2), 0 0 0 2px rgba(45,212,191,0.08)'
            : 'inset 0 1px 0 rgba(255,255,255,0.08)',
          color: checked ? '#5eead4' : 'transparent',
        }}
      >
        <CheckIcon size={14} strokeWidth={3} />
      </span>
      <span
        className="text-sm"
        style={{ color: checked ? 'var(--kvm-text)' : 'var(--kvm-text-muted)' }}
      >
        {label}
      </span>
    </label>
  );
}

function FixedAddressLabel() {
  return (
    <KvmTooltip label="固定地址" placement="bottom" align="center" zIndex={1700}>
      <span className="inline-flex cursor-help items-center">Fixed Address</span>
    </KvmTooltip>
  );
}

function OpenVSwitchLabel() {
  return (
    <KvmTooltip label="开放虚拟交换机" placement="bottom" align="center" zIndex={1700}>
      <span className="inline-flex cursor-help items-center">Open vSwitch</span>
    </KvmTooltip>
  );
}

function validateNetworkPayload(payload: NetworkPoolCreatePayload, existing: NetworkPool[]) {
  const type = payload.type.toLowerCase();
  if (!payload.name.trim()) return '请填写名称';
  if (existing.some(item => item.name.toLowerCase() === payload.name.trim().toLowerCase()))
    return '名称已存在，请重新更换名称';
  if (type === 'bridge') {
    if (!payload.bridge?.trim()) return '请填写桥接名称';
    return '';
  }
  if (!payload.subnet?.trim()) return '请填写子网池';
  if (!isCidr(payload.subnet)) return '请填写正确的 CIDR 子网，例如 192.168.100.0/24';
  if (payload.fixedAddress && !payload.dhcp) return '固定地址需要先启用 DHCP';
  if (existing.some(item => sameSubnet(item.subnet, payload.subnet)))
    return '子网池已存在，请重新更换子网';
  return '';
}

function isCidr(value?: string) {
  if (!value) return false;
  const match = value.trim().match(/^(\d{1,3}\.){3}\d{1,3}\/([1-9]|[12]\d|3[0-2])$/);
  if (!match) return false;
  return value
    .split('/')[0]
    .split('.')
    .every(part => Number(part) >= 0 && Number(part) <= 255);
}

function sameSubnet(existing: string, next?: string) {
  if (!existing || !next) return false;
  return existing === next || existing === gatewayFromCidr(next);
}

function gatewayFromCidr(cidr: string) {
  const [address] = cidr.trim().split('/');
  const parts = address.split('.');
  if (parts.length !== 4) return address;
  parts[3] = '1';
  return parts.join('.');
}

function ConfirmDialog({
  title,
  message,
  busy,
  onClose,
  onConfirm,
}: {
  title: string;
  message: string;
  busy: boolean;
  onClose: () => void;
  onConfirm: () => void;
}) {
  return (
    <DialogPortal>
      <div
        className="fixed inset-0 z-[70] flex items-center justify-center px-4"
        style={{ background: 'rgba(2,6,23,0.36)' }}
      >
        <div
          className="kvm-dialog-panel w-full max-w-sm rounded-xl p-5"
          style={{ borderColor: 'rgba(239,68,68,0.34)' }}
        >
          <div className="flex items-start justify-between gap-3">
            <div>
              <h3 className="text-base font-semibold" style={{ color: 'var(--kvm-text)' }}>
                {title}
              </h3>
              <p className="mt-2 text-sm leading-6" style={{ color: 'var(--kvm-text-muted)' }}>
                {message}
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
          </div>
          <div className="mt-5 flex justify-end gap-2">
            <button
              type="button"
              disabled={busy}
              onClick={onClose}
              className="kvm-action-button h-9 rounded-lg border px-4 text-sm disabled:opacity-60"
              style={buttonStyle}
            >
              取消
            </button>
            <button
              type="button"
              disabled={busy}
              onClick={onConfirm}
              className="kvm-action-button kvm-danger-button inline-flex h-9 items-center gap-2 rounded-lg border px-4 text-sm font-semibold disabled:opacity-60"
              style={{
                borderColor: 'rgba(239,68,68,0.35)',
                color: '#f87171',
                background: 'rgba(239,68,68,0.12)',
              }}
            >
              {busy && <LoaderCircleIcon size={14} className="animate-spin" />}确认删除
            </button>
          </div>
        </div>
      </div>
    </DialogPortal>
  );
}

function MetricCard({
  icon: Icon,
  label,
  value,
}: {
  icon: typeof NetworkIcon;
  label: string;
  value: string;
}) {
  return (
    <div className="kvm-surface-3d rounded-xl p-4">
      <Icon size={17} style={{ color: '#3b82f6' }} />
      <div className="mt-3 text-xs" style={{ color: 'var(--kvm-text-muted)' }}>
        {label}
      </div>
      <div className="mt-1 text-lg font-semibold" style={{ color: 'var(--kvm-text)' }}>
        {value}
      </div>
    </div>
  );
}

function Empty({ text }: { text: string }) {
  return (
    <div
      className="kvm-empty-state col-span-full rounded-xl p-8 text-center text-sm"
      style={{ color: 'var(--kvm-text-muted)' }}
    >
      {text}
    </div>
  );
}

function Field({ label, children }: { label: ReactNode; children: ReactNode }) {
  return (
    <label className="grid gap-2 md:grid-cols-[100px_minmax(0,340px)] md:items-center md:justify-center">
      <span className="text-sm font-semibold md:text-right" style={{ color: 'var(--kvm-text)' }}>
        {label}
      </span>
      {children}
    </label>
  );
}

function Row({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex justify-between gap-3">
      <span>{label}</span>
      <span className="min-w-0 truncate font-mono" style={{ color: 'var(--kvm-text)' }}>
        {value}
      </span>
    </div>
  );
}

function friendlyPoolError(error: unknown) {
  const raw = error instanceof Error ? error.message : '操作失败';
  const compact = raw
    .replace(/\s+/g, ' ')
    .trim()
    .replace(/^创建网络池失败[:：]\s*/, '');
  const lower = compact.toLowerCase();
  if (lower.includes('subnet is too small for dhcp')) return '子网可用地址不足，无法启用 DHCP';
  if (lower.includes('subnet is too small for fixed address'))
    return '子网可用地址不足，无法生成固定地址';
  if (lower.includes('fixed address range is too large'))
    return '固定地址范围过大，请缩小子网或关闭固定地址';
  if (lower.includes('bridge device does not exist'))
    return '桥接设备不存在，请先在接口页创建或选择已有 bridge';
  if (lower.includes('bridge device is required')) return '请填写桥接名称';
  if (lower.includes('ip forwarding is disabled')) return '宿主机 IP 转发未启用，请确保已配置';
  if (lower.includes('ip forwarding sysctl is unavailable'))
    return '宿主机 IP 转发配置不可用，请检查 KVM 网络环境';
  if (lower.includes('already exists')) return '名称已存在，请重新更换名称';
  if (lower.includes('name is required')) return '请填写名称';
  if (lower.includes('unsupported')) return '当前类型或参数不受支持';
  if (lower.includes('virsh') || lower.includes('error:'))
    return '宿主机命令执行失败，请检查名称、子网和权限';
  return compact.length > 120 ? `${compact.slice(0, 120)}...` : compact;
}

function actionColor(tone: 'primary' | 'success' | 'warning' | 'danger') {
  switch (tone) {
    case 'success':
      return {
        background: 'rgba(16,185,129,0.08)',
        buttonBg: 'rgba(16,185,129,0.12)',
        border: 'rgba(16,185,129,0.34)',
        text: 'var(--kvm-status-green-text)',
      };
    case 'warning':
      return {
        background: 'rgba(245,158,11,0.08)',
        buttonBg: 'rgba(245,158,11,0.12)',
        border: 'rgba(245,158,11,0.34)',
        text: 'var(--kvm-status-yellow-text)',
      };
    case 'danger':
      return {
        background: 'rgba(239,68,68,0.08)',
        buttonBg: 'rgba(239,68,68,0.12)',
        border: 'rgba(239,68,68,0.34)',
        text: 'var(--kvm-status-red-text)',
      };
    default:
      return {
        background: 'rgba(59,130,246,0.08)',
        buttonBg: 'rgba(59,130,246,0.12)',
        border: 'rgba(59,130,246,0.34)',
        text: 'var(--kvm-status-blue-text)',
      };
  }
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
