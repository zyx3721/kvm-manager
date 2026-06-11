import { useEffect, useMemo, useState } from 'react';
import { HardDriveIcon, NetworkIcon, PlusIcon, Trash2Icon } from 'lucide-react';
import { toast } from 'sonner';
import { SelectMenu } from '../../../../components/kvm/SelectMenu';
import { KvmTooltip } from '../../../../components/kvm/StatusBadge';
import {
  fetchNetworkPools,
  fetchStoragePools,
  updateVMDevices,
  type NetworkPool,
  type StoragePool,
  type VMConfig,
  type VMConfigDisk,
  type VirtualMachine,
} from '../../../../lib/api';
import { formatBytes } from '../../../../lib/format';
import { PrimaryButton } from '../VMEditControls';
import { DeviceField } from './DeviceField';
import { CardSection, InlineNotice, SummaryCard, fieldStyle, inputClass } from './EditShared';
import { NewDiskCard, volumeExtensionForFormat, type NewDiskDraft } from './NewDiskCard';
import { isVMRunning } from '../../utils/vmStatus';
const bytesPerGB = 1024 ** 3;
const interfaceModelOptions = ['virtio', 'e1000', 'e1000e', 'rtl8139', 'vmxnet3'].map(value => ({
  value,
  label: value,
}));

