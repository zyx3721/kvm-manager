package kvm

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

const mibToKiB = 1024

func (p *VirshProvider) VMConfig(vmName string) (VMConfig, error) {
	return p.vmConfig(vmName, true)
}

func (p *VirshProvider) vmConfig(vmName string, includeDiskCapacity bool) (VMConfig, error) {
	xmlOut, doc, err := p.persistentDomainXML(vmName)
	if err != nil {
		return VMConfig{}, err
	}
	graphics := configGraphics(doc)
	if securityXML, err := p.output("virsh", "--connect", p.libvirtURI, "dumpxml", vmName, "--security-info"); err == nil {
		graphics = mergeConfigGraphics(graphics, configGraphicsFromXML(securityXML))
	}
	stateOut, _ := p.output("virsh", "--connect", p.libvirtURI, "domstate", vmName)
	infoOut, _ := p.output("virsh", "--connect", p.libvirtURI, "dominfo", vmName)
	nodeOut, _ := p.output("virsh", "--connect", p.libvirtURI, "nodeinfo")

	currentCPU := doc.VCPU.Current
	if currentCPU <= 0 {
		currentCPU = doc.VCPU.Value
	}
	memoryDoc := doc
	if normalizeState(stateOut) == "running" {
		if _, inactiveDoc, err := p.inactiveDomainXML(vmName); err == nil {
			memoryDoc = inactiveDoc
		}
	}
	currentMemory, maximumMemory := configMemoryKiB(memoryDoc)

	interfaces := configInterfaces(doc)
	if liveXML, err := p.output("virsh", "--connect", p.libvirtURI, "dumpxml", vmName); err == nil {
		if liveDoc, parseErr := parseDomainXML(liveXML); parseErr == nil {
			interfaces = mergeConfigInterfaces(interfaces, configInterfaces(liveDoc))
		}
	}

	return VMConfig{
		Name:               doc.Name,
		UUID:               doc.UUID,
		OSType:             p.detectOSType(vmName, doc),
		Status:             normalizeState(stateOut),
		Description:        strings.TrimSpace(doc.Description),
		Autostart:          parseDominfoAutostart(infoOut),
		CurrentCPU:         currentCPU,
		MaximumCPU:         doc.VCPU.Value,
		HostCPU:            parseNodeCPU(nodeOut),
		Arch:               strings.TrimSpace(doc.OS.Type.Arch),
		CurrentMemoryBytes: kibToBytes(currentMemory),
		MaximumMemoryBytes: kibToBytes(maximumMemory),
		HostMemoryBytes:    parseNodeMemory(nodeOut),
		MemoryStatsPeriod:  memoryStatsPeriodFromXML(xmlOut),
		Disks:              p.configDisks(vmName, doc, includeDiskCapacity),
		Interfaces:         interfaces,
		CDROMs:             configCDROMs(doc),
		Graphics:           graphics,
		XML:                xmlOut,
	}, nil
}

func (p *VirshProvider) UpdateVMConfig(vmName string, request VMConfigUpdateRequest) (VMConfig, error) {
	if err := validateVMConfigUpdate(request); err != nil {
		return VMConfig{}, err
	}
	stateOut, _ := p.output("virsh", "--connect", p.libvirtURI, "domstate", vmName)
	config, err := p.vmConfig(vmName, false)
	if err != nil {
		return VMConfig{}, err
	}
	running := normalizeState(stateOut) == "running"
	cpuOrMemoryChanged := request.CurrentCPU != config.CurrentCPU ||
		request.MaximumCPU != config.MaximumCPU ||
		request.CurrentMemoryMB != config.CurrentMemoryBytes/(1024*1024) ||
		request.MaximumMemoryMB != config.MaximumMemoryBytes/(1024*1024)
	if cpuOrMemoryChanged {
		if running {
			if err := validateRunningVMConfigExpansion(request, config); err != nil {
				return VMConfig{}, err
			}
			if err := p.updateLiveCPUConfig(vmName, request, config); err != nil {
				return VMConfig{}, err
			}
			if err := p.updateLiveMemoryConfig(vmName, request, config); err != nil {
				return VMConfig{}, err
			}
		} else if err := p.updateCPUConfig(vmName, request); err != nil {
			return VMConfig{}, err
		} else if err := p.updateMemoryConfig(vmName, request); err != nil {
			return VMConfig{}, err
		}
	}
	if request.MemoryStatsPeriod != config.MemoryStatsPeriod {
		if err := p.updateMemoryStatsPeriod(vmName, request.MemoryStatsPeriod, running); err != nil {
			return VMConfig{}, err
		}
	}
	if err := p.updateDescription(vmName, request.Description); err != nil {
		return VMConfig{}, err
	}
	return p.vmConfig(vmName, false)
}

