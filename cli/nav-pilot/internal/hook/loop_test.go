package hook

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// payload builds a postToolUse payload in the camelCase dialect.
func payload(session, tool, args, result string) []byte {
	b, _ := json.Marshal(map[string]any{
		"sessionId":  session,
		"toolName":   tool,
		"toolArgs":   json.RawMessage(args),
		"toolResult": map[string]string{"resultType": "success", "textResultForLlm": result},
	})
	return b
}

func TestParsePayloadReadsBothDialects(t *testing.T) {
	// Both shapes as Copilot CLI 1.0.89 sent them for `echo hi`.
	camel := `{"sessionId":"cf6d","timestamp":1790262473189,"cwd":"/w","toolName":"bash","toolArgs":{"command":"echo hi","description":"Echo hi"},"toolResult":{"resultType":"success","textResultForLlm":"hi\n<shellId: 0 completed with exit code 0>"}}`
	snake := `{"hook_event_name":"PostToolUse","session_id":"cf6d","timestamp":"2026-09-24T15:07:53.189Z","cwd":"/w","tool_name":"Bash","tool_input":{"command":"echo hi","description":"Echo hi"},"tool_result":{"result_type":"success","text_result_for_llm":"hi\n<shellId: 0 completed with exit code 0>"}}`
	for name, raw := range map[string]string{"camelCase": camel, "snake_case": snake} {
		p, err := ParsePayload([]byte(raw))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if p.SessionID != "cf6d" || !strings.EqualFold(p.ToolName, "bash") || !p.HasResult ||
			p.ResultType != "success" || !strings.HasPrefix(p.Result, "hi\n") || len(p.ToolArgs) == 0 {
			t.Errorf("%s: parsed %+v", name, p)
		}
	}
}

func TestSignatureIgnoresTheShellDescription(t *testing.T) {
	a := Signature("bash", json.RawMessage(`{"command":"gh run view 1","description":"Check the run"}`))
	b := Signature("bash", json.RawMessage(`{"description":"Look at the run again","command":"gh run view 1"}`))
	if a != b {
		t.Errorf("same command, different description: %q != %q", a, b)
	}
	if c := Signature("bash", json.RawMessage(`{"command":"gh run view 2"}`)); c == a {
		t.Errorf("different command gave the same signature %q", c)
	}
	// Outside the shell tool a description can be the argument that matters.
	x := Signature("create_issue", json.RawMessage(`{"title":"t","description":"first"}`))
	y := Signature("create_issue", json.RawMessage(`{"title":"t","description":"second"}`))
	if x == y {
		t.Errorf("a non-shell tool lost its description argument: %q", x)
	}
}

func TestLoopRule(t *testing.T) {
	const threshold = 8 // the default: same-result trips at 4
	same := func(int) string { return "status: queued" }
	stamped := func(i int) string {
		return fmt.Sprintf("%s run 4e1f9c2a1b status=queued elapsed %d.%ds",
			time.Date(2026, 9, 24, 10, 0, i, 0, time.UTC).Format(time.RFC3339), i, i+1)
	}
	growing := func(i int) string { return "build " + strings.Repeat("#", i+1) }

	tests := []struct {
		name     string
		calls    int
		result   func(int) string
		wantHit  bool
		wantSame bool // the same-result rule, not the backstop
	}{
		{"three identical answers are not yet a loop", 3, same, false, false},
		{"four identical answers are", 4, same, true, true},
		{"timestamps and ids do not hide an identical loop", 4, stamped, true, true},
		{"a poll whose output really changes runs to the backstop", 7, growing, false, false},
		{"and stops there", 8, growing, true, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var st LoopState
			for i := range tt.calls {
				st = st.Step(`bash({"command":"gh run view"})`, "success", tt.result(i))
			}
			msg := LoopMessage(st, threshold)
			if (msg != "") != tt.wantHit {
				t.Fatalf("after %d calls: message %q, want hit=%v (state %+v)", tt.calls, msg, tt.wantHit, st)
			}
			if tt.wantHit && strings.Contains(msg, "same result") != tt.wantSame {
				t.Errorf("wrong rule named: %q", msg)
			}
		})
	}
}

