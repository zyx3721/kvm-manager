package auth

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"kvm-manager/backend/internal/domain"
)

const (
	wecomAPIBase          = "https://qyapi.weixin.qq.com"
	wecomQRLoginBase      = "https://login.work.weixin.qq.com"
	wecomOAuthBase        = "https://open.weixin.qq.com"
	wecomAuthCenterHealth = "/healthz"
	wecomAuthCenterVerify = "/api/verify"
	wecomModeQRCode       = "qrcode"
	wecomModeInside       = "inside"
)

var ErrAuthProviderDisabled = errors.New("auth provider is disabled or missing")

// WeComUserInfo 企业微信换取到的用户身份，Name 可为空。
type WeComUserInfo struct {
	Userid string
	Name   string
}

// WeComClient 企业微信身份接口抽象，真实实现与 mock 实现均满足该接口。
type WeComClient interface {
	GetUserInfo(ctx context.Context, code string) (*WeComUserInfo, error)
	Ping(ctx context.Context) error
}

// WeComConfig 企业微信直连配置。
type WeComConfig struct {
	CorpID      string `json:"corpId"`
	AgentID     int    `json:"agentId"`
	Secret      string `json:"secret"`
	Mode        string `json:"mode"`
	ExternalURL string `json:"externalUrl"`
	FetchName   bool   `json:"fetchName"`
	Mock        bool   `json:"mock"`
}

func decodeWeComConfig(data []byte) (WeComConfig, error) {
	var cfg WeComConfig
	if len(data) > 0 {
		if err := json.Unmarshal(data, &cfg); err != nil {
			return WeComConfig{}, err
		}
	}
	cfg.CorpID = strings.TrimSpace(cfg.CorpID)
	cfg.Secret = strings.TrimSpace(cfg.Secret)
	cfg.ExternalURL = strings.TrimRight(strings.TrimSpace(cfg.ExternalURL), "/")
	if cfg.Mode != wecomModeInside {
		cfg.Mode = wecomModeQRCode
	}
	if cfg.ExternalURL == "" {
		return WeComConfig{}, fmt.Errorf("wecom external url is required")
	}
	if !cfg.Mock {
		if cfg.CorpID == "" || cfg.Secret == "" {
			return WeComConfig{}, fmt.Errorf("wecom corp id and secret are required")
		}
		if cfg.AgentID <= 0 {
			return WeComConfig{}, fmt.Errorf("wecom agent id is required")
		}
	}
	return cfg, nil
}

// CallbackURL 企业微信 OAuth 回调地址，基于对外访问地址拼接。
func (c WeComConfig) CallbackURL() string {
	return c.ExternalURL + "/api/auth/wecom/callback"
}

// AuthorizeURL 构造企业微信授权跳转：PC 浏览器扫码或企微内置浏览器网页授权。
func (c WeComConfig) AuthorizeURL(state string) string {
	q := url.Values{}
	q.Set("appid", c.CorpID)
	q.Set("agentid", strconv.Itoa(c.AgentID))
	q.Set("redirect_uri", c.CallbackURL())
	q.Set("state", state)
	if c.Mode == wecomModeInside {
		q.Set("response_type", "code")
		q.Set("scope", "snsapi_base")
		return wecomOAuthBase + "/connect/oauth2/authorize?" + q.Encode() + "#wechat_redirect"
	}
	q.Set("login_type", "CorpApp")
	return wecomQRLoginBase + "/wwlogin/sso/login?" + q.Encode()
}

// WeComCenterConfig 统一认证中心接入配置。
type WeComCenterConfig struct {
	BaseURL      string `json:"baseUrl"`
	App          string `json:"app"`
	AppSecret    string `json:"appSecret"`
	VerifyTSSkew int    `json:"verifyTsSkew"`
}

