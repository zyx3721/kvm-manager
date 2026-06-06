import type { StorageVolume } from '../../../lib/api';

export function friendlyVolumeError(error: unknown, fallback: string) {
  const raw = error instanceof Error ? error.message : fallback;
  const compact = raw.replace(/\s+/g, ' ').trim();
  const lower = compact.toLowerCase();
  if (
    lower.includes('target volume already exists') ||
    lower.includes('exists already') ||
    lower.includes('already exists')
  ) {
    return '镜像名称已存在，请更换名称';
  }
  if (
    lower.includes('failed to get shared "write" lock') ||
    lower.includes('failed to get "write" lock') ||
    lower.includes('is already in use')
  ) {
    return '当前镜像正在被虚拟机使用，无法克隆。请先关闭相关虚拟机，或通过快照创建一致性副本';
  }
  if (
    lower.includes('storage volume is used by virtual machine') ||
    lower.includes('正在被虚拟机使用')
  ) {
    return '该存储卷正在被虚拟机使用，请先从虚拟机移除磁盘或关闭相关虚拟机后再删除';
  }
  if (lower.includes('unsupported storage volume format')) return '当前镜像格式不受支持';
  if (lower.includes('storage volume capacity is required')) return '请填写镜像容量';
  if (lower.includes('storage pool and volume name are required')) return '请填写镜像名称';
  if (lower.includes('virsh') || lower.includes('qemu-img') || lower.includes('error:'))
    return '宿主机命令执行失败，请检查名称、格式和权限';
  return compact.length > 120 ? `${compact.slice(0, 120)}...` : compact;
}

export function volumeNameExists(volumes: StorageVolume[], name: string) {
  const target = name.trim().toLowerCase();
  return target !== '' && volumes.some(volume => volume.name.trim().toLowerCase() === target);
}
