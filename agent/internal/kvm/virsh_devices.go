package kvm

import (
	"encoding/xml"
	"fmt"
	"os"
	"strconv"
	"strings"
)

type domainDiskXMLFragment struct {
	Path   string
	Target string
	Bus    string
	Format string
}

type domainInterfaceXMLFragment struct {
	Type       string
	SourceAttr string
	Source     string
	Model      string
}

type domainInterfaceSourceUpdate struct {
	Type       string
	SourceAttr string
	Source     string
}

type vmInterfaceUpdateAction struct {
	Current VMConfigInterface
	Update  domainInterfaceSourceUpdate
}

type vmInterfaceDeleteAction struct {
	Current VMConfigInterface
}

func (p *VirshProvider) updateVMDevicesFromConfig(vmName string, request VMDeviceUpdateRequest, config VMConfig) (VMConfig, error) {
	interfaceUpdates, newInterfaces, deletedInterfaces, diskResizes, newDisks, deletedDisks, err := p.validateVMDeviceUpdateRequest(request, config)
	if err != nil {
		return VMConfig{}, err
	}
	xmlOut, err := p.vmSecurityXML(vmName, config.XML)
	if err != nil {
		return VMConfig{}, err
	}

	createdVolumes := make([]struct {
		Pool string
		Name string
	}, 0, len(newDisks))
	diskFragments := make([]domainDiskXMLFragment, 0, len(newDisks))
	for _, disk := range newDisks {
		volume, err := p.CreateStorageVolume(disk.Pool, StorageVolumeCreateRequest{
			Name:             disk.Name,
			Format:           disk.Format,
			CapacityBytes:    disk.CapacityBytes,
			PreallocMetadata: disk.PreallocMetadata,
		})
		if err != nil {
			cleanupVMCloneVolumes(p, createdVolumes)
			return VMConfig{}, err
		}
		createdVolumes = append(createdVolumes, struct {
			Pool string
			Name string
		}{Pool: disk.Pool, Name: disk.Name})
		diskFragments = append(diskFragments, domainDiskXMLFragment{
			Path:   volume.Path,
			Target: disk.Target,
			Bus:    disk.Bus,
			Format: disk.Format,
		})
	}

	nextXML, err := updateDomainDeviceXML(xmlOut, interfaceSourceUpdatesByKey(interfaceUpdates), newInterfaces, deletedInterfaceKeys(deletedInterfaces), diskFragments, deletedDisks)
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
	for _, disk := range diskResizes {
		if err := p.ensureDiskResizable(disk.Path); err != nil {
			return VMConfig{}, err
		}
		if _, err := p.output("qemu-img", "resize", disk.Path, qemuImgResizeSizeArg(disk.CapacityBytes)); err != nil {
			return VMConfig{}, friendlyDiskResizeError(err)
		}
	}
	for _, disk := range deletedDisks {
		if disk.Pool != "" && disk.VolumeName != "" {
			if err := p.DeleteStorageVolume(disk.Pool, disk.VolumeName); err != nil {
				return VMConfig{}, err
			}
		}
	}
	return p.vmConfig(vmName, true)
}

func qemuImgResizeSizeArg(capacityBytes int64) string {
	return strconv.FormatInt(capacityBytes, 10)
}

func (p *VirshProvider) ensureDiskResizable(path string) error {
	out, err := p.output("qemu-img", "info", "--output=json", path)
	if err != nil {
		return friendlyDiskResizeError(err)
	}
	if qemuImgInfoHasSnapshots(out) {
		return fmt.Errorf("disk image has internal snapshots and cannot be resized")
	}
	return nil
}

func qemuImgInfoHasSnapshots(out string) bool {
	return strings.Contains(strings.ToLower(out), `"snapshots"`)
}

func friendlyDiskResizeError(err error) error {
	lower := strings.ToLower(err.Error())
	if strings.Contains(lower, "image has snapshots") ||
		strings.Contains(lower, "has internal snapshots") ||
		strings.Contains(lower, "does not support resize") {
		return fmt.Errorf("disk image has internal snapshots and cannot be resized")
	}
	return err
}

