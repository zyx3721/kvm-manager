export const KVM_REFRESH_EVENT = 'kvm:refresh';

export function emitKvmRefresh() {
  window.dispatchEvent(new Event(KVM_REFRESH_EVENT));
}

export function onKvmRefresh(handler: () => void) {
  window.addEventListener(KVM_REFRESH_EVENT, handler);
  return () => window.removeEventListener(KVM_REFRESH_EVENT, handler);
}
