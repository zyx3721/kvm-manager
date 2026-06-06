package kvm

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestExtractDomainXML(t *testing.T) {
	input := "WARNING graphics listen address\n<domain type='kvm'>\n  <name>demo</name>\n</domain>\n"
	got := extractDomainXML(input)
	want := "<domain type='kvm'>\n  <name>demo</name>\n</domain>"
	if got != want {
		t.Fatalf("unexpected domain xml:\nwant: %q\n got: %q", want, got)
	}
}

func TestExtractDomainXMLRejectsMissingDomain(t *testing.T) {
	if got := extractDomainXML("WARNING only"); got != "" {
		t.Fatalf("expected empty xml, got %q", got)
	}
}

func TestCreateNetworkArgUsesLibvirtNetworkName(t *testing.T) {
	got := createNetworkArg("br172", "virtio", nil)
	want := "network=br172,model=virtio"
	if got != want {
		t.Fatalf("unexpected network arg: want %q, got %q", want, got)
	}
}

func TestCreateNetworkArgUsesBridgeDeviceForBridgePool(t *testing.T) {
	got := createNetworkArg("br51", "virtio", map[string]NetworkPool{
		"br51": {Name: "br51", Bridge: "br0", Forward: "bridge"},
	})
	want := "bridge=br0,model=virtio"
	if got != want {
		t.Fatalf("unexpected bridge network arg: want %q, got %q", want, got)
	}
}

func TestValidateVMCreateRequestNormalizesOSType(t *testing.T) {
	request := &VMCreateRequest{
		Name:            "demo",
		CurrentCPU:      2,
		MaximumCPU:      2,
		CurrentMemoryMB: 4096,
		MaximumMemoryMB: 4096,
		DiskName:        "demo-vda.qcow2",
		DiskPool:        "default",
		DiskFormat:      "qcow2",
		DiskBus:         "virtio",
		DiskCapacityGB:  40,
		NetworkSource:   "br172",
		NetworkModel:    "virtio",
		OSType:          " Windows ",
	}
	if err := validateVMCreateRequest(request); err != nil {
		t.Fatalf("validate create vm request failed: %v", err)
	}
	if request.OSType != "windows" {
		t.Fatalf("expected os type to be normalized, got %q", request.OSType)
	}
}

func TestValidateVMCreateRequestUsesLegacyDiskFields(t *testing.T) {
	request := &VMCreateRequest{
		Name:            "demo",
		CurrentCPU:      2,
		MaximumCPU:      2,
		CurrentMemoryMB: 4096,
		MaximumMemoryMB: 4096,
		DiskName:        "demo-vda.qcow2",
		DiskPool:        "default",
		DiskFormat:      "qcow2",
		DiskBus:         "virtio",
		DiskCapacityGB:  40,
		NetworkSource:   "default",
	}
	if err := validateVMCreateRequest(request); err != nil {
		t.Fatalf("validate create vm request failed: %v", err)
	}
	if len(request.Disks) != 1 {
		t.Fatalf("expected one normalized disk, got %d", len(request.Disks))
	}
	if request.Disks[0].Name != "demo-vda.qcow2" || request.Disks[0].Pool != "default" {
		t.Fatalf("unexpected normalized disk: %+v", request.Disks[0])
	}
}

func TestValidateVMCreateRequestDefaultsISOBus(t *testing.T) {
	request := validCreateRequestWithDisks([]VMCreateDiskRequest{
		{Name: "demo-vda.qcow2", Pool: "default", Format: "qcow2", Bus: "virtio", CapacityGB: 40},
	})
	if err := validateVMCreateRequest(request); err != nil {
		t.Fatalf("validate create vm request failed: %v", err)
	}
	if request.ISOBus != "sata" {
		t.Fatalf("expected iso bus to default to sata, got %q", request.ISOBus)
	}
}

func TestValidateVMCreateRequestRejectsUnsupportedISOBus(t *testing.T) {
	request := validCreateRequestWithDisks([]VMCreateDiskRequest{
		{Name: "demo-vda.qcow2", Pool: "default", Format: "qcow2", Bus: "virtio", CapacityGB: 40},
	})
	request.ISOBus = "virtio"
	if err := validateVMCreateRequest(request); err == nil {
		t.Fatal("expected unsupported iso bus error")
	}
}

