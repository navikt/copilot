package main

import (
	"encoding/json"
	"os"
	"slices"
	"testing"

	"github.com/navikt/copilot/fleet-gateway/internal/routing"
)

func loadSnapshotRoutes(t *testing.T, path string) []routing.Route {
	t.Helper()
	raw, err := os.ReadFile(path) //nolint:gosec // operator-supplied test input
	if err != nil {
		t.Fatal(err)
	}
	var snap routesSnapshot
	if err = json.Unmarshal(raw, &snap); err != nil {
		t.Fatalf("parse snapshot: %v", err)
	}
	if snap.SchemaVersion != 1 {
		t.Fatalf("unsupported snapshot schema_version %d", snap.SchemaVersion)
	}
	routes, err := snapshotRoutes(snap)
	if err != nil {
		t.Fatalf("snapshot rejected by gateway rules: %v", err)
	}
	return routes
}

func TestRouteDiffFixtures(t *testing.T) {
	file, err := routing.Load("testdata/routediff/routes.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if d := diffRoutes(file, loadSnapshotRoutes(t, "testdata/routediff/same.json")); len(d) != 0 {
		t.Errorf("same: want no diff, got %q", d)
	}
	want := []string{
		"/mcp/team-a/one: enabled file=true snapshot=false",
		"/mcp/team-a/one: tools file=[a b] snapshot=[a c]",
		"/mcp/team-a/one: upstream file=http://one.team-a.svc.cluster.local/mcp snapshot=http://one.team-a.svc.cluster.local/other",
		"/mcp/team-a/three: only in snapshot",
		"/mcp/team-a/two: only in file",
	}
	if d := diffRoutes(file, loadSnapshotRoutes(t, "testdata/routediff/drift.json")); !slices.Equal(d, want) {
		t.Errorf("drift:\n got %q\nwant %q", d, want)
	}
}
