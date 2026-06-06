import { useEffect, useMemo, useRef, useState } from 'react';
import { createPortal } from 'react-dom';
import { Outlet, useLocation, useNavigate } from 'react-router-dom';
import { toast } from 'sonner';
import {
  BellIcon,
  ChevronDownIcon,
  ChevronRightIcon,
  CpuIcon,
  KeyRoundIcon,
  Loader2Icon,
  LogOutIcon,
  MoonIcon,
  PanelLeftCloseIcon,
  PanelLeftOpenIcon,
  SunIcon,
  UserIcon,
} from 'lucide-react';
import { useBaseConfig } from '../../lib/branding';
import { clearSession, getStoredUser, logout, userHasAnyPermission, userHasPermission } from '../../lib/auth';
import {
  fetchNotifications,
  fetchUnreadNotificationCount,
  markAllNotificationsRead,
  clearNotifications,
  markNotificationRead,
  runtimeEventsUrl,
  type Alert,
  type RefreshProgress,
} from '../../lib/api';
import { emitKvmRefresh } from '../../lib/refresh';
import { emitKvmResourceEvent, type KvmResourceEventType } from '../../lib/resourceEvents';
import {
  applyKvmTheme,
  getInitialKvmTheme,
  persistKvmTheme,
  toggleKvmTheme,
  type KvmTheme,
} from '../../lib/utils';
import { KvmTooltip } from '../kvm/StatusBadge';
import { HeaderFullRefreshButton } from './HeaderFullRefreshButton';
import NotificationPanel from './NotificationPanel';
import PasswordDialog from './PasswordDialog';
import { navItems } from './navItems';
import { currentRefreshTask, parseStorageEvent, showVMCloneResult } from './runtimeEvents';
import { hasActiveStorageVolumeTask } from '../../features/storage-pools/utils/storageVolumeTaskRegistry';

const sidebarItemHeight = 46;
const sidebarItemGap = 4;
const sidebarTransitionUnitMs = 1000;
const sidebarTransitionMaxMs = 3000;

