package router

import (
	"kvm-manager/backend/internal/domain"
	"kvm-manager/backend/pkg/agent"
)

type hostListResponse struct {
	Items []domain.Host `json:"items"`
	Total int           `json:"total"`
}

type hostInterfaceListResponse struct {
	Items []agent.HostInterface `json:"items"`
	Total int                   `json:"total"`
}

type hostInterfaceDeviceListResponse struct {
	Items []agent.HostInterfaceDevice `json:"items"`
	Total int                         `json:"total"`
}

type hostInterfaceCreateDocRequest struct {
	Name              string   `json:"name" example:"br0"`
	StartMode         string   `json:"startMode" example:"onboot"`
	Device            string   `json:"device" example:"em1"`
	Type              string   `json:"type" example:"bridge" enums:"bridge,ethernet"`
	STP               string   `json:"stp" example:"on"`
	Delay             string   `json:"delay" example:"0"`
	IPv4Mode          string   `json:"ipv4Mode" example:"static"`
	IPv4Address       string   `json:"ipv4Address" example:"172.18.0.11/24"`
	IPv4Gateway       string   `json:"ipv4Gateway" example:"172.18.0.1"`
	IPv6Mode          string   `json:"ipv6Mode" example:"none"`
	IPv6Address       string   `json:"ipv6Address" example:"2001:db8::10/64"`
	IPv6Gateway       string   `json:"ipv6Gateway" example:"2001:db8::1"`
	ApplySystemConfig bool     `json:"applySystemConfig" example:"true"`
	DNSServers        []string `json:"dnsServers" example:"223.5.5.5,8.8.8.8"`
}

type hostInterfaceStateDocRequest struct {
	Active bool `json:"active" example:"false"`
}

type storagePoolListResponse struct {
	Items []agent.StoragePool `json:"items"`
	Total int                 `json:"total"`
}

type storagePoolCreateDocRequest struct {
	Name       string `json:"name" example:"default"`
	Type       string `json:"type" example:"dir"`
	Path       string `json:"path" example:"/var/lib/libvirt/images"`
	Device     string `json:"device" example:"/dev/sdb"`
	SourceHost string `json:"sourceHost" example:"nfs.example.com"`
	SourcePath string `json:"sourcePath" example:"/srv/storage"`
	Format     string `json:"format" example:"nfs"`
}

type isoFileListResponse struct {
	Items []agent.ISOFile `json:"items"`
	Total int             `json:"total"`
}

type storageVolumeListResponse struct {
	Items []agent.StorageVolume `json:"items"`
	Total int                   `json:"total"`
}

type storageVolumeCreateDocRequest struct {
	Name             string `json:"name" example:"disk-01.qcow2"`
	Format           string `json:"format" example:"qcow2"`
	CapacityBytes    int64  `json:"capacityBytes" example:"21474836480"`
	PreallocMetadata bool   `json:"preallocMetadata" example:"true"`
}

type storageVolumeCloneDocRequest struct {
	Name             string `json:"name" example:"disk-01-clone.qcow2"`
	SourceName       string `json:"sourceName" example:"disk-01.qcow2"`
	Format           string `json:"format" example:"qcow2"`
	Convert          bool   `json:"convert" example:"true"`
	PreallocMetadata bool   `json:"preallocMetadata" example:"true"`
}

type networkPoolListResponse struct {
	Items []agent.NetworkPool `json:"items"`
	Total int                 `json:"total"`
}

type networkPoolCreateDocRequest struct {
	Name         string `json:"name" example:"default"`
	Subnet       string `json:"subnet" example:"192.168.100.0/24"`
	DHCP         bool   `json:"dhcp" example:"true"`
	FixedAddress bool   `json:"fixedAddress" example:"false"`
	Type         string `json:"type" example:"nat"`
	Bridge       string `json:"bridge" example:"br0"`
	OpenVSwitch  bool   `json:"openVSwitch" example:"false"`
}

type poolStateUpdateDocRequest struct {
	Active bool `json:"active" example:"true"`
}

type poolAutostartUpdateDocRequest struct {
	Autostart bool `json:"autostart" example:"true"`
}

