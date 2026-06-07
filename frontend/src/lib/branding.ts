import { useEffect, useState } from 'react';

import { fetchPublicSystemBaseConfig, type SystemBaseConfig } from './api';

export const defaultBaseConfig: SystemBaseConfig = {
  siteName: 'KVM Manager',
  loginName: 'KVM Manager',
  appName: 'KVM Manager',
  appSubtitle: 'VIRTUALIZATION OPS',
  iconData: '/favicon.svg',
  passwordResetCodeTtlMinutes: 10,
  passwordResetCaptchaTtlMinutes: 1,
  passwordResetSendCooldownMinutes: 0.5,
  passwordResetRateLimitMinutes: 5,
  resourceWarningThreshold: 70,
  resourceCriticalThreshold: 85,
  resourceAlertConsecutiveCount: 3,
  agentOfflineFailureCount: 3,
  alertNotificationTimeoutSeconds: 8,
  alertNotificationMaxRetryCount: 6,
  alertNotificationRetryBaseSeconds: 30,
  alertNotificationRetryMaxMinutes: 15,
  alertNotificationBatchSize: 50,
};

let cachedBaseConfig: SystemBaseConfig = defaultBaseConfig;
const listeners = new Set<(config: SystemBaseConfig) => void>();

export function getBaseConfigSnapshot() {
  return cachedBaseConfig;
}

export function setBaseConfigSnapshot(config: SystemBaseConfig) {
  cachedBaseConfig = normalizeBaseConfig(config);
  applyDocumentBranding(cachedBaseConfig);
  listeners.forEach(listener => listener(cachedBaseConfig));
}

export function useBaseConfig() {
  const [config, setConfig] = useState<SystemBaseConfig>(cachedBaseConfig);

  useEffect(() => {
    listeners.add(setConfig);
    return () => {
      listeners.delete(setConfig);
    };
  }, []);

  useEffect(() => {
    let cancelled = false;
    void fetchPublicSystemBaseConfig()
      .then(next => {
        if (!cancelled) setBaseConfigSnapshot(next);
      })
      .catch(() => {
        if (!cancelled) setBaseConfigSnapshot(defaultBaseConfig);
      });
    return () => {
      cancelled = true;
    };
  }, []);

  return config;
}

export function normalizeBaseConfig(config: Partial<SystemBaseConfig>): SystemBaseConfig {
  return {
    siteName: config.siteName?.trim() || defaultBaseConfig.siteName,
    loginName: config.loginName?.trim() || defaultBaseConfig.loginName,
    appName: config.appName?.trim() || defaultBaseConfig.appName,
    appSubtitle: config.appSubtitle?.trim() || defaultBaseConfig.appSubtitle,
    iconData: config.iconData?.trim() || defaultBaseConfig.iconData,
    passwordResetCodeTtlMinutes: normalizeNumber(
      config.passwordResetCodeTtlMinutes,
      defaultBaseConfig.passwordResetCodeTtlMinutes
    ),
    passwordResetCaptchaTtlMinutes: normalizeNumber(
      config.passwordResetCaptchaTtlMinutes,
      defaultBaseConfig.passwordResetCaptchaTtlMinutes
    ),
    passwordResetSendCooldownMinutes: normalizeNumber(
      config.passwordResetSendCooldownMinutes,
      defaultBaseConfig.passwordResetSendCooldownMinutes
    ),
    passwordResetRateLimitMinutes: normalizeNumber(
      config.passwordResetRateLimitMinutes,
      defaultBaseConfig.passwordResetRateLimitMinutes
    ),
    resourceWarningThreshold: normalizeNumber(
      config.resourceWarningThreshold,
      defaultBaseConfig.resourceWarningThreshold
    ),
    resourceCriticalThreshold: normalizeNumber(
      config.resourceCriticalThreshold,
      defaultBaseConfig.resourceCriticalThreshold
    ),
    resourceAlertConsecutiveCount: normalizeNumber(
      config.resourceAlertConsecutiveCount,
      defaultBaseConfig.resourceAlertConsecutiveCount
    ),
    agentOfflineFailureCount: normalizeNumber(
      config.agentOfflineFailureCount,
      defaultBaseConfig.agentOfflineFailureCount
    ),
    alertNotificationTimeoutSeconds: normalizeNumber(
      config.alertNotificationTimeoutSeconds,
      defaultBaseConfig.alertNotificationTimeoutSeconds
    ),
    alertNotificationMaxRetryCount: normalizeNumberAllowZero(
      config.alertNotificationMaxRetryCount,
      defaultBaseConfig.alertNotificationMaxRetryCount
    ),
    alertNotificationRetryBaseSeconds: normalizeNumber(
      config.alertNotificationRetryBaseSeconds,
      defaultBaseConfig.alertNotificationRetryBaseSeconds
    ),
    alertNotificationRetryMaxMinutes: normalizeNumber(
      config.alertNotificationRetryMaxMinutes,
      defaultBaseConfig.alertNotificationRetryMaxMinutes
    ),
    alertNotificationBatchSize: normalizeNumber(
      config.alertNotificationBatchSize,
      defaultBaseConfig.alertNotificationBatchSize
    ),
    created_at: config.created_at,
    updated_at: config.updated_at,
  };
}

export function applyDocumentBranding(config: SystemBaseConfig) {
  if (typeof document === 'undefined') return;
  document.title = config.siteName;
  const icon = document.querySelector<HTMLLinkElement>("link[rel='icon']");
  if (icon) icon.href = config.iconData || defaultBaseConfig.iconData;
}

function normalizeNumber(value: unknown, fallback: number) {
  const numberValue = Number(value);
  return Number.isFinite(numberValue) && numberValue > 0 ? numberValue : fallback;
}

function normalizeNumberAllowZero(value: unknown, fallback: number) {
  const numberValue = Number(value);
  return Number.isFinite(numberValue) && numberValue >= 0 ? numberValue : fallback;
}
