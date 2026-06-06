package router

import (
	"encoding/json"
	"strings"
)

func notificationConfigMap(raw json.RawMessage) map[string]any {
	return configMap(raw)
}

func redactConfigSecrets(raw json.RawMessage, keys []string) json.RawMessage {
	if len(raw) == 0 || len(keys) == 0 {
		return raw
	}
	config := configMap(raw)
	if len(config) == 0 {
		return raw
	}
	changed := false
	for _, key := range keys {
		if stringValue(config[key]) == "" {
			continue
		}
		delete(config, key)
		config[secretPresenceKey(key)] = true
		changed = true
	}
	if !changed {
		return raw
	}
	payload, err := json.Marshal(config)
	if err != nil {
		return raw
	}
	return payload
}

func discardSecretPresenceMarkers(config map[string]any, keys []string) {
	for _, key := range keys {
		delete(config, secretPresenceKey(key))
	}
}

func secretPresenceKey(key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return "hasSecret"
	}
	return "has" + strings.ToUpper(key[:1]) + key[1:]
}
