import React, { useEffect, useState } from 'react';
import { BrowserRouter, Navigate, Route, Routes, useLocation, useNavigate } from 'react-router-dom';
import {
  AlertTriangleIcon,
  CheckCircle2Icon,
  InfoIcon,
  Loader2Icon,
  XCircleIcon,
} from 'lucide-react';
import { Toaster, toast } from 'sonner';
import BootScreen from '../components/boot/BootScreen';
import KvmLayout from '../components/layout/KvmLayout';
import ForgotPasswordPage from '../features/auth/ForgotPasswordPage';
import Login from '../features/auth/LoginPage';
import Dashboard from '../features/dashboard/DashboardPage';
import HostInterfaces from '../features/host-interfaces/HostInterfacesPage';
import Hosts from '../features/hosts/HostsPage';
import NetworkPools from '../features/network-pools/NetworkPoolsPage';
import Operations from '../features/operations/OperationsPage';
import Settings from '../features/settings/SettingsPage';
import Snapshots from '../features/snapshots/SnapshotsPage';
import StoragePools from '../features/storage-pools/StoragePoolsPage';
import VMs from '../features/vms/VMsPage';
import {
  clearSession,
  getStoredUser,
  isAuthenticated,
  isSessionIdleExpired,
  markSessionActivity,
  userHasAnyPermission,
} from '../lib/auth';
import { applyKvmTheme, getInitialKvmTheme } from '../lib/utils';

applyKvmTheme(getInitialKvmTheme());

class ErrorBoundary extends React.Component<{ children: React.ReactNode }, { hasError: boolean }> {
  state = { hasError: false };

  static getDerivedStateFromError() {
    return { hasError: true };
  }

  componentDidCatch(error: Error) {
    console.log('ErrorBoundary caught:', error);
    toast.error('页面发生错误，请刷新重试');
  }

  render() {
    if (this.state.hasError) {
      return (
        <div
          className="flex h-screen items-center justify-center text-sm"
          style={{ color: 'var(--kvm-text-muted)', background: 'var(--kvm-bg)' }}
        >
          页面出现错误，请刷新重试
        </div>
      );
    }
    return this.props.children;
  }
}

function RequireAuth() {
  const location = useLocation();
  const user = getStoredUser();

  if (isAuthenticated() && isSessionIdleExpired()) {
    clearSession();
    toast.info('长时间未操作，请重新登录');
  }

  if (!isAuthenticated()) {
    return <Navigate to="/login" replace state={{ from: location }} />;
  }

  const routePermissions: Record<string, string[]> = {
    '/': ['dashboard.read'],
    '/vms': ['vms.read'],
    '/hosts': ['hosts.read', 'agents.read'],
    '/host-interfaces': ['host.interfaces.read'],
    '/storage-pools': ['storage.read'],
    '/network-pools': ['network.read'],
    '/snapshots': ['snapshots.read'],
    '/operations': ['operations.read'],
    '/settings': [
      'settings.base.read',
      'settings.base.manage',
      'settings.users.read',
      'settings.users.manage',
      'settings.auth.read',
      'settings.auth.manage',
      'settings.notifications.read',
      'settings.notifications.manage',
    ],
  };
  const permissions = routePermissions[location.pathname];
  if (permissions && !userHasAnyPermission(user, permissions)) {
    const fallback = Object.entries(routePermissions).find(([, required]) =>
      userHasAnyPermission(user, required)
    )?.[0];
    if (fallback) return <Navigate to={fallback} replace />;
  }

  return <KvmLayout />;
}

function SessionIdleGuard() {
  const location = useLocation();
  const navigate = useNavigate();

  useEffect(() => {
    if (!isAuthenticated()) return;

    const expireIdleSession = () => {
      clearSession();
      toast.info('长时间未操作，请重新登录');
      if (window.location.pathname !== '/login') {
        navigate('/login', { replace: true, state: { from: location } });
      }
    };
    const checkIdleSession = () => {
      if (isAuthenticated() && isSessionIdleExpired()) {
        expireIdleSession();
      }
    };
    const handleActivity = () => {
      if (!isAuthenticated()) return;
      if (isSessionIdleExpired()) {
        expireIdleSession();
        return;
      }
      markSessionActivity();
    };
    const activityEvents = ['click', 'keydown', 'pointerdown', 'touchstart'] as const;
    const activityListenerOptions = { capture: true, passive: true };

    if (isSessionIdleExpired()) {
      expireIdleSession();
      return;
    } else {
      markSessionActivity();
    }
    activityEvents.forEach(event =>
      window.addEventListener(event, handleActivity, activityListenerOptions)
    );
    window.addEventListener('focus', checkIdleSession);
    document.addEventListener('visibilitychange', checkIdleSession);
    const timer = window.setInterval(checkIdleSession, 60000);

    return () => {
      activityEvents.forEach(event =>
        window.removeEventListener(event, handleActivity, activityListenerOptions)
      );
      window.removeEventListener('focus', checkIdleSession);
      document.removeEventListener('visibilitychange', checkIdleSession);
      window.clearInterval(timer);
    };
  }, [location, navigate]);

  return null;
}

function AppRoutes() {
  return (
    <Routes>
      <Route path="/login" element={<Login />} />
      <Route path="/forgot-password" element={<ForgotPasswordPage />} />
      <Route element={<RequireAuth />}>
        <Route path="/" element={<Dashboard />} />
        <Route path="/vms" element={<VMs />} />
        <Route path="/hosts" element={<Hosts />} />
        <Route path="/host-interfaces" element={<HostInterfaces />} />
        <Route path="/storage-pools" element={<StoragePools />} />
        <Route path="/network-pools" element={<NetworkPools />} />
        <Route path="/snapshots" element={<Snapshots />} />
        <Route path="/operations" element={<Operations />} />
        <Route path="/settings" element={<Settings />} />
      </Route>
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  );
}

const App = () => {
  const [booting, setBooting] = useState(true);

  useEffect(() => {
    const timer = window.setTimeout(() => setBooting(false), 1000);
    return () => window.clearTimeout(timer);
  }, []);

  return (
    <BrowserRouter>
      <ErrorBoundary>
        <SessionIdleGuard />
        {booting ? <BootScreen /> : <AppRoutes />}
      </ErrorBoundary>
      <Toaster
        position="top-right"
        theme="system"
        closeButton
        expand
        visibleToasts={4}
        gap={10}
        duration={3000}
        offset={{ top: 28, right: 20 }}
        mobileOffset={{ top: 16, right: 12, left: 12 }}
        icons={{
          success: <CheckCircle2Icon size={18} />,
          error: <XCircleIcon size={18} />,
          warning: <AlertTriangleIcon size={18} />,
          info: <InfoIcon size={18} />,
          loading: <Loader2Icon className="kvm-toast-spin" size={18} />,
        }}
        toastOptions={{
          classNames: {
            toast: 'kvm-toast',
            title: 'kvm-toast-title',
            description: 'kvm-toast-description',
            closeButton: 'kvm-toast-close',
          },
        }}
      />
    </BrowserRouter>
  );
};

export default App;
