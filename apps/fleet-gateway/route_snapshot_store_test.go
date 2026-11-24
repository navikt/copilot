package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

// restartGateway builds a new pod on gw's store, as a restart would (#2892).
func restartGateway(t *testing.T, gw *Gateway) *Gateway {
	t.Helper()
	cfg := *gw.cfg
	cfg.Store = gw.store
	gw2, err := newGateway(&cfg, gw.now)
	if err != nil {
		t.Fatal(err)
	}
	return gw2
}

func startSync(t *testing.T, gw *Gateway) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	gw.startRouteSync(ctx, time.Hour)
}

// storeLiveSnapshot runs one live sync so the gateway stores the snapshot.
func storeLiveSnapshot(t *testing.T, f *fakeFleetControl, body map[string]any) *Gateway {
	t.Helper()
	f.body = body
	gw, src := fleetControlGateway(t, f)
	gw.syncRoutes(context.Background(), src)
	if gw.awaitingSnapshot() {
		t.Fatal("live sync accepted no snapshot")
	}
	return gw
}

func TestStoredSnapshotServesWhenFleetControlDown(t *testing.T) {
	f := newFakeFleetControl(t)
	gw := storeLiveSnapshot(t, f, snapBody(4, primaryRoute(), snapRoute("alpha")))
	f.status = http.StatusServiceUnavailable

	gw2 := restartGateway(t, gw)
	startSync(t, gw2)
	if got := routePaths(gw2); len(got) != 2 || got[1] != "/mcp/copilot/alpha" {
		t.Fatalf("routes after start from store = %v", got)
	}
	if gw2.awaitingSnapshot() {
		t.Fatal("/ready would answer 503 with a stored snapshot")
	}
	if v := testutil.ToFloat64(routesFromStore); v != 1 {
		t.Fatalf("routes_from_stored_snapshot = %v, want 1", v)
	}
}

func TestStoredSnapshotAllowedRegardlessOfAge(t *testing.T) {
	f := newFakeFleetControl(t)
	body := snapBody(4, primaryRoute(), snapRoute("alpha"))
	body["checked_at"] = time.Now().Add(-time.Minute).UTC().Format(time.RFC3339Nano)
	gw := storeLiveSnapshot(t, f, body)
	f.status = http.StatusServiceUnavailable

	// Age the stored row past maxSnapshotAge, keeping it self-consistent.
	raw, _, _ := gw.store.Get(context.Background(), kindRouteSnapshot, routeSnapshotKey)
	var st storedSnapshot
	if err := json.Unmarshal(raw, &st); err != nil {
		t.Fatal(err)
	}
	var snap map[string]any
	_ = json.Unmarshal(st.Body, &snap)
	st.CheckedAt = time.Now().Add(-48 * time.Hour).UTC()
	snap["checked_at"] = st.CheckedAt.Format(time.RFC3339Nano)
	st.Body, _ = json.Marshal(snap)
	raw, _ = json.Marshal(st)
	_ = gw.store.Put(context.Background(), kindRouteSnapshot, routeSnapshotKey, raw, time.Hour)

	gw2 := restartGateway(t, gw)
	startSync(t, gw2)
	if gw2.awaitingSnapshot() {
		t.Fatal("a 48h old stored snapshot was refused; age is alert-only")
	}
}

func TestNoLiveNoStoredFailsClosed(t *testing.T) {
	f := newFakeFleetControl(t)
	f.status = http.StatusServiceUnavailable
	gw, _ := fleetControlGateway(t, f)
	startSync(t, gw)
	if !gw.awaitingSnapshot() {
		t.Fatalf("serving routes with neither live nor stored snapshot: %v", routePaths(gw))
	}
}

func TestStoredOlderGenerationDoesNotOverrideLive(t *testing.T) {
	f := newFakeFleetControl(t)
	gw := storeLiveSnapshot(t, f, snapBody(3, primaryRoute(), snapRoute("old")))

	// fleet-control is back with a newer generation.
	f.body = snapBody(5, primaryRoute(), snapRoute("new"))
	gw2 := restartGateway(t, gw)
	startSync(t, gw2)
	if got := routePaths(gw2); len(got) != 2 || got[1] != "/mcp/copilot/new" {
		t.Fatalf("routes = %v, want the live generation 5", got)
	}
	if v := testutil.ToFloat64(routesFromStore); v != 0 {
		t.Fatalf("routes_from_stored_snapshot = %v, want 0", v)
	}
}

