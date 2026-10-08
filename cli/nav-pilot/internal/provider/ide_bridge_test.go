package provider

import (
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"slices"
	"testing"
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
	lockDir := t.TempDir()
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
