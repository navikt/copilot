package local

import (
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"
)

// The dispatch gate: what makes local_dispatch = balanced and aggressive
// more than a sentence in a prompt. docs/local-dispatch.md has the design and the review
// that shaped it.
//
// A plugin nav-pilot writes into opencode's plugin directory asks this route
// before each tool call of the cloud orchestrator, and throws the deny text it
// gets back, which opencode hands to the model as the tool's result. It lives
// on the loop guard because the guard is the nav-pilot process that started
// the session: it lives exactly as long as the session, listens on a port cplt
// already lets opencode reach, and keeps the state in memory.
//
// Everything unexpected answers "allow". The gate exists to move work to the
// worker, and a gate that breaks a session to do it is worse than none.

// Dispatch levels, the values of the local_dispatch config key.
const (
	DispatchOff          = "off"
	DispatchConservative = "conservative"
	DispatchBalanced     = "balanced"
	DispatchAggressive   = "aggressive"
)

var dispatchLevel = DispatchBalanced

// SetDispatchLevel records the local_dispatch level for this process. An
// unknown value leaves balanced: config validation has already said so.
func SetDispatchLevel(level string) {
	switch level {
	case DispatchOff, DispatchConservative, DispatchBalanced, DispatchAggressive:
		dispatchLevel = level
	default:
		dispatchLevel = DispatchBalanced
	}
}

// DispatchLevel is the local_dispatch level for this process.
func DispatchLevel() string { return dispatchLevel }

// WorkerOffered reports whether a cloud orchestrator is offered the local
// worker at all: local inference on, and local_dispatch not off.
func WorkerOffered() bool { return enabled && dispatchLevel != DispatchOff }

// GatePath is the route the plugin asks. Not under /v1, so the guard's
// request counter, which separates a declining orchestrator from broken
// wiring, never counts it.
const GatePath = "/nav-pilot/dispatch-gate"

// Gate thresholds. The sizes are the policy's own (splitMulti): 5 files or 10
// call sites. The budget bounds what a misfire costs.
const (
	gateFiles = 5
	gateCalls = 10
)

// GateDenyText is what the orchestrator reads when the gate stops an edit.
const GateDenyText = "nav-pilot (local_dispatch): this is a mechanical change across several files. " +
	"Send it to `local-worker` first: one task per file, naming each place that changes and exactly what it becomes, " +
	"with a check such as a grep. Once a file has been sent to `local-worker`, your own edits to it pass, " +
	"so you can fix or finish what it returns."

// GateCheckpointText replaces GateDenyText at a checkpoint (balanced).
const GateCheckpointText = "nav-pilot (local_dispatch): this looks like a mechanical change across several files. " +
	"Send the rest to `local-worker`: one task per file, naming each place that changes and exactly what it becomes, " +
	"with a check such as a grep. If this change needs a judgement per file, make the same edit again and it goes through."

func (g *dispatchGate) denyText() string {
	if g.rules.Checkpoint {
		return GateCheckpointText
	}
	return GateDenyText
}

// GateCreateText is what the orchestrator reads when it would write a new
// file itself.
const GateCreateText = "nav-pilot (local_dispatch = aggressive): new files go to `local-worker` first. " +
	"Send it a task naming the file, what it must contain and how to check it, such as the test command. " +
	"Once a file has been sent to `local-worker`, your own edits to it pass, so you can fix or finish what it returns."

// GateVerifyText is appended to what `local-worker` returns, so the
// orchestrator reads it at the moment it decides whether to accept the work.
// Probe 6 (mlx-workspace §8.8): a grep passed a broken definition, and a new
// test file that was green but caught nothing was accepted.
const GateVerifyText = "nav-pilot (local_dispatch): before you accept this, build the project and run the tests that cover the change, once every file you sent is done. " +
	"A grep is not a check. If `local-worker` wrote a test, show that it can fail: break the code it tests on purpose, " +
	"for example make the function return a constant, run the test, see it fail, and undo the break."

