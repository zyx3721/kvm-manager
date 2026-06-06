package router

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strings"

	"kvm-manager/backend/internal/domain"
	"kvm-manager/backend/internal/repository"
	"kvm-manager/backend/internal/service/notification"
)

var notificationChannelIDs = map[string]struct{}{
	"webhook":  {},
	"email":    {},
	"lark":     {},
	"wechat":   {},
	"dingtalk": {},
}

type notificationChannelRequest struct {
	Enabled              bool           `json:"enabled"`
	PasswordResetEnabled bool           `json:"passwordResetEnabled"`
	Config               map[string]any `json:"config"`
}

func (r *router) handleListNotificationChannels(w http.ResponseWriter, req *http.Request) {
	items, err := r.store.ListNotificationChannels(req.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list_notifications_failed", "读取通知配置失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": redactNotificationChannels(items), "total": len(items)})
}

func (r *router) handleNotificationRoute(w http.ResponseWriter, req *http.Request) {
	id, action, ok := parseNotificationPath(req.URL.Path)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", "接口不存在")
		return
	}
	if req.Method == http.MethodPut && action == "" {
		r.handleUpdateNotificationChannel(w, req, id)
		return
	}
	if req.Method == http.MethodPost && action == "test" {
		r.handleTestNotificationChannel(w, req, id)
		return
	}
	writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "请求方法不支持")
}

func (r *router) handleUpdateNotificationChannel(w http.ResponseWriter, req *http.Request, id string) {
	if !isNotificationChannelID(id) {
		writeError(w, http.StatusNotFound, "notification_not_found", "通知媒介不存在")
		return
	}
	defer req.Body.Close()
	var body notificationChannelRequest
	if err := decodeJSONBody(w, req, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "通知配置格式不正确")
		return
	}
	previous, _ := r.store.GetNotificationChannel(req.Context(), id)
	config, err := sanitizeNotificationConfigWithPrevious(id, body.Config, notificationConfigMap(previous.Config), body.Enabled || body.PasswordResetEnabled)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_notification_config", notification.UserFacingErrorMessage(err))
		return
	}
	item, err := r.store.UpsertNotificationChannel(req.Context(), id, body.Enabled, body.PasswordResetEnabled, config)
	if err != nil {
		r.logger.Error("save notification channel failed", "error", err, "channel", id)
		writeError(w, http.StatusInternalServerError, "save_notification_failed", "保存通知配置失败")
		return
	}
	_ = r.store.WriteAudit(req.Context(), currentSession(req).User.ID, "settings.notification.update", "notification_channel", id, repository.ClientIP(req), map[string]any{"enabled": body.Enabled, "passwordResetEnabled": body.PasswordResetEnabled})
	writeJSON(w, http.StatusOK, redactNotificationChannel(item))
}

func (r *router) handleTestNotificationChannel(w http.ResponseWriter, req *http.Request, id string) {
	if !isNotificationChannelID(id) {
		writeError(w, http.StatusNotFound, "notification_not_found", "通知媒介不存在")
		return
	}
	channel, err := r.store.GetNotificationChannel(req.Context(), id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeError(w, http.StatusNotFound, "notification_not_found", "通知媒介不存在")
			return
		}
		writeError(w, http.StatusInternalServerError, "get_notification_failed", "读取通知配置失败")
		return
	}
	if err := r.notify.SendTest(req.Context(), channel); err != nil {
		writeError(w, http.StatusServiceUnavailable, "notification_test_failed", notification.UserFacingErrorMessage(err))
		return
	}
	_ = r.store.WriteAudit(req.Context(), currentSession(req).User.ID, "settings.notification.test", "notification_channel", id, repository.ClientIP(req), map[string]any{})
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func isNotificationChannelID(id string) bool {
	_, ok := notificationChannelIDs[id]
	return ok
}

