package notification

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/mail"
	"net/smtp"
	"net/url"
	"strconv"
	"strings"
	"time"

	"kvm-manager/backend/internal/domain"
	"kvm-manager/backend/internal/repository"
)

type Store interface {
	ListNotificationChannels(ctx context.Context) ([]domain.NotificationChannel, error)
	GetSystemBaseConfig(ctx context.Context) (domain.SystemBaseConfig, error)
	MarkAlertNotificationSent(ctx context.Context, id string) error
	MarkAlertNotificationDeliverySent(ctx context.Context, id string) error
	MarkAlertNotificationDeliveryFailed(ctx context.Context, id string, message string) error
	MarkAlertNotificationDeliveryFailedWithConfig(ctx context.Context, id string, message string, config repository.AlertNotificationRetryConfig) error
}

type Service struct {
	store            Store
	logger           *slog.Logger
	http             *http.Client
	alertHTTPTimeout time.Duration
}

func NewService(store Store, logger *slog.Logger) *Service {
	return &Service{store: store, logger: logger, http: &http.Client{Timeout: 8 * time.Second}}
}

type alertNotificationRuntimeConfig struct {
	Timeout time.Duration
	Retry   repository.AlertNotificationRetryConfig
}

func (s *Service) NotifyAlert(ctx context.Context, alert domain.Alert) {
	s.NotifyAlertEvent(ctx, AlertNotificationEvent{Type: EventTypeProblem, Alert: alert})
}

func (s *Service) NotifyAlertEvent(ctx context.Context, event AlertNotificationEvent) {
	config := s.alertNotificationRuntimeConfig(ctx)
	s = s.withAlertHTTPTimeout(config.Timeout)
	channels, err := s.store.ListNotificationChannels(ctx)
	if err != nil {
		s.logger.Warn("list notification channels failed", "error", err)
		return
	}
	sent := false
	activeExternalChannels := 0
	for _, channel := range channels {
		if !channel.Enabled {
			continue
		}
		if event.Type == EventTypeRecovery && !notificationTemplateConfig(channel.Config).SendRecovery {
			continue
		}
		activeExternalChannels++
		if err := s.send(ctx, channel, event); err != nil {
			s.logger.Warn("send alert notification failed", "channel", channel.ID, "alert", event.Alert.ID, "event", event.Type, "error", err)
			continue
		}
		sent = true
	}
	if event.Type == EventTypeProblem && (sent || activeExternalChannels == 0) {
		_ = s.store.MarkAlertNotificationSent(ctx, event.Alert.ID)
	}
}

func (s *Service) NotifyDelivery(ctx context.Context, delivery domain.AlertNotificationDelivery) {
	config := s.alertNotificationRuntimeConfig(ctx)
	s = s.withAlertHTTPTimeout(config.Timeout)
	event := AlertNotificationEvent{Type: delivery.EventType, Alert: delivery.Alert}
	channel, ok, err := s.notificationChannel(ctx, delivery.ChannelID)
	if err != nil {
		s.logger.Warn("list notification channels failed", "error", err)
		return
	}
	if !ok || !channel.Enabled || (event.Type == EventTypeRecovery && !notificationTemplateConfig(channel.Config).SendRecovery) {
		_ = s.store.MarkAlertNotificationDeliverySent(ctx, delivery.ID)
		if event.Type == EventTypeProblem {
			_ = s.store.MarkAlertNotificationSent(ctx, event.Alert.ID)
		}
		return
	}
	if err := s.send(ctx, channel, event); err != nil {
		s.logger.Warn("send alert notification failed", "channel", channel.ID, "alert", event.Alert.ID, "event", event.Type, "error", err)
		_ = s.store.MarkAlertNotificationDeliveryFailedWithConfig(ctx, delivery.ID, UserFacingErrorMessage(err), config.Retry)
		return
	}
	_ = s.store.MarkAlertNotificationDeliverySent(ctx, delivery.ID)
	if event.Type == EventTypeProblem {
		_ = s.store.MarkAlertNotificationSent(ctx, event.Alert.ID)
	}
}

