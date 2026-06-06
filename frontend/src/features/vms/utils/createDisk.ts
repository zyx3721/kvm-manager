export function diskTargetForBus(bus: string, index: number) {
  const prefix = bus === 'ide' ? 'hd' : bus === 'sata' || bus === 'scsi' ? 'sd' : 'vd';
  return `${prefix}${diskLetter(index)}`;
}

export function extraDiskName(systemDiskName: string, bus: string, format: string, index: number) {
  const clean = replaceDiskExtension(systemDiskName, format);
  if (!clean) return '';
  const target = diskTargetForBus(bus, index);
  const systemTarget = diskTargetForBus(bus, 0);
  const systemTargetPattern = new RegExp(`-${systemTarget}(?=\\.)`, 'i');
  if (systemTargetPattern.test(clean)) return clean.replace(systemTargetPattern, `-${target}`);
  const extensionIndex = clean.lastIndexOf('.');
  if (extensionIndex <= 0) return `${clean}-${target}`;
  return `${clean.slice(0, extensionIndex)}-${target}${clean.slice(extensionIndex)}`;
}

export function extensionForFormat(format: string) {
  return format.trim().toLowerCase() === 'qcow2' ? '.qcow2' : '.img';
}

export function replaceDiskExtension(value: string, format: string) {
  const clean = value.trim();
  const extension = extensionForFormat(format);
  if (!clean) return clean;
  const index = clean.lastIndexOf('.');
  if (index <= 0) return `${clean}${extension}`;
  return `${clean.slice(0, index)}${extension}`;
}

export function replaceDiskTargetAndExtension(
  value: string,
  oldBus: string,
  nextBus: string,
  format: string,
  index: number
) {
  const extensionName = replaceDiskExtension(value, format);
  const oldTargetPattern = new RegExp(`-${diskTargetForBus(oldBus, index)}(?=\\.)`, 'i');
  if (oldTargetPattern.test(extensionName))
    return extensionName.replace(oldTargetPattern, `-${diskTargetForBus(nextBus, index)}`);
  return extensionName;
}

export function hasDuplicate(values: string[]) {
  const seen = new Set<string>();
  for (const value of values) {
    const key = value.trim().toLowerCase();
    if (!key) continue;
    if (seen.has(key)) return true;
    seen.add(key);
  }
  return false;
}

export function diskExtensionError(disks: Array<{ name: string; format: string }>) {
  const invalid = disks.find(
    disk => !disk.name.toLowerCase().endsWith(extensionForFormat(disk.format))
  );
  if (!invalid) return '';
  return `${invalid.name || '磁盘卷名'} 名称扩展名必须与格式一致`;
}

function diskLetter(index: number) {
  return String.fromCharCode('a'.charCodeAt(0) + Math.max(0, index));
}
