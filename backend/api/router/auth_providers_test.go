package router

import (
	"strings"
	"testing"

	"kvm-manager/backend/internal/domain"
)

func mustAuthProvider(id, config string) *domain.AuthProvider {
	return &domain.AuthProvider{ID: id, Type: id, Enabled: true, Config: []byte(config)}
}

func TestSanitizeWeComProviderConfigRequiresFieldsWhenEnabled(t *testing.T) {
	// 外部访问地址可选：留空时运行时按当前访问地址推断回调前缀
	config, err := sanitizeAuthProviderConfigWithPrevious("wecom", map[string]any{
		"corpId":  "ww123",
		"agentId": float64(1000002),
		"secret":  "s",
	}, nil, true)
	if err != nil {
		t.Fatalf("config without external url should pass: %v", err)
	}
	if _, exists := config["externalUrl"]; exists {
		t.Fatalf("empty external url should be removed, got %v", config["externalUrl"])
	}
	config, err = sanitizeAuthProviderConfigWithPrevious("wecom", map[string]any{
		"corpId":      "ww123",
		"agentId":     float64(1000002),
		"secret":      "s",
		"externalUrl": "https://kvm.example.com/",
	}, nil, true)
	if err != nil {
		t.Fatalf("valid config should pass: %v", err)
	}
	if config["mode"] != "qrcode" {
		t.Fatalf("mode should default to qrcode, got %v", config["mode"])
	}
	if config["externalUrl"] != "https://kvm.example.com" {
		t.Fatalf("external url trailing slash should be trimmed, got %v", config["externalUrl"])
	}
}

func TestSanitizeWeComProviderConfigKeepsPreviousSecretWhenBlank(t *testing.T) {
	config, err := sanitizeAuthProviderConfigWithPrevious("wecom", map[string]any{
		"corpId":      "ww123",
		"agentId":     float64(1000002),
		"externalUrl": "https://kvm.example.com",
	}, map[string]any{"secret": "old-secret"}, true)
	if err != nil {
		t.Fatalf("blank secret should keep previous value: %v", err)
	}
	if config["secret"] != "old-secret" {
		t.Fatalf("secret should be carried over, got %v", config["secret"])
	}
}

func TestSanitizeWeComProviderConfigAllowsMockWithoutAgentAndSecret(t *testing.T) {
	config, err := sanitizeAuthProviderConfigWithPrevious("wecom", map[string]any{
		"corpId":      "ww123",
		"externalUrl": "https://kvm.example.com",
		"mock":        true,
	}, nil, true)
	if err != nil {
		t.Fatalf("mock config should not require agent id and secret: %v", err)
	}
	if config["mock"] != true {
		t.Fatalf("mock flag should be preserved, got %v", config["mock"])
	}
}

func TestSanitizeWeComProviderConfigRejectsInvalidExternalURL(t *testing.T) {
	if _, err := sanitizeAuthProviderConfigWithPrevious("wecom", map[string]any{
		"corpId":      "ww123",
		"agentId":     float64(1),
		"secret":      "s",
		"externalUrl": "ftp://kvm.example.com",
	}, nil, true); err == nil {
		t.Fatal("non http(s) external url should be rejected")
	}
}

func TestSanitizeWeComCenterProviderConfigRequiresFieldsWhenEnabled(t *testing.T) {
	_, err := sanitizeAuthProviderConfigWithPrevious("wecom_center", map[string]any{
		"baseUrl": "https://auth.example.com",
	}, nil, true)
	if err == nil || !strings.Contains(err.Error(), "应用标识") {
		t.Fatalf("app should be required, got %v", err)
	}
	config, err := sanitizeAuthProviderConfigWithPrevious("wecom_center", map[string]any{
		"baseUrl":   "https://auth.example.com",
		"app":       "kvm",
		"appSecret": "s",
	}, nil, true)
	if err != nil {
		t.Fatalf("valid config should pass: %v", err)
	}
	if config["baseUrl"] != "https://auth.example.com" {
		t.Fatalf("base url mismatch: %v", config["baseUrl"])
	}
}

func TestSanitizeWeComCenterProviderConfigKeepsPreviousAppSecretWhenBlank(t *testing.T) {
	config, err := sanitizeAuthProviderConfigWithPrevious("wecom_center", map[string]any{
		"baseUrl": "https://auth.example.com",
		"app":     "kvm",
	}, map[string]any{"appSecret": "old-secret"}, true)
	if err != nil {
		t.Fatalf("blank app secret should keep previous value: %v", err)
	}
	if config["appSecret"] != "old-secret" {
		t.Fatalf("app secret should be carried over, got %v", config["appSecret"])
	}
}

func TestSanitizeAuthProviderConfigRejectsUnknownID(t *testing.T) {
	if _, err := sanitizeAuthProviderConfigWithPrevious("other", map[string]any{}, nil, true); err == nil {
		t.Fatal("unknown provider id should be rejected")
	}
}

func TestRedactWeComProviderConfigSecrets(t *testing.T) {
	wecom := redactAuthProvider(*mustAuthProvider("wecom", `{"corpId":"ww123","secret":"plain"}`))
	config := configMap(wecom.Config)
	if stringValue(config["secret"]) != "" {
		t.Fatalf("wecom secret should be redacted, got %v", config["secret"])
	}
	if config["hasSecret"] != true {
		t.Fatalf("wecom hasSecret marker missing: %v", config)
	}

	center := redactAuthProvider(*mustAuthProvider("wecom_center", `{"app":"kvm","appSecret":"plain"}`))
	centerConfig := configMap(center.Config)
	if stringValue(centerConfig["appSecret"]) != "" {
		t.Fatalf("center appSecret should be redacted, got %v", centerConfig["appSecret"])
	}
	if centerConfig["hasAppSecret"] != true {
		t.Fatalf("center hasAppSecret marker missing: %v", centerConfig)
	}
}

func TestRedactAuthProviderConfigSecretsKeepsLDAPBehaviour(t *testing.T) {
	item := redactAuthProvider(*mustAuthProvider("ldap", `{"host":"ldap.example.com","bindPassword":"plain"}`))
	config := configMap(item.Config)
	if stringValue(config["bindPassword"]) != "" {
		t.Fatalf("ldap bindPassword should be redacted, got %v", config["bindPassword"])
	}
	if config["hasBindPassword"] != true {
		t.Fatalf("hasBindPassword marker missing: %v", config)
	}
}
