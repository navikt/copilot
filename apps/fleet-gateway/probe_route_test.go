package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The probe is off by default (#2985): no route, and enabling it without its
// config refuses to start.
func TestProbeOffByDefault(t *testing.T) {
	te := newTestEnv(t)
	probeStatus := func(h http.Handler) int {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/probe/v1/tools", strings.NewReader("{}")))
		return rec.Code
	}
	if got := probeStatus(te.handler); got != http.StatusNotFound {
		t.Fatalf("default config: status %d, want 404", got)
	}

	cfg := *te.gw.cfg
	cfg.ProbeEnabled = true
	if err := validateConfig(&cfg); err == nil {
		t.Fatal("PROBE_ENABLED without PROBE_ALLOWED_AZP and token path must fail validation")
	}
	cfg.ProbeAllowedAzp, cfg.ProbeTokenPath = "my-copilot-client-id", "/var/run/secrets/nais.io/serviceaccount/token"
	if err := validateConfig(&cfg); err != nil {
		t.Fatalf("validateConfig: %v", err)
	}
	gw, err := newGateway(&cfg, te.gw.now)
	if err != nil {
		t.Fatalf("newGateway: %v", err)
	}
	if got := probeStatus(gw.routes()); got != http.StatusUnauthorized {
		t.Fatalf("enabled, no token: status %d, want 401", got)
	}
}
