package kvm

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const coldMigrationCopyTimeout = time.Hour

type createdVMDisk struct {
	Pool string
	Name string
}

func (p *VirshProvider) CreateVM(request VMCreateRequest) (VMConfig, error) {
	if strings.TrimSpace(request.XML) != "" {
		return p.createVMFromXML(request)
	}
	if err := validateVMCreateRequest(&request); err != nil {
		return VMConfig{}, err
	}
	if err := p.validateRequestedHostResources(request.MaximumCPU, request.MaximumMemoryMB); err != nil {
		return VMConfig{}, err
	}
	if out, err := p.output("virsh", "--connect", p.libvirtURI, "dominfo", request.Name); err == nil && strings.TrimSpace(out) != "" {
		return VMConfig{}, fmt.Errorf("vm already exists")
	}
	volumes := make([]StorageVolume, 0, len(request.Disks))
	createdDisks := make([]createdVMDisk, 0, len(request.Disks))
	defer func() {
		for i := len(createdDisks) - 1; i >= 0; i-- {
			_ = p.DeleteStorageVolume(createdDisks[i].Pool, createdDisks[i].Name)
		}
	}()
	if request.CreateMode == "template" {
		volume, err := p.CloneStorageVolumeToPool(request.Template.SourcePool, request.Template.TargetPool, StorageVolumeCloneRequest{
			Name:             request.Template.TargetName,
			SourceName:       request.Template.SourceName,
			Format:           request.Template.Format,
			Convert:          request.Template.Convert,
			PreallocMetadata: request.Template.PreallocMetadata,
		})
		if err != nil {
			return VMConfig{}, err
		}
		volumes = append(volumes, volume)
		createdDisks = append(createdDisks, createdVMDisk{Pool: request.Template.TargetPool, Name: request.Template.TargetName})
	} else {
		for _, disk := range request.Disks {
			volume, err := p.CreateStorageVolume(disk.Pool, StorageVolumeCreateRequest{
				Name:             disk.Name,
				Format:           disk.Format,
				CapacityBytes:    disk.CapacityGB * 1024 * 1024 * 1024,
				PreallocMetadata: disk.PreallocMetadata,
			})
			if err != nil {
				return VMConfig{}, err
			}
			volumes = append(volumes, volume)
			createdDisks = append(createdDisks, createdVMDisk{Pool: disk.Pool, Name: disk.Name})
		}
	}

	args := []string{
		"--connect", p.libvirtURI,
		"--name", request.Name,
		"--memory", createMemoryArg(request.CurrentMemoryMB, request.MaximumMemoryMB),
		"--vcpus", createVCPUArg(request.CurrentCPU, request.MaximumCPU),
	}
	for index, disk := range request.Disks {
		args = append(args, "--disk", createDiskArg(volumes[index].Path, disk.Bus, disk.Format))
	}
	args = append(args, "--disk", createCDROMDiskArg(request.ISOPath, request.ISOBus))
	networkArg := createNetworkArg(request.NetworkSource, request.NetworkModel, p.networkPoolsByName())
	args = append(args,
		"--network", networkArg,
		"--graphics", createGraphicsArg(request.Graphics, request.ConsolePassword),
		"--channel", createGuestAgentChannelArg(),
		"--import",
		"--noautoconsole",
	)
	if request.CPUModel != "" {
		args = append(args, "--cpu", request.CPUModel)
	}
	if request.OSType != "" {
		args = append(args, "--os-type", request.OSType)
	}
	if request.ISOPath != "" {
		args = append(args, "--boot", "cdrom,hd")
	} else {
		args = append(args, "--boot", "hd")
	}
	if strings.EqualFold(request.BootFirmware, "uefi") {
		args = append(args, "--boot", "uefi")
	}
	xmlOut, err := p.storageOutput("virt-install", append(args, "--print-xml", "--dry-run")...)
	if err != nil {
		return VMConfig{}, err
	}
	xmlOut = extractDomainXML(xmlOut)
	if xmlOut == "" {
		return VMConfig{}, fmt.Errorf("virt-install generated empty domain xml")
	}
	xmlPath, err := writeTempXML(xmlOut)
	if err != nil {
		return VMConfig{}, err
	}
	defer os.Remove(xmlPath)
	if _, err := p.storageOutput("virsh", "--connect", p.libvirtURI, "define", xmlPath); err != nil {
		return VMConfig{}, err
	}
	createdDisks = nil
	if err := p.configureDefaultMemoryStats(request.Name); err != nil {
		return VMConfig{}, err
	}
	if request.Description != "" {
		if err := p.updateDescription(request.Name, request.Description); err != nil {
			return VMConfig{}, err
		}
	}
	if request.Autostart {
		if err := p.StartVM(request.Name); err != nil && !isDomainAlreadyRunningError(err) {
			return VMConfig{}, err
		}
	}
	return p.vmConfig(request.Name, true)
}

