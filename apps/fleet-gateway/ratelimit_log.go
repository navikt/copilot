package main

import (
	"sync"
	"time"
)

const (
	defaultRateLimitLogCap       = 4096
	defaultRateLimitLogGlobalCap = 100
	defaultRateLimitLogWindow    = time.Minute
)

// rateLimitLogSampler samples rate-limit warnings so each client IP emits at
// most one log line per window and total warnings per window are globally
// capped, while keeping memory bounded.
type rateLimitLogSampler struct {
	mu          sync.Mutex
	lastSeen    map[string]time.Time
	window      time.Duration
	maxSize     int
	globalMax   int
	windowStart time.Time
	count       int
	now         func() time.Time
}

func newRateLimitLogSampler(window time.Duration, maxSize int, now func() time.Time) *rateLimitLogSampler {
	if now == nil {
		now = time.Now
	}
	if window <= 0 {
		window = defaultRateLimitLogWindow
	}
	if maxSize <= 0 {
		maxSize = 1
	}
	return &rateLimitLogSampler{
		lastSeen:  make(map[string]time.Time),
		window:    window,
		maxSize:   maxSize,
		globalMax: defaultRateLimitLogGlobalCap,
		now:       now,
	}
}

// Allow reports whether a rate-limit warning should be logged for ip at the current time.
func (s *rateLimitLogSampler) Allow(ip string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.now()
	if last, ok := s.lastSeen[ip]; ok && now.Before(last.Add(s.window)) {
		return false
	}

	if s.windowStart.IsZero() || !now.Before(s.windowStart.Add(s.window)) {
		s.windowStart = now
		s.count = 0
	}
	if s.count >= s.globalMax {
		return false
	}
	s.count++

	if _, ok := s.lastSeen[ip]; !ok && len(s.lastSeen) >= s.maxSize {
		s.evictOldestLocked()
	}
	s.lastSeen[ip] = now
	return true
}

// Sweep removes expired entries from the sampler map and returns how many were removed.
func (s *rateLimitLogSampler) Sweep() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.now()
	removed := 0
	for ip, ts := range s.lastSeen {
		if !now.Before(ts.Add(s.window)) {
			delete(s.lastSeen, ip)
			removed++
		}
	}
	return removed
}

func (s *rateLimitLogSampler) evictOldestLocked() {
	var oldestIP string
	var oldestTime time.Time
	first := true
	for ip, ts := range s.lastSeen {
		if first || ts.Before(oldestTime) {
			oldestIP = ip
			oldestTime = ts
			first = false
		}
	}
	if !first {
		delete(s.lastSeen, oldestIP)
	}
}
