import { useEffect, useMemo, useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { AlertTriangleIcon, CheckCircle2Icon, Loader2Icon } from 'lucide-react';
import {
  persistSession,
  updateStoredUser,
  WECOM_BIND_MESSAGE,
  type AuthUser,
} from '../../lib/auth';
import { applyKvmTheme, getInitialKvmTheme, persistKvmTheme, type KvmTheme } from '../../lib/utils';

type MeResponse = {
  user: AuthUser;
  expires_at: string;
};

type CallbackView = 'processing' | 'bind-success' | 'error';

/** 解析后端 302 回跳时写入 URL fragment 的登录/绑定结果（token 不经服务端日志与 Referer） */
function parseCallbackResult() {
  const params = new URLSearchParams(window.location.hash.replace(/^#/, ''));
  return {
    token: params.get('token') || '',
    redirect: sanitizeLocalPath(params.get('redirect') || '/'),
    bind: params.get('bind') || '',
    error: params.get('error') || '',
    message: params.get('message') || '',
  };
}

function sanitizeLocalPath(value: string) {
  if (!value.startsWith('/') || value.startsWith('//') || value.startsWith('/\\')) return '/';
  return value;
}

export default function AuthCallbackPage() {
  const navigate = useNavigate();
  // 回调参数只读取一次存入快照：地址栏随后被清理，重渲染不再随 URL 变化
  const result = useMemo(parseCallbackResult, []);
  const [theme] = useState<KvmTheme>(getInitialKvmTheme);
  const [view, setView] = useState<CallbackView>(() => {
    if (result.token) return 'processing';
    if (result.bind === 'success') return 'bind-success';
    return 'error';
  });
  const [error, setError] = useState(
    result.token ? '' : result.message || '登录失败，请重新发起登录'
  );

  useEffect(() => {
    applyKvmTheme(theme);
    persistKvmTheme(theme);
  }, [theme]);

  // 立即清理地址栏中的授权结果，防止用户按 F5 或回退时重复消费已使用的授权码
  useEffect(() => {
    window.history.replaceState(null, '', window.location.pathname + window.location.search);
  }, []);

  // 绑定弹窗向主窗口通知结果并自动关闭：成功 300ms，失败 1.5s
  const isInPopup = Boolean(window.opener);
  useEffect(() => {
    if (result.bind === 'success') {
      updateStoredUser({ wecomBound: true });
      notifyOpenerAndClose(true);
      return;
    }
    if (!result.token) {
      if (isInPopup && result.error) {
        notifyOpenerAndClose(false, error);
      }
      return;
    }
    let cancelled = false;
    void (async () => {
      try {
        const response = await fetch('/api/auth/me', {
          headers: { Authorization: `Bearer ${result.token}` },
        });
        if (!response.ok) throw new Error(`me failed: ${response.status}`);
        const data = (await response.json()) as MeResponse;
        if (cancelled) return;
        persistSession({
          token: result.token,
          expires_at: data.expires_at,
          wecom_bound: true,
          user: data.user,
        });
        navigate(result.redirect, { replace: true });
      } catch {
        if (!cancelled) {
          setView('error');
          setError('登录会话获取失败，请重新发起登录');
        }
      }
    })();
    return () => {
      cancelled = true;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // bfcache 恢复时回调参数已消费，重置到干净登录页避免展示过期状态
  useEffect(() => {
    const handlePageShow = (event: PageTransitionEvent) => {
      if (event.persisted) window.location.replace('/login');
    };
    window.addEventListener('pageshow', handlePageShow);
    return () => window.removeEventListener('pageshow', handlePageShow);
  }, []);

  function notifyOpenerAndClose(ok: boolean, message = '') {
    try {
      window.opener?.postMessage({ type: WECOM_BIND_MESSAGE, ok, message }, window.location.origin);
    } catch {
      // 主窗口可能已关闭，忽略通知失败
    }
    window.setTimeout(() => window.close(), ok ? 300 : 1500);
  }

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
        role={view === 'processing' ? 'status' : 'alert'}
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
          {view === 'processing' && (
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
          )}
          {view === 'bind-success' && (
            <>
              <div
                className="flex h-12 w-12 items-center justify-center rounded-full"
                style={{
                  background: 'rgba(16,185,129,0.12)',
                  border: '1px solid rgba(16,185,129,0.28)',
                }}
              >
                <CheckCircle2Icon size={22} style={{ color: '#34d399' }} />
              </div>
              <p className="text-sm font-medium">企业微信绑定成功</p>
              <p className="text-xs" style={{ color: 'var(--kvm-text-muted)' }}>
                本窗口将自动关闭，如未关闭请手动关闭
              </p>
              <Link
                to="/"
                className="kvm-action-button flex h-11 w-full items-center justify-center rounded-2xl text-sm font-semibold"
                style={{
                  background: 'linear-gradient(135deg, #2563eb, #06b6d4)',
                  color: '#fff',
                  boxShadow: '0 18px 48px rgba(37,99,235,0.35)',
                }}
              >
                进入平台
              </Link>
            </>
          )}
          {view === 'error' && (
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
              <p className="text-sm font-medium">企业微信认证未完成</p>
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
