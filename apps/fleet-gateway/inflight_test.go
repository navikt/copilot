package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httptrace"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Over the cap: 503 with Retry-After, an audit line and outcome over_capacity.
func TestInFlightCapRejectsWithAudit(t *testing.T) {
	var logBuf bytes.Buffer
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelInfo})))
	t.Cleanup(func() { slog.SetDefault(old) })

	te := newTestEnv(t)
	te.gw.cfg.MCPMaxInFlight = 1
	te.handler = te.gw.routes()
	release := make(chan struct{})
	entered := make(chan struct{}, 1)
	te.upstreamServer.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		entered <- struct{}{}
		<-release
		writeJSON(w, http.StatusOK, map[string]any{"jsonrpc": "2.0", "id": 1, "result": map[string]any{}})
	})
	tokenA := te.obtainAccessTokenForResource(te.resource)
	te.mu.Lock()
	te.navIdent = "B654321"
	te.mu.Unlock()
	tokenB := te.obtainAccessTokenForResource(te.resource)
	ts := httptest.NewServer(te.handler)
	defer ts.Close()

	first := make(chan int, 1)
	go func() { first <- capPost(t, ts, tokenA, "") }()
	<-entered
	shed := httpRequestsTotal.WithLabelValues("503", "cap")
	beforeShed := testutil.ToFloat64(shed)
	if code := capPost(t, ts, tokenB, ""); code != http.StatusServiceUnavailable {
		t.Fatalf("over cap: code %d, want 503", code)
	}
	// Labelled so FleetGatewayHigh5xxRate does not count the cap as a failure.
	if got := testutil.ToFloat64(shed); got != beforeShed+1 {
		t.Errorf("over-cap 503 not labelled reason=cap: %v -> %v", beforeShed, got)
	}
	close(release)
	if code := <-first; code != http.StatusOK {
		t.Fatalf("in-cap request: code %d", code)
	}
	if !strings.Contains(logBuf.String(), `"msg":"audit mcp"`) || !strings.Contains(logBuf.String(), `"outcome":"over_capacity"`) {
		t.Fatalf("no audit mcp line with outcome over_capacity:\n%s", logBuf.String())
	}
	if code := capPost(t, ts, tokenA, ""); code != http.StatusOK {
		t.Fatalf("after release: code %d, want 200 (slot not freed)", code)
	}
}

