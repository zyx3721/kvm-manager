package kvm

import (
	"fmt"
	"strings"
)

func buildHostInterfaceXML(request HostInterfaceCreateRequest) (string, error) {
	mode := strings.ToLower(strings.TrimSpace(request.StartMode))
	if mode == "" {
		mode = "none"
	}
	itype := strings.ToLower(strings.TrimSpace(request.Type))
	if itype == "" {
		itype = "bridge"
	}
	xmlText := fmt.Sprintf("<interface type='%s' name='%s'><start mode='%s'/>", xmlEscapeAttr(itype), xmlEscapeAttr(request.Name), xmlEscapeAttr(mode))
	xmlText += buildInterfaceProtocolXML("ipv4", request.IPv4Mode, request.IPv4Address, request.IPv4Gateway)
	xmlText += buildInterfaceProtocolXML("ipv6", request.IPv6Mode, request.IPv6Address, request.IPv6Gateway)
	if itype == "bridge" {
		xmlText += fmt.Sprintf("<bridge stp='%s' delay='%s'>", xmlEscapeAttr(normalizeOnOff(request.STP, "on")), xmlEscapeAttr(firstNonEmptyString(request.Delay, "0")))
		if strings.TrimSpace(request.Device) != "" {
			xmlText += fmt.Sprintf("<interface name='%s' type='ethernet'/>", xmlEscapeAttr(request.Device))
		}
		xmlText += "</bridge>"
	} else if strings.TrimSpace(request.Device) != "" {
		xmlText += fmt.Sprintf("<link dev='%s'/>", xmlEscapeAttr(request.Device))
	}
	xmlText += "</interface>"
	return xmlText, nil
}

func buildInterfaceProtocolXML(family string, mode string, address string, gateway string) string {
	mode = strings.ToLower(strings.TrimSpace(mode))
	switch mode {
	case "dhcp":
		return fmt.Sprintf("<protocol family='%s'><dhcp/></protocol>", family)
	case "static":
		ip, prefix, _ := strings.Cut(strings.TrimSpace(address), "/")
		xmlText := fmt.Sprintf("<protocol family='%s'><ip address='%s' prefix='%s'/>", family, xmlEscapeAttr(ip), xmlEscapeAttr(prefix))
		if strings.TrimSpace(gateway) != "" {
			xmlText += fmt.Sprintf("<route gateway='%s'/>", xmlEscapeAttr(gateway))
		}
		return xmlText + "</protocol>"
	default:
		return ""
	}
}

func validInterfaceStartMode(mode string) bool {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "none", "onboot", "hotplug":
		return true
	default:
		return false
	}
}