func parseNotificationPath(path string) (id string, action string, ok bool) {
	trimmed := strings.Trim(strings.TrimPrefix(path, "/api/settings/notifications/"), "/")
	parts := strings.Split(trimmed, "/")
	if len(parts) == 1 && parts[0] != "" {
		return parts[0], "", true
	}
	if len(parts) == 2 && parts[0] != "" && parts[1] != "" {
		return parts[0], parts[1], true
	}
	return "", "", false
}

func sanitizeNotificationConfig(id string, config map[string]any, enabled bool) (map[string]any, error) {
	return sanitizeNotificationConfigWithPrevious(id, config, nil, enabled)
}

func sanitizeNotificationConfigWithPrevious(id string, config map[string]any, previous map[string]any, enabled bool) (map[string]any, error) {
	if config == nil {
		config = map[string]any{}
	}
	for key, value := range config {
		if text, ok := value.(string); ok {
			config[key] = strings.TrimSpace(text)
		}
	}
	discardSecretPresenceMarkers(config, notificationSecretKeys(id))
	if !enabled {
		return removeEmptyConfigValues(config), nil
	}
	mergeRetainedNotificationSecrets(id, config, previous)
	switch id {
	case "webhook":
		if strings.TrimSpace(stringValue(config["url"])) == "" {
			return nil, fmt.Errorf("Webhook URL 不能为空")
		}
		method := strings.ToUpper(strings.TrimSpace(stringValue(config["method"])))
		if method != "" && method != http.MethodPost && method != http.MethodPut && method != http.MethodPatch {
			return nil, fmt.Errorf("Webhook 请求方法仅支持 POST、PUT 或 PATCH")
		}
		if method != "" {
			config["method"] = method
		}
		if stringValue(config["headers"]) == "" {
			delete(config, "headers")
		} else if _, ok := config["headers"].(map[string]any); !ok {
			if _, ok := config["headers"].(map[string]string); !ok {
				return nil, fmt.Errorf("请求头 JSON 格式不正确")
			}
		}
		return removeEmptyConfigValues(config), nil
	case "email":
		if value, ok := config["to"].(string); ok {
			parts := strings.Split(value, ",")
			items := make([]string, 0, len(parts))
			for _, part := range parts {
				if trimmed := strings.TrimSpace(part); trimmed != "" {
					items = append(items, trimmed)
				}
			}
			config["to"] = items
		}
		if boolValue(config["useTLS"]) && boolValue(config["startTLS"]) {
			return nil, fmt.Errorf("TLS 与 STARTTLS 不能同时启用")
		}
		requiredFields := []struct {
			key   string
			label string
		}{
			{key: "smtpHost", label: "SMTP 主机"},
			{key: "username", label: "用户名"},
			{key: "password", label: "密码"},
			{key: "from", label: "发件人"},
		}
		for _, field := range requiredFields {
			if stringValue(config[field.key]) == "" {
				return nil, fmt.Errorf("%s不能为空", field.label)
			}
		}
		if len(stringList(config["to"])) == 0 {
			return nil, fmt.Errorf("收件人不能为空")
		}
		if boolValue(config["useTLS"]) {
			config["smtpPort"] = 465
		} else if boolValue(config["startTLS"]) {
			config["smtpPort"] = 587
		} else if numberValue(config["smtpPort"]) <= 0 {
			return nil, fmt.Errorf("SMTP 端口不能为空")
		}
	case "lark", "wechat", "dingtalk":
		if strings.TrimSpace(stringValue(config["webhookUrl"])) == "" {
			return nil, fmt.Errorf("机器人 Webhook 不能为空")
		}
	}
	return config, nil
}

func mergeRetainedNotificationSecrets(id string, config map[string]any, previous map[string]any) {
	if len(previous) == 0 {
		return
	}
	for _, key := range notificationSecretKeys(id) {
		if stringValue(config[key]) == "" {
			if value := stringValue(previous[key]); value != "" {
				config[key] = value
			}
		}
	}
}

