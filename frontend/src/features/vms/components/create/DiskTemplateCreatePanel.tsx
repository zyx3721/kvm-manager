import { useEffect, useMemo, useState } from 'react';

import { FileStackIcon } from 'lucide-react';
import { toast } from 'sonner';

import { SelectMenu } from '../../../../components/kvm/SelectMenu';
import {
  fetchISOFiles,
  fetchStorageVolumes,
  type Host,
  type ISOFile,
  type StorageVolume,
  type VMCreatePayload,
} from '../../../../lib/api';
import { formatBytes } from '../../../../lib/format';
import { extensionForFormat, replaceDiskExtension } from '../../utils/createDisk';
import { buildDiskTemplateCreatePayload } from '../../utils/createSubmit';
import {
  BasicInfoPanel,
  ComputePanel,
  NetworkBootPanel,
  PrimaryButton,
} from './CreatePanels';
import {
  Field,
  MetadataToggleRow,
  Panel,
  fieldStyle,
  inputClass,
} from './CreateFormShared';

type Option = { value: string; label: string; tooltip?: string };

const templateVolumeExtensions = ['.qcow2', '.img', '.raw', '.qcow', '.qed'];

export function DiskTemplateCreatePanel({
  agentId,
  hostOptions,
  selectedHost,
  hostMemoryMB,
  storageOptions,
  networkOptions,
  osTypeOptions,
  cpuModels,
  buses,
  isoBuses,
  networkModelOptions,
  graphicsOptions,
  firmwareOptions,
  busy,
  onAgentChange,
  onSubmit,
}: {
  agentId: string;
  hostOptions: Option[];
  selectedHost?: Host;
  hostMemoryMB: number;
  storageOptions: Option[];
  networkOptions: Option[];
  osTypeOptions: Option[];
  cpuModels: Option[];
  buses: Option[];
  isoBuses: Option[];
  networkModelOptions: Option[];
  graphicsOptions: Option[];
  firmwareOptions: Option[];
  busy: boolean;
  onAgentChange: (value: string) => void;
  onSubmit: (result: { payload?: VMCreatePayload; warning?: string; vmName?: string }) => Promise<void>;
}) {
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [autostart, setAutostart] = useState(false);
  const [cpu, setCPU] = useState(2);
  const [maxCPU, setMaxCPU] = useState(2);
  const [memory, setMemory] = useState(4096);
  const [maxMemory, setMaxMemory] = useState(4096);
  const [cpuModel, setCPUModel] = useState('host-passthrough');
  const [osType, setOSType] = useState('linux');
  const [sourcePool, setSourcePool] = useState('');
  const [sourceName, setSourceName] = useState('');
  const [targetPool, setTargetPool] = useState('');
  const [targetName, setTargetName] = useState('');
  const [targetNameTouched, setTargetNameTouched] = useState(false);
  const [bus, setBus] = useState('virtio');
  const [format, setFormat] = useState('qcow2');
  const [preallocMetadata, setPreallocMetadata] = useState(false);
  const [volumes, setVolumes] = useState<StorageVolume[]>([]);
  const [isoPool, setISOPool] = useState('');
  const [isoFiles, setISOFiles] = useState<ISOFile[]>([]);
  const [isoPath, setISOPath] = useState('');
  const [isoBus, setISOBus] = useState('sata');
  const [networkSource, setNetworkSource] = useState('');
  const [networkModel, setNetworkModel] = useState('virtio');
  const [graphics, setGraphics] = useState('vnc');
  const [consolePasswordEnabled, setConsolePasswordEnabled] = useState(false);
  const [consolePassword, setConsolePassword] = useState('');
  const [consolePasswordVisible, setConsolePasswordVisible] = useState(false);
  const [firmware, setFirmware] = useState('bios');

  const templateVolumes = useMemo(() => volumes.filter(isTemplateVolume), [volumes]);
  const selectedVolume = templateVolumes.find(item => item.name === sourceName);
  const volumeOptions = templateVolumes.map(volume => ({
    value: volume.name,
    label: volume.name,
    tooltip: [volume.path, volume.format || volume.type, formatBytes(volume.capacity, 'GB')].filter(Boolean).join(' · '),
  }));
  const isoOptions = [
    { value: '', label: '不挂载 ISO' },
    ...isoFiles.map(file => ({ value: file.path, label: file.name, tooltip: file.path })),
  ];

  useEffect(() => {
    setSourcePool(current => current || storageOptions[0]?.value || '');
    setTargetPool(current => current || storageOptions[0]?.value || '');
    setISOPool(current => current || storageOptions[0]?.value || '');
  }, [storageOptions]);

  useEffect(() => {
    setNetworkSource(current => current || networkOptions[0]?.value || '');
  }, [networkOptions]);

  useEffect(() => {
    if (!agentId || !sourcePool) {
      setVolumes([]);
      setSourceName('');
      return;
    }
    let ignore = false;
    fetchStorageVolumes(agentId, sourcePool)
      .then(response => {
        if (ignore) return;
        const items = response.items.filter(isTemplateVolume);
        setVolumes(response.items);
        setSourceName(current => (items.some(item => item.name === current) ? current : items[0]?.name || ''));
      })
      .catch(error => {
        if (!ignore) {
          setVolumes([]);
          setSourceName('');
          toast.error(error instanceof Error ? error.message : '读取模板磁盘文件失败');
        }
      });
    return () => {
      ignore = true;
    };
  }, [agentId, sourcePool]);

  useEffect(() => {
    if (!agentId || !isoPool) {
      setISOFiles([]);
      setISOPath('');
      return;
    }
    let ignore = false;
    fetchISOFiles(agentId, isoPool)
      .then(response => {
        if (ignore) return;
        setISOFiles(response.items);
        setISOPath(current => (response.items.some(item => item.path === current) ? current : ''));
      })
      .catch(() => {
        if (!ignore) {
          setISOFiles([]);
          setISOPath('');
        }
      });
    return () => {
      ignore = true;
    };
  }, [agentId, isoPool]);

  useEffect(() => {
    const nextFormat = templateFormat(selectedVolume);
    if (!nextFormat || nextFormat === format) return;
    setFormat(nextFormat);
    setPreallocMetadata(false);
    setTargetName(current => (current ? replaceDiskExtension(current, nextFormat) : current));
  }, [format, selectedVolume]);

  useEffect(() => {
    if (targetNameTouched) return;
    const cleanName = name.trim();
    if (!cleanName) {
      setTargetName('');
      return;
    }
    setTargetName(`${cleanName}-vda${extensionForFormat(format)}`);
  }, [format, name, targetNameTouched]);

  function submit() {
    void onSubmit(
      buildDiskTemplateCreatePayload({
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
        sourcePool,
        sourceName,
        targetPool,
        targetName,
        bus,
        format,
        preallocMetadata,
      })
    );
  }

  return (
    <div className="space-y-4">
      <BasicInfoPanel
        agentId={agentId}
        hostOptions={hostOptions}
        name={name}
        description={description}
        osType={osType}
        osTypeOptions={osTypeOptions}
        autostart={autostart}
        busy={busy}
        onAgentChange={onAgentChange}
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
      <Panel title="模板磁盘">
        <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-[minmax(0,1fr)_minmax(0,1.2fr)_minmax(0,1fr)_minmax(0,1.25fr)_minmax(0,0.8fr)]">
          <Field label="模板存储池">
            <SelectMenu
              value={sourcePool}
              options={storageOptions}
              placeholder="选择模板池"
              onChange={value => {
                setSourcePool(value);
                setSourceName('');
              }}
            />
          </Field>
          <Field label="模板文件">
            <SelectMenu
              value={sourceName}
              options={volumeOptions}
              placeholder="请选择磁盘模板"
              maxVisibleItems={6}
              onChange={setSourceName}
            />
          </Field>
          <Field label="目标存储池">
            <SelectMenu
              value={targetPool}
              options={storageOptions}
              placeholder="选择目标池"
              onChange={setTargetPool}
            />
          </Field>
          <Field label="目标系统盘卷名">
            <input
              value={targetName}
              disabled={busy}
              onChange={event => {
                setTargetNameTouched(true);
                setTargetName(event.target.value);
              }}
              className={inputClass}
              style={fieldStyle}
            />
          </Field>
          <Field label="磁盘总线">
            <SelectMenu value={bus} options={buses} placeholder="总线" onChange={setBus} />
          </Field>
        </div>
        <div className="mt-3 flex flex-wrap items-center gap-2 text-xs" style={{ color: 'var(--kvm-text-muted)' }}>
          <span
            className="inline-flex h-7 w-7 items-center justify-center rounded-lg border"
            style={{ background: 'rgba(59,130,246,0.12)', borderColor: 'rgba(59,130,246,0.34)', color: '#93c5fd' }}
          >
            <FileStackIcon size={15} />
          </span>
          <span>仅显示 qcow2、img、raw、qcow、qed 且支持克隆的模板卷</span>
          <span className="font-mono" style={{ color: 'var(--kvm-text)' }}>
            {selectedVolume
              ? `${selectedVolume.format || selectedVolume.type || format} · ${formatBytes(selectedVolume.capacity, 'GB')}`
              : '未选择模板文件'}
          </span>
        </div>
        <MetadataToggleRow
          format={format}
          checked={preallocMetadata}
          disabled={busy}
          onChange={setPreallocMetadata}
        />
      </Panel>
      <Panel title="可选介质">
        <div className="grid gap-3 md:grid-cols-3">
          <Field label="ISO 池">
            <SelectMenu
              value={isoPool}
              options={storageOptions}
              placeholder="选择 ISO 池"
              onChange={setISOPool}
            />
          </Field>
          <Field label="ISO 镜像">
            <SelectMenu
              value={isoPath}
              options={isoOptions}
              placeholder="选择 ISO"
              maxVisibleItems={5}
              onChange={setISOPath}
            />
          </Field>
          <Field label="光驱总线">
            <SelectMenu value={isoBus} options={isoBuses} placeholder="光驱总线" onChange={setISOBus} />
          </Field>
        </div>
      </Panel>
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
        <PrimaryButton label={busy ? '提交中' : '创建'} disabled={busy} onClick={submit} />
      </div>
    </div>
  );
}

function isTemplateVolume(volume: StorageVolume) {
  if (!volume.cloneSupported) return false;
  const format = (volume.format || volume.type || '').trim().toLowerCase();
  if (['qcow2', 'raw', 'qcow', 'qed'].includes(format)) return true;
  const name = volume.name.toLowerCase();
  const path = volume.path.toLowerCase();
  return templateVolumeExtensions.some(extension => name.endsWith(extension) || path.endsWith(extension));
}

function templateFormat(volume?: StorageVolume) {
  if (!volume) return '';
  const format = (volume.format || volume.type || '').trim().toLowerCase();
  if (format === 'qcow2') return 'qcow2';
  if (['raw', 'qcow', 'qed'].includes(format)) return format;
  const value = `${volume.name} ${volume.path}`.toLowerCase();
  if (value.includes('.qcow2')) return 'qcow2';
  if (value.includes('.qcow')) return 'qcow';
  if (value.includes('.qed')) return 'qed';
  if (value.includes('.raw')) return 'raw';
  return 'raw';
}
