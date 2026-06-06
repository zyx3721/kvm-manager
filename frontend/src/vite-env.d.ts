/// <reference types="vite/client" />
/// <reference types="react" />

declare module '@novnc/novnc' {
  export default class RFB {
    scaleViewport: boolean;
    resizeSession: boolean;
    viewOnly: boolean;
    background: string;

    constructor(
      target: HTMLElement,
      url: string,
      options?: { credentials?: Record<string, string>; wsProtocols?: string[] }
    );
    disconnect(): void;
    sendCtrlAltDel(): void;
    sendKey(keysym: number, code: string, down?: boolean): void;
    addEventListener(type: string, listener: (event: Event) => void): void;
    removeEventListener(type: string, listener: (event: Event) => void): void;
  }
}
