package realtime

import (
	"testing"
	"time"

	"kvm-manager/backend/internal/domain"
)

func TestSortHostsByStatusThenName(t *testing.T) {
	hosts := []domain.Host{
		{Name: "zeta", Status: "offline"},
		{Name: "beta", Status: "online"},
		{Name: "alpha", Status: "online"},
		{Name: "gamma", Status: "maintenance"},
		{Name: "delta", Status: "degraded"},
	}

	sortHosts(hosts)

	got := make([]string, 0, len(hosts))
	for _, host := range hosts {
		got = append(got, host.Status+":"+host.Name)
	}
	want := []string{"online:alpha", "online:beta", "degraded:delta", "maintenance:gamma", "offline:zeta"}
	assertStringSliceEqual(t, got, want)
}

func TestSortVMsByStatusThenName(t *testing.T) {
	vms := []domain.VirtualMachine{
		{Name: "zeta", Status: "stopped"},
		{Name: "beta", Status: "running"},
		{Name: "alpha", Status: "running"},
		{Name: "gamma", Status: "error"},
		{Name: "delta", Status: "paused"},
	}

	sortVMs(vms)

	got := make([]string, 0, len(vms))
	for _, vm := range vms {
		got = append(got, vm.Status+":"+vm.Name)
	}
	want := []string{"running:alpha", "running:beta", "paused:delta", "stopped:zeta", "error:gamma"}
	assertStringSliceEqual(t, got, want)
}

func TestSortSnapshotsKeepsNewestFirst(t *testing.T) {
	now := time.Now()
	snapshots := []domain.Snapshot{
		{Name: "old", CreatedAt: now.Add(-time.Hour)},
		{Name: "new", CreatedAt: now},
	}

	sortSnapshots(snapshots)

	got := []string{snapshots[0].Name, snapshots[1].Name}
	want := []string{"new", "old"}
	assertStringSliceEqual(t, got, want)
}

func assertStringSliceEqual(t *testing.T, got []string, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("unexpected length: got %v want %v", got, want)
	}
	for index := range got {
		if got[index] != want[index] {
			t.Fatalf("unexpected order: got %v want %v", got, want)
		}
	}
}