type poolStateUpdateDocResponse struct {
	Active bool `json:"active" example:"true"`
}

type poolAutostartUpdateDocResponse struct {
	Autostart bool `json:"autostart" example:"true"`
}

type vmListResponse struct {
	Items []domain.VirtualMachine `json:"items"`
	Total int                     `json:"total"`
}

type vmSingleRefreshResponse struct {
	Status string                `json:"status" example:"ok"`
	VM     domain.VirtualMachine `json:"vm"`
}

type snapshotListResponse struct {
	Items []domain.Snapshot `json:"items"`
	Total int               `json:"total"`
}

type agentListResponse struct {
	Items []domain.Agent `json:"items"`
	Total int            `json:"total"`
}

type taskListResponse struct {
	Items []domain.Task `json:"items"`
	Total int           `json:"total"`
}

type auditLogListResponse struct {
	Items []domain.AuditLog `json:"items"`
	Total int               `json:"total"`
}

type alertListResponse struct {
	Items []domain.Alert `json:"items"`
	Total int            `json:"total"`
}

type alertNotificationDeliveryListResponse struct {
	Items []domain.AlertNotificationDelivery `json:"items"`
	Total int                                `json:"total"`
}

type unreadNotificationCountResponse struct {
	Count int `json:"count" example:"3"`
}

type notificationChannelListResponse struct {
	Items []domain.NotificationChannel `json:"items"`
	Total int                          `json:"total"`
}

type authProviderListResponse struct {
	Items []domain.AuthProvider `json:"items"`
	Total int                   `json:"total"`
}

type managedUserListResponse struct {
	Items []domain.User `json:"items"`
	Total int           `json:"total"`
}

type userRoleListResponse struct {
	Items []domain.Role `json:"items"`
	Total int           `json:"total"`
}

type userGroupListResponse struct {
	Items []domain.UserGroup `json:"items"`
	Total int                `json:"total"`
}

type permissionListResponse struct {
	Items []domain.Permission `json:"items"`
	Total int                 `json:"total"`
}

type publicAuthProviderListResponse struct {
	Items []domain.PublicAuthProvider `json:"items"`
	Total int                         `json:"total"`
}

type passwordResetChannelListResponse struct {
	Channels          []domain.PublicPasswordResetChannel `json:"channels"`
	VerificationToken string                              `json:"verificationToken"`
}

type passwordResetSendCodeResponse struct {
	Message         string `json:"message" example:"验证码已发送"`
	CooldownSeconds int    `json:"cooldownSeconds" example:"60"`
	ExpiresAt       string `json:"expiresAt"`
}

type notificationChannelRequestDoc struct {
	Enabled              bool           `json:"enabled" example:"true"`
	PasswordResetEnabled bool           `json:"passwordResetEnabled" example:"false"`
	ClearConfig          bool           `json:"clearConfig" example:"false"`
	Config               map[string]any `json:"config" swaggertype:"object"`
}

type notificationTemplatePreviewDoc struct {
	ProblemSubject  string         `json:"problemSubject"`
	ProblemText     string         `json:"problemText"`
	ProblemWebhook  map[string]any `json:"problemWebhook,omitempty" swaggertype:"object"`
	RecoverySubject string         `json:"recoverySubject"`
	RecoveryText    string         `json:"recoveryText"`
	RecoveryWebhook map[string]any `json:"recoveryWebhook,omitempty" swaggertype:"object"`
	ContentType     string         `json:"contentType,omitempty"`
	MessageType     string         `json:"messageType,omitempty"`
	ProblemTitle    string         `json:"problemTitle,omitempty"`
	RecoveryTitle   string         `json:"recoveryTitle,omitempty"`
	ProblemColor    string         `json:"problemColor,omitempty"`
	RecoveryColor   string         `json:"recoveryColor,omitempty"`
}

type authProviderRequestDoc struct {
	Name    string         `json:"name" example:"AD/LDAP"`
	Enabled bool           `json:"enabled" example:"true"`
	Config  map[string]any `json:"config" swaggertype:"object"`
}

