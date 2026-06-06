package kvm

import (
	"encoding/xml"
	"fmt"
	"net"
	"strings"
)

func (p *VirshProvider) ListHostInterfaces() ([]HostInterface, error) {
	libvirtItems, libvirtErr := p.listHostInterfacesLibvirt()
	if libvirtErr == nil && len(libvirtItems) > 0 {
		textItems, _ := p.listHostInterfacesTextFor(libvirtItemNames(libvirtItems))
		return mergeHostInterfaces(libvirtItems, textItems, false), nil
	}
	textItems, textErr := p.listHostInterfacesText()
	if textErr == nil {
		return filterFallbackHostInterfaces(textItems), nil
	}
	if libvirtErr == nil {
		return []HostInterface{}, nil
	}
	return nil, libvirtErr
}

func (p *VirshProvider) listHostInterfacesTextFor(allowed map[string]struct{}) ([]HostInterface, error) {
	if len(allowed) == 0 {
		return []HostInterface{}, nil
	}
	return p.listHostInterfacesTextFiltered(allowed)
}

func (p *VirshProvider) listHostInterfacesText() ([]HostInterface, error) {
	return p.listHostInterfacesTextFiltered(nil)
}

func (p *VirshProvider) listHostInterfacesTextFiltered(allowed map[string]struct{}) ([]HostInterface, error) {
	linkOut, err := p.output("ip", "-o", "link", "show")
	if err != nil {
		return nil, err
	}
	items := make([]HostInterface, 0)
	byName := map[string]int{}
	for _, line := range strings.Split(linkOut, "\n") {
		item := parseLinkLine(line)
		if item.Name == "" {
			continue
		}
		if allowed != nil {
			if _, ok := allowed[item.Name]; !ok {
				continue
			}
		}
		item.Type = p.detectInterfaceType(item.Name, item.Type)
		item.BootMode = interfaceBootMode(item.Name)
		item.STP = p.bridgeSTP(item.Name)
		item.Delay = p.bridgeDelay(item.Name)
		byName[item.Name] = len(items)
		items = append(items, item)
	}
	addrOut, err := p.output("ip", "-o", "addr", "show")
	if err == nil {
		for _, line := range strings.Split(addrOut, "\n") {
			name, family, address := parseAddrLine(line)
			index, ok := byName[name]
			if !ok || address == "" {
				continue
			}
			item := items[index]
			switch family {
			case "inet":
				if item.IPv4 == "" {
					ip := strings.Split(address, "/")[0]
					if isUsableIPv4(ip) {
						item.IPv4 = address
						item.IPv4Mode = "static"
					}
				}
			case "inet6":
				if item.IPv6 == "" && !strings.HasPrefix(strings.ToLower(address), "fe80:") {
					item.IPv6 = address
					item.IPv6Mode = "static"
				} else if item.IPv6 == "" {
					item.IPv6 = address
					item.IPv6Mode = "link-local"
				}
			}
			items[index] = item
		}
	}
	for index := range items {
		if items[index].IPv4Mode == "" {
			items[index].IPv4Mode = "none"
		}
		if items[index].IPv6Mode == "" {
			items[index].IPv6Mode = "none"
		}
	}
	return items, nil
}

func (p *VirshProvider) listHostInterfacesLibvirt() ([]HostInterface, error) {
	entries, err := p.interfaceEntriesLibvirt()
	if err != nil {
		return nil, err
	}
	items := make([]HostInterface, 0, len(entries))
	for _, entry := range entries {
		item, err := p.hostInterfaceFromLibvirt(entry)
		if err != nil {
			continue
		}
		items = append(items, item)
	}
	return items, nil
}

