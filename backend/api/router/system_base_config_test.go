package router

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSanitizeSystemBaseConfigDefaultsIcon(t *testing.T) {
	recorder := httptest.NewRecorder()
	config, ok := sanitizeSystemBaseConfig(recorder, systemBaseConfigRequest{
		SiteName:    " KVM Manager ",
		LoginName:   "KVM Manager",
		AppName:     "KVM Manager",
		AppSubtitle: "VIRTUALIZATION OPS",
	})
	if !ok {
		t.Fatal("expected config to be valid")
	}
	if got := config["siteName"]; got != "KVM Manager" {
		t.Fatalf("siteName = %#v, want KVM Manager", got)
	}
	if got := config["iconData"]; got != "/favicon.svg" {
		t.Fatalf("iconData = %#v, want /favicon.svg", got)
	}
	if got := config["resourceWarningThreshold"]; got != 70 {
		t.Fatalf("resourceWarningThreshold = %#v, want 70", got)
	}
	if got := config["resourceCriticalThreshold"]; got != 85 {
		t.Fatalf("resourceCriticalThreshold = %#v, want 85", got)
	}
	if got := config["passwordResetSendCooldownMinutes"]; got != 0.5 {
		t.Fatalf("passwordResetSendCooldownMinutes = %#v, want 0.5", got)
	}
	if got := config["passwordResetRateLimitMinutes"]; got != 5 {
		t.Fatalf("passwordResetRateLimitMinutes = %#v, want 5", got)
	}
	if got := config["resourceAlertConsecutiveCount"]; got != 3 {
		t.Fatalf("resourceAlertConsecutiveCount = %#v, want 3", got)
	}
	if got := config["agentOfflineFailureCount"]; got != 3 {
		t.Fatalf("agentOfflineFailureCount = %#v, want 3", got)
	}
}

func TestSanitizeSystemBaseConfigRejectsEmptyName(t *testing.T) {
	recorder := httptest.NewRecorder()
	if _, ok := sanitizeSystemBaseConfig(recorder, systemBaseConfigRequest{
		SiteName:    "",
		LoginName:   "KVM Manager",
		AppName:     "KVM Manager",
		AppSubtitle: "VIRTUALIZATION OPS",
	}); ok {
		t.Fatal("expected empty name to be rejected")
	}
}

func TestSanitizeSystemBaseConfigRejectsExternalIconURL(t *testing.T) {
	recorder := httptest.NewRecorder()
	if _, ok := sanitizeSystemBaseConfig(recorder, systemBaseConfigRequest{
		SiteName:    "KVM Manager",
		LoginName:   "KVM Manager",
		AppName:     "KVM Manager",
		AppSubtitle: "VIRTUALIZATION OPS",
		IconData:    "https://example.com/favicon.png",
	}); ok {
		t.Fatal("expected external icon url to be rejected")
	}
}

func TestSanitizeSystemBaseConfigRejectsOversizedIcon(t *testing.T) {
	recorder := httptest.NewRecorder()
	if _, ok := sanitizeSystemBaseConfig(recorder, systemBaseConfigRequest{
		SiteName:    "KVM Manager",
		LoginName:   "KVM Manager",
		AppName:     "KVM Manager",
		AppSubtitle: "VIRTUALIZATION OPS",
		IconData:    "data:image/png;base64," + strings.Repeat("a", 512*1024),
	}); ok {
		t.Fatal("expected oversized icon to be rejected")
	}
}

func TestSanitizeSystemBaseConfigRejectsInvalidThresholdOrder(t *testing.T) {
	recorder := httptest.NewRecorder()
	if _, ok := sanitizeSystemBaseConfig(recorder, systemBaseConfigRequest{
		SiteName:                  "KVM Manager",
		LoginName:                 "KVM Manager",
		AppName:                   "KVM Manager",
		AppSubtitle:               "VIRTUALIZATION OPS",
		ResourceWarningThreshold:  90,
		ResourceCriticalThreshold: 80,
	}); ok {
		t.Fatal("expected invalid threshold order to be rejected")
	}
}

func TestSanitizeSystemBaseConfigAcceptsHalfMinuteCooldown(t *testing.T) {
	recorder := httptest.NewRecorder()
	config, ok := sanitizeSystemBaseConfig(recorder, systemBaseConfigRequest{
		SiteName:                         "KVM Manager",
		LoginName:                        "KVM Manager",
		AppName:                          "KVM Manager",
		AppSubtitle:                      "VIRTUALIZATION OPS",
		PasswordResetSendCooldownMinutes: 0.5,
		PasswordResetRateLimitMinutes:    5,
	})
	if !ok {
		t.Fatal("expected half minute cooldown to be accepted")
	}
	if got := config["passwordResetSendCooldownMinutes"]; got != 0.5 {
		t.Fatalf("passwordResetSendCooldownMinutes = %#v, want 0.5", got)
	}
}

func TestSanitizeSystemBaseConfigRejectsInvalidCooldownStep(t *testing.T) {
	recorder := httptest.NewRecorder()
	if _, ok := sanitizeSystemBaseConfig(recorder, systemBaseConfigRequest{
		SiteName:                         "KVM Manager",
		LoginName:                        "KVM Manager",
		AppName:                          "KVM Manager",
		AppSubtitle:                      "VIRTUALIZATION OPS",
		PasswordResetSendCooldownMinutes: 0.7,
		PasswordResetRateLimitMinutes:    5,
	}); ok {
		t.Fatal("expected non half minute cooldown to be rejected")
	}
}

func TestSanitizeSystemBaseConfigRejectsSmallRateLimitWindow(t *testing.T) {
	recorder := httptest.NewRecorder()
	if _, ok := sanitizeSystemBaseConfig(recorder, systemBaseConfigRequest{
		SiteName:                      "KVM Manager",
		LoginName:                     "KVM Manager",
		AppName:                       "KVM Manager",
		AppSubtitle:                   "VIRTUALIZATION OPS",
		PasswordResetRateLimitMinutes: 4,
	}); ok {
		t.Fatal("expected small rate limit window to be rejected")
	}
}

func TestSanitizeSystemBaseConfigRejectsInvalidAlertConsecutiveCount(t *testing.T) {
	recorder := httptest.NewRecorder()
	if _, ok := sanitizeSystemBaseConfig(recorder, systemBaseConfigRequest{
		SiteName:                      "KVM Manager",
		LoginName:                     "KVM Manager",
		AppName:                       "KVM Manager",
		AppSubtitle:                   "VIRTUALIZATION OPS",
		ResourceAlertConsecutiveCount: 21,
	}); ok {
		t.Fatal("expected invalid alert consecutive count to be rejected")
	}
}

func TestSanitizeSystemBaseConfigRejectsInvalidAgentOfflineFailureCount(t *testing.T) {
	recorder := httptest.NewRecorder()
	if _, ok := sanitizeSystemBaseConfig(recorder, systemBaseConfigRequest{
		SiteName:                 "KVM Manager",
		LoginName:                "KVM Manager",
		AppName:                  "KVM Manager",
		AppSubtitle:              "VIRTUALIZATION OPS",
		AgentOfflineFailureCount: 21,
	}); ok {
		t.Fatal("expected invalid agent offline failure count to be rejected")
	}
}
