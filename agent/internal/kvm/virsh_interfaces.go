package kvm

import (
	"fmt"
	"strings"
)

func (p *VirshProvider) validateVMDeviceInterfaceUpdates(requests []VMDeviceInterfaceRequest, config VMConfig) ([]vmInterfaceUpdateAction, error) {
	interfaceByName := make(map[string]VMConfigInterface, len(config.Interfaces))
	interfaceByMAC := make(map[string]VMConfigInterface, len(config.Interfaces))
	networkPools := p.networkPoolsByName()
	for _, iface := range config.Interfaces {
		if strings.TrimSpace(iface.Name) != "" {
			interfaceByName[iface.Name] = iface
		}
		if strings.TrimSpace(iface.MAC) != "" {
			interfaceByMAC[strings.ToLower(strings.TrimSpace(iface.MAC))] = iface
		}
	}
	updates := make([]vmInterfaceUpdateAction, 0, len(requests))
	seen := map[string]bool{}
	for _, iface := range requests {
		name := strings.TrimSpace(iface.Name)
		mac := strings.ToLower(strings.TrimSpace(iface.MAC))
		source := strings.TrimSpace(iface.Source)
		if name == "" && mac == "" {
			return nil, fmt.Errorf("interface name or mac is required")
		}
		if source == "" {
			return nil, fmt.Errorf("interface source is required")
		}
		current, ok := interfaceByName[name]
		if !ok && mac != "" {
			current, ok = interfaceByMAC[mac]
		}
		if !ok {
			return nil, fmt.Errorf("interface target not found")
		}
		update := domainInterfaceUpdateForSource(current.Type, source, networkPools)
		key := firstNonEmptyString(strings.ToLower(strings.TrimSpace(current.MAC)), strings.TrimSpace(current.Name))
		if seen[key] {
			continue
		}
		seen[key] = true
		updates = append(updates, vmInterfaceUpdateAction{Current: current, Update: update})
	}
	return updates, nil
}

func (p *VirshProvider) validateVMDeviceNewInterfaces(requests []VMDeviceNewInterface) ([]domainInterfaceXMLFragment, error) {
	networkPools := p.networkPoolsByName()
	items := make([]domainInterfaceXMLFragment, 0, len(requests))
	for _, iface := range requests {
		source := strings.TrimSpace(iface.Source)
		model := strings.TrimSpace(iface.Model)
		if model == "" {
			model = "virtio"
		}
		if source == "" {
			return nil, fmt.Errorf("new interface source is required")
		}
		if !supportedVMInterfaceModel(model) {
			return nil, fmt.Errorf("unsupported interface model")
		}
		update := domainInterfaceUpdateForSource("network", source, networkPools)
		items = append(items, domainInterfaceXMLFragment{
			Type:       firstNonEmptyString(update.Type, "network"),
			SourceAttr: firstNonEmptyString(update.SourceAttr, "network"),
			Source:     update.Source,
			Model:      model,
		})
	}
	return items, nil
}

func validateVMDeviceDeletedInterfaces(requests []VMDeviceDeleteInterface, config VMConfig) ([]vmInterfaceDeleteAction, error) {
	interfaceByName := make(map[string]VMConfigInterface, len(config.Interfaces))
	interfaceByMAC := make(map[string]VMConfigInterface, len(config.Interfaces))
	for _, iface := range config.Interfaces {
		if strings.TrimSpace(iface.Name) != "" {
			interfaceByName[iface.Name] = iface
		}
		if strings.TrimSpace(iface.MAC) != "" {
			interfaceByMAC[strings.ToLower(strings.TrimSpace(iface.MAC))] = iface
		}
	}
	items := make([]vmInterfaceDeleteAction, 0, len(requests))
	seen := map[string]bool{}
	for _, iface := range requests {
		name := strings.TrimSpace(iface.Name)
		mac := strings.ToLower(strings.TrimSpace(iface.MAC))
		if name == "" && mac == "" {
			return nil, fmt.Errorf("interface name or mac is required")
		}
		current, ok := interfaceByName[name]
		if !ok && mac != "" {
			current, ok = interfaceByMAC[mac]
		}
		if !ok {
			return nil, fmt.Errorf("interface target not found")
		}
		key := firstNonEmptyString(strings.ToLower(strings.TrimSpace(current.MAC)), strings.TrimSpace(current.Name))
		if seen[key] {
			continue
		}
		seen[key] = true
		items = append(items, vmInterfaceDeleteAction{Current: current})
	}
	return items, nil
}

func interfaceSourceUpdatesByKey(actions []vmInterfaceUpdateAction) map[string]domainInterfaceSourceUpdate {
	items := make(map[string]domainInterfaceSourceUpdate, len(actions)*2)
	for _, action := range actions {
		if action.Current.Name != "" {
			items[action.Current.Name] = action.Update
		}
		if action.Current.MAC != "" {
			items[strings.ToLower(action.Current.MAC)] = action.Update
		}
	}
	return items
}

