package auth

import (
	"bytes"
	"context"
	"crypto/hmac"
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
	wecomAuthCenterHealth = "/healthz"
	wecomAuthCenterVerify = "/api/verify"
)

// 企业微信认证方式：direct 直连自建应用，sso 经由统一认证中心。
const (
	WecomModeDirect = "direct"
	WecomModeSSO    = "sso"
)

var ErrAuthProviderDisabled = errors.New("auth provider is disabled or missing")
var ErrWecomNotBound = errors.New("wecom account is not bound to a platform user")
var ErrWecomAlreadyBound = errors.New("wecom account is already bound to another user")

// WeComUserInfo 企业微信换取到的用户身份，Name 可为空。
type WeComUserInfo struct {
	Userid string
	Name   string
}

// WeComClient 企业微信身份接口抽象，真实实现与测试实现均满足该接口。
type WeComClient interface {
	GetUserInfo(ctx context.Context, code string) (*WeComUserInfo, error)
	Ping(ctx context.Context) error
}

// WecomProviderConfig 企业微信认证配置：authMode 切换直连与统一认证中心，
// 两种模式的凭据分别保存，切换模式互不丢失。
type WecomProviderConfig struct {
	AuthMode       string `json:"authMode"`
	CorpID         string `json:"corpid"`
	AgentID        int    `json:"agentid"`
	Secret         string `json:"secret"`
	RedirectPrefix string `json:"redirectPrefix"`
	SSOBaseURL     string `json:"ssoBaseUrl"`
	SSOAppID       string `json:"ssoAppID"`
	SSOAppSecret   string `json:"ssoAppSecret"`
}

func decodeWecomProviderConfig(data []byte) (WecomProviderConfig, error) {
	// 用 map 手工解析：历史数据中 agentid 可能以字符串数字存储，直接解到 int 会失败
	var raw map[string]any
	if len(data) > 0 {
		if err := json.Unmarshal(data, &raw); err != nil {
			return WecomProviderConfig{}, err
		}
	}
	text := func(key string) string {
		value, _ := raw[key].(string)
		return strings.TrimSpace(value)
	}
	cfg := WecomProviderConfig{
		AuthMode:       text("authMode"),
		CorpID:         text("corpid"),
		AgentID:        wecomIntValue(raw["agentid"]),
		Secret:         text("secret"),
		RedirectPrefix: strings.TrimRight(text("redirectPrefix"), "/"),
		SSOBaseURL:     strings.TrimRight(text("ssoBaseUrl"), "/"),
		SSOAppID:       text("ssoAppID"),
		SSOAppSecret:   text("ssoAppSecret"),
	}
	if cfg.AuthMode != WecomModeSSO {
		cfg.AuthMode = WecomModeDirect
	}
	if cfg.AuthMode == WecomModeSSO {
		if cfg.SSOBaseURL == "" || cfg.SSOAppID == "" || cfg.SSOAppSecret == "" {
			return WecomProviderConfig{}, fmt.Errorf("wecom sso base url, app id and app secret are required")
		}
		return cfg, nil
	}
	if cfg.CorpID == "" || cfg.Secret == "" {
		return WecomProviderConfig{}, fmt.Errorf("wecom corpid and secret are required")
	}
	if cfg.AgentID <= 0 {
		return WecomProviderConfig{}, fmt.Errorf("wecom agentid is required")
	}
	return cfg, nil
}

// wecomIntValue 兼容数字与字符串数字的历史数据形态。
func wecomIntValue(value any) int {
	switch typed := value.(type) {
	case float64:
		return int(typed)
	case int:
		return typed
	case json.Number:
		number, _ := typed.Float64()
		return int(number)
	case string:
		number, _ := strconv.Atoi(strings.TrimSpace(typed))
		return number
	default:
		return 0
	}
}

// redirectBase 回调地址前缀：配置了回调地址前缀则优先，否则按用户当前访问地址推断。
func (c WecomProviderConfig) redirectBase(requestBase string) string {
	if c.RedirectPrefix != "" {
		return c.RedirectPrefix
	}
	return strings.TrimRight(requestBase, "/")
}

