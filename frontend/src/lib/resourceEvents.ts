export const KVM_RESOURCE_EVENT = 'kvm:resource-event';

export type KvmResourceEventType =
  | 'storage.pool.updated'
  | 'network.pool.updated'
  | 'host.interface.updated';

export type KvmResourceEventDetail = {
  type: KvmResourceEventType;
  agentId: string;
  name?: string;
  pool?: string;
};

export function emitKvmResourceEvent(detail: KvmResourceEventDetail) {
  window.dispatchEvent(new CustomEvent<KvmResourceEventDetail>(KVM_RESOURCE_EVENT, { detail }));
}

export function onKvmResourceEvent(
  type: KvmResourceEventType,
  handler: (detail: KvmResourceEventDetail) => void
) {
  const listener = (event: Event) => {
    const detail = (event as CustomEvent<KvmResourceEventDetail>).detail;
    if (!detail || detail.type !== type) return;
    handler(detail);
  };
  window.addEventListener(KVM_RESOURCE_EVENT, listener);
  return () => window.removeEventListener(KVM_RESOURCE_EVENT, listener);
}
