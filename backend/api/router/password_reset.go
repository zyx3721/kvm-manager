package router

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"math"
	"math/big"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"kvm-manager/backend/internal/domain"
	"kvm-manager/backend/internal/repository"
	"kvm-manager/backend/internal/service/auth"
	"kvm-manager/backend/internal/service/notification"
)

const (
	defaultPasswordResetCodeTTL       = 10 * time.Minute
	defaultPasswordResetCaptchaTTL    = time.Minute
	passwordResetVerifyTTL            = 10 * time.Minute
	defaultPasswordResetSendCooldown  = 30 * time.Second
	passwordResetRateLimitMax         = 5
	defaultPasswordResetRateLimitSpan = 5 * time.Minute
)

var emailAddressPattern = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)

type passwordResetCaptchaResponse struct {
	Token       string `json:"token"`
	Question    string `json:"question"`
	ExpiresAt   string `json:"expiresAt"`
	GeneratedAt string `json:"generatedAt"`
}

type passwordResetVerifyRequest struct {
	Username      string `json:"username"`
	CaptchaToken  string `json:"captchaToken"`
	CaptchaAnswer string `json:"captchaAnswer"`
}

type passwordResetSendCodeRequest struct {
	Username          string `json:"username"`
	VerificationToken string `json:"verificationToken"`
	Channel           string `json:"channel"`
	VerifyEmail       string `json:"verifyEmail"`
	To                string `json:"to"`
}

type passwordResetConfirmRequest struct {
	Username        string `json:"username"`
	Code            string `json:"code"`
	NewPassword     string `json:"newPassword"`
	ConfirmPassword string `json:"confirmPassword"`
}

type passwordResetRuntimeSettings struct {
	CodeTTL       time.Duration
	CaptchaTTL    time.Duration
	SendCooldown  time.Duration
	RateLimitMax  int
	RateLimitSpan time.Duration
}

func (r *router) handlePasswordResetCaptcha(w http.ResponseWriter, req *http.Request) {
	settings := r.passwordResetRuntimeSettings(req.Context())
	challenge, err := randomCaptchaChallenge()
	if err != nil {
		r.logger.Error("generate password reset captcha failed", "error", err)
		writeError(w, http.StatusInternalServerError, "captcha_failed", "生成验证码失败")
		return
	}
	expiresAt := time.Now().UTC().Add(settings.CaptchaTTL)
	token := r.signCaptcha(challenge.Answer, expiresAt)
	writeJSON(w, http.StatusOK, passwordResetCaptchaResponse{
		Token:       token,
		Question:    challenge.Question,
		ExpiresAt:   expiresAt.Format(time.RFC3339),
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
	})
}

func (r *router) handlePasswordResetVerify(w http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()
	var body passwordResetVerifyRequest
	if err := decodeJSONBody(w, req, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "请求参数格式不正确")
		return
	}
	username := strings.TrimSpace(body.Username)
	if username == "" {
		writeError(w, http.StatusBadRequest, "missing_username", "请输入用户名")
		return
	}
	if !r.verifyCaptcha(body.CaptchaToken, body.CaptchaAnswer) {
		writeError(w, http.StatusBadRequest, "invalid_captcha", "验证码不正确")
		return
	}
	user, _, err := r.store.FindUserByUsername(req.Context(), username)
	if err != nil || user.Disabled || user.Source != "local" {
		writeError(w, http.StatusBadRequest, "password_reset_unavailable", "当前账号无法找回密码")
		return
	}
	channels, err := r.passwordResetChannels(req)
	if err != nil {
		r.logger.Error("list password reset channels failed", "error", err)
		writeError(w, http.StatusInternalServerError, "list_reset_channels_failed", "读取找回密码媒介失败")
		return
	}
	if len(channels) == 0 {
		writeError(w, http.StatusServiceUnavailable, "no_password_reset_channel", "当前没有可用的找回密码媒介")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"channels": channels, "verificationToken": r.signResetVerification(username, time.Now().UTC().Add(passwordResetVerifyTTL))})
}

