package kvm

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestValidateVMConfigUpdate(t *testing.T) {
	valid := VMConfigUpdateRequest{
		Description:     "test vm",
		CurrentCPU:      2,
		MaximumCPU:      4,
		CurrentMemoryMB: 2048,
		MaximumMemoryMB: 4096,
	}
	if err := validateVMConfigUpdate(valid); err != nil {
		t.Fatalf("expected valid config, got %v", err)
	}

	cases := []struct {
		name    string
		request VMConfigUpdateRequest
	}{
		{name: "zero current cpu", request: VMConfigUpdateRequest{CurrentCPU: 0, MaximumCPU: 4, CurrentMemoryMB: 2048, MaximumMemoryMB: 4096}},
		{name: "current cpu exceeds maximum", request: VMConfigUpdateRequest{CurrentCPU: 8, MaximumCPU: 4, CurrentMemoryMB: 2048, MaximumMemoryMB: 4096}},
		{name: "zero current memory", request: VMConfigUpdateRequest{CurrentCPU: 2, MaximumCPU: 4, CurrentMemoryMB: 0, MaximumMemoryMB: 4096}},
		{name: "current memory exceeds maximum", request: VMConfigUpdateRequest{CurrentCPU: 2, MaximumCPU: 4, CurrentMemoryMB: 8192, MaximumMemoryMB: 4096}},
		{name: "description too long", request: VMConfigUpdateRequest{Description: strings.Repeat("a", 2049), CurrentCPU: 2, MaximumCPU: 4, CurrentMemoryMB: 2048, MaximumMemoryMB: 4096}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := validateVMConfigUpdate(tc.request); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestValidateRunningVMConfigExpansion(t *testing.T) {
	config := VMConfig{
		CurrentCPU:         2,
		MaximumCPU:         4,
		CurrentMemoryBytes: 2 * 1024 * 1024 * 1024,
		MaximumMemoryBytes: 4 * 1024 * 1024 * 1024,
	}
	valid := VMConfigUpdateRequest{
		CurrentCPU:      4,
		MaximumCPU:      4,
		CurrentMemoryMB: 4096,
		MaximumMemoryMB: 4096,
	}
	if err := validateRunningVMConfigExpansion(valid, config); err != nil {
		t.Fatalf("expected running expansion to be valid, got %v", err)
	}

	cases := []struct {
		name    string
		request VMConfigUpdateRequest
	}{
		{name: "change maximum cpu", request: VMConfigUpdateRequest{CurrentCPU: 3, MaximumCPU: 8, CurrentMemoryMB: 3072, MaximumMemoryMB: 4096}},
		{name: "shrink cpu", request: VMConfigUpdateRequest{CurrentCPU: 1, MaximumCPU: 4, CurrentMemoryMB: 3072, MaximumMemoryMB: 4096}},
		{name: "cpu exceeds maximum", request: VMConfigUpdateRequest{CurrentCPU: 8, MaximumCPU: 4, CurrentMemoryMB: 3072, MaximumMemoryMB: 4096}},
		{name: "change maximum memory", request: VMConfigUpdateRequest{CurrentCPU: 3, MaximumCPU: 4, CurrentMemoryMB: 3072, MaximumMemoryMB: 8192}},
		{name: "shrink memory", request: VMConfigUpdateRequest{CurrentCPU: 3, MaximumCPU: 4, CurrentMemoryMB: 1024, MaximumMemoryMB: 4096}},
		{name: "memory exceeds maximum", request: VMConfigUpdateRequest{CurrentCPU: 3, MaximumCPU: 4, CurrentMemoryMB: 8192, MaximumMemoryMB: 4096}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := validateRunningVMConfigExpansion(tc.request, config); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestHasCDROMTarget(t *testing.T) {
	cdroms := []VMConfigCDROM{
		{Name: "sda", Bus: "sata"},
		{Name: "hda", Bus: "ide"},
	}
	if !hasCDROMTarget(cdroms, "sda") {
		t.Fatal("expected sda target to exist")
	}
	if hasCDROMTarget(cdroms, "vda") {
		t.Fatal("expected vda target to be absent")
	}
}

func TestMediaChangeScopeArgs(t *testing.T) {
	running := mediaChangeScopeArgs("running")
	if strings.Join(running, " ") != "--live --config" {
		t.Fatalf("expected live config scope, got %v", running)
	}
	stopped := mediaChangeScopeArgs("stopped")
	if strings.Join(stopped, " ") != "--config" {
		t.Fatalf("expected config scope, got %v", stopped)
	}
}

func TestLiveInterfaceArgs(t *testing.T) {
	attach := liveAttachInterfaceArgs(
		"qemu:///system",
		"vm01",
		domainInterfaceXMLFragment{Type: "bridge", Source: "br0", Model: "virtio"},
		"52:54:00:AA:BB:CC",
	)
	expectedAttach := "--connect qemu:///system attach-interface --domain vm01 --type bridge --source br0 --model virtio --live --config --mac 52:54:00:aa:bb:cc"
	if strings.Join(attach, " ") != expectedAttach {
		t.Fatalf("unexpected attach-interface args: %v", attach)
	}

	detach := liveDetachInterfaceArgs(
		"qemu:///system",
		"vm01",
		VMConfigInterface{Type: "bridge"},
		"52:54:00:AA:BB:CC",
	)
	expectedDetach := "--connect qemu:///system detach-interface --domain vm01 --type bridge --mac 52:54:00:aa:bb:cc --live --config"
	if strings.Join(detach, " ") != expectedDetach {
		t.Fatalf("unexpected detach-interface args: %v", detach)
	}
}

func TestQemuImgResizeSizeArgUsesPlainBytes(t *testing.T) {
	got := qemuImgResizeSizeArg(53687091200)
	if got != "53687091200" {
		t.Fatalf("expected plain byte size, got %q", got)
	}
}

func TestQemuImgInfoHasSnapshots(t *testing.T) {
	out := `{"virtual-size":23622320128,"filename":"/kvm/iso/demo.qcow2","format":"qcow2","snapshots":[{"id":"1","name":"base"}]}`
	if !qemuImgInfoHasSnapshots(out) {
		t.Fatal("expected qemu-img info snapshots to be detected")
	}
}

func TestFriendlyDiskResizeErrorForInternalSnapshots(t *testing.T) {
	err := friendlyDiskResizeError(fmt.Errorf("qemu-img resize /kvm/iso/demo.qcow2 23622320128 failed: exit status 1: qemu-img: Can't resize an image which has snapshots qemu-img: This image does not support resize"))
	if err.Error() != "disk image has internal snapshots and cannot be resized" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSupportedVMInterfaceModelIncludesE1000e(t *testing.T) {
	if !supportedVMInterfaceModel("e1000e") {
		t.Fatal("expected e1000e to be supported")
	}
}

func TestMergeConfigInterfacesFillsLiveTarget(t *testing.T) {
	base := []VMConfigInterface{{
		MAC:    "52:54:00:c3:e9:25",
		Type:   "bridge",
		Source: "brvlan235",
		Model:  "virtio",
	}}
	live := []VMConfigInterface{{
		Name:   "vnet0",
		MAC:    "52:54:00:c3:e9:25",
		Type:   "bridge",
		Source: "brvlan235",
		Model:  "virtio",
	}}

	merged := mergeConfigInterfaces(base, live)
	if len(merged) != 1 {
		t.Fatalf("expected one interface, got %d", len(merged))
	}
	if merged[0].Name != "vnet0" {
		t.Fatalf("expected live target to fill interface name, got %q", merged[0].Name)
	}
}

func TestMergeConfigInterfacesPrefersLiveTypeAndSource(t *testing.T) {
	base := []VMConfigInterface{{
		Name:   "vnet0",
		MAC:    "52:54:00:4a:53:fc",
		Type:   "network",
		Source: "br0",
		Model:  "virtio",
	}}
	live := []VMConfigInterface{{
		Name:   "vnet0",
		MAC:    "52:54:00:4a:53:fc",
		Type:   "bridge",
		Source: "br0",
		Model:  "virtio",
	}}

	merged := mergeConfigInterfaces(base, live)
	if len(merged) != 1 {
		t.Fatalf("expected one interface, got %d", len(merged))
	}
	if merged[0].Type != "bridge" {
		t.Fatalf("expected live interface type, got %q", merged[0].Type)
	}
	if merged[0].Source != "br0" {
		t.Fatalf("expected live interface source, got %q", merged[0].Source)
	}
}

func TestUpdateDomainConsoleXML(t *testing.T) {
	input := `<domain type="kvm">
  <name>demo</name>
  <devices>
    <graphics type="vnc" listen="0.0.0.0"></graphics>
  </devices>
</domain>`
	output, err := updateDomainConsoleXML(input, VMConsoleUpdateRequest{PasswordEnabled: true, Password: "secret"})
	if err != nil {
		t.Fatalf("update console xml failed: %v", err)
	}
	if !strings.Contains(output, `passwd="secret"`) {
		t.Fatalf("expected passwd attr, got %s", output)
	}
	output, err = updateDomainConsoleXML(output, VMConsoleUpdateRequest{PasswordEnabled: false})
	if err != nil {
		t.Fatalf("remove console password failed: %v", err)
	}
	if strings.Contains(output, `passwd=`) {
		t.Fatalf("expected passwd attr to be removed, got %s", output)
	}
}

func TestDomainConsolePasswordEnabled(t *testing.T) {
	withPassword := `<domain type="kvm"><devices><graphics type="vnc" passwd="secret"></graphics></devices></domain>`
	withoutPassword := `<domain type="kvm"><devices><graphics type="vnc"></graphics></devices></domain>`
	if !domainConsolePasswordEnabled(withPassword) {
		t.Fatal("expected vnc console password to be enabled")
	}
	if domainConsolePasswordEnabled(withoutPassword) {
		t.Fatal("expected vnc console password to be disabled")
	}
}

func TestUpdateVNCGraphicsDeviceXML(t *testing.T) {
	input := `<domain type="kvm">
  <name>demo</name>
  <devices>
    <emulator>/usr/bin/qemu-system-x86_64</emulator>
    <graphics type="vnc" port="5901" listen="0.0.0.0">
      <listen type="address" address="0.0.0.0"></listen>
    </graphics>
  </devices>
</domain>`
	output, err := updateVNCGraphicsDeviceXML(input, VMConsoleUpdateRequest{PasswordEnabled: true, Password: "secret"})
	if err != nil {
		t.Fatalf("update vnc graphics device xml failed: %v", err)
	}
	if !strings.HasPrefix(output, `<graphics `) {
		t.Fatalf("expected only graphics device xml, got %s", output)
	}
	if strings.Contains(output, "<domain") || strings.Contains(output, "<emulator") {
		t.Fatalf("expected device xml not full domain xml, got %s", output)
	}
	for _, expected := range []string{`type="vnc"`, `port="5901"`, `passwd="secret"`, `<listen type="address" address="0.0.0.0"`} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected device xml to contain %s, got %s", expected, output)
		}
	}
	output, err = updateVNCGraphicsDeviceXML(input, VMConsoleUpdateRequest{PasswordEnabled: false})
	if err != nil {
		t.Fatalf("clear vnc graphics password failed: %v", err)
	}
	if strings.Contains(output, `passwd=`) {
		t.Fatalf("expected device xml to remove passwd attr, got %s", output)
	}
}

func TestConfigGraphicsUsesSecurityInfoPassword(t *testing.T) {
	doc := domainXML{}
	securityDoc := domainXML{}
	if err := xml.Unmarshal([]byte(`<domain type="kvm"><devices><graphics type="vnc" port="-1"></graphics></devices></domain>`), &doc); err != nil {
		t.Fatalf("unmarshal domain xml failed: %v", err)
	}
	if err := xml.Unmarshal([]byte(`<domain type="kvm"><devices><graphics type="vnc" port="-1" passwd="secret"></graphics></devices></domain>`), &securityDoc); err != nil {
		t.Fatalf("unmarshal security domain xml failed: %v", err)
	}
	if configGraphics(doc).PasswordEnabled {
		t.Fatal("expected normal dumpxml not to expose password")
	}
	if !configGraphics(securityDoc).PasswordEnabled {
		t.Fatal("expected security-info dumpxml to expose password state")
	}
}

func TestParseDomainXMLKeepsCurrentAndMaximumMemoryDistinct(t *testing.T) {
	doc, err := parseDomainXML(`<domain type="kvm"><name>demo</name><memory unit="KiB">4194304</memory><currentMemory unit="KiB">2097152</currentMemory></domain>`)
	if err != nil {
		t.Fatalf("parse domain xml failed: %v", err)
	}
	if doc.CurrentMemory.Value != 2097152 {
		t.Fatalf("expected current memory from currentMemory node, got %d", doc.CurrentMemory.Value)
	}
	if doc.Memory.Value != 4194304 {
		t.Fatalf("expected maximum memory from memory node, got %d", doc.Memory.Value)
	}
}

func TestConfigMemoryKiBKeepsXMLValues(t *testing.T) {
	doc, err := parseDomainXML(`<domain type="kvm"><name>demo</name><memory unit="KiB">4194304</memory><currentMemory unit="KiB">2097152</currentMemory></domain>`)
	if err != nil {
		t.Fatalf("parse domain xml failed: %v", err)
	}
	current, maximum := configMemoryKiB(doc)
	if current != 2097152 {
		t.Fatalf("expected current memory from currentMemory node, got %d", current)
	}
	if maximum != 4194304 {
		t.Fatalf("expected maximum memory from memory node, got %d", maximum)
	}
}

func TestMergeConfigGraphicsPasswordState(t *testing.T) {
	current := configGraphicsFromXML(`<domain type="kvm"><devices><graphics type="vnc" port="5901" passwd="secret"></graphics></devices></domain>`)
	inactive := configGraphicsFromXML(`<domain type="kvm"><devices><graphics type="vnc" port="-1"></graphics></devices></domain>`)
	graphics := mergeConfigGraphics(inactive, current)
	if !graphics.PasswordEnabled {
		t.Fatal("expected merged graphics to keep password enabled from current xml")
	}
	if graphics.Type != "vnc" {
		t.Fatalf("expected vnc graphics, got %q", graphics.Type)
	}
}

func TestConsoleInfoUsesSecurityInfoPassword(t *testing.T) {
	doc := domainXML{}
	if err := xml.Unmarshal([]byte(`<domain type="kvm"><devices><graphics type="vnc" port="5901" listen="0.0.0.0" passwd="secret"></graphics></devices></domain>`), &doc); err != nil {
		t.Fatalf("unmarshal console xml failed: %v", err)
	}
	graphics := configGraphics(doc)
	if !graphics.PasswordEnabled {
		t.Fatal("expected console info source to expose password state")
	}
	if graphics.Listen != "0.0.0.0" || graphics.Port != "5901" {
		t.Fatalf("unexpected console graphics info: %+v", graphics)
	}
}

func TestStoragePoolNameForPath(t *testing.T) {
	pools := []storagePoolTarget{
		{Name: "default", Path: "/storage"},
		{Name: "Storage", Path: "/storage/vm"},
		{Name: "other", Path: "/storage2"},
	}

	if got := storagePoolNameForPath("/storage/vm/10.22.12.175-vda.qcow2", pools); got != "Storage" {
		t.Fatalf("expected Storage, got %q", got)
	}
	if got := storagePoolNameForPath("/storage/vmware/disk.qcow2", pools); got != "default" {
		t.Fatalf("expected default boundary match, got %q", got)
	}
	if got := storagePoolNameForPath("/storage2/disk.qcow2", pools); got != "other" {
		t.Fatalf("expected other, got %q", got)
	}
}

func TestConfigDisksUsesBackingStoreAsSourcePath(t *testing.T) {
	doc, err := parseDomainXML(`<domain type="kvm">
  <name>demo</name>
  <devices>
    <disk type="file" device="disk">
      <driver name="qemu" type="qcow2"/>
      <source file="/storage/vm/demo.snap"/>
      <target dev="vda" bus="virtio"/>
      <backingStore type="file">
        <format type="qcow2"/>
        <source file="/storage/vm/demo.qcow2"/>
        <backingStore/>
      </backingStore>
    </disk>
  </devices>
</domain>`)
	if err != nil {
		t.Fatalf("parse domain xml failed: %v", err)
	}
	provider := NewVirshProvider("qemu:///system", 5*time.Second)
	disks := provider.configDisks("demo", doc, false)
	if len(disks) != 1 {
		t.Fatalf("expected one disk, got %d", len(disks))
	}
	if disks[0].Path != "/storage/vm/demo.snap" {
		t.Fatalf("expected active path, got %q", disks[0].Path)
	}
	if disks[0].SourcePath != "/storage/vm/demo.qcow2" {
		t.Fatalf("expected source path from backing store, got %q", disks[0].SourcePath)
	}
}

func TestBuildCloneDomainXML(t *testing.T) {
	input := `<domain type="kvm">
  <name>demo</name>
  <uuid>old-uuid</uuid>
  <devices>
    <disk type="file" device="disk">
      <source file="/storage/vm/source.qcow2"></source>
      <target dev="vda" bus="virtio"></target>
    </disk>
    <disk type="file" device="cdrom">
      <source file="/iso/source.iso"></source>
      <target dev="sda" bus="sata"></target>
    </disk>
    <interface type="network">
      <mac address="52:54:00:aa:bb:cc"></mac>
      <source network="default"></source>
      <target dev="vnet1"></target>
      <model type="virtio"></model>
    </interface>
  </devices>
</domain>`
	output, err := buildCloneDomainXML(
		input,
		VMCloneRequest{
			Name:            "demo-clone",
			CurrentCPU:      1,
			MaximumCPU:      2,
			CurrentMemoryMB: 1024,
			MaximumMemoryMB: 2048,
			CDROMPolicy:     "disconnect",
		},
		map[string]string{"/storage/vm/source.qcow2": "/storage/vm/source-clone.qcow2"},
		map[string]string{"52:54:00:aa:bb:cc": "52:54:00:11:22:33"},
		map[string]string{"52:54:00:aa:bb:cc": "isolated"},
	)
	if err != nil {
		t.Fatalf("build clone domain xml failed: %v", err)
	}
	for _, expected := range []string{
		"<name>demo-clone</name>",
		`file="/storage/vm/source-clone.qcow2"`,
		`address="52:54:00:11:22:33"`,
		`network="isolated"`,
		`device="cdrom"`,
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected clone xml to contain %s, got %s", expected, output)
		}
	}
	for _, unexpected := range []string{"<uuid>old-uuid</uuid>", `dev="vnet1"`, `/iso/source.iso`} {
		if strings.Contains(output, unexpected) {
			t.Fatalf("expected clone xml to remove %s, got %s", unexpected, output)
		}
	}
}

func TestBuildCloneDomainXMLKeepsBridgeInterfaceSourceAttr(t *testing.T) {
	input := `<domain type="kvm">
  <name>demo</name>
  <devices>
    <interface type="bridge">
      <mac address="52:54:00:aa:bb:cc"></mac>
      <source bridge="br0"></source>
    </interface>
  </devices>
</domain>`
	output, err := buildCloneDomainXML(
		input,
		VMCloneRequest{Name: "demo-clone"},
		nil,
		map[string]string{"52:54:00:aa:bb:cc": "52:54:00:11:22:33"},
		map[string]string{"52:54:00:aa:bb:cc": "br172"},
	)
	if err != nil {
		t.Fatalf("build clone domain xml failed: %v", err)
	}
	if !strings.Contains(output, `bridge="br172"`) {
		t.Fatalf("expected bridge source attr, got %s", output)
	}
	if strings.Contains(output, `network="br172"`) {
		t.Fatalf("expected clone xml not to use network attr for bridge interface, got %s", output)
	}
}

func TestCloneInterfaceMapsFallbackToRequestOrder(t *testing.T) {
	sourceInterfaces := []VMConfigInterface{
		{MAC: "52:54:00:aa:bb:cc", Source: "default"},
	}
	requests := []VMCloneInterfaceRequest{
		{MAC: "52:54:00:11:22:33", Source: "br172"},
	}

	macs := vmCloneInterfaceMACsBySource(requests, sourceInterfaces)
	if got := macs["52:54:00:aa:bb:cc"]; got != "52:54:00:11:22:33" {
		t.Fatalf("expected mac fallback by order, got %q", got)
	}
	sources := vmCloneInterfaceSourcesByName(requests, sourceInterfaces, nil)
	if got := sources["52:54:00:aa:bb:cc"]; got != "br172" {
		t.Fatalf("expected source fallback by order, got %q", got)
	}
}

func TestCloneInterfaceSourcesResolveBridgePoolName(t *testing.T) {
	sourceInterfaces := []VMConfigInterface{
		{Name: "vnet0", MAC: "52:54:00:aa:bb:cc", Type: "bridge"},
	}
	requests := []VMCloneInterfaceRequest{
		{Name: "vnet0", MAC: "52:54:00:11:22:33", Source: "br2212"},
	}

	sources := vmCloneInterfaceSourcesByName(requests, sourceInterfaces, map[string]string{"br2212": "br0"})
	if got := sources["vnet0"]; got != "br0" {
		t.Fatalf("expected bridge pool source to resolve to bridge device, got %q", got)
	}
	if got := sources["52:54:00:aa:bb:cc"]; got != "br0" {
		t.Fatalf("expected mac source to resolve to bridge device, got %q", got)
	}
}

func TestUpdateDomainDeviceXML(t *testing.T) {
	input := `<domain type="kvm">
  <name>demo</name>
  <devices>
    <disk type="file" device="disk">
      <source file="/storage/vm/source.qcow2"></source>
      <target dev="vda" bus="virtio"></target>
    </disk>
    <interface type="network">
      <mac address="52:54:00:aa:bb:cc"></mac>
      <source network="default"></source>
      <target dev="vnet1"></target>
    </interface>
  </devices>
</domain>`
	output, err := updateDomainDeviceXML(
		input,
		map[string]domainInterfaceSourceUpdate{
			"52:54:00:aa:bb:cc": {Type: "network", SourceAttr: "network", Source: "isolated"},
		},
		nil,
		nil,
		[]domainDiskXMLFragment{{Path: "/fast/vm/new.qcow2", Target: "vdb", Bus: "virtio", Format: "qcow2"}},
		nil,
	)
	if err != nil {
		t.Fatalf("update domain device xml failed: %v", err)
	}
	for _, expected := range []string{`file="/storage/vm/source.qcow2"`, `network="isolated"`, `file="/fast/vm/new.qcow2"`, `dev="vdb"`, `type="qcow2"`} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected device xml to contain %s, got %s", expected, output)
		}
	}
	for _, unexpected := range []string{`network="default"`} {
		if strings.Contains(output, unexpected) {
			t.Fatalf("expected device xml to remove %s, got %s", unexpected, output)
		}
	}
}

func TestValidateVMDeviceDeletedDisksRejectsFirstDisk(t *testing.T) {
	config := VMConfig{Disks: []VMConfigDisk{
		{Name: "vda", Path: "/storage/vda.qcow2", Device: "disk"},
		{Name: "vdb", Path: "/storage/vdb.qcow2", Device: "disk"},
	}}
	if _, err := validateVMDeviceDeletedDisks([]VMDeviceDeleteDiskRequest{{Name: "vda"}}, config); err == nil {
		t.Fatal("expected first disk deletion to be rejected")
	}
}

func TestValidateVMDeviceDeletedDisksAllowsNonFirstDisk(t *testing.T) {
	config := VMConfig{Disks: []VMConfigDisk{
		{Name: "vda", Path: "/storage/vda.qcow2", Device: "disk"},
		{Name: "vdb", Path: "/storage/vdb.qcow2", Device: "disk"},
		{Name: "sda", Path: "/iso/os.iso", Device: "cdrom"},
	}}
	items, err := validateVMDeviceDeletedDisks([]VMDeviceDeleteDiskRequest{{Name: "vdb"}}, config)
	if err != nil {
		t.Fatalf("expected non-first disk deletion to be valid, got %v", err)
	}
	if len(items) != 1 || items[0].Target != "vdb" {
		t.Fatalf("unexpected delete actions: %+v", items)
	}
}

func TestUpdateDomainDeviceXMLSwitchesInterfaceToBridge(t *testing.T) {
	input := `<domain type="kvm">
  <name>demo</name>
  <devices>
    <interface type="network">
      <mac address="52:54:00:aa:bb:cc"></mac>
      <source network="default"></source>
    </interface>
  </devices>
</domain>`
	output, err := updateDomainDeviceXML(
		input,
		map[string]domainInterfaceSourceUpdate{
			"52:54:00:aa:bb:cc": {Type: "bridge", SourceAttr: "bridge", Source: "brvlan244"},
		},
		nil,
		nil,
		nil,
		nil,
	)
	if err != nil {
		t.Fatalf("update domain device xml failed: %v", err)
	}
	for _, expected := range []string{`type="bridge"`, `bridge="brvlan244"`} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected device xml to contain %s, got %s", expected, output)
		}
	}
	for _, unexpected := range []string{`type="network"`, `network="default"`} {
		if strings.Contains(output, unexpected) {
			t.Fatalf("expected device xml to remove %s, got %s", unexpected, output)
		}
	}
}

