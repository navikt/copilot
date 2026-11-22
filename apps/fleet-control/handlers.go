package main

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"sync/atomic"
	"time"
)

// routes registers the API. Nothing here writes, and the app has no ingress:
// accessPolicy.inbound admits only fleet-gateway and mcp-registry.
func (s *server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("GET /ready", s.ready)
	mux.HandleFunc("GET /internal/v1/servers", s.listServers)
	mux.HandleFunc("GET /internal/v1/routes", s.listRoutes)
	mux.HandleFunc("GET /internal/v1/snapshots/{kind}", s.snapshot)
	return mux
}

func (s *server) db(w http.ResponseWriter, r *http.Request) (store, context.Context, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	box := s.store.Load()
	if box == nil {
		http.Error(w, "database unavailable", http.StatusServiceUnavailable)
		cancel()
		return nil, nil, nil
	}
	return box.store, ctx, cancel
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
}

// published loads non-revoked servers; on error it answers 503.
func (s *server) published(w http.ResponseWriter, r *http.Request) ([]Server, bool) {
	st, ctx, cancel := s.db(w, r)
	if st == nil {
		return nil, false
	}
	defer cancel()
	all, err := st.Servers(ctx)
	if err != nil {
		slog.Error("read servers", "error", err)
		http.Error(w, "database unavailable", http.StatusServiceUnavailable)
		return nil, false
	}
	out := make([]Server, 0, len(all))
	for _, sv := range all {
		if sv.Status == "published" {
			out = append(out, sv)
		}
	}
	return out, true
}

func (s *server) listServers(w http.ResponseWriter, r *http.Request) {
	servers, ok := s.published(w, r)
	if ok {
		writeJSON(w, map[string]any{"servers": servers})
	}
}

func (s *server) listRoutes(w http.ResponseWriter, r *http.Request) {
	servers, ok := s.published(w, r)
	if !ok {
		return
	}
	routes := []routeItem{}
	for _, sv := range servers {
		if sv.Route != nil {
			routes = append(routes, routeItem{sv.Namespace, sv.Name, *sv.Route})
		}
	}
	writeJSON(w, map[string]any{"routes": routes})
}

// snapshot serves the current generation of a kind. It never serves an empty
// or rejected snapshot: a failure returns the last good one, or 503, and
// consumers keep what they have.
func (s *server) snapshot(w http.ResponseWriter, r *http.Request) {
	kind := r.PathValue("kind")
	if _, ok := s.lastGood[kind]; !ok {
		http.NotFound(w, r)
		return
	}
	snap, err := s.resolve(r.Context(), kind)
	if err != nil {
		http.Error(w, "snapshot unavailable", http.StatusServiceUnavailable)
		return
	}
	etag := `"` + snap.ETag + `"`
	w.Header().Set("ETag", etag)
	if strings.Contains(r.Header.Get("If-None-Match"), etag) {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	field := "servers"
	if kind == "routes" {
		field = "routes"
	}
	writeJSON(w, map[string]any{"schema_version": 1, "generation": snap.Generation, field: snap.Items})
}

// resolve builds the current snapshot; on failure it falls back to the last
// good one in memory, then to the one stored in the database. What it serves
// becomes lastGood, which never moves to a lower generation.
func (s *server) resolve(ctx context.Context, kind string) (*Snapshot, error) {
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
	snap, err := box.Publish(ctx, kind, buildSnapshot)
	if err != nil {
		slog.Error("snapshot rebuild failed", "kind", kind, "error", err)
		if snap = last.Load(); snap == nil {
			stored, lerr := box.LatestSnapshot(ctx, kind)
			if lerr != nil || stored == nil {
				return nil, err
			}
			snap = stored
		}
	}
	return storeIfNewer(last, snap), nil
}

// storeIfNewer sets p to snap unless p already holds a generation at least as
// high, and returns what p holds afterwards.
func storeIfNewer(p *atomic.Pointer[Snapshot], snap *Snapshot) *Snapshot {
	for {
		cur := p.Load()
		if cur != nil && cur.Generation >= snap.Generation {
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
