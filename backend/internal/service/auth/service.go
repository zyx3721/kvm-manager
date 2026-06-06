package auth

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/go-ldap/ldap/v3"
	"golang.org/x/crypto/bcrypt"

	"kvm-manager/backend/internal/domain"
)

var ErrInvalidCredentials = errors.New("invalid username or password")
var ErrInvalidSession = errors.New("invalid or expired session")
var ErrUserNotProvisioned = errors.New("external user is not provisioned")

type Store interface {
	FindUserByUsername(ctx context.Context, username string) (domain.User, string, error)
	UpsertUser(ctx context.Context, username, passwordHash, displayName, role string) (domain.User, error)
	RecordUserLogin(ctx context.Context, userID string) error
	CreateSession(ctx context.Context, token string, userID string, expiresAt time.Time) error
	FindSession(ctx context.Context, token string) (domain.Session, error)
	TouchSession(ctx context.Context, token string, seenAt time.Time) error
	DeleteSession(ctx context.Context, token string) error
	DeleteExpiredSessions(ctx context.Context) error
	GetAuthProvider(ctx context.Context, id string) (domain.AuthProvider, error)
}

const sessionTouchInterval = 5 * time.Minute

type Service struct {
	store          Store
	sessionTTL     time.Duration
	sessionIdleTTL time.Duration
	now            func() time.Time
}

func NewService(store Store, sessionTTL time.Duration) *Service {
	return NewServiceWithIdleTTL(store, sessionTTL, 12*time.Hour)
}

func NewServiceWithIdleTTL(store Store, sessionTTL time.Duration, sessionIdleTTL time.Duration) *Service {
	if sessionIdleTTL <= 0 {
		sessionIdleTTL = 12 * time.Hour
	}
	return &Service{store: store, sessionTTL: sessionTTL, sessionIdleTTL: sessionIdleTTL, now: time.Now}
}

func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(hash), err
}

func VerifyPassword(passwordHash, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(password))
}

func (s *Service) Login(ctx context.Context, username, password string) (domain.Session, error) {
	user, passwordHash, err := s.store.FindUserByUsername(ctx, username)
	if err != nil || user.Disabled {
		return domain.Session{}, ErrInvalidCredentials
	}
	if err := VerifyPassword(passwordHash, password); err != nil {
		return domain.Session{}, ErrInvalidCredentials
	}
	token, err := generateToken(32)
	if err != nil {
		return domain.Session{}, err
	}
	now := s.now()
	expiresAt := now.Add(s.sessionTTL)
	if err := s.store.CreateSession(ctx, token, user.ID, expiresAt); err != nil {
		return domain.Session{}, err
	}
	if err := s.store.RecordUserLogin(ctx, user.ID); err != nil {
		return domain.Session{}, err
	}
	return domain.Session{Token: token, ExpiresAt: expiresAt, LastSeenAt: now, User: user}, nil
}

func (s *Service) LoginWithProvider(ctx context.Context, providerID, username, password string) (domain.Session, error) {
	if providerID == "" || providerID == "local" {
		return s.Login(ctx, username, password)
	}
	provider, err := s.store.GetAuthProvider(ctx, providerID)
	if err != nil || !provider.Enabled || provider.Type != "ldap" {
		return domain.Session{}, ErrInvalidCredentials
	}
	cfg, err := decodeLDAPConfig(provider.Config)
	if err != nil {
		return domain.Session{}, err
	}
	user, err := authenticateLDAP(ctx, cfg, username, password)
	if err != nil {
		return domain.Session{}, ErrInvalidCredentials
	}
	stored, _, err := s.store.FindUserByUsername(ctx, user.Username)
	if err != nil || stored.Disabled {
		return domain.Session{}, ErrUserNotProvisioned
	}
	token, err := generateToken(32)
	if err != nil {
		return domain.Session{}, err
	}
	now := s.now()
	expiresAt := now.Add(s.sessionTTL)
	if err := s.store.CreateSession(ctx, token, stored.ID, expiresAt); err != nil {
		return domain.Session{}, err
	}
	if err := s.store.RecordUserLogin(ctx, stored.ID); err != nil {
		return domain.Session{}, err
	}
	return domain.Session{Token: token, ExpiresAt: expiresAt, LastSeenAt: now, User: stored}, nil
}

