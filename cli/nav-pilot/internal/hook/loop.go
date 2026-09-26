package hook

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/navikt/copilot/cli/nav-pilot/internal/local"
)

// The loop guard for every session: the local guard's result-aware rule
// (internal/local/guard.go, repeatedToolCall), run as a postToolUse hook.
//
// The rule is the same. The same call with the same result [SameResult] times
// in a row is a loop; the same call threshold times in a row is one whatever
// it returns; a short cycle of calls that keeps getting the same answers
// ([local.RepeatedCycle]) is a loop after as many repeats as the same-result
// rule. Results are compared after local.NormaliseResult, as the guard
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
	Call   string `json:"call"` // sha256 of the call's Signature
	N      int    `json:"n"`
	Same   int    `json:"same"`
	Result string `json:"result"` // sha256 of the normalised result
	// Raw is the sha256 of the last result as it came, and Varied says the
	// run's results differed before normalisation (numbers, ids, timestamps).
	// A state from before Raw existed has none, and proves nothing.
	Raw    string `json:"raw,omitempty"`
	Varied bool   `json:"varied,omitempty"`
	// Steps are the latest calls with their results, oldest first, as short
	// hashes: what the cycle rule compares.
	Steps []string `json:"steps,omitempty"`
}

// Step folds one call and its result into the run and says which rule, if
// any, it trips: n is the run of identical calls, same the part of it whose
// results were identical too.
func (s LoopState) Step(call, resultType, result string) LoopState {
	sum := sha256.Sum256([]byte(resultType + "\x00" + local.NormaliseResult(result)))
	h := hex.EncodeToString(sum[:])
	rawSum := sha256.Sum256([]byte(resultType + "\x00" + result))
	raw := hex.EncodeToString(rawSum[:])
	step := sha256.Sum256([]byte(call + "\x00" + h))
	steps := append(slices.Clip(s.Steps), hex.EncodeToString(step[:8]))
	switch {
	case call != s.Call:
		return LoopState{Call: call, N: 1, Same: 1, Result: h, Raw: raw, Steps: steps}
	case h == s.Result:
		return LoopState{Call: call, N: s.N + 1, Same: s.Same + 1, Result: h, Raw: raw, Varied: s.Varied || (s.Raw != "" && raw != s.Raw), Steps: steps}
	default:
		return LoopState{Call: call, N: s.N + 1, Same: 1, Result: h, Raw: raw, Steps: steps}
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
//
// It names no threshold and no config key. The model reads it, and a model
// told how to raise the limit that stopped it has been told how to stop being
// stopped; the human gets that on stderr (see the hook command).
func LoopMessage(s LoopState, threshold int) string {
	const maxCall = 400
	shown := s.Call
	if len(shown) > maxCall {
		shown = shown[:maxCall] + "…"
	}
	period, reps := local.RepeatedCycle(s.Steps)
	switch LoopRule(s, threshold) {
	case "same_result":
		same := "the same result every time"
		if s.Varied {
			same = "the same result every time apart from numbers, ids and timestamps"
		}
		wait := ""
		if isShellCall(s.Call) {
			wait = " If you are waiting for something to finish, use a command that blocks until it is done " +
				"(for GitHub Actions: `gh run watch <run-id> --exit-status`) instead of asking again."
		}
		return fmt.Sprintf(
			"[nav-pilot loop guard] You have made this exact tool call %d times in a row and got %s: %s. "+
				"Repeating it will not change the answer. Stop calling it: use the result you already have, try a different approach, "+
				"or tell the user you are stuck.%s",
			s.Same, same, shown, wait)
	case "cycle":
		return fmt.Sprintf(
			"[nav-pilot loop guard] You are repeating a cycle of %d tool calls and got the same results every time, %d times in a row. This call is part of it: %s. "+
				"Repeating it will not change the answer. Stop calling it: use the result you already have, try a different approach, "+
				"or tell the user you are stuck.",
			period, reps, shown)
	case "backstop":
		return fmt.Sprintf(
			"[nav-pilot loop guard] You have made this exact tool call %d times in a row: %s. The results changed, but that is "+
				"waiting, not working. If you are waiting on something slow, use a command that blocks until it is done; otherwise "+
				"try something else or tell the user.",
			s.N, shown)
	}
	return ""
}

// LoopRule is the rule s trips, as telemetry names it: same_result, cycle,
// backstop, or "" for none.
func LoopRule(s LoopState, threshold int) string {
	period, reps := local.RepeatedCycle(s.Steps)
	switch {
	case s.Same >= SameResult(threshold):
		return "same_result"
	// A run of one call covering the whole cycle is a poll whose results
	// alternate: the backstop's case, not this one.
	case reps >= SameResult(threshold) && s.N < period*reps:
		return "cycle"
	case s.N >= threshold:
		return "backstop"
	}
	return ""
}

// SameResult is the same-result threshold for a given backstop, derived as
// local.SameResultRepeat derives it: half, and never below 2.
func SameResult(threshold int) int { return max(2, threshold/2) }

// shellTools are the shell tool names, as the hook artifacts' matcher
// (bash|shell|execute) lists them, plus PowerShell on Windows.
var shellTools = map[string]bool{"bash": true, "shell": true, "execute": true, "powershell": true}

// isShellCall reports whether a Signature is a shell tool's.
func isShellCall(sig string) bool {
	tool, _, _ := strings.Cut(sig, "(")
	return shellTools[strings.ToLower(tool)]
}

var unsafeID = regexp.MustCompile(`[^A-Za-z0-9_-]`)

// LoopGuard runs the rule for one postToolUse payload and returns the hook's
// stdout. The run is kept in the session's own directory under root (Copilot's
// session-state directory), which Copilot creates for every session and which
// cplt's sandbox lets the session write. The state is hashes and two counts,
// never the call or the result, with the steps capped at what the cycle rule
// needs to count up to the threshold.
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
//
// rule is the rule that tripped, "" when none did.
func LoopGuard(root string, p Payload, threshold int) (out, rule string, err error) {
	id := unsafeID.ReplaceAllString(p.SessionID, "")
	if id == "" || !p.HasResult || p.ToolName == "" {
		return NoChange, "", nil
	}
	dir := filepath.Join(root, id)
	path := filepath.Join(dir, "nav-pilot-loop-guard.json")

	var st LoopState
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, &st)
	}
	// The call is kept as a hash too: its arguments can hold anything.
	sig := Signature(p.ToolName, p.ToolArgs)
	sum := sha256.Sum256([]byte(sig))
	st = st.Step(hex.EncodeToString(sum[:]), p.ResultType, p.Result)
	if keep := local.MaxCyclePeriod * threshold; len(st.Steps) > keep {
		st.Steps = st.Steps[len(st.Steps)-keep:]
	}
	// A count that was not saved is not one to act on: the next call would
	// start from the old state again.
	if err := saveState(dir, path, st); err != nil {
		return NoChange, "", err
	}

	st.Call = sig
	msg := LoopMessage(st, threshold)
	if msg == "" {
		return NoChange, "", nil
	}
	return ModifiedResult(p.ResultType, msg+"\n\nThe tool result, unchanged:\n"+p.Result), LoopRule(st, threshold), nil
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
