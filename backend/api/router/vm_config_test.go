package router

import (
	"strings"
	"testing"
)

func TestValidateVMConfigUpdate(t *testing.T) {
	valid := updateVMConfigRequest{
		Description:     "test vm",
		CurrentCPU:      2,
		MaximumCPU:      4,
		CurrentMemoryMB: 2048,
		MaximumMemoryMB: 4096,
	}
	if err := validateVMConfigUpdate(valid); err != nil {
		t.Fatalf("expected valid config, got %v", err)
	}

	cases := []struct {
		name string
		body updateVMConfigRequest
	}{
		{name: "zero current cpu", body: updateVMConfigRequest{CurrentCPU: 0, MaximumCPU: 4, CurrentMemoryMB: 2048, MaximumMemoryMB: 4096}},
		{name: "current cpu exceeds maximum", body: updateVMConfigRequest{CurrentCPU: 8, MaximumCPU: 4, CurrentMemoryMB: 2048, MaximumMemoryMB: 4096}},
		{name: "zero current memory", body: updateVMConfigRequest{CurrentCPU: 2, MaximumCPU: 4, CurrentMemoryMB: 0, MaximumMemoryMB: 4096}},
		{name: "current memory exceeds maximum", body: updateVMConfigRequest{CurrentCPU: 2, MaximumCPU: 4, CurrentMemoryMB: 8192, MaximumMemoryMB: 4096}},
		{name: "description too long", body: updateVMConfigRequest{Description: strings.Repeat("a", 2049), CurrentCPU: 2, MaximumCPU: 4, CurrentMemoryMB: 2048, MaximumMemoryMB: 4096}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := validateVMConfigUpdate(tc.body); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}
