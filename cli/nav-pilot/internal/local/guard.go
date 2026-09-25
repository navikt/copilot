package local

// The loop guard.
//
// Local models get stuck. Benchmarking these models we measured runs of 203 and
// 220 identical consecutive tool calls at the 36 GB memory cap, roughly once
// every eleven tasks: the model asks for the same read, gets the same answer,
// and asks again until something outside it intervenes. Nothing inside the
// session stops it. AGENTS.md rule 8 tells the model not to do it and the model
// does it anyway, which is the expected outcome — an instruction is a request,
// and a runaway loop is precisely the state in which requests stop landing.
//
// So the stop lives in nav-pilot, where it can be enforced. Local dispatch is
// the one path where nav-pilot can see tool calls at all: the client talks
// OpenAI chat-completions to a server on this machine, so a proxy in front of
// it reads the conversation. On every hosted path the traffic goes to GitHub
// and nav-pilot sees nothing, which is why this is a local-only guard and not a
// general one.
//
// The check is stateless. Each request carries the whole conversation, so the
// run of identical calls is already in the message list the model is about to
// extend — there is nothing to remember between requests, and nothing to get
// wrong when two sessions share a server.

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

// GuardPort is only what a client configuration written by an older nav-pilot
// points at. The guard now takes an ephemeral port per session, because it used
// to be one listener per machine: a second concurrent session found the port
// taken and the launch failed outright, and two terminal tabs in two
// repositories is ordinary work rather than an edge case.
//
// The port reaches the client through the configuration each launch writes
// anyway, which is why this could move: the Copilot CLI takes it in
// COPILOT_PROVIDER_BASE_URL and opencode in the provider block, both written at
// launch and removed at exit. The channel that did not exist when this was fixed
// turned out to be the one already in use.
const GuardPort = DefaultPort + 1

// DefaultLoopGuardRepeat is how many identical consecutive tool calls end the
// turn whatever they return. It is the backstop, not the main rule: see
// [SameResultRepeat]. Eight is well past anything a working agent does — a
// retry is one or two — and far short of the 203 we measured. A poll whose
// output keeps changing still stops here, because a model that has asked the
// same thing eight times running is waiting rather than working.
const DefaultLoopGuardRepeat = 8

// loopGuardRepeat is the active threshold. nav-pilot sets it from config.
var loopGuardRepeat = DefaultLoopGuardRepeat

// SetLoopGuardRepeat sets how many identical consecutive tool calls end the
// turn. Values below 2 are ignored: one tool call is not a loop, and a
// threshold that trips on the first call would disable local dispatch rather
// than guard it.
func SetLoopGuardRepeat(n int) {
	if n >= 2 {
		loopGuardRepeat = n
	}
}

// LoopGuardRepeat reports the active threshold.
func LoopGuardRepeat() int { return loopGuardRepeat }

// SameResultRepeat is how many identical calls that also got back identical
// results end the turn: half of [LoopGuardRepeat], and never below 2. Four at
// the default.
//
// It is lower than the backstop because an unchanged result is the evidence
// the call alone lacks. A classifier shown only the calls could not tell loops
// from legitimate polls (P(loop) 0.88 on loops, 0.42–0.78 on polls); what
// separates them is whether anything came back different. The same question
// answered the same way four times means the model has had that answer three
// times already and is not using it. Derived from the one knob rather than a
// second one, so a developer who raises local_loop_guard to let a slow poll
// run relaxes both rules together.
func SameResultRepeat() int { return max(2, loopGuardRepeat/2) }

// maxRequestBody caps what the guard reads into memory to inspect. A 64k-token
// context is a few hundred kilobytes; this is generous enough that a real
// request never hits it and small enough that a wrong one cannot exhaust
// memory. A body over the cap is forwarded unread rather than refused.
const maxRequestBody = 32 << 20

// OnLoopGuard is called with the rule (same_result, cycle, backstop) each time
// the guard refuses a request. nav-pilot points it at telemetry; this package
// does not import it.
var OnLoopGuard = func(rule string) {}

