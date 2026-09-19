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

func TestDecodeWeComConfigNormalizesFields(t *testing.T) {
	config := []byte(`{"corpId":" ww123 ","agentId":1000002,"secret":" s ","externalUrl":"https://kvm.example.com/","mode":"other"}`)
	cfg, err := decodeWeComConfig(config)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if cfg.CorpID != "ww123" || cfg.Secret != "s" {
		t.Fatalf("fields not trimmed: %+v", cfg)
	}
	if cfg.ExternalURL != "https://kvm.example.com" {
		t.Fatalf("external url trailing slash should be trimmed, got %q", cfg.ExternalURL)
	}
	if cfg.Mode != wecomModeQRCode {
		t.Fatalf("unknown mode should fall back to qrcode, got %q", cfg.Mode)
	}
}

func TestDecodeWeComConfigAllowsEmptyExternalURL(t *testing.T) {
	// 外部访问地址改为可选：留空时按用户当前访问地址推断回调前缀
	config := []byte(`{"corpId":"ww123","agentId":1000002,"secret":"s"}`)
	if _, err := decodeWeComConfig(config); err != nil {
		t.Fatalf("empty external url should be allowed: %v", err)
	}
}

func TestDecodeWeComConfigRequiresFieldsUnlessMock(t *testing.T) {
	empty := []byte(`{}`)
	if _, err := decodeWeComConfig(empty); err == nil {
		t.Fatal("expected error when corp id and secret are missing")
	}
	mock := []byte(`{"mock":true}`)
	if _, err := decodeWeComConfig(mock); err != nil {
		t.Fatalf("mock config should not require corp id and secret: %v", err)
	}
}

func TestWeComConfigAuthorizeURL(t *testing.T) {
	cfg := WeComConfig{CorpID: "ww123", AgentID: 1000002, ExternalURL: "https://kvm.example.com"}
	qrcodeURL := cfg.AuthorizeURL("state-1", "http://inferred.example.com")
	for _, want := range []string{
		"https://login.work.weixin.qq.com/wwlogin/sso/login?",
		"appid=ww123",
		"agentid=1000002",
		"redirect_uri=https%3A%2F%2Fkvm.example.com%2Fapi%2Fauth%2Fwecom%2Fcallback",
		"state=state-1",
		"login_type=CorpApp",
	} {
		if !strings.Contains(qrcodeURL, want) {
			t.Fatalf("qrcode url missing %s: %s", want, qrcodeURL)
		}
	}
	// 未配置外部访问地址时按请求地址推断
	cfg.ExternalURL = ""
	inferredURL := cfg.AuthorizeURL("state-2", "https://kvm.local")
	if !strings.Contains(inferredURL, "redirect_uri=https%3A%2F%2Fkvm.local%2Fapi%2Fauth%2Fwecom%2Fcallback") {
		t.Fatalf("inferred redirect_uri mismatch: %s", inferredURL)
	}
	cfg.Mode = wecomModeInside
	insideURL := cfg.AuthorizeURL("state-3", "https://kvm.local")
	for _, want := range []string{
		"https://open.weixin.qq.com/connect/oauth2/authorize?",
		"response_type=code",
		"scope=snsapi_base",
		"#wechat_redirect",
	} {
		if !strings.Contains(insideURL, want) {
			t.Fatalf("inside url missing %s: %s", want, insideURL)
		}
	}
	if strings.Contains(insideURL, "login_type") {
		t.Fatalf("inside url should not carry login_type: %s", insideURL)
	}
}

func TestWeComCenterConfigDefaultsAndLoginURL(t *testing.T) {
	cfg, err := decodeWeComCenterConfig([]byte(`{"baseUrl":"https://auth.example.com/","app":"kvm","appSecret":"s"}`))
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if cfg.VerifyTSSkew != 60 {
		t.Fatalf("default ts skew should be 60, got %d", cfg.VerifyTSSkew)
	}
	if cfg.BaseURL != "https://auth.example.com" {
		t.Fatalf("base url trailing slash should be trimmed, got %q", cfg.BaseURL)
	}
	loginURL := cfg.LoginURL("/vms")
	if loginURL != "https://auth.example.com/login?app=kvm&redirect=%2Fvms" {
		t.Fatalf("unexpected login url: %s", loginURL)
	}
}