func (r *router) handlePasswordResetSendCode(w http.ResponseWriter, req *http.Request) {
	settings := r.passwordResetRuntimeSettings(req.Context())
	defer req.Body.Close()
	var body passwordResetSendCodeRequest
	if err := decodeJSONBody(w, req, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "请求参数格式不正确")
		return
	}
	username := strings.TrimSpace(body.Username)
	channelID := strings.TrimSpace(body.Channel)
	contact := strings.TrimSpace(body.To)
	verifyEmail := strings.TrimSpace(body.VerifyEmail)
	if username == "" || channelID == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "用户名和通知媒介不能为空")
		return
	}
	if !emailAddressPattern.MatchString(verifyEmail) {
		writeError(w, http.StatusBadRequest, "invalid_verify_email", "请输入有效的验证邮箱")
		return
	}
	if !r.verifyResetVerification(body.VerificationToken, username) {
		writeError(w, http.StatusBadRequest, "invalid_verification", "请先完成用户名和图形验证码校验")
		return
	}
	user, _, err := r.store.FindUserByUsername(req.Context(), username)
	if err != nil || user.Disabled || user.Source != "local" {
		writeError(w, http.StatusBadRequest, "password_reset_unavailable", "当前账号无法找回密码")
		return
	}
	if !strings.EqualFold(strings.TrimSpace(user.Email), verifyEmail) {
		writeError(w, http.StatusBadRequest, "verify_email_mismatch", "验证邮箱与当前账号不匹配")
		return
	}
	channel, err := r.store.GetNotificationChannel(req.Context(), channelID)
	if err != nil || !channel.PasswordResetEnabled || !isPasswordResetChannel(channel.ID) {
		writeError(w, http.StatusBadRequest, "invalid_channel", "通知媒介不可用于找回密码")
		return
	}
	if channel.ID == "email" {
		contact = user.Email
	} else {
		contact = ""
	}
	now := time.Now().UTC()
	lastSentAt, coolingDown, err := r.store.LatestRecentPasswordResetTokenCreatedAt(req.Context(), user.ID, now.Add(-settings.SendCooldown))
	if err != nil {
		r.logger.Error("count password reset cooldown failed", "error", err)
		writeError(w, http.StatusInternalServerError, "password_reset_failed", "发送验证码失败")
		return
	}
	if coolingDown {
		remainingSeconds := int(math.Ceil(lastSentAt.Add(settings.SendCooldown).Sub(now).Seconds()))
		if remainingSeconds < 1 {
			remainingSeconds = 1
		}
		writeError(w, http.StatusTooManyRequests, "password_reset_cooldown", fmt.Sprintf("验证码已发送，请于 %d 秒后再试", remainingSeconds))
		return
	}
	recent, err := r.store.CountRecentPasswordResetTokens(req.Context(), user.ID, now.Add(-settings.RateLimitSpan))
	if err != nil {
		r.logger.Error("count password reset tokens failed", "error", err)
		writeError(w, http.StatusInternalServerError, "password_reset_failed", "发送验证码失败")
		return
	}
	if recent >= settings.RateLimitMax {
		writeError(w, http.StatusTooManyRequests, "password_reset_limited", fmt.Sprintf("验证码请求过于频繁，请在 %d 分钟后重试", durationMinutes(settings.RateLimitSpan)))
		return
	}
	code, err := randomDigits(6)
	if err != nil {
		r.logger.Error("generate password reset code failed", "error", err)
		writeError(w, http.StatusInternalServerError, "password_reset_failed", "发送验证码失败")
		return
	}
	expiresAt := time.Now().UTC().Add(settings.CodeTTL)
	if _, err := r.store.CreatePasswordResetToken(req.Context(), user.ID, channel.ID, contact, code, repository.ClientIP(req), expiresAt); err != nil {
		r.logger.Error("create password reset token failed", "error", err)
		writeError(w, http.StatusInternalServerError, "password_reset_failed", "发送验证码失败")
		return
	}
	err = r.notify.SendPasswordReset(req.Context(), channel, notification.PasswordResetMessage{Username: user.Username, Code: code, ExpiresAt: expiresAt, RequestIP: repository.ClientIP(req), To: contact})
	if err != nil {
		r.logger.Warn("send password reset notification failed", "channel", channel.ID, "error", err)
		writeError(w, http.StatusServiceUnavailable, "password_reset_send_failed", notification.UserFacingErrorMessage(err))
		return
	}
	_ = r.store.WriteAudit(req.Context(), user.ID, "auth.password.reset.request", "user", user.ID, repository.ClientIP(req), map[string]any{"username": user.Username, "channel": channel.ID})
	writeJSON(w, http.StatusOK, map[string]any{"message": "验证码已发送", "cooldownSeconds": int(settings.SendCooldown.Seconds()), "expiresAt": expiresAt.Format(time.RFC3339)})
}

