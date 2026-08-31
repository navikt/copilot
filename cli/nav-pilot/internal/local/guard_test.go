package local

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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
	g, err := StartGuard(upstream.URL)
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

	g, err := StartGuard(upstream.URL)
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

	g, err := StartGuard(upstream.URL)
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
	call, n := repeatedToolCall(body)
	if n != 6 {
		t.Errorf("repeatedToolCall counted %d repeats in a real Copilot CLI request, want the 6 the client actually made", n)
	}
	if !strings.Contains(call, "view(") || !strings.Contains(call, "/work/calc.py") {
		t.Errorf("the repeated call is named %q; a developer reading the refusal learns nothing from that", call)
	}
}
