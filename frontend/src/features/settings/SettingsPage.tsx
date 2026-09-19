import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { toast } from 'sonner';
import {
  BellRingIcon,
  MegaphoneIcon,
  NetworkIcon,
  SlidersHorizontalIcon,
  SettingsIcon,
  UsersRoundIcon,
} from 'lucide-react';
import AuthSettingsPanel from './components/AuthSettingsPanel';
import { BaseSettingsPanel } from './components/BaseSettingsPanel';
import { NotificationSettingsPanel } from './components/NotificationSettingsPanel';
import { UserSettingsPanel } from './components/UserSettingsPanel';
import { can } from '../../lib/permissions';

type SettingsTab = 'base' | 'users' | 'auth' | 'notifications';

export default function SettingsPage() {
  const [tab, setTab] = useState<SettingsTab>('base');
  const [error, setError] = useState('');
  const canReadBase = can('settings.base.read');
  const canManageBase = can('settings.base.manage');
  const canReadUsers = can('settings.users.read');
  const canManageUsers = can('settings.users.manage');
  const canReadAuth = can('settings.auth.read');
  const canManageAuth = can('settings.auth.manage');
  const canReadNotifications = can('settings.notifications.read');
  const canManageNotifications = can('settings.notifications.manage');
  const visibleTabs = useMemo(
    () =>
      [
        {
          id: 'base' as const,
          icon: SlidersHorizontalIcon,
          label: '基础配置',
          visible: canReadBase || canManageBase,
        },
        {
          id: 'users' as const,
          icon: UsersRoundIcon,
          label: '用户配置',
          visible: canReadUsers || canManageUsers,
        },
        {
          id: 'auth' as const,
          icon: NetworkIcon,
          label: '认证配置',
          visible: canReadAuth || canManageAuth,
        },
        {
          id: 'notifications' as const,
          icon: BellRingIcon,
          label: '通知配置',
          visible: canReadNotifications || canManageNotifications,
        },
      ].filter(item => item.visible),
    [
      canManageAuth,
      canManageBase,
      canManageNotifications,
      canManageUsers,
      canReadAuth,
      canReadBase,
      canReadNotifications,
      canReadUsers,
    ]
  );

  const load = useCallback(async () => {
    setError('');
    try {
      // 认证配置由 AuthSettingsPanel 自行加载
      await Promise.all([]);
    } catch (err) {
      const message = err instanceof Error ? err.message : '读取系统配置失败';
      toast.error(message);
      setError(isPermissionMessage(message) ? '' : message);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);
  useEffect(() => {
    if (!visibleTabs.length) return;
    if (!visibleTabs.some(item => item.id === tab)) {
      setTab(visibleTabs[0].id);
    }
  }, [tab, visibleTabs]);
  return (
    <SettingsPageFrame>
      <div className="flex items-center justify-between gap-4">
        <div>
          <h1 className="text-lg font-semibold" style={{ color: 'var(--kvm-text)' }}>
            系统配置
          </h1>
          <p className="mt-1 text-sm" style={{ color: 'var(--kvm-text-muted)' }}>
            管理平台用户、认证与通知媒介
          </p>
        </div>
        <div
          className="hidden items-center gap-2 rounded-lg px-3 py-2 text-sm md:flex"
          style={{
            color: 'var(--kvm-text-muted)',
            background: 'rgba(255,255,255,0.04)',
            border: '1px solid var(--kvm-border)',
          }}
        >
          <SettingsIcon size={15} />
          配置中心
        </div>
      </div>
      <section
        className="flex flex-wrap gap-2 rounded-xl p-3"
        style={{ background: 'var(--kvm-card)', border: '1px solid var(--kvm-border)' }}
      >
        {visibleTabs.map(item => (
          <SettingsTabButton
            key={item.id}
            active={tab === item.id}
            icon={item.icon}
            label={item.label}
            onClick={() => setTab(item.id)}
          />
        ))}
      </section>
      {error && (
        <div
          className="rounded-xl p-4 text-sm"
          style={{
            background: 'rgba(245,158,11,0.1)',
            border: '1px solid rgba(245,158,11,0.25)',
            color: '#f59e0b',
          }}
        >
          {error}
        </div>
      )}
      {visibleTabs.length === 0 && (
        <div
          className="kvm-empty-state rounded-xl p-8 text-center text-sm"
          style={{ color: 'var(--kvm-text-muted)' }}
        >
          暂无可查看的配置项
        </div>
      )}
      {tab === 'base' && (canReadBase || canManageBase) && (
        <SettingsConfigSection
          title="基础配置"
          description="维护站点名称、登录展示、控制台品牌和固定数字配置规划。"
          badge={<SettingsSectionBadge icon={SlidersHorizontalIcon} label="系统基础" />}
        >
          <BaseSettingsPanel canManage={canManageBase} />
        </SettingsConfigSection>
      )}
      {tab === 'users' && (canReadUsers || canManageUsers) && (
        <SettingsConfigSection
          title="用户配置"
          description="维护平台用户、用户群组和角色权限。启用 AD/LDAP 后，外部账号也必须先在用户中创建并启用。"
          badge={<SettingsSectionBadge icon={UsersRoundIcon} label="用户权限" />}
        >
          <UserSettingsPanel canManage={canManageUsers} />
        </SettingsConfigSection>
      )}
      {tab === 'notifications' && (canReadNotifications || canManageNotifications) && (
        <SettingsConfigSection
          title="通知媒介"
          description="站内告警通过右上角通知中心查看，外部媒介启用后会接收活跃告警、恢复通知推送；也可用于找回密码。"
          badge={<SettingsSectionBadge icon={MegaphoneIcon} label="告警通知" />}
        >
          <NotificationSettingsPanel canManage={canManageNotifications} />
        </SettingsConfigSection>
      )}
      {tab === 'auth' && (canReadAuth || canManageAuth) && (
        <SettingsConfigSection
          title="认证配置"
          description="启用外部认证后，登录界面会显示对应登录方式。"
          badge={<SettingsSectionBadge icon={NetworkIcon} label="身份认证" />}
        >
          <AuthSettingsPanel canManage={canManageAuth} />
        </SettingsConfigSection>
      )}
    </SettingsPageFrame>
  );
}

function SettingsTabButton({
  active,
  icon: Icon,
  label,
  onClick,
}: {
  active: boolean;
  icon: React.ElementType;
  label: string;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className="kvm-action-button flex h-10 items-center gap-2 rounded-lg px-4 text-sm font-medium"
      style={{
        color: active ? 'var(--kvm-accent-hover)' : 'var(--kvm-text-muted)',
        background: active ? 'rgba(59,130,246,0.14)' : 'transparent',
        border: active ? '1px solid rgba(59,130,246,0.32)' : '1px solid transparent',
      }}
    >
      <Icon size={16} />
      {label}
    </button>
  );
}

function SettingsPageFrame({ children }: { children: React.ReactNode }) {
  return (
    <div data-cmp="SettingsPage" className="flex h-full min-h-0 flex-col gap-6 overflow-hidden p-6">
      {children}
    </div>
  );
}

function SettingsConfigSection({
  title,
  description,
  badge,
  children,
}: {
  title: string;
  description: string;
  badge: React.ReactNode;
  children: React.ReactNode;
}) {
  return (
    <section
      className="flex min-h-0 flex-1 flex-col overflow-hidden rounded-xl p-5"
      style={{ background: 'var(--kvm-card)', border: '1px solid var(--kvm-border)' }}
    >
      <div className="mb-5 flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
        <div>
          <h2 className="text-sm font-semibold" style={{ color: 'var(--kvm-text)' }}>
            {title}
          </h2>
          <p className="mt-1 text-xs" style={{ color: 'var(--kvm-text-muted)' }}>
            {description}
          </p>
        </div>
        {badge}
      </div>
      {children}
    </section>
  );
}

function SettingsSectionBadge({ icon: Icon, label }: { icon: React.ElementType; label: string }) {
  return (
    <div
      className="flex items-center gap-2 rounded-lg px-3 py-2 text-xs"
      style={{
        color: 'var(--kvm-accent-text)',
        background: 'rgba(59,130,246,0.1)',
        border: '1px solid rgba(59,130,246,0.24)',
      }}
    >
      <Icon size={14} />
      {label}
    </div>
  );
}

function isPermissionMessage(message: string) {
  return message.includes('当前用户无权执行此操作');
}
