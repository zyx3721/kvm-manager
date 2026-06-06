package kvm

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const networkScriptsDir = "/etc/sysconfig/network-scripts"

func (p *VirshProvider) backupHostInterfaceBridgeIFCFG(request HostInterfaceCreateRequest) error {
	if strings.ToLower(strings.TrimSpace(request.Type)) != "bridge" || strings.TrimSpace(request.Device) == "" {
		return nil
	}
	if err := backupIFCFGIfExists(filepath.Join(networkScriptsDir, "ifcfg-"+strings.TrimSpace(request.Name))); err != nil {
		return fmt.Errorf("backup bridge interface config failed: %w", err)
	}
	if err := backupIFCFGIfExists(filepath.Join(networkScriptsDir, "ifcfg-"+strings.TrimSpace(request.Device))); err != nil {
		return fmt.Errorf("backup bridge device config failed: %w", err)
	}
	return nil
}

func backupIFCFGIfExists(path string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read system interface config failed: %w", err)
	}
	backupPath := path + "." + time.Now().Format("20060102150405") + ".bak"
	if err := os.WriteFile(backupPath, content, 0o600); err != nil {
		return fmt.Errorf("backup system interface config failed: %w", err)
	}
	return nil
}
