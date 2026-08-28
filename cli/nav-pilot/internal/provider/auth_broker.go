package provider

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const maxBrokerTokenSize = 8 * 1024

// tokenBroker is a one-shot Unix-domain-socket server. It accepts exactly one
// client connection, sends the token, reads an ACK, then closes. The token
// buffer is zeroized after sending.
//
// Security properties:
//   - The socket is created in a mode-0700 temporary directory owned by the
//     current user, so other users cannot reach it.
//   - The token is never written to disk; it travels only through the socket.
//   - The server accepts only one connection; subsequent attempts are rejected.
//   - Zeroizing the buffer reduces the window during which the token is in
//     memory, though it is not a hard security boundary against agents running
//     as the same UID.
type tokenBroker struct {
	socketPath string
	listener   net.Listener
	mu         sync.Mutex
	served     bool
}

// newTokenBroker creates a Unix-socket server in a private temp directory and
// starts listening. Call Serve to block until one client is handled (or timeout),
// then call Close to clean up.
func newTokenBroker() (*tokenBroker, error) {
	dir, err := os.MkdirTemp("", "nav-pilot-auth-*")
	if err != nil {
		return nil, fmt.Errorf("token broker: could not create socket dir: %w", err)
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		_ = os.RemoveAll(dir)
		return nil, fmt.Errorf("token broker: could not secure socket dir: %w", err)
	}

	socketPath := filepath.Join(dir, "token.sock")
	ln, err := net.Listen("unix", socketPath)
	if err != nil {
		_ = os.RemoveAll(dir)
		return nil, fmt.Errorf("token broker: could not listen on socket: %w", err)
	}

	return &tokenBroker{
		socketPath: socketPath,
		listener:   ln,
	}, nil
}

// SocketPath returns the path to the Unix socket. Pass this to the sandboxed
// process via an environment variable so the gh-wrapper can connect.
func (b *tokenBroker) SocketPath() string {
	return b.socketPath
}

// Serve waits for exactly one client connection, sends token, reads the "ok\n"
// ACK, then returns. If the timeout elapses before a client connects, it returns
// an error. The token slice is zeroized before Serve returns.
func (b *tokenBroker) Serve(token []byte, timeout time.Duration) error {
	defer zeroize(token)

	if len(token) == 0 {
		return errors.New("token broker: token must not be empty")
	}
	if len(token) > maxBrokerTokenSize {
		return fmt.Errorf("token broker: token exceeds maximum size of %d bytes", maxBrokerTokenSize)
	}

	if ul, ok := b.listener.(*net.UnixListener); ok {
		if err := ul.SetDeadline(time.Now().Add(timeout)); err != nil {
			return fmt.Errorf("token broker: could not set deadline: %w", err)
		}
	}

	conn, err := b.listener.Accept()
	if err != nil {
		return fmt.Errorf("token broker: no client connected within %s: %w", timeout, err)
	}
	// Stop listening immediately after the first connection to keep the broker one-shot.
	_ = b.listener.Close()
	defer conn.Close()
	b.mu.Lock()
	if b.served {
		b.mu.Unlock()
		return errors.New("token broker: already served one client; refusing second connection")
	}
	b.served = true
	b.mu.Unlock()

	// Write token followed by newline so the reader knows where it ends.
	// Use an explicit allocation to avoid append mutating token's backing array.
	payload := make([]byte, len(token)+1)
	copy(payload, token)
	payload[len(token)] = '\n'
	// Wipe the copy unconditionally: a failed or partial write must not leave
	// the token sitting in heap memory until GC.
	defer zeroize(payload)
	if err := writeAll(conn, payload); err != nil {
		return fmt.Errorf("token broker: failed to send token: %w", err)
	}

	// Read ACK from client ("ok\n"). Set a short read deadline.
	if tc, ok := conn.(*net.UnixConn); ok {
		_ = tc.SetReadDeadline(time.Now().Add(2 * time.Second))
	}
	ack := make([]byte, 3)
	n, _ := io.ReadAtLeast(conn, ack, 1)
	_ = n // ACK is best-effort; we already sent the token.

	return nil
}

// Close closes the listener and removes the socket directory.
func (b *tokenBroker) Close() {
	_ = b.listener.Close()
	_ = os.RemoveAll(filepath.Dir(b.socketPath))
}

// ReadFromBroker connects to the broker socket, reads the token line, sends ACK,
// and returns the token. Used by the gh-wrapper / tests to consume the broker.
func ReadFromBroker(socketPath string, timeout time.Duration) ([]byte, error) {
	conn, err := net.DialTimeout("unix", socketPath, timeout)
	if err != nil {
		return nil, fmt.Errorf("broker client: could not connect to %s: %w", socketPath, err)
	}
	defer conn.Close()

	if tc, ok := conn.(*net.UnixConn); ok {
		_ = tc.SetReadDeadline(time.Now().Add(timeout))
	}

	// Read until newline.
	var buf []byte
	oneByte := make([]byte, 1)
	for {
		n, err := conn.Read(oneByte)
		if n > 0 {
			if oneByte[0] == '\n' {
				break
			}
			if len(buf) >= maxBrokerTokenSize {
				return nil, fmt.Errorf("broker client: token exceeds maximum size of %d bytes", maxBrokerTokenSize)
			}
			buf = append(buf, oneByte[0])
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil, errors.New("broker client: stream closed before token terminator")
			}
			return nil, fmt.Errorf("broker client: read error: %w", err)
		}
	}

	// Send ACK.
	_, _ = conn.Write([]byte("ok\n"))

	if len(buf) == 0 {
		return nil, errors.New("broker client: received empty token")
	}
	return buf, nil
}

// zeroize overwrites a byte slice with zeros to reduce the window during which
// a token is held in memory.
func zeroize(b []byte) {
	for i := range b {
		b[i] = 0
	}
}

func writeAll(w io.Writer, payload []byte) error {
	total := 0
	for total < len(payload) {
		n, err := w.Write(payload[total:])
		if n > 0 {
			total += n
		}
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrShortWrite
		}
	}
	return nil
}
