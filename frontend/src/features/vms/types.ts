import type { VirtualMachine } from '../../lib/api';

export type VMAction =
  | 'start'
  | 'resume'
  | 'stop'
  | 'force-stop'
  | 'shutdown'
  | 'force-shutdown'
  | 'pause'
  | 'reboot'
  | 'force-reboot'
  | 'delete'
  | 'force-delete';

export type VMBulkAction = VMAction;

export type PendingAction = {
  vm: VirtualMachine;
  action: VMAction | 'console';
};

export const actionMeta: Record<
  VMAction,
  { label: string; tone: 'normal' | 'warning' | 'danger'; description: string }
> = {
  start: {
    label: '启动',
    tone: 'normal',
    description: '将启动已停止的虚拟机，或恢复已暂停的虚拟机',
  },
  resume: { label: '恢复', tone: 'normal', description: '将恢复已暂停的虚拟机' },
  pause: { label: '暂停', tone: 'warning', description: '将暂停正在运行的虚拟机' },
  reboot: { label: '重启', tone: 'warning', description: '将重启正在运行的虚拟机' },
  'force-reboot': { label: '强制重启', tone: 'danger', description: '将强制重置正在运行的虚拟机' },
  stop: { label: '停止', tone: 'danger', description: '将正常停止正在运行的虚拟机' },
  'force-stop': { label: '强制停止', tone: 'danger', description: '将强制停止正在运行的虚拟机' },
  shutdown: { label: '关机', tone: 'danger', description: '将正常关闭正在运行的虚拟机' },
  'force-shutdown': {
    label: '强制关机',
    tone: 'danger',
    description: '将强制关闭正在运行的虚拟机',
  },
  delete: { label: '删除', tone: 'danger', description: '将删除虚拟机定义并移除关联存储' },
  'force-delete': {
    label: '强制删除',
    tone: 'danger',
    description: '将强制关闭虚拟机后删除定义并移除关联存储',
  },
};
