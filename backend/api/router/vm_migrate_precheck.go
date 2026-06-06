package router

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"

	"kvm-manager/backend/internal/domain"
	"kvm-manager/backend/pkg/agent"
)

type migrationPrecheckReport struct {
	Passed         bool                    `json:"passed"`
	Items          []migrationPrecheckItem `json:"items"`
	FailureStatus  int                     `json:"-"`
	FailureCode    string                  `json:"-"`
	FailureMessage string                  `json:"-"`
}

type migrationPrecheckItem struct {
	Key     string `json:"key"`
	Label   string `json:"label"`
	Status  string `json:"status"`
	Message string `json:"message"`
	Code    string `json:"code,omitempty"`
}

func precheckVMMigration(ctx context.Context, sourceCfg agent.Config, sourceTLSInsecure bool, targetCfg agent.Config, targetTLSInsecure bool, vm domain.VirtualMachine, request agent.VMMigrateRequest) (int, string, string) {
	report := runVMMigrationPrecheck(ctx, sourceCfg, sourceTLSInsecure, targetCfg, targetTLSInsecure, vm, request)
	if report.FailureMessage != "" {
		return report.FailureStatus, report.FailureCode, report.FailureMessage
	}
	return 0, "", ""
}

func runVMMigrationPrecheck(ctx context.Context, sourceCfg agent.Config, sourceTLSInsecure bool, targetCfg agent.Config, targetTLSInsecure bool, vm domain.VirtualMachine, request agent.VMMigrateRequest) migrationPrecheckReport {
	report := migrationPrecheckReport{Passed: true}
	sourceClient := agent.NewClient(sourceTLSInsecure)
	sourceConfig, err := sourceClient.VMConfig(ctx, sourceCfg, vm.Name)
	if err != nil {
		return report.withFailure("source-status", "源虚拟机状态", "读取源虚拟机配置失败："+agent.UserFacingErrorMessage(err), http.StatusServiceUnavailable, "vm_migrate_precheck_failed")
	}
	if request.Live && !strings.EqualFold(sourceConfig.Status, "running") {
		return report.withFailure("source-status", "源虚拟机状态", "非运行状态的虚拟机请选择冷迁移", http.StatusBadRequest, "invalid_vm_migrate")
	}
	if !request.Live && strings.EqualFold(sourceConfig.Status, "running") {
		return report.withFailure("source-status", "源虚拟机状态", "运行中的虚拟机请选择热迁移，或先关闭虚拟机后再冷迁移", http.StatusBadRequest, "invalid_vm_migrate")
	}
	report.addPassed("source-status", "源虚拟机状态", "源虚拟机状态与迁移方式匹配")

	targetClient := agent.NewClient(targetTLSInsecure)
	targetInfo, err := targetClient.HostInfo(ctx, targetCfg)
	if err != nil {
		return report.withFailure("target-host", "目标宿主机资源", "检查目标宿主机资源失败："+agent.UserFacingErrorMessage(err), http.StatusServiceUnavailable, "vm_migrate_target_precheck_failed")
	}
	if err := validateMigrationTargetResources(sourceConfig, targetInfo); err != nil {
		return report.withFailure("target-resource", "目标 CPU / 内存", err.Error(), http.StatusBadRequest, "vm_migrate_resource_mismatch")
	}
	report.addPassed("target-resource", "目标 CPU / 内存", "目标宿主机 CPU 和内存满足虚拟机最大配置")
	if err := validateMigrationTargetCapabilities(sourceConfig, targetInfo); err != nil {
		return report.withFailure("target-capability", "目标 CPU 架构", err.Error(), http.StatusBadRequest, "vm_migrate_cpu_incompatible")
	}
	report.addPassed("target-capability", "目标 CPU 架构", "目标宿主机能力与虚拟机基础架构匹配")
	targetVMs, err := targetClient.ListVMsFast(ctx, targetCfg)
	if err != nil {
		return report.withFailure("target-name", "目标同名虚拟机", "检查目标宿主机虚拟机名称失败："+agent.UserFacingErrorMessage(err), http.StatusServiceUnavailable, "vm_migrate_name_precheck_failed")
	}
	if err := validateMigrationTargetVMName(vm.Name, targetVMs); err != nil {
		return report.withFailure("target-name", "目标同名虚拟机", err.Error(), http.StatusBadRequest, "vm_migrate_name_exists")
	}
	report.addPassed("target-name", "目标同名虚拟机", "目标宿主机未发现同名虚拟机")

	targetNetworks, err := targetClient.ListNetworkPools(ctx, targetCfg)
	if err != nil {
		return report.withFailure("target-network", "目标网络池", "检查目标宿主机网络池失败："+agent.UserFacingErrorMessage(err), http.StatusServiceUnavailable, "vm_migrate_network_precheck_failed")
	}
	if err := validateMigrationNetworkPools(sourceConfig.Interfaces, targetNetworks); err != nil {
		return report.withFailure("target-network", "目标网络池", err.Error(), http.StatusBadRequest, "vm_migrate_network_mismatch")
	}
	report.addPassed("target-network", "目标网络池", "目标宿主机网络池或桥接设备匹配")

	if !request.CopyDisks {
		sourcePools, err := sourceClient.ListStoragePools(ctx, sourceCfg)
		if err != nil {
			return report.withFailure("shared-storage", "共享存储", "检查源宿主机存储池失败："+agent.UserFacingErrorMessage(err), http.StatusServiceUnavailable, "vm_migrate_storage_precheck_failed")
		}
		targetPools, err := targetClient.ListStoragePools(ctx, targetCfg)
		if err != nil {
			return report.withFailure("shared-storage", "共享存储", "检查目标宿主机存储池失败："+agent.UserFacingErrorMessage(err), http.StatusServiceUnavailable, "vm_migrate_storage_precheck_failed")
		}
		if err := validateMigrationSharedStorage(sourceConfig.Disks, sourcePools, targetPools); err != nil {
			return report.withFailure("shared-storage", "共享存储", err.Error(), http.StatusBadRequest, "vm_migrate_storage_mismatch")
		}
		report.addPassed("shared-storage", "共享存储", "源宿主机和目标宿主机存储池显示共享特征")
	} else {
		report.addSkipped("shared-storage", "共享存储", "已选择复制本地磁盘，跳过共享存储检查")
		targetPools, err := targetClient.ListStoragePools(ctx, targetCfg)
		if err != nil {
			return report.withFailure("target-storage", "目标存储池", "检查目标宿主机存储池失败："+agent.UserFacingErrorMessage(err), http.StatusServiceUnavailable, "vm_migrate_storage_precheck_failed")
		}
		if err := validateMigrationCopyTargetPools(sourceConfig.Disks, targetPools); err != nil {
			return report.withFailure("target-storage", "目标存储池", err.Error(), http.StatusBadRequest, "vm_migrate_storage_mismatch")
		}
		if status, code, message := precheckMigrationCopyTargetVolumes(ctx, targetClient, targetCfg, sourceConfig.Disks, targetPools); message != "" {
			return report.withFailure("target-disk", "目标磁盘文件", message, status, code)
		}
		report.addPassed("target-storage", "目标存储池", "目标宿主机存在源磁盘路径所在的存储池，可用于迁移复制磁盘")
	}
	if !isQemuSSHMigrationURI(request.DestinationURI) {
		if request.CopyDisks {
			return report.withFailure("migration-channel", "迁移通道", "复制本地磁盘需要 qemu+ssh:// 迁移 URI", http.StatusBadRequest, "invalid_vm_migrate")
		}
		report.addSkipped("migration-channel", "迁移通道", "当前迁移 URI 不是 qemu+ssh，跳过 SSH 免密检测")
		return report
	}
	result, err := sourceClient.CheckMigrationConnection(ctx, sourceCfg, agent.MigrationConnectionCheckRequest{DestinationURI: request.DestinationURI, Live: request.Live})
	if err != nil {
		return report.withFailure("migration-channel", "迁移通道", "检查迁移通道失败："+agent.UserFacingErrorMessage(err), http.StatusServiceUnavailable, "vm_migrate_connection_precheck_failed")
	}
	if !result.OK {
		code := "vm_migrate_connection_unavailable"
		if result.PasswordRequired || strings.Contains(result.Message, "SSH 指纹") {
			code = "vm_migrate_ssh_password_required"
		} else if strings.Contains(result.Message, "主机名解析为 localhost") {
			code = "vm_migrate_target_hostname_localhost"
		}
		return report.withFailure("migration-channel", "迁移通道", firstNonEmptyString(result.Message, "迁移通道不可用"), http.StatusBadRequest, code)
	}
	report.addPassed("migration-channel", "迁移通道", "源宿主机可非交互连接目标 libvirt")
	return report
}

