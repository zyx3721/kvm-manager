package auth

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"kvm-manager/backend/internal/domain"
)

func TestSignWeComCenterTicketMatchesAuthCenterContract(t *testing.T) {
	// 与统一认证中心 /api/verify 的签名约定一致：
	// sign = hex(HMAC-SHA256(key=app_secret, msg=app+"\n"+ticket+"\n"+ts))
	got := signWeComCenterTicket("secret", "app", "ticket", 1726650000)
	want := "a126dc99eaba9945da2deaffce569dc3fe332992e69a243a734597a331ceaadf"
	if got != want {
		t.Fatalf("sign mismatch: got %s want %s", got, want)
	}
	if len(got) != 64 {
		t.Fatalf("sign should be 64 hex chars, got %d", len(got))
	}
}

func TestDecodeWecomProviderConfigNormalizesFields(t *testing.T) {
	config := []byte(`{"authMode":"direct","corpid":" ww123 ","agentid":1000002,"secret":" s ","redirectPrefix":"https://kvm.example.com/"}`)
	cfg, err := decodeWecomProviderConfig(config)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if cfg.CorpID != "ww123" || cfg.Secret != "s" {
		t.Fatalf("fields not trimmed: %+v", cfg)
	}
	if cfg.RedirectPrefix != "https://kvm.example.com" {
		t.Fatalf("redirect prefix trailing slash should be trimmed, got %q", cfg.RedirectPrefix)
	}
}

func TestDecodeWecomProviderConfigDefaultsToDirect(t *testing.T) {
	config := []byte(`{"corpid":"ww123","agentid":1000002,"secret":"s"}`)
	cfg, err := decodeWecomProviderConfig(config)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if cfg.AuthMode != WecomModeDirect {
		t.Fatalf("unknown auth mode should fall back to direct, got %q", cfg.AuthMode)
	}
}

func TestDecodeWecomProviderConfigRequiresModeCredentials(t *testing.T) {
	direct := []byte(`{"authMode":"direct"}`)
	if _, err := decodeWecomProviderConfig(direct); err == nil {
		t.Fatal("direct mode should require corpid, agentid and secret")
	}
	sso := []byte(`{"authMode":"sso"}`)
	if _, err := decodeWecomProviderConfig(sso); err == nil {
		t.Fatal("sso mode should require base url, app id and app secret")
	}
	// 切换模式互不丢失：sso 模式不校验直连凭据
	ssoFull := []byte(`{"authMode":"sso","ssoBaseUrl":"https://auth.example.com","ssoAppID":"kvm","ssoAppSecret":"s"}`)
	if _, err := decodeWecomProviderConfig(ssoFull); err != nil {
		t.Fatalf("sso config should not require direct credentials: %v", err)
	}
}

func TestWecomProviderConfigAuthorizeURL(t *testing.T) {
	cfg := WecomProviderConfig{AuthMode: WecomModeDirect, CorpID: "ww123", AgentID: 1000002, RedirectPrefix: "https://kvm.example.com"}
	authorizeURL := cfg.AuthorizeURL("state-1", "http://inferred.example.com")
	for _, want := range []string{
		"https://login.work.weixin.qq.com/wwlogin/sso/login?",
		"login_type=CorpApp",
		"appid=ww123",
		"agentid=1000002",
		"redirect_uri=https%3A%2F%2Fkvm.example.com%2Fapi%2Fauth%2Fwecom%2Fcallback",
		"state=state-1",
	} {
		if !strings.Contains(authorizeURL, want) {
			t.Fatalf("authorize url missing %s: %s", want, authorizeURL)
		}
	}
	// 未配置回调地址前缀时按请求地址推断
	cfg.RedirectPrefix = ""
	inferredURL := cfg.AuthorizeURL("state-2", "https://kvm.local")
	if !strings.Contains(inferredURL, "redirect_uri=https%3A%2F%2Fkvm.local%2Fapi%2Fauth%2Fwecom%2Fcallback") {
		t.Fatalf("inferred redirect_uri mismatch: %s", inferredURL)
	}
}

func TestWecomProviderConfigSSOLoginURL(t *testing.T) {
	cfg := WecomProviderConfig{AuthMode: WecomModeSSO, SSOBaseURL: "https://auth.example.com", SSOAppID: "kvm"}
	loginURL := cfg.WecomSSOLoginURL("/vms")
	if loginURL != "https://auth.example.com/login?app=kvm&redirect=%2Fvms" {
		t.Fatalf("unexpected sso login url: %s", loginURL)
	}
}

