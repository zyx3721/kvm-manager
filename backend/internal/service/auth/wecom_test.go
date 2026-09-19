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

func TestDecodeWeComConfigRequiresFieldsUnlessMock(t *testing.T) {
	empty := []byte(`{"externalUrl":"https://kvm.example.com"}`)
	if _, err := decodeWeComConfig(empty); err == nil {
		t.Fatal("expected error when corp id and secret are missing")
	}
	mock := []byte(`{"externalUrl":"https://kvm.example.com","mock":true}`)
	if _, err := decodeWeComConfig(mock); err != nil {
		t.Fatalf("mock config should not require corp id and secret: %v", err)
	}
}

func TestWeComConfigAuthorizeURL(t *testing.T) {
	cfg := WeComConfig{CorpID: "ww123", AgentID: 1000002, ExternalURL: "https://kvm.example.com"}
	qrcodeURL := cfg.AuthorizeURL("state-1")
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
	cfg.Mode = wecomModeInside
	insideURL := cfg.AuthorizeURL("state-2")
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

	target, err := service.WeComAuthorize(context.Background(), "/vms", "10.0.0.1")
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
		if !strings.Contains(target, "state="+rec.State) {
			t.Fatalf("target should carry stored state: %s", target)
		}
	}
}

func TestWeComAuthorizeRequiresEnabledProvider(t *testing.T) {
	store := &fakeStore{}
	service := wecomTestService(t, store)
	if _, err := service.WeComAuthorize(context.Background(), "/", ""); !errors.Is(err, ErrAuthProviderDisabled) {
		t.Fatalf("disabled provider should reject, got %v", err)
	}
}

func TestWeComCallbackConsumesStateOnce(t *testing.T) {
	store := &fakeStore{
		provider: enabledWecomProvider(t, `{"externalUrl":"https://kvm.example.com","mock":true}`),
		user:     domain.User{ID: "u-1", Username: "mockuser", Disabled: false},
	}
	service := wecomTestService(t, store)

	target, err := service.WeComAuthorize(context.Background(), "/vms", "")
	if err != nil {
		t.Fatalf("authorize failed: %v", err)
	}
	parsed, err := url.Parse(target)
	if err != nil {
		t.Fatalf("parse target failed: %v", err)
	}
	state := parsed.Query().Get("state")
	if state == "" {
		t.Fatalf("target should carry state: %s", target)
	}
	code := parsed.Query().Get("code")

	session, redirect, err := service.WeComCallback(context.Background(), code, state)
	if err != nil {
		t.Fatalf("callback failed: %v", err)
	}
	if redirect != "/vms" {
		t.Fatalf("redirect should come from state record, got %q", redirect)
	}
	if session.Token == "" || session.User.Username != "mockuser" {
		t.Fatalf("session should be issued for provisioned user: %+v", session)
	}
	// state 取出即删：同一 state 第二次消费必须失败
	if _, _, err := service.WeComCallback(context.Background(), code, state); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("replayed state should reject, got %v", err)
	}
}

func TestWeComCallbackRejectsForeignState(t *testing.T) {
	store := &fakeStore{
		provider: enabledWecomProvider(t, `{"externalUrl":"https://kvm.example.com","mock":true}`),
		user:     domain.User{ID: "u-1", Username: "mockuser"},
	}
	service := wecomTestService(t, store)
	if err := store.CreateAuthState(context.Background(), domain.AuthState{
		State: "abc", Provider: "wecom_center", ExpiresAt: time.Now().Add(time.Minute),
	}); err != nil {
		t.Fatalf("seed state failed: %v", err)
	}
	if _, _, err := service.WeComCallback(context.Background(), "mock-1", "abc"); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("state from another provider should reject, got %v", err)
	}
	if err := store.CreateAuthState(context.Background(), domain.AuthState{
		State: "expired", Provider: "wecom", ExpiresAt: time.Now().Add(-time.Minute),
	}); err != nil {
		t.Fatalf("seed state failed: %v", err)
	}
	if _, _, err := service.WeComCallback(context.Background(), "mock-1", "expired"); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("expired state should reject, got %v", err)
	}
}

func TestWeComCallbackRejectsUnprovisionedUser(t *testing.T) {
	store := &fakeStore{provider: enabledWecomProvider(t, `{"externalUrl":"https://kvm.example.com","mock":true}`)}
	service := wecomTestService(t, store)
	if err := store.CreateAuthState(context.Background(), domain.AuthState{
		State: "state-1", Provider: "wecom", ExpiresAt: time.Now().Add(time.Minute),
	}); err != nil {
		t.Fatalf("seed state failed: %v", err)
	}
	_, _, err := service.WeComCallback(context.Background(), "mock-1", "state-1")
	var provisioned NotProvisionedError
	if !errors.As(err, &provisioned) || provisioned.Userid != "mockuser" {
		t.Fatalf("unprovisioned user should return NotProvisionedError with userid, got %v", err)
	}
	if !errors.Is(err, ErrUserNotProvisioned) {
		t.Fatalf("should unwrap to ErrUserNotProvisioned, got %v", err)
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
	}
	service := wecomTestService(t, store)

	session, err := service.WeComCenterCallback(context.Background(), "ticket-1")
	if err != nil {
		t.Fatalf("callback failed: %v", err)
	}
	if session.User.Username != "zhangsan" {
		t.Fatalf("session user mismatch: %+v", session.User)
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
	if _, err := service.WeComCenterCallback(context.Background(), "bad-ticket"); err == nil {
		t.Fatal("center 401 should reject")
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