func (p *VirshProvider) UpdateVMAutostart(vmName string, request VMAutostartUpdateRequest) error {
	return p.updateAutostart(vmName, request.Autostart)
}

func (p *VirshProvider) UpdateVMConsole(vmName string, request VMConsoleUpdateRequest) (VMConfig, error) {
	request.Password = strings.TrimSpace(request.Password)
	if request.PasswordEnabled && request.Password == "" {
		return VMConfig{}, fmt.Errorf("console password is required")
	}
	stateOut, _ := p.output("virsh", "--connect", p.libvirtURI, "domstate", vmName)
	running := normalizeState(stateOut) == "running"
	if running && !request.PasswordEnabled {
		if securityXML, err := p.output("virsh", "--connect", p.libvirtURI, "dumpxml", vmName, "--security-info"); err == nil && domainConsolePasswordEnabled(securityXML) {
			return VMConfig{}, fmt.Errorf("running vm console password cannot be disabled")
		}
	}
	scope := "--config"
	if running {
		scope = "--live --config"
	}
	xmlOut, err := p.output("virsh", "--connect", p.libvirtURI, "dumpxml", vmName, "--security-info")
	if err != nil {
		return VMConfig{}, err
	}
	deviceXML, err := updateVNCGraphicsDeviceXML(xmlOut, request)
	if err != nil {
		return VMConfig{}, err
	}
	if err := p.updateDeviceXML(vmName, deviceXML, strings.Fields(scope)...); err != nil {
		return VMConfig{}, err
	}
	config, err := p.vmConfig(vmName, false)
	if err != nil {
		return VMConfig{}, err
	}
	config.Graphics.PasswordEnabled = false
	if securityXML, err := p.output("virsh", "--connect", p.libvirtURI, "dumpxml", vmName, "--security-info"); err == nil {
		config.Graphics.PasswordEnabled = domainConsolePasswordEnabled(securityXML)
	}
	if request.PasswordEnabled && !config.Graphics.PasswordEnabled {
		return VMConfig{}, fmt.Errorf("console password was not applied")
	}
	if !request.PasswordEnabled && config.Graphics.PasswordEnabled {
		return VMConfig{}, fmt.Errorf("console password was not removed")
	}
	return config, nil
}

func (p *VirshProvider) RenameVM(vmName string, request VMRenameRequest) (VMConfig, error) {
	nextName := strings.TrimSpace(request.Name)
	if nextName == "" {
		return VMConfig{}, fmt.Errorf("vm name is required")
	}
	if nextName == vmName {
		return p.vmConfig(vmName, false)
	}
	stateOut, _ := p.output("virsh", "--connect", p.libvirtURI, "domstate", vmName)
	if normalizeState(stateOut) == "running" {
		return VMConfig{}, fmt.Errorf("vm is running")
	}
	if _, err := p.output("virsh", "--connect", p.libvirtURI, "domrename", vmName, nextName); err != nil {
		return VMConfig{}, err
	}
	return p.vmConfig(nextName, false)
}

