package main

import (
	"bytes"
	"fmt"
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

	authClientIP := "198.51.100.78"
	verifier := "abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG"
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, te.authorizeURL(nil, verifier), nil)
		req.RemoteAddr = authClientIP + ":4322"
		rec := httptest.NewRecorder()
		te.handler.ServeHTTP(rec, req)
	}
	if got := testutil.ToFloat64(rateLimitedTotal.WithLabelValues(rateLimitEndpointAuthorize)); got != beforeAuth+1 {
		t.Fatalf("authorize rate_limited_total = %v, want %v", got, beforeAuth+1)
	}

	beforeConsent := testutil.ToFloat64(rateLimitedTotal.WithLabelValues(rateLimitEndpointConsent))
	consentClientIP := "198.51.100.79"
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPost, "/consent", strings.NewReader("id=missing&csrf=bad"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.RemoteAddr = consentClientIP + ":4323"
		rec := httptest.NewRecorder()
		te.handler.ServeHTTP(rec, req)
		if i == 1 && rec.Code != http.StatusTooManyRequests {
			t.Fatalf("consent second request: got %d, want 429", rec.Code)
		}
	}
	if got := testutil.ToFloat64(rateLimitedTotal.WithLabelValues(rateLimitEndpointConsent)); got != beforeConsent+1 {
		t.Fatalf("consent rate_limited_total = %v, want %v", got, beforeConsent+1)
	}

	logs := logBuf.String()
	if !strings.Contains(logs, `"msg":"rate limited"`) {
		t.Fatalf("expected slog warning 'rate limited' in logs, got %s", logs)
	}
	if !strings.Contains(logs, `"endpoint":"token"`) || !strings.Contains(logs, `"endpoint":"authorize"`) {
		t.Fatalf("expected endpoint labels in rate limited log lines, got %s", logs)
	}
	if strings.Contains(logs, clientIP) || strings.Contains(logs, authClientIP) {
		t.Fatalf("rate limited log must not contain client IP (%q or %q), got %s", clientIP, authClientIP, logs)
	}
}

