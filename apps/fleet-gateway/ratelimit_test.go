package main

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/navikt/copilot/fleet-gateway/internal/metrics"
	"github.com/navikt/copilot/fleet-gateway/internal/ratelimit"
)

func TestRateLimit_PerEndpointIsolationAndSweep(t *testing.T) {
	te := newTestEnv(t)
	te.gw.cfg.DCREnabled = true
	for _, ep := range metrics.RateLimitedEndpoints {
		te.gw.limiters[ep] = ratelimit.New(2, 100, time.Minute, 10, func() time.Time { return te.now })
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
	for _, ep := range metrics.RateLimitedEndpoints {
		if n := te.gw.limiters[ep].Len(); n != 0 {
			t.Fatalf("limiter[%s] buckets after sweepAll = %d, want 0", ep, n)
		}
	}
}

func TestRateLimit_MetricsAndWarnLog(t *testing.T) {
	metrics.Init(nil)
	srv := httptest.NewServer(metrics.Server("").Handler)
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/metrics")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	metricsBody, _ := io.ReadAll(resp.Body)
	for _, ep := range metrics.RateLimitedEndpoints {
		want := `fleet_gateway_rate_limited_total{oauth_endpoint="` + ep + `"}`
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
	for _, ep := range metrics.RateLimitedEndpoints {
		te.gw.limiters[ep] = ratelimit.New(1, 100, time.Minute, 10, func() time.Time { return te.now })
	}
	te.handler = te.gw.routes()

	beforeToken := testutil.ToFloat64(metrics.RateLimited.WithLabelValues(metrics.EndpointToken))
	beforeAuth := testutil.ToFloat64(metrics.RateLimited.WithLabelValues(metrics.EndpointAuthorize))
	beforeRevoke := testutil.ToFloat64(metrics.RateLimited.WithLabelValues(metrics.EndpointRevoke))

	clientIP := "198.51.100.77"
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPost, "/token", strings.NewReader("grant_type=authorization_code"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.RemoteAddr = clientIP + ":4321"
		rec := httptest.NewRecorder()
		te.handler.ServeHTTP(rec, req)
	}

	if got := testutil.ToFloat64(metrics.RateLimited.WithLabelValues(metrics.EndpointToken)); got != beforeToken+1 {
		t.Fatalf("token rate_limited_total = %v, want %v", got, beforeToken+1)
	}
	if got := testutil.ToFloat64(metrics.RateLimited.WithLabelValues(metrics.EndpointAuthorize)); got != beforeAuth {
		t.Fatalf("authorize rate_limited_total changed unexpectedly: got %v, want %v", got, beforeAuth)
	}
	if got := testutil.ToFloat64(metrics.RateLimited.WithLabelValues(metrics.EndpointRevoke)); got != beforeRevoke {
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
	if got := testutil.ToFloat64(metrics.RateLimited.WithLabelValues(metrics.EndpointAuthorize)); got != beforeAuth+1 {
		t.Fatalf("authorize rate_limited_total = %v, want %v", got, beforeAuth+1)
	}

	beforeConsent := testutil.ToFloat64(metrics.RateLimited.WithLabelValues(metrics.EndpointConsent))
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
	if got := testutil.ToFloat64(metrics.RateLimited.WithLabelValues(metrics.EndpointConsent)); got != beforeConsent+1 {
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
	metrics.Init(nil)

	var logBuf bytes.Buffer
	prevLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logBuf, nil)))
	t.Cleanup(func() {
		slog.SetDefault(prevLogger)
	})

	te := newTestEnv(t)
	te.gw.limiters[metrics.EndpointToken] = ratelimit.New(1, 1000, time.Minute, 10, te.gw.now)
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

	before := testutil.ToFloat64(metrics.RateLimited.WithLabelValues(metrics.EndpointToken))

	const flood = 25
	for i := 0; i < flood; i++ {
		if code := sendTokenReq(); code != http.StatusTooManyRequests {
			t.Fatalf("flood request %d: got %d, want 429", i, code)
		}
		te.advance(time.Second)
	}

	if got := testutil.ToFloat64(metrics.RateLimited.WithLabelValues(metrics.EndpointToken)); got != before+float64(flood) {
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

	if got := testutil.ToFloat64(metrics.RateLimited.WithLabelValues(metrics.EndpointToken)); got != before+float64(flood+secondFlood) {
		t.Fatalf("rate_limited_total after second flood = %v, want %v", got, before+float64(flood+secondFlood))
	}
	if got := strings.Count(logBuf.String(), `"msg":"rate limited"`); got != 2 {
		t.Fatalf("logged warnings after advancing past 1m = %d, want 2; logs=%s", got, logBuf.String())
	}
}

func TestRateLimit_Register429JSONAndLogPerEndpoint(t *testing.T) {
	var logBuf bytes.Buffer
	prevLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logBuf, nil)))
	t.Cleanup(func() {
		slog.SetDefault(prevLogger)
	})

	te := newTestEnv(t)
	te.gw.cfg.DCREnabled = true
	te.gw.limiters[metrics.EndpointToken] = ratelimit.New(1, 100, time.Minute, 10, func() time.Time { return te.now })
	te.gw.limiters[metrics.EndpointRegister] = ratelimit.New(1, 100, time.Minute, 10, func() time.Time { return te.now })
	te.handler = te.gw.routes()

	clientIP := "198.51.100.88"
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPost, "/token", strings.NewReader("grant_type=authorization_code"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.RemoteAddr = clientIP + ":5001"
		te.handler.ServeHTTP(httptest.NewRecorder(), req)
	}

	var regRec *httptest.ResponseRecorder
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(`{}`))
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = clientIP + ":5002"
		regRec = httptest.NewRecorder()
		te.handler.ServeHTTP(regRec, req)
	}
	if regRec.Code != http.StatusTooManyRequests {
		t.Fatalf("/register second request: got %d, want 429", regRec.Code)
	}
	if ct := regRec.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Fatalf("/register 429 Content-Type = %q, want application/json", ct)
	}
	var body map[string]string
	if err := json.Unmarshal(regRec.Body.Bytes(), &body); err != nil {
		t.Fatalf("/register 429 body is not valid JSON: %v (%q)", err, regRec.Body.String())
	}
	if body["error"] != "temporarily_unavailable" {
		t.Fatalf("/register 429 error = %q, want temporarily_unavailable (body=%q)", body["error"], regRec.Body.String())
	}

	logs := logBuf.String()
	if !strings.Contains(logs, `"endpoint":"token"`) || !strings.Contains(logs, `"endpoint":"register"`) {
		t.Fatalf("expected rate limited logs for both token and register for same IP, got %s", logs)
	}
}
