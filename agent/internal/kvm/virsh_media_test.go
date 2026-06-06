package kvm

import (
	"strings"
	"testing"
)

func TestMediaChangeArgsUsesLiveConfigForRunningVM(t *testing.T) {
	args := mediaChangeArgs("qemu:///system", "vm01", []string{"sda", "/iso/CentOS-7.iso", "--insert"}, "running")
	expectedPrimary := "--connect qemu:///system change-media vm01 sda /iso/CentOS-7.iso --insert --live --config"
	if strings.Join(args.primary, " ") != expectedPrimary {
		t.Fatalf("unexpected primary args: %v", args.primary)
	}
	if len(args.fallback) != 0 {
		t.Fatalf("expected no fallback args, got %v", args.fallback)
	}
}

func TestMediaChangeArgsSkipsFallbackForStoppedVM(t *testing.T) {
	args := mediaChangeArgs("qemu:///system", "vm01", []string{"sda", "--eject"}, "stopped")
	expectedPrimary := "--connect qemu:///system change-media vm01 sda --eject --config"
	if strings.Join(args.primary, " ") != expectedPrimary {
		t.Fatalf("unexpected primary args: %v", args.primary)
	}
	if len(args.fallback) != 0 {
		t.Fatalf("expected no fallback args, got %v", args.fallback)
	}
}

func TestUpdateCDROMBootOrderXMLConnectsMediaFirst(t *testing.T) {
	input := `<domain type="kvm"><devices><disk type="file" device="disk"><target dev="vda" bus="virtio"/><boot order="1"/></disk><disk type="file" device="disk"><target dev="vdb" bus="virtio"/><boot order="2"/></disk><disk type="file" device="cdrom"><target dev="sda" bus="sata"/></disk></devices></domain>`
	output, err := updateCDROMBootOrderXML(input, "sda", true)
	if err != nil {
		t.Fatalf("expected boot order update, got %v", err)
	}
	if !strings.Contains(output, `<target dev="vda" bus="virtio"></target><boot order="2"></boot>`) {
		t.Fatalf("expected first disk to boot second, got %s", output)
	}
	if strings.Contains(output, `dev="vdb" bus="virtio"></target><boot`) {
		t.Fatalf("expected secondary disk boot order to be removed, got %s", output)
	}
	if !strings.Contains(output, `<target dev="sda" bus="sata"></target><boot order="1"></boot>`) {
		t.Fatalf("expected cdrom to boot first, got %s", output)
	}
}

func TestUpdateCDROMBootOrderXMLDisconnectRestoresDiskFirst(t *testing.T) {
	input := `<domain type="kvm"><devices><disk type="file" device="disk"><target dev="vda" bus="virtio"/><boot order="2"/></disk><disk type="file" device="cdrom"><target dev="sda" bus="sata"/><boot order="1"/></disk></devices></domain>`
	output, err := updateCDROMBootOrderXML(input, "sda", false)
	if err != nil {
		t.Fatalf("expected boot order update, got %v", err)
	}
	if !strings.Contains(output, `<target dev="vda" bus="virtio"></target><boot order="1"></boot>`) {
		t.Fatalf("expected first disk to boot first, got %s", output)
	}
	if strings.Contains(output, `device="cdrom"><target dev="sda" bus="sata"></target><boot`) {
		t.Fatalf("expected cdrom boot order to be removed, got %s", output)
	}
}

func TestUpdateCDROMBootOrderXMLUsesOSBootWhenPresent(t *testing.T) {
	input := `<domain type="kvm"><os><type arch="x86_64">hvm</type><boot dev="hd"/></os><devices><disk type="file" device="disk"><target dev="vda" bus="virtio"/></disk><disk type="file" device="cdrom"><target dev="sda" bus="sata"/></disk></devices></domain>`
	output, err := updateCDROMBootOrderXML(input, "sda", true)
	if err != nil {
		t.Fatalf("expected os boot order update, got %v", err)
	}
	if !strings.Contains(output, `<boot dev="cdrom"></boot><boot dev="hd"></boot>`) {
		t.Fatalf("expected os cdrom and hd boot order, got %s", output)
	}
	if strings.Contains(output, `order=`) {
		t.Fatalf("expected no per-device boot order when os boot is present, got %s", output)
	}
}

func TestUpdateCDROMBootOrderXMLDisconnectUsesOSBootWhenPresent(t *testing.T) {
	input := `<domain type="kvm"><os><type arch="x86_64">hvm</type><boot dev="cdrom"/><boot dev="hd"/></os><devices><disk type="file" device="disk"><target dev="vda" bus="virtio"/></disk><disk type="file" device="cdrom"><target dev="sda" bus="sata"/></disk></devices></domain>`
	output, err := updateCDROMBootOrderXML(input, "sda", false)
	if err != nil {
		t.Fatalf("expected os boot order update, got %v", err)
	}
	if strings.Contains(output, `dev="cdrom"`) {
		t.Fatalf("expected cdrom os boot to be removed, got %s", output)
	}
	if !strings.Contains(output, `<boot dev="hd"></boot>`) {
		t.Fatalf("expected hd os boot order, got %s", output)
	}
}

func TestUpdateCDROMBootOrderXMLKeepsGraphicsPassword(t *testing.T) {
	input := `<domain type="kvm"><os><type arch="x86_64">hvm</type><boot dev="hd"/></os><devices><disk type="file" device="disk"><target dev="vda" bus="virtio"/></disk><disk type="file" device="cdrom"><target dev="sda" bus="sata"/></disk><graphics type="vnc" port="-1" passwd="secret"/></devices></domain>`
	output, err := updateCDROMBootOrderXML(input, "sda", true)
	if err != nil {
		t.Fatalf("expected boot order update, got %v", err)
	}
	if !strings.Contains(output, `passwd="secret"`) {
		t.Fatalf("expected graphics password to be preserved, got %s", output)
	}
}
