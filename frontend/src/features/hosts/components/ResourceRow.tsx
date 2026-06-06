import { MetricBar } from '../../../components/kvm/StatusBadge';

export function ResourceRow({
  icon: Icon,
  label,
  value,
}: {
  icon: React.ElementType;
  label: string;
  value: number;
}) {
  return (
    <div>
      <div className="mb-1.5 flex items-center justify-between text-xs">
        <span className="flex items-center gap-1.5" style={{ color: 'var(--kvm-text-muted)' }}>
          <Icon size={14} />
          {label}
        </span>
        <span className="font-mono tabular-nums" style={{ color: 'var(--kvm-text)' }}>
          {value}%
        </span>
      </div>
      <MetricBar value={value} />
    </div>
  );
}