func deletedInterfaceKeys(actions []vmInterfaceDeleteAction) map[string]bool {
	items := make(map[string]bool, len(actions)*2)
	for _, action := range actions {
		if action.Current.Name != "" {
			items[action.Current.Name] = true
		}
		if action.Current.MAC != "" {
			items[strings.ToLower(action.Current.MAC)] = true
		}
	}
	return items
}

func (p *VirshProvider) networkPoolsByName() map[string]NetworkPool {
	pools, err := p.ListNetworkPools()
	if err != nil {
		return map[string]NetworkPool{}
	}
	items := make(map[string]NetworkPool, len(pools))
	for _, pool := range pools {
		items[pool.Name] = pool
	}
	return items
}

func (p *VirshProvider) changeLiveInterface(vmName string, action vmInterfaceUpdateAction) error {
	if sameInterfaceSource(action.Current, action.Update) {
		return nil
	}
	if err := p.detachLiveInterface(vmName, action.Current); err != nil {
		return err
	}
	next := domainInterfaceXMLFragment{
		Type:       firstNonEmptyString(action.Update.Type, action.Current.Type, "network"),
		SourceAttr: firstNonEmptyString(action.Update.SourceAttr, interfaceSourceAttrName(action.Update.Type)),
		Source:     action.Update.Source,
		Model:      firstNonEmptyString(action.Current.Model, "virtio"),
	}
	return p.attachLiveInterface(vmName, next, action.Current.MAC)
}

func (p *VirshProvider) attachLiveInterface(vmName string, iface domainInterfaceXMLFragment, mac string) error {
	args := liveAttachInterfaceArgs(p.libvirtURI, vmName, iface, mac)
	_, err := p.output("virsh", args...)
	return err
}

func (p *VirshProvider) detachLiveInterface(vmName string, iface VMConfigInterface) error {
	mac := strings.ToLower(strings.TrimSpace(iface.MAC))
	if mac == "" {
		return fmt.Errorf("interface mac is required for live detach")
	}
	args := liveDetachInterfaceArgs(p.libvirtURI, vmName, iface, mac)
	_, err := p.output("virsh", args...)
	return err
}

func liveAttachInterfaceArgs(libvirtURI string, vmName string, iface domainInterfaceXMLFragment, mac string) []string {
	args := []string{
		"--connect", libvirtURI,
		"attach-interface",
		"--domain", vmName,
		"--type", firstNonEmptyString(iface.Type, "network"),
		"--source", iface.Source,
		"--model", firstNonEmptyString(iface.Model, "virtio"),
		"--live",
		"--config",
	}
	if strings.TrimSpace(mac) != "" {
		args = append(args, "--mac", strings.ToLower(strings.TrimSpace(mac)))
	}
	return args
}

func liveDetachInterfaceArgs(libvirtURI string, vmName string, iface VMConfigInterface, mac string) []string {
	return []string{
		"--connect", libvirtURI,
		"detach-interface",
		"--domain", vmName,
		"--type", firstNonEmptyString(iface.Type, "network"),
		"--mac", strings.ToLower(strings.TrimSpace(mac)),
		"--live",
		"--config",
	}
}

func sameInterfaceSource(current VMConfigInterface, update domainInterfaceSourceUpdate) bool {
	return strings.EqualFold(strings.TrimSpace(current.Type), strings.TrimSpace(update.Type)) &&
		strings.EqualFold(strings.TrimSpace(current.Source), strings.TrimSpace(update.Source))
}

func supportedVMInterfaceModel(model string) bool {
	switch strings.ToLower(strings.TrimSpace(model)) {
	case "virtio", "e1000", "e1000e", "rtl8139", "vmxnet3":
		return true
	default:
		return false
	}
}

func domainInterfaceUpdateForSource(currentType string, source string, pools map[string]NetworkPool) domainInterfaceSourceUpdate {
	source = strings.TrimSpace(source)
	if pool, ok := pools[source]; ok {
		if strings.EqualFold(strings.TrimSpace(pool.Forward), "bridge") && strings.TrimSpace(pool.Bridge) != "" {
			return domainInterfaceSourceUpdate{Type: "bridge", SourceAttr: "bridge", Source: strings.TrimSpace(pool.Bridge)}
		}
		return domainInterfaceSourceUpdate{Type: "network", SourceAttr: "network", Source: strings.TrimSpace(pool.Name)}
	}
	interfaceType := strings.ToLower(strings.TrimSpace(currentType))
	return domainInterfaceSourceUpdate{Type: interfaceType, SourceAttr: interfaceSourceAttrName(interfaceType), Source: source}
}
