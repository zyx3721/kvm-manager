package realtime

import (
	"testing"

	"kvm-manager/backend/internal/domain"
)

func TestRecordThresholdAlertSampleRequiresConsecutiveHits(t *testing.T) {
	service := New(nil, nil, "", nil, nil, 1, 0)

	if got := service.recordThresholdAlertSample("virtual_machine", "vm-1:memory", "虚拟机内存使用率过高", true); got != 1 {
		t.Fatalf("expected first hit count 1, got %d", got)
	}
	if got := service.recordThresholdAlertSample("virtual_machine", "vm-1:memory", "虚拟机内存使用率过高", true); got != 2 {
		t.Fatalf("expected second hit count 2, got %d", got)
	}
	if got := service.recordThresholdAlertSample("virtual_machine", "vm-1:memory", "虚拟机内存使用率过高", true); got != alertConsecutiveLimit {
		t.Fatalf("expected third hit count %d, got %d", alertConsecutiveLimit, got)
	}
	if got := service.recordThresholdAlertSample("virtual_machine", "vm-1:memory", "虚拟机内存使用率过高", false); got != 0 {
		t.Fatalf("expected reset count 0, got %d", got)
	}
	if got := service.recordThresholdAlertSample("virtual_machine", "vm-1:memory", "虚拟机内存使用率过高", true); got != 1 {
		t.Fatalf("expected count to restart at 1, got %d", got)
	}
}

func TestVMAlertMetadataIncludesIPAndDescription(t *testing.T) {
	metadata := vmAlertMetadata(
		domain.Agent{Name: "node-a"},
		domain.VirtualMachine{Name: "finance", PrimaryIP: "192.168.1.106", Description: "demo"},
		map[string]any{"metric": "cpu", "value": 90},
	)

	if metadata["agent"] != "node-a" || metadata["vm"] != "finance" {
		t.Fatalf("basic vm alert metadata invalid: %#v", metadata)
	}
	if metadata["vmIp"] != "192.168.1.106" {
		t.Fatalf("vmIp = %v", metadata["vmIp"])
	}
	if metadata["vmDescription"] != "demo" {
		t.Fatalf("vmDescription = %v", metadata["vmDescription"])
	}
	if metadata["metric"] != "cpu" || metadata["value"] != 90 {
		t.Fatalf("extra metadata invalid: %#v", metadata)
	}
}

func TestVMAlertMetadataUsesFallbackForEmptyIPAndDescription(t *testing.T) {
	metadata := vmAlertMetadata(
		domain.Agent{Name: "node-a"},
		domain.VirtualMachine{Name: "finance", PrimaryIP: " ", Description: ""},
		nil,
	)

	if metadata["vmIp"] != "-" {
		t.Fatalf("vmIp = %v", metadata["vmIp"])
	}
	if metadata["vmDescription"] != "-" {
		t.Fatalf("vmDescription = %v", metadata["vmDescription"])
	}
}
