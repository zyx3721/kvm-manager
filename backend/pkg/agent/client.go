package agent

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

var inactiveDomainSnapshotDeletePattern = regexp.MustCompile(`(?i)cannot delete inactive domain with ([0-9]+) snapshots?`)

type Client struct{ httpClient *http.Client }

const storageOperationTimeout = time.Hour
const hostInterfaceOperationTimeout = 30 * time.Second

type HTTPStatusError struct {
	StatusCode int
	Status     string
	Message    string
}

func (e HTTPStatusError) Error() string {
	return fmt.Sprintf("agent returned %s", e.Status)
}

func UserFacingErrorMessage(err error) string {
	if strings.HasPrefix(err.Error(), "Agent ") {
		return err.Error()
	}
	var statusErr HTTPStatusError
	if errors.As(err, &statusErr) {
		switch statusErr.StatusCode {
		case http.StatusUnauthorized:
			return "Agent 认证失败，令牌不正确或已失效"
		case http.StatusForbidden:
			return "Agent 拒绝访问，当前令牌没有权限"
		case http.StatusNotFound:
			return "Agent 接口不存在，请确认 Agent 地址是否填写到服务根路径"
		default:
			if strings.TrimSpace(statusErr.Message) != "" {
				return normalizeAgentUserFacingMessage(statusErr.Message)
			}
			return "Agent 返回异常状态：" + statusErr.Status
		}
	}
	if errors.Is(err, context.Canceled) {
		return "Agent 请求已取消"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "Agent 连接超时，请确认地址、端口和网络可达"
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return "Agent 连接超时，请确认地址、端口和网络可达"
	}
	var unknownAuthority x509.UnknownAuthorityError
	if errors.As(err, &unknownAuthority) {
		return "Agent TLS 证书不受信任，如使用自签名证书，可临时启用跳过 TLS 校验"
	}
	lowerMessage := strings.ToLower(err.Error())
	switch {
	case strings.Contains(lowerMessage, "connection refused"):
		return "Agent 连接被拒绝，请确认 Agent 服务已启动，且端口开放"
	case strings.Contains(lowerMessage, "no such host"):
		return "Agent 地址无法解析，请检查域名或主机名是否正确"
	case strings.Contains(lowerMessage, "server gave http response to https client"):
		return "Agent 协议不匹配，当前使用 HTTPS 访问了 HTTP 服务"
	default:
		return "Agent 连接失败"
	}
}

