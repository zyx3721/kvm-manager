package kvm

import (
	"encoding/xml"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func (p *VirshProvider) HostInfo() (HostInfo, error) {
	hostname, _ := p.output("hostname")
	hostAddress := p.hostAddress()
	version, _ := p.output("virsh", "--connect", p.libvirtURI, "version")
	info, err := p.output("virsh", "--connect", p.libvirtURI, "nodeinfo")
	if err != nil {
		return HostInfo{}, err
	}
	memoryBytes := parseNodeMemory(info)
	storageBytes, storageUsage := p.storageUsage()
	hostIO := p.hostIORates()
	return HostInfo{
		Hostname:             strings.TrimSpace(hostname),
		HostAddress:          hostAddress,
		Status:               "online",
		KVMVersion:           firstLine(version),
		KVMFullVersion:       strings.TrimSpace(version),
		CPUModel:             parseNodeText(info, "CPU model:"),
		CPUCores:             parseNodeCPU(info),
		CPUUsage:             p.hostCPUUsage(),
		MemoryBytes:          memoryBytes,
		MemoryUsage:          p.hostMemoryUsage(memoryBytes),
		StorageBytes:         storageBytes,
		StorageUsage:         storageUsage,
		DiskReadBytesPerSec:  hostIO.diskReadBytesPerSec,
		DiskWriteBytesPerSec: hostIO.diskWriteBytesPerSec,
		NetworkRxBytesPerSec: hostIO.networkRxBytesPerSec,
		NetworkTxBytesPerSec: hostIO.networkTxBytesPerSec,
		Capabilities:         []string{"host.info", "host.interfaces.read", "host.interfaces.manage", "vm.list", "vm.console", "vm.start", "vm.shutdown", "vm.suspend", "vm.resume", "vm.reboot", "vm.reset", "vm.destroy", "vm.delete", "snapshot.list", "snapshot.create", "snapshot.revert", "snapshot.delete"},
	}, nil
}

func (p *VirshProvider) hostAddress() string {
	if out, err := p.output("sh", "-c", "ip -4 route get 1.1.1.1 | awk '{for(i=1;i<=NF;i++) if ($i==\"src\") {print $(i+1); exit}}'"); err == nil {
		if ip := strings.TrimSpace(out); isUsableIPv4(ip) {
			return ip
		}
	}
	if out, err := p.output("sh", "-c", "ip -o -4 addr show scope global | awk '{print $4}' | cut -d/ -f1"); err == nil {
		for _, field := range strings.Fields(out) {
			if isUsableIPv4(field) {
				return field
			}
		}
	}
	if out, err := p.output("hostname", "-I"); err == nil {
		for _, field := range strings.Fields(out) {
			if isUsableIPv4(field) {
				return field
			}
		}
	}
	return ""
}

func isUsableIPv4(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || strings.HasPrefix(value, "127.") || strings.HasPrefix(value, "169.254.") {
		return false
	}
	parts := strings.Split(value, ".")
	if len(parts) != 4 {
		return false
	}
	for _, part := range parts {
		parsed, err := strconv.Atoi(part)
		if err != nil || parsed < 0 || parsed > 255 {
			return false
		}
	}
	return true
}

func (p *VirshProvider) hostCPUUsage() int {
	out1, err1 := p.output("sh", "-c", "awk '/^cpu / {print $2+$3+$4+$5+$6+$7+$8, $5}' /proc/stat")
	if err1 != nil {
		return 0
	}
	time.Sleep(time.Second)
	out2, err2 := p.output("sh", "-c", "awk '/^cpu / {print $2+$3+$4+$5+$6+$7+$8, $5}' /proc/stat")
	if err2 != nil {
		return 0
	}
	t1, i1 := parseTwoFloats(out1)
	t2, i2 := parseTwoFloats(out2)
	if t2 <= t1 {
		return 0
	}
	return clampPercent(int(((t2 - t1) - (i2 - i1)) * 100 / (t2 - t1)))
}

func (p *VirshProvider) hostMemoryUsage(totalBytes int64) int {
	out, err := p.output("sh", "-c", "awk '/MemTotal:/ {t=$2} /MemAvailable:/ {a=$2} END {if (t>0) print t, a}' /proc/meminfo")
	if err != nil {
		return 0
	}
	totalKiB, availableKiB := parseTwoFloats(out)
	if totalKiB <= 0 || availableKiB < 0 {
		if totalBytes <= 0 {
			return 0
		}
		return 0
	}
	return clampPercent(int((totalKiB - availableKiB) * 100 / totalKiB))
}

func (p *VirshProvider) storageUsage() (int64, int) {
	out, err := p.output("df", "-PB1", "--local")
	if err != nil {
		return 0, 0
	}
	total, used := summarizeLocalDiskFilesystems(parseDFStorageRows(out))
	if total <= 0 {
		return 0, 0
	}
	return total, clampPercent(int(used * 100 / total))
}

type dfStorageRow struct {
	Filesystem string
	Total      int64
	Used       int64
	MountedOn  string
}

func parseDFStorageRows(output string) []dfStorageRow {
	items := make([]dfStorageRow, 0)
	for index, line := range strings.Split(strings.TrimSpace(output), "\n") {
		if index == 0 || strings.TrimSpace(line) == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 6 {
			continue
		}
		total, _ := strconv.ParseInt(fields[1], 10, 64)
		used, _ := strconv.ParseInt(fields[2], 10, 64)
		items = append(items, dfStorageRow{
			Filesystem: fields[0],
			Total:      total,
			Used:       used,
			MountedOn:  fields[len(fields)-1],
		})
	}
	return items
}

func summarizeLocalDiskFilesystems(items []dfStorageRow) (int64, int64) {
	total := int64(0)
	used := int64(0)
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		if item.Total <= 0 || !isHostDiskFilesystem(item.Filesystem, item.MountedOn) {
			continue
		}
		source := hostDiskFilesystemSource(item)
		if _, ok := seen[source]; ok {
			continue
		}
		seen[source] = struct{}{}
		total += item.Total
		used += item.Used
	}
	return total, used
}

