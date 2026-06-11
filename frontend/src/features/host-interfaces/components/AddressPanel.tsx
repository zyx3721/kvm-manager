import { SelectMenu } from '../../../components/kvm/SelectMenu';

const fieldStyle = {
  background: 'var(--kvm-control-bg)',
  border: '1px solid var(--kvm-border)',
  color: 'var(--kvm-text)',
};

export function AddressPanel({
  title,
  modeLabel,
  staticSummary,
  inactiveSummary,
  mode,
  options,
  address,
  gateway,
  dnsServers,
  showSecondaryDNS,
  addressPlaceholder,
  gatewayPlaceholder,
  onMode,
  onAddress,
  onGateway,
  onDNS,
  onShowSecondaryDNS,
}: {
  title: string;
  modeLabel: string;
  staticSummary: string;
  inactiveSummary: string;
  mode: string;
  options: { value: string; label: string }[];
  address: string;
  gateway: string;
  dnsServers: string[];
  showSecondaryDNS: boolean;
  addressPlaceholder: string;
  gatewayPlaceholder: string;
  onMode: (value: string) => void;
  onAddress: (value: string) => void;
  onGateway: (value: string) => void;
  onDNS: (index: number, value: string) => void;
  onShowSecondaryDNS: () => void;
}) {
  const staticMode = mode === 'static';
  return (
    <section
      className="rounded-xl border p-4"
      style={{
        borderColor: staticMode ? 'rgba(45,212,191,0.34)' : 'var(--kvm-border)',
        background: 'var(--kvm-control-bg-soft)',
      }}
    >
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="min-w-0">
          <h3 className="text-sm font-semibold" style={{ color: 'var(--kvm-text)' }}>
            {title}
          </h3>
          <p className="mt-1 text-xs leading-5" style={{ color: 'var(--kvm-text-muted)' }}>
            {staticMode ? staticSummary : inactiveSummary}
          </p>
        </div>
        <div className="w-full sm:w-36 md:w-42">
          <span className="sr-only">{modeLabel}</span>
          <SelectMenu
            value={mode}
            placeholder={`请选择 ${title} 模式`}
            options={options}
            placement="top"
            onChange={onMode}
          />
        </div>
      </div>
      <div className="mt-4 min-h-[116px]">
        {staticMode ? (
          <div className="space-y-3">
            <div className="space-y-2">
              <span className="text-xs font-semibold" style={{ color: 'var(--kvm-text-muted)' }}>
                {title} 地址
              </span>
              <input
                value={address}
                onChange={event => onAddress(event.target.value)}
                placeholder={addressPlaceholder}
                className="h-10 w-full rounded-lg px-3 text-sm outline-none"
                style={fieldStyle}
              />
            </div>
            <div className="space-y-2">
              <span className="text-xs font-semibold" style={{ color: 'var(--kvm-text-muted)' }}>
                {title} 网关
              </span>
              <input
                value={gateway}
                onChange={event => onGateway(event.target.value)}
                placeholder={gatewayPlaceholder}
                className="h-10 w-full rounded-lg px-3 text-sm outline-none"
                style={fieldStyle}
              />
            </div>
            {title === 'IPv4' && (
              <div
                className="space-y-3 rounded-lg border p-3"
                style={{ borderColor: 'var(--kvm-border)', background: 'rgba(148,163,184,0.06)' }}
              >
                <div className="space-y-2">
                  <span
                    className="text-xs font-semibold"
                    style={{ color: 'var(--kvm-text-muted)' }}
                  >
                    DNS1 地址
                  </span>
                  <input
                    value={dnsServers[0] || ''}
                    onChange={event => onDNS(0, event.target.value)}
                    placeholder="223.5.5.5"
                    className="h-9 w-full rounded-lg px-3 text-sm outline-none"
                    style={fieldStyle}
                  />
                </div>
                {showSecondaryDNS ? (
                  <div className="space-y-2">
                    <span
                      className="text-xs font-semibold"
                      style={{ color: 'var(--kvm-text-muted)' }}
                    >
                      DNS2 地址
                    </span>
                    <input
                      value={dnsServers[1] || ''}
                      onChange={event => onDNS(1, event.target.value)}
                      placeholder="8.8.8.8"
                      className="h-9 w-full rounded-lg px-3 text-sm outline-none"
                      style={fieldStyle}
                    />
                  </div>
                ) : (
                  <button
                    type="button"
                    onClick={onShowSecondaryDNS}
                    className="kvm-action-button rounded-lg border px-3 py-1.5 text-xs font-semibold"
                    style={{
                      borderColor: 'var(--kvm-border)',
                      color: 'var(--kvm-accent-text)',
                      background: 'rgba(59,130,246,0.08)',
                    }}
                  >
                    添加DNS2地址
                  </button>
                )}
              </div>
            )}
          </div>
        ) : (
          <div
            className="flex h-[116px] items-center rounded-lg border border-dashed px-4 text-sm"
            style={{
              borderColor: 'var(--kvm-border)',
              color: 'var(--kvm-text-muted)',
              background: 'rgba(148,163,184,0.06)',
            }}
          >
            {mode === 'dhcp' ? `${title} 将通过 DHCP 自动获取地址` : `${title} 当前不写入地址配置`}
          </div>
        )}
      </div>
    </section>
  );
}
