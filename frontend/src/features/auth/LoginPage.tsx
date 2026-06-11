import React, { type FormEvent, useEffect, useMemo, useRef, useState } from 'react';
import { Link, useLocation, useNavigate } from 'react-router-dom';
import { toast } from 'sonner';
import {
  CheckIcon,
  ChevronDownIcon,
  EyeIcon,
  EyeOffIcon,
  LockIcon,
  MoonIcon,
  SunIcon,
  UserIcon,
} from 'lucide-react';
import { isAuthenticated, login as loginRequest } from '../../lib/auth';
import { KvmTooltip } from '../../components/kvm/StatusBadge';
import { fetchPublicAuthProviders, type PublicAuthProvider } from '../../lib/api';
import { useBaseConfig } from '../../lib/branding';
import {
  applyKvmTheme,
  getInitialKvmTheme,
  persistKvmTheme,
  toggleKvmTheme,
  type KvmTheme,
} from '../../lib/utils';

export default function Login() {
  const navigate = useNavigate();
  const location = useLocation();
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [provider, setProvider] = useState('local');
  const [providers, setProviders] = useState<PublicAuthProvider[]>([]);
  const [showPassword, setShowPassword] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [theme, setTheme] = useState<KvmTheme>(getInitialKvmTheme);
  const baseConfig = useBaseConfig();

  const redirectPath = useMemo(() => {
    const state = location.state as { from?: { pathname?: string } } | null;
    return state?.from?.pathname || '/';
  }, [location.state]);

  useEffect(() => {
    applyKvmTheme(theme);
    persistKvmTheme(theme);
  }, [theme]);

  useEffect(() => {
    if (isAuthenticated()) {
      navigate(redirectPath, { replace: true });
    }
  }, [navigate, redirectPath]);

  useEffect(() => {
    void fetchPublicAuthProviders()
      .then(response => setProviders(response.items))
      .catch(() => setProviders([]));
  }, []);

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const normalizedUsername = username.trim();

    if (!normalizedUsername || !password) {
      setError('用户名或密码不能为空');
      return;
    }

    setLoading(true);
    setError('');

    try {
      const session = await loginRequest(normalizedUsername, password, provider);
      toast.success(`欢迎回来，${session.user.displayName || session.user.username}`);
      navigate(redirectPath, { replace: true });
    } catch (err) {
      const message = err instanceof Error ? err.message : '登录失败，请稍后重试';
      setError(message);
    } finally {
      setLoading(false);
    }
  }

  return (
    <main
      data-cmp="Login"
      className="relative flex min-h-dvh items-center justify-center overflow-hidden px-4 py-8 sm:px-6"
      style={{
        background:
          'radial-gradient(circle at 50% 0%, rgba(59,130,246,0.24), transparent 30%), radial-gradient(circle at 12% 22%, rgba(6,182,212,0.18), transparent 28%), radial-gradient(circle at 88% 82%, rgba(16,185,129,0.14), transparent 30%), var(--kvm-login-bg)',
        color: 'var(--kvm-text)',
      }}
    >
      <KvmTooltip
        label={theme === 'dark' ? '切换浅色背景' : '切换深色背景'}
        placement="bottom"
        className="absolute right-5 top-5 z-20 sm:right-6 sm:top-6"
      >
        <button
          type="button"
          onClick={() => setTheme(toggleKvmTheme)}
          className="kvm-action-button flex h-11 w-11 items-center justify-center rounded-lg border"
          style={{
            background: 'var(--kvm-control-bg)',
            borderColor: 'var(--kvm-border)',
            color: 'var(--kvm-text-muted)',
            boxShadow: 'var(--kvm-menu-shadow)',
          }}
          aria-label={theme === 'dark' ? '切换浅色背景' : '切换深色背景'}
        >
          {theme === 'dark' ? <SunIcon size={18} /> : <MoonIcon size={18} />}
        </button>
      </KvmTooltip>
      <div
        className="kvm-login-orb absolute left-[-6rem] top-20 h-64 w-64 rounded-full"
        aria-hidden="true"
      />
      <div
        className="kvm-login-orb absolute bottom-[-4rem] right-[-5rem] h-80 w-80 rounded-full"
        aria-hidden="true"
      />

      <section className="relative z-10 flex w-full max-w-[460px] flex-col items-center gap-5">
        <div className="kvm-login-reveal flex flex-col items-center text-center">
          <img
            className="mb-4 h-20 w-20 drop-shadow-[0_18px_42px_rgba(6,182,212,0.18)]"
            src={baseConfig.iconData}
            alt={baseConfig.loginName}
          />
          <p className="kvm-gradient-text text-2xl font-bold tracking-wide">
            {baseConfig.loginName}
          </p>
          <p
            className="mt-1 text-xs uppercase tracking-[0.28em]"
            style={{ color: 'var(--kvm-text-muted)' }}
          >
            Infrastructure Control Plane
          </p>
        </div>

        <section
          className="kvm-login-frame kvm-login-reveal w-full rounded-[28px] p-1"
          style={{ animationDelay: '80ms' }}
        >
          <div
            className="rounded-[24px] p-6 sm:p-8"
            style={{
              background: 'var(--kvm-login-panel-bg)',
              border: '1px solid var(--kvm-border)',
              backdropFilter: 'blur(18px)',
              boxShadow: 'var(--kvm-login-panel-shadow)',
            }}
          >
            <div className="mb-7 text-center">
              <h1
                className="text-2xl font-bold"
                style={{ color: 'var(--kvm-text)', textShadow: '0 0 22px rgba(6,182,212,0.16)' }}
              >
                欢迎回来
              </h1>
              <p className="mt-2 text-sm" style={{ color: 'var(--kvm-text-muted)' }}>
                登录您的账户以继续
              </p>
            </div>

            <form className="space-y-5" onSubmit={handleSubmit} noValidate>
              {providers.length > 0 && (
                <LoginProviderSelect
                  value={provider}
                  providers={providers}
                  onChange={setProvider}
                />
              )}
              <div>
                <label className="mb-2 block text-sm font-medium" htmlFor="username">
                  用户名
                </label>
                <div className="relative">
                  <UserIcon
                    className="absolute left-4 top-1/2 -translate-y-1/2"
                    size={17}
                    style={{ color: 'var(--kvm-text-muted)' }}
                    aria-hidden="true"
                  />
                  <input
                    id="username"
                    name="username"
                    autoComplete="username"
                    placeholder="请输入用户名"
                    value={username}
                    onChange={event => setUsername(event.target.value)}
                    className="h-12 w-full rounded-2xl py-3 pl-12 pr-4 text-sm outline-none transition-all"
                    style={{
                      background: 'var(--kvm-control-bg)',
                      border: '1px solid var(--kvm-border)',
                      color: 'var(--kvm-text)',
                    }}
                    aria-invalid={Boolean(error)}
                  />
                </div>
              </div>

              <div>
                <div className="mb-2 flex items-center justify-between gap-3">
                  <label className="block text-sm font-medium" htmlFor="password">
                    密码
                  </label>
                  <Link
                    to="/forgot-password"
                    className="text-xs font-semibold transition-colors"
                    style={{ color: 'var(--kvm-accent-text)' }}
                  >
                    忘记密码?
                  </Link>
                </div>
                <div className="relative">
                  <LockIcon
                    className="absolute left-4 top-1/2 -translate-y-1/2"
                    size={17}
                    style={{ color: 'var(--kvm-text-muted)' }}
                    aria-hidden="true"
                  />
                  <input
                    id="password"
                    name="password"
                    type={showPassword ? 'text' : 'password'}
                    autoComplete="current-password"
                    placeholder="请输入密码"
                    value={password}
                    onChange={event => setPassword(event.target.value)}
                    className="h-12 w-full rounded-2xl py-3 pl-12 pr-12 text-sm outline-none transition-all"
                    style={{
                      background: 'var(--kvm-control-bg)',
                      border: '1px solid var(--kvm-border)',
                      color: 'var(--kvm-text)',
                    }}
                    aria-invalid={Boolean(error)}
                    aria-describedby="login-error"
                  />
                  <button
                    type="button"
                    onClick={() => setShowPassword(value => !value)}
                    className="absolute right-2 top-1/2 flex h-9 w-9 -translate-y-1/2 items-center justify-center rounded-xl transition-colors"
                    style={{ color: 'var(--kvm-text-muted)' }}
                    aria-label={showPassword ? '隐藏密码' : '显示密码'}
                  >
                    {showPassword ? <EyeOffIcon size={17} /> : <EyeIcon size={17} />}
                  </button>
                </div>
              </div>

              <div aria-live="polite" className="min-h-6">
                {error && (
                  <p
                    id="login-error"
                    role="alert"
                    className="rounded-xl px-3 py-2 text-xs"
                    style={{
                      background: 'rgba(239,68,68,0.12)',
                      border: '1px solid rgba(239,68,68,0.28)',
                      color: '#fca5a5',
                    }}
                  >
                    {error}
                  </p>
                )}
              </div>

              <button
                type="submit"
                disabled={loading}
                className="kvm-login-submit flex h-12 w-full items-center justify-center gap-2 rounded-2xl text-sm font-semibold transition-all disabled:cursor-not-allowed disabled:opacity-60"
                style={{
                  background: 'linear-gradient(135deg, #2563eb, #06b6d4)',
                  color: '#fff',
                  boxShadow: '0 18px 48px rgba(37,99,235,0.35)',
                }}
              >
                {loading && (
                  <span
                    className="h-4 w-4 animate-spin rounded-full border-2 border-white/30 border-t-white"
                    aria-hidden="true"
                  />
                )}
                {loading ? '登录中...' : '登录'}
              </button>
            </form>
          </div>
        </section>

        <p
          className="kvm-login-reveal text-center text-xs"
          style={{ color: 'var(--kvm-text-muted)', animationDelay: '140ms' }}
        >
          (c) 2026 {baseConfig.siteName}. Secure virtualization operations console.
        </p>
      </section>
    </main>
  );
}