func (p *VirshProvider) updateLiveVMDevicesFromConfig(vmName string, request VMDeviceUpdateRequest, config VMConfig) (VMConfig, error) {
	if len(request.DeletedDisks) > 0 {
		return VMConfig{}, fmt.Errorf("live device update does not support disk deletion")
	}
	if len(request.Interfaces) > 0 || len(request.NewInterfaces) > 0 || len(request.DeletedInterfaces) > 0 {
		return VMConfig{}, fmt.Errorf("live device update does not support interface changes")
	}
	interfaceUpdates, err := p.validateVMDeviceInterfaceUpdates(request.Interfaces, config)
	if err != nil {
		return VMConfig{}, err
	}
	newInterfaces, err := p.validateVMDeviceNewInterfaces(request.NewInterfaces)
	if err != nil {
		return VMConfig{}, err
	}
	deletedInterfaces, err := validateVMDeviceDeletedInterfaces(request.DeletedInterfaces, config)
	if err != nil {
		return VMConfig{}, err
	}
	diskResizes, err := validateVMDeviceDiskResizes(request.DiskResizes, config)
	if err != nil {
		return VMConfig{}, err
	}
	newDisks, err := p.validateVMDeviceNewDisks(request.NewDisks, config)
	if err != nil {
		return VMConfig{}, err
	}
	for _, disk := range diskResizes {
		if _, err := p.output("virsh", "--connect", p.libvirtURI, "blockresize", vmName, disk.Target, strconv.FormatInt(disk.CapacityBytes, 10)+"B"); err != nil {
			return VMConfig{}, err
		}
	}
	for _, disk := range newDisks {
		volume, err := p.CreateStorageVolume(disk.Pool, StorageVolumeCreateRequest{
			Name:             disk.Name,
			Format:           disk.Format,
			CapacityBytes:    disk.CapacityBytes,
			PreallocMetadata: disk.PreallocMetadata,
		})
		if err != nil {
			return VMConfig{}, err
		}
		if _, err := p.output("virsh", "--connect", p.libvirtURI, "attach-disk", vmName, volume.Path, disk.Target, "--targetbus", disk.Bus, "--driver", "qemu", "--subdriver", disk.Format, "--live", "--config"); err != nil {
			cleanupVMCloneVolumes(p, []struct {
				Pool string
				Name string
			}{{Pool: disk.Pool, Name: disk.Name}})
			return VMConfig{}, err
		}
	}
	for _, iface := range interfaceUpdates {
		if err := p.changeLiveInterface(vmName, iface); err != nil {
			return VMConfig{}, err
		}
	}
	for _, iface := range deletedInterfaces {
		if err := p.detachLiveInterface(vmName, iface.Current); err != nil {
			return VMConfig{}, err
		}
	}
	for _, iface := range newInterfaces {
		if err := p.attachLiveInterface(vmName, iface, ""); err != nil {
			return VMConfig{}, err
		}
	}
	return p.vmConfig(vmName, true)
}

func (p *VirshProvider) validateVMDeviceUpdateRequest(request VMDeviceUpdateRequest, config VMConfig) ([]vmInterfaceUpdateAction, []domainInterfaceXMLFragment, []vmInterfaceDeleteAction, []vmDiskResizeAction, []VMDeviceNewDiskRequest, []vmDiskDeleteAction, error) {
	interfaceUpdates, err := p.validateVMDeviceInterfaceUpdates(request.Interfaces, config)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, err
	}
	newInterfaces, err := p.validateVMDeviceNewInterfaces(request.NewInterfaces)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, err
	}
	deletedInterfaces, err := validateVMDeviceDeletedInterfaces(request.DeletedInterfaces, config)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, err
	}
	diskResizes, err := validateVMDeviceDiskResizes(request.DiskResizes, config)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, err
	}
	newDisks, err := p.validateVMDeviceNewDisks(request.NewDisks, config)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, err
	}
	deletedDisks, err := validateVMDeviceDeletedDisks(request.DeletedDisks, config)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, err
	}
	return interfaceUpdates, newInterfaces, deletedInterfaces, diskResizes, newDisks, deletedDisks, nil
}