func (p *VirshProvider) createVMFromXML(request VMCreateRequest) (VMConfig, error) {
	request.XML = strings.TrimSpace(request.XML)
	if request.XML == "" {
		return VMConfig{}, fmt.Errorf("vm xml is required")
	}
	doc, err := parseDomainXML(request.XML)
	if err != nil {
		return VMConfig{}, fmt.Errorf("vm xml is invalid")
	}
	if strings.TrimSpace(doc.Name) == "" {
		return VMConfig{}, fmt.Errorf("vm xml name is required")
	}
	request.Name = strings.TrimSpace(doc.Name)
	if out, err := p.output("virsh", "--connect", p.libvirtURI, "dominfo", request.Name); err == nil && strings.TrimSpace(out) != "" {
		return VMConfig{}, fmt.Errorf("vm already exists")
	}
	xmlPath, err := writeTempXML(request.XML)
	if err != nil {
		return VMConfig{}, err
	}
	defer os.Remove(xmlPath)
	if _, err := p.storageOutput("virsh", "--connect", p.libvirtURI, "define", xmlPath); err != nil {
		return VMConfig{}, err
	}
	if err := p.configureDefaultMemoryStats(request.Name); err != nil {
		return VMConfig{}, err
	}
	if request.Autostart {
		if err := p.StartVM(request.Name); err != nil && !isDomainAlreadyRunningError(err) {
			return VMConfig{}, err
		}
	}
	return p.vmConfig(request.Name, true)
}

func isDomainAlreadyRunningError(err error) bool {
	if err == nil {
		return false
	}
	detail := strings.ToLower(err.Error())
	return strings.Contains(detail, "domain is already running") ||
		strings.Contains(detail, "already running")
}

func extractDomainXML(output string) string {
	output = strings.TrimSpace(output)
	start := strings.Index(output, "<domain")
	if start < 0 {
		return ""
	}
	end := strings.LastIndex(output, "</domain>")
	if end < 0 {
		return ""
	}
	end += len("</domain>")
	if end <= start {
		return ""
	}
	return strings.TrimSpace(output[start:end])
}

func (p *VirshProvider) MigrateVM(vmName string, request VMMigrateRequest) error {
	vmName = strings.TrimSpace(vmName)
	request.DestinationURI = strings.TrimSpace(request.DestinationURI)
	if vmName == "" || request.DestinationURI == "" {
		return fmt.Errorf("vm name and destination uri are required")
	}
	if _, err := url.ParseRequestURI(request.DestinationURI); err != nil {
		return fmt.Errorf("destination uri is invalid")
	}
	if request.Live && request.CopyDisks {
		return p.migrateLiveVMWithDiskCopy(vmName, request)
	}
	if !request.Live && request.CopyDisks {
		return p.migrateStoppedVMWithDiskCopy(vmName, request)
	}
	args := []string{"--connect", p.libvirtURI, "migrate"}
	if request.Live {
		args = append(args, "--live")
	}
	if request.Persistent {
		args = append(args, "--persistent")
	}
	if request.UndefineSource {
		args = append(args, "--undefinesource")
	}
	if request.AutoConverge {
		args = append(args, "--auto-converge")
	}
	if request.PostCopy {
		args = append(args, "--postcopy")
	}
	args = append(args, vmName, request.DestinationURI)
	if _, err := p.storageOutput("virsh", args...); err != nil {
		return err
	}
	return nil
}

