-- 企业微信认证统一为单提供方：直连与统一认证中心合并为 wecom 行，config.authMode 切换。
UPDATE auth_providers SET name = '企业微信' WHERE id = 'wecom';

-- 旧直连字段映射到新命名（未配置过的空行跳过，保持 '{}'）
UPDATE auth_providers
SET config = jsonb_build_object(
    'authMode', 'direct',
    'corpid', config ->> 'corpId',
    'agentid', config -> 'agentId',
    'secret', config ->> 'secret',
    'redirectPrefix', COALESCE(config ->> 'externalUrl', '')
)
WHERE id = 'wecom'
  AND config <> '{}' :: jsonb
  AND NOT (config ? 'authMode');

-- 认证中心已启用时整体切换为 sso 模式并吸收其配置
UPDATE auth_providers w
SET config = jsonb_build_object(
    'authMode', 'sso',
    'ssoBaseUrl', c.config ->> 'baseUrl',
    'ssoAppID', c.config ->> 'app',
    'ssoAppSecret', c.config ->> 'appSecret'
),
enabled = TRUE
FROM auth_providers c
WHERE w.id = 'wecom'
  AND c.id = 'wecom_center'
  AND c.enabled;

DELETE FROM auth_providers WHERE id = 'wecom_center';
