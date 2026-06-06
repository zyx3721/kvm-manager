package realtime

import (
	"testing"

	"kvm-manager/backend/internal/domain"
)

func TestUsablePrimaryIPRejectsLoopbackAndLinkLocal(t *testing.T) {
	if usablePrimaryIP("127.0.0.1") {
		t.Fatal("expected loopback ip to be unusable")
	}

	if usablePrimaryIP("169.254.10.20") {
		t.Fatal("expected link-local ip to be unusable")
	}

	if !usablePrimaryIP("192.168.1.179") {
		t.Fatal("expected normal private ip to be usable")
	}

	if usablePrimaryIP("  ") {
		t.Fatal("expected blank ip to be unusable")
	}

	if !usablePrimaryIP(" 10.0.0.5 ") {
		t.Fatal("expected trimmed private ip to be usable")
	}
}

func TestMergeFastRuntimePreservesPreviousOSTypeWhenFastLeavesItEmpty(t *testing.T) {
	current := testVirtualMachineForMerge("stopped", "")
	current.OSType = ""
	previous := testVirtualMachineForMerge("stopped", "")
	previous.OSType = "CentOS Linux 7 (Core)"

	merged := mergeFastRuntimeDetails(current, previous)
	if merged.OSType != previous.OSType {
		t.Fatalf("expected previous os type %q, got %q", previous.OSType, merged.OSType)
	}
}

func TestMergeFastRuntimePreservesMoreSpecificPreviousOSType(t *testing.T) {
	current := testVirtualMachineForMerge("stopped", "")
	current.OSType = "CentOS"
	previous := testVirtualMachineForMerge("running", "192.168.1.179")
	previous.OSType = "CentOS Linux 7 (Core)"

	merged := mergeFastRuntimeDetails(current, previous)
	if merged.OSType != previous.OSType {
		t.Fatalf("expected previous os type %q, got %q", previous.OSType, merged.OSType)
	}
}

func TestMergeFastRuntimeUsesMoreSpecificCurrentOSType(t *testing.T) {
	current := testVirtualMachineForMerge("running", "192.168.1.180")
	current.OSType = "CentOS Linux 7 (Core)"
	previous := testVirtualMachineForMerge("stopped", "")
	previous.OSType = "CentOS"

	merged := mergeFastRuntimeDetails(current, previous)
	if merged.OSType != current.OSType {
		t.Fatalf("expected current os type %q, got %q", current.OSType, merged.OSType)
	}
}

func TestMergeFastRuntimePreservesPreviousDescriptionWhenFastLeavesItEmpty(t *testing.T) {
	current := testVirtualMachineForMerge("running", "192.168.1.180")
	current.Description = ""
	previous := testVirtualMachineForMerge("running", "192.168.1.180")
	previous.Description = "Production web service"

	merged := mergeFastRuntimeDetails(current, previous)
	if merged.Description != previous.Description {
		t.Fatalf("expected previous description %q, got %q", previous.Description, merged.Description)
	}
}

func TestMergeFastRuntimeUsesCurrentDescriptionWhenPresent(t *testing.T) {
	current := testVirtualMachineForMerge("running", "192.168.1.180")
	current.Description = "Updated service"
	previous := testVirtualMachineForMerge("running", "192.168.1.180")
	previous.Description = "Production web service"

	merged := mergeFastRuntimeDetails(current, previous)
	if merged.Description != current.Description {
		t.Fatalf("expected current description %q, got %q", current.Description, merged.Description)
	}
}

func TestMergeFastRuntimePreservesPreviousDiskDetailsWhenFastSkipsDiskCollection(t *testing.T) {
	current := testVirtualMachineForMerge("running", "192.168.1.180")
	current.DiskBytes = 0
	current.DiskUsedBytes = 0
	current.Disks = nil
	current.DiskUsage = 0
	current.DiskUsageAvailable = false
	previous := testVirtualMachineForMerge("running", "192.168.1.180")
	previous.DiskBytes = 100
	previous.DiskUsedBytes = 25
	previous.DiskUsage = 25
	previous.DiskUsageAvailable = true
	previous.Disks = []domain.VMDisk{{Name: "vda", Bytes: 100, UsedBytes: 25}}

	merged := mergeFastRuntimeDetails(current, previous)
	if merged.DiskBytes != previous.DiskBytes || merged.DiskUsedBytes != previous.DiskUsedBytes || merged.DiskUsage != previous.DiskUsage || !merged.DiskUsageAvailable {
		t.Fatalf("expected previous disk details to be preserved, got %+v", merged)
	}
	if len(merged.Disks) != 1 || merged.Disks[0].UsedBytes != 25 {
		t.Fatalf("expected previous disk list to be preserved, got %+v", merged.Disks)
	}
}

func TestMergeFastRuntimePreservesPreviousPrimaryIPWhenCurrentIsUnusable(t *testing.T) {
	current := testVirtualMachineForMerge("running", "169.254.10.20")
	previous := testVirtualMachineForMerge("running", "192.168.1.179")

	merged := mergeFastRuntimeDetails(current, previous)
	if merged.PrimaryIP != previous.PrimaryIP {
		t.Fatalf("expected previous primary ip %q, got %q", previous.PrimaryIP, merged.PrimaryIP)
	}
}

func TestMergeFastRuntimePreservesPreviousMemoryUsageWhenFastSkipsGuestMemory(t *testing.T) {
	current := testVirtualMachineForMerge("running", "192.168.1.180")
	current.MemoryUsage = 0
	current.MemoryUsageAvailable = false
	previous := testVirtualMachineForMerge("running", "192.168.1.180")
	previous.MemoryUsage = 42
	previous.MemoryUsageAvailable = true

	merged := mergeFastRuntimeDetails(current, previous)
	if merged.MemoryUsage != previous.MemoryUsage || !merged.MemoryUsageAvailable {
		t.Fatalf("expected previous memory usage to be preserved, got %+v", merged)
	}
}

func testVirtualMachineForMerge(status string, primaryIP string) domain.VirtualMachine {
	return domain.VirtualMachine{
		Status:    status,
		PrimaryIP: primaryIP,
		OSType:    "CentOS",
		DiskBytes: 1,
		Disks:     []domain.VMDisk{{Name: "vda", Bytes: 1}},
	}
}
