package router

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strings"
)

var inactiveDomainSnapshotDeletePattern = regexp.MustCompile(`(?i)cannot delete inactive domain with ([0-9]+) snapshots?`)

func decodeJSONBody(w http.ResponseWriter, req *http.Request, target any) error {
	return decodeJSONBodyLimit(w, req, 1<<20, target)
}

func decodeJSONBodyLimit(w http.ResponseWriter, req *http.Request, maxBytes int64, target any) error {
	decoder := json.NewDecoder(http.MaxBytesReader(w, req.Body, maxBytes))
	decoder.DisallowUnknownFields()
	return decoder.Decode(target)
}

func operationErrorMessage(summary string, err error) string {
	detail := strings.TrimSpace(err.Error())
	if detail == "" {
		return summary
	}
	detail = strings.Join(strings.Fields(detail), " ")
	lowerDetail := strings.ToLower(detail)
	switch {
	case strings.Contains(lowerDetail, "snapshot-revert") && strings.Contains(lowerDetail, "no such file or directory"):
		return snapshotRevertMissingDiskMessage(detail)
	case isGuestMemoryAllocationError(lowerDetail):
		return "目标宿主机可用内存不足，无法为迁移虚拟机分配内存，请释放目标宿主机内存或降低虚拟机内存后重试"
	case strings.Contains(lowerDetail, "cannot access storage file") && strings.Contains(lowerDetail, "no such file or directory") && isMigrationStorageAccessError(lowerDetail):
		return migrationStorageAccessMessage(detail)
	case strings.Contains(lowerDetail, "cannot access storage file") && strings.Contains(lowerDetail, "no such file or directory"):
		return "虚拟机挂载的镜像或光驱文件不存在，请检查 ISO/磁盘路径是否仍在宿主机上，或断开无效介质后重试"
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
	case strings.Contains(lowerDetail, "storage pool must be stopped before deletion"):
		return "请先停止存储池后再删除"
	case isStoragePoolUnmountBusyError(lowerDetail):
		return "存储池挂载目录正在被使用中，无法停止"
	case strings.Contains(lowerDetail, "storage volume is used by virtual machine"):
		return "该存储卷正在被虚拟机使用，请先从虚拟机移除磁盘或关闭相关虚拟机后再删除"
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
	case strings.Contains(lowerDetail, "vm is running"):
		return "虚拟机正在运行，请先关闭虚拟机后再操作"
	case strings.Contains(lowerDetail, "live maximum cpu cannot change"):
		return "虚拟机运行中只能调整当前 CPU，最大 CPU 需关机后修改"
	case strings.Contains(lowerDetail, "live cpu cannot shrink"):
		return "虚拟机运行中 CPU 只能扩容，不能缩容"
	case strings.Contains(lowerDetail, "live cpu cannot exceed maximum cpu"):
		return "当前 CPU 不能超过已预留的最大 CPU"
	case strings.Contains(lowerDetail, "live maximum memory cannot change"):
		return "虚拟机运行中只能调整当前内存，最大内存需关机后修改"
	case strings.Contains(lowerDetail, "live memory cannot shrink"):
		return "虚拟机运行中内存只能扩容，不能缩容"
	case strings.Contains(lowerDetail, "live memory cannot exceed maximum memory"):
		return "当前内存不能超过已预留的最大内存"
	case strings.Contains(lowerDetail, "attach-interface") || strings.Contains(lowerDetail, "detach-interface"):
		return "虚拟机网卡热插拔失败，请检查网络池和网卡配置"
	case strings.Contains(lowerDetail, "interface already exists"):
		return "接口名称已存在，请重新更换名称"
	case strings.Contains(lowerDetail, "interface device does not exist") || strings.Contains(lowerDetail, "bridge device does not exist"):
		return "绑定设备不存在，请重新选择设备"
	case strings.Contains(lowerDetail, "interface start mode is invalid"):
		return "启动模式不受支持，请选择 none、onboot 或 hotplug"
	case strings.Contains(lowerDetail, "unsupported interface type"):
		return "当前接口类型不受支持"
	case strings.Contains(lowerDetail, "interface must be stopped before deletion"):
		return "请先停止接口后再删除"
	case strings.Contains(lowerDetail, "bridge delay is invalid"):
		return "Delay 必须是有效数字"
	case strings.Contains(lowerDetail, "ipv4 address is required"):
		return "请填写 IPv4 地址"
	case strings.Contains(lowerDetail, "ipv6 address is required"):
		return "请填写 IPv6 地址"
	case strings.Contains(lowerDetail, "ipv4 address is invalid"):
		return "IPv4 地址格式不正确，请使用 CIDR 格式"
	case strings.Contains(lowerDetail, "ipv6 address is invalid"):
		return "IPv6 地址格式不正确，请使用 CIDR 格式"
	case strings.Contains(lowerDetail, "cdrom target is required"):
		return "请选择要连接的光驱"
	case strings.Contains(lowerDetail, "iso path is required"):
		return "请选择 ISO 文件"
	case strings.Contains(lowerDetail, "target cdrom not found"):
		return "未找到指定光驱"
	case strings.Contains(lowerDetail, "network pool must be stopped before deletion"):
		return "请先停止网络池后再删除"
	case strings.Contains(lowerDetail, "subnet is too small for dhcp"):
		return "子网可用地址不足，无法启用 DHCP"
	case strings.Contains(lowerDetail, "subnet is too small for fixed address"):
		return "子网可用地址不足，无法生成固定地址"
	case strings.Contains(lowerDetail, "fixed address range is too large"):
		return "固定地址范围过大，请缩小子网或关闭固定地址"
	case strings.Contains(lowerDetail, "storage volume") && (strings.Contains(lowerDetail, "already exists") || strings.Contains(lowerDetail, "exists already")):
		return "镜像名称已存在，请更换名称"
	case strings.Contains(lowerDetail, "target volume already exists"):
		return "镜像名称已存在，请更换名称"
	case strings.Contains(lowerDetail, "already exists"):
		return "名称已存在，请重新更换名称"
	case strings.Contains(lowerDetail, "name is required"):
		return "请填写名称"
	case strings.Contains(lowerDetail, "must be an absolute path"):
		return "请填写绝对路径"
	case strings.Contains(lowerDetail, "must be a directory"):
		return "路径必须是目录"
	case strings.Contains(lowerDetail, "must be a block device"):
		return "LVM 设备路径必须是块设备"
	case strings.Contains(lowerDetail, "storage source conflict with pool"):
		return storageSourceConflictMessage(detail)
	case strings.Contains(lowerDetail, "path is required") || strings.Contains(lowerDetail, "target"):
		return "请填写正确的路径"
	case strings.Contains(lowerDetail, "live disk snapshot not supported"):
		return "当前宿主机 QEMU/libvirt 不支持运行中内部磁盘快照。请先关闭虚拟机后创建快照，或升级宿主机 QEMU/libvirt 后重试"
	case strings.Contains(lowerDetail, "non-migratable device") && isMigrationOperation(summary, lowerDetail):
		return migrationNonMigratableDeviceMessage(detail)
	case strings.Contains(lowerDetail, "non-migratable device"):
		return "当前虚拟机包含不支持保存运行状态的设备。请取消“包括虚拟机内存”后重试，或调整虚拟硬件后再创建内存快照"
	case strings.Contains(lowerDetail, "guest agent"):
		return "静默客户机文件系统需要虚拟机内 QEMU Guest Agent 可用，请确认已安装并运行后重试"
	}
	if len(detail) > 500 {
		detail = detail[:500]
	}
	return detail
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

func isMigrationOperation(summary string, lowerDetail string) bool {
	return strings.Contains(summary, "迁移") ||
		strings.Contains(lowerDetail, " migrate ") ||
		strings.Contains(lowerDetail, "qemu+ssh://")
}

func migrationNonMigratableDeviceMessage(detail string) string {
	device := quotedErrorValueAfter(detail, "non-migratable device")
	if device == "" {
		return "当前虚拟机包含热迁移不支持的设备，请移除或更换该设备后重试，或关机后执行冷迁移"
	}
	if slash := strings.LastIndex(device, "/"); slash >= 0 && slash+1 < len(device) {
		device = device[slash+1:]
	}
	return "当前虚拟机包含热迁移不支持的设备 " + device + "，请移除或更换该设备后重试，或关机后执行冷迁移"
}

func isStoragePoolUnmountBusyError(lowerDetail string) bool {
	return strings.Contains(lowerDetail, "pool-destroy") &&
		strings.Contains(lowerDetail, "umount") &&
		strings.Contains(lowerDetail, "device is busy")
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

func storageSourceConflictMessage(detail string) string {
	pool := quotedErrorValueAfter(detail, "Storage source conflict with pool:")
	if pool == "" {
		return "存储池路径已被其他存储池使用，请更换路径或先处理冲突的存储池"
	}
	return "存储池路径已被存储池 " + pool + " 使用，请更换路径或先处理该存储池"
}

func quotedErrorValueAfter(detail string, marker string) string {
	lower := strings.ToLower(detail)
	start := strings.Index(lower, strings.ToLower(marker))
	if start < 0 {
		return ""
	}
	rest := strings.TrimSpace(detail[start+len(marker):])
	if rest == "" {
		return ""
	}
	quoteStart := strings.Index(rest, "'")
	if quoteStart < 0 {
		return ""
	}
	rest = rest[quoteStart+1:]
	quoteEnd := strings.Index(rest, "'")
	if quoteEnd < 0 {
		return ""
	}
	return strings.TrimSpace(rest[:quoteEnd])
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

func parseVMConsolePath(path string) (name string, ok bool) {
	trimmed := strings.Trim(strings.TrimPrefix(path, "/v1/vms/"), "/")
	parts := strings.Split(trimmed, "/")
	if len(parts) != 3 || parts[1] != "console" || parts[2] != "ws" {
		return "", false
	}
	return parts[0], parts[0] != ""
}

func parseVMPath(path string) (name string, action string, ok bool) {
	trimmed := strings.Trim(strings.TrimPrefix(path, "/v1/vms/"), "/")
	if trimmed == "" {
		return "", "", false
	}
	parts := strings.Split(trimmed, "/")
	if len(parts) != 2 {
		return "", "", false
	}
	return parts[0], parts[1], true
}

func parseStoragePoolPath(path string) (name string, action string, ok bool) {
	trimmed := strings.Trim(strings.TrimPrefix(path, "/v1/storage-pools/"), "/")
	parts := strings.Split(trimmed, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		if len(parts) == 3 && parts[0] != "" && parts[1] == "volumes" && parts[2] == "upload" {
			return parts[0], "volumes/upload", true
		}
		if len(parts) == 3 && parts[0] != "" && parts[1] == "volumes" && parts[2] == "clone" {
			return parts[0], "volumes/clone", true
		}
		return "", "", false
	}
	return parts[0], parts[1], true
}

func parseNetworkPoolPath(path string) (name string, action string, ok bool) {
	trimmed := strings.Trim(strings.TrimPrefix(path, "/v1/network-pools/"), "/")
	parts := strings.Split(trimmed, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}

func parseSnapshotActionPath(path string) (name string, snapshot string, action string, ok bool) {
	trimmed := strings.Trim(strings.TrimPrefix(path, "/v1/vms/"), "/")
	parts := strings.Split(trimmed, "/")
	if len(parts) != 4 || parts[1] != "snapshots" || parts[0] == "" || parts[2] == "" || parts[3] == "" {
		return "", "", "", false
	}
	return parts[0], parts[2], parts[3], true
}
