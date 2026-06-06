package agent

import (
	"context"
	"encoding/json"
	"net/url"
)

type VirtualMachine struct {
	Name                 string   `json:"name"`
	UUID                 string   `json:"uuid"`
	Description          string   `json:"description"`
	OSType               string   `json:"osType"`
	Status               string   `json:"status"`
	CPUCores             int      `json:"cpuCores"`
	MemoryBytes          int64    `json:"memoryBytes"`
	DiskBytes            int64    `json:"diskBytes"`
	DiskUsedBytes        int64    `json:"diskUsedBytes"`
	Disks                []VMDisk `json:"disks"`
	PrimaryIP            string   `json:"primaryIp"`
	CPUUsage             int      `json:"cpuUsage"`
	CPUUsageAvailable    bool     `json:"cpuUsageAvailable"`
	MemoryUsage          int      `json:"memoryUsage"`
	MemoryUsageAvailable bool     `json:"memoryUsageAvailable"`
	DiskUsage            int      `json:"diskUsage"`
	DiskUsageAvailable   bool     `json:"diskUsageAvailable"`
	DiskReadBytesPerSec  int64    `json:"diskReadBytesPerSecond"`
	DiskWriteBytesPerSec int64    `json:"diskWriteBytesPerSecond"`
	NetworkRxBytesPerSec int64    `json:"networkRxBytesPerSecond"`
	NetworkTxBytesPerSec int64    `json:"networkTxBytesPerSecond"`
	UptimeSeconds        int64    `json:"uptimeSeconds"`
}

type VMDisk struct {
	Name      string `json:"name"`
	Path      string `json:"path"`
	Bytes     int64  `json:"bytes"`
	UsedBytes int64  `json:"usedBytes"`
}

type VMConfig struct {
	Name               string              `json:"name"`
	UUID               string              `json:"uuid"`
	OSType             string              `json:"osType"`
	Status             string              `json:"status"`
	Description        string              `json:"description"`
	Autostart          bool                `json:"autostart"`
	CurrentCPU         int                 `json:"currentCpu"`
	MaximumCPU         int                 `json:"maximumCpu"`
	HostCPU            int                 `json:"hostCpu"`
	Arch               string              `json:"arch"`
	CurrentMemoryBytes int64               `json:"currentMemoryBytes"`
	MaximumMemoryBytes int64               `json:"maximumMemoryBytes"`
	HostMemoryBytes    int64               `json:"hostMemoryBytes"`
	MemoryStatsPeriod  int                 `json:"memoryStatsPeriod"`
	Disks              []VMConfigDisk      `json:"disks"`
	Interfaces         []VMConfigInterface `json:"interfaces"`
	CDROMs             []VMConfigCDROM     `json:"cdroms"`
	Graphics           VMConfigGraphics    `json:"graphics"`
	XML                string              `json:"xml"`
}

type VMConfigUpdateRequest struct {
	Description       string `json:"description"`
	CurrentCPU        int    `json:"currentCpu"`
	MaximumCPU        int    `json:"maximumCpu"`
	CurrentMemoryMB   int64  `json:"currentMemoryMB"`
	MaximumMemoryMB   int64  `json:"maximumMemoryMB"`
	MemoryStatsPeriod int    `json:"memoryStatsPeriod"`
}

type VMRenameRequest struct {
	Name string `json:"name"`
}

type VMAutostartUpdateRequest struct {
	Autostart bool `json:"autostart"`
}

type VMConsoleUpdateRequest struct {
	PasswordEnabled bool   `json:"passwordEnabled"`
	Password        string `json:"password"`
}

type VMMediaConnectRequest struct {
	Target  string `json:"target"`
	ISOPath string `json:"isoPath"`
}

type VMMediaDisconnectRequest struct {
	Target string `json:"target"`
}

type VMXMLUpdateRequest struct {
	XML string `json:"xml"`
}

