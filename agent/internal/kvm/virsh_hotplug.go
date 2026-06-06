package kvm

import (
	"fmt"
	"strconv"
)

func validateRunningVMConfigExpansion(request VMConfigUpdateRequest, config VMConfig) error {
	currentMemoryMB := config.CurrentMemoryBytes / (1024 * 1024)
	maximumMemoryMB := config.MaximumMemoryBytes / (1024 * 1024)
	if request.MaximumCPU != config.MaximumCPU {
		return fmt.Errorf("live maximum cpu cannot change")
	}
	if request.CurrentCPU < config.CurrentCPU {
		return fmt.Errorf("live cpu cannot shrink")
	}
	if request.CurrentCPU > config.MaximumCPU {
		return fmt.Errorf("live cpu cannot exceed maximum cpu")
	}
	if request.MaximumMemoryMB != maximumMemoryMB {
		return fmt.Errorf("live maximum memory cannot change")
	}
	if request.CurrentMemoryMB < currentMemoryMB {
		return fmt.Errorf("live memory cannot shrink")
	}
	if request.CurrentMemoryMB > maximumMemoryMB {
		return fmt.Errorf("live memory cannot exceed maximum memory")
	}
	return nil
}

func (p *VirshProvider) updateLiveCPUConfig(vmName string, request VMConfigUpdateRequest, config VMConfig) error {
	if request.CurrentCPU == config.CurrentCPU {
		return nil
	}
	currentCPU := strconv.Itoa(request.CurrentCPU)
	_, err := p.output("virsh", "--connect", p.libvirtURI, "setvcpus", vmName, currentCPU, "--live", "--config")
	return err
}

func (p *VirshProvider) updateLiveMemoryConfig(vmName string, request VMConfigUpdateRequest, config VMConfig) error {
	currentMemoryMB := config.CurrentMemoryBytes / (1024 * 1024)
	if request.CurrentMemoryMB == currentMemoryMB {
		return nil
	}
	currentKiB := strconv.FormatInt(request.CurrentMemoryMB*mibToKiB, 10)
	_, err := p.output("virsh", "--connect", p.libvirtURI, "setmem", vmName, currentKiB, "--live", "--config")
	return err
}