func TestLDAPProvider(ctx context.Context, provider domain.AuthProvider) (LDAPTestResult, error) {
	cfg, err := decodeLDAPConfig(provider.Config)
	if err != nil {
		return LDAPTestResult{}, err
	}
	conn, err := dialLDAP(ctx, cfg)
	if err != nil {
		return LDAPTestResult{}, err
	}
	defer conn.Close()
	if cfg.BindDN != "" {
		if err := conn.Bind(cfg.BindDN, cfg.BindPassword); err != nil {
			return LDAPTestResult{}, err
		}
	}
	matchedUsers, err := countLDAPTestUsers(conn, cfg)
	if err != nil {
		return LDAPTestResult{}, err
	}
	return LDAPTestResult{MatchedUsers: matchedUsers}, nil
}

func LDAPUserMessage(err error) string {
	if err == nil {
		return ""
	}
	var unknownAuthority x509.UnknownAuthorityError
	if errors.As(err, &unknownAuthority) {
		return "LDAP TLS 证书不受信任，请导入可信证书或勾选跳过证书校验"
	}
	var hostError x509.HostnameError
	if errors.As(err, &hostError) {
		return "LDAP TLS 证书域名与服务器地址不匹配，请检查证书或勾选跳过证书校验"
	}
	var certInvalidError x509.CertificateInvalidError
	if errors.As(err, &certInvalidError) {
		return "LDAP TLS 证书无效或已过期，请检查证书配置"
	}
	var netError net.Error
	if errors.As(err, &netError) && netError.Timeout() {
		return "LDAP 服务连接超时，请检查服务器地址、端口和网络连通性"
	}
	if errors.Is(err, io.EOF) {
		return "LDAP 服务提前断开连接，请确认端口协议是否匹配"
	}
	message := strings.ToLower(err.Error())
	if strings.Contains(message, "connection refused") {
		return "LDAP 服务拒绝连接，请检查端口是否开放"
	}
	if strings.Contains(message, "connection reset") {
		return "LDAP 连接被重置，请确认端口协议和 TLS 配置是否匹配"
	}
	if strings.Contains(message, "first record does not look like a tls handshake") {
		return "当前端口不是 LDAPS 服务，请改用 389 或关闭 LDAPS"
	}
	if strings.Contains(message, "unsupported protocol version") || strings.Contains(message, "protocol version 301") {
		return "LDAP TLS 版本过低，请启用 TLS 1.2+"
	}
	if strings.Contains(message, "start tls") || strings.Contains(message, "starttls") {
		return "LDAP 服务不支持 StartTLS 或 StartTLS 握手失败，请检查服务端配置"
	}
	if strings.Contains(message, "invalid credentials") {
		return "LDAP 绑定账号或密码不正确"
	}
	var filterErr ldapFilterSearchError
	if errors.As(err, &filterErr) && strings.Contains(message, "filter compile error") {
		return filterErr.Source + "格式不正确，请填写完整 LDAP 过滤器"
	}
	if strings.Contains(message, "filter compile error") {
		return "LDAP 过滤器格式不正确，请检查用户过滤器或用户组过滤器"
	}
	return "认证服务连接测试失败：" + err.Error()
}

func (s *Service) Validate(ctx context.Context, token string) (domain.Session, error) {
	if token == "" {
		return domain.Session{}, ErrInvalidSession
	}
	_ = s.store.DeleteExpiredSessions(ctx)
	session, err := s.store.FindSession(ctx, token)
	now := s.now()
	if err != nil || !session.ExpiresAt.After(now) || !session.LastSeenAt.Add(s.sessionIdleTTL).After(now) || session.User.Disabled {
		return domain.Session{}, ErrInvalidSession
	}
	if now.Sub(session.LastSeenAt) >= sessionTouchInterval {
		if err := s.store.TouchSession(ctx, token, now); err != nil {
			return domain.Session{}, err
		}
		session.LastSeenAt = now
	}
	return session, nil
}