type VMDeviceUpdateRequest struct {
	Interfaces        []VMDeviceInterfaceRequest  `json:"interfaces"`
	NewInterfaces     []VMDeviceNewInterface      `json:"newInterfaces"`
	DeletedInterfaces []VMDeviceDeleteInterface   `json:"deletedInterfaces"`
	DiskResizes       []VMDeviceDiskResizeRequest `json:"diskResizes"`
	NewDisks          []VMDeviceNewDiskRequest    `json:"newDisks"`
	DeletedDisks      []VMDeviceDeleteDiskRequest `json:"deletedDisks"`
}

type VMDeviceInterfaceRequest struct {
	Name   string `json:"name"`
	MAC    string `json:"mac"`
	Source string `json:"source"`
}

type VMDeviceNewInterface struct {
	Source string `json:"source"`
	Model  string `json:"model"`
}

type VMDeviceDeleteInterface struct {
	Name string `json:"name"`
	MAC  string `json:"mac"`
}

type VMDeviceDiskResizeRequest struct {
	Name          string `json:"name"`
	CapacityBytes int64  `json:"capacityBytes"`
}

type VMDeviceDeleteDiskRequest struct {
	Name string `json:"name"`
}

type VMDeviceNewDiskRequest struct {
	Name             string `json:"name"`
	Pool             string `json:"pool"`
	Target           string `json:"target"`
	Bus              string `json:"bus"`
	Format           string `json:"format"`
	CapacityBytes    int64  `json:"capacityBytes"`
	PreallocMetadata bool   `json:"preallocMetadata"`
}

type VMCloneRequest struct {
	Name            string                    `json:"name"`
	Description     string                    `json:"description"`
	Autostart       bool                      `json:"autostart"`
	CurrentCPU      int                       `json:"currentCpu"`
	MaximumCPU      int                       `json:"maximumCpu"`
	CurrentMemoryMB int64                     `json:"currentMemoryMB"`
	MaximumMemoryMB int64                     `json:"maximumMemoryMB"`
	CDROMPolicy     string                    `json:"cdromPolicy"`
	Interfaces      []VMCloneInterfaceRequest `json:"interfaces"`
	Disks           []VMCloneDiskRequest      `json:"disks"`
}

type VMCloneInterfaceRequest struct {
	Name   string `json:"name"`
	MAC    string `json:"mac"`
	Source string `json:"source"`
}

type VMCloneDiskRequest struct {
	Name             string `json:"name"`
	Pool             string `json:"pool"`
	SourcePath       string `json:"sourcePath"`
	TargetName       string `json:"targetName"`
	PreallocMetadata bool   `json:"preallocMetadata"`
}

type VMCreateRequest struct {
	CreateMode       string                `json:"createMode"`
	Name             string                `json:"name"`
	Description      string                `json:"description"`
	Autostart        bool                  `json:"autostart"`
	CurrentCPU       int                   `json:"currentCpu"`
	MaximumCPU       int                   `json:"maximumCpu"`
	CurrentMemoryMB  int64                 `json:"currentMemoryMB"`
	MaximumMemoryMB  int64                 `json:"maximumMemoryMB"`
	CPUModel         string                `json:"cpuModel"`
	OSType           string                `json:"osType"`
	Disks            []VMCreateDiskRequest `json:"disks"`
	DiskName         string                `json:"diskName"`
	DiskPool         string                `json:"diskPool"`
	DiskFormat       string                `json:"diskFormat"`
	DiskBus          string                `json:"diskBus"`
	DiskCapacityGB   int64                 `json:"diskCapacityGB"`
	PreallocMetadata bool                  `json:"preallocMetadata"`
	ISOPath          string                `json:"isoPath"`
	ISOBus           string                `json:"isoBus"`
	NetworkSource    string                `json:"networkSource"`
	NetworkModel     string                `json:"networkModel"`
	Graphics         string                `json:"graphics"`
	ConsolePassword  string                `json:"consolePassword"`
	BootFirmware     string                `json:"bootFirmware"`
	Template         VMCreateTemplate      `json:"template"`
	XML              string                `json:"xml"`
}

type VMCreateTemplate struct {
	SourcePool       string `json:"sourcePool"`
	SourceName       string `json:"sourceName"`
	TargetPool       string `json:"targetPool"`
	TargetName       string `json:"targetName"`
	Bus              string `json:"bus"`
	Format           string `json:"format"`
	Convert          bool   `json:"convert"`
	PreallocMetadata bool   `json:"preallocMetadata"`
}

