import { useEffect, useState, type ReactNode } from 'react';
import { XIcon } from 'lucide-react';
import { toast } from 'sonner';

import { DialogPortal } from '../../../components/kvm/DialogPortal';
import { SelectMenu } from '../../../components/kvm/SelectMenu';
import {
  fetchHostInterfaceDevices,
  type HostInterface,
  type HostInterfaceCreatePayload,
  type HostInterfaceDevice,
} from '../../../lib/api';
import { AddressPanel } from './AddressPanel';

const buttonStyle = {
  background: 'var(--kvm-control-bg)',
  borderColor: 'var(--kvm-border)',
  color: 'var(--kvm-text)',
};

const primaryButtonStyle = {
  background: 'rgba(59,130,246,0.15)',
  borderColor: 'rgba(59,130,246,0.42)',
  color: 'var(--kvm-accent-text)',
};

const fieldStyle = {
  background: 'var(--kvm-control-bg)',
  border: '1px solid var(--kvm-border)',
  color: 'var(--kvm-text)',
};

export function HostInterfaceCreateDialog({
  agentId,
  hostName,
  onClose,
  onSubmit,
}: {
  agentId: string;
  hostName: string;
  interfaces: HostInterface[];
  onClose: () => void;
  onSubmit: (payload: HostInterfaceCreatePayload) => Promise<void>;
}) {
  const [name, setName] = useState('');
  const [startMode, setStartMode] = useState('onboot');
  const [device, setDevice] = useState('');
  const [type, setType] = useState('bridge');
  const [stp, setStp] = useState('on');
  const [delay, setDelay] = useState('0');
  const [ipv4Mode, setIpv4Mode] = useState('dhcp');
  const [ipv4Address, setIpv4Address] = useState('');
  const [ipv4Gateway, setIpv4Gateway] = useState('');
  const [ipv6Mode, setIpv6Mode] = useState('none');
  const [ipv6Address, setIpv6Address] = useState('');
  const [ipv6Gateway, setIpv6Gateway] = useState('');
  const [dnsServers, setDNSServers] = useState(['', '']);
  const [showSecondaryDNS, setShowSecondaryDNS] = useState(false);
  const [devices, setDevices] = useState<HostInterfaceDevice[]>([]);
  const [deviceLoading, setDeviceLoading] = useState(true);
  const [submitting, setSubmitting] = useState(false);
  const payload = {
    name,
    startMode,
    device,
    type,
    stp,
    delay,
    ipv4Mode,
    ipv4Address,
    ipv4Gateway,
    ipv6Mode,
    ipv6Address,
    ipv6Gateway,
    applySystemConfig: dnsServers.some(item => item.trim()),
    dnsServers: dnsServers.map(item => item.trim()).filter(Boolean),
  };
  const deviceOptions = devices.map(item => ({ value: item.name, label: item.name }));
  const setDNS = (index: number, value: string) => setDNSServers(current => current.map((item, itemIndex) => itemIndex === index ? value : item));

  useEffect(() => {
    let cancelled = false;
    setDeviceLoading(true);
    fetchHostInterfaceDevices(agentId)
      .then(body => {
        if (!cancelled) setDevices(body.items);
      })
      .catch(error => {
        if (!cancelled) {
          setDevices([]);
          toast.error(error instanceof Error ? error.message : '读取接口设备失败');
        }
      })
      .finally(() => {
        if (!cancelled) setDeviceLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [agentId]);

  return (
    <DialogPortal>
      <div className="kvm-dialog-backdrop fixed inset-0 z-50 flex items-center justify-center px-3">
        <div className="kvm-dialog-panel max-h-[88vh] w-[min(94vw,760px)] overflow-hidden rounded-2xl">
          <header className="flex items-center justify-between border-b px-5 py-4" style={{ borderColor: 'var(--kvm-border)' }}>
            <div>
              <h2 className="text-base font-semibold" style={{ color: 'var(--kvm-text)' }}>新增接口</h2>
              <p className="mt-1 text-xs" style={{ color: 'var(--kvm-text-muted)' }}>{hostName}</p>
            </div>
            <button type="button" onClick={onClose} disabled={submitting} aria-label="关闭" className="kvm-action-button inline-flex h-8 w-8 items-center justify-center rounded-lg border disabled:cursor-not-allowed disabled:opacity-60" style={buttonStyle}><XIcon size={15} /></button>
          </header>
          <div className="kvm-hidden-scrollbar max-h-[calc(88vh-138px)] overflow-y-auto">
            <div className="mx-auto max-w-[620px] space-y-4 p-5 md:pl-2 md:pr-8">
              <Field label="名称"><input value={name} onChange={event => setName(event.target.value)} placeholder="br0" className="h-10 w-full rounded-lg px-3 text-sm outline-none" style={fieldStyle} /></Field>
              <Field label="启动模式">
                <SelectMenu value={startMode} placeholder="请选择启动模式" options={startModeOptions()} optionTooltipPlacement="right" onChange={setStartMode} />
              </Field>
              <Field label="设备">
                <SelectMenu value={device} disabled={deviceLoading} placeholder={deviceLoading ? '正在读取设备' : '不绑定设备'} options={[{ value: '', label: '不绑定设备' }, ...deviceOptions]} maxVisibleItems={8} onChange={setDevice} />
              </Field>
              <Field label="类型">
                <SelectMenu value={type} placeholder="请选择接口类型" options={interfaceTypeOptions()} onChange={setType} />
              </Field>
              {type === 'bridge' && <Field label="STP">
                <SelectMenu value={stp} placeholder="请选择 STP" options={['on', 'off'].map(item => ({ value: item, label: item }))} onChange={setStp} />
              </Field>}
              {type === 'bridge' && <Field label="Delay"><input value={delay} onChange={event => setDelay(event.target.value)} placeholder="0" className="h-10 w-full rounded-lg px-3 text-sm outline-none" style={fieldStyle} /></Field>}
            </div>
            <div className="mx-auto grid max-w-[680px] gap-4 p-5 md:grid-cols-2">
              <AddressPanel
                title="IPv4"
                modeLabel="IPv4 模式"
                staticSummary="静态地址会在创建前检查重复 IP 和重复子网"
                inactiveSummary="DHCP 自动获取地址，可切换为 Static 手动填写"
                mode={ipv4Mode}
                options={ipv4ModeOptions()}
                address={ipv4Address}
                gateway={ipv4Gateway}
                dnsServers={dnsServers}
                showSecondaryDNS={showSecondaryDNS}
                addressPlaceholder="192.168.0.11/24"
                gatewayPlaceholder="192.168.0.1"
                onMode={setIpv4Mode}
                onAddress={setIpv4Address}
                onGateway={setIpv4Gateway}
                onDNS={setDNS}
                onShowSecondaryDNS={() => setShowSecondaryDNS(true)}
              />
              <AddressPanel
                title="IPv6"
                modeLabel="IPv6 模式"
                staticSummary="静态地址会在创建前检查重复 IP 和重复子网"
                inactiveSummary="默认不配置 IPv6，需要时可切换为 DHCP 或 Static"
                mode={ipv6Mode}
                options={ipv6ModeOptions()}
                address={ipv6Address}
                gateway={ipv6Gateway}
                dnsServers={dnsServers}
                showSecondaryDNS={showSecondaryDNS}
                addressPlaceholder="2001:db8::10/64"
                gatewayPlaceholder="2001:db8::1"
                onMode={setIpv6Mode}
                onAddress={setIpv6Address}
                onGateway={setIpv6Gateway}
                onDNS={setDNS}
                onShowSecondaryDNS={() => setShowSecondaryDNS(true)}
              />
            </div>
          </div>
          <footer className="flex justify-end gap-2 border-t px-5 py-4" style={{ borderColor: 'var(--kvm-border)' }}>
            <button type="button" onClick={onClose} disabled={submitting} className="kvm-action-button rounded-lg border px-4 py-2 text-sm disabled:cursor-not-allowed disabled:opacity-60" style={buttonStyle}>关闭</button>
            <button
              type="button"
              disabled={submitting}
              onClick={async () => {
                setSubmitting(true);
                try {
                  await onSubmit(payload);
                } finally {
                  setSubmitting(false);
                }
              }}
              className="kvm-action-button rounded-lg border px-4 py-2 text-sm font-semibold disabled:cursor-not-allowed disabled:opacity-60"
              style={primaryButtonStyle}
            >
              {submitting ? '创建中' : '创建'}
            </button>
          </footer>
        </div>
      </div>
    </DialogPortal>
  );
}

function Field({ label, children }: { label: string; children: ReactNode }) {
  return <label className="grid gap-2 md:grid-cols-[110px_minmax(0,430px)] md:items-center"><span className="text-sm font-semibold md:text-right" style={{ color: 'var(--kvm-text)' }}>{label}</span>{children}</label>;
}

function startModeOptions() {
  return [
    { value: 'none', label: 'none', tooltip: '不设置自动启动，由管理员手动启动接口' },
    { value: 'onboot', label: 'onboot', tooltip: '宿主机启动时自动启动该接口' },
    { value: 'hotplug', label: 'hotplug', tooltip: '设备热插拔出现时自动启动该接口' },
  ];
}

function interfaceTypeOptions() {
  return [
    { value: 'bridge', label: 'bridge' },
    { value: 'ethernet', label: 'ethernet' },
  ];
}

function ipv4ModeOptions() {
  return [
    { value: 'dhcp', label: 'DHCP' },
    { value: 'static', label: 'Static' },
    { value: 'none', label: 'No configuation' },
  ];
}

function ipv6ModeOptions() {
  return [
    { value: 'none', label: 'No configuation' },
    { value: 'dhcp', label: 'DHCP' },
    { value: 'static', label: 'Static' },
  ];
}