func (p *VirshProvider) hostInterfaceFromLibvirt(entry hostInterfaceListEntry) (HostInterface, error) {
	name := entry.Name
	xmlText, err := p.interfaceXML(name)
	if err != nil {
		return HostInterface{}, err
	}
	var doc interfaceXMLDoc
	if err := xml.Unmarshal([]byte(xmlText), &doc); err != nil {
		return HostInterface{}, err
	}
	item := HostInterface{
		Name:         firstNonEmptyString(strings.TrimSpace(doc.Name), name),
		Type:         strings.TrimSpace(doc.Type),
		MAC:          firstNonEmptyString(strings.ToLower(strings.TrimSpace(doc.MAC.Address)), entry.MAC),
		BootMode:     strings.TrimSpace(doc.Start.Mode),
		Status:       entry.Status,
		BridgeDevice: strings.TrimSpace(doc.Bridge.Interface.Name),
		STP:          strings.TrimSpace(doc.Bridge.STP),
		Delay:        strings.TrimSpace(doc.Bridge.Delay),
	}
	if item.BridgeDevice == "" {
		item.BridgeDevice = p.activeInterfaceBridgeDevice(name)
	}
	for index, protocol := range doc.Protocols {
		family := strings.ToLower(strings.TrimSpace(protocol.Family))
		mode, address := protocolAddressMode(protocol)
		switch {
		case family == "ipv4" || family == "" && index == 0:
			item.IPv4Mode = mode
			item.IPv4 = address
		case family == "ipv6" || family == "" && index == 1:
			item.IPv6Mode = mode
			item.IPv6 = address
		}
	}
	fillInterfaceDefaults(&item)
	return item, nil
}

func (p *VirshProvider) interfaceXML(name string) (string, error) {
	if out, err := p.output("virsh", "--connect", p.libvirtURI, "iface-dumpxml", name, "--inactive"); err == nil {
		return out, nil
	}
	return p.output("virsh", "--connect", p.libvirtURI, "iface-dumpxml", name)
}

func (p *VirshProvider) activeInterfaceBridgeDevice(name string) string {
	xmlText, err := p.output("virsh", "--connect", p.libvirtURI, "iface-dumpxml", name)
	if err != nil {
		return ""
	}
	var doc interfaceXMLDoc
	if err := xml.Unmarshal([]byte(xmlText), &doc); err != nil {
		return ""
	}
	return strings.TrimSpace(doc.Bridge.Interface.Name)
}

func (p *VirshProvider) UpdateHostInterfaceState(name string, active bool) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("interface name is required")
	}
	if !validInterfaceName(name) {
		return fmt.Errorf("interface name is invalid")
	}
	if active {
		return p.startHostInterface(name)
	}
	return p.stopHostInterface(name)
}

func (p *VirshProvider) startHostInterface(name string) error {
	if _, err := p.output("virsh", "--connect", p.libvirtURI, "iface-start", name); err != nil {
		if p.hostInterfaceReachedState(name, "up") {
			return nil
		}
		return err
	}
	return nil
}

func (p *VirshProvider) stopHostInterface(name string) error {
	if _, err := p.output("virsh", "--connect", p.libvirtURI, "iface-destroy", name); err != nil {
		if p.hostInterfaceReachedState(name, "down") {
			return nil
		}
		return err
	}
	return nil
}

func (p *VirshProvider) hostInterfaceReachedState(name string, status string) bool {
	item, err := p.hostInterface(name)
	return err == nil && item.Status == status
}

func (p *VirshProvider) DeleteHostInterface(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("interface name is required")
	}
	if !validInterfaceName(name) {
		return fmt.Errorf("interface name is invalid")
	}
	item, err := p.hostInterface(name)
	if err != nil {
		return err
	}
	if item.Status == "up" {
		return fmt.Errorf("interface must be stopped before deletion")
	}
	if _, err = p.output("virsh", "--connect", p.libvirtURI, "iface-undefine", name); err != nil {
		return err
	}
	return p.cleanupDeletedHostInterfaceLink(item)
}

func (p *VirshProvider) hostInterface(name string) (HostInterface, error) {
	items, err := p.ListHostInterfaces()
	if err != nil {
		return HostInterface{}, err
	}
	for _, item := range items {
		if item.Name == name {
			return item, nil
		}
	}
	return HostInterface{}, fmt.Errorf("interface not found")
}

func (p *VirshProvider) interfaceExists(name string) (bool, error) {
	_, err := p.output("ip", "link", "show", name)
	if err == nil {
		return true, nil
	}
	if strings.Contains(strings.ToLower(err.Error()), "does not exist") || strings.Contains(strings.ToLower(err.Error()), "cannot find device") {
		return false, nil
	}
	return false, err
}