func (s *Service) notificationChannel(ctx context.Context, id string) (domain.NotificationChannel, bool, error) {
	channels, err := s.store.ListNotificationChannels(ctx)
	if err != nil {
		return domain.NotificationChannel{}, false, err
	}
	for _, channel := range channels {
		if channel.ID == id {
			return channel, true, nil
		}
	}
	return domain.NotificationChannel{}, false, nil
}

func (s *Service) SendTest(ctx context.Context, channel domain.NotificationChannel) error {
	now := time.Now().UTC()
	alert := domain.Alert{ID: "test", Level: "info", Status: "active", Title: "KVM Manager 测试通知", Message: "这是一条通知媒介测试消息", SourceType: "system", SourceID: "notification-test", FirstSeenAt: now, LastSeenAt: now}
	config := s.alertNotificationRuntimeConfig(ctx)
	return s.withAlertHTTPTimeout(config.Timeout).send(ctx, channel, AlertNotificationEvent{Type: EventTypeProblem, Alert: alert})
}

func (s *Service) alertNotificationRuntimeConfig(ctx context.Context) alertNotificationRuntimeConfig {
	config := alertNotificationRuntimeConfig{
		Timeout: 8 * time.Second,
		Retry: repository.AlertNotificationRetryConfig{
			MaxRetryCount:    6,
			BaseDelaySeconds: 30,
			MaxDelayMinutes:  15,
		},
	}
	if s == nil || s.store == nil {
		return config
	}
	baseConfig, err := s.store.GetSystemBaseConfig(ctx)
	if err != nil {
		if s.logger != nil {
			s.logger.Warn("load alert notification settings failed", "error", err)
		}
		return config
	}
	config.Timeout = time.Duration(clampConfigInt(baseConfig.AlertNotificationTimeoutSeconds, 3, 60, 8)) * time.Second
	config.Retry = repository.AlertNotificationRetryConfig{
		MaxRetryCount:    clampConfigInt(baseConfig.AlertNotificationMaxRetryCount, 0, 10, 6),
		BaseDelaySeconds: clampConfigInt(baseConfig.AlertNotificationRetryBaseSeconds, 10, 300, 30),
		MaxDelayMinutes:  clampConfigInt(baseConfig.AlertNotificationRetryMaxMinutes, 1, 120, 15),
	}
	return config
}

func (s *Service) httpClient(ctx context.Context) *http.Client {
	timeout := s.alertHTTPTimeout
	if timeout <= 0 {
		timeout = 8 * time.Second
	}
	if s == nil || s.http == nil {
		return &http.Client{Timeout: timeout}
	}
	if s.http.Timeout == timeout {
		return s.http
	}
	next := *s.http
	next.Timeout = timeout
	return &next
}

func (s *Service) withAlertHTTPTimeout(timeout time.Duration) *Service {
	if s == nil || timeout <= 0 {
		return s
	}
	next := *s
	next.alertHTTPTimeout = timeout
	return &next
}

func clampConfigInt(value int, min int, max int, fallback int) int {
	if value < min || value > max {
		return fallback
	}
	return value
}

type PasswordResetMessage struct {
	Username  string
	Code      string
	ExpiresAt time.Time
	RequestIP string
	To        string
}

func (s *Service) SendPasswordReset(ctx context.Context, channel domain.NotificationChannel, message PasswordResetMessage) error {
	switch channel.ID {
	case "webhook":
		var cfg webhookConfig
		if err := decodeConfig(channel.Config, &cfg); err != nil {
			return err
		}
		return s.sendPasswordResetWebhook(ctx, cfg, message)
	case "email":
		var cfg emailConfig
		if err := decodeConfig(channel.Config, &cfg); err != nil {
			return err
		}
		return s.sendPasswordResetEmail(ctx, cfg, message)
	case "lark":
		var cfg robotConfig
		if err := decodeConfig(channel.Config, &cfg); err != nil {
			return err
		}
		return s.sendLarkPasswordResetCard(ctx, cfg, message)
	case "lark_app":
		var cfg larkAppConfig
		if err := decodeConfig(channel.Config, &cfg); err != nil {
			return err
		}
		return s.sendLarkAppPasswordReset(ctx, cfg, message)
	case "wechat":
		var cfg robotConfig
		if err := decodeConfig(channel.Config, &cfg); err != nil {
			return err
		}
		return s.sendWechatMarkdown(ctx, cfg, passwordResetRobotText(message))
	case "wechat_app":
		var cfg wechatAppConfig
		if err := decodeConfig(channel.Config, &cfg); err != nil {
			return err
		}
		return s.sendWechatAppPasswordReset(ctx, cfg, message)
	case "dingtalk":
		var cfg robotConfig
		if err := decodeConfig(channel.Config, &cfg); err != nil {
			return err
		}
		return s.sendDingTalkMarkdown(ctx, cfg, "KVM Manager 密码找回", dingtalkMarkdownLineBreaks(passwordResetRobotText(message)))
	case "dingtalk_app":
		var cfg dingTalkAppConfig
		if err := decodeConfig(channel.Config, &cfg); err != nil {
			return err
		}
		return s.sendDingTalkAppPasswordReset(ctx, cfg, message)
	default:
		return fmt.Errorf("不支持的通知媒介 %s", channel.ID)
	}
}

