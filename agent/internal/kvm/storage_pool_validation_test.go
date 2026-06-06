package kvm

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateStoragePoolCreateRequestDir(t *testing.T) {
	dir := t.TempDir()
	if err := validateStoragePoolCreateRequest(StoragePoolCreateRequest{Name: "images", Type: "dir", Path: dir}); err != nil {
		t.Fatalf("expected valid dir pool, got %v", err)
	}

	if err := validateStoragePoolCreateRequest(StoragePoolCreateRequest{Name: "images", Type: "dir", Path: filepath.Join(dir, "missing")}); err != nil {
		t.Fatalf("expected missing dir to be left for libvirt handling, got %v", err)
	}
	if err := validateStoragePoolCreateRequest(StoragePoolCreateRequest{Name: "images", Type: "dir", Path: "123"}); err == nil {
		t.Fatal("expected relative path validation error")
	}
}

func TestValidateStoragePoolCreateRequestLogical(t *testing.T) {
	dir := t.TempDir()
	regularFile := filepath.Join(dir, "disk.img")
	if err := os.WriteFile(regularFile, []byte("test"), 0o600); err != nil {
		t.Fatalf("create temp file failed: %v", err)
	}

	cases := []struct {
		name   string
		device string
	}{
		{name: "relative device", device: "sdb"},
		{name: "regular file", device: regularFile},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateStoragePoolCreateRequest(StoragePoolCreateRequest{Name: "logical", Type: "logical", Device: tc.device})
			if err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestValidateStoragePoolCreateRequestUnsupportedType(t *testing.T) {
	if err := validateStoragePoolCreateRequest(StoragePoolCreateRequest{Name: "unsupported", Type: "unsupported", Path: "/srv/storage"}); err == nil {
		t.Fatal("expected unsupported type validation error")
	}
}
