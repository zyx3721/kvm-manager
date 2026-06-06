package kvm

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"strings"
)

func (p *VirshProvider) ConnectVMMedia(vmName string, request VMMediaConnectRequest) (VMConfig, error) {
	request.Target = strings.TrimSpace(request.Target)
	request.ISOPath = strings.TrimSpace(request.ISOPath)
	if request.Target == "" {
		return VMConfig{}, fmt.Errorf("cdrom target is required")
	}
	if request.ISOPath == "" {
		return VMConfig{}, fmt.Errorf("iso path is required")
	}
	config, err := p.vmConfig(vmName, false)
	if err != nil {
		return VMConfig{}, err
	}
	if normalizeState(config.Status) == "running" {
		return VMConfig{}, fmt.Errorf("vm is running")
	}
	if !hasCDROMTarget(config.CDROMs, request.Target) {
		return VMConfig{}, fmt.Errorf("target cdrom not found")
	}
	args := mediaChangeArgs(p.libvirtURI, vmName, []string{request.Target, request.ISOPath, "--insert"}, config.Status)
	if err := p.runMediaChange(args); err != nil {
		return VMConfig{}, err
	}
	if err := p.updateMediaBootOrder(vmName, request.Target, true); err != nil {
		return VMConfig{}, err
	}
	return p.vmConfig(vmName, false)
}

func (p *VirshProvider) DisconnectVMMedia(vmName string, request VMMediaDisconnectRequest) (VMConfig, error) {
	request.Target = strings.TrimSpace(request.Target)
	if request.Target == "" {
		return VMConfig{}, fmt.Errorf("cdrom target is required")
	}
	config, err := p.vmConfig(vmName, false)
	if err != nil {
		return VMConfig{}, err
	}
	if normalizeState(config.Status) == "running" {
		return VMConfig{}, fmt.Errorf("vm is running")
	}
	if !hasCDROMTarget(config.CDROMs, request.Target) {
		return VMConfig{}, fmt.Errorf("target cdrom not found")
	}
	args := mediaChangeArgs(p.libvirtURI, vmName, []string{request.Target, "--eject"}, config.Status)
	if err := p.runMediaChange(args); err != nil {
		return VMConfig{}, err
	}
	if err := p.updateMediaBootOrder(vmName, request.Target, false); err != nil {
		return VMConfig{}, err
	}
	return p.vmConfig(vmName, false)
}

func (p *VirshProvider) runMediaChange(args mediaChangeCommandArgs) error {
	if _, err := p.output("virsh", args.primary...); err != nil {
		if len(args.fallback) == 0 {
			return err
		}
		if _, fallbackErr := p.output("virsh", args.fallback...); fallbackErr != nil {
			return fallbackErr
		}
	}
	return nil
}

type mediaChangeCommandArgs struct {
	primary  []string
	fallback []string
}

func mediaChangeArgs(libvirtURI string, vmName string, actionArgs []string, status string) mediaChangeCommandArgs {
	base := append([]string{"--connect", libvirtURI, "change-media", vmName}, actionArgs...)
	primary := append(append([]string{}, base...), mediaChangeScopeArgs(status)...)
	return mediaChangeCommandArgs{primary: primary}
}

func hasCDROMTarget(cdroms []VMConfigCDROM, target string) bool {
	for _, cdrom := range cdroms {
		if cdrom.Name == target {
			return true
		}
	}
	return false
}

func mediaChangeScopeArgs(status string) []string {
	if normalizeState(status) == "running" {
		return []string{"--live", "--config"}
	}
	return []string{"--config"}
}

func (p *VirshProvider) updateMediaBootOrder(vmName string, target string, connected bool) error {
	xmlOut, _, err := p.persistentDomainXML(vmName)
	if err != nil {
		return err
	}
	xmlOut, err = p.vmSecurityXML(vmName, xmlOut)
	if err != nil {
		return err
	}
	nextXML, err := updateCDROMBootOrderXML(xmlOut, target, connected)
	if err != nil {
		return err
	}
	path, err := writeTempXML(nextXML)
	if err != nil {
		return err
	}
	defer os.Remove(path)
	_, err = p.output("virsh", "--connect", p.libvirtURI, "define", path)
	return err
}

