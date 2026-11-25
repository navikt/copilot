package api

import (
	"net/http"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/navikt/copilot/fleet-control/internal/metrics"
	"github.com/navikt/copilot/fleet-control/internal/model"
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

func TestSnapshotMetrics(t *testing.T) {
	srv := newServer()
	srv.store.Store(&storeBox{&fakeStore{servers: testServers(2), snapshots: map[string]*model.Snapshot{}}})
	metrics.SnapshotLastSuccess.WithLabelValues("routes").Set(0)
	metrics.SnapshotBytes.WithLabelValues("routes").Set(0)
	if rec := get(t, srv.routes(testAuthz(t), "", nil, nil), "/internal/v1/snapshots/routes", ""); rec.Code != http.StatusOK {
		t.Fatalf("snapshot: %d", rec.Code)
	}
	if testutil.ToFloat64(metrics.SnapshotLastSuccess.WithLabelValues("routes")) == 0 || testutil.ToFloat64(metrics.SnapshotBytes.WithLabelValues("routes")) == 0 {
		t.Error("snapshot build not recorded")
	}
}
