package local

// Adopting a server another nav-pilot invocation started.
//
// `alpha local start`, `stop` and `status` are three separate processes, and
// the server has to outlive all three — a developer starts it once and launches
// clients against it for the rest of the day. So start records what it launched
// and exits, and the later commands adopt that process rather than each
// reimplementing "is it alive, is it answering".
//
// [Attach] returns a [Server], not a second status type, so the five health
// states runtime.go models are classified by the same code for an adopted
// server as for one this process started. The only thing an adopted process
// cannot report is *how* it died: waitpid belongs to the parent, so a death is
// observed by polling and carries no signal or exit code.
//
// There is no supervisor goroutine here, unlike [Server.Start]. Liveness is
// decided once, when the process is adopted, because these commands live for a
// second or two: a background poll would only ever catch a process that died
// inside that second, and would outlive the command that started it. The poll
// exists for the one caller that needs it — Stop, waiting out its grace period
// after the SIGTERM.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

// State is what start leaves behind for stop and status to find.
type State struct {
	PID     int       `json:"pid"`
	Model   string    `json:"model"`
	Port    int       `json:"port"`
	Started time.Time `json:"started"`
}

func statePath() string { return filepath.Join(dataDir(), "server.json") }

// SaveState records a running server.
func SaveState(s State) error {
	data, err := json.Marshal(s)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dataDir(), 0o755); err != nil {
		return fmt.Errorf("recording the local server: %w", err)
	}
	if err := os.WriteFile(statePath(), data, 0o644); err != nil {
		return fmt.Errorf("recording the local server: %w", err)
	}
	return nil
}

// LoadState returns the recorded server, and false when none is recorded. A
// state file that cannot be read or parsed is reported as an error rather than
// as "nothing running": the process may well be running, and silently starting
// a second one on the same port is worse than saying so.
func LoadState() (State, bool, error) {
	var s State
	data, err := os.ReadFile(statePath())
	if err != nil {
		if os.IsNotExist(err) {
			return s, false, nil
		}
		return s, false, fmt.Errorf("reading %s: %w", statePath(), err)
	}
	if err := json.Unmarshal(data, &s); err != nil {
		return s, false, fmt.Errorf("%s is not readable as a recorded server: %w", statePath(), err)
	}
	return s, s.PID > 0, nil
}

// ClearState forgets the recorded server.
func ClearState() error {
	if err := os.Remove(statePath()); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("forgetting the local server: %w", err)
	}
	return nil
}

// alive reports whether a pid is a live process. EPERM counts as alive: a
// process this user may not signal is still a process, and reporting it dead
// would have status invite a second server onto the same port.
var alive = func(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}

// Attach adopts a recorded server so [Server.Health], [Server.Status] and
// [Server.Stop] work on it. A process that is already gone is adopted as
// crashed, decided before returning: a caller must not be told "starting"
// about a pid that no longer exists.
func Attach(s State) *Server {
	srv := &Server{Port: s.Port, model: s.Model, started: s.Started}
	p := &attachedProc{pid: s.PID, done: make(chan struct{})}
	srv.proc = p
	if !alive(s.PID) {
		info := exitInfo{Code: -1, Err: errors.New("the server process is gone")}
		p.settle(info)
		srv.exit = &info
	}
	return srv
}

// attachPollInterval is how often an adopted process is checked for liveness.
// A var so tests do not wait on it.
var attachPollInterval = 200 * time.Millisecond

// attachedProc is a process this program did not fork. It cannot be waited on,
// so death is observed by polling, and the exit carries no signal or code —
// only the parent gets those, and the parent has exited.
type attachedProc struct {
	pid   int
	watch sync.Once
	once  sync.Once
	info  exitInfo
	done  chan struct{}
}

func (p *attachedProc) PID() int { return p.pid }

func (p *attachedProc) Signal(sig os.Signal) error {
	s, ok := sig.(syscall.Signal)
	if !ok {
		return fmt.Errorf("cannot send %v to an adopted process", sig)
	}
	// Negative pid first: start put the server in its own process group, so
	// this reaches whatever it spawned rather than orphaning it.
	if err := syscall.Kill(-p.pid, s); err == nil {
		return nil
	}
	return syscall.Kill(p.pid, s)
}

func (p *attachedProc) Wait() exitInfo {
	p.watch.Do(func() {
		go func() {
			for alive(p.pid) {
				time.Sleep(attachPollInterval)
			}
			p.settle(exitInfo{Code: -1, Err: errors.New("the server process is gone")})
		}()
	})
	<-p.done
	return p.info
}

// settle records the exit exactly once, whether it was observed by polling or
// decided up front by [Attach].
func (p *attachedProc) settle(info exitInfo) {
	p.once.Do(func() {
		p.info = info
		close(p.done)
	})
}

// Installed reports whether the local-inference environment is provisioned to
// the versions this binary pins. It is the "installed" half of the opt-in: a
// developer who has never run init has no environment, so nothing about local
// inference may appear anywhere.
func Installed() bool {
	s, err := readStamp()
	return err == nil && s.MLXLM == mlxLMVersion && s.MLX == mlxVersion
}

// ResidentMemoryMB reports a process's resident set size in mebibytes, 0 when
// it cannot be read. This is the number a developer watching a local model
// actually needs: the wired limit is a cap, and how close the server is to it
// is the difference between a slow machine and one that stops drawing.
//
// `ps` rather than a syscall, for the reason [sysctlInt] shells out: the
// alternative on macOS is proc_pidinfo through cgo, for one integer.
func ResidentMemoryMB(ctx context.Context, pid int) int {
	out, err := runCommand(ctx, "ps", []string{"-o", "rss=", "-p", strconv.Itoa(pid)}, nil)
	if err != nil {
		return 0
	}
	kb, err := strconv.Atoi(strings.TrimSpace(out))
	if err != nil {
		return 0
	}
	return kb / 1024
}
