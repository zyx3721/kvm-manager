package kvm

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func (p *VirshProvider) CloneVM(vmName string, request VMCloneRequest) (VMConfig, error) {
	request.Name = strings.TrimSpace(request.Name)
	if request.Name == "" {
		return VMConfig{}, fmt.Errorf("clone vm name is required")
	}
	if request.Autostart {
		request.CDROMPolicy = "disconnect"
	}
	if err := validateVMCloneOptions(request); err != nil {
		return VMConfig{}, err
	}
	if request.MaximumCPU > 0 || request.MaximumMemoryMB > 0 {
		if err := p.validateRequestedHostResources(request.MaximumCPU, request.MaximumMemoryMB); err != nil {
			return VMConfig{}, err
		}
	}
	if len(request.Disks) == 0 {
		return VMConfig{}, fmt.Errorf("clone disk is required")
	}
	stateOut, _ := p.output("virsh", "--connect", p.libvirtURI, "domstate", vmName)
	if normalizeState(stateOut) == "running" {
		return VMConfig{}, fmt.Errorf("vm is running")
	}
	if out, err := p.output("virsh", "--connect", p.libvirtURI, "dominfo", request.Name); err == nil && strings.TrimSpace(out) != "" {
		return VMConfig{}, fmt.Errorf("clone vm already exists")
	}

	sourceConfig, err := p.vmConfig(vmName, false)
	if err != nil {
		return VMConfig{}, err
	}
	diskRequests, err := validateVMCloneDisks(request.Disks, sourceConfig.Disks)
	if err != nil {
		return VMConfig{}, err
	}
	networkPoolSources := p.networkPoolSourcesByName()
	interfaceMACs := vmCloneInterfaceMACsBySource(request.Interfaces, sourceConfig.Interfaces)
	interfaceSources := vmCloneInterfaceSourcesByName(request.Interfaces, sourceConfig.Interfaces, networkPoolSources)

	createdVolumes := make([]struct {
		Pool string
		Name string
	}, 0, len(diskRequests))
	diskPaths := make(map[string]string, len(diskRequests))
	for _, disk := range diskRequests {
		sourcePool := storagePoolNameForPath(disk.SourcePath, p.storagePoolTargets())
		if sourcePool == "" {
			sourcePool = disk.Pool
		}
		volume, err := p.CloneStorageVolumeToPool(sourcePool, disk.Pool, StorageVolumeCloneRequest{
			Name:             disk.TargetName,
			SourceName:       filepath.Base(disk.SourcePath),
			PreallocMetadata: disk.PreallocMetadata,
		})
		if err != nil {
			cleanupVMCloneVolumes(p, createdVolumes)
			return VMConfig{}, err
		}
		createdVolumes = append(createdVolumes, struct {
			Pool string
			Name string
		}{Pool: disk.Pool, Name: volume.Name})
		diskPaths[disk.SourcePath] = volume.Path
	}

	nextXML, err := buildCloneDomainXML(sourceConfig.XML, request, diskPaths, interfaceMACs, interfaceSources)
	if err != nil {
		cleanupVMCloneVolumes(p, createdVolumes)
		return VMConfig{}, err
	}
	path, err := writeTempXML(nextXML)
	if err != nil {
		cleanupVMCloneVolumes(p, createdVolumes)
		return VMConfig{}, err
	}
	defer os.Remove(path)
	if _, err := p.output("virsh", "--connect", p.libvirtURI, "define", path); err != nil {
		cleanupVMCloneVolumes(p, createdVolumes)
		return VMConfig{}, err
	}
	if err := p.updateDescription(request.Name, request.Description); err != nil {
		return VMConfig{}, err
	}
	if request.Autostart {
		if err := p.StartVM(request.Name); err != nil {
			return VMConfig{}, err
		}
	}
	config, err := p.vmConfig(request.Name, false)
	if err != nil {
		return VMConfig{}, err
	}
	return config, nil
}

