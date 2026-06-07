package notification

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"kvm-manager/backend/internal/domain"
)

const (
	EventTypeProblem  = "problem"
	EventTypeRecovery = "recovery"
)

const (
	defaultProblemTemplate         = "[{{alert.level}}] {{alert.title}}\n{{alert.message}}\n来源：{{alert.sourceType}}/{{alert.sourceId}}"
	defaultRecoveryTemplate        = "[恢复] {{alert.title}}\n{{alert.message}}\n来源：{{alert.sourceType}}/{{alert.sourceId}}\n恢复时间：{{alert.resolvedAt}}\n持续时间：{{alert.duration}}"
	defaultProblemSubjectTemplate  = "{{alert.title}}"
	defaultRecoverySubjectTemplate = "恢复：{{alert.title}}"
	defaultWebhookProblemPayload   = `{"id":"{{alert.id}}","eventType":"{{event.type}}","level":"{{alert.level}}","title":"{{alert.title}}","message":"{{alert.message}}","sourceType":"{{alert.sourceType}}","sourceId":"{{alert.sourceId}}","lastSeenAt":"{{alert.lastSeenAt}}"}`
	defaultWebhookRecoveryPayload  = `{"id":"{{alert.id}}","eventType":"{{event.type}}","level":"{{alert.level}}","title":"{{alert.title}}","message":"{{alert.message}}","sourceType":"{{alert.sourceType}}","sourceId":"{{alert.sourceId}}","lastSeenAt":"{{alert.lastSeenAt}}","resolvedAt":"{{alert.resolvedAt}}","duration":"{{alert.duration}}"}`
)

var templateTokenPattern = regexp.MustCompile(`\{\{\s*([a-zA-Z0-9_.-]+)\s*\}\}`)

type AlertNotificationEvent struct {
	Type  string
	Alert domain.Alert
}

type templateConfig struct {
	ProblemTemplate           string `json:"problemTemplate"`
	RecoveryTemplate          string `json:"recoveryTemplate"`
	ProblemSubjectTemplate    string `json:"problemSubjectTemplate"`
	RecoverySubjectTemplate   string `json:"recoverySubjectTemplate"`
	LarkProblemTitleTemplate  string `json:"larkProblemTitleTemplate"`
	LarkRecoveryTitleTemplate string `json:"larkRecoveryTitleTemplate"`
	LarkProblemCardTemplate   string `json:"larkProblemCardTemplate"`
	LarkRecoveryCardTemplate  string `json:"larkRecoveryCardTemplate"`
	EmailContentType          string `json:"emailContentType"`
	LarkMessageType           string `json:"larkMessageType"`
	WechatMessageType         string `json:"wechatMessageType"`
	DingTalkMessageType       string `json:"dingtalkMessageType"`
	SendRecovery              bool   `json:"sendRecovery"`
	WebhookProblemPayload     string `json:"webhookProblemPayload"`
	WebhookRecoveryPayload    string `json:"webhookRecoveryPayload"`
}

func notificationTemplateConfig(data []byte) templateConfig {
	var cfg templateConfig
	if len(data) == 0 {
		return cfg
	}
	_ = json.Unmarshal(data, &cfg)
	return cfg
}

func ValidateTemplateConfig(data []byte) error {
	cfg := notificationTemplateConfig(data)
	testAlert := domain.Alert{
		ID:          "test",
		Level:       "warning",
		Status:      "active",
		SourceType:  "system",
		SourceID:    "template-test",
		Title:       "模板测试告警",
		Message:     "这是一条模板测试消息",
		FirstSeenAt: time.Now().UTC().Add(-10 * time.Minute),
		LastSeenAt:  time.Now().UTC(),
		Metadata:    json.RawMessage(`{"agent":"测试节点","metric":"cpu","value":90,"limit":85}`),
	}
	if strings.TrimSpace(cfg.WebhookProblemPayload) != "" {
		if _, err := alertWebhookPayload(AlertNotificationEvent{Type: EventTypeProblem, Alert: testAlert}, cfg); err != nil {
			return err
		}
	}
	resolvedAt := time.Now().UTC()
	testAlert.Status = "resolved"
	testAlert.ResolvedAt = &resolvedAt
	if strings.TrimSpace(cfg.WebhookRecoveryPayload) != "" {
		if _, err := alertWebhookPayload(AlertNotificationEvent{Type: EventTypeRecovery, Alert: testAlert}, cfg); err != nil {
			return err
		}
	}
	return nil
}