func TestVerifyWeComCenterTicketRejectsEmptyTicket(t *testing.T) {
	cfg := WeComCenterConfig{BaseURL: "https://auth.example.com", App: "kvm", AppSecret: "s"}
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
	return domain.AuthProvider{ID: "wecom", Type: "wecom", Name: "企业微信·直连", Enabled: true, Config: []byte(config)}
}

func TestWeComAuthorizeMockStoresState(t *testing.T) {
	store := &fakeStore{provider: enabledWecomProvider(t, `{"externalUrl":"https://kvm.example.com","mock":true}`)}
	service := wecomTestService(t, store)

	target, err := service.WeComAuthorize(context.Background(), "/vms", "10.0.0.1", "")
	if err != nil {
		t.Fatalf("authorize failed: %v", err)
	}
	if !strings.HasPrefix(target, "/api/auth/wecom/callback?code=mock-") {
		t.Fatalf("mock authorize should jump to local callback, got %s", target)
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
		if !strings.Contains(target, "state="+rec.State) {
			t.Fatalf("target should carry stored state: %s", target)
		}
	}
}

func TestWeComStateTTLFollowsBaseConfig(t *testing.T) {
	store := &fakeStore{
		provider:   enabledWecomProvider(t, `{"externalUrl":"https://kvm.example.com","mock":true}`),
		baseConfig: domain.SystemBaseConfig{WecomStateTTLMinutes: 30},
	}
	service := wecomTestService(t, store)
	if _, err := service.WeComAuthorize(context.Background(), "/", "", ""); err != nil {
		t.Fatalf("authorize failed: %v", err)
	}
	for _, rec := range store.states {
		if delta := time.Until(rec.ExpiresAt); delta < 29*time.Minute || delta > 31*time.Minute {
			t.Fatalf("state ttl should follow base config 30m, got %v", delta)
		}
	}
}

func TestWeComAuthorizeRequiresEnabledProvider(t *testing.T) {
	store := &fakeStore{}
	service := wecomTestService(t, store)
	if _, err := service.WeComAuthorize(context.Background(), "/", "", ""); !errors.Is(err, ErrAuthProviderDisabled) {
		t.Fatalf("disabled provider should reject, got %v", err)
	}
}

func runMockWecomCallback(t *testing.T, store *fakeStore, redirect, remoteIP string) (WecomResult, string, string, error) {
	t.Helper()
	service := wecomTestService(t, store)
	target, err := service.WeComAuthorize(context.Background(), redirect, remoteIP, "")
	if err != nil {
		t.Fatalf("authorize failed: %v", err)
	}
	parsed, err := url.Parse(target)
	if err != nil {
		t.Fatalf("parse target failed: %v", err)
	}
	state := parsed.Query().Get("state")
	code := parsed.Query().Get("code")
	result, err := service.WeComCallback(context.Background(), code, state, "")
	return result, state, code, err
}

func TestWeComCallbackConsumesStateOnce(t *testing.T) {
	store := &fakeStore{
		provider: enabledWecomProvider(t, `{"externalUrl":"https://kvm.example.com","mock":true}`),
		user:     domain.User{ID: "u-1", Username: "zhangsan", Disabled: false},
		bindings: map[string]string{"mockuser": "u-1"},
	}

	result, state, code, err := runMockWecomCallback(t, store, "/vms", "")
	if err != nil {
		t.Fatalf("callback failed: %v", err)
	}
	if result.Kind != AuthPurposeLogin {
		t.Fatalf("expected login result, got %+v", result)
	}
	if result.Redirect != "/vms" {
		t.Fatalf("redirect should come from state record, got %q", result.Redirect)
	}
	if result.Session.Token == "" || result.Session.User.Username != "zhangsan" {
		t.Fatalf("session should be issued for bound user: %+v", result.Session)
	}
	if !result.Session.WecomBound {
		t.Fatalf("login session should carry wecomBound=true: %+v", result.Session)
	}
	// state 取出即删：同一 state 第二次消费必须失败
	service := wecomTestService(t, store)
	if _, err := service.WeComCallback(context.Background(), code, state, ""); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("replayed state should reject, got %v", err)
	}
}

