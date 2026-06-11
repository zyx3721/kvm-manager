import { useEffect, useRef, useState } from 'react';
import { ChevronDownIcon, Maximize2Icon } from 'lucide-react';
import type { VirtualMachine } from '../../../lib/api';
import { vmConsoleUrl } from '../../../lib/api';
import { KvmTooltip } from '../../../components/kvm/StatusBadge';

export function ConsoleViewer({ vm, password = '' }: { vm: VirtualMachine; password?: string }) {
  const containerRef = useRef<HTMLDivElement | null>(null);
  const frameRef = useRef<HTMLDivElement | null>(null);
  const sendMenuRef = useRef<HTMLDivElement | null>(null);
  const rfbRef = useRef<ConsoleRFB | null>(null);
  const [status, setStatus] = useState('正在连接控制台');
  const [sendMenuOpen, setSendMenuOpen] = useState(false);
  const [fullscreen, setFullscreen] = useState(false);

  useEffect(() => {
    const target = containerRef.current;
    if (!target) return undefined;
    let disposed = false;
    let connected = false;
    let rfb: ConsoleRFB | null = null;
    const timeoutId = window.setTimeout(() => {
      if (!connected) setStatus('控制台连接超时，请检查 VNC 图形端口和 Agent 日志');
    }, 12000);
    const onConnect = () => {
      connected = true;
      window.clearTimeout(timeoutId);
      setStatus('控制台已连接');
    };
    const onDisconnect = (event: Event) => {
      window.clearTimeout(timeoutId);
      const clean = (event as CustomEvent<{ clean?: boolean }>).detail?.clean;
      setStatus(clean ? '控制台已断开' : '控制台连接失败或被远端断开');
    };
    const onCredentials = (event: Event) => {
      window.clearTimeout(timeoutId);
      const types = (event as CustomEvent<{ types?: string[] }>).detail?.types?.join('、');
      setStatus(types ? `VNC 需要凭据：${types}` : 'VNC 需要密码，当前通道尚未支持输入');
    };
    const onSecurityFailure = (event: Event) => {
      window.clearTimeout(timeoutId);
      const detail = (event as CustomEvent<{ reason?: string }>).detail;
      setStatus(detail?.reason ? `VNC 安全协商失败：${detail.reason}` : 'VNC 安全协商失败');
    };

    import('@novnc/novnc')
      .then(module => {
        if (disposed || !containerRef.current) return;
        const trimmedPassword = password.trim();
        rfb = new module.default(containerRef.current, vmConsoleUrl(vm.id), {
          ...(trimmedPassword ? { credentials: { password: trimmedPassword } } : {}),
          wsProtocols: ['binary'],
        });
        rfb.scaleViewport = true;
        rfb.resizeSession = true;
        rfb.viewOnly = false;
        rfb.background = '#050816';
        rfbRef.current = rfb;
        rfb.addEventListener('connect', onConnect);
        rfb.addEventListener('disconnect', onDisconnect);
        rfb.addEventListener('credentialsrequired', onCredentials);
        rfb.addEventListener('securityfailure', onSecurityFailure);
      })
      .catch(() => {
        window.clearTimeout(timeoutId);
        setStatus('控制台组件加载失败');
      });

    return () => {
      disposed = true;
      window.clearTimeout(timeoutId);
      if (rfb) {
        rfb.removeEventListener('connect', onConnect);
        rfb.removeEventListener('disconnect', onDisconnect);
        rfb.removeEventListener('credentialsrequired', onCredentials);
        rfb.removeEventListener('securityfailure', onSecurityFailure);
        rfb.disconnect();
      }
      rfbRef.current = null;
    };
  }, [vm.id, password]);

  useEffect(() => {
    function handleFullscreenChange() {
      setFullscreen(document.fullscreenElement === frameRef.current);
    }
    document.addEventListener('fullscreenchange', handleFullscreenChange);
    return () => document.removeEventListener('fullscreenchange', handleFullscreenChange);
  }, []);

  useEffect(() => {
    function handleConsoleFullscreenKey(event: KeyboardEvent) {
      if (event.key !== 'F11') return;
      event.preventDefault();
      event.stopPropagation();
      toggleFullscreen();
    }
    window.addEventListener('keydown', handleConsoleFullscreenKey, { capture: true });
    return () =>
      window.removeEventListener('keydown', handleConsoleFullscreenKey, { capture: true });
  }, []);

  useEffect(() => {
    if (!sendMenuOpen) return;
    function handlePointerDown(event: PointerEvent) {
      const menu = sendMenuRef.current;
      const target = event.target as Node | null;
      if (!menu || !target || menu.contains(target)) return;
      setSendMenuOpen(false);
    }
    function handleEscape(event: KeyboardEvent) {
      if (event.key === 'Escape') setSendMenuOpen(false);
    }
    window.addEventListener('pointerdown', handlePointerDown, { capture: true });
    window.addEventListener('keydown', handleEscape);
    return () => {
      window.removeEventListener('pointerdown', handlePointerDown, { capture: true });
      window.removeEventListener('keydown', handleEscape);
    };
  }, [sendMenuOpen]);

  function sendKeySequence(sequence: ConsoleKeySequence) {
    const rfb = rfbRef.current;
    if (!rfb) return;
    setSendMenuOpen(false);
    if (sequence.kind === 'ctrl-alt-del') {
      rfb.sendCtrlAltDel();
      return;
    }
    rfb.sendKey(0xffe3, 'ControlLeft', true);
    rfb.sendKey(0xffe9, 'AltLeft', true);
    rfb.sendKey(sequence.keysym, sequence.code, true);
    rfb.sendKey(sequence.keysym, sequence.code, false);
    rfb.sendKey(0xffe9, 'AltLeft', false);
    rfb.sendKey(0xffe3, 'ControlLeft', false);
  }

  function toggleFullscreen() {
    if (document.fullscreenElement === frameRef.current) {
      void document.exitFullscreen?.();
      return;
    }
    void frameRef.current?.requestFullscreen?.();
  }

  return (
    <div
      ref={frameRef}
      className={
        fullscreen ? 'flex h-screen w-screen flex-col space-y-3 bg-[#050816] p-3' : 'space-y-3'
      }
    >
      <div className="flex shrink-0 flex-wrap items-center justify-between gap-3 pr-12">
        <div className="flex min-w-0 flex-1 flex-wrap items-center gap-3">
          <div className="flex items-center gap-2">
            <span
              className="h-2 w-2 rounded-full"
              style={{ background: '#3b82f6', boxShadow: '0 0 18px #3b82f6' }}
            />
            <h2 className="text-base font-semibold" style={{ color: 'var(--kvm-text)' }}>
              控制台
            </h2>
          </div>
          <span className="font-mono text-xs" style={{ color: 'var(--kvm-text-muted)' }}>
            {vm.name}
          </span>
          <div ref={sendMenuRef} className="relative">
            <button
              type="button"
              onClick={() => setSendMenuOpen(open => !open)}
              className="kvm-action-button flex items-center gap-1.5 rounded-lg border px-3 py-1.5 text-sm"
              style={{
                background: 'rgba(255,255,255,0.035)',
                borderColor: 'rgba(76,103,150,0.42)',
                color: 'var(--kvm-text)',
              }}
            >
              Send key(s)
              <ChevronDownIcon size={14} />
            </button>
            {sendMenuOpen && (
              <div
                className="kvm-console-send-menu absolute left-0 top-full z-50 mt-2 w-44 overflow-hidden rounded-lg border py-1 shadow-2xl"
                style={{
                  background: 'var(--kvm-popover-bg)',
                  borderColor: 'var(--kvm-popover-border)',
                  color: 'var(--kvm-text)',
                  cursor: 'default',
                }}
              >
                {consoleKeySequences.map(item => (
                  <button
                    key={item.label}
                    type="button"
                    onClick={() => sendKeySequence(item)}
                    className="kvm-console-send-menu-item block w-full px-3 py-2 text-left text-sm"
                    style={{ cursor: 'pointer' }}
                  >
                    {item.label}
                  </button>
                ))}
              </div>
            )}
          </div>
          <KvmTooltip
            label={fullscreen ? '退出控制台全屏（F11）' : '控制台全屏（F11）'}
            placement="bottom"
            portalRoot={fullscreen ? frameRef.current : undefined}
          >
            <button
              type="button"
              aria-label={fullscreen ? '退出控制台全屏' : '控制台全屏'}
              onClick={toggleFullscreen}
              className="kvm-action-button flex items-center gap-1.5 rounded-lg border px-3 py-1.5 text-sm"
              style={{
                background: 'rgba(79,70,229,0.16)',
                borderColor: 'rgba(99,102,241,0.5)',
                color: '#c4b5fd',
              }}
            >
              <Maximize2Icon size={14} />
              Fullscreen
            </button>
          </KvmTooltip>
        </div>
        <KvmTooltip
          label={status}
          placement="bottom"
          align="end"
          multiline
          portalRoot={fullscreen ? frameRef.current : undefined}
          className="min-w-0 max-w-[360px] flex-1 truncate text-right text-xs"
        >
          <span style={{ color: 'var(--kvm-text-muted)' }}>{status}</span>
        </KvmTooltip>
      </div>
      <div
        className={
          (fullscreen ? 'min-h-0 flex-1 ' : 'h-[min(78vh,820px)] min-h-[520px] ') +
          'overflow-hidden rounded-lg border'
        }
        style={{ background: '#050816', borderColor: 'rgba(76,103,150,0.45)' }}
      >
        <div ref={containerRef} className="h-full w-full" />
      </div>
    </div>
  );
}

type ConsoleKeySequence =
  | { label: string; kind: 'ctrl-alt-del' }
  | { label: string; kind: 'ctrl-alt-f'; keysym: number; code: string };

const consoleKeySequences: ConsoleKeySequence[] = [
  { label: 'Ctrl+Alt+Del', kind: 'ctrl-alt-del' },
  ...Array.from({ length: 12 }, (_, index) => {
    const n = index + 1;
    return {
      label: `Ctrl+Alt+F${n}`,
      kind: 'ctrl-alt-f' as const,
      keysym: 0xffbe + index,
      code: `F${n}`,
    };
  }),
];

type ConsoleRFB = {
  disconnect: () => void;
  sendCtrlAltDel: () => void;
  sendKey: (keysym: number, code: string, down?: boolean) => void;
  scaleViewport: boolean;
  resizeSession: boolean;
  viewOnly: boolean;
  background: string;
  addEventListener: (type: string, listener: (event: Event) => void) => void;
  removeEventListener: (type: string, listener: (event: Event) => void) => void;
};
