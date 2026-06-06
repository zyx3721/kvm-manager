package kvm

import (
	"encoding/xml"
	"strings"
)

func (p *VirshProvider) ListHostInterfaceDevices() ([]HostInterfaceDevice, error) {
	out, err := p.output("virsh", "--connect", p.libvirtURI, "nodedev-list", "--cap", "net")
	if err != nil {
		return p.listHostInterfaceDevicesText()
	}
	seen := map[string]struct{}{}
	items := make([]HostInterfaceDevice, 0)
	for _, line := range strings.Split(out, "\n") {
		nodeName := strings.TrimSpace(line)
		if nodeName == "" {
			continue
		}
		device, err := p.nodeDeviceInterfaceName(nodeName)
		if err != nil || device == "" {
			continue
		}
		if _, ok := seen[device]; ok {
			continue
		}
		seen[device] = struct{}{}
		items = append(items, HostInterfaceDevice{Name: device})
	}
	if len(items) > 0 {
		return items, nil
	}
	return p.listHostInterfaceDevicesText()
}

func (p *VirshProvider) listHostInterfaceDevicesText() ([]HostInterfaceDevice, error) {
	items, err := p.listHostInterfacesText()
	if err != nil {
		return nil, err
	}
	result := make([]HostInterfaceDevice, 0, len(items))
	for _, item := range items {
		if strings.TrimSpace(item.Name) == "" {
			continue
		}
		result = append(result, HostInterfaceDevice{Name: item.Name})
	}
	return result, nil
}

func (p *VirshProvider) nodeDeviceInterfaceName(nodeName string) (string, error) {
	xmlText, err := p.output("virsh", "--connect", p.libvirtURI, "nodedev-dumpxml", nodeName)
	if err != nil {
		return "", err
	}
	var doc nodeDeviceXMLDoc
	if err := xml.Unmarshal([]byte(xmlText), &doc); err != nil {
		return "", err
	}
	return strings.TrimSpace(doc.Capability.Interface), nil
}

type nodeDeviceXMLDoc struct {
	Capability struct {
		Type      string `xml:"type,attr"`
		Interface string `xml:"interface"`
	} `xml:"capability"`
}
