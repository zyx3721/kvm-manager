package notification

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

const (
	larkTenantTokenURL    = "https://open.feishu.cn/open-apis/auth/v3/tenant_access_token/internal"
	larkMessageURL        = "https://open.feishu.cn/open-apis/im/v1/messages"
	wechatTokenURL        = "https://qyapi.weixin.qq.com/cgi-bin/gettoken"
	wechatMessageURL      = "https://qyapi.weixin.qq.com/cgi-bin/message/send"
	dingTalkTokenURL      = "https://oapi.dingtalk.com/gettoken"
	dingTalkWorkNoticeURL = "https://oapi.dingtalk.com/topapi/message/corpconversation/asyncsend_v2"
)

type larkAppConfig struct {
	AppID         string `json:"appId"`
	AppSecret     string `json:"appSecret"`
	ReceiveIDType string `json:"receiveIdType"`
	ReceiveID     string `json:"receiveId"`
	templateConfig
}

type wechatAppConfig struct {
	CorpID  string `json:"corpId"`
	AgentID string `json:"agentId"`
	Secret  string `json:"secret"`
	ToUser  string `json:"toUser"`
	ToParty string `json:"toParty"`
	ToTag   string `json:"toTag"`
	templateConfig
}

type dingTalkAppConfig struct {
	AppKey     string `json:"appKey"`
	AppSecret  string `json:"appSecret"`
	AgentID    string `json:"agentId"`
	UserIDList string `json:"useridList"`
	DeptIDList string `json:"deptIdList"`
	templateConfig
}

func (s *Service) sendLarkApp(ctx context.Context, cfg larkAppConfig, event AlertNotificationEvent) error {
	token, err := s.larkTenantAccessToken(ctx, cfg)
	if err != nil {
		return err
	}
	return s.postLarkAppMessage(ctx, cfg, token, larkAppAlertPayload(event, cfg.templateConfig))
}

func (s *Service) sendLarkAppPasswordReset(ctx context.Context, cfg larkAppConfig, message PasswordResetMessage) error {
	token, err := s.larkTenantAccessToken(ctx, cfg)
	if err != nil {
		return err
	}
	card := map[string]any{
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
	}
	payload := map[string]any{
		"msg_type": "interactive",
		"content":  mustJSONString(card),
	}
	return s.postLarkAppMessage(ctx, cfg, token, payload)
}

func larkAppAlertPayload(event AlertNotificationEvent, cfg templateConfig) map[string]any {
	payload := larkAlertPayload(event, cfg)
	if content, ok := payload["content"]; ok {
		payload["content"] = mustJSONString(content)
	}
	if card, ok := payload["card"]; ok {
		payload["content"] = mustJSONString(card)
		delete(payload, "card")
	}
	return payload
}

func (s *Service) larkTenantAccessToken(ctx context.Context, cfg larkAppConfig) (string, error) {
	if strings.TrimSpace(cfg.AppID) == "" {
		return "", fmt.Errorf("飞书 App ID 不能为空")
	}
	if strings.TrimSpace(cfg.AppSecret) == "" {
		return "", fmt.Errorf("飞书 App Secret 不能为空")
	}
	payload := map[string]string{"app_id": cfg.AppID, "app_secret": cfg.AppSecret}
	var result struct {
		Code              int    `json:"code"`
		Msg               string `json:"msg"`
		TenantAccessToken string `json:"tenant_access_token"`
	}
	if err := s.postJSONDecode(ctx, larkTenantTokenURL, payload, nil, &result); err != nil {
		return "", err
	}
	if result.Code != 0 {
		return "", fmt.Errorf("飞书获取访问令牌失败：%s", firstNonEmpty(result.Msg, strconv.Itoa(result.Code)))
	}
	if strings.TrimSpace(result.TenantAccessToken) == "" {
		return "", fmt.Errorf("飞书获取访问令牌失败")
	}
	return result.TenantAccessToken, nil
}

func (s *Service) postLarkAppMessage(ctx context.Context, cfg larkAppConfig, token string, payload map[string]any) error {
	receiveIDType := strings.TrimSpace(cfg.ReceiveIDType)
	receiveID := strings.TrimSpace(cfg.ReceiveID)
	if receiveIDType == "" || receiveID == "" {
		return fmt.Errorf("飞书接收 ID 类型和接收 ID 不能为空")
	}
	payload["receive_id"] = receiveID
	targetURL := larkMessageURL + "?receive_id_type=" + url.QueryEscape(receiveIDType)
	headers := map[string]string{"Authorization": "Bearer " + token}
	var result struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	if err := s.postJSONDecode(ctx, targetURL, payload, headers, &result); err != nil {
		return err
	}
	if result.Code != 0 {
		return fmt.Errorf("飞书应用消息发送失败：%s", firstNonEmpty(result.Msg, strconv.Itoa(result.Code)))
	}
	return nil
}

