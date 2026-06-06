package kvm

import "strings"

type hostInterfaceListEntry struct {
	Name   string
	Status string
	MAC    string
}

func parseInterfaceListNames(output string) []string {
	entries := parseInterfaceListEntries(output)
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name)
	}
	return names
}

func parseInterfaceListEntries(output string) []hostInterfaceListEntry {
	entries := make([]hostInterfaceListEntry, 0)
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) == 0 {
			continue
		}
		name := fields[0]
		if name == "Name" || strings.HasPrefix(name, "-") {
			continue
		}
		entry := hostInterfaceListEntry{Name: name}
		if len(fields) > 1 {
			entry.Status = normalizeLibvirtInterfaceState(fields[1])
		}
		if len(fields) > 2 {
			entry.MAC = strings.ToLower(fields[2])
		}
		entries = append(entries, entry)
	}
	return entries
}

func appendUniqueInterfaceListEntries(entries []hostInterfaceListEntry, seen map[string]struct{}, output string) []hostInterfaceListEntry {
	for _, entry := range parseInterfaceListEntries(output) {
		if _, ok := seen[entry.Name]; ok {
			continue
		}
		seen[entry.Name] = struct{}{}
		entries = append(entries, entry)
	}
	return entries
}

func (p *VirshProvider) interfaceEntriesLibvirt() ([]hostInterfaceListEntry, error) {
	out, firstErr := p.output("virsh", "--connect", p.libvirtURI, "iface-list", "--all")
	if firstErr == nil {
		entries := parseInterfaceListEntries(out)
		if len(entries) > 0 {
			return entries, nil
		}
	}

	seen := map[string]struct{}{}
	entries := make([]hostInterfaceListEntry, 0)
	for _, args := range [][]string{
		{"--connect", p.libvirtURI, "iface-list"},
		{"--connect", p.libvirtURI, "iface-list", "--inactive"},
	} {
		out, err := p.output("virsh", args...)
		if err != nil {
			continue
		}
		entries = appendUniqueInterfaceListEntries(entries, seen, out)
	}
	if len(entries) == 0 && firstErr != nil {
		return nil, firstErr
	}
	return entries, nil
}

func libvirtItemNames(items []HostInterface) map[string]struct{} {
	names := make(map[string]struct{}, len(items))
	for _, item := range items {
		if strings.TrimSpace(item.Name) == "" {
			continue
		}
		names[item.Name] = struct{}{}
	}
	return names
}

func mergeHostInterfaces(primary []HostInterface, secondary []HostInterface, includeSecondaryOnly bool) []HostInterface {
	result := make([]HostInterface, 0, len(primary)+len(secondary))
	indexByName := map[string]int{}
	for _, item := range primary {
		fillInterfaceDefaults(&item)
		indexByName[item.Name] = len(result)
		result = append(result, item)
	}
	for _, item := range secondary {
		fillInterfaceDefaults(&item)
		if index, ok := indexByName[item.Name]; ok {
			result[index] = mergeHostInterface(result[index], item)
			continue
		}
		if !includeSecondaryOnly {
			continue
		}
		indexByName[item.Name] = len(result)
		result = append(result, item)
	}
	return result
}

func mergeHostInterface(primary HostInterface, secondary HostInterface) HostInterface {
	primary.MAC = firstNonEmptyString(primary.MAC, secondary.MAC)
	primary.Status = firstNonEmptyString(primary.Status, secondary.Status)
	primary.Type = firstNonEmptyString(primary.Type, secondary.Type)
	primary.IPv4 = firstNonEmptyString(primary.IPv4, secondary.IPv4)
	primary.IPv4Mode = firstNonEmptyString(primary.IPv4Mode, secondary.IPv4Mode)
	primary.IPv6 = firstNonEmptyString(primary.IPv6, secondary.IPv6)
	primary.IPv6Mode = firstNonEmptyString(primary.IPv6Mode, secondary.IPv6Mode)
	if primary.IPv6 != "" && primary.IPv6Mode == "none" && secondary.IPv6Mode == "link-local" {
		primary.IPv6Mode = "link-local"
	}
	primary.BridgeDevice = firstNonEmptyString(primary.BridgeDevice, secondary.BridgeDevice)
	primary.BootMode = firstNonEmptyString(primary.BootMode, secondary.BootMode)
	primary.STP = firstNonEmptyString(primary.STP, secondary.STP)
	primary.Delay = firstNonEmptyString(primary.Delay, secondary.Delay)
	fillInterfaceDefaults(&primary)
	return primary
}

func filterFallbackHostInterfaces(items []HostInterface) []HostInterface {
	result := make([]HostInterface, 0, len(items))
	for _, item := range items {
		if !isDisplayableFallbackHostInterface(item) {
			continue
		}
		fillInterfaceDefaults(&item)
		result = append(result, item)
	}
	return result
}

func isDisplayableFallbackHostInterface(item HostInterface) bool {
	name := strings.ToLower(strings.TrimSpace(item.Name))
	if name == "" {
		return false
	}
	if strings.Contains(name, ".") {
		return false
	}
	if name == "idrac" || strings.Contains(name, "idrac") || strings.Contains(name, "ipmi") || strings.Contains(name, "bmc") {
		return false
	}
	hiddenPrefixes := []string{
		"vnet",
		"tap",
		"veth",
		"docker",
		"br-",
		"cni",
		"flannel",
		"vxlan",
		"tun",
	}
	for _, prefix := range hiddenPrefixes {
		if strings.HasPrefix(name, prefix) {
			return false
		}
	}
	return true
}
