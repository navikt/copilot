package main

import (
	"sync"
	"time"
)

// Upstream response cache. kodeverk-api serves the same reference data to every
// caller (no per-user fields, path and query carry no identity), so one cache
// is shared across users. The caller's token is still validated and exchanged
// on every tool call before the cache is read.
const (
	cacheTTL      = 10 * time.Minute
	cacheMaxBytes = 32 << 20 // fits the 200 MiB GOMEMLIMIT budget in kodeverk.go
)

type cacheEntry struct {
	body    []byte
	expires time.Time
}

type responseCache struct {
	mu      sync.Mutex
	now     func() time.Time
	entries map[string]cacheEntry
	order   []string // insertion order, oldest first
	size    int
}

func newResponseCache() *responseCache {
	return &responseCache{now: time.Now, entries: map[string]cacheEntry{}}
}

func (c *responseCache) get(key string) ([]byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[key]
	if !ok {
		return nil, false
	}
	if !c.now().Before(e.expires) {
		c.remove(key)
		return nil, false
	}
	return e.body, true
}

func (c *responseCache) put(key string, body []byte) {
	entrySize := len(key) + len(body)
	if entrySize > cacheMaxBytes {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.remove(key)
	now := c.now()
	for len(c.order) > 0 && !now.Before(c.entries[c.order[0]].expires) {
		c.remove(c.order[0])
	}
	for c.size+entrySize > cacheMaxBytes && len(c.order) > 0 {
		c.remove(c.order[0])
	}
	c.entries[key] = cacheEntry{body: body, expires: now.Add(cacheTTL)}
	c.order = append(c.order, key)
	c.size += entrySize
}

func (c *responseCache) remove(key string) {
	old, ok := c.entries[key]
	if !ok {
		return
	}
	c.size -= len(key) + len(old.body)
	delete(c.entries, key)
	for i, k := range c.order {
		if k == key {
			c.order = append(c.order[:i], c.order[i+1:]...)
			break
		}
	}
}
