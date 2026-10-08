package provider

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
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

// ideSockets returns the sockets of live IDE lock files in lockDir.
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

// pidAlive reports whether pid is running. EPERM counts as running.
func pidAlive(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}
