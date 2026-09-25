package local

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

// stubOwnership replaces the guard's ownership proof. The real one needs a
// recorded server and an lsof that agrees with it, which a unit test has no way
// to arrange; every test below that is about the loop guard says "yes, it is
// still ours" and moves on.
func stubOwnership(t *testing.T, f func() error) {
	t.Helper()
	orig := ownershipCheck
	ownershipCheck = f
	t.Cleanup(func() { ownershipCheck = orig })
}

// assistantCall is one assistant turn that made a tool call. Its id pairs with
// no result, so on its own it is a call whose result is not in the request.
func assistantCall(name, args string) string {
	return fmt.Sprintf(
		`{"role":"assistant","tool_calls":[{"id":"call_%d","type":"function","function":{"name":%q,"arguments":%q}}]}`,
		len(name)+len(args), name, args)
}

// toolResult answers a call id no assistant message made: an unpaired result.
const toolResult = `{"role":"tool","tool_call_id":"call_x","content":"same answer as last time"}`

// step is one call and its result, paired by id the way both clients send them.
func step(id, name, args, result string) []string {
	return []string{
		fmt.Sprintf(`{"role":"assistant","tool_calls":[{"id":%q,"type":"function","function":{"name":%q,"arguments":%q}}]}`,
			id, name, args),
		fmt.Sprintf(`{"role":"tool","tool_call_id":%q,"content":%q}`, id, result),
	}
}

// conversation builds a request body out of raw message objects.
func conversation(messages ...string) []byte {
	return []byte(`{"model":"m","messages":[` + strings.Join(messages, ",") + `]}`)
}

// repeat is n identical calls that each got the same answer back, the shape a
// runaway loop leaves in the message list.
func repeat(n int, name, args string) []string {
	return poll(n, name, args, func(int) string { return "same answer as last time" })
}

// poll is n identical calls whose i-th result is result(i).
func poll(n int, name, args string, result func(i int) string) []string {
	var out []string
	for i := range n {
		out = append(out, step(fmt.Sprintf("call_%d", i), name, args, result(i))...)
	}
	return out
}