func (p *VirshProvider) UpdateVMXML(vmName string, request VMXMLUpdateRequest) (VMConfig, error) {
	request.XML = strings.TrimSpace(request.XML)
	if request.XML == "" {
		return VMConfig{}, fmt.Errorf("vm xml is required")
	}
	stateOut, _ := p.output("virsh", "--connect", p.libvirtURI, "domstate", vmName)
	if normalizeState(stateOut) == "running" {
		return VMConfig{}, fmt.Errorf("vm is running")
	}
	var doc domainXML
	if err := xml.Unmarshal([]byte(request.XML), &doc); err != nil {
		return VMConfig{}, fmt.Errorf("vm xml is invalid")
	}
	if strings.TrimSpace(doc.Name) == "" {
		return VMConfig{}, fmt.Errorf("vm xml name is required")
	}
	if strings.TrimSpace(doc.Name) != vmName {
		return VMConfig{}, fmt.Errorf("vm xml name mismatch")
	}
	nextXML := request.XML
	if securityXML, err := p.vmSecurityXML(vmName, ""); err == nil {
		if mergedXML, mergeErr := preserveSecurityGraphicsPassword(nextXML, securityXML); mergeErr == nil {
			nextXML = mergedXML
		}
	}
	path, err := writeTempXML(nextXML)
	if err != nil {
		return VMConfig{}, err
	}
	defer os.Remove(path)
	if _, err := p.output("virsh", "--connect", p.libvirtURI, "define", path); err != nil {
		return VMConfig{}, err
	}
	return p.vmConfig(vmName, false)
}

func preserveSecurityGraphicsPassword(input string, securityInput string) (string, error) {
	password := securityVNCGraphicsPassword(securityInput)
	if password == "" || !hasVNCGraphics(input) || domainConsolePasswordEnabled(input) {
		return input, nil
	}
	decoder := xml.NewDecoder(strings.NewReader(input))
	var output bytes.Buffer
	encoder := xml.NewEncoder(&output)
	applied := false

	for {
		token, err := decoder.Token()
		if err != nil {
			if err == io.EOF {
				break
			}
			return "", err
		}
		if item, ok := token.(xml.StartElement); ok && item.Name.Local == "graphics" && strings.EqualFold(attrValue(item.Attr, "type"), "vnc") && !applied {
			item.Attr = setOrRemoveAttr(item.Attr, "passwd", true, password)
			token = item
			applied = true
		}
		if err := encoder.EncodeToken(token); err != nil {
			return "", err
		}
	}
	if err := encoder.Flush(); err != nil {
		return "", err
	}
	return output.String(), nil
}

func securityVNCGraphicsPassword(input string) string {
	decoder := xml.NewDecoder(strings.NewReader(input))
	for {
		token, err := decoder.Token()
		if err != nil {
			return ""
		}
		item, ok := token.(xml.StartElement)
		if !ok || item.Name.Local != "graphics" || !strings.EqualFold(attrValue(item.Attr, "type"), "vnc") {
			continue
		}
		return strings.TrimSpace(attrValue(item.Attr, "passwd"))
	}
}

func hasVNCGraphics(input string) bool {
	decoder := xml.NewDecoder(strings.NewReader(input))
	for {
		token, err := decoder.Token()
		if err != nil {
			return false
		}
		item, ok := token.(xml.StartElement)
		if ok && item.Name.Local == "graphics" && strings.EqualFold(attrValue(item.Attr, "type"), "vnc") {
			return true
		}
	}
}

func (p *VirshProvider) UpdateVMDevices(vmName string, request VMDeviceUpdateRequest) (VMConfig, error) {
	stateOut, _ := p.output("virsh", "--connect", p.libvirtURI, "domstate", vmName)
	config, err := p.vmConfig(vmName, false)
	if err != nil {
		return VMConfig{}, err
	}
	if normalizeState(stateOut) == "running" {
		return p.updateLiveVMDevicesFromConfig(vmName, request, config)
	}
	return p.updateVMDevicesFromConfig(vmName, request, config)
}

