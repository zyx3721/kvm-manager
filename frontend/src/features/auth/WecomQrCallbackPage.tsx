import { useEffect } from 'react';
import { Loader2Icon } from 'lucide-react';

/** WecomQrCallbackPage 企微扫码中转页：iframe 内静默占位由父页面接管，顶层整页回跳转发登录页现有回调逻辑 */
export default function WecomQrCallbackPage() {
  const embedded = window.self !== window.top;

  useEffect(() => {
    if (!embedded) {
      window.location.replace('/login' + window.location.search);
    }
  }, [embedded]);

  return (
    <main
      data-cmp="WecomQrCallback"
      className="relative flex min-h-dvh items-center justify-center overflow-hidden px-4 py-8 sm:px-6"
      style={{
        background: 'var(--kvm-login-bg)',
        color: 'var(--kvm-text)',
      }}
    >
      <section
        className="kvm-login-frame w-full max-w-[420px] rounded-[24px] p-1"
        role="status"
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
          <Loader2Icon
            className="animate-spin"
            size={30}
            style={{ color: 'var(--kvm-accent-text)' }}
          />
          <p className="text-sm font-medium">
            {embedded ? '扫码成功，正在登录' : '企业微信授权回跳处理中'}
          </p>
          <p className="text-xs" style={{ color: 'var(--kvm-text-muted)' }}>
            {embedded ? '请勿关闭当前页面' : '即将返回登录页继续'}
          </p>
        </div>
      </section>
    </main>
  );
}
