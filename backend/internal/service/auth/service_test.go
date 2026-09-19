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

	provider    domain.AuthProvider
	user        domain.User
	userByID    domain.User
	createdUser string
	states      map[string]domain.AuthState
	bindings    map[string]string
	baseConfig  domain.SystemBaseConfig
}

func (s *fakeStore) FindUserByUsername(_ context.Context, username string) (domain.User, string, error) {
	if s.user.Username != "" && s.user.Username == username {
		return s.user, "password-hash", nil
	}
	return domain.User{}, "", errors.New("not found")
}

func (s *fakeStore) FindUserByID(_ context.Context, id string) (domain.User, error) {
	if s.userByID.ID != "" && s.userByID.ID == id {
		return s.userByID, nil
	}
	return domain.User{}, errors.New("not found")
}

func (s *fakeStore) FindUserByWecomAccount(_ context.Context, userid string) (domain.User, error) {
	if ownerID, ok := s.bindings[userid]; ok && ownerID == s.user.ID {
		return s.user, nil
	}
	return domain.User{}, errors.New("not found")
}

func (s *fakeStore) BindWecomAccount(_ context.Context, userID, userid string) error {
	if ownerID, ok := s.bindings[userid]; ok && ownerID != userID {
		return ErrWecomAlreadyBound
	}
	if s.bindings == nil {
		s.bindings = map[string]string{}
	}
	for account, owner := range s.bindings {
		if owner == userID {
			delete(s.bindings, account)
		}
	}
	s.bindings[userid] = userID
	return nil
}

func (s *fakeStore) UnbindWecomAccount(_ context.Context, userID string) (string, error) {
	for account, owner := range s.bindings {
		if owner == userID {
			delete(s.bindings, account)
			return account, nil
		}
	}
	return "", nil
}

func (s *fakeStore) UserHasWecomBinding(_ context.Context, userID string) (bool, error) {
	for _, owner := range s.bindings {
		if owner == userID {
			return true, nil
		}
	}
	return false, nil
}

func (s *fakeStore) GetSystemBaseConfig(context.Context) (domain.SystemBaseConfig, error) {
	return s.baseConfig, nil
}

func (s *fakeStore) UpsertUser(context.Context, string, string, string, string) (domain.User, error) {
	return domain.User{}, errors.New("not implemented")
}

func (s *fakeStore) RecordUserLogin(_ context.Context, userID string) error {
	s.createdUser = userID
	return nil
}

func (s *fakeStore) CreateSession(_ context.Context, token, _ string, _ time.Time) error {
	s.session = domain.Session{Token: token}
	return nil
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

func (s *fakeStore) GetAuthProvider(_ context.Context, _ string) (domain.AuthProvider, error) {
	if s.provider.ID == "" {
		return domain.AuthProvider{}, errors.New("not found")
	}
	return s.provider, nil
}

func (s *fakeStore) CreateAuthState(_ context.Context, item domain.AuthState) error {
	if s.states == nil {
		s.states = map[string]domain.AuthState{}
	}
	s.states[item.State] = item
	return nil
}

func (s *fakeStore) TakeAuthState(_ context.Context, state string) (domain.AuthState, error) {
	item, ok := s.states[state]
	if !ok {
		return domain.AuthState{}, errors.New("not found")
	}
	delete(s.states, state)
	return item, nil
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
