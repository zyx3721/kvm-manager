package kvm

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestDescribeVMFastDoesNotGuessOSTypeFromName(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "virsh")
	content := `#!/bin/sh
if [ "$3" = "domstate" ]; then
  echo "shut off"
  exit 0
fi
if [ "$3" = "dumpxml" ]; then
  cat <<'XML'
<domain type='kvm'>
  <name>ct7-template</name>
  <uuid>11111111-1111-1111-1111-111111111111</uuid>
  <memory unit='KiB'>1048576</memory>
  <currentMemory unit='KiB'>524288</currentMemory>
  <vcpu current='1'>2</vcpu>
  <os><type arch='x86_64'>hvm</type></os>
  <devices></devices>
</domain>
XML
  exit 0
fi
exit 0
`
	perm := os.FileMode(0o755)
	if runtime.GOOS == "windows" {
		script += ".bat"
		content = `@echo off
if "%3"=="domstate" goto domstate
if "%3"=="dumpxml" goto dumpxml
exit /b 0
:domstate
echo shut off
exit /b 0
:dumpxml
echo ^<domain type='kvm'^>
echo   ^<name^>ct7-template^</name^>
echo   ^<uuid^>11111111-1111-1111-1111-111111111111^</uuid^>
echo   ^<memory unit='KiB'^>1048576^</memory^>
echo   ^<currentMemory unit='KiB'^>524288^</currentMemory^>
echo   ^<vcpu current='1'^>2^</vcpu^>
echo   ^<os^>^<type arch='x86_64'^>hvm^</type^>^</os^>
echo   ^<devices^>^</devices^>
echo ^</domain^>
exit /b 0
`
		perm = 0o644
	}
	if err := os.WriteFile(script, []byte(content), perm); err != nil {
		t.Fatalf("write virsh stub: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	vm, err := NewVirshProvider("qemu:///system", 5*time.Second).describeVM("ct7-template", cpuUsageSample{}, ioRateSample{}, true)
	if err != nil {
		t.Fatalf("describe vm: %v", err)
	}
	if strings.TrimSpace(vm.OSType) != "" {
		t.Fatalf("expected fast refresh to leave os type empty, got %q", vm.OSType)
	}
}

func TestDescribeVMReadsDescriptionFromXML(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "virsh")
	content := `#!/bin/sh
if [ "$3" = "domstate" ]; then
  echo "shut off"
  exit 0
fi
if [ "$3" = "dumpxml" ]; then
  cat <<'XML'
<domain type='kvm'>
  <name>demo</name>
  <uuid>11111111-1111-1111-1111-111111111111</uuid>
  <description>Production web service</description>
  <memory unit='KiB'>1048576</memory>
  <currentMemory unit='KiB'>524288</currentMemory>
  <vcpu current='1'>2</vcpu>
  <os><type arch='x86_64'>hvm</type></os>
  <devices></devices>
</domain>
XML
  exit 0
fi
exit 0
`
	perm := os.FileMode(0o755)
	if runtime.GOOS == "windows" {
		script += ".bat"
		content = `@echo off
if "%3"=="domstate" goto domstate
if "%3"=="dumpxml" goto dumpxml
exit /b 0
:domstate
echo shut off
exit /b 0
:dumpxml
echo ^<domain type='kvm'^>
echo   ^<name^>demo^</name^>
echo   ^<uuid^>11111111-1111-1111-1111-111111111111^</uuid^>
echo   ^<description^>Production web service^</description^>
echo   ^<memory unit='KiB'^>1048576^</memory^>
echo   ^<currentMemory unit='KiB'^>524288^</currentMemory^>
echo   ^<vcpu current='1'^>2^</vcpu^>
echo   ^<os^>^<type arch='x86_64'^>hvm^</type^>^</os^>
echo   ^<devices^>^</devices^>
echo ^</domain^>
exit /b 0
`
		perm = 0o644
	}
	if err := os.WriteFile(script, []byte(content), perm); err != nil {
		t.Fatalf("write virsh stub: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	vm, err := NewVirshProvider("qemu:///system", 5*time.Second).describeVM("demo", cpuUsageSample{}, ioRateSample{}, false)
	if err != nil {
		t.Fatalf("describe vm: %v", err)
	}
	if vm.Description != "Production web service" {
		t.Fatalf("expected xml description, got %q", vm.Description)
	}
}

func TestPrimaryIPUsesDomifaddrAgentBeforeLease(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "virsh")
	content := `#!/bin/sh
if [ "$3" = "qemu-agent-command" ]; then
  echo "qemu-agent-command should not be used for primary ip" >&2
  exit 1
fi
if [ "$3" = "domifaddr" ] && [ "$5" = "--source" ] && [ "$6" = "agent" ]; then
  cat <<'OUT'
 Name       MAC address          Protocol     Address
-------------------------------------------------------------------------------
 lo         00:00:00:00:00:00    ipv4         127.0.0.1/8
 eth0       52:54:00:4f:d5:e0    ipv4         192.168.51.56/24
OUT
  exit 0
fi
if [ "$3" = "domifaddr" ] && [ "$5" = "--source" ] && [ "$6" = "lease" ]; then
  echo "eth0 52:54:00:4f:d5:e0 ipv4 192.168.51.99/24"
  exit 0
fi
exit 0
`
	perm := os.FileMode(0o755)
	if runtime.GOOS == "windows" {
		script += ".bat"
		content = `@echo off
if "%3"=="qemu-agent-command" (
  echo qemu-agent-command should not be used for primary ip 1>&2
  exit /b 1
)
if "%3"=="domifaddr" if "%5"=="--source" if "%6"=="agent" goto domifaddr_agent
if "%3"=="domifaddr" if "%5"=="--source" if "%6"=="lease" goto domifaddr_lease
exit /b 0
:domifaddr_agent
echo  Name       MAC address          Protocol     Address
echo -------------------------------------------------------------------------------
echo  lo         00:00:00:00:00:00    ipv4         127.0.0.1/8
echo  eth0       52:54:00:4f:d5:e0    ipv4         192.168.51.56/24
exit /b 0
:domifaddr_lease
echo eth0 52:54:00:4f:d5:e0 ipv4 192.168.51.99/24
exit /b 0
`
		perm = 0o644
	}
	if err := os.WriteFile(script, []byte(content), perm); err != nil {
		t.Fatalf("write virsh stub: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	ip := NewVirshProvider("qemu:///system", 5*time.Second).primaryIP("demo")
	if ip != "192.168.51.56" {
		t.Fatalf("expected primary ip from domifaddr agent, got %q", ip)
	}
}

func TestPrimaryIPFallsBackToLease(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "virsh")
	content := `#!/bin/sh
if [ "$3" = "domifaddr" ] && [ "$5" = "--source" ] && [ "$6" = "agent" ]; then
  exit 1
fi
if [ "$3" = "domifaddr" ] && [ "$5" = "--source" ] && [ "$6" = "lease" ]; then
  echo "eth0 52:54:00:4f:d5:e0 ipv4 192.168.51.99/24"
  exit 0
fi
exit 0
`
	perm := os.FileMode(0o755)
	if runtime.GOOS == "windows" {
		script += ".bat"
		content = `@echo off
if "%3"=="domifaddr" if "%5"=="--source" if "%6"=="agent" exit /b 1
if "%3"=="domifaddr" if "%5"=="--source" if "%6"=="lease" goto domifaddr_lease
exit /b 0
:domifaddr_lease
echo eth0 52:54:00:4f:d5:e0 ipv4 192.168.51.99/24
exit /b 0
`
		perm = 0o644
	}
	if err := os.WriteFile(script, []byte(content), perm); err != nil {
		t.Fatalf("write virsh stub: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	ip := NewVirshProvider("qemu:///system", 5*time.Second).primaryIP("demo")
	if ip != "192.168.51.99" {
		t.Fatalf("expected primary ip from domifaddr lease, got %q", ip)
	}
}

func TestDescribeVMUsesXMLCurrentMemoryForListSpec(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "virsh")
	content := `#!/bin/sh
if [ "$3" = "domstate" ]; then
  echo "running"
  exit 0
fi
if [ "$3" = "dominfo" ]; then
  echo "Max memory:     4194304 KiB"
  echo "Used memory:    4194304 KiB"
  exit 0
fi
if [ "$3" = "dommemstat" ]; then
  echo "actual 4194304"
  echo "usable 1048576"
  exit 0
fi
if [ "$3" = "dumpxml" ] && [ "$5" = "--inactive" ]; then
  cat <<'XML'
<domain type='kvm'>
  <name>demo</name>
  <uuid>11111111-1111-1111-1111-111111111111</uuid>
  <memory unit='KiB'>4194304</memory>
  <currentMemory unit='KiB'>2097152</currentMemory>
  <vcpu current='1'>2</vcpu>
  <os><type arch='x86_64'>hvm</type></os>
  <devices></devices>
</domain>
XML
  exit 0
fi
if [ "$3" = "dumpxml" ]; then
  cat <<'XML'
<domain type='kvm'>
  <name>demo</name>
  <uuid>11111111-1111-1111-1111-111111111111</uuid>
  <memory unit='KiB'>4194304</memory>
  <currentMemory unit='KiB'>4194304</currentMemory>
  <vcpu current='1'>2</vcpu>
  <os><type arch='x86_64'>hvm</type></os>
  <devices></devices>
</domain>
XML
  exit 0
fi
exit 0
`
	perm := os.FileMode(0o755)
	if runtime.GOOS == "windows" {
		script += ".bat"
		content = `@echo off
if "%3"=="domstate" goto domstate
if "%3"=="dominfo" goto dominfo
if "%3"=="dommemstat" goto dommemstat
if "%3"=="dumpxml" if "%5"=="--inactive" goto inactive_dumpxml
if "%3"=="dumpxml" goto dumpxml
exit /b 0
:domstate
echo running
exit /b 0
:dominfo
echo Max memory:     4194304 KiB
echo Used memory:    4194304 KiB
exit /b 0
:dommemstat
echo actual 4194304
echo usable 1048576
exit /b 0
:inactive_dumpxml
echo ^<domain type='kvm'^>
echo   ^<name^>demo^</name^>
echo   ^<uuid^>11111111-1111-1111-1111-111111111111^</uuid^>
echo   ^<memory unit='KiB'^>4194304^</memory^>
echo   ^<currentMemory unit='KiB'^>2097152^</currentMemory^>
echo   ^<vcpu current='1'^>2^</vcpu^>
echo   ^<os^>^<type arch='x86_64'^>hvm^</type^>^</os^>
echo   ^<devices^>^</devices^>
echo ^</domain^>
exit /b 0
:dumpxml
echo ^<domain type='kvm'^>
echo   ^<name^>demo^</name^>
echo   ^<uuid^>11111111-1111-1111-1111-111111111111^</uuid^>
echo   ^<memory unit='KiB'^>4194304^</memory^>
echo   ^<currentMemory unit='KiB'^>4194304^</currentMemory^>
echo   ^<vcpu current='1'^>2^</vcpu^>
echo   ^<os^>^<type arch='x86_64'^>hvm^</type^>^</os^>
echo   ^<devices^>^</devices^>
echo ^</domain^>
exit /b 0
`
		perm = 0o644
	}
	if err := os.WriteFile(script, []byte(content), perm); err != nil {
		t.Fatalf("write virsh stub: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	vm, err := NewVirshProvider("qemu:///system", 5*time.Second).describeVM("demo", cpuUsageSample{}, ioRateSample{}, false)
	if err != nil {
		t.Fatalf("describe vm: %v", err)
	}
	if vm.MemoryBytes != 2*1024*1024*1024 {
		t.Fatalf("expected list memory spec from inactive XML currentMemory, got %d", vm.MemoryBytes)
	}
	if vm.MemoryUsage != 75 || !vm.MemoryUsageAvailable {
		t.Fatalf("expected memory usage from dommemstat actual/usable, got usage=%d available=%t", vm.MemoryUsage, vm.MemoryUsageAvailable)
	}
}

func TestDescribeVMFastUsesDommemstatForRunningVM(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "virsh")
	content := `#!/bin/sh
if [ "$3" = "domstate" ]; then
  echo "running"
  exit 0
fi
if [ "$3" = "dumpxml" ]; then
  cat <<'XML'
<domain type='kvm'>
  <name>demo-fast</name>
  <uuid>11111111-1111-1111-1111-111111111111</uuid>
  <memory unit='KiB'>4194304</memory>
  <currentMemory unit='KiB'>4194304</currentMemory>
  <vcpu current='1'>2</vcpu>
  <os><type arch='x86_64'>hvm</type></os>
  <devices></devices>
</domain>
XML
  exit 0
fi
if [ "$3" = "dommemstat" ]; then
  echo "actual 4194304"
  echo "usable 1048576"
  exit 0
fi
exit 0
`
	perm := os.FileMode(0o755)
	if runtime.GOOS == "windows" {
		script += ".bat"
		content = `@echo off
if "%3"=="domstate" goto domstate
if "%3"=="dumpxml" goto dumpxml
if "%3"=="dommemstat" goto dommemstat
exit /b 0
:domstate
echo running
exit /b 0
:dumpxml
echo ^<domain type='kvm'^>
echo   ^<name^>demo-fast^</name^>
echo   ^<uuid^>11111111-1111-1111-1111-111111111111^</uuid^>
echo   ^<memory unit='KiB'^>4194304^</memory^>
echo   ^<currentMemory unit='KiB'^>4194304^</currentMemory^>
echo   ^<vcpu current='1'^>2^</vcpu^>
echo   ^<os^>^<type arch='x86_64'^>hvm^</type^>^</os^>
echo   ^<devices^>^</devices^>
echo ^</domain^>
exit /b 0
:dommemstat
echo actual 4194304
echo usable 1048576
exit /b 0
`
		perm = 0o644
	}
	if err := os.WriteFile(script, []byte(content), perm); err != nil {
		t.Fatalf("write virsh stub: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	vm, err := NewVirshProvider("qemu:///system", 5*time.Second).describeVM("demo-fast", cpuUsageSample{}, ioRateSample{}, true)
	if err != nil {
		t.Fatalf("describe vm: %v", err)
	}
	if vm.MemoryUsage != 75 || !vm.MemoryUsageAvailable {
		t.Fatalf("expected fast memory usage from dommemstat, got usage=%d available=%t", vm.MemoryUsage, vm.MemoryUsageAvailable)
	}
}

func TestCreateSnapshotUsesInternalSnapshotCommand(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell stub uses POSIX argument handling")
	}
	dir := t.TempDir()
	logPath := filepath.Join(dir, "commands.log")
	script := filepath.Join(dir, "virsh")
	content := `#!/bin/sh
printf '%s\n' "$*" >> "` + logPath + `"
if [ "$3" = "dumpxml" ]; then
  cat <<'XML'
<domain type='kvm'>
  <name>demo</name>
  <devices>
    <disk type='file' device='disk'>
      <source file='/var/lib/libvirt/images/demo.qcow2'/>
      <target dev='vda' bus='virtio'/>
    </disk>
    <disk type='file' device='cdrom'>
      <source file='/var/lib/libvirt/images/installer.iso'/>
      <target dev='sda' bus='sata'/>
    </disk>
  </devices>
</domain>
XML
  exit 0
fi
exit 0
`
	if err := os.WriteFile(script, []byte(content), 0o755); err != nil {
		t.Fatalf("write virsh stub: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	err := NewVirshProvider("qemu:///system", 5*time.Second).CreateSnapshot("demo", SnapshotCreateRequest{
		Name:        "snap-demo",
		Description: "before upgrade",
	})
	if err != nil {
		t.Fatalf("create snapshot: %v", err)
	}
	logBytes, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read command log: %v", err)
	}
	log := string(logBytes)
	if !strings.Contains(log, "--connect qemu:///system snapshot-create-as demo snap-demo --description before upgrade --atomic") {
		t.Fatalf("expected internal snapshot create command in log:\n%s", log)
	}
	for _, forbidden := range []string{"dumpxml", "--disk-only", "--diskspec", "snapshot=internal", "--quiesce", "blockcommit"} {
		if strings.Contains(log, forbidden) {
			t.Fatalf("did not expect %q in log:\n%s", forbidden, log)
		}
	}
}

func TestParseSnapshotListKeepsCreationTimeAndState(t *testing.T) {
	out := `
 Name                           Creation Time               State
------------------------------------------------------------
 snap-ct7-template-202606020108 2026-06-02 01:09:02 +0800 disk-snapshot
`
	snapshots := parseSnapshotList(out)
	if len(snapshots) != 1 {
		t.Fatalf("expected one snapshot, got %d", len(snapshots))
	}
	if snapshots[0].Name != "snap-ct7-template-202606020108" {
		t.Fatalf("unexpected snapshot name: %q", snapshots[0].Name)
	}
	if snapshots[0].CreatedAt != "2026-06-02 01:09:02 +0800" {
		t.Fatalf("unexpected creation time: %q", snapshots[0].CreatedAt)
	}
	if snapshots[0].Status != "disk-snapshot" {
		t.Fatalf("unexpected state: %q", snapshots[0].Status)
	}
}

func TestDeleteSnapshotUsesDirectSnapshotDelete(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell stub uses POSIX argument handling")
	}
	dir := t.TempDir()
	logPath := filepath.Join(dir, "commands.log")
	script := filepath.Join(dir, "virsh")
	content := `#!/bin/sh
printf '%s\n' "$*" >> "` + logPath + `"
exit 0
`
	if err := os.WriteFile(script, []byte(content), 0o755); err != nil {
		t.Fatalf("write virsh stub: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	err := NewVirshProvider("qemu:///system", 5*time.Second).DeleteSnapshot("demo", "snap-demo")
	if err != nil {
		t.Fatalf("delete snapshot: %v", err)
	}
	logBytes, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read command log: %v", err)
	}
	log := string(logBytes)
	if !strings.Contains(log, "--connect qemu:///system snapshot-delete demo snap-demo") {
		t.Fatalf("expected direct snapshot delete command in log:\n%s", log)
	}
	for _, forbidden := range []string{"blockcommit", "snapshot-dumpxml", "--metadata", "vol-delete"} {
		if strings.Contains(log, forbidden) {
			t.Fatalf("did not expect %q in log:\n%s", forbidden, log)
		}
	}
}
