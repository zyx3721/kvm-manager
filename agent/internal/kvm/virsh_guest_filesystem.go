package kvm

import (
	"encoding/csv"
	"log/slog"
	"path"
	"sort"
	"strconv"
	"strings"
)

const kibibyte = int64(1024)

type guestFilesystemUsage struct {
	VMName    string
	Name      string
	UsedBytes int64
}

type guestFilesystemNode struct {
	Name   string
	Type   string
	Parent string
	Size   int64
}

func (p *VirshProvider) guestFilesystemUsagesByVM(names []string) map[string][]guestFilesystemUsage {
	if len(names) == 0 {
		return map[string][]guestFilesystemUsage{}
	}
	out, err := p.output("virt-df", "--csv")
	if err != nil {
		p.logGuestFilesystemWarning("collect guest filesystem usage list failed", "", err)
		return map[string][]guestFilesystemUsage{}
	}
	items := parseVirtDFCSV(out)
	result := make(map[string][]guestFilesystemUsage)
	wanted := make(map[string]bool, len(names))
	for _, name := range names {
		wanted[strings.TrimSpace(name)] = true
	}
	for _, item := range items {
		if !wanted[item.VMName] {
			continue
		}
		result[item.VMName] = append(result[item.VMName], item)
	}
	if len(result) == 0 {
		p.logGuestFilesystemWarning("collect guest filesystem usage list returned no matching filesystems", "", nil)
	}
	return result
}

func (p *VirshProvider) guestFilesystemUsageByDisk(vmName string, doc domainXML, filesystems []guestFilesystemUsage) map[string]int64 {
	targets := diskTargetsByGuestDevice(doc)
	if len(targets) == 0 {
		return map[string]int64{}
	}
	if len(filesystems) == 0 {
		dfOut, err := p.output("virt-df", "--csv", "-d", vmName)
		if err != nil {
			p.logGuestFilesystemWarning("collect guest filesystem usage failed", vmName, err)
			return map[string]int64{}
		}
		filesystems = parseVirtDFCSV(dfOut)
		if len(filesystems) == 0 {
			p.logGuestFilesystemWarning("collect guest filesystem usage returned no filesystems", vmName, nil)
			return map[string]int64{}
		}
	}
	fsOut, err := p.output("virt-filesystems", "--csv", "-d", vmName, "--all", "--long")
	if err != nil {
		p.logGuestFilesystemWarning("collect guest filesystem topology failed", vmName, err)
		return map[string]int64{}
	}
	nodes := parseVirtFilesystemsCSV(fsOut)
	if len(nodes) == 0 {
		p.logGuestFilesystemWarning("collect guest filesystem topology returned no nodes", vmName, nil)
		return map[string]int64{}
	}
	usageByDisk := make(map[string]int64)
	for _, filesystem := range filesystems {
		device, ok := resolveFilesystemDevice(filesystem.Name, nodes)
		if !ok {
			p.logGuestFilesystemWarning("guest filesystem could not be mapped to a disk", vmName, nil, slog.String("filesystem", filesystem.Name))
			continue
		}
		target, ok := targets[device]
		if !ok || target == "" {
			p.logGuestFilesystemWarning("guest filesystem device has no matching domain disk", vmName, nil, slog.String("filesystem", filesystem.Name), slog.String("device", device))
			continue
		}
		usageByDisk[target] += filesystem.UsedBytes
	}
	if len(usageByDisk) == 0 {
		p.logGuestFilesystemWarning("guest filesystem usage could not be mapped to any disk", vmName, nil)
	}
	return usageByDisk
}