func (s *Service) send(ctx context.Context, channel domain.NotificationChannel, event AlertNotificationEvent) error {
	switch channel.ID {
	case "webhook":
		var cfg webhookConfig
		if err := decodeConfig(channel.Config, &cfg); err != nil {
			return err
		}
		return s.sendWebhook(ctx, cfg, event)
	case "lark":
		var cfg robotConfig
		if err := decodeConfig(channel.Config, &cfg); err != nil {
			return err
		}
		return s.sendLark(ctx, cfg, event)
	case "lark_app":
		var cfg larkAppConfig
		if err := decodeConfig(channel.Config, &cfg); err != nil {
			return err
		}
		return s.sendLarkApp(ctx, cfg, event)
	case "wechat":
		var cfg robotConfig
		if err := decodeConfig(channel.Config, &cfg); err != nil {
			return err
		}
		return s.sendWechat(ctx, cfg, event)
	case "wechat_app":
		var cfg wechatAppConfig
		if err := decodeConfig(channel.Config, &cfg); err != nil {
			return err
		}
		return s.sendWechatApp(ctx, cfg, event)
	case "dingtalk":
		var cfg robotConfig
		if err := decodeConfig(channel.Config, &cfg); err != nil {
			return err
		}
		return s.sendDingTalk(ctx, cfg, event)
	case "dingtalk_app":
		var cfg dingTalkAppConfig
		if err := decodeConfig(channel.Config, &cfg); err != nil {
			return err
		}
		return s.sendDingTalkApp(ctx, cfg, event)
	case "email":
		var cfg emailConfig
		if err := decodeConfig(channel.Config, &cfg); err != nil {
			return err
		}
		return s.sendEmail(ctx, cfg, event)
	default:
		return fmt.Errorf("不支持的通知媒介 %s", channel.ID)
	}
}

func UserFacingErrorMessage(err error) string {
	if err == nil {
		return ""
	}
	message := err.Error()
	if strings.HasPrefix(message, "通知") || strings.HasPrefix(message, "Webhook") || strings.HasPrefix(message, "SMTP") || strings.HasPrefix(message, "机器人") || strings.HasPrefix(message, "不支持的通知媒介") {
		return message
	}
	if errors.Is(err, context.Canceled) {
		return "通知发送已取消"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "通知发送超时"
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return "通知发送超时"
	}
	return "通知发送失败"
}

type webhookConfig struct {
	URL     string            `json:"url"`
	Method  string            `json:"method"`
	Secret  string            `json:"secret"`
	Headers map[string]string `json:"headers"`
	templateConfig
}

type robotConfig struct {
	WebhookURL string `json:"webhookUrl"`
	Secret     string `json:"secret"`
	templateConfig
}

type emailConfig struct {
	SMTPHost          string   `json:"smtpHost"`
	SMTPPort          int      `json:"smtpPort"`
	Username          string   `json:"username"`
	Password          string   `json:"password"`
	From              string   `json:"from"`
	FromName          string   `json:"fromName"`
	To                []string `json:"to"`
	UseTLS            bool     `json:"useTLS"`
	StartTLS          bool     `json:"startTLS"`
	AllowInsecureAuth bool     `json:"allowInsecureAuth"`
	templateConfig
}