type VMCreateDiskRequest struct {
	Name             string `json:"name"`
	Pool             string `json:"pool"`
	Format           string `json:"format"`
	Bus              string `json:"bus"`
	CapacityGB       int64  `json:"capacityGB"`
	PreallocMetadata bool   `json:"preallocMetadata"`
}

type VMMigrateRequest struct {
	DestinationURI string `json:"destinationUri"`
	Live           bool   `json:"live"`
	CopyDisks      bool   `json:"copyDisks"`
	Persistent     bool   `json:"persistent"`
	UndefineSource bool   `json:"undefineSource"`
	AutoConverge   bool   `json:"autoConverge"`
	PostCopy       bool   `json:"postCopy"`
}

type MigrationConnectionCheckRequest struct {
	DestinationURI string `json:"destinationUri"`
	Live           bool   `json:"live"`
}

type MigrationConnectionCheckResult struct {
	OK               bool   `json:"ok"`
	PasswordRequired bool   `json:"passwordRequired"`
	Message          string `json:"message"`
}

type MigrationSSHKeySetupRequest struct {
	DestinationURI string `json:"destinationUri"`
	Username       string `json:"username"`
	Password       string `json:"password"`
}

type MigrationHostnameSetupRequest struct {
	DestinationURI string `json:"destinationUri"`
	Hostname       string `json:"hostname"`
}

type VMConfigDisk struct {
	Name       string `json:"name"`
	Path       string `json:"path"`
	SourcePath string `json:"sourcePath"`
	Pool       string `json:"pool"`
	Bus        string `json:"bus"`
	Device     string `json:"device"`
	Type       string `json:"type"`
	Bytes      int64  `json:"bytes"`
}

type VMConfigInterface struct {
	Name   string `json:"name"`
	MAC    string `json:"mac"`
	Type   string `json:"type"`
	Source string `json:"source"`
	Model  string `json:"model"`
}

type VMConfigCDROM struct {
	Name      string `json:"name"`
	Path      string `json:"path"`
	Bus       string `json:"bus"`
	Connected bool   `json:"connected"`
}

type VMConfigGraphics struct {
	Type            string `json:"type"`
	Listen          string `json:"listen"`
	Port            string `json:"port"`
	PasswordEnabled bool   `json:"passwordEnabled"`
}

type ConsoleInfo struct {
	Type            string `json:"type"`
	Listen          string `json:"listen"`
	Port            int    `json:"port"`
	PasswordEnabled bool   `json:"passwordEnabled"`
}

type Snapshot struct {
	Name      string `json:"name"`
	Status    string `json:"status"`
	CreatedAt string `json:"createdAt"`
}

type SnapshotCreateRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type SnapshotListResponse struct {
	Items []Snapshot `json:"items"`
	Total int        `json:"total"`
}

type VMListResponse struct {
	Items []VirtualMachine `json:"items"`
	Total int              `json:"total"`
}

func (c *Client) ListVMs(ctx context.Context, cfg Config) ([]VirtualMachine, error) {
	return c.listVMs(ctx, cfg, "")
}

func (c *Client) ListVMsFast(ctx context.Context, cfg Config) ([]VirtualMachine, error) {
	return c.listVMs(ctx, cfg, "fast")
}

func (c *Client) VM(ctx context.Context, cfg Config, vmName string) (VirtualMachine, error) {
	var vm VirtualMachine
	if err := c.get(ctx, cfg, "/v1/vms/"+urlPathEscape(vmName)+"/refresh", &vm); err != nil {
		return VirtualMachine{}, err
	}
	return vm, nil
}

func (c *Client) listVMs(ctx context.Context, cfg Config, level string) ([]VirtualMachine, error) {
	var response VMListResponse
	path := "/v1/vms"
	if level != "" {
		path += "?level=" + url.QueryEscape(level)
	}
	if err := c.get(ctx, cfg, path, &response); err != nil {
		return nil, err
	}
	return response.Items, nil
}

