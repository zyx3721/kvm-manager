package kvm

import (
	"os/exec"
	"testing"
)

func TestApplyCommandEnvironmentUsesDirectBackendForLibguestfsCommands(t *testing.T) {
	cmd := exec.Command("virt-df", "--csv")
	applyCommandEnvironment(cmd, "virt-df")
	if !hasEnvVar(cmd.Env, "LIBGUESTFS_BACKEND") {
		t.Fatal("expected LIBGUESTFS_BACKEND to be set for virt-df")
	}
}

func TestApplyCommandEnvironmentDoesNotSetDirectBackendForVirsh(t *testing.T) {
	cmd := exec.Command("virsh", "list")
	applyCommandEnvironment(cmd, "virsh")
	if hasEnvVar(cmd.Env, "LIBGUESTFS_BACKEND") {
		t.Fatal("did not expect LIBGUESTFS_BACKEND for virsh")
	}
}