// ServerURL is where the local model server listens. The address is fixed for
// the same reason [GuardPort] is: it is written into a client configuration
// file by one command and read by another process entirely.
func ServerURL() string {
	st, ok, err := LoadState()
	if err != nil || !ok {
		// No recorded server. Callers reach this only when they are about to
		// fail an ownership check anyway, and a wrong address fails more
		// clearly than an empty one.
		return fmt.Sprintf("http://127.0.0.1:%d", DefaultPort)
	}
	return fmt.Sprintf("http://127.0.0.1:%d", st.ServerPort())
}

// Guard is the proxy the client talks to instead of the server.
type Guard struct {
	ln  net.Listener
	srv *http.Server

	// conns counts the connection goroutines the server has open, so [Guard.Close]
	// can wait for them instead of merely cutting them loose. A handler that
	// outlives its caller keeps reading package state the caller is already
	// tearing down.
	conns sync.WaitGroup

	// cancelHandlers ends every request in flight. It is what makes the wait in
	// [Guard.Close] bounded, and closing the connections is not a substitute for
	// it: see the note there.
	cancelHandlers context.CancelFunc

	// statsPath is resolved when the guard starts, not per request. A handler
	// runs on its own goroutine and can outlive whatever set the directories up.
	statsPath string

	// requests counts everything the client sent through the guard, completions
	// and model lists alike. Completions alone cannot tell a refusal from a
	// wiring failure; this can.
	requests atomic.Int64

	// completions counts what actually reached the local server through this
	// guard. Counted here rather than parsed out of the client's transcript
	// because this is the only place that sees every one of them, and because a
	// session that dispatched nothing has to be distinguishable from a session
	// whose transcript we failed to parse.
	completions atomic.Int64

	// sampling is the body fields the manifest overrides on every completion,
	// already encoded. Nil when the active model sets none, and then the body
	// is forwarded exactly as the client sent it.
	sampling map[string]json.RawMessage
}

// samplingParams are the manifest params the guard writes into completion
// requests. Local models run greedy today from both clients: mlx-lm defaults
// --temp to 0.0, opencode sends no temperature for a custom model, and the
// Copilot CLI sends "temperature": 0 explicitly. MLX_TEMP becomes --temp, the
// server's default, which a request's own value overrides, so it would reach
// opencode and never the Copilot CLI. The guard sees both clients' requests, so
// it is the one place that can set sampling for both, and it is the only
// mechanism: no server flag is derived from these.
var samplingParams = []struct {
	key, field, want string
	ok               func(float64) bool
}{
	{"MLX_NAV_PILOT_TEMPERATURE", "temperature", "from 0 to 2", func(v float64) bool { return v >= 0 && v <= 2 }},
	{"MLX_NAV_PILOT_TOP_P", "top_p", "above 0 and at most 1", func(v float64) bool { return v > 0 && v <= 1 }},
}

// samplingOverride reads the sampling params a model sets. Nil with no error
// when it sets none. The manifest check calls it too, so an out-of-range value
// refuses the manifest rather than reaching a request.
func samplingOverride(params map[string]string) (map[string]json.RawMessage, error) {
	var out map[string]json.RawMessage
	for _, s := range samplingParams {
		raw := strings.TrimSpace(params[s.key])
		if raw == "" {
			continue
		}
		// NaN and the infinities fail the range check, and re-encoding the parsed
		// number keeps forms ParseFloat accepts but JSON does not ("0x1p-1") out of
		// the body.
		v, err := strconv.ParseFloat(raw, 64)
		if err != nil || !s.ok(v) {
			return nil, fmt.Errorf("param %s is %q, want a number %s", s.key, raw, s.want)
		}
		if out == nil {
			out = map[string]json.RawMessage{}
		}
		out[s.field], _ = json.Marshal(v)
	}
	return out, nil
}

// withSampling replaces the sampling fields in a completion body. A body that
// is not a JSON object is returned unchanged, like everything else the guard
// cannot read.
func withSampling(body []byte, sampling map[string]json.RawMessage) []byte {
	var req map[string]json.RawMessage
	if json.Unmarshal(body, &req) != nil || req == nil {
		return body
	}
	for k, v := range sampling {
		req[k] = v
	}
	out, err := json.Marshal(req)
	if err != nil {
		return body
	}
	return out
}