func normalizeAgentUserFacingMessage(message string) string {
	detail := strings.Join(strings.Fields(strings.TrimSpace(message)), " ")
	lowerDetail := strings.ToLower(detail)
	switch {
	case strings.Contains(lowerDetail, "snapshot-revert") && strings.Contains(lowerDetail, "no such file or directory"):
		return snapshotRevertMissingDiskMessage(detail)
	case isGuestMemoryAllocationError(lowerDetail):
		return "目标宿主机可用内存不足，无法为迁移虚拟机分配内存，请释放目标宿主机内存或降低虚拟机内存后重试"
	case strings.Contains(lowerDetail, "cannot access storage file") && strings.Contains(lowerDetail, "no such file or directory") && isMigrationStorageAccessError(lowerDetail):
		return migrationStorageAccessMessage(detail)
	case strings.Contains(lowerDetail, "disk image has internal snapshots and cannot be resized") ||
		strings.Contains(lowerDetail, "image has snapshots") ||
		strings.Contains(lowerDetail, "does not support resize"):
		return "磁盘镜像包含内部快照，无法直接扩容。请先删除该虚拟机的内部快照，或重新创建无内部快照的磁盘后再扩容"
	case strings.Contains(lowerDetail, "domain is already paused"):
		return "虚拟机当前已暂停"
	case isDomainAlreadyRunningError(lowerDetail):
		if isVirshAction(lowerDetail, "start") {
			return "虚拟机当前已运行"
		}
		return "虚拟机当前已运行"
	case isDomainNotRunningError(lowerDetail):
		switch {
		case isVirshAction(lowerDetail, "shutdown"):
			return "虚拟机当前已关机"
		case isVirshAction(lowerDetail, "destroy"):
			return "虚拟机当前已关机"
		case isVirshAction(lowerDetail, "suspend"):
			return "虚拟机当前未运行，无法暂停"
		case isVirshAction(lowerDetail, "reboot") || isVirshAction(lowerDetail, "reset"):
			return "虚拟机当前未运行，无法重启"
		default:
			return "虚拟机当前未运行，无法执行该操作"
		}
	case strings.Contains(lowerDetail, "domain is not paused"):
		return "虚拟机当前未暂停，无法恢复"
	case strings.Contains(lowerDetail, "domain is paused"):
		return "虚拟机当前已暂停，请先恢复后再执行该操作"
	case strings.Contains(lowerDetail, "cannot delete inactive domain with") && strings.Contains(lowerDetail, "snapshot"):
		return vmDeleteHasSnapshotsMessage(detail)
	case strings.Contains(lowerDetail, "must be stopped before delete"):
		return "请先关闭虚拟机后再删除"
	case strings.Contains(lowerDetail, "target storage pool") && strings.Contains(lowerDetail, "is unavailable"):
		return targetStoragePoolUnavailableMessage(detail)
	case strings.Contains(lowerDetail, "target storage pools are unavailable"):
		return "目标宿主机存储池不可用，请确认目标宿主机存在可用存储池"
	case strings.Contains(lowerDetail, "target disk already exists"):
		return "目标宿主机已存在同名磁盘文件，请先删除目标磁盘或更换源虚拟机磁盘名称后再迁移"
	case strings.Contains(lowerDetail, "migration disk storage pool is unavailable"):
		return "源虚拟机磁盘无法识别所属存储池，无法执行冷迁移复制磁盘"
	case strings.Contains(lowerDetail, "migration disk path is unavailable"):
		return "源虚拟机磁盘路径不可用，无法执行冷迁移复制磁盘"
	case strings.Contains(lowerDetail, "hostname on destination resolved to localhost") && strings.Contains(lowerDetail, "migration requires an fqdn"):
		return "目标宿主机主机名解析为 localhost，热迁移需要目标主机名解析到真实网络地址，请检查目标宿主机 hostname 和 /etc/hosts"
	case strings.Contains(lowerDetail, "vm xml name mismatch"):
		return "XML 中的虚拟机名称必须与当前虚拟机名称一致，如需改名请在基本信息中修改"
	case strings.Contains(lowerDetail, "maximum cpu exceeds host cpu"):
		return "最大 CPU 不能超过宿主机逻辑 CPU"
	case strings.Contains(lowerDetail, "maximum memory exceeds host memory"):
		return "最大内存不能超过宿主机总内存"
	default:
		return detail
	}
}

func snapshotRevertMissingDiskMessage(detail string) string {
	path := missingQemuImgPath(detail)
	if path == "" {
		return "快照恢复失败：缺少磁盘文件，请找回文件或删除失效快照"
	}
	return "快照恢复失败：缺少磁盘文件 " + path + "，请找回文件或删除失效快照"
}

func isMigrationStorageAccessError(lowerDetail string) bool {
	return strings.Contains(lowerDetail, " migrate ") ||
		strings.Contains(lowerDetail, "qemu+ssh://")
}

func isGuestMemoryAllocationError(lowerDetail string) bool {
	return strings.Contains(lowerDetail, "cannot allocate memory") &&
		(strings.Contains(lowerDetail, "cannot set up guest memory") ||
			strings.Contains(lowerDetail, "pc.ram") ||
			strings.Contains(lowerDetail, "qemu unexpectedly closed the monitor"))
}

func migrationStorageAccessMessage(detail string) string {
	path := missingLibvirtStoragePath(detail)
	if path == "" {
		return "目标宿主机无法访问源磁盘路径，请确认目标宿主机存在该路径所在目录或存储池后重试"
	}
	return "目标宿主机无法访问源磁盘路径 " + path + "，请确认目标宿主机存在该路径所在目录或存储池后重试"
}

func targetStoragePoolUnavailableMessage(detail string) string {
	path := migrationTargetDiskPath(detail)
	if path != "" {
		return "目标宿主机没有源磁盘路径 " + path + " 所在的存储池，无法执行迁移复制磁盘"
	}
	fields := strings.Fields(detail)
	for i := 0; i+3 < len(fields); i++ {
		if strings.EqualFold(fields[i], "target") &&
			strings.EqualFold(fields[i+1], "storage") &&
			strings.EqualFold(fields[i+2], "pool") {
			return "目标宿主机存储池不满足源磁盘路径要求，无法执行迁移复制磁盘"
		}
	}
	return "目标宿主机存储池不满足源磁盘路径要求，无法执行迁移复制磁盘"
}

func migrationTargetDiskPath(detail string) string {
	const marker = "target storage pool path for disk "
	lower := strings.ToLower(detail)
	start := strings.Index(lower, marker)
	if start < 0 {
		return ""
	}
	start += len(marker)
	end := strings.Index(strings.ToLower(detail[start:]), " is unavailable")
	if end < 0 {
		return ""
	}
	return strings.TrimSpace(detail[start : start+end])
}