func TestRepeatedToolCall(t *testing.T) {
	user := `{"role":"user","content":"do the thing"}`
	// Progress a normaliser cannot mistake for noise: the output grows. A
	// changing number alone would normalise away (see NormaliseResult).
	changing := func(i int) string { return "build " + strings.Repeat("#", i+1) }
	stamped := func(i int) string {
		return fmt.Sprintf("%s run 4e1f9c2a1b status=queued elapsed %d.%ds", time.Date(2026, 9, 24, 10, 0, i, 0, time.UTC).Format(time.RFC3339), i, i)
	}

	tests := []struct {
		name     string
		messages []string
		wantN    int
		wantSame int
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
			wantSame: 1,
			wantCall: `read({"path":"a.go"})`,
		},
		{
			name:     "the measured failure: the same call and the same answer over and over",
			messages: append([]string{user}, repeat(12, "read", `{"path":"a.go"}`)...),
			wantN:    12,
			wantSame: 12,
			wantCall: `read({"path":"a.go"})`,
		},
		{
			name:     "a poll whose output changes is the same call but not the same result",
			messages: append([]string{user}, poll(7, "bash", `{"cmd":"make status"}`, changing)...),
			wantN:    7,
			wantSame: 1,
		},
		{
			name: "a poll whose 7th result differs resets the same-result run",
			messages: append([]string{user}, poll(7, "bash", `{"cmd":"make status"}`, func(i int) string {
				if i == 6 {
					return "done"
				}
				return "running"
			})...),
			wantN:    7,
			wantSame: 1,
		},
		{
			name:     "timestamps, ids and durations do not make a stuck poll look like progress",
			messages: append([]string{user}, poll(6, "bash", `{"cmd":"gh run view"}`, stamped)...),
			wantN:    6,
			wantSame: 6,
			wantCall: `bash({"cmd":"gh run view"})`,
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
				step("w", "write", `{"path":"a.go"}`, "ok")...),
			wantN:    1,
			wantSame: 1,
			wantCall: `write({"path":"a.go"})`,
		},
		{
			name: "a user message ends the run",
			messages: append(append([]string{user}, repeat(9, "read", `{"path":"a.go"}`)...),
				user),
		},
		{
			name: "an assistant message with no tool call ends the run",
			messages: append(append([]string{user}, repeat(9, "read", `{"path":"a.go"}`)...),
				`{"role":"assistant","content":"here is what I found"}`),
		},
		{
			name: "parallel calls: the whole set and all its results repeat",
			messages: []string{user,
				`{"role":"assistant","tool_calls":[{"id":"a1","function":{"name":"read","arguments":"x"}},{"id":"b1","function":{"name":"read","arguments":"y"}}]}`,
				`{"role":"tool","tool_call_id":"b1","content":"Y"}`,
				`{"role":"tool","tool_call_id":"a1","content":"X"}`,
				`{"role":"assistant","tool_calls":[{"id":"a2","function":{"name":"read","arguments":"x"}},{"id":"b2","function":{"name":"read","arguments":"y"}}]}`,
				`{"role":"tool","tool_call_id":"a2","content":"X"}`,
				`{"role":"tool","tool_call_id":"b2","content":"Y"}`,
			},
			wantN:    2,
			wantSame: 2,
			wantCall: `read(x), read(y)`,
		},
		{
			name: "parallel calls: the same set in another order is the same step",
			messages: []string{user,
				`{"role":"assistant","tool_calls":[{"id":"a1","function":{"name":"read","arguments":"x"}},{"id":"b1","function":{"name":"read","arguments":"y"}}]}`,
				`{"role":"tool","tool_call_id":"a1","content":"X"}`,
				`{"role":"tool","tool_call_id":"b1","content":"Y"}`,
				`{"role":"assistant","tool_calls":[{"id":"b2","function":{"name":"read","arguments":"y"}},{"id":"a2","function":{"name":"read","arguments":"x"}}]}`,
				`{"role":"tool","tool_call_id":"b2","content":"Y"}`,
				`{"role":"tool","tool_call_id":"a2","content":"X"}`,
			},
			wantN:    2,
			wantSame: 2,
			wantCall: `read(x), read(y)`,
		},
		{
			name: "parallel calls: results swapped between the calls are not the same result",
			messages: []string{user,
				`{"role":"assistant","tool_calls":[{"id":"a1","function":{"name":"read","arguments":"x"}},{"id":"b1","function":{"name":"read","arguments":"y"}}]}`,
				`{"role":"tool","tool_call_id":"a1","content":"X"}`,
				`{"role":"tool","tool_call_id":"b1","content":"Y"}`,
				`{"role":"assistant","tool_calls":[{"id":"b2","function":{"name":"read","arguments":"y"}},{"id":"a2","function":{"name":"read","arguments":"x"}}]}`,
				`{"role":"tool","tool_call_id":"b2","content":"X"}`,
				`{"role":"tool","tool_call_id":"a2","content":"Y"}`,
			},
			wantN:    2,
			wantSame: 1,
		},
		{
			name: "parallel calls: one changed result breaks the same-result run",
			messages: []string{user,
				`{"role":"assistant","tool_calls":[{"id":"a1","function":{"name":"read","arguments":"x"}},{"id":"b1","function":{"name":"read","arguments":"y"}}]}`,
				`{"role":"tool","tool_call_id":"a1","content":"X"}`,
				`{"role":"tool","tool_call_id":"b1","content":"Y"}`,
				`{"role":"assistant","tool_calls":[{"id":"a2","function":{"name":"read","arguments":"x"}},{"id":"b2","function":{"name":"read","arguments":"y"}}]}`,
				`{"role":"tool","tool_call_id":"a2","content":"X"}`,
				`{"role":"tool","tool_call_id":"b2","content":"Y2"}`,
			},
			wantN:    2,
			wantSame: 1,
		},
		{
			name:     "the newest call has no result yet: the pairs before it decide",
			messages: append(append([]string{user}, repeat(5, "read", `{"path":"a.go"}`)...), assistantCall("read", `{"path":"a.go"}`)),
			wantN:    6,
			wantSame: 5,
		},
		{
			name: "an unpaired result in the middle ends the same-result run",
			messages: append(append(append([]string{user}, repeat(3, "read", `{"path":"a.go"}`)...),
				assistantCall("read", `{"path":"a.go"}`), toolResult),
				repeat(2, "read", `{"path":"a.go"}`)...),
			wantN:    6,
			wantSame: 2,
		},
		{
			name:     "results that never pair count no same-result run",
			messages: append([]string{user}, assistantCall("bash", "ls"), toolResult, assistantCall("bash", "ls"), toolResult),
			wantN:    2,
			wantSame: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			call, n, same, _, _ := repeatedToolCall(conversation(tt.messages...))
			if n != tt.wantN || same != tt.wantSame {
				t.Errorf("repeatedToolCall() n, same = %d, %d, want %d, %d", n, same, tt.wantN, tt.wantSame)
			}
			if tt.wantCall != "" && call != tt.wantCall {
				t.Errorf("repeatedToolCall() call = %q, want %q", call, tt.wantCall)
			}
		})
	}
}

