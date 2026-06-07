package router

import (
	"math"
	"net/http"
	"strings"

	"kvm-manager/backend/internal/domain"
	"kvm-manager/backend/internal/repository"
)

type systemBaseConfigRequest struct {
	SiteName                          string  `json:"siteName"`
	LoginName                         string  `json:"loginName"`
	AppName                           string  `json:"appName"`
	AppSubtitle                       string  `json:"appSubtitle"`
	IconData                          string  `json:"iconData"`
	PasswordResetCodeTTLMinutes       int     `json:"passwordResetCodeTtlMinutes"`
	PasswordResetCaptchaTTLMinutes    int     `json:"passwordResetCaptchaTtlMinutes"`
	PasswordResetSendCooldownMinutes  float64 `json:"passwordResetSendCooldownMinutes"`
	PasswordResetRateLimitMinutes     int     `json:"passwordResetRateLimitMinutes"`
	ResourceWarningThreshold          int     `json:"resourceWarningThreshold"`
	ResourceCriticalThreshold         int     `json:"resourceCriticalThreshold"`
	ResourceAlertConsecutiveCount     int     `json:"resourceAlertConsecutiveCount"`
	AgentOfflineFailureCount          int     `json:"agentOfflineFailureCount"`
	AlertNotificationTimeoutSeconds   int     `json:"alertNotificationTimeoutSeconds"`
	AlertNotificationMaxRetryCount    *int    `json:"alertNotificationMaxRetryCount"`
	AlertNotificationRetryBaseSeconds int     `json:"alertNotificationRetryBaseSeconds"`
	AlertNotificationRetryMaxMinutes  int     `json:"alertNotificationRetryMaxMinutes"`
	AlertNotificationBatchSize        int     `json:"alertNotificationBatchSize"`
}

func (r *router) handlePublicSystemBaseConfig(w http.ResponseWriter, req *http.Request) {
	config, err := r.store.GetSystemBaseConfig(req.Context())
	if err != nil {
		writeJSON(w, http.StatusOK, defaultSystemBaseConfig())
		return
	}
	writeJSON(w, http.StatusOK, config)
}

func defaultSystemBaseConfig() domain.SystemBaseConfig {
	return domain.SystemBaseConfig{
		SiteName:                          "KVM Manager",
		LoginName:                         "KVM Manager",
		AppName:                           "KVM Manager",
		AppSubtitle:                       "VIRTUALIZATION OPS",
		IconData:                          "/favicon.svg",
		PasswordResetCodeTTLMinutes:       10,
		PasswordResetCaptchaTTLMinutes:    1,
		PasswordResetSendCooldownMinutes:  0.5,
		PasswordResetRateLimitMinutes:     5,
		ResourceWarningThreshold:          70,
		ResourceCriticalThreshold:         85,
		ResourceAlertConsecutiveCount:     3,
		AgentOfflineFailureCount:          3,
		AlertNotificationTimeoutSeconds:   8,
		AlertNotificationMaxRetryCount:    6,
		AlertNotificationRetryBaseSeconds: 30,
		AlertNotificationRetryMaxMinutes:  15,
		AlertNotificationBatchSize:        50,
	}
}

func (r *router) handleGetSystemBaseConfig(w http.ResponseWriter, req *http.Request) {
	config, err := r.store.GetSystemBaseConfig(req.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "get_base_config_failed", "读取基础配置失败")
		return
	}
	writeJSON(w, http.StatusOK, config)
}

func (r *router) handleUpdateSystemBaseConfig(w http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()
	var body systemBaseConfigRequest
	if err := decodeJSONBodyLimit(w, req, 2<<20, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "基础配置格式不正确")
		return
	}
	payload, ok := sanitizeSystemBaseConfig(w, body)
	if !ok {
		return
	}
	config, err := r.store.UpsertSystemBaseConfig(req.Context(), payload)
	if err != nil {
		r.logger.Error("save system base config failed", "error", err)
		writeError(w, http.StatusInternalServerError, "save_base_config_failed", "保存基础配置失败")
		return
	}
	_ = r.store.WriteAudit(req.Context(), currentSession(req).User.ID, "settings.base.update", "system_setting", "base_config", repository.ClientIP(req), map[string]any{"siteName": config.SiteName})
	writeJSON(w, http.StatusOK, config)
}