func (r *migrationPrecheckReport) addPassed(key string, label string, message string) {
	r.Items = append(r.Items, migrationPrecheckItem{Key: key, Label: label, Status: "passed", Message: message})
}

func (r *migrationPrecheckReport) addSkipped(key string, label string, message string) {
	r.Items = append(r.Items, migrationPrecheckItem{Key: key, Label: label, Status: "skipped", Message: message})
}

func (r migrationPrecheckReport) withFailure(key string, label string, message string, status int, code string) migrationPrecheckReport {
	r.Passed = false
	r.FailureStatus = status
	r.FailureCode = code
	r.FailureMessage = message
	r.Items = append(r.Items, migrationPrecheckItem{Key: key, Label: label, Status: "failed", Message: message, Code: code})
	return r
}

func isQemuSSHMigrationURI(destinationURI string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(destinationURI)), "qemu+ssh://")
}

func validateMigrationTargetResources(config agent.VMConfig, target agent.HostInfo) error {
	requiredCPU := config.MaximumCPU
	if requiredCPU <= 0 {
		requiredCPU = config.CurrentCPU
	}
	if target.CPUCores > 0 && requiredCPU > target.CPUCores {
		return newMigratePrecheckError("目标宿主机 CPU 不足，虚拟机最大 CPU 为 %d，目标宿主机逻辑 CPU 为 %d", requiredCPU, target.CPUCores)
	}
	requiredMemory := config.MaximumMemoryBytes
	if requiredMemory <= 0 {
		requiredMemory = config.CurrentMemoryBytes
	}
	if target.MemoryBytes > 0 && requiredMemory > target.MemoryBytes {
		return newMigratePrecheckError("目标宿主机内存不足，虚拟机最大内存为 %s，目标宿主机总内存为 %s", formatMigrationBytes(requiredMemory), formatMigrationBytes(target.MemoryBytes))
	}
	return nil
}

