package provider

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"syscall"
)

// ideTerminalEnv is how Copilot CLI tells it runs in a JetBrains terminal.
// cplt's env allowlist carries TERM_PROGRAM (VS Code) but not this one.
const ideTerminalEnv = "TERMINAL_EMULATOR"

// ideSocketShape is the socket an IDE's Copilot plugin listens on, relative to
// the temp dir: github-copilot-<rand>/.copilot-ide-<id>/m.sock. The lock file
// that names it lives in ~/.copilot/ide, which the sandboxed agent can write,
// so a lock file is never trusted to name an arbitrary socket: only this shape
// in the user's own temp dir is granted.
var ideSocketShape = regexp.MustCompile(`^github-copilot-[^/]+/\.copilot-ide-[^/]+/m\.sock$`)

// ideLock is the part of a ~/.copilot/ide/*.lock file the bridge reads.
type ideLock struct {
	Scheme     string `json:"scheme"`
	SocketPath string `json:"socketPath"`
	PID        int    `json:"pid"`
}

// ideBridgeFlags are the cplt flags that let Copilot CLI reach the IDE it was
// started from (file and selection context, diffs in the IDE). Without them
// cplt blocks the IDE's Unix socket and strips TERMINAL_EMULATOR.
//
// Not enough on its own today: Copilot CLI drops every lock file whose pid it
// cannot signal, and cplt only allows signals inside the sandbox
// (github/copilot-cli#4909). The flags are in place for when that is fixed.
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

// ideSockets returns the canonical socket paths of the live IDE lock files in
// lockDir: unix scheme, a running pid, and a socket owned by uid in the
// ideSocketShape under tmpDir.
func ideSockets(lockDir, tmpDir string, uid int, alive func(int) bool) []string {
	tmp, err := filepath.EvalSymlinks(tmpDir)
	if err != nil {
		return nil
	}
	locks, _ := filepath.Glob(filepath.Join(lockDir, "*.lock"))
	var socks []string
	for _, f := range locks {
		data, err := os.ReadFile(f)
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
		socks = append(socks, sock)
	}
	return socks
}

// pidAlive reports whether pid is running. EPERM means it exists but is not
// ours to signal, which is still alive.
func pidAlive(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}