func decodeWeComCenterConfig(data []byte) (WeComCenterConfig, error) {
	var cfg WeComCenterConfig
	if len(data) > 0 {
		if err := json.Unmarshal(data, &cfg); err != nil {
			return WeComCenterConfig{}, err
		}
	}
	cfg.BaseURL = strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	cfg.App = strings.TrimSpace(cfg.App)
	cfg.AppSecret = strings.TrimSpace(cfg.AppSecret)
	if cfg.BaseURL == "" || cfg.App == "" || cfg.AppSecret == "" {
		return WeComCenterConfig{}, fmt.Errorf("wecom auth center base url, app and app secret are required")
	}
	if cfg.VerifyTSSkew <= 0 {
		cfg.VerifyTSSkew = 60
	}
	return cfg, nil
}

// LoginURL 构造统一认证中心登录跳转，redirect 为本平台站内路径。
func (c WeComCenterConfig) LoginURL(redirect string) string {
	q := url.Values{}
	q.Set("app", c.App)
	q.Set("redirect", redirect)
	return c.BaseURL + "/login?" + q.Encode()
}

// signWeComCenterTicket 与认证中心 /api/verify 约定一致：
// sign = hex(HMAC-SHA256(key=app_secret, msg=app+"\n"+ticket+"\n"+ts))。
func signWeComCenterTicket(appSecret, app, ticket string, ts int64) string {
	mac := hmac.New(sha256.New, []byte(appSecret))
	fmt.Fprintf(mac, "%s\n%s\n%s", app, ticket, strconv.FormatInt(ts, 10))
	return hex.EncodeToString(mac.Sum(nil))
}