func validateMigrationTargetCapabilities(config agent.VMConfig, target agent.HostInfo) error {
	if len(target.Capabilities) == 0 {
		return newMigratePrecheckError("目标宿主机能力信息不可用，无法确认是否支持 KVM 迁移")
	}
	if !hasCapability(target.Capabilities, "vm.list") {
		return newMigratePrecheckError("目标宿主机未返回虚拟机管理能力，无法确认是否支持 KVM 迁移")
	}
	if !migrationCPUArchitectureCompatible(config.Arch, target.CPUModel) {
		return newMigratePrecheckError("目标宿主机 CPU 架构与虚拟机不兼容，虚拟机架构为 %s，目标 CPU 为 %s", strings.TrimSpace(config.Arch), strings.TrimSpace(target.CPUModel))
	}
	return nil
}

func validateMigrationTargetVMName(vmName string, targetVMs []agent.VirtualMachine) error {
	name := strings.TrimSpace(vmName)
	if name == "" {
		return newMigratePrecheckError("虚拟机名称为空，无法迁移")
	}
	for _, item := range targetVMs {
		if strings.EqualFold(strings.TrimSpace(item.Name), name) {
			return newMigratePrecheckError("目标宿主机已存在同名虚拟机 %s，请先重命名或删除目标虚拟机", name)
		}
	}
	return nil
}

func validateMigrationNetworkPools(interfaces []agent.VMConfigInterface, targetPools []agent.NetworkPool) error {
	targetByName := make(map[string]agent.NetworkPool, len(targetPools))
	targetByBridge := make(map[string]agent.NetworkPool, len(targetPools))
	for _, pool := range targetPools {
		targetByName[strings.ToLower(strings.TrimSpace(pool.Name))] = pool
		if strings.TrimSpace(pool.Bridge) != "" {
			targetByBridge[strings.ToLower(strings.TrimSpace(pool.Bridge))] = pool
		}
	}
	for _, iface := range interfaces {
		source := strings.TrimSpace(iface.Source)
		if source == "" {
			continue
		}
		pool, ok := targetByName[strings.ToLower(source)]
		if !ok {
			pool, ok = targetByBridge[strings.ToLower(source)]
		}
		if !ok {
			return newMigratePrecheckError("目标宿主机缺少网络池或桥接设备 %s", source)
		}
		if !migrationNetworkPoolIsActive(pool.State) {
			return newMigratePrecheckError("目标宿主机网络池 %s 当前不是 active 状态", source)
		}
	}
	return nil
}

func migrationNetworkPoolIsActive(state string) bool {
	switch strings.ToLower(strings.TrimSpace(state)) {
	case "", "yes", "active", "running", "up":
		return true
	default:
		return false
	}
}

