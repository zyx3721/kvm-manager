package router

import "testing"

func TestValidateRunningVMDevicesUpdateAllowsLiveDiskChanges(t *testing.T) {
	valid := updateVMDevicesRequest{
		DiskResizes: []updateVMDeviceDiskResizeRequest{{Name: "vda", CapacityBytes: 20 * 1024 * 1024 * 1024}},
	}
	if err := validateRunningVMDevicesUpdate(valid); err != nil {
		t.Fatalf("expected disk resize to be valid, got %v", err)
	}
	validNewDisk := updateVMDevicesRequest{
		NewDisks: []updateVMDeviceNewDiskRequest{{Name: "demo-vdb.qcow2", Pool: "default", Target: "vdb", Bus: "virtio", Format: "qcow2", CapacityBytes: 20 * 1024 * 1024 * 1024}},
	}
	if err := validateRunningVMDevicesUpdate(validNewDisk); err != nil {
		t.Fatalf("expected new disk to be valid, got %v", err)
	}
	invalidNewDisk := updateVMDevicesRequest{
		NewDisks: []updateVMDeviceNewDiskRequest{{Name: "demo-vdb.img", Pool: "default", Target: "vdb", Bus: "virtio", Format: "qcow2", CapacityBytes: 20 * 1024 * 1024 * 1024}},
	}
	if err := validateVMDevicesUpdate(invalidNewDisk); err == nil {
		t.Fatal("expected invalid running new disk payload to be rejected by full validation")
	}
	if err := validateRunningVMDevicesUpdate(invalidNewDisk); err != nil {
		t.Fatalf("expected running guard to allow validatable live disk changes only by device type, got %v", err)
	}

	cases := []struct {
		name string
		body updateVMDevicesRequest
	}{
		{
			name: "deleted disk",
			body: updateVMDevicesRequest{DeletedDisks: []updateVMDeviceDeleteDiskRequest{{Name: "vdb"}}},
		},
		{
			name: "interface update",
			body: updateVMDevicesRequest{Interfaces: []updateVMDeviceInterfaceRequest{{Name: "vnet0", Source: "default"}}},
		},
		{
			name: "new interface",
			body: updateVMDevicesRequest{NewInterfaces: []updateVMDeviceNewInterface{{Source: "default", Model: "virtio"}}},
		},
		{
			name: "deleted interface",
			body: updateVMDevicesRequest{DeletedInterfaces: []updateVMDeviceDeleteInterface{{Name: "vnet1"}}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := validateRunningVMDevicesUpdate(tc.body); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestValidateVMDevicesUpdateChecksNewInterface(t *testing.T) {
	valid := updateVMDevicesRequest{NewInterfaces: []updateVMDeviceNewInterface{{Source: "default", Model: "virtio"}}}
	if err := validateVMDevicesUpdate(valid); err != nil {
		t.Fatalf("expected new interface to be valid, got %v", err)
	}
	validE1000e := updateVMDevicesRequest{NewInterfaces: []updateVMDeviceNewInterface{{Source: "default", Model: "e1000e"}}}
	if err := validateVMDevicesUpdate(validE1000e); err != nil {
		t.Fatalf("expected e1000e interface to be valid, got %v", err)
	}
	missingSource := updateVMDevicesRequest{NewInterfaces: []updateVMDeviceNewInterface{{Model: "virtio"}}}
	if err := validateVMDevicesUpdate(missingSource); err == nil {
		t.Fatal("expected missing source to be rejected")
	}
	unsupportedModel := updateVMDevicesRequest{NewInterfaces: []updateVMDeviceNewInterface{{Source: "default", Model: "bad"}}}
	if err := validateVMDevicesUpdate(unsupportedModel); err == nil {
		t.Fatal("expected unsupported model to be rejected")
	}
}

func TestValidateVMDevicesUpdateChecksNewDiskExtension(t *testing.T) {
	cases := []struct {
		name    string
		disk    updateVMDeviceNewDiskRequest
		wantErr bool
	}{
		{
			name: "qcow2 extension",
			disk: updateVMDeviceNewDiskRequest{Name: "demo-vdb.qcow2", Pool: "default", Target: "vdb", Bus: "virtio", Format: "qcow2", CapacityBytes: 20 * 1024 * 1024 * 1024},
		},
		{
			name: "raw img extension",
			disk: updateVMDeviceNewDiskRequest{Name: "demo-vdb.img", Pool: "default", Target: "vdb", Bus: "virtio", Format: "raw", CapacityBytes: 20 * 1024 * 1024 * 1024},
		},
		{
			name: "qcow img extension",
			disk: updateVMDeviceNewDiskRequest{Name: "demo-vdb.img", Pool: "default", Target: "vdb", Bus: "virtio", Format: "qcow", CapacityBytes: 20 * 1024 * 1024 * 1024},
		},
		{
			name:    "raw raw extension rejected",
			disk:    updateVMDeviceNewDiskRequest{Name: "demo-vdb.raw", Pool: "default", Target: "vdb", Bus: "virtio", Format: "raw", CapacityBytes: 20 * 1024 * 1024 * 1024},
			wantErr: true,
		},
		{
			name:    "qcow2 img extension rejected",
			disk:    updateVMDeviceNewDiskRequest{Name: "demo-vdb.img", Pool: "default", Target: "vdb", Bus: "virtio", Format: "qcow2", CapacityBytes: 20 * 1024 * 1024 * 1024},
			wantErr: true,
		},
		{
			name:    "path separator rejected",
			disk:    updateVMDeviceNewDiskRequest{Name: "nested/demo-vdb.qcow2", Pool: "default", Target: "vdb", Bus: "virtio", Format: "qcow2", CapacityBytes: 20 * 1024 * 1024 * 1024},
			wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateVMDevicesUpdate(updateVMDevicesRequest{NewDisks: []updateVMDeviceNewDiskRequest{tc.disk}})
			if tc.wantErr && err == nil {
				t.Fatal("expected validation error")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("expected valid request, got %v", err)
			}
		})
	}
}