func parseVirtDFCSV(value string) []guestFilesystemUsage {
	rows, err := csv.NewReader(strings.NewReader(value)).ReadAll()
	if err != nil || len(rows) < 2 {
		return []guestFilesystemUsage{}
	}
	header := csvHeaderIndexes(rows[0])
	filesystemIndex := firstCSVIndex(header, "filesystem")
	usedIndex := firstCSVIndex(header, "used", "used 1k-blocks")
	if filesystemIndex < 0 || usedIndex < 0 {
		return []guestFilesystemUsage{}
	}
	items := make([]guestFilesystemUsage, 0, len(rows)-1)
	for _, row := range rows[1:] {
		if filesystemIndex >= len(row) || usedIndex >= len(row) {
			continue
		}
		vmName, name := splitVirtFilesystemName(row[filesystemIndex])
		usedBlocks, err := strconv.ParseInt(strings.TrimSpace(row[usedIndex]), 10, 64)
		if name == "" || err != nil || usedBlocks < 0 {
			continue
		}
		items = append(items, guestFilesystemUsage{VMName: vmName, Name: name, UsedBytes: usedBlocks * kibibyte})
	}
	return items
}

func parseVirtFilesystemsCSV(value string) map[string]guestFilesystemNode {
	rows, err := csv.NewReader(strings.NewReader(value)).ReadAll()
	if err != nil || len(rows) < 2 {
		return map[string]guestFilesystemNode{}
	}
	header := csvHeaderIndexes(rows[0])
	nameIndex := firstCSVIndex(header, "name")
	typeIndex := firstCSVIndex(header, "type")
	parentIndex := firstCSVIndex(header, "parent")
	sizeIndex := firstCSVIndex(header, "size")
	if nameIndex < 0 || typeIndex < 0 || parentIndex < 0 || sizeIndex < 0 {
		return map[string]guestFilesystemNode{}
	}
	items := make(map[string]guestFilesystemNode)
	for _, row := range rows[1:] {
		if nameIndex >= len(row) || typeIndex >= len(row) || parentIndex >= len(row) || sizeIndex >= len(row) {
			continue
		}
		name := normalizeGuestDevice(row[nameIndex])
		if name == "" {
			continue
		}
		parent := normalizeGuestDevice(row[parentIndex])
		size, _ := strconv.ParseInt(strings.TrimSpace(row[sizeIndex]), 10, 64)
		node := guestFilesystemNode{
			Name:   name,
			Type:   strings.ToLower(strings.TrimSpace(row[typeIndex])),
			Parent: parent,
			Size:   size,
		}
		if existing, ok := items[name]; ok && filesystemNodePriority(existing.Type) > filesystemNodePriority(node.Type) {
			continue
		}
		items[name] = node
	}
	return items
}

func resolveFilesystemDevice(name string, nodes map[string]guestFilesystemNode) (string, bool) {
	name = normalizeVirtFilesystemName(name)
	if name == "" {
		return "", false
	}
	node, ok := nodes[name]
	if !ok {
		return directGuestDevice(name)
	}
	if node.Type == "filesystem" {
		if device, ok := resolveFilesystemDeviceParent(node.Name, nodes); ok {
			return device, true
		}
	}
	return resolveNodeDevice(node.Name, nodes, map[string]bool{})
}

func resolveFilesystemDeviceParent(name string, nodes map[string]guestFilesystemNode) (string, bool) {
	candidates := make([]string, 0)
	for _, node := range nodes {
		if node.Name == name && node.Type != "filesystem" {
			candidates = append(candidates, node.Name)
		}
	}
	if len(candidates) == 0 {
		return resolveNodeDevice(name, nodes, map[string]bool{})
	}
	sort.Strings(candidates)
	resolved := ""
	for _, candidate := range candidates {
		device, ok := resolveNodeDevice(candidate, nodes, map[string]bool{})
		if !ok {
			continue
		}
		if resolved != "" && resolved != device {
			return "", false
		}
		resolved = device
	}
	return resolved, resolved != ""
}