func validateVMCloneOptions(request VMCloneRequest) error {
	if len(request.Description) > 2048 {
		return fmt.Errorf("description is too long")
	}
	if request.CurrentCPU < 0 || request.MaximumCPU < 0 || request.CurrentMemoryMB < 0 || request.MaximumMemoryMB < 0 {
		return fmt.Errorf("clone resource values must not be negative")
	}
	if request.CurrentCPU > 0 && request.MaximumCPU > 0 && request.CurrentCPU > request.MaximumCPU {
		return fmt.Errorf("current cpu cannot exceed maximum cpu")
	}
	if request.CurrentMemoryMB > 0 && request.MaximumMemoryMB > 0 && request.CurrentMemoryMB > request.MaximumMemoryMB {
		return fmt.Errorf("current memory cannot exceed maximum memory")
	}
	policy := strings.ToLower(strings.TrimSpace(request.CDROMPolicy))
	if policy != "" && policy != "inherit" && policy != "disconnect" {
		return fmt.Errorf("clone cdrom policy is invalid")
	}
	return nil
}

func validateVMCloneDisks(requests []VMCloneDiskRequest, sourceDisks []VMConfigDisk) ([]VMCloneDiskRequest, error) {
	sourceByName := make(map[string]VMConfigDisk, len(sourceDisks))
	for _, disk := range sourceDisks {
		sourceByName[disk.Name] = disk
	}
	items := make([]VMCloneDiskRequest, 0, len(requests))
	seenTargets := make(map[string]bool, len(requests))
	for _, request := range requests {
		request.Name = strings.TrimSpace(request.Name)
		request.Pool = strings.TrimSpace(request.Pool)
		request.SourcePath = strings.TrimSpace(request.SourcePath)
		request.TargetName = strings.TrimSpace(request.TargetName)
		if request.Name == "" || request.Pool == "" || request.TargetName == "" {
			return nil, fmt.Errorf("clone disk name, pool and target are required")
		}
		source, ok := sourceByName[request.Name]
		if !ok {
			return nil, fmt.Errorf("clone disk target not found")
		}
		if request.SourcePath == "" {
			request.SourcePath = source.Path
		}
		if source.Path == "" || request.SourcePath != source.Path {
			return nil, fmt.Errorf("clone disk source path mismatch")
		}
		if seenTargets[request.Pool+"/"+request.TargetName] {
			return nil, fmt.Errorf("clone disk target duplicated")
		}
		seenTargets[request.Pool+"/"+request.TargetName] = true
		items = append(items, request)
	}
	return items, nil
}

func vmCloneInterfaceMACsBySource(requests []VMCloneInterfaceRequest, sourceInterfaces []VMConfigInterface) map[string]string {
	result := make(map[string]string, len(sourceInterfaces))
	requestByName := make(map[string]string, len(requests))
	requestByIndex := make(map[int]string, len(requests))
	for index, request := range requests {
		name := strings.TrimSpace(request.Name)
		mac := strings.TrimSpace(request.MAC)
		if name != "" && mac != "" {
			requestByName[name] = mac
		}
		if mac != "" {
			requestByIndex[index] = mac
		}
	}
	for index, iface := range sourceInterfaces {
		if iface.MAC == "" {
			continue
		}
		if mac := requestByName[iface.Name]; mac != "" {
			result[iface.MAC] = mac
			continue
		}
		if mac := requestByIndex[index]; mac != "" {
			result[iface.MAC] = mac
			continue
		}
		result[iface.MAC] = randomQEMUMAC()
	}
	return result
}

func vmCloneInterfaceSourcesByName(requests []VMCloneInterfaceRequest, sourceInterfaces []VMConfigInterface, networkPoolSources map[string]string) map[string]string {
	validSources := make(map[string]bool, len(sourceInterfaces))
	macByName := make(map[string]string, len(sourceInterfaces))
	macByIndex := make(map[int]string, len(sourceInterfaces))
	typeByName := make(map[string]string, len(sourceInterfaces))
	typeByIndex := make(map[int]string, len(sourceInterfaces))
	for index, iface := range sourceInterfaces {
		if strings.TrimSpace(iface.Name) != "" {
			validSources[iface.Name] = true
			macByName[iface.Name] = iface.MAC
			typeByName[iface.Name] = iface.Type
		}
		if strings.TrimSpace(iface.MAC) != "" {
			macByIndex[index] = iface.MAC
		}
		typeByIndex[index] = iface.Type
	}
	result := make(map[string]string, len(sourceInterfaces))
	for index, request := range requests {
		name := strings.TrimSpace(request.Name)
		source := strings.TrimSpace(request.Source)
		if name != "" && source != "" && validSources[name] {
			source = resolveInterfaceSourceValue(typeByName[name], source, networkPoolSources)
			result[name] = source
			if mac := macByName[name]; mac != "" {
				result[mac] = source
			}
			continue
		}
		if source != "" {
			source = resolveInterfaceSourceValue(typeByIndex[index], source, networkPoolSources)
			if mac := macByIndex[index]; mac != "" {
				result[mac] = source
			}
		}
	}
	return result
}