func missingQemuImgPath(detail string) string {
	const marker = "Could not open '"
	start := strings.LastIndex(detail, marker)
	if start < 0 {
		return ""
	}
	start += len(marker)
	end := strings.Index(detail[start:], "'")
	if end < 0 {
		return ""
	}
	return detail[start : start+end]
}

func missingLibvirtStoragePath(detail string) string {
	const marker = "Cannot access storage file '"
	lower := strings.ToLower(detail)
	start := strings.LastIndex(lower, strings.ToLower(marker))
	if start < 0 {
		return ""
	}
	start += len(marker)
	end := strings.Index(detail[start:], "'")
	if end < 0 {
		return ""
	}
	return detail[start : start+end]
}

func vmDeleteHasSnapshotsMessage(detail string) string {
	matches := inactiveDomainSnapshotDeletePattern.FindStringSubmatch(detail)
	if len(matches) == 2 {
		return "删除虚拟机失败：仍存在 " + matches[1] + " 个快照，请先删除快照后重试"
	}
	return "删除虚拟机失败：仍存在快照，请先删除快照后重试"
}

func isDomainAlreadyRunningError(detail string) bool {
	return strings.Contains(detail, "domain is already running") ||
		strings.Contains(detail, "domain is already active") ||
		strings.Contains(detail, "domain already active") ||
		strings.Contains(detail, "domain already exists as active")
}

func isDomainNotRunningError(detail string) bool {
	return strings.Contains(detail, "domain is not running") ||
		strings.Contains(detail, "domain is not active") ||
		strings.Contains(detail, "domain not active")
}

func isVirshAction(detail string, action string) bool {
	return strings.Contains(detail, " "+action+" ") ||
		strings.Contains(detail, " "+action+":") ||
		strings.Contains(detail, "failed to "+action+" domain")
}

type Config struct {
	Endpoint    string
	Token       string
	TLSInsecure bool
}

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

type StoragePoolListResponse struct {
	Items []StoragePool `json:"items"`
	Total int           `json:"total"`
}

type HostInterfaceListResponse struct {
	Items []HostInterface `json:"items"`
	Total int             `json:"total"`
}

type HostInterfaceDeviceListResponse struct {
	Items []HostInterfaceDevice `json:"items"`
	Total int                   `json:"total"`
}

type ISOFileListResponse struct {
	Items []ISOFile `json:"items"`
	Total int       `json:"total"`
}

type StorageVolumeListResponse struct {
	Items []StorageVolume `json:"items"`
	Total int             `json:"total"`
}

type NetworkPoolListResponse struct {
	Items []NetworkPool `json:"items"`
	Total int           `json:"total"`
}

func NewClient(tlsInsecure bool) *Client {
	return NewClientWithTimeout(tlsInsecure, 10*time.Second)
}

func NewClientWithTimeout(tlsInsecure bool, timeout time.Duration) *Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: tlsInsecure}
	return &Client{httpClient: &http.Client{Timeout: timeout, Transport: transport}}
}

func (c *Client) withTimeout(timeout time.Duration) *Client {
	if timeout <= 0 || c == nil || c.httpClient == nil {
		return c
	}
	next := *c.httpClient
	next.Timeout = timeout
	return &Client{httpClient: &next}
}

func (c *Client) HostInfo(ctx context.Context, cfg Config) (HostInfo, error) {
	var info HostInfo
	if err := c.get(ctx, cfg, "/v1/host", &info); err != nil {
		return HostInfo{}, err
	}
	return info, nil
}

func (c *Client) ListHostInterfaces(ctx context.Context, cfg Config) ([]HostInterface, error) {
	var response HostInterfaceListResponse
	if err := c.get(ctx, cfg, "/v1/host/interfaces", &response); err != nil {
		return nil, err
	}
	return response.Items, nil
}

func (c *Client) CreateHostInterface(ctx context.Context, cfg Config, request HostInterfaceCreateRequest) (HostInterface, error) {
	body, err := json.Marshal(request)
	if err != nil {
		return HostInterface{}, err
	}
	var item HostInterface
	if err := c.withTimeout(hostInterfaceOperationTimeout).postWithTarget(ctx, cfg, "/v1/host/interfaces", body, &item); err != nil {
		return HostInterface{}, err
	}
	return item, nil
}