func TestVerifyWeComCenterTicketRejectsEmptyTicket(t *testing.T) {
	cfg := WecomProviderConfig{AuthMode: WecomModeSSO, SSOBaseURL: "https://auth.example.com", SSOAppID: "kvm", SSOAppSecret: "s"}
	if _, err := verifyWeComCenterTicket(context.Background(), cfg, "  "); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("empty ticket should reject with ErrInvalidState, got %v", err)
	}
}

func wecomTestService(t *testing.T, store *fakeStore) *Service {
	t.Helper()
	return NewService(store, time.Hour)
}

func enabledWecomProvider(t *testing.T, config string) domain.AuthProvider {
	t.Helper()
	return domain.AuthProvider{ID: "wecom", Type: "wecom", Name: "企业微信", Enabled: true, Config: []byte(config)}
}

func directWecomConfig() string {
	return `{"authMode":"direct","corpid":"ww123","agentid":1000002,"secret":"s","redirectPrefix":"https://kvm.example.com"}`
}

// fakeWecomClient 测试注入：任意 code 返回固定企微账号。
type fakeWecomClient struct {
	userid string
}

func (f fakeWecomClient) GetUserInfo(context.Context, string) (*WeComUserInfo, error) {
	return &WeComUserInfo{Userid: f.userid}, nil
}

func (fakeWecomClient) Ping(context.Context) error { return nil }

// useFakeWecomClient 替换企微客户端工厂，测试结束自动还原。
func useFakeWecomClient(t *testing.T, userid string) {
	t.Helper()
	wecomClientMu.Lock()
	previousFactory := wecomClientFactory
	previousHold := wecomClientHold
	previousKey := wecomClientKey
	wecomClientFactory = func(WecomProviderConfig) WeComClient {
		return fakeWecomClient{userid: userid}
	}
	wecomClientHold = nil
	wecomClientKey = ""
	wecomClientMu.Unlock()
	t.Cleanup(func() {
		wecomClientMu.Lock()
		wecomClientFactory = previousFactory
		wecomClientHold = previousHold
		wecomClientKey = previousKey
		wecomClientMu.Unlock()
	})
}

func TestWeComLoginURLDirectStoresState(t *testing.T) {
	store := &fakeStore{provider: enabledWecomProvider(t, directWecomConfig())}
	service := wecomTestService(t, store)

	target, err := service.WeComLoginURL(context.Background(), "/vms", "10.0.0.1", "")
	if err != nil {
		t.Fatalf("login url failed: %v", err)
	}
	if !strings.HasPrefix(target, "https://login.work.weixin.qq.com/wwlogin/sso/login?") {
		t.Fatalf("direct login should jump to wecom authorize page, got %s", target)
	}
	if len(store.states) != 1 {
		t.Fatalf("one state should be stored, got %d", len(store.states))
	}
	for _, rec := range store.states {
		if rec.Redirect != "/vms" || rec.RemoteIP != "10.0.0.1" || rec.Provider != "wecom" {
			t.Fatalf("state record mismatch: %+v", rec)
		}
		if rec.Purpose != AuthPurposeLogin {
			t.Fatalf("login authorize purpose mismatch: %+v", rec)
		}
	}
}

func TestWeComLoginURLSSOReturnsCenterAddress(t *testing.T) {
	store := &fakeStore{provider: enabledWecomProvider(t, `{"authMode":"sso","ssoBaseUrl":"https://auth.example.com","ssoAppID":"kvm","ssoAppSecret":"s"}`)}
	service := wecomTestService(t, store)

	target, err := service.WeComLoginURL(context.Background(), "/vms", "", "")
	if err != nil {
		t.Fatalf("login url failed: %v", err)
	}
	if target != "https://auth.example.com/login?app=kvm&redirect=%2Fvms" {
		t.Fatalf("sso login should return center address, got %s", target)
	}
	if len(store.states) != 0 {
		t.Fatalf("sso login should not sign local state, got %d", len(store.states))
	}
}

func TestWeComStateTTLFollowsBaseConfig(t *testing.T) {
	store := &fakeStore{
		provider:   enabledWecomProvider(t, directWecomConfig()),
		baseConfig: domain.SystemBaseConfig{WecomStateTTLMinutes: 30},
	}
	service := wecomTestService(t, store)
	if _, err := service.WeComLoginURL(context.Background(), "/", "", ""); err != nil {
		t.Fatalf("login url failed: %v", err)
	}
	for _, rec := range store.states {
		if delta := time.Until(rec.ExpiresAt); delta < 29*time.Minute || delta > 31*time.Minute {
			t.Fatalf("state ttl should follow base config 30m, got %v", delta)
		}
	}
}

