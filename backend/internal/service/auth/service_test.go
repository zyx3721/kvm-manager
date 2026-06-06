package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"kvm-manager/backend/internal/domain"
)

type fakeStore struct {
	session domain.Session
	touched bool
	touchAt time.Time
}

func (s *fakeStore) FindUserByUsername(context.Context, string) (domain.User, string, error) {
	return domain.User{}, "", errors.New("not implemented")
}

func (s *fakeStore) UpsertUser(context.Context, string, string, string, string) (domain.User, error) {
	return domain.User{}, errors.New("not implemented")
}

func (s *fakeStore) RecordUserLogin(context.Context, string) error {
	return errors.New("not implemented")
}

func (s *fakeStore) CreateSession(context.Context, string, string, time.Time) error {
	return errors.New("not implemented")
}

func (s *fakeStore) FindSession(context.Context, string) (domain.Session, error) {
	if s.session.Token == "" {
		return domain.Session{}, errors.New("not found")
	}
	return s.session, nil
}

func (s *fakeStore) TouchSession(_ context.Context, _ string, seenAt time.Time) error {
	s.touched = true
	s.touchAt = seenAt
	return nil
}

func (s *fakeStore) DeleteSession(context.Context, string) error {
	return nil
}

func (s *fakeStore) DeleteExpiredSessions(context.Context) error {
	return nil
}

func (s *fakeStore) GetAuthProvider(context.Context, string) (domain.AuthProvider, error) {
	return domain.AuthProvider{}, errors.New("not implemented")
}

func TestValidateAllowsSessionWithinIdleTTL(t *testing.T) {
	now := time.Date(2026, 6, 5, 10, 0, 0, 0, time.UTC)
	store := &fakeStore{
		session: domain.Session{
			Token:      "token",
			ExpiresAt:  now.Add(time.Hour),
			LastSeenAt: now.Add(-11 * time.Hour),
			User:       domain.User{ID: "user-1", Username: "admin"},
		},
	}
	service := NewServiceWithIdleTTL(store, 24*time.Hour, 12*time.Hour)
	service.now = func() time.Time { return now }

	if _, err := service.Validate(t.Context(), "token"); err != nil {
		t.Fatalf("validate session: %v", err)
	}
}

func TestValidateRejectsSessionAfterIdleTTL(t *testing.T) {
	now := time.Date(2026, 6, 5, 10, 0, 0, 0, time.UTC)
	store := &fakeStore{
		session: domain.Session{
			Token:      "token",
			ExpiresAt:  now.Add(time.Hour),
			LastSeenAt: now.Add(-13 * time.Hour),
			User:       domain.User{ID: "user-1", Username: "admin"},
		},
	}
	service := NewServiceWithIdleTTL(store, 24*time.Hour, 12*time.Hour)
	service.now = func() time.Time { return now }

	if _, err := service.Validate(t.Context(), "token"); !errors.Is(err, ErrInvalidSession) {
		t.Fatalf("expected invalid session, got %v", err)
	}
}

func TestValidateTouchesStaleSession(t *testing.T) {
	now := time.Date(2026, 6, 5, 10, 0, 0, 0, time.UTC)
	store := &fakeStore{
		session: domain.Session{
			Token:      "token",
			ExpiresAt:  now.Add(time.Hour),
			LastSeenAt: now.Add(-6 * time.Minute),
			User:       domain.User{ID: "user-1", Username: "admin"},
		},
	}
	service := NewServiceWithIdleTTL(store, 24*time.Hour, 12*time.Hour)
	service.now = func() time.Time { return now }

	session, err := service.Validate(t.Context(), "token")
	if err != nil {
		t.Fatalf("validate session: %v", err)
	}
	if !store.touched {
		t.Fatal("expected stale session to be touched")
	}
	if !store.touchAt.Equal(now) {
		t.Fatalf("touchAt = %s, want %s", store.touchAt, now)
	}
	if !session.LastSeenAt.Equal(now) {
		t.Fatalf("session LastSeenAt = %s, want %s", session.LastSeenAt, now)
	}
}