// TestRepeatedToolCallCycles pins the cycle rule: a model warned about one
// repeated call can keep looping by cycling between a few calls that all
// answer the same.
func TestRepeatedToolCallCycles(t *testing.T) {
	user := `{"role":"user","content":"do the thing"}`
	// calls is one step per "name args => result" entry, each with its own id.
	calls := func(seq ...string) []string {
		out := []string{user}
		for i, c := range seq {
			call, result, _ := strings.Cut(c, " => ")
			name, args, _ := strings.Cut(call, " ")
			out = append(out, step(fmt.Sprintf("call_%d", i), name, args, result)...)
		}
		return out
	}
	times := func(n int, seq ...string) []string {
		var out []string
		for range n {
			out = append(out, seq...)
		}
		return out
	}
	a, b, c := `view {"path":"f"} => x`, `view {"path":"f","view_range":[1,-1]} => x`, `view {"path":"f","view_range":[1,1]} => x`
	edit := func(i int) string { return fmt.Sprintf(`edit {"line":"v%d"} => ok`, i) }
	test := func(i int) string { return "bash go test => FAIL " + strings.Repeat("x", i+1) }

	tests := []struct {
		name       string
		seq        []string
		wantPeriod int
		wantReps   int
	}{
		{"A B C with the same results, repeated", times(4, a, b, c), 3, 4},
		{"one short of four repeats", times(4, a, b, c)[1:], 3, 3},
		{"A B A B", times(4, a, b), 2, 4},
		{"a cycle whose results change is not a repeat", []string{
			"read x => 1a", "read y => b", "read x => 2a", "read y => bb", "read x => 3aa", "read y => bbb",
			"read x => 4aaa", "read y => bbbb",
		}, 0, 0},
		{"edit then test, with a new edit and new output each time", []string{
			edit(0), test(0), edit(1), test(1), edit(2), test(2), edit(3), test(3), edit(4), test(4),
		}, 0, 0},
		{"one call over and over is the one-call rule's, not a cycle", times(8, a), 0, 0},
		{"a cycle only counts from where it starts", append(times(5, a), times(2, b, c)...), 2, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, _, cycle, reps := repeatedToolCall(conversation(calls(tt.seq...)...))
			if len(cycle) != tt.wantPeriod || reps != tt.wantReps {
				t.Errorf("cycle %q repeated %d times, want %d calls repeated %d times", cycle, reps, tt.wantPeriod, tt.wantReps)
			}
		})
	}

	// The cycle is named in the order the model made the calls.
	_, _, _, cycle, _ := repeatedToolCall(conversation(calls(times(4, a, b, c)...)...))
	want := []string{`view({"path":"f"})`, `view({"path":"f","view_range":[1,-1]})`, `view({"path":"f","view_range":[1,1]})`}
	if !slices.Equal(cycle, want) {
		t.Errorf("cycle = %q, want %q", cycle, want)
	}
}

