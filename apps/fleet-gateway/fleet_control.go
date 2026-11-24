package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
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
// routes answer 503 and /ready fails until the first valid snapshot (#2797),
// live or stored (#2892).
// The file only pins the primary resource.
const (
	routesSourceGit          = "git"
	routesSourceFleetControl = "fleet-control"

	fleetControlSyncInterval = 30 * time.Second
	maxSnapshotBytes         = 1 << 20
	// maxSnapshotAge bounds checked_at: a snapshot fleet-control has not
	// confirmed against its database for this long may be stale (#2849).
	maxSnapshotAge = 10 * time.Minute
)

// fleetControlRoutes fetches the routes snapshot. It is used by one goroutine
// at a time, so etag and generation need no lock.
type fleetControlRoutes struct {
	cfg        *Config
	client     *http.Client
	etag       string
	generation int64
	routesHash [sha256.Size]byte // of the accepted generation's routes
	accepted   []byte            // body of the last accepted snapshot, persisted by syncRoutes
	checkedAt  time.Time
}

type routesSnapshot struct {
	SchemaVersion int       `json:"schema_version"`
	Generation    int64     `json:"generation"`
	CheckedAt     time.Time `json:"checked_at"`
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
		// A replica serving a stale copy answers 304 too; refuse it so the age gauge keeps rising.
		checked, perr := time.Parse(time.RFC3339Nano, resp.Header.Get("Fleet-Checked-At"))
		if perr != nil {
			return nil, errors.New("304 without a valid Fleet-Checked-At header")
		}
		if err = checkFresh(checked); err != nil {
			return nil, err
		}
		return nil, nil
	case resp.StatusCode != http.StatusOK:
		return nil, fmt.Errorf("fleet-control: status %d", resp.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxSnapshotBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read snapshot: %w", err)
	}
	snap, routes, hash, err := f.parseSnapshot(raw, current, true)
	if err != nil {
		return nil, err
	}
	if f.generation > 0 && snap.Generation == f.generation && hash != f.routesHash {
		return nil, fmt.Errorf("snapshot generation %d changed body without a new generation", snap.Generation)
	}
	if snap.Generation < f.generation {
		return nil, fmt.Errorf("snapshot generation %d is older than %d", snap.Generation, f.generation)
	}
	// No shrink guard here: fleet-control applies it between its own
	// publications and exempts revocations. Comparing with this table instead
	// could reject valid snapshots forever and keep revoked routes live.
	f.etag = resp.Header.Get("ETag")
	f.generation = snap.Generation
	f.routesHash = hash
	f.accepted = raw
	f.checkedAt = snap.CheckedAt
	return routes, nil
}

// parseSnapshot validates a snapshot body, live or stored. checkAge applies
// the past bound of maxSnapshotAge; a stored snapshot skips it (#2892).
func (f *fleetControlRoutes) parseSnapshot(raw []byte, current *RouteTable, checkAge bool) (routesSnapshot, []PilotRoute, [sha256.Size]byte, error) {
	var snap routesSnapshot
	var hash [sha256.Size]byte
	if len(raw) > maxSnapshotBytes {
		return snap, nil, hash, errors.New("snapshot too large")
	}
	if err := json.Unmarshal(raw, &snap); err != nil {
		return snap, nil, hash, fmt.Errorf("parse snapshot: %w", err)
	}
	if snap.SchemaVersion != 1 {
		return snap, nil, hash, fmt.Errorf("unsupported snapshot schema_version %d", snap.SchemaVersion)
	}
	if snap.Generation <= 0 {
		return snap, nil, hash, fmt.Errorf("snapshot generation %d must be positive", snap.Generation)
	}
	// A missing checked_at parses as zero and is refused here, so an old
	// fleet-control fails closed at startup and static afterwards.
	if snap.CheckedAt.IsZero() {
		return snap, nil, hash, errors.New("snapshot has no checked_at")
	}
	if err := checkFresh(snap.CheckedAt); err != nil && (checkAge || snap.CheckedAt.After(time.Now())) {
		return snap, nil, hash, err
	}
	routesJSON, err := json.Marshal(snap.Routes)
	if err != nil {
		return snap, nil, hash, err
	}
	hash = sha256.Sum256(routesJSON)
	routes, err := snapshotRoutes(snap)
	if err != nil {
		return snap, nil, hash, err
	}
	for i := range routes {
		routes[i].FullRouteID = f.cfg.BaseURL + routes[i].Route
	}
	// The primary resource is the token audience in PRM: never move it to another route.
	if _, ok := (&RouteTable{Routes: routes}).routeByResource(current.PrimaryResource); !ok {
		return snap, nil, hash, fmt.Errorf("snapshot drops the primary resource %q", current.PrimaryResource)
	}
	return snap, routes, hash, nil
}

// checkFresh refuses a checked_at more than maxSnapshotAge in the past or the future.
func checkFresh(checked time.Time) error {
	if age := time.Since(checked); age > maxSnapshotAge || age < -maxSnapshotAge {
		return fmt.Errorf("snapshot checked_at %s is more than %s from now", checked.Format(time.RFC3339), maxSnapshotAge)
	}
	return nil
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
	g.swapRoutes(routes)
	routesFromStore.Set(0)
	slog.Info("route table loaded from fleet-control", "routes", len(routes), "generation", f.generation)
	g.storeSnapshot(ctx, f)
}

