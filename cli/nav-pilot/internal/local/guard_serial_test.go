package local

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestGuardSerialisesCompletions: concurrent completions must reach the local
// server one at a time.
//
// mlx-lm batches concurrent requests into shared attention, and prompts of
// different lengths make that batching raise [broadcast_shapes] and leave the
// server hung — alive, accepting connections, answering nothing, unrecoverable
// without a restart. A cloud orchestrator asked to refactor ten files dispatched
// ten subagents at once and wedged it on the first attempt.
func TestGuardSerialisesCompletions(t *testing.T) {
	// Independent of what ran before it: the ownership gate caches for a few
	// seconds, so without these this passed only when another test had primed it.
	stubDirs(t)
	stubOwnership(t, func() error { return nil })
	var inFlight, peak int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&inFlight, 1)
		for {
			old := atomic.LoadInt32(&peak)
			if n <= old || atomic.CompareAndSwapInt32(&peak, old, n) {
				break
			}
		}
		time.Sleep(20 * time.Millisecond)
		atomic.AddInt32(&inFlight, -1)
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	handler := guardHandler(&Guard{}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp, err := http.Post(upstream.URL, "application/json", strings.NewReader("{}"))
		if err != nil {
			t.Error(err)
			return
		}
		resp.Body.Close()
		w.WriteHeader(http.StatusOK)
	}), "")

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			body := fmt.Sprintf(`{"messages":[{"role":"user","content":%q}]}`, strings.Repeat("x", i*10))
			req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
			handler.ServeHTTP(httptest.NewRecorder(), req)
		}(i)
	}
	wg.Wait()

	if got := atomic.LoadInt32(&peak); got != 1 {
		t.Errorf("peak concurrent completions at the local server = %d, want 1", got)
	}
}

// TestTwoGuardsStillReachTheServerOneAtATime is the case per-session guards
// created. Each guard serialises its own traffic, so two sessions on one machine
// would send one request apiece and reach mlx-lm concurrently, which is exactly
// the concurrency that wedges it. The lock is held across processes for that
// reason, and two guards in one process is the closest a unit test gets.
func TestTwoGuardsStillReachTheServerOneAtATime(t *testing.T) {
	stubDirs(t)
	var inFlight, peak int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&inFlight, 1)
		for {
			old := atomic.LoadInt32(&peak)
			if n <= old || atomic.CompareAndSwapInt32(&peak, old, n) {
				break
			}
		}
		time.Sleep(30 * time.Millisecond)
		atomic.AddInt32(&inFlight, -1)
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	call := func(handler http.Handler, i int) {
		body := fmt.Sprintf(`{"messages":[{"role":"user","content":%q}]}`, strings.Repeat("x", i*10))
		req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
		handler.ServeHTTP(httptest.NewRecorder(), req)
	}
	forward := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp, err := http.Post(upstream.URL, "application/json", strings.NewReader("{}"))
		if err != nil {
			t.Error(err)
			return
		}
		resp.Body.Close()
		w.WriteHeader(http.StatusOK)
	})
	stubOwnership(t, func() error { return nil })
	sessionA, sessionB := guardHandler(&Guard{}, forward, ""), guardHandler(&Guard{}, forward, "")

	var wg sync.WaitGroup
	for i := 0; i < 6; i++ {
		wg.Add(2)
		go func(i int) { defer wg.Done(); call(sessionA, i) }(i)
		go func(i int) { defer wg.Done(); call(sessionB, i) }(i)
	}
	wg.Wait()

	if got := atomic.LoadInt32(&peak); got != 1 {
		t.Errorf("two sessions put %d requests at the server at once, want 1", got)
	}
}

// TestGuardCountsWhatItForwarded: the dispatch count has to come from the guard,
// because zero is the value that matters and zero is unprovable from a transcript.
//
// A session that handed nothing to the local worker and a session whose transcript
// we failed to parse look identical downstream. Counting here separates them: the
// guard is the only thing that sees every completion, and it sees them whether or
// not anything later can read the log.
func TestGuardCountsWhatItForwarded(t *testing.T) {
	stubDirs(t)
	stubOwnership(t, func() error { return nil })
	g := &Guard{}
	handler := guardHandler(g, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}), "")

	if g.Completions() != 0 {
		t.Errorf("a guard that has forwarded nothing counts %d", g.Completions())
	}
	for i := 0; i < 3; i++ {
		body := `{"messages":[{"role":"user","content":"hei"}]}`
		req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
		handler.ServeHTTP(httptest.NewRecorder(), req)
	}
	// Anything that is not a completion is not a dispatch.
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/v1/models", nil))

	if got := g.Completions(); got != 3 {
		t.Errorf("Completions() = %d after three completions and one model list, want 3", got)
	}
	// A nil guard is what a hosted session has, and asking it must not panic.
	var absent *Guard
	if absent.Completions() != 0 {
		t.Error("a nil guard reported completions")
	}
}
