package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/navikt/copilot/fleet-control/internal/auth"
	"github.com/navikt/copilot/fleet-control/internal/metrics"
	"github.com/navikt/copilot/fleet-control/internal/model"
)

// routes registers the API. The app has no ingress: accessPolicy.inbound
// admits only its callers, and each /internal route also needs that caller's
// Entra app token (#2623). The /admin write routes exist only when
// adminClientID is set, and need a person's token obtained by that client.
//
// The /runtime routes exist only when ra is set (FLEET_RUNTIME_CLIENT_IDS),
// and need an Entra app token from an allowlisted fleet-runtime client.
func (s *server) routes(a *auth.Authz, adminClientID string, teams auth.TeamDirectory, ra *runtimeAuth) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("GET /ready", s.ready)
	mux.Handle("GET /metrics", promhttp.Handler())
	mux.HandleFunc("GET /internal/v1/snapshots/routes", a.Require(s.snapshotOf("routes"), auth.CallerGateway))
	mux.HandleFunc("GET /internal/v1/snapshots/registry", a.Require(s.snapshotOf("registry"), auth.CallerRegistry))
	if adminClientID != "" {
		// The only /admin/v1 route left (#2895): everything else is under /apis.
		mux.HandleFunc("PUT /admin/v1/jobs/{namespace}/kill-switch", s.admin(a, adminClientID, teams, putKillSwitch(s.operatorTeam)))
	}
	s.apisRoutes(mux, a, adminClientID, teams)

	if ra != nil {
		mux.HandleFunc("POST /runtime/v1/claim", s.runtimeRoute(ra, ra.claim))
		mux.HandleFunc("POST /runtime/v1/heartbeat", s.runtimeRoute(ra, ra.heartbeat))
		mux.HandleFunc("POST /runtime/v1/complete", s.runtimeRoute(ra, finish(true)))
		mux.HandleFunc("POST /runtime/v1/fail", s.runtimeRoute(ra, finish(false)))
	}
	return metrics.Instrument(mux)
}

// ready is true when both snapshot kinds can be served, from memory or the
// database, so a database outage does not pull pods that hold a good cache.
func (s *server) ready(w http.ResponseWriter, r *http.Request) {
	for kind := range s.lastGood {
		if s.lastGood[kind].Load() != nil {
			continue
		}
		if _, err := s.resolve(r.Context(), kind); err != nil {
			http.Error(w, kind+" snapshot unavailable", http.StatusServiceUnavailable)
			return
		}
	}
	w.WriteHeader(http.StatusOK)
	if s.teamsUnbound {
		_, _ = w.Write([]byte("team lookups unavailable: no Console service account bound\n"))
	}
}

// snapshot serves the current generation of a kind. It never serves an empty
// or rejected snapshot: a failure returns the last good one, or 503, and
// consumers keep what they have.
func (s *server) snapshotOf(kind string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) { s.snapshot(w, r, kind) }
}

func (s *server) snapshot(w http.ResponseWriter, r *http.Request, kind string) {
	snap, err := s.resolve(r.Context(), kind)
	if err != nil {
		http.Error(w, "snapshot unavailable", http.StatusServiceUnavailable)
		return
	}
	etag := `"` + snap.ETag + `"`
	w.Header().Set("ETag", etag)
	// Also on 304, so a gateway sees a stale replica without a body.
	w.Header().Set("Fleet-Checked-At", snap.CheckedAt.UTC().Format(time.RFC3339Nano))
	if strings.Contains(r.Header.Get("If-None-Match"), etag) {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	field := "servers"
	if kind == "routes" {
		field = "routes"
	}
	// checked_at lets a gateway without local state refuse a stale snapshot
	// from a replica that lost its database (#2849).
	writeJSON(w, map[string]any{"schema_version": 1, "generation": snap.Generation, "checked_at": snap.CheckedAt.UTC().Format(time.RFC3339Nano), field: snap.Items})
}

// resolve builds the current snapshot; on failure it falls back to the last
// good one in memory, then to the one stored in the database. What it serves
// becomes lastGood, which never moves to a lower generation.
func (s *server) resolve(ctx context.Context, kind string) (*model.Snapshot, error) {
	last := s.lastGood[kind]
	box := s.store.Load()
	if box == nil {
		if snap := last.Load(); snap != nil {
			return snap, nil
		}
		return nil, errDBUnavailable
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	start := time.Now()
	snap, err := box.Publish(ctx, kind, buildSnapshot)
	if err == nil {
		metrics.SnapshotBuildDuration.WithLabelValues(kind).Observe(time.Since(start).Seconds())
		metrics.SnapshotBytes.WithLabelValues(kind).Set(float64(len(snap.Items)))
		metrics.SnapshotLastSuccess.WithLabelValues(kind).SetToCurrentTime()
	}
	if err != nil {
		slog.Error("snapshot rebuild failed", "kind", kind, "error", err)
		// The stored snapshot is still the newest, so it counts as checked.
		// Only without the database does lastGood serve, and its checked_at ages.
		stored, lerr := box.LatestSnapshot(ctx, kind)
		if lerr != nil || stored == nil {
			if snap = last.Load(); snap == nil {
				return nil, err
			}
			return storeIfNewer(last, snap), nil
		}
		snap = stored
	}
	checked := *snap // never mutate a snapshot another request may hold
	checked.CheckedAt = time.Now()
	return storeIfNewer(last, &checked), nil
}

// storeIfNewer sets p to snap unless p already holds a higher generation, or
// the same one checked no earlier, and returns what p holds afterwards.
func storeIfNewer(p *atomic.Pointer[model.Snapshot], snap *model.Snapshot) *model.Snapshot {
	for {
		cur := p.Load()
		if cur != nil && (cur.Generation > snap.Generation || cur.Generation == snap.Generation && !snap.CheckedAt.After(cur.CheckedAt)) {
			return cur
		}
		if p.CompareAndSwap(cur, snap) {
			return snap
		}
	}
}

var errDBUnavailable = errors.New("database unavailable")

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(v)
}
