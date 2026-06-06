package router

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"kvm-manager/backend/internal/domain"
	"kvm-manager/backend/internal/repository"
	"kvm-manager/backend/pkg/agent"
	"kvm-manager/backend/pkg/tokencrypto"
)

const agentProbeTimeout = 5 * time.Second

func (r *router) handleListAgents(w http.ResponseWriter, req *http.Request) {
	if !r.ensurePermission(w, req, "agents.read") {
		return
	}
	agents, err := r.store.ListAgents(req.Context())
	if err != nil {
		r.logger.Error("list agents failed", "error", err)
		writeError(w, http.StatusInternalServerError, "list_agents_failed", "读取 Agent 列表失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": agents, "total": len(agents)})
}

func (r *router) handleCreateAgent(w http.ResponseWriter, req *http.Request) {
	if !r.ensurePermission(w, req, "agents.manage") {
		return
	}
	defer req.Body.Close()
	var body createAgentRequest
	if err := decodeJSONBody(w, req, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "请求体必须是有效的 Agent JSON")
		return
	}
	body.Name = strings.TrimSpace(body.Name)
	body.Endpoint = strings.TrimRight(strings.TrimSpace(body.Endpoint), "/")
	body.Token = strings.TrimSpace(body.Token)
	if body.Name == "" || body.Endpoint == "" || body.Token == "" {
		writeError(w, http.StatusBadRequest, "missing_agent_fields", "Agent 名称、地址和令牌不能为空")
		return
	}
	if !strings.HasPrefix(body.Endpoint, "http://") && !strings.HasPrefix(body.Endpoint, "https://") {
		writeError(w, http.StatusBadRequest, "invalid_endpoint", "Agent 地址必须以 http:// 或 https:// 开头")
		return
	}
	if err := r.ensureAgentNameUnique(req.Context(), body.Name); err != nil {
		if errors.Is(err, errAgentNameDuplicate) {
			writeError(w, http.StatusConflict, "agent_exists", "Agent 名称已存在")
			return
		}
		r.logger.Error("check agent name failed", "error", err)
		writeError(w, http.StatusInternalServerError, "check_agent_name_failed", "检查 Agent 名称失败")
		return
	}
	if err := r.ensureAgentEndpointUnique(req.Context(), body.Endpoint); err != nil {
		switch {
		case errors.Is(err, errAgentEndpointDuplicate), errors.Is(err, errAgentEndpointIPDuplicate):
			writeError(w, http.StatusConflict, "agent_endpoint_exists", "Agent 地址已存在")
		case errors.Is(err, errInvalidAgentEndpoint):
			writeError(w, http.StatusBadRequest, "invalid_endpoint", invalidAgentEndpointMessage(err))
		default:
			r.logger.Error("check agent endpoint failed", "error", err)
			writeError(w, http.StatusInternalServerError, "check_agent_endpoint_failed", "检查 Agent 地址失败")
		}
		return
	}
	tokenCiphertext, err := tokencrypto.Seal(r.cfg.JWT.Secret, body.Token)
	if err != nil {
		r.logger.Error("seal agent token failed", "error", err)
		writeError(w, http.StatusInternalServerError, "create_agent_failed", "保存 Agent 令牌失败")
		return
	}
	agentRecord, err := r.store.CreateAgent(req.Context(), body.Name, body.Endpoint, body.Token, tokenCiphertext, body.TLSInsecure)
	if err != nil {
		if repository.IsUniqueViolation(err) {
			writeError(w, http.StatusConflict, "agent_exists", "Agent 名称已存在")
			return
		}
		r.logger.Error("create agent failed", "error", err)
		writeError(w, http.StatusInternalServerError, "create_agent_failed", "创建 Agent 失败")
		return
	}
	_ = r.store.WriteAudit(req.Context(), currentSession(req).User.ID, "agent.create", "agent", agentRecord.ID, repository.ClientIP(req), map[string]any{"name": agentRecord.Name, "endpoint": agentRecord.Endpoint})
	writeJSON(w, http.StatusCreated, agentRecord)
}

