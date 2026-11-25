package cache

import (
	"sync"
	"time"
)

// defaultMaxEntries bounds in-memory cache growth. Per-user cache keys
// (user_metrics_*, user_daily_credits_*, user_weekly_trends_*, etc.) grow
// unboundedly between TTL-triggered cleanups under normal operation; this cap
// is a safety valve against unbounded memory growth if usage patterns spike
// (e.g. many distinct users/params hitting cache within a single TTL window).
const defaultMaxEntries = 5000

type item struct {
	value      interface{}
	expiration time.Time
}

// Cache is a simple in-memory cache with TTL and a soft maximum entry count.
type Cache struct {
	mu       sync.RWMutex
	entries  map[string]item
	ttl      time.Duration
	maxSize  int
	stopOnce sync.Once
	stopCh   chan struct{}
	doneCh   chan struct{}
}

// New returns a cache with the given TTL and starts a background cleanup goroutine.
func New(ttl time.Duration) *Cache {
	return newWithMaxSize(ttl, defaultMaxEntries)
}

// newWithMaxSize is like New but allows overriding the entry cap —
// primarily for tests that need to exercise eviction without inserting
// thousands of entries.
func newWithMaxSize(ttl time.Duration, maxSize int) *Cache {
	c := &Cache{
		entries: make(map[string]item),
		ttl:     ttl,
		maxSize: maxSize,
		stopCh:  make(chan struct{}),
		doneCh:  make(chan struct{}),
	}

	go func() {
		ticker := time.NewTicker(ttl)
		defer ticker.Stop()
		defer close(c.doneCh)
		for {
			select {
			case <-ticker.C:
				c.Cleanup()
			case <-c.stopCh:
				return
			}
		}
	}()

	return c
}

func (c *Cache) Get(key string) (interface{}, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, exists := c.entries[key]
	if !exists {
		return nil, false
	}

	if time.Now().After(entry.expiration) {
		return nil, false
	}

	return entry.value, true
}

// Set stores a value in the cache with the configured TTL. If the cache is
// at capacity and this is a new key, expired entries are evicted first; if
// that isn't enough, entries are evicted in (unspecified) map iteration
// order as a last-resort safety valve against unbounded growth.
func (c *Cache) Set(key string, value interface{}) {
	c.SetWithTTL(key, value, c.ttl)
}

// SetWithTTL stores a value with a custom TTL. Useful for negative caching
// (storing "no data" results for a shorter period than the default).
func (c *Cache) SetWithTTL(key string, value interface{}, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.entries[key]; !exists && c.maxSize > 0 && len(c.entries) >= c.maxSize {
		c.evictLocked()
	}

	c.entries[key] = item{
		value:      value,
		expiration: time.Now().Add(ttl),
	}
}

// evictLocked makes room for at least one new entry. Caller must hold c.mu.
func (c *Cache) evictLocked() {
	now := time.Now()
	for key, entry := range c.entries {
		if now.After(entry.expiration) {
			delete(c.entries, key)
		}
	}
	if len(c.entries) < c.maxSize {
		return
	}
	// Still over capacity after removing expired entries: fall back to
	// evicting arbitrary entries (Go map iteration order is randomized)
	// until back under the cap.
	for key := range c.entries {
		delete(c.entries, key)
		if len(c.entries) < c.maxSize {
			return
		}
	}
}

func (c *Cache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.entries, key)
}

func (c *Cache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries = make(map[string]item)
}

func (c *Cache) Cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for key, entry := range c.entries {
		if now.After(entry.expiration) {
			delete(c.entries, key)
		}
	}
}

func (c *Cache) Stop() {
	c.stopOnce.Do(func() {
		close(c.stopCh)
		<-c.doneCh
	})
}