func validateVMConfigUpdate(request VMConfigUpdateRequest) error {
	if request.CurrentCPU <= 0 || request.MaximumCPU <= 0 {
		return fmt.Errorf("cpu values must be positive")
	}
	if request.CurrentCPU > request.MaximumCPU {
		return fmt.Errorf("current cpu cannot exceed maximum cpu")
	}
	if request.CurrentMemoryMB <= 0 || request.MaximumMemoryMB <= 0 {
		return fmt.Errorf("memory values must be positive")
	}
	if request.CurrentMemoryMB > request.MaximumMemoryMB {
		return fmt.Errorf("current memory cannot exceed maximum memory")
	}
	if len(request.Description) > 2048 {
		return fmt.Errorf("description is too long")
	}
	if request.MemoryStatsPeriod < 0 || request.MemoryStatsPeriod > 86400 {
		return fmt.Errorf("memory stats period is invalid")
	}
	return nil
}

func updateDomainDeviceXML(input string, interfaceSources map[string]domainInterfaceSourceUpdate, newInterfaces []domainInterfaceXMLFragment, deletedInterfaces map[string]bool, newDisks []domainDiskXMLFragment, deletedDisks []vmDiskDeleteAction) (string, error) {
	decoder := xml.NewDecoder(strings.NewReader(input))
	var output bytes.Buffer
	encoder := xml.NewEncoder(&output)
	var stack []string
	interfaceDepth := 0
	var interfaceTokens []xml.Token
	diskDepth := 0
	var diskTokens []xml.Token
	deletedDiskTargets := map[string]bool{}
	for _, disk := range deletedDisks {
		deletedDiskTargets[strings.TrimSpace(disk.Target)] = true
	}

	for {
		token, err := decoder.Token()
		if err != nil {
			if err == io.EOF {
				break
			}
			return "", err
		}
		switch item := token.(type) {
		case xml.StartElement:
			if diskDepth > 0 {
				diskDepth++
				diskTokens = append(diskTokens, item)
				continue
			}
			if interfaceDepth > 0 {
				interfaceDepth++
				interfaceTokens = append(interfaceTokens, item)
				continue
			}
			stack = append(stack, item.Name.Local)
			if item.Name.Local == "interface" {
				interfaceDepth = 1
				interfaceTokens = []xml.Token{item}
				continue
			}
			if item.Name.Local == "disk" {
				diskDepth = 1
				diskTokens = []xml.Token{item}
				continue
			}
			if err := encoder.EncodeToken(item); err != nil {
				return "", err
			}
		case xml.EndElement:
			if diskDepth > 0 {
				diskTokens = append(diskTokens, item)
				diskDepth--
				if diskDepth == 0 {
					if !shouldDeleteDiskTokens(diskTokens, deletedDiskTargets) {
						for _, token := range diskTokens {
							if err := encoder.EncodeToken(token); err != nil {
								return "", err
							}
						}
					}
					diskTokens = nil
					if len(stack) > 0 {
						stack = stack[:len(stack)-1]
					}
				}
				continue
			}
			if interfaceDepth > 0 {
				interfaceTokens = append(interfaceTokens, item)
				interfaceDepth--
				if interfaceDepth == 0 {
					if !shouldDeleteInterfaceTokens(interfaceTokens, deletedInterfaces) {
						if err := encodeUpdatedInterfaceTokens(encoder, interfaceTokens, interfaceSources); err != nil {
							return "", err
						}
					}
					interfaceTokens = nil
					if len(stack) > 0 {
						stack = stack[:len(stack)-1]
					}
				}
				continue
			}
			if item.Name.Local == "devices" {
				for _, iface := range newInterfaces {
					if err := encodeDomainInterfaceXMLFragment(encoder, iface); err != nil {
						return "", err
					}
				}
				for _, disk := range newDisks {
					if err := encodeDomainDiskXMLFragment(encoder, disk); err != nil {
						return "", err
					}
				}
			}
			if err := encoder.EncodeToken(item); err != nil {
				return "", err
			}
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		default:
			if diskDepth > 0 {
				diskTokens = append(diskTokens, token)
				continue
			}
			if interfaceDepth > 0 {
				interfaceTokens = append(interfaceTokens, token)
				continue
			}
			if err := encoder.EncodeToken(token); err != nil {
				return "", err
			}
		}
	}
	if err := encoder.Flush(); err != nil {
		return "", err
	}
	return output.String(), nil
}

