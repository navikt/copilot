// Package inflight caps concurrent MCP requests and SSE streams on one pod.
package inflight

import (
	"sync"
)

// Slots is a counting semaphore for MCP requests. A nil Slots never limits,
// for Gateways built without routes().
type Slots chan struct{}

// TryAcquire takes a slot without waiting. release is idempotent, so the SSE
// path can hand its slot back early and the deferred call is a no-op.
func (s Slots) TryAcquire() (release func(), ok bool) {
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

// PerKey caps concurrent buffered MCP requests or open SSE streams per key
// (user or route) on a pod. A nil *PerKey never limits, for Gateways built without routes().
type PerKey struct {
	mu  sync.Mutex
	n   map[string]int
	max int
}

// TryAcquire takes a per-user slot without waiting. release is idempotent, so
// the SSE path can hand its slot back early and the deferred call is a no-op.
func (u *PerKey) TryAcquire(key string) (release func(), ok bool) {
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

// NewPerKey returns a PerKey allowing limit concurrent holders per key.
func NewPerKey(limit int) *PerKey { return &PerKey{max: limit} }

// Len reports how many keys hold at least one slot.
// Exported because the gateway tests in package main check it after streams close.
func (u *PerKey) Len() int {
	u.mu.Lock()
	defer u.mu.Unlock()
	return len(u.n)
}