func (c *Client) ListHostInterfaceDevices(ctx context.Context, cfg Config) ([]HostInterfaceDevice, error) {
	var response HostInterfaceDeviceListResponse
	if err := c.get(ctx, cfg, "/v1/host/interfaces/devices/list", &response); err != nil {
		return nil, err
	}
	return response.Items, nil
}

func (c *Client) DeleteHostInterface(ctx context.Context, cfg Config, name string) error {
	return c.delete(ctx, cfg, "/v1/host/interfaces/"+urlPathEscape(name)+"/delete")
}

func (c *Client) UpdateHostInterfaceState(ctx context.Context, cfg Config, name string, request PoolStateUpdateRequest) error {
	body, err := json.Marshal(request)
	if err != nil {
		return err
	}
	return c.put(ctx, cfg, "/v1/host/interfaces/"+urlPathEscape(name)+"/state", body, nil)
}

func (c *Client) ListStoragePools(ctx context.Context, cfg Config) ([]StoragePool, error) {
	var response StoragePoolListResponse
	if err := c.get(ctx, cfg, "/v1/storage-pools", &response); err != nil {
		return nil, err
	}
	return response.Items, nil
}

func (c *Client) CreateStoragePool(ctx context.Context, cfg Config, request StoragePoolCreateRequest) (StoragePool, error) {
	body, err := json.Marshal(request)
	if err != nil {
		return StoragePool{}, err
	}
	var pool StoragePool
	if err := c.postWithTarget(ctx, cfg, "/v1/storage-pools", body, &pool); err != nil {
		return StoragePool{}, err
	}
	return pool, nil
}

func (c *Client) ListISOFiles(ctx context.Context, cfg Config, poolName string) ([]ISOFile, error) {
	var response ISOFileListResponse
	if err := c.get(ctx, cfg, "/v1/storage-pools/"+urlPathEscape(poolName)+"/iso-files", &response); err != nil {
		return nil, err
	}
	return response.Items, nil
}

func (c *Client) ListStorageVolumes(ctx context.Context, cfg Config, poolName string) ([]StorageVolume, error) {
	var response StorageVolumeListResponse
	if err := c.get(ctx, cfg, "/v1/storage-pools/"+urlPathEscape(poolName)+"/volumes", &response); err != nil {
		return nil, err
	}
	return response.Items, nil
}

func (c *Client) CreateStorageVolume(ctx context.Context, cfg Config, poolName string, request StorageVolumeCreateRequest) (StorageVolume, error) {
	body, err := json.Marshal(request)
	if err != nil {
		return StorageVolume{}, err
	}
	var volume StorageVolume
	if err := c.postWithTarget(ctx, cfg, "/v1/storage-pools/"+urlPathEscape(poolName)+"/volumes", body, &volume); err != nil {
		return StorageVolume{}, err
	}
	return volume, nil
}

func (c *Client) CloneStorageVolume(ctx context.Context, cfg Config, poolName string, request StorageVolumeCloneRequest) (StorageVolume, error) {
	body, err := json.Marshal(request)
	if err != nil {
		return StorageVolume{}, err
	}
	var volume StorageVolume
	if err := c.withTimeout(storageOperationTimeout).postWithTarget(ctx, cfg, "/v1/storage-pools/"+urlPathEscape(poolName)+"/volumes/clone", body, &volume); err != nil {
		return StorageVolume{}, err
	}
	return volume, nil
}

func (c *Client) DeleteStorageVolume(ctx context.Context, cfg Config, poolName string, volumeName string) error {
	path := "/v1/storage-pools/" + urlPathEscape(poolName) + "/volumes?name=" + url.QueryEscape(volumeName)
	return c.delete(ctx, cfg, path)
}

func (c *Client) DeleteStoragePool(ctx context.Context, cfg Config, poolName string) error {
	return c.delete(ctx, cfg, "/v1/storage-pools/"+urlPathEscape(poolName)+"/delete")
}

func (c *Client) UpdateStoragePoolState(ctx context.Context, cfg Config, poolName string, request PoolStateUpdateRequest) error {
	body, err := json.Marshal(request)
	if err != nil {
		return err
	}
	return c.put(ctx, cfg, "/v1/storage-pools/"+urlPathEscape(poolName)+"/state", body, nil)
}

func (c *Client) UpdateStoragePoolAutostart(ctx context.Context, cfg Config, poolName string, request PoolAutostartUpdateRequest) error {
	body, err := json.Marshal(request)
	if err != nil {
		return err
	}
	return c.put(ctx, cfg, "/v1/storage-pools/"+urlPathEscape(poolName)+"/autostart", body, nil)
}

