package kvm

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestParseVirtDFCSV(t *testing.T) {
	out := `VirtualMachine,Filesystem,1K-blocks,Used,Available,Use%
192.168.12.175,/dev/sda2,1038336,120984,917352,12%
192.168.12.175,/dev/sys/root,38774276,3695440,35078836,10%
192.168.12.175,/dev/vgdata/data,103076792,15043656,82774080,15%
`
	items := parseVirtDFCSV(out)
	if len(items) != 3 {
		t.Fatalf("expected 3 filesystems, got %d", len(items))
	}
	if items[2].Name != "/dev/vgdata/data" {
		t.Fatalf("unexpected filesystem name: %q", items[2].Name)
	}
	if items[2].VMName != "192.168.12.175" {
		t.Fatalf("unexpected vm name: %q", items[2].VMName)
	}
	if items[2].UsedBytes != 15043656*kibibyte {
		t.Fatalf("unexpected used bytes: %d", items[2].UsedBytes)
	}
}

func TestResolveFilesystemDeviceForPartitionsAndLVM(t *testing.T) {
	nodes := parseVirtFilesystemsCSV(virtFilesystemSampleCSV)

	device, ok := resolveFilesystemDevice("/dev/sda2", nodes)
	if !ok || device != "/dev/sda" {
		t.Fatalf("expected /dev/sda2 to resolve to /dev/sda, got %q ok=%t", device, ok)
	}

	device, ok = resolveFilesystemDevice("/dev/sys/root", nodes)
	if !ok || device != "/dev/sda" {
		t.Fatalf("expected /dev/sys/root to resolve to /dev/sda, got %q ok=%t", device, ok)
	}

	device, ok = resolveFilesystemDevice("/dev/vgdata/data", nodes)
	if !ok || device != "/dev/sdb" {
		t.Fatalf("expected /dev/vgdata/data to resolve to /dev/sdb, got %q ok=%t", device, ok)
	}
}

func TestGuestFilesystemUsageByDiskUsesVirtDFAndDomblkinfoCapacity(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell stub uses POSIX argument handling")
	}
	dir := t.TempDir()
	writeExecutable(t, filepath.Join(dir, "virt-df"), `#!/bin/sh
cat <<'CSV'
VirtualMachine,Filesystem,1K-blocks,Used,Available,Use%
demo,/dev/sda2,1038336,120984,917352,12%
demo,/dev/sys/root,38774276,3695440,35078836,10%
demo,/dev/vgdata/data,103076792,15043656,82774080,15%
CSV
`)
	writeExecutable(t, filepath.Join(dir, "virt-filesystems"), `#!/bin/sh
cat <<'CSV'
`+virtFilesystemSampleCSV+`CSV
`)
	writeExecutable(t, filepath.Join(dir, "virsh"), `#!/bin/sh
if [ "$3" = "domblkinfo" ] && [ "$5" = "/kvm/demo-root.qcow2" ]; then
  echo "Capacity: 42949672960"
  exit 0
fi
if [ "$3" = "domblkinfo" ] && [ "$5" = "/kvm/demo-data.qcow2" ]; then
  echo "Capacity: 107374182400"
  exit 0
fi
exit 1
`)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	doc := domainXML{}
	doc.Devices.Disks = []domainDiskXML{
		{Device: "disk", Source: domainDiskSourceXML{File: "/kvm/demo-root.qcow2"}},
		{Device: "disk", Source: domainDiskSourceXML{File: "/kvm/demo-data.qcow2"}},
	}
	doc.Devices.Disks[0].Target.Dev = "vda"
	doc.Devices.Disks[1].Target.Dev = "vdb"

	disks, total, used, available := NewVirshProvider("qemu:///system", 5*time.Second).diskDetails("demo", doc, false, nil)
	if !available {
		t.Fatal("expected disk usage to be available")
	}
	if total != 150323855360 {
		t.Fatalf("unexpected total bytes: %d", total)
	}
	expectedVDAUsed := (int64(120984) + int64(3695440)) * kibibyte
	expectedVDBUsed := int64(15043656) * kibibyte
	if used != expectedVDAUsed+expectedVDBUsed {
		t.Fatalf("unexpected used bytes: %d", used)
	}
	if len(disks) != 2 || disks[0].UsedBytes != expectedVDAUsed || disks[1].UsedBytes != expectedVDBUsed {
		t.Fatalf("unexpected disk usage: %+v", disks)
	}
}

