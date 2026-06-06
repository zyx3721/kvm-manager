package router

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"kvm-manager/backend/internal/repository"
	"kvm-manager/backend/pkg/agent"
	"kvm-manager/backend/pkg/tokencrypto"
)

var vmConsoleUpgrader = websocket.Upgrader{
	ReadBufferSize:  32 * 1024,
	WriteBufferSize: 32 * 1024,
	Subprotocols:    []string{"binary"},
	CheckOrigin: func(req *http.Request) bool {
		return true
	},
}

func (r *router) handleVMConsoleWS(w http.ResponseWriter, req *http.Request, id string) {
	vm, ok := r.runtime.GetVM(id)
	if !ok {
		writeError(w, http.StatusNotFound, "vm_not_found", "虚拟机不存在")
		return
	}
	if vm.Status != "running" {
		writeError(w, http.StatusBadRequest, "vm_not_running", "虚拟机未运行，无法打开控制台")
		return
	}
	agentRecord, err := r.store.GetAgent(req.Context(), vm.HostID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeError(w, http.StatusBadRequest, "agent_not_bound", "该虚拟机所属宿主机未绑定 Agent")
			return
		}
		writeError(w, http.StatusInternalServerError, "get_agent_failed", "读取 Agent 失败")
		return
	}
	token, err := tokencrypto.Open(r.cfg.JWT.Secret, agentRecord.TokenCiphertext)
	if err != nil || strings.TrimSpace(token) == "" {
		r.logger.Error("open agent token for console failed", "error", err, "agent", agentRecord.ID)
		writeError(w, http.StatusBadRequest, "agent_token_unavailable", "Agent 令牌不可用于打开控制台")
		return
	}
	agentURL, err := agentConsoleURL(agentRecord.Endpoint, vm.Name)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_agent_endpoint", "Agent 地址不正确")
		return
	}

	frontendConn, err := vmConsoleUpgrader.Upgrade(w, req, nil)
	if err != nil {
		return
	}
	defer frontendConn.Close()

	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
		TLSClientConfig:  &tls.Config{InsecureSkipVerify: agentRecord.TLSInsecure},
		Subprotocols:     []string{"binary"},
	}
	header := http.Header{}
	header.Set("Authorization", "Bearer "+token)
	agentConn, resp, err := dialer.DialContext(req.Context(), agentURL, header)
	if err != nil {
		if resp != nil {
			r.logger.Error("agent console websocket failed", "error", err, "status", resp.Status)
		} else {
			r.logger.Error("agent console websocket failed", "error", err)
		}
		_ = frontendConn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseTryAgainLater, "console unavailable"))
		return
	}
	defer agentConn.Close()

	session := currentSession(req)
	_ = r.store.WriteAudit(req.Context(), session.User.ID, "vm.console", "virtual_machine", id, repository.ClientIP(req), map[string]any{"name": vm.Name, "agent": agentRecord.Name})
	proxyWebSockets(frontendConn, agentConn)
}

func (r *router) handleGetVMConsole(w http.ResponseWriter, req *http.Request, id string) {
	vm, ok := r.runtime.GetVM(id)
	if !ok {
		writeError(w, http.StatusNotFound, "vm_not_found", "虚拟机不存在")
		return
	}
	agentRecord, token, ok := r.agentTokenForID(w, req, vm.HostID)
	if !ok {
		return
	}
	info, err := agent.NewClient(agentRecord.TLSInsecure).ConsoleInfo(req.Context(), agent.Config{Endpoint: agentRecord.Endpoint, Token: token, TLSInsecure: agentRecord.TLSInsecure}, vm.Name)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "agent_vm_console_failed", agent.UserFacingErrorMessage(err))
		return
	}
	writeJSON(w, http.StatusOK, info)
}

func parseVMConsolePath(path string) (id string, ok bool) {
	trimmed := strings.Trim(strings.TrimPrefix(path, "/api/vms/"), "/")
	parts := strings.Split(trimmed, "/")
	if len(parts) != 3 || parts[1] != "console" || parts[2] != "ws" {
		return "", false
	}
	return parts[0], parts[0] != ""
}

func agentConsoleURL(endpoint string, vmName string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(endpoint))
	if err != nil {
		return "", err
	}
	switch parsed.Scheme {
	case "http":
		parsed.Scheme = "ws"
	case "https":
		parsed.Scheme = "wss"
	case "ws", "wss":
	default:
		return "", fmt.Errorf("unsupported agent endpoint scheme %q", parsed.Scheme)
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + "/v1/vms/" + urlPathEscape(vmName) + "/console/ws"
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String(), nil
}

func proxyWebSockets(left *websocket.Conn, right *websocket.Conn) {
	var once sync.Once
	closeBoth := func() {
		once.Do(func() {
			_ = left.Close()
			_ = right.Close()
		})
	}
	done := make(chan struct{}, 2)
	pipe := func(dst *websocket.Conn, src *websocket.Conn) {
		defer func() { closeBoth(); done <- struct{}{} }()
		for {
			messageType, payload, err := src.ReadMessage()
			if err != nil {
				return
			}
			if messageType != websocket.BinaryMessage && messageType != websocket.TextMessage {
				continue
			}
			if err := dst.WriteMessage(messageType, payload); err != nil {
				return
			}
		}
	}
	go pipe(left, right)
	go pipe(right, left)
	<-done
}

func urlPathEscape(value string) string {
	return strings.ReplaceAll(url.QueryEscape(value), "+", "%20")
}