func TestWeComLoginURLRequiresEnabledProvider(t *testing.T) {
	store := &fakeStore{}
	service := wecomTestService(t, store)
	if _, err := service.WeComLoginURL(context.Background(), "/", "", ""); !errors.Is(err, ErrAuthProviderDisabled) {
		t.Fatalf("disabled provider should reject, got %v", err)
	}
}

// runWecomCallback 用注入的假客户端走完一次登录或绑定回调，返回结果。
func runWecomCallback(t *testing.T, service *Service, target, redirect string) (WecomResult, error) {
	t.Helper()
	parsed, err := url.Parse(target)
	if err != nil {
		t.Fatalf("parse target failed: %v", err)
	}
	return service.WeComCallback(context.Background(), "good-code", parsed.Query().Get("state"))
}

func TestWeComBindThenLoginFlow(t *testing.T) {
	useFakeWecomClient(t, "zhangsan-wecom-id")
	store := &fakeStore{
		provider: enabledWecomProvider(t, directWecomConfig()),
		user:     domain.User{ID: "u-1", Username: "zhangsan", Disabled: false},
		userByID: domain.User{ID: "u-1", Username: "zhangsan"},
	}
	service := wecomTestService(t, store)

	// 绑定
	bindTarget, err := service.WeComBindURL(context.Background(), "u-1", "10.0.0.1", "")
	if err != nil {
		t.Fatalf("bind url failed: %v", err)
	}
	bindState := parseStateFromTarget(t, bindTarget)
	for _, rec := range store.states {
		if rec.Purpose != AuthPurposeBind || rec.UserID != "u-1" {
			t.Fatalf("bind state mismatch: %+v", rec)
		}
	}
	bindResult, err := runWecomCallback(t, service, bindTarget, "")
	if err != nil {
		t.Fatalf("bind callback failed: %v", err)
	}
	if bindResult.Kind != AuthPurposeBind || bindResult.Userid != "zhangsan-wecom-id" || bindResult.Username != "zhangsan" {
		t.Fatalf("bind result mismatch: %+v", bindResult)
	}
	if store.bindings["zhangsan-wecom-id"] != "u-1" {
		t.Fatalf("binding should be stored, got %v", store.bindings)
	}

	// 绑定后登录
	loginTarget, err := service.WeComLoginURL(context.Background(), "/vms", "", "")
	if err != nil {
		t.Fatalf("login url failed: %v", err)
	}
	loginResult, err := runWecomCallback(t, service, loginTarget, "")
	if err != nil {
		t.Fatalf("login callback failed: %v", err)
	}
	if loginResult.Kind != AuthPurposeLogin || loginResult.Redirect != "/vms" {
		t.Fatalf("login result mismatch: %+v", loginResult)
	}
	if loginResult.Session.Token == "" || loginResult.Session.User.Username != "zhangsan" {
		t.Fatalf("session should be issued for bound user: %+v", loginResult.Session)
	}
	if !loginResult.Session.WecomBound {
		t.Fatalf("login session should carry wecomBound=true: %+v", loginResult.Session)
	}

	// state 取出即删：重放同一绑定回调必须失败
	_, err = service.WeComCallback(context.Background(), "good-code", bindState)
	if !errors.Is(err, ErrInvalidState) {
		t.Fatalf("replayed state should reject, got %v", err)
	}
}

func parseStateFromTarget(t *testing.T, target string) string {
	t.Helper()
	parsed, err := url.Parse(target)
	if err != nil {
		t.Fatalf("parse target failed: %v", err)
	}
	return parsed.Query().Get("state")
}

func TestWeComCallbackRejectsUnboundAccount(t *testing.T) {
	useFakeWecomClient(t, "zhangsan-wecom-id")
	store := &fakeStore{
		provider: enabledWecomProvider(t, directWecomConfig()),
		user:     domain.User{ID: "u-1", Username: "zhangsan"},
	}
	service := wecomTestService(t, store)
	target, err := service.WeComLoginURL(context.Background(), "/", "", "")
	if err != nil {
		t.Fatalf("login url failed: %v", err)
	}
	parsed, _ := url.Parse(target)
	_, err = service.WeComCallback(context.Background(), "good-code", parsed.Query().Get("state"))
	if !errors.Is(err, ErrWecomNotBound) {
		t.Fatalf("unbound account should reject with ErrWecomNotBound, got %v", err)
	}
	var loginErr WecomLoginError
	if !errors.As(err, &loginErr) || loginErr.Userid != "zhangsan-wecom-id" {
		t.Fatalf("login error should carry wecom userid for audit, got %v", err)
	}
}

