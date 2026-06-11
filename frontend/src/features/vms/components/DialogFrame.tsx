import React from 'react';
import { XIcon } from 'lucide-react';
import { DialogPortal } from '../../../components/kvm/DialogPortal';

export function DialogFrame({
  title,
  tone,
  children,
  onClose,
  wide,
  hideHeader,
}: {
  title?: string;
  tone: 'normal' | 'warning' | 'danger';
  children: React.ReactNode;
  onClose: () => void;
  wide?: boolean;
  hideHeader?: boolean;
}) {
  const accent = tone === 'danger' ? '#ef4444' : tone === 'warning' ? '#f59e0b' : '#3b82f6';
  return (
    <DialogPortal>
      <div className="kvm-dialog-backdrop fixed inset-0 z-50 flex items-center justify-center px-4">
        <div
          className={
            'kvm-dialog-panel relative w-full rounded-xl p-5 shadow-2xl ' +
            (wide ? 'max-w-5xl' : 'max-w-md')
          }
        >
          {!hideHeader && (
            <div className="mb-4 flex items-center justify-between gap-3">
              <div className="flex items-center gap-2">
                <span
                  className="h-2 w-2 rounded-full"
                  style={{ background: accent, boxShadow: `0 0 18px ${accent}` }}
                />
                <h2 className="text-base font-semibold" style={{ color: 'var(--kvm-text)' }}>
                  {title}
                </h2>
              </div>
              <button
                type="button"
                onClick={onClose}
                className="kvm-action-button flex h-8 w-8 items-center justify-center rounded-lg"
                style={{
                  color: 'var(--kvm-text-muted)',
                  background: 'var(--kvm-control-bg)',
                  border: '1px solid var(--kvm-border)',
                }}
                aria-label="关闭"
              >
                <XIcon size={15} />
              </button>
            </div>
          )}
          {hideHeader && (
            <button
              type="button"
              onClick={onClose}
              className="kvm-action-button absolute right-6 top-6 flex h-8 w-8 items-center justify-center rounded-lg"
              style={{
                color: 'var(--kvm-text-muted)',
                background: 'var(--kvm-control-bg)',
                border: '1px solid var(--kvm-border)',
              }}
              aria-label="关闭"
            >
              <XIcon size={15} />
            </button>
          )}
          {children}
        </div>
      </div>
    </DialogPortal>
  );
}