// SSE streams leave the in-flight cap once streaming starts, so open streams
// beyond MCP_MAX_IN_FLIGHT do not block tool calls; MCP_MAX_SSE bounds them.
func TestSSEStreamsDoNotStarveToolCalls(t *testing.T) {
	te := newTestEnv(t)
	te.gw.cfg.MCPMaxInFlight = 2
	te.gw.cfg.MCPMaxSSE = 16
	te.gw.cfg.SSEMaxDuration = 10 * time.Second
	te.gw.cfg.SSEIdleTimeout = 10 * time.Second
	te.handler = te.gw.routes()
	stop := make(chan struct{})
	te.upstreamServer.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("sse") == "" {
			writeJSON(w, http.StatusOK, map[string]any{"jsonrpc": "2.0", "id": 1, "result": map[string]any{}})
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		select {
		case <-stop:
		case <-r.Context().Done():
		}
	})
	token := te.obtainAccessTokenForResource(te.resource)
	ts := httptest.NewServer(te.handler)
	defer ts.Close()
	defer close(stop)

	for i := range 16 {
		req, _ := http.NewRequest(http.MethodPost, ts.URL+te.gw.cfg.Routes[0].Route+"?sse=1", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"x"}}`))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
		resp, err := ts.Client().Do(req) //nolint:gosec // test server URL
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("SSE stream %d: code %d", i, resp.StatusCode)
		}
	}
	if code := capPost(t, ts, token, "?sse=1"); code != http.StatusServiceUnavailable {
		t.Fatalf("17th SSE stream: code %d, want 503 (MCP_MAX_SSE)", code)
	}
	for i := range 3 {
		if code := capPost(t, ts, token, ""); code != http.StatusOK {
			t.Fatalf("tool call %d with 16 open streams: code %d, want 200", i, code)
		}
	}
}

func capPost(t *testing.T, ts *httptest.Server, token, query string) int {
	t.Helper()
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/mcp/team-etterlevelse/etterlevelse"+query, strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	if strings.Contains(query, "sse") {
		req.Header.Set("Accept", "application/json, text/event-stream")
	}
	resp, err := ts.Client().Do(req) //nolint:gosec // test server URL
	if err != nil {
		t.Error(err)
		return 0
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, resp.Body)
	return resp.StatusCode
}

// readChunks must not reallocate: io.ReadAll on an unknown-length 10 MiB body
// allocates roughly 2-3x that while growing.
func TestReadChunksAllocatesBodySize(t *testing.T) {
	const size = 10 << 20
	var ms0, ms1 runtime.MemStats
	runtime.ReadMemStats(&ms0)
	chunks, n, err := readChunks(io.LimitReader(zeroReader{}, size), size+1)
	runtime.ReadMemStats(&ms1)
	if err != nil || n != size || len(chunks) == 0 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	if alloc := ms1.TotalAlloc - ms0.TotalAlloc; alloc > size*11/10 {
		t.Fatalf("allocated %d bytes for a %d-byte body, want <= 1.1x", alloc, size)
	}
}

type zeroReader struct{}

func (zeroReader) Read(p []byte) (int, error) { clear(p); return len(p), nil }

// A real server enforces WriteTimeout (httptest.ResponseRecorder does not).
// A call that uses most of its token and upstream budgets must still finish.
func TestWriteTimeoutCoversTokenAndUpstream(t *testing.T) {
	const upstream = 300 * time.Millisecond
	const token = 300 * time.Millisecond
	te := newTestEnv(t)
	te.gw.cfg.ProxyHeaderTimeout = upstream
	te.handler = te.gw.routes()
	gwToken := te.obtainAccessTokenForResource(te.resource)
	texas := te.texasServer.Config.Handler
	te.texasServer.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(token * 8 / 10)
		texas.ServeHTTP(w, r)
	})
	te.upstreamServer.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(upstream * 8 / 10)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		time.Sleep(upstream * 8 / 10)
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{}}`))
	})
	ts := httptest.NewUnstartedServer(te.handler)
	ts.Config.WriteTimeout = mcpWriteTimeout(50*time.Millisecond, token, upstream, 50*time.Millisecond)
	ts.Start()
	defer ts.Close()

	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/mcp/team-etterlevelse/etterlevelse", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
	req.Header.Set("Authorization", "Bearer "+gwToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := ts.Client().Do(req) //nolint:gosec // test server URL
	if err != nil {
		t.Fatalf("cut off by WriteTimeout %v: %v", ts.Config.WriteTimeout, err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil || resp.StatusCode != http.StatusOK || !strings.Contains(string(body), `"result"`) {
		t.Fatalf("code %d body %q err %v, want full 200 within WriteTimeout %v", resp.StatusCode, body, err, ts.Config.WriteTimeout)
	}
}

func TestUpstreamTimeoutConfig(t *testing.T) {
	setBaseValidEnv(t)
	t.Setenv("UPSTREAM_TIMEOUT", "")
	t.Setenv("MCP_MAX_IN_FLIGHT", "")
	t.Setenv("MCP_MAX_SSE", "")
	cfg, err := loadConfigFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ProxyHeaderTimeout != 60*time.Second || cfg.MCPMaxInFlight != 16 || cfg.MCPMaxSSE != 64 {
		t.Fatalf("defaults: timeout %v, in-flight %d; want 60s, 16", cfg.ProxyHeaderTimeout, cfg.MCPMaxInFlight)
	}

	t.Setenv("UPSTREAM_TIMEOUT", "90s")
	t.Setenv("MCP_MAX_IN_FLIGHT", "4")
	cfg, err = loadConfigFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ProxyHeaderTimeout != 90*time.Second || cfg.MCPMaxInFlight != 4 {
		t.Fatalf("from env: timeout %v, in-flight %d; want 90s, 4", cfg.ProxyHeaderTimeout, cfg.MCPMaxInFlight)
	}
}

// Recomputes the README's memory budget from the code's limits and checks that
// it fits under GOMEMLIMIT, which must stay at or below 90 % of the pod limit.
func TestMemoryBudgetFitsManifests(t *testing.T) {
	const mib = 1 << 20
	perCall := float64(maxMCPResponseBodyBytes+32<<10) + 4*float64(maxMCPRequestBodyBytes) + 0.1*mib
	budget := defaultMCPMaxInFlight*perCall + defaultMCPMaxSSE*48<<10 + 30*mib
	for _, f := range []string{".nais/dev-gcp.yaml", ".nais/prod-gcp.yaml"} {
		raw, err := os.ReadFile(f) //nolint:gosec // fixed manifest paths
		if err != nil {
			t.Fatal(err)
		}
		vars := map[string]float64{}
		for _, line := range strings.Split(string(raw), "\n") {
			k, v, ok := strings.Cut(line, ": ")
			if !ok || (k != "memory_limit" && k != "gomemlimit") {
				continue
			}
			v = strings.TrimSuffix(strings.TrimSuffix(strings.Trim(v, `"`), "iB"), "i")
			n, err := strconv.ParseFloat(strings.TrimSuffix(v, "M"), 64)
			if err != nil || !strings.HasSuffix(v, "M") {
				t.Fatalf("%s: %s=%q, want a value in Mi/MiB", f, k, v)
			}
			vars[k] = n * mib
		}
		if vars["gomemlimit"] < budget || vars["gomemlimit"] > 0.9*vars["memory_limit"] {
			t.Fatalf("%s: gomemlimit %.0f MiB, memory_limit %.0f MiB; want budget %.0f MiB <= gomemlimit <= 90 %% of limit",
				f, vars["gomemlimit"]/mib, vars["memory_limit"]/mib, budget/mib)
		}
	}
}

// A body that stops short of its Content-Length is a failed upstream read,
// not a complete response: 502 stream_error, never a truncated 200.
func TestTruncatedUpstreamBodyIs502(t *testing.T) {
	var logBuf bytes.Buffer
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelInfo})))
	t.Cleanup(func() { slog.SetDefault(old) })

	te := newTestEnv(t)
	te.upstreamServer.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		conn, buf, err := w.(http.Hijacker).Hijack()
		if err != nil {
			t.Error(err)
			return
		}
		defer func() { _ = conn.Close() }()
		_, _ = buf.WriteString("HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: 1000\r\n\r\n" + strings.Repeat("x", 100))
		_ = buf.Flush()
	})
	token := te.obtainAccessTokenForResource(te.resource)
	ts := httptest.NewServer(te.handler)
	defer ts.Close()
	if code := capPost(t, ts, token, ""); code != http.StatusBadGateway {
		t.Fatalf("truncated body: code %d, want 502", code)
	}
	if !strings.Contains(logBuf.String(), `"outcome":"stream_error"`) {
		t.Fatalf("no audit line with outcome stream_error:\n%s", logBuf.String())
	}
}