// AuthorizeURL 直连模式构造企业微信授权跳转（PC 浏览器扫码）。
func (c WecomProviderConfig) AuthorizeURL(state, requestBase string) string {
	q := url.Values{}
	q.Set("login_type", "CorpApp")
	q.Set("appid", c.CorpID)
	q.Set("agentid", strconv.Itoa(c.AgentID))
	q.Set("redirect_uri", c.redirectBase(requestBase)+"/api/auth/wecom/callback")
	q.Set("state", state)
	return wecomQRLoginBase + "/wwlogin/sso/login?" + q.Encode()
}

// WecomSSOLoginURL 统一认证中心模式返回认证中心登录页地址，本系统不签发 state。
func (c WecomProviderConfig) WecomSSOLoginURL(redirect string) string {
	q := url.Values{}
	q.Set("app", c.SSOAppID)
	if redirect != "" {
		q.Set("redirect", redirect)
	}
	return c.SSOBaseURL + "/login?" + q.Encode()
}

// signWeComCenterTicket 与认证中心 /api/verify 约定一致：
// sign = hex(HMAC-SHA256(key=app_secret, msg=app+"\n"+ticket+"\n"+ts))。
func signWeComCenterTicket(appSecret, app, ticket string, ts int64) string {
	mac := hmac.New(sha256.New, []byte(appSecret))
	fmt.Fprintf(mac, "%s\n%s\n%s", app, ticket, strconv.FormatInt(ts, 10))
	return hex.EncodeToString(mac.Sum(nil))
}

// verifyWeComCenterTicket 后端发起 verify 换取身份，app_secret 不出服务端。
func verifyWeComCenterTicket(ctx context.Context, cfg WecomProviderConfig, ticket string) (*WeComUserInfo, error) {
	if strings.TrimSpace(ticket) == "" {
		return nil, ErrInvalidState
	}
	ts := time.Now().Unix()
	payload, err := json.Marshal(map[string]any{
		"app":    cfg.SSOAppID,
		"ticket": ticket,
		"ts":     ts,
		"sign":   signWeComCenterTicket(cfg.SSOAppSecret, cfg.SSOAppID, ticket, ts),
	})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.SSOBaseURL+wecomAuthCenterVerify, bytes.NewReader(payload))
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
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
		var failure struct {
			Error string `json:"error"`
		}
		_ = json.Unmarshal(body, &failure)
		return nil, fmt.Errorf("wecom auth center verify failed with http %d error=%s", resp.StatusCode, failure.Error)
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
	corpID  string
	agentID int
	secret  string
	base    string

	httpc *http.Client

	mu          sync.Mutex
	accessToken string
	tokenExpiry time.Time
	now         func() time.Time
}

func newWeComRealClient(cfg WecomProviderConfig) *wecomRealClient {
	return &wecomRealClient{
		corpID:  cfg.CorpID,
		agentID: cfg.AgentID,
		secret:  cfg.Secret,
		base:    wecomAPIBase,
		httpc:   &http.Client{Timeout: 10 * time.Second},
		now:     time.Now,
	}
}

// GetUserInfo 用 code 换取用户身份；access_token 失效时清缓存重试一次。
func (c *wecomRealClient) GetUserInfo(ctx context.Context, code string) (*WeComUserInfo, error) {
	userInfo, err := c.getUserInfo(ctx, code)
	if err != nil && isWecomTokenInvalidError(err) {
		c.invalidateToken()
		return c.getUserInfo(ctx, code)
	}
	return userInfo, err
}

func (c *wecomRealClient) getUserInfo(ctx context.Context, code string) (*WeComUserInfo, error) {
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
	return &WeComUserInfo{Userid: resp.Userid}, nil
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

// isWecomTokenInvalidError 识别 access_token 过期/失效错误码：40014 不合法、42001 已过期。
func isWecomTokenInvalidError(err error) bool {
	if err == nil {
		return false
	}
	message := err.Error()
	return strings.Contains(message, "errcode=40014") || strings.Contains(message, "errcode=42001")
}

func (c *wecomRealClient) invalidateToken() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.accessToken = ""
	c.tokenExpiry = time.Time{}
}

