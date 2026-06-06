import { toast } from 'sonner';

import { type VirtualMachine } from '../../../../lib/api';

export const memoryOptions = [1024, 2048, 4096, 8192, 16384, 32768, 65536, 131072];

export function numberRange(start: number, end: number) {
  const safeStart = Math.max(1, start);
  const safeEnd = Math.max(safeStart, end);
  return Array.from({ length: safeEnd - safeStart + 1 }, (_, index) => safeStart + index);
}

export function nearestMemoryOption(value: number) {
  return memoryOptions.find(item => item >= value) ?? memoryOptions[memoryOptions.length - 1];
}

export function memoryRangeOptions(limitMB: number, stepMB = 1024) {
  const safeStep = Math.max(1, stepMB);
  const safeLimit = Math.max(safeStep, Math.floor(limitMB));
  const options: number[] = [];
  for (let value = safeStep; value <= safeLimit; value += safeStep) {
    options.push(value);
  }
  if (options[options.length - 1] !== safeLimit) {
    options.push(safeLimit);
  }
  return options;
}

export function bytesToMB(value: number) {
  return Math.max(1, Math.round(value / 1024 / 1024));
}

export function parsePositiveInteger(label: string, value: string, custom: boolean) {
  const text = value.trim();
  if (custom && !/^\d+$/.test(text)) {
    toast.error(`${label}必须为纯数字`);
    return null;
  }
  const parsed = Number(text);
  if (!Number.isInteger(parsed) || parsed <= 0) {
    toast.error(`${label}必须大于 0`);
    return null;
  }
  return parsed;
}

export function buildPreviewXML(vm: VirtualMachine, memoryMB: number) {
  const memoryKiB = memoryMB * 1024;
  const diskXML = (
    vm.disks.length > 0
      ? vm.disks
      : [{ name: 'vda', path: '', bytes: vm.diskBytes, usedBytes: vm.diskUsedBytes }]
  )
    .map(
      disk =>
        `    <disk type='file' device='disk'>\n      <source file='${escapeXML(disk.path || '/var/lib/libvirt/images/' + vm.name + '-' + disk.name + '.qcow2')}'/>\n      <target dev='${escapeXML(disk.name || 'vda')}' bus='virtio'/>\n    </disk>`
    )
    .join('\n');
  return `<domain type='kvm'>\n  <name>${escapeXML(vm.name)}</name>\n  <uuid>${escapeXML(vm.uuid || '')}</uuid>\n  <memory unit='KiB'>${memoryKiB}</memory>\n  <currentMemory unit='KiB'>${memoryKiB}</currentMemory>\n  <vcpu placement='static' current='${vm.cpuCores}'>${vm.cpuCores}</vcpu>\n  <os>\n    <type arch='x86_64'>hvm</type>\n  </os>\n  <devices>\n${diskXML}\n    <interface type='network'>\n      <model type='virtio'/>\n    </interface>\n  </devices>\n</domain>`;
}

function escapeXML(value: string) {
  return value
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&apos;');
}