func (r *router) handleAgentProbe(w http.ResponseWriter, req *http.Request) {
	if !r.ensurePermission(w, req, "agents.manage") {
		return
	}
	defer req.Body.Close()
	var body createAgentRequest
	if err := decodeJSONBody(w, req, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "请求体必须是有效的 Agent JSON")
		return
	}
	body.Endpoint = strings.TrimRight(strings.TrimSpace(body.Endpoint), "/")
	body.Token = strings.TrimSpace(body.Token)
	if body.Endpoint == "" || body.Token == "" {
		writeError(w, http.StatusBadRequest, "missing_agent_fields", "Agent 地址和令牌不能为空")
		return
	}
	if !strings.HasPrefix(body.Endpoint, "http://") && !strings.HasPrefix(body.Endpoint, "https://") {
		writeError(w, http.StatusBadRequest, "invalid_endpoint", "Agent 地址必须以 http:// 或 https:// 开头")
		return
	}
	info, err := probeAgentHost(req.Context(), body.Endpoint, body.Token, body.TLSInsecure)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "agent_unreachable", agentConnectionErrorMessage(err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "host": info})
}

func (r *router) handleAgentRoute(w http.ResponseWriter, req *http.Request) {
	if !r.ensurePermission(w, req, "agents.manage") {
		return
	}
	id, action, ok := parseAgentPath(req.URL.Path)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", "接口不存在")
		return
	}
	if req.Method == http.MethodDelete {
		if action != "" {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "请求方法不支持")
			return
		}
		r.handleAgentDelete(w, req, id)
		return
	}
	if req.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "请求方法不支持")
		return
	}
	switch action {
	case "test-connection":
		r.handleAgentTest(w, req, id)
	case "sync":
		r.handleAgentSync(w, req, id)
	default:
		writeError(w, http.StatusNotFound, "unknown_action", "不支持的 Agent 操作")
	}
}

func (r *router) handleAgentDelete(w http.ResponseWriter, req *http.Request, id string) {
	agentRecord, err := r.store.GetAgent(req.Context(), id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeError(w, http.StatusNotFound, "agent_not_found", "Agent 不存在")
			return
		}
		writeError(w, http.StatusInternalServerError, "get_agent_failed", "读取 Agent 失败")
		return
	}
	if err := r.store.DeleteAgent(req.Context(), id); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeError(w, http.StatusNotFound, "agent_not_found", "Agent 不存在")
			return
		}
		r.logger.Error("delete agent failed", "error", err, "agent", id)
		writeError(w, http.StatusInternalServerError, "delete_agent_failed", "删除 Agent 失败")
		return
	}
	if err := r.store.ResolveActiveAlertsBySource(req.Context(), "agent", id); err != nil {
		r.logger.Warn("resolve agent alerts after delete failed", "error", err, "agent", id)
	}
	r.runtime.RemoveAgent(id)
	r.runtime.Broadcast("runtime.updated")
	_ = r.store.WriteAudit(req.Context(), currentSession(req).User.ID, "agent.delete", "agent", id, repository.ClientIP(req), map[string]any{"name": agentRecord.Name, "endpoint": agentRecord.Endpoint})
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (r *router) handleAgentTest(w http.ResponseWriter, req *http.Request, id string) {
	agentRecord, token, ok := r.agentWithOptionalToken(w, req, id)
	if !ok {
		return
	}
	info, err := probeAgentHost(req.Context(), agentRecord.Endpoint, token, agentRecord.TLSInsecure)
	if err != nil {
		message := agentConnectionErrorMessage(err)
		_ = r.store.UpdateAgentHealth(req.Context(), id, "offline", "", []string{}, message)
		_ = r.store.WriteAudit(req.Context(), currentSession(req).User.ID, "agent.test", "agent", id, repository.ClientIP(req), map[string]any{"agent": agentRecord.Name, "status": "offline", "error": message})
		writeError(w, http.StatusServiceUnavailable, "agent_unreachable", message)
		return
	}
	_ = r.store.UpdateAgentHealth(req.Context(), id, "online", info.KVMVersion, info.Capabilities, "")
	_ = r.store.WriteAudit(req.Context(), currentSession(req).User.ID, "agent.test", "agent", id, repository.ClientIP(req), map[string]any{"agent": agentRecord.Name, "status": "online"})
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "host": info})
}

func probeAgentHost(ctx context.Context, endpoint, token string, tlsInsecure bool) (agent.HostInfo, error) {
	ctx, cancel := context.WithTimeout(ctx, agentProbeTimeout)
	defer cancel()
	client := agent.NewClientWithTimeout(tlsInsecure, agentProbeTimeout)
	return client.HostInfo(ctx, agent.Config{Endpoint: endpoint, Token: token, TLSInsecure: tlsInsecure})
}

var (
	errAgentNameDuplicate       = errors.New("agent name already exists")
	errAgentEndpointDuplicate   = errors.New("agent endpoint already exists")
	errAgentEndpointIPDuplicate = errors.New("agent endpoint ip already exists")
	errInvalidAgentEndpoint     = errors.New("invalid agent endpoint")
)

