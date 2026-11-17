package main

import (
	"context"
	"errors"
	"sync"
	"time"
)

const (
	kindPendingConsent = "pending_consent"
	kindPendingEntra   = "pending_entra"
	kindCode           = "code"
	kindSession        = "session"
)

var errStoreFull = errors.New("store capacity reached")

// SessionStore persists short-lived authorization state, authorization codes,
// gateway sessions, and revoked token JTIs across gateway replicas.
type SessionStore interface {
	Put(ctx context.Context, kind, key string, val []byte, ttl time.Duration) error
	Get(ctx context.Context, kind, key string) ([]byte, bool, error)
	Pop(ctx context.Context, kind, key string) ([]byte, bool, error)
	Delete(ctx context.Context, kind, key string) error
	Revoke(ctx context.Context, jti string, expiresAt time.Time) error
	IsRevoked(ctx context.Context, jti string) (bool, error)
	Sweep(ctx context.Context) (int, error)
	Close()
}

type memoryItem struct {
	value     []byte
	expiresAt time.Time
}

type memoryStore struct {
	mu      sync.Mutex
	items   map[string]map[string]memoryItem
	revoked map[string]time.Time
	caps    map[string]int
	now     func() time.Time
}

func newMemoryStore(now func() time.Time, caps map[string]int) *memoryStore {
	if now == nil {
		now = time.Now
	}
	copiedCaps := make(map[string]int, len(caps))
	for k, v := range caps {
		copiedCaps[k] = v
	}
	return &memoryStore{
		items:   make(map[string]map[string]memoryItem),
		revoked: make(map[string]time.Time),
		caps:    copiedCaps,
		now:     now,
	}
}

func (s *memoryStore) Put(ctx context.Context, kind, key string, val []byte, ttl time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.now()
	if capLimit, hasCap := s.caps[kind]; hasCap && capLimit > 0 {
		s.sweepKindLocked(kind, now)
		bucket := s.items[kind]
		if _, exists := bucket[key]; !exists && len(bucket) >= capLimit {
			return errStoreFull
		}
	}

	bucket := s.items[kind]
	if bucket == nil {
		bucket = make(map[string]memoryItem)
		s.items[kind] = bucket
	}

	copied := make([]byte, len(val))
	copy(copied, val)
	bucket[key] = memoryItem{
		value:     copied,
		expiresAt: now.Add(ttl),
	}
	return nil
}

func (s *memoryStore) Get(ctx context.Context, kind, key string) ([]byte, bool, error) {
	if err := ctx.Err(); err != nil {
		return nil, false, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	bucket := s.items[kind]
	if bucket == nil {
		return nil, false, nil
	}
	entry, ok := bucket[key]
	if !ok {
		return nil, false, nil
	}
	if !s.now().Before(entry.expiresAt) {
		delete(bucket, key)
		return nil, false, nil
	}
	copied := make([]byte, len(entry.value))
	copy(copied, entry.value)
	return copied, true, nil
}

func (s *memoryStore) Pop(ctx context.Context, kind, key string) ([]byte, bool, error) {
	if err := ctx.Err(); err != nil {
		return nil, false, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	bucket := s.items[kind]
	if bucket == nil {
		return nil, false, nil
	}
	entry, ok := bucket[key]
	if !ok {
		return nil, false, nil
	}
	delete(bucket, key)
	if !s.now().Before(entry.expiresAt) {
		return nil, false, nil
	}
	copied := make([]byte, len(entry.value))
	copy(copied, entry.value)
	return copied, true, nil
}

func (s *memoryStore) Delete(ctx context.Context, kind, key string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if bucket := s.items[kind]; bucket != nil {
		delete(bucket, key)
	}
	return nil
}

func (s *memoryStore) Revoke(ctx context.Context, jti string, expiresAt time.Time) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if existing, ok := s.revoked[jti]; !ok || expiresAt.After(existing) {
		s.revoked[jti] = expiresAt
	}
	return nil
}

func (s *memoryStore) IsRevoked(ctx context.Context, jti string) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	exp, ok := s.revoked[jti]
	if !ok {
		return false, nil
	}
	if !s.now().Before(exp) {
		delete(s.revoked, jti)
		return false, nil
	}
	return true, nil
}

func (s *memoryStore) Sweep(ctx context.Context) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.now()
	removed := 0
	for kind := range s.items {
		removed += s.sweepKindLocked(kind, now)
	}
	for jti, exp := range s.revoked {
		if !now.Before(exp) {
			delete(s.revoked, jti)
			removed++
		}
	}
	return removed, nil
}

func (s *memoryStore) sweepKindLocked(kind string, now time.Time) int {
	bucket := s.items[kind]
	if bucket == nil {
		return 0
	}
	removed := 0
	for k, entry := range bucket {
		if !now.Before(entry.expiresAt) {
			delete(bucket, k)
			removed++
		}
	}
	return removed
}

func (s *memoryStore) Close() {}
