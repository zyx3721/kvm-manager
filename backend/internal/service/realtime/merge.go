package realtime

import (
	"context"
	"strings"

	"kvm-manager/backend/internal/domain"
)

func (s *Service) mergeFastRuntime(ctx context.Context, vms map[string]domain.VirtualMachine) {
	for id, current := range vms {
		previous, ok, err := s.runtimeStore.GetVM(ctx, id)
		if err != nil || !ok {
			continue
		}
		current = mergeFastRuntimeDetails(current, previous)
		vms[id] = current
	}
}

func mergeFastRuntimeDetails(current domain.VirtualMachine, previous domain.VirtualMachine) domain.VirtualMachine {
	if shouldPreservePreviousOSType(current.OSType, previous.OSType) {
		current.OSType = previous.OSType
	}
	if strings.TrimSpace(current.Description) == "" {
		current.Description = previous.Description
	}
	if !usablePrimaryIP(current.PrimaryIP) {
		current.PrimaryIP = previous.PrimaryIP
	}
	if current.DiskBytes <= 0 {
		current.DiskBytes = previous.DiskBytes
		current.DiskUsedBytes = previous.DiskUsedBytes
		current.Disks = previous.Disks
		current.DiskUsage = previous.DiskUsage
		current.DiskUsageAvailable = previous.DiskUsageAvailable
	}
	if !current.MemoryUsageAvailable && previous.MemoryUsageAvailable {
		current.MemoryUsage = previous.MemoryUsage
		current.MemoryUsageAvailable = previous.MemoryUsageAvailable
	}
	return current
}

func (s *Service) mergeRuntimeDetails(ctx context.Context, vms map[string]domain.VirtualMachine) {
	for id, current := range vms {
		previous, ok, err := s.runtimeStore.GetVM(ctx, id)
		if err != nil || !ok {
			continue
		}
		if shouldPreservePreviousOSType(current.OSType, previous.OSType) {
			current.OSType = previous.OSType
		}
		vms[id] = current
	}
}

func shouldPreservePreviousOSType(current string, previous string) bool {
	current = strings.TrimSpace(current)
	previous = strings.TrimSpace(previous)
	if previous == "" {
		return false
	}
	if current == "" {
		return true
	}
	if strings.EqualFold(current, previous) {
		return false
	}
	return osTypeConfidence(current) < osTypeConfidence(previous)
}

func osTypeConfidence(value string) int {
	lower := strings.ToLower(strings.TrimSpace(value))
	if lower == "" {
		return 0
	}
	if strings.HasSuffix(lower, " virtualization") ||
		lower == "windows" ||
		lower == "centos" ||
		lower == "ubuntu" ||
		lower == "debian" ||
		lower == "rocky linux" ||
		lower == "almalinux" ||
		lower == "fedora" {
		return 1
	}
	return 2
}

func usablePrimaryIP(value string) bool {
	value = strings.TrimSpace(value)
	return value != "" && !strings.HasPrefix(value, "127.") && !strings.HasPrefix(value, "169.254.")
}