type TemplatePreview struct {
	ProblemSubject  string         `json:"problemSubject"`
	ProblemText     string         `json:"problemText"`
	ProblemWebhook  map[string]any `json:"problemWebhook,omitempty"`
	RecoverySubject string         `json:"recoverySubject"`
	RecoveryText    string         `json:"recoveryText"`
	RecoveryWebhook map[string]any `json:"recoveryWebhook,omitempty"`
	ContentType     string         `json:"contentType,omitempty"`
	MessageType     string         `json:"messageType,omitempty"`
	ProblemTitle    string         `json:"problemTitle,omitempty"`
	RecoveryTitle   string         `json:"recoveryTitle,omitempty"`
	ProblemColor    string         `json:"problemColor,omitempty"`
	RecoveryColor   string         `json:"recoveryColor,omitempty"`
}

func PreviewTemplateConfig(channelID string, data []byte) (TemplatePreview, error) {
	cfg := notificationTemplateConfig(data)
	problemAlert, recoveryAlert := templatePreviewAlerts()
	problem := AlertNotificationEvent{Type: EventTypeProblem, Alert: problemAlert}
	recovery := AlertNotificationEvent{Type: EventTypeRecovery, Alert: recoveryAlert}
	preview := TemplatePreview{
		ProblemSubject:  alertSubject(problem, cfg),
		ProblemText:     alertText(problem, cfg),
		RecoverySubject: alertSubject(recovery, cfg),
		RecoveryText:    alertText(recovery, cfg),
	}
	switch channelID {
	case "email":
		preview.ContentType = emailContentType(cfg)
	case "lark", "lark_app":
		preview.MessageType = larkMessageType(cfg)
		if preview.MessageType == "post" || preview.MessageType == "interactive" {
			preview.ProblemTitle = larkTitle(problem, cfg)
			preview.RecoveryTitle = larkTitle(recovery, cfg)
		}
		if preview.MessageType == "interactive" {
			preview.ProblemColor = larkCardTemplate(problem, cfg)
			preview.RecoveryColor = larkCardTemplate(recovery, cfg)
		}
	case "wechat", "wechat_app":
		preview.MessageType = wechatMessageType(cfg)
	case "dingtalk", "dingtalk_app":
		preview.MessageType = dingTalkMessageType(cfg)
	case "webhook":
		payload, err := alertWebhookPayload(problem, cfg)
		if err != nil {
			return TemplatePreview{}, err
		}
		preview.ProblemWebhook = payload
		payload, err = alertWebhookPayload(recovery, cfg)
		if err != nil {
			return TemplatePreview{}, err
		}
		preview.RecoveryWebhook = payload
	}
	return preview, nil
}

func templatePreviewAlerts() (domain.Alert, domain.Alert) {
	firstSeen := time.Date(2026, 6, 6, 9, 30, 0, 0, time.Local)
	lastSeen := time.Date(2026, 6, 6, 9, 45, 0, 0, time.Local)
	resolvedAt := time.Date(2026, 6, 6, 10, 5, 0, 0, time.Local)
	alert := domain.Alert{
		ID:          "preview-alert",
		Level:       "warning",
		Status:      "active",
		SourceType:  "virtual_machine",
		SourceID:    "vm-demo:cpu",
		Title:       "虚拟机CPU使用率过高",
		Message:     "虚拟机 demo CPU 使用率达到 90%",
		Metadata:    json.RawMessage(`{"agent":"node-a","vm":"demo","vmIp":"192.168.1.106","vmDescription":"demo","metric":"cpu","value":90,"limit":85,"consecutive":3}`),
		FirstSeenAt: firstSeen,
		LastSeenAt:  lastSeen,
	}
	recovery := alert
	recovery.Status = "resolved"
	recovery.ResolvedAt = &resolvedAt
	return alert, recovery
}

func alertText(event AlertNotificationEvent, cfg templateConfig) string {
	template := strings.TrimSpace(cfg.ProblemTemplate)
	if event.Type == EventTypeRecovery {
		template = strings.TrimSpace(cfg.RecoveryTemplate)
	}
	if template == "" {
		template = defaultProblemTemplate
		if event.Type == EventTypeRecovery {
			template = defaultRecoveryTemplate
		}
	}
	return renderAlertTemplate(template, event)
}

