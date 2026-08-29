package local

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// assistantCall is one assistant turn that made a tool call.
func assistantCall(name, args string) string {
	return fmt.Sprintf(
		`{"role":"assistant","tool_calls":[{"id":"call_%d","type":"function","function":{"name":%q,"arguments":%q}}]}`,
		len(name)+len(args), name, args)
}

const toolResult = `{"role":"tool","tool_call_id":"call_x","content":"same answer as last time"}`

// conversation builds a request body out of raw message objects.
func conversation(messages ...string) []byte {
	return []byte(`{"model":"m","messages":[` + strings.Join(messages, ",") + `]}`)
}

// repeat is n identical call/result pairs, the shape a runaway loop leaves in
// the message list.
func repeat(n int, name, args string) []string {
	var out []string
	for range n {
		out = append(out, assistantCall(name, args), toolResult)
	}
	return out
}

func TestRepeatedToolCall(t *testing.T) {
	user := `{"role":"user","content":"do the thing"}`

	tests := []struct {
		name     string
		messages []string
		wantN    int
		wantCall string
	}{
		{
			name:     "a fresh conversation is not a loop",
			messages: []string{user},
		},
		{
			name:     "one call is not a loop",
			messages: append([]string{user}, repeat(1, "read", `{"path":"a.go"}`)...),
			wantN:    1,
			wantCall: `read({"path":"a.go"})`,
		},
		{
			name:     "the measured failure: the same call over and over",
			messages: append([]string{user}, repeat(12, "read", `{"path":"a.go"}`)...),
			wantN:    12,
			wantCall: `read({"path":"a.go"})`,
		},
		{
			name: "different arguments are progress, not a loop",
			messages: append([]string{user},
				assistantCall("read", `{"path":"a.go"}`), toolResult,
				assistantCall("read", `{"path":"b.go"}`), toolResult,
				assistantCall("read", `{"path":"c.go"}`), toolResult),
			wantN:    1,
			wantCall: `read({"path":"c.go"})`,
		},
		{
			name: "an earlier run does not count once the model moved on",
			messages: append(append([]string{user}, repeat(9, "read", `{"path":"a.go"}`)...),
				assistantCall("write", `{"path":"a.go"}`), toolResult),
			wantN:    1,
			wantCall: `write({"path":"a.go"})`,
		},
		{
			name: "a user message ends the run",
			messages: append(append([]string{user}, repeat(9, "read", `{"path":"a.go"}`)...),
				user),
			wantN: 0,
		},
		{
			name: "an assistant message with no tool call ends the run",
			messages: append(append([]string{user}, repeat(9, "read", `{"path":"a.go"}`)...),
				`{"role":"assistant","content":"here is what I found"}`),
			wantN: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			call, n := repeatedToolCall(conversation(tt.messages...))
			if n != tt.wantN {
				t.Errorf("repeatedToolCall() n = %d, want %d", n, tt.wantN)
			}
			if tt.wantCall != "" && call != tt.wantCall {
				t.Errorf("repeatedToolCall() call = %q, want %q", call, tt.wantCall)
			}
		})
	}
}

// TestRepeatedToolCallIgnoresUnreadableBodies pins fail-open: a body the guard
// cannot parse is not a loop, so it is forwarded rather than refused.
func TestRepeatedToolCallIgnoresUnreadableBodies(t *testing.T) {
	for _, body := range []string{"", "not json", `{"messages":"nope"}`, `{}`} {
		if _, n := repeatedToolCall([]byte(body)); n != 0 {
			t.Errorf("repeatedToolCall(%q) = %d, want 0", body, n)
		}
	}
}

