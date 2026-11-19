package main

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestIPRateLimiter_EvictsOldestOnFull(t *testing.T) {
	now := time.Date(2026, 11, 19, 8, 0, 0, 0, time.UTC)
	l := newIPRateLimiter(2, 100, time.Minute, 2, func() time.Time { return now })

	// IP1 enters at t=0 (resetAt = t+60s) and exhausts its per-IP limit (2).
	if !l.Allow("198.51.100.1") {
		t.Fatal("IP1 first request rejected")
	}
	if !l.Allow("198.51.100.1") {
		t.Fatal("IP1 second request rejected")
	}
	if l.Allow("198.51.100.1") {
		t.Fatal("IP1 third request should be rate limited")
	}

	// IP2 enters at t=10s (resetAt = t+70s) with 1 request.
	now = now.Add(10 * time.Second)
	if !l.Allow("198.51.100.2") {
		t.Fatal("IP2 first request rejected")
	}

	// At t=20s neither IP1 nor IP2 has expired, so the table (maxSize=2) is full.
	// IP3 must still be allowed and evict IP1 (earliest resetAt).
	now = now.Add(10 * time.Second)
	if !l.Allow("198.51.100.3") {
		t.Fatal("IP3 should be allowed by evicting oldest IP")
	}
	if len(l.buckets) != 2 {
		t.Fatalf("bucket count = %d, want 2", len(l.buckets))
	}

	// IP2 was not evicted, so it has only 1 remaining slot in its window.
	if !l.Allow("198.51.100.2") {
		t.Fatal("IP2 second request should still be allowed")
	}
	if l.Allow("198.51.100.2") {
		t.Fatal("IP2 third request should be rejected; existing window must remain intact")
	}

	// IP1 was evicted, so returning within its original window gives it a fresh window (2 requests).
	// Calling IP1 now evicts IP3 (resetAt t+70s vs IP2 t+70s? wait: IP2 resetAt is t+70s, IP3 resetAt is t+80s).
	if !l.Allow("198.51.100.1") {
		t.Fatal("evicted IP1 should get a fresh window (1/2)")
	}
	if !l.Allow("198.51.100.1") {
		t.Fatal("evicted IP1 should get a fresh window (2/2)")
	}
	if l.Allow("198.51.100.1") {
		t.Fatal("IP1 third request in fresh window should be rejected")
	}
}

func TestIPRateLimiter_GlobalLimitStillBlocksWhenFull(t *testing.T) {
	now := time.Date(2026, 11, 19, 8, 0, 0, 0, time.UTC)
	l := newIPRateLimiter(5, 2, time.Minute, 2, func() time.Time { return now })

	if !l.Allow("198.51.100.1") || !l.Allow("198.51.100.2") {
		t.Fatal("expected initial requests to succeed up to globalLimit=2")
	}
	if l.Allow("198.51.100.3") {
		t.Fatal("expected global limit to reject IP3 even though eviction would make room")
	}
}

func TestRateLimit_PerEndpointIsolationAndSweep(t *testing.T) {
	te := newTestEnv(t)
	te.gw.cfg.DCREnabled = true
	for _, ep := range rateLimitedEndpoints {
		te.gw.limiters[ep] = newIPRateLimiter(2, 100, time.Minute, 10, func() time.Time { return te.now })
	}
	te.handler = te.gw.routes()

	clientIP := "198.51.100.42"
	// Exhaust /token for clientIP.
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPost, "/token", strings.NewReader("grant_type=authorization_code"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.RemoteAddr = clientIP + ":1000"
		rec := httptest.NewRecorder()
		te.handler.ServeHTTP(rec, req)
		if rec.Code == http.StatusTooManyRequests {
			t.Fatalf("unexpected 429 on /token attempt %d", i)
		}
	}
	blockedToken := httptest.NewRequest(http.MethodPost, "/token", strings.NewReader("grant_type=authorization_code"))
	blockedToken.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	blockedToken.RemoteAddr = clientIP + ":1000"
	recToken := httptest.NewRecorder()
	te.handler.ServeHTTP(recToken, blockedToken)
	if recToken.Code != http.StatusTooManyRequests {
		t.Fatalf("/token third request: got %d, want 429", recToken.Code)
	}
	if !strings.Contains(recToken.Body.String(), `"temporarily_unavailable"`) {
		t.Fatalf("/token 429 body = %q, want JSON error temporarily_unavailable", recToken.Body.String())
	}

	// Exhausting /token must not block /authorize, /revoke, or /register for the same IP.
	verifier := "abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG"
	authReq := httptest.NewRequest(http.MethodGet, te.authorizeURL(nil, verifier), nil)
	authReq.RemoteAddr = clientIP + ":1001"
	authRec := httptest.NewRecorder()
	te.handler.ServeHTTP(authRec, authReq)
	if authRec.Code != http.StatusFound {
		t.Fatalf("/authorize after /token exhaustion: got %d, want 302", authRec.Code)
	}

	revokeReq := httptest.NewRequest(http.MethodPost, "/revoke", strings.NewReader("token=abc"))
	revokeReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	revokeReq.RemoteAddr = clientIP + ":1002"
	revokeRec := httptest.NewRecorder()
	te.handler.ServeHTTP(revokeRec, revokeReq)
	if revokeRec.Code == http.StatusTooManyRequests {
		t.Fatalf("/revoke after /token exhaustion: got unexpected 429")
	}

	regReq := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(`{}`))
	regReq.Header.Set("Content-Type", "application/json")
	regReq.RemoteAddr = clientIP + ":1003"
	regRec := httptest.NewRecorder()
	te.handler.ServeHTTP(regRec, regReq)
	if regRec.Code == http.StatusTooManyRequests {
		t.Fatalf("/register after /token exhaustion: got unexpected 429")
	}

	// sweepAll sweeps all endpoint limiters after the window expires.
	te.advance(2 * time.Minute)
	te.gw.sweepAll()
	for _, ep := range rateLimitedEndpoints {
		if n := len(te.gw.limiters[ep].buckets); n != 0 {
			t.Fatalf("limiter[%s] buckets after sweepAll = %d, want 0", ep, n)
		}
	}
}