func buildCloneDomainXML(input string, request VMCloneRequest, diskPaths map[string]string, interfaceMACs map[string]string, interfaceSources map[string]string) (string, error) {
	decoder := xml.NewDecoder(strings.NewReader(input))
	var output bytes.Buffer
	encoder := xml.NewEncoder(&output)
	var stack []string
	skipDepth := 0
	cdromPolicy := strings.ToLower(strings.TrimSpace(request.CDROMPolicy))
	currentInterfaceMAC := ""
	currentInterfaceTarget := ""
	currentInterfaceType := ""

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
			if skipDepth > 0 {
				skipDepth++
				continue
			}
			parent := currentParent(stack)
			stack = append(stack, item.Name.Local)
			if parent == "domain" && item.Name.Local == "uuid" {
				skipDepth = 1
				stack = stack[:len(stack)-1]
				continue
			}
			if parent == "domain" && item.Name.Local == "description" {
				skipDepth = 1
				stack = stack[:len(stack)-1]
				continue
			}
			if parent == "domain" && (item.Name.Local == "memory" || item.Name.Local == "currentMemory") && request.MaximumMemoryMB > 0 {
				setAttr(&item.Attr, "unit", "KiB")
			}
			if parent == "domain" && item.Name.Local == "vcpu" && request.MaximumCPU > 0 {
				setAttr(&item.Attr, "current", strconv.Itoa(firstPositiveInt(request.CurrentCPU, request.MaximumCPU)))
			}
			if parent == "disk" && item.Name.Local == "source" {
				sourcePath := firstNonEmptyString(attrValue(item.Attr, "file"), attrValue(item.Attr, "dev"))
				if path := diskPaths[sourcePath]; path != "" {
					setDiskSourcePath(&item.Attr, sourcePath, path)
				} else if cdromPolicy == "disconnect" {
					item.Attr = removeDiskSourcePathAttrs(item.Attr)
				}
			}
			if parent == "interface" && item.Name.Local == "target" {
				currentInterfaceTarget = attrValue(item.Attr, "dev")
				skipDepth = 1
				stack = stack[:len(stack)-1]
				continue
			}
			if parent == "interface" && item.Name.Local == "source" {
				if source := firstNonEmptyString(interfaceSources[currentInterfaceTarget], interfaceSources[currentInterfaceMAC]); source != "" {
					item.Attr = setInterfaceSourceAttrs(item.Attr, interfaceSourceAttrName(currentInterfaceType), source)
				}
			}
			if parent == "interface" && item.Name.Local == "mac" {
				currentInterfaceMAC = attrValue(item.Attr, "address")
				if mac := interfaceMACs[attrValue(item.Attr, "address")]; mac != "" {
					setAttr(&item.Attr, "address", mac)
				}
			}
			if item.Name.Local == "interface" {
				currentInterfaceType = attrValue(item.Attr, "type")
			}
			if err := encoder.EncodeToken(item); err != nil {
				return "", err
			}
		case xml.EndElement:
			if skipDepth > 0 {
				skipDepth--
				continue
			}
			if err := encoder.EncodeToken(item); err != nil {
				return "", err
			}
			if item.Name.Local == "interface" {
				currentInterfaceMAC = ""
				currentInterfaceTarget = ""
				currentInterfaceType = ""
			}
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		case xml.CharData:
			if skipDepth > 0 {
				continue
			}
			if len(stack) == 2 && stack[0] == "domain" && stack[1] == "name" {
				if err := encoder.EncodeToken(xml.CharData([]byte(request.Name))); err != nil {
					return "", err
				}
				continue
			}
			if len(stack) == 2 && stack[0] == "domain" && stack[1] == "memory" && request.MaximumMemoryMB > 0 {
				if err := encoder.EncodeToken(xml.CharData([]byte(strconv.FormatInt(request.MaximumMemoryMB*mibToKiB, 10)))); err != nil {
					return "", err
				}
				continue
			}
			if len(stack) == 2 && stack[0] == "domain" && stack[1] == "currentMemory" && request.CurrentMemoryMB > 0 {
				if err := encoder.EncodeToken(xml.CharData([]byte(strconv.FormatInt(firstPositiveInt64(request.CurrentMemoryMB, request.MaximumMemoryMB)*mibToKiB, 10)))); err != nil {
					return "", err
				}
				continue
			}
			if len(stack) == 2 && stack[0] == "domain" && stack[1] == "vcpu" && request.MaximumCPU > 0 {
				if err := encoder.EncodeToken(xml.CharData([]byte(strconv.Itoa(request.MaximumCPU)))); err != nil {
					return "", err
				}
				continue
			}
			if err := encoder.EncodeToken(item); err != nil {
				return "", err
			}
		default:
			if skipDepth > 0 {
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

func firstPositiveInt(values ...int) int {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func firstPositiveInt64(values ...int64) int64 {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func currentParent(stack []string) string {
	if len(stack) == 0 {
		return ""
	}
	return stack[len(stack)-1]
}

func attrValue(attrs []xml.Attr, name string) string {
	for _, attr := range attrs {
		if attr.Name.Local == name {
			return attr.Value
		}
	}
	return ""
}

func setAttr(attrs *[]xml.Attr, name string, value string) {
	for i := range *attrs {
		if (*attrs)[i].Name.Local == name {
			(*attrs)[i].Value = value
			return
		}
	}
	*attrs = append(*attrs, xml.Attr{Name: xml.Name{Local: name}, Value: value})
}

func setDiskSourcePath(attrs *[]xml.Attr, sourcePath string, value string) {
	for i := range *attrs {
		if (*attrs)[i].Name.Local == "file" && (*attrs)[i].Value == sourcePath {
			(*attrs)[i].Value = value
			return
		}
	}
	for i := range *attrs {
		if (*attrs)[i].Name.Local == "dev" && (*attrs)[i].Value == sourcePath {
			(*attrs)[i].Value = value
			return
		}
	}
	setAttr(attrs, "file", value)
}

func setInterfaceSourceAttrs(attrs []xml.Attr, sourceAttr string, source string) []xml.Attr {
	next := attrs[:0]
	for _, attr := range attrs {
		if attr.Name.Local == "network" || attr.Name.Local == "bridge" || attr.Name.Local == "dev" {
			continue
		}
		next = append(next, attr)
	}
	next = append(next, xml.Attr{Name: xml.Name{Local: sourceAttr}, Value: source})
	return next
}

func interfaceSourceAttrName(interfaceType string) string {
	switch strings.ToLower(strings.TrimSpace(interfaceType)) {
	case "bridge":
		return "bridge"
	case "direct":
		return "dev"
	default:
		return "network"
	}
}

func resolveInterfaceSourceValue(interfaceType string, source string, networkPoolSources map[string]string) string {
	source = strings.TrimSpace(source)
	if strings.EqualFold(strings.TrimSpace(interfaceType), "bridge") {
		if bridge := strings.TrimSpace(networkPoolSources[source]); bridge != "" {
			return bridge
		}
	}
	return source
}

func removeDiskSourcePathAttrs(attrs []xml.Attr) []xml.Attr {
	next := attrs[:0]
	for _, attr := range attrs {
		if attr.Name.Local == "file" || attr.Name.Local == "dev" {
			continue
		}
		next = append(next, attr)
	}
	return next
}

func writeTempXML(content string) (string, error) {
	file, err := os.CreateTemp("", "kvm-manager-clone-*.xml")
	if err != nil {
		return "", err
	}
	path := file.Name()
	if _, err := file.WriteString(content); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return "", err
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return "", err
	}
	return path, nil
}

func randomQEMUMAC() string {
	buf := make([]byte, 3)
	if _, err := rand.Read(buf); err != nil {
		return "52:54:00:00:00:00"
	}
	return "52:54:00:" + hex.EncodeToString(buf[0:1]) + ":" + hex.EncodeToString(buf[1:2]) + ":" + hex.EncodeToString(buf[2:3])
}

func cleanupVMCloneVolumes(p *VirshProvider, volumes []struct {
	Pool string
	Name string
}) {
	for i := len(volumes) - 1; i >= 0; i-- {
		_ = p.DeleteStorageVolume(volumes[i].Pool, volumes[i].Name)
	}
}