export default function KvmLayout() {
  const navigate = useNavigate();
  const location = useLocation();
  const [sidebarOpen, setSidebarOpen] = useState(true);
  const [userMenuOpen, setUserMenuOpen] = useState(false);
  const [notificationOpen, setNotificationOpen] = useState(false);
  const [notifications, setNotifications] = useState<Alert[]>([]);
  const [unreadCount, setUnreadCount] = useState(0);
  const [passwordDialogOpen, setPasswordDialogOpen] = useState(false);
  const [syncStatus, setSyncStatus] = useState<{ active: boolean; text: string }>({
    active: false,
    text: '',
  });
  const [theme, setTheme] = useState<KvmTheme>(getInitialKvmTheme);
  const baseConfig = useBaseConfig();
  const [user] = useState(() => getStoredUser());
  const userMenuRef = useRef<HTMLDivElement | null>(null);
  const notificationRef = useRef<HTMLDivElement | null>(null);
  const notificationPanelRef = useRef<HTMLDivElement | null>(null);
  const syncTimerRef = useRef<number | null>(null);
  const [notificationPanelPosition, setNotificationPanelPosition] = useState({ top: 0, right: 16 });

  const visibleNavItems = navItems.filter(item => userHasAnyPermission(user, item.permissions));
  const current = navItems.find(item => item.path === location.pathname) ?? visibleNavItems[0];
  const activeNavIndex = Math.max(0, visibleNavItems.findIndex(item => item.path === current?.path));
  const previousNavIndexRef = useRef(activeNavIndex);
  const [sidebarIndicatorDuration, setSidebarIndicatorDuration] = useState(0);
  const canManageAlerts = userHasPermission(user, 'alerts.manage');
  const canManageAgents = userHasPermission(user, 'agents.manage');
  const displayName = user?.username || 'admin';
  const sidebarIndicatorStyle = useMemo(
    () =>
      ({
        '--kvm-sidebar-active-top': `${activeNavIndex * (sidebarItemHeight + sidebarItemGap)}px`,
        '--kvm-sidebar-active-duration': `${sidebarIndicatorDuration}ms`,
      }) as React.CSSProperties,
    [activeNavIndex, sidebarIndicatorDuration]
  );

  useEffect(() => {
    const previousIndex = previousNavIndexRef.current;
    const distance = Math.abs(activeNavIndex - previousIndex);
    setSidebarIndicatorDuration(Math.min(sidebarTransitionMaxMs, distance * sidebarTransitionUnitMs));
    previousNavIndexRef.current = activeNavIndex;
  }, [activeNavIndex]);

  const handleLogout = async () => {
    setUserMenuOpen(false);
    await logout();
    toast.success('已退出登录');
    navigate('/login', { replace: true });
  };

  const handlePasswordChanged = () => {
    clearSession();
    navigate('/login', { replace: true });
  };

  const loadNotifications = async () => {
    const [items, count] = await Promise.all([
      fetchNotifications(20),
      fetchUnreadNotificationCount(),
    ]);
    setNotifications(items.items);
    setUnreadCount(count.count);
  };

  const handleNotificationToggle = () => {
    setNotificationOpen(value => {
      const next = !value;
      if (next) {
        updateNotificationPanelPosition();
        void loadNotifications().catch(() => undefined);
      }
      return next;
    });
  };

  const handleReadAll = async () => {
    if (!canManageAlerts) {
      toast.error('当前用户无权执行此操作');
      return;
    }
    await markAllNotificationsRead();
    await loadNotifications();
  };

  const handleClearNotifications = async () => {
    if (!canManageAlerts) {
      toast.error('当前用户无权执行此操作');
      return;
    }
    await clearNotifications();
    await loadNotifications();
  };

  const handleNotificationClick = async (item: Alert) => {
    if (!item.readAt) {
      await markNotificationRead(item.id).catch(() => undefined);
    }
    setNotificationOpen(false);
    navigate('/operations');
  };

  useEffect(() => {
    applyKvmTheme(theme);
    persistKvmTheme(theme);
  }, [theme]);

  useEffect(() => {
    if (visibleNavItems.length === 0) {
      toast.error('当前用户无权执行任何操作', { id: 'no-user-permissions' });
    }
  }, [visibleNavItems.length]);

  useEffect(() => {
    const close = (event: MouseEvent) => {
      const target = event.target as Node;
      if (!userMenuRef.current?.contains(target)) setUserMenuOpen(false);
      if (!notificationRef.current?.contains(target) && !notificationPanelRef.current?.contains(target)) {
        setNotificationOpen(false);
      }
    };
    window.addEventListener('mousedown', close);
    return () => window.removeEventListener('mousedown', close);
  }, []);

  useEffect(() => {
    if (!notificationOpen) return;
    updateNotificationPanelPosition();
    window.addEventListener('resize', updateNotificationPanelPosition);
    window.addEventListener('scroll', updateNotificationPanelPosition, true);
    return () => {
      window.removeEventListener('resize', updateNotificationPanelPosition);
      window.removeEventListener('scroll', updateNotificationPanelPosition, true);
    };
  }, [notificationOpen]);

  function updateNotificationPanelPosition() {
    const rect = notificationRef.current?.getBoundingClientRect();
    if (!rect) return;
    setNotificationPanelPosition({
      top: rect.bottom + 10,
      right: Math.max(16, window.innerWidth - rect.right),
    });
  }

  useEffect(() => {
    void loadNotifications().catch(() => undefined);
  }, []);

  useEffect(() => {
    const source = new EventSource(runtimeEventsUrl());
    const clearSyncTimer = () => {
      if (syncTimerRef.current !== null) {
        window.clearTimeout(syncTimerRef.current);
        syncTimerRef.current = null;
      }
    };
    const finishSync = () => {
      clearSyncTimer();
      setSyncStatus({ active: false, text: '' });
      emitKvmRefresh();
    };
    const armSyncTimer = () => {
      clearSyncTimer();
      syncTimerRef.current = window.setTimeout(
        () => setSyncStatus({ active: false, text: '' }),
        120000
      );
    };
    const showProgress = async () => {
      const task = await currentRefreshTask().catch(() => null);
      const payload = task?.payload as RefreshProgress | undefined;
      const total = payload?.totalAgents ?? 0;
      const done = (payload?.syncedAgents ?? 0) + (payload?.failedAgents ?? 0);
      const currentAgent = payload?.currentAgent ? ` · ${payload.currentAgent}` : '';
      setSyncStatus({
        active: true,
        text: total > 0 ? `同步中 ${done}/${total}${currentAgent}` : '同步中',
      });
      armSyncTimer();
    };
    source.addEventListener('sync.queued', () => {
      setSyncStatus({ active: true, text: '同步排队中' });
      armSyncTimer();
    });
    source.addEventListener('sync.started', () => {
      setSyncStatus({ active: true, text: '同步已开始' });
      armSyncTimer();
    });
    source.addEventListener('sync.progress', () => void showProgress());
    source.addEventListener('runtime.updated', () => {
      emitKvmRefresh();
      void loadNotifications().catch(() => undefined);
      window.setTimeout(
        () =>
          void currentRefreshTask()
            .then(task => {
              if (!task) setSyncStatus({ active: false, text: '' });
            })
            .catch(() => undefined),
        400
      );
    });
    const showStorageResult = (event: MessageEvent) => {
      const payload = parseStorageEvent(event.data);
      if (!payload.message) return;
      if (hasActiveStorageVolumeTask(payload.operation)) return;
      if (payload.type === 'failed') toast.error(payload.message);
      else toast.success(payload.message);
    };
    const emitResourceUpdate = (type: KvmResourceEventType) => (event: MessageEvent) => {
      const payload = parseResourceUpdateEvent(event.data);
      if (!payload.agentId) return;
      emitKvmResourceEvent({ type, ...payload });
    };
    source.addEventListener('storage.volume.completed', showStorageResult);
    source.addEventListener('storage.volume.failed', showStorageResult);
    source.addEventListener('storage.pool.updated', emitResourceUpdate('storage.pool.updated'));
    source.addEventListener('network.pool.updated', emitResourceUpdate('network.pool.updated'));
    source.addEventListener('host.interface.updated', emitResourceUpdate('host.interface.updated'));
    source.addEventListener('vm.clone.completed', showVMCloneResult);
    source.addEventListener('vm.clone.failed', showVMCloneResult);
    source.addEventListener('vm.create.completed', showVMCloneResult);
    source.addEventListener('vm.create.failed', showVMCloneResult);
    source.addEventListener('vm.migrate.completed', showVMCloneResult);
    source.addEventListener('vm.migrate.failed', showVMCloneResult);
    source.addEventListener('sync.failed', finishSync);
    source.addEventListener('sync.finished', finishSync);
    return () => {
      clearSyncTimer();
      source.close();
    };
  }, []);

  return (
    <div
      data-cmp="KvmLayout"
      className="flex h-screen w-full overflow-hidden"
      style={{ background: 'var(--kvm-bg)' }}
    >
      <aside
        className="flex flex-col kvm-hidden-scrollbar overflow-y-auto"
        style={{
          width: sidebarOpen ? '240px' : '64px',
          minWidth: sidebarOpen ? '240px' : '64px',
          background: 'var(--kvm-sidebar)',
          borderRight: '1px solid var(--kvm-border)',
          transition: 'width 0.25s ease, min-width 0.25s ease',
          position: 'relative',
          zIndex: 10,
        }}
      >
        <div
          className="flex items-center gap-3 px-3 py-4"
          style={{ borderBottom: '1px solid var(--kvm-border)', minHeight: '64px' }}
        >
          <img
            src={baseConfig.iconData}
            alt={baseConfig.appName}
            className="h-9 w-9 shrink-0 rounded-lg object-contain"
          />
          <div
            className={`min-w-0 flex-1 overflow-hidden transition-all duration-200 ${sidebarOpen ? 'opacity-100 w-auto' : 'opacity-0 w-0'}`}
          >
            <div className="kvm-gradient-text font-bold text-base whitespace-nowrap">
              {baseConfig.appName}
            </div>
            <div
              className="text-xs tracking-widest whitespace-nowrap"
              style={{ color: 'var(--kvm-text-muted)' }}
            >
              {baseConfig.appSubtitle}
            </div>
          </div>
        </div>

        <nav className="relative flex-1 px-2 py-4" style={sidebarIndicatorStyle}>
          {visibleNavItems.length > 0 && <span className="kvm-sidebar-active-indicator" aria-hidden="true" />}
          {visibleNavItems.map(item => {
            const Icon = item.icon;
            const isActive = current.path === item.path;
            return (
              <button
                key={item.path}
                type="button"
                onClick={() => navigate(item.path)}
                className={`kvm-sidebar-button w-full flex items-center rounded-lg mb-1 transition-all duration-150 ${isActive ? 'kvm-sidebar-item-active' : ''}`}
                style={{
                  height: `${sidebarItemHeight}px`,
                  padding: sidebarOpen ? '9px 12px' : '10px',
                  justifyContent: sidebarOpen ? 'flex-start' : 'center',
                  color: isActive ? '#3b82f6' : 'var(--kvm-text-muted)',
                }}
                aria-current={isActive ? 'page' : undefined}
              >
                <KvmTooltip
                  label={item.label}
                  placement="right"
                  className="inline-flex min-w-0 items-center"
                >
                  <Icon size={17} />
                  <span
                    className={`ml-3 text-sm whitespace-nowrap transition-all duration-200 ${sidebarOpen ? 'opacity-100 w-auto' : 'opacity-0 w-0 overflow-hidden'}`}
                    style={{ fontWeight: isActive ? 600 : 400 }}
                  >
                    {item.label}
                  </span>
                </KvmTooltip>
                {isActive && sidebarOpen && (
                  <span
                    className="ml-auto w-1.5 h-1.5 rounded-full"
                    style={{ background: '#3b82f6' }}
                  />
                )}
              </button>
            );
          })}
        </nav>

        <div
          className="flex items-center justify-center py-4"
          style={{ borderTop: '1px solid var(--kvm-border)' }}
        >
          <KvmTooltip label={sidebarOpen ? '收起侧边栏' : '展开侧边栏'} placement="top">
            <button
              type="button"
              onClick={() => setSidebarOpen(!sidebarOpen)}
              className="kvm-sidebar-toggle kvm-action-button flex h-9 w-9 items-center justify-center rounded-lg border transition-colors"
              aria-label={sidebarOpen ? '收起侧边栏' : '展开侧边栏'}
              style={{
                color: 'var(--kvm-accent-text)',
                background: 'var(--kvm-control-bg)',
                borderColor: 'var(--kvm-border)',
              }}
            >
              {sidebarOpen ? <PanelLeftCloseIcon size={17} /> : <PanelLeftOpenIcon size={17} />}
            </button>
          </KvmTooltip>
        </div>
      </aside>

      <div className="flex flex-col flex-1 min-w-0">
        <header
          className="flex items-center justify-between px-6"
          style={{
            height: '64px',
            background: 'var(--kvm-header)',
            borderBottom: '1px solid var(--kvm-border)',
            flexShrink: 0,
            position: 'relative',
            zIndex: 30,
          }}
        >
          <div className="flex items-center gap-2 min-w-0">
            <span className="text-sm" style={{ color: 'var(--kvm-text-muted)' }}>
              KVM 管理控制台
            </span>
            <ChevronRightIcon size={14} style={{ color: 'var(--kvm-text-muted)' }} />
            <span className="text-sm font-medium truncate" style={{ color: 'var(--kvm-text)' }}>
              {current?.label ?? '无可用页面'}
            </span>
          </div>

          <div className="flex items-center gap-3">
            {syncStatus.active && (
              <div
                className="hidden items-center gap-2 rounded-lg border px-3 py-2 text-xs md:flex"
                style={{
                  color: 'var(--kvm-accent-text)',
                  background: 'rgba(59,130,246,0.1)',
                  borderColor: 'rgba(59,130,246,0.24)',
                }}
              >
                <Loader2Icon size={14} className="animate-spin" />
                <span>{syncStatus.text}</span>
              </div>
            )}
            {canManageAgents && <HeaderFullRefreshButton />}
            <div ref={notificationRef} className="relative">
              <KvmTooltip label="通知消息" placement="bottom">
                <button
                  type="button"
                  onClick={handleNotificationToggle}
                  className="kvm-action-button relative flex h-[42px] w-[42px] items-center justify-center rounded-lg border"
                  style={{
                    background: 'var(--kvm-control-bg)',
                    borderColor: notificationOpen ? 'rgba(59,130,246,0.48)' : 'var(--kvm-border)',
                    color: 'var(--kvm-text-muted)',
                  }}
                  aria-label="通知消息"
                  aria-haspopup="dialog"
                  aria-expanded={notificationOpen}
                >
                  <BellIcon size={17} />
                  {unreadCount > 0 && (
                    <span
                      className="absolute -right-1 -top-1 flex h-5 min-w-5 items-center justify-center rounded-full px-1 text-[11px] font-semibold"
                      style={{ background: '#ef4444', color: '#fff', border: '2px solid var(--kvm-header)' }}
                    >
                      {unreadCount > 99 ? '99+' : unreadCount}
                    </span>
                  )}
                </button>
              </KvmTooltip>
              {notificationOpen && typeof document !== 'undefined' ? createPortal(
                <NotificationPanel
                  ref={notificationPanelRef}
                  items={notifications}
                  unreadCount={unreadCount}
                  position={notificationPanelPosition}
                  onReadAll={() => void handleReadAll()}
                  onClear={() => void handleClearNotifications()}
                  onOpen={item => void handleNotificationClick(item)}
                  onClose={() => setNotificationOpen(false)}
                />,
                document.body
              ) : null}
            </div>
            <KvmTooltip
              label={theme === 'dark' ? '切换浅色背景' : '切换深色背景'}
              placement="bottom"
            >
              <button
                type="button"
                onClick={() => setTheme(toggleKvmTheme)}
                className="kvm-action-button flex h-[42px] w-[42px] items-center justify-center rounded-lg border"
                style={{
                  background: 'var(--kvm-control-bg)',
                  borderColor: 'var(--kvm-border)',
                  color: 'var(--kvm-text-muted)',
                }}
                aria-label={theme === 'dark' ? '切换浅色背景' : '切换深色背景'}
              >
                {theme === 'dark' ? <SunIcon size={17} /> : <MoonIcon size={17} />}
              </button>
            </KvmTooltip>
            <div ref={userMenuRef} className="relative">
              <KvmTooltip
                label={displayName}
                placement="bottom"
                align="end"
                className="inline-flex"
              >
                <button
                  type="button"
                  onClick={() => setUserMenuOpen(value => !value)}
                  className="kvm-action-button grid h-[42px] w-[150px] grid-cols-[26px_minmax(0,1fr)_14px] items-center gap-2 rounded-lg px-3 py-1.5"
                  style={{
                    background: 'var(--kvm-control-bg)',
                    border: '1px solid var(--kvm-border)',
                    color: 'var(--kvm-text)',
                  }}
                  aria-haspopup="menu"
                  aria-expanded={userMenuOpen}
                >
                  <div
                    className="flex items-center justify-center rounded-full"
                    style={{
                      width: '26px',
                      height: '26px',
                      background: 'linear-gradient(135deg, #3b82f6, #06b6d4)',
                    }}
                  >
                    <UserIcon size={14} color="#fff" />
                  </div>
                  <span className="min-w-0 truncate text-sm">
                    {displayName}
                  </span>
                  <ChevronDownIcon
                    size={14}
                    className={
                      userMenuOpen ? 'rotate-180 transition-transform' : 'transition-transform'
                    }
                    style={{ color: 'var(--kvm-text-muted)' }}
                  />
                </button>
              </KvmTooltip>

              {userMenuOpen && (
                <div
                  className="absolute right-0 top-[calc(100%+8px)] z-[1300] w-full rounded-lg p-2 shadow-2xl"
                  role="menu"
                  style={{
                    background: 'var(--kvm-menu-bg)',
                    border: '1px solid var(--kvm-border)',
                    boxShadow: 'var(--kvm-menu-shadow)',
                  }}
                >
                  <button
                    type="button"
                    className="kvm-action-button flex h-10 w-full items-center gap-2 rounded-lg px-3 text-left text-sm"
                    role="menuitem"
                    onClick={() => {
                      setUserMenuOpen(false);
                      setPasswordDialogOpen(true);
                    }}
                    style={{ color: 'var(--kvm-text)', background: 'transparent' }}
                  >
                    <KeyRoundIcon size={16} />
                    修改密码
                  </button>
                  <button
                    type="button"
                    className="kvm-action-button kvm-danger-button flex h-10 w-full items-center gap-2 rounded-lg px-3 text-left text-sm"
                    role="menuitem"
                    onClick={() => void handleLogout()}
                    style={{ color: 'var(--kvm-text)', background: 'transparent' }}
                  >
                    <LogOutIcon size={16} />
                    退出系统
                  </button>
                </div>
              )}
            </div>
          </div>
        </header>

        <main
          className="flex-1 overflow-y-auto kvm-hidden-scrollbar"
          style={{ background: 'var(--kvm-bg)' }}
          data-px-slot
        >
          {visibleNavItems.length === 0 ? (
            <div className="flex h-full items-center justify-center px-6 text-sm" style={{ color: 'var(--kvm-text-muted)' }}>
              暂无可访问的功能
            </div>
          ) : (
            <div key={location.pathname} className="kvm-page-reveal h-full min-h-full">
              <Outlet />
            </div>
          )}
        </main>
      </div>
      <PasswordDialog
        open={passwordDialogOpen}
        onClose={() => setPasswordDialogOpen(false)}
        onSuccess={handlePasswordChanged}
      />
    </div>
  );
}

function parseResourceUpdateEvent(data: string) {
  try {
    const event = JSON.parse(data) as { payload?: { agentId?: unknown; name?: unknown; pool?: unknown } };
    return {
      agentId: typeof event.payload?.agentId === 'string' ? event.payload.agentId : '',
      name: typeof event.payload?.name === 'string' ? event.payload.name : undefined,
      pool: typeof event.payload?.pool === 'string' ? event.payload.pool : undefined,
    };
  } catch {
    return { agentId: '' };
  }
}