func (p *VirshProvider) migrateLiveVMWithDiskCopy(vmName string, request VMMigrateRequest) error {
	if !isQemuSSHMigrationURI(request.DestinationURI) {
		return fmt.Errorf("live migration with disk copy requires qemu+ssh destination uri")
	}
	stateOut, _ := p.output("virsh", "--connect", p.libvirtURI, "domstate", vmName)
	if normalizeState(stateOut) != "running" {
		return fmt.Errorf("non-running vm requires cold migration")
	}
	config, err := p.vmConfig(vmName, false)
	if err != nil {
		return err
	}
	target, err := migrationSSHTarget(request.DestinationURI, "")
	if err != nil {
		return err
	}
	targetPools, err := p.remoteStoragePoolTargets(target)
	if err != nil {
		return err
	}
	if _, err := p.copyMigrationDisksToTarget(config.Disks, target, targetPools); err != nil {
		return err
	}
	args := []string{"--connect", p.libvirtURI, "migrate", "--live", "--unsafe"}
	if request.Persistent {
		args = append(args, "--persistent")
	}
	if request.AutoConverge {
		args = append(args, "--auto-converge")
	}
	if request.PostCopy {
		args = append(args, "--postcopy")
	}
	args = append(args, vmName, request.DestinationURI)
	if _, err := p.storageOutput("virsh", args...); err != nil {
		return err
	}
	if request.UndefineSource {
		return p.destroyThenUndefineVMAndDeleteConfigDisks(vmName, config)
	}
	return nil
}

func (p *VirshProvider) migrateStoppedVMWithDiskCopy(vmName string, request VMMigrateRequest) error {
	if !isQemuSSHMigrationURI(request.DestinationURI) {
		return fmt.Errorf("cold migration with disk copy requires qemu+ssh destination uri")
	}
	stateOut, _ := p.output("virsh", "--connect", p.libvirtURI, "domstate", vmName)
	if normalizeState(stateOut) == "running" {
		return fmt.Errorf("running vm requires live migration")
	}
	config, err := p.vmConfig(vmName, false)
	if err != nil {
		return err
	}
	target, err := migrationSSHTarget(request.DestinationURI, "")
	if err != nil {
		return err
	}
	targetPools, err := p.remoteStoragePoolTargets(target)
	if err != nil {
		return err
	}
	diskPaths, err := p.copyMigrationDisksToTarget(config.Disks, target, targetPools)
	if err != nil {
		return err
	}
	nextXML, err := buildMigrationDomainXML(config.XML, diskPaths)
	if err != nil {
		return err
	}
	remoteXML := "/tmp/kvm-manager-migrate-" + vmName + ".xml"
	if err := p.copyTextToMigrationTarget(target, remoteXML, nextXML); err != nil {
		return err
	}
	if _, err := p.outputWithTimeout(coldMigrationCopyTimeout, "ssh", append(migrationSSHArgs(target), "virsh", "--connect", p.libvirtURI, "define", remoteXML)...); err != nil {
		return err
	}
	_, _ = p.outputWithTimeout(15*time.Second, "ssh", append(migrationSSHArgs(target), "rm", "-f", remoteXML)...)
	if request.UndefineSource {
		if err := p.undefineVMAndDeleteConfigDisks(vmName, config); err != nil {
			return err
		}
	}
	return nil
}