// The SSE cap is checked before the upstream call: a 503 must mean the tool
// never ran, and no session was bound.
func TestSSECapCheckedBeforeUpstream(t *testing.T) {
	te := newTestEnv(t)
	te.gw.cfg.MCPMaxSSE = 1
	te.gw.cfg.SSEMaxDuration = 10 * time.Second
	te.gw.cfg.SSEIdleTimeout = 10 * time.Second
	te.handler = te.gw.routes()
	var hits atomic.Int32
	stop := make(chan struct{})
	te.upstreamServer.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("Mcp-Session-Id", "s-"+strconv.Itoa(int(hits.Load())))
		if r.URL.Query().Get("sse") == "" {
			writeJSON(w, http.StatusOK, map[string]any{"jsonrpc": "2.0", "id": 1, "result": map[string]any{}})
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		select {
		case <-stop:
		case <-r.Context().Done():
		}
	})
	token := te.obtainAccessTokenForResource(te.resource)
	ts := httptest.NewServer(te.handler)
	defer ts.Close()
	defer close(stop)

	req, _ := http.NewRequest(http.MethodPost, ts.URL+te.gw.cfg.Routes[0].Route+"?sse=1", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json, text/event-stream")
	resp, err := ts.Client().Do(req) //nolint:gosec // test server URL
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("first stream: code %d", resp.StatusCode)
	}
	before := hits.Load()
	shed := httpRequestsTotal.WithLabelValues("503", "cap")
	beforeShed := testutil.ToFloat64(shed)
	if code := capPost(t, ts, token, "?sse=1"); code != http.StatusServiceUnavailable {
		t.Fatalf("over SSE cap: code %d, want 503", code)
	}
	if got := testutil.ToFloat64(shed); got != beforeShed+1 {
		t.Errorf("over-cap 503 not labelled reason=cap: %v -> %v", beforeShed, got)
	}
	if hits.Load() != before {
		t.Fatalf("upstream called %d times for a request refused at the SSE cap", hits.Load()-before)
	}
	if _, found, _ := te.gw.store.Get(t.Context(), kindMCPSession, te.gw.cfg.Routes[0].Route+"\ns-2"); found {
		t.Fatal("session bound for a request refused at the SSE cap")
	}
}