// GateNudgeText is added once per turn as a message when the orchestrator
// writes text after `local-worker` returned and no build or test command has
// run since. Worded for the case where it is about to run one.
const GateNudgeText = "nav-pilot (local_dispatch): no build or test command has run since `local-worker` returned. " +
	"Unless you are about to run one, build the project and run the tests that cover the change before you answer, " +
	"and fix or redo what fails."

// GateRetryText is appended to a build or test that failed after
// `local-worker` created a file this turn. One retry with the check's output
// is the lever bench-frontier measures as retry2: on create-file it took
// verified results from 5/20 to 15/20, at 322 s against 618 s per verified
// result (mlx-workspace #126).
const GateRetryText = "nav-pilot (local_dispatch): this check failed after `local-worker` created a file. Send it back to `local-worker` once, in the same task (pass its task_id), with the failing output above and the words: The change is not done yet. Fix it. Change nothing else. If the check fails again, fix it yourself."

// GateRetryDoneText is appended when the check fails again after that one
// retry: two attempts is the bound.
const GateRetryDoneText = "nav-pilot (local_dispatch): the check failed again after the retry. Fix it yourself, and do not send it to `local-worker` again."

// GateRequest is what the plugin sends for one tool call. No file contents:
// the path, the shell command, and a task's prompt when it goes to the worker.
// The one exception is an edit's oldString when it replaces every match,
// which the gate counts in the file.
type GateRequest struct {
	Session  string `json:"session"`
	Turn     int    `json:"turn"`
	Agent    string `json:"agent"`
	Tool     string `json:"tool"`
	Path     string `json:"path"`
	Command  string `json:"command"`
	Subagent string `json:"subagent"`
	Prompt   string `json:"prompt"`
	// Create: the call would create a file (a write, or an edit with an
	// empty oldString). An edit of a path that is not there is a mistake
	// opencode reports itself.
	Create bool `json:"create"`
	// ReplaceAll and Old: an edit that replaces every match of Old.
	ReplaceAll bool   `json:"replaceAll"`
	Old        string `json:"old"`
	// Phase is "" before a tool call, "after" once a task to the worker or a
	// bash command has returned, "text" when the orchestrator has written
	// text, and "worker" when the worker is about to create a file.
	Phase string `json:"phase"`
	// Worker is the worker's session, on a task's "after".
	Worker string `json:"worker"`
	// Exit is a bash command's exit code, on its "after"; nil when unknown.
	Exit *int `json:"exit"`
}

type gateTurn struct {
	turn       int
	files      map[string]bool
	sites      int // call sites edited: 1 per edit, the matches of a replacement
	denies     int
	refused    map[string]bool // files refused this turn, for a checkpoint retry
	dispatched bool
	sent       []string // prompts sent to the worker this turn
	unverified bool     // the worker returned, and no build or test has run since
	nudged     bool
	created    bool   // the worker created a file this turn
	checking   string // the build or test that runs since the worker returned
	retried    bool   // the worker has had its one retry this turn
	settled    bool   // the retry's check has run and been counted
}

// GateRules says which rules a session's gate runs. Each needs its class
// trusted in delegate mode: the gate never denies on an untrusted class's
// account.
type GateRules struct {
	// Multi: the edit that reaches a 5th file or a 10th call site in a turn,
	// and scripted per-file shell edits (edit-multi-mechanical).
	Multi bool
	// Create: a file the orchestrator would create itself (create-file).
	Create bool
	// Root is the session's project directory. The gate stats only paths
	// under it: the route listens on localhost, and must not answer "is this
	// file there" about the rest of the disk.
	Root string
	// Checkpoint: a refused file passes when the orchestrator makes the same
	// edit again, and one refusal per turn is the budget. balanced runs this:
	// the default cannot tell an ordinary five-file feature from a mechanical
	// change, and must not hold one hostage. Without it (aggressive), a
	// refused file passes only once it has been sent to the worker.
	Checkpoint bool
}

func (r GateRules) budget() int {
	if r.Checkpoint {
		return 1
	}
	return 2
}

// Any reports whether there is a rule to run at all.
func (r GateRules) Any() bool { return r.Multi || r.Create }