func decodeConfig(data []byte, target any) error {
	if len(data) == 0 {
		return fmt.Errorf("通知配置为空")
	}
	return json.Unmarshal(data, target)
}

func (s *Service) sendWebhook(ctx context.Context, cfg webhookConfig, event AlertNotificationEvent) error {
	if strings.TrimSpace(cfg.URL) == "" {
		return fmt.Errorf("Webhook URL 不能为空")
	}
	method := strings.ToUpper(strings.TrimSpace(cfg.Method))
	if method == "" {
		method = http.MethodPost
	}
	if method != http.MethodPost && method != http.MethodPut && method != http.MethodPatch {
		return fmt.Errorf("Webhook 请求方法仅支持 POST、PUT 或 PATCH")
	}
	payload, err := alertWebhookPayload(event, cfg.templateConfig)
	if err != nil {
		return err
	}
	return s.postJSON(ctx, method, cfg.URL, payload, cfg.Headers)
}

func (s *Service) sendPasswordResetWebhook(ctx context.Context, cfg webhookConfig, message PasswordResetMessage) error {
	if strings.TrimSpace(cfg.URL) == "" {
		return fmt.Errorf("Webhook URL 不能为空")
	}
	method := strings.ToUpper(strings.TrimSpace(cfg.Method))
	if method == "" {
		method = http.MethodPost
	}
	if method != http.MethodPost && method != http.MethodPut && method != http.MethodPatch {
		return fmt.Errorf("Webhook 请求方法仅支持 POST、PUT 或 PATCH")
	}
	payload := map[string]any{
		"type":      "password_reset",
		"title":     "KVM Manager 密码找回",
		"username":  message.Username,
		"code":      message.Code,
		"expiresAt": message.ExpiresAt.Format(time.RFC3339),
		"requestIP": message.RequestIP,
	}
	return s.postJSON(ctx, method, cfg.URL, payload, cfg.Headers)
}

func (s *Service) sendLark(ctx context.Context, cfg robotConfig, event AlertNotificationEvent) error {
	payload := larkAlertPayload(event, cfg.templateConfig)
	if secret := strings.TrimSpace(cfg.Secret); secret != "" {
		timestamp := strconv.FormatInt(time.Now().Unix(), 10)
		payload["timestamp"] = timestamp
		payload["sign"] = signLark(timestamp, secret)
	}
	return s.postJSON(ctx, http.MethodPost, cfg.WebhookURL, payload, nil)
}

func larkAlertPayload(event AlertNotificationEvent, cfg templateConfig) map[string]any {
	subject := larkTitle(event, cfg)
	text := alertText(event, cfg)
	switch larkMessageType(cfg) {
	case "post":
		return map[string]any{
			"msg_type": "post",
			"content": map[string]any{
				"post": map[string]any{
					"zh_cn": map[string]any{
						"title":   subject,
						"content": larkPostContent(text),
					},
				},
			},
		}
	case "interactive":
		return map[string]any{
			"msg_type": "interactive",
			"card": map[string]any{
				"config": map[string]bool{"wide_screen_mode": true},
				"header": map[string]any{
					"template": larkCardTemplate(event, cfg),
					"title":    map[string]string{"tag": "plain_text", "content": subject},
				},
				"elements": []map[string]any{
					{"tag": "div", "text": map[string]string{"tag": "lark_md", "content": text}},
				},
			},
		}
	default:
		return map[string]any{"msg_type": "text", "content": map[string]string{"text": text}}
	}
}

func larkPostContent(text string) [][]map[string]string {
	lines := strings.Split(text, "\n")
	content := make([][]map[string]string, 0, len(lines))
	for _, line := range lines {
		content = append(content, []map[string]string{{"tag": "text", "text": line}})
	}
	return content
}

func larkTitle(event AlertNotificationEvent, cfg templateConfig) string {
	template := strings.TrimSpace(cfg.LarkProblemTitleTemplate)
	if event.Type == EventTypeRecovery {
		template = strings.TrimSpace(cfg.LarkRecoveryTitleTemplate)
	}
	if template == "" {
		return alertSubject(event, cfg)
	}
	return renderAlertTemplate(template, event)
}

