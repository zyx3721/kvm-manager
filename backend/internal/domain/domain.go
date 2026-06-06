package domain

import (
	"encoding/json"
	"time"
)

type User struct {
	ID          string     `json:"id"`
	Username    string     `json:"username"`
	Email       string     `json:"email"`
	DisplayName string     `json:"displayName"`
	Role        string     `json:"role"`
	Source      string     `json:"source"`
	Roles       []Role     `json:"roles,omitempty"`
	Permissions []string   `json:"permissions,omitempty"`
	Disabled    bool       `json:"disabled"`
	LastLoginAt *time.Time `json:"lastLoginAt,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type Role struct {
	ID          string    `json:"id"`
	Key         string    `json:"key"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Permissions []string  `json:"permissions"`
	Builtin     bool      `json:"builtin"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type UserGroup struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Disabled    bool      `json:"disabled"`
	Members     []User    `json:"members,omitempty"`
	Roles       []Role    `json:"roles,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Permission struct {
	Key                   string `json:"key"`
	Name                  string `json:"name"`
	Description           string `json:"description"`
	Category              string `json:"category"`
	ImpliedReadPermission string `json:"impliedReadPermission,omitempty"`
}

type SystemBaseConfig struct {
	SiteName                         string    `json:"siteName"`
	LoginName                        string    `json:"loginName"`
	AppName                          string    `json:"appName"`
	AppSubtitle                      string    `json:"appSubtitle"`
	IconData                         string    `json:"iconData"`
	PasswordResetCodeTTLMinutes      int       `json:"passwordResetCodeTtlMinutes"`
	PasswordResetCaptchaTTLMinutes   int       `json:"passwordResetCaptchaTtlMinutes"`
	PasswordResetSendCooldownMinutes float64   `json:"passwordResetSendCooldownMinutes"`
	PasswordResetRateLimitMinutes    int       `json:"passwordResetRateLimitMinutes"`
	ResourceWarningThreshold         int       `json:"resourceWarningThreshold"`
	ResourceCriticalThreshold        int       `json:"resourceCriticalThreshold"`
	ResourceAlertConsecutiveCount    int       `json:"resourceAlertConsecutiveCount"`
	AgentOfflineFailureCount         int       `json:"agentOfflineFailureCount"`
	CreatedAt                        time.Time `json:"created_at"`
	UpdatedAt                        time.Time `json:"updated_at"`
}

type AuthProvider struct {
	ID        string          `json:"id"`
	Type      string          `json:"type"`
	Name      string          `json:"name"`
	Enabled   bool            `json:"enabled"`
	Config    json.RawMessage `json:"config" swaggertype:"object"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

type PublicAuthProvider struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
}

type Session struct {
	Token      string    `json:"token"`
	ExpiresAt  time.Time `json:"expires_at"`
	LastSeenAt time.Time `json:"last_seen_at"`
	User       User      `json:"user"`
}

type Host struct {
	ID                   string    `json:"id"`
	Name                 string    `json:"name"`
	Address              string    `json:"address"`
	Hostname             string    `json:"hostname"`
	Cluster              string    `json:"cluster"`
	Status               string    `json:"status"`
	CPUCores             int       `json:"cpuCores"`
	CPUUsage             int       `json:"cpuUsage"`
	MemoryBytes          int64     `json:"memoryBytes"`
	MemoryUsage          int       `json:"memoryUsage"`
	StorageBytes         int64     `json:"storageBytes"`
	StorageUsage         int       `json:"storageUsage"`
	DiskReadBytesPerSec  int64     `json:"diskReadBytesPerSecond"`
	DiskWriteBytesPerSec int64     `json:"diskWriteBytesPerSecond"`
	NetworkRxBytesPerSec int64     `json:"networkRxBytesPerSecond"`
	NetworkTxBytesPerSec int64     `json:"networkTxBytesPerSecond"`
	VMCount              int       `json:"vmCount"`
	KVMVersion           string    `json:"kvmVersion"`
	KVMFullVersion       string    `json:"kvmFullVersion"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}

