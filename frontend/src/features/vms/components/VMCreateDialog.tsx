import { useEffect, useMemo, useRef, useState } from 'react';
import { FolderPlusIcon, XIcon } from 'lucide-react';
import { toast } from 'sonner';

import {
  fetchISOFiles,
  fetchNetworkPools,
  fetchStoragePools,
  fetchVMs,
  type Host,
  type ISOFile,
  type NetworkPool,
  type StoragePool,
  type VirtualMachine,
  type VMCreatePayload,
} from '../../../lib/api';
import { DialogPortal } from '../../../components/kvm/DialogPortal';
import {
  diskTargetForBus,
  extraDiskName,
  replaceDiskExtension,
  replaceDiskTargetAndExtension,
} from '../utils/createDisk';
import { buildBlankCreatePayload, buildXMLCreatePayload } from '../utils/createSubmit';
import { runVMCreate } from '../utils/runVMCreate';
import { DialogTabs, type VMDialogTab } from './DialogTabs';
import {
  BasicInfoPanel,
  BlankStoragePanel,
  ComputePanel,
  NetworkBootPanel,
  PrimaryButton,
  XMLCreatePanel,
} from './create/CreatePanels';
import { TemplateCreatePanel } from './create/TemplateCreatePanel';
import type { CreateDiskDraft } from './create/CreateExtraDiskCard';

const formats = ['qcow2', 'raw', 'qcow', 'qed'].map(value => ({ value, label: value }));
const buses = ['virtio', 'sata', 'scsi', 'ide'].map(value => ({ value, label: value }));
const isoBuses = ['sata', 'ide', 'scsi', 'usb'].map(value => ({ value, label: value }));
const cpuModels = [
  { value: 'host-passthrough', label: '主机直通', tooltip: 'host-passthrough' },
  { value: 'host-model', label: '主机模型', tooltip: 'host-model' },
];
const osTypeOptions = [
  { value: 'linux', label: 'Linux' },
  { value: 'windows', label: 'Windows' },
];
const networkModelOptions = [
  { value: 'virtio', label: 'virtio' },
  { value: 'e1000', label: 'e1000' },
  { value: 'e1000e', label: 'e1000e' },
  { value: 'rtl8139', label: 'rtl8139' },
  { value: 'vmxnet3', label: 'vmxnet3' },
];
const graphicsOptions = [{ value: 'vnc', label: 'VNC' }];
const firmwareOptions = [
  { value: 'bios', label: 'BIOS' },
  { value: 'uefi', label: 'UEFI' },
];

