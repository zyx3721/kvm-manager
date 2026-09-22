CREATE TABLE IF NOT EXISTS login_failure_log (
    username TEXT NOT NULL,
    failed_at BIGINT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_login_failure_log_username ON login_failure_log(username);