func (g *Gateway) swapRoutes(routes []PilotRoute) {
	rt := newRouteTable(&Config{Routes: routes})
	rt.PrimaryResource = g.table().PrimaryResource // parseSnapshot checked it is in routes
	g.routeTable.Store(rt)
	routesLoaded.Set(float64(len(routes)))
	initMetrics(routes) // idempotent; pre-creates series for new routes and tools
}

// storedSnapshot is the last accepted live snapshot, kept in the gateway's
// own store so a pod can start while fleet-control is down (#2892). Body is
// the response exactly as received; the other fields must agree with it.
type storedSnapshot struct {
	Body       []byte    `json:"body"`
	Generation int64     `json:"generation"`
	Hash       string    `json:"hash"` // hex sha256 of the routes, as in fleetControlRoutes.routesHash
	CheckedAt  time.Time `json:"checked_at"`
}

const (
	kindRouteSnapshot = "route_snapshot"
	routeSnapshotKey  = "fleet-control"
	// Age is alert-only (#2826), so the row must outlive any outage.
	routeSnapshotTTL = 10 * 365 * 24 * time.Hour
)

// storeSnapshot persists the snapshot fetch just accepted. Failure only logs:
// the live table is already in place.
func (g *Gateway) storeSnapshot(ctx context.Context, f *fleetControlRoutes) {
	raw, err := json.Marshal(storedSnapshot{Body: f.accepted, Generation: f.generation, Hash: hex.EncodeToString(f.routesHash[:]), CheckedAt: f.checkedAt})
	// The newer checked_at wins. Not generation, which can reset when
	// fleet-control's database is restored. The row stays sealed (integrity
	// against DB writers), and SealedStore has no CompareAndSwap for sealed kinds.
	// shortcut: two replicas can race between Get and Put and store the older snapshot; acceptable, the next accepted live snapshot overwrites it.
	if err == nil {
		var old []byte
		var ok bool
		// A row that cannot be read (corrupt, other key) counts as absent, or it would block storage until its TTL.
		if old, ok, err = g.store.Get(ctx, kindRouteSnapshot, routeSnapshotKey); err != nil {
			slog.Warn("stored route snapshot unreadable, replacing it", "error", err)
			ok = false
		}
		var cur storedSnapshot
		if ok && json.Unmarshal(old, &cur) == nil && cur.CheckedAt.After(f.checkedAt) {
			return
		}
		if err = g.store.Put(ctx, kindRouteSnapshot, routeSnapshotKey, raw, routeSnapshotTTL); err == nil {
			return
		}
	}
	slog.Warn("could not store route snapshot", "error", err)
}

// loadStoredSnapshot serves the stored snapshot when startup found no live
// one. It runs the live validation except the age bound, and leaves f's
// generation, hash and etag alone, so the next live snapshot always wins.
func (g *Gateway) loadStoredSnapshot(ctx context.Context, f *fleetControlRoutes) {
	raw, ok, err := g.store.Get(ctx, kindRouteSnapshot, routeSnapshotKey)
	if err != nil || !ok {
		slog.Warn("no stored route snapshot to start from", "error", err)
		return
	}
	var st storedSnapshot
	if err = json.Unmarshal(raw, &st); err != nil {
		slog.Error("stored route snapshot rejected", "error", err)
		return
	}
	snap, routes, hash, err := f.parseSnapshot(st.Body, g.table(), false)
	if err == nil && (snap.Generation != st.Generation || hex.EncodeToString(hash[:]) != st.Hash || !snap.CheckedAt.Equal(st.CheckedAt)) {
		err = errors.New("stored generation, hash or checked_at does not match the body")
	}
	if err != nil {
		slog.Error("stored route snapshot rejected", "error", err)
		return
	}
	g.swapRoutes(routes)
	routesFromStore.Set(1)
	slog.Warn("route table loaded from stored snapshot, fleet-control unreachable", "routes", len(routes), "generation", snap.Generation, "age", time.Since(snap.CheckedAt).Round(time.Second).String())
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
	if g.awaitingSnapshot() {
		g.loadStoredSnapshot(ctx, f)
	}
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

// snapshotRoutes validates snapshot routes by the same rules as the Git file.
func snapshotRoutes(snap routesSnapshot) ([]PilotRoute, error) {
	routes := make([]PilotRoute, 0, len(snap.Routes))
	for _, r := range snap.Routes {
		if r.Path != "/mcp/"+r.Namespace+"/"+r.Name {
			return nil, fmt.Errorf("route %q does not match %s/%s", r.Path, r.Namespace, r.Name)
		}
		enabled := r.Enabled
		pr := PilotRoute{Route: r.Path, Team: r.Namespace, Target: r.Target, UpstreamURL: r.UpstreamURL, Enabled: &enabled, Tools: r.Tools}
		// Same rules as the Git file; passAllTools is operator-only and never comes from fleet-control.
		if err := applyToolPolicy(&pr); err != nil {
			return nil, err
		}
		routes = append(routes, pr)
	}
	return validatePilotRoutes(routes) // rejects an empty list
}