func larkCardTemplate(event AlertNotificationEvent, cfg templateConfig) string {
	template := strings.TrimSpace(cfg.LarkProblemCardTemplate)
	if event.Type == EventTypeRecovery {
		template = strings.TrimSpace(cfg.LarkRecoveryCardTemplate)
	}
	if IsLarkCardTemplate(template) {
		return template
	}
	if event.Type == EventTypeRecovery {
		return "green"
	}
	return "red"
}

func (s *Service) sendLarkMarkdown(ctx context.Context, cfg robotConfig, text string) error {
	payload := map[string]any{"msg_type": "text", "content": map[string]string{"text": text}}
	if secret := strings.TrimSpace(cfg.Secret); secret != "" {
		timestamp := strconv.FormatInt(time.Now().Unix(), 10)
		payload["timestamp"] = timestamp
		payload["sign"] = signLark(timestamp, secret)
	}
	return s.postJSON(ctx, http.MethodPost, cfg.WebhookURL, payload, nil)
}

func (s *Service) sendLarkPasswordResetCard(ctx context.Context, cfg robotConfig, message PasswordResetMessage) error {
	payload := map[string]any{
		"msg_type": "interactive",
		"card": map[string]any{
			"config": map[string]bool{"wide_screen_mode": true},
			"header": map[string]any{
				"template": "green",
				"title":    map[string]string{"tag": "plain_text", "content": "KVM Manager 密码找回"},
			},
			"elements": []map[string]any{
				{"tag": "div", "text": map[string]string{"tag": "lark_md", "content": fmt.Sprintf("账号：%s\n验证码：%s\n有效期至：%s\n请求来源：%s", message.Username, message.Code, message.ExpiresAt.Local().Format("2006-01-02 15:04:05"), message.RequestIP)}},
				{"tag": "hr"},
				{"tag": "div", "text": map[string]string{"tag": "plain_text", "content": "如果不是您本人操作，请忽略本消息并检查平台账号安全。"}},
			},
		},
	}
	if secret := strings.TrimSpace(cfg.Secret); secret != "" {
		timestamp := strconv.FormatInt(time.Now().Unix(), 10)
		payload["timestamp"] = timestamp
		payload["sign"] = signLark(timestamp, secret)
	}
	return s.postJSON(ctx, http.MethodPost, cfg.WebhookURL, payload, nil)
}

func (s *Service) sendWechat(ctx context.Context, cfg robotConfig, event AlertNotificationEvent) error {
	text := alertText(event, cfg.templateConfig)
	if wechatMessageType(cfg.templateConfig) == "markdown" {
		return s.postJSON(ctx, http.MethodPost, cfg.WebhookURL, map[string]any{"msgtype": "markdown", "markdown": map[string]string{"content": text}}, nil)
	}
	return s.postJSON(ctx, http.MethodPost, cfg.WebhookURL, map[string]any{"msgtype": "text", "text": map[string]string{"content": text}}, nil)
}

func (s *Service) sendWechatMarkdown(ctx context.Context, cfg robotConfig, text string) error {
	return s.postJSON(ctx, http.MethodPost, cfg.WebhookURL, map[string]any{"msgtype": "markdown", "markdown": map[string]string{"content": text}}, nil)
}

func (s *Service) sendDingTalk(ctx context.Context, cfg robotConfig, event AlertNotificationEvent) error {
	webhookURL := cfg.WebhookURL
	if secret := strings.TrimSpace(cfg.Secret); secret != "" {
		var err error
		webhookURL, err = signDingTalkURL(webhookURL, secret)
		if err != nil {
			return err
		}
	}
	text := alertText(event, cfg.templateConfig)
	if dingTalkMessageType(cfg.templateConfig) == "markdown" {
		return s.postJSON(ctx, http.MethodPost, webhookURL, map[string]any{"msgtype": "markdown", "markdown": map[string]string{"title": alertSubject(event, cfg.templateConfig), "text": dingtalkMarkdownLineBreaks(text)}}, nil)
	}
	return s.postJSON(ctx, http.MethodPost, webhookURL, map[string]any{"msgtype": "text", "text": map[string]string{"content": text}}, nil)
}

