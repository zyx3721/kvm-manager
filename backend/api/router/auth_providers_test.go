package router

import (
	"strings"
	"testing"

	"kvm-manager/backend/internal/domain"
)

func mustAuthProvider(id, config string) *domain.AuthProvider {
	return &domain.AuthProvider{ID: id, Type: id, Enabled: true, Config: []byte(config)}
}

func TestSanitizeWecomProviderConfigRequiresDirectFieldsWhenEnabled(t *testing.T) {
	_, err := sanitizeAuthProviderConfigWithPrevious("wecom", map[string]any{
		"authMode": "direct",
		"corpid":   "ww123",
	}, nil, true)
	if err == nil || !strings.Contains(err.Error(), "AgentID") {
		t.Fatalf("agentid should be required, got %v", err)
	}
	config, err := sanitizeAuthProviderConfigWithPrevious("wecom", map[string]any{
		"authMode": "direct",
		"corpid":   "ww123",
		"agentid":  float64(1000002),
		"secret":   "s",
	}, nil, true)
	if err != nil {
		t.Fatalf("valid direct config should pass: %v", err)
	}
	if config["authMode"] != "direct" {
		t.Fatalf("authMode mismatch: %v", config["authMode"])
	}
}

func TestSanitizeWecomProviderConfigRequiresSSOFieldsWhenEnabled(t *testing.T) {
	_, err := sanitizeAuthProviderConfigWithPrevious("wecom", map[string]any{
		"authMode": "sso",
	}, nil, true)
	if err == nil || !strings.Contains(err.Error(), "认证中心地址") {
		t.Fatalf("sso base url should be required, got %v", err)
	}
	config, err := sanitizeAuthProviderConfigWithPrevious("wecom", map[string]any{
		"authMode":     "sso",
		"ssoBaseUrl":   "https://auth.example.com/",
		"ssoAppID":     "kvm",
		"ssoAppSecret": "s",
	}, nil, true)
	if err != nil {
		t.Fatalf("valid sso config should pass: %v", err)
	}
	if config["ssoBaseUrl"] != "https://auth.example.com" {
		t.Fatalf("sso base url trailing slash should be trimmed, got %v", config["ssoBaseUrl"])
	}
	// sso 模式不要求直连凭据，切换模式互不丢失；应用密钥留空时续存旧值
	config, err = sanitizeAuthProviderConfigWithPrevious("wecom", map[string]any{
		"authMode":   "sso",
		"ssoBaseUrl": "https://auth.example.com",
		"ssoAppID":   "kvm",
	}, map[string]any{"ssoAppSecret": "kept"}, true)
	if err != nil {
		t.Fatalf("sso config with previous secret should pass: %v", err)
	}
	if config["ssoAppSecret"] != "kept" {
		t.Fatalf("sso app secret should be carried over, got %v", config["ssoAppSecret"])
	}
}

func TestSanitizeWecomProviderConfigRejectsUnknownAuthMode(t *testing.T) {
	if _, err := sanitizeAuthProviderConfigWithPrevious("wecom", map[string]any{
		"authMode": "other",
	}, nil, true); err == nil {
		t.Fatal("unknown auth mode should be rejected")
	}
}

func TestSanitizeWecomProviderConfigKeepsPreviousSecretsWhenBlank(t *testing.T) {
	config, err := sanitizeAuthProviderConfigWithPrevious("wecom", map[string]any{
		"authMode": "direct",
		"corpid":   "ww123",
		"agentid":  float64(1000002),
	}, map[string]any{"secret": "old-secret", "ssoAppSecret": "old-sso-secret"}, true)
	if err != nil {
		t.Fatalf("blank secrets should keep previous values: %v", err)
	}
	if config["secret"] != "old-secret" {
		t.Fatalf("secret should be carried over, got %v", config["secret"])
	}
	if config["ssoAppSecret"] != "old-sso-secret" {
		t.Fatalf("sso app secret should be carried over, got %v", config["ssoAppSecret"])
	}
}

func TestSanitizeWecomProviderConfigRejectsInvalidRedirectPrefix(t *testing.T) {
	if _, err := sanitizeAuthProviderConfigWithPrevious("wecom", map[string]any{
		"authMode":       "direct",
		"corpid":         "ww123",
		"agentid":        float64(1),
		"secret":         "s",
		"redirectPrefix": "ftp://kvm.example.com",
	}, nil, true); err == nil {
		t.Fatal("non http(s) redirect prefix should be rejected")
	}
}

func TestSanitizeWecomProviderConfigAllowsEmptyRedirectPrefix(t *testing.T) {
	// 回调地址前缀可选：留空时运行时按当前访问地址推断
	config, err := sanitizeAuthProviderConfigWithPrevious("wecom", map[string]any{
		"authMode": "direct",
		"corpid":   "ww123",
		"agentid":  float64(1),
		"secret":   "s",
	}, nil, true)
	if err != nil {
		t.Fatalf("empty redirect prefix should be allowed: %v", err)
	}
	if _, exists := config["redirectPrefix"]; exists {
		t.Fatalf("empty redirect prefix should be removed, got %v", config["redirectPrefix"])
	}
}

func TestSanitizeAuthProviderConfigRejectsUnknownID(t *testing.T) {
	if _, err := sanitizeAuthProviderConfigWithPrevious("other", map[string]any{}, nil, true); err == nil {
		t.Fatal("unknown provider id should be rejected")
	}
}

func TestRedactWecomProviderConfigSecrets(t *testing.T) {
	item := redactAuthProvider(*mustAuthProvider("wecom", `{"authMode":"sso","corpid":"ww123","secret":"plain","ssoAppSecret":"plain-sso"}`))
	config := configMap(item.Config)
	if stringValue(config["secret"]) != "" || stringValue(config["ssoAppSecret"]) != "" {
		t.Fatalf("wecom secrets should be redacted, got %v", config)
	}
	if config["hasSecret"] != true || config["hasSsoAppSecret"] != true {
		t.Fatalf("secret presence markers missing: %v", config)
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