export function DevicesPanel({
  vm,
  config,
  diskLabel,
  onConfigChange,
}: {
  vm: VirtualMachine;
  config: VMConfig | null;
  diskLabel: string;
  onConfigChange: (config: VMConfig) => void;
}) {
  const disks = useMemo<VMConfigDisk[]>(
    () =>
      config?.disks.length
        ? config.disks
        : vm.disks.length > 0
          ? vm.disks.map(disk => ({
              ...disk,
              sourcePath: disk.path,
              pool: '',
              bus: '',
              device: 'disk',
              type: 'file',
            }))
          : [
              {
                name: 'disk',
                path: '-',
                sourcePath: '-',
                bytes: vm.diskBytes,
                pool: '',
                bus: '',
                device: 'disk',
                type: 'file',
              },
            ],
    [config?.disks, vm.diskBytes, vm.disks]
  );
  const interfaces = useMemo(
    () => (config?.interfaces.length ? config.interfaces : []),
    [config?.interfaces]
  );
  const [networkPools, setNetworkPools] = useState<NetworkPool[]>([]);
  const [storagePools, setStoragePools] = useState<StoragePool[]>([]);
  const [interfacePools, setInterfacePools] = useState<Record<string, string>>({});
  const [deletedInterfaces, setDeletedInterfaces] = useState<Record<string, boolean>>({});
  const [newInterfaces, setNewInterfaces] = useState<
    Array<{ id: string; source: string; model: string }>
  >([]);
  const [diskCapacities, setDiskCapacities] = useState<Record<string, string>>({});
  const [deletedDisks, setDeletedDisks] = useState<Record<string, boolean>>({});
  const [newDisks, setNewDisks] = useState<NewDiskDraft[]>([]);
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const running = isVMRunning(config?.status || vm.status);
  const diskByName = useMemo(() => new Map(disks.map(disk => [disk.name, disk])), [disks]);
  const diskResizes = useMemo(
    () =>
      disks
        .filter(disk => !deletedDisks[disk.name])
        .map(disk => ({ disk, capacityBytes: gbInputToBytes(diskCapacities[disk.name]) }))
        .filter(item => item.capacityBytes > item.disk.bytes),
    [deletedDisks, diskCapacities, disks]
  );
  const activeInterfaces = useMemo(
    () => interfaces.filter(item => !deletedInterfaces[interfaceKey(item)]),
    [deletedInterfaces, interfaces]
  );
  const activeDisks = useMemo(
    () => disks.filter(disk => !deletedDisks[disk.name]),
    [deletedDisks, disks]
  );
  const networkChanged = interfaces.some(item => {
    if (deletedInterfaces[interfaceKey(item)]) return false;
    return (
      (interfacePools[interfaceKey(item)] || networkPoolValueForInterface(item, networkPools)) !==
      networkPoolValueForInterface(item, networkPools)
    );
  });
  const interfaceDeleteChanged = Object.values(deletedInterfaces).some(Boolean);
  const diskDeleteChanged = Object.values(deletedDisks).some(Boolean);
  const newInterfaceReady = newInterfaces.length > 0;
  const newDiskReady = newDisks.length > 0;
  const changed =
    networkChanged ||
    interfaceDeleteChanged ||
    newInterfaceReady ||
    diskResizes.length > 0 ||
    diskDeleteChanged ||
    newDiskReady;

  useEffect(() => {
    setInterfacePools(
      Object.fromEntries(
        interfaces.map(item => [
          interfaceKey(item),
          networkPoolValueForInterface(item, networkPools),
        ])
      )
    );
    setDeletedInterfaces({});
  }, [interfaces, networkPools]);

  useEffect(() => {
    setDiskCapacities(
      Object.fromEntries(disks.map(disk => [disk.name, bytesToGBInput(disk.bytes)]))
    );
    setDeletedDisks({});
  }, [disks]);

  useEffect(() => {
    if (!vm.hostId) return;
    let ignore = false;
    setLoading(true);
    Promise.all([fetchNetworkPools(vm.hostId), fetchStoragePools(vm.hostId)])
      .then(([networkBody, storageBody]) => {
        if (ignore) return;
        setNetworkPools(networkBody.items);
        setStoragePools(storageBody.items);
        setInterfacePools(
          Object.fromEntries(
            interfaces.map(item => [
              interfaceKey(item),
              networkPoolValueForInterface(item, networkBody.items),
            ])
          )
        );
      })
      .catch(error => toast.error(error instanceof Error ? error.message : '读取资源池失败'))
      .finally(() => {
        if (!ignore) setLoading(false);
      });
    return () => {
      ignore = true;
    };
  }, [interfaces, vm.hostId]);

  async function handleSave() {
    if (!config) return toast.warning('虚拟机配置尚未加载完成');
    if (loading) return toast.warning('资源池正在加载，请稍后再试');
    const shrinkDisks = disks.filter(disk => {
      const value = gbInputToBytes(diskCapacities[disk.name]);
      return value > 0 && value < disk.bytes;
    });
    if (shrinkDisks.length > 0) {
      return toast.warning(
        `${shrinkDisks.map(disk => disk.name || '磁盘').join('、')} 磁盘不能修改小于当前容量`
      );
    }
    if (!changed) return toast.warning('请先修改配置');
    if (running && diskDeleteChanged) {
      return toast.warning('虚拟机正在运行，不支持删除磁盘，请先关闭虚拟机后再操作');
    }
    if (running && (networkChanged || interfaceDeleteChanged || newInterfaceReady)) {
      return toast.warning('虚拟机正在运行，不支持修改网络设备，请先关闭虚拟机后再操作');
    }
    if (activeDisks.length === 0) return toast.warning('至少保留一块磁盘');

    const resizePayload = disks
      .filter(disk => !deletedDisks[disk.name])
      .map(disk => ({ name: disk.name, capacityBytes: gbInputToBytes(diskCapacities[disk.name]) }))
      .filter(item => item.capacityBytes > (diskByName.get(item.name)?.bytes || 0));

    const normalizedNewDisks = newDisks.map(disk => ({
      name: disk.name.trim(),
      pool: disk.pool.trim(),
      target: disk.target.trim(),
      bus: disk.bus.trim(),
      format: disk.format.trim(),
      capacityBytes: gbInputToBytes(disk.capacityGB),
      preallocMetadata: disk.preallocMetadata,
    }));
    if (
      normalizedNewDisks.some(
        item =>
          !item.name ||
          !item.pool ||
          !item.target ||
          !item.bus ||
          !item.format ||
          item.capacityBytes <= 0
      )
    ) {
      return toast.warning('请完整填写新增磁盘配置');
    }
    const extensionError = newDiskExtensionError(normalizedNewDisks);
    if (extensionError) return toast.warning(extensionError);
    if (
      hasDuplicate(
        normalizedNewDisks.map(item => item.target),
        disks.map(disk => disk.name)
      )
    ) {
      return toast.warning('新增磁盘目标设备不能重复');
    }
    if (hasDuplicate(normalizedNewDisks.map(item => item.name))) {
      return toast.warning('新增磁盘卷名称不能重复');
    }

    const normalizedNewInterfaces = newInterfaces.map(item => ({
      source: item.source.trim(),
      model: item.model.trim() || 'virtio',
    }));
    if (normalizedNewInterfaces.some(item => !item.source || !item.model)) {
      return toast.warning('请完整填写新增网卡配置');
    }

    const payload = {
      interfaces: running
        ? []
        : interfaces
            .filter(item => !deletedInterfaces[interfaceKey(item)])
            .map(item => ({
              name: item.name,
              mac: item.mac,
              source: (interfacePools[interfaceKey(item)] || item.source || '').trim(),
            })),
      newInterfaces: running ? [] : normalizedNewInterfaces,
      deletedInterfaces: running
        ? []
        : interfaces
            .filter(item => deletedInterfaces[interfaceKey(item)])
            .map(item => ({ name: item.name, mac: item.mac })),
      diskResizes: resizePayload,
      newDisks: normalizedNewDisks,
      deletedDisks: disks
        .filter(disk => deletedDisks[disk.name])
        .map(disk => ({ name: disk.name })),
    };
    if (payload.interfaces.some(item => !item.source)) return toast.warning('请选择网络池');
    setSaving(true);
    try {
      const result = await updateVMDevices(vm.id, payload);
      onConfigChange(result.config);
      setNewDisks([]);
      setNewInterfaces([]);
      setDeletedInterfaces({});
      setDeletedDisks({});
      toast.success('修改成功');
    } catch (error) {
      toast.error(error instanceof Error ? error.message : '修改失败');
    } finally {
      setSaving(false);
    }
  }

  function addDisk() {
    const target = nextDiskTarget(disks, newDisks);
    const format = 'qcow2';
    const pool = preferredStoragePool(disks, storagePools);
    setNewDisks(current => [
      ...current,
      {
        id: `${Date.now()}-${target}`,
        target,
        name: `${vm.name}-${target}.${format}`,
        pool,
        bus: 'virtio',
        format,
        capacityGB: '20',
        preallocMetadata: false,
      },
    ]);
  }

  function addInterface() {
    setNewInterfaces(current => [
      ...current,
      {
        id: `${Date.now()}-${current.length}`,
        source: networkPools[0]?.name || '',
        model: 'virtio',
      },
    ]);
  }

  return (
    <section className="mx-auto max-w-4xl space-y-4">
      <div className="grid gap-3 md:grid-cols-3">
        <SummaryCard
          icon={NetworkIcon}
          label="网络吞吐"
          value={`${Math.round((vm.networkRxBytesPerSecond + vm.networkTxBytesPerSecond) / 1024)} KB/s`}
          color="#22d3ee"
        />
        <SummaryCard
          icon={NetworkIcon}
          label="网络设备"
          value={`${activeInterfaces.length + newInterfaces.length} 块网卡`}
          color="#60a5fa"
        />
        <SummaryCard icon={HardDriveIcon} label="存储设备" value={diskLabel} color="#f59e0b" />
      </div>
      <InlineNotice tone={running ? 'warning' : 'info'}>
        {running
          ? '虚拟机正在运行，支持扩容已有磁盘和添加新磁盘；网络设备和删除磁盘需关机后操作'
          : '网络池可直接切换；支持新增/删除网卡、扩容/新增/删除磁盘，删除磁盘会同步删除对应存储卷'}
      </InlineNotice>
      <CardSection title="网络设备">
        <div className="space-y-2.5">
          {interfaces.length ? (
            interfaces.map((item, index) => {
              const key = interfaceKey(item);
              const deleting = deletedInterfaces[key];
              const value = interfacePools[key] || networkPoolValueForInterface(item, networkPools);
              const selectedPool = networkPools.find(pool => pool.name === value);
              return (
                <DeviceRow
                  key={item.name || item.mac || index}
                  name={item.name || `网卡 ${index + 1}`}
                  detail={deleting ? '保存后删除此网卡' : networkDeviceDetail(item, selectedPool)}
                  danger={deleting}
                >
                  <div className="grid gap-2 sm:grid-cols-[minmax(0,1fr)_36px]">
                    <SelectMenu
                      value={value}
                      disabled={
                        running || deleting || loading || saving || networkPools.length === 0
                      }
                      placeholder="选择网络池"
                      maxVisibleItems={4}
                      options={networkPools.map(pool => ({
                        value: pool.name,
                        label: pool.name,
                        tooltip: networkPoolTooltip(pool),
                      }))}
                      onChange={next => setInterfacePools(current => ({ ...current, [key]: next }))}
                    />
                    <IconAction
                      label={deleting ? '取消删除网卡' : '删除网卡'}
                      disabled={running || saving}
                      danger={!deleting}
                      active={deleting}
                      onClick={() =>
                        setDeletedInterfaces(current => ({ ...current, [key]: !current[key] }))
                      }
                    />
                  </div>
                </DeviceRow>
              );
            })
          ) : (
            <EmptyText>{vm.primaryIp || '未识别到网络设备'}</EmptyText>
          )}
          {newInterfaces.map((item, index) => (
            <DeviceRow
              key={item.id}
              name={`新增网卡 ${index + 1}`}
              detail="保存后写入虚拟机 XML"
              active
            >
              <div className="grid items-end gap-2 sm:grid-cols-[minmax(0,1fr)_110px_36px]">
                <DeviceField label="网络池">
                  <SelectMenu
                    value={item.source}
                    disabled={running || loading || saving || networkPools.length === 0}
                    placeholder="选择网络池"
                    maxVisibleItems={4}
                    options={networkPools.map(pool => ({
                      value: pool.name,
                      label: pool.name,
                      tooltip: networkPoolTooltip(pool),
                    }))}
                    onChange={source =>
                      setNewInterfaces(current =>
                        current.map(next => (next.id === item.id ? { ...next, source } : next))
                      )
                    }
                  />
                </DeviceField>
                <DeviceField label="网卡模型">
                  <SelectMenu
                    value={item.model}
                    disabled={running || loading || saving}
                    placeholder="模型"
                    options={interfaceModelOptions}
                    onChange={model =>
                      setNewInterfaces(current =>
                        current.map(next => (next.id === item.id ? { ...next, model } : next))
                      )
                    }
                  />
                </DeviceField>
                <IconAction
                  label="移除新增网卡"
                  disabled={running || saving}
                  danger
                  onClick={() =>
                    setNewInterfaces(current => current.filter(next => next.id !== item.id))
                  }
                />
              </div>
            </DeviceRow>
          ))}
          <button
            type="button"
            disabled={running || loading || saving || networkPools.length === 0}
            onClick={addInterface}
            className="kvm-action-button inline-flex h-9 items-center gap-2 rounded-lg border px-3 text-sm font-semibold disabled:cursor-not-allowed disabled:opacity-60"
            style={{
              background: 'rgba(96,165,250,0.1)',
              borderColor: 'rgba(96,165,250,0.32)',
              color: 'var(--kvm-accent-text)',
            }}
          >
            <PlusIcon size={15} />
            添加新网卡
          </button>
        </div>
      </CardSection>
      <CardSection title="存储设备">
        <div className="space-y-3">
          {disks.map((disk, index) => (
            <ExistingDiskRow
              key={disk.name || disk.path}
              disk={disk}
              value={diskCapacities[disk.name] || bytesToGBInput(disk.bytes)}
              deleting={Boolean(deletedDisks[disk.name])}
              disabled={loading || saving || Boolean(deletedDisks[disk.name])}
              onChange={value => setDiskCapacities(current => ({ ...current, [disk.name]: value }))}
              onToggleDelete={() =>
                setDeletedDisks(current => ({ ...current, [disk.name]: !current[disk.name] }))
              }
              deleteDisabled={
                index === 0 ||
                running ||
                saving ||
                (activeDisks.length <= 1 && !deletedDisks[disk.name])
              }
            />
          ))}
          {newDisks.map(disk => (
            <NewDiskCard
              key={disk.id}
              disk={disk}
              disabled={loading || saving}
              storagePools={storagePools}
              onChange={next =>
                setNewDisks(current =>
                  current.map(item => (item.id === disk.id ? { ...item, ...next } : item))
                )
              }
              onRemove={() => setNewDisks(current => current.filter(item => item.id !== disk.id))}
            />
          ))}
          <button
            type="button"
            disabled={loading || saving || storagePools.length === 0}
            onClick={addDisk}
            className="kvm-action-button inline-flex h-9 items-center gap-2 rounded-lg border px-3 text-sm font-semibold disabled:cursor-not-allowed disabled:opacity-60"
            style={{
              background: 'rgba(45,212,191,0.1)',
              borderColor: 'rgba(45,212,191,0.32)',
              color: 'var(--kvm-check-toggle-active-text)',
            }}
          >
            <PlusIcon size={15} />
            添加新磁盘
          </button>
        </div>
      </CardSection>
      <div className="flex justify-end">
        <PrimaryButton
          label={saving ? '修改中' : '修改'}
          disabled={saving}
          onClick={() => void handleSave()}
        />
      </div>
    </section>
  );
}

