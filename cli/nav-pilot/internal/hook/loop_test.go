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
		out = LoopGuard(dir, p, 8)
		// A second session doing the same thing in between must not add to s1's run.
		q, _ := ParsePayload(payload("s2", "bash", `{"command":"ls"}`, "a.go"))
		LoopGuard(dir, q, 8)
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
			if out := LoopGuard(dir, p, 2); out != NoChange {
				t.Errorf("%s: %s", name, out)
			}
		}
	}
	// A state directory that cannot be created is a pass, not a failure.
	blocker := filepath.Join(dir, "file")
	os.WriteFile(blocker, nil, 0o600)
	p, _ := ParsePayload(payload("s", "bash", `{}`, "x"))
	for range 10 {
		if out := LoopGuard(filepath.Join(blocker, "sub"), p, 2); out != NoChange {
			t.Errorf("unwritable state dir: %s", out)
		}
	}
}

func TestLoopGuardRemovesStaleSessions(t *testing.T) {
	dir := t.TempDir()
	old := filepath.Join(dir, "loop-old.json")
	os.WriteFile(old, []byte("{}"), 0o600)
	past := time.Now().Add(-2 * LoopStateTTL)
	os.Chtimes(old, past, past)

	p, _ := ParsePayload(payload("new", "bash", `{}`, "x"))
	LoopGuard(dir, p, 8)
	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Errorf("stale state survived a new session: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "loop-new.json")); err != nil {
		t.Errorf("new session has no state: %v", err)
	}
}
