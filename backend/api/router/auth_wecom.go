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

// wecomAuthorizeEmbed 内嵌二维码登录参数：iframe 地址与回跳中转路由。
type wecomAuthorizeEmbed struct {
	AuthMode     string `json:"auth_mode"`
	IframeURL    string `json:"iframe_url"`
	CallbackPath string `json:"callback_path"`
}

// wecomAuthorizeResponse 登录跳转地址与内嵌二维码参数，embed 为空表示仅支持整页跳转。
type wecomAuthorizeResponse struct {
	URL   string               `json:"url"`
	Embed *wecomAuthorizeEmbed `json:"embed,omitempty"`
}

// handleWecomAuthorize 获取企业微信扫码登录地址与内嵌二维码参数：直连返回企微授权页，统一认证中心返回认证中心登录页。
func (r *router) handleWecomAuthorize(w http.ResponseWriter, req *http.Request) {
	redirect := sanitizeLocalPath(req.URL.Query().Get("redirect"))
	payload, err := r.auth.WeComLoginPayload(req.Context(), redirect, repository.ClientIP(req), wecomRequestBaseURL(req))
	if err != nil {
		r.logger.Error("wecom authorize failed", "error", err)
		writeError(w, http.StatusServiceUnavailable, "wecom_authorize_failed", auth.WeComUserMessage(err))
		return
	}
	response := wecomAuthorizeResponse{URL: payload.URL}
	if payload.Embed != nil {
		response.Embed = &wecomAuthorizeEmbed{
			AuthMode:     payload.Embed.AuthMode,
			IframeURL:    payload.Embed.IframeURL,
			CallbackPath: payload.Embed.CallbackPath,
		}
	}
	writeJSON(w, http.StatusOK, response)
}

func (r *router) handleWecomCallback(w http.ResponseWriter, req *http.Request) {
	r.wecomDirectCallback(w, req, auth.WecomFrontendCallbackPath)
}

// handleWecomEmbedCallback 内嵌二维码直连回调：处理逻辑与整页回调一致，结果 302 回内嵌中转路由供父页面读取。
func (r *router) handleWecomEmbedCallback(w http.ResponseWriter, req *http.Request) {
	r.wecomDirectCallback(w, req, auth.WecomEmbedCallbackPath)
}

// wecomDirectCallback 企业微信直连回调共用处理：消费 code 与 state 后按用途分派登录或绑定，结果 302 回指定前端回调页。
func (r *router) wecomDirectCallback(w http.ResponseWriter, req *http.Request, frontendPath string) {
	code := req.URL.Query().Get("code")
	state := req.URL.Query().Get("state")
	result, err := r.auth.WeComCallback(req.Context(), code, state)
	if err != nil {
		r.logger.Warn("wecom login failed", "error", err)
		auditUserID := wecomFailureAuditUserID(err)
		_ = r.store.WriteAudit(req.Context(), auditUserID, "auth.wecom.failed", auditWecomResourceType(auditUserID), auditWecomResourceID(auditUserID), repository.ClientIP(req), wecomFailureMetadata(err))
		r.redirectAuthResult(w, req, frontendPath, authFailureValues(err))
		return
	}
	r.finishWecomResult(w, req, result, frontendPath)
}

// handleWecomSSOCallback 统一认证中心票据回调：verify 换取身份后按 redirect 中的绑定票据分派登录或绑定，
// redirect 等于内嵌中转路径时按内嵌模式回跳。
func (r *router) handleWecomSSOCallback(w http.ResponseWriter, req *http.Request) {
	ticket := req.URL.Query().Get("ticket")
	redirect := req.URL.Query().Get("redirect")
	frontendPath := auth.WecomFrontendCallbackPath
	if redirect == auth.WecomEmbedCallbackPath {
		frontendPath = auth.WecomEmbedCallbackPath
	}
	result, err := r.auth.WeComSSOCallback(req.Context(), ticket, redirect)
	if err != nil {
		r.logger.Warn("wecom sso login failed", "error", err)
		auditUserID := wecomFailureAuditUserID(err)
		_ = r.store.WriteAudit(req.Context(), auditUserID, "auth.wecom.failed", auditWecomResourceType(auditUserID), auditWecomResourceID(auditUserID), repository.ClientIP(req), wecomFailureMetadata(err))
		r.redirectAuthResult(w, req, frontendPath, authFailureValues(err))
		return
	}
	r.finishWecomResult(w, req, result, frontendPath)
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
func (r *router) finishWecomResult(w http.ResponseWriter, req *http.Request, result auth.WecomResult, frontendPath string) {
	if result.Kind == auth.AuthPurposeBind {
		// 绑定作用于系统用户，资源与用户列按登录审计同风格记录发起绑定者
		_ = r.store.WriteAudit(req.Context(), result.BindUserID, "auth.wecom.bind", "user", result.BindUserID, repository.ClientIP(req), map[string]any{
			"username": result.Username,
			"userid":   result.Userid,
		})
		r.redirectAuthResult(w, req, frontendPath, url.Values{"bind": {"success"}})
		return
	}
	session := result.Session
	_ = r.store.WriteAudit(req.Context(), session.User.ID, "auth.login", "user", session.User.ID, repository.ClientIP(req), map[string]any{
		"username": session.User.Username,
		"provider": "wecom",
	})
	r.redirectAuthResult(w, req, frontendPath, url.Values{
		"token":       {session.Token},
		"redirect":    {sanitizeLocalPath(result.Redirect)},
		"wecom_bound": {"true"},
	})
}

// redirectAuthResult 通过 URL fragment 回传结果：token 不进服务端日志与 Referer。
func (r *router) redirectAuthResult(w http.ResponseWriter, req *http.Request, frontendPath string, values url.Values) {
	http.Redirect(w, req, frontendPath+"#"+values.Encode(), http.StatusFound)
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

// wecomFailureAuditUserID 提取失败场景中已确定的系统用户（绑定冲突、账号被禁用），未知时为空。
func wecomFailureAuditUserID(err error) string {
	var loginErr auth.WecomLoginError
	if errors.As(err, &loginErr) {
		return loginErr.BindUserID
	}
	return ""
}

// auditWecomResourceType 失败审计的资源类型：能定位系统用户时按用户记录，否则按认证提供方记录。
func auditWecomResourceType(auditUserID string) string {
	if auditUserID != "" {
		return "user"
	}
	return "auth_provider"
}

// auditWecomResourceID 失败审计的资源 ID：能定位系统用户时为其 ID，否则为提供方 ID。
func auditWecomResourceID(auditUserID string) string {
	if auditUserID != "" {
		return auditUserID
	}
	return "wecom"
}

// wecomFailureMetadata 失败审计 metadata：携带失败原因、企微账号，以及已知系统用户名。
func wecomFailureMetadata(err error) map[string]any {
	metadata := map[string]any{
		"reason": wecomFailureReason(err),
		"userid": wecomFailureUser(err),
	}
	var loginErr auth.WecomLoginError
	if errors.As(err, &loginErr) && loginErr.Username != "" {
		metadata["username"] = loginErr.Username
	}
	return metadata
}
