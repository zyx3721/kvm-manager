package kvm

import "io"

type HostInfo struct {
	Hostname             string   `json:"hostname"`
	HostAddress          string   `json:"hostAddress"`
	Status               string   `json:"status"`
	KVMVersion           string   `json:"kvmVersion"`
	KVMFullVersion       string   `json:"kvmFullVersion"`
	CPUModel             string   `json:"cpuModel"`
	CPUCores             int      `json:"cpuCores"`
	CPUUsage             int      `json:"cpuUsage"`
	MemoryBytes          int64    `json:"memoryBytes"`
	MemoryUsage          int      `json:"memoryUsage"`
	StorageBytes         int64    `json:"storageBytes"`
	StorageUsage         int      `json:"storageUsage"`
	DiskReadBytesPerSec  int64    `json:"diskReadBytesPerSecond"`
	DiskWriteBytesPerSec int64    `json:"diskWriteBytesPerSecond"`
	NetworkRxBytesPerSec int64    `json:"networkRxBytesPerSecond"`
	NetworkTxBytesPerSec int64    `json:"networkTxBytesPerSecond"`
	Capabilities         []string `json:"capabilities"`
}

type HostInterface struct {
	Name         string `json:"name"`
	Type         string `json:"type"`
	MAC          string `json:"mac"`
	IPv4         string `json:"ipv4"`
	IPv4Mode     string `json:"ipv4Mode"`
	IPv6         string `json:"ipv6"`
	IPv6Mode     string `json:"ipv6Mode"`
	BridgeDevice string `json:"bridgeDevice"`
	BootMode     string `json:"bootMode"`
	Status       string `json:"status"`
	STP          string `json:"stp"`
	Delay        string `json:"delay"`
}

type HostInterfaceCreateRequest struct {
	Name              string   `json:"name"`
	StartMode         string   `json:"startMode"`
	Device            string   `json:"device"`
	Type              string   `json:"type"`
	STP               string   `json:"stp"`
	Delay             string   `json:"delay"`
	IPv4Mode          string   `json:"ipv4Mode"`
	IPv4Address       string   `json:"ipv4Address"`
	IPv4Gateway       string   `json:"ipv4Gateway"`
	IPv6Mode          string   `json:"ipv6Mode"`
	IPv6Address       string   `json:"ipv6Address"`
	IPv6Gateway       string   `json:"ipv6Gateway"`
	ApplySystemConfig bool     `json:"applySystemConfig"`
	DNSServers        []string `json:"dnsServers"`
}

type HostInterfaceDevice struct {
	Name string `json:"name"`
}

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

type Snapshot struct {
	Name      string `json:"name"`
	Status    string `json:"status"`
	CreatedAt string `json:"createdAt"`
}

type SnapshotCreateRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type StoragePool struct {
	Name           string `json:"name"`
	Type           string `json:"type"`
	State          string `json:"state"`
	Autostart      bool   `json:"autostart"`
	Path           string `json:"path"`
	CapacitySource string `json:"capacitySource"`
	Capacity       int64  `json:"capacity"`
	Allocation     int64  `json:"allocation"`
	Available      int64  `json:"available"`
	VolumeCount    int    `json:"volumeCount"`
}

type StoragePoolCreateRequest struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	Path       string `json:"path"`
	Device     string `json:"device"`
	SourceHost string `json:"sourceHost"`
	SourcePath string `json:"sourcePath"`
	Format     string `json:"format"`
}

type ISOFile struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	Bytes int64  `json:"bytes"`
	Pool  string `json:"pool"`
}

type StorageVolume struct {
	Name            string `json:"name"`
	Path            string `json:"path"`
	Type            string `json:"type"`
	Format          string `json:"format"`
	Capacity        int64  `json:"capacity"`
	Allocation      int64  `json:"allocation"`
	Pool            string `json:"pool"`
	CloneSupported  bool   `json:"cloneSupported"`
	DeleteSupported bool   `json:"deleteSupported"`
}

type StorageVolumeCreateRequest struct {
	Name             string `json:"name"`
	Format           string `json:"format"`
	CapacityBytes    int64  `json:"capacityBytes"`
	PreallocMetadata bool   `json:"preallocMetadata"`
}

type StorageVolumeCloneRequest struct {
	Name             string `json:"name"`
	SourceName       string `json:"sourceName"`
	Format           string `json:"format"`
	Convert          bool   `json:"convert"`
	PreallocMetadata bool   `json:"preallocMetadata"`
}

type PoolStateUpdateRequest struct {
	Active bool `json:"active"`
}

type PoolAutostartUpdateRequest struct {
	Autostart bool `json:"autostart"`
}

