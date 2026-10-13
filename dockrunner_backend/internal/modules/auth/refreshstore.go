package auth

import (
	"context"
	"sync"
	"time"
)

type refreshEntry struct {
	jti     string
	expires time.Time
}

type RefreshStore struct {
	mu  sync.Mutex
	ttl time.Duration
	all map[string]refreshEntry
}

func NewRefreshStore(ttl time.Duration) *RefreshStore {
	return &RefreshStore{ttl: ttl, all: map[string]refreshEntry{}}
}

func (s *RefreshStore) Store(_ context.Context, userID, jti string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.all[userID] = refreshEntry{jti: jti, expires: time.Now().Add(s.ttl)}
	return nil
}

func (s *RefreshStore) CheckAndRotate(_ context.Context, userID, oldJTI, newJTI string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	current, ok := s.all[userID]
	if !ok || current.jti != oldJTI || time.Now().After(current.expires) {
		delete(s.all, userID)
		return ErrInvalidCredentials
	}
	s.all[userID] = refreshEntry{jti: newJTI, expires: time.Now().Add(s.ttl)}
	return nil
}

func (s *RefreshStore) Revoke(_ context.Context, userID string) error {
	if userID == "" {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.all, userID)
	return nil
}