// TestRepeatedToolCallIgnoresUnreadableBodies pins fail-open: a body the guard
// cannot parse is not a loop, so it is forwarded rather than refused.
func TestRepeatedToolCallIgnoresUnreadableBodies(t *testing.T) {
	for _, body := range []string{"", "not json", `{"messages":"nope"}`, `{}`} {
		if _, n, _, _, _ := repeatedToolCall([]byte(body)); n != 0 {
			t.Errorf("repeatedToolCall(%q) = %d, want 0", body, n)
		}
	}
}

// TestGuardAbortsTheTurnOnARunawayLoop is the whole point of the package: a
// request that would extend a run of identical calls never reaches the server.
func TestGuardAbortsTheTurnOnARunawayLoop(t *testing.T) {
	stubDirs(t) // lockServer writes under HOME; without this the suite flocks the developer's own
	stubOwnership(t, func() error { return nil })
	var forwarded int
	handler := guardHandler(&Guard{}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		forwarded++
		body, _ := json.Marshal(map[string]any{"forwarded": true})
		w.Write(body)
	}), "")

	post := func(body []byte) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(string(body)))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec
	}

	changing := func(i int) string { return "tick " + strings.Repeat(".", i+1) }
	tests := []struct {
		name     string
		messages []string
		refuse   bool
		wantMsg  []string
		notMsg   []string
	}{
		{
			name:     "same call, same result, one short of the threshold",
			messages: repeat(SameResultRepeat()-1, "bash", `{"cmd":"ls"}`),
		},
		{
			name:     "same call, same result, at the threshold",
			messages: repeat(SameResultRepeat(), "bash", `{"cmd":"ls"}`),
			refuse:   true,
			wantMsg:  []string{fmt.Sprintf("repeated the same tool call with the same result %d times", SameResultRepeat()), "will not change the answer", "try something else"},
		},
		{
			name: "a cycle of calls with the same results, at the threshold",
			messages: func() []string {
				var out []string
				for i := range SameResultRepeat() {
					out = append(out, step(fmt.Sprintf("l%d", i), "bash", `{"cmd":"ls"}`, "a.go")...)
					out = append(out, step(fmt.Sprintf("p%d", i), "bash", `{"cmd":"pwd"}`, "/w")...)
				}
				return out
			}(),
			refuse:  true,
			wantMsg: []string{fmt.Sprintf("repeating a cycle of 2 tool calls and got the same results %d times", SameResultRepeat()), `bash({"cmd":"pwd"})`, "will not change the answer"},
		},
		{
			name:     "a changing poll below the backstop",
			messages: poll(loopGuardRepeat-1, "bash", `{"cmd":"ls"}`, changing),
		},
		{
			name:     "the backstop stops an endless poll even though its results change",
			messages: poll(loopGuardRepeat, "bash", `{"cmd":"ls"}`, changing),
			refuse:   true,
			wantMsg:  []string{fmt.Sprintf("made the same tool call %d times in a row, even though the results changed", loopGuardRepeat), "try another approach", "local_loop_guard"},
			// Its results did change, so it must not be told otherwise.
			notMsg: []string{"will not change the answer", "not progress"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := forwarded
			rec := post(conversation(tt.messages...))
			if !tt.refuse {
				if rec.Code != http.StatusOK || forwarded != before+1 {
					t.Fatalf("refused: %d %s", rec.Code, rec.Body)
				}
				return
			}
			if rec.Code != http.StatusBadRequest || forwarded != before {
				t.Fatalf("allowed through: %d, forwarded %d", rec.Code, forwarded-before)
			}
			var parsed struct {
				Error struct {
					Message string `json:"message"`
					Type    string `json:"type"`
					Code    string `json:"code"`
				} `json:"error"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &parsed); err != nil {
				t.Fatalf("the refusal is not JSON the client can render: %v", err)
			}
			if parsed.Error.Type != "nav_pilot_loop_guard" || parsed.Error.Code != "loop_guard" {
				t.Errorf("error type, code = %q, %q, want nav_pilot_loop_guard, loop_guard", parsed.Error.Type, parsed.Error.Code)
			}
			// Naming the call is the requirement: "it looped" is not actionable.
			for _, want := range append([]string{`bash({"cmd":"ls"})`}, tt.wantMsg...) {
				if !strings.Contains(parsed.Error.Message, want) {
					t.Errorf("the refusal does not say %q: %q", want, parsed.Error.Message)
				}
			}
			for _, not := range tt.notMsg {
				if strings.Contains(parsed.Error.Message, not) {
					t.Errorf("the refusal says %q: %q", not, parsed.Error.Message)
				}
			}
		})
	}
}

func TestSameResultRepeatIsHalfTheBackstop(t *testing.T) {
	orig := loopGuardRepeat
	t.Cleanup(func() { loopGuardRepeat = orig })
	for _, tt := range []struct{ backstop, want int }{{8, 4}, {20, 10}, {3, 2}, {2, 2}} {
		loopGuardRepeat = tt.backstop
		if got := SameResultRepeat(); got != tt.want {
			t.Errorf("local_loop_guard %d: SameResultRepeat() = %d, want %d", tt.backstop, got, tt.want)
		}
	}
}

// TestGuardForwardsEverythingElse: the guard is not a filter. Only the one
// request shape it understands is ever refused.
func TestGuardForwardsEverythingElse(t *testing.T) {
	stubDirs(t) // lockServer writes under HOME; without this the suite flocks the developer's own
	stubOwnership(t, func() error { return nil })
	var seen []string
	handler := guardHandler(&Guard{}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := json.Marshal(r.URL.Path)
		seen = append(seen, r.Method+" "+r.URL.Path)
		w.Write(body)
	}), "")

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
	stubDirs(t) // lockServer writes under HOME; without this the suite flocks the developer's own
	stubOwnership(t, func() error { return nil })
	body := conversation(`{"role":"user","content":"hei"}`)
	var got []byte
	handler := guardHandler(&Guard{}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, _ = io.ReadAll(r.Body)
	}), "")
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(string(body)))
	handler.ServeHTTP(httptest.NewRecorder(), req)
	if string(got) != string(body) {
		t.Errorf("upstream received %q, want the request body %q", got, body)
	}
}

// TestGuardSampling: with no sampling in the manifest the body is forwarded
// byte for byte; with it, the manifest's values replace the client's, whether
// the client sent an explicit 0 (the Copilot CLI) or nothing (opencode).
func TestGuardSampling(t *testing.T) {
	stubDirs(t) // lockServer writes under HOME; without this the suite flocks the developer's own
	stubOwnership(t, func() error { return nil })
	send := func(g *Guard, path, body string) []byte {
		var got []byte
		handler := guardHandler(g, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			got, _ = io.ReadAll(r.Body)
		}), "")
		handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, path, strings.NewReader(body)))
		return got
	}
	copilot := `{"model":"m","messages":[{"role":"user","content":"<hei>"}],"temperature":0,"top_p":0.95}`
	opencode := `{"model":"m","messages":[{"role":"user","content":"hei"}]}`

	if got := send(&Guard{}, "/v1/chat/completions", copilot); string(got) != copilot {
		t.Errorf("unset: upstream received %s, want the body untouched %s", got, copilot)
	}

	sampling, err := samplingOverride(map[string]string{"MLX_NAV_PILOT_TEMPERATURE": "0.7", "MLX_NAV_PILOT_TOP_P": "0.8"})
	if err != nil {
		t.Fatal(err)
	}
	for _, body := range []string{copilot, opencode} {
		var got struct {
			Temperature *float64 `json:"temperature"`
			TopP        *float64 `json:"top_p"`
			Messages    []any    `json:"messages"`
		}
		if err := json.Unmarshal(send(&Guard{sampling: sampling}, "/v1/chat/completions", body), &got); err != nil {
			t.Fatalf("set: upstream body is not JSON: %v", err)
		}
		if got.Temperature == nil || *got.Temperature != 0.7 || got.TopP == nil || *got.TopP != 0.8 || len(got.Messages) != 1 {
			t.Errorf("set: %s forwarded as %+v, want temperature 0.7, top_p 0.8 and the messages kept", body, got)
		}
	}

	if got := send(&Guard{sampling: sampling}, "/v1/completions", copilot); string(got) != copilot {
		t.Errorf("non-chat endpoint: upstream received %s, want the body untouched", got)
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
	stubDirs(t) // lockServer writes under HOME; without this the suite flocks the developer's own
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"choices":[]}`))
	}))
	defer upstream.Close()

	stubOwnership(t, func() error { return nil })
	g, err := StartGuard(upstream.URL, Model{})
	if err != nil {
		t.Fatalf("StartGuard: %v", err)
	}
	// A defer and not a t.Cleanup, and it matters which: cleanups run after
	// every defer in the test body, so this closes the guard, and waits for its
	// handler goroutines, before stubDirs writes the directory overrides back.
	// The handler reads those through statePath while it is serving, so without
	// the wait the two overlap and -race says so.
	defer func() {
		if err := g.Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
	}()

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