func (s *Service) sendDingTalkMarkdown(ctx context.Context, cfg robotConfig, title, text string) error {
	webhookURL := cfg.WebhookURL
	if secret := strings.TrimSpace(cfg.Secret); secret != "" {
		var err error
		webhookURL, err = signDingTalkURL(webhookURL, secret)
		if err != nil {
			return err
		}
	}
	return s.postJSON(ctx, http.MethodPost, webhookURL, map[string]any{"msgtype": "markdown", "markdown": map[string]string{"title": title, "text": text}}, nil)
}

func (s *Service) postJSON(ctx context.Context, method string, targetURL string, payload any, headers map[string]string) error {
	if strings.TrimSpace(targetURL) == "" {
		return fmt.Errorf("Webhook URL 不能为空")
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, method, targetURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	for key, value := range headers {
		if strings.TrimSpace(key) != "" {
			req.Header.Set(key, value)
		}
	}
	resp, err := s.httpClient(ctx).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("Webhook 返回异常状态：%s", resp.Status)
	}
	return nil
}

func signLark(timestamp string, secret string) string {
	mac := hmac.New(sha256.New, []byte(timestamp+"\n"+secret))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func signDingTalkURL(rawURL string, secret string) (string, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}
	timestamp := strconv.FormatInt(time.Now().UnixMilli(), 10)
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(timestamp + "\n" + secret))
	query := parsed.Query()
	query.Set("timestamp", timestamp)
	query.Set("sign", base64.StdEncoding.EncodeToString(mac.Sum(nil)))
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func (s *Service) sendEmail(ctx context.Context, cfg emailConfig, event AlertNotificationEvent) error {
	if cfg.SMTPHost == "" {
		return fmt.Errorf("SMTP 主机不能为空")
	}
	if cfg.SMTPPort <= 0 {
		return fmt.Errorf("SMTP 端口不能为空")
	}
	if cfg.Username == "" {
		return fmt.Errorf("用户名不能为空")
	}
	if cfg.Password == "" {
		return fmt.Errorf("密码不能为空")
	}
	if cfg.From == "" {
		return fmt.Errorf("发件人不能为空")
	}
	if len(cfg.To) == 0 {
		return fmt.Errorf("收件人不能为空")
	}
	addr := fmt.Sprintf("%s:%d", cfg.SMTPHost, cfg.SMTPPort)
	subject := alertSubject(event, cfg.templateConfig)
	message := []byte("From: " + formatMailFrom(cfg) + "\r\n" + "To: " + strings.Join(cfg.To, ",") + "\r\n" + "Subject: " + subject + "\r\n" + "Content-Type: " + emailContentType(cfg.templateConfig) + "; charset=UTF-8\r\n\r\n" + alertText(event, cfg.templateConfig))
	return s.sendSMTPMessage(ctx, cfg, addr, cfg.To, message)
}

func emailContentType(cfg templateConfig) string {
	if strings.EqualFold(strings.TrimSpace(cfg.EmailContentType), "text/html") {
		return "text/html"
	}
	return "text/plain"
}

func larkMessageType(cfg templateConfig) string {
	switch strings.ToLower(strings.TrimSpace(cfg.LarkMessageType)) {
	case "post", "interactive":
		return strings.ToLower(strings.TrimSpace(cfg.LarkMessageType))
	default:
		return "text"
	}
}

func IsLarkCardTemplate(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "red", "green", "blue", "orange", "yellow", "purple", "wathet", "turquoise", "indigo", "grey":
		return true
	default:
		return false
	}
}

func wechatMessageType(cfg templateConfig) string {
	if strings.EqualFold(strings.TrimSpace(cfg.WechatMessageType), "markdown") {
		return "markdown"
	}
	return "text"
}

func dingTalkMessageType(cfg templateConfig) string {
	if strings.EqualFold(strings.TrimSpace(cfg.DingTalkMessageType), "markdown") {
		return "markdown"
	}
	return "text"
}

