package kvm

import (
	"fmt"
	"path"
	"strings"
)

func (p *VirshProvider) StartVM(vmName string) error {
	_, err := p.output("virsh", "--connect", p.libvirtURI, "start", vmName)
	return err
}

func (p *VirshProvider) ShutdownVM(vmName string) error {
	_, err := p.output("virsh", "--connect", p.libvirtURI, "shutdown", vmName)
	return err
}

func (p *VirshProvider) PauseVM(vmName string) error {
	_, err := p.output("virsh", "--connect", p.libvirtURI, "suspend", vmName)
	return err
}

func (p *VirshProvider) ResumeVM(vmName string) error {
	_, err := p.output("virsh", "--connect", p.libvirtURI, "resume", vmName)
	return err
}

func (p *VirshProvider) RebootVM(vmName string) error {
	_, err := p.output("virsh", "--connect", p.libvirtURI, "reboot", vmName)
	return err
}

func (p *VirshProvider) ResetVM(vmName string) error {
	_, err := p.output("virsh", "--connect", p.libvirtURI, "reset", vmName)
	return err
}

func (p *VirshProvider) DestroyVM(vmName string) error {
	_, err := p.output("virsh", "--connect", p.libvirtURI, "destroy", vmName)
	return err
}

func (p *VirshProvider) RevertSnapshot(vmName string, snapshotName string) error {
	_, err := p.output("virsh", "--connect", p.libvirtURI, "snapshot-revert", vmName, snapshotName)
	return err
}

func (p *VirshProvider) CreateSnapshot(vmName string, request SnapshotCreateRequest) error {
	args := []string{"--connect", p.libvirtURI, "snapshot-create-as", vmName, request.Name}
	if request.Description != "" {
		args = append(args, "--description", request.Description)
	}
	args = append(args, "--atomic")
	_, err := p.output("virsh", args...)
	return err
}

func (p *VirshProvider) DeleteSnapshot(vmName string, snapshotName string) error {
	_, err := p.output("virsh", "--connect", p.libvirtURI, "snapshot-delete", vmName, snapshotName)
	return err
}

func (p *VirshProvider) DeleteVM(vmName string) error {
	stateOut, err := p.output("virsh", "--connect", p.libvirtURI, "domstate", vmName)
	if err != nil {
		return err
	}
	if normalizeState(stateOut) != "stopped" {
		return fmt.Errorf("vm %s must be stopped before delete", vmName)
	}
	return p.undefineVMAndDeleteDisks(vmName)
}

func (p *VirshProvider) ForceDeleteVM(vmName string) error {
	stateOut, err := p.output("virsh", "--connect", p.libvirtURI, "domstate", vmName)
	if err != nil {
		return err
	}
	if normalizeState(stateOut) != "stopped" {
		if _, err := p.output("virsh", "--connect", p.libvirtURI, "destroy", vmName); err != nil {
			return err
		}
	}
	return p.undefineVMAndDeleteDisks(vmName)
}

func (p *VirshProvider) undefineVMAndDeleteDisks(vmName string) error {
	config, err := p.vmConfig(vmName, true)
	if err != nil {
		return err
	}
	return p.undefineVMAndDeleteConfigDisks(vmName, config)
}

func (p *VirshProvider) undefineVMAndDeleteConfigDisks(vmName string, config VMConfig) error {
	return p.deleteConfigDisksAfterUndefine(vmName, config)
}

func (p *VirshProvider) destroyThenUndefineVMAndDeleteConfigDisks(vmName string, config VMConfig) error {
	if _, err := p.output("virsh", "--connect", p.libvirtURI, "destroy", vmName); err != nil && !isInactiveDomainError(err.Error()) {
		return err
	}
	return p.undefineVMAndDeleteConfigDisks(vmName, config)
}

func (p *VirshProvider) deleteConfigDisksAfterUndefine(vmName string, config VMConfig) error {
	diskVolumes := []struct {
		Pool string
		Name string
	}{}
	for _, disk := range config.Disks {
		poolName, volumeName := diskVolumeForDelete(disk)
		if poolName == "" || volumeName == "" {
			continue
		}
		diskVolumes = append(diskVolumes, struct {
			Pool string
			Name string
		}{Pool: poolName, Name: volumeName})
	}
	if _, err := p.output("virsh", "--connect", p.libvirtURI, "undefine", vmName); err != nil {
		return err
	}
	for _, volume := range diskVolumes {
		if err := p.DeleteStorageVolume(volume.Pool, volume.Name); err != nil {
			return err
		}
	}
	return nil
}

func isInactiveDomainError(message string) bool {
	lower := strings.ToLower(message)
	return strings.Contains(lower, "domain is not running") ||
		strings.Contains(lower, "domain is not active") ||
		strings.Contains(lower, "not running")
}

func diskVolumeForDelete(disk VMConfigDisk) (string, string) {
	poolName := strings.TrimSpace(disk.Pool)
	volumePath := strings.TrimSpace(disk.Path)
	if poolName == "" || volumePath == "" {
		return "", ""
	}
	volumePath = strings.ReplaceAll(volumePath, "\\", "/")
	volumeName := path.Base(volumePath)
	if volumeName == "." || volumeName == "/" || volumeName == "\\" {
		return "", ""
	}
	return poolName, volumeName
}
