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

// GateRequest is what the plugin sends for one tool call. No file contents:
// the path, the shell command, and a task's prompt when it goes to the worker.
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
}

type gateTurn struct {
	turn       int
	files      map[string]bool
	calls      int
	denies     int
	refused    map[string]bool // files refused this turn, for a checkpoint retry
	dispatched bool
	sent       []string // prompts sent to the worker this turn
}

// GateRules says which rules a session's gate runs. Each needs its class
// trusted in delegate mode: the gate never denies on an untrusted class's
// account.
type GateRules struct {
	// Multi: the edit that reaches a 5th file or a 10th edit in a turn, and
	// scripted per-file shell edits (edit-multi-mechanical).
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
	// serverUp is the liveness check, asked only before a deny. A var in the
	// struct so a test can take the server down.
	serverUp func() bool
	counts   map[string]int64
}

func newDispatchGate(target string, rules GateRules) *dispatchGate {
	return &dispatchGate{
		rules:    rules,
		sessions: map[string]*gateTurn{},
		counts:   map[string]int64{},
		serverUp: func() bool { return tcpUp(target) },
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
	st := g.sessions[r.Session]
	if st == nil || st.turn != r.Turn {
		st = &gateTurn{turn: r.Turn, files: map[string]bool{}, refused: map[string]bool{}}
		g.sessions[r.Session] = st
	}

	var files []string
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
		files = []string{r.Path}
	case "bash":
		scripted, files = ShellEdits(r.Command)
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
	for _, f := range files {
		if !st.exempt(f) {
			counted = append(counted, filepath.Base(f))
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
	// The call count needs several files: ten edits of one file is someone
	// fixing a test, not a mechanical change across files.
	over := (newFiles > 0 && len(st.files)+newFiles >= gateFiles) || (len(st.files) >= 2 && st.calls+1 >= gateCalls)
	if over && !passesAll(st, counted, g.rules) && st.denies < g.rules.budget() && g.serverUp() {
		for _, f := range counted {
			st.refused[f] = true
		}
		st.denies++
		g.counts["deny_files"]++
		return g.denyText(), "deny_files"
	}
	st.calls++
	for _, f := range counted {
		st.files[f] = true
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
	deny, _ := g.decide(req)
	if deny == "" {
		_, _ = w.Write([]byte("{}"))
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]string{"deny": deny})
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
	loop := 0
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
		var fs []string
		var inPlace bool
		if filepath.Base(toks[at]) == "perl" {
			fs, inPlace = perlFiles(toks[at+1:])
		} else {
			fs, inPlace = sedFiles(toks[at+1:])
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
			files = append(files, fs[0])
		}
	}
	return false, files
}

// sedFiles returns the files of an in-place sed, and false when it is not
// in place.
func sedFiles(args []string) ([]string, bool) {
	inPlace, script := false, false
	var files []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--in-place" || strings.HasPrefix(a, "--in-place="):
			inPlace = true
		case a == "-i":
			inPlace = true
			// BSD sed: -i takes the backup suffix as the next word, '' for none.
			if i+1 < len(args) && args[i+1] == "" {
				i++
			}
		case a == "-e" || a == "-f" || a == "--expression" || a == "--file":
			script = true
			i++
		case strings.HasPrefix(a, "-") && len(a) > 1:
			if sedInline.MatchString(a) {
				inPlace = true
			}
		case !script:
			script = true
		default:
			files = append(files, a)
		}
	}
	return files, inPlace
}

// perlFiles returns the files of an in-place perl (-i, -pi, -pie, -i.bak).
func perlFiles(args []string) ([]string, bool) {
	inPlace := false
	var files []string
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
				i++ // the script
			}
		default:
			files = append(files, a)
		}
	}
	return files, inPlace
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