// The in-flight slot is taken after authentication, so requests with bogus
// bearers get 401 even when every slot is held.
func TestBogusBearerDoesNotTakeSlot(t *testing.T) {
	te := newTestEnv(t)
	te.gw.cfg.MCPMaxInFlight = 1
	te.handler = te.gw.routes()
	release := make(chan struct{})
	entered := make(chan struct{}, 1)
	te.upstreamServer.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		entered <- struct{}{}
		<-release
		writeJSON(w, http.StatusOK, map[string]any{"jsonrpc": "2.0", "id": 1, "result": map[string]any{}})
	})
	token := te.obtainAccessTokenForResource(te.resource)
	ts := httptest.NewServer(te.handler)
	defer ts.Close()
	defer close(release) // before ts.Close, so a failed assertion cannot hang it
	first := make(chan int, 1)
	go func() { first <- capPost(t, ts, token, "") }()
	<-entered
	if code := capPost(t, ts, "bogus", ""); code != http.StatusUnauthorized {
		t.Fatalf("bogus bearer with all slots held: code %d, want 401", code)
	}
}

// One route cannot hold every global slot: MCP_MAX_IN_FLIGHT_PER_ROUTE caps it
// below MCP_MAX_IN_FLIGHT.
func TestPerRouteInFlightCap(t *testing.T) {
	te := newTestEnv(t)
	te.gw.cfg.MCPMaxInFlight = 4
	te.gw.cfg.MCPMaxInFlightPerRoute = 1
	te.handler = te.gw.routes()
	release := make(chan struct{})
	entered := make(chan struct{}, 1)
	te.upstreamServer.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		entered <- struct{}{}
		<-release
		writeJSON(w, http.StatusOK, map[string]any{"jsonrpc": "2.0", "id": 1, "result": map[string]any{}})
	})
	token := te.obtainAccessTokenForResource(te.resource)
	ts := httptest.NewServer(te.handler)
	defer ts.Close()
	defer close(release) // before ts.Close, so a failed assertion cannot hang it
	first := make(chan int, 1)
	go func() { first <- capPost(t, ts, token, "") }()
	<-entered
	shed := httpRequestsTotal.WithLabelValues("503", "cap")
	beforeShed := testutil.ToFloat64(shed)
	if code := capPost(t, ts, token, ""); code != http.StatusServiceUnavailable {
		t.Fatalf("second request on a route at its cap: code %d, want 503", code)
	}
	if got := testutil.ToFloat64(shed); got != beforeShed+1 {
		t.Errorf("over-cap 503 not labelled reason=cap: %v -> %v", beforeShed, got)
	}
	if free := cap(te.gw.inFlight) - len(te.gw.inFlight); free != 3 {
		t.Fatalf("global slots free = %d, want 3 (route cap must refuse before taking a global slot)", free)
	}
}

