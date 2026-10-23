package provider

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"syscall"
)

// ideTerminalEnv marks a JetBrains terminal. cplt strips it by default.
const ideTerminalEnv = "TERMINAL_EMULATOR"

// ideSocketShape is the IDE plugin's socket, relative to the temp dir. The
// agent can write ~/.copilot/ide, so only this shape is granted.
var ideSocketShape = regexp.MustCompile(`^github-copilot-[^/]+/\.copilot-ide-[^/]+/m\.sock$`)

// ideLock is the part of a ~/.copilot/ide/*.lock file we read.
type ideLock struct {
	Scheme     string `json:"scheme"`
	SocketPath string `json:"socketPath"`
	PID        int    `json:"pid"`
}

// ideBridgeFlags are the cplt flags that let Copilot CLI reach the IDE.
// Not enough until github/copilot-cli#4909 is fixed.
func ideBridgeFlags() []string {
	var flags []string
	if os.Getenv(ideTerminalEnv) != "" {
		flags = append(flags, "--pass-env", ideTerminalEnv)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return flags
	}
	for _, sock := range ideSockets(filepath.Join(home, ".copilot", "ide"), os.TempDir(), os.Getuid(), pidAlive) {
		flags = append(flags, "--allow-socket", sock)
	}
	return flags
}

// Bounds on lockDir, which the agent can fill. A real lock is ~400 bytes.
const (
	maxIDELockEntries = 256
	maxIDELockBytes   = 4096
	maxIDESockets     = 8
)

// ideSockets returns the sockets of live IDE lock files in lockDir.
func ideSockets(lockDir, tmpDir string, uid int, alive func(int) bool) []string {
	tmp, err := filepath.EvalSymlinks(tmpDir)
	if err != nil {
		return nil
	}
	dir, err := os.Open(lockDir)
	if err != nil {
		return nil
	}
	entries, _ := dir.ReadDir(maxIDELockEntries)
	dir.Close()
	var socks []string
	for _, e := range entries {
		if len(socks) == maxIDESockets {
			break
		}
		if !e.Type().IsRegular() || filepath.Ext(e.Name()) != ".lock" {
			continue
		}
		data, err := readIDELock(filepath.Join(lockDir, e.Name()))
		if err != nil {
			continue
		}
		var l ideLock
		if json.Unmarshal(data, &l) != nil || l.Scheme != "unix" || l.PID <= 0 || !alive(l.PID) {
			continue
		}
		sock, err := filepath.EvalSymlinks(l.SocketPath)
		if err != nil {
			continue
		}
		rel, err := filepath.Rel(tmp, sock)
		if err != nil || !ideSocketShape.MatchString(filepath.ToSlash(rel)) {
			continue
		}
		fi, err := os.Lstat(sock)
		if err != nil || fi.Mode().Type() != os.ModeSocket {
			continue
		}
		if st, ok := fi.Sys().(*syscall.Stat_t); !ok || int(st.Uid) != uid {
			continue
		}
		if !slices.Contains(socks, sock) {
			socks = append(socks, sock)
		}
	}
	return socks
}

// readIDELock reads a lock file without following a link or blocking on a
// fifo, and refuses anything but a small regular file.
func readIDELock(path string) ([]byte, error) {
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	if fi, err := f.Stat(); err != nil || !fi.Mode().IsRegular() {
		return nil, errors.New("not a regular file")
	}
	data, err := io.ReadAll(io.LimitReader(f, maxIDELockBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxIDELockBytes {
		return nil, errors.New("lock file too large")
	}
	return data, nil
}

// pidAlive reports whether pid is running. EPERM counts as running.
func pidAlive(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}