func TestGuestFilesystemUsagesByVMMatchesVirtualMachineColumn(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell stub uses POSIX argument handling")
	}
	dir := t.TempDir()
	writeExecutable(t, filepath.Join(dir, "virt-df"), `#!/bin/sh
cat <<'CSV'
VirtualMachine,Filesystem,1K-blocks,Used,Available,Use%
192.168.12.175,/dev/sda2,1038336,120984,917352,11.7
192.168.12.176,/dev/sda2,1038336,120984,917352,11.7
ct7-template,/dev/sys/root,38774276,1690544,37083732,4.4
CSV
`)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	items := NewVirshProvider("qemu:///system", 5*time.Second).guestFilesystemUsagesByVM([]string{"192.168.12.175", "ct7-template"})
	if len(items) != 2 {
		t.Fatalf("expected two matching VMs, got %#v", items)
	}
	if got := items["192.168.12.175"][0].Name; got != "/dev/sda2" {
		t.Fatalf("unexpected filesystem for ip vm: %q", got)
	}
	if got := items["ct7-template"][0].UsedBytes; got != int64(1690544)*kibibyte {
		t.Fatalf("unexpected used bytes for template: %d", got)
	}
	if _, ok := items["192.168.12.176"]; ok {
		t.Fatal("did not expect unmatched VM to be included")
	}
}

func TestDiskDetailsSetsUsedZeroWhenVirtDFFails(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell stub uses POSIX argument handling")
	}
	dir := t.TempDir()
	writeExecutable(t, filepath.Join(dir, "virt-df"), `#!/bin/sh
exit 1
`)
	writeExecutable(t, filepath.Join(dir, "virsh"), `#!/bin/sh
if [ "$3" = "domblkinfo" ]; then
  echo "Capacity: 107374182400"
  echo "Allocation: 99999999999"
  exit 0
fi
exit 1
`)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	doc := domainXML{}
	doc.Devices.Disks = []domainDiskXML{{Device: "disk", Source: domainDiskSourceXML{File: "/kvm/demo.qcow2"}}}
	doc.Devices.Disks[0].Target.Dev = "vda"

	disks, total, used, available := NewVirshProvider("qemu:///system", 5*time.Second).diskDetails("demo", doc, false, nil)
	if !available || total != 107374182400 || used != 0 {
		t.Fatalf("unexpected totals: total=%d used=%d available=%t", total, used, available)
	}
	if len(disks) != 1 || disks[0].UsedBytes != 0 {
		t.Fatalf("expected used bytes to stay zero without virt-df, got %+v", disks)
	}
}

func writeExecutable(t *testing.T, name string, content string) {
	t.Helper()
	if err := os.WriteFile(name, []byte(content), 0o755); err != nil {
		t.Fatalf("write stub %s: %v", name, err)
	}
}

const virtFilesystemSampleCSV = `Name,Type,VFS,Label,MBR,Size,Parent
/dev/sda1,filesystem,unknown,-,-,2097152,-
/dev/sda2,filesystem,xfs,-,-,1073741824,-
/dev/sys/root,filesystem,xfs,-,-,39724253184,-
/dev/sys/swap,filesystem,swap,-,-,2147483648,-
/dev/vgdata/data,filesystem,ext4,-,-,107369988096,-
/dev/sys/root,lv,-,-,-,39724253184,/dev/sys
/dev/sys/swap,lv,-,-,-,2147483648,/dev/sys
/dev/vgdata/data,lv,-,-,-,107369988096,/dev/vgdata
/dev/sys,vg,-,-,-,41871736832,/dev/sda3
/dev/vgdata,vg,-,-,-,107369988096,/dev/sda3
/dev/sda3,pv,-,-,-,41871736832,-
/dev/sdb,pv,-,-,-,107369988096,-
/dev/sda1,partition,-,-,83,2097152,/dev/sda
/dev/sda2,partition,-,-,83,1073741824,/dev/sda
/dev/sda3,partition,-,-,8e,41872785408,/dev/sda
/dev/sda,device,-,-,-,42949672960,-
/dev/sdb,device,-,-,-,107374182400,-
`
