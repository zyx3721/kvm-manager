package router

import (
	"encoding/json"
	"testing"

	"kvm-manager/backend/internal/domain"
)

func TestSanitizeNotificationConfigAllowsEmptyWhenDisabled(t *testing.T) {
	config, err := sanitizeNotificationConfig("webhook", map[string]any{"url": ""}, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(config) != 0 {
		t.Fatalf("expected empty config, got %#v", config)
	}
}

func TestSanitizeNotificationConfigRequiresFieldsWhenEnabled(t *testing.T) {
	if _, err := sanitizeNotificationConfig("webhook", map[string]any{"url": ""}, true); err == nil {
		t.Fatal("expected required field error")
	}
}

func TestSanitizeWebhookNotificationConfigDoesNotBackfillOptionalFields(t *testing.T) {
	config, err := sanitizeNotificationConfig("webhook", map[string]any{"url": "https://example.com/hook"}, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := config["method"]; ok {
		t.Fatalf("expected method to stay omitted, got %#v", config["method"])
	}
	if _, ok := config["headers"]; ok {
		t.Fatalf("expected headers to stay omitted, got %#v", config["headers"])
	}
}

func TestSanitizeWebhookNotificationConfigKeepsExplicitMethod(t *testing.T) {
	config, err := sanitizeNotificationConfig("webhook", map[string]any{"url": "https://example.com/hook", "method": "put"}, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := config["method"]; got != "PUT" {
		t.Fatalf("method = %#v, want PUT", got)
	}
}

func TestSanitizeWebhookNotificationConfigRejectsInvalidHeaders(t *testing.T) {
	if _, err := sanitizeNotificationConfig("webhook", map[string]any{"url": "https://example.com/hook", "headers": "X-Test: yes"}, true); err == nil {
		t.Fatal("expected invalid headers to be rejected")
	}
}

func TestSanitizeEmailNotificationConfigRequiresAllFieldsWhenEnabled(t *testing.T) {
	valid := map[string]any{
		"smtpHost": "smtp.example.com",
		"smtpPort": float64(465),
		"username": "alert@example.com",
		"password": "secret",
		"from":     "alert@example.com",
		"to":       "ops@example.com",
	}
	for _, field := range []string{"smtpHost", "smtpPort", "username", "password", "from", "to"} {
		config := make(map[string]any, len(valid))
		for key, value := range valid {
			config[key] = value
		}
		if field == "smtpPort" {
			config[field] = float64(0)
		} else {
			config[field] = ""
		}
		if _, err := sanitizeNotificationConfig("email", config, true); err == nil {
			t.Fatalf("expected required field error for %s", field)
		}
	}
}

func TestSanitizeEmailNotificationConfigSplitsAndTrimsRecipients(t *testing.T) {
	config, err := sanitizeNotificationConfig("email", map[string]any{
		"smtpHost": " smtp.example.com ",
		"smtpPort": float64(25),
		"username": " alert@example.com ",
		"password": " unit-test-placeholder-secret ",
		"from":     " alert@example.com ",
		"to":       " ops@example.com, admin@example.com , ",
	}, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := stringList(config["to"])
	want := []string{"ops@example.com", "admin@example.com"}
	if len(got) != len(want) {
		t.Fatalf("to = %#v, want %#v", got, want)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("to = %#v, want %#v", got, want)
		}
	}
	if gotHost := config["smtpHost"]; gotHost != "smtp.example.com" {
		t.Fatalf("smtpHost = %#v, want smtp.example.com", gotHost)
	}
}

func TestSanitizeEmailNotificationConfigForcesTLSDefaultPorts(t *testing.T) {
	for name, input := range map[string]struct {
		useTLS   bool
		startTLS bool
		want     int
	}{
		"tls":      {useTLS: true, want: 465},
		"starttls": {startTLS: true, want: 587},
	} {
		t.Run(name, func(t *testing.T) {
			config, err := sanitizeNotificationConfig("email", map[string]any{
				"smtpHost": "smtp.example.com",
				"smtpPort": float64(2525),
				"username": "alert@example.com",
				"password": "secret",
				"from":     "alert@example.com",
				"to":       "ops@example.com",
				"useTLS":   input.useTLS,
				"startTLS": input.startTLS,
			}, true)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got := numberValue(config["smtpPort"]); got != float64(input.want) {
				t.Fatalf("smtpPort = %v, want %d", got, input.want)
			}
		})
	}
}

func TestSanitizeEmailNotificationConfigRejectsMutualEncryptionModes(t *testing.T) {
	_, err := sanitizeNotificationConfig("email", map[string]any{
		"smtpHost": "smtp.example.com",
		"smtpPort": float64(465),
		"username": "alert@example.com",
		"password": "secret",
		"from":     "alert@example.com",
		"to":       "ops@example.com",
		"useTLS":   true,
		"startTLS": true,
	}, true)
	if err == nil {
		t.Fatal("expected mutual encryption mode error")
	}
}

func TestSanitizeEmailNotificationConfigKeepsPreviousPasswordWhenBlank(t *testing.T) {
	config, err := sanitizeNotificationConfigWithPrevious("email", map[string]any{
		"smtpHost":    "smtp.example.com",
		"smtpPort":    float64(465),
		"username":    "alert@example.com",
		"password":    "",
		"hasPassword": true,
		"from":        "alert@example.com",
		"to":          "ops@example.com",
	}, map[string]any{"password": "old-secret"}, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := config["password"]; got != "old-secret" {
		t.Fatalf("password = %#v, want old-secret", got)
	}
	if _, ok := config["hasPassword"]; ok {
		t.Fatalf("expected hasPassword marker to be discarded, got %#v", config["hasPassword"])
	}
}

func TestRedactNotificationConfigSecrets(t *testing.T) {
	item := redactNotificationChannel(testNotificationChannel("email", map[string]any{"password": "secret", "smtpHost": "smtp.example.com"}))
	config := decodeRawConfig(t, item.Config)
	if _, ok := config["password"]; ok {
		t.Fatalf("expected password to be redacted, got %#v", config["password"])
	}
	if config["hasPassword"] != true {
		t.Fatalf("expected hasPassword marker, got %#v", config["hasPassword"])
	}
	if config["smtpHost"] != "smtp.example.com" {
		t.Fatalf("expected non-secret field to stay visible, got %#v", config["smtpHost"])
	}
}

func TestSanitizeRobotNotificationConfigKeepsPreviousSecretWhenBlank(t *testing.T) {
	config, err := sanitizeNotificationConfigWithPrevious("lark", map[string]any{
		"webhookUrl": "https://example.com/lark",
		"secret":     "",
		"hasSecret":  true,
	}, map[string]any{"secret": "unit-test-placeholder-secret"}, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := config["secret"]; got != "unit-test-placeholder-secret" {
		t.Fatalf("secret = %#v, want retained placeholder secret", got)
	}
	if _, ok := config["hasSecret"]; ok {
		t.Fatalf("expected hasSecret marker to be discarded, got %#v", config["hasSecret"])
	}
}

func TestRedactRobotNotificationConfigSecrets(t *testing.T) {
	item := redactNotificationChannel(testNotificationChannel("dingtalk", map[string]any{"secret": "unit-test-placeholder-secret", "webhookUrl": "https://example.com/dingtalk"}))
	config := decodeRawConfig(t, item.Config)
	if _, ok := config["secret"]; ok {
		t.Fatalf("expected secret to be redacted, got %#v", config["secret"])
	}
	if config["hasSecret"] != true {
		t.Fatalf("expected hasSecret marker, got %#v", config["hasSecret"])
	}
	if config["webhookUrl"] != "https://example.com/dingtalk" {
		t.Fatalf("expected non-secret field to stay visible, got %#v", config["webhookUrl"])
	}
}

func TestSanitizeAuthProviderConfigAllowsEmptyWhenDisabled(t *testing.T) {
	config, err := sanitizeAuthProviderConfig("ldap", map[string]any{"host": "", "baseDN": ""}, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(config) != 0 {
		t.Fatalf("expected empty config, got %#v", config)
	}
}

func TestSanitizeAuthProviderConfigRequiresFieldsWhenEnabled(t *testing.T) {
	if _, err := sanitizeAuthProviderConfig("ldap", map[string]any{"host": "", "baseDN": ""}, true); err == nil {
		t.Fatal("expected required field error")
	}
}

func TestSanitizeAuthProviderConfigRequiresAllRequiredFieldsWhenEnabled(t *testing.T) {
	valid := map[string]any{
		"host":         "ldap.example.com",
		"port":         float64(389),
		"baseDN":       "dc=example,dc=com",
		"userFilter":   "(sAMAccountName={username})",
		"bindDN":       "cn=readonly,dc=example,dc=com",
		"bindPassword": "secret",
	}
	for _, field := range []string{"host", "port", "baseDN", "userFilter", "bindDN", "bindPassword"} {
		config := make(map[string]any, len(valid))
		for key, value := range valid {
			config[key] = value
		}
		if field == "port" {
			config[field] = float64(0)
		} else {
			config[field] = ""
		}
		if _, err := sanitizeAuthProviderConfig("ldap", config, true); err == nil {
			t.Fatalf("expected required field error for %s", field)
		}
	}
}

func TestSanitizeAuthProviderConfigRemovesHiddenAndEmptyOptionalFields(t *testing.T) {
	config, err := sanitizeAuthProviderConfig("ldap", map[string]any{
		"host":                 "ldap.example.com",
		"port":                 float64(389),
		"baseDN":               "dc=example,dc=com",
		"userFilter":           "(sAMAccountName={username})",
		"bindDN":               "cn=readonly,dc=example,dc=com",
		"bindPassword":         "secret",
		"usernameAttribute":    "",
		"displayNameAttribute": "displayName",
		"emailAttribute":       "mail",
		"timeoutSeconds":       float64(0),
		"defaultRole":          "operator",
		"adminGroupDN":         "cn=admins,dc=example,dc=com",
	}, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, field := range []string{"usernameAttribute", "displayNameAttribute", "emailAttribute", "timeoutSeconds", "defaultRole", "adminGroupDN"} {
		if _, ok := config[field]; ok {
			t.Fatalf("expected optional field %s to be removed, got %#v", field, config[field])
		}
	}
}

func TestSanitizeAuthProviderConfigRejectsMutualTLSModes(t *testing.T) {
	_, err := sanitizeAuthProviderConfig("ldap", map[string]any{
		"host":         "ldap.example.com",
		"port":         float64(636),
		"baseDN":       "dc=example,dc=com",
		"userFilter":   "(sAMAccountName={username})",
		"bindDN":       "cn=readonly,dc=example,dc=com",
		"bindPassword": "secret",
		"useTLS":       true,
		"startTLS":     true,
	}, true)
	if err == nil {
		t.Fatal("expected mutual TLS mode error")
	}
}

func TestSanitizeAuthProviderConfigForcesTLSDefaultPorts(t *testing.T) {
	for name, input := range map[string]struct {
		useTLS   bool
		startTLS bool
		want     int
	}{
		"ldaps":    {useTLS: true, want: 636},
		"starttls": {startTLS: true, want: 389},
	} {
		t.Run(name, func(t *testing.T) {
			config, err := sanitizeAuthProviderConfig("ldap", map[string]any{
				"host":         "ldap.example.com",
				"port":         float64(1234),
				"baseDN":       "dc=example,dc=com",
				"userFilter":   "(sAMAccountName={username})",
				"bindDN":       "cn=readonly,dc=example,dc=com",
				"bindPassword": "secret",
				"useTLS":       input.useTLS,
				"startTLS":     input.startTLS,
			}, true)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got := numberValue(config["port"]); got != float64(input.want) {
				t.Fatalf("port = %v, want %d", got, input.want)
			}
		})
	}
}

func TestSanitizeAuthProviderConfigKeepsPreviousBindPasswordWhenBlank(t *testing.T) {
	config, err := sanitizeAuthProviderConfigWithPrevious("ldap", map[string]any{
		"host":            "ldap.example.com",
		"port":            float64(389),
		"baseDN":          "dc=example,dc=com",
		"userFilter":      "(sAMAccountName={username})",
		"bindDN":          "cn=readonly,dc=example,dc=com",
		"bindPassword":    "",
		"hasBindPassword": true,
	}, map[string]any{"bindPassword": "old-secret"}, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := config["bindPassword"]; got != "old-secret" {
		t.Fatalf("bindPassword = %#v, want old-secret", got)
	}
	if _, ok := config["hasBindPassword"]; ok {
		t.Fatalf("expected hasBindPassword marker to be discarded, got %#v", config["hasBindPassword"])
	}
}

func TestRedactAuthProviderConfigSecrets(t *testing.T) {
	item := redactAuthProvider(testAuthProvider(map[string]any{"bindPassword": "secret", "host": "ldap.example.com"}))
	config := decodeRawConfig(t, item.Config)
	if _, ok := config["bindPassword"]; ok {
		t.Fatalf("expected bindPassword to be redacted, got %#v", config["bindPassword"])
	}
	if config["hasBindPassword"] != true {
		t.Fatalf("expected hasBindPassword marker, got %#v", config["hasBindPassword"])
	}
	if config["host"] != "ldap.example.com" {
		t.Fatalf("expected non-secret field to stay visible, got %#v", config["host"])
	}
}

func decodeRawConfig(t *testing.T, raw json.RawMessage) map[string]any {
	t.Helper()
	var config map[string]any
	if err := json.Unmarshal(raw, &config); err != nil {
		t.Fatalf("decode config: %v", err)
	}
	return config
}

func testNotificationChannel(id string, config map[string]any) domain.NotificationChannel {
	payload, _ := json.Marshal(config)
	return domain.NotificationChannel{ID: id, Config: payload}
}

func testAuthProvider(config map[string]any) domain.AuthProvider {
	payload, _ := json.Marshal(config)
	return domain.AuthProvider{ID: "ldap", Type: "ldap", Name: "AD/LDAP", Config: payload}
}
