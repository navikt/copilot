package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Routes come from the Git file (PILOT_ROUTES_FILE) unless ROUTES_SOURCE is
// fleet-control (#2641). Then the gateway serves no routes from the file: MCP
// routes answer 503 and /ready fails until the first valid snapshot (#2797).
// The file only pins the primary resource.
const (
	routesSourceGit          = "git"
	routesSourceFleetControl = "fleet-control"

	fleetControlSyncInterval = 30 * time.Second
	maxSnapshotBytes         = 1 << 20
)

// fleetControlRoutes fetches the routes snapshot. It is used by one goroutine
// at a time, so etag and generation need no lock.
type fleetControlRoutes struct {
	cfg        *Config
	client     *http.Client
	etag       string
	generation int64
}

type routesSnapshot struct {
	SchemaVersion int   `json:"schema_version"`
	Generation    int64 `json:"generation"`
	Routes        []struct {
		Namespace   string   `json:"namespace"`
		Name        string   `json:"name"`
		Path        string   `json:"path"`
		Target      string   `json:"target"`
		UpstreamURL string   `json:"upstream_url"`
		Enabled     bool     `json:"enabled"`
		Tools       []string `json:"tools"`
	} `json:"routes"`
}

// m2mToken gets an app token for fleet-control from Texas.
func (f *fleetControlRoutes) m2mToken(ctx context.Context) (string, error) {
	payload, err := json.Marshal(map[string]string{"identity_provider": "entra_id", "target": f.cfg.FleetControlAudience})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, f.cfg.TexasTokenEndpoint, bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := f.client.Do(req) //nolint:gosec // endpoint validated at startup
	if err != nil {
		return "", fmt.Errorf("texas token request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("texas token request: status %d", resp.StatusCode)
	}
	var out map[string]any
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxUpstreamBodyBytes)).Decode(&out); err != nil {
		return "", errors.New("invalid texas token response")
	}
	token, _ := out["access_token"].(string)
	if token == "" {
		return "", errors.New("invalid texas token response")
	}
	return token, nil
}

