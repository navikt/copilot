package main

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/navikt/copilot/fleet-gateway/internal/httpx"
	"github.com/navikt/copilot/fleet-gateway/internal/metrics"
)

func TestPerUserInFlightConfig(t *testing.T) {
	setBaseValidEnv(t)
	t.Setenv("MCP_MAX_IN_FLIGHT_PER_USER", "")
	cfg, err := loadConfigFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MCPMaxInFlightPerUser != defaultMCPMaxInFlightPerUser {
		t.Fatalf("default MCPMaxInFlightPerUser = %d, want %d", cfg.MCPMaxInFlightPerUser, defaultMCPMaxInFlightPerUser)
	}

	t.Setenv("MCP_MAX_IN_FLIGHT_PER_USER", "0")
	cfg, err = loadConfigFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MCPMaxInFlightPerUser != defaultMCPMaxInFlightPerUser {
		t.Fatalf("zero MCPMaxInFlightPerUser = %d, want %d", cfg.MCPMaxInFlightPerUser, defaultMCPMaxInFlightPerUser)
	}

	t.Setenv("MCP_MAX_IN_FLIGHT_PER_USER", "-3")
	cfg, err = loadConfigFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MCPMaxInFlightPerUser != defaultMCPMaxInFlightPerUser {
		t.Fatalf("negative MCPMaxInFlightPerUser = %d, want %d", cfg.MCPMaxInFlightPerUser, defaultMCPMaxInFlightPerUser)
	}

	t.Setenv("MCP_MAX_IN_FLIGHT_PER_USER", "6")
	cfg, err = loadConfigFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MCPMaxInFlightPerUser != 6 {
		t.Fatalf("explicit MCPMaxInFlightPerUser = %d, want 6", cfg.MCPMaxInFlightPerUser)
	}
}

func TestPerUserInFlightCap(t *testing.T) {
	var logBuf syncBuf
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelInfo})))
	t.Cleanup(func() { slog.SetDefault(old) })

	te := newTestEnv(t)
	te.gw.cfg.MCPMaxInFlight = 8
	te.gw.cfg.MCPMaxInFlightPerRoute = 8
	te.gw.cfg.MCPMaxInFlightPerUser = 2
	te.handler = te.gw.routes()

	release := make(chan struct{})
	var releaseOnce sync.Once
	stopHold := func() { releaseOnce.Do(func() { close(release) }) }
	defer stopHold()

	entered := make(chan struct{}, 2)
	te.upstreamServer.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if testQuery(r).Get("hold") == "1" {
			entered <- struct{}{}
			<-release
		}
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"jsonrpc": "2.0", "id": 1, "result": map[string]any{}})
	})

	tokenA := te.obtainAccessTokenForResource(te.resource)
	te.mu.Lock()
	te.navIdent = "B654321"
	te.mu.Unlock()
	tokenB := te.obtainAccessTokenForResource(te.resource)

	ts := httptest.NewServer(withTestQuery(te.handler))
	defer ts.Close()

	route := te.gw.cfg.Routes[0].Route

	held := make(chan int, 2)
	for range 2 {
		go func() { held <- capPost(t, ts, tokenA, "?hold=1") }()
	}
	for range 2 {
		<-entered
	}

	shed := metrics.HTTPRequests.WithLabelValues("503", "user_cap")
	beforeShed := testutil.ToFloat64(shed)
	mcpDeny := metrics.MCPRequests.WithLabelValues(route, "user_cap", "deny", "user_cap", "")
	beforeDeny := testutil.ToFloat64(mcpDeny)
	mcpAllow := metrics.MCPRequests.WithLabelValues(route, "user_cap", "allow", "", "")
	beforeAllow := testutil.ToFloat64(mcpAllow)

	// (a) Next call for user A gets 503 with Retry-After and user_cap shed/audit.
	req, err := http.NewRequest(http.MethodPost, ts.URL+route, strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+tokenA)
	req.Header.Set("Content-Type", "application/json")
	resp, err := ts.Client().Do(req) //nolint:gosec // test server URL
	if err != nil {
		t.Fatal(err)
	}
	respBody, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("over per-user cap: status %d, want 503 (%s)", resp.StatusCode, respBody)
	}
	if got := resp.Header.Get("Retry-After"); got != "1" {
		t.Fatalf("Retry-After = %q, want %q", got, "1")
	}
	var errJSON map[string]string
	if err := json.Unmarshal(respBody, &errJSON); err != nil {
		t.Fatalf("decode 503 body: %v (%s)", err, respBody)
	}
	if errJSON["error"] != "too_many_requests" || errJSON["message"] != "too many concurrent MCP requests for this user; retry shortly" {
		t.Fatalf("unexpected 503 body: %+v", errJSON)
	}
	if got := testutil.ToFloat64(shed); got != beforeShed+1 {
		t.Errorf("metrics.HTTPRequests(503, user_cap) = %v, want %v", got, beforeShed+1)
	}
	if got := testutil.ToFloat64(mcpDeny); got != beforeDeny+1 {
		t.Errorf("metrics.MCPRequests(user_cap, deny) = %v, want %v", got, beforeDeny+1)
	}
	if got := testutil.ToFloat64(mcpAllow); got != beforeAllow {
		t.Errorf("metrics.MCPRequests(user_cap, allow) = %v, want %v", got, beforeAllow)
	}

	var foundAudit bool
	for _, line := range strings.Split(logBuf.String(), "\n") {
		if !strings.Contains(line, `"outcome":"user_cap"`) {
			continue
		}
		foundAudit = true
		if !strings.Contains(line, `"decision":{"decision":false,"context":{"reason":"user_cap"}}`) {
			t.Errorf("audit line not decision=false reason=user_cap:\n%s", line)
		}
	}
	if !foundAudit {
		t.Fatalf("no audit mcp line with outcome user_cap:\n%s", logBuf.String())
	}

	// While user A is capped, a second user is served.
	if code := capPost(t, ts, tokenB, ""); code != http.StatusOK {
		t.Fatalf("second user while first user capped: code %d, want 200", code)
	}

	// (b) Release held calls and verify user A is served again.
	stopHold()
	for range 2 {
		if code := <-held; code != http.StatusOK {
			t.Fatalf("held request: code %d, want 200", code)
		}
	}
	if code := capPost(t, ts, tokenA, ""); code != http.StatusOK {
		t.Fatalf("user A after held calls finished: code %d, want 200", code)
	}
}

