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

// writeOverCapacity answers 503 with Retry-After instead of queueing.
func writeOverCapacity(w http.ResponseWriter, what string) {
	w.Header().Set("Retry-After", "1")
	writeJSON(w, http.StatusServiceUnavailable, map[string]string{
		"error":   "too_many_requests",
		"message": "fleet-gateway is at its limit of concurrent " + what + "; retry shortly",
	})
}
