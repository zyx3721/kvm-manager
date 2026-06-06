//go:build !windows

package kvm

import (
	"fmt"
	"os"
	"strings"
	"syscall"
)

func storagePoolCapacitySource(path string) string {
	target := strings.TrimSpace(path)
	if target == "" {
		return ""
	}
	info, err := os.Stat(target)
	if err != nil {
		return ""
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return ""
	}
	return fmt.Sprintf("dev:%d", uint64(stat.Dev))
}
