package kvm

import (
	"fmt"
	"strings"
)

func (p *VirshProvider) validateNetworkPoolEnvironment(request NetworkPoolCreateRequest) error {
	switch strings.ToLower(strings.TrimSpace(request.Type)) {
	case "nat", "route":
		out, err := p.output("sh", "-c", "sysctl -n net.ipv4.ip_forward 2>/dev/null")
		if err != nil {
			return fmt.Errorf("ip forwarding sysctl is unavailable")
		}
		if strings.TrimSpace(out) != "1" {
			return fmt.Errorf("ip forwarding is disabled")
		}
	case "bridge":
		bridge := strings.TrimSpace(request.Bridge)
		if bridge == "" {
			return fmt.Errorf("bridge device is required")
		}
		if _, err := p.output("sh", "-c", "test -d /sys/class/net/"+shellQuote(bridge)+"/bridge"); err != nil {
			return fmt.Errorf("bridge device does not exist")
		}
	}
	return nil
}
