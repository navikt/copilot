package main

import (
	"testing"
	"time"
)

func TestCacheStopIsIdempotent(t *testing.T) {
	cache := NewCache(10 * time.Millisecond)
	cache.Stop()
	cache.Stop()
}
