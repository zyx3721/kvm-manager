import type { Host, VMCreatePayload } from '../../../lib/api';
import { diskExtensionError, extraDiskName, hasDuplicate } from './createDisk';
import { domainNameFromXML } from './xml';

type CommonCreateState = {
  agentId: string;
  name: string;
  description: string;
  autostart: boolean;
  cpu: number;
  maxCPU: number;
  memory: number;
  maxMemory: number;
  cpuModel: string;
  osType: string;
  isoPath: string;
  isoBus: string;
  networkSource: string;
  networkModel: string;
  graphics: string;
  consolePasswordEnabled: boolean;
  consolePassword: string;
  firmware: string;
  selectedHost?: Host;
  hostMemoryMB: number;
};

export type BlankCreateState = CommonCreateState & {
  diskName: string;
  diskPool: string;
  diskFormat: string;
  diskBus: string;
  diskSize: number;
  preallocMetadata: boolean;
  extraDisks: Array<{ capacityGB: string }>;
};

export type DiskTemplateCreateState = CommonCreateState & {
  sourcePool: string;
  sourceName: string;
  targetPool: string;
  targetName: string;
  bus: string;
  format: string;
  preallocMetadata: boolean;
};

export function buildBlankCreatePayload(state: BlankCreateState): {
  payload?: VMCreatePayload;
  warning?: string;
  vmName?: string;
} {
  const vmName = state.name.trim();
  const normalizedDisks = [
    {
      name: state.diskName.trim(),
      pool: state.diskPool.trim(),
      format: state.diskFormat.trim(),
      bus: state.diskBus.trim(),
      capacityGB: state.diskSize,
      preallocMetadata: state.diskFormat === 'qcow2' && state.preallocMetadata,
    },
    ...state.extraDisks.map((disk, index) => ({
      name: extraDiskName(state.diskName, state.diskBus, state.diskFormat, index + 1).trim(),
      pool: state.diskPool.trim(),
      format: state.diskFormat.trim(),
      bus: state.diskBus.trim(),
      capacityGB: Number(disk.capacityGB),
      preallocMetadata: state.diskFormat === 'qcow2' && state.preallocMetadata,
    })),
  ];
  const commonWarning = validateCommonCreateState(state, vmName);
  if (commonWarning) return { warning: commonWarning };
  if (!state.diskPool || !state.networkSource) return { warning: '请选择存储池和网络池' };
  if (
    normalizedDisks.some(
      disk =>
        !disk.name ||
        !disk.pool ||
        !disk.format ||
        !disk.bus ||
        !Number.isFinite(disk.capacityGB) ||
        disk.capacityGB <= 0
    )
  )
    return { warning: '请完整填写磁盘配置' };
  if (hasDuplicate(normalizedDisks.map(disk => `${disk.pool}/${disk.name}`)))
    return { warning: '磁盘卷名称不能重复' };
  const extensionError = diskExtensionError(normalizedDisks);
  if (extensionError) return { warning: extensionError };
  return {
    vmName,
    payload: {
      ...commonPayload(state, vmName),
      createMode: 'blank',
      disks: normalizedDisks,
      diskName: state.diskName,
      diskPool: state.diskPool,
      diskFormat: state.diskFormat,
      diskBus: state.diskBus,
      diskCapacityGB: state.diskSize,
      preallocMetadata: state.diskFormat === 'qcow2' && state.preallocMetadata,
    },
  };
}

export function buildDiskTemplateCreatePayload(state: DiskTemplateCreateState): {
  payload?: VMCreatePayload;
  warning?: string;
  vmName?: string;
} {
  const vmName = state.name.trim();
  const commonWarning = validateCommonCreateState(state, vmName);
  if (commonWarning) return { warning: commonWarning };
  if (!state.sourcePool.trim() || !state.sourceName.trim() || !state.targetPool.trim() || !state.targetName.trim() || !state.bus.trim()) {
    return { warning: '请完整填写模板磁盘配置' };
  }
  const extensionError = diskExtensionError([{ name: state.targetName.trim(), format: state.format }]);
  if (extensionError) return { warning: extensionError };
  return {
    vmName,
    payload: {
      ...commonPayload(state, vmName),
      createMode: 'template',
      disks: [],
      diskName: state.targetName.trim(),
      diskPool: state.targetPool.trim(),
      diskFormat: state.format.trim(),
      diskBus: state.bus.trim(),
      diskCapacityGB: 0,
      preallocMetadata: state.format === 'qcow2' && state.preallocMetadata,
      template: {
        sourcePool: state.sourcePool.trim(),
        sourceName: state.sourceName.trim(),
        targetPool: state.targetPool.trim(),
        targetName: state.targetName.trim(),
        bus: state.bus.trim(),
        format: state.format.trim(),
        convert: false,
        preallocMetadata: state.format === 'qcow2' && state.preallocMetadata,
      },
    },
  };
}

export function buildXMLCreatePayload({
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
}: {
  agentId: string;
  xml: string;
  description: string;
  autostart: boolean;
  cpu: number;
  maxCPU: number;
  memory: number;
  maxMemory: number;
  cpuModel: string;
  osType: string;
}): { payload?: VMCreatePayload; warning?: string; vmName?: string } {
  const nextXML = xml.trim();
  const vmName = domainNameFromXML(nextXML);
  if (!agentId) return { warning: '请选择宿主机' };
  if (!nextXML) return { warning: '虚拟机 XML 不能为空' };
  if (!vmName) return { warning: '请在 XML 中配置虚拟机名称' };
  return {
    vmName,
    payload: {
      agentId,
      createMode: 'xml',
      name: vmName,
      description: description.trim(),
      autostart,
      currentCpu: cpu,
      maximumCpu: maxCPU,
      currentMemoryMB: memory,
      maximumMemoryMB: maxMemory,
      cpuModel,
      osType,
      disks: [],
      diskName: '',
      diskPool: '',
      diskFormat: '',
      diskBus: '',
      diskCapacityGB: 0,
      preallocMetadata: false,
      isoPath: '',
      isoBus: '',
      networkSource: '',
      networkModel: '',
      graphics: '',
      consolePassword: '',
      bootFirmware: '',
      xml: nextXML,
    },
  };
}

function validateCommonCreateState(state: CommonCreateState, vmName: string) {
  if (!state.agentId) return '请选择宿主机';
  if (!vmName) return '请输入虚拟机名称';
  if (state.selectedHost?.cpuCores && state.maxCPU > state.selectedHost.cpuCores)
    return '最大 CPU 不能超过宿主机逻辑 CPU';
  if (state.hostMemoryMB > 0 && state.maxMemory > state.hostMemoryMB)
    return '最大内存不能超过宿主机总内存';
  if (state.consolePasswordEnabled && !state.consolePassword.trim()) return '请输入控制台密码';
  return '';
}

function commonPayload(state: CommonCreateState, vmName: string) {
  return {
    agentId: state.agentId,
    name: vmName,
    description: state.description.trim(),
    autostart: state.autostart,
    currentCpu: state.cpu,
    maximumCpu: state.maxCPU,
    currentMemoryMB: state.memory,
    maximumMemoryMB: state.maxMemory,
    cpuModel: state.cpuModel,
    osType: state.osType,
    isoPath: state.isoPath,
    isoBus: state.isoBus,
    networkSource: state.networkSource,
    networkModel: state.networkModel,
    graphics: state.graphics,
    consolePassword: state.consolePasswordEnabled ? state.consolePassword.trim() : '',
    bootFirmware: state.firmware,
  };
}
