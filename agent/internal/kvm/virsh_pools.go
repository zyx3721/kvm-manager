package kvm

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func (p *VirshProvider) ListStoragePools() ([]StoragePool, error) {
	out, err := p.output("virsh", "--connect", p.libvirtURI, "pool-list", "--all", "--name")
	if err != nil {
		return nil, err
	}
	items := make([]StoragePool, 0)
	for _, line := range strings.Split(out, "\n") {
		name := strings.TrimSpace(line)
		if name == "" {
			continue
		}
		pool, err := p.storagePool(name)
		if err == nil {
			items = append(items, pool)
		}
	}
	return items, nil
}

func (p *VirshProvider) CreateStoragePool(request StoragePoolCreateRequest) (StoragePool, error) {
	request.Name = strings.TrimSpace(request.Name)
	request.Type = strings.ToLower(strings.TrimSpace(request.Type))
	if request.Type == "iso" {
		request.Type = "dir"
	}
	if request.Name == "" {
		return StoragePool{}, fmt.Errorf("storage pool name is required")
	}
	if err := validateStoragePoolCreateRequest(request); err != nil {
		return StoragePool{}, err
	}
	if err := p.defineStoragePool(request); err != nil {
		return StoragePool{}, err
	}
	if _, err := p.output("virsh", "--connect", p.libvirtURI, "pool-build", request.Name); err != nil && request.Type != "dir" {
		p.cleanupDefinedStoragePool(request.Name)
		return StoragePool{}, err
	}
	if _, err := p.output("virsh", "--connect", p.libvirtURI, "pool-start", request.Name); err != nil {
		p.cleanupDefinedStoragePool(request.Name)
		return StoragePool{}, err
	}
	_, _ = p.output("virsh", "--connect", p.libvirtURI, "pool-autostart", request.Name)
	return p.storagePool(request.Name)
}

func (p *VirshProvider) cleanupDefinedStoragePool(name string) {
	name = strings.TrimSpace(name)
	if name == "" {
		return
	}
	_, _ = p.output("virsh", "--connect", p.libvirtURI, "pool-destroy", name)
	_, _ = p.output("virsh", "--connect", p.libvirtURI, "pool-undefine", name)
}

func (p *VirshProvider) ListISOFiles(poolName string) ([]ISOFile, error) {
	volumes, err := p.ListStorageVolumes(poolName)
	if err != nil {
		return nil, err
	}
	items := make([]ISOFile, 0)
	for _, volume := range volumes {
		if !strings.HasSuffix(strings.ToLower(volume.Name), ".iso") && !strings.HasSuffix(strings.ToLower(volume.Path), ".iso") {
			continue
		}
		items = append(items, ISOFile{Name: volume.Name, Path: volume.Path, Bytes: volume.Capacity, Pool: volume.Pool})
	}
	return items, nil
}

func (p *VirshProvider) ListStorageVolumes(poolName string) ([]StorageVolume, error) {
	poolName = strings.TrimSpace(poolName)
	if poolName == "" {
		return nil, fmt.Errorf("storage pool name is required")
	}
	_, _ = p.output("virsh", "--connect", p.libvirtURI, "pool-refresh", poolName)
	out, err := p.output("virsh", "--connect", p.libvirtURI, "vol-list", poolName, "--details")
	if err != nil {
		return nil, err
	}
	return p.parseStorageVolumes(poolName, out), nil
}

func (p *VirshProvider) DeleteStorageVolume(poolName string, volumeName string) error {
	poolName = strings.TrimSpace(poolName)
	volumeName = strings.TrimSpace(volumeName)
	if poolName == "" || volumeName == "" {
		return fmt.Errorf("storage pool and volume name are required")
	}
	volume, err := p.storageVolume(poolName, volumeName)
	if err != nil {
		return err
	}
	if !strings.EqualFold(storageVolumeFormat(volume.Name, volume.Path), "iso") {
		if vmName, used := p.volumeUsedByVM(volume.Path); used {
			return fmt.Errorf("storage volume is used by virtual machine %s", vmName)
		}
	}
	_, err = p.output("virsh", "--connect", p.libvirtURI, "vol-delete", "--pool", poolName, volumeName)
	return err
}

