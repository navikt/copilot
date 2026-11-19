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
	if !ok || !c.now().Before(e.expires) {
		return nil, false
	}
	return e.body, true
}

func (c *responseCache) put(key string, body []byte) {
	if len(body) > cacheMaxBytes {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if old, ok := c.entries[key]; ok {
		c.size -= len(old.body)
		delete(c.entries, key)
		for i, k := range c.order {
			if k == key {
				c.order = append(c.order[:i], c.order[i+1:]...)
				break
			}
		}
	}
	for c.size+len(body) > cacheMaxBytes && len(c.order) > 0 {
		oldest := c.order[0]
		c.order = c.order[1:]
		c.size -= len(c.entries[oldest].body)
		delete(c.entries, oldest)
	}
	c.entries[key] = cacheEntry{body: body, expires: c.now().Add(cacheTTL)}
	c.order = append(c.order, key)
	c.size += len(body)
}
