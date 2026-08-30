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
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/navikt/copilot/cli/nav-pilot/internal/domain"
)

// State is what start leaves behind for stop and status to find.
//
// The port is recorded because it is no longer fixed. It used to be 8080, which
// is what a developer's own Ktor or Spring service binds, so the first alpha user
// and their own application wanted the same number: one of them would refuse to
// start, and the refusal nav-pilot printed told the developer to kill the process
// holding it, which would have been their own. The server now asks the kernel for
// a free port and writes it here, and every reader that already opens this file
// to find the pid learns the port from the same read.
type State struct {
	PID     int       `json:"pid"`
	Model   string    `json:"model"`
	Started time.Time `json:"started"`

	// Port the server is listening on. Zero in a file written by an older
	// nav-pilot, which meant 8080 by convention; [State.ServerPort] applies
	// that so an upgrade does not orphan a running server.
	Port int `json:"port,omitempty"`

	// Lstart is the kernel's start time for PID, verbatim from `ps -o lstart=`.
	// It is the pid's identity, and the reason a pid alone is not one: this
	// file outlives a reboot, after which the kernel has handed that number to
	// whatever asked for it next. Without this, `stop` signals a stranger's
	// process group and `status` reports it as the local server.
	Lstart string `json:"lstart"`
}

// ServerPort is the port this server listens on, honouring a file written before
// the port was recorded.
func (st State) ServerPort() int {
	if st.Port == 0 {
		return DefaultPort
	}
	return st.Port
}

func statePath() string { return filepath.Join(dataDir(), "server.json") }

// SaveState records a running server. The process start time is read here
// rather than passed in, so no caller can record a pid without the identity
// that makes it safe to signal later.
func SaveState(s State) error {
	if s.Lstart == "" {
		s.Lstart = processStart(s.PID)
	}
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
		return s, false, fmt.Errorf(
			"%s is not readable as a recorded server: %w.\n\n"+
				"  That file is nav-pilot's record, not the server: deleting it gets start, stop and status working again and does not stop anything that is still running.\n\n    %s\n\n"+
				"  If a server is still up, it is on 127.0.0.1:%d:\n\n    %s",
			statePath(), err, domain.Bold("rm "+statePath()), DefaultPort,
			domain.Bold(fmt.Sprintf("lsof -ti tcp:%d | xargs kill", DefaultPort)))
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

// processStart reads the kernel's start time for a pid, "" when it cannot be
// read. `ps` for the same reason [ResidentMemoryMB] shells out: the
// alternative on macOS is proc_pidinfo through cgo, for one string. A func var
// so tests can retire a pid without a process.
var processStart = func(pid int) string {
	if pid <= 0 {
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), probeTimeout)
	defer cancel()
	out, err := runCommand(ctx, "ps", []string{"-o", "lstart=", "-p", strconv.Itoa(pid)}, nil)
	if err != nil {
		return ""
	}
	// Whitespace-normalised: the field is padded to a fixed width and the
	// padding is not part of the identity.
	return strings.Join(strings.Fields(out), " ")
}

// isRecorded reports whether pid is still the process whose start time was
// recorded. An empty start time on either side is a mismatch, not a pass: this
// proves identity, and "cannot tell" is not proof — a record written before
// nav-pilot recorded start times is exactly the record that may predate a
// reboot.
func isRecorded(pid int, lstart string) bool {
	return lstart != "" && alive(pid) && processStart(pid) == lstart
}

// EnsureOwnServer proves the server the guard forwards to is the one nav-pilot
// started, and refuses when it cannot.
//
// [Server.Start] refuses a port it does not own; this is that same rule at the
// other end of the day. The loop guard proxies to the address recorded when the
// server started, so a server that crashed hours ago and left that port to
// whatever bound it next would have every prompt of the session forwarded to a
// stranger, with nothing on screen to say so. The launch calls it once; the guard calls it again per
// completion behind a short cache, because a session outlives its launch by
// hours and the crash it is looking for happens in the middle of one.
//
// Three things have to hold: something is recorded, the recorded pid is still
// that process, and it is the process holding the port. It refuses rather than
// adopts — a stranger answering with the right model id is still a stranger,
// for the reason [servedModel] states.
// ErrNoServerRecorded is EnsureOwnServer's "nothing is running here". It is a
// distinct error because it is the only one a caller may answer by starting a
// server: every other failure means a server may well be running and this process
// merely cannot prove it, and starting a second one then puts 42 GB of weights on
// a 48 GB machine.
var ErrNoServerRecorded = errors.New("no local server is recorded as running")

