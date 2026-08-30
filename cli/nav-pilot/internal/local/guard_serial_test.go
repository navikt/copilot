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

	handler := guardHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp, err := http.Post(upstream.URL, "application/json", strings.NewReader("{}"))
		if err != nil {
			t.Error(err)
			return
		}
		resp.Body.Close()
		w.WriteHeader(http.StatusOK)
	}))

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