type NetworkPool struct {
	Name           string                `json:"name"`
	State          string                `json:"state"`
	Autostart      bool                  `json:"autostart"`
	Bridge         string                `json:"bridge"`
	Forward        string                `json:"forward"`
	Subnet         string                `json:"subnet"`
	DHCP           bool                  `json:"dhcp"`
	DHCPStart      string                `json:"dhcpStart"`
	DHCPEnd        string                `json:"dhcpEnd"`
	FixedAddresses []NetworkFixedAddress `json:"fixedAddresses"`
	OpenVSwitch    bool                  `json:"openVSwitch"`
}

type NetworkFixedAddress struct {
	Address string `json:"address"`
	MAC     string `json:"mac"`
}

type NetworkPoolCreateRequest struct {
	Name         string `json:"name"`
	Subnet       string `json:"subnet"`
	DHCP         bool   `json:"dhcp"`
	FixedAddress bool   `json:"fixedAddress"`
	Type         string `json:"type"`
	Bridge       string `json:"bridge"`
	OpenVSwitch  bool   `json:"openVSwitch"`
}

type ConsoleInfo struct {
	Type            string `json:"type"`
	Listen          string `json:"listen"`
	Port            int    `json:"port"`
	PasswordEnabled bool   `json:"passwordEnabled"`
}

type Provider interface {
	HostInfo() (HostInfo, error)
	ListHostInterfaces() ([]HostInterface, error)
	ListHostInterfaceDevices() ([]HostInterfaceDevice, error)
	CreateHostInterface(request HostInterfaceCreateRequest) (HostInterface, error)
	UpdateHostInterfaceState(name string, active bool) error
	DeleteHostInterface(name string) error
	ListVMs() ([]VirtualMachine, error)
	ListVMsFast() ([]VirtualMachine, error)
	VM(vmName string) (VirtualMachine, error)
	VMConfig(vmName string) (VMConfig, error)
	UpdateVMConfig(vmName string, request VMConfigUpdateRequest) (VMConfig, error)
	RenameVM(vmName string, request VMRenameRequest) (VMConfig, error)
	UpdateVMAutostart(vmName string, request VMAutostartUpdateRequest) error
	UpdateVMConsole(vmName string, request VMConsoleUpdateRequest) (VMConfig, error)
	ConnectVMMedia(vmName string, request VMMediaConnectRequest) (VMConfig, error)
	DisconnectVMMedia(vmName string, request VMMediaDisconnectRequest) (VMConfig, error)
	UpdateVMXML(vmName string, request VMXMLUpdateRequest) (VMConfig, error)
	UpdateVMDevices(vmName string, request VMDeviceUpdateRequest) (VMConfig, error)
	CloneVM(vmName string, request VMCloneRequest) (VMConfig, error)
	CreateVM(request VMCreateRequest) (VMConfig, error)
	MigrateVM(vmName string, request VMMigrateRequest) error
	CheckMigrationConnection(request MigrationConnectionCheckRequest) (MigrationConnectionCheckResult, error)
	SetupMigrationSSHKey(request MigrationSSHKeySetupRequest) (MigrationConnectionCheckResult, error)
	SetupMigrationHostname(request MigrationHostnameSetupRequest) (MigrationConnectionCheckResult, error)
	ListSnapshots(vmName string) ([]Snapshot, error)
	CreateSnapshot(vmName string, request SnapshotCreateRequest) error
	ListStoragePools() ([]StoragePool, error)
	CreateStoragePool(request StoragePoolCreateRequest) (StoragePool, error)
	ListISOFiles(poolName string) ([]ISOFile, error)
	ListStorageVolumes(poolName string) ([]StorageVolume, error)
	CreateStorageVolume(poolName string, request StorageVolumeCreateRequest) (StorageVolume, error)
	UploadStorageVolume(poolName string, request StorageVolumeCreateRequest, content io.Reader) (StorageVolume, error)
	CloneStorageVolume(poolName string, request StorageVolumeCloneRequest) (StorageVolume, error)
	DeleteStoragePool(poolName string) error
	DeleteStorageVolume(poolName string, volumeName string) error
	UpdateStoragePoolState(poolName string, request PoolStateUpdateRequest) error
	UpdateStoragePoolAutostart(poolName string, request PoolAutostartUpdateRequest) error
	ListNetworkPools() ([]NetworkPool, error)
	CreateNetworkPool(request NetworkPoolCreateRequest) (NetworkPool, error)
	DeleteNetworkPool(poolName string) error
	UpdateNetworkPoolState(poolName string, request PoolStateUpdateRequest) error
	UpdateNetworkPoolAutostart(poolName string, request PoolAutostartUpdateRequest) error
	ConsoleInfo(vmName string) (ConsoleInfo, error)
	RevertSnapshot(vmName string, snapshotName string) error
	DeleteSnapshot(vmName string, snapshotName string) error
	StartVM(vmName string) error
	ShutdownVM(vmName string) error
	PauseVM(vmName string) error
	ResumeVM(vmName string) error
	RebootVM(vmName string) error
	ResetVM(vmName string) error
	DestroyVM(vmName string) error
	DeleteVM(vmName string) error
	ForceDeleteVM(vmName string) error
}
