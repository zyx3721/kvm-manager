package kvm

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

func (p *VirshProvider) CreateHostInterface(request HostInterfaceCreateRequest) (HostInterface, error) {
	normalized, err := p.normalizeHostInterfaceCreateRequest(request)
	if err != nil {
		return HostInterface{}, err
	}
	if err := p.validateHostInterfaceAddressUniqueness(normalized); err != nil {
		return HostInterface{}, err
	}
	if err := p.backupHostInterfaceBridgeIFCFG(normalized); err != nil {
		return HostInterface{}, err
	}
	xmlText, err := buildHostInterfaceXML(normalized)
	if err != nil {
		return HostInterface{}, err
	}
	tmp, err := os.CreateTemp("", "kvm-manager-interface-*.xml")
	if err != nil {
		return HostInterface{}, err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := tmp.WriteString(xmlText); err != nil {
		_ = tmp.Close()
		return HostInterface{}, err
	}
	if err := tmp.Close(); err != nil {
		return HostInterface{}, err
	}
	if _, err := p.output("virsh", "--connect", p.libvirtURI, "iface-define", tmpPath); err != nil {
		return HostInterface{}, err
	}
	if normalized.StartMode != "none" {
		if err := p.startHostInterface(normalized.Name); err != nil {
			_, _ = p.output("virsh", "--connect", p.libvirtURI, "iface-undefine", normalized.Name)
			return HostInterface{}, err
		}
	}
	if normalized.ApplySystemConfig {
		if err := p.applyHostInterfaceSystemConfig(normalized.Name, normalized); err != nil {
			return HostInterface{}, err
		}
	}
	return p.hostInterface(normalized.Name)
}

func (p *VirshProvider) normalizeHostInterfaceCreateRequest(request HostInterfaceCreateRequest) (HostInterfaceCreateRequest, error) {
	name := strings.TrimSpace(request.Name)
	if name == "" {
		return HostInterfaceCreateRequest{}, fmt.Errorf("interface name is required")
	}
	if !validInterfaceName(name) {
		return HostInterfaceCreateRequest{}, fmt.Errorf("interface name is invalid")
	}
	startMode := strings.ToLower(strings.TrimSpace(request.StartMode))
	if startMode == "" {
		startMode = "none"
	}
	if !validInterfaceStartMode(startMode) {
		return HostInterfaceCreateRequest{}, fmt.Errorf("interface start mode is invalid")
	}
	itype := strings.ToLower(strings.TrimSpace(request.Type))
	if itype == "" {
		itype = "bridge"
	}
	if itype != "bridge" && itype != "ethernet" {
		return HostInterfaceCreateRequest{}, fmt.Errorf("unsupported interface type")
	}
	if exists, err := p.interfaceExists(name); err != nil {
		return HostInterfaceCreateRequest{}, err
	} else if exists {
		return HostInterfaceCreateRequest{}, fmt.Errorf("interface already exists")
	}
	device := strings.TrimSpace(request.Device)
	if device != "" {
		if exists, err := p.interfaceExists(device); err != nil {
			return HostInterfaceCreateRequest{}, err
		} else if !exists {
			return HostInterfaceCreateRequest{}, fmt.Errorf("interface device does not exist")
		}
		if err := p.validateHostInterfaceDeviceAvailable(device); err != nil {
			return HostInterfaceCreateRequest{}, err
		}
	}
	stp := normalizeOnOff(request.STP, "on")
	delay := strings.TrimSpace(request.Delay)
	if delay == "" {
		delay = "0"
	}
	if _, err := strconv.ParseFloat(delay, 64); err != nil {
		return HostInterfaceCreateRequest{}, fmt.Errorf("bridge delay is invalid")
	}
	if err := validateAddressMode(request.IPv4Mode, request.IPv4Address, "ipv4"); err != nil {
		return HostInterfaceCreateRequest{}, err
	}
	if err := validateAddressMode(request.IPv6Mode, request.IPv6Address, "ipv6"); err != nil {
		return HostInterfaceCreateRequest{}, err
	}
	if err := validateInterfaceGateway(request.IPv4Mode, request.IPv4Address, request.IPv4Gateway, "ipv4"); err != nil {
		return HostInterfaceCreateRequest{}, err
	}
	if err := validateInterfaceGateway(request.IPv6Mode, request.IPv6Address, request.IPv6Gateway, "ipv6"); err != nil {
		return HostInterfaceCreateRequest{}, err
	}
	if err := validateDNSServers(request.DNSServers); err != nil {
		return HostInterfaceCreateRequest{}, err
	}
	request.Name = name
	request.StartMode = startMode
	request.Device = device
	request.Type = itype
	request.STP = stp
	request.Delay = delay
	request.IPv4Mode = strings.TrimSpace(request.IPv4Mode)
	request.IPv4Address = strings.TrimSpace(request.IPv4Address)
	request.IPv4Gateway = strings.TrimSpace(request.IPv4Gateway)
	request.IPv6Mode = strings.TrimSpace(request.IPv6Mode)
	request.IPv6Address = strings.TrimSpace(request.IPv6Address)
	request.IPv6Gateway = strings.TrimSpace(request.IPv6Gateway)
	return request, nil
}

func (p *VirshProvider) validateHostInterfaceDeviceAvailable(device string) error {
	device = strings.TrimSpace(device)
	if device == "" {
		return nil
	}
	items, err := p.ListHostInterfaces()
	if err != nil {
		return err
	}
	if hostInterfaceDeviceInUse(items, device) {
		return fmt.Errorf("interface device already in use")
	}
	return nil
}

func hostInterfaceDeviceInUse(items []HostInterface, device string) bool {
	device = strings.TrimSpace(device)
	if device == "" {
		return false
	}
	for _, item := range items {
		if strings.EqualFold(strings.TrimSpace(item.BridgeDevice), device) {
			return true
		}
	}
	return false
}
