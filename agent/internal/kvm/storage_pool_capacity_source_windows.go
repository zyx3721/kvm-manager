//go:build windows

package kvm

import (
	"path/filepath"
	"strings"
)

func storagePoolCapacitySource(path string) string {
	volume := filepath.VolumeName(filepath.Clean(strings.TrimSpace(path)))
	if volume == "" {
		return ""
	}
	return "volume:" + strings.ToLower(volume)
}