func isHostDiskFilesystem(filesystem string, mountedOn string) bool {
	fs := strings.TrimSpace(filesystem)
	mount := strings.TrimSpace(mountedOn)
	if fs == "" || mount == "" {
		return false
	}
	if strings.HasPrefix(fs, "/dev/") || strings.HasPrefix(fs, "UUID=") || strings.HasPrefix(fs, "LABEL=") {
		return true
	}
	if strings.HasPrefix(fs, "tmpfs") || strings.HasPrefix(fs, "devtmpfs") || strings.HasPrefix(fs, "overlay") {
		return false
	}
	switch fs {
	case "udev", "shm", "run", "none", "proc", "sysfs", "cgroup", "cgroup2", "devpts", "securityfs", "debugfs", "tracefs", "fusectl", "configfs", "efivarfs", "mqueue", "hugetlbfs", "pstore", "binfmt_misc", "autofs", "nsfs", "rpc_pipefs":
		return false
	default:
		return false
	}
}

func hostDiskFilesystemSource(item dfStorageRow) string {
	fs := strings.TrimSpace(item.Filesystem)
	if fs != "" {
		return fs
	}
	return "mount:" + cleanPoolPath(item.MountedOn)
}

func parsePoolBytes(info string, prefix string) int64 {
	for _, line := range strings.Split(info, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, prefix) {
			fields := strings.Fields(strings.TrimSpace(strings.TrimPrefix(line, prefix)))
			if len(fields) > 0 {
				value, _ := strconv.ParseFloat(fields[0], 64)
				return int64(value)
			}
		}
	}
	return 0
}

func parsePoolPath(value string) string {
	type poolXML struct {
		Target struct {
			Path string `xml:"path"`
		} `xml:"target"`
	}
	var doc poolXML
	if err := xml.Unmarshal([]byte(value), &doc); err != nil {
		return ""
	}
	if doc.Target.Path == "" {
		return ""
	}
	return filepath.Clean(doc.Target.Path)
}