func TestRateLimit_MetricsAndWarnLog(t *testing.T) {
	initMetrics(nil)
	srv := httptest.NewServer(metricsServer("").Handler)
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/metrics")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	metricsBody, _ := io.ReadAll(resp.Body)
	for _, ep := range rateLimitedEndpoints {
		want := `fleet_gateway_rate_limited_total{endpoint="` + ep + `"}`
		if !strings.Contains(string(metricsBody), want) {
			t.Errorf("/metrics missing pre-initialized series %s", want)
		}
	}

	var logBuf bytes.Buffer
	prevLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logBuf, nil)))
	t.Cleanup(func() {
		slog.SetDefault(prevLogger)
	})

	te := newTestEnv(t)
	for _, ep := range rateLimitedEndpoints {
		te.gw.limiters[ep] = newIPRateLimiter(1, 100, time.Minute, 10, func() time.Time { return te.now })
	}
	te.handler = te.gw.routes()

	beforeToken := testutil.ToFloat64(rateLimitedTotal.WithLabelValues(rateLimitEndpointToken))
	beforeAuth := testutil.ToFloat64(rateLimitedTotal.WithLabelValues(rateLimitEndpointAuthorize))
	beforeRevoke := testutil.ToFloat64(rateLimitedTotal.WithLabelValues(rateLimitEndpointRevoke))

	clientIP := "198.51.100.77"
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPost, "/token", strings.NewReader("grant_type=authorization_code"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.RemoteAddr = clientIP + ":4321"
		rec := httptest.NewRecorder()
		te.handler.ServeHTTP(rec, req)
	}

	if got := testutil.ToFloat64(rateLimitedTotal.WithLabelValues(rateLimitEndpointToken)); got != beforeToken+1 {
		t.Fatalf("token rate_limited_total = %v, want %v", got, beforeToken+1)
	}
	if got := testutil.ToFloat64(rateLimitedTotal.WithLabelValues(rateLimitEndpointAuthorize)); got != beforeAuth {
		t.Fatalf("authorize rate_limited_total changed unexpectedly: got %v, want %v", got, beforeAuth)
	}
	if got := testutil.ToFloat64(rateLimitedTotal.WithLabelValues(rateLimitEndpointRevoke)); got != beforeRevoke {
		t.Fatalf("revoke rate_limited_total changed unexpectedly: got %v, want %v", got, beforeRevoke)
	}

	verifier := "abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG"
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, te.authorizeURL(nil, verifier), nil)
		req.RemoteAddr = clientIP + ":4322"
		rec := httptest.NewRecorder()
		te.handler.ServeHTTP(rec, req)
	}
	if got := testutil.ToFloat64(rateLimitedTotal.WithLabelValues(rateLimitEndpointAuthorize)); got != beforeAuth+1 {
		t.Fatalf("authorize rate_limited_total = %v, want %v", got, beforeAuth+1)
	}

	logs := logBuf.String()
	if !strings.Contains(logs, `"msg":"rate limited"`) {
		t.Fatalf("expected slog warning 'rate limited' in logs, got %s", logs)
	}
	if !strings.Contains(logs, `"endpoint":"token"`) || !strings.Contains(logs, `"endpoint":"authorize"`) {
		t.Fatalf("expected endpoint labels in rate limited log lines, got %s", logs)
	}
	if strings.Contains(logs, clientIP) {
		t.Fatalf("rate limited log must not contain client IP %q, got %s", clientIP, logs)
	}
}