func largeJSONRPCPayload() []byte {
	const prefix = `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"x","arguments":{"data":"`
	const suffix = `"}}}`
	return []byte(prefix + strings.Repeat("a", maxMCPRequestBodyBytes-len(prefix)-len(suffix)-64) + suffix)
}

func TestSSEEarlyFlushUpstreamGetsFullBody(t *testing.T) {
	te := newTestEnv(t)
	te.gw.cfg.MCPMaxSSE = 16
	te.gw.cfg.SSEMaxDuration = 10 * time.Second
	te.gw.cfg.SSEIdleTimeout = 10 * time.Second
	te.handler = te.gw.routes()

	payload := largeJSONRPCPayload()
	var gotLen atomic.Int64
	var readErr atomic.Value
	te.upstreamServer.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("sse") != "1" {
			writeJSON(w, http.StatusOK, map[string]any{"jsonrpc": "2.0", "id": 1, "result": map[string]any{}})
			return
		}
		if err := http.NewResponseController(w).EnableFullDuplex(); err != nil {
			readErr.Store(err.Error())
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()

		body, err := io.ReadAll(r.Body)
		if err != nil {
			readErr.Store(err.Error())
			return
		}
		gotLen.Store(int64(len(body)))
		_, _ = io.WriteString(w, "data: "+strconv.Itoa(len(body))+"\n\n")
		w.(http.Flusher).Flush()
	})

	token := te.obtainAccessTokenForResource(te.resource)
	ts := httptest.NewServer(te.handler)
	defer ts.Close()

	req, err := http.NewRequest(http.MethodPost, ts.URL+te.gw.cfg.Routes[0].Route+"?sse=1", bytes.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	resp, err := ts.Client().Do(req) //nolint:gosec // test server URL
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("SSE early flush: code %d, want 200", resp.StatusCode)
	}
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading SSE response: %v", err)
	}
	if errMsg, _ := readErr.Load().(string); errMsg != "" {
		t.Fatalf("upstream read error: %s", errMsg)
	}
	if got := gotLen.Load(); got != int64(len(payload)) {
		t.Fatalf("upstream received %d bytes, want %d", got, len(payload))
	}
	wantEvent := "data: " + strconv.Itoa(len(payload)) + "\n\n"
	if !strings.Contains(string(respBody), wantEvent) {
		t.Fatalf("client SSE response = %q, want %q", string(respBody), wantEvent)
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestSSEEarlyFlushRetryUpstreamGetsFullBody(t *testing.T) {
	te := newTestEnv(t)
	te.gw.cfg.MCPMaxSSE = 16
	te.gw.cfg.SSEMaxDuration = 10 * time.Second
	te.gw.cfg.SSEIdleTimeout = 10 * time.Second
	te.handler = te.gw.routes()

	base := te.gw.proxyClient.Transport
	var attempts atomic.Int32
	te.gw.proxyClient.Transport = roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		if attempts.Add(1) == 1 {
			var head [64]byte
			if _, err := io.ReadFull(req.Body, head[:]); err != nil {
				return nil, err
			}
			_ = req.Body.Close()
			if trace := httptrace.ContextClientTrace(req.Context()); trace != nil && trace.WroteRequest != nil {
				trace.WroteRequest(httptrace.WroteRequestInfo{Err: errors.New("simulated connection reset")})
			}
			if req.GetBody == nil {
				return nil, errors.New("expected non-nil req.GetBody on retry")
			}
			retryBody, err := req.GetBody()
			if err != nil {
				return nil, err
			}
			retryReq := req.Clone(req.Context())
			retryReq.Body = retryBody
			retryReq.GetBody = req.GetBody
			return base.RoundTrip(retryReq)
		}
		return base.RoundTrip(req)
	})

	payload := largeJSONRPCPayload()
	var gotLen atomic.Int64
	var readErr atomic.Value
	clientGotHeaders := make(chan struct{})
	upstreamDone := make(chan struct{})
	te.upstreamServer.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer close(upstreamDone)
		if err := http.NewResponseController(w).EnableFullDuplex(); err != nil {
			readErr.Store(err.Error())
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()

		select {
		case <-clientGotHeaders:
		case <-t.Context().Done():
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			readErr.Store(err.Error())
			return
		}
		gotLen.Store(int64(len(body)))
		_, _ = io.WriteString(w, "data: "+strconv.Itoa(len(body))+"\n\n")
		w.(http.Flusher).Flush()
	})

	token := te.obtainAccessTokenForResource(te.resource)
	ts := httptest.NewServer(te.handler)
	defer ts.Close()

	req, err := http.NewRequest(http.MethodPost, ts.URL+te.gw.cfg.Routes[0].Route+"?sse=1", bytes.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	resp, err := ts.Client().Do(req) //nolint:gosec // test server URL
	close(clientGotHeaders)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("SSE early flush retry: code %d, want 200", resp.StatusCode)
	}
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading SSE response: %v", err)
	}
	<-upstreamDone
	if attempts.Load() != 1 {
		t.Fatalf("proxy attempts = %d, want 1 outer call that retried via GetBody", attempts.Load())
	}
	if errMsg, _ := readErr.Load().(string); errMsg != "" {
		t.Fatalf("upstream read error: %s", errMsg)
	}
	if got := gotLen.Load(); got != int64(len(payload)) {
		t.Fatalf("upstream received %d bytes, want %d", got, len(payload))
	}
	wantEvent := "data: " + strconv.Itoa(len(payload)) + "\n\n"
	if !strings.Contains(string(respBody), wantEvent) {
		t.Fatalf("client SSE response = %q, want %q", string(respBody), wantEvent)
	}
}