type Agent struct {
	ID                 string     `json:"id"`
	Name               string     `json:"name"`
	Endpoint           string     `json:"endpoint"`
	TLSInsecure        bool       `json:"tlsInsecure"`
	Status             string     `json:"status"`
	Version            string     `json:"version"`
	Capabilities       []byte     `json:"capabilities"`
	LastHeartbeatAt    *time.Time `json:"lastHeartbeatAt,omitempty"`
	LastError          string     `json:"lastError"`
	TokenCiphertext    string     `json:"-"`
	FailureCount       int        `json:"failureCount"`
	LastSyncStartedAt  *time.Time `json:"lastSyncStartedAt,omitempty"`
	LastSyncFinishedAt *time.Time `json:"lastSyncFinishedAt,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

type Snapshot struct {
	ID          string    `json:"id"`
	HostID      string    `json:"hostId"`
	HostName    string    `json:"hostName"`
	VMID        string    `json:"vmId"`
	VMName      string    `json:"vmName"`
	Name        string    `json:"name"`
	DisplayName string    `json:"displayName"`
	Description string    `json:"description"`
	Tags        []string  `json:"tags"`
	SizeBytes   int64     `json:"sizeBytes"`
	Type        string    `json:"type"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type VirtualMachine struct {
	ID                   string    `json:"id"`
	HostID               string    `json:"hostId"`
	HostName             string    `json:"hostName"`
	Name                 string    `json:"name"`
	UUID                 string    `json:"uuid"`
	Description          string    `json:"description"`
	OSType               string    `json:"osType"`
	Status               string    `json:"status"`
	CPUCores             int       `json:"cpuCores"`
	MemoryBytes          int64     `json:"memoryBytes"`
	DiskBytes            int64     `json:"diskBytes"`
	DiskUsedBytes        int64     `json:"diskUsedBytes"`
	Disks                []VMDisk  `json:"disks"`
	PrimaryIP            string    `json:"primaryIp"`
	CPUUsage             int       `json:"cpuUsage"`
	CPUUsageAvailable    bool      `json:"cpuUsageAvailable"`
	MemoryUsage          int       `json:"memoryUsage"`
	MemoryUsageAvailable bool      `json:"memoryUsageAvailable"`
	DiskUsage            int       `json:"diskUsage"`
	DiskUsageAvailable   bool      `json:"diskUsageAvailable"`
	DiskReadBytesPerSec  int64     `json:"diskReadBytesPerSecond"`
	DiskWriteBytesPerSec int64     `json:"diskWriteBytesPerSecond"`
	NetworkRxBytesPerSec int64     `json:"networkRxBytesPerSecond"`
	NetworkTxBytesPerSec int64     `json:"networkTxBytesPerSecond"`
	UptimeSeconds        int64     `json:"uptimeSeconds"`
	IsTemplate           bool      `json:"isTemplate"`
	TemplateID           string    `json:"templateId,omitempty"`
	TemplateName         string    `json:"templateName,omitempty"`
	TemplateDescription  string    `json:"templateDescription,omitempty"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}

type VMDisk struct {
	Name      string `json:"name"`
	Path      string `json:"path"`
	Bytes     int64  `json:"bytes"`
	UsedBytes int64  `json:"usedBytes"`
}

type VMTemplateMark struct {
	ID          string    `json:"id"`
	AgentID     string    `json:"agentId"`
	VMUUID      string    `json:"vmUuid"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedBy   string    `json:"createdBy,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Task struct {
	ID           string          `json:"id"`
	Type         string          `json:"type"`
	Status       string          `json:"status"`
	TargetType   string          `json:"targetType"`
	TargetID     string          `json:"targetId"`
	Payload      json.RawMessage `json:"payload" swaggertype:"object"`
	ErrorMessage string          `json:"errorMessage,omitempty"`
	CreatedBy    string          `json:"createdBy"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
	FinishedAt   *time.Time      `json:"finished_at,omitempty"`
}

type AuditLog struct {
	ID           string          `json:"id"`
	UserID       string          `json:"userId"`
	Username     string          `json:"username"`
	Action       string          `json:"action"`
	ResourceType string          `json:"resourceType"`
	ResourceID   string          `json:"resourceId"`
	IPAddress    string          `json:"ipAddress"`
	Metadata     json.RawMessage `json:"metadata" swaggertype:"object"`
	CreatedAt    time.Time       `json:"created_at"`
}

type Alert struct {
	ID                 string          `json:"id"`
	Level              string          `json:"level"`
	Status             string          `json:"status"`
	SourceType         string          `json:"sourceType"`
	SourceID           string          `json:"sourceId"`
	Title              string          `json:"title"`
	Message            string          `json:"message"`
	Metadata           json.RawMessage `json:"metadata" swaggertype:"object"`
	FirstSeenAt        time.Time       `json:"firstSeenAt"`
	LastSeenAt         time.Time       `json:"lastSeenAt"`
	ResolvedAt         *time.Time      `json:"resolvedAt,omitempty"`
	NotificationSentAt *time.Time      `json:"notificationSentAt,omitempty"`
	ReadAt             *time.Time      `json:"readAt,omitempty"`
	DismissedAt        *time.Time      `json:"dismissedAt,omitempty"`
	CreatedAt          time.Time       `json:"created_at"`
	UpdatedAt          time.Time       `json:"updated_at"`
}

type DashboardSummary struct {
	TotalHosts    int              `json:"totalHosts"`
	OnlineHosts   int              `json:"onlineHosts"`
	TotalVMs      int              `json:"totalVMs"`
	RunningVMs    int              `json:"runningVMs"`
	StoppedVMs    int              `json:"stoppedVMs"`
	PausedVMs     int              `json:"pausedVMs"`
	ErrorVMs      int              `json:"errorVMs"`
	TotalVCPUs    int              `json:"totalVCPUs"`
	UsedVCPUs     int              `json:"usedVCPUs"`
	AverageCPU    int              `json:"averageCpu"`
	AverageMemory int              `json:"averageMemory"`
	TotalMemory   int64            `json:"totalMemoryBytes"`
	UsedMemory    int64            `json:"usedMemoryBytes"`
	TotalDisk     int64            `json:"totalDiskBytes"`
	UsedDisk      int64            `json:"usedDiskBytes"`
	StatusCounts  map[string]int   `json:"statusCounts"`
	RecentEvents  []AuditLog       `json:"recentEvents"`
	RecentVMs     []VirtualMachine `json:"recentVMs"`
	ActiveAlerts  []Alert          `json:"activeAlerts"`
}
