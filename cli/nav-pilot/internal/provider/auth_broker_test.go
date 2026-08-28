package provider

import (
	"bytes"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestTokenBroker_OneShotServe(t *testing.T) {
	b, err := newTokenBroker()
	if err != nil {
		t.Skipf("skipping: Unix socket unavailable in this environment: %v", err)
	}
	defer b.Close()

	token := []byte("ghu_brokertest")
	errCh := make(chan error, 1)
	go func() {
		errCh <- b.Serve(token, 2*time.Second)
	}()

	got, err := ReadFromBroker(b.SocketPath(), 2*time.Second)
	if err != nil {
		t.Fatalf("ReadFromBroker: %v", err)
	}
	if string(got) != "ghu_brokertest" {
		t.Errorf("got token %q, want %q", got, "ghu_brokertest")
	}

	if err := <-errCh; err != nil {
		t.Errorf("Serve returned error: %v", err)
	}
}

func TestTokenBroker_ZeroizesAfterServe(t *testing.T) {
	b, err := newTokenBroker()
	if err != nil {
		t.Skipf("skipping: Unix socket unavailable in this environment: %v", err)
	}
	defer b.Close()

	token := []byte("ghu_zeroize_me")
	errCh := make(chan error, 1)
	go func() {
		errCh <- b.Serve(token, 2*time.Second)
	}()

	_, err = ReadFromBroker(b.SocketPath(), 2*time.Second)
	if err != nil {
		t.Fatalf("ReadFromBroker: %v", err)
	}
	<-errCh

	// After Serve returns, the original token slice should be zeroized.
	for i, b := range token {
		if b != 0 {
			t.Errorf("token[%d] = %d, want 0 (not zeroized)", i, b)
		}
	}
}

func TestTokenBroker_Timeout(t *testing.T) {
	b, err := newTokenBroker()
	if err != nil {
		t.Skipf("skipping: Unix socket unavailable in this environment: %v", err)
	}
	defer b.Close()

	// Very short timeout — no client connects.
	err = b.Serve([]byte("ghu_timeout"), 50*time.Millisecond)
	if err == nil {
		t.Fatal("expected timeout error when no client connects")
	}
}

func TestTokenBroker_RejectsOversizedToken(t *testing.T) {
	b, err := newTokenBroker()
	if err != nil {
		t.Skipf("skipping: Unix socket unavailable in this environment: %v", err)
	}
	defer b.Close()

	token := make([]byte, maxBrokerTokenSize+1)
	for i := range token {
		token[i] = 'a'
	}

	err = b.Serve(token, 50*time.Millisecond)
	if err == nil {
		t.Fatal("expected oversized token to be rejected")
	}
	if got := err.Error(); got == "" || !strings.Contains(got, "exceeds maximum size") {
		t.Fatalf("expected max-size error, got %v", err)
	}
	for i, b := range token {
		if b != 0 {
			t.Fatalf("token[%d] = %d, want 0", i, b)
		}
	}
}

func TestTokenBroker_AcceptsTokenAtExactSizeLimit(t *testing.T) {
	b, err := newTokenBroker()
	if err != nil {
		t.Skipf("skipping: Unix socket unavailable in this environment: %v", err)
	}
	defer b.Close()

	token := bytes.Repeat([]byte("a"), maxBrokerTokenSize)
	errCh := make(chan error, 1)
	go func() {
		errCh <- b.Serve(token, 2*time.Second)
	}()

	got, err := ReadFromBroker(b.SocketPath(), 2*time.Second)
	if err != nil {
		t.Fatalf("ReadFromBroker: %v", err)
	}
	if len(got) != maxBrokerTokenSize {
		t.Fatalf("token size = %d, want %d", len(got), maxBrokerTokenSize)
	}
	if err := <-errCh; err != nil {
		t.Fatalf("Serve returned error: %v", err)
	}
}

func TestTokenBroker_RejectsEmptyToken(t *testing.T) {
	b, err := newTokenBroker()
	if err != nil {
		t.Skipf("skipping: Unix socket unavailable in this environment: %v", err)
	}
	defer b.Close()

	err = b.Serve([]byte{}, 50*time.Millisecond)
	if err == nil {
		t.Fatal("expected empty token to be rejected")
	}
	if !strings.Contains(err.Error(), "must not be empty") {
		t.Fatalf("expected empty-token validation error, got: %v", err)
	}
}

func TestTokenBroker_SocketPermissions(t *testing.T) {
	b, err := newTokenBroker()
	if err != nil {
		t.Skipf("skipping: Unix socket unavailable in this environment: %v", err)
	}
	defer b.Close()

	// The parent directory should be mode 0700.
	socketDir := filepath.Dir(b.SocketPath())
	info, err := os.Stat(socketDir)
	if err != nil {
		t.Skipf("cannot stat socket dir: %v", err)
	}
	perm := info.Mode().Perm()
	if perm != 0o700 {
		t.Errorf("socket dir permissions = %04o, want 0700", perm)
	}
}

func TestTokenBroker_Cleanup(t *testing.T) {
	b, err := newTokenBroker()
	if err != nil {
		t.Skipf("skipping: Unix socket unavailable in this environment: %v", err)
	}
	socketPath := b.SocketPath()
	b.Close()

	if _, err := os.Stat(socketPath); !os.IsNotExist(err) {
		t.Error("expected socket path to be removed after Close()")
	}
}

func TestReadFromBroker_NoServer(t *testing.T) {
	_, err := ReadFromBroker("/tmp/nav-pilot-nonexistent-test.sock", 100*time.Millisecond)
	if err == nil {
		t.Fatal("expected error connecting to non-existent socket")
	}
}

func TestReadFromBroker_RejectsTruncatedToken(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix sockets are not supported on windows")
	}

	dir := t.TempDir()
	socketPath := filepath.Join(dir, "truncated.sock")
	ln, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Skipf("unix socket unavailable in this environment: %v", err)
	}
	defer ln.Close()

	done := make(chan error, 1)
	go func() {
		conn, acceptErr := ln.Accept()
		if acceptErr != nil {
			done <- acceptErr
			return
		}
		defer conn.Close()
		_, writeErr := conn.Write([]byte("ghu_truncated_no_newline"))
		done <- writeErr
	}()

	_, err = ReadFromBroker(socketPath, time.Second)
	if err == nil {
		_ = ln.Close()
		t.Fatal("expected error for truncated token without newline terminator")
	}

	select {
	case gorErr := <-done:
		if gorErr != nil && !errors.Is(gorErr, net.ErrClosed) {
			t.Fatalf("unexpected server goroutine error: %v", gorErr)
		}
	case <-time.After(time.Second):
		_ = ln.Close()
		t.Fatal("timed out waiting for server goroutine to finish")
	}
}