// isProviderAPIPath reports whether a path is one the provider config would make
// a client ask for. Anything else reaching this port came from something that is
// not the client, and must not count as the client having seen the worker.
func isProviderAPIPath(p string) bool {
	return strings.HasSuffix(p, "/chat/completions") || strings.HasSuffix(p, "/models")
}

// SawTraffic reports whether the client sent the guard anything at all.
//
// False with zero completions means opencode never used the provider block:
// the wiring did not reach it, which is a defect here. True with zero
// completions means it saw the local worker and chose not to dispatch, which is
// the orchestrator's judgement and the thing worth measuring.
//
// Safe on a nil Guard, like Completions, so a hosted session can ask.
func (g *Guard) SawTraffic() bool {
	if g == nil {
		return false
	}
	return g.requests.Load() > 0
}

// Completions is how many prompts this session sent to the local model.
func (g *Guard) Completions() int64 {
	if g == nil {
		return 0
	}
	return g.completions.Load()
}

// URL is where this guard listens: the address a client is pointed at, never the
// server's. Everything the client sends has to pass the thing that can stop a
// runaway loop.
func (g *Guard) URL() string {
	if g == nil || g.ln == nil {
		return ""
	}
	return fmt.Sprintf("http://127.0.0.1:%d", g.ln.Addr().(*net.TCPAddr).Port)
}

// Port is the ephemeral port this guard listens on, or 0 when it is not
// listening. The launch paths need the number rather than the URL: cplt's
// sandbox blocks localhost by default, so the port has to be named to it with
// `--allow-localhost`.
func (g *Guard) Port() int {
	if g == nil || g.ln == nil {
		return 0
	}
	return g.ln.Addr().(*net.TCPAddr).Port
}

// StartGuard puts the guard in front of the local server serving m and returns
// once it is listening. Close it when the session ends: the guard's goroutines outlive the
// call that started them, and only [Guard.Close] waits for them.
func StartGuard(target string, m Model) (*Guard, error) {
	sampling, err := samplingOverride(m.Params)
	if err != nil {
		return nil, fmt.Errorf("local model %s: %w", m.Model, err)
	}
	u, err := url.Parse(target)
	if err != nil {
		return nil, fmt.Errorf("the local server address %q is not a URL: %w", target, err)
	}
	// Port 0: one guard per session, not per machine. Several sessions share the
	// one server behind them, and completions serialise at each guard, so they
	// queue rather than reaching mlx-lm concurrently.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("could not listen for the local-inference loop guard: %w", err)
	}

	proxy := httputil.NewSingleHostReverseProxy(u)
	// Immediate flush: the client watches the gap between streamed tokens and
	// gives up on a long one, and on local hardware at a large context a
	// single token can take minutes. Buffering would turn that budget into a
	// dropped connection mid-generation.
	proxy.FlushInterval = -1

	// The parent of every request context this guard serves. net/http derives a
	// connection context from BaseContext and the request context from that, so
	// cancelling this one ends every handler in flight at whatever it is waiting
	// on, without a second cancellation channel to thread through each of them.
	handlerCtx, cancelHandlers := context.WithCancel(context.Background())

	// statsPath is resolved here and not per request, because the handler runs
	// on its own goroutine and must not read the directory globals while
	// something else is changing them.
	g := &Guard{ln: ln, statsPath: statsPath(), cancelHandlers: cancelHandlers, sampling: sampling}
	g.srv = &http.Server{
		Handler:           guardHandler(g, proxy, target),
		ReadHeaderTimeout: 30 * time.Second,
		// No write timeout: a completion on a local model legitimately takes
		// longer than any number that would be safe on a network service.

		BaseContext: func(net.Listener) context.Context { return handlerCtx },

		// The counter [Guard.Close] waits on. ConnState and not a wrapper around
		// the handler, because the wrapper's Add would race the Wait: a
		// connection accepted just before Close could reach it after the counter
		// had already fallen to zero, and joining a WaitGroup at zero while a
		// Wait is running is the misuse its own documentation forbids.
		//
		// ConnState has no such window, which rests on two things inside
		// net/http rather than on its documented API. Both are worth re-checking
		// on a Go upgrade, and both are one grep away in server.go. Verified on
		// go1.26.7:
		//
		//   - server.go:3463 sets StateNew on the accept loop's own goroutine,
		//     immediately before `go c.serve(connCtx)`, and carries the literal
		//     comment "before Serve can return". So every Add happens before
		//     Serve returns.
		//   - Server.Close calls listenerGroup.Wait() internally (server.go:3111),
		//     so it returns only once Serve has.
		//
		// Close therefore cannot reach the Wait below until every Add is done.
		ConnState: func(_ net.Conn, state http.ConnState) {
			switch state {
			case http.StateNew:
				g.conns.Add(1)
			case http.StateHijacked, http.StateClosed:
				g.conns.Done()
			}
		},
	}
	go func() { _ = g.srv.Serve(ln) }()
	return g, nil
}

