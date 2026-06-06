package kvm

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func validateStoragePoolCreateRequest(request StoragePoolCreateRequest) error {
	switch request.Type {
	case "dir":
		return validateStoragePoolTargetPath(request.Path, "storage pool path")
	case "logical":
		return validateStoragePoolBlockDevice(request.Device)
	case "netfs":
		if err := validateStoragePoolAbsolutePath(request.SourcePath, "netfs remote path"); err != nil {
			return err
		}
		return validateStoragePoolTargetPath(request.Path, "netfs local path")
	case "iscsi":
		if strings.TrimSpace(request.SourceHost) == "" {
			return fmt.Errorf("iscsi host is required")
		}
		if strings.TrimSpace(request.SourcePath) == "" {
			return fmt.Errorf("iscsi target is required")
		}
		return validateStoragePoolAbsolutePath(request.Path, "iscsi target path")
	default:
		return fmt.Errorf("unsupported storage pool type")
	}
}

func validateStoragePoolTargetPath(path string, label string) error {
	if err := validateStoragePoolAbsolutePath(path, label); err != nil {
		return err
	}
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("%s cannot be accessed", label)
	}
	if !info.IsDir() {
		return fmt.Errorf("%s must be a directory", label)
	}
	return nil
}

func validateStoragePoolBlockDevice(path string) error {
	if err := validateStoragePoolAbsolutePath(path, "storage pool device"); err != nil {
		return err
	}
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("storage pool device does not exist")
		}
		return fmt.Errorf("storage pool device cannot be accessed")
	}
	if info.IsDir() {
		return fmt.Errorf("storage pool device must be a block device")
	}
	if info.Mode()&os.ModeDevice == 0 {
		return fmt.Errorf("storage pool device must be a block device")
	}
	return nil
}

func validateStoragePoolAbsolutePath(path string, label string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return fmt.Errorf("%s is required", label)
	}
	if !isAbsolutePath(path) {
		return fmt.Errorf("%s must be an absolute path", label)
	}
	return nil
}

func isAbsolutePath(path string) bool {
	return !strings.ContainsRune(path, 0) && (strings.HasPrefix(path, "/") || filepath.IsAbs(path))
}