func (s *Service) sendWechatApp(ctx context.Context, cfg wechatAppConfig, event AlertNotificationEvent) error {
	token, err := s.wechatAccessToken(ctx, cfg)
	if err != nil {
		return err
	}
	return s.postWechatAppMessage(ctx, cfg, token, wechatAppAlertPayload(event, cfg.templateConfig))
}

func (s *Service) sendWechatAppPasswordReset(ctx context.Context, cfg wechatAppConfig, message PasswordResetMessage) error {
	token, err := s.wechatAccessToken(ctx, cfg)
	if err != nil {
		return err
	}
	payload := wechatAppTextPayload(cfg, passwordResetRobotText(message), true)
	return s.postWechatAppMessage(ctx, cfg, token, payload)
}

func wechatAppAlertPayload(event AlertNotificationEvent, cfg templateConfig) map[string]any {
	return wechatAppTextPayload(wechatAppConfig{templateConfig: cfg}, alertText(event, cfg), wechatMessageType(cfg) == "markdown")
}

func wechatAppTextPayload(cfg wechatAppConfig, text string, markdown bool) map[string]any {
	payload := map[string]any{
		"touser":  strings.TrimSpace(cfg.ToUser),
		"toparty": strings.TrimSpace(cfg.ToParty),
		"totag":   strings.TrimSpace(cfg.ToTag),
		"agentid": cfg.AgentID,
		"safe":    0,
	}
	if markdown {
		payload["msgtype"] = "markdown"
		payload["markdown"] = map[string]string{"content": text}
		return payload
	}
	payload["msgtype"] = "text"
	payload["text"] = map[string]string{"content": text}
	return payload
}

func (s *Service) wechatAccessToken(ctx context.Context, cfg wechatAppConfig) (string, error) {
	if strings.TrimSpace(cfg.CorpID) == "" {
		return "", fmt.Errorf("企业 ID 不能为空")
	}
	if strings.TrimSpace(cfg.Secret) == "" {
		return "", fmt.Errorf("应用 Secret 不能为空")
	}
	targetURL := wechatTokenURL + "?corpid=" + url.QueryEscape(cfg.CorpID) + "&corpsecret=" + url.QueryEscape(cfg.Secret)
	var result struct {
		ErrCode     int    `json:"errcode"`
		ErrMsg      string `json:"errmsg"`
		AccessToken string `json:"access_token"`
	}
	if err := s.getJSONDecode(ctx, targetURL, &result); err != nil {
		return "", err
	}
	if result.ErrCode != 0 {
		return "", fmt.Errorf("企业微信获取访问令牌失败：%s", firstNonEmpty(result.ErrMsg, strconv.Itoa(result.ErrCode)))
	}
	if strings.TrimSpace(result.AccessToken) == "" {
		return "", fmt.Errorf("企业微信获取访问令牌失败")
	}
	return result.AccessToken, nil
}

func (s *Service) postWechatAppMessage(ctx context.Context, cfg wechatAppConfig, token string, payload map[string]any) error {
	if strings.TrimSpace(cfg.AgentID) == "" {
		return fmt.Errorf("应用 AgentId 不能为空")
	}
	if strings.TrimSpace(cfg.ToUser) == "" && strings.TrimSpace(cfg.ToParty) == "" && strings.TrimSpace(cfg.ToTag) == "" {
		return fmt.Errorf("企业微信接收人、部门或标签至少填写一项")
	}
	payload["touser"] = strings.TrimSpace(cfg.ToUser)
	payload["toparty"] = strings.TrimSpace(cfg.ToParty)
	payload["totag"] = strings.TrimSpace(cfg.ToTag)
	payload["agentid"] = cfg.AgentID
	targetURL := wechatMessageURL + "?access_token=" + url.QueryEscape(token)
	var result struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	if err := s.postJSONDecode(ctx, targetURL, payload, nil, &result); err != nil {
		return err
	}
	if result.ErrCode != 0 {
		return fmt.Errorf("企业微信应用消息发送失败：%s", firstNonEmpty(result.ErrMsg, strconv.Itoa(result.ErrCode)))
	}
	return nil
}

func (s *Service) sendDingTalkApp(ctx context.Context, cfg dingTalkAppConfig, event AlertNotificationEvent) error {
	token, err := s.dingTalkAccessToken(ctx, cfg)
	if err != nil {
		return err
	}
	return s.postDingTalkAppMessage(ctx, cfg, token, dingTalkAppAlertPayload(event, cfg.templateConfig))
}