// Close stops the guard and returns once its goroutines have finished. Safe on
// a nil Guard so a caller can defer it beside a failed start.
//
// The wait is the point. http.Server.Close returns as soon as the listener and
// the connections are shut, while the goroutines that were serving them are
// still unwinding, and one of those still reads the package-level directory
// overrides through [statePath]. In a test that is a data race against the
// cleanup that restores them; in a session it is a handler outliving the launch
// that owns it. Waiting means the read is over before anything writes, rather
// than merely unreported.
//
// The cancellation first is what keeps that wait short, and closing the
// connections is emphatically not a substitute for it. net/http cancels a
// request context on a lost connection only from the background read of the
// request body, and for a request whose body the handler has not read yet that
// read is deferred until the body hits EOF (server.go registers it through
// registerOnHitEOF). This handler queues on [lockServer] long before it reads a
// body, so a handler parked on the machine-wide lock while another session
// completes would keep polling with a live context, and cutting its socket
// would not tell it otherwise. Close would then block for as long as the other
// session took. Cancelling the base context reaches it where closing the socket
// does not.
//
// Not http.Server.Shutdown: that waits for the request in flight to answer, and
// a completion on a local model runs for minutes. What is waited for here is
// only the unwinding of requests already told to stop.
//
// No timeout on the wait, deliberately. A timeout that fired would let a live
// handler run on past the caller's teardown, which is the race this exists to
// remove, so it would trade a bounded wait for the original bug. The bound
// comes from the cancellation instead. The one step it cannot reach is the
// ownership probe, which takes no context: three subprocess and HTTP probes at
// probeTimeout each, so a little under half a minute in the worst case, and in
// practice zero because Close runs after the client process is gone.
func (g *Guard) Close() error {
	if g == nil {
		return nil
	}
	g.cancelHandlers()
	err := g.srv.Close()
	g.conns.Wait()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// ownershipCheck proves the fixed port still belongs to nav-pilot's own server.
// A var so a test can drive the gate without a recorded server and an lsof that
// agrees with it.
var ownershipCheck = EnsureOwnServer

// ownershipTTL is how long one proof is trusted for.
//
// The launch proves ownership once and the guard then proxies to the port it
// captured at session start for the rest of the session, so a server that dies
// at noon and leaves the port to whatever binds it next has every later prompt
// forwarded to a stranger. (The port is ephemeral now rather than a fixed 8080,
// which narrows who can squat it but does not close the window.) Re-proving it per request would be right and costs two
// subprocesses (ps, lsof); a few seconds of cache makes that free next to a
// completion that takes seconds to minutes on local hardware.
var ownershipTTL = 3 * time.Second

// ownershipGate returns the cached ownership check for one guard. Per guard
// rather than per package so the cache dies with the session it belongs to.
//
// ponytail: the window is the TTL — a server that dies just after a pass is
// proxied to until the entry expires. No check closes it completely, since the
// server can die between the proof and the write; shorten the TTL if seconds
// are too many.
func ownershipGate() func() error {
	var (
		mu      sync.Mutex
		expires time.Time
		err     error
	)
	return func() error {
		mu.Lock()
		defer mu.Unlock()
		if now := time.Now(); !now.Before(expires) {
			err = ownershipCheck()
			expires = now.Add(ownershipTTL)
		}
		return err
	}
}

// guardHandler inspects a completion request and either refuses it or forwards
// it, unchanged unless the manifest sets sampling ([samplingParams]).
//
// Everything it cannot read — another path, a body over the cap, a body that is
// not the JSON it expects — is forwarded untouched. The guard exists to stop
// one measured failure, and a guard that fails closed on a shape it did not
// anticipate would break working sessions to protect them.
func guardHandler(g *Guard, proxy http.Handler, target string) http.Handler {
	owned := ownershipGate()
	// One completion at a time, across every session on this machine. mlx-lm
	// batches concurrent requests into shared attention, and with prompts of
	// different lengths that batching raises [broadcast_shapes] and leaves the
	// server hung — alive, accepting
	// connections, answering nothing, and unrecoverable without a restart. It is
	// an upstream bug (ml-explore/mlx-lm #1139, #1256) and not one we can fix, but
	// it is trivially reachable from here: a cloud orchestrator that fans a
	// refactor out across ten files dispatches ten subagents at once, which is
	// exactly what an orchestrator asked to work file by file decided to do. It
	// wedged the server on the first try and took local inference down for the
	// whole machine.
	//
	// Serialising costs nothing real. One local model on one GPU has no spare
	// capacity for a second stream anyway; the requests were already going to
	// queue, and now they queue somewhere that cannot corrupt a KV cache.
	//
	// The lock is a file rather than a channel because the guard is per session
	// and the server is per machine: two nav-pilot sessions each hold their own
	// guard and would otherwise send one request apiece, which is exactly the
	// concurrency that wedges mlx-lm. A channel would serialise each session
	// against itself and leave them racing each other.
	oneAtATime := make(chan struct{}, 1)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Counted for API-shaped requests only, not for everything that reaches
		// the port. A session that dispatched nothing is three different things,
		// and this is what separates them: a client that asked for the model list
		// and then sent no completion saw the worker and declined, while nothing
		// at all means the wiring never reached it.
		//
		// The filter matters. This listens on localhost, and localhost ports get
		// probed by browsers, security agents and stray curls. One of those would
		// flip this to true and permanently misclassify broken wiring as an
		// orchestrator refusal — the exact confusion the counter exists to end.
		if isProviderAPIPath(r.URL.Path) {
			g.requests.Add(1)
		}
		if r.Method != http.MethodPost || !strings.HasSuffix(r.URL.Path, "/chat/completions") {
			proxy.ServeHTTP(w, r)
			return
		}
		select {
		case oneAtATime <- struct{}{}:
			defer func() { <-oneAtATime }()
		case <-r.Context().Done():
			// The client gave up while queued. Nothing to answer.
			return
		}
		g.completions.Add(1)
		release, err := lockServer(r.Context())
		if err != nil {
			// Queued behind another session and the client gave up, or the lock
			// file is unusable. Forwarding anyway risks the wedge this exists to
			// prevent, so say so rather than gamble.
			writeGuardError(w, "nav-pilot did not forward this request: "+err.Error(), "local_server_busy")
			return
		}
		defer release()
		if err := owned(); err != nil {
			writeGuardError(w, "nav-pilot did not forward this request: "+err.Error(), "local_server_lost")
			return
		}
		// The ownership check proves the *recorded* server. This guard forwards to
		// the address it captured when the session started, and a stop-and-start
		// mid-session records a new port while this keeps sending to the old one:
		// the check would pass for a server nothing here is talking to. Compare
		// them, so the proof covers the address actually in use.
		if target != "" {
			if st, ok, err := LoadState(); err == nil && ok {
				if now := fmt.Sprintf("http://127.0.0.1:%d", st.ServerPort()); now != target {
					writeGuardError(w, "nav-pilot did not forward this request: the local server was restarted on "+
						now+" and this session is bound to "+target+". Start a new session.", "local_server_moved")
					return
				}
			}
		}
		// One byte past the cap, so a body at the limit is detectable rather than
		// silently truncated. Reading exactly maxRequestBody returns no error at
		// the boundary, and forwarding that prefix with a rewritten
		// Content-Length is a corrupted request the server cannot parse.
		body, err := io.ReadAll(io.LimitReader(r.Body, maxRequestBody+1))
		if err != nil || len(body) > maxRequestBody {
			// Too big to inspect for a loop, so forward it unread, which is what
			// the guard promises for anything it cannot parse.
			r.Body = io.NopCloser(io.MultiReader(bytes.NewReader(body), r.Body))
			proxy.ServeHTTP(w, r)
			return
		}
		r.Body.Close()

		call, n, same, cycle, reps := repeatedToolCall(body)
		switch {
		case same >= SameResultRepeat():
			OnLoopGuard("same_result")
			writeLoopGuardError(w, call, same, 1, true)
			return
		// One call with alternating results is a poll: the backstop's case.
		case reps >= SameResultRepeat() && slices.ContainsFunc(cycle, func(c string) bool { return c != cycle[0] }):
			OnLoopGuard("cycle")
			writeLoopGuardError(w, strings.Join(cycle, " → "), reps, len(cycle), true)
			return
		case n >= loopGuardRepeat:
			OnLoopGuard("backstop")
			writeLoopGuardError(w, call, n, 1, false)
			return
		}
		// Only when the manifest sets sampling: otherwise the body goes on
		// byte for byte. A body over the cap was forwarded above without it.
		if g.sampling != nil {
			body = withSampling(body, g.sampling)
		}
		r.Body = io.NopCloser(bytes.NewReader(body))
		r.ContentLength = int64(len(body))
		started := time.Now()
		tap := &usageTap{ResponseWriter: w}
		proxy.ServeHTTP(tap, r)
		if tap.ok() {
			in, out := tap.usage()
			recordCompletionAt(g.statsPath, in, out, time.Since(started).Seconds())
		}
	})
}

