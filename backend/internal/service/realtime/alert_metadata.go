package realtime

import (
	"strings"

	"kvm-manager/backend/internal/domain"
)

func vmAlertMetadata(item domain.Agent, vm domain.VirtualMachine, extra map[string]any) map[string]any {
	metadata := map[string]any{
		"agent":         item.Name,
		"vm":            vm.Name,
		"vmIp":          templateFallback(vm.PrimaryIP),
		"vmDescription": templateFallback(vm.Description),
	}
	for key, value := range extra {
		metadata[key] = value
	}
	return metadata
}

func templateFallback(value string) string {
	if strings.TrimSpace(value) == "" {
		return "-"
	}
	return value
}