func (r *router) passwordResetRuntimeSettings(ctx context.Context) passwordResetRuntimeSettings {
	settings := passwordResetRuntimeSettings{
		CodeTTL:       defaultPasswordResetCodeTTL,
		CaptchaTTL:    defaultPasswordResetCaptchaTTL,
		SendCooldown:  defaultPasswordResetSendCooldown,
		RateLimitMax:  passwordResetRateLimitMax,
		RateLimitSpan: defaultPasswordResetRateLimitSpan,
	}
	if r == nil || r.store == nil {
		return settings
	}
	config, err := r.store.GetSystemBaseConfig(ctx)
	if err != nil {
		if r.logger != nil {
			r.logger.Warn("load password reset runtime settings failed", "error", err)
		}
		return settings
	}
	settings.CodeTTL = intMinutesAsDuration(config.PasswordResetCodeTTLMinutes, defaultPasswordResetCodeTTL)
	settings.CaptchaTTL = intMinutesAsDuration(config.PasswordResetCaptchaTTLMinutes, defaultPasswordResetCaptchaTTL)
	settings.SendCooldown = minutesAsDuration(config.PasswordResetSendCooldownMinutes, defaultPasswordResetSendCooldown)
	settings.RateLimitSpan = intMinutesAsDuration(config.PasswordResetRateLimitMinutes, defaultPasswordResetRateLimitSpan)
	return settings
}

func (r *router) handlePasswordResetConfirm(w http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()
	var body passwordResetConfirmRequest
	if err := decodeJSONBody(w, req, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "请求参数格式不正确")
		return
	}
	username := strings.TrimSpace(body.Username)
	code := strings.TrimSpace(body.Code)
	if username == "" || code == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "用户名和验证码不能为空")
		return
	}
	if len(body.NewPassword) < 6 || len(body.ConfirmPassword) < 6 {
		writeError(w, http.StatusBadRequest, "invalid_password", "密码至少 6 个字符")
		return
	}
	if body.NewPassword != body.ConfirmPassword {
		writeError(w, http.StatusBadRequest, "password_mismatch", "新密码与确认密码不一致")
		return
	}
	token, user, err := r.store.FindUsablePasswordResetToken(req.Context(), username, code)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeError(w, http.StatusBadRequest, "invalid_reset_code", "验证码不正确或已过期")
			return
		}
		r.logger.Error("find password reset token failed", "error", err)
		writeError(w, http.StatusInternalServerError, "password_reset_failed", "密码重置失败")
		return
	}
	if user.Disabled || user.Source != "local" {
		writeError(w, http.StatusBadRequest, "password_reset_unavailable", "当前账号无法找回密码")
		return
	}
	hash, err := auth.HashPassword(body.NewPassword)
	if err != nil {
		r.logger.Error("hash reset password failed", "error", err)
		writeError(w, http.StatusInternalServerError, "password_reset_failed", "密码重置失败")
		return
	}
	if err := r.store.UpdateUserPassword(req.Context(), user.ID, hash); err != nil {
		r.logger.Error("update reset password failed", "error", err)
		writeError(w, http.StatusInternalServerError, "password_reset_failed", "密码重置失败")
		return
	}
	if err := r.store.MarkPasswordResetTokenUsed(req.Context(), token.ID); err != nil {
		r.logger.Warn("mark password reset token used failed", "error", err)
	}
	if err := r.store.DeleteUserSessions(req.Context(), user.ID); err != nil {
		r.logger.Warn("delete sessions after password reset failed", "error", err)
	}
	_ = r.store.WriteAudit(req.Context(), user.ID, "auth.password.reset.confirm", "user", user.ID, repository.ClientIP(req), map[string]any{"username": user.Username, "channel": token.ChannelID})
	writeJSON(w, http.StatusOK, map[string]string{"message": "密码已重置"})
}

func (r *router) passwordResetChannels(req *http.Request) ([]domain.PublicPasswordResetChannel, error) {
	items, err := r.store.ListNotificationChannels(req.Context())
	if err != nil {
		return nil, err
	}
	channels := make([]domain.PublicPasswordResetChannel, 0)
	for _, item := range items {
		if !item.PasswordResetEnabled || !isPasswordResetChannel(item.ID) {
			continue
		}
		channels = append(channels, passwordResetChannelMeta(item.ID))
	}
	return channels, nil
}

func isPasswordResetChannel(id string) bool {
	return id == "email"
}

func passwordResetChannelMeta(id string) domain.PublicPasswordResetChannel {
	switch id {
	case "email":
		return domain.PublicPasswordResetChannel{ID: id, Name: "邮箱", Description: "发送找回密码验证码到账号配置邮箱", RequiresTo: true}
	default:
		return domain.PublicPasswordResetChannel{}
	}
}

