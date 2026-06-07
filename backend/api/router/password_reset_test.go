package router

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"kvm-manager/backend/config"
)

func TestPasswordResetChannelRules(t *testing.T) {
	for _, id := range []string{"email"} {
		if !isPasswordResetChannel(id) {
			t.Fatalf("expected %s to be usable for password reset", id)
		}
	}
	for _, id := range []string{"", "webhook", "lark", "wechat", "dingtalk", "lark_app", "wechat_app", "dingtalk_app", "sms", "ldap"} {
		if isPasswordResetChannel(id) {
			t.Fatalf("expected %s not to be usable for password reset", id)
		}
	}
}

func TestPasswordResetRateLimitSettings(t *testing.T) {
	if passwordResetRateLimitMax != 5 {
		t.Fatalf("passwordResetRateLimitMax = %d, want 5", passwordResetRateLimitMax)
	}
	if defaultPasswordResetRateLimitSpan != 5*time.Minute {
		t.Fatalf("defaultPasswordResetRateLimitSpan = %v, want 5m", defaultPasswordResetRateLimitSpan)
	}
	if defaultPasswordResetSendCooldown != 30*time.Second {
		t.Fatalf("defaultPasswordResetSendCooldown = %v, want 30s", defaultPasswordResetSendCooldown)
	}
}

func TestPasswordResetRuntimeSettingsDefaultsWithoutStore(t *testing.T) {
	r := &router{cfg: config.Config{JWT: config.JWTConfig{Secret: "test-secret"}}, logger: slog.Default()}
	settings := r.passwordResetRuntimeSettings(context.Background())
	if settings.CodeTTL != defaultPasswordResetCodeTTL {
		t.Fatalf("CodeTTL = %v, want %v", settings.CodeTTL, defaultPasswordResetCodeTTL)
	}
	if settings.CaptchaTTL != defaultPasswordResetCaptchaTTL {
		t.Fatalf("CaptchaTTL = %v, want %v", settings.CaptchaTTL, defaultPasswordResetCaptchaTTL)
	}
	if settings.SendCooldown != defaultPasswordResetSendCooldown {
		t.Fatalf("SendCooldown = %v, want %v", settings.SendCooldown, defaultPasswordResetSendCooldown)
	}
	if settings.RateLimitSpan != defaultPasswordResetRateLimitSpan {
		t.Fatalf("RateLimitSpan = %v, want %v", settings.RateLimitSpan, defaultPasswordResetRateLimitSpan)
	}
}

func TestPasswordResetCaptchaSignature(t *testing.T) {
	r := &router{cfg: config.Config{JWT: config.JWTConfig{Secret: "test-secret"}}, logger: slog.Default()}
	token := r.signCaptcha(12, time.Now().UTC().Add(time.Minute))
	if !r.verifyCaptcha(token, "12") {
		t.Fatal("expected captcha to verify")
	}
	if r.verifyCaptcha(token, "11") {
		t.Fatal("expected wrong answer to fail")
	}
	if r.verifyCaptcha("not-a-valid-token", "12") {
		t.Fatal("expected malformed token to fail")
	}
	expired := r.signCaptcha(12, time.Now().UTC().Add(-time.Minute))
	if r.verifyCaptcha(expired, "12") {
		t.Fatal("expected expired token to fail")
	}
}

func TestPasswordResetVerificationSignature(t *testing.T) {
	r := &router{cfg: config.Config{JWT: config.JWTConfig{Secret: "test-secret"}}, logger: slog.Default()}
	token := r.signResetVerification(" admin ", time.Now().UTC().Add(time.Minute))
	if !r.verifyResetVerification(token, "admin") {
		t.Fatal("expected verification token to verify")
	}
	if !r.verifyResetVerification(token, " admin ") {
		t.Fatal("expected verification token to trim username")
	}
	if r.verifyResetVerification(token, "viewer") {
		t.Fatal("expected mismatched username to fail")
	}
	if r.verifyResetVerification("not-a-valid-token", "admin") {
		t.Fatal("expected malformed token to fail")
	}
	expired := r.signResetVerification("admin", time.Now().UTC().Add(-time.Minute))
	if r.verifyResetVerification(expired, "admin") {
		t.Fatal("expected expired token to fail")
	}
}