// verifyWeComCenterTicket 后端发起 verify 换取身份，app_secret 不出服务端。
func verifyWeComCenterTicket(ctx context.Context, cfg WeComCenterConfig, ticket string) (*WeComUserInfo, error) {
	if strings.TrimSpace(ticket) == "" {
		return nil, ErrInvalidState
	}
	ts := time.Now().Unix()
	payload, err := json.Marshal(map[string]any{
		"app":    cfg.App,
		"ticket": ticket,
		"ts":     ts,
		"sign":   signWeComCenterTicket(cfg.AppSecret, cfg.App, ticket, ts),
	})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.BaseURL+wecomAuthCenterVerify, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	httpc := &http.Client{Timeout: 10 * time.Second}
	resp, err := httpc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request wecom auth center failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil, fmt.Errorf("wecom auth center verify failed with http %d", resp.StatusCode)
	}
	var data struct {
		Userid string `json:"userid"`
		Name   string `json:"name"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 64<<10)).Decode(&data); err != nil {
		return nil, fmt.Errorf("decode wecom auth center response failed: %w", err)
	}
	if strings.TrimSpace(data.Userid) == "" {
		return nil, fmt.Errorf("wecom auth center verify returned empty userid")
	}
	return &WeComUserInfo{Userid: data.Userid, Name: data.Name}, nil
}

// wecomRealClient 企业微信真实实现：gettoken 缓存 + getuserinfo（可选 user/get 补全姓名）。
type wecomRealClient struct {
	corpID    string
	agentID   int
	secret    string
	fetchName bool
	base      string

	httpc *http.Client

	mu          sync.Mutex
	accessToken string
	tokenExpiry time.Time
	now         func() time.Time
}

func newWeComRealClient(cfg WeComConfig) *wecomRealClient {
	return &wecomRealClient{
		corpID:    cfg.CorpID,
		agentID:   cfg.AgentID,
		secret:    cfg.Secret,
		fetchName: cfg.FetchName,
		base:      wecomAPIBase,
		httpc:     &http.Client{Timeout: 10 * time.Second},
		now:       time.Now,
	}
}

// GetUserInfo 用 code 换取用户身份。
func (c *wecomRealClient) GetUserInfo(ctx context.Context, code string) (*WeComUserInfo, error) {
	token, err := c.token(ctx)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Errcode int    `json:"errcode"`
		Errmsg  string `json:"errmsg"`
		Userid  string `json:"userid"`
	}
	if err := c.getJSON(ctx, fmt.Sprintf("%s/cgi-bin/auth/getuserinfo?access_token=%s&code=%s", c.base, url.QueryEscape(token), url.QueryEscape(code)), &resp); err != nil {
		return nil, err
	}
	if resp.Errcode != 0 {
		return nil, fmt.Errorf("wecom getuserinfo errcode=%d errmsg=%s", resp.Errcode, resp.Errmsg)
	}
	if resp.Userid == "" {
		return nil, errors.New("wecom getuserinfo returned empty userid (user may not be a corp member)")
	}
	userInfo := &WeComUserInfo{Userid: resp.Userid}
	if c.fetchName {
		userInfo.Name, _ = c.fetchDisplayName(ctx, token, resp.Userid)
	}
	return userInfo, nil
}

// Ping 校验企业凭证：调用 gettoken，成功即认为配置可用。
func (c *wecomRealClient) Ping(ctx context.Context) error {
	_, err := c.token(ctx)
	return err
}

// token 返回缓存的 access_token；到期前 5 分钟主动刷新，并发调用仅触发一次请求。
func (c *wecomRealClient) token(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.accessToken != "" && c.now().Before(c.tokenExpiry) {
		return c.accessToken, nil
	}
	var resp struct {
		Errcode     int    `json:"errcode"`
		Errmsg      string `json:"errmsg"`
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	u := fmt.Sprintf("%s/cgi-bin/gettoken?corpid=%s&corpsecret=%s", c.base, url.QueryEscape(c.corpID), url.QueryEscape(c.secret))
	if err := c.getJSON(ctx, u, &resp); err != nil {
		return "", err
	}
	if resp.Errcode != 0 {
		return "", fmt.Errorf("wecom gettoken errcode=%d errmsg=%s", resp.Errcode, resp.Errmsg)
	}
	if resp.AccessToken == "" {
		return "", errors.New("wecom gettoken returned empty access_token")
	}
	c.accessToken = resp.AccessToken
	c.tokenExpiry = c.now().Add(time.Duration(resp.ExpiresIn-300) * time.Second)
	return c.accessToken, nil
}

// fetchName 通过通讯录接口补全姓名，姓名属于展示信息，取不到不阻断登录。
func (c *wecomRealClient) fetchDisplayName(ctx context.Context, token, userid string) (string, error) {
	var resp struct {
		Errcode int    `json:"errcode"`
		Errmsg  string `json:"errmsg"`
		Name    string `json:"name"`
	}
	if err := c.getJSON(ctx, fmt.Sprintf("%s/cgi-bin/user/get?access_token=%s&userid=%s", c.base, url.QueryEscape(token), url.QueryEscape(userid)), &resp); err != nil {
		return "", err
	}
	if resp.Errcode != 0 {
		return "", fmt.Errorf("wecom user/get errcode=%d errmsg=%s", resp.Errcode, resp.Errmsg)
	}
	return resp.Name, nil
}

func (c *wecomRealClient) getJSON(ctx context.Context, u string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return fmt.Errorf("build wecom request failed: %w", err)
	}
	httpResp, err := c.httpc.Do(req)
	if err != nil {
		return fmt.Errorf("request wecom api failed: %w", err)
	}
	defer httpResp.Body.Close()
	if httpResp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, httpResp.Body)
		return fmt.Errorf("wecom api http %d: %s", httpResp.StatusCode, u)
	}
	if err := json.NewDecoder(io.LimitReader(httpResp.Body, 256<<10)).Decode(out); err != nil {
		return fmt.Errorf("decode wecom response failed: %w", err)
	}
	return nil
}

// wecomMockClient 本地演练用：任意 code 返回固定模拟用户，Ping 恒通过。
type wecomMockClient struct{}

func (wecomMockClient) GetUserInfo(context.Context, string) (*WeComUserInfo, error) {
	return &WeComUserInfo{Userid: "mockuser", Name: "mock user"}, nil
}

func (wecomMockClient) Ping(context.Context) error { return nil }

var (
	wecomClientMu   sync.Mutex
	wecomClientKey  string
	wecomClientHold WeComClient
)

// getWeComClient 按配置指纹缓存真实客户端，避免每次扫码都重新获取 access_token。
// 配置变更（如更换 Secret）后指纹变化会自动重建。
func getWeComClient(cfg WeComConfig) WeComClient {
	if cfg.Mock {
		return wecomMockClient{}
	}
	key := strings.Join([]string{cfg.CorpID, strconv.Itoa(cfg.AgentID), cfg.Secret, strconv.FormatBool(cfg.FetchName)}, "|")
	wecomClientMu.Lock()
	defer wecomClientMu.Unlock()
	if wecomClientHold != nil && wecomClientKey == key {
		return wecomClientHold
	}
	client := newWeComRealClient(cfg)
	wecomClientHold = client
	wecomClientKey = key
	return client
}

func mockWeComCode() (string, error) {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return "mock-" + hex.EncodeToString(buf), nil
}

// TestWeComProvider 企业微信直连连接测试：校验企业凭证可换取 access_token。
func TestWeComProvider(ctx context.Context, provider domain.AuthProvider) error {
	cfg, err := decodeWeComConfig(provider.Config)
	if err != nil {
		return err
	}
	return getWeComClient(cfg).Ping(ctx)
}

// TestWeComCenterProvider 统一认证中心连接测试：认证中心健康检查通过即可。
func TestWeComCenterProvider(ctx context.Context, provider domain.AuthProvider) error {
	cfg, err := decodeWeComCenterConfig(provider.Config)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, cfg.BaseURL+wecomAuthCenterHealth, nil)
	if err != nil {
		return err
	}
	httpc := &http.Client{Timeout: 10 * time.Second}
	resp, err := httpc.Do(req)
	if err != nil {
		return fmt.Errorf("request wecom auth center failed: %w", err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("wecom auth center health check failed with http %d", resp.StatusCode)
	}
	return nil
}

// WeComUserMessage 将企业微信认证错误翻译为用户可见的中文提示。
func WeComUserMessage(err error) string {
	if err == nil {
		return ""
	}
	message := strings.ToLower(err.Error())
	switch {
	case errors.Is(err, ErrAuthProviderDisabled):
		return "企业微信认证未启用，请先在系统配置中开启"
	case errors.Is(err, ErrInvalidState):
		return "登录状态已过期或无效，请重新发起企业微信登录"
	case strings.Contains(message, "gettoken"):
		return "企业微信应用凭证校验失败，请检查企业 ID、应用 AgentId 与 Secret 配置"
	case strings.Contains(message, "request wecom auth center failed"):
		return "统一认证中心连接失败，请检查认证中心地址与网络连通性"
	case strings.Contains(message, "wecom auth center verify failed"):
		return "统一认证中心票据校验失败，登录已过期，请重新发起登录"
	case strings.Contains(message, "wecom auth center"):
		return "统一认证中心返回数据异常，请稍后重试"
	case strings.Contains(message, "request wecom api failed") || strings.Contains(message, "wecom api http"):
		return "企业微信接口连接失败，请检查网络连通性与对外访问地址配置"
	case strings.Contains(message, "not be a corp member"):
		return "当前企业微信账号不是该应用的可见成员，请联系管理员调整应用可见范围"
	case strings.Contains(message, "external url"):
		return "外部访问地址未配置，请完善企业微信认证配置"
	case strings.Contains(message, "corp id and secret") || strings.Contains(message, "agent id"):
		return "企业微信应用配置不完整，请完善企业 ID、AgentId 与 Secret"
	case strings.Contains(message, "base url, app and app secret"):
		return "统一认证中心配置不完整，请完善认证中心地址、应用标识与应用密钥"
	}
	return "企业微信认证失败：" + err.Error()
}