func encodeUpdatedInterfaceTokens(encoder *xml.Encoder, tokens []xml.Token, updates map[string]domainInterfaceSourceUpdate) error {
	mac, target := interfaceTokenIdentifiers(tokens)
	update, ok := firstInterfaceSourceUpdate(updates, target, mac)
	if !ok || update.Source == "" {
		for _, token := range tokens {
			if err := encoder.EncodeToken(token); err != nil {
				return err
			}
		}
		return nil
	}
	for _, token := range tokens {
		switch item := token.(type) {
		case xml.StartElement:
			if item.Name.Local == "interface" && update.Type != "" {
				setAttr(&item.Attr, "type", update.Type)
			}
			if item.Name.Local == "source" {
				attr := firstNonEmptyString(update.SourceAttr, interfaceSourceAttrName(update.Type))
				item.Attr = setInterfaceSourceAttrs(item.Attr, attr, update.Source)
			}
			if err := encoder.EncodeToken(item); err != nil {
				return err
			}
		default:
			if err := encoder.EncodeToken(token); err != nil {
				return err
			}
		}
	}
	return nil
}

func shouldDeleteInterfaceTokens(tokens []xml.Token, deleted map[string]bool) bool {
	if len(deleted) == 0 {
		return false
	}
	mac, target := interfaceTokenIdentifiers(tokens)
	return deleted[target] || deleted[strings.ToLower(mac)]
}

func shouldDeleteDiskTokens(tokens []xml.Token, deletedTargets map[string]bool) bool {
	if len(deletedTargets) == 0 {
		return false
	}
	target := ""
	device := ""
	for _, token := range tokens {
		item, ok := token.(xml.StartElement)
		if !ok {
			continue
		}
		if item.Name.Local == "disk" {
			device = attrValue(item.Attr, "device")
		}
		if item.Name.Local == "target" {
			target = attrValue(item.Attr, "dev")
		}
	}
	return (device == "" || strings.EqualFold(device, "disk")) && deletedTargets[target]
}

func interfaceTokenIdentifiers(tokens []xml.Token) (string, string) {
	mac := ""
	target := ""
	for _, token := range tokens {
		item, ok := token.(xml.StartElement)
		if !ok {
			continue
		}
		if item.Name.Local == "mac" {
			mac = attrValue(item.Attr, "address")
		}
		if item.Name.Local == "target" {
			target = attrValue(item.Attr, "dev")
		}
	}
	return mac, target
}

func firstInterfaceSourceUpdate(updates map[string]domainInterfaceSourceUpdate, keys ...string) (domainInterfaceSourceUpdate, bool) {
	for _, key := range keys {
		if strings.TrimSpace(key) == "" {
			continue
		}
		if update, ok := updates[key]; ok {
			return update, true
		}
	}
	return domainInterfaceSourceUpdate{}, false
}

type vmDiskResizeAction struct {
	Target        string
	Path          string
	CapacityBytes int64
}

type vmDiskDeleteAction struct {
	Target     string
	Path       string
	Pool       string
	VolumeName string
}

func validateVMDeviceDiskResizes(requests []VMDeviceDiskResizeRequest, config VMConfig) ([]vmDiskResizeAction, error) {
	diskByName := make(map[string]VMConfigDisk, len(config.Disks))
	for _, disk := range config.Disks {
		diskByName[disk.Name] = disk
	}
	resizes := make([]vmDiskResizeAction, 0, len(requests))
	for _, disk := range requests {
		name := strings.TrimSpace(disk.Name)
		if name == "" || disk.CapacityBytes <= 0 {
			return nil, fmt.Errorf("disk name and capacity are required")
		}
		current, ok := diskByName[name]
		if !ok {
			return nil, fmt.Errorf("disk target not found")
		}
		if strings.TrimSpace(current.Path) == "" {
			return nil, fmt.Errorf("disk source path is unavailable")
		}
		if current.Bytes > 0 && disk.CapacityBytes < current.Bytes {
			return nil, fmt.Errorf("disk capacity cannot shrink")
		}
		if current.Bytes > 0 && disk.CapacityBytes == current.Bytes {
			continue
		}
		resizes = append(resizes, vmDiskResizeAction{Target: current.Name, Path: current.Path, CapacityBytes: disk.CapacityBytes})
	}
	return resizes, nil
}