func TestSSEStreamsDoNotRetainRequestBody(t *testing.T) {
	te := newTestEnv(t)
	te.gw.cfg.MCPMaxSSE = 64
	te.gw.cfg.SSEMaxDuration = 10 * time.Second
	te.gw.cfg.SSEIdleTimeout = 10 * time.Second
	te.handler = te.gw.routes()

	stop := make(chan struct{})
	te.upstreamServer.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		select {
		case <-stop:
		case <-r.Context().Done():
		}
	})
	token := te.obtainAccessTokenForResource(te.resource)
	ts := httptest.NewServer(te.handler)
	defer ts.Close()
	defer close(stop)

	payload := largeJSONRPCPayload()

	runtime.GC()
	runtime.GC()
	var base runtime.MemStats
	runtime.ReadMemStats(&base)

	for i := range 64 {
		req, err := http.NewRequest(http.MethodPost, ts.URL+te.gw.cfg.Routes[0].Route, bytes.NewReader(payload))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
		resp, err := ts.Client().Do(req) //nolint:gosec // test server URL
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("SSE stream %d: code %d", i, resp.StatusCode)
		}
	}

	runtime.GC()
	runtime.GC()
	var after runtime.MemStats
	runtime.ReadMemStats(&after)

	const mib = 1 << 20
	var growth uint64
	if after.HeapAlloc > base.HeapAlloc {
		growth = after.HeapAlloc - base.HeapAlloc
	}
	t.Logf("heap growth across 64 open SSE streams: %.2f MiB (baseline %.2f MiB, after %.2f MiB)",
		float64(growth)/mib, float64(base.HeapAlloc)/mib, float64(after.HeapAlloc)/mib)
	if growth > 24*mib {
		t.Fatalf("heap growth %.2f MiB exceeds 24 MiB", float64(growth)/mib)
	}
}