export function VMCreateDialog({ hosts, onClose }: { hosts: Host[]; onClose: () => void }) {
  const [activeTab, setActiveTab] = useState<VMDialogTab>('form');
  const [agentId, setAgentId] = useState(hosts[0]?.id || '');
  const [storagePools, setStoragePools] = useState<StoragePool[]>([]);
  const [networkPools, setNetworkPools] = useState<NetworkPool[]>([]);
  const [isoFiles, setISOFiles] = useState<ISOFile[]>([]);
  const [templates, setTemplates] = useState<VirtualMachine[]>([]);
  const [templateId, setTemplateId] = useState('');
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [autostart, setAutostart] = useState(false);
  const [cpu, setCPU] = useState(2);
  const [maxCPU, setMaxCPU] = useState(2);
  const [memory, setMemory] = useState(4096);
  const [maxMemory, setMaxMemory] = useState(4096);
  const [cpuModel, setCPUModel] = useState('host-passthrough');
  const [osType, setOSType] = useState('linux');
  const [diskPool, setDiskPool] = useState('');
  const [isoPool, setISOPool] = useState('');
  const [diskFormat, setDiskFormat] = useState('qcow2');
  const [diskBus, setDiskBus] = useState('virtio');
  const [diskSize, setDiskSize] = useState(40);
  const [diskName, setDiskName] = useState('');
  const [diskNameTouched, setDiskNameTouched] = useState(false);
  const [preallocMetadata, setPreallocMetadata] = useState(false);
  const [extraDisks, setExtraDisks] = useState<CreateDiskDraft[]>([]);
  const [isoPath, setISOPath] = useState('');
  const [isoBus, setISOBus] = useState('sata');
  const [networkSource, setNetworkSource] = useState('');
  const [networkModel, setNetworkModel] = useState('virtio');
  const [graphics, setGraphics] = useState('vnc');
  const [consolePasswordEnabled, setConsolePasswordEnabled] = useState(false);
  const [consolePassword, setConsolePassword] = useState('');
  const [consolePasswordVisible, setConsolePasswordVisible] = useState(false);
  const [firmware, setFirmware] = useState('bios');
  const [xml, setXML] = useState('');
  const [busy, setBusy] = useState(false);

  const diskExtension = diskFormat === 'qcow2' ? '.qcow2' : '.img';
  const systemDiskTarget = diskTargetForBus(diskBus, 0);
  const autoDiskName = name.trim() ? `${name.trim()}-${systemDiskTarget}${diskExtension}` : '';
  const previousNameRef = useRef(name);
  const selectedHost = hosts.find(host => host.id === agentId);
  const hostMemoryMB = selectedHost?.memoryBytes
    ? Math.floor(selectedHost.memoryBytes / 1024 / 1024)
    : 0;

  useEffect(() => {
    if (!agentId) return;
    setStoragePools([]);
    setNetworkPools([]);
    setISOFiles([]);
    Promise.all([fetchStoragePools(agentId), fetchNetworkPools(agentId)])
      .then(([storage, network]) => {
        setStoragePools(storage.items);
        setNetworkPools(network.items);
        setDiskPool(storage.items[0]?.name || '');
        setISOPool(storage.items[0]?.name || '');
        setExtraDisks([]);
        setNetworkSource(network.items[0]?.name || '');
      })
      .catch(error => toast.error(error instanceof Error ? error.message : '读取宿主机资源池失败'));
  }, [agentId]);

  useEffect(() => {
    if (previousNameRef.current === name) {
      if (!diskNameTouched) setDiskName(autoDiskName);
      return;
    }
    previousNameRef.current = name;
    setDiskName(autoDiskName);
    setDiskNameTouched(false);
  }, [autoDiskName, diskNameTouched, name]);

  useEffect(() => {
    setExtraDisks(current =>
      current.map((disk, index) => ({
        ...disk,
        name: extraDiskName(diskName, diskBus, diskFormat, index + 1),
        pool: diskPool,
        format: diskFormat,
        bus: diskBus,
        preallocMetadata: diskFormat === 'qcow2' && preallocMetadata,
        nameTouched: false,
      }))
    );
  }, [diskName, diskPool, diskFormat, diskBus, preallocMetadata]);

  useEffect(() => {
    if (!agentId || !isoPool) return;
    fetchISOFiles(agentId, isoPool)
      .then(response => setISOFiles(response.items))
      .catch(() => setISOFiles([]));
  }, [agentId, isoPool]);

  useEffect(() => {
    fetchVMs()
      .then(response => {
        const items = response.items.filter(vm => vm.isTemplate);
        setTemplates(items);
        setTemplateId(current => current || items[0]?.id || '');
      })
      .catch(() => setTemplates([]));
  }, []);

  const hostOptions = useMemo(
    () =>
      hosts.map(host => ({
        value: host.id,
        label: host.address || host.hostname || host.name || host.id,
        tooltip: host.name || host.hostname || host.id,
      })),
    [hosts]
  );
  const storageOptions = storagePools.map(pool => ({
    value: pool.name,
    label: pool.name,
    tooltip: pool.path || '-',
  }));
  const networkOptions = networkPools.map(pool => ({
    value: pool.name,
    label: pool.name,
    tooltip: pool.bridge || pool.forward || '-',
  }));
  const isoOptions = [
    { value: '', label: '不挂载 ISO' },
    ...isoFiles.map(file => ({ value: file.path, label: file.name, tooltip: file.path })),
  ];
  async function submit() {
    if (activeTab === 'xml') {
      const result = buildXMLCreatePayload({
        agentId,
        xml,
        description,
        autostart,
        cpu,
        maxCPU,
        memory,
        maxMemory,
        cpuModel,
        osType,
      });
      await submitPayload(result);
      return;
    }
    if (activeTab === 'template') {
      return;
    }
    await submitPayload(
      buildBlankCreatePayload({
        agentId,
        name,
        description,
        autostart,
        cpu,
        maxCPU,
        memory,
        maxMemory,
        cpuModel,
        osType,
        diskName,
        diskPool,
        diskFormat,
        diskBus,
        diskSize,
        preallocMetadata,
        extraDisks,
        isoPath,
        isoBus,
        networkSource,
        networkModel,
        graphics,
        consolePasswordEnabled,
        consolePassword,
        firmware,
        selectedHost,
        hostMemoryMB,
      })
    );
  }

  async function submitPayload({
    payload,
    warning,
    vmName,
  }: {
    payload?: VMCreatePayload;
    warning?: string;
    vmName?: string;
  }) {
    if (warning || !payload || !vmName) {
      toast.warning(warning || '创建虚拟机参数不完整');
      return;
    }
    setBusy(true);
    try {
      await runVMCreate({ vmName, payload, onQueued: onClose });
    } finally {
      setBusy(false);
    }
  }

  function updateSystemBus(bus: string) {
    setDiskBus(bus);
    if (!diskNameTouched) return;
    const nextName = replaceDiskTargetAndExtension(diskName, diskBus, bus, diskFormat, 0);
    if (nextName !== diskName) setDiskName(nextName);
  }

  function updateSystemFormat(format: string) {
    setDiskFormat(format);
    setPreallocMetadata(false);
    if (diskNameTouched) setDiskName(replaceDiskExtension(diskName, format));
  }

  function addExtraDisk() {
    const index = extraDisks.length + 1;
    const bus = diskBus || 'virtio';
    const format = diskFormat || 'qcow2';
    const pool = diskPool || storagePools[0]?.name || '';
    setExtraDisks(current => [
      ...current,
      {
        id: `${Date.now()}-${index}`,
        name: extraDiskName(diskName, bus, format, index),
        pool,
        format,
        bus,
        capacityGB: '20',
        preallocMetadata: format === 'qcow2' && preallocMetadata,
        nameTouched: false,
      },
    ]);
  }

  function updateExtraDisk(id: string, patch: Partial<CreateDiskDraft>) {
    setExtraDisks(current =>
      current.map(disk => {
        if (disk.id !== id) return disk;
        return { ...disk, capacityGB: patch.capacityGB ?? disk.capacityGB };
      })
    );
  }

  return (
    <DialogPortal>
      <div className="kvm-dialog-backdrop fixed inset-0 z-50 flex items-center justify-center px-3 py-5">
        <div className="kvm-dialog-panel flex max-h-[88vh] w-[min(92vw,920px)] flex-col overflow-hidden rounded-2xl shadow-2xl">
          <header
            className="flex min-h-14 items-center justify-between border-b px-4 py-2.5"
            style={{ borderColor: 'var(--kvm-border)' }}
          >
            <div className="flex items-center gap-2">
              <FolderPlusIcon size={17} style={{ color: '#5eead4' }} />
              <h2 className="text-sm font-semibold" style={{ color: 'var(--kvm-text)' }}>
                创建虚拟机
              </h2>
            </div>
            <button
              type="button"
              onClick={onClose}
              className="kvm-action-button flex h-8 w-8 items-center justify-center rounded-lg border"
              style={{
                background: 'var(--kvm-control-bg)',
                borderColor: 'var(--kvm-border)',
                color: 'var(--kvm-text-muted)',
              }}
              aria-label="关闭创建窗口"
            >
              <XIcon size={15} />
            </button>
          </header>
          <DialogTabs active={activeTab} disabled={busy} onChange={setActiveTab} />
          <main
            className="kvm-hidden-scrollbar flex-1 overflow-y-auto p-4"
            style={{ background: 'var(--kvm-control-bg-soft)' }}
          >
            <div className="kvm-dialog-card space-y-4 rounded-2xl p-4">
              {activeTab === 'xml' ? (
                <>
                  <XMLCreatePanel
                    agentId={agentId}
                    hostOptions={hostOptions}
                    xml={xml}
                    busy={busy}
                    onAgentChange={setAgentId}
                    onXMLChange={setXML}
                  />
                  <div className="flex justify-end">
                    <PrimaryButton
                      label={busy ? '提交中' : '创建'}
                      disabled={busy}
                      onClick={() => void submit()}
                    />
                  </div>
                </>
              ) : activeTab === 'template' ? (
                <TemplateCreatePanel
                  templates={templates}
                  selectedId={templateId}
                  agentId={agentId}
                  hostOptions={hostOptions}
                  selectedHost={selectedHost}
                  hostMemoryMB={hostMemoryMB}
                  storageOptions={storageOptions}
                  networkOptions={networkOptions}
                  osTypeOptions={osTypeOptions}
                  cpuModels={cpuModels}
                  buses={buses}
                  isoBuses={isoBuses}
                  networkModelOptions={networkModelOptions}
                  graphicsOptions={graphicsOptions}
                  firmwareOptions={firmwareOptions}
                  busy={busy}
                  onSelectedIdChange={setTemplateId}
                  onAgentChange={setAgentId}
                  onSubmit={submitPayload}
                  onQueued={onClose}
                />
              ) : (
                <>
                  <BasicInfoPanel
                    agentId={agentId}
                    hostOptions={hostOptions}
                    name={name}
                    description={description}
                    osType={osType}
                    osTypeOptions={osTypeOptions}
                    autostart={autostart}
                    busy={busy}
                    onAgentChange={setAgentId}
                    onNameChange={setName}
                    onDescriptionChange={setDescription}
                    onOSTypeChange={setOSType}
                    onAutostartChange={setAutostart}
                  />
                  <ComputePanel
                    cpu={cpu}
                    maxCPU={maxCPU}
                    memory={memory}
                    maxMemory={maxMemory}
                    cpuModel={cpuModel}
                    cpuModels={cpuModels}
                    hostCpu={selectedHost?.cpuCores}
                    hostMemoryMB={hostMemoryMB}
                    onCPUChange={setCPU}
                    onMaxCPUChange={setMaxCPU}
                    onMemoryChange={setMemory}
                    onMaxMemoryChange={setMaxMemory}
                    onCPUModelChange={setCPUModel}
                  />
                  <BlankStoragePanel
                    busy={busy}
                    storagePoolsLength={storagePools.length}
                    systemDiskTarget={systemDiskTarget}
                    diskPool={diskPool}
                    diskFormat={diskFormat}
                    diskBus={diskBus}
                    diskSize={diskSize}
                    diskName={diskName}
                    preallocMetadata={preallocMetadata}
                    extraDisks={extraDisks}
                    storageOptions={storageOptions}
                    formats={formats}
                    buses={buses}
                    isoPool={isoPool}
                    isoPath={isoPath}
                    isoBus={isoBus}
                    isoOptions={isoOptions}
                    isoBuses={isoBuses}
                    onDiskPoolChange={setDiskPool}
                    onDiskFormatChange={updateSystemFormat}
                    onDiskBusChange={updateSystemBus}
                    onDiskSizeChange={setDiskSize}
                    onDiskNameChange={setDiskName}
                    onDiskNameTouched={() => setDiskNameTouched(true)}
                    onPreallocChange={setPreallocMetadata}
                    onISOPoolChange={setISOPool}
                    onISOPathChange={setISOPath}
                    onISOBusChange={setISOBus}
                    onAddExtraDisk={addExtraDisk}
                    onUpdateExtraDisk={(id, capacityGB) => updateExtraDisk(id, { capacityGB })}
                    onRemoveExtraDisk={id =>
                      setExtraDisks(current => current.filter(item => item.id !== id))
                    }
                  />
                  <NetworkBootPanel
                    networkSource={networkSource}
                    networkOptions={networkOptions}
                    networkModel={networkModel}
                    networkModelOptions={networkModelOptions}
                    firmware={firmware}
                    firmwareOptions={firmwareOptions}
                    graphics={graphics}
                    graphicsOptions={graphicsOptions}
                    consolePasswordEnabled={consolePasswordEnabled}
                    consolePassword={consolePassword}
                    consolePasswordVisible={consolePasswordVisible}
                    busy={busy}
                    onNetworkSourceChange={setNetworkSource}
                    onNetworkModelChange={setNetworkModel}
                    onFirmwareChange={setFirmware}
                    onGraphicsChange={setGraphics}
                    onConsolePasswordEnabledChange={setConsolePasswordEnabled}
                    onConsolePasswordChange={setConsolePassword}
                    onConsolePasswordVisibleChange={setConsolePasswordVisible}
                  />
                  <div className="flex justify-end">
                    <PrimaryButton
                      label={busy ? '提交中' : '创建'}
                      disabled={busy}
                      onClick={() => void submit()}
                    />
                  </div>
                </>
              )}
            </div>
          </main>
        </div>
      </div>
    </DialogPortal>
  );
}