func (s *Service) Logout(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	return s.store.DeleteSession(ctx, token)
}

func generateToken(size int) (string, error) {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

type LDAPConfig struct {
	Host                 string `json:"host"`
	Port                 int    `json:"port"`
	BaseDN               string `json:"baseDN"`
	UserFilter           string `json:"userFilter"`
	BindDN               string `json:"bindDN"`
	BindPassword         string `json:"bindPassword"`
	UseTLS               bool   `json:"useTLS"`
	StartTLS             bool   `json:"startTLS"`
	InsecureSkipVerify   bool   `json:"insecureSkipVerify"`
	UsernameAttribute    string `json:"usernameAttribute"`
	DisplayNameAttribute string `json:"displayNameAttribute"`
	EmailAttribute       string `json:"emailAttribute"`
	TimeoutSeconds       int    `json:"timeoutSeconds"`
	GroupFilter          string `json:"groupFilter"`
}

type LDAPUser struct {
	Username    string
	DisplayName string
}

type LDAPTestResult struct {
	MatchedUsers int
}

type ldapFilterSearchError struct {
	Source string
	Err    error
}

func (err ldapFilterSearchError) Error() string {
	return err.Source + " search failed: " + err.Err.Error()
}

func (err ldapFilterSearchError) Unwrap() error {
	return err.Err
}

func decodeLDAPConfig(data []byte) (LDAPConfig, error) {
	var cfg LDAPConfig
	if len(data) > 0 {
		if err := json.Unmarshal(data, &cfg); err != nil {
			return LDAPConfig{}, err
		}
	}
	cfg.Host = strings.TrimSpace(cfg.Host)
	cfg.BaseDN = strings.TrimSpace(cfg.BaseDN)
	cfg.UserFilter = strings.TrimSpace(cfg.UserFilter)
	cfg.BindDN = strings.TrimSpace(cfg.BindDN)
	cfg.UsernameAttribute = strings.TrimSpace(cfg.UsernameAttribute)
	cfg.DisplayNameAttribute = strings.TrimSpace(cfg.DisplayNameAttribute)
	cfg.EmailAttribute = strings.TrimSpace(cfg.EmailAttribute)
	cfg.GroupFilter = strings.TrimSpace(cfg.GroupFilter)
	if cfg.Port == 0 {
		if cfg.UseTLS {
			cfg.Port = 636
		} else {
			cfg.Port = 389
		}
	}
	if cfg.UseTLS && cfg.StartTLS {
		return LDAPConfig{}, fmt.Errorf("LDAPS and StartTLS cannot both be enabled")
	}
	if cfg.UserFilter == "" {
		cfg.UserFilter = "(sAMAccountName={username})"
	}
	if cfg.UsernameAttribute == "" {
		cfg.UsernameAttribute = "sAMAccountName"
	}
	if cfg.DisplayNameAttribute == "" {
		cfg.DisplayNameAttribute = "displayName"
	}
	if cfg.EmailAttribute == "" {
		cfg.EmailAttribute = "mail"
	}
	if cfg.TimeoutSeconds <= 0 || cfg.TimeoutSeconds > 30 {
		cfg.TimeoutSeconds = 8
	}
	if cfg.Host == "" || cfg.BaseDN == "" {
		return LDAPConfig{}, fmt.Errorf("LDAP host and base DN are required")
	}
	return cfg, nil
}

func authenticateLDAP(ctx context.Context, cfg LDAPConfig, username, password string) (LDAPUser, error) {
	username = strings.TrimSpace(username)
	if username == "" || password == "" {
		return LDAPUser{}, ErrInvalidCredentials
	}
	conn, err := dialLDAP(ctx, cfg)
	if err != nil {
		return LDAPUser{}, err
	}
	defer conn.Close()
	if cfg.BindDN != "" {
		if err := conn.Bind(cfg.BindDN, cfg.BindPassword); err != nil {
			return LDAPUser{}, err
		}
	}
	filter := ldapLoginUserFilter(cfg, username)
	search := ldap.NewSearchRequest(
		cfg.BaseDN,
		ldap.ScopeWholeSubtree,
		ldap.NeverDerefAliases,
		1,
		cfg.TimeoutSeconds,
		false,
		filter,
		[]string{cfg.UsernameAttribute, cfg.DisplayNameAttribute, cfg.EmailAttribute, "dn"},
		nil,
	)
	result, err := conn.Search(search)
	if err != nil || len(result.Entries) != 1 {
		return LDAPUser{}, ErrInvalidCredentials
	}
	entry := result.Entries[0]
	if err := conn.Bind(entry.DN, password); err != nil {
		return LDAPUser{}, ErrInvalidCredentials
	}
	resolvedUsername := entry.GetAttributeValue(cfg.UsernameAttribute)
	if strings.TrimSpace(resolvedUsername) == "" {
		resolvedUsername = username
	}
	displayName := entry.GetAttributeValue(cfg.DisplayNameAttribute)
	if strings.TrimSpace(displayName) == "" {
		displayName = resolvedUsername
	}
	return LDAPUser{Username: resolvedUsername, DisplayName: displayName}, nil
}

func ldapLoginUserFilter(cfg LDAPConfig, username string) string {
	userFilter := strings.ReplaceAll(cfg.UserFilter, "{username}", ldap.EscapeFilter(username))
	groupFilter := ldapNormalizedGroupFilter(cfg)
	if groupFilter == "" {
		return userFilter
	}
	return "(&" + userFilter + groupFilter + ")"
}

func ldapNormalizedGroupFilter(cfg LDAPConfig) string {
	groupFilter := strings.TrimSpace(cfg.GroupFilter)
	if groupFilter == "" {
		return ""
	}
	if strings.HasPrefix(groupFilter, "(") {
		return groupFilter
	}
	return "(memberOf=" + ldap.EscapeFilter(groupFilter) + ")"
}

func ldapTestUserFilter(cfg LDAPConfig) (string, string) {
	if cfg.GroupFilter != "" {
		return ldapNormalizedGroupFilter(cfg), "用户组过滤器"
	}
	return strings.ReplaceAll(cfg.UserFilter, "{username}", "*"), "用户过滤器"
}

func countLDAPTestUsers(conn *ldap.Conn, cfg LDAPConfig) (int, error) {
	filter, source := ldapTestUserFilter(cfg)
	search := ldap.NewSearchRequest(
		cfg.BaseDN,
		ldap.ScopeWholeSubtree,
		ldap.NeverDerefAliases,
		0,
		cfg.TimeoutSeconds,
		false,
		filter,
		[]string{"dn"},
		nil,
	)
	result, err := conn.SearchWithPaging(search, 500)
	if err != nil {
		return 0, ldapFilterSearchError{Source: source, Err: err}
	}
	return len(result.Entries), nil
}

func dialLDAP(ctx context.Context, cfg LDAPConfig) (*ldap.Conn, error) {
	network := "tcp"
	address := net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))
	timeout := time.Duration(cfg.TimeoutSeconds) * time.Second
	tlsConfig := &tls.Config{ServerName: cfg.Host, InsecureSkipVerify: cfg.InsecureSkipVerify}
	var conn *ldap.Conn
	var err error
	if cfg.UseTLS {
		dialer := &tls.Dialer{NetDialer: &net.Dialer{Timeout: timeout}, Config: tlsConfig}
		rawConn, dialErr := dialer.DialContext(ctx, network, address)
		if dialErr != nil {
			return nil, dialErr
		}
		conn = ldap.NewConn(rawConn, true)
		conn.Start()
	} else {
		dialer := &net.Dialer{Timeout: timeout}
		rawConn, dialErr := dialer.DialContext(ctx, network, address)
		if dialErr != nil {
			return nil, dialErr
		}
		conn = ldap.NewConn(rawConn, false)
		conn.Start()
		if cfg.StartTLS {
			err = conn.StartTLS(tlsConfig)
		}
	}
	if err != nil {
		conn.Close()
		return nil, err
	}
	conn.SetTimeout(time.Duration(cfg.TimeoutSeconds) * time.Second)
	return conn, nil
}
