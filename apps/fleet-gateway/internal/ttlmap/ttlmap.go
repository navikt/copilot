// Package ttlmap is a bounded in-memory map whose entries expire.
package ttlmap

import (
	"sync"
	"time"
)

type entry[V any] struct {
	value     V
	expiresAt time.Time
}

// Map holds at most maxSize unexpired entries.
type Map[V any] struct {
	mu      sync.Mutex
	items   map[string]entry[V]
	maxSize int
	now     func() time.Time
}

// New returns an empty Map; a non-positive maxSize means 1.
func New[V any](maxSize int, now func() time.Time) *Map[V] {
	if now == nil {
		now = time.Now
	}
	if maxSize <= 0 {
		maxSize = 1
	}
	return &Map[V]{
		items:   make(map[string]entry[V]),
		maxSize: maxSize,
		now:     now,
	}
}

// Set stores val for ttl and reports false when the map is full of live entries.
func (m *Map[V]) Set(key string, val V, ttl time.Duration) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := m.now()
	if _, exists := m.items[key]; !exists && len(m.items) >= m.maxSize {
		m.sweepLocked(now)
		if len(m.items) >= m.maxSize {
			return false
		}
	}
	m.items[key] = entry[V]{value: val, expiresAt: now.Add(ttl)}
	return true
}

// Get returns the unexpired value for key.
func (m *Map[V]) Get(key string) (V, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	entry, ok := m.items[key]
	if !ok {
		var zero V
		return zero, false
	}
	if !m.now().Before(entry.expiresAt) {
		delete(m.items, key)
		var zero V
		return zero, false
	}
	return entry.value, true
}

// Pop returns and removes the unexpired value for key.
func (m *Map[V]) Pop(key string) (V, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	entry, ok := m.items[key]
	if !ok {
		var zero V
		return zero, false
	}
	delete(m.items, key)
	if !m.now().Before(entry.expiresAt) {
		var zero V
		return zero, false
	}
	return entry.value, true
}

// Sweep drops expired entries and returns how many it removed.
func (m *Map[V]) Sweep() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.sweepLocked(m.now())
}

// Len reports the number of stored entries, expired ones included.
func (m *Map[V]) Len() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.items)
}

func (m *Map[V]) sweepLocked(now time.Time) int {
	removed := 0
	for k, entry := range m.items {
		if !now.Before(entry.expiresAt) {
			delete(m.items, k)
			removed++
		}
	}
	return removed
}