func (p *VirshProvider) cleanupDeletedHostInterfaceLink(item HostInterface) error {
	name := strings.TrimSpace(item.Name)
	if !shouldCleanupDeletedHostInterfaceLink(item) {
		return nil
	}
	exists, err := p.interfaceExists(name)
	if err != nil {
		return fmt.Errorf("check deleted interface runtime link failed: %w", err)
	}
	if !exists {
		return nil
	}
	if !p.runtimeInterfaceIsBridge(name) {
		return fmt.Errorf("interface definition deleted but runtime non-bridge device %s still exists", name)
	}
	_, _ = p.output("ip", "link", "set", name, "down")
	if _, err := p.output("ip", "link", "delete", name, "type", "bridge"); err != nil {
		return fmt.Errorf("interface definition deleted but runtime bridge device still exists: %w", err)
	}
	exists, err = p.interfaceExists(name)
	if err != nil {
		return fmt.Errorf("verify deleted interface runtime link failed: %w", err)
	}
	if exists {
		return fmt.Errorf("interface definition deleted but runtime bridge device %s still exists", name)
	}
	return nil
}

func shouldCleanupDeletedHostInterfaceLink(item HostInterface) bool {
	return strings.EqualFold(strings.TrimSpace(item.Type), "bridge") && strings.TrimSpace(item.Name) != ""
}

func (p *VirshProvider) runtimeInterfaceIsBridge(name string) bool {
	if strings.TrimSpace(name) == "" {
		return false
	}
	_, err := p.output("sh", "-c", "test -d /sys/class/net/"+shellQuote(name)+"/bridge")
	return err == nil
}

func parseLinkLine(line string) HostInterface {
	line = strings.TrimSpace(line)
	if line == "" {
		return HostInterface{}
	}
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return HostInterface{}
	}
	name := strings.TrimSuffix(fields[1], ":")
	if at := strings.Index(name, "@"); at >= 0 {
		name = name[:at]
	}
	item := HostInterface{Name: name, Type: "ethernet", Status: "unknown"}
	if strings.Contains(line, "state UP") {
		item.Status = "up"
	} else if strings.Contains(line, "state DOWN") {
		item.Status = "down"
	}
	for index, field := range fields {
		switch field {
		case "link/loopback":
			item.Type = "loopback"
		case "link/ether":
			item.Type = "ethernet"
			if index+1 < len(fields) {
				item.MAC = strings.ToLower(fields[index+1])
			}
		case "master":
			if index+1 < len(fields) {
				item.BridgeDevice = fields[index+1]
			}
		}
	}
	return item
}

func parseAddrLine(line string) (name string, family string, address string) {
	fields := strings.Fields(strings.TrimSpace(line))
	if len(fields) < 4 {
		return "", "", ""
	}
	name = strings.TrimSuffix(fields[1], ":")
	if at := strings.Index(name, "@"); at >= 0 {
		name = name[:at]
	}
	family = fields[2]
	address = fields[3]
	return name, family, address
}

func (p *VirshProvider) detectInterfaceType(name string, fallback string) string {
	if strings.EqualFold(name, "lo") {
		return "loopback"
	}
	if _, err := p.output("sh", "-c", "test -d /sys/class/net/"+shellQuote(name)+"/bridge"); err == nil {
		return "bridge"
	}
	return fallback
}

type interfaceXMLDoc struct {
	XMLName xml.Name `xml:"interface"`
	Type    string   `xml:"type,attr"`
	Name    string   `xml:"name,attr"`
	Start   struct {
		Mode string `xml:"mode,attr"`
	} `xml:"start"`
	MAC struct {
		Address string `xml:"address,attr"`
	} `xml:"mac"`
	Protocols []struct {
		Family string    `xml:"family,attr"`
		DHCP   *struct{} `xml:"dhcp"`
		IP     struct {
			Address string `xml:"address,attr"`
			Prefix  string `xml:"prefix,attr"`
		} `xml:"ip"`
		Route struct {
			Gateway string `xml:"gateway,attr"`
		} `xml:"route"`
	} `xml:"protocol"`
	Bridge struct {
		STP       string `xml:"stp,attr"`
		Delay     string `xml:"delay,attr"`
		Interface struct {
			Name string `xml:"name,attr"`
		} `xml:"interface"`
	} `xml:"bridge"`
}

