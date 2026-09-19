package auth

import (
	"context"
	"net/url"
	"strings"
	"time"

	"kvm-manager/backend/internal/domain"
)

// AuthStateTTL OAuth state 有效期，超时未完成的扫码流程自动作废。
const AuthStateTTL = 5 * time.Minute

const (
	authProviderWecom       = "wecom"
	authProviderWecomCenter = "wecom_center"
)

// WeComAuthorize 发起企业微信直连登录：生成一次性 state 并返回授权跳转地址。
// mock 模式下直接返回本平台回调地址，模拟扫码成功，便于本地演练完整流程。
func (s *Service) WeComAuthorize(ctx context.Context, redirect, remoteIP string) (string, error) {
	provider, err := s.enabledAuthProvider(ctx, authProviderWecom)
	if err != nil {
		return "", err
	}
	cfg, err := decodeWeComConfig(provider.Config)
	if err != nil {
		return "", err
	}
	state, err := generateToken(16)
	if err != nil {
		return "", err
	}
	if err := s.store.CreateAuthState(ctx, domain.AuthState{
		State:     state,
		Provider:  authProviderWecom,
		Redirect:  redirect,
		RemoteIP:  remoteIP,
		ExpiresAt: s.now().Add(AuthStateTTL),
	}); err != nil {
		return "", err
	}
	if cfg.Mock {
		code, err := mockWeComCode()
		if err != nil {
			return "", err
		}
		return "/api/auth/wecom/callback?code=" + url.QueryEscape(code) + "&state=" + url.QueryEscape(state), nil
	}
	return cfg.AuthorizeURL(state), nil
}

// WeComCallback 企业微信直连回调：消费 state、code 换身份，映射本地账号后签发会话。
// 返回值为登录前用户原本要前往的站内路径。
func (s *Service) WeComCallback(ctx context.Context, code, state string) (domain.Session, string, error) {
	rec, err := s.takeAuthState(ctx, state, authProviderWecom)
	if err != nil {
		return domain.Session{}, "", err
	}
	provider, err := s.enabledAuthProvider(ctx, authProviderWecom)
	if err != nil {
		return domain.Session{}, "", err
	}
	cfg, err := decodeWeComConfig(provider.Config)
	if err != nil {
		return domain.Session{}, "", err
	}
	userInfo, err := getWeComClient(cfg).GetUserInfo(ctx, code)
	if err != nil {
		return domain.Session{}, "", err
	}
	session, err := s.provisionAndLogin(ctx, userInfo.Userid)
	if err != nil {
		return domain.Session{}, "", err
	}
	return session, rec.Redirect, nil
}

// WeComCenterAuthorize 发起统一认证中心登录：跳转认证中心扫码，票据回调由后端处理。
func (s *Service) WeComCenterAuthorize(ctx context.Context, redirect string) (string, error) {
	provider, err := s.enabledAuthProvider(ctx, authProviderWecomCenter)
	if err != nil {
		return "", err
	}
	cfg, err := decodeWeComCenterConfig(provider.Config)
	if err != nil {
		return "", err
	}
	return cfg.LoginURL(redirect), nil
}

// WeComCenterCallback 统一认证中心回调：后端持 app_secret 发起 verify 换取身份后签发会话。
func (s *Service) WeComCenterCallback(ctx context.Context, ticket string) (domain.Session, error) {
	provider, err := s.enabledAuthProvider(ctx, authProviderWecomCenter)
	if err != nil {
		return domain.Session{}, err
	}
	cfg, err := decodeWeComCenterConfig(provider.Config)
	if err != nil {
		return domain.Session{}, err
	}
	userInfo, err := verifyWeComCenterTicket(ctx, cfg, ticket)
	if err != nil {
		return domain.Session{}, err
	}
	return s.provisionAndLogin(ctx, userInfo.Userid)
}

// provisionAndLogin 企业微信身份映射本地账号：与 AD/LDAP 一致，不自动建户，需管理员预开通。
// 未开通时携带企微账号返回，便于审计排查；errors.Is 仍命中 ErrUserNotProvisioned。
func (s *Service) provisionAndLogin(ctx context.Context, userid string) (domain.Session, error) {
	userid = strings.TrimSpace(userid)
	if userid == "" {
		return domain.Session{}, ErrUserNotProvisioned
	}
	stored, _, err := s.store.FindUserByUsername(ctx, userid)
	if err != nil || stored.Disabled {
		return domain.Session{}, NotProvisionedError{Userid: userid}
	}
	return s.issueSession(ctx, stored)
}

// NotProvisionedError 标识企业微信账号未在平台开通，并携带企微账号供审计记录。
type NotProvisionedError struct {
	Userid string
}

func (e NotProvisionedError) Error() string { return ErrUserNotProvisioned.Error() }

func (e NotProvisionedError) Unwrap() error { return ErrUserNotProvisioned }

func (s *Service) enabledAuthProvider(ctx context.Context, id string) (domain.AuthProvider, error) {
	provider, err := s.store.GetAuthProvider(ctx, id)
	if err != nil || !provider.Enabled {
		return domain.AuthProvider{}, ErrAuthProviderDisabled
	}
	return provider, nil
}

// takeAuthState 取出即删并校验归属与有效期，防止重放与跨提供方复用。
func (s *Service) takeAuthState(ctx context.Context, state, provider string) (domain.AuthState, error) {
	if strings.TrimSpace(state) == "" {
		return domain.AuthState{}, ErrInvalidState
	}
	rec, err := s.store.TakeAuthState(ctx, state)
	if err != nil || rec.Provider != provider || !rec.ExpiresAt.After(s.now()) {
		return domain.AuthState{}, ErrInvalidState
	}
	return rec, nil
}