func TestWeComCallbackRejectsInvalidState(t *testing.T) {
	useFakeWecomClient(t, "zhangsan-wecom-id")
	store := &fakeStore{
		provider: enabledWecomProvider(t, directWecomConfig()),
		user:     domain.User{ID: "u-1", Username: "zhangsan"},
		bindings: map[string]string{"zhangsan-wecom-id": "u-1"},
	}
	service := wecomTestService(t, store)
	if err := store.CreateAuthState(context.Background(), domain.AuthState{
		State: "expired", Provider: "wecom", Purpose: AuthPurposeLogin, ExpiresAt: time.Now().Add(-time.Minute),
	}); err != nil {
		t.Fatalf("seed state failed: %v", err)
	}
	if _, err := service.WeComCallback(context.Background(), "good-code", "expired"); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("expired state should reject, got %v", err)
	}
	if _, err := service.WeComCallback(context.Background(), "good-code", "missing"); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("unknown state should reject, got %v", err)
	}
}

func TestWeComBindConflict(t *testing.T) {
	useFakeWecomClient(t, "zhangsan-wecom-id")
	store := &fakeStore{
		provider: enabledWecomProvider(t, directWecomConfig()),
		user:     domain.User{ID: "u-1", Username: "zhangsan"},
		userByID: domain.User{ID: "u-1", Username: "zhangsan"},
		bindings: map[string]string{"zhangsan-wecom-id": "u-2"},
	}
	service := wecomTestService(t, store)
	target, err := service.WeComBindURL(context.Background(), "u-1", "", "")
	if err != nil {
		t.Fatalf("bind url failed: %v", err)
	}
	parsed, _ := url.Parse(target)
	_, err = service.WeComCallback(context.Background(), "good-code", parsed.Query().Get("state"))
	if !errors.Is(err, ErrWecomAlreadyBound) {
		t.Fatalf("conflicting bind should reject with ErrWecomAlreadyBound, got %v", err)
	}
}

func TestWeComUnbind(t *testing.T) {
	store := &fakeStore{bindings: map[string]string{"zhangsan-wecom-id": "u-1"}}
	service := wecomTestService(t, store)
	userid, err := service.WeComUnbind(context.Background(), "u-1")
	if err != nil || userid != "zhangsan-wecom-id" {
		t.Fatalf("unbind failed: %v %q", err, userid)
	}
	if len(store.bindings) != 0 {
		t.Fatalf("binding should be removed, got %v", store.bindings)
	}
	// 未绑定时解绑返回空账号，不报错
	userid, err = service.WeComUnbind(context.Background(), "u-1")
	if err != nil || userid != "" {
		t.Fatalf("unbind without binding should be noop, got %v %q", err, userid)
	}
}

