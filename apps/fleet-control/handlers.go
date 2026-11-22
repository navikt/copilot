package main

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
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

func (s *server) ready(w http.ResponseWriter, r *http.Request) {
	st, ctx, cancel := s.db(w, r)
	if st == nil {
		return
	}
	defer cancel()
	if err := st.Ping(ctx); err != nil {
		http.Error(w, "database unavailable", http.StatusServiceUnavailable)
		return
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
// or rejected snapshot: a failure returns the last good one from memory, or
// 503, and consumers keep what they have.
func (s *server) snapshot(w http.ResponseWriter, r *http.Request) {
	kind := r.PathValue("kind")
	last, ok := s.lastGood[kind]
	if !ok {
		http.NotFound(w, r)
		return
	}
	snap, err := s.currentSnapshot(r.Context(), kind)
	if err != nil {
		slog.Error("snapshot unavailable", "kind", kind, "error", err)
		if snap = last.Load(); snap == nil {
			snap = s.storedSnapshot(r.Context(), kind)
		}
		if snap == nil {
			http.Error(w, "snapshot unavailable", http.StatusServiceUnavailable)
			return
		}
	} else {
		last.Store(snap)
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

// storedSnapshot returns the last good snapshot persisted in the database,
// used after a restart when a guard rejects the rebuilt one.
func (s *server) storedSnapshot(ctx context.Context, kind string) *Snapshot {
	box := s.store.Load()
	if box == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	snap, err := box.LatestSnapshot(ctx, kind)
	if err != nil {
		return nil
	}
	return snap
}

func (s *server) currentSnapshot(ctx context.Context, kind string) (*Snapshot, error) {
	box := s.store.Load()
	if box == nil {
		return nil, errDBUnavailable
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	servers, err := box.Servers(ctx)
	if err != nil {
		return nil, err
	}
	prev, err := box.LatestSnapshot(ctx, kind)
	if err != nil {
		return nil, err
	}
	next, err := buildSnapshot(kind, servers, prev)
	if err != nil || next == prev {
		return next, err
	}
	if err := box.InsertSnapshot(ctx, next); err != nil {
		return nil, err
	}
	// Reread: another replica may have stored this generation first.
	return box.LatestSnapshot(ctx, kind)
}

var errDBUnavailable = errors.New("database unavailable")

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(v)
}
