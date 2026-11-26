package main

import (
	"github.com/navikt/copilot/fleet-gateway/internal/metrics"
	"github.com/navikt/copilot/fleet-gateway/internal/ratelimit"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
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
		t.Fatal("PROBE_ENABLED without PROBE_ALLOWED_AZP, token path and cluster must fail validation")
	}
	cfg.ProbeAllowedAzp, cfg.ProbeTokenPath = "my-copilot-client-id", "/var/run/secrets/nais.io/serviceaccount/token"
	if err := validateConfig(&cfg); err == nil {
		t.Fatal("PROBE_ENABLED without NAIS_CLUSTER_NAME must fail validation")
	}
	cfg.ProbeCluster = "dev-gcp"
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

// A bearer-less or garbage request is limited per IP before token introspection.
func TestProbeRateLimitedPerIPBeforeAuth(t *testing.T) {
	te := newTestEnv(t)
	cfg := *te.gw.cfg
	cfg.ProbeEnabled, cfg.ProbeAllowedAzp, cfg.ProbeTokenPath, cfg.ProbeCluster = true, "azp", "/x", "dev-gcp"
	gw, err := newGateway(&cfg, te.gw.now)
	if err != nil {
		t.Fatal(err)
	}
	gw.limiters[metrics.EndpointProbe] = ratelimit.New(1, 100, time.Minute, 10, te.gw.now)
	h := gw.routes()
	var got []int
	for range 2 {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/probe/v1/tools", strings.NewReader("{}")))
		got = append(got, rec.Code)
	}
	if got[0] != http.StatusUnauthorized || got[1] != http.StatusTooManyRequests {
		t.Fatalf("statuses %v, want [401 429]", got)
	}
}