func validateVMDeviceDeletedDisks(requests []VMDeviceDeleteDiskRequest, config VMConfig) ([]vmDiskDeleteAction, error) {
	if len(requests) == 0 {
		return nil, nil
	}
	diskByName := make(map[string]VMConfigDisk, len(config.Disks))
	normalDisks := 0
	firstNormalDisk := ""
	for _, disk := range config.Disks {
		if disk.Device == "" || strings.EqualFold(disk.Device, "disk") {
			normalDisks++
			if firstNormalDisk == "" {
				firstNormalDisk = strings.TrimSpace(disk.Name)
			}
		}
		diskByName[disk.Name] = disk
	}
	if len(requests) >= normalDisks {
		return nil, fmt.Errorf("cannot delete all disks")
	}
	items := make([]vmDiskDeleteAction, 0, len(requests))
	seen := map[string]bool{}
	for _, disk := range requests {
		name := strings.TrimSpace(disk.Name)
		if name == "" {
			return nil, fmt.Errorf("disk name is required")
		}
		if seen[name] {
			continue
		}
		seen[name] = true
		current, ok := diskByName[name]
		if !ok {
			return nil, fmt.Errorf("disk target not found")
		}
		if current.Device != "" && !strings.EqualFold(current.Device, "disk") {
			return nil, fmt.Errorf("only normal disks can be deleted")
		}
		if name == firstNormalDisk {
			return nil, fmt.Errorf("first disk cannot be deleted")
		}
		if strings.TrimSpace(current.Path) == "" {
			return nil, fmt.Errorf("disk source path is unavailable")
		}
		pool, volume := diskVolumeForDelete(current)
		items = append(items, vmDiskDeleteAction{Target: current.Name, Path: current.Path, Pool: pool, VolumeName: volume})
	}
	return items, nil
}