func buildMigrationDomainXML(input string, diskPaths map[string]string) (string, error) {
	decoder := xml.NewDecoder(strings.NewReader(input))
	var output strings.Builder
	encoder := xml.NewEncoder(&output)
	var stack []string
	for {
		token, err := decoder.Token()
		if err != nil {
			if err == io.EOF {
				break
			}
			return "", err
		}
		if item, ok := token.(xml.StartElement); ok {
			parent := currentParent(stack)
			stack = append(stack, item.Name.Local)
			if parent == "disk" && item.Name.Local == "source" {
				sourcePath := firstNonEmptyString(attrValue(item.Attr, "file"), attrValue(item.Attr, "dev"))
				if path := diskPaths[sourcePath]; path != "" {
					setDiskSourcePath(&item.Attr, sourcePath, path)
				}
			}
			if err := encoder.EncodeToken(item); err != nil {
				return "", err
			}
			continue
		}
		if item, ok := token.(xml.EndElement); ok {
			if err := encoder.EncodeToken(item); err != nil {
				return "", err
			}
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
			continue
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

func (p *VirshProvider) remoteStoragePoolTargets(target migrationTarget) ([]storagePoolTarget, error) {
	out, err := p.outputWithTimeout(coldMigrationCopyTimeout, "ssh", append(migrationSSHArgs(target), "virsh", "--connect", p.libvirtURI, "pool-list", "--all", "--name")...)
	if err != nil {
		return nil, err
	}
	items := make([]storagePoolTarget, 0)
	for _, line := range strings.Split(out, "\n") {
		name := strings.TrimSpace(line)
		if name == "" {
			continue
		}
		dump, err := p.outputWithTimeout(coldMigrationCopyTimeout, "ssh", append(migrationSSHArgs(target), "virsh", "--connect", p.libvirtURI, "pool-dumpxml", name)...)
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
	if len(items) == 0 {
		return nil, fmt.Errorf("target storage pools are unavailable")
	}
	return items, nil
}

func (p *VirshProvider) copyMigrationDisksToTarget(disks []VMConfigDisk, target migrationTarget, targetPools []storagePoolTarget) (map[string]string, error) {
	diskPaths := make(map[string]string, len(disks))
	seenTargets := make(map[string]bool, len(disks))
	for _, disk := range disks {
		sourcePath := strings.TrimSpace(disk.Path)
		if sourcePath == "" {
			return nil, fmt.Errorf("migration disk path is unavailable")
		}
		targetPool := storagePoolTargetForPath(sourcePath, targetPools)
		if strings.TrimSpace(targetPool.Path) == "" {
			return nil, fmt.Errorf("target storage pool path for disk %s is unavailable", sourcePath)
		}
		targetPath := filepath.ToSlash(filepath.Clean(sourcePath))
		if seenTargets[targetPath] {
			return nil, fmt.Errorf("target migration disk path duplicated")
		}
		seenTargets[targetPath] = true
		if err := p.ensureRemotePathAvailable(target, targetPath); err != nil {
			return nil, err
		}
		if _, err := p.outputWithTimeout(coldMigrationCopyTimeout, "scp", append(migrationSCPArgs(target), sourcePath, migrationSSHHost(target)+":"+targetPath)...); err != nil {
			return nil, err
		}
		diskPaths[sourcePath] = targetPath
		if strings.TrimSpace(disk.SourcePath) != "" {
			diskPaths[disk.SourcePath] = targetPath
		}
	}
	return diskPaths, nil
}

func storagePoolTargetForPath(path string, pools []storagePoolTarget) storagePoolTarget {
	path = cleanPoolPath(path)
	if path == "" {
		return storagePoolTarget{}
	}
	best := storagePoolTarget{}
	bestLength := 0
	for _, pool := range pools {
		poolPath := cleanPoolPath(pool.Path)
		if poolPath == "" || !pathHasPoolPrefix(path, poolPath) {
			continue
		}
		if len(poolPath) > bestLength {
			best = pool
			bestLength = len(poolPath)
		}
	}
	return best
}

func (p *VirshProvider) ensureRemotePathAvailable(target migrationTarget, path string) error {
	dir := filepath.ToSlash(filepath.Dir(path))
	if strings.TrimSpace(dir) == "" || dir == "." {
		return fmt.Errorf("target disk directory is invalid")
	}
	if _, err := p.outputWithTimeout(coldMigrationCopyTimeout, "ssh", append(migrationSSHArgs(target), "mkdir", "-p", dir)...); err != nil {
		return err
	}
	if _, err := p.outputWithTimeout(coldMigrationCopyTimeout, "ssh", append(migrationSSHArgs(target), "test", "!", "-e", path)...); err != nil {
		return fmt.Errorf("target disk already exists")
	}
	return nil
}

func (p *VirshProvider) copyTextToMigrationTarget(target migrationTarget, path string, content string) error {
	tmp, err := os.CreateTemp("", "kvm-manager-migrate-*.xml")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := tmp.WriteString(content); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	_, err = p.outputWithTimeout(30*time.Second, "scp", append(migrationSCPArgs(target), tmpPath, migrationSSHHost(target)+":"+path)...)
	return err
}

func migrationSSHHost(target migrationTarget) string {
	host := target.Host
	if strings.Contains(host, ":") && !strings.HasPrefix(host, "[") {
		host = "[" + host + "]"
	}
	if strings.TrimSpace(target.Username) != "" {
		host = target.Username + "@" + host
	}
	return host
}

func migrationSSHArgs(target migrationTarget) []string {
	args := []string{}
	if strings.TrimSpace(target.Port) != "" && target.Port != "22" {
		args = append(args, "-p", target.Port)
	}
	return append(args, migrationSSHHost(target))
}

func migrationSCPArgs(target migrationTarget) []string {
	args := []string{}
	if strings.TrimSpace(target.Port) != "" && target.Port != "22" {
		args = append(args, "-P", target.Port)
	}
	return args
}

func validateVMCreateRequest(request *VMCreateRequest) error {
	request.Name = strings.TrimSpace(request.Name)
	request.Description = strings.TrimSpace(request.Description)
	request.CPUModel = strings.TrimSpace(request.CPUModel)
	request.DiskName = strings.TrimSpace(request.DiskName)
	request.DiskPool = strings.TrimSpace(request.DiskPool)
	request.DiskFormat = strings.ToLower(strings.TrimSpace(request.DiskFormat))
	request.DiskBus = strings.TrimSpace(request.DiskBus)
	request.ISOPath = strings.TrimSpace(request.ISOPath)
	request.ISOBus = strings.ToLower(strings.TrimSpace(request.ISOBus))
	request.NetworkSource = strings.TrimSpace(request.NetworkSource)
	request.NetworkModel = strings.TrimSpace(request.NetworkModel)
	request.Graphics = strings.TrimSpace(request.Graphics)
	request.ConsolePassword = strings.TrimSpace(request.ConsolePassword)
	request.BootFirmware = strings.TrimSpace(request.BootFirmware)
	request.CreateMode = strings.ToLower(strings.TrimSpace(request.CreateMode))
	request.OSType = strings.ToLower(strings.TrimSpace(request.OSType))
	request.Template = normalizeVMCreateTemplate(request.Template)
	normalizeVMCreateDisks(request)
	if request.Name == "" || request.NetworkSource == "" {
		return fmt.Errorf("vm name and network fields are required")
	}
	if request.CurrentCPU <= 0 || request.MaximumCPU <= 0 || request.CurrentCPU > request.MaximumCPU {
		return fmt.Errorf("cpu values are invalid")
	}
	if request.CurrentMemoryMB <= 0 || request.MaximumMemoryMB <= 0 || request.CurrentMemoryMB > request.MaximumMemoryMB {
		return fmt.Errorf("memory values are invalid")
	}
	if request.CreateMode == "" {
		request.CreateMode = "blank"
	}
	switch request.CreateMode {
	case "blank":
		if err := validateVMCreateDisks(request.Disks); err != nil {
			return err
		}
	case "template":
		if err := validateVMCreateTemplate(request.Template); err != nil {
			return err
		}
		request.Disks = []VMCreateDiskRequest{{
			Name:             request.Template.TargetName,
			Pool:             request.Template.TargetPool,
			Format:           request.Template.Format,
			Bus:              request.Template.Bus,
			CapacityGB:       1,
			PreallocMetadata: request.Template.PreallocMetadata,
		}}
	default:
		return fmt.Errorf("vm create mode is invalid")
	}
	if request.NetworkModel == "" {
		request.NetworkModel = "virtio"
	}
	if !supportedNetworkModel(request.NetworkModel) {
		return fmt.Errorf("network model is invalid")
	}
	if request.Graphics == "" {
		request.Graphics = "vnc"
	}
	if !strings.EqualFold(request.Graphics, "vnc") {
		return fmt.Errorf("graphics type is invalid")
	}
	if request.CPUModel == "" {
		request.CPUModel = "host-passthrough"
	}
	if request.OSType == "" {
		request.OSType = "linux"
	}
	if request.OSType != "linux" && request.OSType != "windows" {
		return fmt.Errorf("os type is invalid")
	}
	if request.ISOBus == "" {
		request.ISOBus = "sata"
	}
	if !supportedCDROMBus(request.ISOBus) {
		return fmt.Errorf("iso bus is invalid")
	}
	return nil
}

func normalizeVMCreateDisks(request *VMCreateRequest) {
	if request.CreateMode == "template" {
		return
	}
	if len(request.Disks) == 0 {
		request.Disks = []VMCreateDiskRequest{{
			Name:             request.DiskName,
			Pool:             request.DiskPool,
			Format:           request.DiskFormat,
			Bus:              request.DiskBus,
			CapacityGB:       request.DiskCapacityGB,
			PreallocMetadata: request.PreallocMetadata,
		}}
	}
	for index := range request.Disks {
		disk := &request.Disks[index]
		disk.Name = strings.TrimSpace(disk.Name)
		disk.Pool = strings.TrimSpace(disk.Pool)
		disk.Format = strings.ToLower(strings.TrimSpace(disk.Format))
		disk.Bus = strings.TrimSpace(disk.Bus)
	}
	if len(request.Disks) > 0 {
		systemDisk := request.Disks[0]
		request.DiskName = systemDisk.Name
		request.DiskPool = systemDisk.Pool
		request.DiskFormat = systemDisk.Format
		request.DiskBus = systemDisk.Bus
		request.DiskCapacityGB = systemDisk.CapacityGB
		request.PreallocMetadata = systemDisk.PreallocMetadata
	}
}

func normalizeVMCreateTemplate(template VMCreateTemplate) VMCreateTemplate {
	template.SourcePool = strings.TrimSpace(template.SourcePool)
	template.SourceName = strings.TrimSpace(template.SourceName)
	template.TargetPool = strings.TrimSpace(template.TargetPool)
	template.TargetName = strings.TrimSpace(template.TargetName)
	template.Bus = strings.TrimSpace(template.Bus)
	template.Format = strings.ToLower(strings.TrimSpace(template.Format))
	return template
}

func validateVMCreateTemplate(template VMCreateTemplate) error {
	if template.SourcePool == "" || template.SourceName == "" || template.TargetPool == "" || template.TargetName == "" || template.Bus == "" {
		return fmt.Errorf("template fields are required")
	}
	if strings.ContainsAny(template.SourceName, `/\`) || strings.ContainsAny(template.TargetName, `/\`) {
		return fmt.Errorf("template volume name must not contain path separators")
	}
	if !supportedVolumeFormat(template.Format) {
		return fmt.Errorf("unsupported storage volume format")
	}
	if !storageVolumeNameMatchesFormat(template.TargetName, template.Format) {
		return fmt.Errorf("template target name extension must match format")
	}
	if !supportedDiskBus(template.Bus) {
		return fmt.Errorf("disk bus is invalid")
	}
	return nil
}

func validateVMCreateDisks(disks []VMCreateDiskRequest) error {
	if len(disks) == 0 {
		return fmt.Errorf("disk configuration is required")
	}
	seenNames := map[string]struct{}{}
	systemDisk := disks[0]
	for index, disk := range disks {
		if disk.Name == "" || disk.Pool == "" || disk.Format == "" || disk.Bus == "" {
			return fmt.Errorf("disk fields are required")
		}
		if disk.CapacityGB <= 0 {
			return fmt.Errorf("disk capacity is required")
		}
		if strings.ContainsAny(disk.Name, `/\`) {
			return fmt.Errorf("disk name must not contain path separators")
		}
		if !supportedVolumeFormat(disk.Format) {
			return fmt.Errorf("unsupported storage volume format")
		}
		if !storageVolumeNameMatchesFormat(disk.Name, disk.Format) {
			return fmt.Errorf("disk name extension must match format")
		}
		if index > 0 && !vmCreateDiskSettingsMatchSystem(systemDisk, disk) {
			return fmt.Errorf("data disk settings must match system disk")
		}
		key := strings.ToLower(disk.Pool + "/" + disk.Name)
		if _, ok := seenNames[key]; ok {
			return fmt.Errorf("disk name must not be duplicated")
		}
		seenNames[key] = struct{}{}
	}
	return nil
}

func vmCreateDiskSettingsMatchSystem(systemDisk VMCreateDiskRequest, disk VMCreateDiskRequest) bool {
	return strings.EqualFold(systemDisk.Pool, disk.Pool) &&
		strings.EqualFold(systemDisk.Format, disk.Format) &&
		strings.EqualFold(systemDisk.Bus, disk.Bus) &&
		systemDisk.PreallocMetadata == disk.PreallocMetadata
}

func createVCPUArg(current int, maximum int) string {
	if maximum > current {
		return strconv.Itoa(current) + ",maxvcpus=" + strconv.Itoa(maximum)
	}
	return strconv.Itoa(current)
}

func createMemoryArg(current int64, maximum int64) string {
	if maximum > current {
		return strconv.FormatInt(current, 10) + ",maxmemory=" + strconv.FormatInt(maximum, 10)
	}
	return strconv.FormatInt(current, 10)
}

func (p *VirshProvider) validateRequestedHostResources(maximumCPU int, maximumMemoryMB int64) error {
	nodeOut, err := p.output("virsh", "--connect", p.libvirtURI, "nodeinfo")
	if err != nil {
		return err
	}
	hostCPU := parseNodeCPU(nodeOut)
	hostMemoryBytes := parseNodeMemory(nodeOut)
	return validateRequestedHostResourceLimits(hostCPU, hostMemoryBytes, maximumCPU, maximumMemoryMB)
}

func validateRequestedHostResourceLimits(hostCPU int, hostMemoryBytes int64, maximumCPU int, maximumMemoryMB int64) error {
	if hostCPU > 0 && maximumCPU > hostCPU {
		return fmt.Errorf("maximum cpu exceeds host cpu")
	}
	if hostMemoryBytes > 0 && maximumMemoryMB*1024*1024 > hostMemoryBytes {
		return fmt.Errorf("maximum memory exceeds host memory")
	}
	return nil
}

func createDiskArg(path string, bus string, format string) string {
	return "path=" + path + ",bus=" + bus + ",format=" + format
}

func createCDROMDiskArg(path string, bus string) string {
	parts := []string{}
	if strings.TrimSpace(path) != "" {
		parts = append(parts, "path="+strings.TrimSpace(path))
	}
	parts = append(parts, "device=cdrom", "readonly=on", "bus="+strings.TrimSpace(bus))
	return strings.Join(parts, ",")
}

func createNetworkArg(source string, model string, pools map[string]NetworkPool) string {
	source = strings.TrimSpace(source)
	model = strings.TrimSpace(model)
	if pool, ok := pools[source]; ok && strings.EqualFold(strings.TrimSpace(pool.Forward), "bridge") && strings.TrimSpace(pool.Bridge) != "" {
		return "bridge=" + strings.TrimSpace(pool.Bridge) + ",model=" + model
	}
	return "network=" + source + ",model=" + model
}

func createGraphicsArg(graphics string, password string) string {
	arg := "vnc,listen=0.0.0.0"
	if strings.TrimSpace(password) != "" {
		arg += ",password=" + strings.TrimSpace(password)
	}
	return arg
}

func createGuestAgentChannelArg() string {
	return "unix,target_type=virtio,name=org.qemu.guest_agent.0"
}

func supportedNetworkModel(model string) bool {
	switch strings.ToLower(strings.TrimSpace(model)) {
	case "virtio", "e1000", "e1000e", "rtl8139", "vmxnet3":
		return true
	default:
		return false
	}
}

func supportedCDROMBus(bus string) bool {
	switch strings.ToLower(strings.TrimSpace(bus)) {
	case "sata", "ide", "scsi", "usb":
		return true
	default:
		return false
	}
}

func supportedDiskBus(bus string) bool {
	switch strings.ToLower(strings.TrimSpace(bus)) {
	case "virtio", "sata", "scsi", "ide":
		return true
	default:
		return false
	}
}
