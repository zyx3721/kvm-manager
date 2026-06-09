package kvm

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestParseStorageVolumeNameCountSkipsBlankLines(t *testing.T) {
	output := "\nbase.qcow2\n\nseed.iso\n  \nextra.raw\n"
	if got := parseStorageVolumeNameCount(output); got != 3 {
		t.Fatalf("expected 3 volume names, got %d", got)
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
if [ "$3" = "vol-list" ] && [ "$5" = "--name" ]; then
  printf 'demo.qcow2\nseed.iso\n'
  exit 0
fi
if [ "$3" = "vol-list" ]; then
  echo "full vol-list should not be used for storage pool summary" >&2
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
	if !strings.Contains(log, "vol-list images --name") {
		t.Fatalf("expected lightweight vol-list command in log:\n%s", log)
	}
	if strings.Contains(log, "qemu-img") {
		t.Fatalf("did not expect qemu-img during storage pool list:\n%s", log)
	}
	if strings.Contains(log, "--details") {
		t.Fatalf("did not expect full volume details during storage pool list:\n%s", log)
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
