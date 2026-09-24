package hook

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/navikt/copilot/cli/nav-pilot/internal/local"
)

// The loop guard for every session: the local guard's result-aware rule
// (internal/local/guard.go, repeatedToolCall), run as a postToolUse hook.
//
// The rule is the same. The same call with the same result [SameResult] times
// in a row is a loop; the same call threshold times in a row is one whatever
// it returns. Results are compared after local.NormaliseResult, as the guard
// compares them.
//
// What differs is the reach. The guard sees the whole conversation on every
// request, so it needs no memory; a hook sees one call at a time, so it keeps
// the run in a small file per session. And the guard can refuse the request,
// which ends the turn; a postToolUse hook cannot end anything. What it can do
// is replace the result the model reads, so on a hit the model gets a message
// that it is stuck, with the result underneath, instead of the same answer
// again. A preToolUse deny would not stop the turn either — it too only hands
// the model a message — and would need the same state read on every call
// before it runs, so it buys nothing over this.

// LoopState is the run a session is on, as kept between hook calls.
type LoopState struct {
	Call   string `json:"call"`
	N      int    `json:"n"`
	Same   int    `json:"same"`
	Result string `json:"result"` // sha256 of the normalised result
}

// Step folds one call and its result into the run and says which rule, if
// any, it trips: n is the run of identical calls, same the part of it whose
// results were identical too.
func (s LoopState) Step(call, resultType, result string) LoopState {
	sum := sha256.Sum256([]byte(resultType + "\x00" + local.NormaliseResult(result)))
	h := hex.EncodeToString(sum[:])
	switch {
	case call != s.Call:
		return LoopState{Call: call, N: 1, Same: 1, Result: h}
	case h == s.Result:
		return LoopState{Call: call, N: s.N + 1, Same: s.Same + 1, Result: h}
	default:
		return LoopState{Call: call, N: s.N + 1, Same: 1, Result: h}
	}
}

// Signature is what makes two calls the same call: the tool name and its
// arguments, with keys in a fixed order. A shell tool's "description" is
// left out: the model writes a new one for the same command at will, and
// counting it would let an identical `gh run view` look like a new call every
// time. Other tools keep it, since there it can be a real argument.
func Signature(tool string, args json.RawMessage) string {
	var v any
	if err := json.Unmarshal(args, &v); err != nil {
		return tool + "(" + string(args) + ")"
	}
	if m, ok := v.(map[string]any); ok && shellTools[strings.ToLower(tool)] {
		delete(m, "description")
	}
	b, err := json.Marshal(v)
	if err != nil {
		return tool + "(" + string(args) + ")"
	}
	return tool + "(" + string(b) + ")"
}

// LoopMessage is what the model reads instead of the plain result once a rule
// trips, or "" when none does. It says which rule, and only the same-result
// rule calls it a loop that will not change the answer: the backstop also
// catches polls whose output was changing.
func LoopMessage(s LoopState, threshold int) string {
	const maxCall = 400
	shown := s.Call
	if len(shown) > maxCall {
		shown = shown[:maxCall] + "…"
	}
	switch {
	case s.Same >= SameResult(threshold):
		return fmt.Sprintf(
			"[nav-pilot loop guard] You have made this exact tool call %d times in a row and got the same result every time: %s. "+
				"Repeating it will not change the answer. Stop calling it: use the result you already have, try a different approach, "+
				"or tell the user you are stuck. (Threshold: `nav-pilot config set local_loop_guard <n>`, current %d.)",
			s.Same, shown, threshold)
	case s.N >= threshold:
		return fmt.Sprintf(
			"[nav-pilot loop guard] You have made this exact tool call %d times in a row: %s. The results changed, but that is "+
				"waiting, not working. If you are waiting on something slow, use a command that blocks until it is done; otherwise "+
				"try something else or tell the user. (Threshold: `nav-pilot config set local_loop_guard <n>`, current %d.)",
			s.N, shown, threshold)
	}
	return ""
}

// SameResult is the same-result threshold for a given backstop, derived as
// local.SameResultRepeat derives it: half, and never below 2.
func SameResult(threshold int) int { return max(2, threshold/2) }

// shellTools are the shell tool names, as the hook artifacts' matcher
// (bash|shell|execute) lists them, plus PowerShell on Windows.
var shellTools = map[string]bool{"bash": true, "shell": true, "execute": true, "powershell": true}

var unsafeID = regexp.MustCompile(`[^A-Za-z0-9_-]`)

// LoopGuard runs the rule for one postToolUse payload and returns the hook's
// stdout. The run is kept in the session's own directory under root (Copilot's
// session-state directory), which Copilot creates for every session and which
// cplt's sandbox lets the session write. The state is a hash and two counts,
// never the call or the result.
//
// Every failure answers NoChange: a guard that cannot keep its state lets the
// call through rather than get in the way. A state that could not be saved is
// also returned as err, since a guard that cannot count never trips and would
// otherwise look like one that has nothing to report.
//
// ponytail: read-modify-write without a lock. Parallel tool calls in one step
// can race and lose a count, which errs toward not stopping; the guard treats
// parallel calls as one step and this hook sees them one by one, so a loop of
// parallel calls is left to the per-call view. Add flock if it matters.
func LoopGuard(root string, p Payload, threshold int) (string, error) {
	id := unsafeID.ReplaceAllString(p.SessionID, "")
	if id == "" || !p.HasResult || p.ToolName == "" {
		return NoChange, nil
	}
	dir := filepath.Join(root, id)
	path := filepath.Join(dir, "nav-pilot-loop-guard.json")

	var st LoopState
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, &st)
	}
	st = st.Step(Signature(p.ToolName, p.ToolArgs), p.ResultType, p.Result)
	err := saveState(dir, path, st)

	msg := LoopMessage(st, threshold)
	if msg == "" {
		return NoChange, err
	}
	return ModifiedResult(p.ResultType, msg+"\n\nThe tool result, unchanged:\n"+p.Result), err
}

func saveState(dir, path string, st LoopState) error {
	data, err := json.Marshal(st)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