func (c *Client) ListSnapshots(ctx context.Context, cfg Config, vmName string) ([]Snapshot, error) {
	var response SnapshotListResponse
	if err := c.get(ctx, cfg, "/v1/vms/"+urlPathEscape(vmName)+"/snapshots", &response); err != nil {
		return nil, err
	}
	return response.Items, nil
}

func (c *Client) VMConfig(ctx context.Context, cfg Config, vmName string) (VMConfig, error) {
	var config VMConfig
	if err := c.get(ctx, cfg, "/v1/vms/"+urlPathEscape(vmName)+"/config", &config); err != nil {
		return VMConfig{}, err
	}
	return config, nil
}

func (c *Client) ConsoleInfo(ctx context.Context, cfg Config, vmName string) (ConsoleInfo, error) {
	var info ConsoleInfo
	if err := c.get(ctx, cfg, "/v1/vms/"+urlPathEscape(vmName)+"/console", &info); err != nil {
		return ConsoleInfo{}, err
	}
	return info, nil
}

func (c *Client) UpdateVMConfig(ctx context.Context, cfg Config, vmName string, request VMConfigUpdateRequest) (VMConfig, error) {
	body, err := json.Marshal(request)
	if err != nil {
		return VMConfig{}, err
	}
	var config VMConfig
	if err := c.put(ctx, cfg, "/v1/vms/"+urlPathEscape(vmName)+"/config", body, &config); err != nil {
		return VMConfig{}, err
	}
	return config, nil
}

func (c *Client) RenameVM(ctx context.Context, cfg Config, vmName string, request VMRenameRequest) (VMConfig, error) {
	body, err := json.Marshal(request)
	if err != nil {
		return VMConfig{}, err
	}
	var config VMConfig
	if err := c.put(ctx, cfg, "/v1/vms/"+urlPathEscape(vmName)+"/rename", body, &config); err != nil {
		return VMConfig{}, err
	}
	return config, nil
}

func (c *Client) UpdateVMAutostart(ctx context.Context, cfg Config, vmName string, request VMAutostartUpdateRequest) error {
	body, err := json.Marshal(request)
	if err != nil {
		return err
	}
	return c.put(ctx, cfg, "/v1/vms/"+urlPathEscape(vmName)+"/autostart", body, nil)
}

func (c *Client) UpdateVMConsole(ctx context.Context, cfg Config, vmName string, request VMConsoleUpdateRequest) (VMConfig, error) {
	body, err := json.Marshal(request)
	if err != nil {
		return VMConfig{}, err
	}
	var config VMConfig
	if err := c.put(ctx, cfg, "/v1/vms/"+urlPathEscape(vmName)+"/console", body, &config); err != nil {
		return VMConfig{}, err
	}
	return config, nil
}

func (c *Client) ConnectVMMedia(ctx context.Context, cfg Config, vmName string, request VMMediaConnectRequest) (VMConfig, error) {
	body, err := json.Marshal(request)
	if err != nil {
		return VMConfig{}, err
	}
	var config VMConfig
	if err := c.put(ctx, cfg, "/v1/vms/"+urlPathEscape(vmName)+"/media", body, &config); err != nil {
		return VMConfig{}, err
	}
	return config, nil
}

func (c *Client) DisconnectVMMedia(ctx context.Context, cfg Config, vmName string, request VMMediaDisconnectRequest) (VMConfig, error) {
	body, err := json.Marshal(request)
	if err != nil {
		return VMConfig{}, err
	}
	var config VMConfig
	if err := c.deleteWithTarget(ctx, cfg, "/v1/vms/"+urlPathEscape(vmName)+"/media", body, &config); err != nil {
		return VMConfig{}, err
	}
	return config, nil
}

func (c *Client) UpdateVMXML(ctx context.Context, cfg Config, vmName string, request VMXMLUpdateRequest) (VMConfig, error) {
	body, err := json.Marshal(request)
	if err != nil {
		return VMConfig{}, err
	}
	var config VMConfig
	if err := c.put(ctx, cfg, "/v1/vms/"+urlPathEscape(vmName)+"/xml", body, &config); err != nil {
		return VMConfig{}, err
	}
	return config, nil
}