function ExistingDiskRow({
  disk,
  value,
  disabled,
  deleting,
  onChange,
  onToggleDelete,
  deleteDisabled,
}: {
  disk: VMConfigDisk;
  value: string;
  disabled?: boolean;
  deleting?: boolean;
  onChange: (value: string) => void;
  onToggleDelete: () => void;
  deleteDisabled?: boolean;
}) {
  return (
    <DeviceRow
      name={disk.name || '磁盘'}
      detail={deleting ? '保存后删除此磁盘和对应存储卷' : diskDetail(disk)}
      danger={deleting}
    >
      <div className="grid gap-2 sm:grid-cols-[minmax(0,1fr)_36px]">
        <div className="relative">
          <input
            value={value}
            type="number"
            min={Math.ceil(disk.bytes / bytesPerGB)}
            step="1"
            disabled={disabled}
            onChange={event => onChange(event.target.value)}
            className={inputClass + ' pr-10 disabled:opacity-60'}
            style={fieldStyle}
            aria-label={`${disk.name} 扩容容量`}
          />
          <span
            className="pointer-events-none absolute right-3 top-1/2 -translate-y-1/2 text-xs"
            style={{ color: 'var(--kvm-text-muted)' }}
          >
            GB
          </span>
        </div>
        <IconAction
          label={deleting ? '取消删除磁盘' : '删除磁盘'}
          disabled={deleteDisabled}
          danger={!deleting}
          active={deleting}
          onClick={onToggleDelete}
        />
      </div>
    </DeviceRow>
  );
}