func TestWeComCallbackRejectsUnboundAccount(t *testing.T) {
	store := &fakeStore{provider: enabledWecomProvider(t, `{"externalUrl":"https://kvm.example.com","mock":true}`)}
	_, _, _, err := runMockWecomCallback(t, store, "/", "")
	if !errors.Is(err, ErrWecomNotBound) {
		t.Fatalf("unbound account should reject with ErrWecomNotBound, got %v", err)
	}
}

func TestWeComCallbackRejectsForeignState(t *testing.T) {
	store := &fakeStore{
		provider: enabledWecomProvider(t, `{"externalUrl":"https://kvm.example.com","mock":true}`),
		user:     domain.User{ID: "u-1", Username: "zhangsan"},
		bindings: map[string]string{"mockuser": "u-1"},
	}
	service := wecomTestService(t, store)
	if err := store.CreateAuthState(context.Background(), domain.AuthState{
		State: "abc", Provider: "wecom_center", Purpose: AuthPurposeLogin, ExpiresAt: time.Now().Add(time.Minute),
	}); err != nil {
		t.Fatalf("seed state failed: %v", err)
	}
	if _, err := service.WeComCallback(context.Background(), "mock-1", "abc", ""); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("state from another provider should reject, got %v", err)
	}
	if err := store.CreateAuthState(context.Background(), domain.AuthState{
		State: "expired", Provider: "wecom", Purpose: AuthPurposeLogin, ExpiresAt: time.Now().Add(-time.Minute),
	}); err != nil {
		t.Fatalf("seed state failed: %v", err)
	}
	if _, err := service.WeComCallback(context.Background(), "mock-1", "expired", ""); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("expired state should reject, got %v", err)
	}
}

func TestWeComBindFlow(t *testing.T) {
	store := &fakeStore{
		provider: enabledWecomProvider(t, `{"externalUrl":"https://kvm.example.com","mock":true}`),
		user:     domain.User{ID: "u-1", Username: "zhangsan", Disabled: false},
		userByID: domain.User{ID: "u-1", Username: "zhangsan"},
	}
	service := wecomTestService(t, store)

	target, err := service.WeComBindAuthorize(context.Background(), "u-1", "10.0.0.1", "")
	if err != nil {
		t.Fatalf("bind authorize failed: %v", err)
	}
	parsed, err := url.Parse(target)
	if err != nil {
		t.Fatalf("parse target failed: %v", err)
	}
	state := parsed.Query().Get("state")
	for _, rec := range store.states {
		if rec.Purpose != AuthPurposeBind || rec.UserID != "u-1" {
			t.Fatalf("bind state mismatch: %+v", rec)
		}
	}
	result, err := service.WeComCallback(context.Background(), parsed.Query().Get("code"), state, "")
	if err != nil {
		t.Fatalf("bind callback failed: %v", err)
	}
	if result.Kind != AuthPurposeBind || result.Userid != "mockuser" || result.Username != "zhangsan" {
		t.Fatalf("bind result mismatch: %+v", result)
	}
	if owner := store.bindings["mockuser"]; owner != "u-1" {
		t.Fatalf("binding should be stored, got %v", store.bindings)
	}
	// 绑定成功后即可登录
	loginResult, _, _, err := runMockWecomCallback(t, store, "/", "")
	if err != nil {
		t.Fatalf("login after bind failed: %v", err)
	}
	if loginResult.Session.Token == "" {
		t.Fatalf("session should be issued after binding")
	}
}