func rewriteDomainMemoryXML(input string, currentKiB int64, maximumKiB int64) (string, error) {
	decoder := xml.NewDecoder(strings.NewReader(input))
	var output bytes.Buffer
	encoder := xml.NewEncoder(&output)
	stack := make([]string, 0)
	rewriteValue := int64(0)
	memoryUpdated := false
	currentMemoryUpdated := false

	for {
		token, err := decoder.Token()
		if err != nil {
			if err == io.EOF {
				break
			}
			return "", err
		}
		switch item := token.(type) {
		case xml.StartElement:
			stack = append(stack, item.Name.Local)
			if len(stack) == 2 && stack[0] == "domain" {
				switch stack[1] {
				case "memory":
					rewriteValue = maximumKiB
					memoryUpdated = true
				case "currentMemory":
					rewriteValue = currentKiB
					currentMemoryUpdated = true
				}
			}
			if err := encoder.EncodeToken(item); err != nil {
				return "", err
			}
		case xml.EndElement:
			if err := encoder.EncodeToken(item); err != nil {
				return "", err
			}
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
			rewriteValue = 0
		case xml.CharData:
			if rewriteValue > 0 {
				token = xml.CharData([]byte(strconv.FormatInt(rewriteValue, 10)))
			}
			if err := encoder.EncodeToken(token); err != nil {
				return "", err
			}
		default:
			if err := encoder.EncodeToken(token); err != nil {
				return "", err
			}
		}
	}
	if err := encoder.Flush(); err != nil {
		return "", err
	}
	if !memoryUpdated || !currentMemoryUpdated {
		return "", fmt.Errorf("domain memory nodes not found")
	}
	return output.String(), nil
}

func (p *VirshProvider) updateCPUConfig(vmName string, request VMConfigUpdateRequest) error {
	maxCPU := strconv.Itoa(request.MaximumCPU)
	currentCPU := strconv.Itoa(request.CurrentCPU)

	if _, err := p.output("virsh", "--connect", p.libvirtURI, "setvcpus", vmName, maxCPU, "--maximum", "--config"); err != nil {
		return err
	}
	if _, err := p.output("virsh", "--connect", p.libvirtURI, "setvcpus", vmName, currentCPU, "--config"); err != nil {
		return err
	}
	return nil
}

func (p *VirshProvider) updateMemoryConfig(vmName string, request VMConfigUpdateRequest) error {
	maxKiB := strconv.FormatInt(request.MaximumMemoryMB*mibToKiB, 10)
	currentKiB := strconv.FormatInt(request.CurrentMemoryMB*mibToKiB, 10)

	if _, err := p.output("virsh", "--connect", p.libvirtURI, "setmaxmem", vmName, maxKiB, "--config"); err != nil {
		return err
	}
	if _, err := p.output("virsh", "--connect", p.libvirtURI, "setmem", vmName, currentKiB, "--config"); err != nil {
		return err
	}
	return nil
}

func (p *VirshProvider) updateDescription(vmName string, description string) error {
	description = strings.TrimSpace(description)
	stateOut, _ := p.output("virsh", "--connect", p.libvirtURI, "domstate", vmName)
	running := normalizeState(stateOut) == "running"
	if description == "" {
		if running {
			_, err := p.output("virsh", "--connect", p.libvirtURI, "desc", vmName, "--live", "--config", "--new-desc", "")
			return err
		}
		_, err := p.output("virsh", "--connect", p.libvirtURI, "desc", vmName, "--config", "--new-desc", "")
		return err
	}
	args := []string{"--connect", p.libvirtURI, "desc", vmName, "--config", "--new-desc", description}
	if running {
		args = []string{"--connect", p.libvirtURI, "desc", vmName, "--live", "--config", "--new-desc", description}
	}
	_, err := p.output("virsh", args...)
	return err
}

func (p *VirshProvider) defineXML(nextXML string) error {
	file, err := os.CreateTemp("", "kvm-manager-domain-*.xml")
	if err != nil {
		return err
	}
	path := file.Name()
	defer os.Remove(path)
	if _, err := file.WriteString(nextXML); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	_, err = p.output("virsh", "--connect", p.libvirtURI, "define", path)
	return err
}