func EnsureOwnServer() error {
	st, ok, err := LoadState()
	if err != nil {
		return err
	}
	start := domain.Bold("nav-pilot alpha local start")
	if !ok {
		return fmt.Errorf("%w.\n\n  Start one first:\n\n    %s", ErrNoServerRecorded, start)
	}
	if !isRecorded(st.PID, st.Lstart) {
		return fmt.Errorf(
			"the recorded local %s server (pid %d) is not running any more.\n\n"+
				"  Refusing: the loop guard forwards to %s, and nav-pilot cannot tell whether that is still its own server or whatever took the port after it died.\n\n"+
				"  Start it again:\n\n    %s",
			st.Model, st.PID, ServerURL(), start)
	}
	if !slices.Contains(portListeners(st.ServerPort()), st.PID) {
		return fmt.Errorf(
			"the recorded local %s server (pid %d) is not what is listening on 127.0.0.1:%d%s.\n\n"+
				"  Refusing: the loop guard forwards there, so every prompt in this session would go to a server nav-pilot did not start and cannot vouch for.\n\n"+
				"  Stop it, then start again:\n\n    %s\n    %s",
			st.Model, st.PID, st.ServerPort(),
			describeForeignServer(context.Background(), ServerURL()),
			domain.Bold("nav-pilot alpha local stop"), start)
	}
	return nil
}

// portListeners reports the pids listening on a TCP port. `lsof` because it is
// what answers "who holds this port" on macOS without a privileged interface,
// and it is already the command nav-pilot's refusals tell a developer to run.
// No answer means no proof of ownership, which [EnsureOwnServer] treats as a
// refusal.
// lsof lives in /usr/sbin, which a login shell has on its PATH and a detached or
// launchd-started process often does not. Looked up by name it is simply not
// found there, portListeners returns nothing, and EnsureOwnServer reads "no proof
// of ownership" and refuses a server that is running perfectly well — telling the
// developer to kill the process holding the port, which is their own.
const lsofPath = "/usr/sbin/lsof"

var portListeners = func(port int) []int {
	ctx, cancel := context.WithTimeout(context.Background(), probeTimeout)
	defer cancel()
	out, err := runCommand(ctx, lsofPath, []string{"-ti", "tcp:" + strconv.Itoa(port), "-sTCP:LISTEN"}, nil)
	if err != nil {
		return nil
	}
	var pids []int
	for _, f := range strings.Fields(out) {
		if pid, err := strconv.Atoi(f); err == nil {
			pids = append(pids, pid)
		}
	}
	return pids
}

// Attach adopts a recorded server so [Server.Health], [Server.Status] and
// [Server.Stop] work on it. A process that is already gone is adopted as
// crashed, decided before returning: a caller must not be told "starting"
// about a pid that no longer exists.
func Attach(s State) *Server {
	// The recorded port, not the old constant. Attaching on 8080 meant `status`
	// health-probed whatever the developer had there, posted a chat completion at
	// their own Ktor service, and reported a healthy local server as starting
	// forever because a connection refusal classifies as still loading.
	srv := &Server{Port: s.ServerPort(), model: s.Model, started: s.Started}
	p := &attachedProc{pid: s.PID, lstart: s.Lstart, done: make(chan struct{})}
	srv.proc = p
	if !p.live() {
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
	pid    int
	lstart string
	watch  sync.Once
	once   sync.Once
	info   exitInfo
	done   chan struct{}
}

func (p *attachedProc) PID() int { return p.pid }

// live reports whether the pid is still the process that was recorded, not
// merely whether something answers to that number.
func (p *attachedProc) live() bool { return isRecorded(p.pid, p.lstart) }

func (p *attachedProc) Signal(sig os.Signal) error {
	s, ok := sig.(syscall.Signal)
	if !ok {
		return fmt.Errorf("cannot send %v to an adopted process", sig)
	}
	// Identity before the signal, and before the negative pid in particular:
	// server.json outlives a reboot, and the process group of a pid the kernel
	// has since handed to something else belongs to a stranger.
	if !p.live() {
		return fmt.Errorf(
			"refusing to send %v to pid %d: it is not the process nav-pilot recorded, so its process group is not nav-pilot's to signal",
			sig, p.pid)
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
			for p.live() {
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
