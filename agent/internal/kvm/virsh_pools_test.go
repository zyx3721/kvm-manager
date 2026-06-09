package kvm

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestParseStorageVolumeDetailsCountSkipsDirectoryVolumes(t *testing.T) {
	output := `
Name                 Path                                      Type   Capacity  Allocation
-----------------------------------------------------------------------------------------
backup               /data/backup                              dir    0.00 B    0.00 B
base.qcow2           /data/base.qcow2                          file   10.00 GiB 1.00 GiB

seed.iso             /data/seed.iso                            file   4.00 GiB 4.00 GiB
extra.raw            /data/extra.raw                           block  8.00 GiB 2.00 GiB
`
	if got := parseStorageVolumeDetailsCount(output); got != 3 {
		t.Fatalf("expected 3 volumes, got %d", got)
	}
}

func TestParseStorageVolumesSkipsDirectoryVolumes(t *testing.T) {
	output := `
Name                 Path                                      Type   Capacity  Allocation
-----------------------------------------------------------------------------------------
backup               /data/backup                              dir    0.00 B    0.00 B
demo.qcow2           /data/demo.qcow2                          file   10.00 GiB 1.00 GiB
disk.raw             /dev/vg0/disk.raw                         block  20.00 GiB 2.00 GiB
`
	volumes := NewVirshProvider("qemu:///system", time.Second).parseStorageVolumes("data", output)
	if len(volumes) != 2 {
		t.Fatalf("expected directory volume to be skipped, got %d volumes: %#v", len(volumes), volumes)
	}
	for _, volume := range volumes {
		if strings.EqualFold(volume.Type, "dir") {
			t.Fatalf("did not expect dir volume in result: %#v", volumes)
		}
	}
	if volumes[0].Name != "demo.qcow2" || volumes[1].Name != "disk.raw" {
		t.Fatalf("unexpected volumes: %#v", volumes)
	}
}

func TestListStoragePoolsUsesLightweightVolumeCount(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell stub uses POSIX argument handling")
	}
	dir := t.TempDir()
	logPath := filepath.Join(dir, "commands.log")
	virshPath := filepath.Join(dir, "virsh")
	qemuImgPath := filepath.Join(dir, "qemu-img")

	virshContent := `#!/bin/sh
printf 'virsh %s\n' "$*" >> "` + logPath + `"
if [ "$3" = "pool-list" ]; then
  echo "images"
  exit 0
fi
if [ "$3" = "pool-info" ]; then
  cat <<'OUT'
Name:           images
UUID:           00000000-0000-0000-0000-000000000000
State:          running
Persistent:     yes
Autostart:      yes
Capacity:       1000
Allocation:     400
Available:      600
OUT
  exit 0
fi
if [ "$3" = "pool-dumpxml" ]; then
  cat <<'XML'
<pool type='dir'>
  <name>images</name>
  <target><path>/var/lib/libvirt/images</path></target>
</pool>
XML
  exit 0
fi
if [ "$3" = "pool-refresh" ]; then
  exit 0
fi
if [ "$3" = "vol-list" ] && [ "$5" = "--details" ]; then
  cat <<'OUT'
Name                 Path                                      Type   Capacity  Allocation
-----------------------------------------------------------------------------------------
backup               /var/lib/libvirt/images/backup            dir    0.00 B    0.00 B
demo.qcow2           /var/lib/libvirt/images/demo.qcow2        file   10.00 GiB 1.00 GiB
seed.iso             /var/lib/libvirt/images/seed.iso          file   4.00 GiB 4.00 GiB
OUT
  exit 0
fi
if [ "$3" = "vol-list" ]; then
  echo "unexpected vol-list arguments" >&2
  exit 1
fi
exit 1
`
	qemuImgContent := `#!/bin/sh
printf 'qemu-img %s\n' "$*" >> "` + logPath + `"
exit 1
`
	if err := os.WriteFile(virshPath, []byte(virshContent), 0o755); err != nil {
		t.Fatalf("write virsh stub: %v", err)
	}
	if err := os.WriteFile(qemuImgPath, []byte(qemuImgContent), 0o755); err != nil {
		t.Fatalf("write qemu-img stub: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	pools, err := NewVirshProvider("qemu:///system", 5*time.Second).ListStoragePools()
	if err != nil {
		t.Fatalf("list storage pools: %v", err)
	}
	if len(pools) != 1 {
		t.Fatalf("expected one storage pool, got %d", len(pools))
	}
	if pools[0].VolumeCount != 2 {
		t.Fatalf("expected lightweight volume count 2, got %d; commands:\n%s", pools[0].VolumeCount, readTestLog(t, logPath))
	}

	logBytes, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read command log: %v", err)
	}
	log := string(logBytes)
	if !strings.Contains(log, "vol-list images --details") {
		t.Fatalf("expected lightweight vol-list command in log:\n%s", log)
	}
	if strings.Contains(log, "--name") {
		t.Fatalf("did not expect unsupported vol-list --name during storage pool list:\n%s", log)
	}
	if strings.Contains(log, "qemu-img") {
		t.Fatalf("did not expect qemu-img during storage pool list:\n%s", log)
	}
}

func readTestLog(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		return "<unavailable>"
	}
	return string(content)
}