type systemBaseConfigRequestDoc struct {
	SiteName                          string  `json:"siteName" example:"KVM Manager"`
	LoginName                         string  `json:"loginName" example:"KVM Manager"`
	AppName                           string  `json:"appName" example:"KVM Manager"`
	AppSubtitle                       string  `json:"appSubtitle" example:"VIRTUALIZATION OPS"`
	IconData                          string  `json:"iconData" example:"/favicon.svg"`
	PasswordResetCodeTTLMinutes       int     `json:"passwordResetCodeTtlMinutes" example:"10"`
	PasswordResetCaptchaTTLMinutes    int     `json:"passwordResetCaptchaTtlMinutes" example:"1"`
	PasswordResetSendCooldownMinutes  float64 `json:"passwordResetSendCooldownMinutes" example:"0.5"`
	PasswordResetRateLimitMinutes     int     `json:"passwordResetRateLimitMinutes" example:"5"`
	ResourceWarningThreshold          int     `json:"resourceWarningThreshold" example:"70"`
	ResourceCriticalThreshold         int     `json:"resourceCriticalThreshold" example:"85"`
	ResourceAlertConsecutiveCount     int     `json:"resourceAlertConsecutiveCount" example:"3"`
	AgentOfflineFailureCount          int     `json:"agentOfflineFailureCount" example:"3"`
	AlertNotificationTimeoutSeconds   int     `json:"alertNotificationTimeoutSeconds" example:"8"`
	AlertNotificationMaxRetryCount    int     `json:"alertNotificationMaxRetryCount" example:"6"`
	AlertNotificationRetryBaseSeconds int     `json:"alertNotificationRetryBaseSeconds" example:"30"`
	AlertNotificationRetryMaxMinutes  int     `json:"alertNotificationRetryMaxMinutes" example:"15"`
	AlertNotificationBatchSize        int     `json:"alertNotificationBatchSize" example:"50"`
}

type managedUserRequestDoc struct {
	Username    string   `json:"username" example:"zhangsan"`
	Password    string   `json:"password" example:"change-me-123"`
	Email       string   `json:"email" example:"zhangsan@example.com"`
	DisplayName string   `json:"displayName" example:"张三"`
	RoleKeys    []string `json:"roleKeys" example:"operator"`
	Disabled    bool     `json:"disabled" example:"false"`
}

type managedUserDisabledRequestDoc struct {
	Disabled bool `json:"disabled" example:"true"`
}

type userRoleRequestDoc struct {
	Key         string   `json:"key" example:"ops-custom"`
	Name        string   `json:"name" example:"自定义运维"`
	Description string   `json:"description" example:"允许日常运维操作"`
	Permissions []string `json:"permissions" example:"vms.read,vms.power"`
}

type userGroupRequestDoc struct {
	Name        string   `json:"name" example:"运维组"`
	Description string   `json:"description" example:"日常虚拟化运维团队"`
	Disabled    bool     `json:"disabled" example:"false"`
	MemberIDs   []string `json:"memberIds"`
	RoleKeys    []string `json:"roleKeys" example:"operator"`
}

type healthResponse struct {
	Status   string `json:"status"`
	Database string `json:"database"`
	Redis    string `json:"redis"`
	Time     string `json:"time"`
}

type meResponse struct {
	User       domain.User `json:"user"`
	ExpiresAt  string      `json:"expires_at"`
	LastSeenAt string      `json:"last_seen_at"`
}

type statusResponse struct {
	Status string `json:"status"`
}

type authProviderTestResponse struct {
	Status       string `json:"status" example:"ok"`
	MatchedUsers int    `json:"matchedUsers" example:"12"`
}

type messageResponse struct {
	Message string `json:"message"`
}

type refreshResponse struct {
	Status string      `json:"status"`
	Task   domain.Task `json:"task"`
}

type taskDetailResponse struct {
	Task domain.Task `json:"task"`
}

type agentProbeResponse struct {
	Status string `json:"status"`
	Host   any    `json:"host"`
}