func (s *Service) sendPasswordResetEmail(ctx context.Context, cfg emailConfig, message PasswordResetMessage) error {
	if cfg.SMTPHost == "" {
		return fmt.Errorf("SMTP 主机不能为空")
	}
	if cfg.SMTPPort <= 0 {
		return fmt.Errorf("SMTP 端口不能为空")
	}
	if cfg.Username == "" {
		return fmt.Errorf("用户名不能为空")
	}
	if cfg.Password == "" {
		return fmt.Errorf("密码不能为空")
	}
	if cfg.From == "" {
		return fmt.Errorf("发件人不能为空")
	}
	to := strings.TrimSpace(message.To)
	if to == "" {
		return fmt.Errorf("找回密码收件人不能为空")
	}
	addr := fmt.Sprintf("%s:%d", cfg.SMTPHost, cfg.SMTPPort)
	subject := "KVM Manager 密码找回验证码"
	body := passwordResetEmailHTML(message)
	raw := []byte("From: " + formatMailFrom(cfg) + "\r\n" + "To: " + to + "\r\n" + "Subject: " + subject + "\r\n" + "MIME-Version: 1.0\r\n" + "Content-Type: text/html; charset=UTF-8\r\n\r\n" + body)
	return s.sendSMTPMessage(ctx, cfg, addr, []string{to}, raw)
}

func formatMailFrom(cfg emailConfig) string {
	name := strings.TrimSpace(cfg.FromName)
	if name == "" {
		return cfg.From
	}
	return (&mail.Address{Name: name, Address: cfg.From}).String()
}

func (s *Service) sendSMTPMessage(ctx context.Context, cfg emailConfig, addr string, recipients []string, message []byte) error {
	if cfg.UseTLS && cfg.StartTLS {
		return fmt.Errorf("TLS 与 STARTTLS 不能同时启用")
	}
	timeout := s.alertHTTPTimeout
	if timeout <= 0 {
		timeout = 8 * time.Second
	}
	auth := smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.SMTPHost)
	if !cfg.UseTLS && !cfg.StartTLS {
		client, err := dialPlainSMTP(ctx, addr, cfg.SMTPHost, timeout)
		if err != nil {
			return smtpError(err)
		}
		defer client.Quit()
		if cfg.AllowInsecureAuth {
			auth = plainInsecureAuthPayload(cfg.Username, cfg.Password)
		}
		return smtpError(writeSMTPMessage(client, cfg.From, recipients, message, auth))
	}
	var client *smtp.Client
	var err error
	if cfg.UseTLS {
		client, err = dialTLSSMTP(ctx, addr, cfg.SMTPHost, timeout)
	} else {
		client, err = dialStartTLSSMTP(ctx, addr, cfg.SMTPHost, timeout)
	}
	if err != nil {
		return smtpError(err)
	}
	defer client.Quit()
	return smtpError(writeSMTPMessage(client, cfg.From, recipients, message, auth))
}

func writeSMTPMessage(client *smtp.Client, from string, recipients []string, message []byte, auth smtp.Auth) error {
	if err := client.Auth(auth); err != nil {
		return err
	}
	if err := client.Mail(from); err != nil {
		return err
	}
	for _, to := range recipients {
		if err := client.Rcpt(to); err != nil {
			return err
		}
	}
	writer, err := client.Data()
	if err != nil {
		return err
	}
	if _, err := writer.Write(message); err != nil {
		_ = writer.Close()
		return err
	}
	return writer.Close()
}

type plainInsecureAuth string

func plainInsecureAuthPayload(username string, password string) smtp.Auth {
	return plainInsecureAuth("\x00" + username + "\x00" + password)
}

func (a plainInsecureAuth) Start(*smtp.ServerInfo) (string, []byte, error) {
	return "PLAIN", []byte(a), nil
}

func (a plainInsecureAuth) Next([]byte, bool) ([]byte, error) {
	return nil, nil
}

func dialPlainSMTP(ctx context.Context, addr string, host string, timeout time.Duration) (*smtp.Client, error) {
	conn, err := dialSMTPConn(ctx, addr, timeout)
	if err != nil {
		return nil, err
	}
	client, err := smtp.NewClient(conn, host)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	return client, nil
}

func dialTLSSMTP(ctx context.Context, addr string, host string, timeout time.Duration) (*smtp.Client, error) {
	conn, err := dialSMTPConn(ctx, addr, timeout)
	if err != nil {
		return nil, err
	}
	tlsConn := tls.Client(conn, &tls.Config{ServerName: host})
	if err := tlsConn.Handshake(); err != nil {
		_ = tlsConn.Close()
		return nil, err
	}
	client, err := smtp.NewClient(tlsConn, host)
	if err != nil {
		_ = tlsConn.Close()
		return nil, err
	}
	return client, nil
}