// under returns path cleaned when it lies under root, and false otherwise.
func under(root, path string) (string, bool) {
	if root == "" || !filepath.IsAbs(path) {
		return "", false
	}
	clean := filepath.Clean(path)
	prefix := filepath.Clean(root) + string(filepath.Separator)
	if !strings.HasPrefix(clean, prefix) {
		return "", false
	}
	return clean, true
}

// dispatchGate is one session's gate state, keyed by opencode session.
type dispatchGate struct {
	mu       sync.Mutex
	rules    GateRules
	sessions map[string]*gateTurn
	// workerCreated: worker sessions that created a file, until the task
	// that ran them returns. Reported before the write runs: a write that
	// then fails still counts. ponytail: files the worker creates with a
	// shell command are not seen; add a post-tool report if that matters. ponytail: a task that fails or runs in the
	// background never returns here, so its entry stays; one bool per worker
	// session.
	workerCreated map[string]bool
	// serverUp is the liveness check, asked only before a deny. A var in the
	// struct so a test can take the server down.
	serverUp func() bool
	counts   map[string]int64
}

func newDispatchGate(target string, rules GateRules) *dispatchGate {
	return &dispatchGate{
		rules:         rules,
		sessions:      map[string]*gateTurn{},
		workerCreated: map[string]bool{},
		counts:        map[string]int64{},
		serverUp:      func() bool { return tcpUp(target) },
	}
}

// tcpUp is the whole liveness check: the server's port accepts a connection.
// Not the ownership proof the completions take (ps and lsof, seconds when the
// cache is cold), which would outlast the plugin's 2 s budget and turn every
// deny into an allow. A dead server refuses at once; a live one that is not
// ours is caught where it matters, on the completion itself.
func tcpUp(target string) bool {
	u, err := url.Parse(target)
	if err != nil || u.Host == "" {
		return false
	}
	c, err := net.DialTimeout("tcp", u.Host, 200*time.Millisecond)
	if err != nil {
		return false
	}
	c.Close()
	return true
}