type agentSyncResponse struct {
	Status string `json:"status"`
	Agent  string `json:"agent"`
}

type agentTokenRequest struct {
	Token string `json:"token" example:"please-change-agent-token"`
}

type vmActionResponse struct {
	VM   domain.VirtualMachine `json:"vm"`
	Task domain.Task           `json:"task"`
}

type vmConfigDocResponse struct {
	Name               string                 `json:"name" example:"prod-web-01"`
	UUID               string                 `json:"uuid" example:"550e8400-e29b-41d4-a716-446655440000"`
	OSType             string                 `json:"osType" example:"linux"`
	Status             string                 `json:"status" example:"running"`
	Description        string                 `json:"description" example:"生产 Web 服务"`
	Autostart          bool                   `json:"autostart" example:"true"`
	CurrentCPU         int                    `json:"currentCpu" example:"4"`
	MaximumCPU         int                    `json:"maximumCpu" example:"8"`
	HostCPU            int                    `json:"hostCpu" example:"32"`
	Arch               string                 `json:"arch" example:"x86_64"`
	CurrentMemoryBytes int64                  `json:"currentMemoryBytes" example:"8589934592"`
	MaximumMemoryBytes int64                  `json:"maximumMemoryBytes" example:"17179869184"`
	HostMemoryBytes    int64                  `json:"hostMemoryBytes" example:"137438953472"`
	MemoryStatsPeriod  int                    `json:"memoryStatsPeriod" example:"5"`
	Disks              []vmConfigDiskDoc      `json:"disks"`
	Interfaces         []vmConfigInterfaceDoc `json:"interfaces"`
	CDROMs             []vmConfigCDROMDoc     `json:"cdroms"`
	Graphics           vmConfigGraphicsDoc    `json:"graphics"`
	XML                string                 `json:"xml" example:"<domain type='kvm'>...</domain>"`
}

type vmConfigUpdateDocRequest struct {
	Description       string `json:"description" example:"生产 Web 服务"`
	CurrentCPU        int    `json:"currentCpu" example:"4"`
	MaximumCPU        int    `json:"maximumCpu" example:"8"`
	CurrentMemoryMB   int64  `json:"currentMemoryMB" example:"8192"`
	MaximumMemoryMB   int64  `json:"maximumMemoryMB" example:"16384"`
	MemoryStatsPeriod int    `json:"memoryStatsPeriod" example:"5"`
}

type vmRenameDocRequest struct {
	Name string `json:"name" example:"prod-web-02"`
}

type vmAutostartUpdateDocRequest struct {
	Autostart bool `json:"autostart" example:"true"`
}

type vmMediaConnectDocRequest struct {
	Target  string `json:"target" example:"sda"`
	ISOPath string `json:"isoPath" example:"/var/lib/libvirt/images/CentOS-7.iso"`
}

type vmMediaDisconnectDocRequest struct {
	Target string `json:"target" example:"sda"`
}

type vmXMLUpdateDocRequest struct {
	XML string `json:"xml" example:"<domain type='kvm'>...</domain>"`
}

type vmDeviceUpdateDocRequest struct {
	Interfaces        []vmDeviceInterfaceDocRequest       `json:"interfaces"`
	NewInterfaces     []vmDeviceNewInterfaceDocRequest    `json:"newInterfaces"`
	DeletedInterfaces []vmDeviceDeleteInterfaceDocRequest `json:"deletedInterfaces"`
	DiskResizes       []vmDeviceDiskResizeDocRequest      `json:"diskResizes"`
	NewDisks          []vmDeviceNewDiskDocRequest         `json:"newDisks"`
	DeletedDisks      []vmDeviceDeleteDiskDocRequest      `json:"deletedDisks"`
}

type vmDeviceInterfaceDocRequest struct {
	Name   string `json:"name" example:"vnet0"`
	MAC    string `json:"mac" example:"52:54:00:12:34:56"`
	Source string `json:"source" example:"default"`
}

type vmDeviceNewInterfaceDocRequest struct {
	Source string `json:"source" example:"default"`
	Model  string `json:"model" example:"e1000e"`
}