func (p *VirshProvider) updateDeviceXML(vmName string, deviceXML string, scopes ...string) error {
	file, err := os.CreateTemp("", "kvm-manager-device-*.xml")
	if err != nil {
		return err
	}
	path := file.Name()
	defer os.Remove(path)
	if _, err := file.WriteString(deviceXML); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	args := []string{"--connect", p.libvirtURI, "update-device", vmName, path}
	for _, scope := range scopes {
		if strings.TrimSpace(scope) != "" {
			args = append(args, strings.TrimSpace(scope))
		}
	}
	_, err = p.output("virsh", args...)
	return err
}

func updateDomainConsoleXML(input string, request VMConsoleUpdateRequest) (string, error) {
	decoder := xml.NewDecoder(strings.NewReader(input))
	var output bytes.Buffer
	encoder := xml.NewEncoder(&output)
	updated := false

	for {
		token, err := decoder.Token()
		if err != nil {
			if err == io.EOF {
				break
			}
			return "", err
		}
		if item, ok := token.(xml.StartElement); ok && item.Name.Local == "graphics" && attrValue(item.Attr, "type") == "vnc" {
			item.Attr = setOrRemoveAttr(item.Attr, "passwd", request.PasswordEnabled, request.Password)
			token = item
			updated = true
		}
		if err := encoder.EncodeToken(token); err != nil {
			return "", err
		}
	}
	if err := encoder.Flush(); err != nil {
		return "", err
	}
	if !updated {
		return "", fmt.Errorf("vnc graphics not found")
	}
	return output.String(), nil
}

func domainConsolePasswordEnabled(input string) bool {
	decoder := xml.NewDecoder(strings.NewReader(input))
	for {
		token, err := decoder.Token()
		if err != nil {
			return false
		}
		item, ok := token.(xml.StartElement)
		if !ok || item.Name.Local != "graphics" || !strings.EqualFold(attrValue(item.Attr, "type"), "vnc") {
			continue
		}
		return strings.TrimSpace(attrValue(item.Attr, "passwd")) != ""
	}
}

func updateVNCGraphicsDeviceXML(input string, request VMConsoleUpdateRequest) (string, error) {
	decoder := xml.NewDecoder(strings.NewReader(input))
	var output bytes.Buffer
	encoder := xml.NewEncoder(&output)
	capturing := false
	depth := 0

	for {
		token, err := decoder.Token()
		if err != nil {
			if err == io.EOF {
				break
			}
			return "", err
		}
		switch item := token.(type) {
		case xml.StartElement:
			if !capturing && item.Name.Local == "graphics" && attrValue(item.Attr, "type") == "vnc" {
				item.Attr = setOrRemoveAttr(item.Attr, "passwd", request.PasswordEnabled, request.Password)
				capturing = true
				depth = 1
				if err := encoder.EncodeToken(item); err != nil {
					return "", err
				}
				continue
			}
			if capturing {
				depth++
				if err := encoder.EncodeToken(item); err != nil {
					return "", err
				}
			}
		case xml.EndElement:
			if capturing {
				if err := encoder.EncodeToken(item); err != nil {
					return "", err
				}
				depth--
				if depth == 0 {
					if err := encoder.Flush(); err != nil {
						return "", err
					}
					return output.String(), nil
				}
			}
		default:
			if capturing {
				if err := encoder.EncodeToken(token); err != nil {
					return "", err
				}
			}
		}
	}
	return "", fmt.Errorf("vnc graphics not found")
}

func setOrRemoveAttr(attrs []xml.Attr, name string, keep bool, value string) []xml.Attr {
	next := make([]xml.Attr, 0, len(attrs)+1)
	found := false
	for _, attr := range attrs {
		if attr.Name.Local != name {
			next = append(next, attr)
			continue
		}
		found = true
		if keep {
			attr.Value = value
			next = append(next, attr)
		}
	}
	if keep && !found {
		next = append(next, xml.Attr{Name: xml.Name{Local: name}, Value: value})
	}
	return next
}