// decide returns the deny text, or "" to allow, and the telemetry outcome.
func (g *dispatchGate) decide(r GateRequest) (deny, outcome string) {
	if r.Session == "" || r.Agent == "" || r.Agent == WorkerAgent {
		return "", ""
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	st := g.turn(r)

	var edits []shellEdit
	scripted := false
	switch r.Tool {
	case "task":
		if r.Subagent != WorkerAgent {
			return "", ""
		}
		st.sent = append(st.sent, r.Prompt)
		first := !st.dispatched
		st.dispatched = true
		if first && st.denies > 0 {
			g.counts["dispatched_after_deny"]++
			return "", "dispatched_after_deny"
		}
		return "", ""
	case "edit", "write":
		if r.Path == "" {
			return "", ""
		}
		switch exists(g.rules.Root, r.Path) {
		case statError:
			return "", ""
		case statMissing:
			// A new file: create-file, not a mechanical edit.
			if !r.Create || !g.rules.Create || st.exempt(r.Path) || st.passes(r.Path, g.rules) || st.denies >= g.rules.budget() || !g.serverUp() {
				return "", ""
			}
			st.refused[r.Path] = true
			st.denies++
			g.counts["deny_create"]++
			return GateCreateText, "deny_create"
		}
		e := shellEdit{file: r.Path}
		if r.Tool == "edit" && r.ReplaceAll && r.Old != "" {
			if re, err := regexp.Compile(regexp.QuoteMeta(r.Old)); err == nil {
				e.re, e.global = re, true
			}
		}
		edits = []shellEdit{e}
	case "bash":
		if st.unverified && Verifies(r.Command) {
			st.unverified = false
			if st.created {
				st.checking = r.Command
			}
		}
		scripted, edits = shellEdits(r.Command)
	default:
		return "", ""
	}
	if !g.rules.Multi {
		return "", ""
	}

	if scripted {
		if st.dispatched || st.passes(r.Command, g.rules) || st.denies >= g.rules.budget() || !g.serverUp() {
			return "", ""
		}
		st.refused[r.Command] = true
		st.denies++
		g.counts["deny_scripted"]++
		return g.denyText(), "deny_scripted"
	}
	// Keyed by name: an edit names a file by its absolute path, a sed in the
	// shell by a relative one (after any cd), and one file must not count
	// twice. Two different files with one name count once, which errs
	// toward allowing.
	var counted []string
	sites := 0
	for _, e := range edits {
		if !st.exempt(e.file) {
			counted = append(counted, filepath.Base(e.file))
			sites += e.sites(g.rules.Root)
		}
	}
	if len(counted) == 0 {
		return "", ""
	}
	newFiles := 0
	for _, f := range counted {
		if !st.files[f] {
			newFiles++
		}
	}
	// The site count needs two files already: ten edits of one file is
	// someone fixing a test, not a mechanical change across files. A
	// replacement counts each place it changes, so a job of 60 call sites in
	// 3 files (probe 6, r6: two edits, then one sed with /g) reaches it too.
	byFiles := newFiles > 0 && len(st.files)+newFiles >= gateFiles
	bySites := len(st.files) >= 2 && st.sites+sites >= gateCalls
	if (byFiles || bySites) && !passesAll(st, counted, g.rules) && st.denies < g.rules.budget() && g.serverUp() {
		for _, f := range counted {
			st.refused[f] = true
		}
		st.denies++
		outcome := "deny_files"
		if !byFiles {
			outcome = "deny_sites"
		}
		g.counts[outcome]++
		return g.denyText(), outcome
	}
	st.sites += sites
	for _, f := range counted {
		st.files[f] = true
	}
	return "", ""
}

// turn returns the session's state for this turn, fresh when the turn has
// changed. The caller holds g.mu.
func (g *dispatchGate) turn(r GateRequest) *gateTurn {
	st := g.sessions[r.Session]
	if st == nil || st.turn != r.Turn {
		st = &gateTurn{turn: r.Turn, files: map[string]bool{}, refused: map[string]bool{}}
		g.sessions[r.Session] = st
	}
	return st
}

// verify answers the plugin's calls after the fact: the text to append to
// what the worker returned ("after"), and, once per turn, the reminder to
// send when the orchestrator writes text while the worker's work is
// unverified ("text"). The key is the JSON field the plugin reads.
func (g *dispatchGate) verify(r GateRequest) (key, text string) {
	if r.Phase == "worker" {
		if r.Session != "" && r.Create && exists(g.rules.Root, r.Path) == statMissing {
			g.mu.Lock()
			g.workerCreated[r.Session] = true
			g.mu.Unlock()
		}
		return "", ""
	}
	if r.Session == "" || r.Agent == "" || r.Agent == WorkerAgent {
		return "", ""
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	st := g.turn(r)
	switch {
	case r.Phase == "after" && r.Tool == "task" && r.Subagent == WorkerAgent:
		st.unverified = true
		if g.workerCreated[r.Worker] {
			st.created = true
			delete(g.workerCreated, r.Worker)
		}
		return "append", GateVerifyText
	case r.Phase == "after" && r.Tool == "bash" && st.checking != "" && r.Command == st.checking:
		// Cleared on any return of the check, so a timed-out one (no exit
		// code) does not hang over a later run of the same command.
		st.checking = ""
		if r.Exit == nil || st.settled {
			break
		}
		switch {
		case *r.Exit != 0 && !st.retried:
			st.retried = true
			g.counts["create_retry"]++
			return "append", GateRetryText
		case *r.Exit != 0:
			st.settled = true
			g.counts["create_retry_failed"]++
			return "append", GateRetryDoneText
		case st.retried:
			st.settled = true
			g.counts["create_retry_passed"]++
		}
	case r.Phase == "text" && st.unverified && !st.nudged:
		st.nudged = true
		g.counts["verify_nudge"]++
		return "nudge", GateNudgeText
	}
	return "", ""
}

type statResult int

const (
	statFound statResult = iota
	statMissing
	statError
)

// exists: the path is there. The plugin sends absolute paths, and the guard
// runs on the same machine as the session. Anything but "not there" (a
// relative path, one outside the project, a permission error) is an error,
// which allows the call.
func exists(root, path string) statResult {
	clean, ok := under(root, path)
	if !ok {
		return statError
	}
	_, err := os.Stat(clean)
	switch {
	case err == nil:
		return statFound
	case errors.Is(err, fs.ErrNotExist):
		return statMissing
	}
	return statError
}

// passes: at a checkpoint, the same call again after a refusal goes through.
func (t *gateTurn) passes(key string, r GateRules) bool {
	return r.Checkpoint && t.refused[key]
}

func passesAll(t *gateTurn, keys []string, r GateRules) bool {
	if !r.Checkpoint || len(keys) == 0 {
		return false
	}
	for _, k := range keys {
		if !t.refused[k] {
			return false
		}
	}
	return true
}

// exempt: a file whose name was in a prompt sent to the worker this turn. The
// orchestrator is finishing or fixing what the worker returned, or taking the
// task back after the worker failed, which is what the policy tells it to do.
//
// ponytail: matched by file name as a whole word, not by path. A prompt
// naming src/Foo.kt also exempts test/Foo.kt; the gate errs toward letting
// work through, and the orchestrator names files as it pleases (relative,
// absolute, bare).
func (t *gateTurn) exempt(path string) bool {
	name := regexp.MustCompile(`(^|[^\w.-])` + regexp.QuoteMeta(filepath.Base(path)) + `($|[^\w.-])`)
	for _, p := range t.sent {
		if name.MatchString(p) {
			return true
		}
	}
	return false
}

// serve is the route. Always 200 with {"deny": "..."} or {}.
func (g *dispatchGate) serve(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var req GateRequest
	if r.Method != http.MethodPost || json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req) != nil {
		_, _ = w.Write([]byte("{}"))
		return
	}
	key, text := "deny", ""
	if req.Phase == "" {
		text, _ = g.decide(req)
	} else {
		key, text = g.verify(req)
	}
	if text == "" {
		_, _ = w.Write([]byte("{}"))
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]string{key: text})
}

