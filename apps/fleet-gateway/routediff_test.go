package main

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"testing"
)

// diffRoutes compares the Git route file with a fleet-control snapshot after
// both are validated, so upstreams compare in canonical .svc.cluster.local form.
func diffRoutes(file, snap []PilotRoute) []string {
	byRoute := func(rs []PilotRoute) map[string]PilotRoute {
		m := map[string]PilotRoute{}
		for _, r := range rs {
			m[r.Route] = r
		}
		return m
	}
	f, s := byRoute(file), byRoute(snap)
	var out []string
	for route, a := range f {
		b, ok := s[route]
		if !ok {
			out = append(out, route+": only in file")
			continue
		}
		at, bt := slices.Sorted(slices.Values(a.Tools)), slices.Sorted(slices.Values(b.Tools))
		for _, d := range []struct{ field, file, snap string }{
			{"team", a.Team, b.Team},
			{"target", a.Target, b.Target},
			{"upstream", a.ParsedURL.String(), b.ParsedURL.String()},
			{"enabled", fmt.Sprint(!a.isDisabled()), fmt.Sprint(!b.isDisabled())},
			{"tools", fmt.Sprint(at), fmt.Sprint(bt)},
			{"passAllTools", fmt.Sprint(a.PassAllTools), fmt.Sprint(b.PassAllTools)},
		} {
			if d.file != d.snap {
				out = append(out, fmt.Sprintf("%s: %s file=%s snapshot=%s", route, d.field, d.file, d.snap))
			}
		}
	}
	for route := range s {
		if _, ok := f[route]; !ok {
			out = append(out, route+": only in snapshot")
		}
	}
	slices.Sort(out)
	return out
}

func loadSnapshotRoutes(t *testing.T, path string) []PilotRoute {
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
	file, err := loadPilotRoutes("testdata/routediff/routes.yaml")
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

// TestRouteDiffLive is the pre-switch check (#2760). It runs only with
// ROUTEDIFF_SNAPSHOT set and fails on any difference:
//
//	ROUTEDIFF_SNAPSHOT=/path/snapshot.json go test -run TestRouteDiffLive -v .
//
// ROUTEDIFF_FILE defaults to pilot-routes.prod.yaml.
func TestRouteDiffLive(t *testing.T) {
	snapPath := os.Getenv("ROUTEDIFF_SNAPSHOT")
	if snapPath == "" {
		t.Skip("ROUTEDIFF_SNAPSHOT not set")
	}
	filePath := os.Getenv("ROUTEDIFF_FILE")
	if filePath == "" {
		filePath = "pilot-routes.prod.yaml"
	}
	file, err := loadPilotRoutes(filePath)
	if err != nil {
		t.Fatal(err)
	}
	snap := loadSnapshotRoutes(t, snapPath)
	for _, d := range diffRoutes(file, snap) {
		t.Error(d)
	}
	t.Logf("compared %d file routes with %d snapshot routes", len(file), len(snap))
}