func (p *VirshProvider) updateAutostart(vmName string, autostart bool) error {
	args := []string{"--connect", p.libvirtURI, "autostart", vmName}
	if !autostart {
		args = append(args, "--disable")
	}
	_, err := p.output("virsh", args...)
	return err
}

func (p *VirshProvider) configDisks(vmName string, doc domainXML, includeCapacity bool) []VMConfigDisk {
	items := make([]VMConfigDisk, 0)
	pools := p.storagePoolTargets()
	for _, disk := range doc.Devices.Disks {
		if disk.Device != "disk" {
			continue
		}
		path := disk.Source.File
		if path == "" {
			path = disk.Source.Dev
		}
		sourcePath := diskBaseSourcePath(disk)
		item := VMConfigDisk{
			Name:       disk.Target.Dev,
			Path:       path,
			SourcePath: sourcePath,
			Pool:       storagePoolNameForPath(firstNonEmptyString(sourcePath, path), pools),
			Bus:        disk.Target.Bus,
			Device:     disk.Device,
			Type:       disk.Type,
		}
		if includeCapacity && path != "" {
			if out, err := p.output("virsh", "--connect", p.libvirtURI, "domblkinfo", vmName, path); err == nil {
				item.Bytes = parseDomblkInfoBytes(out, "Capacity:")
			}
			if item.Bytes <= 0 {
				if out, err := p.output("qemu-img", "info", "-U", "--output=json", path); err == nil {
					item.Bytes = parseJSONInt64(out, "virtual-size")
				}
			}
		}
		items = append(items, item)
	}
	return items
}

func diskBaseSourcePath(disk domainDiskXML) string {
	source := strings.TrimSpace(firstNonEmptyString(disk.Source.File, disk.Source.Dev))
	for backing := disk.BackingStore; backing != nil; backing = backing.BackingStore {
		backingSource := strings.TrimSpace(firstNonEmptyString(backing.Source.File, backing.Source.Dev))
		if backingSource != "" {
			source = backingSource
		}
	}
	return source
}

type storagePoolTarget struct {
	Name string
	Path string
}

func (p *VirshProvider) storagePoolTargets() []storagePoolTarget {
	out, err := p.output("virsh", "--connect", p.libvirtURI, "pool-list", "--all", "--name")
	if err != nil {
		return nil
	}
	items := make([]storagePoolTarget, 0)
	for _, line := range strings.Split(out, "\n") {
		name := strings.TrimSpace(line)
		if name == "" {
			continue
		}
		dump, err := p.output("virsh", "--connect", p.libvirtURI, "pool-dumpxml", name)
		if err != nil {
			continue
		}
		doc := storagePoolXML{}
		if err := xml.Unmarshal([]byte(dump), &doc); err != nil {
			continue
		}
		path := cleanPoolPath(doc.Target.Path)
		if path == "" {
			continue
		}
		items = append(items, storagePoolTarget{Name: name, Path: path})
	}
	return items
}

func storagePoolNameForPath(path string, pools []storagePoolTarget) string {
	path = cleanPoolPath(path)
	if path == "" {
		return ""
	}
	bestName := ""
	bestLength := 0
	for _, pool := range pools {
		poolPath := cleanPoolPath(pool.Path)
		if poolPath == "" || !pathHasPoolPrefix(path, poolPath) {
			continue
		}
		if len(poolPath) > bestLength {
			bestName = pool.Name
			bestLength = len(poolPath)
		}
	}
	return bestName
}

func pathHasPoolPrefix(path string, poolPath string) bool {
	if path == poolPath {
		return true
	}
	if !strings.HasPrefix(path, poolPath) {
		return false
	}
	remainder := strings.TrimPrefix(path, poolPath)
	return strings.HasPrefix(remainder, "/") || strings.HasPrefix(remainder, "\\")
}

func configInterfaces(doc domainXML) []VMConfigInterface {
	items := make([]VMConfigInterface, 0, len(doc.Devices.Interfaces))
	for _, iface := range doc.Devices.Interfaces {
		items = append(items, VMConfigInterface{
			Name:   iface.Target.Dev,
			MAC:    iface.MAC.Address,
			Type:   iface.Type,
			Source: firstNonEmptyString(iface.Source.Network, iface.Source.Bridge, iface.Source.Dev),
			Model:  iface.Model.Type,
		})
	}
	return items
}