// snapshot returns the gate's outcomes so far, for the exit-time telemetry.
func (g *dispatchGate) snapshot() map[string]int64 {
	g.mu.Lock()
	defer g.mu.Unlock()
	out := make(map[string]int64, len(g.counts))
	for k, v := range g.counts {
		out[k] = v
	}
	return out
}

var (
	sedInline  = regexp.MustCompile(`^-[a-zA-Z]*i`)
	perlInline = regexp.MustCompile(`^-[a-zA-Z0-9]*i`)
)

// ShellEdits reads a bash command for in-place edits. scripted is an in-place
// sed or perl whose script or files are computed per file: inside a for,
// while or until loop, or with a $ or a backtick outside single quotes. That
// is the form Sonnet 5 used to do a 12-call-site job in one call (probe 4:
// grep -rl … | while read f; do sed -i … "$f"; done). files are the single
// literal files of the other in-place segments. A segment naming several
// files, or taking them from xargs or find -exec, is one search-and-replace,
// which the policy keeps with the orchestrator, so it contributes nothing.
//
// ponytail: a lexer, not a shell parser. Heredocs, python -c and cat > file
// are not seen; telemetry shows them as few denies with few dispatches.
func ShellEdits(cmd string) (scripted bool, files []string) {
	scripted, edits := shellEdits(cmd)
	for _, e := range edits {
		files = append(files, e.file)
	}
	return scripted, files
}

// shellEdit is one in-place edit of one file. re is its substitution's
// pattern when the gate could read it, and global its /g flag.
type shellEdit struct {
	file   string
	dir    string // the directory a relative file is in, from a cd before it; relative to the root unless absolute
	re     *regexp.Regexp
	global bool
}

