UPDATE system_settings
SET value = value || '{"appSubtitle":"VIRTUALIZATION OPS"}'::jsonb,
    updated_at = now()
WHERE key = 'base_config'
  AND NOT (value ? 'appSubtitle');
