package security

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

type Authenticator struct {
	token string
}

func NewAuthenticator(token string) Authenticator {
	return Authenticator{token: token}
}

func (a Authenticator) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		value := req.Header.Get("Authorization")
		if !strings.HasPrefix(value, "Bearer ") {
			http.Error(w, "缺少 Bearer 令牌", http.StatusUnauthorized)
			return
		}
		token := strings.TrimSpace(strings.TrimPrefix(value, "Bearer "))
		if subtle.ConstantTimeCompare([]byte(token), []byte(a.token)) != 1 {
			http.Error(w, "Bearer 令牌无效", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, req)
	})
}