// sites is the number of places the edit changes: the matches of its
// pattern in the file, per line as sed and perl -p work, and 1 when the gate
// cannot tell. Only a file under root is read.
//
// ponytail: the file is read before the edit runs, once per edit call. A
// pattern RE2 cannot compile (a backreference, a lookaround) counts 1.
func (e shellEdit) sites(root string) int {
	if e.re == nil {
		return 1
	}
	path := e.file
	if !filepath.IsAbs(path) {
		path = filepath.Join(root, e.dir, path)
		if filepath.IsAbs(e.dir) {
			path = filepath.Join(e.dir, e.file)
		}
	}
	// Resolved first, so a link cannot lead the read out of the project.
	if real, err := filepath.EvalSymlinks(path); err == nil {
		path = real
	}
	clean, ok := under(root, path)
	if !ok {
		return 1
	}
	// A regular file only: a FIFO would block the read, and the gate's lock.
	if fi, err := os.Stat(clean); err != nil || !fi.Mode().IsRegular() {
		return 1
	}
	f, err := os.Open(clean)
	if err != nil {
		return 1
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, 4<<20))
	if err != nil {
		return 1
	}
	n := 0
	for _, line := range strings.Split(string(b), "\n") {
		if e.global {
			n += len(e.re.FindAllStringIndex(line, -1))
		} else if e.re.MatchString(line) {
			n++
		}
	}
	return max(1, n)
}

func shellEdits(cmd string) (scripted bool, edits []shellEdit) {
	loop := 0
	dir := ""
	for _, seg := range splitSegments(cmd) {
		toks := shellWords(seg.text)
		for len(toks) > 0 && (toks[0] == "do" || toks[0] == "then" || toks[0] == "{" || toks[0] == "(") {
			toks = toks[1:]
		}
		if len(toks) == 0 {
			continue
		}
		switch toks[0] {
		case "for", "while", "until":
			loop++
		case "done":
			loop = max(0, loop-1)
		case "cd":
			// Where a relative file is, for counting its sites: relative to
			// the project root unless a cd made it absolute.
			switch {
			case len(toks) != 2 || seg.dynamic:
				dir = ""
			case filepath.IsAbs(toks[1]):
				dir = toks[1]
			default:
				dir = filepath.Join(dir, toks[1])
			}
		}
		at := -1
		for j, t := range toks {
			if b := filepath.Base(t); b == "sed" || b == "gsed" || b == "perl" {
				at = j
				break
			}
		}
		if at < 0 {
			continue
		}
		var fs, scripts []string
		var inPlace, ere bool
		if filepath.Base(toks[at]) == "perl" {
			fs, scripts, inPlace = perlFiles(toks[at+1:])
			ere = true
		} else {
			fs, scripts, inPlace, ere = sedFiles(toks[at+1:])
		}
		if !inPlace {
			continue
		}
		// A computed value on one literal file is one edit of that file (a
		// version bump from $NEW). A loop, or a file that is itself computed,
		// is one edit per file.
		if loop > 0 || seg.dynamic && (len(fs) != 1 || strings.ContainsAny(fs[0], "$`")) {
			return true, nil
		}
		indirect := false
		for _, t := range toks[:at] {
			if t == "xargs" || t == "-exec" || t == "-execdir" {
				indirect = true
			}
		}
		if !indirect && len(fs) == 1 {
			e := shellEdit{file: fs[0], dir: dir}
			// One substitution only, and none with a computed value, whose
			// pattern may hold the value.
			if len(scripts) == 1 && !seg.dynamic {
				e.re, e.global = substitution(scripts[0], ere)
			}
			edits = append(edits, e)
		}
	}
	return false, edits
}

// substitution reads an s command with no address (s/pat/repl/flags, any
// delimiter) and returns its pattern as RE2, and whether it has the g flag.
// nil when it is anything else or does not compile.
func substitution(script string, ere bool) (*regexp.Regexp, bool) {
	script = strings.TrimSpace(script)
	if len(script) < 4 || script[0] != 's' {
		return nil, false
	}
	delim := script[1]
	if delim == '\\' || delim == '\n' || delim == ' ' {
		return nil, false
	}
	var parts []string
	var cur strings.Builder
	for i := 2; i < len(script); i++ {
		c := script[i]
		switch {
		case c == '\\' && i+1 < len(script) && script[i+1] == delim:
			cur.WriteString(regexp.QuoteMeta(string(delim)))
			i++
		case c == '\\' && i+1 < len(script):
			cur.WriteByte(c)
			cur.WriteByte(script[i+1])
			i++
		case c == delim:
			parts = append(parts, cur.String())
			cur.Reset()
		default:
			cur.WriteByte(c)
		}
	}
	if len(parts) != 2 || strings.ContainsAny(cur.String(), ";}\n") {
		return nil, false
	}
	pat := parts[0]
	if !ere {
		pat = breToRE2(pat)
	}
	pat = strings.NewReplacer(`\<`, `\b`, `\>`, `\b`).Replace(pat)
	re, err := regexp.Compile(pat)
	if err != nil {
		return nil, false
	}
	return re, strings.Contains(cur.String(), "g")
}