function DeviceRow({
  name,
  detail,
  active,
  danger,
  children,
}: {
  name: string;
  detail: string;
  active?: boolean;
  danger?: boolean;
  children: React.ReactNode;
}) {
  return (
    <div
      className="grid gap-3 rounded-lg px-3 py-2.5 md:grid-cols-[140px_minmax(0,1fr)_minmax(180px,280px)] md:items-center"
      style={{
        background: active
          ? 'rgba(59,130,246,0.08)'
          : danger
            ? 'rgba(239,68,68,0.08)'
            : 'var(--kvm-control-bg-soft)',
        border: `1px solid ${active ? 'rgba(96,165,250,0.34)' : danger ? 'rgba(248,113,113,0.34)' : 'var(--kvm-border)'}`,
      }}
    >
      <div className="text-right text-sm" style={{ color: 'var(--kvm-text)' }}>
        {name}
      </div>
      <div className="min-w-0">
        <div className="break-all font-mono text-sm" style={{ color: 'var(--kvm-text-muted)' }}>
          {detail}
        </div>
      </div>
      {children}
    </div>
  );
}

function IconAction({
  label,
  disabled,
  danger,
  active,
  onClick,
}: {
  label: string;
  disabled?: boolean;
  danger?: boolean;
  active?: boolean;
  onClick: () => void;
}) {
  return (
    <KvmTooltip label={label} placement="top">
      <button
        type="button"
        disabled={disabled}
        onClick={onClick}
        className="kvm-action-button inline-flex h-10 w-9 items-center justify-center rounded-lg border disabled:cursor-not-allowed disabled:opacity-50"
        style={{
          background: active ? 'rgba(239,68,68,0.16)' : 'var(--kvm-control-bg)',
          borderColor: active || danger ? 'rgba(248,113,113,0.42)' : 'var(--kvm-border)',
          color: active || danger ? '#fca5a5' : 'var(--kvm-text-muted)',
        }}
        aria-label={label}
      >
        <Trash2Icon size={15} />
      </button>
    </KvmTooltip>
  );
}