func (s *Service) sendDingTalkAppPasswordReset(ctx context.Context, cfg dingTalkAppConfig, message PasswordResetMessage) error {
	token, err := s.dingTalkAccessToken(ctx, cfg)
	if err != nil {
		return err
	}
	payload := dingTalkAppMarkdownPayload("KVM Manager 密码找回", dingtalkMarkdownLineBreaks(passwordResetRobotText(message)))
	return s.postDingTalkAppMessage(ctx, cfg, token, payload)
}

func dingTalkAppAlertPayload(event AlertNotificationEvent, cfg templateConfig) map[string]any {
	text := alertText(event, cfg)
	if dingTalkMessageType(cfg) == "markdown" {
		return dingTalkAppMarkdownPayload(alertSubject(event, cfg), dingtalkMarkdownLineBreaks(text))
	}
	return map[string]any{"msgtype": "text", "text": map[string]string{"content": text}}
}

func dingTalkAppMarkdownPayload(title string, text string) map[string]any {
	return map[string]any{"msgtype": "markdown", "markdown": map[string]string{"title": title, "text": text}}
}

func (s *Service) dingTalkAccessToken(ctx context.Context, cfg dingTalkAppConfig) (string, error) {
	if strings.TrimSpace(cfg.AppKey) == "" {
		return "", fmt.Errorf("钉钉 AppKey 不能为空")
	}
	if strings.TrimSpace(cfg.AppSecret) == "" {
		return "", fmt.Errorf("钉钉 AppSecret 不能为空")
	}
	targetURL := dingTalkTokenURL + "?appkey=" + url.QueryEscape(cfg.AppKey) + "&appsecret=" + url.QueryEscape(cfg.AppSecret)
	var result struct {
		ErrCode     int    `json:"errcode"`
		ErrMsg      string `json:"errmsg"`
		AccessToken string `json:"access_token"`
	}
	if err := s.getJSONDecode(ctx, targetURL, &result); err != nil {
		return "", err
	}
	if result.ErrCode != 0 {
		return "", fmt.Errorf("钉钉获取访问令牌失败：%s", firstNonEmpty(result.ErrMsg, strconv.Itoa(result.ErrCode)))
	}
	if strings.TrimSpace(result.AccessToken) == "" {
		return "", fmt.Errorf("钉钉获取访问令牌失败")
	}
	return result.AccessToken, nil
}

func (s *Service) postDingTalkAppMessage(ctx context.Context, cfg dingTalkAppConfig, token string, message map[string]any) error {
	agentID, err := strconv.ParseInt(strings.TrimSpace(cfg.AgentID), 10, 64)
	if err != nil || agentID <= 0 {
		return fmt.Errorf("应用 AgentId 必须是数字")
	}
	if strings.TrimSpace(cfg.UserIDList) == "" && strings.TrimSpace(cfg.DeptIDList) == "" {
		return fmt.Errorf("钉钉用户列表或部门列表至少填写一项")
	}
	payload := map[string]any{
		"agent_id": agentID,
		"msg":      message,
	}
	if userIDs := strings.TrimSpace(cfg.UserIDList); userIDs != "" {
		payload["userid_list"] = userIDs
	}
	if deptIDs := strings.TrimSpace(cfg.DeptIDList); deptIDs != "" {
		payload["dept_id_list"] = deptIDs
	}
	targetURL := dingTalkWorkNoticeURL + "?access_token=" + url.QueryEscape(token)
	var result struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	if err := s.postJSONDecode(ctx, targetURL, payload, nil, &result); err != nil {
		return err
	}
	if result.ErrCode != 0 {
		return fmt.Errorf("钉钉应用消息发送失败：%s", firstNonEmpty(result.ErrMsg, strconv.Itoa(result.ErrCode)))
	}
	return nil
}

func (s *Service) getJSONDecode(ctx context.Context, targetURL string, target any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return err
	}
	resp, err := s.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return decodeNotificationResponse(resp, target)
}

func (s *Service) postJSONDecode(ctx context.Context, targetURL string, payload any, headers map[string]string, target any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, targetURL, bytes.NewReader(body))
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
	return decodeNotificationResponse(resp, target)
}

func decodeNotificationResponse(resp *http.Response, target any) error {
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("通知接口返回异常状态：%s", resp.Status)
	}
	if len(bytes.TrimSpace(body)) == 0 {
		return nil
	}
	return json.Unmarshal(body, target)
}

func mustJSONString(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		return "{}"
	}
	return string(data)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return "unknown"
}