// usageTap keeps the last few KB the server sent, so the guard can read the
// usage block out of it once the response is done.
//
// The tail rather than the whole body, because a response here is a refactor's
// worth of generated code and buffering it to count two integers would double
// the memory for nothing. Both response shapes put usage at the end: a plain
// JSON completion has it after choices, and a stream sends it in the final
// data: frame. 8 KB covers both with room to spare.
//
// Writes pass straight through first. Flush is forwarded because the guard sets
// FlushInterval to -1 precisely so tokens reach the client as they are
// generated, and swallowing Flush here would undo that.
type usageTap struct {
	http.ResponseWriter
	tail   []byte
	status int
}

const usageTailBytes = 8 << 10

// WriteHeader records the status so a failed completion is not counted as one.
// An upstream 500 writes a body and no usage block, which would otherwise land
// in the stats as a request that generated nothing — indistinguishable from a
// streaming client that simply did not ask for usage.
func (t *usageTap) WriteHeader(code int) {
	t.status = code
	t.ResponseWriter.WriteHeader(code)
}

func (t *usageTap) ok() bool {
	// Zero means WriteHeader was never called, which net/http treats as 200.
	return t.status == 0 || (t.status >= 200 && t.status < 300)
}

func (t *usageTap) Write(b []byte) (int, error) {
	n, err := t.ResponseWriter.Write(b)
	if n > 0 {
		t.tail = append(t.tail, b[:n]...)
		if len(t.tail) > usageTailBytes {
			t.tail = t.tail[len(t.tail)-usageTailBytes:]
		}
	}
	return n, err
}