func (p *VirshProvider) CreateStorageVolume(poolName string, request StorageVolumeCreateRequest) (StorageVolume, error) {
	poolName = strings.TrimSpace(poolName)
	request.Name = strings.TrimSpace(request.Name)
	request.Format = strings.ToLower(strings.TrimSpace(request.Format))
	if poolName == "" || request.Name == "" {
		return StorageVolume{}, fmt.Errorf("storage pool and volume name are required")
	}
	if request.CapacityBytes <= 0 {
		return StorageVolume{}, fmt.Errorf("storage volume capacity is required")
	}
	if !supportedVolumeFormat(request.Format) {
		return StorageVolume{}, fmt.Errorf("unsupported storage volume format")
	}
	args := []string{
		"--connect", p.libvirtURI,
		"vol-create-as", "--pool", poolName,
		"--name", request.Name,
		"--capacity", strconv.FormatInt(request.CapacityBytes, 10) + "B",
		"--format", request.Format,
	}
	if request.PreallocMetadata && request.Format == "qcow2" {
		args = append(args, "--prealloc-metadata")
	}
	_, err := p.output("virsh", args...)
	if err != nil {
		return StorageVolume{}, err
	}
	return p.storageVolume(poolName, request.Name)
}

func (p *VirshProvider) CloneStorageVolume(poolName string, request StorageVolumeCloneRequest) (StorageVolume, error) {
	return p.CloneStorageVolumeToPool(poolName, poolName, request)
}