func invalidAgentEndpointMessage(err error) string {
	message := strings.TrimSpace(err.Error())
	prefix := errInvalidAgentEndpoint.Error() + ":"
	if strings.HasPrefix(message, prefix) {
		return strings.TrimSpace(strings.TrimPrefix(message, prefix))
	}
	return "Agent 地址格式不正确"
}

type agentEndpointIdentity struct {
	normalized string
	ips        []string
}

func (r *router) ensureAgentNameUnique(ctx context.Context, name string) error {
	if _, err := r.store.GetAgentByName(ctx, name); err == nil {
		return errAgentNameDuplicate
	} else if !errors.Is(err, repository.ErrNotFound) {
		return err
	}
	return nil
}

func (r *router) ensureAgentEndpointUnique(ctx context.Context, endpoint string) error {
	identity, err := resolveAgentEndpointIdentity(ctx, endpoint)
	if err != nil {
		return err
	}
	if _, err := r.store.GetAgentByEndpoint(ctx, identity.normalized); err == nil {
		return errAgentEndpointDuplicate
	} else if !errors.Is(err, repository.ErrNotFound) {
		return err
	}
	existingAgents, err := r.store.ListAgents(ctx)
	if err != nil {
		return err
	}
	for _, item := range existingAgents {
		existing, err := resolveAgentEndpointIdentity(ctx, item.Endpoint)
		if err != nil {
			continue
		}
		if endpointIdentityMatches(identity, existing) {
			return errAgentEndpointIPDuplicate
		}
	}
	return nil
}

func resolveAgentEndpointIdentity(ctx context.Context, endpoint string) (agentEndpointIdentity, error) {
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return agentEndpointIdentity{}, fmt.Errorf("%w: Agent 地址格式不正确", errInvalidAgentEndpoint)
	}
	if parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return agentEndpointIdentity{}, fmt.Errorf("%w: Agent 地址不能包含认证信息、查询参数或片段", errInvalidAgentEndpoint)
	}
	host := parsed.Hostname()
	port := parsed.Port()
	if host == "" {
		return agentEndpointIdentity{}, fmt.Errorf("%w: Agent 地址缺少主机名", errInvalidAgentEndpoint)
	}
	if port == "" {
		if parsed.Scheme == "http" {
			port = "80"
		} else if parsed.Scheme == "https" {
			port = "443"
		}
	}
	if _, err := net.LookupPort("tcp", port); err != nil {
		return agentEndpointIdentity{}, fmt.Errorf("%w: Agent 地址端口不正确", errInvalidAgentEndpoint)
	}
	ips := make([]string, 0, 1)
	if parsedIP := net.ParseIP(host); parsedIP != nil {
		ips = append(ips, parsedIP.String())
	} else {
		lookupCtx, cancel := context.WithTimeout(ctx, agentProbeTimeout)
		defer cancel()
		resolved, err := net.DefaultResolver.LookupIPAddr(lookupCtx, host)
		if err != nil {
			return agentEndpointIdentity{}, fmt.Errorf("%w: Agent 地址无法解析", errInvalidAgentEndpoint)
		}
		seen := map[string]struct{}{}
		for _, addr := range resolved {
			ip := addr.IP.String()
			if ip == "" {
				continue
			}
			if _, ok := seen[ip]; ok {
				continue
			}
			seen[ip] = struct{}{}
			ips = append(ips, ip)
		}
	}
	if len(ips) == 0 {
		return agentEndpointIdentity{}, fmt.Errorf("%w: Agent 地址没有可用 IP", errInvalidAgentEndpoint)
	}
	sort.Strings(ips)
	parsed.Host = net.JoinHostPort(strings.ToLower(host), port)
	parsed.Path = strings.TrimRight(parsed.EscapedPath(), "/")
	return agentEndpointIdentity{normalized: parsed.String(), ips: ips}, nil
}

func endpointIdentityMatches(next agentEndpointIdentity, existing agentEndpointIdentity) bool {
	if next.normalized == existing.normalized {
		return true
	}
	if !sameEndpointSchemePortPath(next.normalized, existing.normalized) {
		return false
	}
	for _, nextIP := range next.ips {
		for _, existingIP := range existing.ips {
			if nextIP == existingIP {
				return true
			}
		}
	}
	return false
}

