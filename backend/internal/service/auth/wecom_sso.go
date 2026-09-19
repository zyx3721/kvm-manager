package auth

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"time"

	"kvm-manager/backend/internal/domain"
)

// OAuth state 有效期兜底值；实际有效期按基础配置「企业微信扫码有效期」读取。
const AuthStateTTL = 5 * time.Minute

const authProviderWecom = "wecom"

// state 用途：登录与绑定严格区分，防止跨用途混用。
const (
	AuthPurposeLogin = "login"
	AuthPurposeBind  = "bind"
)

// 前端回调页路径，登录与绑定的结果都 302 到这里。
const WecomFrontendCallbackPath = "/auth/callback"

// WecomResult OAuth 回调处理结果：登录返回会话，绑定返回企微账号与绑定用户。
type WecomResult struct {
	Kind       string
	Session    domain.Session
	Redirect   string
	Userid     string
	Username   string
	BindUserID string
}

// WeComLoginURL 获取企业微信扫码登录地址：直连返回企微授权页，统一认证中心返回认证中心登录页。
func (s *Service) WeComLoginURL(ctx context.Context, redirect, remoteIP, requestBase string) (string, error) {
	_, cfg, err := s.enabledWecomProvider(ctx)
	if err != nil {
		return "", err
	}
	if cfg.AuthMode == WecomModeSSO {
		return cfg.WecomSSOLoginURL(redirect), nil
	}
	state, err := s.newWecomState(ctx, AuthPurposeLogin, "", redirect, remoteIP)
	if err != nil {
		return "", err
	}
	return cfg.AuthorizeURL(state, requestBase), nil
}

// WeComBindURL 获取当前用户的企微绑定地址：直连签发 bind state 扫码，
// 统一认证中心签发一次性绑定票据嵌入 redirect，回调时凭票据定位发起绑定的用户。
func (s *Service) WeComBindURL(ctx context.Context, userID, remoteIP, requestBase string) (string, error) {
	_, cfg, err := s.enabledWecomProvider(ctx)
	if err != nil {
		return "", err
	}
	if cfg.AuthMode == WecomModeSSO {
		bindTicket, err := s.newWecomState(ctx, AuthPurposeBind, userID, "", "")
		if err != nil {
			return "", err
		}
		redirect := WecomFrontendCallbackPath + "?bind=" + url.QueryEscape(bindTicket)
		return cfg.WecomSSOLoginURL(redirect), nil
	}
	state, err := s.newWecomState(ctx, AuthPurposeBind, userID, "", remoteIP)
	if err != nil {
		return "", err
	}
	return cfg.AuthorizeURL(state, requestBase), nil
}

// WeComCallback 企业微信直连回调：消费 state 后按用途分派登录或绑定。
func (s *Service) WeComCallback(ctx context.Context, code, state string) (WecomResult, error) {
	rec, err := s.takeAuthState(ctx, state, authProviderWecom)
	if err != nil {
		return WecomResult{}, err
	}
	_, cfg, err := s.enabledWecomProvider(ctx)
	if err != nil {
		return WecomResult{}, err
	}
	userInfo, err := getWeComClient(cfg).GetUserInfo(ctx, code)
	if err != nil {
		return WecomResult{}, err
	}
	return s.completeWecomFlow(ctx, rec, userInfo.Userid)
}

// WeComSSOCallback 统一认证中心回调：redirect 携带绑定票据时走绑定，否则登录。
func (s *Service) WeComSSOCallback(ctx context.Context, ticket, redirect string) (WecomResult, error) {
	_, cfg, err := s.enabledWecomProvider(ctx)
	if err != nil {
		return WecomResult{}, err
	}
	userInfo, err := verifyWeComCenterTicket(ctx, cfg, ticket)
	if err != nil {
		return WecomResult{}, err
	}
	if bindTicket := wecomBindTicketFromRedirect(redirect); bindTicket != "" {
		rec, err := s.takeAuthState(ctx, bindTicket, authProviderWecom)
		if err != nil {
			return WecomResult{}, err
		}
		return s.completeWecomFlow(ctx, rec, userInfo.Userid)
	}
	rec := domain.AuthState{Provider: authProviderWecom, Purpose: AuthPurposeLogin, Redirect: redirect}
	return s.completeWecomFlow(ctx, rec, userInfo.Userid)
}

// WeComUnbind 解除当前用户的企业微信绑定，返回被解绑的企微账号（未绑定为空）。
func (s *Service) WeComUnbind(ctx context.Context, userID string) (string, error) {
	return s.store.UnbindWecomAccount(ctx, userID)
}

// WecomBoundFor 查询用户是否已绑定企业微信账号。
func (s *Service) WecomBoundFor(ctx context.Context, userID string) (bool, error) {
	return s.store.UserHasWecomBinding(ctx, userID)
}

