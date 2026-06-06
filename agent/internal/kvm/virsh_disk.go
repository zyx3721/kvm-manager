package kvm

import (
	"strconv"
	"strings"
)

func (p *VirshProvider) diskBytes(doc domainXML) int64 {
	_, total, _, _ := p.diskDetails("", doc, false, nil)
	return total
}

func (p *VirshProvider) diskDetails(vmName string, doc domainXML, fast bool, filesystems []guestFilesystemUsage) ([]VMDisk, int64, int64, bool) {
	disks := make([]VMDisk, 0)
	var total int64
	var used int64
	usageByDisk := map[string]int64{}
	if !fast && strings.TrimSpace(vmName) != "" {
		usageByDisk = p.guestFilesystemUsageByDisk(vmName, doc, filesystems)
	}
	for _, disk := range doc.Devices.Disks {
		if disk.Device != "disk" {
			continue
		}
		path := disk.Source.File
		if path == "" {
			path = disk.Source.Dev
		}
		if path == "" {
			continue
		}
		item := VMDisk{Name: disk.Target.Dev, Path: path}
		if !fast && vmName != "" && item.Name != "" {
			if out, err := p.output("virsh", "--connect", p.libvirtURI, "domblkinfo", vmName, path); err == nil {
				item.Bytes = parseDomblkInfoBytes(out, "Capacity:")
			}
			item.UsedBytes = usageByDisk[item.Name]
		}
		if item.Bytes > 0 {
			total += item.Bytes
			used += item.UsedBytes
			disks = append(disks, item)
		}
	}
	return disks, total, used, total > 0
}

func parseDomblkInfoBytes(info string, prefix string) int64 {
	for _, line := range strings.Split(info, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, prefix) {
			return parseLibvirtSize(strings.TrimSpace(strings.TrimPrefix(line, prefix)))
		}
	}
	return 0
}

func parseLibvirtSize(value string) int64 {
	fields := strings.Fields(value)
	if len(fields) == 0 {
		return 0
	}
	number, err := strconv.ParseFloat(fields[0], 64)
	if err != nil || number <= 0 {
		return 0
	}
	unit := "B"
	if len(fields) > 1 {
		unit = strings.ToLower(strings.TrimSpace(fields[1]))
	}
	multiplier := float64(1)
	switch unit {
	case "b", "byte", "bytes":
		multiplier = 1
	case "kib":
		multiplier = 1024
	case "mib":
		multiplier = 1024 * 1024
	case "gib":
		multiplier = 1024 * 1024 * 1024
	case "tib":
		multiplier = 1024 * 1024 * 1024 * 1024
	case "kb", "k":
		multiplier = 1000
	case "mb", "m":
		multiplier = 1000 * 1000
	case "gb", "g":
		multiplier = 1000 * 1000 * 1000
	case "tb", "t":
		multiplier = 1000 * 1000 * 1000 * 1000
	}
	return int64(number * multiplier)
}