func dialStartTLSSMTP(ctx context.Context, addr string, host string, timeout time.Duration) (*smtp.Client, error) {
	client, err := dialPlainSMTP(ctx, addr, host, timeout)
	if err != nil {
		return nil, err
	}
	if ok, _ := client.Extension("STARTTLS"); !ok {
		_ = client.Close()
		return nil, fmt.Errorf("SMTP 服务不支持 STARTTLS")
	}
	if err := client.StartTLS(&tls.Config{ServerName: host}); err != nil {
		_ = client.Close()
		return nil, err
	}
	return client, nil
}

func dialSMTPConn(ctx context.Context, addr string, timeout time.Duration) (net.Conn, error) {
	dialer := net.Dialer{Timeout: timeout}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, err
	}
	if timeout > 0 {
		_ = conn.SetDeadline(time.Now().Add(timeout))
	}
	return conn, nil
}

func smtpError(err error) error {
	if err == nil {
		return nil
	}
	if strings.HasPrefix(err.Error(), "SMTP ") || strings.HasPrefix(err.Error(), "TLS ") {
		return err
	}
	return fmt.Errorf("SMTP 发送失败：%w", err)
}

func passwordResetRobotText(message PasswordResetMessage) string {
	return fmt.Sprintf("## KVM Manager 密码找回\n\n账号：%s\n验证码：%s\n有效期至：%s\n请求来源：%s\n\n如果不是您本人操作，请忽略本消息并检查平台账号安全。", message.Username, message.Code, message.ExpiresAt.Local().Format("2006-01-02 15:04:05"), message.RequestIP)
}

func dingtalkMarkdownLineBreaks(text string) string {
	return strings.ReplaceAll(text, "\n", "  \n")
}

func passwordResetEmailHTML(message PasswordResetMessage) string {
	return fmt.Sprintf(`<!doctype html>
<html>
<body style="margin:0;background:#f5f7fb;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',sans-serif;color:#172033;">
  <table role="presentation" width="100%%" cellspacing="0" cellpadding="0" style="background:#f5f7fb;padding:32px 12px;">
    <tr>
      <td align="center">
        <table role="presentation" width="560" cellspacing="0" cellpadding="0" style="max-width:560px;background:#ffffff;border:1px solid #e4e9f2;border-radius:14px;overflow:hidden;">
          <tr>
            <td style="padding:28px 32px 18px;background:#0f766e;color:#ffffff;">
              <div style="font-size:20px;font-weight:700;">KVM Manager 密码找回</div>
              <div style="margin-top:8px;font-size:13px;opacity:.86;">请使用以下验证码完成密码重置</div>
            </td>
          </tr>
          <tr>
            <td style="padding:30px 32px;">
              <div style="font-size:14px;color:#526071;">账号</div>
              <div style="margin-top:6px;font-size:18px;font-weight:700;color:#172033;">%s</div>
              <div style="margin-top:24px;padding:18px 20px;border-radius:12px;background:#ecfdf5;border:1px solid #a7f3d0;text-align:center;">
                <div style="font-size:13px;color:#047857;">验证码</div>
                <div style="margin-top:8px;font-size:34px;letter-spacing:8px;font-weight:800;color:#065f46;">%s</div>
              </div>
              <div style="margin-top:22px;font-size:14px;line-height:1.8;color:#526071;">
                有效期至：<strong style="color:#172033;">%s</strong><br>
                请求来源：<strong style="color:#172033;">%s</strong>
              </div>
              <div style="margin-top:24px;padding:14px 16px;border-radius:10px;background:#fff7ed;border:1px solid #fed7aa;color:#9a3412;font-size:13px;line-height:1.7;">
                如果不是您本人操作，请忽略本邮件并检查平台账号安全。
              </div>
            </td>
          </tr>
        </table>
      </td>
    </tr>
  </table>
</body>
</html>`, message.Username, message.Code, message.ExpiresAt.Local().Format("2006-01-02 15:04:05"), message.RequestIP)
}
