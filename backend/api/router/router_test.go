package router

import "testing"

func TestParseVMPath(t *testing.T) {
	id, action, ok := parseVMPath("/api/vms/123/start")
	if !ok || id != "123" || action != "start" {
		t.Fatalf("unexpected parse result: id=%q action=%q ok=%v", id, action, ok)
	}
	id, action, ok = parseVMPath("/api/vms/123")
	if !ok || id != "123" || action != "" {
		t.Fatalf("unexpected parse result: id=%q action=%q ok=%v", id, action, ok)
	}
	id, action, ok = parseVMPath("/api/vms/123/config")
	if !ok || id != "123" || action != "config" {
		t.Fatalf("unexpected config parse result: id=%q action=%q ok=%v", id, action, ok)
	}

	id, action, ok = parseVMPath("/api/vms/123/autostart")
	if !ok || id != "123" || action != "autostart" {
		t.Fatalf("unexpected autostart parse result: id=%q action=%q ok=%v", id, action, ok)
	}
}

func TestParseSnapshotPath(t *testing.T) {
	id, action, ok := parseSnapshotPath("/api/snapshots/vm-1:snapshot:base/revert")
	if !ok || id != "vm-1:snapshot:base" || action != "revert" {
		t.Fatalf("unexpected parse result: id=%q action=%q ok=%v", id, action, ok)
	}
	_, _, ok = parseSnapshotPath("/api/snapshots/vm-1:snapshot:base")
	if ok {
		t.Fatal("expected path without action to be rejected")
	}
}

func TestMapVMAction(t *testing.T) {
	action, ok := mapVMAction("stop")
	if !ok || action != "shutdown" {
		t.Fatalf("unexpected stop mapping: %q %v", action, ok)
	}
	action, ok = mapVMAction("pause")
	if !ok || action != "suspend" {
		t.Fatalf("unexpected pause mapping: %q %v", action, ok)
	}
	action, ok = mapVMAction("resume")
	if !ok || action != "resume" {
		t.Fatalf("unexpected resume mapping: %q %v", action, ok)
	}
	action, ok = mapVMAction("force-stop")
	if !ok || action != "destroy" {
		t.Fatalf("unexpected force-stop mapping: %q %v", action, ok)
	}
	action, ok = mapVMAction("shutdown")
	if !ok || action != "shutdown" {
		t.Fatalf("unexpected shutdown mapping: %q %v", action, ok)
	}
	action, ok = mapVMAction("force-shutdown")
	if !ok || action != "destroy" {
		t.Fatalf("unexpected force-shutdown mapping: %q %v", action, ok)
	}
	action, ok = mapVMAction("force-reboot")
	if !ok || action != "reset" {
		t.Fatalf("unexpected force-reboot mapping: %q %v", action, ok)
	}
	action, ok = mapVMAction("delete")
	if !ok || action != "delete" {
		t.Fatalf("unexpected delete mapping: %q %v", action, ok)
	}
	action, ok = mapVMAction("force-delete")
	if !ok || action != "force-delete" {
		t.Fatalf("unexpected force-delete mapping: %q %v", action, ok)
	}
}

func TestShouldUpdateStatusAndDelayFullSyncAfterVMAction(t *testing.T) {
	for _, action := range []string{"start", "resume", "reboot", "force-reboot", "shutdown", "stop", "force-shutdown", "force-stop", "pause"} {
		if !shouldUpdateStatusAndDelayFullSyncAfterVMAction(action) {
			t.Fatalf("expected %s to update status and delay full sync", action)
		}
	}
	for _, action := range []string{"delete", "force-delete"} {
		if shouldUpdateStatusAndDelayFullSyncAfterVMAction(action) {
			t.Fatalf("expected %s to sync immediately", action)
		}
	}
}

func TestShouldRemoveVMAndDelayFullSyncAfterVMAction(t *testing.T) {
	for _, action := range []string{"delete", "force-delete"} {
		if !shouldRemoveVMAndDelayFullSyncAfterVMAction(action) {
			t.Fatalf("expected %s to remove vm and delay full sync", action)
		}
	}
	for _, action := range []string{"start", "stop", "force-stop"} {
		if shouldRemoveVMAndDelayFullSyncAfterVMAction(action) {
			t.Fatalf("expected %s not to remove vm", action)
		}
	}
}

func TestNormalizeHostMetricAgentID(t *testing.T) {
	if got := normalizeHostMetricAgentID("all"); got != "" {
		t.Fatalf("expected all to query every host, got %q", got)
	}
	if got := normalizeHostMetricAgentID("ALL"); got != "" {
		t.Fatalf("expected ALL to query every host, got %q", got)
	}
	if got := normalizeHostMetricAgentID("agent-1"); got != "agent-1" {
		t.Fatalf("expected concrete agent id to be preserved, got %q", got)
	}
}

func TestEndpointIdentityMatchesResolvedIP(t *testing.T) {
	byIP := agentEndpointIdentity{normalized: "http://127.0.0.1:9443", ips: []string{"127.0.0.1"}}
	byDomain := agentEndpointIdentity{normalized: "http://localhost:9443", ips: []string{"127.0.0.1", "::1"}}
	if !endpointIdentityMatches(byDomain, byIP) {
		t.Fatal("expected endpoint identities with the same resolved IP to match")
	}
}

func TestEndpointIdentityDoesNotMatchDifferentPath(t *testing.T) {
	left := agentEndpointIdentity{normalized: "http://127.0.0.1:9443/agent-a", ips: []string{"127.0.0.1"}}
	right := agentEndpointIdentity{normalized: "http://localhost:9443/agent-b", ips: []string{"127.0.0.1"}}
	if endpointIdentityMatches(left, right) {
		t.Fatal("expected endpoints with different paths not to match")
	}
}

func TestResolveAgentEndpointIdentityNormalizesDefaultPort(t *testing.T) {
	identity, err := resolveAgentEndpointIdentity(t.Context(), "http://127.0.0.1")
	if err != nil {
		t.Fatalf("resolve endpoint identity failed: %v", err)
	}
	if identity.normalized != "http://127.0.0.1:80" {
		t.Fatalf("unexpected normalized endpoint: %q", identity.normalized)
	}
}
