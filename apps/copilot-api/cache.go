package main

import (
	"sync"
	"time"
)

// CacheEntry represents a single cached value with expiration
type CacheEntry struct {
	value      interface{}
	expiration time.Time
}

// Cache is a simple in-memory cache with TTL
type Cache struct {
	mu       sync.RWMutex
	entries  map[string]CacheEntry
	ttl      time.Duration
	stopOnce sync.Once
	stopCh   chan struct{}
	doneCh   chan struct{}
}

// NewCache creates a new cache with the specified TTL and starts a background cleanup goroutine
func NewCache(ttl time.Duration) *Cache {
	c := &Cache{
		entries: make(map[string]CacheEntry),
		ttl:     ttl,
		stopCh:  make(chan struct{}),
		doneCh:  make(chan struct{}),
	}

	// Run cache cleanup every TTL interval
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

// Get retrieves a value from the cache
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

// Set stores a value in the cache with the configured TTL
func (c *Cache) Set(key string, value interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries[key] = CacheEntry{
		value:      value,
		expiration: time.Now().Add(c.ttl),
	}
}

// Delete removes a value from the cache
func (c *Cache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.entries, key)
}

// Clear removes all entries from the cache
func (c *Cache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries = make(map[string]CacheEntry)
}

// Cleanup removes expired entries (should be called periodically)
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

// Stop stops the background cleanup goroutine.
func (c *Cache) Stop() {
	c.stopOnce.Do(func() {
		close(c.stopCh)
		<-c.doneCh
	})
}
