package realtime

import (
	"testing"

	"kvm-manager/backend/internal/domain"
)

func TestApplyDashboardVMStatsExcludesTemplates(t *testing.T) {
	summary := domain.DashboardSummary{StatusCounts: map[string]int{}}
	vms := []domain.VirtualMachine{
		{ID: "vm-running", Name: "web", Status: "running", CPUCores: 2},
		{ID: "vm-stopped", Name: "db", Status: "stopped", CPUCores: 4},
		{ID: "template-running", Name: "tpl-online", Status: "running", CPUCores: 8, IsTemplate: true},
		{ID: "template-stopped", Name: "tpl-offline", Status: "stopped", CPUCores: 16, IsTemplate: true},
	}

	applyDashboardVMStats(&summary, vms)

	if summary.TotalVMs != 2 {
		t.Fatalf("TotalVMs = %d, want 2", summary.TotalVMs)
	}
	if summary.RunningVMs != 1 {
		t.Fatalf("RunningVMs = %d, want 1", summary.RunningVMs)
	}
	if summary.StoppedVMs != 1 {
		t.Fatalf("StoppedVMs = %d, want 1", summary.StoppedVMs)
	}
	if summary.UsedVCPUs != 6 {
		t.Fatalf("UsedVCPUs = %d, want 6", summary.UsedVCPUs)
	}
	if len(summary.RecentVMs) != 2 {
		t.Fatalf("RecentVMs length = %d, want 2", len(summary.RecentVMs))
	}
	for _, vm := range summary.RecentVMs {
		if vm.IsTemplate {
			t.Fatalf("RecentVMs contains template: %+v", vm)
		}
	}
}