// TestGuardCloseWaitsForItsHandlers pins what the loop-guard data race was
// really about. http.Server.Close returns as soon as the connections are cut,
// with the goroutines that were serving them still unwinding and still reading
// the package-level directory overrides; a test's cleanup then writes those
// back underneath a live reader, and a session's launch tears down around one.
// Close has to outlast its handlers, so the read is finished rather than merely
// unreported.
//
// The handler is held inside the ownership check rather than at the upstream,
// because cutting the connection cancels the request context and a proxied call
// unwinds on its own: what has to be proven is that Close waits even for work
// no cancellation reaches.
func TestGuardCloseWaitsForItsHandlers(t *testing.T) {
	stubDirs(t)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"choices":[]}`))
	}))
	defer upstream.Close()

	inHandler, release := make(chan struct{}), make(chan struct{})
	var handlerDone atomic.Bool
	stubOwnership(t, func() error {
		close(inHandler)
		<-release
		handlerDone.Store(true)
		return nil
	})

	g, err := StartGuard(upstream.URL, Model{})
	if err != nil {
		t.Fatalf("StartGuard: %v", err)
	}
	body := conversation(`{"role":"user","content":"hei"}`)
	go func() {
		resp, err := http.Post(g.URL()+"/v1/chat/completions", "application/json", strings.NewReader(string(body)))
		if err == nil {
			resp.Body.Close()
		}
	}()
	<-inHandler

	// The handshake is not decoration. Without it the goroutine below might not
	// have been scheduled at all inside the window, and "Close has not returned
	// yet" would pass for a Close that had never been called. Waiting for
	// entering means the window times a Close that has actually begun.
	entering, closed := make(chan struct{}), make(chan error, 1)
	go func() {
		close(entering)
		closed <- g.Close()
	}()
	<-entering
	select {
	case err := <-closed:
		t.Fatalf("Close returned while a handler was still running: %v", err)
	case <-time.After(100 * time.Millisecond):
	}

	close(release)
	select {
	case err := <-closed:
		if err != nil {
			t.Fatalf("Close: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Close never returned after the handler finished")
	}
	if !handlerDone.Load() {
		t.Error("Close returned before the handler had finished")
	}
}

// TestGuardCloseDoesNotWaitForAnotherSessionsLock is the bound on that wait.
//
// Waiting for the handlers is only safe if something reliably tells them to
// stop, and cutting the connection does not: net/http cancels a request context
// on connection loss from the background read of the request body, and for a
// body the handler has not read yet that read is deferred until EOF. The guard
// queues on the machine-wide server lock well before it reads a body, so a
// handler parked there when the session ends has a live context and a lock held
// by somebody else.
//
// Two terminals is ordinary work: session A is mid-completion holding the lock,
// session B has a prompt queued behind it and is interrupted. B's exit must not
// wait for A, which may be minutes away, or forever if A is in the wedged state
// the lock exists to guard against.
func TestGuardCloseDoesNotWaitForAnotherSessionsLock(t *testing.T) {
	data, _ := stubDirs(t)
	stubOwnership(t, func() error { return nil })
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"choices":[]}`))
	}))
	defer upstream.Close()

	// The other session, holding the lock for the whole test and never giving it
	// up. lockServer polls for it once a second and would never be handed it.
	held, err := os.OpenFile(filepath.Join(data, "server.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatalf("opening the lock: %v", err)
	}
	defer held.Close()
	if err := syscall.Flock(int(held.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		t.Fatalf("taking the lock: %v", err)
	}

	g, err := StartGuard(upstream.URL, Model{})
	if err != nil {
		t.Fatalf("StartGuard: %v", err)
	}
	body := conversation(`{"role":"user","content":"hei"}`)
	go func() {
		resp, err := http.Post(g.URL()+"/v1/chat/completions", "application/json", strings.NewReader(string(body)))
		if err == nil {
			resp.Body.Close()
		}
	}()
	// Completions is incremented on the line before lockServer, so this says the
	// handler is at the lock and nowhere earlier. The pause covers the few
	// instructions between the two.
	deadline := time.Now().Add(10 * time.Second)
	for g.Completions() == 0 {
		if time.Now().After(deadline) {
			t.Fatal("the request never reached the guard")
		}
		time.Sleep(time.Millisecond)
	}
	time.Sleep(200 * time.Millisecond)

	closed := make(chan error, 1)
	go func() { closed <- g.Close() }()
	select {
	case err := <-closed:
		if err != nil {
			t.Fatalf("Close: %v", err)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("Close blocked behind a lock held by another session")
	}
}

// TestGuardRefusesAServerItCanNoLongerVouchFor is the hole the launch-time
// check left open: it proved ownership once and the guard then proxied to a
// fixed 127.0.0.1:8080 for the rest of the day, so a server that died at noon
// and left the port to whatever bound it next had every later prompt forwarded
// to a stranger, silently.
func TestGuardRefusesAServerItCanNoLongerVouchFor(t *testing.T) {
	stubDirs(t) // lockServer writes under HOME; without this the suite flocks the developer's own
	origTTL := ownershipTTL
	ownershipTTL = 0
	t.Cleanup(func() { ownershipTTL = origTTL })

	var lost error
	stubOwnership(t, func() error { return lost })

	var forwarded int
	handler := guardHandler(&Guard{}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		forwarded++
	}), "")
	post := func() *httptest.ResponseRecorder {
		body := conversation(`{"role":"user","content":"hei"}`)
		req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(string(body)))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec
	}

	if rec := post(); rec.Code != http.StatusOK || forwarded != 1 {
		t.Fatalf("a prompt to a server nav-pilot owns = %d (forwarded %d), want 200 and forwarded", rec.Code, forwarded)
	}

	// The server dies mid-session and something else takes the port.
	lost = fmt.Errorf("the recorded local m server (pid 4242) is not what is listening on 127.0.0.1:%d", DefaultPort)

	rec := post()
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("a prompt was forwarded to a server nav-pilot cannot vouch for: %d", rec.Code)
	}
	if forwarded != 1 {
		t.Errorf("the guard forwarded a request it refused (%d forwarded, want 1)", forwarded)
	}

	// Same envelope as the loop guard, so the client renders a message rather
	// than a transport failure.
	var parsed struct {
		Error struct {
			Message string `json:"message"`
			Type    string `json:"type"`
			Code    string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &parsed); err != nil {
		t.Fatalf("the refusal is not JSON the client can render: %v", err)
	}
	if parsed.Error.Type != "nav_pilot_local_server_lost" || parsed.Error.Code != "local_server_lost" {
		t.Errorf("error type/code = %q/%q, want nav_pilot_local_server_lost/local_server_lost", parsed.Error.Type, parsed.Error.Code)
	}
	if !strings.Contains(parsed.Error.Message, lost.Error()) {
		t.Errorf("the refusal does not say what went wrong: %q", parsed.Error.Message)
	}
}

