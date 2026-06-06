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