func sanitizeSystemBaseConfig(w http.ResponseWriter, body systemBaseConfigRequest) (map[string]any, bool) {
	siteName := strings.TrimSpace(body.SiteName)
	loginName := strings.TrimSpace(body.LoginName)
	appName := strings.TrimSpace(body.AppName)
	appSubtitle := strings.TrimSpace(body.AppSubtitle)
	iconData := strings.TrimSpace(body.IconData)
	if siteName == "" || loginName == "" || appName == "" || appSubtitle == "" {
		writeError(w, http.StatusBadRequest, "invalid_base_config", "名称配置不能为空")
		return nil, false
	}
	if len([]rune(siteName)) > 60 || len([]rune(loginName)) > 60 || len([]rune(appName)) > 60 || len([]rune(appSubtitle)) > 60 {
		writeError(w, http.StatusBadRequest, "invalid_base_config", "名称长度不能超过 60 个字符")
		return nil, false
	}
	if iconData == "" {
		iconData = "/favicon.svg"
	}
	if len(iconData) > 512*1024 {
		writeError(w, http.StatusBadRequest, "invalid_base_config", "图标文件不能超过 512KB")
		return nil, false
	}
	if !strings.HasPrefix(iconData, "data:image/") && !strings.HasPrefix(iconData, "/") {
		writeError(w, http.StatusBadRequest, "invalid_base_config", "图标必须是站内路径或图片 Data URL")
		return nil, false
	}
	passwordResetCodeTTLMinutes := positiveOrDefault(body.PasswordResetCodeTTLMinutes, 10)
	passwordResetCaptchaTTLMinutes := positiveOrDefault(body.PasswordResetCaptchaTTLMinutes, 1)
	passwordResetSendCooldownMinutes := positiveFloatOrDefault(body.PasswordResetSendCooldownMinutes, 0.5)
	passwordResetRateLimitMinutes := positiveOrDefault(body.PasswordResetRateLimitMinutes, 5)
	resourceWarningThreshold := positiveOrDefault(body.ResourceWarningThreshold, 70)
	resourceCriticalThreshold := positiveOrDefault(body.ResourceCriticalThreshold, 85)
	resourceAlertConsecutiveCount := positiveOrDefault(body.ResourceAlertConsecutiveCount, 3)
	agentOfflineFailureCount := positiveOrDefault(body.AgentOfflineFailureCount, 3)
	alertNotificationTimeoutSeconds := positiveOrDefault(body.AlertNotificationTimeoutSeconds, 8)
	alertNotificationMaxRetryCount := nonNegativePointerOrDefault(body.AlertNotificationMaxRetryCount, 6)
	alertNotificationRetryBaseSeconds := positiveOrDefault(body.AlertNotificationRetryBaseSeconds, 30)
	alertNotificationRetryMaxMinutes := positiveOrDefault(body.AlertNotificationRetryMaxMinutes, 15)
	alertNotificationBatchSize := positiveOrDefault(body.AlertNotificationBatchSize, 50)
	if passwordResetCodeTTLMinutes < 1 || passwordResetCodeTTLMinutes > 60 {
		writeError(w, http.StatusBadRequest, "invalid_base_config", "找回密码验证码有效期需在 1 到 60 分钟之间")
		return nil, false
	}
	if passwordResetCaptchaTTLMinutes < 1 || passwordResetCaptchaTTLMinutes > 10 {
		writeError(w, http.StatusBadRequest, "invalid_base_config", "图形验证码有效期需在 1 到 10 分钟之间")
		return nil, false
	}
	if passwordResetSendCooldownMinutes < 0.5 || passwordResetSendCooldownMinutes > 10 || !isHalfMinuteStep(passwordResetSendCooldownMinutes) {
		writeError(w, http.StatusBadRequest, "invalid_base_config", "找回密码发送冷却需在 0.5 到 10 分钟之间，且按 0.5 分钟递增")
		return nil, false
	}
	if passwordResetRateLimitMinutes < 5 || passwordResetRateLimitMinutes > 10 {
		writeError(w, http.StatusBadRequest, "invalid_base_config", "找回密码频率窗口需在 5 到 10 分钟之间")
		return nil, false
	}
	if resourceWarningThreshold < 1 || resourceWarningThreshold > 99 || resourceCriticalThreshold < 2 || resourceCriticalThreshold > 100 || resourceWarningThreshold >= resourceCriticalThreshold {
		writeError(w, http.StatusBadRequest, "invalid_base_config", "资源阈值需满足 1 <= 警告阈值 < 严重阈值 <= 100")
		return nil, false
	}
	if resourceAlertConsecutiveCount < 1 || resourceAlertConsecutiveCount > 20 {
		writeError(w, http.StatusBadRequest, "invalid_base_config", "资源告警连续次数需在 1 到 20 次之间")
		return nil, false
	}
	if agentOfflineFailureCount < 1 || agentOfflineFailureCount > 20 {
		writeError(w, http.StatusBadRequest, "invalid_base_config", "Agent 离线失败次数需在 1 到 20 次之间")
		return nil, false
	}
	if alertNotificationTimeoutSeconds < 3 || alertNotificationTimeoutSeconds > 60 {
		writeError(w, http.StatusBadRequest, "invalid_base_config", "告警通知发送超时需在 3 到 60 秒之间")
		return nil, false
	}
	if alertNotificationMaxRetryCount < 0 || alertNotificationMaxRetryCount > 10 {
		writeError(w, http.StatusBadRequest, "invalid_base_config", "告警通知最大重试次数需在 0 到 10 次之间")
		return nil, false
	}
	if alertNotificationRetryBaseSeconds < 10 || alertNotificationRetryBaseSeconds > 300 {
		writeError(w, http.StatusBadRequest, "invalid_base_config", "告警通知重试基础间隔需在 10 到 300 秒之间")
		return nil, false
	}
	if alertNotificationRetryMaxMinutes < 1 || alertNotificationRetryMaxMinutes > 120 {
		writeError(w, http.StatusBadRequest, "invalid_base_config", "告警通知重试最大间隔需在 1 到 120 分钟之间")
		return nil, false
	}
	if alertNotificationRetryMaxMinutes*60 < alertNotificationRetryBaseSeconds {
		writeError(w, http.StatusBadRequest, "invalid_base_config", "告警通知重试最大间隔不能小于基础间隔")
		return nil, false
	}
	if alertNotificationBatchSize < 10 || alertNotificationBatchSize > 100 {
		writeError(w, http.StatusBadRequest, "invalid_base_config", "告警通知处理批量需在 10 到 100 条之间")
		return nil, false
	}
	return map[string]any{
		"siteName":                          siteName,
		"loginName":                         loginName,
		"appName":                           appName,
		"appSubtitle":                       appSubtitle,
		"iconData":                          iconData,
		"passwordResetCodeTtlMinutes":       passwordResetCodeTTLMinutes,
		"passwordResetCaptchaTtlMinutes":    passwordResetCaptchaTTLMinutes,
		"passwordResetSendCooldownMinutes":  passwordResetSendCooldownMinutes,
		"passwordResetRateLimitMinutes":     passwordResetRateLimitMinutes,
		"resourceWarningThreshold":          resourceWarningThreshold,
		"resourceCriticalThreshold":         resourceCriticalThreshold,
		"resourceAlertConsecutiveCount":     resourceAlertConsecutiveCount,
		"agentOfflineFailureCount":          agentOfflineFailureCount,
		"alertNotificationTimeoutSeconds":   alertNotificationTimeoutSeconds,
		"alertNotificationMaxRetryCount":    alertNotificationMaxRetryCount,
		"alertNotificationRetryBaseSeconds": alertNotificationRetryBaseSeconds,
		"alertNotificationRetryMaxMinutes":  alertNotificationRetryMaxMinutes,
		"alertNotificationBatchSize":        alertNotificationBatchSize,
	}, true
}

func positiveOrDefault(value int, fallback int) int {
	if value <= 0 {
		return fallback
	}
	return value
}

func nonNegativePointerOrDefault(value *int, fallback int) int {
	if value == nil || *value < 0 {
		return fallback
	}
	return *value
}

func positiveFloatOrDefault(value float64, fallback float64) float64 {
	if value <= 0 {
		return fallback
	}
	return value
}

func isHalfMinuteStep(value float64) bool {
	steps := value * 2
	return math.Abs(steps-math.Round(steps)) < 0.000001
}
