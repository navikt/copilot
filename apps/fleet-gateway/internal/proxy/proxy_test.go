package proxy

import (
	"io"
	"net/http"
	"runtime"
	"testing"
)

// readChunks must not reallocate: io.ReadAll on an unknown-length 10 MiB body
// allocates roughly 2-3x that while growing.
func TestReadChunksAllocatesBodySize(t *testing.T) {
	const size = 10 << 20
	var ms0, ms1 runtime.MemStats
	runtime.ReadMemStats(&ms0)
	chunks, n, err := readChunks(io.LimitReader(zeroReader{}, size), size+1)
	runtime.ReadMemStats(&ms1)
	if err != nil || n != size || len(chunks) == 0 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	if alloc := ms1.TotalAlloc - ms0.TotalAlloc; alloc > size*11/10 {
		t.Fatalf("allocated %d bytes for a %d-byte body, want <= 1.1x", alloc, size)
	}
}

type zeroReader struct{}

func (zeroReader) Read(p []byte) (int, error) { clear(p); return len(p), nil }

func TestAcceptsEventStream_QZero(t *testing.T) {
	for accept, want := range map[string]bool{
		"text/event-stream;q=0":                   false,
		"application/json, text/event-stream;q=0": false,
		"*/*;q=0.0, application/json":             false,
		"text/event-stream;q=0, */*;q=1":          false,
		"*/*;q=1, text/event-stream;q=0":          false,
		"text/event-stream;q=1, */*;q=0":          true,
		"text/event-stream;q=0.5":                 true,
		"application/json, text/event-stream":     true,
	} {
		if got := acceptsEventStream(http.Header{"Accept": {accept}}); got != want {
			t.Errorf("%q: got %v, want %v", accept, got, want)
		}
	}
}

func TestHardenedProxyClient_CapsResponseHeaders(t *testing.T) {
	tr, ok := NewHardenedClient(0).Transport.(*http.Transport)
	if !ok || tr.MaxResponseHeaderBytes != 64<<10 {
		t.Fatalf("MaxResponseHeaderBytes = %v, want 64 KiB", tr.MaxResponseHeaderBytes)
	}
}