// TestGuardOwnershipCheckIsCachedBetweenPrompts is what makes the check
// affordable: it shells out to ps and lsof, and a session sends many prompts.
// One proof covers every prompt inside the TTL.
func TestGuardOwnershipCheckIsCachedBetweenPrompts(t *testing.T) {
	stubDirs(t) // lockServer writes under HOME; without this the suite flocks the developer's own
	var checks int
	stubOwnership(t, func() error {
		checks++
		return nil
	})
	handler := guardHandler(&Guard{}, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}), "")
	for range 5 {
		body := conversation(`{"role":"user","content":"hei"}`)
		req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(string(body)))
		handler.ServeHTTP(httptest.NewRecorder(), req)
	}
	if checks != 1 {
		t.Errorf("the ownership check ran %d times for five prompts inside the TTL, want 1", checks)
	}
}

// TestGuardChecksOwnershipOnlyForCompletions: the check costs two subprocesses,
// and everything the guard forwards untouched must stay free.
func TestGuardChecksOwnershipOnlyForCompletions(t *testing.T) {
	var checks int
	stubOwnership(t, func() error {
		checks++
		return nil
	})
	handler := guardHandler(&Guard{}, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}), "")
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/v1/models", nil))
	if checks != 0 {
		t.Errorf("GET /v1/models ran the ownership check %d times, want 0", checks)
	}
}