func (t *usageTap) Flush() {
	if f, ok := t.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// usage finds the last usage block in the tail. Zero for both when there is
// none, which is a real case rather than an error: a streaming client that did
// not ask for stream_options.include_usage is never sent one.
func (t *usageTap) usage() (int64, int64) {
	i := bytes.LastIndex(t.tail, []byte(`"usage"`))
	if i < 0 {
		return 0, 0
	}
	var found struct {
		PromptTokens     int64 `json:"prompt_tokens"`
		CompletionTokens int64 `json:"completion_tokens"`
	}
	// From the opening brace after the key to the end of the tail: the decoder
	// stops at the end of the object and ignores whatever follows it.
	rest := t.tail[i:]
	if j := bytes.IndexByte(rest, '{'); j >= 0 {
		if json.Unmarshal(objectAt(rest[j:]), &found) == nil {
			return found.PromptTokens, found.CompletionTokens
		}
	}
	return 0, 0
}

// objectAt returns the JSON object beginning at b[0], or nil when the tail was
// cut before it closed.
func objectAt(b []byte) []byte {
	depth := 0
	for i, c := range b {
		switch c {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return b[:i+1]
			}
		}
	}
	return nil
}

// chatRequest is the sliver of the request the guard reads. Every other field
// is left in the bytes that are forwarded.
type chatRequest struct {
	Messages []struct {
		Role       string          `json:"role"`
		Content    json.RawMessage `json:"content"`
		ToolCallID string          `json:"tool_call_id"`
		ToolCalls  []struct {
			ID       string `json:"id"`
			Function struct {
				Name      string `json:"name"`
				Arguments string `json:"arguments"`
			} `json:"function"`
		} `json:"tool_calls"`
	} `json:"messages"`
}

