package proxy

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type fakeSSEWriter struct {
	header       http.Header
	mu           sync.Mutex
	buf          bytes.Buffer
	statusCode   int
	flushes      atomic.Int64
	flushFn      func(callNum int64) error
	deadlineOnce sync.Once
	deadlineCh   chan struct{}
}

func newFakeSSEWriter(flushFn func(callNum int64) error) *fakeSSEWriter {
	return &fakeSSEWriter{
		header:     make(http.Header),
		flushFn:    flushFn,
		deadlineCh: make(chan struct{}),
	}
}

func (f *fakeSSEWriter) Header() http.Header {
	return f.header
}

func (f *fakeSSEWriter) Write(p []byte) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.buf.Write(p)
}

func (f *fakeSSEWriter) WriteHeader(statusCode int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.statusCode = statusCode
}

func (f *fakeSSEWriter) FlushError() error {
	n := f.flushes.Add(1)
	if f.flushFn != nil {
		return f.flushFn(n)
	}
	return nil
}

func (f *fakeSSEWriter) SetWriteDeadline(_ time.Time) error {
	f.deadlineOnce.Do(func() {
		close(f.deadlineCh)
	})
	return nil
}

type blockingReadCloser struct {
	closeOnce sync.Once
	closed    chan struct{}
}

func newBlockingReadCloser() *blockingReadCloser {
	return &blockingReadCloser{closed: make(chan struct{})}
}

func (b *blockingReadCloser) Read(_ []byte) (int, error) {
	<-b.closed
	return 0, errors.New("body closed")
}

func (b *blockingReadCloser) Close() error {
	b.closeOnce.Do(func() {
		close(b.closed)
	})
	return nil
}

type plainWriter struct {
	header http.Header
	buf    bytes.Buffer
}

func (p *plainWriter) Header() http.Header {
	if p.header == nil {
		p.header = make(http.Header)
	}
	return p.header
}

func (p *plainWriter) Write(b []byte) (int, error) {
	return p.buf.Write(b)
}

func (p *plainWriter) WriteHeader(_ int) {}

func TestStreamSSEFlushError(t *testing.T) {
	wantErr := errors.New("broken pipe")

	t.Run("flush error on initial flush", func(t *testing.T) {
		w := newFakeSSEWriter(func(_ int64) error {
			return wantErr
		})
		rc := http.NewResponseController(w)
		body := io.NopCloser(strings.NewReader("data: x\n\n"))

		err := streamSSE(context.Background(), w, rc, body, time.Second, time.Second, 0)
		if !errors.Is(err, wantErr) {
			t.Fatalf("streamSSE() error = %v, want %v", err, wantErr)
		}
	})

	t.Run("flush error after body write before EOF", func(t *testing.T) {
		w := newFakeSSEWriter(func(callNum int64) error {
			if callNum == 1 {
				return nil
			}
			return wantErr
		})
		rc := http.NewResponseController(w)
		body := io.NopCloser(strings.NewReader("data: x\n\n"))

		err := streamSSE(context.Background(), w, rc, body, time.Second, time.Second, 0)
		if !errors.Is(err, wantErr) {
			t.Fatalf("streamSSE() error = %v, want %v", err, wantErr)
		}
		if got := w.flushes.Load(); got < 2 {
			t.Fatalf("flush count = %d, want at least 2", got)
		}
	})

	t.Run("flush error takes precedence over max bytes error", func(t *testing.T) {
		w := newFakeSSEWriter(func(callNum int64) error {
			if callNum == 1 {
				return nil
			}
			return wantErr
		})
		rc := http.NewResponseController(w)
		body := io.NopCloser(strings.NewReader("data: x\n\n"))

		err := streamSSE(context.Background(), w, rc, body, time.Second, time.Second, 4)
		if !errors.Is(err, wantErr) {
			t.Fatalf("streamSSE() error = %v, want %v", err, wantErr)
		}
	})

	t.Run("http.ErrNotSupported is ignored", func(t *testing.T) {
		w := &plainWriter{}
		rc := http.NewResponseController(w)
		body := io.NopCloser(strings.NewReader("data: x\n\n"))

		if err := streamSSE(context.Background(), w, rc, body, time.Second, time.Second, 0); err != nil {
			t.Fatalf("streamSSE() error = %v, want nil", err)
		}
		if got := w.buf.String(); got != "data: x\n\n" {
			t.Fatalf("written body = %q, want %q", got, "data: x\n\n")
		}
	})

	t.Run("nil ResponseController is safe", func(t *testing.T) {
		w := &plainWriter{}
		body := io.NopCloser(strings.NewReader("data: x\n\n"))

		if err := streamSSE(context.Background(), w, nil, body, time.Second, time.Second, 0); err != nil {
			t.Fatalf("streamSSE() error = %v, want nil", err)
		}
	})
}

func TestStreamSSEInitialFlushRespectsMaxDuration(t *testing.T) {
	var w *fakeSSEWriter
	w = newFakeSSEWriter(func(_ int64) error {
		<-w.deadlineCh
		return os.ErrDeadlineExceeded
	})
	defer func() { _ = w.SetWriteDeadline(time.Time{}) }()

	rc := http.NewResponseController(w)
	body := newBlockingReadCloser()
	defer func() { _ = body.Close() }()

	errCh := make(chan error, 1)
	go func() {
		errCh <- streamSSE(context.Background(), w, rc, body, 50*time.Millisecond, 0, 0)
	}()

	select {
	case err := <-errCh:
		if err == nil {
			t.Fatal("streamSSE() error = nil, want non-nil error")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("streamSSE() did not return within 2s after maxDuration elapsed")
	}

	if got := w.flushes.Load(); got < 1 {
		t.Fatalf("flush count = %d, want at least 1", got)
	}
}