func TestLiveSnapshotReplacesStoredOne(t *testing.T) {
	f := newFakeFleetControl(t)
	gw := storeLiveSnapshot(t, f, snapBody(9, primaryRoute(), snapRoute("stored")))
	f.status = http.StatusServiceUnavailable
	gw2 := restartGateway(t, gw)
	startSync(t, gw2)

	// A live snapshot with a lower generation than the stored one still wins:
	// the stored row must never pin the generation (a DB writer could block updates).
	f.status = http.StatusOK
	f.body = snapBody(2, primaryRoute(), snapRoute("live"))
	gw2.syncRoutes(context.Background(), &fleetControlRoutes{cfg: gw2.cfg, client: f.srv.Client()})
	if got := routePaths(gw2); len(got) != 2 || got[1] != "/mcp/copilot/live" {
		t.Fatalf("routes = %v, want the live snapshot", got)
	}
}

func TestCorruptStoredSnapshotFailsClosed(t *testing.T) {
	cases := map[string]func(st *storedSnapshot){
		"garbage body":        func(st *storedSnapshot) { st.Body = []byte("{not json") },
		"hash mismatch":       func(st *storedSnapshot) { st.Hash = "00" },
		"generation mismatch": func(st *storedSnapshot) { st.Generation++ },
		"future checked_at": func(st *storedSnapshot) {
			var snap map[string]any
			_ = json.Unmarshal(st.Body, &snap)
			st.CheckedAt = time.Now().Add(time.Hour).UTC()
			snap["checked_at"] = st.CheckedAt.Format(time.RFC3339Nano)
			st.Body, _ = json.Marshal(snap)
		},
		"no checked_at": func(st *storedSnapshot) {
			var snap map[string]any
			_ = json.Unmarshal(st.Body, &snap)
			delete(snap, "checked_at")
			st.Body, _ = json.Marshal(snap)
			st.CheckedAt = time.Time{}
		},
		"invalid route": func(st *storedSnapshot) {
			var snap map[string]any
			_ = json.Unmarshal(st.Body, &snap)
			r := snapRoute("evil")
			r["path"] = "/mcp/other/evil"
			snap["routes"] = append(snap["routes"].([]any), r)
			st.Body, _ = json.Marshal(snap)
			// Keep the row self-consistent so only route validation can refuse it.
			var parsed routesSnapshot
			_ = json.Unmarshal(st.Body, &parsed)
			routesJSON, _ := json.Marshal(parsed.Routes)
			h := sha256.Sum256(routesJSON)
			st.Hash = hex.EncodeToString(h[:])
		},
	}
	for name, corrupt := range cases {
		t.Run(name, func(t *testing.T) {
			f := newFakeFleetControl(t)
			gw := storeLiveSnapshot(t, f, snapBody(4, primaryRoute(), snapRoute("alpha")))
			f.status = http.StatusServiceUnavailable
			raw, _, _ := gw.store.Get(context.Background(), kindRouteSnapshot, routeSnapshotKey)
			var st storedSnapshot
			if err := json.Unmarshal(raw, &st); err != nil {
				t.Fatal(err)
			}
			corrupt(&st)
			raw, _ = json.Marshal(st)
			_ = gw.store.Put(context.Background(), kindRouteSnapshot, routeSnapshotKey, raw, time.Hour)

			gw2 := restartGateway(t, gw)
			startSync(t, gw2)
			if !gw2.awaitingSnapshot() {
				t.Fatalf("corrupt stored snapshot served: %v", routePaths(gw2))
			}
		})
	}
}

func TestRejectedLiveSnapshotIsNotStored(t *testing.T) {
	f := newFakeFleetControl(t)
	gw := storeLiveSnapshot(t, f, snapBody(4, primaryRoute(), snapRoute("alpha")))
	before, _, _ := gw.store.Get(context.Background(), kindRouteSnapshot, routeSnapshotKey)
	bad := snapRoute("evil")
	bad["path"] = "/mcp/other/evil"
	f.body = snapBody(5, primaryRoute(), bad)
	f.etag = `"e2"`
	gw.syncRoutes(context.Background(), &fleetControlRoutes{cfg: gw.cfg, client: f.srv.Client()})
	after, _, _ := gw.store.Get(context.Background(), kindRouteSnapshot, routeSnapshotKey)
	if string(after) != string(before) {
		t.Fatal("a rejected live snapshot replaced the stored one")
	}
}

func TestOlderSnapshotDoesNotOverwriteNewerStored(t *testing.T) {
	f := newFakeFleetControl(t)
	gw, _ := fleetControlGateway(t, f)
	checkNewerWins(t, f, gw)
}