func sameEndpointSchemePortPath(a string, b string) bool {
	left, leftErr := url.Parse(a)
	right, rightErr := url.Parse(b)
	if leftErr != nil || rightErr != nil {
		return false
	}
	return left.Scheme == right.Scheme && left.Port() == right.Port() && strings.TrimRight(left.EscapedPath(), "/") == strings.TrimRight(right.EscapedPath(), "/")
}

func agentConnectionErrorMessage(err error) string {
	return agent.UserFacingErrorMessage(err)
}

func (r *router) handleAgentSync(w http.ResponseWriter, req *http.Request, id string) {
	agentRecord, token, ok := r.agentWithOptionalToken(w, req, id)
	if !ok {
		return
	}
	tokenCiphertext, err := tokencrypto.Seal(r.cfg.JWT.Secret, token)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "seal_agent_token_failed", "保存 Agent 令牌失败")
		return
	}
	_ = r.store.UpdateAgentTokenCiphertext(req.Context(), id, tokenCiphertext)
	if err := r.runtime.SyncAgentWithToken(req.Context(), id, token); err != nil {
		writeError(w, http.StatusServiceUnavailable, "agent_sync_failed", agentConnectionErrorMessage(err))
		return
	}
	r.runtime.Broadcast("runtime.updated")
	_ = r.store.WriteAudit(req.Context(), currentSession(req).User.ID, "agent.sync", "agent", id, repository.ClientIP(req), map[string]any{"agent": agentRecord.Name})
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "agent": agentRecord.Name})
}

func (r *router) agentWithToken(w http.ResponseWriter, req *http.Request, id string) (domain.Agent, string, bool) {
	agentRecord, err := r.store.GetAgent(req.Context(), id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeError(w, http.StatusNotFound, "agent_not_found", "Agent 不存在")
			return domain.Agent{}, "", false
		}
		writeError(w, http.StatusInternalServerError, "get_agent_failed", "读取 Agent 失败")
		return domain.Agent{}, "", false
	}
	var body struct {
		Token string `json:"token"`
	}
	_ = decodeJSONBody(w, req, &body)
	body.Token = strings.TrimSpace(body.Token)
	if body.Token == "" {
		writeError(w, http.StatusBadRequest, "missing_agent_token", "执行 Agent 操作时必须提供令牌")
		return domain.Agent{}, "", false
	}
	tokenHash, err := r.store.GetAgentTokenHash(req.Context(), id)
	if err != nil || repository.HashTokenForLookup(body.Token) != tokenHash {
		writeError(w, http.StatusUnauthorized, "invalid_agent_token", "Agent 令牌不正确")
		return domain.Agent{}, "", false
	}
	return agentRecord, body.Token, true
}

func (r *router) agentWithOptionalToken(w http.ResponseWriter, req *http.Request, id string) (domain.Agent, string, bool) {
	agentRecord, err := r.store.GetAgent(req.Context(), id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeError(w, http.StatusNotFound, "agent_not_found", "Agent 不存在")
			return domain.Agent{}, "", false
		}
		writeError(w, http.StatusInternalServerError, "get_agent_failed", "读取 Agent 失败")
		return domain.Agent{}, "", false
	}
	var body struct {
		Token string `json:"token"`
	}
	_ = decodeJSONBody(w, req, &body)
	body.Token = strings.TrimSpace(body.Token)
	if body.Token != "" {
		tokenHash, err := r.store.GetAgentTokenHash(req.Context(), id)
		if err != nil || repository.HashTokenForLookup(body.Token) != tokenHash {
			writeError(w, http.StatusUnauthorized, "invalid_agent_token", "Agent 令牌不正确")
			return domain.Agent{}, "", false
		}
		return agentRecord, body.Token, true
	}
	token, err := tokencrypto.Open(r.cfg.JWT.Secret, agentRecord.TokenCiphertext)
	if err != nil || strings.TrimSpace(token) == "" {
		writeError(w, http.StatusBadRequest, "agent_token_unavailable", "Agent 令牌不可用，请重新保存 Agent")
		return domain.Agent{}, "", false
	}
	return agentRecord, token, true
}

func parseAgentPath(path string) (id string, action string, ok bool) {
	trimmed := strings.Trim(strings.TrimPrefix(path, "/api/agents/"), "/")
	if trimmed == "" {
		return "", "", false
	}
	parts := strings.Split(trimmed, "/")
	if len(parts) == 1 && parts[0] != "" {
		return parts[0], "", true
	}
	if len(parts) == 2 && parts[0] != "" && parts[1] != "" {
		return parts[0], parts[1], true
	}
	return "", "", false
}
