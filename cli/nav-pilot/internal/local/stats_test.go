package local

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// TestStatsSurviveConcurrentWriters: two nav-pilot sessions share one server
// and both record through this. A read-modify-write counter would lose one of
// every pair; the append has to keep both.
func TestStatsSurviveConcurrentWriters(t *testing.T) {
	stubDirs(t)

	const writers, each = 8, 25
	var wg sync.WaitGroup
	for w := 0; w < writers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < each; i++ {
				RecordCompletion(10, 3, 0.5)
			}
		}()
	}
	wg.Wait()

	s, err := ReadStats()
	if err != nil {
		t.Fatal(err)
	}
	if s.Requests != writers*each {
		t.Errorf("recorded %d requests, want %d — an append was lost", s.Requests, writers*each)
	}
	if s.TokensOut != int64(writers*each*3) {
		t.Errorf("TokensOut = %d, want %d", s.TokensOut, writers*each*3)
	}
	if s.Since.IsZero() {
		t.Error("no earliest timestamp")
	}
}

// TestStatsSkipTornLine: a killed process can leave half a line. That should
// cost one completion in the total, not the whole file.
func TestStatsSkipTornLine(t *testing.T) {
	stubDirs(t)
	RecordCompletion(5, 5, 1)
	appendRaw(t, `{"at":"2026-08-31T0`)
	RecordCompletion(5, 5, 1)

	s, err := ReadStats()
	if err != nil {
		t.Fatal(err)
	}
	if s.Requests != 2 {
		t.Errorf("Requests = %d, want 2 (the torn line must be skipped, not fatal)", s.Requests)
	}
}

// TestStatsCountMissingUsage: a streaming client that never asks for usage
// reports none. Those requests must still count, and must be named, so a small
// token total is never read as a small amount of work.
func TestStatsCountMissingUsage(t *testing.T) {
	stubDirs(t)
	RecordCompletion(0, 0, 2)
	RecordCompletion(100, 40, 3)

	s, err := ReadStats()
	if err != nil {
		t.Fatal(err)
	}
	if s.Requests != 2 || s.WithoutUsage != 1 {
		t.Errorf("Requests=%d WithoutUsage=%d, want 2 and 1", s.Requests, s.WithoutUsage)
	}
	if s.TokensOut != 40 {
		t.Errorf("TokensOut = %d, want 40", s.TokensOut)
	}
}

// TestStatsReadMissingFile: status runs before anything has been recorded, and
// that is not an error.
func TestStatsReadMissingFile(t *testing.T) {
	stubDirs(t)
	s, err := ReadStats()
	if err != nil {
		t.Fatalf("reading stats that do not exist: %v", err)
	}
	if s.Requests != 0 {
		t.Errorf("Requests = %d on a fresh machine", s.Requests)
	}
}

// TestUsageTapReadsBothResponseShapes: the tap has to find usage in a plain
// JSON completion and in the final frame of a stream, because the two clients
// we support use one each.
func TestUsageTapReadsBothResponseShapes(t *testing.T) {
	for _, tc := range []struct {
		name     string
		body     string
		in, out  int64
		flushing bool
	}{
		{
			name: "plain JSON completion",
			body: `{"id":"x","choices":[{"message":{"content":"hei"}}],` +
				`"usage":{"prompt_tokens":1200,"completion_tokens":64,"total_tokens":1264}}`,
			in: 1200, out: 64,
		},
		{
			name: "stream with a trailing usage frame",
			body: "data: {\"choices\":[{\"delta\":{\"content\":\"he\"}}]}\n\n" +
				"data: {\"choices\":[{\"delta\":{\"content\":\"i\"}}]}\n\n" +
				"data: {\"choices\":[],\"usage\":{\"prompt_tokens\":90,\"completion_tokens\":7}}\n\n" +
				"data: [DONE]\n\n",
			in: 90, out: 7, flushing: true,
		},
		{
			name: "stream that never reports usage",
			body: "data: {\"choices\":[{\"delta\":{\"content\":\"hei\"}}]}\n\ndata: [DONE]\n\n",
			in:   0, out: 0, flushing: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			tap := &usageTap{ResponseWriter: rec}
			if tc.flushing {
				for _, frame := range strings.SplitAfter(tc.body, "\n\n") {
					if frame == "" {
						continue
					}
					if _, err := tap.Write([]byte(frame)); err != nil {
						t.Fatal(err)
					}
					tap.Flush()
				}
			} else if _, err := tap.Write([]byte(tc.body)); err != nil {
				t.Fatal(err)
			}

			in, out := tap.usage()
			if in != tc.in || out != tc.out {
				t.Errorf("usage() = %d, %d; want %d, %d", in, out, tc.in, tc.out)
			}
			// Whatever the tap read, the client must have received all of it.
			if got := rec.Body.String(); got != tc.body {
				t.Errorf("the tap altered the response body")
			}
		})
	}
}

// TestUsageTapKeepsOnlyTheTail: a refactor's worth of generated code must not
// be held in memory to count two integers.
func TestUsageTapKeepsOnlyTheTail(t *testing.T) {
	rec := httptest.NewRecorder()
	tap := &usageTap{ResponseWriter: rec}

	big := strings.Repeat("x", 400<<10)
	if _, err := tap.Write([]byte(big)); err != nil {
		t.Fatal(err)
	}
	if len(tap.tail) > usageTailBytes {
		t.Errorf("tail grew to %d bytes, cap is %d", len(tap.tail), usageTailBytes)
	}
	// Usage arriving after all that is still found.
	trailer := `"usage":{"prompt_tokens":3,"completion_tokens":4}}`
	if _, err := tap.Write([]byte(trailer)); err != nil {
		t.Fatal(err)
	}
	if in, out := tap.usage(); in != 3 || out != 4 {
		t.Errorf("usage() = %d, %d after a large body; want 3, 4", in, out)
	}
	if want := len(big) + len(trailer); rec.Body.Len() != want {
		t.Errorf("the client received %d bytes, want %d — the tap dropped part of the response",
			rec.Body.Len(), want)
	}
}

// TestUsageTapForwardsFlush: the guard sets FlushInterval to -1 so tokens reach
// the client as they are generated. A tap that swallowed Flush would undo that
// and the client would time out mid-generation.
func TestUsageTapForwardsFlush(t *testing.T) {
	rec := httptest.NewRecorder()
	tap := &usageTap{ResponseWriter: rec}
	var _ http.Flusher = tap
	tap.Flush()
	if !rec.Flushed {
		t.Error("Flush did not reach the underlying writer")
	}
}

func appendRaw(t *testing.T, line string) {
	t.Helper()
	f, err := openAppend(statsPath())
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := fmt.Fprintln(f, line); err != nil {
		t.Fatal(err)
	}
}