func resolveNodeDevice(name string, nodes map[string]guestFilesystemNode, seen map[string]bool) (string, bool) {
	name = normalizeGuestDevice(name)
	if name == "" || seen[name] {
		return "", false
	}
	if device, ok := directGuestDevice(name); ok {
		return device, true
	}
	seen[name] = true
	node, ok := nodes[name]
	if !ok || node.Parent == "" {
		return "", false
	}
	if node.Type == "vg" {
		if parent, ok := nodes[node.Parent]; ok && node.Size > 0 && parent.Size > 0 && parent.Size < node.Size {
			if replacement, ok := uniqueSameSizePV(node, nodes); ok {
				return resolveNodeDevice(replacement, nodes, seen)
			}
			return "", false
		}
	}
	return resolveNodeDevice(node.Parent, nodes, seen)
}

func uniqueSameSizePV(target guestFilesystemNode, nodes map[string]guestFilesystemNode) (string, bool) {
	match := ""
	for _, node := range nodes {
		if node.Type != "pv" || node.Size <= 0 || node.Size != target.Size {
			continue
		}
		if match != "" && match != node.Name {
			return "", false
		}
		match = node.Name
	}
	return match, match != ""
}

func filesystemNodePriority(nodeType string) int {
	switch strings.ToLower(strings.TrimSpace(nodeType)) {
	case "lv":
		return 5
	case "vg":
		return 4
	case "pv":
		return 3
	case "partition":
		return 2
	case "device":
		return 1
	case "filesystem":
		return 0
	default:
		return 0
	}
}

func directGuestDevice(name string) (string, bool) {
	name = normalizeGuestDevice(name)
	if name == "" {
		return "", false
	}
	base := strings.TrimPrefix(path.Base(name), "x")
	if strings.HasPrefix(base, "vd") || strings.HasPrefix(base, "sd") || strings.HasPrefix(base, "hd") {
		device := base
		for len(device) > 0 && device[len(device)-1] >= '0' && device[len(device)-1] <= '9' {
			device = device[:len(device)-1]
		}
		if len(device) >= 3 {
			return "/dev/" + device, true
		}
	}
	return "", false
}

func diskTargetsByGuestDevice(doc domainXML) map[string]string {
	targets := make(map[string]string)
	index := 0
	for _, disk := range doc.Devices.Disks {
		if disk.Device != "disk" || strings.TrimSpace(disk.Target.Dev) == "" {
			continue
		}
		device := guestDiskDeviceName(index)
		if device != "" {
			targets[device] = disk.Target.Dev
		}
		index++
	}
	return targets
}

func guestDiskDeviceName(index int) string {
	if index < 0 {
		return ""
	}
	letters := ""
	for {
		letters = string(rune('a'+index%26)) + letters
		index = index/26 - 1
		if index < 0 {
			break
		}
	}
	return "/dev/sd" + letters
}

func normalizeVirtFilesystemName(value string) string {
	_, filesystem := splitVirtFilesystemName(value)
	return filesystem
}

func splitVirtFilesystemName(value string) (string, string) {
	value = strings.TrimSpace(value)
	vmName := ""
	if idx := strings.Index(value, ":"); idx >= 0 {
		vmName = strings.TrimSpace(value[:idx])
		value = value[idx+1:]
	}
	return vmName, normalizeGuestDevice(value)
}

func normalizeGuestDevice(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || value == "-" {
		return ""
	}
	return value
}

func csvHeaderIndexes(row []string) map[string]int {
	indexes := make(map[string]int, len(row))
	for index, value := range row {
		indexes[strings.ToLower(strings.TrimSpace(value))] = index
	}
	return indexes
}

func firstCSVIndex(indexes map[string]int, names ...string) int {
	for _, name := range names {
		if index, ok := indexes[strings.ToLower(strings.TrimSpace(name))]; ok {
			return index
		}
	}
	return -1
}

func (p *VirshProvider) logGuestFilesystemWarning(message string, vmName string, err error, attrs ...slog.Attr) {
	if p.logger == nil {
		return
	}
	args := make([]any, 0, 4+len(attrs)*2)
	if strings.TrimSpace(vmName) != "" {
		args = append(args, "vm", vmName)
	}
	if err != nil {
		args = append(args, "error", err)
	}
	for _, attr := range attrs {
		args = append(args, attr.Key, attr.Value.Any())
	}
	p.logger.Warn(message, args...)
}
