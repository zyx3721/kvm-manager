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

func (r *router) handleWecomAuthorize(w http.ResponseWriter, req *http.Request) {
	redirect := sanitizeLocalPath(req.URL.Query().Get("redirect"))
	target, err := r.auth.WeComAuthorize(req.Context(), redirect, repository.ClientIP(req))
	if err != nil {
		r.logger.Error("wecom authorize failed", "error", err)
		writeError(w, http.StatusServiceUnavailable, "wecom_authorize_failed", auth.WeComUserMessage(err))
		return
	}
	http.Redirect(w, req, target, http.StatusFound)
}

func (r *router) handleWecomCallback(w http.ResponseWriter, req *http.Request) {
	code := req.URL.Query().Get("code")
	state := req.URL.Query().Get("state")
	session, redirect, err := r.auth.WeComCallback(req.Context(), code, state)
	if err != nil {
		r.logger.Warn("wecom login failed", "error", err)
		_ = r.store.WriteAudit(req.Context(), "", "auth.wecom.failed", "auth_provider", "wecom", repository.ClientIP(req), map[string]any{
			"reason": wecomFailureReason(err),
			"userid": wecomFailureUser(err),
		})
		r.redirectAuthResult(w, req, authFailureValues(err))
		return
	}
	_ = r.store.WriteAudit(req.Context(), session.User.ID, "auth.login", "user", session.User.ID, repository.ClientIP(req), map[string]any{
		"username": session.User.Username,
		"provider": "wecom",
	})
	r.redirectAuthResult(w, req, url.Values{"token": {session.Token}, "redirect": {sanitizeLocalPath(redirect)}})
}

func (r *router) handleWecomCenterAuthorize(w http.ResponseWriter, req *http.Request) {
	redirect := sanitizeLocalPath(req.URL.Query().Get("redirect"))
	target, err := r.auth.WeComCenterAuthorize(req.Context(), redirect)
	if err != nil {
		r.logger.Error("wecom center authorize failed", "error", err)
		writeError(w, http.StatusServiceUnavailable, "wecom_center_authorize_failed", auth.WeComUserMessage(err))
		return
	}
	http.Redirect(w, req, target, http.StatusFound)
}

func (r *router) handleWecomCenterCallback(w http.ResponseWriter, req *http.Request) {
	ticket := req.URL.Query().Get("ticket")
	redirect := sanitizeLocalPath(req.URL.Query().Get("redirect"))
	session, err := r.auth.WeComCenterCallback(req.Context(), ticket)
	if err != nil {
		r.logger.Warn("wecom center login failed", "error", err)
		_ = r.store.WriteAudit(req.Context(), "", "auth.wecom.failed", "auth_provider", "wecom_center", repository.ClientIP(req), map[string]any{
			"reason": wecomFailureReason(err),
		})
		r.redirectAuthResult(w, req, authFailureValues(err))
		return
	}
	_ = r.store.WriteAudit(req.Context(), session.User.ID, "auth.login", "user", session.User.ID, repository.ClientIP(req), map[string]any{
		"username": session.User.Username,
		"provider": "wecom_center",
	})
	r.redirectAuthResult(w, req, url.Values{"token": {session.Token}, "redirect": {redirect}})
}

// redirectAuthResult 通过 URL fragment 回传结果：token 不进服务端日志与 Referer。
func (r *router) redirectAuthResult(w http.ResponseWriter, req *http.Request, values url.Values) {
	http.Redirect(w, req, "/auth/callback#"+values.Encode(), http.StatusFound)
}

func authFailureValues(err error) url.Values {
	switch {
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
	case errors.Is(err, auth.ErrInvalidState):
		return "state_invalid"
	case errors.Is(err, auth.ErrUserNotProvisioned):
		return "user_not_provisioned"
	case errors.Is(err, auth.ErrAuthProviderDisabled):
		return "provider_disabled"
	}
	return "wecom_api_failed"
}

// wecomFailureUser 提取企微账号便于管理员排查开通情况，仅限未开通场景，不含敏感信息。
func wecomFailureUser(err error) string {
	var provisioned auth.NotProvisionedError
	if errors.As(err, &provisioned) {
		return provisioned.Userid
	}
	return ""
}
