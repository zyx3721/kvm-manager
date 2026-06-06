package kvm

import (
	"encoding/xml"
	"fmt"
	"strconv"
	"strings"
)

func (p *VirshProvider) ConsoleInfo(vmName string) (ConsoleInfo, error) {
	xmlOut, err := p.output("virsh", "--connect", p.libvirtURI, "dumpxml", vmName, "--security-info")
	if err != nil {
		return ConsoleInfo{}, err
	}
	var doc domainXML
	if err := xml.Unmarshal([]byte(xmlOut), &doc); err != nil {
		return ConsoleInfo{}, err
	}
	for _, graphics := range doc.Devices.Graphics {
		if strings.ToLower(strings.TrimSpace(graphics.Type)) != "vnc" {
			continue
		}
		port := 0
		if parsed, err := strconv.Atoi(strings.TrimSpace(graphics.Port)); err == nil && parsed > 0 {
			port = parsed
		}
		listen := strings.TrimSpace(graphics.Listen)
		if listen == "" || listen == "0.0.0.0" || listen == "::" || listen == "[::]" {
			listen = "127.0.0.1"
		}
		return ConsoleInfo{Type: "vnc", Listen: listen, Port: port, PasswordEnabled: strings.TrimSpace(graphics.Passwd) != ""}, nil
	}
	return ConsoleInfo{}, fmt.Errorf("vm %s has no vnc console", vmName)
}
