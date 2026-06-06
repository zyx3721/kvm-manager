import {
  BoxesIcon,
  CameraIcon,
  DatabaseIcon,
  LayoutDashboardIcon,
  ListChecksIcon,
  NetworkIcon,
  ServerIcon,
  SettingsIcon,
  WaypointsIcon,
  type LucideIcon,
} from 'lucide-react';

export type NavItem = {
  label: string;
  icon: LucideIcon;
  path: string;
  permissions: string[];
};

export const navItems: NavItem[] = [
  { label: '总览', icon: LayoutDashboardIcon, path: '/', permissions: ['dashboard.read'] },
  { label: '虚拟机 / 模板', icon: BoxesIcon, path: '/vms', permissions: ['vms.read'] },
  { label: '宿主机', icon: ServerIcon, path: '/hosts', permissions: ['hosts.read', 'agents.read'] },
  {
    label: '接口',
    icon: WaypointsIcon,
    path: '/host-interfaces',
    permissions: ['host.interfaces.read'],
  },
  { label: '存储池', icon: DatabaseIcon, path: '/storage-pools', permissions: ['storage.read'] },
  { label: '网络池', icon: NetworkIcon, path: '/network-pools', permissions: ['network.read'] },
  { label: '快照', icon: CameraIcon, path: '/snapshots', permissions: ['snapshots.read'] },
  {
    label: '任务 / 审计',
    icon: ListChecksIcon,
    path: '/operations',
    permissions: ['operations.read'],
  },
  {
    label: '系统配置',
    icon: SettingsIcon,
    path: '/settings',
    permissions: [
      'settings.base.read',
      'settings.base.manage',
      'settings.users.read',
      'settings.users.manage',
      'settings.auth.read',
      'settings.auth.manage',
      'settings.notifications.read',
      'settings.notifications.manage',
    ],
  },
];