// A request shed at the SSE cap was allowed by no check: the audit says
// decision false with the outcome as reason, and the metric has no allow.
func TestSSECapShedIsNotAuditedAsAllow(t *testing.T) {
	var logBuf syncBuf
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelInfo})))
	t.Cleanup(func() { slog.SetDefault(old) })

	te := newTestEnv(t)
	te.gw.cfg.MCPMaxSSE = 1
	te.gw.cfg.SSEMaxDuration = 10 * time.Second
	te.gw.cfg.SSEIdleTimeout = 10 * time.Second
	te.handler = te.gw.routes()
	stop := make(chan struct{})
	te.upstreamServer.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		select {
		case <-stop:
		case <-r.Context().Done():
		}
	})
	token := te.obtainAccessTokenForResource(te.resource)
	ts := httptest.NewServer(te.handler)
	defer ts.Close()
	defer close(stop)

	route := te.gw.cfg.Routes[0].Route
	req, _ := http.NewRequest(http.MethodPost, ts.URL+route, strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	resp, err := ts.Client().Do(req) //nolint:gosec // test server URL
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()

	allow := mcpRequestsTotal.WithLabelValues(route, "sse_over_capacity", "allow")
	before := testutil.ToFloat64(allow)
	if code := capPost(t, ts, token, "?sse=1"); code != http.StatusServiceUnavailable {
		t.Fatalf("second SSE stream: code %d, want 503", code)
	}
	if got := testutil.ToFloat64(allow); got != before {
		t.Errorf("shed counted as decision=allow: %v -> %v", before, got)
	}
	var found bool
	for _, line := range strings.Split(logBuf.String(), "\n") {
		if !strings.Contains(line, `"outcome":"sse_over_capacity"`) {
			continue
		}
		found = true
		if !strings.Contains(line, `"decision":{"decision":false,"context":{"reason":"sse_over_capacity"}}`) {
			t.Errorf("audit line not decision=false reason=sse_over_capacity:\n%s", line)
		}
	}
	if !found {
		t.Fatalf("no audit line with outcome sse_over_capacity:\n%s", logBuf.String())
	}
}

// A downstream client that stops reading must not pin the SSE handler and its
// MCP_MAX_SSE slot past SSE_MAX_DURATION / SSE_IDLE_TIMEOUT.
func TestSSEBlockedDownstreamReleasesSlotOnCancel(t *testing.T) {
	te := newTestEnv(t)
	te.gw.cfg.MCPMaxSSE = 1
	te.gw.cfg.SSEMaxDuration = 300 * time.Millisecond
	te.gw.cfg.SSEIdleTimeout = 200 * time.Millisecond
	te.gw.cfg.SSEMaxBytes = 64 << 20
	te.handler = te.gw.routes()

	stop := make(chan struct{})
	chunk := []byte("data: " + strings.Repeat("x", 64*1024) + "\n\n")
	te.upstreamServer.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		if r.URL.Query().Get("probe") == "1" {
			_, _ = io.WriteString(w, "data: ok\n\n")
			return
		}
		flusher, _ := w.(http.Flusher)
		if flusher != nil {
			flusher.Flush()
		}
		for {
			select {
			case <-stop:
				return
			case <-r.Context().Done():
				return
			default:
			}
			if _, err := w.Write(chunk); err != nil {
				return
			}
			if flusher != nil {
				flusher.Flush()
			}
		}
	})

	token := te.obtainAccessTokenForResource(te.resource)
	route := te.gw.cfg.Routes[0].Route
	firstDone := make(chan struct{})
	var firstOnce sync.Once
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == route && r.URL.Query().Get("probe") == "" {
			defer firstOnce.Do(func() { close(firstDone) })
		}
		te.handler.ServeHTTP(w, r)
	}))
	defer ts.Close()
	defer close(stop)

	conn, err := net.Dial("tcp", ts.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()

	reqBody := `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`
	reqStr := fmt.Sprintf(
		"POST %s HTTP/1.1\r\nHost: %s\r\nAuthorization: Bearer %s\r\nContent-Type: application/json\r\nAccept: application/json, text/event-stream\r\nContent-Length: %d\r\n\r\n%s",
		route, ts.Listener.Addr().String(), token, len(reqBody), reqBody,
	)
	if _, err = io.WriteString(conn, reqStr); err != nil {
		t.Fatal(err)
	}

	statusLine, err := bufio.NewReaderSize(conn, 512).ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(statusLine, "200 OK") {
		t.Fatalf("first SSE stream status line = %q, want 200 OK", statusLine)
	}

	// Never drain conn further so TCP buffers fill and streamSSE's w.Write blocks.
	select {
	case <-firstDone:
	case <-time.After(5 * time.Second):
		t.Fatalf("SSE handler stayed blocked past timeout (sseSlots=%d/%d)",
			len(te.gw.sseStreams), cap(te.gw.sseStreams))
	}

	if used := len(te.gw.sseStreams); used != 0 {
		t.Fatalf("SSE slot still held after handler returned: %d", used)
	}
	if code := capPost(t, ts, token, "?sse=1&probe=1"); code != http.StatusOK {
		t.Fatalf("follow-up SSE request with MCPMaxSSE=1: code %d, want 200", code)
	}
}

