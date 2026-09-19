package router

import (
	"errors"
	"net/http"
	"net/url"
	"strings"

	"kvm-manager/backend/internal/repository"
	"kvm-manager/backend/internal/service/auth"
)

// sanitizeLocalPath 仅允许站内相对路径，防止开放重定向。
func sanitizeLocalPath(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || !strings.HasPrefix(value, "/") {
		return "/"
	}
	if strings.HasPrefix(value, "//") || strings.HasPrefix(value, "/\\") {
		return "/"
	}
	return value
}

// wecomRequestBaseURL 按用户当前访问地址推断回调前缀：优先反向代理协议头，叠加请求 Host。
func wecomRequestBaseURL(req *http.Request) string {
	scheme := "http"
	if proto := strings.TrimSpace(req.Header.Get("X-Forwarded-Proto")); proto != "" {
		scheme = proto
	} else if req.TLS != nil {
		scheme = "https"
	}
	return scheme + "://" + req.Host
}

// handleWecomAuthorize 获取企业微信扫码登录地址：直连返回企微授权页，统一认证中心返回认证中心登录页。
func (r *router) handleWecomAuthorize(w http.ResponseWriter, req *http.Request) {
	redirect := sanitizeLocalPath(req.URL.Query().Get("redirect"))
	target, err := r.auth.WeComLoginURL(req.Context(), redirect, repository.ClientIP(req), wecomRequestBaseURL(req))
	if err != nil {
		r.logger.Error("wecom authorize failed", "error", err)
		writeError(w, http.StatusServiceUnavailable, "wecom_authorize_failed", auth.WeComUserMessage(err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"url": target})
}

func (r *router) handleWecomCallback(w http.ResponseWriter, req *http.Request) {
	code := req.URL.Query().Get("code")
	state := req.URL.Query().Get("state")
	result, err := r.auth.WeComCallback(req.Context(), code, state)
	if err != nil {
		r.logger.Warn("wecom login failed", "error", err)
		_ = r.store.WriteAudit(req.Context(), "", "auth.wecom.failed", "auth_provider", "wecom", repository.ClientIP(req), map[string]any{
			"reason": wecomFailureReason(err),
			"userid": wecomFailureUser(err),
		})
		r.redirectAuthResult(w, req, authFailureValues(err))
		return
	}
	r.finishWecomResult(w, req, result)
}

// handleWecomSSOCallback 统一认证中心票据回调：verify 换取身份后按 redirect 中的绑定票据分派登录或绑定。
func (r *router) handleWecomSSOCallback(w http.ResponseWriter, req *http.Request) {
	ticket := req.URL.Query().Get("ticket")
	redirect := req.URL.Query().Get("redirect")
	result, err := r.auth.WeComSSOCallback(req.Context(), ticket, redirect)
	if err != nil {
		r.logger.Warn("wecom sso login failed", "error", err)
		_ = r.store.WriteAudit(req.Context(), "", "auth.wecom.failed", "auth_provider", "wecom", repository.ClientIP(req), map[string]any{
			"reason": wecomFailureReason(err),
			"userid": wecomFailureUser(err),
		})
		r.redirectAuthResult(w, req, authFailureValues(err))
		return
	}
	r.finishWecomResult(w, req, result)
}

// handleWecomBindURL 获取当前用户的企微绑定地址：按认证方式分派直连扫码或统一认证中心跳转。
func (r *router) handleWecomBindURL(w http.ResponseWriter, req *http.Request) {
	userID := currentSession(req).User.ID
	target, err := r.auth.WeComBindURL(req.Context(), userID, repository.ClientIP(req), wecomRequestBaseURL(req))
	if err != nil {
		r.logger.Warn("wecom bind authorize failed", "error", err)
		writeError(w, http.StatusServiceUnavailable, "wecom_bind_url_failed", auth.WeComUserMessage(err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"url": target})
}

// handleWecomUnbind 解除当前用户的企业微信绑定。
func (r *router) handleWecomUnbind(w http.ResponseWriter, req *http.Request) {
	session := currentSession(req)
	userid, err := r.auth.WeComUnbind(req.Context(), session.User.ID)
	if err != nil {
		r.logger.Error("wecom unbind failed", "error", err)
		writeError(w, http.StatusInternalServerError, "wecom_unbind_failed", "解绑企业微信失败，请稍后重试")
		return
	}
	_ = r.store.WriteAudit(req.Context(), session.User.ID, "auth.wecom.unbind", "auth_provider", "wecom", repository.ClientIP(req), map[string]any{
		"username": session.User.Username,
		"userid":   userid,
	})
	display := userid
	if display == "" {
		display = "-"
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "userid": display})
}

// finishWecomResult 回调成功收口：登录写登录审计并回传会话，绑定写绑定审计并通知前端弹窗。
func (r *router) finishWecomResult(w http.ResponseWriter, req *http.Request, result auth.WecomResult) {
	if result.Kind == auth.AuthPurposeBind {
		_ = r.store.WriteAudit(req.Context(), result.Session.User.ID, "auth.wecom.bind", "auth_provider", "wecom", repository.ClientIP(req), map[string]any{
			"username": result.Username,
			"userid":   result.Userid,
		})
		r.redirectAuthResult(w, req, url.Values{"bind": {"success"}})
		return
	}
	session := result.Session
	_ = r.store.WriteAudit(req.Context(), session.User.ID, "auth.login", "user", session.User.ID, repository.ClientIP(req), map[string]any{
		"username": session.User.Username,
		"provider": "wecom",
	})
	r.redirectAuthResult(w, req, url.Values{
		"token":       {session.Token},
		"redirect":    {sanitizeLocalPath(result.Redirect)},
		"wecom_bound": {"true"},
	})
}

// redirectAuthResult 通过 URL fragment 回传结果：token 不进服务端日志与 Referer。
func (r *router) redirectAuthResult(w http.ResponseWriter, req *http.Request, values url.Values) {
	http.Redirect(w, req, auth.WecomFrontendCallbackPath+"#"+values.Encode(), http.StatusFound)
}

func authFailureValues(err error) url.Values {
	switch {
	case errors.Is(err, auth.ErrWecomNotBound):
		return url.Values{"error": {"wecom_not_bound"}, "message": {auth.WeComUserMessage(err)}}
	case errors.Is(err, auth.ErrWecomAlreadyBound):
		return url.Values{"error": {"wecom_already_bound"}, "message": {auth.WeComUserMessage(err)}}
	case errors.Is(err, auth.ErrInvalidState):
		return url.Values{"error": {"invalid_state"}, "message": {auth.WeComUserMessage(err)}}
	case errors.Is(err, auth.ErrUserNotProvisioned):
		return url.Values{"error": {"user_not_provisioned"}, "message": {auth.WeComUserMessage(err)}}
	case errors.Is(err, auth.ErrAuthProviderDisabled):
		return url.Values{"error": {"provider_disabled"}, "message": {auth.WeComUserMessage(err)}}
	}
	return url.Values{"error": {"wecom_failed"}, "message": {auth.WeComUserMessage(err)}}
}

func wecomFailureReason(err error) string {
	switch {
	case errors.Is(err, auth.ErrWecomNotBound):
		return "user_not_bound"
	case errors.Is(err, auth.ErrWecomAlreadyBound):
		return "bind_conflict"
	case errors.Is(err, auth.ErrInvalidState):
		return "state_invalid"
	case errors.Is(err, auth.ErrUserNotProvisioned):
		return "user_disabled"
	case errors.Is(err, auth.ErrAuthProviderDisabled):
		return "provider_disabled"
	}
	return "wecom_api_failed"
}

// wecomFailureUser 提取企微账号便于管理员排查绑定情况，仅限登录失败场景，不含敏感信息。
func wecomFailureUser(err error) string {
	var loginErr auth.WecomLoginError
	if errors.As(err, &loginErr) {
		return loginErr.Userid
	}
	return ""
}
