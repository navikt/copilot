package provider

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"slices"
	"syscall"
	"testing"
	"time"
)

// shortTempDir keeps socket paths under macOS's 104-byte limit.
func shortTempDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "ide")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return dir
}

func listenUnix(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	l, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { l.Close() })
}

func writeLock(t *testing.T, dir, name string, l ideLock) {
	t.Helper()
	data, err := json.Marshal(l)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name+".lock"), data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestIDESockets(t *testing.T) {
	tmp := shortTempDir(t)
	canonTmp, err := filepath.EvalSymlinks(tmp)
	if err != nil {
		t.Fatal(err)
	}
	// Glob metacharacters in the path must not hide the lock files.
	lockDir := filepath.Join(t.TempDir(), "a[b")
	if err := os.Mkdir(lockDir, 0o700); err != nil {
		t.Fatal(err)
	}
	const livePID, deadPID = 100, 200
	alive := func(pid int) bool { return pid == livePID }

	good := filepath.Join(tmp, "github-copilot-a", ".copilot-ide-1", "m.sock")
	listenUnix(t, good)
	writeLock(t, lockDir, "good", ideLock{Scheme: "unix", SocketPath: good, PID: livePID})

	dead := filepath.Join(tmp, "github-copilot-b", ".copilot-ide-2", "m.sock")
	listenUnix(t, dead)
	writeLock(t, lockDir, "dead", ideLock{Scheme: "unix", SocketPath: dead, PID: deadPID})

	// A forged lock file naming another socket.
	other := filepath.Join(tmp, "docker.sock")
	listenUnix(t, other)
	writeLock(t, lockDir, "other", ideLock{Scheme: "unix", SocketPath: other, PID: livePID})

	writeLock(t, lockDir, "tcp", ideLock{Scheme: "tcp", SocketPath: good, PID: livePID})

	notSock := filepath.Join(tmp, "github-copilot-c", ".copilot-ide-3", "m.sock")
	if err := os.MkdirAll(filepath.Dir(notSock), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(notSock, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	writeLock(t, lockDir, "file", ideLock{Scheme: "unix", SocketPath: notSock, PID: livePID})

	if err := os.WriteFile(filepath.Join(lockDir, "broken.lock"), []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}

	got := ideSockets(lockDir, tmp, os.Getuid(), alive)
	want := []string{filepath.Join(canonTmp, "github-copilot-a", ".copilot-ide-1", "m.sock")}
	if !slices.Equal(got, want) {
		t.Fatalf("ideSockets = %q, want %q", got, want)
	}

	if got := ideSockets(lockDir, tmp, os.Getuid()+1, alive); len(got) != 0 {
		t.Errorf("socket owned by another uid granted: %q", got)
	}
}

func TestPidAlive(t *testing.T) {
	if !pidAlive(os.Getpid()) {
		t.Error("own pid reported dead")
	}
	// pid 1 is root's: EPERM.
	if os.Getuid() != 0 && !pidAlive(1) {
		t.Error("EPERM pid reported dead")
	}
	if pidAlive(1 << 30) {
		t.Error("nonexistent pid reported alive")
	}
}

func TestIDESocketsHostileLockDir(t *testing.T) {
	tmp := shortTempDir(t)
	lockDir := t.TempDir()
	alive := func(int) bool { return true }

	sock := filepath.Join(tmp, "github-copilot-a", ".copilot-ide-1", "m.sock")
	listenUnix(t, sock)
	lock := ideLock{Scheme: "unix", SocketPath: sock, PID: 1}
	for _, name := range []string{"a", "b", "c"} {
		writeLock(t, lockDir, name, lock)
	}
	if err := os.Symlink("/dev/zero", filepath.Join(lockDir, "zero.lock")); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(filepath.Join(lockDir, "fifo.lock"), 0o600); err != nil {
		t.Fatal(err)
	}
	big := make([]byte, maxIDELockBytes+1)
	if err := os.WriteFile(filepath.Join(lockDir, "big.lock"), big, 0o600); err != nil {
		t.Fatal(err)
	}

	done := make(chan []string, 1)
	go func() { done <- ideSockets(lockDir, tmp, os.Getuid(), alive) }()
	select {
	case got := <-done:
		if len(got) != 1 {
			t.Fatalf("duplicate locks not collapsed: %q", got)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("ideSockets hung on a hostile lock dir")
	}

	for i := range maxIDESockets + 2 {
		s := filepath.Join(tmp, fmt.Sprintf("github-copilot-%d", i), ".copilot-ide-1", "m.sock")
		listenUnix(t, s)
		writeLock(t, lockDir, fmt.Sprintf("n%d", i), ideLock{Scheme: "unix", SocketPath: s, PID: 1})
	}
	if got := ideSockets(lockDir, tmp, os.Getuid(), alive); len(got) != maxIDESockets {
		t.Errorf("got %d sockets, want the cap of %d", len(got), maxIDESockets)
	}
}