func TestPerUserSSEConfig(t *testing.T) {
	setBaseValidEnv(t)
	t.Setenv("MCP_MAX_SSE_PER_USER", "")
	cfg, err := loadConfigFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MCPMaxSSEPerUser != defaultMCPMaxSSEPerUser {
		t.Fatalf("default MCPMaxSSEPerUser = %d, want %d", cfg.MCPMaxSSEPerUser, defaultMCPMaxSSEPerUser)
	}

	t.Setenv("MCP_MAX_SSE_PER_USER", "0")
	cfg, err = loadConfigFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MCPMaxSSEPerUser != defaultMCPMaxSSEPerUser {
		t.Fatalf("zero MCPMaxSSEPerUser = %d, want %d", cfg.MCPMaxSSEPerUser, defaultMCPMaxSSEPerUser)
	}

	t.Setenv("MCP_MAX_SSE_PER_USER", "-3")
	cfg, err = loadConfigFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MCPMaxSSEPerUser != defaultMCPMaxSSEPerUser {
		t.Fatalf("negative MCPMaxSSEPerUser = %d, want %d", cfg.MCPMaxSSEPerUser, defaultMCPMaxSSEPerUser)
	}

	t.Setenv("MCP_MAX_SSE_PER_USER", "6")
	cfg, err = loadConfigFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MCPMaxSSEPerUser != 6 {
		t.Fatalf("explicit MCPMaxSSEPerUser = %d, want 6", cfg.MCPMaxSSEPerUser)
	}
}