func notificationSecretKeys(id string) []string {
	switch id {
	case "email":
		return []string{"password"}
	case "lark", "dingtalk":
		return []string{"secret"}
	default:
		return nil
	}
}

func redactNotificationChannels(items []domain.NotificationChannel) []domain.NotificationChannel {
	redacted := make([]domain.NotificationChannel, len(items))
	for index, item := range items {
		redacted[index] = redactNotificationChannel(item)
	}
	return redacted
}

func redactNotificationChannel(item domain.NotificationChannel) domain.NotificationChannel {
	item.Config = redactConfigSecrets(item.Config, notificationSecretKeys(item.ID))
	return item
}

func stringValue(value any) string {
	text, _ := value.(string)
	return strings.TrimSpace(text)
}

func removeEmptyConfigValues(config map[string]any) map[string]any {
	cleaned := make(map[string]any, len(config))
	for key, value := range config {
		switch typed := value.(type) {
		case string:
			if strings.TrimSpace(typed) != "" {
				cleaned[key] = strings.TrimSpace(typed)
			}
		case []string:
			items := make([]string, 0, len(typed))
			for _, item := range typed {
				if trimmed := strings.TrimSpace(item); trimmed != "" {
					items = append(items, trimmed)
				}
			}
			if len(items) > 0 {
				cleaned[key] = items
			}
		case []any:
			items := make([]any, 0, len(typed))
			for _, item := range typed {
				if text, ok := item.(string); ok {
					if trimmed := strings.TrimSpace(text); trimmed != "" {
						items = append(items, trimmed)
					}
					continue
				}
				if item != nil {
					items = append(items, item)
				}
			}
			if len(items) > 0 {
				cleaned[key] = items
			}
		case map[string]any:
			if len(removeEmptyConfigValues(typed)) > 0 {
				cleaned[key] = removeEmptyConfigValues(typed)
			}
		case map[string]string:
			items := make(map[string]string, len(typed))
			for itemKey, itemValue := range typed {
				if trimmed := strings.TrimSpace(itemValue); trimmed != "" {
					items[itemKey] = trimmed
				}
			}
			if len(items) > 0 {
				cleaned[key] = items
			}
		case nil:
			continue
		case float64:
			if math.IsNaN(typed) || math.IsInf(typed, 0) || typed <= 0 {
				continue
			}
			cleaned[key] = typed
		case float32:
			if math.IsNaN(float64(typed)) || math.IsInf(float64(typed), 0) || typed <= 0 {
				continue
			}
			cleaned[key] = typed
		case int:
			if typed <= 0 {
				continue
			}
			cleaned[key] = typed
		case int8:
			if typed <= 0 {
				continue
			}
			cleaned[key] = typed
		case int16:
			if typed <= 0 {
				continue
			}
			cleaned[key] = typed
		case int32:
			if typed <= 0 {
				continue
			}
			cleaned[key] = typed
		case int64:
			if typed <= 0 {
				continue
			}
			cleaned[key] = typed
		case uint:
			if typed == 0 {
				continue
			}
			cleaned[key] = typed
		case uint8:
			if typed == 0 {
				continue
			}
			cleaned[key] = typed
		case uint16:
			if typed == 0 {
				continue
			}
			cleaned[key] = typed
		case uint32:
			if typed == 0 {
				continue
			}
			cleaned[key] = typed
		case uint64:
			if typed == 0 {
				continue
			}
			cleaned[key] = typed
		default:
			cleaned[key] = value
		}
	}
	return cleaned
}

func numberValue(value any) float64 {
	switch typed := value.(type) {
	case float64:
		return typed
	case int:
		return float64(typed)
	case json.Number:
		number, _ := typed.Float64()
		return number
	default:
		return 0
	}
}

func stringList(value any) []string {
	switch typed := value.(type) {
	case []string:
		return typed
	case []any:
		items := make([]string, 0, len(typed))
		for _, item := range typed {
			if text := stringValue(item); text != "" {
				items = append(items, text)
			}
		}
		return items
	default:
		return nil
	}
}