func validateMigrationSharedStorage(disks []agent.VMConfigDisk, sourcePools []agent.StoragePool, targetPools []agent.StoragePool) error {
	sourceByName := make(map[string]agent.StoragePool, len(sourcePools))
	for _, pool := range sourcePools {
		sourceByName[strings.ToLower(strings.TrimSpace(pool.Name))] = pool
	}
	targetByName := make(map[string]agent.StoragePool, len(targetPools))
	for _, pool := range targetPools {
		targetByName[strings.ToLower(strings.TrimSpace(pool.Name))] = pool
	}
	for _, disk := range disks {
		if !migrationDiskRequiresStorageCheck(disk) {
			continue
		}
		poolName := strings.TrimSpace(disk.Pool)
		if poolName == "" {
			return newMigratePrecheckError("虚拟机磁盘 %s 未识别到存储池，无法确认共享存储；如需迁移本地磁盘请勾选复制本地磁盘", firstNonEmptyString(disk.Name, disk.Path))
		}
		targetPool, ok := targetByName[strings.ToLower(poolName)]
		if !ok {
			return newMigratePrecheckError("目标宿主机缺少存储池 %s；如需迁移本地磁盘请勾选复制本地磁盘", poolName)
		}
		if strings.TrimSpace(targetPool.State) != "" && !strings.EqualFold(targetPool.State, "running") {
			return newMigratePrecheckError("目标宿主机存储池 %s 当前不是 running 状态", poolName)
		}
		sourcePool, ok := sourceByName[strings.ToLower(poolName)]
		if !ok {
			return newMigratePrecheckError("源宿主机存储池 %s 信息不可用，无法确认共享存储；如需迁移本地磁盘请勾选复制本地磁盘", poolName)
		}
		if !migrationStoragePoolLooksShared(sourcePool, targetPool) {
			return newMigratePrecheckError("源宿主机与目标宿主机的存储池 %s 未显示共享存储特征；如需迁移本地磁盘请勾选复制本地磁盘", poolName)
		}
	}
	return nil
}

func validateMigrationCopyTargetPools(disks []agent.VMConfigDisk, targetPools []agent.StoragePool) error {
	_, err := migrationCopyTargetDisks(disks, targetPools)
	return err
}

func precheckMigrationCopyTargetVolumes(ctx context.Context, targetClient *agent.Client, targetCfg agent.Config, disks []agent.VMConfigDisk, targetPools []agent.StoragePool) (int, string, string) {
	targetDisks, err := migrationCopyTargetDisks(disks, targetPools)
	if err != nil {
		return http.StatusBadRequest, "vm_migrate_storage_mismatch", err.Error()
	}
	volumesByPool := map[string][]agent.StorageVolume{}
	for _, target := range targetDisks {
		poolName := strings.TrimSpace(target.Pool.Name)
		if poolName == "" {
			return http.StatusBadRequest, "vm_migrate_storage_mismatch", "目标宿主机存储池名称不可用，无法检查目标磁盘是否已存在"
		}
		key := strings.ToLower(poolName)
		if _, ok := volumesByPool[key]; !ok {
			volumes, err := targetClient.ListStorageVolumes(ctx, targetCfg, poolName)
			if err != nil {
				return http.StatusServiceUnavailable, "vm_migrate_target_disk_precheck_failed", "检查目标宿主机磁盘文件失败：" + agent.UserFacingErrorMessage(err)
			}
			volumesByPool[key] = volumes
		}
		if migrationTargetDiskExists(target.Path, target.VolumeName, volumesByPool[key]) {
			return http.StatusBadRequest, "vm_migrate_target_disk_exists", fmt.Sprintf("目标宿主机已存在磁盘文件 %s，请先删除目标磁盘或更换源虚拟机磁盘名称后再迁移", target.Path)
		}
	}
	return 0, "", ""
}

type migrationCopyTargetDisk struct {
	Path       string
	VolumeName string
	Pool       agent.StoragePool
}

func migrationCopyTargetDisks(disks []agent.VMConfigDisk, targetPools []agent.StoragePool) ([]migrationCopyTargetDisk, error) {
	targets := make([]migrationCopyTargetDisk, 0, len(disks))
	for _, disk := range disks {
		if !migrationDiskRequiresStorageCheck(disk) {
			continue
		}
		diskPath := cleanMigrationStoragePath(firstNonEmptyString(disk.Path, disk.SourcePath))
		if diskPath == "" {
			return nil, newMigratePrecheckError("磁盘 %s 路径不可用，无法执行迁移复制磁盘", firstNonEmptyString(disk.Name, "-"))
		}
		targetPool := migrationStoragePoolForDiskPath(diskPath, targetPools)
		if strings.TrimSpace(targetPool.Path) == "" {
			return nil, newMigratePrecheckError("目标宿主机没有源磁盘路径 %s 所在的存储池，无法执行迁移复制磁盘", diskPath)
		}
		targets = append(targets, migrationCopyTargetDisk{Path: diskPath, VolumeName: filepath.Base(diskPath), Pool: targetPool})
	}
	return targets, nil
}