func TestPerUserSSECap(t *testing.T) {
	var logBuf syncBuf
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelInfo})))
	t.Cleanup(func() { slog.SetDefault(old) })

	te := newTestEnv(t)
	te.gw.cfg.MCPMaxSSE = 16
	te.handler = te.gw.routes()

	release := make(chan struct{})
	var releaseOnce sync.Once
	stopHold := func() { releaseOnce.Do(func() { close(release) }) }
	defer stopHold()

	te.upstreamServer.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet || testQuery(r).Get("sse") == "1" {
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(w, "data: ready\n\n")
			w.(http.Flusher).Flush()
			if testQuery(r).Get("once") == "1" {
				return
			}
			select {
			case <-release:
			case <-r.Context().Done():
			}
			return
		}
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"jsonrpc": "2.0", "id": 1, "result": map[string]any{}})
	})

	tokenA := te.obtainAccessTokenForResource(te.resource)
	te.mu.Lock()
	te.navIdent = "B654321"
	te.mu.Unlock()
	tokenB := te.obtainAccessTokenForResource(te.resource)

	var activeHandlers sync.WaitGroup
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		activeHandlers.Add(1)
		defer activeHandlers.Done()
		withTestQuery(te.handler).ServeHTTP(w, r)
	}))
	defer ts.Close()

	route := te.gw.cfg.Routes[0].Route

	openSSE := func(token, query string) *http.Response {
		t.Helper()
		req, err := http.NewRequest(http.MethodGet, ts.URL+route+query, nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Accept", "text/event-stream")
		resp, err := ts.Client().Do(req) //nolint:gosec // test server URL
		if err != nil {
			t.Fatal(err)
		}
		return resp
	}

	streamBodies := make([]io.ReadCloser, 0, 4)
	for i := range 4 {
		resp := openSSE(tokenA, "")
		defer func() { _ = resp.Body.Close() }()
		streamBodies = append(streamBodies, resp.Body)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("SSE stream %d: status %d, want 200", i, resp.StatusCode)
		}
	}

	shed := metrics.HTTPRequests.WithLabelValues("503", "user_sse_cap")
	beforeShed := testutil.ToFloat64(shed)
	mcpDeny := metrics.MCPRequests.WithLabelValues(route, "user_sse_cap", "deny", "user_sse_cap", "")
	beforeDeny := testutil.ToFloat64(mcpDeny)
	mcpAllow := metrics.MCPRequests.WithLabelValues(route, "user_sse_cap", "allow", "", "")
	beforeAllow := testutil.ToFloat64(mcpAllow)

	// (a) 5th concurrent SSE stream for user A gets 503 with reason=user_sse_cap.
	fifth := openSSE(tokenA, "")
	respBody, _ := io.ReadAll(fifth.Body)
	_ = fifth.Body.Close()

	if fifth.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("5th SSE stream: status %d, want 503 (%s)", fifth.StatusCode, respBody)
	}
	if got := fifth.Header.Get("Retry-After"); got != "1" {
		t.Fatalf("Retry-After = %q, want %q", got, "1")
	}
	var errJSON map[string]string
	if err := json.Unmarshal(respBody, &errJSON); err != nil {
		t.Fatalf("decode 503 body: %v (%s)", err, respBody)
	}
	if errJSON["error"] != "too_many_requests" || errJSON["message"] != "too many concurrent SSE streams for this user; retry shortly" {
		t.Fatalf("unexpected 503 body: %+v", errJSON)
	}
	if got := testutil.ToFloat64(shed); got != beforeShed+1 {
		t.Errorf("metrics.HTTPRequests(503, user_sse_cap) = %v, want %v", got, beforeShed+1)
	}
	if got := testutil.ToFloat64(mcpDeny); got != beforeDeny+1 {
		t.Errorf("metrics.MCPRequests(user_sse_cap, deny) = %v, want %v", got, beforeDeny+1)
	}
	if got := testutil.ToFloat64(mcpAllow); got != beforeAllow {
		t.Errorf("metrics.MCPRequests(user_sse_cap, allow) = %v, want %v", got, beforeAllow)
	}

	var foundAudit bool
	for _, line := range strings.Split(logBuf.String(), "\n") {
		if !strings.Contains(line, `"outcome":"user_sse_cap"`) {
			continue
		}
		foundAudit = true
		if !strings.Contains(line, `"decision":{"decision":false,"context":{"reason":"user_sse_cap"}}`) {
			t.Errorf("audit line not decision=false reason=user_sse_cap:\n%s", line)
		}
	}
	if !foundAudit {
		t.Fatalf("no audit mcp line with outcome user_sse_cap:\n%s", logBuf.String())
	}

	// Another user can still open an SSE stream while user A is at the per-user SSE cap.
	otherResp := openSSE(tokenB, "?once=1")
	_, _ = io.Copy(io.Discard, otherResp.Body)
	_ = otherResp.Body.Close()
	if otherResp.StatusCode != http.StatusOK {
		t.Fatalf("second user SSE stream while user A capped: status %d, want 200", otherResp.StatusCode)
	}

	// (b) While user A holds 4 open SSE streams, tool-call POSTs still succeed.
	for i := range 4 {
		if code := capPost(t, ts, tokenA, ""); code != http.StatusOK {
			t.Fatalf("tool-call POST %d while holding 4 SSE streams: code %d, want 200", i, code)
		}
	}

	// (c) After streams close, slots are freed and the map entry is deleted.
	stopHold()
	for _, body := range streamBodies {
		_, _ = io.Copy(io.Discard, body)
		_ = body.Close()
	}
	activeHandlers.Wait()

	if n := te.gw.userSSE.Len(); n != 0 {
		t.Fatalf("userSSE after streams closed: len=%d, want 0", n)
	}

	if inFlightMapLen := te.gw.userInFlight.Len(); inFlightMapLen != 0 {
		t.Fatalf("userInFlight map len = %d after streams closed, want 0", inFlightMapLen)
	}

	reopened := openSSE(tokenA, "?once=1")
	_, _ = io.Copy(io.Discard, reopened.Body)
	_ = reopened.Body.Close()
	if reopened.StatusCode != http.StatusOK {
		t.Fatalf("SSE stream after slots freed: status %d, want 200", reopened.StatusCode)
	}
}