// breToRE2 turns a POSIX basic regex into RE2: in a BRE, ( ) { } + ? | are
// literal, and escaped they are operators (+ ? | as GNU extensions).
func breToRE2(p string) string {
	var b strings.Builder
	for i := 0; i < len(p); i++ {
		c := p[i]
		switch {
		case c == '\\' && i+1 < len(p) && strings.IndexByte("(){}+?|", p[i+1]) >= 0:
			b.WriteByte(p[i+1])
			i++
		case c == '\\' && i+1 < len(p):
			b.WriteByte(c)
			b.WriteByte(p[i+1])
			i++
		case strings.IndexByte("(){}+?|", c) >= 0:
			b.WriteByte('\\')
			b.WriteByte(c)
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}

// sedFiles returns the files and scripts of an in-place sed, whether it is
// in place, and whether its regexes are extended (-E, -r).
func sedFiles(args []string) (files, scripts []string, inPlace, ere bool) {
	script := false
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--in-place" || strings.HasPrefix(a, "--in-place="):
			inPlace = true
		case a == "--regexp-extended":
			ere = true
		case a == "-i":
			inPlace = true
			// BSD sed: -i takes the backup suffix as the next word, '' for none.
			if i+1 < len(args) && args[i+1] == "" {
				i++
			}
		case a == "-e" || a == "--expression":
			script = true
			if i+1 < len(args) {
				scripts = append(scripts, args[i+1])
			}
			i++
		case a == "-f" || a == "--file":
			script = true
			scripts = append(scripts, "") // unreadable here
			i++
		case strings.HasPrefix(a, "-") && len(a) > 1:
			if sedInline.MatchString(a) {
				inPlace = true
			}
			// What follows an i is the backup suffix (-i.orig), not flags.
			flags := a[1:]
			if k := strings.IndexByte(flags, 'i'); k >= 0 {
				flags = flags[:k]
			}
			if !strings.HasPrefix(a, "--") && strings.ContainsAny(flags, "Er") {
				ere = true
			}
		case !script:
			script = true
			scripts = append(scripts, a)
		default:
			files = append(files, a)
		}
	}
	return files, scripts, inPlace, ere
}

// perlFiles returns the files and scripts of an in-place perl (-i, -pi,
// -pie, -i.bak), and whether it is in place.
func perlFiles(args []string) (files, scripts []string, inPlace bool) {
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case strings.HasPrefix(a, "-M") || strings.HasPrefix(a, "-I") || strings.HasPrefix(a, "-m"):
			// A module or an include dir, whose name may hold an i.
		case strings.HasPrefix(a, "-") && len(a) > 1:
			flags := strings.TrimPrefix(a, "-")
			if perlInline.MatchString(a) {
				inPlace = true
			}
			if strings.HasSuffix(flags, "e") || strings.HasSuffix(flags, "E") {
				if i+1 < len(args) {
					scripts = append(scripts, args[i+1])
				}
				i++ // the script
			}
		default:
			files = append(files, a)
		}
	}
	return files, scripts, inPlace
}

