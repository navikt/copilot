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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"time"
)

// GuardPort is where the guard listens. Fixed rather than ephemeral because the
// client is pointed at it by a configuration file written when the server is
// set up, not by this process — one number in one place beats a port handed
// over at launch through a channel that does not exist yet.
//
// ponytail: one guard per machine. Two concurrent local sessions is a second
// listener on this port and a clear bind failure, not corruption. Give the
// guard an ephemeral port and templated client config if anyone needs two.
const GuardPort = DefaultPort + 1

// DefaultLoopGuardRepeat is how many identical consecutive tool calls end the
// turn. Eight is well past anything a working agent does — a retry is one or
// two, a poll loop that means something varies its arguments — and far short of
// the 203 we measured.
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

// maxRequestBody caps what the guard reads into memory to inspect. A 64k-token
// context is a few hundred kilobytes; this is generous enough that a real
// request never hits it and small enough that a wrong one cannot exhaust
// memory. A body over the cap is forwarded unread rather than refused.
const maxRequestBody = 32 << 20

// ServerURL is where the local model server listens. The address is fixed for
// the same reason [GuardPort] is: it is written into a client configuration
// file by one command and read by another process entirely.
func ServerURL() string { return fmt.Sprintf("http://127.0.0.1:%d", DefaultPort) }

// GuardURL is where the loop guard listens — the address a client is pointed
// at, never the server's. Everything the client sends has to pass the thing
// that can stop a runaway loop.
func GuardURL() string { return fmt.Sprintf("http://127.0.0.1:%d", GuardPort) }

// Guard is the proxy the client talks to instead of the server.
type Guard struct {
	ln  net.Listener
	srv *http.Server
}

// StartGuard puts the guard in front of a local server and returns once it is
// listening. Close it when the session ends.
func StartGuard(target string) (*Guard, error) {
	u, err := url.Parse(target)
	if err != nil {
		return nil, fmt.Errorf("the local server address %q is not a URL: %w", target, err)
	}
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", GuardPort))
	if err != nil {
		return nil, fmt.Errorf(
			"could not listen on 127.0.0.1:%d for the local-inference loop guard: %w\n\n  Another nav-pilot session is probably already running one",
			GuardPort, err)
	}

	proxy := httputil.NewSingleHostReverseProxy(u)
	// Immediate flush: the client watches the gap between streamed tokens and
	// gives up on a long one, and on local hardware at a large context a
	// single token can take minutes. Buffering would turn that budget into a
	// dropped connection mid-generation.
	proxy.FlushInterval = -1

	g := &Guard{ln: ln}
	g.srv = &http.Server{
		Handler:           guardHandler(proxy),
		ReadHeaderTimeout: 30 * time.Second,
		// No write timeout: a completion on a local model legitimately takes
		// longer than any number that would be safe on a network service.
	}
	go g.srv.Serve(ln)
	return g, nil
}

// Close stops the guard. Safe on a nil Guard so a caller can defer it beside a
// failed start.
func (g *Guard) Close() error {
	if g == nil {
		return nil
	}
	if err := g.srv.Close(); err != nil && !errors.Is(err, http.ErrServerClosed) {
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
// The launch proves ownership once and the guard then proxies to a fixed
// 127.0.0.1:8080 for the rest of the session, so a server that dies at noon and
// leaves the port to whatever binds it next has every later prompt forwarded to
// a stranger. Re-proving it per request would be right and costs two
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
// it unchanged.
//
// Everything it cannot read — another path, a body over the cap, a body that is
// not the JSON it expects — is forwarded untouched. The guard exists to stop
// one measured failure, and a guard that fails closed on a shape it did not
// anticipate would break working sessions to protect them.
func guardHandler(proxy http.Handler) http.Handler {
	owned := ownershipGate()
	// One completion at a time. mlx-lm batches concurrent requests into shared
	// attention, and with prompts of different lengths that batching raises
	// [broadcast_shapes] and leaves the server hung — alive, accepting
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
	oneAtATime := make(chan struct{}, 1)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
		if err := owned(); err != nil {
			writeGuardError(w, "nav-pilot did not forward this request: "+err.Error(), "local_server_lost")
			return
		}
		body, err := io.ReadAll(io.LimitReader(r.Body, maxRequestBody))
		if err != nil {
			proxy.ServeHTTP(w, r)
			return
		}
		r.Body.Close()
		r.Body = io.NopCloser(bytes.NewReader(body))
		r.ContentLength = int64(len(body))

		if call, n := repeatedToolCall(body); n >= loopGuardRepeat {
			writeLoopGuardError(w, call, n)
			return
		}
		proxy.ServeHTTP(w, r)
	})
}

// chatRequest is the sliver of the request the guard reads. Every other field
// is left in the bytes that are forwarded.
type chatRequest struct {
	Messages []struct {
		Role      string `json:"role"`
		Content   any    `json:"content"`
		ToolCalls []struct {
			Function struct {
				Name      string `json:"name"`
				Arguments string `json:"arguments"`
			} `json:"function"`
		} `json:"tool_calls"`
	} `json:"messages"`
}

// repeatedToolCall counts the run of identical tool calls the conversation ends
// on, and names it.
//
// It walks backwards from the newest message. Tool results are skipped — they
// are the other half of each call. An assistant message with tool calls extends
// the run when it matches, and ends it when it does not. Anything else ends it
// too: a user message is a new turn, and an assistant message without tool
// calls is the model having said something instead of looping.
//
// The call ids are deliberately not part of the comparison. They differ on
// every call by construction, so including them would compare nothing.
func repeatedToolCall(body []byte) (string, int) {
	var req chatRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return "", 0
	}
	want, n := "", 0
	for i := len(req.Messages) - 1; i >= 0; i-- {
		m := req.Messages[i]
		if m.Role == "tool" {
			continue
		}
		if m.Role != "assistant" || len(m.ToolCalls) == 0 {
			break
		}
		var parts []string
		for _, c := range m.ToolCalls {
			parts = append(parts, c.Function.Name+"("+c.Function.Arguments+")")
		}
		sig := strings.Join(parts, ", ")
		if n == 0 {
			want = sig
		} else if sig != want {
			break
		}
		n++
	}
	return want, n
}

// writeLoopGuardError ends the turn. It answers in the error envelope the
// client already knows how to render, so the developer reads what nav-pilot
// stopped and why rather than a transport failure.
//
// The repeated call is named in full up to a limit: the arguments are what
// distinguishes "reading the same file forever" from "grepping in a circle",
// and a truncated name alone would leave a developer guessing.
func writeLoopGuardError(w http.ResponseWriter, call string, n int) {
	const maxCall = 400
	shown := call
	if len(shown) > maxCall {
		shown = shown[:maxCall] + "…"
	}
	msg := fmt.Sprintf(
		"nav-pilot stopped this turn: the local model made the same tool call %d times in a row without the answer changing — %s. "+
			"That is a runaway loop, not progress; it does not recover on its own. "+
			"Start a new turn with a narrower task, or raise the threshold with `nav-pilot config set local_loop_guard <n>` (current: %d).",
		n, shown, loopGuardRepeat)

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
