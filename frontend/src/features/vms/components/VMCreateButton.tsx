import { FolderPlusIcon } from 'lucide-react';

export function VMCreateButton({ onClick }: { onClick: () => void }) {
  return (
    <button
      type="button"
      onClick={onClick}
      className="kvm-action-button inline-flex h-10 items-center gap-2 rounded-lg border px-4 text-sm font-semibold"
      style={{
        background: 'rgba(45,212,191,0.12)',
        borderColor: 'rgba(45,212,191,0.34)',
        color: 'var(--kvm-check-toggle-active-text)',
      }}
    >
      <FolderPlusIcon size={16} />
      创建虚拟机
    </button>
  );
}
