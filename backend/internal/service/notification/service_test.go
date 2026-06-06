package notification

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"kvm-manager/backend/internal/domain"
)

func TestSendPasswordResetLarkUsesGreenCard(t *testing.T) {
	var payload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg, err := json.Marshal(robotConfig{WebhookURL: server.URL})
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}

	service := NewService(nil, nil)
	err = service.SendPasswordReset(context.Background(), domain.NotificationChannel{ID: "lark", Config: cfg}, PasswordResetMessage{
		Username:  "admin",
		Code:      "123456",
		ExpiresAt: time.Date(2026, 5, 31, 2, 21, 54, 0, time.UTC),
		RequestIP: "127.0.0.1:62304",
	})
	if err != nil {
		t.Fatalf("send password reset: %v", err)
	}

	if got := payload["msg_type"]; got != "interactive" {
		t.Fatalf("msg_type = %v, want interactive", got)
	}
	card, ok := payload["card"].(map[string]any)
	if !ok {
		t.Fatalf("card payload missing or invalid: %#v", payload["card"])
	}
	header, ok := card["header"].(map[string]any)
	if !ok {
		t.Fatalf("card header missing or invalid: %#v", card["header"])
	}
	if got := header["template"]; got != "green" {
		t.Fatalf("header template = %v, want green", got)
	}
}

func TestAlertTextUsesCustomRecoveryTemplate(t *testing.T) {
	resolvedAt := time.Date(2026, 6, 6, 12, 30, 0, 0, time.UTC)
	alert := domain.Alert{
		ID:          "alert-1",
		Level:       "warning",
		Status:      "resolved",
		SourceType:  "virtual_machine",
		SourceID:    "vm-1:cpu",
		Title:       "虚拟机CPU使用率过高",
		Message:     "虚拟机 demo CPU 使用率达到 90%",
		Metadata:    json.RawMessage(`{"agent":"node-a","vm":"demo","value":90}`),
		FirstSeenAt: resolvedAt.Add(-90 * time.Minute),
		LastSeenAt:  resolvedAt.Add(-10 * time.Minute),
		ResolvedAt:  &resolvedAt,
	}

	text := alertText(AlertNotificationEvent{Type: EventTypeRecovery, Alert: alert}, templateConfig{
		RecoveryTemplate: "{{event.statusText}} {{alert.title}} {{metadata.vm}} {{alert.duration}}",
	})

	if text != "恢复 虚拟机CPU使用率过高 demo 1小时30分钟" {
		t.Fatalf("text = %q", text)
	}
}

func TestAlertWebhookPayloadUsesCustomJSONTemplate(t *testing.T) {
	alert := domain.Alert{
		ID:         "alert-1",
		Level:      "critical",
		Status:     "active",
		SourceType: "agent",
		SourceID:   "agent-1",
		Title:      "Agent node-a 离线",
		Message:    "连续同步失败，Agent 已标记为离线",
		Metadata:   json.RawMessage(`{"agent":"node-a","failureCount":3}`),
		LastSeenAt: time.Date(2026, 6, 6, 12, 0, 0, 0, time.UTC),
	}

	payload, err := alertWebhookPayload(AlertNotificationEvent{Type: EventTypeProblem, Alert: alert}, templateConfig{
		WebhookProblemPayload: `{"event":"{{event.type}}","name":"{{metadata.agent}}","title":"{{alert.title}}"}`,
	})
	if err != nil {
		t.Fatalf("alertWebhookPayload: %v", err)
	}
	if payload["event"] != "problem" || payload["name"] != "node-a" || payload["title"] != "Agent node-a 离线" {
		t.Fatalf("payload = %#v", payload)
	}
}

func TestAlertSubjectUsesCustomTemplates(t *testing.T) {
	alert := domain.Alert{
		ID:         "alert-1",
		Level:      "warning",
		Status:     "resolved",
		SourceType: "host",
		SourceID:   "agent-1:cpu",
		Title:      "宿主机CPU使用率过高",
	}

	got := alertSubject(AlertNotificationEvent{Type: EventTypeRecovery, Alert: alert}, templateConfig{
		RecoverySubjectTemplate: "OK {{alert.title}} {{alert.levelText}}",
	})
	if got != "OK 宿主机CPU使用率过高 警告" {
		t.Fatalf("subject = %q", got)
	}
}

func TestLarkAlertPayloadSupportsPostAndInteractive(t *testing.T) {
	alert := domain.Alert{ID: "alert-1", Level: "critical", Title: "Agent 离线", Message: "连续同步失败"}
	event := AlertNotificationEvent{Type: EventTypeProblem, Alert: alert}

	post := larkAlertPayload(event, templateConfig{LarkMessageType: "post"})
	if post["msg_type"] != "post" {
		t.Fatalf("post msg_type = %v", post["msg_type"])
	}
	content, ok := post["content"].(map[string]any)
	if !ok || content["post"] == nil {
		t.Fatalf("post content invalid: %#v", post)
	}

	card := larkAlertPayload(event, templateConfig{LarkMessageType: "interactive"})
	if card["msg_type"] != "interactive" || card["card"] == nil {
		t.Fatalf("interactive payload invalid: %#v", card)
	}
}

func TestAlertMessageTypeDefaults(t *testing.T) {
	if got := emailContentType(templateConfig{}); got != "text/plain" {
		t.Fatalf("email content type = %q", got)
	}
	if got := emailContentType(templateConfig{EmailContentType: "text/html"}); got != "text/html" {
		t.Fatalf("email html content type = %q", got)
	}
	if got := wechatMessageType(templateConfig{WechatMessageType: "markdown"}); got != "markdown" {
		t.Fatalf("wechat message type = %q", got)
	}
	if got := dingTalkMessageType(templateConfig{DingTalkMessageType: "markdown"}); got != "markdown" {
		t.Fatalf("dingtalk message type = %q", got)
	}
}