func (c *Client) ListNetworkPools(ctx context.Context, cfg Config) ([]NetworkPool, error) {
	var response NetworkPoolListResponse
	if err := c.get(ctx, cfg, "/v1/network-pools", &response); err != nil {
		return nil, err
	}
	return response.Items, nil
}

func (c *Client) CreateNetworkPool(ctx context.Context, cfg Config, request NetworkPoolCreateRequest) (NetworkPool, error) {
	body, err := json.Marshal(request)
	if err != nil {
		return NetworkPool{}, err
	}
	var pool NetworkPool
	if err := c.postWithTarget(ctx, cfg, "/v1/network-pools", body, &pool); err != nil {
		return NetworkPool{}, err
	}
	return pool, nil
}

func (c *Client) DeleteNetworkPool(ctx context.Context, cfg Config, poolName string) error {
	return c.delete(ctx, cfg, "/v1/network-pools/"+urlPathEscape(poolName)+"/delete")
}

func (c *Client) UpdateNetworkPoolState(ctx context.Context, cfg Config, poolName string, request PoolStateUpdateRequest) error {
	body, err := json.Marshal(request)
	if err != nil {
		return err
	}
	return c.put(ctx, cfg, "/v1/network-pools/"+urlPathEscape(poolName)+"/state", body, nil)
}

func (c *Client) UpdateNetworkPoolAutostart(ctx context.Context, cfg Config, poolName string, request PoolAutostartUpdateRequest) error {
	body, err := json.Marshal(request)
	if err != nil {
		return err
	}
	return c.put(ctx, cfg, "/v1/network-pools/"+urlPathEscape(poolName)+"/autostart", body, nil)
}

func (c *Client) RunVMAction(ctx context.Context, cfg Config, vmName string, action string) error {
	return c.post(ctx, cfg, "/v1/vms/"+urlPathEscape(vmName)+"/"+action, nil)
}

func (c *Client) get(ctx context.Context, cfg Config, path string, target any) error {
	return c.do(ctx, http.MethodGet, cfg, path, nil, target)
}

func (c *Client) post(ctx context.Context, cfg Config, path string, body []byte) error {
	return c.do(ctx, http.MethodPost, cfg, path, body, nil)
}

func (c *Client) postWithTarget(ctx context.Context, cfg Config, path string, body []byte, target any) error {
	return c.do(ctx, http.MethodPost, cfg, path, body, target)
}

func (c *Client) put(ctx context.Context, cfg Config, path string, body []byte, target any) error {
	return c.do(ctx, http.MethodPut, cfg, path, body, target)
}

func (c *Client) delete(ctx context.Context, cfg Config, path string) error {
	return c.do(ctx, http.MethodDelete, cfg, path, nil, nil)
}

func (c *Client) deleteWithTarget(ctx context.Context, cfg Config, path string, body []byte, target any) error {
	return c.do(ctx, http.MethodDelete, cfg, path, body, target)
}

func (c *Client) do(ctx context.Context, method string, cfg Config, path string, body []byte, target any) error {
	return c.doWithContentType(ctx, method, cfg, path, body, "", target)
}

func (c *Client) doWithContentType(ctx context.Context, method string, cfg Config, path string, body []byte, contentType string, target any) error {
	return c.doReaderWithContentType(ctx, method, cfg, path, bytes.NewReader(body), contentType, target)
}

func (c *Client) doReaderWithContentType(ctx context.Context, method string, cfg Config, path string, body io.Reader, contentType string, target any) error {
	endpoint := strings.TrimRight(cfg.Endpoint, "/") + path
	req, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+cfg.Token)
	if body != nil && method != http.MethodGet {
		if contentType == "" {
			contentType = "application/json"
		}
		req.Header.Set("Content-Type", contentType)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return HTTPStatusError{StatusCode: resp.StatusCode, Status: resp.Status, Message: readAgentErrorMessage(resp.Body)}
	}
	if target == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(target)
}

func readAgentErrorMessage(body io.Reader) string {
	var payload struct {
		Message string `json:"message"`
		Error   string `json:"error"`
	}
	if err := json.NewDecoder(io.LimitReader(body, 4096)).Decode(&payload); err != nil {
		return ""
	}
	message := strings.TrimSpace(payload.Message)
	if message != "" {
		return message
	}
	return strings.TrimSpace(payload.Error)
}

func urlPathEscape(value string) string {
	return strings.ReplaceAll(url.QueryEscape(value), "+", "%20")
}
