package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

var ErrLoginLocked = errors.New("login locked")

// LoginLockedError 登录锁定错误，Minutes 为按最近一次密码失败时间推算的剩余等待分钟数
type LoginLockedError struct {
	Minutes int64
}

func (e LoginLockedError) Error() string {
	return fmt.Sprintf("密码连续错误次数过多，请于 %d 分钟后再试", e.Minutes)
}

func (e LoginLockedError) Is(target error) bool { return target == ErrLoginLocked }

// EnsureLoginAllowed 判定账号密码失败次数达到阈值且仍在锁定时长内时返回 LoginLockedError，admin 不受限
func (s *Service) EnsureLoginAllowed(ctx context.Context, username string) error {
	name := strings.ToLower(strings.TrimSpace(username))
	if name == "" || strings.EqualFold(strings.TrimSpace(username), "admin") {
		return nil
	}
	count, lastFailedAt, err := s.store.CountLoginFailures(ctx, name)
	if err != nil {
		return err
	}
	if count < int64(s.loginMaxFailures) || lastFailedAt <= 0 {
		return nil
	}
	if remaining := lastFailedAt + int64(s.loginLockoutMinutes)*60 - s.now().Unix(); remaining > 0 {
		return LoginLockedError{Minutes: (remaining + 59) / 60}
	}
	return nil
}

// RecordLoginFailure 记录一次密码校验失败（用户名统一小写存储），并顺带清理超过一天的旧记录
func (s *Service) RecordLoginFailure(ctx context.Context, username string) error {
	name := strings.ToLower(strings.TrimSpace(username))
	if name == "" {
		return nil
	}
	return s.store.CreateLoginFailure(ctx, name, s.now().Unix())
}

// ClearLoginFailures 清除账号的失败计数，登录成功或找回密码重置成功后调用，返回实际清除的记录数
func (s *Service) ClearLoginFailures(ctx context.Context, username string) (int64, error) {
	name := strings.ToLower(strings.TrimSpace(username))
	if name == "" {
		return 0, nil
	}
	return s.store.ClearLoginFailures(ctx, name)
}