function EmptyText({ children }: { children: React.ReactNode }) {
  return (
    <div
      className="rounded-lg border px-3 py-2 text-xs"
      style={{ borderColor: 'var(--kvm-border)', color: 'var(--kvm-text-muted)' }}
    >
      {children}
    </div>
  );
}

function diskDetail(disk: VMConfigDisk) {
  const displayPath = disk.sourcePath || disk.path;
  const volumeName = displayPath ? displayPath.split(/[\\/]/).pop() || displayPath : '-';
  return [`${volumeName} (${formatBytes(disk.bytes, 'GB')})`, disk.pool, disk.bus]
    .filter(Boolean)
    .join(' · ');
}

function networkDeviceDetail(item: VMConfig['interfaces'][number], pool: NetworkPool | undefined) {
  const selected = pool ? networkPoolDeviceDetail(pool) : { source: item.source, type: item.type };
  return [item.mac || '-', selected.source || '', selected.type || '-', item.model || '-']
    .filter(Boolean)
    .join(' · ');
}

function networkPoolDeviceDetail(pool: NetworkPool) {
  const bridge = pool.forward === 'bridge' && pool.bridge;
  return { source: bridge || pool.name, type: bridge ? 'bridge' : 'network' };
}

function networkPoolValueForInterface(iface: VMConfig['interfaces'][number], pools: NetworkPool[]) {
  const source = iface.source.trim();
  if (!source) return '';
  return (
    pools.find(pool => pool.bridge === source)?.name ||
    pools.find(pool => pool.name === source)?.name ||
    source
  );
}