func updateCDROMBootOrderXML(input string, target string, connected bool) (string, error) {
	target = strings.TrimSpace(target)
	if target == "" {
		return "", fmt.Errorf("cdrom target is required")
	}
	if hasOSBootElements(input) {
		output, err := updateOSBootOrderXML(input, connected)
		if err != nil {
			return "", err
		}
		if !hasCDROMTargetInXML(input, target) {
			return "", fmt.Errorf("target cdrom not found")
		}
		return output, nil
	}
	decoder := xml.NewDecoder(strings.NewReader(input))
	var output bytes.Buffer
	encoder := xml.NewEncoder(&output)
	var diskTokens []xml.Token
	diskDepth := 0
	updated := false
	normalDiskBootSet := false

	for {
		token, err := decoder.Token()
		if err != nil {
			if err == io.EOF {
				break
			}
			return "", err
		}
		if diskDepth > 0 {
			diskTokens = append(diskTokens, xml.CopyToken(token))
			switch token.(type) {
			case xml.StartElement:
				diskDepth++
			case xml.EndElement:
				diskDepth--
				if diskDepth == 0 {
					nextTokens, ok := updateDiskBootTokens(diskTokens, target, connected, &normalDiskBootSet)
					if ok {
						updated = true
					}
					for _, item := range nextTokens {
						if err := encoder.EncodeToken(item); err != nil {
							return "", err
						}
					}
					diskTokens = nil
				}
			}
			continue
		}
		if item, ok := token.(xml.StartElement); ok && item.Name.Local == "disk" {
			diskDepth = 1
			diskTokens = []xml.Token{xml.CopyToken(token)}
			continue
		}
		if err := encoder.EncodeToken(token); err != nil {
			return "", err
		}
	}
	if err := encoder.Flush(); err != nil {
		return "", err
	}
	if !updated {
		return "", fmt.Errorf("target cdrom not found")
	}
	return output.String(), nil
}

func hasOSBootElements(input string) bool {
	decoder := xml.NewDecoder(strings.NewReader(input))
	inOS := false
	osDepth := 0
	for {
		token, err := decoder.Token()
		if err != nil {
			return false
		}
		switch item := token.(type) {
		case xml.StartElement:
			if item.Name.Local == "os" && !inOS {
				inOS = true
				osDepth = 1
				continue
			}
			if inOS {
				osDepth++
				if item.Name.Local == "boot" && strings.TrimSpace(attrValue(item.Attr, "dev")) != "" {
					return true
				}
			}
		case xml.EndElement:
			if inOS {
				osDepth--
				if osDepth == 0 {
					inOS = false
				}
			}
		}
	}
}

func updateOSBootOrderXML(input string, connected bool) (string, error) {
	decoder := xml.NewDecoder(strings.NewReader(input))
	var output bytes.Buffer
	encoder := xml.NewEncoder(&output)
	var osTokens []xml.Token
	osDepth := 0
	updated := false

	for {
		token, err := decoder.Token()
		if err != nil {
			if err == io.EOF {
				break
			}
			return "", err
		}
		if osDepth > 0 {
			osTokens = append(osTokens, xml.CopyToken(token))
			switch token.(type) {
			case xml.StartElement:
				osDepth++
			case xml.EndElement:
				osDepth--
				if osDepth == 0 {
					for _, item := range rewriteOSBootTokens(osTokens, connected) {
						if err := encoder.EncodeToken(item); err != nil {
							return "", err
						}
					}
					updated = true
					osTokens = nil
				}
			}
			continue
		}
		if item, ok := token.(xml.StartElement); ok && item.Name.Local == "os" {
			osDepth = 1
			osTokens = []xml.Token{xml.CopyToken(token)}
			continue
		}
		if err := encoder.EncodeToken(token); err != nil {
			return "", err
		}
	}
	if err := encoder.Flush(); err != nil {
		return "", err
	}
	if !updated {
		return "", fmt.Errorf("domain os node not found")
	}
	return output.String(), nil
}

func rewriteOSBootTokens(tokens []xml.Token, connected bool) []xml.Token {
	next := make([]xml.Token, 0, len(tokens)+2)
	inserted := false
	skipBootDepth := 0
	for _, token := range tokens {
		if skipBootDepth > 0 {
			switch token.(type) {
			case xml.StartElement:
				skipBootDepth++
			case xml.EndElement:
				skipBootDepth--
			}
			continue
		}
		if item, ok := token.(xml.StartElement); ok && item.Name.Local == "boot" && strings.TrimSpace(attrValue(item.Attr, "dev")) != "" {
			skipBootDepth = 1
			continue
		}
		if item, ok := token.(xml.EndElement); ok && item.Name.Local == "os" && !inserted {
			if connected {
				next = appendOSBootToken(next, "cdrom")
				next = appendOSBootToken(next, "hd")
			} else {
				next = appendOSBootToken(next, "hd")
			}
			inserted = true
		}
		next = append(next, token)
	}
	return next
}

