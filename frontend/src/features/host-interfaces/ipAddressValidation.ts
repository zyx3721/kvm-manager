import type { HostInterface, HostInterfaceCreatePayload } from '../../lib/api';

type IPFamily = 'ipv4' | 'ipv6';

type ParsedCIDR = {
  family: IPFamily;
  ip: bigint;
  network: bigint;
  prefix: number;
};

export function validateHostInterfaceAddressConflicts(
  payload: HostInterfaceCreatePayload,
  existing: HostInterface[]
) {
  return (
    validateDNSServers(payload.dnsServers) ||
    validateFamilyAddressConflicts(
      payload.ipv4Mode,
      payload.ipv4Address,
      payload.ipv4Gateway,
      'ipv4',
      'IPv4',
      payload.device,
      existing
    ) ||
    validateFamilyAddressConflicts(
      payload.ipv6Mode,
      payload.ipv6Address,
      payload.ipv6Gateway,
      'ipv6',
      'IPv6',
      payload.device,
      existing
    )
  );
}

function validateDNSServers(values: string[]) {
  for (const value of values) {
    if (parseIPv4(value) === null && parseIPv6(value) === null) return 'DNS 地址格式不正确';
  }
  return '';
}

function validateFamilyAddressConflicts(
  mode: string,
  address: string,
  gateway: string,
  family: IPFamily,
  label: string,
  allowedSourceDevice: string,
  existing: HostInterface[]
) {
  if (mode !== 'static') return '';
  const requested = parseCIDR(address, family);
  if (!requested) return `${label} 地址格式不正确，请使用 CIDR 格式`;
  const gatewayText = gateway.trim();
  if (gatewayText) {
    const gatewayIP = family === 'ipv4' ? parseIPv4(gatewayText) : parseIPv6(gatewayText);
    if (gatewayIP === null) return `${label} 网关格式不正确`;
    if (!containsIP(requested, gatewayIP)) return `${label} 网关必须与地址处于同一子网`;
  }

  for (const item of existing) {
    if (isAllowedBridgeSourceAddress(item, allowedSourceDevice)) continue;
    const existingAddress = family === 'ipv4' ? item.ipv4 : item.ipv6;
    const parsed = parseCIDR(existingAddress, family);
    if (!parsed) continue;
    if (requested.ip === parsed.ip) return `${label} 地址已被接口 ${item.name} 使用`;
    if (sameSubnet(requested, parsed))
      return `${label} 子网已被接口 ${item.name} 使用，请避免同宿主机重复子网`;
  }
  return '';
}

function isAllowedBridgeSourceAddress(item: HostInterface, allowedSourceDevice: string) {
  const normalized = allowedSourceDevice.trim().toLowerCase();
  if (!normalized) return false;
  return item.name.trim().toLowerCase() === normalized;
}

function sameSubnet(left: ParsedCIDR, right: ParsedCIDR) {
  if (left.family !== right.family) return false;
  return containsIP(left, right.ip) || containsIP(right, left.ip);
}

function containsIP(network: ParsedCIDR, ip: bigint) {
  const bits = network.family === 'ipv4' ? 32 : 128;
  const hostBits = BigInt(bits - network.prefix);
  const targetNetwork = hostBits === 0n ? ip : (ip >> hostBits) << hostBits;
  return targetNetwork === network.network;
}

function parseCIDR(address: string, family: IPFamily): ParsedCIDR | null {
  const [ipText, prefixText, extra] = address.trim().split('/');
  if (!ipText || !prefixText || extra !== undefined) return null;
  const prefix = Number(prefixText);
  const bits = family === 'ipv4' ? 32 : 128;
  if (!Number.isInteger(prefix) || prefix < 0 || prefix > bits) return null;

  const ip = family === 'ipv4' ? parseIPv4(ipText) : parseIPv6(ipText);
  if (ip === null) return null;
  const hostBits = BigInt(bits - prefix);
  const network = hostBits === 0n ? ip : (ip >> hostBits) << hostBits;
  return { family, ip, network, prefix };
}

function parseIPv4(address: string): bigint | null {
  const parts = address.trim().split('.');
  if (parts.length !== 4) return null;
  let result = 0n;
  for (const part of parts) {
    if (!/^\d{1,3}$/.test(part)) return null;
    const value = Number(part);
    if (value < 0 || value > 255) return null;
    result = (result << 8n) + BigInt(value);
  }
  return result;
}

function parseIPv6(address: string): bigint | null {
  const value = address.trim().toLowerCase();
  if (!value || value.includes('.') || value.includes(':::')) return null;

  const sections = value.split('::');
  if (sections.length > 2) return null;

  const left = splitIPv6Section(sections[0]);
  const right = sections.length === 2 ? splitIPv6Section(sections[1]) : [];
  if (!left || !right) return null;

  const missing = 8 - left.length - right.length;
  if (sections.length === 1 && missing !== 0) return null;
  if (sections.length === 2 && missing < 1) return null;

  const groups = sections.length === 2 ? [...left, ...Array(missing).fill(0), ...right] : left;
  if (groups.length !== 8) return null;

  let result = 0n;
  for (const group of groups) {
    result = (result << 16n) + BigInt(group);
  }
  return result;
}

function splitIPv6Section(section: string) {
  if (!section) return [];
  const groups = section.split(':').map(part => {
    if (!/^[0-9a-f]{1,4}$/.test(part)) return null;
    return Number.parseInt(part, 16);
  });
  if (groups.some(group => group === null)) return null;
  return groups as number[];
}
