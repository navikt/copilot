package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/navikt/copilot/fleet-control/internal/metrics"
)

func TestHTTPMetricsUseRoutePattern(t *testing.T) {
	h := newServer().routes(testAuthz(t), adminID, nil, nil)
	pattern := "GET " + apisPrefix + "/namespaces/{namespace}/agentprofiles/{name}"
	before := testutil.ToFloat64(metrics.HTTPRequests.WithLabelValues("GET /health", "200"))
	beforeSnap := testutil.ToFloat64(metrics.HTTPRequests.WithLabelValues("GET /internal/v1/snapshots/routes", "503"))
	beforeMiss := testutil.ToFloat64(metrics.HTTPRequests.WithLabelValues("unmatched", "404"))
	get(t, h, "/health", "")
	get(t, h, "/internal/v1/snapshots/routes", "")
	get(t, h, "/no/such/path-123", "")
	if got := testutil.ToFloat64(metrics.HTTPRequests.WithLabelValues("GET /health", "200")) - before; got != 1 {
		t.Errorf("health: %v, want 1", got)
	}
	if got := testutil.ToFloat64(metrics.HTTPRequests.WithLabelValues("GET /internal/v1/snapshots/routes", "503")) - beforeSnap; got != 1 {
		t.Errorf("snapshot 503: %v, want 1", got)
	}
	if got := testutil.ToFloat64(metrics.HTTPRequests.WithLabelValues("unmatched", "404")) - beforeMiss; got != 1 {
		t.Errorf("unmatched: %v, want 1", got)
	}
	// A path with values must count under its pattern, never the raw path.
	n := testutil.CollectAndCount(metrics.HTTPRequests)
	get(t, h, apisPrefix+"/namespaces/team-a/agentprofiles/p1", "")
	get(t, h, apisPrefix+"/namespaces/team-b/agentprofiles/p2", "")
	if got := testutil.CollectAndCount(metrics.HTTPRequests); got > n+1 {
		t.Errorf("label series grew by %d for two raw paths, want at most 1", got-n)
	}
	mfs, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatal(err)
	}
	var found float64
	for _, mf := range mfs {
		if mf.GetName() != "fleet_control_http_requests_total" {
			continue
		}
		for _, m := range mf.GetMetric() {
			for _, l := range m.GetLabel() {
				if l.GetName() == "route" && l.GetValue() == pattern {
					found += m.GetCounter().GetValue()
				}
			}
		}
	}
	if found < 2 {
		t.Errorf("no series for pattern %q", pattern)
	}
}

func TestConsoleMetrics(t *testing.T) {
	ctx := context.Background()
	var calls int
	ok := fakeConsole(t, map[string][]string{"kari@nav.no": {"team-a"}}, &calls)
	defer ok.Close()
	unavailable := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusServiceUnavailable) }))
	defer unavailable.Close()
	broken := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("not json")) }))
	defer broken.Close()

	for _, tc := range []struct{ url, outcome string }{{ok.URL, "ok"}, {unavailable.URL, "503"}, {broken.URL, "error"}} {
		before := testutil.ToFloat64(metrics.ConsoleLookups.WithLabelValues(tc.outcome))
		_, _ = newConsoleTeams(tc.url, tokenFile(t)).Teams(ctx, "kari@nav.no")
		if got := testutil.ToFloat64(metrics.ConsoleLookups.WithLabelValues(tc.outcome)) - before; got != 1 {
			t.Errorf("%s: %v, want 1", tc.outcome, got)
		}
	}
}

func TestSnapshotMetrics(t *testing.T) {
	srv := newServer()
	srv.store.Store(&storeBox{&fakeStore{servers: testServers(2), snapshots: map[string]*Snapshot{}}})
	metrics.SnapshotLastSuccess.WithLabelValues("routes").Set(0)
	metrics.SnapshotBytes.WithLabelValues("routes").Set(0)
	if rec := get(t, srv.routes(testAuthz(t), "", nil, nil), "/internal/v1/snapshots/routes", ""); rec.Code != http.StatusOK {
		t.Fatalf("snapshot: %d", rec.Code)
	}
	if testutil.ToFloat64(metrics.SnapshotLastSuccess.WithLabelValues("routes")) == 0 || testutil.ToFloat64(metrics.SnapshotBytes.WithLabelValues("routes")) == 0 {
		t.Error("snapshot build not recorded")
	}
}