var (
	wecomClientMu      sync.Mutex
	wecomClientKey     string
	wecomClientHold    WeComClient
	wecomClientFactory func(WecomProviderConfig) WeComClient
)

// getWeComClient 按凭据指纹缓存客户端，避免每次扫码都重新获取 access_token；
// 凭据变更（如更换 Secret）后指纹变化会自动重建。
func getWeComClient(cfg WecomProviderConfig) WeComClient {
	key := strings.Join([]string{cfg.CorpID, strconv.Itoa(cfg.AgentID), cfg.Secret}, "|")
	wecomClientMu.Lock()
	defer wecomClientMu.Unlock()
	if wecomClientHold != nil && wecomClientKey == key {
		return wecomClientHold
	}
	factory := wecomClientFactory
	if factory == nil {
		factory = func(c WecomProviderConfig) WeComClient { return newWeComRealClient(c) }
	}
	client := factory(cfg)
	wecomClientHold = client
	wecomClientKey = key
	return client
}

// TestWeComProvider 企业微信连接测试：直连校验企业凭证可换取 access_token，统一认证中心调用健康检查。
func TestWeComProvider(ctx context.Context, provider domain.AuthProvider) error {
	cfg, err := decodeWecomProviderConfig(provider.Config)
	if err != nil {
		return err
	}
	if cfg.AuthMode == WecomModeSSO {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, cfg.SSOBaseURL+wecomAuthCenterHealth, nil)
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
	return getWeComClient(cfg).Ping(ctx)
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
	case errors.Is(err, ErrWecomNotBound):
		return "该企业微信账号尚未绑定系统用户，请先使用账号密码登录后在右上角绑定企微账号"
	case errors.Is(err, ErrWecomAlreadyBound):
		return "该企业微信账号已绑定其他用户"
	case errors.Is(err, ErrInvalidState):
		return "登录状态已过期或无效，请重新发起企业微信登录"
	case strings.Contains(message, "errcode=40029"):
		return "企业微信授权码无效或已使用，请重新扫码"
	case strings.Contains(message, "errcode=60020"):
		return "企业微信应用 IP 不在可信域名内，请检查应用配置"
	case strings.Contains(message, "gettoken"):
		return "企业微信应用凭证校验失败，请检查企业 ID（corpid）、应用 AgentID 与应用 Secret 配置"
	case strings.Contains(message, "request wecom auth center failed"):
		return "统一认证中心连接失败，请检查认证中心地址与网络连通性"
	case strings.Contains(message, "error=invalid_app"):
		return "统一认证中心未登记该应用标识，请检查应用标识配置"
	case strings.Contains(message, "error=invalid_sign"):
		return "统一认证中心签名校验失败，请检查应用密钥配置"
	case strings.Contains(message, "error=expired_ts"):
		return "统一认证中心校验时间偏差过大，请校准系统时间后重试"
	case strings.Contains(message, "error=invalid_ticket"):
		return "统一认证中心票据无效或已使用，请重新发起登录"
	case strings.Contains(message, "wecom auth center verify failed"):
		return "统一认证中心票据校验失败，登录已过期，请重新发起登录"
	case strings.Contains(message, "wecom auth center"):
		return "统一认证中心返回数据异常，请稍后重试"
	case strings.Contains(message, "request wecom api failed") || strings.Contains(message, "wecom api http"):
		return "企业微信接口连接失败，请检查网络连通性与回调地址前缀配置"
	case strings.Contains(message, "not be a corp member"):
		return "当前企业微信账号不是该应用的可见成员，请联系管理员调整应用可见范围"
	case strings.Contains(message, "corpid and secret") || strings.Contains(message, "agentid"):
		return "企业微信应用配置不完整，请完善企业 ID（corpid）、应用 AgentID 与应用 Secret"
	case strings.Contains(message, "sso base url, app id and app secret"):
		return "统一认证中心配置不完整，请完善认证中心地址、应用标识与应用密钥"
	}
	return "企业微信认证失败：" + err.Error()
}
