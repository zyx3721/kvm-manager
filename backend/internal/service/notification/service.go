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
)

type Store interface {
	ListNotificationChannels(ctx context.Context) ([]domain.NotificationChannel, error)
	MarkAlertNotificationSent(ctx context.Context, id string) error
}

type Service struct {
	store  Store
	logger *slog.Logger
	http   *http.Client
}

func NewService(store Store, logger *slog.Logger) *Service {
	return &Service{store: store, logger: logger, http: &http.Client{Timeout: 8 * time.Second}}
}

func (s *Service) NotifyAlert(ctx context.Context, alert domain.Alert) {
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
		activeExternalChannels++
		if err := s.send(ctx, channel, alert); err != nil {
			s.logger.Warn("send alert notification failed", "channel", channel.ID, "alert", alert.ID, "error", err)
			continue
		}
		sent = true
	}
	if sent || activeExternalChannels == 0 {
		_ = s.store.MarkAlertNotificationSent(ctx, alert.ID)
	}
}

func (s *Service) SendTest(ctx context.Context, channel domain.NotificationChannel) error {
	alert := domain.Alert{ID: "test", Level: "info", Title: "KVM Manager 测试通知", Message: "这是一条通知媒介测试消息", SourceType: "system", SourceID: "notification-test", LastSeenAt: time.Now().UTC()}
	return s.send(ctx, channel, alert)
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
		return sendPasswordResetEmail(cfg, message)
	case "lark":
		var cfg robotConfig
		if err := decodeConfig(channel.Config, &cfg); err != nil {
			return err
		}
		return s.sendLarkPasswordResetCard(ctx, cfg, message)
	case "wechat":
		var cfg robotConfig
		if err := decodeConfig(channel.Config, &cfg); err != nil {
			return err
		}
		return s.sendWechatMarkdown(ctx, cfg, passwordResetRobotText(message))
	case "dingtalk":
		var cfg robotConfig
		if err := decodeConfig(channel.Config, &cfg); err != nil {
			return err
		}
		return s.sendDingTalkMarkdown(ctx, cfg, "KVM Manager 密码找回", dingtalkMarkdownLineBreaks(passwordResetRobotText(message)))
	default:
		return fmt.Errorf("不支持的通知媒介 %s", channel.ID)
	}
}

