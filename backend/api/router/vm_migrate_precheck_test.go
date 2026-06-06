package router

import (
	"strings"
	"testing"

	"kvm-manager/backend/pkg/agent"
)

func TestValidateMigrationTargetCapabilitiesAllowsMissingVMArch(t *testing.T) {
	config := agent.VMConfig{Status: "stopped"}
	target := agent.HostInfo{Capabilities: []string{"vm.list"}, CPUModel: "x86_64"}
	if err := validateMigrationTargetCapabilities(config, target); err != nil {
		t.Fatalf("expected capability check to pass without arch, got %v", err)
	}
}

func TestMigrationCPUArchitectureCompatibleRejectsMismatch(t *testing.T) {
	if migrationCPUArchitectureCompatible("x86_64", "aarch64") {
		t.Fatal("expected x86 vm and arm target to be incompatible")
	}
	if !migrationCPUArchitectureCompatible("x86_64", "Intel Core Processor") {
		t.Fatal("expected unknown target model family to stay permissive")
	}
}

func TestValidateMigrationTargetResourcesRejectsInsufficientHost(t *testing.T) {
	config := agent.VMConfig{MaximumCPU: 8, MaximumMemoryBytes: 16 * 1024 * 1024 * 1024}
	target := agent.HostInfo{CPUCores: 4, MemoryBytes: 32 * 1024 * 1024 * 1024}
	if err := validateMigrationTargetResources(config, target); err == nil || !strings.Contains(err.Error(), "CPU 不足") {
		t.Fatalf("expected cpu capacity error, got %v", err)
	}
}

func TestValidateMigrationTargetVMNameRejectsDuplicate(t *testing.T) {
	targetVMs := []agent.VirtualMachine{{Name: "Prod-Web-01"}}
	if err := validateMigrationTargetVMName("prod-web-01", targetVMs); err == nil || !strings.Contains(err.Error(), "同名虚拟机") {
		t.Fatalf("expected duplicate vm name error, got %v", err)
	}
}

func TestValidateMigrationTargetVMNameAllowsUnique(t *testing.T) {
	targetVMs := []agent.VirtualMachine{{Name: "db-01"}}
	if err := validateMigrationTargetVMName("web-01", targetVMs); err != nil {
		t.Fatalf("expected unique vm name to pass, got %v", err)
	}
}

func TestValidateMigrationNetworkPoolsMatchesBridgeDevice(t *testing.T) {
	interfaces := []agent.VMConfigInterface{{Source: "br0"}}
	pools := []agent.NetworkPool{{Name: "bridge-net", Bridge: "br0", State: "active"}}
	if err := validateMigrationNetworkPools(interfaces, pools); err != nil {
		t.Fatalf("expected bridge device match, got %v", err)
	}
}

func TestValidateMigrationNetworkPoolsAllowsVirshActiveYes(t *testing.T) {
	interfaces := []agent.VMConfigInterface{{Source: "br0"}}
	pools := []agent.NetworkPool{{Name: "bridge-net", Bridge: "br0", State: "yes"}}
	if err := validateMigrationNetworkPools(interfaces, pools); err != nil {
		t.Fatalf("expected virsh active yes to pass, got %v", err)
	}
}

func TestValidateMigrationNetworkPoolsRejectsMissingSource(t *testing.T) {
	interfaces := []agent.VMConfigInterface{{Source: "default"}}
	if err := validateMigrationNetworkPools(interfaces, nil); err == nil || !strings.Contains(err.Error(), "缺少网络池") {
		t.Fatalf("expected missing network error, got %v", err)
	}
}

func TestValidateMigrationNetworkPoolsRejectsInactiveState(t *testing.T) {
	interfaces := []agent.VMConfigInterface{{Source: "br0"}}
	pools := []agent.NetworkPool{{Name: "bridge-net", Bridge: "br0", State: "no"}}
	if err := validateMigrationNetworkPools(interfaces, pools); err == nil || !strings.Contains(err.Error(), "不是 active 状态") {
		t.Fatalf("expected inactive network error, got %v", err)
	}
}