// completeWecomFlow 登录与绑定的统一收口：直连与认证中心两种模式的审计、文案、冲突语义保持一致。
func (s *Service) completeWecomFlow(ctx context.Context, rec domain.AuthState, userid string) (WecomResult, error) {
	if rec.Purpose == AuthPurposeBind {
		if err := s.store.BindWecomAccount(ctx, rec.UserID, userid); err != nil {
			// 绑定冲突时携带发起绑定者，供审计记录操作用户
			return WecomResult{}, WecomLoginError{Userid: userid, BindUserID: rec.UserID, Err: err}
		}
		// 回调请求无会话头，绑定审计所需的用户身份在此一并带出
		username := ""
		if bound, err := s.store.FindUserByID(ctx, rec.UserID); err == nil {
			username = bound.Username
		}
		return WecomResult{Kind: AuthPurposeBind, Userid: userid, Username: username, BindUserID: rec.UserID}, nil
	}
	session, err := s.loginByWecomAccount(ctx, userid)
	if err != nil {
		var loginErr WecomLoginError
		if errors.As(err, &loginErr) {
			return WecomResult{}, loginErr
		}
		return WecomResult{}, WecomLoginError{Userid: userid, Err: err}
	}
	return WecomResult{Kind: AuthPurposeLogin, Session: session, Redirect: rec.Redirect}, nil
}

// WecomLoginError 企业微信登录失败时携带已知身份，供审计记录定位用户。
// Userid 为企微账号；BindUserID/Username 为已确定的系统用户（绑定冲突、账号被禁用场景），未知时为空。
type WecomLoginError struct {
	Userid     string
	BindUserID string
	Username   string
	Err        error
}

func (e WecomLoginError) Error() string { return e.Err.Error() }

func (e WecomLoginError) Unwrap() error { return e.Err }

// loginByWecomAccount 仅按绑定关系定位用户：未绑定拒绝登录，不自动建户、不按用户名兜底。
func (s *Service) loginByWecomAccount(ctx context.Context, userid string) (domain.Session, error) {
	userid = strings.TrimSpace(userid)
	if userid == "" {
		return domain.Session{}, ErrWecomNotBound
	}
	stored, err := s.store.FindUserByWecomAccount(ctx, userid)
	if err != nil {
		return domain.Session{}, WecomLoginError{Userid: userid, Err: ErrWecomNotBound}
	}
	if stored.Disabled {
		// 绑定关系存在但账号被禁用：系统用户身份已知，供审计记录
		return domain.Session{}, WecomLoginError{Userid: userid, BindUserID: stored.ID, Username: stored.Username, Err: ErrUserNotProvisioned}
	}
	return s.issueSession(ctx, stored)
}

// wecomBindTicketFromRedirect 从认证中心带回的站内路径中提取绑定票据。
func wecomBindTicketFromRedirect(redirect string) string {
	parsed, err := url.Parse(redirect)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(parsed.Query().Get("bind"))
}

func (s *Service) enabledWecomProvider(ctx context.Context) (domain.AuthProvider, WecomProviderConfig, error) {
	provider, err := s.enabledAuthProvider(ctx, authProviderWecom)
	if err != nil {
		return domain.AuthProvider{}, WecomProviderConfig{}, err
	}
	cfg, err := decodeWecomProviderConfig(provider.Config)
	if err != nil {
		return domain.AuthProvider{}, WecomProviderConfig{}, err
	}
	return provider, cfg, nil
}

// newWecomState 签发一次性 state，有效期按基础配置「企业微信扫码有效期」读取（1-60 分钟）。
func (s *Service) newWecomState(ctx context.Context, purpose, userID, redirect, remoteIP string) (string, error) {
	state, err := generateToken(16)
	if err != nil {
		return "", err
	}
	if err := s.store.CreateAuthState(ctx, domain.AuthState{
		State:     state,
		Provider:  authProviderWecom,
		Purpose:   purpose,
		UserID:    userID,
		Redirect:  redirect,
		RemoteIP:  remoteIP,
		ExpiresAt: s.now().Add(s.wecomStateTTL(ctx)),
	}); err != nil {
		return "", err
	}
	return state, nil
}

// wecomStateTTL 读取基础配置的企业微信扫码有效期，配置异常时回退 5 分钟。
func (s *Service) wecomStateTTL(ctx context.Context) time.Duration {
	config, err := s.store.GetSystemBaseConfig(ctx)
	if err != nil || config.WecomStateTTLMinutes < 1 || config.WecomStateTTLMinutes > 60 {
		return AuthStateTTL
	}
	return time.Duration(config.WecomStateTTLMinutes) * time.Minute
}

func (s *Service) enabledAuthProvider(ctx context.Context, id string) (domain.AuthProvider, error) {
	provider, err := s.store.GetAuthProvider(ctx, id)
	if err != nil || !provider.Enabled {
		return domain.AuthProvider{}, ErrAuthProviderDisabled
	}
	return provider, nil
}

// takeAuthState 取出即删并校验归属与有效期，防止重放。
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