// repeatedToolCall counts the run of identical tool calls the conversation ends
// on, and names it. n is the run of identical calls; same is the part of that
// run whose results were identical too.
//
// It walks backwards from the newest message. Tool results are collected by
// tool_call_id and paired with the assistant message that made the calls. An
// assistant message with tool calls extends the run when its calls match, and
// ends it when they do not. Anything else ends it too: a user message is a new
// turn, and an assistant message without tool calls is the model having said
// something instead of looping. Parallel calls in one message are one step:
// the signature is all of its calls, and its result is all of their results,
// compared as a set so the same calls in another order are the same step.
//
// A result that differs from the newer one ends same but not n: the call did
// something. A message with a call that got no result cannot show a repeat, so
// it ends same too — except the newest, whose results may simply not be in
// this request yet, which is skipped and leaves the pairs before it to decide.
//
// The call ids are deliberately not part of the comparison. They differ on
// every call by construction, so including them would compare nothing.
//
// Results are compared after [NormaliseResult]: output that embeds a timestamp,
// a duration or a counter would otherwise make a stuck loop look like progress,
// and only the backstop would catch it. The Copilot CLI postToolUse hook
// (`nav-pilot hook loop-guard`) applies the same rule with the same
// normalisation to sessions this guard never sees.
//
// cycle and reps are the cycle rule: the latest steps, each a call with its
// result, checked by [RepeatedCycle] for a short cycle of calls that
// keeps getting the same answers. cycle names its calls, oldest first.
func repeatedToolCall(body []byte) (call string, n, same int, cycle []string, reps int) {
	var req chatRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return "", 0, 0, nil, 0
	}
	results := map[string]string{}
	wantResult, sameOpen, runOpen := "", true, true
	// The latest steps, newest first, for the cycle rule: enough for the
	// longest cycle to repeat as often as the backstop allows.
	var keys, sigs []string
	limit := MaxCyclePeriod * loopGuardRepeat
	for i := len(req.Messages) - 1; i >= 0; i-- {
		m := req.Messages[i]
		if m.Role == "tool" {
			results[m.ToolCallID] = NormaliseResult(string(m.Content))
			continue
		}
		if m.Role != "assistant" || len(m.ToolCalls) == 0 {
			break
		}
		// Parallel calls are a set: sorted, each call kept with its own result,
		// so the same calls issued in another order are still the same step.
		pairs := make([][2]string, 0, len(m.ToolCalls))
		paired := true
		for _, c := range m.ToolCalls {
			r, ok := results[c.ID]
			paired = paired && ok && c.ID != ""
			pairs = append(pairs, [2]string{c.Function.Name + "(" + c.Function.Arguments + ")", r})
		}
		clear(results)
		slices.SortFunc(pairs, func(a, b [2]string) int {
			return cmp.Or(cmp.Compare(a[0], b[0]), cmp.Compare(a[1], b[1]))
		})
		parts, outs := make([]string, len(pairs)), make([]string, len(pairs))
		for i, p := range pairs {
			parts[i], outs[i] = p[0], p[1]
		}
		sig, result := strings.Join(parts, ", "), strings.Join(outs, "\x00")
		newest := n == 0
		if n == 0 {
			call = sig
		} else if sig != call {
			runOpen = false
		}
		// A step without its result cannot repeat another. The newest may only
		// be waiting for it and is left out; an older one breaks any cycle.
		switch {
		case len(keys) >= limit:
		case paired:
			keys, sigs = append(keys, sig+"\x01"+result), append(sigs, sig)
		case !newest:
			keys, sigs = append(keys, "\x02"+strconv.Itoa(i)), append(sigs, sig)
		}
		if !runOpen {
			if len(keys) >= limit {
				break
			}
			continue
		}
		n++
		switch {
		case !sameOpen:
		case !paired && newest:
		case !paired:
			sameOpen = false
		case same == 0:
			wantResult, same = result, 1
		case result == wantResult:
			same++
		default:
			sameOpen = false
		}
	}
	slices.Reverse(keys)
	period, reps := RepeatedCycle(keys)
	if reps > 0 {
		cycle = slices.Clone(sigs[:period])
		slices.Reverse(cycle)
	}
	return call, n, same, cycle, reps
}

