package kvm

import (
	"os/exec"
	"strings"
	"testing"
)

func TestApplyCommandEnvironmentUsesDirectBackendForLibguestfsCommands(t *testing.T) {
	cmd := exec.Command("virt-df", "--csv")
	applyCommandEnvironment(cmd, "virt-df")
	if !hasEnvVar(cmd.Env, "LIBGUESTFS_BACKEND") {
		t.Fatal("expected LIBGUESTFS_BACKEND to be set for virt-df")
	}
	assertCLocale(t, cmd.Env)
}

func TestApplyCommandEnvironmentUsesCLocaleForAllCommands(t *testing.T) {
	cmd := exec.Command("qemu-img", "info", "demo.qcow2")
	applyCommandEnvironment(cmd, "qemu-img")
	if hasEnvVar(cmd.Env, "LIBGUESTFS_BACKEND") {
		t.Fatal("did not expect LIBGUESTFS_BACKEND for qemu-img")
	}
	assertCLocale(t, cmd.Env)
}

func assertCLocale(t *testing.T, env []string) {
	t.Helper()
	for _, key := range []string{"LC_ALL", "LANG", "LANGUAGE"} {
		if value := lastEnvValue(env, key); value != "C" {
			t.Fatalf("expected %s=C, got %q", key, value)
		}
	}
}

func lastEnvValue(env []string, key string) string {
	prefix := key + "="
	value := ""
	for _, item := range env {
		if strings.HasPrefix(item, prefix) {
			value = strings.TrimPrefix(item, prefix)
		}
	}
	return value
}