func mergeConfigInterfaces(base []VMConfigInterface, live []VMConfigInterface) []VMConfigInterface {
	liveByMAC := make(map[string]VMConfigInterface, len(live))
	for _, iface := range live {
		mac := strings.ToLower(strings.TrimSpace(iface.MAC))
		if mac != "" {
			liveByMAC[mac] = iface
		}
	}
	next := make([]VMConfigInterface, 0, len(base)+len(live))
	seen := map[string]bool{}
	for _, iface := range base {
		mac := strings.ToLower(strings.TrimSpace(iface.MAC))
		if liveIface, ok := liveByMAC[mac]; ok {
			iface.Name = firstNonEmptyString(liveIface.Name, iface.Name)
			iface.Type = firstNonEmptyString(liveIface.Type, iface.Type)
			iface.Source = firstNonEmptyString(liveIface.Source, iface.Source)
			iface.Model = firstNonEmptyString(liveIface.Model, iface.Model)
		}
		if mac != "" {
			seen[mac] = true
		}
		next = append(next, iface)
	}
	for _, iface := range live {
		mac := strings.ToLower(strings.TrimSpace(iface.MAC))
		if mac != "" && seen[mac] {
			continue
		}
		next = append(next, iface)
	}
	return next
}

func configCDROMs(doc domainXML) []VMConfigCDROM {
	items := make([]VMConfigCDROM, 0)
	for _, disk := range doc.Devices.Disks {
		if disk.Device != "cdrom" {
			continue
		}
		path := disk.Source.File
		if path == "" {
			path = disk.Source.Dev
		}
		items = append(items, VMConfigCDROM{Name: disk.Target.Dev, Path: path, Bus: disk.Target.Bus, Connected: path != ""})
	}
	return items
}

func configGraphics(doc domainXML) VMConfigGraphics {
	for _, graphics := range doc.Devices.Graphics {
		if !strings.EqualFold(strings.TrimSpace(graphics.Type), "vnc") {
			continue
		}
		return VMConfigGraphics{
			Type:            "vnc",
			Listen:          strings.TrimSpace(graphics.Listen),
			Port:            strings.TrimSpace(graphics.Port),
			PasswordEnabled: strings.TrimSpace(graphics.Passwd) != "",
		}
	}
	for _, graphics := range doc.Devices.Graphics {
		if strings.TrimSpace(graphics.Type) == "" {
			continue
		}
		return VMConfigGraphics{
			Type:            strings.TrimSpace(graphics.Type),
			Listen:          strings.TrimSpace(graphics.Listen),
			Port:            strings.TrimSpace(graphics.Port),
			PasswordEnabled: strings.TrimSpace(graphics.Passwd) != "",
		}
	}
	return VMConfigGraphics{}
}

func configGraphicsFromXML(input string) VMConfigGraphics {
	doc := domainXML{}
	if err := xml.Unmarshal([]byte(input), &doc); err != nil {
		return VMConfigGraphics{}
	}
	return configGraphics(doc)
}

func mergeConfigGraphics(base VMConfigGraphics, next VMConfigGraphics) VMConfigGraphics {
	if base.Type == "" && next.Type != "" {
		base.Type = next.Type
	}
	if base.Listen == "" && next.Listen != "" {
		base.Listen = next.Listen
	}
	if base.Port == "" && next.Port != "" {
		base.Port = next.Port
	}
	base.PasswordEnabled = base.PasswordEnabled || next.PasswordEnabled
	return base
}

func parseDominfoAutostart(out string) bool {
	for _, line := range strings.Split(out, "\n") {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 || !strings.EqualFold(strings.TrimSpace(parts[0]), "Autostart") {
			continue
		}
		value := strings.ToLower(strings.TrimSpace(parts[1]))
		return value == "enable" || value == "enabled" || value == "yes"
	}
	return false
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