// Cancelling an SSE stream must not leave a stale write deadline that cuts a
// subsequent request on a keep-alive connection.
func TestSSECancelDoesNotCutFollowUpKeepAlive(t *testing.T) {
	te := newTestEnv(t)
	te.gw.cfg.SSEMaxDuration = 30 * time.Millisecond
	te.gw.cfg.SSEIdleTimeout = 30 * time.Millisecond
	te.handler = te.gw.routes()

	te.upstreamServer.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		if r.URL.Query().Get("sse") == "1" {
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
			_, _ = io.WriteString(w, "data: hi\n\n")
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
			<-r.Context().Done()
			return
		}
		time.Sleep(200 * time.Millisecond)
		writeJSON(w, http.StatusOK, map[string]any{"jsonrpc": "2.0", "id": 1, "result": map[string]any{"ok": true}})
	})

	token := te.obtainAccessTokenForResource(te.resource)
	route := te.gw.cfg.Routes[0].Route
	ts := httptest.NewServer(te.handler)
	defer ts.Close()

	client := &http.Client{
		Transport: &http.Transport{
			MaxConnsPerHost:     1,
			MaxIdleConnsPerHost: 1,
		},
	}
	defer client.CloseIdleConnections()

	for i := range 20 {
		sseReq, _ := http.NewRequest(http.MethodPost, ts.URL+route+"?sse=1", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
		sseReq.Header.Set("Authorization", "Bearer "+token)
		sseReq.Header.Set("Content-Type", "application/json")
		sseReq.Header.Set("Accept", "application/json, text/event-stream")
		sseResp, err := client.Do(sseReq) //nolint:gosec // test server URL
		if err != nil {
			t.Fatalf("iter %d sse Do: %v", i, err)
		}
		_, sseReadErr := io.Copy(io.Discard, sseResp.Body)
		_ = sseResp.Body.Close()

		var reused atomic.Bool
		followReq, _ := http.NewRequest(http.MethodPost, ts.URL+route, strings.NewReader(`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`))
		followReq.Header.Set("Authorization", "Bearer "+token)
		followReq.Header.Set("Content-Type", "application/json")
		followReq = followReq.WithContext(httptrace.WithClientTrace(followReq.Context(), &httptrace.ClientTrace{
			GotConn: func(info httptrace.GotConnInfo) {
				reused.Store(info.Reused)
			},
		}))
		followResp, err := client.Do(followReq) //nolint:gosec // test server URL
		if err != nil {
			t.Fatalf("iter %d follow-up Do (reused=%v, sseReadErr=%v): %v", i, reused.Load(), sseReadErr, err)
		}
		body, readErr := io.ReadAll(followResp.Body)
		_ = followResp.Body.Close()
		if readErr != nil {
			t.Fatalf("iter %d follow-up read (reused=%v, sseReadErr=%v): %v", i, reused.Load(), sseReadErr, readErr)
		}
		if followResp.StatusCode != http.StatusOK || !strings.Contains(string(body), `"ok": true`) {
			t.Fatalf("iter %d follow-up status=%d body=%q", i, followResp.StatusCode, body)
		}
	}
}

// syncBuf lets the test read the log while server goroutines still write.
type syncBuf struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuf) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuf) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}