func (c *Client) UpdateVMDevices(ctx context.Context, cfg Config, vmName string, request VMDeviceUpdateRequest) (VMConfig, error) {
	body, err := json.Marshal(request)
	if err != nil {
		return VMConfig{}, err
	}
	var config VMConfig
	if err := c.put(ctx, cfg, "/v1/vms/"+urlPathEscape(vmName)+"/devices", body, &config); err != nil {
		return VMConfig{}, err
	}
	return config, nil
}

func (c *Client) CloneVM(ctx context.Context, cfg Config, vmName string, request VMCloneRequest) (VMConfig, error) {
	body, err := json.Marshal(request)
	if err != nil {
		return VMConfig{}, err
	}
	var config VMConfig
	if err := c.withTimeout(storageOperationTimeout).postWithTarget(ctx, cfg, "/v1/vms/"+urlPathEscape(vmName)+"/clone", body, &config); err != nil {
		return VMConfig{}, err
	}
	return config, nil
}

func (c *Client) CreateVM(ctx context.Context, cfg Config, request VMCreateRequest) (VMConfig, error) {
	body, err := json.Marshal(request)
	if err != nil {
		return VMConfig{}, err
	}
	var config VMConfig
	if err := c.withTimeout(storageOperationTimeout).postWithTarget(ctx, cfg, "/v1/vms", body, &config); err != nil {
		return VMConfig{}, err
	}
	return config, nil
}

func (c *Client) MigrateVM(ctx context.Context, cfg Config, vmName string, request VMMigrateRequest) error {
	body, err := json.Marshal(request)
	if err != nil {
		return err
	}
	return c.withTimeout(storageOperationTimeout).post(ctx, cfg, "/v1/vms/"+urlPathEscape(vmName)+"/migrate", body)
}

func (c *Client) CheckMigrationConnection(ctx context.Context, cfg Config, request MigrationConnectionCheckRequest) (MigrationConnectionCheckResult, error) {
	body, err := json.Marshal(request)
	if err != nil {
		return MigrationConnectionCheckResult{}, err
	}
	var result MigrationConnectionCheckResult
	if err := c.postWithTarget(ctx, cfg, "/v1/migration/check", body, &result); err != nil {
		return MigrationConnectionCheckResult{}, err
	}
	return result, nil
}

func (c *Client) SetupMigrationSSHKey(ctx context.Context, cfg Config, request MigrationSSHKeySetupRequest) (MigrationConnectionCheckResult, error) {
	body, err := json.Marshal(request)
	if err != nil {
		return MigrationConnectionCheckResult{}, err
	}
	var result MigrationConnectionCheckResult
	if err := c.postWithTarget(ctx, cfg, "/v1/migration/ssh-key", body, &result); err != nil {
		return MigrationConnectionCheckResult{}, err
	}
	return result, nil
}

func (c *Client) SetupMigrationHostname(ctx context.Context, cfg Config, request MigrationHostnameSetupRequest) (MigrationConnectionCheckResult, error) {
	body, err := json.Marshal(request)
	if err != nil {
		return MigrationConnectionCheckResult{}, err
	}
	var result MigrationConnectionCheckResult
	if err := c.postWithTarget(ctx, cfg, "/v1/migration/hostname", body, &result); err != nil {
		return MigrationConnectionCheckResult{}, err
	}
	return result, nil
}

func (c *Client) RunSnapshotAction(ctx context.Context, cfg Config, vmName string, snapshotName string, action string) error {
	path := "/v1/vms/" + urlPathEscape(vmName) + "/snapshots/" + urlPathEscape(snapshotName) + "/" + urlPathEscape(action)
	return c.post(ctx, cfg, path, nil)
}

func (c *Client) CreateSnapshot(ctx context.Context, cfg Config, vmName string, request SnapshotCreateRequest) error {
	body, err := json.Marshal(request)
	if err != nil {
		return err
	}
	return c.post(ctx, cfg, "/v1/vms/"+urlPathEscape(vmName)+"/snapshots", body)
}