// TestGuardAbortsTheTurnOnARunawayLoop is the whole point of the package: a
// request that would extend a run of identical calls never reaches the server.
func TestGuardAbortsTheTurnOnARunawayLoop(t *testing.T) {
	var forwarded int
	handler := guardHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		forwarded++
		body, _ := json.Marshal(map[string]any{"forwarded": true})
		w.Write(body)
	}))

	post := func(body []byte) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(string(body)))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec
	}

	// One short of the threshold: still the model's business.
	under := post(conversation(repeat(loopGuardRepeat-1, "bash", `{"cmd":"ls"}`)...))
	if under.Code != http.StatusOK {
		t.Fatalf("a run of %d was refused: %d %s", loopGuardRepeat-1, under.Code, under.Body)
	}
	if forwarded != 1 {
		t.Fatalf("forwarded %d requests, want 1", forwarded)
	}

	over := post(conversation(repeat(loopGuardRepeat, "bash", `{"cmd":"ls"}`)...))
	if over.Code != http.StatusBadRequest {
		t.Fatalf("a run of %d was allowed through: %d", loopGuardRepeat, over.Code)
	}
	if forwarded != 1 {
		t.Errorf("the guard forwarded a request it should have refused (%d forwarded)", forwarded)
	}

	var parsed struct {
		Error struct {
			Message string `json:"message"`
			Type    string `json:"type"`
		} `json:"error"`
	}
	if err := json.Unmarshal(over.Body.Bytes(), &parsed); err != nil {
		t.Fatalf("the refusal is not JSON the client can render: %v", err)
	}
	if parsed.Error.Type != "nav_pilot_loop_guard" {
		t.Errorf("error type = %q, want nav_pilot_loop_guard", parsed.Error.Type)
	}
	// Naming the call is the requirement: "it looped" is not actionable.
	if !strings.Contains(parsed.Error.Message, `bash({"cmd":"ls"})`) {
		t.Errorf("the refusal does not name the repeated call: %q", parsed.Error.Message)
	}
}

// TestGuardForwardsEverythingElse: the guard is not a filter. Only the one
// request shape it understands is ever refused.
func TestGuardForwardsEverythingElse(t *testing.T) {
	var seen []string
	handler := guardHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := json.Marshal(r.URL.Path)
		seen = append(seen, r.Method+" "+r.URL.Path)
		w.Write(body)
	}))

	loop := conversation(repeat(loopGuardRepeat*3, "bash", `{"cmd":"ls"}`)...)
	for _, req := range []*http.Request{
		httptest.NewRequest(http.MethodGet, "/v1/models", nil),
		httptest.NewRequest(http.MethodPost, "/v1/completions", strings.NewReader(string(loop))),
	} {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("%s %s was refused: %d", req.Method, req.URL.Path, rec.Code)
		}
	}
	if len(seen) != 2 {
		t.Errorf("forwarded %v, want both requests through", seen)
	}
}

// TestGuardForwardsTheBodyItRead: reading the body to inspect it must not
// consume it.
func TestGuardForwardsTheBodyItRead(t *testing.T) {
	body := conversation(`{"role":"user","content":"hei"}`)
	var got []byte
	handler := guardHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, _ = io.ReadAll(r.Body)
	}))
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(string(body)))
	handler.ServeHTTP(httptest.NewRecorder(), req)
	if string(got) != string(body) {
		t.Errorf("upstream received %q, want the request body %q", got, body)
	}
}

func TestSetLoopGuardRepeatRefusesAThresholdThatIsNotAGuard(t *testing.T) {
	orig := loopGuardRepeat
	t.Cleanup(func() { loopGuardRepeat = orig })

	SetLoopGuardRepeat(20)
	if LoopGuardRepeat() != 20 {
		t.Errorf("LoopGuardRepeat() = %d, want 20", LoopGuardRepeat())
	}
	for _, n := range []int{1, 0, -3} {
		SetLoopGuardRepeat(n)
		if LoopGuardRepeat() != 20 {
			t.Errorf("SetLoopGuardRepeat(%d) took effect; a threshold below 2 disables dispatch rather than guarding it", n)
		}
	}
}

// TestStartGuardProxiesToTheServer covers the wiring end to end: a real
// listener, a real proxy, a real upstream.
func TestStartGuardProxiesToTheServer(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"choices":[]}`))
	}))
	defer upstream.Close()

	g, err := StartGuard(upstream.URL)
	if err != nil {
		t.Skipf("could not bind the guard port on this machine: %v", err)
	}
	defer g.Close()

	body := conversation(`{"role":"user","content":"hei"}`)
	resp, err := http.Post(g.URL()+"/v1/chat/completions", "application/json", strings.NewReader(string(body)))
	if err != nil {
		t.Fatalf("POST through the guard: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("guard returned %s for a request that is not a loop", resp.Status)
	}
}
