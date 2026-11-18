package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestInFlightLimitRejectsOverCap(t *testing.T) {
	release := make(chan struct{})
	started := make(chan struct{}, 2)
	h := inFlightLimit(make(chan struct{}, 2), func(w http.ResponseWriter, _ *http.Request) {
		started <- struct{}{}
		<-release
		w.WriteHeader(http.StatusOK)
	})

	done := make(chan int, 2)
	for range 2 {
		go func() {
			rec := httptest.NewRecorder()
			h(rec, httptest.NewRequest(http.MethodPost, "/mcp/ns/x", nil))
			done <- rec.Code
		}()
	}
	<-started
	<-started

	rec := httptest.NewRecorder()
	third := make(chan struct{})
	go func() {
		h(rec, httptest.NewRequest(http.MethodPost, "/mcp/ns/x", nil))
		close(third)
	}()
	select {
	case <-third:
	case <-time.After(2 * time.Second):
		close(release)
		t.Fatal("third request was let through instead of rejected")
	}
	if rec.Code != http.StatusServiceUnavailable || rec.Header().Get("Retry-After") == "" {
		t.Fatalf("third request: code %d, Retry-After %q; want 503 with Retry-After", rec.Code, rec.Header().Get("Retry-After"))
	}

	close(release)
	for range 2 {
		if code := <-done; code != http.StatusOK {
			t.Fatalf("in-cap request got %d", code)
		}
	}
	rec = httptest.NewRecorder()
	h(rec, httptest.NewRequest(http.MethodPost, "/mcp/ns/x", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("after release: code %d, want 200 (slots not freed)", rec.Code)
	}
}

func TestUpstreamTimeoutConfig(t *testing.T) {
	setBaseValidEnv(t)
	t.Setenv("UPSTREAM_TIMEOUT", "")
	t.Setenv("MCP_MAX_IN_FLIGHT", "")
	cfg, err := loadConfigFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ProxyHeaderTimeout != 60*time.Second || cfg.MCPMaxInFlight != 16 {
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