func alertSubject(event AlertNotificationEvent, cfg templateConfig) string {
	template := strings.TrimSpace(cfg.ProblemSubjectTemplate)
	if event.Type == EventTypeRecovery {
		template = strings.TrimSpace(cfg.RecoverySubjectTemplate)
	}
	if template == "" {
		template = defaultProblemSubjectTemplate
		if event.Type == EventTypeRecovery {
			template = defaultRecoverySubjectTemplate
		}
	}
	return renderAlertTemplate(template, event)
}

func alertWebhookPayload(event AlertNotificationEvent, cfg templateConfig) (map[string]any, error) {
	template := strings.TrimSpace(cfg.WebhookProblemPayload)
	if event.Type == EventTypeRecovery {
		template = strings.TrimSpace(cfg.WebhookRecoveryPayload)
	}
	if template == "" {
		template = defaultWebhookProblemPayload
		if event.Type == EventTypeRecovery {
			template = defaultWebhookRecoveryPayload
		}
	}
	rendered := renderAlertTemplate(template, event)
	var payload map[string]any
	decoder := json.NewDecoder(strings.NewReader(rendered))
	decoder.UseNumber()
	if err := decoder.Decode(&payload); err != nil {
		return nil, fmt.Errorf("Webhook JSON 模板渲染结果不是有效 JSON：%w", err)
	}
	return payload, nil
}

func renderAlertTemplate(template string, event AlertNotificationEvent) string {
	values := alertTemplateValues(event)
	return templateTokenPattern.ReplaceAllStringFunc(template, func(token string) string {
		matches := templateTokenPattern.FindStringSubmatch(token)
		if len(matches) != 2 {
			return token
		}
		return values[matches[1]]
	})
}

func alertTemplateValues(event AlertNotificationEvent) map[string]string {
	alert := event.Alert
	metadata := map[string]any{}
	if len(alert.Metadata) > 0 {
		_ = json.Unmarshal(alert.Metadata, &metadata)
	}
	values := map[string]string{
		"event.type":        event.Type,
		"event.statusText":  eventStatusText(event.Type),
		"alert.id":          alert.ID,
		"alert.level":       alert.Level,
		"alert.levelText":   alertLevelText(alert.Level),
		"alert.status":      alert.Status,
		"alert.title":       alert.Title,
		"alert.message":     alert.Message,
		"alert.sourceType":  alert.SourceType,
		"alert.sourceId":    alert.SourceID,
		"alert.firstSeenAt": formatTemplateTime(alert.FirstSeenAt),
		"alert.lastSeenAt":  formatTemplateTime(alert.LastSeenAt),
		"alert.resolvedAt":  formatOptionalTemplateTime(alert.ResolvedAt),
		"alert.duration":    alertDuration(alert),
	}
	for key, value := range metadata {
		values["metadata."+key] = stringifyTemplateValue(value)
	}
	return values
}

func eventStatusText(eventType string) string {
	if eventType == EventTypeRecovery {
		return "恢复"
	}
	return "告警"
}

func alertLevelText(level string) string {
	switch level {
	case "critical":
		return "严重"
	case "warning":
		return "警告"
	case "info":
		return "信息"
	default:
		return level
	}
}

func formatTemplateTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.Local().Format("2006-01-02 15:04:05")
}

func formatOptionalTemplateTime(value *time.Time) string {
	if value == nil {
		return ""
	}
	return formatTemplateTime(*value)
}

func alertDuration(alert domain.Alert) string {
	if alert.ResolvedAt == nil {
		return ""
	}
	duration := alert.ResolvedAt.Sub(alert.FirstSeenAt)
	if duration < 0 {
		return ""
	}
	if duration < time.Minute {
		return fmt.Sprintf("%d秒", int(duration.Seconds()))
	}
	if duration < time.Hour {
		return fmt.Sprintf("%d分钟", int(duration.Minutes()))
	}
	if duration < 24*time.Hour {
		return fmt.Sprintf("%d小时%d分钟", int(duration.Hours()), int(duration.Minutes())%60)
	}
	return fmt.Sprintf("%d天%d小时", int(duration.Hours())/24, int(duration.Hours())%24)
}

func stringifyTemplateValue(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return typed
	case json.Number:
		return typed.String()
	case float64:
		return fmt.Sprintf("%v", typed)
	case bool:
		if typed {
			return "true"
		}
		return "false"
	default:
		var buffer bytes.Buffer
		encoder := json.NewEncoder(&buffer)
		encoder.SetEscapeHTML(false)
		if err := encoder.Encode(typed); err != nil {
			return fmt.Sprintf("%v", typed)
		}
		return strings.TrimSpace(buffer.String())
	}
}
