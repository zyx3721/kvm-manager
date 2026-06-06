package kvm

import (
	"fmt"
	"net"
	"strings"
)

const maxFixedAddressEntries = 4096

func networkXML(request NetworkPoolCreateRequest) (string, error) {
	switch request.Type {
	case "nat", "route", "isolate", "bridge":
	default:
		return "", fmt.Errorf("unsupported network type")
	}
	if request.FixedAddress && !request.DHCP {
		return "", fmt.Errorf("fixed address requires dhcp")
	}
	forward := ""
	if request.Type == "nat" || request.Type == "route" {
		forward = fmt.Sprintf("\n  <forward mode='%s'/>", request.Type)
	} else if request.Type == "bridge" {
		forward = "\n  <forward mode='bridge'/>"
	}
	bridge := ""
	if strings.TrimSpace(request.Bridge) != "" {
		bridge = fmt.Sprintf("\n  <bridge name='%s'/>", escapeXMLAttr(request.Bridge))
	}
	virtualPort := ""
	if request.Type == "bridge" && request.OpenVSwitch {
		virtualPort = "\n  <virtualport type='openvswitch'/>"
	}
	ip := ""
	if strings.TrimSpace(request.Subnet) != "" && request.Type != "bridge" {
		address, netmask, dhcpStart, dhcpEnd := cidrNetwork(request.Subnet)
		dhcp := ""
		if request.DHCP {
			if dhcpStart == "" || dhcpEnd == "" {
				return "", fmt.Errorf("subnet is too small for dhcp")
			}
			hosts := ""
			if request.FixedAddress {
				items, err := fixedAddressEntries(request.Subnet)
				if err != nil {
					return "", err
				}
				for _, item := range items {
					hosts += fmt.Sprintf("\n      <host mac='%s' ip='%s'/>", escapeXMLAttr(item.MAC), escapeXMLAttr(item.Address))
				}
			}
			dhcp = "\n    <dhcp>\n      <range start='" + escapeXMLAttr(dhcpStart) + "' end='" + escapeXMLAttr(dhcpEnd) + "'/>" + hosts + "\n    </dhcp>"
		}
		ip = fmt.Sprintf("\n  <ip address='%s' netmask='%s'>%s\n  </ip>", escapeXMLAttr(address), escapeXMLAttr(netmask), dhcp)
	}
	return fmt.Sprintf("<network>\n  <name>%s</name>%s%s%s%s\n</network>", escapeXMLAttr(request.Name), forward, bridge, virtualPort, ip), nil
}

func cidrGateway(cidr string) (string, string) {
	address, netmask, _, _ := cidrNetwork(cidr)
	return address, netmask
}

func cidrNetwork(cidr string) (string, string, string, string) {
	parts := strings.Split(strings.TrimSpace(cidr), "/")
	if len(parts) != 2 {
		address := strings.TrimSpace(cidr)
		return address, "255.255.255.0", address, address
	}
	network, prefix, ok := ipv4CIDR(strings.TrimSpace(cidr))
	if !ok {
		address := strings.TrimSpace(parts[0])
		return address, "255.255.255.0", address, address
	}
	mask := uint32(0xffffffff) << uint(32-prefix)
	gateway, start, end := cidrUsableRange(network, mask, prefix)
	startText := ""
	endText := ""
	if start != 0 || end != 0 {
		startText = ipv4String(start)
		endText = ipv4String(end)
	}
	return ipv4String(gateway), ipv4String(mask), startText, endText
}

func fixedAddressEntries(cidr string) ([]NetworkFixedAddress, error) {
	start, end, ok := fixedAddressRange(cidr)
	if !ok {
		return nil, fmt.Errorf("subnet is too small for fixed address")
	}
	total := int(end - start + 1)
	if total > maxFixedAddressEntries {
		return nil, fmt.Errorf("fixed address range is too large")
	}
	entries := make([]NetworkFixedAddress, 0, total)
	for ip := start; ip <= end; ip++ {
		entries = append(entries, NetworkFixedAddress{
			Address: ipv4String(ip),
			MAC:     fixedAddressMAC(ip),
		})
	}
	return entries, nil
}

func fixedAddressRange(cidr string) (uint32, uint32, bool) {
	network, prefix, ok := ipv4CIDR(cidr)
	if !ok {
		return 0, 0, false
	}
	mask := uint32(0xffffffff) << uint(32-prefix)
	_, start, end := cidrUsableRange(network, mask, prefix)
	return start, end, start != 0 && end != 0 && start <= end
}

func cidrUsableRange(network uint32, mask uint32, prefix int) (uint32, uint32, uint32) {
	broadcast := network | ^mask
	if prefix > 30 || broadcast <= network+1 {
		return network, 0, 0
	}
	gateway := network + 1
	start := gateway + 1
	end := broadcast - 1
	if start > end {
		return gateway, 0, 0
	}
	return gateway, start, end
}

func ipv4CIDR(cidr string) (uint32, int, bool) {
	ip, network, err := net.ParseCIDR(strings.TrimSpace(cidr))
	if err != nil {
		return 0, 0, false
	}
	ip4 := ip.To4()
	if ip4 == nil {
		return 0, 0, false
	}
	prefix, bits := network.Mask.Size()
	if bits != 32 || prefix < 0 || prefix > 32 {
		return 0, 0, false
	}
	return ipv4Uint32(ip4) & (uint32(0xffffffff) << uint(32-prefix)), prefix, true
}

func ipv4Uint32(ip net.IP) uint32 {
	return uint32(ip[0])<<24 | uint32(ip[1])<<16 | uint32(ip[2])<<8 | uint32(ip[3])
}

func ipv4String(ip uint32) string {
	return fmt.Sprintf("%d.%d.%d.%d", byte(ip>>24), byte(ip>>16), byte(ip>>8), byte(ip))
}

func fixedAddressMAC(ip uint32) string {
	return fmt.Sprintf("52:54:00:%02x:%02x:%02x", byte(ip>>16), byte(ip>>8), byte(ip))
}

type networkXMLDoc struct {
	Bridge struct {
		Name string `xml:"name,attr"`
	} `xml:"bridge"`
	VirtualPort struct {
		Type string `xml:"type,attr"`
	} `xml:"virtualport"`
	Forward struct {
		Mode string `xml:"mode,attr"`
	} `xml:"forward"`
	IP struct {
		Address string `xml:"address,attr"`
		DHCP    struct {
			Ranges []struct {
				Start string `xml:"start,attr"`
				End   string `xml:"end,attr"`
			} `xml:"range"`
			Hosts []struct {
				MAC string `xml:"mac,attr"`
				IP  string `xml:"ip,attr"`
			} `xml:"host"`
		} `xml:"dhcp"`
	} `xml:"ip"`
}

func (doc networkXMLDoc) fixedAddresses() []NetworkFixedAddress {
	items := make([]NetworkFixedAddress, 0, len(doc.IP.DHCP.Hosts))
	for _, host := range doc.IP.DHCP.Hosts {
		items = append(items, NetworkFixedAddress{Address: host.IP, MAC: host.MAC})
	}
	return items
}
