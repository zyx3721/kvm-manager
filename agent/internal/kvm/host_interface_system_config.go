package kvm

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func validateAddressMode(mode string, address string, family string) error {
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode == "" || mode == "none" || mode == "dhcp" {
		return nil
	}
	if mode != "static" {
		return fmt.Errorf("%s mode is unsupported", family)
	}
	if strings.TrimSpace(address) == "" {
		return fmt.Errorf("%s address is required", family)
	}
	ip, _, err := net.ParseCIDR(strings.TrimSpace(address))
	if err != nil || ip == nil {
		return fmt.Errorf("%s address is invalid", family)
	}
	if family == "ipv4" && ip.To4() == nil {
		return fmt.Errorf("ipv4 address is invalid")
	}
	if family == "ipv6" && ip.To4() != nil {
		return fmt.Errorf("ipv6 address is invalid")
	}
	return nil
}

func validateInterfaceGateway(mode string, address string, gateway string, family string) error {
	if strings.ToLower(strings.TrimSpace(mode)) != "static" || strings.TrimSpace(gateway) == "" {
		return nil
	}
	ip, network, err := parseInterfaceCIDR(address, family)
	if err != nil {
		return err
	}
	gatewayIP := net.ParseIP(strings.TrimSpace(gateway))
	if gatewayIP == nil {
		return fmt.Errorf("%s gateway is invalid", family)
	}
	if family == "ipv4" {
		if gatewayIP.To4() == nil {
			return fmt.Errorf("ipv4 gateway is invalid")
		}
		gatewayIP = gatewayIP.To4()
	}
	if family == "ipv6" && gatewayIP.To4() != nil {
		return fmt.Errorf("ipv6 gateway is invalid")
	}
	if ip.Equal(gatewayIP) {
		return fmt.Errorf("%s gateway cannot equal address", family)
	}
	if !network.Contains(gatewayIP) {
		return fmt.Errorf("%s gateway must be in the same subnet", family)
	}
	return nil
}

func validateDNSServers(values []string) error {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if net.ParseIP(trimmed) == nil {
			return fmt.Errorf("dns server is invalid")
		}
	}
	return nil
}

func uniqueTrimmedStrings(values []string) []string {
	seen := map[string]struct{}{}
	items := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		items = append(items, trimmed)
	}
	return items
}

func (p *VirshProvider) applyHostInterfaceSystemConfig(name string, request HostInterfaceCreateRequest) error {
	dns := uniqueTrimmedStrings(request.DNSServers)
	if !request.ApplySystemConfig || len(dns) == 0 {
		return nil
	}
	if p.applyHostInterfaceDNSWithNMCLI(name, dns) {
		return nil
	}
	return p.applyHostInterfaceDNSWithIFCFG(name, dns)
}

func (p *VirshProvider) applyHostInterfaceDNSWithNMCLI(name string, dns []string) bool {
	if _, err := p.output("sh", "-c", "command -v nmcli >/dev/null 2>&1"); err != nil {
		return false
	}
	ipv4, ipv6 := splitDNSServers(dns)
	if len(ipv4) > 0 {
		if _, err := p.output("nmcli", "connection", "modify", name, "ipv4.ignore-auto-dns", "yes", "ipv4.dns", strings.Join(ipv4, " ")); err != nil {
			return false
		}
	}
	if len(ipv6) > 0 {
		if _, err := p.output("nmcli", "connection", "modify", name, "ipv6.ignore-auto-dns", "yes", "ipv6.dns", strings.Join(ipv6, " ")); err != nil {
			return false
		}
	}
	return true
}

func splitDNSServers(dns []string) ([]string, []string) {
	ipv4 := make([]string, 0, len(dns))
	ipv6 := make([]string, 0, len(dns))
	for _, value := range dns {
		ip := net.ParseIP(strings.TrimSpace(value))
		if ip == nil {
			continue
		}
		if ip.To4() != nil {
			ipv4 = append(ipv4, value)
		} else {
			ipv6 = append(ipv6, value)
		}
	}
	return ipv4, ipv6
}

func (p *VirshProvider) applyHostInterfaceDNSWithIFCFG(name string, dns []string) error {
	path := filepath.Join("/etc/sysconfig/network-scripts", "ifcfg-"+name)
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("system interface config not found")
	}
	backupPath := path + ".kvm-manager." + time.Now().Format("20060102150405") + ".bak"
	if err := os.WriteFile(backupPath, content, 0o600); err != nil {
		return fmt.Errorf("backup system interface config failed: %w", err)
	}
	next := updateIFCFGDNS(string(content), dns)
	if err := os.WriteFile(path, []byte(next), 0o600); err != nil {
		return fmt.Errorf("write system interface dns failed: %w", err)
	}
	if err := os.Remove(backupPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove system interface config backup failed: %w", err)
	}
	return nil
}

func updateIFCFGDNS(content string, dns []string) string {
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	next := make([]string, 0, len(lines)+len(dns))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "DNS") {
			key, _, found := strings.Cut(trimmed, "=")
			if found && isIFCFGDNSKey(key) {
				continue
			}
		}
		next = append(next, line)
	}
	for index, value := range dns {
		next = append(next, fmt.Sprintf("DNS%d=%s", index+1, value))
	}
	return strings.TrimRight(strings.Join(next, "\n"), "\n") + "\n"
}

func isIFCFGDNSKey(key string) bool {
	if !strings.HasPrefix(key, "DNS") || len(key) == 3 {
		return false
	}
	for _, r := range key[3:] {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