func (p *VirshProvider) CloneStorageVolumeToPool(sourcePoolName string, targetPoolName string, request StorageVolumeCloneRequest) (StorageVolume, error) {
	sourcePoolName = strings.TrimSpace(sourcePoolName)
	targetPoolName = strings.TrimSpace(targetPoolName)
	request.Name = strings.TrimSpace(request.Name)
	request.SourceName = strings.TrimSpace(request.SourceName)
	request.Format = strings.ToLower(strings.TrimSpace(request.Format))
	if sourcePoolName == "" || targetPoolName == "" || request.Name == "" || request.SourceName == "" {
		return StorageVolume{}, fmt.Errorf("storage pool, source volume and target volume are required")
	}
	if strings.ContainsAny(request.Name, `/\`) {
		return StorageVolume{}, fmt.Errorf("storage volume name must not contain path separators")
	}
	source, err := p.storageVolume(sourcePoolName, request.SourceName)
	if err != nil {
		return StorageVolume{}, err
	}
	if !request.Convert && sourcePoolName == targetPoolName {
		args := []string{"--connect", p.libvirtURI, "vol-clone", "--pool", targetPoolName, request.SourceName, request.Name}
		if request.PreallocMetadata && strings.EqualFold(source.Format, "qcow2") {
			args = append(args, "--prealloc-metadata")
		}
		if _, err := p.storageOutput("virsh", args...); err != nil {
			return StorageVolume{}, err
		}
		return p.storageVolume(targetPoolName, request.Name)
	}
	if !request.Convert {
		request.Format = firstNonEmptyString(source.Format, storageVolumeFormat(source.Name, source.Path))
	}
	if !supportedVolumeFormat(request.Format) {
		return StorageVolume{}, fmt.Errorf("unsupported storage volume format")
	}
	if source.Path == "" || !filepath.IsAbs(source.Path) {
		return StorageVolume{}, fmt.Errorf("source volume path is unavailable")
	}
	targetPool, err := p.storagePool(targetPoolName)
	if err != nil {
		return StorageVolume{}, err
	}
	if strings.TrimSpace(targetPool.Path) == "" {
		return StorageVolume{}, fmt.Errorf("target storage pool path is unavailable")
	}
	targetPath := filepath.Join(targetPool.Path, request.Name)
	if _, err := os.Stat(targetPath); err == nil {
		return StorageVolume{}, fmt.Errorf("target volume already exists")
	} else if err != nil && !os.IsNotExist(err) {
		return StorageVolume{}, err
	}
	if _, err := p.storageOutput("qemu-img", "convert", "-O", request.Format, source.Path, targetPath); err != nil {
		_ = os.Remove(targetPath)
		return StorageVolume{}, err
	}
	_, _ = p.output("virsh", "--connect", p.libvirtURI, "pool-refresh", targetPoolName)
	return p.storageVolume(targetPoolName, request.Name)
}

func (p *VirshProvider) UploadStorageVolume(poolName string, request StorageVolumeCreateRequest, content io.Reader) (StorageVolume, error) {
	poolName = strings.TrimSpace(poolName)
	request.Name = strings.TrimSpace(request.Name)
	if poolName == "" || request.Name == "" {
		return StorageVolume{}, fmt.Errorf("storage pool and volume name are required")
	}
	request.Format = firstNonEmptyString(strings.ToLower(strings.TrimSpace(request.Format)), storageVolumeFormat(request.Name, request.Name))
	tmp, err := os.CreateTemp("", "kvm-manager-volume-*")
	if err != nil {
		return StorageVolume{}, err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := io.Copy(tmp, content); err != nil {
		_ = tmp.Close()
		return StorageVolume{}, err
	}
	if err := tmp.Close(); err != nil {
		return StorageVolume{}, err
	}
	info, err := os.Stat(tmpPath)
	if err != nil {
		return StorageVolume{}, err
	}
	request.CapacityBytes = info.Size()
	if request.CapacityBytes <= 0 {
		return StorageVolume{}, fmt.Errorf("storage volume capacity is required")
	}
	if _, err := p.output(
		"virsh", "--connect", p.libvirtURI,
		"vol-create-as", "--pool", poolName,
		"--name", request.Name,
		"--capacity", strconv.FormatInt(request.CapacityBytes, 10)+"B",
		"--format", request.Format,
	); err != nil {
		return StorageVolume{}, err
	}
	if _, err := p.storageOutput("virsh", "--connect", p.libvirtURI, "vol-upload", "--pool", poolName, request.Name, tmpPath); err != nil {
		_, _ = p.output("virsh", "--connect", p.libvirtURI, "vol-delete", "--pool", poolName, request.Name)
		return StorageVolume{}, err
	}
	return p.storageVolume(poolName, request.Name)
}

func (p *VirshProvider) DeleteStoragePool(poolName string) error {
	poolName = strings.TrimSpace(poolName)
	if poolName == "" {
		return fmt.Errorf("storage pool name is required")
	}
	info, err := p.output("virsh", "--connect", p.libvirtURI, "pool-info", poolName)
	if err != nil {
		return err
	}
	if strings.EqualFold(parsePoolInfoText(info, "State:"), "running") {
		return fmt.Errorf("storage pool must be stopped before deletion")
	}
	_, err = p.output("virsh", "--connect", p.libvirtURI, "pool-undefine", poolName)
	return err
}

func (p *VirshProvider) storageVolume(poolName string, volumeName string) (StorageVolume, error) {
	volumes, err := p.ListStorageVolumes(poolName)
	if err != nil {
		return StorageVolume{}, err
	}
	for _, volume := range volumes {
		if volume.Name == volumeName {
			return volume, nil
		}
	}
	return StorageVolume{}, fmt.Errorf("storage volume not found")
}

func (p *VirshProvider) UpdateStoragePoolState(poolName string, request PoolStateUpdateRequest) error {
	poolName = strings.TrimSpace(poolName)
	if poolName == "" {
		return fmt.Errorf("storage pool name is required")
	}
	action := "pool-destroy"
	if request.Active {
		action = "pool-start"
	}
	_, err := p.output("virsh", "--connect", p.libvirtURI, action, poolName)
	return err
}

func (p *VirshProvider) UpdateStoragePoolAutostart(poolName string, request PoolAutostartUpdateRequest) error {
	poolName = strings.TrimSpace(poolName)
	if poolName == "" {
		return fmt.Errorf("storage pool name is required")
	}
	args := []string{"--connect", p.libvirtURI, "pool-autostart", poolName}
	if !request.Autostart {
		args = append(args, "--disable")
	}
	_, err := p.output("virsh", args...)
	return err
}

func (p *VirshProvider) storagePool(name string) (StoragePool, error) {
	info, err := p.output("virsh", "--connect", p.libvirtURI, "pool-info", name, "--bytes")
	if err != nil {
		return StoragePool{}, err
	}
	dump, _ := p.output("virsh", "--connect", p.libvirtURI, "pool-dumpxml", name)
	doc := storagePoolXML{}
	_ = xml.Unmarshal([]byte(dump), &doc)
	return StoragePool{
		Name:           name,
		Type:           firstNonEmptyString(doc.Type, parsePoolInfoText(info, "Type:")),
		State:          strings.ToLower(parsePoolInfoText(info, "State:")),
		Autostart:      strings.EqualFold(parsePoolInfoText(info, "Autostart:"), "yes"),
		Path:           doc.Target.Path,
		CapacitySource: storagePoolCapacitySource(doc.Target.Path),
		Capacity:       parsePoolBytes(info, "Capacity:"),
		Allocation:     parsePoolBytes(info, "Allocation:"),
		Available:      parsePoolBytes(info, "Available:"),
		VolumeCount: func() int {
			volumes, err := p.ListStorageVolumes(name)
			if err != nil {
				return 0
			}
			return len(volumes)
		}(),
	}, nil
}

func (p *VirshProvider) defineStoragePool(request StoragePoolCreateRequest) error {
	switch request.Type {
	case "dir":
		if strings.TrimSpace(request.Path) == "" {
			return fmt.Errorf("storage pool path is required")
		}
		_, err := p.output("virsh", "--connect", p.libvirtURI, "pool-define-as", request.Name, "dir", "--target", request.Path)
		return err
	case "logical":
		if strings.TrimSpace(request.Device) == "" {
			return fmt.Errorf("storage pool device is required")
		}
		target := firstNonEmptyString(request.Path, "/dev/"+request.Name)
		_, err := p.output("virsh", "--connect", p.libvirtURI, "pool-define-as", request.Name, "logical", "--source-dev", request.Device, "--target", target)
		return err
	case "netfs":
		if strings.TrimSpace(request.SourceHost) == "" || strings.TrimSpace(request.SourcePath) == "" || strings.TrimSpace(request.Path) == "" {
			return fmt.Errorf("netfs host, remote path and local path are required")
		}
		args := []string{"--connect", p.libvirtURI, "pool-define-as", request.Name, "netfs", "--source-host", request.SourceHost, "--source-path", request.SourcePath, "--target", request.Path}
		if strings.TrimSpace(request.Format) != "" && !strings.EqualFold(request.Format, "auto") {
			args = append(args, "--source-format", request.Format)
		}
		_, err := p.output("virsh", args...)
		return err
	case "iscsi":
		if strings.TrimSpace(request.SourceHost) == "" || strings.TrimSpace(request.SourcePath) == "" || strings.TrimSpace(request.Path) == "" {
			return fmt.Errorf("iscsi host, target and path are required")
		}
		_, err := p.output("virsh", "--connect", p.libvirtURI, "pool-define-as", request.Name, "iscsi", "--source-host", request.SourceHost, "--source-dev", request.SourcePath, "--target", request.Path)
		return err
	default:
		return fmt.Errorf("unsupported storage pool type")
	}
}

func (p *VirshProvider) ListNetworkPools() ([]NetworkPool, error) {
	out, err := p.output("virsh", "--connect", p.libvirtURI, "net-list", "--all", "--name")
	if err != nil {
		return nil, err
	}
	items := make([]NetworkPool, 0)
	for _, line := range strings.Split(out, "\n") {
		name := strings.TrimSpace(line)
		if name == "" {
			continue
		}
		item, err := p.networkPool(name)
		if err == nil {
			items = append(items, item)
		}
	}
	return items, nil
}

func (p *VirshProvider) networkPoolSourcesByName() map[string]string {
	pools, err := p.ListNetworkPools()
	if err != nil {
		return map[string]string{}
	}
	sources := make(map[string]string, len(pools))
	for _, pool := range pools {
		if strings.TrimSpace(pool.Bridge) != "" {
			sources[pool.Name] = pool.Bridge
		}
	}
	return sources
}

func (p *VirshProvider) CreateNetworkPool(request NetworkPoolCreateRequest) (NetworkPool, error) {
	request.Name = strings.TrimSpace(request.Name)
	request.Type = strings.ToLower(strings.TrimSpace(request.Type))
	if request.Name == "" {
		return NetworkPool{}, fmt.Errorf("network name is required")
	}
	if err := p.validateNetworkPoolEnvironment(request); err != nil {
		return NetworkPool{}, err
	}
	content, err := networkXML(request)
	if err != nil {
		return NetworkPool{}, err
	}
	file, err := os.CreateTemp("", "kvm-manager-network-*.xml")
	if err != nil {
		return NetworkPool{}, err
	}
	path := file.Name()
	defer os.Remove(path)
	if _, err := io.WriteString(file, content); err != nil {
		_ = file.Close()
		return NetworkPool{}, err
	}
	if err := file.Close(); err != nil {
		return NetworkPool{}, err
	}
	if _, err := p.output("virsh", "--connect", p.libvirtURI, "net-define", path); err != nil {
		return NetworkPool{}, err
	}
	if _, err := p.output("virsh", "--connect", p.libvirtURI, "net-start", request.Name); err != nil {
		return NetworkPool{}, err
	}
	_, _ = p.output("virsh", "--connect", p.libvirtURI, "net-autostart", request.Name)
	return p.networkPool(request.Name)
}

func (p *VirshProvider) UpdateNetworkPoolState(poolName string, request PoolStateUpdateRequest) error {
	poolName = strings.TrimSpace(poolName)
	if poolName == "" {
		return fmt.Errorf("network pool name is required")
	}
	action := "net-destroy"
	if request.Active {
		action = "net-start"
	}
	_, err := p.output("virsh", "--connect", p.libvirtURI, action, poolName)
	return err
}

func (p *VirshProvider) UpdateNetworkPoolAutostart(poolName string, request PoolAutostartUpdateRequest) error {
	poolName = strings.TrimSpace(poolName)
	if poolName == "" {
		return fmt.Errorf("network pool name is required")
	}
	args := []string{"--connect", p.libvirtURI, "net-autostart", poolName}
	if !request.Autostart {
		args = append(args, "--disable")
	}
	_, err := p.output("virsh", args...)
	return err
}

func (p *VirshProvider) DeleteNetworkPool(poolName string) error {
	poolName = strings.TrimSpace(poolName)
	if poolName == "" {
		return fmt.Errorf("network pool name is required")
	}
	info, err := p.output("virsh", "--connect", p.libvirtURI, "net-info", poolName)
	if err != nil {
		return err
	}
	if strings.EqualFold(parsePoolInfoText(info, "Active:"), "yes") {
		return fmt.Errorf("network pool must be stopped before deletion")
	}
	_, err = p.output("virsh", "--connect", p.libvirtURI, "net-undefine", poolName)
	return err
}

func (p *VirshProvider) networkPool(name string) (NetworkPool, error) {
	info, err := p.output("virsh", "--connect", p.libvirtURI, "net-info", name)
	if err != nil {
		return NetworkPool{}, err
	}
	dump, _ := p.output("virsh", "--connect", p.libvirtURI, "net-dumpxml", name)
	doc := networkXMLDoc{}
	_ = xml.Unmarshal([]byte(dump), &doc)
	return NetworkPool{
		Name:      name,
		State:     strings.ToLower(parsePoolInfoText(info, "Active:")),
		Autostart: strings.EqualFold(parsePoolInfoText(info, "Autostart:"), "yes"),
		Bridge:    doc.Bridge.Name,
		Forward:   doc.Forward.Mode,
		Subnet:    doc.IP.Address,
		DHCP:      len(doc.IP.DHCP.Ranges) > 0,
		DHCPStart: func() string {
			if len(doc.IP.DHCP.Ranges) == 0 {
				return ""
			}
			return doc.IP.DHCP.Ranges[0].Start
		}(),
		DHCPEnd: func() string {
			if len(doc.IP.DHCP.Ranges) == 0 {
				return ""
			}
			return doc.IP.DHCP.Ranges[0].End
		}(),
		FixedAddresses: doc.fixedAddresses(),
		OpenVSwitch:    strings.EqualFold(doc.VirtualPort.Type, "openvswitch"),
	}, nil
}

func parsePoolInfoText(info string, prefix string) string {
	for _, line := range strings.Split(info, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(line, prefix))
		}
	}
	return ""
}

func (p *VirshProvider) parseStorageVolumes(poolName string, output string) []StorageVolume {
	items := make([]StorageVolume, 0)
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 || strings.EqualFold(fields[0], "Name") || strings.HasPrefix(fields[0], "-") {
			continue
		}
		volumeType := fields[2]
		format := ""
		capacity := int64(0)
		allocation := int64(0)
		if len(fields) >= 5 {
			capacity = parseLibvirtSize(fields[3] + " " + fields[4])
		}
		if len(fields) >= 7 {
			allocation = parseLibvirtSize(fields[5] + " " + fields[6])
		}
		name := fields[0]
		path := fields[1]
		format = p.detectStorageVolumeFormat(name, path)
		items = append(items, StorageVolume{
			Name:            name,
			Path:            path,
			Type:            volumeType,
			Format:          format,
			Capacity:        capacity,
			Allocation:      allocation,
			Pool:            poolName,
			CloneSupported:  canCloneVolume(format, name, path),
			DeleteSupported: true,
		})
	}
	return items
}

func (p *VirshProvider) detectStorageVolumeFormat(name string, path string) string {
	if strings.EqualFold(filepath.Ext(firstNonEmptyString(name, path)), ".iso") || strings.EqualFold(filepath.Ext(path), ".iso") {
		return "iso"
	}
	if strings.TrimSpace(path) != "" {
		if out, err := p.output("qemu-img", "info", "-U", "--output=json", path); err == nil {
			var info struct {
				Format string `json:"format"`
			}
			if err := json.Unmarshal([]byte(out), &info); err == nil && strings.TrimSpace(info.Format) != "" {
				return strings.ToLower(strings.TrimSpace(info.Format))
			}
		}
	}
	return storageVolumeFormat(name, path)
}

func storageVolumeFormat(name string, path string) string {
	value := strings.ToLower(firstNonEmptyString(filepath.Ext(name), filepath.Ext(path)))
	value = strings.TrimPrefix(value, ".")
	switch value {
	case "iso", "qcow2", "qcow", "qed", "raw":
		return value
	default:
		return "raw"
	}
}

func canCloneVolume(format string, name string, path string) bool {
	normalized := strings.ToLower(firstNonEmptyString(format, storageVolumeFormat(name, path)))
	return supportedVolumeFormat(normalized)
}

func (p *VirshProvider) volumeUsedByVM(volumePath string) (string, bool) {
	target := strings.TrimSpace(volumePath)
	if target == "" {
		return "", false
	}
	vms, err := p.ListVMsFast()
	if err != nil {
		return "", false
	}
	for _, vm := range vms {
		config, err := p.vmConfig(vm.Name, false)
		if err != nil {
			continue
		}
		for _, disk := range config.Disks {
			if disk.Path == target {
				return vm.Name, true
			}
		}
	}
	return "", false
}

func supportedVolumeFormat(format string) bool {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "qcow2", "qcow", "qed", "raw":
		return true
	default:
		return false
	}
}

func escapeXMLAttr(value string) string {
	var b strings.Builder
	_ = xml.EscapeText(&b, []byte(value))
	return b.String()
}

type storagePoolXML struct {
	Type   string `xml:"type,attr"`
	Source struct {
		Name string `xml:"name"`
		Host struct {
			Name string `xml:"name,attr"`
		} `xml:"host"`
		Dir struct {
			Path string `xml:"path,attr"`
		} `xml:"dir"`
		Format struct {
			Type string `xml:"type,attr"`
		} `xml:"format"`
	} `xml:"source"`
	Target struct {
		Path string `xml:"path"`
	} `xml:"target"`
}

func cleanPoolPath(path string) string {
	if strings.TrimSpace(path) == "" {
		return ""
	}
	return filepath.Clean(path)
}
