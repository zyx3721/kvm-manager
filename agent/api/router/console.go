package router

import (
	"errors"
	"io"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var consoleUpgrader = websocket.Upgrader{
	ReadBufferSize:  32 * 1024,
	WriteBufferSize: 32 * 1024,
	Subprotocols:    []string{"binary"},
	CheckOrigin: func(req *http.Request) bool {
		return true
	},
}

func (r *Router) handleConsoleWS(w http.ResponseWriter, req *http.Request, name string) {
	info, err := r.provider.ConsoleInfo(name)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "console_unavailable", "控制台不可用")
		return
	}
	if info.Type != "vnc" {
		writeError(w, http.StatusBadRequest, "console_type_unsupported", "控制台类型不支持")
		return
	}
	tcpConn, err := net.DialTimeout("tcp", net.JoinHostPort(info.Listen, strconv.Itoa(info.Port)), 5*time.Second)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "console_connect_failed", "控制台连接失败")
		return
	}
	wsConn, err := consoleUpgrader.Upgrade(w, req, nil)
	if err != nil {
		_ = tcpConn.Close()
		return
	}
	proxyWebSocketToTCP(wsConn, tcpConn)
}

func proxyWebSocketToTCP(wsConn *websocket.Conn, tcpConn net.Conn) {
	defer wsConn.Close()
	defer tcpConn.Close()

	var once sync.Once
	closeBoth := func() {
		once.Do(func() {
			_ = wsConn.Close()
			_ = tcpConn.Close()
		})
	}

	done := make(chan struct{}, 2)
	go func() {
		defer func() { closeBoth(); done <- struct{}{} }()
		buf := make([]byte, 32*1024)
		for {
			n, err := tcpConn.Read(buf)
			if n > 0 {
				if writeErr := wsConn.WriteMessage(websocket.BinaryMessage, buf[:n]); writeErr != nil {
					return
				}
			}
			if err != nil {
				return
			}
		}
	}()

	go func() {
		defer func() { closeBoth(); done <- struct{}{} }()
		for {
			messageType, payload, err := wsConn.ReadMessage()
			if err != nil {
				return
			}
			if messageType != websocket.BinaryMessage && messageType != websocket.TextMessage {
				continue
			}
			if _, err := tcpConn.Write(payload); err != nil && !errors.Is(err, io.ErrClosedPipe) {
				return
			}
		}
	}()

	<-done
}