// Verifies reports whether a bash command builds the project or runs its
// tests: a segment that runs a build tool or test runner.
//
// ponytail: a list of the common tools, not every build system. One missing
// here costs one reminder too many (verify_nudge), never a refusal.
func Verifies(cmd string) bool {
	for _, seg := range splitSegments(cmd) {
		toks := shellWords(seg.text)
		for len(toks) > 0 && (strings.Contains(toks[0], "=") || slices.Contains([]string{"do", "then", "{", "(", "time", "env", "command", "nice"}, toks[0])) {
			toks = toks[1:]
		}
		if len(toks) > 1 && toks[0] == "timeout" {
			toks = toks[2:]
		}
		if len(toks) == 0 {
			continue
		}
		arg := func(i int) string {
			if i < len(toks) {
				return toks[i]
			}
			return ""
		}
		switch filepath.Base(toks[0]) {
		case "gradle", "gradlew", "mvn", "mvnw", "tsc", "pytest", "jest", "vitest", "make", "sbt", "bazel", "bazelisk":
			return true
		case "go", "cargo", "dotnet", "swift", "deno", "mix":
			if slices.Contains([]string{"test", "build", "vet", "check"}, arg(1)) {
				return true
			}
		case "npm", "pnpm", "yarn", "bun":
			s := arg(1)
			if s == "run" || s == "run-script" {
				s = arg(2)
			}
			if strings.Contains(s, "test") || strings.Contains(s, "build") || strings.Contains(s, "check") || s == "tsc" {
				return true
			}
		case "npx", "pnpx", "bunx":
			if slices.Contains([]string{"tsc", "jest", "vitest", "playwright"}, arg(1)) {
				return true
			}
		case "python", "python3", "uv", "poetry":
			if slices.Contains(toks, "pytest") {
				return true
			}
		}
	}
	return false
}

// segment is one simple command. dynamic: it holds a $ or a backtick outside
// single quotes, so what it runs is computed when it runs.
type segment struct {
	text    string
	dynamic bool
}

// splitSegments splits a command on ; & | and newlines, outside quotes.
func splitSegments(cmd string) []segment {
	var out []segment
	var cur strings.Builder
	dynamic := false
	var q byte
	flush := func() {
		if s := strings.TrimSpace(cur.String()); s != "" {
			out = append(out, segment{text: s, dynamic: dynamic})
		}
		cur.Reset()
		dynamic = false
	}
	for i := 0; i < len(cmd); i++ {
		c := cmd[i]
		switch {
		case q == '\'':
			if c == q {
				q = 0
			}
		case q == '"':
			if c == q {
				q = 0
			} else if c == '$' || c == '`' {
				dynamic = true
			} else if c == '\\' && i+1 < len(cmd) {
				cur.WriteByte(c)
				i++
				c = cmd[i]
			}
		case c == '\'' || c == '"':
			q = c
		case c == '$' || c == '`':
			dynamic = true
		case c == ';' || c == '\n' || c == '|' || c == '&':
			flush()
			continue
		}
		cur.WriteByte(c)
	}
	flush()
	return out
}

// shellWords splits one segment into words, removing quotes. An empty quoted
// word, such as the empty backup suffix after BSD sed's -i, is kept as an
// empty string.
func shellWords(s string) []string {
	var out []string
	var cur strings.Builder
	inWord := false
	var q byte
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case q == '"' && c == '\\' && i+1 < len(s):
			i++
			cur.WriteByte(s[i])
		case q != 0:
			if c == q {
				q = 0
			} else {
				cur.WriteByte(c)
			}
		case c == '\'' || c == '"':
			q, inWord = c, true
		case c == '\\' && i+1 < len(s):
			i++
			cur.WriteByte(s[i])
			inWord = true
		case c == ' ' || c == '\t':
			if inWord {
				out = append(out, cur.String())
				cur.Reset()
				inWord = false
			}
		default:
			cur.WriteByte(c)
			inWord = true
		}
	}
	if inWord {
		out = append(out, cur.String())
	}
	return out
}

// DispatchGateRules is which gate rules a level runs for a model: balanced
// enforces the multi-file split, aggressive also new files. Only classes the
// manifest trusts in delegate mode, so the policy text and the gate are cut
// from the same decision.
func DispatchGateRules(level string, c *Capabilities) GateRules {
	send, _ := c.DelegateTrusted()
	multi := slices.Contains(send, "edit-multi-mechanical")
	switch level {
	case DispatchBalanced:
		return GateRules{Multi: multi, Checkpoint: true}
	case DispatchAggressive:
		return GateRules{Multi: multi, Create: slices.Contains(send, "create-file")}
	}
	return GateRules{}
}
