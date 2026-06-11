import { useEffect, useMemo, useState } from 'react';

import { CpuIcon, HardDriveIcon, MemoryStickIcon, NetworkIcon, ShuffleIcon } from 'lucide-react';
import { toast } from 'sonner';

import { KvmTooltip } from '../../../../components/kvm/StatusBadge';
import { SelectMenu } from '../../../../components/kvm/SelectMenu';
import {
  fetchNetworkPools,
  fetchStoragePools,
  type NetworkPool,
  type StoragePool,
  type VMConfig,
  type VMClonePayload,
  type VirtualMachine,
} from '../../../../lib/api';
import { formatBytes, formatOSType } from '../../../../lib/format';
import { runVMClone } from '../../utils/runVMClone';
import { runVMTemplateCreate } from '../../utils/runVMTemplateCreate';
import { isVMRunning } from '../../utils/vmStatus';
import { PrimaryButton } from '../VMEditControls';
import { fieldStyle, inputClass, InlineNotice } from './EditShared';

type CDROMPolicy = 'inherit' | 'disconnect';

export function ClonePanel({
  vm,
  config,
  mode = 'clone',
  onCloned,
}: {
  vm: VirtualMachine;
  config: VMConfig | null;
  mode?: 'clone' | 'template';
  onCloned?: () => void;
}) {
  const disks = useMemo(
    () =>
      config?.disks.length
        ? config.disks
        : vm.disks.length > 0
          ? vm.disks.map(disk => ({ ...disk, pool: '', bus: '', device: 'disk', type: 'file' }))
          : [
              {
                name: 'disk',
                path: '',
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
  const cdroms = useMemo(() => config?.cdroms || [], [config?.cdroms]);
  const sourceCPU = config?.currentCpu || vm.cpuCores || 1;
  const sourceMaxCPU = config?.maximumCpu || sourceCPU;
  const sourceMemoryMB = Math.max(
    1,
    Math.round((config?.currentMemoryBytes || vm.memoryBytes) / 1024 / 1024)
  );
  const sourceMaxMemoryMB = Math.max(
    sourceMemoryMB,
    Math.round((config?.maximumMemoryBytes || vm.memoryBytes) / 1024 / 1024)
  );
  const hostCPU = config?.hostCpu || 0;
  const hostMemoryMB = config?.hostMemoryBytes
    ? Math.floor(config.hostMemoryBytes / 1024 / 1024)
    : 0;
  const [cloneName, setCloneName] = useState('');
  const [description, setDescription] = useState('');
  const [autostart, setAutostart] = useState(false);
  const [currentCPU, setCurrentCPU] = useState(sourceCPU);
  const [maximumCPU, setMaximumCPU] = useState(sourceMaxCPU);
  const [currentMemoryMB, setCurrentMemoryMB] = useState(sourceMemoryMB);
  const [maximumMemoryMB, setMaximumMemoryMB] = useState(sourceMaxMemoryMB);
  const [cdromPolicy, setCDROMPolicy] = useState<CDROMPolicy>('inherit');
  const [macs, setMacs] = useState<Record<string, string>>(() => randomMacsByInterface(interfaces));
  const [interfacePools, setInterfacePools] = useState<Record<string, string>>(() =>
    Object.fromEntries(interfaces.map(item => [item.name, item.source]))
  );
  const [metadata, setMetadata] = useState<Record<string, boolean>>({});
  const [diskNames, setDiskNames] = useState<Record<string, string>>({});
  const [diskPools, setDiskPools] = useState<Record<string, string>>(() =>
    Object.fromEntries(disks.map(disk => [disk.name, disk.pool]))
  );
  const [networkPools, setNetworkPools] = useState<NetworkPool[]>([]);
  const [storagePools, setStoragePools] = useState<StoragePool[]>([]);
  const [busy, setBusy] = useState(false);
  const running = isVMRunning(vm.status);
  const isTemplateMode = mode === 'template';
  const totalDiskBytes =
    disks.reduce((sum, disk) => sum + Math.max(0, disk.bytes || 0), 0) || vm.diskBytes;
  const connectedCDROMs = cdroms.filter(cdrom => cdrom.connected).length;

  useEffect(() => {
    setCloneName('');
    setDescription('');
    setAutostart(false);
    setCurrentCPU(sourceCPU);
    setMaximumCPU(sourceMaxCPU);
    setCurrentMemoryMB(sourceMemoryMB);
    setMaximumMemoryMB(sourceMaxMemoryMB);
    setCDROMPolicy('inherit');
    setDiskNames({});
    setDiskPools(Object.fromEntries(disks.map(disk => [disk.name, disk.pool])));
    setMetadata({});
  }, [disks, sourceCPU, sourceMaxCPU, sourceMaxMemoryMB, sourceMemoryMB, vm.id, vm.name]);

  useEffect(() => {
    setInterfacePools(
      Object.fromEntries(
        interfaces.map(item => [item.name, networkPoolValueForInterface(item, networkPools)])
      )
    );
  }, [interfaces, networkPools]);

  useEffect(() => {
    const name = cloneName.trim();
    if (!name) return;
    setDiskNames(Object.fromEntries(disks.map(disk => [disk.name, diskVolumeName(name, disk)])));
  }, [cloneName, disks]);

  useEffect(() => {
    if (autostart) setCDROMPolicy('disconnect');
  }, [autostart]);

  useEffect(() => {
    setMacs(current => {
      const next = randomMacsByInterface(interfaces, current);
      return sameStringRecord(current, next) ? current : next;
    });
  }, [interfaces, vm.id]);

  useEffect(() => {
    if (!vm.hostId) return;
    let ignore = false;
    fetchNetworkPools(vm.hostId)
      .then(networkBody => {
        if (ignore) return;
        setNetworkPools(networkBody.items);
        setInterfacePools(
          Object.fromEntries(
            interfaces.map(item => [
              item.name,
              networkPoolValueForInterface(item, networkBody.items),
            ])
          )
        );
      })
      .catch(error => toast.error(error instanceof Error ? error.message : '读取网络池失败'));
    fetchStoragePools(vm.hostId)
      .then(storageBody => {
        if (!ignore) setStoragePools(storageBody.items);
      })
      .catch(error => toast.error(error instanceof Error ? error.message : '读取存储池失败'));
    return () => {
      ignore = true;
    };
  }, [interfaces, vm.hostId]);

  async function submitClone() {
    if (running) {
      return toast.warning(
        isTemplateMode
          ? '模板虚拟机正在运行，无法创建，请先关闭模板虚拟机后再操作'
          : '虚拟机正在运行，无法克隆，请先关闭虚拟机后再操作'
      );
    }
    const name = cloneName.trim();
    if (!name) return toast.warning(isTemplateMode ? '请输入虚拟机名称' : '请输入克隆名称');
    if (!config) return toast.warning('虚拟机配置尚未加载完成');
    if (currentCPU <= 0 || maximumCPU <= 0 || currentCPU > maximumCPU)
      return toast.warning('CPU 配置不正确');
    if (currentMemoryMB <= 0 || maximumMemoryMB <= 0 || currentMemoryMB > maximumMemoryMB)
      return toast.warning('内存配置不正确');
    if (hostCPU > 0 && maximumCPU > hostCPU)
      return toast.warning('最大 CPU 不能超过宿主机逻辑 CPU');
    if (hostMemoryMB > 0 && maximumMemoryMB > hostMemoryMB)
      return toast.warning('最大内存不能超过宿主机总内存');
    const payload: VMClonePayload = {
      name,
      description: description.trim(),
      autostart,
      currentCpu: currentCPU,
      maximumCpu: maximumCPU,
      currentMemoryMB,
      maximumMemoryMB,
      cdromPolicy: autostart ? 'disconnect' : cdromPolicy,
      interfaces: interfaces.map(item => ({
        name: item.name,
        mac: (macs[item.name] || '').trim(),
        source: networkPoolSourceForInterface(
          item,
          interfacePools[item.name] || item.source || '',
          networkPools
        ),
      })),
      disks: disks.map(disk => ({
        name: disk.name,
        pool: (diskPools[disk.name] || disk.pool || '').trim(),
        sourcePath: disk.path,
        targetName: (diskNames[disk.name] || '').trim(),
        preallocMetadata: isQCOW2Disk(disk) && Boolean(metadata[disk.name]),
      })),
    };
    if (payload.interfaces.some(item => !item.mac))
      return toast.warning('请输入所有网卡的新 MAC 地址');
    if (payload.interfaces.some(item => !item.source))
      return toast.warning('请选择所有网卡的网络池');
    if (payload.disks.some(disk => !disk.pool))
      return toast.warning('磁盘所属存储池未识别，无法克隆');
    if (payload.disks.some(disk => !disk.targetName))
      return toast.warning('请输入克隆后的存储卷名称');
    const extensionError = cloneDiskExtensionError(disks, payload.disks);
    if (extensionError) return toast.warning(extensionError);
    setBusy(true);
    try {
      if (isTemplateMode) {
        await runVMTemplateCreate({
          templateId: vm.id,
          vmName: name,
          payload,
          onQueued: onCloned,
        });
      } else {
        await runVMClone({
          vmId: vm.id,
          cloneName: name,
          payload,
          onQueued: onCloned,
        });
      }
    } catch (error) {
      toast.error(
        error instanceof Error
          ? error.message
          : isTemplateMode
            ? '从模板创建虚拟机失败'
            : '虚拟机克隆失败'
      );
    } finally {
      setBusy(false);
    }
  }

  return (
    <section className="grid h-full min-h-0 w-full gap-4 xl:grid-cols-[260px_minmax(0,1fr)]">
      <aside className="min-h-0 space-y-3 overflow-hidden">
        <Panel title={isTemplateMode ? '模板虚拟机' : '源虚拟机'}>
          <div className="space-y-2">
            <InfoLine label="名称" value={vm.name} strong />
            <InfoLine label="系统" value={formatOSType(vm.osType, vm.name)} />
            <InfoLine label="宿主机" value={vm.hostName || '-'} />
            <InfoLine label="IP" value={vm.primaryIp || '-'} />
          </div>
        </Panel>
        <div className="grid grid-cols-2 gap-2">
          <Metric icon={CpuIcon} label="CPU" value={`${sourceCPU}/${sourceMaxCPU} vCPU`} />
          <Metric
            icon={MemoryStickIcon}
            label="内存"
            value={`${formatMemoryMB(sourceMemoryMB)} / ${formatMemoryMB(sourceMaxMemoryMB)}`}
          />
          <Metric
            icon={HardDriveIcon}
            label="磁盘"
            value={`${disks.length} 块 · ${formatBytes(totalDiskBytes, 'GB')}`}
          />
          <Metric icon={NetworkIcon} label="网卡" value={`${interfaces.length} 张`} />
        </div>
        <InlineNotice tone={running ? 'warning' : 'info'}>
          {running
            ? isTemplateMode
              ? '模板虚拟机正在运行，需关机后才能创建'
              : '虚拟机正在运行，需关机后才能克隆'
            : isTemplateMode
              ? '创建会复制模板磁盘卷，并继承模板虚拟机 XML 基本配置'
              : '克隆会复制磁盘卷，并继承源虚拟机 XML 基本配置'}
        </InlineNotice>
      </aside>

      <div className="h-full min-h-0 overflow-hidden">
        <div className="flex h-full min-h-0 flex-col overflow-hidden">
          <div className="kvm-hidden-scrollbar min-h-0 flex-1 space-y-3 overflow-y-auto pr-1">
            <Panel title="基础配置">
              <div className="grid gap-3 lg:grid-cols-2">
                <Field label={isTemplateMode ? '虚拟机名称' : '克隆名称'}>
                  <input
                    value={cloneName}
                    disabled={busy}
                    onChange={event => setCloneName(event.target.value)}
                    placeholder={isTemplateMode ? 'new-vm-name' : vm.name}
                    className={inputClass}
                    style={fieldStyle}
                  />
                </Field>
                <Field label="克隆后直接启动">
                  <Toggle
                    checked={autostart}
                    disabled={busy}
                    label={autostart ? '已启用' : '不启用'}
                    onChange={setAutostart}
                  />
                </Field>
                <Field label="描述" wide>
                  <input
                    value={description}
                    disabled={busy}
                    onChange={event => setDescription(event.target.value)}
                    placeholder="可选"
                    className={inputClass}
                    style={fieldStyle}
                  />
                </Field>
              </div>
            </Panel>

            <Panel title="资源配置">
              <div className="grid gap-3 lg:grid-cols-4">
                <NumberField
                  label="当前 CPU"
                  value={currentCPU}
                  disabled={busy}
                  onChange={setCurrentCPU}
                />
                <NumberField
                  label="最大 CPU"
                  value={maximumCPU}
                  disabled={busy}
                  onChange={setMaximumCPU}
                />
                <NumberField
                  label="当前内存 MB"
                  value={currentMemoryMB}
                  disabled={busy}
                  onChange={setCurrentMemoryMB}
                />
                <NumberField
                  label="最大内存 MB"
                  value={maximumMemoryMB}
                  disabled={busy}
                  onChange={setMaximumMemoryMB}
                />
              </div>
              {(hostCPU > 0 || hostMemoryMB > 0) && (
                <div className="mt-2 text-xs" style={{ color: 'var(--kvm-text-muted)' }}>
                  宿主机上限：{hostCPU || '-'} vCPU / {formatHostMemoryLimit(hostMemoryMB)}
                </div>
              )}
            </Panel>

            <Panel title="网络配置">
              <div className="space-y-2">
                {interfaces.length ? (
                  interfaces.map(item => (
                    <div
                      key={item.name || item.mac}
                      className="rounded-lg border p-3"
                      style={{
                        borderColor: 'var(--kvm-border)',
                        background: 'var(--kvm-control-bg-soft)',
                      }}
                    >
                      <div className="mx-auto grid w-full max-w-[820px] gap-3 md:grid-cols-[70px_130px_minmax(0,1fr)_150px] md:items-center">
                        <DeviceSummary
                          className="text-center md:justify-self-center"
                          title={interfaceName(item, vm)}
                          tooltip={interfaceTooltip(item)}
                        />
                        <SelectMenu
                          value={interfacePools[item.name] || item.source || ''}
                          disabled={busy}
                          placeholder="选择网络池"
                          maxVisibleItems={4}
                          options={networkPoolOptionsForInterface(item, networkPools)}
                          onChange={value =>
                            setInterfacePools(current => ({ ...current, [item.name]: value }))
                          }
                        />
                        <input
                          value={macs[item.name] || ''}
                          disabled={busy}
                          onChange={event =>
                            setMacs(current => ({ ...current, [item.name]: event.target.value }))
                          }
                          className={inputClass + ' min-w-0'}
                          style={fieldStyle}
                        />
                        <button
                          type="button"
                          disabled={busy}
                          onClick={() =>
                            setMacs(current => ({ ...current, [item.name]: randomMacAddress() }))
                          }
                          className="kvm-action-button flex h-9 min-w-0 w-full items-center justify-center gap-2 whitespace-nowrap rounded-lg border px-2.5 text-xs font-semibold disabled:opacity-60"
                          style={{
                            background: 'rgba(59,130,246,0.12)',
                            borderColor: 'rgba(59,130,246,0.38)',
                            color: '#93c5fd',
                          }}
                        >
                          <ShuffleIcon size={15} />
                          随机 MAC 地址
                        </button>
                      </div>
                    </div>
                  ))
                ) : (
                  <EmptyText>未识别到可克隆的网络设备</EmptyText>
                )}
              </div>
            </Panel>

            <Panel title="存储配置">
              <div className="space-y-2">
                {disks.map(disk => (
                  <div
                    key={disk.name}
                    className="rounded-lg border p-3"
                    style={{
                      borderColor: 'var(--kvm-border)',
                      background: 'var(--kvm-control-bg-soft)',
                    }}
                  >
                    <div className="mx-auto grid w-full max-w-[820px] gap-3 md:grid-cols-[70px_130px_minmax(0,1fr)_105px] md:items-center">
                      <DeviceSummary
                        className="text-center md:justify-self-center"
                        title={diskName(disk)}
                        tooltip={diskTooltip(disk)}
                      />
                      <SelectMenu
                        value={diskPools[disk.name] || disk.pool || ''}
                        disabled={busy || storagePools.length === 0}
                        placeholder="选择存储池"
                        placement="top"
                        maxVisibleItems={4}
                        options={storagePools.map(pool => ({
                          value: pool.name,
                          label: pool.name,
                          tooltip: pool.path || '-',
                        }))}
                        onChange={value =>
                          setDiskPools(current => ({ ...current, [disk.name]: value }))
                        }
                      />
                      <input
                        value={diskNames[disk.name] || ''}
                        disabled={busy}
                        onChange={event =>
                          setDiskNames(current => ({ ...current, [disk.name]: event.target.value }))
                        }
                        placeholder={diskPlaceholderName(vm.name, disk)}
                        className={inputClass + ' min-w-0'}
                        style={fieldStyle}
                      />
                      {isQCOW2Disk(disk) ? (
                        <div
                          className="flex h-9 w-full items-center justify-between gap-2 rounded-lg border px-3"
                          style={{
                            borderColor: 'var(--kvm-border)',
                            background: 'var(--kvm-control-bg)',
                          }}
                        >
                          <KvmTooltip label="预分配 qcow2 元数据" placement="top" align="center">
                            <span
                              className="text-xs font-semibold"
                              style={{ color: 'var(--kvm-text)' }}
                              aria-label="Metadata 预分配 qcow2 元数据，写入更稳定"
                            >
                              Metadata
                            </span>
                          </KvmTooltip>
                          <MetadataCheckbox
                            checked={Boolean(metadata[disk.name])}
                            disabled={busy}
                            onChange={checked =>
                              setMetadata(current => ({ ...current, [disk.name]: checked }))
                            }
                          />
                        </div>
                      ) : (
                        <span />
                      )}
                    </div>
                  </div>
                ))}
              </div>
            </Panel>

            <Panel title="介质策略">
              <div className="grid gap-3 lg:grid-cols-[220px_1fr] lg:items-center">
                <SelectMenu
                  value={cdromPolicy}
                  disabled={busy}
                  placeholder="选择介质策略"
                  placement="top"
                  options={[
                    { value: 'inherit', label: '继承当前介质', disabled: autostart },
                    { value: 'disconnect', label: '克隆后断开介质' },
                  ]}
                  onChange={value =>
                    setCDROMPolicy((autostart ? 'disconnect' : value) as CDROMPolicy)
                  }
                />
                <div className="text-xs" style={{ color: 'var(--kvm-text-muted)' }}>
                  {autostart
                    ? '克隆后直接启动时会自动断开介质'
                    : `已连接介质 ${connectedCDROMs} 个，策略会应用到克隆后的 XML`}
                </div>
              </div>
            </Panel>
          </div>

          <div
            className="mt-3 flex shrink-0 justify-end border-t pt-3"
            style={{ borderColor: 'var(--kvm-border)' }}
          >
            <PrimaryButton
              label={
                busy ? (isTemplateMode ? '创建中' : '克隆中') : isTemplateMode ? '创建' : '克隆'
              }
              disabled={busy}
              onClick={() => void submitClone()}
            />
          </div>
        </div>
      </div>
    </section>
  );
}

function Panel({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <section
      className="rounded-xl border p-4"
      style={{ background: 'var(--kvm-control-bg-soft)', borderColor: 'var(--kvm-border)' }}
    >
      <h3 className="mb-3 text-sm font-semibold" style={{ color: 'var(--kvm-text)' }}>
        {title}
      </h3>
      {children}
    </section>
  );
}

function Field({
  label,
  wide,
  children,
}: {
  label: string;
  wide?: boolean;
  children: React.ReactNode;
}) {
  return (
    <label className={wide ? 'lg:col-span-2' : ''}>
      <div className="mb-1.5 text-xs font-semibold" style={{ color: 'var(--kvm-text-muted)' }}>
        {label}
      </div>
      {children}
    </label>
  );
}

function NumberField({
  label,
  value,
  disabled,
  onChange,
}: {
  label: string;
  value: number;
  disabled?: boolean;
  onChange: (value: number) => void;
}) {
  return (
    <Field label={label}>
      <input
        value={value}
        type="number"
        min={1}
        disabled={disabled}
        onChange={event => onChange(Number(event.target.value))}
        className={inputClass}
        style={fieldStyle}
      />
    </Field>
  );
}

function Toggle({
  checked,
  disabled,
  label,
  onChange,
}: {
  checked: boolean;
  disabled?: boolean;
  label: string;
  onChange: (value: boolean) => void;
}) {
  return (
    <button
      type="button"
      disabled={disabled}
      onClick={() => onChange(!checked)}
      className="kvm-action-button h-9 rounded-lg border px-3 text-sm font-semibold disabled:opacity-60"
      style={{
        background: checked ? 'rgba(45,212,191,0.12)' : 'var(--kvm-control-bg)',
        borderColor: checked ? 'rgba(45,212,191,0.34)' : 'var(--kvm-border)',
        color: checked ? 'var(--kvm-check-toggle-active-text)' : 'var(--kvm-text-muted)',
      }}
    >
      {label}
    </button>
  );
}

function Metric({
  icon: Icon,
  label,
  value,
}: {
  icon: typeof CpuIcon;
  label: string;
  value: string;
}) {
  return (
    <div
      className="rounded-xl border p-3"
      style={{ background: 'var(--kvm-control-bg-soft)', borderColor: 'var(--kvm-border)' }}
    >
      <div
        className="flex items-center gap-1.5 text-[11px]"
        style={{ color: 'var(--kvm-text-muted)' }}
      >
        <Icon size={13} />
        {label}
      </div>
      <div className="mt-1 truncate text-xs font-semibold" style={{ color: 'var(--kvm-text)' }}>
        {value}
      </div>
    </div>
  );
}

function InfoLine({ label, value, strong }: { label: string; value: string; strong?: boolean }) {
  return (
    <div className="flex items-center justify-between gap-3 text-xs">
      <span style={{ color: 'var(--kvm-text-muted)' }}>{label}</span>
      <span
        className="truncate font-mono"
        style={{ color: strong ? 'var(--kvm-text)' : 'var(--kvm-text-muted)' }}
      >
        {value}
      </span>
    </div>
  );
}

function DeviceSummary({
  title,
  tooltip,
  className = '',
}: {
  title: string;
  tooltip?: string;
  className?: string;
}) {
  const content = (
    <div className="truncate text-sm font-semibold" style={{ color: 'var(--kvm-text)' }}>
      {title}
    </div>
  );
  return (
    <div className={'min-w-0 ' + className}>
      {tooltip ? (
        <KvmTooltip label={tooltip} placement="top" align="center">
          {content}
        </KvmTooltip>
      ) : (
        content
      )}
    </div>
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

function MetadataCheckbox({
  checked,
  disabled,
  onChange,
}: {
  checked: boolean;
  disabled?: boolean;
  onChange: (value: boolean) => void;
}) {
  return (
    <label
      className={
        'inline-flex h-5 w-5 shrink-0 items-center justify-center rounded-md ' +
        (disabled ? 'cursor-not-allowed opacity-60' : 'cursor-pointer')
      }
    >
      <input
        type="checkbox"
        checked={checked}
        disabled={disabled}
        onChange={event => onChange(event.target.checked)}
        className="h-4 w-4 cursor-pointer accent-cyan-400 disabled:cursor-not-allowed"
        aria-label="Metadata"
      />
    </label>
  );
}

function interfaceName(iface: VMConfig['interfaces'][number] | undefined, vm: VirtualMachine) {
  if (!iface) return vm.primaryIp ? '主网络' : '网络';
  return iface.name || '网络';
}

function interfaceTooltip(iface: VMConfig['interfaces'][number] | undefined) {
  if (!iface) return '';
  return [iface.source, iface.type, iface.model, iface.mac].filter(Boolean).join(' · ');
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

function networkPoolOptionsForInterface(
  iface: VMConfig['interfaces'][number],
  pools: NetworkPool[]
) {
  const options = pools.map(pool => ({
    value: pool.name,
    label: pool.name,
    tooltip: pool.bridge || '-',
  }));
  const current = networkPoolValueForInterface(iface, pools);
  if (current && !options.some(option => option.value === current)) {
    return [{ value: current, label: current, tooltip: iface.source || '-' }, ...options];
  }
  return options;
}

function networkPoolSourceForInterface(
  iface: VMConfig['interfaces'][number],
  value: string,
  pools: NetworkPool[]
) {
  const source = value.trim();
  if (!source) return '';
  if (iface.type === 'bridge') {
    return pools.find(pool => pool.name === source)?.bridge || source;
  }
  return source;
}

function diskName(disk: VMConfig['disks'][number]) {
  return disk.name || 'disk';
}

function diskTooltip(disk: VMConfig['disks'][number]) {
  return [disk.pool, disk.path, disk.bus].filter(Boolean).join(' · ');
}

function isQCOW2Disk(disk: VMConfig['disks'][number]) {
  return volumeNameFromPath(disk.path).toLowerCase().endsWith('.qcow2');
}

function diskPlaceholderName(vmName: string, disk: VMConfig['disks'][number]) {
  return diskVolumeName(vmName, disk);
}

function diskVolumeName(vmName: string, disk: VMConfig['disks'][number]) {
  const diskName = disk.name || 'disk';
  const extension = volumeExtension(disk.path) || volumeExtension(diskName) || '.qcow2';
  return `${vmName}-${diskName}${extension}`;
}

function volumeExtension(path: string) {
  const name = volumeNameFromPath(path);
  const dotIndex = name.lastIndexOf('.');
  return dotIndex > 0 ? name.slice(dotIndex) : '';
}

function cloneDiskExtensionError(disks: VMConfig['disks'], targets: VMClonePayload['disks']) {
  for (const target of targets) {
    const sourceDisk = disks.find(disk => disk.name === target.name);
    const sourceExtension = volumeExtension(sourceDisk?.path || target.sourcePath).toLowerCase();
    const targetExtension = volumeExtension(target.targetName).toLowerCase();
    if (sourceExtension && sourceExtension !== targetExtension) {
      return `${target.name} 的目标卷扩展名必须和源磁盘 ${sourceExtension} 一致`;
    }
  }
  return '';
}

function volumeNameFromPath(path: string) {
  const normalized = path.trim().replace(/\\/g, '/');
  return normalized.split('/').filter(Boolean).pop() || '';
}

function randomMacsByInterface(
  interfaces: VMConfig['interfaces'],
  current: Record<string, string> = {}
) {
  return Object.fromEntries(
    interfaces.map(item => [item.name, current[item.name] || randomMacAddress()])
  );
}

function sameStringRecord(left: Record<string, string>, right: Record<string, string>) {
  const leftKeys = Object.keys(left);
  const rightKeys = Object.keys(right);
  return leftKeys.length === rightKeys.length && leftKeys.every(key => left[key] === right[key]);
}

function randomMacAddress() {
  const parts = Array.from({ length: 3 }, () =>
    Math.floor(Math.random() * 256)
      .toString(16)
      .padStart(2, '0')
  );
  return `52:54:00:${parts.join(':')}`;
}

function formatMemoryMB(value: number) {
  if (value >= 1024) return `${(value / 1024).toFixed(value % 1024 === 0 ? 0 : 1)} GB`;
  return `${value} MB`;
}

function formatHostMemoryLimit(valueMB: number) {
  if (valueMB <= 0) return '-';
  const valueGB = valueMB / 1024;
  return `${valueGB.toFixed(1)} GB`;
}