type vmDeviceDeleteInterfaceDocRequest struct {
	Name string `json:"name" example:"vnet0"`
	MAC  string `json:"mac" example:"52:54:00:12:34:56"`
}

type vmDeviceDiskResizeDocRequest struct {
	Name          string `json:"name" example:"vda"`
	CapacityBytes int64  `json:"capacityBytes" example:"214748364800"`
}

type vmDeviceDeleteDiskDocRequest struct {
	Name string `json:"name" example:"vdb"`
}

type vmDeviceNewDiskDocRequest struct {
	Name             string `json:"name" example:"prod-web-01-vdb.qcow2"`
	Pool             string `json:"pool" example:"default"`
	Target           string `json:"target" example:"vdb"`
	Bus              string `json:"bus" example:"virtio"`
	Format           string `json:"format" example:"qcow2"`
	CapacityBytes    int64  `json:"capacityBytes" example:"107374182400"`
	PreallocMetadata bool   `json:"preallocMetadata" example:"true"`
}

type vmCloneDocRequest struct {
	Name            string                       `json:"name" example:"prod-web-01-clone"`
	Description     string                       `json:"description" example:"生产 Web 服务克隆"`
	Autostart       bool                         `json:"autostart" example:"false"`
	CurrentCPU      int                          `json:"currentCpu" example:"2"`
	MaximumCPU      int                          `json:"maximumCpu" example:"4"`
	CurrentMemoryMB int64                        `json:"currentMemoryMB" example:"4096"`
	MaximumMemoryMB int64                        `json:"maximumMemoryMB" example:"8192"`
	CDROMPolicy     string                       `json:"cdromPolicy" example:"disconnect"`
	Interfaces      []vmCloneInterfaceDocRequest `json:"interfaces"`
	Disks           []vmCloneDiskDocRequest      `json:"disks"`
}

type vmCloneInterfaceDocRequest struct {
	Name   string `json:"name" example:"vnet0"`
	MAC    string `json:"mac" example:"52:54:00:12:34:56"`
	Source string `json:"source" example:"default"`
}

type vmCloneDiskDocRequest struct {
	Name             string `json:"name" example:"vda"`
	Pool             string `json:"pool" example:"default"`
	SourcePath       string `json:"sourcePath" example:"/var/lib/libvirt/images/prod-web-01.qcow2"`
	TargetName       string `json:"targetName" example:"prod-web-01-clone.qcow2"`
	PreallocMetadata bool   `json:"preallocMetadata" example:"true"`
}

type vmCreateDiskDocRequest struct {
	Name             string `json:"name" example:"prod-web-02-vda.qcow2"`
	Pool             string `json:"pool" example:"default"`
	Format           string `json:"format" example:"qcow2"`
	Bus              string `json:"bus" example:"virtio"`
	CapacityGB       int64  `json:"capacityGB" example:"100"`
	PreallocMetadata bool   `json:"preallocMetadata" example:"true"`
}

type vmCreateDocRequest struct {
	AgentID          string                   `json:"agentId" example:"agent-01"`
	Name             string                   `json:"name" example:"prod-web-02"`
	Description      string                   `json:"description" example:"生产 Web 服务"`
	Autostart        bool                     `json:"autostart" example:"false"`
	CurrentCPU       int                      `json:"currentCpu" example:"2"`
	MaximumCPU       int                      `json:"maximumCpu" example:"4"`
	CurrentMemoryMB  int64                    `json:"currentMemoryMB" example:"4096"`
	MaximumMemoryMB  int64                    `json:"maximumMemoryMB" example:"8192"`
	CPUModel         string                   `json:"cpuModel" example:"host-passthrough"`
	OSType           string                   `json:"osType" example:"linux"`
	Disks            []vmCreateDiskDocRequest `json:"disks"`
	DiskName         string                   `json:"diskName" example:"prod-web-02-vda.qcow2"`
	DiskPool         string                   `json:"diskPool" example:"default"`
	DiskFormat       string                   `json:"diskFormat" example:"qcow2"`
	DiskBus          string                   `json:"diskBus" example:"virtio"`
	DiskCapacityGB   int64                    `json:"diskCapacityGB" example:"100"`
	PreallocMetadata bool                     `json:"preallocMetadata" example:"true"`
	ISOPath          string                   `json:"isoPath" example:"/var/lib/libvirt/images/CentOS-7.iso"`
	ISOBus           string                   `json:"isoBus" example:"sata"`
	NetworkSource    string                   `json:"networkSource" example:"default"`
	NetworkModel     string                   `json:"networkModel" example:"virtio"`
	Graphics         string                   `json:"graphics" example:"vnc"`
	ConsolePassword  string                   `json:"consolePassword" example:""`
	BootFirmware     string                   `json:"bootFirmware" example:"bios"`
	XML              string                   `json:"xml" example:"<domain type='kvm'>...</domain>"`
}