func TestValidateMigrationSharedStorageAllowsSamePathDirPool(t *testing.T) {
	disks := []agent.VMConfigDisk{{Name: "vda", Device: "disk", Pool: "default"}}
	sourcePools := []agent.StoragePool{{Name: "default", Type: "dir", State: "running", Path: "/mnt/shared/libvirt"}}
	targetPools := []agent.StoragePool{{Name: "default", Type: "dir", State: "running", Path: "/mnt/shared/libvirt"}}
	if err := validateMigrationSharedStorage(disks, sourcePools, targetPools); err != nil {
		t.Fatalf("expected same path dir pool to pass, got %v", err)
	}
}

func TestValidateMigrationSharedStorageRejectsLocalPoolMismatch(t *testing.T) {
	disks := []agent.VMConfigDisk{{Name: "vda", Device: "disk", Pool: "default"}}
	sourcePools := []agent.StoragePool{{Name: "default", Type: "dir", State: "running", Path: "/var/lib/libvirt/images"}}
	targetPools := []agent.StoragePool{{Name: "default", Type: "dir", State: "running", Path: "/data/libvirt/images"}}
	if err := validateMigrationSharedStorage(disks, sourcePools, targetPools); err == nil || !strings.Contains(err.Error(), "未显示共享存储特征") {
		t.Fatalf("expected storage mismatch error, got %v", err)
	}
}

func TestValidateMigrationCopyTargetPoolsRejectsDiskPathOutsideTargetPools(t *testing.T) {
	disks := []agent.VMConfigDisk{{Name: "vda", Device: "disk", Pool: "Storage", Path: "/kvm/images/vm1.qcow2"}}
	targetPools := []agent.StoragePool{{Name: "default", Type: "dir", State: "running", Path: "/var/lib/libvirt/images"}}
	if err := validateMigrationCopyTargetPools(disks, targetPools); err == nil || !strings.Contains(err.Error(), "没有源磁盘路径 /kvm/images/vm1.qcow2 所在的存储池") {
		t.Fatalf("expected missing target pool path error, got %v", err)
	}
}

func TestValidateMigrationCopyTargetPoolsAllowsDifferentPoolNameContainingPath(t *testing.T) {
	disks := []agent.VMConfigDisk{{Name: "vda", Device: "disk", Pool: "Storage", Path: "/kvm/images/vm1.qcow2"}}
	targetPools := []agent.StoragePool{{Name: "default", Type: "dir", State: "running", Path: "/kvm/images"}}
	if err := validateMigrationCopyTargetPools(disks, targetPools); err != nil {
		t.Fatalf("expected target pool path containing disk path to pass, got %v", err)
	}
}

func TestMigrationTargetDiskExistsMatchesPath(t *testing.T) {
	volumes := []agent.StorageVolume{{Name: "other.qcow2", Path: "/kvm/images/vm1.qcow2"}}
	if !migrationTargetDiskExists("/kvm/images/vm1.qcow2", "vm1.qcow2", volumes) {
		t.Fatal("expected existing target disk path to be detected")
	}
}

func TestMigrationTargetDiskExistsMatchesVolumeName(t *testing.T) {
	volumes := []agent.StorageVolume{{Name: "vm1.qcow2", Path: "/kvm/images/renamed.qcow2"}}
	if !migrationTargetDiskExists("/kvm/images/vm1.qcow2", "vm1.qcow2", volumes) {
		t.Fatal("expected existing target disk volume name to be detected")
	}
}

func TestMigrationCopyTargetDisksUsesMostSpecificPool(t *testing.T) {
	disks := []agent.VMConfigDisk{{Name: "vda", Device: "disk", Pool: "Storage", Path: "/kvm/images/project/vm1.qcow2"}}
	targetPools := []agent.StoragePool{
		{Name: "root-images", Type: "dir", State: "running", Path: "/kvm/images"},
		{Name: "project-images", Type: "dir", State: "running", Path: "/kvm/images/project"},
	}
	targets, err := migrationCopyTargetDisks(disks, targetPools)
	if err != nil {
		t.Fatalf("expected target disk to pass, got %v", err)
	}
	if len(targets) != 1 || targets[0].Pool.Name != "project-images" {
		t.Fatalf("expected most specific pool, got %+v", targets)
	}
}