func TestReadFromBroker_TokenSplitAcrossReads_Succeeds(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix sockets are not supported on windows")
	}

	dir := t.TempDir()
	socketPath := filepath.Join(dir, "split.sock")
	ln, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Skipf("unix socket unavailable in this environment: %v", err)
	}
	defer ln.Close()

	done := make(chan error, 1)
	go func() {
		conn, acceptErr := ln.Accept()
		if acceptErr != nil {
			done <- acceptErr
			return
		}
		defer conn.Close()

		chunks := []string{"ghu_", "split_", "token", "\n"}
		for _, c := range chunks {
			if _, writeErr := conn.Write([]byte(c)); writeErr != nil {
				done <- writeErr
				return
			}
		}
		done <- nil
	}()

	got, err := ReadFromBroker(socketPath, time.Second)
	if err != nil {
		t.Fatalf("ReadFromBroker returned error: %v", err)
	}
	if string(got) != "ghu_split_token" {
		t.Fatalf("got token %q, want %q", got, "ghu_split_token")
	}

	select {
	case gorErr := <-done:
		if gorErr != nil && !errors.Is(gorErr, net.ErrClosed) {
			t.Fatalf("unexpected server goroutine error: %v", gorErr)
		}
	case <-time.After(time.Second):
		_ = ln.Close()
		t.Fatal("timed out waiting for server goroutine to finish")
	}
}

func TestReadFromBroker_EmptyConnection_Fails(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix sockets are not supported on windows")
	}

	dir := t.TempDir()
	socketPath := filepath.Join(dir, "empty.sock")
	ln, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Skipf("unix socket unavailable in this environment: %v", err)
	}
	defer ln.Close()

	done := make(chan error, 1)
	go func() {
		conn, acceptErr := ln.Accept()
		if acceptErr != nil {
			done <- acceptErr
			return
		}
		_ = conn.Close()
		done <- nil
	}()

	_, err = ReadFromBroker(socketPath, time.Second)
	if err == nil {
		t.Fatal("expected error for empty connection")
	}
	if !strings.Contains(err.Error(), "stream closed before token terminator") {
		t.Fatalf("expected EOF-before-terminator error, got: %v", err)
	}

	select {
	case gorErr := <-done:
		if gorErr != nil && !errors.Is(gorErr, net.ErrClosed) {
			t.Fatalf("unexpected server goroutine error: %v", gorErr)
		}
	case <-time.After(time.Second):
		_ = ln.Close()
		t.Fatal("timed out waiting for server goroutine to finish")
	}
}