func protocolAddressMode(protocol struct {
	Family string    `xml:"family,attr"`
	DHCP   *struct{} `xml:"dhcp"`
	IP     struct {
		Address string `xml:"address,attr"`
		Prefix  string `xml:"prefix,attr"`
	} `xml:"ip"`
	Route struct {
		Gateway string `xml:"gateway,attr"`
	} `xml:"route"`
}) (string, string) {
	if strings.TrimSpace(protocol.IP.Address) != "" {
		address := strings.TrimSpace(protocol.IP.Address)
		if strings.TrimSpace(protocol.IP.Prefix) != "" {
			address += "/" + strings.TrimSpace(protocol.IP.Prefix)
		}
		return "static", address
	}
	if protocol.DHCP != nil {
		return "dhcp", ""
	}
	return "none", ""
}

func fillInterfaceDefaults(item *HostInterface) {
	if item.Type == "" {
		item.Type = "ethernet"
	}
	if item.Status == "" {
		item.Status = "unknown"
	}
	if item.BootMode == "" {
		item.BootMode = "none"
	}
	if item.IPv4Mode == "" {
		item.IPv4Mode = "none"
	}
	if item.IPv6Mode == "" {
		item.IPv6Mode = "none"
	}
}

func normalizeLibvirtInterfaceState(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "active", "up":
		return "up"
	case "inactive", "down":
		return "down"
	default:
		return normalizeInterfaceStatus(value)
	}
}

func normalizeInterfaceStatus(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "up":
		return "up"
	case "down":
		return "down"
	default:
		if strings.TrimSpace(value) == "" {
			return "unknown"
		}
		return strings.ToLower(strings.TrimSpace(value))
	}
}

func interfaceBootMode(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "none"
	}
	return "none"
}

func (p *VirshProvider) bridgeSTP(name string) string {
	out, err := p.output("sh", "-c", "cat /sys/class/net/"+shellQuote(name)+"/bridge/stp_state 2>/dev/null")
	if err != nil {
		return ""
	}
	if strings.TrimSpace(out) == "1" {
		return "on"
	}
	return "off"
}

func (p *VirshProvider) bridgeDelay(name string) string {
	out, err := p.output("sh", "-c", "cat /sys/class/net/"+shellQuote(name)+"/bridge/forward_delay 2>/dev/null")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

func validInterfaceName(name string) bool {
	if len(name) < 1 || len(name) > 15 {
		return false
	}
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' || r == '.' {
			continue
		}
		return false
	}
	return name != "." && name != ".."
}

func normalizeOnOff(value string, fallback string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "on", "yes", "true", "1":
		return "on"
	case "off", "no", "false", "0":
		return "off"
	default:
		return fallback
	}
}

func parseInterfaceCIDR(address string, family string) (net.IP, *net.IPNet, error) {
	ip, network, err := net.ParseCIDR(strings.TrimSpace(address))
	if err != nil || ip == nil || network == nil {
		return nil, nil, fmt.Errorf("%s address is invalid", family)
	}
	if family == "ipv4" && ip.To4() == nil {
		return nil, nil, fmt.Errorf("ipv4 address is invalid")
	}
	if family == "ipv6" && ip.To4() != nil {
		return nil, nil, fmt.Errorf("ipv6 address is invalid")
	}
	return ip, network, nil
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

func xmlEscapeAttr(value string) string {
	value = strings.ReplaceAll(value, "&", "&amp;")
	value = strings.ReplaceAll(value, "'", "&apos;")
	value = strings.ReplaceAll(value, "\"", "&quot;")
	value = strings.ReplaceAll(value, "<", "&lt;")
	value = strings.ReplaceAll(value, ">", "&gt;")
	return value
}