func TestValidateVMCreateRequestRejectsDuplicateDiskNames(t *testing.T) {
	request := validCreateRequestWithDisks([]VMCreateDiskRequest{
		{Name: "demo-vda.qcow2", Pool: "default", Format: "qcow2", Bus: "virtio", CapacityGB: 40},
		{Name: "demo-vda.qcow2", Pool: "default", Format: "qcow2", Bus: "virtio", CapacityGB: 20},
	})
	if err := validateVMCreateRequest(request); err == nil {
		t.Fatal("expected duplicate disk name error")
	}
}

func TestValidateVMCreateRequestRejectsMismatchedDataDiskSettings(t *testing.T) {
	request := validCreateRequestWithDisks([]VMCreateDiskRequest{
		{Name: "demo-vda.qcow2", Pool: "default", Format: "qcow2", Bus: "virtio", CapacityGB: 40, PreallocMetadata: true},
		{Name: "demo-vdb.img", Pool: "default", Format: "raw", Bus: "virtio", CapacityGB: 20, PreallocMetadata: false},
	})
	if err := validateVMCreateRequest(request); err == nil {
		t.Fatal("expected mismatched data disk settings error")
	}
}

func TestValidateVMCreateRequestAllowsTemplateMode(t *testing.T) {
	request := validCreateRequestWithDisks(nil)
	request.CreateMode = "template"
	request.Template = VMCreateTemplate{
		SourcePool: "default",
		SourceName: "centos-template.qcow2",
		TargetPool: "default",
		TargetName: "demo-vda.qcow2",
		Bus:        "virtio",
		Format:     "qcow2",
	}
	if err := validateVMCreateRequest(request); err != nil {
		t.Fatalf("validate template create vm request failed: %v", err)
	}
	if len(request.Disks) != 1 || request.Disks[0].Name != "demo-vda.qcow2" {
		t.Fatalf("expected template target disk to be normalized, got %+v", request.Disks)
	}
}

func TestValidateVMCreateRequestRejectsTemplatePathSeparators(t *testing.T) {
	request := validCreateRequestWithDisks(nil)
	request.CreateMode = "template"
	request.Template = VMCreateTemplate{
		SourcePool: "default",
		SourceName: "../template.qcow2",
		TargetPool: "default",
		TargetName: "demo-vda.qcow2",
		Bus:        "virtio",
		Format:     "qcow2",
	}
	if err := validateVMCreateRequest(request); err == nil {
		t.Fatal("expected template path separator error")
	}
}

func TestValidateRequestedHostResourceLimitsRejectsOverflow(t *testing.T) {
	if err := validateRequestedHostResourceLimits(4, 8*1024*1024*1024, 8, 4096); err == nil {
		t.Fatal("expected cpu limit error")
	}
	if err := validateRequestedHostResourceLimits(8, 4*1024*1024*1024, 4, 8192); err == nil {
		t.Fatal("expected memory limit error")
	}
}

func TestCreateDiskArgUsesEachDiskSettings(t *testing.T) {
	got := createDiskArg("/var/lib/libvirt/images/demo-vdb.img", "sata", "raw")
	want := "path=/var/lib/libvirt/images/demo-vdb.img,bus=sata,format=raw"
	if got != want {
		t.Fatalf("unexpected disk arg: want %q, got %q", want, got)
	}
}

func TestCreateCDROMDiskArgUsesDiskDevice(t *testing.T) {
	got := createCDROMDiskArg("/iso/CentOS-7.iso", "sata")
	want := "path=/iso/CentOS-7.iso,device=cdrom,readonly=on,bus=sata"
	if got != want {
		t.Fatalf("unexpected cdrom disk arg: want %q, got %q", want, got)
	}
}

func TestCreateCDROMDiskArgSupportsEmptyMedia(t *testing.T) {
	got := createCDROMDiskArg("", "sata")
	want := "device=cdrom,readonly=on,bus=sata"
	if got != want {
		t.Fatalf("unexpected empty cdrom disk arg: want %q, got %q", want, got)
	}
}

func TestCreateGuestAgentChannelArg(t *testing.T) {
	got := createGuestAgentChannelArg()
	want := "unix,target_type=virtio,name=org.qemu.guest_agent.0"
	if got != want {
		t.Fatalf("unexpected guest agent channel arg: want %q, got %q", want, got)
	}
}