func TestWeComBindConflict(t *testing.T) {
	store := &fakeStore{
		provider: enabledWecomProvider(t, `{"externalUrl":"https://kvm.example.com","mock":true}`),
		user:     domain.User{ID: "u-1", Username: "zhangsan"},
		userByID: domain.User{ID: "u-1", Username: "zhangsan"},
		bindings: map[string]string{"mockuser": "u-2"},
	}
	service := wecomTestService(t, store)
	if _, err := service.WeComBindAuthorize(context.Background(), "u-1", "", ""); err != nil {
		t.Fatalf("bind authorize failed: %v", err)
	}
	var state string
	for key := range store.states {
		state = key
	}
	if _, err := service.WeComCallback(context.Background(), "mock-1", state, ""); !errors.Is(err, ErrWecomAlreadyBound) {
		t.Fatalf("conflicting bind should reject with ErrWecomAlreadyBound, got %v", err)
	}
}

func TestWeComUnbind(t *testing.T) {
	store := &fakeStore{
		bindings: map[string]string{"mockuser": "u-1"},
	}
	service := wecomTestService(t, store)
	userid, err := service.WeComUnbind(context.Background(), "u-1")
	if err != nil || userid != "mockuser" {
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

func TestWeComCenterCallbackIssuesSession(t *testing.T) {
	var received map[string]any
	server := newStubAuthCenter(t, &received, func(w http.ResponseWriter) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"userid":"zhangsan","name":"张三"}`))
	})
	config := []byte(`{"baseUrl":"` + server.url + `","app":"kvm","appSecret":"secret"}`)
	store := &fakeStore{
		provider: domain.AuthProvider{ID: "wecom_center", Type: "wecom_center", Enabled: true, Config: []byte(config)},
		user:     domain.User{ID: "u-2", Username: "zhangsan"},
		bindings: map[string]string{"zhangsan": "u-2"},
	}
	service := wecomTestService(t, store)

	result, err := service.WeComCenterCallback(context.Background(), "ticket-1", "/vms")
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

func TestWeComCenterBindFlow(t *testing.T) {
	var received map[string]any
	server := newStubAuthCenter(t, &received, func(w http.ResponseWriter) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"userid":"zhangsan"}`))
	})
	config := []byte(`{"baseUrl":"` + server.url + `","app":"kvm","appSecret":"secret"}`)
	store := &fakeStore{
		provider: domain.AuthProvider{ID: "wecom_center", Type: "wecom_center", Enabled: true, Config: []byte(config)},
		user:     domain.User{ID: "u-2", Username: "zhangsan"},
		userByID: domain.User{ID: "u-2", Username: "zhangsan"},
	}
	service := wecomTestService(t, store)

	target, err := service.WeComCenterBindAuthorize(context.Background(), "u-2")
	if err != nil {
		t.Fatalf("bind authorize failed: %v", err)
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
	bindTicket := parseBindTicket(t, redirect)

	result, err := service.WeComCenterCallback(context.Background(), "ticket-1", redirect)
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
	if _, err := service.WeComCenterCallback(context.Background(), "ticket-2", redirect); err == nil {
		t.Fatal("replayed bind ticket should reject")
	}
	if bindTicket == "" {
		t.Fatal("bind ticket should be present")
	}
}

func parseBindTicket(t *testing.T, redirect string) string {
	t.Helper()
	parsed, err := url.Parse(redirect)
	if err != nil {
		t.Fatalf("parse redirect failed: %v", err)
	}
	return parsed.Query().Get("bind")
}

func TestWeComCenterCallbackRejectsCenterFailure(t *testing.T) {
	server := newStubAuthCenter(t, nil, func(w http.ResponseWriter) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"invalid_ticket"}`))
	})
	config := []byte(`{"baseUrl":"` + server.url + `","app":"kvm","appSecret":"secret"}`)
	store := &fakeStore{
		provider: domain.AuthProvider{ID: "wecom_center", Type: "wecom_center", Enabled: true, Config: []byte(config)},
	}
	service := wecomTestService(t, store)
	if _, err := service.WeComCenterCallback(context.Background(), "bad-ticket", "/"); err == nil {
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