func TestReadFromBroker_RejectsOversizedToken(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix sockets are not supported on windows")
	}

	dir := t.TempDir()
	socketPath := filepath.Join(dir, "oversized.sock")
	ln, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Skipf("unix socket unavailable in this environment: %v", err)
	}
	defer ln.Close()

	done := make(chan error, 1)
	go func() {
		conn, acceptErr := ln.Accept()
		if acceptErr != nil {
			done <- acceptErr
			return
		}
		defer conn.Close()

		payload := make([]byte, maxBrokerTokenSize+1)
		for i := range payload {
			payload[i] = 'a'
		}
		_, writeErr := conn.Write(payload)
		done <- writeErr
	}()

	_, err = ReadFromBroker(socketPath, time.Second)
	if err == nil {
		_ = ln.Close()
		t.Fatal("expected error for oversized token")
	}
	if got := err.Error(); got == "" {
		t.Fatal("expected a non-empty error")
	}

	select {
	case gorErr := <-done:
		if gorErr != nil && !errors.Is(gorErr, net.ErrClosed) {
			t.Fatalf("unexpected server goroutine error: %v", gorErr)
		}
	case <-time.After(time.Second):
		_ = ln.Close()
		t.Fatal("timed out waiting for server goroutine to finish")
	}
}

func TestReadFromBroker_UnexpectedReadError_Fails(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix sockets are not supported on windows")
	}

	dir := t.TempDir()
	socketPath := filepath.Join(dir, "read-timeout.sock")
	ln, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Skipf("unix socket unavailable in this environment: %v", err)
	}
	defer ln.Close()

	done := make(chan error, 1)
	go func() {
		conn, acceptErr := ln.Accept()
		if acceptErr != nil {
			done <- acceptErr
			return
		}
		defer conn.Close()
		// Keep the connection open without sending data so the client read deadline expires.
		time.Sleep(250 * time.Millisecond)
		done <- nil
	}()

	_, err = ReadFromBroker(socketPath, 50*time.Millisecond)
	if err == nil {
		t.Fatal("expected read error when server does not send token data")
	}
	if !strings.Contains(err.Error(), "broker client: read error") {
		t.Fatalf("expected read-error wrapper, got: %v", err)
	}

	select {
	case gorErr := <-done:
		if gorErr != nil && !errors.Is(gorErr, net.ErrClosed) {
			t.Fatalf("unexpected server goroutine error: %v", gorErr)
		}
	case <-time.After(time.Second):
		_ = ln.Close()
		t.Fatal("timed out waiting for server goroutine to finish")
	}
}

type chunkWriter struct {
	buf   bytes.Buffer
	chunk int
}

func (w *chunkWriter) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	n := w.chunk
	if n <= 0 || n > len(p) {
		n = len(p)
	}
	_, _ = w.buf.Write(p[:n])
	return n, nil
}

type failAfterCallsWriter struct {
	buf        bytes.Buffer
	chunk      int
	call       int
	failOnCall int
	err        error
}

func (w *failAfterCallsWriter) Write(p []byte) (int, error) {
	w.call++
	if w.call == w.failOnCall {
		return 0, w.err
	}
	if len(p) == 0 {
		return 0, nil
	}
	n := w.chunk
	if n <= 0 || n > len(p) {
		n = len(p)
	}
	_, _ = w.buf.Write(p[:n])
	return n, nil
}

type zeroProgressWriter struct{}

func (zeroProgressWriter) Write(_ []byte) (int, error) {
	return 0, nil
}

func TestWriteAll_PartialWrites(t *testing.T) {
	payload := []byte("ghu_partial_write_token")
	w := &chunkWriter{chunk: 3}

	if err := writeAll(w, payload); err != nil {
		t.Fatalf("writeAll returned error: %v", err)
	}
	if got := w.buf.String(); got != string(payload) {
		t.Fatalf("written payload = %q, want %q", got, string(payload))
	}
}

func TestWriteAll_ErrorAfterPartialWrite(t *testing.T) {
	payload := []byte("ghu_partial_then_error")
	wantErr := errors.New("boom")
	w := &failAfterCallsWriter{
		chunk:      4,
		failOnCall: 3,
		err:        wantErr,
	}

	err := writeAll(w, payload)
	if !errors.Is(err, wantErr) {
		t.Fatalf("writeAll error = %v, want %v", err, wantErr)
	}
	if got := w.buf.String(); got == "" {
		t.Fatal("expected partial payload to be written before failure")
	}
	if got := w.buf.String(); got == string(payload) {
		t.Fatal("expected truncated payload on write failure, got full payload")
	}
}

func TestWriteAll_ZeroProgressReturnsShortWrite(t *testing.T) {
	payload := []byte("ghu_no_progress")
	err := writeAll(zeroProgressWriter{}, payload)
	if !errors.Is(err, io.ErrShortWrite) {
		t.Fatalf("writeAll error = %v, want %v", err, io.ErrShortWrite)
	}
}

func TestZeroize(t *testing.T) {
	buf := []byte("sensitive-token-data")
	zeroize(buf)
	for i, b := range buf {
		if b != 0 {
			t.Errorf("buf[%d] = %d, want 0", i, b)
		}
	}
}
