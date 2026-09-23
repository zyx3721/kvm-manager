import { Loader2Icon } from 'lucide-react';
import { useCallback, useEffect, useRef, useState } from 'react';
import { fetchWecomAuthorize, type WecomAuthorizeEmbed } from '../../lib/auth';

type WecomQrLoginProps = {
  onSuccess: (token: string) => void;
  onError: (message: string) => void;
  onFallback: () => void;
};

/** parseFrameResult 从 iframe 内回跳地址的 fragment 解析后端回传的登录结果 */
function parseFrameResult(hash: string) {
  const params = new URLSearchParams(hash.replace(/^#/, ''));
  return { token: params.get('token') || '', message: params.get('message') || '' };
}

/** WecomQrLogin 内嵌企业微信扫码二维码：iframe 加载扫码页，iframe 内回跳中转路由时同源读取后端回传的登录结果 */
export function WecomQrLogin({ onSuccess, onError, onFallback }: WecomQrLoginProps) {
  const [embed, setEmbed] = useState<WecomAuthorizeEmbed | null>(null);
  const iframeRef = useRef<HTMLIFrameElement | null>(null);
  const handledRef = useRef(false);
  const onSuccessRef = useRef(onSuccess);
  const onErrorRef = useRef(onError);
  const onFallbackRef = useRef(onFallback);
  onSuccessRef.current = onSuccess;
  onErrorRef.current = onError;
  onFallbackRef.current = onFallback;

  useEffect(() => {
    let cancelled = false;
    fetchWecomAuthorize()
      .then(response => {
        if (cancelled) return;
        if (!response.embed) {
          onFallbackRef.current();
          return;
        }
        setEmbed(response.embed);
      })
      .catch(() => {
        if (!cancelled) onFallbackRef.current();
      });
    return () => {
      cancelled = true;
    };
  }, []);

  const handleIframeLoad = useCallback(() => {
    if (handledRef.current || !embed) return;
    const frame = iframeRef.current;
    if (!frame) return;
    let hash: string;
    try {
      const frameLocation = frame.contentWindow?.location;
      if (!frameLocation) return;
      if (
        frameLocation.pathname !== embed.callback_path &&
        frameLocation.pathname !== '/auth/callback'
      ) {
        return;
      }
      hash = frameLocation.hash;
    } catch {
      return;
    }
    const result = parseFrameResult(hash);
    if (!result.token && !result.message) return;
    handledRef.current = true;
    if (result.token) {
      onSuccessRef.current(result.token);
      return;
    }
    onErrorRef.current(result.message || '企业微信登录失败，请重新扫码');
  }, [embed]);

  if (!embed) {
    return (
      <div
        className="flex h-[420px] w-full items-center justify-center rounded-2xl"
        style={{ background: 'var(--kvm-control-bg)', border: '1px solid var(--kvm-border)' }}
      >
        <Loader2Icon
          size={24}
          className="animate-spin"
          style={{ color: 'var(--kvm-accent-text)' }}
        />
      </div>
    );
  }

  return (
    <iframe
      ref={iframeRef}
      src={embed.iframe_url}
      title="企业微信扫码登录"
      onLoad={handleIframeLoad}
      className="w-full rounded-2xl"
      style={{ height: 420, border: '1px solid var(--kvm-border)', background: '#ffffff' }}
    />
  );
}
