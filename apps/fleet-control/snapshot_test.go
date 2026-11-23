package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

func testServers(n int) []Server {
	out := make([]Server, n)
	for i := range out {
		name := fmt.Sprintf("s%d", i)
		out[i] = Server{
			Namespace: "copilot", Name: name, Status: "published",
			RegistryName: "io.github.navikt/" + name,
			Registry:     json.RawMessage(`{"name":"io.github.navikt/` + name + `"}`),
			Route:        &Route{Path: "/mcp/copilot/" + name, Target: "dev-gcp:copilot:" + name, UpstreamURL: "http://" + name + ".copilot/", Enabled: true, Tools: []string{"t"}},
		}
	}
	return out
}

func TestBuildSnapshotFailsClosed(t *testing.T) {
	for _, kind := range []string{"routes", "registry"} {
		t.Run(kind, func(t *testing.T) {
			if _, err := buildSnapshot(kind, nil, nil); !errors.Is(err, errEmptySnapshot) {
				t.Fatalf("empty: got %v", err)
			}
			all := testServers(20)
			first, err := buildSnapshot(kind, all, nil)
			if err != nil || first.Generation != 1 {
				t.Fatalf("first: %v %+v", err, first)
			}
			if same, _ := buildSnapshot(kind, all, first); same != first {
				t.Error("unchanged content must keep the generation")
			}
			// 20 entries: the limit is max(3, 4) = 4 removals.
			if next, err := buildSnapshot(kind, all[:16], first); err != nil || next.Generation != 2 {
				t.Errorf("dropping 4 of 20 must pass: %v", err)
			}
			if _, err := buildSnapshot(kind, all[:15], first); err == nil {
				t.Error("dropping 5 of 20 must be rejected")
			}
			revoked := testServers(20)
			for i := 10; i < 20; i++ {
				revoked[i].Status = "revoked"
			}
			if _, err := buildSnapshot(kind, revoked, first); err != nil {
				t.Errorf("revokes do not count against the limit: %v", err)
			}
			allRevoked := testServers(20)
			for i := range allRevoked {
				allRevoked[i].Status = "revoked"
			}
			if _, err := buildSnapshot(kind, allRevoked, first); !errors.Is(err, errEmptySnapshot) {
				t.Errorf("revoking everything still yields an empty snapshot: %v", err)
			}
		})
	}
	small, _ := buildSnapshot("routes", testServers(5), nil)
	if _, err := buildSnapshot("routes", testServers(2), small); err != nil {
		t.Errorf("small sets may drop 3: %v", err)
	}
	if _, err := buildSnapshot("routes", testServers(1), small); err == nil {
		t.Error("small sets may not drop 4")
	}
	dup := testServers(2)
	dup[1].Route.Path = dup[0].Route.Path
	if _, err := buildSnapshot("routes", dup, nil); err == nil {
		t.Error("duplicate paths must be rejected")
	}
}