type vmTemplateMarkDocRequest struct {
	Name        string `json:"name" example:"CentOS 7 基础模板"`
	Description string `json:"description" example:"已安装 Guest Agent 和基础运维工具"`
}

type vmTemplateMarkDocResponse struct {
	Template domain.VMTemplateMark `json:"template"`
	Task     domain.Task           `json:"task"`
}

type vmTemplateUnmarkDocResponse struct {
	Status string      `json:"status" example:"ok"`
	Task   domain.Task `json:"task"`
}

type vmMigrateDocRequest struct {
	TargetAgentID  string `json:"targetAgentId" example:"agent-02"`
	DestinationURI string `json:"destinationUri" example:"qemu+ssh://compute02/system"`
	Live           bool   `json:"live" example:"true"`
	CopyDisks      bool   `json:"copyDisks" example:"true"`
	Persistent     bool   `json:"persistent" example:"true"`
	UndefineSource bool   `json:"undefineSource" example:"true"`
	AutoConverge   bool   `json:"autoConverge" example:"true"`
	PostCopy       bool   `json:"postCopy" example:"false"`
}

type vmMigrationPrecheckDocResponse struct {
	Passed bool                         `json:"passed" example:"true"`
	Items  []vmMigrationPrecheckItemDoc `json:"items"`
}

type vmMigrationPrecheckItemDoc struct {
	Key     string `json:"key" example:"migration-channel"`
	Label   string `json:"label" example:"迁移通道"`
	Status  string `json:"status" example:"passed"`
	Message string `json:"message" example:"迁移通道可非交互连接"`
	Code    string `json:"code,omitempty" example:"vm_migrate_ssh_password_required"`
}

type vmMigrationSSHKeyDocRequest struct {
	TargetAgentID  string `json:"targetAgentId" example:"agent-02"`
	DestinationURI string `json:"destinationUri" example:"qemu+ssh://compute02/system"`
	Username       string `json:"username" example:"root"`
	Password       string `json:"password" example:""`
}

type vmMigrationSSHKeyDocResponse struct {
	Status string                               `json:"status" example:"ok"`
	Result agent.MigrationConnectionCheckResult `json:"result"`
}

type vmMigrationHostnameDocRequest struct {
	TargetAgentID  string `json:"targetAgentId" example:"agent-02"`
	DestinationURI string `json:"destinationUri" example:"qemu+ssh://compute02/system"`
	Hostname       string `json:"hostname" example:"kvm02"`
}

type vmMigrationHostnameDocResponse struct {
	Status string                               `json:"status" example:"ok"`
	Result agent.MigrationConnectionCheckResult `json:"result"`
}

type vmConfigUpdateDocResponse struct {
	Config vmConfigDocResponse `json:"config"`
	Task   domain.Task         `json:"task"`
}

type vmRenameDocResponse struct {
	Config vmConfigDocResponse `json:"config"`
	Task   domain.Task         `json:"task"`
}

type vmAutostartUpdateDocResponse struct {
	Autostart bool        `json:"autostart" example:"true"`
	Task      domain.Task `json:"task"`
}

type vmConsoleInfoDocResponse struct {
	Type            string `json:"type" example:"vnc"`
	Listen          string `json:"listen" example:"127.0.0.1"`
	Port            int    `json:"port" example:"5901"`
	PasswordEnabled bool   `json:"passwordEnabled" example:"true"`
}

