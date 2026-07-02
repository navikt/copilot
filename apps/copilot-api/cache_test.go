package main

import (
	"fmt"
	"testing"
	"time"
)

func TestCacheStopIsIdempotent(t *testing.T) {
	cache := NewCache(10 * time.Millisecond)
	cache.Stop()
	cache.Stop()
}

func TestCacheEvictsExpiredEntriesBeforeGrowingPastCap(t *testing.T) {
	// Long TTL so entries don't expire naturally during the test, but we
	// manually backdate some entries' expiration to simulate staleness.
	cache := newCacheWithMaxSize(time.Hour, 3)
	defer cache.Stop()

	cache.Set("a", 1)
	cache.Set("b", 2)

	// Manually expire "a" so it should be reclaimed on the next Set instead
	// of evicting a fresh entry.
	cache.mu.Lock()
	entry := cache.entries["a"]
	entry.expiration = time.Now().Add(-time.Minute)
	cache.entries["a"] = entry
	cache.mu.Unlock()

	cache.Set("c", 3)
	cache.Set("d", 4) // triggers eviction since len == maxSize (3) before insert

	if _, ok := cache.Get("a"); ok {
		t.Error("expected expired entry 'a' to have been evicted, but it's still present")
	}
	if _, ok := cache.Get("b"); !ok {
		t.Error("expected fresh entry 'b' to survive eviction of the expired entry")
	}
	if _, ok := cache.Get("d"); !ok {
		t.Error("expected newly-inserted entry 'd' to be present")
	}
}

func TestCacheNeverExceedsMaxSize(t *testing.T) {
	const maxSize = 10
	cache := newCacheWithMaxSize(time.Hour, maxSize)
	defer cache.Stop()

	for i := 0; i < maxSize*5; i++ {
		cache.Set(fmt.Sprintf("key-%d", i), i)
	}

	cache.mu.RLock()
	size := len(cache.entries)
	cache.mu.RUnlock()

	if size > maxSize {
		t.Errorf("cache grew to %d entries, want <= %d", size, maxSize)
	}
}

func TestCacheUpdatingExistingKeyDoesNotEvict(t *testing.T) {
	cache := newCacheWithMaxSize(time.Hour, 2)
	defer cache.Stop()

	cache.Set("a", 1)
	cache.Set("b", 2)
	// Updating an existing key should never trigger eviction, since it
	// doesn't grow the map.
	cache.Set("a", 99)

	got, ok := cache.Get("a")
	if !ok || got != 99 {
		t.Errorf("Get(a) = %v, %v; want 99, true", got, ok)
	}
	if _, ok := cache.Get("b"); !ok {
		t.Error("expected 'b' to survive an update to an already-existing key")
	}
}