func migrationDiskPathInStoragePool(diskPath string, pools []agent.StoragePool) bool {
	return strings.TrimSpace(migrationStoragePoolForDiskPath(diskPath, pools).Path) != ""
}

func migrationStoragePoolForDiskPath(diskPath string, pools []agent.StoragePool) agent.StoragePool {
	diskPath = cleanMigrationStoragePath(diskPath)
	if diskPath == "" {
		return agent.StoragePool{}
	}
	best := agent.StoragePool{}
	bestLength := 0
	for _, pool := range pools {
		poolPath := cleanMigrationStoragePath(pool.Path)
		if poolPath != "" && migrationPathHasPoolPrefix(diskPath, poolPath) {
			if len(poolPath) > bestLength {
				best = pool
				bestLength = len(poolPath)
			}
		}
	}
	return best
}

func migrationTargetDiskExists(diskPath string, volumeName string, volumes []agent.StorageVolume) bool {
	diskPath = cleanMigrationStoragePath(diskPath)
	volumeName = strings.ToLower(strings.TrimSpace(volumeName))
	for _, volume := range volumes {
		if diskPath != "" && cleanMigrationStoragePath(volume.Path) == diskPath {
			return true
		}
		if volumeName != "" && strings.ToLower(strings.TrimSpace(volume.Name)) == volumeName {
			return true
		}
	}
	return false
}

func cleanMigrationStoragePath(path string) string {
	if strings.TrimSpace(path) == "" {
		return ""
	}
	return filepath.ToSlash(filepath.Clean(strings.TrimSpace(path)))
}

func migrationPathHasPoolPrefix(path string, poolPath string) bool {
	if path == poolPath {
		return true
	}
	if !strings.HasPrefix(path, poolPath) {
		return false
	}
	remainder := strings.TrimPrefix(path, poolPath)
	return strings.HasPrefix(remainder, "/") || strings.HasPrefix(remainder, "\\")
}

func migrationDiskRequiresStorageCheck(disk agent.VMConfigDisk) bool {
	device := strings.TrimSpace(disk.Device)
	return device == "" || strings.EqualFold(device, "disk")
}

func migrationStoragePoolLooksShared(source agent.StoragePool, target agent.StoragePool) bool {
	sourceType := strings.ToLower(strings.TrimSpace(source.Type))
	targetType := strings.ToLower(strings.TrimSpace(target.Type))
	if sourceType != "" && targetType != "" && sourceType != targetType {
		return false
	}
	switch firstNonEmptyString(sourceType, targetType) {
	case "netfs", "iscsi", "iscsi-direct", "rbd", "gluster", "sheepdog", "mpath", "zfs", "logical":
		return true
	}
	if strings.TrimSpace(source.Path) != "" && strings.TrimSpace(target.Path) != "" && strings.TrimSpace(source.Path) == strings.TrimSpace(target.Path) {
		return true
	}
	return false
}

func hasCapability(items []string, target string) bool {
	for _, item := range items {
		if strings.EqualFold(strings.TrimSpace(item), target) {
			return true
		}
	}
	return false
}

func migrationCPUArchitectureCompatible(vmArch string, targetCPUModel string) bool {
	vmFamily := cpuArchitectureFamily(vmArch)
	targetFamily := cpuArchitectureFamily(targetCPUModel)
	return vmFamily == "" || targetFamily == "" || vmFamily == targetFamily
}

func cpuArchitectureFamily(value string) string {
	lower := strings.ToLower(strings.TrimSpace(value))
	switch {
	case lower == "":
		return ""
	case strings.Contains(lower, "x86_64") || strings.Contains(lower, "amd64") || strings.Contains(lower, "i686") || strings.Contains(lower, "i386"):
		return "x86"
	case strings.Contains(lower, "aarch64") || strings.Contains(lower, "arm64"):
		return "arm64"
	case strings.Contains(lower, "ppc64"):
		return "ppc64"
	case strings.Contains(lower, "s390x"):
		return "s390x"
	}
	return ""
}

type migratePrecheckError string

func (e migratePrecheckError) Error() string {
	return string(e)
}

func newMigratePrecheckError(format string, args ...any) error {
	return migratePrecheckError(fmt.Sprintf(format, args...))
}

func formatMigrationBytes(value int64) string {
	if value >= 1024*1024*1024 {
		return fmt.Sprintf("%.1f GB", float64(value)/float64(1024*1024*1024))
	}
	if value >= 1024*1024 {
		return fmt.Sprintf("%.1f MB", float64(value)/float64(1024*1024))
	}
	return fmt.Sprintf("%d B", value)
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return "-"
}
