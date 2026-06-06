package kvm

import (
	"strings"
	"testing"
)

func TestMigrationTargetHostnameLocalhostDetectsLocalhost(t *testing.T) {
	for _, message := range []string{
		"hostname on destination resolved to localhost, but migration requires an FQDN",
		"127.0.0.1 localhost compute02",
		"127.0.1.1 compute02",
		"::1 localhost",
	} {
		if !migrationTargetHostnameLocalhost(message) {
			t.Fatalf("expected localhost hostname message to be detected: %q", message)
		}
	}
}

func TestMigrationTargetHostnameLocalhostAllowsRemoteAddress(t *testing.T) {
	if migrationTargetHostnameLocalhost("192.168.51.47 compute02.example.local compute02") {
		t.Fatal("expected remote address to pass")
	}
}

func TestMigrationHostnameLooksLocalhost(t *testing.T) {
	for _, hostname := range []string{"", "localhost", "localhost.localdomain", "127.0.0.1", "127.0.1.1", "::1"} {
		if !migrationHostnameLooksLocalhost(hostname) {
			t.Fatalf("expected hostname to look local: %q", hostname)
		}
	}
}

func TestMigrationHostnameLooksLocalhostAllowsFQDN(t *testing.T) {
	if migrationHostnameLooksLocalhost("compute02.example.local") {
		t.Fatal("expected fqdn to pass")
	}
}

func TestNormalizeMigrationHostname(t *testing.T) {
	hostname, err := normalizeMigrationHostname("KVM02.example.local")
	if err != nil {
		t.Fatalf("normalize migration hostname failed: %v", err)
	}
	if hostname != "kvm02.example.local" {
		t.Fatalf("unexpected normalized hostname: %q", hostname)
	}
	for _, invalid := range []string{"localhost", "127.0.0.1", "bad/name", "-bad", "bad-"} {
		if _, err := normalizeMigrationHostname(invalid); err == nil {
			t.Fatalf("expected invalid hostname to fail: %q", invalid)
		}
	}
}

func TestMigrationHostsEntryCommandReplacesOldHostname(t *testing.T) {
	command := migrationHostsEntryCommand("192.168.51.47", "kvm02")
	if !strings.Contains(command, "sed -i.bak -E") {
		t.Fatalf("expected command to remove old hostname entries: %s", command)
	}
	if strings.Contains(command, "mktemp") {
		t.Fatalf("did not expect temporary file in hosts command: %s", command)
	}
	if !strings.Contains(command, "192.168.51.47 kvm02") {
		t.Fatalf("expected command to append target ip and hostname: %s", command)
	}
}

func TestMigrationRemoteShellArgsWrapCommand(t *testing.T) {
	args := migrationRemoteShellArgs(migrationTarget{Username: "root", Host: "192.168.51.47", Port: "22"}, "sed -i.bak -E '/kvm02/d' /etc/hosts && printf '%s\\n' '192.168.51.47 kvm02' >> /etc/hosts")
	last := args[len(args)-1]
	if !strings.HasPrefix(last, "sh -c ") {
		t.Fatalf("expected remote shell command to be wrapped as one ssh argument: %#v", args)
	}
	if strings.Contains(last, "sh -c tmp=") {
		t.Fatalf("expected sh script to be quoted: %q", last)
	}
}