func TestBuildMigrationDomainXMLRewritesDiskPathsOnly(t *testing.T) {
	input := `<domain type='kvm'><name>demo</name><uuid>11111111-1111-1111-1111-111111111111</uuid><devices><disk type='file' device='disk'><source file='/source/demo.qcow2'/><target dev='vda' bus='virtio'/></disk><interface type='network'><mac address='52:54:00:aa:bb:cc'/><source network='default'/></interface></devices></domain>`
	output, err := buildMigrationDomainXML(input, map[string]string{"/source/demo.qcow2": "/target/demo.qcow2"})
	if err != nil {
		t.Fatalf("build migration xml failed: %v", err)
	}
	for _, expected := range []string{
		`<name>demo</name>`,
		`<uuid>11111111-1111-1111-1111-111111111111</uuid>`,
		`file="/target/demo.qcow2"`,
		`address="52:54:00:aa:bb:cc"`,
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected migration xml to contain %q, got %s", expected, output)
		}
	}
	if strings.Contains(output, "/source/demo.qcow2") {
		t.Fatalf("expected source disk path to be replaced, got %s", output)
	}
}

func TestStoragePoolTargetForPathAllowsDifferentPoolNameCoveringPath(t *testing.T) {
	pools := []storagePoolTarget{
		{Name: "default", Path: "/var/lib/libvirt/images"},
		{Name: "target-local", Path: "/kvm/images"},
	}

	target := storagePoolTargetForPath("/kvm/images/vm1.qcow2", pools)

	if target.Name != "target-local" {
		t.Fatalf("expected target-local pool to cover disk path, got %+v", target)
	}
}

func TestStoragePoolTargetForPathRejectsPartialPrefix(t *testing.T) {
	pools := []storagePoolTarget{{Name: "target-local", Path: "/kvm/images"}}

	target := storagePoolTargetForPath("/kvm/images2/vm1.qcow2", pools)

	if target.Name != "" {
		t.Fatalf("expected partial prefix to be rejected, got %+v", target)
	}
}

func TestMigrationSSHArgsUsePortOption(t *testing.T) {
	target := migrationTarget{Username: "root", Host: "10.0.0.2", Port: "2222"}
	sshArgs := migrationSSHArgs(target)
	if strings.Join(sshArgs, " ") != "-p 2222 root@10.0.0.2" {
		t.Fatalf("unexpected ssh args: %v", sshArgs)
	}
	scpArgs := migrationSCPArgs(target)
	if strings.Join(scpArgs, " ") != "-P 2222" {
		t.Fatalf("unexpected scp args: %v", scpArgs)
	}
}

func TestDiskVolumeForDeleteUsesOnlyOrdinaryDiskVolume(t *testing.T) {
	poolName, volumeName := diskVolumeForDelete(VMConfigDisk{Pool: "default", Path: "/kvm/images/demo-vda.qcow2"})
	if poolName != "default" || volumeName != "demo-vda.qcow2" {
		t.Fatalf("unexpected disk volume: pool=%q name=%q", poolName, volumeName)
	}
	poolName, volumeName = diskVolumeForDelete(VMConfigDisk{Pool: "", Path: "/kvm/iso/CentOS.iso"})
	if poolName != "" || volumeName != "" {
		t.Fatalf("expected disk without pool to be ignored, got pool=%q name=%q", poolName, volumeName)
	}
}

func TestUndefineVMAndDeleteConfigDisksDeletesOnlyOrdinaryDiskVolumes(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell stub uses POSIX argument handling")
	}
	dir := t.TempDir()
	logPath := filepath.Join(dir, "commands.log")
	virsh := filepath.Join(dir, "virsh")
	content := `#!/bin/sh
printf 'virsh %s\n' "$*" >> "` + logPath + `"
exit 0
`
	if err := os.WriteFile(virsh, []byte(content), 0o755); err != nil {
		t.Fatalf("write virsh stub: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	config := VMConfig{Disks: []VMConfigDisk{
		{Pool: "default", Path: "/kvm/images/demo-vda.qcow2"},
		{Pool: "", Path: "/iso/CentOS.iso"},
	}}

	err := NewVirshProvider("qemu:///system", 5*time.Second).undefineVMAndDeleteConfigDisks("demo", config)
	if err != nil {
		t.Fatalf("cleanup migrated source vm: %v", err)
	}

	logBytes, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read command log: %v", err)
	}
	log := string(logBytes)
	if !strings.Contains(log, "virsh --connect qemu:///system undefine demo") {
		t.Fatalf("expected undefine command in log:\n%s", log)
	}
	if !strings.Contains(log, "virsh --connect qemu:///system vol-delete --pool default demo-vda.qcow2") {
		t.Fatalf("expected disk volume delete command in log:\n%s", log)
	}
	if strings.Contains(log, "CentOS.iso") {
		t.Fatalf("did not expect ISO to be deleted:\n%s", log)
	}
}

