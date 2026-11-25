// Package ratelimit holds the per-client-IP request limiter and the sampler
// that bounds rate-limit log lines.
package ratelimit

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Defaults used when New gets a non-positive value.
const (
	DefaultRequests       = 30
	DefaultGlobalRequests = 300
	DefaultWindow         = time.Minute
)

// ClientIP resolves the client IP for rate limiting.
// Only the Nais ingress is trusted: we take the LAST valid IP address in
// X-Forwarded-For (appended by the trusted ingress) so earlier client-supplied
// values cannot spoof the rate-limit key, and fall back to r.RemoteAddr when
// X-Forwarded-For is absent or unparseable.
func ClientIP(r *http.Request) string {
	if r == nil {
		return "unknown"
	}
	var lastToken string
	for _, hdr := range r.Header.Values("X-Forwarded-For") {
		for _, part := range strings.Split(hdr, ",") {
			if trimmed := strings.TrimSpace(part); trimmed != "" {
				lastToken = trimmed
			}
		}
	}
	if lastToken != "" {
		if ip := parseIPToken(lastToken); ip != "" {
			return ip
		}
	}
	return parseRemoteAddrIP(r.RemoteAddr)
}

func parseIPToken(token string) string {
	if host, _, err := net.SplitHostPort(token); err == nil && host != "" {
		token = host
	}
	token = strings.TrimSuffix(strings.TrimPrefix(token, "["), "]")
	if ip := net.ParseIP(token); ip != nil {
		return ip.String()
	}
	return ""
}

func parseRemoteAddrIP(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err == nil && host != "" {
		return host
	}
	if remoteAddr != "" {
		return remoteAddr
	}
	return "unknown"
}

type bucket struct {
	count   int
	resetAt time.Time
}

// Limiter caps requests per client IP and globally within a fixed window,
// keeping at most maxSize IP buckets.
type Limiter struct {
	mu          sync.Mutex
	buckets     map[string]bucket
	limit       int
	globalLimit int
	global      bucket
	window      time.Duration
	maxSize     int
	now         func() time.Time
}

// New returns a Limiter; non-positive arguments fall back to the defaults.
func New(limit, globalLimit int, window time.Duration, maxSize int, now func() time.Time) *Limiter {
	if now == nil {
		now = time.Now
	}
	if limit <= 0 {
		limit = DefaultRequests
	}
	if globalLimit <= 0 {
		globalLimit = DefaultGlobalRequests
	}
	if window <= 0 {
		window = DefaultWindow
	}
	if maxSize <= 0 {
		maxSize = 1
	}
	return &Limiter{
		buckets:     make(map[string]bucket),
		limit:       limit,
		globalLimit: globalLimit,
		window:      window,
		maxSize:     maxSize,
		now:         now,
	}
}

// Allow records one request from ip and reports whether it is within limits.
func (l *Limiter) Allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	if !now.Before(l.global.resetAt) {
		l.global = bucket{count: 0, resetAt: now.Add(l.window)}
	}
	if l.global.count >= l.globalLimit {
		return false
	}

	b, ok := l.buckets[ip]
	if !ok || !now.Before(b.resetAt) {
		if !ok && len(l.buckets) >= l.maxSize {
			l.sweepLocked(now)
			if len(l.buckets) >= l.maxSize {
				l.evictOldestLocked()
			}
		}
		l.buckets[ip] = bucket{count: 1, resetAt: now.Add(l.window)}
		l.global.count++
		return true
	}
	if b.count >= l.limit {
		return false
	}
	b.count++
	l.buckets[ip] = b
	l.global.count++
	return true
}

// Len reports the number of tracked client buckets.
func (l *Limiter) Len() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.buckets)
}

// Sweep drops expired buckets and returns how many it removed.
func (l *Limiter) Sweep() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.sweepLocked(l.now())
}

func (l *Limiter) sweepLocked(now time.Time) int {
	removed := 0
	for ip, b := range l.buckets {
		if !now.Before(b.resetAt) {
			delete(l.buckets, ip)
			removed++
		}
	}
	return removed
}

func (l *Limiter) evictOldestLocked() {
	var oldestIP string
	var oldestReset time.Time
	first := true
	for ip, b := range l.buckets {
		if first || b.resetAt.Before(oldestReset) {
			oldestIP = ip
			oldestReset = b.resetAt
			first = false
		}
	}
	if !first {
		delete(l.buckets, oldestIP)
	}
}