function interfaceKey(item: VMConfig['interfaces'][number]) {
  return item.name || item.mac || `${item.type}:${item.source}:${item.model}`;
}

function networkPoolTooltip(pool: NetworkPool) {
  return [pool.bridge || '-', pool.forward || 'isolated'].join(' · ');
}

function gbInputToBytes(value: string | undefined) {
  const parsed = Number(value);
  if (!Number.isFinite(parsed) || parsed <= 0) return 0;
  return Math.round(parsed * bytesPerGB);
}

function bytesToGBInput(bytes: number) {
  if (!Number.isFinite(bytes) || bytes <= 0) return '0';
  return String(Math.ceil(bytes / bytesPerGB));
}

function nextDiskTarget(disks: VMConfigDisk[], drafts: NewDiskDraft[]) {
  const used = new Set([...disks.map(disk => disk.name), ...drafts.map(disk => disk.target)]);
  for (let code = 'b'.charCodeAt(0); code <= 'z'.charCodeAt(0); code += 1) {
    const target = `vd${String.fromCharCode(code)}`;
    if (!used.has(target)) return target;
  }
  return `vd${used.size + 1}`;
}

function hasDuplicate(values: string[], existing: string[] = []) {
  const seen = new Set(existing.map(item => item.trim()).filter(Boolean));
  for (const value of values) {
    const item = value.trim();
    if (!item) continue;
    if (seen.has(item)) return true;
    seen.add(item);
  }
  return false;
}

function preferredStoragePool(disks: VMConfigDisk[], storagePools: StoragePool[]) {
  const knownPools = new Set(storagePools.map(pool => pool.name));
  const existingPool = disks.map(disk => disk.pool).find(pool => pool && knownPools.has(pool));
  return existingPool || storagePools[0]?.name || '';
}

function newDiskExtensionError(disks: Array<{ name: string; format: string }>) {
  for (const disk of disks) {
    const extension = volumeExtensionForFormat(disk.format);
    if (!disk.name.toLowerCase().endsWith(extension)) {
      return `${disk.name} 扩展名必须与 ${disk.format} 格式匹配`;
    }
  }
  return '';
}