func TestRateLimit_LogSamplingAndCounter(t *testing.T) {
	initMetrics(nil)

	var logBuf bytes.Buffer
	prevLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logBuf, nil)))
	t.Cleanup(func() {
		slog.SetDefault(prevLogger)
	})

	te := newTestEnv(t)
	te.gw.limiters[rateLimitEndpointToken] = newIPRateLimiter(1, 1000, time.Minute, 10, te.gw.now)
	te.handler = te.gw.routes()

	clientIP := "198.51.100.99"
	sendTokenReq := func() int {
		req := httptest.NewRequest(http.MethodPost, "/token", strings.NewReader("grant_type=authorization_code"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.RemoteAddr = clientIP + ":5000"
		rec := httptest.NewRecorder()
		te.handler.ServeHTTP(rec, req)
		return rec.Code
	}

	// Exhaust the 1-request allowance first.
	if code := sendTokenReq(); code == http.StatusTooManyRequests {
		t.Fatalf("initial /token request unexpectedly rate limited")
	}

	before := testutil.ToFloat64(rateLimitedTotal.WithLabelValues(rateLimitEndpointToken))

	const flood = 25
	for i := 0; i < flood; i++ {
		if code := sendTokenReq(); code != http.StatusTooManyRequests {
			t.Fatalf("flood request %d: got %d, want 429", i, code)
		}
		te.advance(time.Second)
	}

	if got := testutil.ToFloat64(rateLimitedTotal.WithLabelValues(rateLimitEndpointToken)); got != before+float64(flood) {
		t.Fatalf("rate_limited_total after flood = %v, want %v", got, before+float64(flood))
	}
	if got := strings.Count(logBuf.String(), `"msg":"rate limited"`); got != 1 {
		t.Fatalf("logged warnings during 1m flood = %d, want 1; logs=%s", got, logBuf.String())
	}
	if strings.Contains(logBuf.String(), clientIP) {
		t.Fatalf("rate limited log must not contain client IP %q, got %s", clientIP, logBuf.String())
	}

	// Advance fake clock past 1 minute from the logged rejection; the limiter window has also reset.
	te.advance(time.Minute)
	if code := sendTokenReq(); code == http.StatusTooManyRequests {
		t.Fatalf("post-window /token request unexpectedly rate limited")
	}

	const secondFlood = 5
	for i := 0; i < secondFlood; i++ {
		if code := sendTokenReq(); code != http.StatusTooManyRequests {
			t.Fatalf("second flood request %d: got %d, want 429", i, code)
		}
	}

	if got := testutil.ToFloat64(rateLimitedTotal.WithLabelValues(rateLimitEndpointToken)); got != before+float64(flood+secondFlood) {
		t.Fatalf("rate_limited_total after second flood = %v, want %v", got, before+float64(flood+secondFlood))
	}
	if got := strings.Count(logBuf.String(), `"msg":"rate limited"`); got != 2 {
		t.Fatalf("logged warnings after advancing past 1m = %d, want 2; logs=%s", got, logBuf.String())
	}
}

func TestRateLimitLogSampler_BoundedEvictsOldest(t *testing.T) {
	now := time.Date(2026, 11, 19, 8, 0, 0, 0, time.UTC)
	s := newRateLimitLogSampler(time.Minute, 3, func() time.Time { return now })

	if !s.Allow("198.51.100.1") {
		t.Fatal("IP1 first Allow = false, want true")
	}
	if s.Allow("198.51.100.1") {
		t.Fatal("IP1 repeat within window = true, want false")
	}

	now = now.Add(time.Second)
	if !s.Allow("198.51.100.2") {
		t.Fatal("IP2 first Allow = false, want true")
	}
	now = now.Add(time.Second)
	if !s.Allow("198.51.100.3") {
		t.Fatal("IP3 first Allow = false, want true")
	}

	// Map is at capacity 3; inserting IP4 must evict IP1 (oldest timestamp at t=0).
	now = now.Add(time.Second)
	if !s.Allow("198.51.100.4") {
		t.Fatal("IP4 first Allow = false, want true")
	}
	if len(s.lastSeen) != 3 {
		t.Fatalf("lastSeen len = %d, want 3", len(s.lastSeen))
	}
	if _, ok := s.lastSeen["198.51.100.1"]; ok {
		t.Fatal("expected oldest entry 198.51.100.1 to be evicted")
	}
	for _, ip := range []string{"198.51.100.2", "198.51.100.3", "198.51.100.4"} {
		if _, ok := s.lastSeen[ip]; !ok {
			t.Fatalf("expected entry %s to remain", ip)
		}
	}

	// Default-cap sampler stays bounded under a large flood of unique IPs.
	bounded := newRateLimitLogSampler(defaultRateLimitLogWindow, defaultRateLimitLogCap, func() time.Time { return now })
	for i := 0; i < defaultRateLimitLogCap+200; i++ {
		now = now.Add(time.Millisecond)
		ip := fmt.Sprintf("203.0.%d.%d", i/256, i%256)
		if !bounded.Allow(ip) {
			t.Fatalf("unique IP %s first Allow = false, want true", ip)
		}
		if len(bounded.lastSeen) > defaultRateLimitLogCap {
			t.Fatalf("lastSeen len = %d exceeded cap %d", len(bounded.lastSeen), defaultRateLimitLogCap)
		}
	}
	if len(bounded.lastSeen) != defaultRateLimitLogCap {
		t.Fatalf("lastSeen len = %d, want %d", len(bounded.lastSeen), defaultRateLimitLogCap)
	}

	// Sweep clears expired entries once the window elapses.
	now = now.Add(defaultRateLimitLogWindow)
	if removed := bounded.Sweep(); removed != defaultRateLimitLogCap {
		t.Fatalf("Sweep removed %d entries, want %d", removed, defaultRateLimitLogCap)
	}
	if len(bounded.lastSeen) != 0 {
		t.Fatalf("lastSeen len after Sweep = %d, want 0", len(bounded.lastSeen))
	}
}