func TestUpdateDomainDeviceXMLKeepsGraphicsPassword(t *testing.T) {
	input := `<domain type="kvm">
  <name>demo</name>
  <devices>
    <interface type="network">
      <mac address="52:54:00:aa:bb:cc"></mac>
      <source network="default"></source>
    </interface>
    <graphics type="vnc" port="-1" passwd="secret"></graphics>
  </devices>
</domain>`
	output, err := updateDomainDeviceXML(
		input,
		map[string]domainInterfaceSourceUpdate{
			"52:54:00:aa:bb:cc": {Type: "network", SourceAttr: "network", Source: "isolated"},
		},
		nil,
		nil,
		nil,
		nil,
	)
	if err != nil {
		t.Fatalf("update domain device xml failed: %v", err)
	}
	if !strings.Contains(output, `passwd="secret"`) {
		t.Fatalf("expected graphics password to be preserved, got %s", output)
	}
}

func TestPreserveSecurityGraphicsPasswordForXMLUpdate(t *testing.T) {
	input := `<domain type="kvm">
  <name>demo</name>
  <devices>
    <graphics type="vnc" port="-1"></graphics>
  </devices>
</domain>`
	securityInput := `<domain type="kvm">
  <name>demo</name>
  <devices>
    <graphics type="vnc" port="-1" passwd="secret"></graphics>
  </devices>
</domain>`
	output, err := preserveSecurityGraphicsPassword(input, securityInput)
	if err != nil {
		t.Fatalf("preserve graphics password failed: %v", err)
	}
	if !strings.Contains(output, `passwd="secret"`) {
		t.Fatalf("expected graphics password to be preserved, got %s", output)
	}
}