// writeLoopGuardError ends the turn. It answers in the error envelope the
// client already knows how to render, so the developer reads what nav-pilot
// stopped and why rather than a transport failure.
//
// The repeated call is named in full up to a limit: the arguments are what
// distinguishes "reading the same file forever" from "grepping in a circle",
// and a truncated name alone would leave a developer guessing.
//
// sameResult says which rule tripped. The model reads this too, so it says
// what was repeated and what to do instead. Only the same-result rule may call
// it a loop that will not change the answer: the backstop also stops polls
// whose output was changing, and telling a model its progress was not progress
// would be false.
//
// A period above 1 is the cycle rule: call is the calls of the cycle and n
// how many times the cycle repeated.
func writeLoopGuardError(w http.ResponseWriter, call string, n, period int, sameResult bool) {
	const maxCall = 400
	shown := call
	if len(shown) > maxCall {
		shown = shown[:maxCall] + "…"
	}
	msg := fmt.Sprintf(
		"nav-pilot stopped this turn: the local model made the same tool call %d times in a row, even though the results changed — %s. "+
			"If it is waiting on something slow, wait longer between calls or try another approach. "+
			"Start a new turn, or raise the threshold with `nav-pilot config set local_loop_guard <n>` (current: %d).",
		n, shown, loopGuardRepeat)
	if sameResult {
		msg = fmt.Sprintf(
			"nav-pilot stopped this turn: the local model repeated the same tool call with the same result %d times — %s. "+
				"That is a runaway loop, not progress; repeating it will not change the answer, so try something else. "+
				"Start a new turn with a narrower task, or raise the threshold with `nav-pilot config set local_loop_guard <n>` (current: %d).",
			n, shown, loopGuardRepeat)
	}
	if period > 1 {
		msg = fmt.Sprintf(
			"nav-pilot stopped this turn: the local model is repeating a cycle of %d tool calls and got the same results %d times — %s. "+
				"That is a runaway loop, not progress; repeating it will not change the answer, so try something else. "+
				"Start a new turn with a narrower task, or raise the threshold with `nav-pilot config set local_loop_guard <n>` (current: %d).",
			period, n, shown, loopGuardRepeat)
	}

	writeGuardError(w, msg, "loop_guard")
}

// writeGuardError is the envelope both refusals answer in — the shape the
// client already renders as a message rather than as a transport failure.
func writeGuardError(w http.ResponseWriter, msg, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]any{
			"message": msg,
			"type":    "nav_pilot_" + code,
			"code":    code,
		},
	})
}

// lockServer takes the machine-wide lock on the local server, waiting until it is
// free or the request is abandoned. The returned function releases it.
//
// flock on a file beside the recorded server, so the lock is held by the kernel
// against the open descriptor: a session that crashes releases it, which a lock
// written into a file's contents would not. Sessions are few and completions are
// long, so polling once a second costs nothing measurable and avoids a blocking
// flock that no context can interrupt.
func lockServer(ctx context.Context) (func(), error) {
	dir := filepath.Dir(statePath())
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("creating the local server directory: %w", err)
	}
	f, err := os.OpenFile(filepath.Join(dir, "server.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("opening the local server lock: %w", err)
	}
	for {
		if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err == nil {
			return func() {
				_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
				_ = f.Close()
			}, nil
		}
		select {
		case <-ctx.Done():
			_ = f.Close()
			return nil, fmt.Errorf("gave up waiting for the local server: %w", ctx.Err())
		case <-time.After(time.Second):
		}
	}
}