func TestDestroyThenUndefineVMAndDeleteConfigDisksDestroysFirst(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell stub uses POSIX argument handling")
	}
	dir := t.TempDir()
	logPath := filepath.Join(dir, "commands.log")
	virsh := filepath.Join(dir, "virsh")
	content := `#!/bin/sh
printf 'virsh %s\n' "$*" >> "` + logPath + `"
exit 0
`
	if err := os.WriteFile(virsh, []byte(content), 0o755); err != nil {
		t.Fatalf("write virsh stub: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	config := VMConfig{Disks: []VMConfigDisk{{Pool: "default", Path: "/kvm/images/demo-vda.qcow2"}}}

	err := NewVirshProvider("qemu:///system", 5*time.Second).destroyThenUndefineVMAndDeleteConfigDisks("demo", config)
	if err != nil {
		t.Fatalf("cleanup migrated live source vm: %v", err)
	}

	logBytes, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read command log: %v", err)
	}
	log := string(logBytes)
	destroyIndex := strings.Index(log, "virsh --connect qemu:///system destroy demo")
	undefineIndex := strings.Index(log, "virsh --connect qemu:///system undefine demo")
	deleteIndex := strings.Index(log, "virsh --connect qemu:///system vol-delete --pool default demo-vda.qcow2")
	if destroyIndex < 0 || undefineIndex < 0 || deleteIndex < 0 {
		t.Fatalf("expected destroy, undefine and disk delete commands in log:\n%s", log)
	}
	if !(destroyIndex < undefineIndex && undefineIndex < deleteIndex) {
		t.Fatalf("expected destroy before undefine before delete, got log:\n%s", log)
	}
}

func TestMigrateLiveVMWithDiskCopyCopiesDisksThenRunsLiveMigrate(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell stub uses POSIX argument handling")
	}
	dir := t.TempDir()
	logPath := filepath.Join(dir, "commands.log")
	writeStub := func(name string, body string) {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
			t.Fatalf("write %s stub: %v", name, err)
		}
	}
	writeStub("virsh", `#!/bin/sh
printf 'virsh %s\n' "$*" >> "`+logPath+`"
case "$*" in
  *" domstate demo"*) printf 'running\n';;
  *" dumpxml demo"*) printf '%s\n' "<domain type='kvm'><name>demo</name><devices><disk type='file' device='disk'><source file='/kvm/images/demo.qcow2'/><target dev='vda' bus='virtio'/></disk></devices></domain>";;
esac
exit 0
`)
	writeStub("ssh", `#!/bin/sh
printf 'ssh %s\n' "$*" >> "`+logPath+`"
case "$*" in
  *"pool-list --all --name"*) printf 'default\n';;
  *"pool-dumpxml default"*) printf '%s\n' "<pool type='dir'><name>default</name><target><path>/kvm/images</path></target></pool>";;
esac
exit 0
`)
	writeStub("scp", `#!/bin/sh
printf 'scp %s\n' "$*" >> "`+logPath+`"
exit 0
`)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	err := NewVirshProvider("qemu:///system", 5*time.Second).MigrateVM("demo", VMMigrateRequest{
		DestinationURI: "qemu+ssh://root@10.0.0.2/system",
		Live:           true,
		CopyDisks:      true,
		Persistent:     true,
		AutoConverge:   true,
		PostCopy:       true,
	})
	if err != nil {
		t.Fatalf("live migrate with disk copy failed: %v", err)
	}
	logBytes, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read command log: %v", err)
	}
	log := string(logBytes)
	copyIndex := strings.Index(log, "scp /kvm/images/demo.qcow2 root@10.0.0.2:/kvm/images/demo.qcow2")
	migrateIndex := strings.Index(log, "virsh --connect qemu:///system migrate --live --unsafe --persistent --auto-converge --postcopy demo qemu+ssh://root@10.0.0.2/system")
	if copyIndex < 0 || migrateIndex < 0 {
		t.Fatalf("expected disk copy and live migrate commands in log:\n%s", log)
	}
	if copyIndex > migrateIndex {
		t.Fatalf("expected disk copy before live migrate:\n%s", log)
	}
	if strings.Contains(log, "--copy-storage-all") {
		t.Fatalf("did not expect --copy-storage-all in log:\n%s", log)
	}
}

func validCreateRequestWithDisks(disks []VMCreateDiskRequest) *VMCreateRequest {
	return &VMCreateRequest{
		Name:            "demo",
		CurrentCPU:      2,
		MaximumCPU:      2,
		CurrentMemoryMB: 4096,
		MaximumMemoryMB: 4096,
		Disks:           disks,
		NetworkSource:   "default",
	}
}