// fetch returns the validated routes, or nil routes and no error when the
// snapshot is unchanged (304). Any error means: keep what you have.
func (f *fleetControlRoutes) fetch(ctx context.Context, current *RouteTable) ([]PilotRoute, error) {
	token, err := f.m2mToken(ctx)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, f.cfg.FleetControlURL+"/internal/v1/snapshots/routes", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if f.etag != "" {
		req.Header.Set("If-None-Match", f.etag)
	}
	resp, err := f.client.Do(req) //nolint:gosec // URL validated at startup
	if err != nil {
		return nil, fmt.Errorf("fleet-control request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	switch {
	case resp.StatusCode == http.StatusNotModified && f.etag != "":
		return nil, nil
	case resp.StatusCode != http.StatusOK:
		return nil, fmt.Errorf("fleet-control: status %d", resp.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxSnapshotBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read snapshot: %w", err)
	}
	if len(raw) > maxSnapshotBytes {
		return nil, errors.New("snapshot too large")
	}
	var snap routesSnapshot
	if err = json.Unmarshal(raw, &snap); err != nil {
		return nil, fmt.Errorf("parse snapshot: %w", err)
	}
	if snap.SchemaVersion != 1 {
		return nil, fmt.Errorf("unsupported snapshot schema_version %d", snap.SchemaVersion)
	}
	if snap.Generation <= 0 {
		return nil, fmt.Errorf("snapshot generation %d must be positive", snap.Generation)
	}
	if f.generation > 0 && snap.Generation == f.generation && resp.Header.Get("ETag") != f.etag {
		return nil, fmt.Errorf("snapshot generation %d changed body without a new generation", snap.Generation)
	}
	if snap.Generation < f.generation {
		return nil, fmt.Errorf("snapshot generation %d is older than %d", snap.Generation, f.generation)
	}
	routes := make([]PilotRoute, 0, len(snap.Routes))
	for _, r := range snap.Routes {
		if r.Path != "/mcp/"+r.Namespace+"/"+r.Name {
			return nil, fmt.Errorf("route %q does not match %s/%s", r.Path, r.Namespace, r.Name)
		}
		enabled := r.Enabled
		pr := PilotRoute{Route: r.Path, Team: r.Namespace, Target: r.Target, UpstreamURL: r.UpstreamURL, Enabled: &enabled, Tools: r.Tools}
		// Same rules as the Git file; passAllTools is operator-only and never comes from fleet-control.
		if err = applyToolPolicy(&pr); err != nil {
			return nil, err
		}
		routes = append(routes, pr)
	}
	routes, err = validatePilotRoutes(routes) // rejects an empty list
	if err != nil {
		return nil, err
	}
	for i := range routes {
		routes[i].FullRouteID = f.cfg.BaseURL + routes[i].Route
	}
	// The primary resource is the token audience in PRM: never move it to another route.
	if _, ok := (&RouteTable{Routes: routes}).routeByResource(current.PrimaryResource); !ok {
		return nil, fmt.Errorf("snapshot drops the primary resource %q", current.PrimaryResource)
	}
	// No shrink guard here: fleet-control applies it between its own
	// publications and exempts revocations. Comparing with this table instead
	// could reject valid snapshots forever and keep revoked routes live.
	f.etag = resp.Header.Get("ETag")
	f.generation = snap.Generation
	return routes, nil
}

// syncRoutes fetches once and swaps the route table on a new, valid snapshot.
func (g *Gateway) syncRoutes(ctx context.Context, f *fleetControlRoutes) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	routes, err := f.fetch(ctx, g.table())
	if err != nil {
		routeSyncFailures.Inc()
		slog.Warn("fleet-control routes not updated, keeping current table", "error", err, "routes", len(g.table().Routes))
		return
	}
	markRouteSynced()
	if routes == nil {
		return
	}
	rt := newRouteTable(&Config{Routes: routes})
	rt.PrimaryResource = g.table().PrimaryResource // fetch checked it is in routes
	g.routeTable.Store(rt)
	routesLoaded.Set(float64(len(routes)))
	initMetrics(routes) // idempotent; pre-creates series for new routes and tools
	slog.Info("route table loaded from fleet-control", "routes", len(routes), "generation", f.generation)
}

// awaitingSnapshot reports fleet-control mode before the first valid snapshot.
// A valid snapshot is never empty, so an empty table means none has arrived.
func (g *Gateway) awaitingSnapshot() bool {
	return g.cfg.RoutesSource == routesSourceFleetControl && len(g.table().Routes) == 0
}

// startRouteSync syncs at startup and then every interval until ctx ends.
func (g *Gateway) startRouteSync(ctx context.Context, interval time.Duration) {
	if g.cfg.RoutesSource != routesSourceFleetControl {
		return
	}
	f := &fleetControlRoutes{cfg: g.cfg, client: g.cfg.HTTPClient}
	g.syncRoutes(ctx, f)
	go func() {
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				g.syncRoutes(ctx, f)
			}
		}
	}()
}

var (
	lastRouteSync     atomic.Int64 // unix nanoseconds of the last successful fetch, 0 before the first
	routeSyncAgeGauge sync.Once
)

// markRouteSynced records a successful fetch (new or unchanged snapshot). The
// age gauge is registered on the first one, so no series exists before it (#2637).
func markRouteSynced() {
	lastRouteSync.Store(time.Now().UnixNano())
	routeSyncAgeGauge.Do(func() {
		promauto.NewGaugeFunc(prometheus.GaugeOpts{
			Name: "fleet_gateway_route_snapshot_age_seconds",
			Help: "Seconds since fleet-gateway last fetched a valid or unchanged fleet-control route snapshot. Absent until the first one.",
		}, func() float64 { return time.Since(time.Unix(0, lastRouteSync.Load())).Seconds() })
	})
}