func appendOSBootToken(tokens []xml.Token, device string) []xml.Token {
	start := xml.StartElement{
		Name: xml.Name{Local: "boot"},
		Attr: []xml.Attr{{Name: xml.Name{Local: "dev"}, Value: device}},
	}
	return append(tokens, start, xml.EndElement{Name: start.Name})
}

func hasCDROMTargetInXML(input string, target string) bool {
	decoder := xml.NewDecoder(strings.NewReader(input))
	inDisk := false
	diskDepth := 0
	diskDevice := ""
	for {
		token, err := decoder.Token()
		if err != nil {
			return false
		}
		switch item := token.(type) {
		case xml.StartElement:
			if !inDisk && item.Name.Local == "disk" {
				inDisk = true
				diskDepth = 1
				diskDevice = strings.TrimSpace(attrValue(item.Attr, "device"))
				continue
			}
			if inDisk {
				diskDepth++
				if item.Name.Local == "target" && strings.EqualFold(diskDevice, "cdrom") && strings.TrimSpace(attrValue(item.Attr, "dev")) == target {
					return true
				}
			}
		case xml.EndElement:
			if inDisk {
				diskDepth--
				if diskDepth == 0 {
					inDisk = false
					diskDevice = ""
				}
			}
		}
	}
}

func updateDiskBootTokens(tokens []xml.Token, cdromTarget string, connected bool, normalDiskBootSet *bool) ([]xml.Token, bool) {
	device, target := diskTokenIdentity(tokens)
	if target == "" {
		return tokens, false
	}
	if strings.EqualFold(device, "cdrom") {
		if target != cdromTarget {
			return withoutBootTokens(tokens), false
		}
		if !connected {
			return withoutBootTokens(tokens), true
		}
		return setDiskBootOrder(tokens, 1), true
	}
	if device == "" || strings.EqualFold(device, "disk") {
		if normalDiskBootSet != nil && !*normalDiskBootSet {
			*normalDiskBootSet = true
			if connected {
				return setDiskBootOrder(tokens, 2), false
			}
			return setDiskBootOrder(tokens, 1), false
		}
		return withoutBootTokens(tokens), false
	}
	return tokens, false
}

func diskTokenIdentity(tokens []xml.Token) (string, string) {
	device := ""
	target := ""
	for _, token := range tokens {
		item, ok := token.(xml.StartElement)
		if !ok {
			continue
		}
		if item.Name.Local == "disk" {
			device = strings.TrimSpace(attrValue(item.Attr, "device"))
		}
		if item.Name.Local == "target" {
			target = strings.TrimSpace(attrValue(item.Attr, "dev"))
		}
	}
	return device, target
}

func withoutBootTokens(tokens []xml.Token) []xml.Token {
	next := make([]xml.Token, 0, len(tokens))
	skipBootDepth := 0
	for _, token := range tokens {
		if skipBootDepth > 0 {
			switch token.(type) {
			case xml.StartElement:
				skipBootDepth++
			case xml.EndElement:
				skipBootDepth--
			}
			continue
		}
		if item, ok := token.(xml.StartElement); ok && item.Name.Local == "boot" {
			skipBootDepth = 1
			continue
		}
		next = append(next, token)
	}
	return next
}

func setDiskBootOrder(tokens []xml.Token, order int) []xml.Token {
	next := make([]xml.Token, 0, len(tokens)+2)
	inserted := false
	cleaned := withoutBootTokens(tokens)
	for _, token := range cleaned {
		next = append(next, token)
		if !inserted {
			if item, ok := token.(xml.EndElement); ok && item.Name.Local == "target" {
				next = append(next, xml.StartElement{
					Name: xml.Name{Local: "boot"},
					Attr: []xml.Attr{{Name: xml.Name{Local: "order"}, Value: fmt.Sprintf("%d", order)}},
				})
				next = append(next, xml.EndElement{Name: xml.Name{Local: "boot"}})
				inserted = true
			}
		}
	}
	return next
}