func TestRegistrySnapshotIsPublicEntryOnly(t *testing.T) {
	snap, err := buildSnapshot("registry", testServers(1), nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(snap.Items); got != `[{"name":"io.github.navikt/s0"}]` {
		t.Errorf("registry snapshot leaks route data: %s", got)
	}
}

// fakeStore keeps snapshots in memory; err makes every call fail.
type fakeStore struct {
	servers   []Server
	snapshots map[string]*Snapshot
	err       error
}

func (f *fakeStore) Ping(context.Context) error { return f.err }
func (f *fakeStore) Servers(context.Context) ([]Server, error) {
	return f.servers, f.err
}
func (f *fakeStore) LatestSnapshot(_ context.Context, kind string) (*Snapshot, error) {
	return f.snapshots[kind], f.err
}
func (f *fakeStore) Publish(_ context.Context, kind string, build func(string, []Server, *Snapshot) (*Snapshot, error)) (*Snapshot, error) {
	if f.err != nil {
		return nil, f.err
	}
	next, err := build(kind, f.servers, f.snapshots[kind])
	if err == nil {
		f.snapshots[kind] = next
	}
	return next, err
}

func TestStoreIfNewerNeverGoesBack(t *testing.T) {
	var p atomic.Pointer[Snapshot]
	var wg sync.WaitGroup
	for g := int64(1); g <= 200; g++ {
		wg.Go(func() { storeIfNewer(&p, &Snapshot{Generation: g}) })
	}
	wg.Wait()
	if got := p.Load().Generation; got != 200 {
		t.Fatalf("got generation %d, want 200", got)
	}
	if got := storeIfNewer(&p, &Snapshot{Generation: 3}); got.Generation != 200 {
		t.Errorf("an older generation replaced a newer one: %d", got.Generation)
	}
}

func TestReadyUsesCachedSnapshots(t *testing.T) {
	fs := &fakeStore{servers: testServers(3), snapshots: map[string]*Snapshot{}}
	srv := newServer()
	srv.store.Store(&storeBox{fs})
	h := srv.routes(testAuthz(t), "", nil, nil)
	if rec := get(t, h, "/ready", ""); rec.Code != http.StatusOK {
		t.Fatalf("ready with database: %d %s", rec.Code, rec.Body)
	}
	fs.err = errors.New("down")
	if rec := get(t, h, "/ready", ""); rec.Code != http.StatusOK {
		t.Errorf("ready during outage with both snapshots cached: %d", rec.Code)
	}
	cold := newServer()
	cold.store.Store(&storeBox{fs})
	if rec := get(t, cold.routes(testAuthz(t), "", nil, nil), "/ready", ""); rec.Code != http.StatusServiceUnavailable {
		t.Errorf("ready during outage without cache: %d", rec.Code)
	}
}

func get(t *testing.T, h http.Handler, path, inm string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	tok := gatewayID
	if path == "/internal/v1/servers" || path == "/internal/v1/snapshots/registry" {
		tok = registryID
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	if inm != "" {
		req.Header.Set("If-None-Match", inm)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestHandlers(t *testing.T) {
	srv := newServer()
	h := srv.routes(testAuthz(t), "", nil, nil)
	for _, p := range []string{"/ready", "/internal/v1/servers", "/internal/v1/routes", "/internal/v1/snapshots/routes"} {
		if rec := get(t, h, p, ""); rec.Code != http.StatusServiceUnavailable {
			t.Errorf("%s before the database is up: got %d", p, rec.Code)
		}
	}
	if rec := get(t, h, "/health", ""); rec.Code != http.StatusOK {
		t.Errorf("health: %d", rec.Code)
	}

	fs := &fakeStore{servers: testServers(3), snapshots: map[string]*Snapshot{}}
	fs.servers[2].Status = "revoked"
	srv.store.Store(&storeBox{fs})

	rec := get(t, h, "/internal/v1/routes", "")
	if rec.Code != http.StatusOK || strings.Contains(rec.Body.String(), "s2") {
		t.Errorf("routes: %d %s", rec.Code, rec.Body)
	}
	rec = get(t, h, "/internal/v1/servers", "")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"registryName":"io.github.navikt/s1"`) || strings.Contains(rec.Body.String(), "upstream_url") {
		t.Errorf("servers: %d %s", rec.Code, rec.Body)
	}

	rec = get(t, h, "/internal/v1/snapshots/routes", "")
	var body struct {
		Generation int64            `json:"generation"`
		Routes     []map[string]any `json:"routes"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || rec.Code != http.StatusOK || body.Generation != 1 || len(body.Routes) != 2 {
		t.Fatalf("snapshot: %d %s", rec.Code, rec.Body)
	}
	etag := rec.Header().Get("ETag")
	if rec := get(t, h, "/internal/v1/snapshots/routes", etag); rec.Code != http.StatusNotModified {
		t.Errorf("If-None-Match: got %d", rec.Code)
	}
	if rec := get(t, h, "/internal/v1/snapshots/nope", ""); rec.Code != http.StatusNotFound {
		t.Errorf("unknown kind: got %d", rec.Code)
	}
	if rec := get(t, h, "/internal/v1/snapshots/registry", ""); rec.Code != http.StatusOK || strings.Contains(rec.Body.String(), "upstream_url") {
		t.Errorf("registry snapshot: %d %s", rec.Code, rec.Body)
	}

	// A database outage serves the last good snapshot, never an empty one.
	fs.err = errors.New("down")
	if rec := get(t, h, "/internal/v1/snapshots/routes", ""); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "s1") {
		t.Errorf("outage with last good: %d %s", rec.Code, rec.Body)
	}
	if rec := get(t, h, "/internal/v1/servers", ""); rec.Code != http.StatusServiceUnavailable {
		t.Errorf("servers during outage: %d", rec.Code)
	}
	fresh := newServer()
	fresh.store.Store(&storeBox{fs})
	if rec := get(t, fresh.routes(testAuthz(t), "", nil, nil), "/internal/v1/snapshots/routes", ""); rec.Code != http.StatusServiceUnavailable {
		t.Errorf("outage after restart: %d", rec.Code)
	}

	// After a restart, a rejected rebuild serves the snapshot stored in the database.
	stored := &fakeStore{servers: testServers(1), snapshots: map[string]*Snapshot{}}
	stored.snapshots["routes"], _ = buildSnapshot("routes", testServers(10), nil)
	restarted := newServer()
	restarted.store.Store(&storeBox{stored})
	if rec := get(t, restarted.routes(testAuthz(t), "", nil, nil), "/internal/v1/snapshots/routes", ""); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "s9") {
		t.Errorf("rejected rebuild after restart: %d %s", rec.Code, rec.Body)
	}
	if restarted.lastGood["routes"].Load() == nil {
		t.Error("the stored fallback must become lastGood")
	}

	// Everything revoked or deleted: the snapshot is rejected, not emptied.
	empty := newServer()
	empty.store.Store(&storeBox{&fakeStore{snapshots: map[string]*Snapshot{}}})
	if rec := get(t, empty.routes(testAuthz(t), "", nil, nil), "/internal/v1/snapshots/routes", ""); rec.Code != http.StatusServiceUnavailable {
		t.Errorf("empty snapshot: %d %s", rec.Code, rec.Body)
	}
}