func TestPreserveSecurityGraphicsPasswordKeepsExplicitPassword(t *testing.T) {
	input := `<domain type="kvm">
  <name>demo</name>
  <devices>
    <graphics type="vnc" port="-1" passwd="next"></graphics>
  </devices>
</domain>`
	securityInput := `<domain type="kvm">
  <name>demo</name>
  <devices>
    <graphics type="vnc" port="-1" passwd="secret"></graphics>
  </devices>
</domain>`
	output, err := preserveSecurityGraphicsPassword(input, securityInput)
	if err != nil {
		t.Fatalf("preserve graphics password failed: %v", err)
	}
	if !strings.Contains(output, `passwd="next"`) || strings.Contains(output, `passwd="secret"`) {
		t.Fatalf("expected explicit graphics password to be kept, got %s", output)
	}
}

func TestUpdateDomainDeviceXMLAddsAndDeletesDevices(t *testing.T) {
	input := `<domain type="kvm">
  <name>demo</name>
  <devices>
    <disk type="file" device="disk">
      <source file="/storage/vm/source.qcow2"></source>
      <target dev="vda" bus="virtio"></target>
    </disk>
    <disk type="file" device="disk">
      <source file="/storage/vm/old.qcow2"></source>
      <target dev="vdb" bus="virtio"></target>
    </disk>
    <interface type="network">
      <mac address="52:54:00:aa:bb:cc"></mac>
      <source network="default"></source>
      <target dev="vnet1"></target>
    </interface>
  </devices>
</domain>`
	output, err := updateDomainDeviceXML(
		input,
		nil,
		[]domainInterfaceXMLFragment{{Type: "network", SourceAttr: "network", Source: "isolated", Model: "virtio"}},
		map[string]bool{"52:54:00:aa:bb:cc": true},
		nil,
		[]vmDiskDeleteAction{{Target: "vdb", Path: "/storage/vm/old.qcow2"}},
	)
	if err != nil {
		t.Fatalf("update domain device xml failed: %v", err)
	}
	for _, expected := range []string{`file="/storage/vm/source.qcow2"`, `network="isolated"`, `type="virtio"`} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected device xml to contain %s, got %s", expected, output)
		}
	}
	for _, unexpected := range []string{`/storage/vm/old.qcow2`, `52:54:00:aa:bb:cc`, `network="default"`} {
		if strings.Contains(output, unexpected) {
			t.Fatalf("expected device xml to remove %s, got %s", unexpected, output)
		}
	}
}

