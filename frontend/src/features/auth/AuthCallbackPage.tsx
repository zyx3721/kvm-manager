import { useEffect, useMemo, useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { AlertTriangleIcon, Loader2Icon } from 'lucide-react';
import { persistSession, type AuthUser } from '../../lib/auth';
import { applyKvmTheme, getInitialKvmTheme, persistKvmTheme, type KvmTheme } from '../../lib/utils';
type MeResponse = {
  user: AuthUser;
  expires_at: string;
};

/** 解析后端 302 回跳时写入 URL fragment 的登录结果（token 不经服务端日志与 Referer） */
function parseCallbackResult() {
  const params = new URLSearchParams(window.location.hash.replace(/^#/, ''));
  const redirect = sanitizeLocalPath(params.get('redirect') || '/');
  const token = params.get('token') || '';
  const error = params.get('error') || '';
  const message = params.get('message') || '';
  return { token, redirect, error, message };
}

function sanitizeLocalPath(value: string) {
  if (!value.startsWith('/') || value.startsWith('//') || value.startsWith('/\\')) return '/';
  return value;
}

export default function AuthCallbackPage() {
  const navigate = useNavigate();
  const result = useMemo(parseCallbackResult, []);
  const [theme] = useState<KvmTheme>(getInitialKvmTheme);
  const [pending, setPending] = useState(Boolean(result.token));
  const [error, setError] = useState(
    result.token ? '' : result.message || '登录失败，请重新发起登录'
  );

  useEffect(() => {
    applyKvmTheme(theme);
    persistKvmTheme(theme);
  }, [theme]);

  useEffect(() => {
    if (!result.token) return;
    let cancelled = false;
    void (async () => {
      try {
        const response = await fetch('/api/auth/me', {
          headers: { Authorization: `Bearer ${result.token}` },
        });
        if (!response.ok) throw new Error(`me failed: ${response.status}`);
        const data = (await response.json()) as MeResponse;
        if (cancelled) return;
        persistSession({ token: result.token, expires_at: data.expires_at, user: data.user });
        navigate(result.redirect, { replace: true });
      } catch {
        if (!cancelled) {
          setPending(false);
          setError('登录会话获取失败，请重新发起登录');
        }
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [navigate, result]);

  return (
    <main
      data-cmp="AuthCallback"
      className="relative flex min-h-dvh items-center justify-center overflow-hidden px-4 py-8 sm:px-6"
      style={{
        background:
          'radial-gradient(circle at 50% 0%, rgba(59,130,246,0.24), transparent 30%), radial-gradient(circle at 12% 22%, rgba(6,182,212,0.18), transparent 28%), radial-gradient(circle at 88% 82%, rgba(16,185,129,0.14), transparent 30%), var(--kvm-login-bg)',
        color: 'var(--kvm-text)',
      }}
    >
      <section
        className="kvm-login-frame kvm-login-reveal w-full max-w-[420px] rounded-[24px] p-1"
        role={pending ? 'status' : 'alert'}
        aria-live="polite"
      >
        <div
          className="flex flex-col items-center gap-4 rounded-[20px] px-8 py-10 text-center"
          style={{
            background: 'var(--kvm-login-panel-bg)',
            border: '1px solid var(--kvm-border)',
            backdropFilter: 'blur(18px)',
            boxShadow: 'var(--kvm-login-panel-shadow)',
          }}
        >
          {pending ? (
            <>
              <Loader2Icon
                className="animate-spin"
                size={30}
                style={{ color: 'var(--kvm-accent-text)' }}
              />
              <p className="text-sm font-medium">企业微信登录成功，正在进入平台</p>
              <p className="text-xs" style={{ color: 'var(--kvm-text-muted)' }}>
                请稍候，正在建立登录会话
              </p>
            </>
          ) : (
            <>
              <div
                className="flex h-12 w-12 items-center justify-center rounded-full"
                style={{
                  background: 'rgba(239,68,68,0.12)',
                  border: '1px solid rgba(239,68,68,0.28)',
                }}
              >
                <AlertTriangleIcon size={22} style={{ color: '#f87171' }} />
              </div>
              <p className="text-sm font-medium">企业微信登录未完成</p>
              <p
                className="rounded-xl px-3 py-2 text-xs leading-5"
                style={{
                  background: 'rgba(239,68,68,0.12)',
                  border: '1px solid rgba(239,68,68,0.28)',
                  color: '#fca5a5',
                }}
              >
                {error}
              </p>
              <Link
                to="/login"
                className="kvm-action-button flex h-11 w-full items-center justify-center rounded-2xl text-sm font-semibold"
                style={{
                  background: 'linear-gradient(135deg, #2563eb, #06b6d4)',
                  color: '#fff',
                  boxShadow: '0 18px 48px rgba(37,99,235,0.35)',
                }}
              >
                返回登录
              </Link>
            </>
          )}
        </div>
      </section>
    </main>
  );
}