func (r *router) signCaptcha(answer int, expiresAt time.Time) string {
	payload := fmt.Sprintf("%d:%d", answer, expiresAt.Unix())
	mac := hmac.New(sha256.New, []byte(r.cfg.JWT.Secret))
	_, _ = mac.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString([]byte(payload + ":" + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))))
}

func (r *router) verifyCaptcha(token, answerText string) bool {
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(token))
	if err != nil {
		return false
	}
	parts := strings.Split(string(raw), ":")
	if len(parts) != 3 {
		return false
	}
	expiresUnix, err := parseInt64(parts[1])
	if err != nil || time.Now().UTC().After(time.Unix(expiresUnix, 0)) {
		return false
	}
	payload := parts[0] + ":" + parts[1]
	mac := hmac.New(sha256.New, []byte(r.cfg.JWT.Secret))
	_, _ = mac.Write([]byte(payload))
	if !hmac.Equal([]byte(parts[2]), []byte(base64.RawURLEncoding.EncodeToString(mac.Sum(nil)))) {
		return false
	}
	return strings.TrimSpace(answerText) == parts[0]
}

func (r *router) signResetVerification(username string, expiresAt time.Time) string {
	normalized := strings.TrimSpace(username)
	payload := fmt.Sprintf("%s:%d", normalized, expiresAt.Unix())
	mac := hmac.New(sha256.New, []byte(r.cfg.JWT.Secret))
	_, _ = mac.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString([]byte(payload + ":" + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))))
}

func (r *router) verifyResetVerification(token, username string) bool {
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(token))
	if err != nil {
		return false
	}
	parts := strings.Split(string(raw), ":")
	if len(parts) != 3 {
		return false
	}
	expiresUnix, err := parseInt64(parts[1])
	if err != nil || time.Now().UTC().After(time.Unix(expiresUnix, 0)) {
		return false
	}
	if parts[0] != strings.TrimSpace(username) {
		return false
	}
	payload := parts[0] + ":" + parts[1]
	mac := hmac.New(sha256.New, []byte(r.cfg.JWT.Secret))
	_, _ = mac.Write([]byte(payload))
	return hmac.Equal([]byte(parts[2]), []byte(base64.RawURLEncoding.EncodeToString(mac.Sum(nil))))
}

func randomInt(min, max int64) (int, error) {
	if max < min {
		return 0, fmt.Errorf("invalid random range")
	}
	value, err := rand.Int(rand.Reader, big.NewInt(max-min+1))
	if err != nil {
		return 0, err
	}
	return int(value.Int64() + min), nil
}

type captchaChallenge struct {
	Question string
	Answer   int
}

func randomCaptchaChallenge() (captchaChallenge, error) {
	operatorIndex, err := randomInt(0, 2)
	if err != nil {
		return captchaChallenge{}, err
	}
	left, err := randomInt(2, 9)
	if err != nil {
		return captchaChallenge{}, err
	}
	right, err := randomInt(2, 9)
	if err != nil {
		return captchaChallenge{}, err
	}
	switch operatorIndex {
	case 0:
		return captchaChallenge{Question: fmt.Sprintf("%d + %d", left, right), Answer: left + right}, nil
	case 1:
		if left < right {
			left, right = right, left
		}
		return captchaChallenge{Question: fmt.Sprintf("%d - %d", left, right), Answer: left - right}, nil
	default:
		return captchaChallenge{Question: fmt.Sprintf("%d × %d", left, right), Answer: left * right}, nil
	}
}

func randomDigits(length int) (string, error) {
	if length <= 0 {
		return "", fmt.Errorf("invalid code length")
	}
	var builder strings.Builder
	for builder.Len() < length {
		value, err := rand.Int(rand.Reader, big.NewInt(10))
		if err != nil {
			return "", err
		}
		builder.WriteByte(byte('0' + value.Int64()))
	}
	return builder.String(), nil
}

func parseInt64(value string) (int64, error) {
	return strconv.ParseInt(value, 10, 64)
}

func intMinutesAsDuration(minutes int, fallback time.Duration) time.Duration {
	if minutes <= 0 {
		return fallback
	}
	return time.Duration(minutes) * time.Minute
}

func minutesAsDuration(minutes float64, fallback time.Duration) time.Duration {
	if minutes <= 0 {
		return fallback
	}
	return time.Duration(minutes * float64(time.Minute))
}

func durationMinutes(value time.Duration) int {
	minutes := int(value / time.Minute)
	if minutes < 1 {
		return 1
	}
	return minutes
}