type vmConsoleUpdateDocRequest struct {
	PasswordEnabled bool   `json:"passwordEnabled" example:"true"`
	Password        string `json:"password" example:"secret123"`
}

type vmConsoleUpdateDocResponse struct {
	Config vmConfigDocResponse `json:"config"`
	Task   domain.Task         `json:"task"`
}

type vmMediaConnectDocResponse struct {
	Config vmConfigDocResponse `json:"config"`
	Task   domain.Task         `json:"task"`
}

type vmMediaDisconnectDocResponse struct {
	Config vmConfigDocResponse `json:"config"`
	Task   domain.Task         `json:"task"`
}

type vmXMLUpdateDocResponse struct {
	Config vmConfigDocResponse `json:"config"`
	Task   domain.Task         `json:"task"`
}

type vmDeviceUpdateDocResponse struct {
	Config vmConfigDocResponse `json:"config"`
	Task   domain.Task         `json:"task"`
}

type vmCloneDocResponse struct {
	Status string      `json:"status" example:"queued"`
	Task   domain.Task `json:"task"`
}

type vmAsyncTaskDocResponse struct {
	Status string      `json:"status" example:"queued"`
	Task   domain.Task `json:"task"`
}

type vmConfigDiskDoc struct {
	Name       string `json:"name" example:"vda"`
	Path       string `json:"path" example:"/var/lib/libvirt/images/prod-web-01.snap"`
	SourcePath string `json:"sourcePath" example:"/var/lib/libvirt/images/prod-web-01.qcow2"`
	Pool       string `json:"pool" example:"default"`
	Bus        string `json:"bus" example:"virtio"`
	Device     string `json:"device" example:"disk"`
	Type       string `json:"type" example:"file"`
	Bytes      int64  `json:"bytes" example:"107374182400"`
}

type vmConfigInterfaceDoc struct {
	Name   string `json:"name" example:"vnet0"`
	MAC    string `json:"mac" example:"52:54:00:12:34:56"`
	Type   string `json:"type" example:"network"`
	Source string `json:"source" example:"default"`
	Model  string `json:"model" example:"virtio"`
}

type vmConfigCDROMDoc struct {
	Name      string `json:"name" example:"sda"`
	Path      string `json:"path" example:"/iso/ubuntu.iso"`
	Bus       string `json:"bus" example:"sata"`
	Connected bool   `json:"connected" example:"true"`
}

type vmConfigGraphicsDoc struct {
	Type            string `json:"type" example:"vnc"`
	Listen          string `json:"listen" example:"0.0.0.0"`
	Port            string `json:"port" example:"5901"`
	PasswordEnabled bool   `json:"passwordEnabled" example:"true"`
}

type snapshotActionResponse struct {
	Snapshot domain.Snapshot `json:"snapshot"`
	Task     domain.Task     `json:"task"`
}

type metricPointResponse struct {
	Time                    string `json:"time" example:"2026-05-21T12:00:00Z"`
	CPU                     int    `json:"cpu" example:"42"`
	Memory                  int    `json:"memory" example:"58"`
	Storage                 int    `json:"storage,omitempty" example:"61"`
	Disk                    int    `json:"disk,omitempty" example:"35"`
	DiskReadBytesPerSecond  int64  `json:"diskReadBytesPerSecond,omitempty" example:"204800"`
	DiskWriteBytesPerSecond int64  `json:"diskWriteBytesPerSecond,omitempty" example:"102400"`
	NetworkRxBytesPerSecond int64  `json:"networkRxBytesPerSecond,omitempty" example:"409600"`
	NetworkTxBytesPerSecond int64  `json:"networkTxBytesPerSecond,omitempty" example:"307200"`
	VMCount                 int    `json:"vmCount,omitempty" example:"12"`
}

type metricSeriesDocResponse struct {
	Range  string                `json:"range" example:"1h"`
	Bucket string                `json:"bucket" example:"1m0s"`
	Items  []metricPointResponse `json:"items"`
}
