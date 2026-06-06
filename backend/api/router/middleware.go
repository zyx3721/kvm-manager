package router

import (
	"context"
	"net/http"
	"strings"
	"time"
)

type sessionContextKey struct{}

func (r *router) withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		origin := req.Header.Get("Origin")
		if origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Credentials", "true")
		}
		if req.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, req)
	})
}

func (r *router) withRequestLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, req)
		r.logger.Info("http request", "method", req.Method, "path", req.URL.Path, "duration", time.Since(start))
	})
}

func (r *router) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		token := bearerToken(req)
		if token == "" {
			token = strings.TrimSpace(req.URL.Query().Get("token"))
		}
		session, err := r.auth.Validate(req.Context(), token)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "unauthorized", "会话已失效，请重新登录")
			return
		}
		ctx := context.WithValue(req.Context(), sessionContextKey{}, session)
		next.ServeHTTP(w, req.WithContext(ctx))
	})
}

func (r *router) requirePermission(permission string, next http.Handler) http.Handler {
	return r.requireAuth(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		session := currentSession(req)
		if !hasPermission(session.User, permission) {
			writeError(w, http.StatusForbidden, "permission_denied", "当前用户无权执行此操作")
			return
		}
		next.ServeHTTP(w, req)
	}))
}

func bearerToken(req *http.Request) string {
	value := req.Header.Get("Authorization")
	if value == "" {
		return ""
	}
	prefix := "Bearer "
	if !strings.HasPrefix(value, prefix) {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(value, prefix))
}