function LoginProviderSelect({
  value,
  providers,
  onChange,
}: {
  value: string;
  providers: PublicAuthProvider[];
  onChange: (value: string) => void;
}) {
  const [open, setOpen] = useState(false);
  const rootRef = useRef<HTMLDivElement | null>(null);
  const options = [
    { id: 'local', name: '本地账号' },
    ...providers.map(item => ({ id: item.id, name: item.name })),
  ];
  const selected = options.find(item => item.id === value) ?? options[0];

  useEffect(() => {
    function handlePointerDown(event: MouseEvent) {
      if (!rootRef.current?.contains(event.target as Node)) {
        setOpen(false);
      }
    }
    function handleKeyDown(event: KeyboardEvent) {
      if (event.key === 'Escape') {
        setOpen(false);
      }
    }
    window.addEventListener('mousedown', handlePointerDown);
    window.addEventListener('keydown', handleKeyDown);
    return () => {
      window.removeEventListener('mousedown', handlePointerDown);
      window.removeEventListener('keydown', handleKeyDown);
    };
  }, []);

  return (
    <div ref={rootRef} className="relative">
      <label className="mb-2 block text-sm font-medium" htmlFor="login-provider-button">
        登录方式
      </label>
      <button
        id="login-provider-button"
        type="button"
        onClick={() => setOpen(current => !current)}
        className="kvm-action-button flex h-12 w-full items-center justify-between gap-3 rounded-2xl px-4 text-left text-sm outline-none transition-all"
        style={{
          background: 'var(--kvm-control-bg)',
          border: open ? '1px solid rgba(59,130,246,0.48)' : '1px solid var(--kvm-border)',
          color: 'var(--kvm-text)',
        }}
        aria-haspopup="listbox"
        aria-expanded={open}
      >
        <span className="truncate">{selected.name}</span>
        <ChevronDownIcon
          size={16}
          className={open ? 'rotate-180 transition-transform' : 'transition-transform'}
          style={{ color: 'var(--kvm-text-muted)' }}
        />
      </button>
      {open && (
        <div
          className="absolute left-0 right-0 top-[calc(100%+8px)] z-30 rounded-xl p-1 shadow-2xl"
          role="listbox"
          style={{
            background: 'var(--kvm-menu-bg)',
            border: '1px solid var(--kvm-popover-border)',
            boxShadow: 'var(--kvm-menu-shadow)',
          }}
        >
          {options.map(item => {
            const active = item.id === value;
            return (
              <button
                key={item.id}
                type="button"
                role="option"
                aria-selected={active}
                onClick={() => {
                  onChange(item.id);
                  setOpen(false);
                }}
                className="kvm-action-button flex h-10 w-full items-center justify-between rounded-lg px-3 text-left text-sm"
                style={{
                  background: active ? 'rgba(59,130,246,0.14)' : 'transparent',
                  color: active ? 'var(--kvm-accent-text)' : 'var(--kvm-text)',
                }}
              >
                <span className="truncate">{item.name}</span>
                {active && <CheckIcon size={15} />}
              </button>
            );
          })}
        </div>
      )}
    </div>
  );
}
