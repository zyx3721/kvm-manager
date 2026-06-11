import { AlertTriangleIcon } from 'lucide-react';
import { forwardRef } from 'react';
import type { Alert } from '../../lib/api';

export type NotificationPanelPosition = {
  top: number;
  right: number;
};

type NotificationPanelProps = {
  items: Alert[];
  unreadCount: number;
  position: NotificationPanelPosition;
  onReadAll: () => void;
  onClear: () => void;
  onOpen: (item: Alert) => void;
  onClose: () => void;
};

const NotificationPanel = forwardRef<HTMLDivElement, NotificationPanelProps>(
  function NotificationPanel(
    { items, unreadCount, position, onReadAll, onClear, onOpen, onClose },
    ref
  ) {
    return (
      <div
        ref={ref}
        className="kvm-notification-panel fixed z-[1300] flex w-[min(420px,calc(100vw-2rem))] flex-col overflow-hidden rounded-xl shadow-2xl"
        role="dialog"
        aria-label="通知消息"
        style={{
          top: position.top,
          right: position.right,
        }}
      >
        <div
          className="flex items-center justify-between gap-3 px-5 py-4"
          style={{ borderBottom: '1px solid var(--kvm-border)' }}
        >
          <div>
            <div className="text-sm font-semibold" style={{ color: 'var(--kvm-text)' }}>
              通知消息
            </div>
            <div className="mt-1 text-xs" style={{ color: 'var(--kvm-text-muted)' }}>
              {unreadCount > 0 ? `${unreadCount} 条未读告警` : '暂无未读告警'}
            </div>
          </div>
          <div className="flex items-center gap-3 text-xs">
            <button
              type="button"
              onClick={onReadAll}
              className="kvm-action-button"
              style={{ color: 'var(--kvm-accent-text)' }}
            >
              全部已读
            </button>
            <button
              type="button"
              onClick={onClear}
              className="kvm-action-button"
              style={{ color: '#f87171' }}
            >
              清空
            </button>
            <button
              type="button"
              onClick={onClose}
              className="kvm-action-button md:hidden"
              style={{ color: 'var(--kvm-text-muted)' }}
            >
              关闭
            </button>
          </div>
        </div>
        <div className="kvm-hidden-scrollbar max-h-[28rem] overflow-y-auto">
          {items.length === 0 ? (
            <div
              className="px-5 py-10 text-center text-sm"
              style={{ color: 'var(--kvm-text-muted)' }}
            >
              没有更多了
            </div>
          ) : (
            items.map(item => (
              <button
                key={item.id}
                type="button"
                onClick={() => onOpen(item)}
                className="kvm-action-button flex w-full gap-3 px-5 py-4 text-left"
                style={{ borderBottom: '1px solid rgba(148,163,184,0.14)' }}
              >
                <div
                  className="mt-0.5 flex h-9 w-9 shrink-0 items-center justify-center rounded-lg"
                  style={{
                    color: alertTone(item.level),
                    background: `${alertTone(item.level)}18`,
                    border: `1px solid ${alertTone(item.level)}33`,
                  }}
                >
                  <AlertTriangleIcon size={17} />
                </div>
                <div className="min-w-0 flex-1">
                  <div className="flex items-center gap-2">
                    {!item.readAt && (
                      <span
                        className="h-1.5 w-1.5 shrink-0 rounded-full"
                        style={{ background: '#ef4444' }}
                      />
                    )}
                    <span
                      className="truncate text-sm font-semibold"
                      style={{ color: 'var(--kvm-text)' }}
                    >
                      {item.title}
                    </span>
                    <span
                      className="ml-auto shrink-0 text-xs"
                      style={{ color: 'var(--kvm-text-muted)' }}
                    >
                      {formatTimeAgo(item.lastSeenAt)}
                    </span>
                  </div>
                  <div
                    className="mt-1 line-clamp-2 text-sm leading-5"
                    style={{ color: 'var(--kvm-text-muted)' }}
                  >
                    {item.message}
                  </div>
                </div>
              </button>
            ))
          )}
        </div>
      </div>
    );
  }
);

export default NotificationPanel;

function alertTone(level: string) {
  if (level === 'critical') return '#f87171';
  if (level === 'warning') return '#f59e0b';
  return '#60a5fa';
}

function formatTimeAgo(value: string) {
  const time = new Date(value).getTime();
  const diff = Date.now() - time;
  if (!Number.isFinite(time) || diff < 0) return '刚刚';
  const minute = 60 * 1000;
  const hour = 60 * minute;
  const day = 24 * hour;
  if (diff < minute) return '刚刚';
  if (diff < hour) return `${Math.floor(diff / minute)}分钟前`;
  if (diff < day) return `${Math.floor(diff / hour)}小时前`;
  return `${Math.floor(diff / day)}天前`;
}
