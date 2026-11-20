package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"sync"
	"time"
)

const (
	kindPendingConsent = "pending_consent"
	// pending_entra_v2: consent is asked after the Entra callback. A new kind so
	// old pods (consent before Entra) never pop it and skip consent mid-rollout.
	kindPendingEntra       = "pending_entra_v2"
	kindPendingEntraLegacy = "pending_entra" // written by old pods; always gets the consent page
	kindCode               = "code"
	kindSession            = "session"
	kindConsent            = "consent" // remembered consent per (NAVident, client_id, resource)
	kindMCPSession         = "mcp_session"
	kindMCPSessionUser     = "mcp_session_user" // per-user count of mcp_session bindings
	kindDCRClient          = "dcr_client"
	kindRefreshGrace       = "refresh_grace"       // pair issued for a rotated RT, keyed by its jti
	kindUserRevoked        = "user_revoked"        // per-user lockout record keyed by HMAC(NAVident)
	kindUserRevokedMarker  = "user_revoked_marker" // set when a lockout record is written; startup refuses without LOCKOUT_KEY
	kindAuditOutbox        = "audit_outbox"        // queued audit entries written by CLI, drained by server sweeper
)

var errStoreFull = errors.New("store capacity reached")

// SessionStore persists short-lived authorization state, authorization codes,
// gateway sessions, and revoked token JTIs across gateway replicas.
type SessionStore interface {
	Put(ctx context.Context, kind, key string, val []byte, ttl time.Duration) error
	Get(ctx context.Context, kind, key string) ([]byte, bool, error)
	Pop(ctx context.Context, kind, key string) ([]byte, bool, error)
	Delete(ctx context.Context, kind, key string) error
	// CompareAndSwap atomically replaces the live value for (kind, key) with new
	// (nil deletes) if the current live value equals old (nil means no live row),
	// returning whether the swap occurred.
	CompareAndSwap(ctx context.Context, kind, key string, oldVal, newVal []byte, ttl time.Duration) (bool, error)
	Revoke(ctx context.Context, jti string, expiresAt time.Time) error
	RevokeOnce(ctx context.Context, jti string, expiresAt time.Time) (bool, error)
	IsRevoked(ctx context.Context, jti string) (bool, error)
	// Incr bumps a fixed-window counter shared by every replica and returns the
	// count in the current window, which starts on the first Incr after expiry.
	Incr(ctx context.Context, key string, window time.Duration) (int, error)
	// PutEvict is Put that, when kind is at its cap, first deletes the live item
	// of kind with the earliest expiry. Eviction and insert are one atomic step,
	// so concurrent callers cannot take the freed slot. evicted reports whether
	// an item was dropped.
	PutEvict(ctx context.Context, kind, key string, val []byte, ttl time.Duration) (evicted bool, err error)
	// CountPrefix counts the live items of kind whose key starts with prefix.
	CountPrefix(ctx context.Context, kind, prefix string) (int, error)
	Sweep(ctx context.Context) (int, error)
	Ping(ctx context.Context) error
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
	counts  map[string]rateWindow
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
		counts:  make(map[string]rateWindow),
		caps:    copiedCaps,
		now:     now,
	}
}

func (s *memoryStore) Put(ctx context.Context, kind, key string, val []byte, ttl time.Duration) error {
	_, err := s.put(ctx, kind, key, val, ttl, false)
	return err
}

func (s *memoryStore) PutEvict(ctx context.Context, kind, key string, val []byte, ttl time.Duration) (bool, error) {
	return s.put(ctx, kind, key, val, ttl, true)
}

func (s *memoryStore) put(ctx context.Context, kind, key string, val []byte, ttl time.Duration, evict bool) (evicted bool, err error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.now()
	if capLimit, hasCap := s.caps[kind]; hasCap && capLimit > 0 {
		s.sweepKindLocked(kind, now)
		bucket := s.items[kind]
		if _, exists := bucket[key]; !exists && len(bucket) >= capLimit {
			if !evict || len(bucket) == 0 {
				return false, errStoreFull
			}
			// sweepKindLocked left only live items, so this frees a counted slot.
			oldest, oldestAt := "", time.Time{}
			for k, e := range bucket {
				if oldest == "" || e.expiresAt.Before(oldestAt) {
					oldest, oldestAt = k, e.expiresAt
				}
			}
			delete(bucket, oldest)
			evicted = true
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
	return evicted, nil
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

func (s *memoryStore) CompareAndSwap(ctx context.Context, kind, key string, oldVal, newVal []byte, ttl time.Duration) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.now()
	bucket := s.items[kind]
	var entry memoryItem
	var live bool
	if bucket != nil {
		var exists bool
		entry, exists = bucket[key]
		if exists {
			if now.Before(entry.expiresAt) {
				live = true
			} else {
				delete(bucket, key)
			}
		}
	}

	if oldVal == nil {
		if live {
			return false, nil
		}
		if newVal == nil {
			return true, nil
		}
		if capLimit, hasCap := s.caps[kind]; hasCap && capLimit > 0 {
			s.sweepKindLocked(kind, now)
			if len(bucket) >= capLimit {
				return false, errStoreFull
			}
		}
		if bucket == nil {
			bucket = make(map[string]memoryItem)
			s.items[kind] = bucket
		}
		copied := make([]byte, len(newVal))
		copy(copied, newVal)
		bucket[key] = memoryItem{
			value:     copied,
			expiresAt: now.Add(ttl),
		}
		return true, nil
	}

	if !live || !bytes.Equal(entry.value, oldVal) {
		return false, nil
	}
	if newVal == nil {
		delete(bucket, key)
		return true, nil
	}
	copied := make([]byte, len(newVal))
	copy(copied, newVal)
	bucket[key] = memoryItem{
		value:     copied,
		expiresAt: now.Add(ttl),
	}
	return true, nil
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

func (s *memoryStore) RevokeOnce(ctx context.Context, jti string, expiresAt time.Time) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.now()
	if existing, ok := s.revoked[jti]; ok && now.Before(existing) {
		return false, nil
	}
	s.revoked[jti] = expiresAt
	return true, nil
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

func (s *memoryStore) Incr(ctx context.Context, key string, window time.Duration) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.now()
	c, ok := s.counts[key]
	if !ok || !now.Before(c.resetAt) {
		c = rateWindow{resetAt: now.Add(window)}
	}
	c.count++
	s.counts[key] = c
	return c.count, nil
}

func (s *memoryStore) CountPrefix(ctx context.Context, kind, prefix string) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	n := 0
	for key, entry := range s.items[kind] {
		if strings.HasPrefix(key, prefix) && now.Before(entry.expiresAt) {
			n++
		}
	}
	return n, nil
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
	for k, c := range s.counts {
		if !now.Before(c.resetAt) {
			delete(s.counts, k)
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

func (s *memoryStore) Ping(_ context.Context) error { return nil }

func (s *memoryStore) Close() {}