func (p *VirshProvider) validateVMDeviceNewDisks(requests []VMDeviceNewDiskRequest, config VMConfig) ([]VMDeviceNewDiskRequest, error) {
	targets := make(map[string]bool, len(config.Disks)+len(requests))
	for _, disk := range config.Disks {
		targets[strings.TrimSpace(disk.Name)] = true
	}
	volumesByPool := make(map[string]map[string]bool)
	next := make([]VMDeviceNewDiskRequest, 0, len(requests))
	for _, disk := range requests {
		disk.Name = strings.TrimSpace(disk.Name)
		disk.Pool = strings.TrimSpace(disk.Pool)
		disk.Target = strings.TrimSpace(disk.Target)
		disk.Bus = strings.TrimSpace(disk.Bus)
		disk.Format = strings.ToLower(strings.TrimSpace(disk.Format))
		if disk.Name == "" || disk.Pool == "" || disk.Target == "" || disk.Bus == "" || disk.Format == "" || disk.CapacityBytes <= 0 {
			return nil, fmt.Errorf("new disk fields are required")
		}
		if strings.ContainsAny(disk.Name, `/\`) {
			return nil, fmt.Errorf("new disk name must not contain path separators")
		}
		if !supportedVolumeFormat(disk.Format) {
			return nil, fmt.Errorf("unsupported storage volume format")
		}
		if !storageVolumeNameMatchesFormat(disk.Name, disk.Format) {
			return nil, fmt.Errorf("new disk name extension must match format")
		}
		if targets[disk.Target] {
			return nil, fmt.Errorf("disk target already exists")
		}
		targets[disk.Target] = true
		volumes, ok := volumesByPool[disk.Pool]
		if !ok {
			items, err := p.ListStorageVolumes(disk.Pool)
			if err != nil {
				return nil, err
			}
			volumes = make(map[string]bool, len(items))
			for _, item := range items {
				volumes[item.Name] = true
			}
			volumesByPool[disk.Pool] = volumes
		}
		if volumes[disk.Name] {
			return nil, fmt.Errorf("storage volume already exists")
		}
		volumes[disk.Name] = true
		next = append(next, disk)
	}
	return next, nil
}

func storageVolumeNameMatchesFormat(name string, format string) bool {
	extension := ".img"
	if strings.EqualFold(strings.TrimSpace(format), "qcow2") {
		extension = ".qcow2"
	}
	return strings.HasSuffix(strings.ToLower(strings.TrimSpace(name)), extension)
}

func encodeDomainDiskXMLFragment(encoder *xml.Encoder, disk domainDiskXMLFragment) error {
	start := xml.StartElement{
		Name: xml.Name{Local: "disk"},
		Attr: []xml.Attr{
			{Name: xml.Name{Local: "type"}, Value: "file"},
			{Name: xml.Name{Local: "device"}, Value: "disk"},
		},
	}
	if err := encoder.EncodeToken(start); err != nil {
		return err
	}
	if err := encoder.EncodeToken(xml.StartElement{
		Name: xml.Name{Local: "driver"},
		Attr: []xml.Attr{
			{Name: xml.Name{Local: "name"}, Value: "qemu"},
			{Name: xml.Name{Local: "type"}, Value: disk.Format},
		},
	}); err != nil {
		return err
	}
	if err := encoder.EncodeToken(xml.EndElement{Name: xml.Name{Local: "driver"}}); err != nil {
		return err
	}
	if err := encoder.EncodeToken(xml.StartElement{
		Name: xml.Name{Local: "source"},
		Attr: []xml.Attr{{Name: xml.Name{Local: "file"}, Value: disk.Path}},
	}); err != nil {
		return err
	}
	if err := encoder.EncodeToken(xml.EndElement{Name: xml.Name{Local: "source"}}); err != nil {
		return err
	}
	if err := encoder.EncodeToken(xml.StartElement{
		Name: xml.Name{Local: "target"},
		Attr: []xml.Attr{
			{Name: xml.Name{Local: "dev"}, Value: disk.Target},
			{Name: xml.Name{Local: "bus"}, Value: disk.Bus},
		},
	}); err != nil {
		return err
	}
	if err := encoder.EncodeToken(xml.EndElement{Name: xml.Name{Local: "target"}}); err != nil {
		return err
	}
	return encoder.EncodeToken(xml.EndElement{Name: start.Name})
}

func encodeDomainInterfaceXMLFragment(encoder *xml.Encoder, iface domainInterfaceXMLFragment) error {
	start := xml.StartElement{
		Name: xml.Name{Local: "interface"},
		Attr: []xml.Attr{{Name: xml.Name{Local: "type"}, Value: firstNonEmptyString(iface.Type, "network")}},
	}
	if err := encoder.EncodeToken(start); err != nil {
		return err
	}
	sourceAttr := firstNonEmptyString(iface.SourceAttr, interfaceSourceAttrName(iface.Type))
	if err := encoder.EncodeToken(xml.StartElement{
		Name: xml.Name{Local: "source"},
		Attr: []xml.Attr{{Name: xml.Name{Local: sourceAttr}, Value: iface.Source}},
	}); err != nil {
		return err
	}
	if err := encoder.EncodeToken(xml.EndElement{Name: xml.Name{Local: "source"}}); err != nil {
		return err
	}
	if err := encoder.EncodeToken(xml.StartElement{
		Name: xml.Name{Local: "model"},
		Attr: []xml.Attr{{Name: xml.Name{Local: "type"}, Value: firstNonEmptyString(iface.Model, "virtio")}},
	}); err != nil {
		return err
	}
	if err := encoder.EncodeToken(xml.EndElement{Name: xml.Name{Local: "model"}}); err != nil {
		return err
	}
	return encoder.EncodeToken(xml.EndElement{Name: start.Name})
}