// TestGuardReadsARealCopilotCLIRequest is the evidence that the loop guard
// works for the Copilot CLI and not only for opencode.
//
// The two clients reach the same server, but nothing guaranteed they describe a
// tool call the same way, and a guard that quietly forwards everything is worse
// than no guard: the launch tells the developer a runaway loop will be stopped.
//
// testdata/copilot-cli-loop.json is a request captured from Copilot CLI 1.0.81
// in BYOK mode (COPILOT_PROVIDER_BASE_URL at a recording proxy, wire API
// "completions"), answered every time with the same `view` call so the client
// looped for real. Only the parts the guard never reads were shortened — the
// tool catalogue, and the system, user and tool-result contents. Every message
// role, every tool_calls object and every argument string is what the client
// sent.
func TestGuardReadsARealCopilotCLIRequest(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("testdata", "copilot-cli-loop.json"))
	if err != nil {
		t.Fatal(err)
	}
	call, n, same, _, _ := repeatedToolCall(body)
	if n != 6 || same != 6 {
		t.Errorf("repeatedToolCall counted %d repeats (%d with the same result) in a real Copilot CLI request, want the 6 the client actually made", n, same)
	}
	if !strings.Contains(call, "view(") || !strings.Contains(call, "/work/calc.py") {
		t.Errorf("the repeated call is named %q; a developer reading the refusal learns nothing from that", call)
	}
}