func TestParseDomifaddrIPSkipsLoopbackAndLinkLocal(t *testing.T) {
	out := ` Name       MAC address          Protocol     Address
-------------------------------------------------------------------------------
 vnet0      52:54:00:11:22:33    ipv4         127.0.0.1/8
 vnet1      52:54:00:11:22:34    ipv4         169.254.10.20/16
 vnet2      52:54:00:11:22:35    ipv4         10.22.12.179/24`

	if got := parseDomifaddrIP(out); got != "10.22.12.179" {
		t.Fatalf("expected usable ip, got %q", got)
	}
}

func TestMemoryStatsPeriodFromXML(t *testing.T) {
	input := `<domain type="kvm">
  <devices>
    <memballoon model="virtio">
      <stats period="5"></stats>
    </memballoon>
  </devices>
</domain>`

	if got := memoryStatsPeriodFromXML(input); got != 5 {
		t.Fatalf("expected memory stats period 5, got %d", got)
	}
}

func TestVMMemoryUsageFallsBackToAvailable(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "virsh")
	content := `#!/bin/sh
if [ "$3" = "dommemstat" ]; then
  echo "actual 4096"
  echo "available 1024"
  exit 0
fi
exit 0
`
	perm := os.FileMode(0o755)
	if runtime.GOOS == "windows" {
		script += ".bat"
		content = `@echo off
if "%3"=="dommemstat" goto dommemstat
exit /b 0
:dommemstat
echo actual 4096
echo available 1024
exit /b 0
`
		perm = 0o644
	}
	if err := os.WriteFile(script, []byte(content), perm); err != nil {
		t.Fatalf("write virsh stub: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	usage, ok := NewVirshProvider("qemu:///system", 5*time.Second).vmMemoryUsage("demo", 4096*1024, "running")
	if usage != 75 || !ok {
		t.Fatalf("expected memory usage from available fallback, got usage=%d available=%t", usage, ok)
	}
}

func TestVMMemoryUsageUnavailableWithoutUsableOrAvailable(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "virsh")
	content := `#!/bin/sh
if [ "$3" = "dommemstat" ]; then
  echo "actual 4096"
  exit 0
fi
exit 0
`
	perm := os.FileMode(0o755)
	if runtime.GOOS == "windows" {
		script += ".bat"
		content = `@echo off
if "%3"=="dommemstat" goto dommemstat
exit /b 0
:dommemstat
echo actual 4096
exit /b 0
`
		perm = 0o644
	}
	if err := os.WriteFile(script, []byte(content), perm); err != nil {
		t.Fatalf("write virsh stub: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	usage, ok := NewVirshProvider("qemu:///system", 5*time.Second).vmMemoryUsage("demo", 4096*1024, "running")
	if usage != 0 || ok {
		t.Fatalf("expected unavailable memory usage, got usage=%d available=%t", usage, ok)
	}
}