func TestLoopRuleResetsOnAnotherCall(t *testing.T) {
	var st LoopState
	for range 3 {
		st = st.Step("read(a)", "success", "x")
	}
	st = st.Step("read(b)", "success", "x")
	st = st.Step("read(a)", "success", "x")
	if st.N != 1 || st.Same != 1 {
		t.Errorf("run did not restart after another call: %+v", st)
	}
}

func TestLoopGuardKeepsStatePerSession(t *testing.T) {
	dir := t.TempDir()
	var out string
	for range 4 {
		p, _ := ParsePayload(payload("s1", "bash", `{"command":"ls"}`, "a.go"))
		out, _ = LoopGuard(dir, p, 8)
		// A second session doing the same thing in between must not add to s1's run.
		q, _ := ParsePayload(payload("s2", "bash", `{"command":"ls"}`, "a.go"))
		LoopGuard(dir, q, 8)
	}
	// The run lives in the session's own directory, where Copilot keeps the
	// session, as hashes and counts: neither the arguments nor the result.
	state, err := os.ReadFile(filepath.Join(dir, "s1", "nav-pilot-loop-guard.json"))
	if err != nil {
		t.Errorf("no state in the session directory: %v", err)
	}
	if strings.Contains(string(state), "ls") || strings.Contains(string(state), "a.go") {
		t.Errorf("state holds the call or the result: %s", state)
	}
	var got struct {
		ModifiedResult struct {
			ResultType string `json:"resultType"`
			Text       string `json:"textResultForLlm"`
		} `json:"modifiedResult"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil || !strings.Contains(got.ModifiedResult.Text, "loop guard") {
		t.Fatalf("4th identical call: output %s", out)
	}
	if got.ModifiedResult.ResultType != "success" || !strings.HasSuffix(got.ModifiedResult.Text, "a.go") {
		t.Errorf("the original result and type must survive the rewrite: %+v", got.ModifiedResult)
	}
}

func TestLoopGuardFailsOpen(t *testing.T) {
	dir := t.TempDir()
	for name, raw := range map[string][]byte{
		"no session id": payload("", "bash", `{}`, "x"),
		"no result":     []byte(`{"sessionId":"s","toolName":"bash","toolArgs":{}}`),
		"id of dots":    payload("../..", "bash", `{}`, "x"),
	} {
		p, _ := ParsePayload(raw)
		for range 10 {
			if out, _ := LoopGuard(dir, p, 2); out != NoChange {
				t.Errorf("%s: %s", name, out)
			}
		}
	}
	// A state directory that cannot be created is a pass, and says so.
	blocker := filepath.Join(dir, "file")
	os.WriteFile(blocker, nil, 0o600)
	p, _ := ParsePayload(payload("s", "bash", `{}`, "x"))
	for range 10 {
		out, err := LoopGuard(blocker, p, 2)
		if out != NoChange {
			t.Errorf("unwritable state dir: %s", out)
		}
		if err == nil {
			t.Errorf("unwritable state dir: no error to report")
		}
	}
}

func TestLoopRuleCatchesShortCycles(t *testing.T) {
	const threshold = 8 // the default: a cycle trips after 4 repeats
	type call struct{ sig, result string }
	times := func(n int, cycle ...call) []call {
		var out []call
		for range n {
			out = append(out, cycle...)
		}
		return out
	}
	a, b, c := call{"view(a)", "x"}, call{"view(a,[1,-1])", "x"}, call{"view(a,[1,1])", "x"}
	var edits []call
	for i := range 5 {
		edits = append(edits, call{fmt.Sprintf("edit(v%d)", i), "ok"}, call{"bash(go test)", "FAIL " + strings.Repeat("x", i+1)})
	}

	tests := []struct {
		name    string
		calls   []call
		wantHit bool
	}{
		{"A B C with the same results, three times", times(3, a, b, c), false},
		{"four times", times(4, a, b, c), true},
		{"A B A B, four times", times(4, a, b), true},
		{"a cycle whose results change", []call{
			{"read(x)", "a"}, {"read(y)", "b"}, {"read(x)", "aa"}, {"read(y)", "bb"},
			{"read(x)", "aaa"}, {"read(y)", "bbb"}, {"read(x)", "aaaa"}, {"read(y)", "bbbb"},
		}, false},
		{"edit then test, new edit and new output each time", edits, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var st LoopState
			for _, c := range tt.calls {
				st = st.Step(c.sig, "success", c.result)
			}
			msg := LoopMessage(st, threshold)
			if (msg != "") != tt.wantHit {
				t.Fatalf("message %q, want hit=%v", msg, tt.wantHit)
			}
			if tt.wantHit && (!strings.Contains(msg, "repeating a cycle of") || !strings.Contains(msg, "same results")) {
				t.Errorf("a cycle must be named as one: %q", msg)
			}
		})
	}
}

// One call whose results alternate is a poll that changed: at the threshold
// the backstop names it, not the cycle rule.
func TestLoopRuleLeavesAlternatingPollsToTheBackstop(t *testing.T) {
	var st LoopState
	for i := range 8 {
		st = st.Step("poll", "success", []string{"running", "queued"}[i%2])
	}
	msg := LoopMessage(st, 8)
	if !strings.Contains(msg, "The results changed") || strings.Contains(msg, "cycle") {
		t.Errorf("wrong rule named: %q", msg)
	}
}

// TestLoopGuardReplaysTheGPT5MiniEvasion replays the calls gpt-5-mini made in
// Copilot session 3d20bfa3 (navikt/mlx-workspace
// bench/loop-hook-20260925-002708.json): 23 identical reads of a file that
// said "status: waiting", warned from the 4th, then 54 more that cycled the
// `view` range between none, [1,-1] and [1,1] with the same answers, which
// the one-call rules never saw. Only the shape is kept: the path and the file
// are stand-ins.
func TestLoopGuardReplaysTheGPT5MiniEvasion(t *testing.T) {
	dir := t.TempDir()
	const path = `"path":"/w/ready.txt"`
	whole, line := "status: waiting\n", "status: waiting"
	seq := make([][2]string, 0, 77)
	for range 23 {
		seq = append(seq, [2]string{`{` + path + `}`, whole})
	}
	for range 18 {
		seq = append(seq,
			[2]string{`{` + path + `,"view_range":[1,-1]}`, whole},
			[2]string{`{` + path + `,"view_range":[1,1]}`, line},
			[2]string{`{` + path + `}`, whole})
	}
	var warned []int
	for i, s := range seq {
		p, _ := ParsePayload(payload("3d20bfa3", "view", s[0], s[1]))
		out, err := LoopGuard(dir, p, 8)
		if err != nil {
			t.Fatal(err)
		}
		if out != NoChange {
			warned = append(warned, i+1)
		}
	}
	// Calls 4–23 as before; then, once the cycle has gone round four times,
	// every call to the end instead of none. The last plain read is the first
	// step of the cycle, so the fourth round ends on call 23+4*3-1.
	want := []int{}
	for n := 4; n <= 23; n++ {
		want = append(want, n)
	}
	for n := 23 + 4*3 - 1; n <= len(seq); n++ {
		want = append(want, n)
	}
	if fmt.Sprint(warned) != fmt.Sprint(want) {
		t.Errorf("warned on calls %v,\nwant %v", warned, want)
	}
	state, _ := os.ReadFile(filepath.Join(dir, "3d20bfa3", "nav-pilot-loop-guard.json"))
	var st LoopState
	if err := json.Unmarshal(state, &st); err != nil || len(st.Steps) != 3*8 {
		t.Errorf("steps kept: %d, want the cap of %d (%v)", len(st.Steps), 3*8, err)
	}
	if strings.Contains(string(state), "ready") || strings.Contains(string(state), "waiting") {
		t.Errorf("state holds the call or the result: %s", state)
	}
}