// Prod seals every store value; SealedStore refuses CompareAndSwap on sealed kinds.
func TestRouteSnapshotThroughSealedStore(t *testing.T) {
	f := newFakeFleetControl(t)
	gw, _ := fleetControlGateway(t, f)
	kr, err := parseKeyRing("k1:" + testKeyBase64(t, false))
	if err != nil {
		t.Fatal(err)
	}
	gw.store = newSealedStore(newMemoryStore(gw.now, nil), kr)
	checkNewerWins(t, f, gw)

	f.status = http.StatusServiceUnavailable
	gw2 := restartGateway(t, gw)
	startSync(t, gw2)
	if got := routePaths(gw2); len(got) != 2 || got[1] != "/mcp/copilot/reset" {
		t.Fatalf("routes from sealed stored snapshot = %v", got)
	}
}

func checkNewerWins(t *testing.T, f *fakeFleetControl, gw *Gateway) {
	t.Helper()
	newer := snapBody(2, primaryRoute(), snapRoute("newer"))
	newer["checked_at"] = time.Now().UTC().Format(time.RFC3339Nano)
	f.body = newer
	gw.syncRoutes(context.Background(), &fleetControlRoutes{cfg: gw.cfg, client: f.srv.Client()})
	stored, ok, _ := gw.store.Get(context.Background(), kindRouteSnapshot, routeSnapshotKey)
	if !ok {
		t.Fatal("accepted live snapshot was not stored")
	}

	// A lagging replica on the same store accepts an older checked_at with a higher generation.
	older := snapBody(7, primaryRoute(), snapRoute("older"))
	older["checked_at"] = time.Now().Add(-time.Minute).UTC().Format(time.RFC3339Nano)
	f.body = older
	f.etag = `"e2"`
	gw.syncRoutes(context.Background(), &fleetControlRoutes{cfg: gw.cfg, client: f.srv.Client()})
	if after, _, _ := gw.store.Get(context.Background(), kindRouteSnapshot, routeSnapshotKey); string(after) != string(stored) {
		t.Fatal("an older checked_at overwrote the newer stored snapshot")
	}

	// A newer checked_at with a lower generation (restored fleet-control) replaces it.
	reset := snapBody(1, primaryRoute(), snapRoute("reset"))
	f.body = reset
	f.etag = `"e3"`
	gw.syncRoutes(context.Background(), &fleetControlRoutes{cfg: gw.cfg, client: f.srv.Client()})
	after, _, _ := gw.store.Get(context.Background(), kindRouteSnapshot, routeSnapshotKey)
	var st storedSnapshot
	_ = json.Unmarshal(after, &st)
	if st.Generation != 1 {
		t.Fatalf("stored generation = %d, want 1 after a generation reset", st.Generation)
	}
}

func TestUnreadableStoredSnapshotIsReplaced(t *testing.T) {
	f := newFakeFleetControl(t)
	gw, _ := fleetControlGateway(t, f)
	kr, err := parseKeyRing("k1:" + testKeyBase64(t, false))
	if err != nil {
		t.Fatal(err)
	}
	other, err := parseKeyRing("k2:" + testKeyBase64(t, true))
	if err != nil {
		t.Fatal(err)
	}
	inner := newMemoryStore(gw.now, nil)
	// A row sealed with a key this pod does not hold.
	if err = newSealedStore(inner, other).Put(context.Background(), kindRouteSnapshot, routeSnapshotKey, []byte(`{"checked_at":"2999-01-01T00:00:00Z"}`), time.Hour); err != nil {
		t.Fatal(err)
	}
	gw.store = newSealedStore(inner, kr)
	if _, _, err = gw.store.Get(context.Background(), kindRouteSnapshot, routeSnapshotKey); err == nil {
		t.Fatal("seeded row is readable; the test proves nothing")
	}
	gw.syncRoutes(context.Background(), &fleetControlRoutes{cfg: gw.cfg, client: f.srv.Client()})
	raw, ok, err := gw.store.Get(context.Background(), kindRouteSnapshot, routeSnapshotKey)
	if err != nil || !ok {
		t.Fatalf("new snapshot not stored over an unreadable row: ok=%v err=%v", ok, err)
	}
	var st storedSnapshot
	if err = json.Unmarshal(raw, &st); err != nil || st.Generation != 1 {
		t.Fatalf("stored = %s, err %v", raw, err)
	}
}
