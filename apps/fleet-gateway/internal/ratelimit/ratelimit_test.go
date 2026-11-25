package ratelimit

import (
	"fmt"
	"testing"
	"time"
)

func TestIPRateLimiter_EvictsOldestOnFull(t *testing.T) {
	now := time.Date(2026, 11, 19, 8, 0, 0, 0, time.UTC)
	l := New(2, 100, time.Minute, 2, func() time.Time { return now })

	// IP1 enters at t=0 (resetAt = t+60s) and exhausts its per-IP limit (2).
	if !l.Allow("198.51.100.1") {
		t.Fatal("IP1 first request rejected")
	}
	if !l.Allow("198.51.100.1") {
		t.Fatal("IP1 second request rejected")
	}
	if l.Allow("198.51.100.1") {
		t.Fatal("IP1 third request should be rate limited")
	}

	// IP2 enters at t=10s (resetAt = t+70s) with 1 request.
	now = now.Add(10 * time.Second)
	if !l.Allow("198.51.100.2") {
		t.Fatal("IP2 first request rejected")
	}

	// At t=20s neither IP1 nor IP2 has expired, so the table (maxSize=2) is full.
	// IP3 must still be allowed and evict IP1 (earliest resetAt).
	now = now.Add(10 * time.Second)
	if !l.Allow("198.51.100.3") {
		t.Fatal("IP3 should be allowed by evicting oldest IP")
	}
	if len(l.buckets) != 2 {
		t.Fatalf("bucket count = %d, want 2", len(l.buckets))
	}

	// IP2 was not evicted, so it has only 1 remaining slot in its window.
	if !l.Allow("198.51.100.2") {
		t.Fatal("IP2 second request should still be allowed")
	}
	if l.Allow("198.51.100.2") {
		t.Fatal("IP2 third request should be rejected; existing window must remain intact")
	}

	// IP1 was evicted, so returning within its original window gives it a fresh window (2 requests).
	// Calling IP1 now evicts IP3 (resetAt t+70s vs IP2 t+70s? wait: IP2 resetAt is t+70s, IP3 resetAt is t+80s).
	if !l.Allow("198.51.100.1") {
		t.Fatal("evicted IP1 should get a fresh window (1/2)")
	}
	if !l.Allow("198.51.100.1") {
		t.Fatal("evicted IP1 should get a fresh window (2/2)")
	}
	if l.Allow("198.51.100.1") {
		t.Fatal("IP1 third request in fresh window should be rejected")
	}
}

func TestIPRateLimiter_GlobalLimitStillBlocksWhenFull(t *testing.T) {
	now := time.Date(2026, 11, 19, 8, 0, 0, 0, time.UTC)
	l := New(5, 2, time.Minute, 2, func() time.Time { return now })

	if !l.Allow("198.51.100.1") || !l.Allow("198.51.100.2") {
		t.Fatal("expected initial requests to succeed up to globalLimit=2")
	}
	if l.Allow("198.51.100.3") {
		t.Fatal("expected global limit to reject IP3 even though eviction would make room")
	}
}

func TestRateLimitLogSampler_BoundedEvictsOldest(t *testing.T) {
	now := time.Date(2026, 11, 19, 8, 0, 0, 0, time.UTC)
	s := NewLogSampler(time.Minute, 3, func() time.Time { return now })

	if !s.Allow("198.51.100.1") {
		t.Fatal("IP1 first Allow = false, want true")
	}
	if s.Allow("198.51.100.1") {
		t.Fatal("IP1 repeat within window = true, want false")
	}

	now = now.Add(time.Second)
	if !s.Allow("198.51.100.2") {
		t.Fatal("IP2 first Allow = false, want true")
	}
	now = now.Add(time.Second)
	if !s.Allow("198.51.100.3") {
		t.Fatal("IP3 first Allow = false, want true")
	}

	// Map is at capacity 3; inserting IP4 must evict IP1 (oldest timestamp at t=0).
	now = now.Add(time.Second)
	if !s.Allow("198.51.100.4") {
		t.Fatal("IP4 first Allow = false, want true")
	}
	if len(s.lastSeen) != 3 {
		t.Fatalf("lastSeen len = %d, want 3", len(s.lastSeen))
	}
	if _, ok := s.lastSeen["198.51.100.1"]; ok {
		t.Fatal("expected oldest entry 198.51.100.1 to be evicted")
	}
	for _, ip := range []string{"198.51.100.2", "198.51.100.3", "198.51.100.4"} {
		if _, ok := s.lastSeen[ip]; !ok {
			t.Fatalf("expected entry %s to remain", ip)
		}
	}

	// Default-cap sampler stays bounded under a large flood of unique IPs
	// spaced across windows so the global per-window cap does not trigger.
	bounded := NewLogSampler(DefaultLogWindow, DefaultLogCap, func() time.Time { return now })
	for i := 0; i < DefaultLogCap+200; i++ {
		now = now.Add(time.Second)
		ip := fmt.Sprintf("203.0.%d.%d", i/256, i%256)
		if !bounded.Allow(ip) {
			t.Fatalf("unique IP %s first Allow = false, want true", ip)
		}
		if len(bounded.lastSeen) > DefaultLogCap {
			t.Fatalf("lastSeen len = %d exceeded cap %d", len(bounded.lastSeen), DefaultLogCap)
		}
	}
	if len(bounded.lastSeen) != DefaultLogCap {
		t.Fatalf("lastSeen len = %d, want %d", len(bounded.lastSeen), DefaultLogCap)
	}

	// Sweep clears expired entries once the window elapses.
	now = now.Add(DefaultLogWindow)
	if removed := bounded.Sweep(); removed != DefaultLogCap {
		t.Fatalf("Sweep removed %d entries, want %d", removed, DefaultLogCap)
	}
	if len(bounded.lastSeen) != 0 {
		t.Fatalf("lastSeen len after Sweep = %d, want 0", len(bounded.lastSeen))
	}
}

func TestRateLimitLogSampler_GlobalCap(t *testing.T) {
	now := time.Date(2026, 11, 19, 8, 0, 0, 0, time.UTC)
	s := NewLogSampler(time.Minute, DefaultLogCap, func() time.Time { return now })

	allowed := 0
	for i := 0; i < 10000; i++ {
		ip := fmt.Sprintf("198.51.%d.%d", i/256, i%256)
		if s.Allow(ip) {
			allowed++
		}
	}
	if allowed != DefaultLogGlobalCap {
		t.Fatalf("allowed in one window = %d, want %d", allowed, DefaultLogGlobalCap)
	}
	if len(s.lastSeen) != DefaultLogGlobalCap {
		t.Fatalf("lastSeen len = %d, want %d", len(s.lastSeen), DefaultLogGlobalCap)
	}

	now = now.Add(time.Minute)
	if !s.Allow("203.0.113.1") {
		t.Fatal("fresh IP after advancing clock by one window = false, want true")
	}
}