func (s *Service) send(ctx context.Context, channel domain.NotificationChannel, alert domain.Alert) error {
	switch channel.ID {
	case "webhook":
		var cfg webhookConfig
		if err := decodeConfig(channel.Config, &cfg); err != nil {
			return err
		}
		return s.sendWebhook(ctx, cfg, alert)
	case "lark":
		var cfg robotConfig
		if err := decodeConfig(channel.Config, &cfg); err != nil {
			return err
		}
		return s.sendLark(ctx, cfg, alert)
	case "wechat":
		var cfg robotConfig
		if err := decodeConfig(channel.Config, &cfg); err != nil {
			return err
		}
		return s.sendWechat(ctx, cfg, alert)
	case "dingtalk":
		var cfg robotConfig
		if err := decodeConfig(channel.Config, &cfg); err != nil {
			return err
		}
		return s.sendDingTalk(ctx, cfg, alert)
	case "email":
		var cfg emailConfig
		if err := decodeConfig(channel.Config, &cfg); err != nil {
			return err
		}
		return sendEmail(cfg, alert)
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
}

type robotConfig struct {
	WebhookURL string `json:"webhookUrl"`
	Secret     string `json:"secret"`
}

type emailConfig struct {
	SMTPHost string   `json:"smtpHost"`
	SMTPPort int      `json:"smtpPort"`
	Username string   `json:"username"`
	Password string   `json:"password"`
	From     string   `json:"from"`
	FromName string   `json:"fromName"`
	To       []string `json:"to"`
	UseTLS   bool     `json:"useTLS"`
	StartTLS bool     `json:"startTLS"`
}

func decodeConfig(data []byte, target any) error {
	if len(data) == 0 {
		return fmt.Errorf("通知配置为空")
	}
	return json.Unmarshal(data, target)
}

func (s *Service) sendWebhook(ctx context.Context, cfg webhookConfig, alert domain.Alert) error {
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
	payload := map[string]any{"id": alert.ID, "level": alert.Level, "title": alert.Title, "message": alert.Message, "sourceType": alert.SourceType, "sourceId": alert.SourceID, "lastSeenAt": alert.LastSeenAt.Format(time.RFC3339)}
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

func (s *Service) sendLark(ctx context.Context, cfg robotConfig, alert domain.Alert) error {
	payload := map[string]any{"msg_type": "text", "content": map[string]string{"text": alertText(alert)}}
	if secret := strings.TrimSpace(cfg.Secret); secret != "" {
		timestamp := strconv.FormatInt(time.Now().Unix(), 10)
		payload["timestamp"] = timestamp
		payload["sign"] = signLark(timestamp, secret)
	}
	return s.postJSON(ctx, http.MethodPost, cfg.WebhookURL, payload, nil)
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

func (s *Service) sendWechat(ctx context.Context, cfg robotConfig, alert domain.Alert) error {
	return s.postJSON(ctx, http.MethodPost, cfg.WebhookURL, map[string]any{"msgtype": "text", "text": map[string]string{"content": alertText(alert)}}, nil)
}

func (s *Service) sendWechatMarkdown(ctx context.Context, cfg robotConfig, text string) error {
	return s.postJSON(ctx, http.MethodPost, cfg.WebhookURL, map[string]any{"msgtype": "markdown", "markdown": map[string]string{"content": text}}, nil)
}

func (s *Service) sendDingTalk(ctx context.Context, cfg robotConfig, alert domain.Alert) error {
	webhookURL := cfg.WebhookURL
	if secret := strings.TrimSpace(cfg.Secret); secret != "" {
		var err error
		webhookURL, err = signDingTalkURL(webhookURL, secret)
		if err != nil {
			return err
		}
	}
	return s.postJSON(ctx, http.MethodPost, webhookURL, map[string]any{"msgtype": "text", "text": map[string]string{"content": alertText(alert)}}, nil)
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
	resp, err := s.http.Do(req)
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

func sendEmail(cfg emailConfig, alert domain.Alert) error {
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
	message := []byte("From: " + formatMailFrom(cfg) + "\r\n" + "To: " + strings.Join(cfg.To, ",") + "\r\n" + "Subject: " + alert.Title + "\r\n" + "Content-Type: text/plain; charset=UTF-8\r\n\r\n" + alertText(alert))
	return sendSMTPMessage(cfg, addr, cfg.To, message)
}

func sendPasswordResetEmail(cfg emailConfig, message PasswordResetMessage) error {
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
	return sendSMTPMessage(cfg, addr, []string{to}, raw)
}

func formatMailFrom(cfg emailConfig) string {
	name := strings.TrimSpace(cfg.FromName)
	if name == "" {
		return cfg.From
	}
	return (&mail.Address{Name: name, Address: cfg.From}).String()
}

func sendSMTPMessage(cfg emailConfig, addr string, recipients []string, message []byte) error {
	if cfg.UseTLS && cfg.StartTLS {
		return fmt.Errorf("TLS 与 STARTTLS 不能同时启用")
	}
	auth := smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.SMTPHost)
	if !cfg.UseTLS && !cfg.StartTLS {
		return smtp.SendMail(addr, auth, cfg.From, recipients, message)
	}
	var client *smtp.Client
	var err error
	if cfg.UseTLS {
		client, err = dialTLSSMTP(addr, cfg.SMTPHost)
	} else {
		client, err = dialStartTLSSMTP(addr, cfg.SMTPHost)
	}
	if err != nil {
		return err
	}
	defer client.Quit()
	if err := client.Auth(auth); err != nil {
		return err
	}
	if err := client.Mail(cfg.From); err != nil {
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

func dialTLSSMTP(addr string, host string) (*smtp.Client, error) {
	conn, err := tls.Dial("tcp", addr, &tls.Config{ServerName: host})
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

func dialStartTLSSMTP(addr string, host string) (*smtp.Client, error) {
	client, err := smtp.Dial(addr)
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

func alertText(alert domain.Alert) string {
	return fmt.Sprintf("[%s] %s\n%s\n来源：%s/%s", alert.Level, alert.Title, alert.Message, alert.SourceType, alert.SourceID)
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
