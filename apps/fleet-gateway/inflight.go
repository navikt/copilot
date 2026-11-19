package main

import (
	"net/http"
	"sync"
)

// slots is a counting semaphore for MCP requests. A nil slots never limits,
// for Gateways built without routes().
type slots chan struct{}

// tryAcquire takes a slot without waiting. release is idempotent, so the SSE
// path can hand its slot back early and the deferred call is a no-op.
func (s slots) tryAcquire() (release func(), ok bool) {
	if s == nil {
		return func() {}, true
	}
	select {
	case s <- struct{}{}:
		return sync.OnceFunc(func() { <-s }), true
	default:
		return nil, false
	}
}

// userSlots caps concurrent buffered MCP requests or open SSE streams per user
// on a pod. A nil *userSlots never limits, for Gateways built without routes().
type userSlots struct {
	mu  sync.Mutex
	n   map[string]int
	max int
}

// tryAcquire takes a per-user slot without waiting. release is idempotent, so
// the SSE path can hand its slot back early and the deferred call is a no-op.
func (u *userSlots) tryAcquire(key string) (release func(), ok bool) {
	if u == nil {
		return func() {}, true
	}
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.n[key] >= u.max {
		return nil, false
	}
	if u.n == nil {
		u.n = make(map[string]int)
	}
	u.n[key]++
	return sync.OnceFunc(func() {
		u.mu.Lock()
		defer u.mu.Unlock()
		if cur := u.n[key]; cur <= 1 {
			delete(u.n, key)
		} else {
			u.n[key] = cur - 1
		}
	}), true
}

// writeOverCapacity answers 503 with Retry-After instead of queueing.
func writeOverCapacity(w http.ResponseWriter, what string) {
	markShed(w, "cap")
	w.Header().Set("Retry-After", "1")
	writeJSON(w, http.StatusServiceUnavailable, map[string]string{
		"error":   "too_many_requests",
		"message": "fleet-gateway is at its limit of concurrent " + what + "; retry shortly",
	})
}
