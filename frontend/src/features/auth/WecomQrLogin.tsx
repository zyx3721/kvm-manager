import { Loader2Icon } from 'lucide-react';
import { useCallback, useEffect, useRef, useState } from 'react';
import { fetchWecomAuthorize, type WecomAuthorizeEmbed } from '../../lib/auth';

/** WecomEmbedSuccess 内嵌二维码扫码确认后交由父页面完成登录的授权参数 */
export type WecomEmbedSuccess = {
  authMode: 'direct' | 'sso';
  code?: string;
  state?: string;
  ticket?: string;
};

type WecomQrLoginProps = {
  onSuccess: (payload: WecomEmbedSuccess) => void;
};

type WecomDirectPanelParams = {
  appid: string;
  agentid?: string;
  redirectUri: string;
  state: string;
};

/** parseDirectPanelParams 从 authorize 下发的 wwlogin 链接解析官方登录面板参数，后端接口无需扩展 */
function parseDirectPanelParams(embed: WecomAuthorizeEmbed): WecomDirectPanelParams | null {
  let url: URL;
  try {
    url = new URL(embed.iframe_url);
  } catch {
    return null;
  }
  const appid = url.searchParams.get('appid') ?? '';
  const redirectUri = url.searchParams.get('redirect_uri') ?? '';
  if (!appid || !redirectUri) return null;
  return {
    appid,
    agentid: url.searchParams.get('agentid') ?? undefined,
    redirectUri,
    state: embed.state ?? url.searchParams.get('state') ?? '',
  };
}

/** WecomQrLogin 内嵌企业微信扫码登录：直连模式渲染官方登录面板回调返回 code（登录失败由面板内企微错误页展示），SSO 模式内嵌认证中心页面回跳后同源读取参数；加载类失败在组件内展示提示，不做整页跳转降级 */
export function WecomQrLogin({ onSuccess }: WecomQrLoginProps) {
  const [embed, setEmbed] = useState<WecomAuthorizeEmbed | null>(null);
  const [loadError, setLoadError] = useState('');
  const iframeRef = useRef<HTMLIFrameElement | null>(null);
  const panelHostRef = useRef<HTMLDivElement | null>(null);
  const handledRef = useRef(false);
  const onSuccessRef = useRef(onSuccess);
  onSuccessRef.current = onSuccess;

  useEffect(() => {
    let cancelled = false;
    fetchWecomAuthorize()
      .then(response => {
        if (cancelled) return;
        if (!response.embed) {
          setLoadError('企业微信登录暂不可用，请检查认证配置');
          return;
        }
        setEmbed(response.embed);
      })
      .catch(() => {
        if (!cancelled) setLoadError('企业微信登录加载失败，请刷新页面重试');
      });
    return () => {
      cancelled = true;
    };
  }, []);

  const handleIframeLoad = useCallback(() => {
    if (handledRef.current || !embed) return;
    const frame = iframeRef.current;
    if (!frame) return;
    let query: string;
    let legacyTicket = '';
    try {
      const frameLocation = frame.contentWindow?.location;
      if (!frameLocation) return;
      if (frameLocation.pathname === '/login' && frameLocation.search) {
        query = frameLocation.search;
        legacyTicket = new URLSearchParams(query).get('ticket') ?? '';
        if (!legacyTicket) return;
      } else if (frameLocation.pathname !== embed.callback_path) {
        return;
      }
    } catch {
      return;
    }
    const params = new URLSearchParams(query);
    handledRef.current = true;
    onSuccessRef.current({
      authMode: 'sso',
      ticket: params.get('ticket') ?? legacyTicket,
    });
  }, [embed]);

  useEffect(() => {
    if (!embed || embed.auth_mode !== 'direct') return;
    const host = panelHostRef.current;
    if (!host) return;
    const panelParams = parseDirectPanelParams(embed);
    if (!panelParams) {
      setLoadError('企业微信登录面板加载失败，请刷新页面重试');
      return;
    }
    let cancelled = false;
    let instance: { unmount(): void } | null = null;
    void (async () => {
      try {
        const ww = await import('@wecom/jssdk');
        if (cancelled) return;
        instance = ww.createWWLoginPanel({
          el: host,
          params: {
            login_type: ww.WWLoginType.corpApp,
            appid: panelParams.appid,
            agentid: panelParams.agentid,
            redirect_uri: panelParams.redirectUri,
            state: panelParams.state,
            redirect_type: ww.WWLoginRedirectType.callback,
            panel_size: ww.WWLoginPanelSizeType.small,
            color_scheme:
              document.documentElement.dataset.kvmTheme === 'light'
                ? ww.ColorScheme.Light
                : ww.ColorScheme.Dark,
            lang: ww.WWLoginLangType.zh,
          },
          onLoginSuccess: ({ code }) => {
            if (cancelled || handledRef.current) return;
            handledRef.current = true;
            onSuccessRef.current({
              authMode: 'direct',
              code,
              state: panelParams.state,
            });
          },
        });
      } catch {
        instance = null;
      }
      if (!cancelled && !instance) {
        setLoadError('企业微信登录面板加载失败，请刷新页面重试');
      }
    })();
    return () => {
      cancelled = true;
      instance?.unmount();
    };
  }, [embed]);

  if (loadError) {
    return (
      <div
        className="flex h-[420px] w-full flex-col items-center justify-center gap-3 rounded-2xl px-6 text-center"
        style={{ background: 'var(--kvm-control-bg)', border: '1px solid var(--kvm-border)' }}
      >
        <p className="text-sm font-medium" style={{ color: 'var(--kvm-text)' }}>
          {loadError}
        </p>
        <p className="text-xs" style={{ color: 'var(--kvm-text-muted)' }}>
          企业微信侧的具体错误会展示在扫码面板中
        </p>
      </div>
    );
  }

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

  if (embed.auth_mode === 'direct') {
    return (
      <div
        ref={panelHostRef}
        className="flex w-full justify-center rounded-2xl"
        style={{ minHeight: 380 }}
      />
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