func TestWeComSSOCallbackIssuesSession(t *testing.T) {
	var received map[string]any
	server := newStubAuthCenter(t, &received, func(w http.ResponseWriter) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"userid":"zhangsan","name":"张三"}`))
	})
	config := []byte(`{"authMode":"sso","ssoBaseUrl":"` + server.url + `","ssoAppID":"kvm","ssoAppSecret":"secret"}`)
	store := &fakeStore{
		provider: enabledWecomProvider(t, string(config)),
		user:     domain.User{ID: "u-2", Username: "zhangsan"},
		bindings: map[string]string{"zhangsan": "u-2"},
	}
	service := wecomTestService(t, store)

	result, err := service.WeComSSOCallback(context.Background(), "ticket-1", "/vms")
	if err != nil {
		t.Fatalf("callback failed: %v", err)
	}
	if result.Kind != AuthPurposeLogin || result.Redirect != "/vms" || result.Session.User.Username != "zhangsan" {
		t.Fatalf("login result mismatch: %+v", result)
	}
	if received["app"] != "kvm" || received["ticket"] != "ticket-1" {
		t.Fatalf("verify payload mismatch: %+v", received)
	}
	sign, _ := received["sign"].(string)
	ts, _ := received["ts"].(float64)
	want := signWeComCenterTicket("secret", "kvm", "ticket-1", int64(ts))
	if sign != want {
		t.Fatalf("verify sign mismatch: got %s want %s", sign, want)
	}
}

func TestWeComSSOBindFlow(t *testing.T) {
	var received map[string]any
	server := newStubAuthCenter(t, &received, func(w http.ResponseWriter) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"userid":"zhangsan"}`))
	})
	config := []byte(`{"authMode":"sso","ssoBaseUrl":"` + server.url + `","ssoAppID":"kvm","ssoAppSecret":"secret"}`)
	store := &fakeStore{
		provider: enabledWecomProvider(t, string(config)),
		user:     domain.User{ID: "u-2", Username: "zhangsan"},
		userByID: domain.User{ID: "u-2", Username: "zhangsan"},
	}
	service := wecomTestService(t, store)

	target, err := service.WeComBindURL(context.Background(), "u-2", "", "")
	if err != nil {
		t.Fatalf("bind url failed: %v", err)
	}
	// 绑定票据应嵌入认证中心 redirect 参数
	parsed, err := url.Parse(target)
	if err != nil {
		t.Fatalf("parse target failed: %v", err)
	}
	redirect := parsed.Query().Get("redirect")
	if !strings.HasPrefix(redirect, WecomFrontendCallbackPath+"?bind=") {
		t.Fatalf("bind redirect should embed bind ticket: %q", redirect)
	}

	result, err := service.WeComSSOCallback(context.Background(), "ticket-1", redirect)
	if err != nil {
		t.Fatalf("bind callback failed: %v", err)
	}
	if result.Kind != AuthPurposeBind || result.Userid != "zhangsan" || result.Username != "zhangsan" {
		t.Fatalf("bind result mismatch: %+v", result)
	}
	if store.bindings["zhangsan"] != "u-2" {
		t.Fatalf("binding should be stored, got %v", store.bindings)
	}
	// 绑定票据一次性：重复使用必须失败
	if _, err := service.WeComSSOCallback(context.Background(), "ticket-2", redirect); err == nil {
		t.Fatal("replayed bind ticket should reject")
	}
}

func TestWeComSSOCallbackRejectsCenterFailure(t *testing.T) {
	server := newStubAuthCenter(t, nil, func(w http.ResponseWriter) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"invalid_ticket"}`))
	})
	config := []byte(`{"authMode":"sso","ssoBaseUrl":"` + server.url + `","ssoAppID":"kvm","ssoAppSecret":"secret"}`)
	store := &fakeStore{provider: enabledWecomProvider(t, string(config))}
	service := wecomTestService(t, store)
	if _, err := service.WeComSSOCallback(context.Background(), "bad-ticket", "/"); err == nil {
		t.Fatal("center 401 should reject")
	}
}

func TestWeComUserMessageTranslations(t *testing.T) {
	cases := map[string]string{
		"wecom getuserinfo errcode=40029 errmsg=invalid code":                "企业微信授权码无效或已使用，请重新扫码",
		"wecom getuserinfo errcode=60020 errmsg=not allow":                   "企业微信应用 IP 不在可信域名内，请检查应用配置",
		"wecom auth center verify failed with http 401 error=invalid_ticket": "统一认证中心票据无效或已使用，请重新发起登录",
		"wecom auth center verify failed with http 401 error=invalid_sign":   "统一认证中心签名校验失败，请检查应用密钥配置",
	}
	for raw, want := range cases {
		if got := WeComUserMessage(errors.New(raw)); got != want {
			t.Fatalf("message mismatch for %q: got %s want %s", raw, got, want)
		}
	}
	if got := WeComUserMessage(ErrWecomNotBound); !strings.Contains(got, "尚未绑定系统用户") {
		t.Fatalf("unbound message mismatch: %s", got)
	}
	if got := WeComUserMessage(ErrWecomAlreadyBound); !strings.Contains(got, "已绑定其他用户") {
		t.Fatalf("conflict message mismatch: %s", got)
	}
}

// newStubAuthCenter 启动本地 HTTP 服务模拟统一认证中心 /api/verify。
func newStubAuthCenter(t *testing.T, received *map[string]any, respond func(w http.ResponseWriter)) *stubAuthCenter {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if received != nil {
			var payload map[string]any
			_ = json.Unmarshal(body, &payload)
			*received = payload
		}
		respond(w)
	}))
	t.Cleanup(server.Close)
	return &stubAuthCenter{url: server.URL}
}

type stubAuthCenter struct {
	url string
}
