package main

import (
	"fmt"
	"encoding/json"
	"net/url"
	"os"
	"sort"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"

	"github.com/navikt/copilot/fleet-gateway/internal/routing"
)

// Registry entries that point at the gateway must match the enabled routes in
// pilot-routes.prod.yaml, and the other way round (#2336). The path comes from
// the route definition, so it survives a change of route scheme.
// A registry entry's tools must equal its route's tools (#2604).

const gatewayHostPrefix = "fleet-gateway."

func registryGatewayPaths(t *testing.T) map[string]string {
	t.Helper()
	raw, err := os.ReadFile("../mcp-registry/allowlist.json")
	if err != nil {
		t.Fatalf("read registry: %v", err)
	}
	var doc struct {
		Servers []struct {
			Name    string `json:"name"`
			Remotes []struct {
				URL string `json:"url"`
			} `json:"remotes"`
		} `json:"servers"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parse registry: %v", err)
	}
	paths := map[string]string{} // path -> server name
	for _, s := range doc.Servers {
		for _, r := range s.Remotes {
			u, err := url.Parse(strings.ReplaceAll(r.URL, "{{domain_internal}}", "internal.invalid"))
			if err != nil {
				t.Fatalf("%s: bad remote url %q: %v", s.Name, r.URL, err)
			}
			if !strings.HasPrefix(u.Hostname(), gatewayHostPrefix) {
				continue
			}
			if prev, dup := paths[u.Path]; dup {
				t.Errorf("registry entries %s and %s share gateway path %s", prev, s.Name, u.Path)
			}
			paths[u.Path] = s.Name
		}
	}
	return paths
}

func enabledProdRoutes(t *testing.T) map[string]bool {
	t.Helper()
	routes, err := routing.Load("pilot-routes.prod.yaml")
	if err != nil {
		t.Fatalf("load prod routes: %v", err)
	}
	set := map[string]bool{}
	for _, r := range routes {
		if !r.Disabled() {
			set[r.Route] = true
		}
	}
	return set
}

func diffPaths(have map[string]string, wantRoutes map[string]bool) (missingEntry, missingRoute []string) {
	for r := range wantRoutes {
		if _, ok := have[r]; !ok {
			missingEntry = append(missingEntry, r)
		}
	}
	for p := range have {
		if !wantRoutes[p] {
			missingRoute = append(missingRoute, p)
		}
	}
	sort.Strings(missingEntry)
	sort.Strings(missingRoute)
	return
}

// routesSourceFromFleetControl reports whether the nais vars say routes come
// from fleet-control. Missing or unknown values fail, so a rename cannot
// silently disable the check.
func routesSourceFromFleetControl(raw []byte) (bool, error) {
	var vars struct {
		RoutesSource string `yaml:"routes_source"`
	}
	if err := yaml.Unmarshal(raw, &vars); err != nil {
		return false, err
	}
	switch vars.RoutesSource {
	case "fleet-control":
		return true, nil
	case "git":
		return false, nil
	}
	return false, fmt.Errorf("routes_source %q is not fleet-control or git", vars.RoutesSource)
}

func prodRoutesFromFleetControl(t *testing.T) bool {
	t.Helper()
	raw, err := os.ReadFile(".nais/prod-gcp.yaml")
	if err != nil {
		t.Fatalf("read prod nais vars: %v", err)
	}
	fc, err := routesSourceFromFleetControl(raw)
	if err != nil {
		t.Fatalf("prod-gcp.yaml: %v", err)
	}
	return fc
}

func TestRoutesSourceFromFleetControl(t *testing.T) {
	for _, c := range []struct {
		in      string
		want    bool
		wantErr bool
	}{
		{"routes_source: fleet-control\n", true, false},
		{"routes_source: \"fleet-control\"\n", true, false},
		{"routes_source: git\n", false, false},
		{"other: 1\n", false, true},
		{"routes_source: bogus\n", false, true},
	} {
		got, err := routesSourceFromFleetControl([]byte(c.in))
		if got != c.want || (err != nil) != c.wantErr {
			t.Errorf("%q: got %v, %v", c.in, got, err)
		}
	}
}

func TestRegistryMatchesProdRoutes(t *testing.T) {
	missingEntry, missingRoute := diffPaths(registryGatewayPaths(t), enabledProdRoutes(t))
	for _, r := range missingEntry {
		t.Errorf("enabled prod route %s has no registry entry in apps/mcp-registry/allowlist.json", r)
	}
	// Prod loads routes from fleet-control and teams self-register there, so such
	// routes never appear in pilot-routes.prod.yaml. mcp-registry reading from
	// fleet-control (#2765) replaces this check; it tightens again if prod goes
	// back to git.
	if prodRoutesFromFleetControl(t) {
		missingRoute = nil
	}
	for _, p := range missingRoute {
		t.Errorf("registry gateway path %s has no enabled route in pilot-routes.prod.yaml", p)
	}
}

// Control: the comparison reports drift in both directions.
func TestRegistryRouteDiffDetectsDrift(t *testing.T) {
	entry, route := diffPaths(map[string]string{"/a": "x", "/b": "y"}, map[string]bool{"/a": true, "/c": true})
	if len(entry) != 1 || entry[0] != "/c" || len(route) != 1 || route[0] != "/b" {
		t.Fatalf("got %v %v", entry, route)
	}
}

func TestRegistryToolsMatchRouteTools(t *testing.T) {
	raw, err := os.ReadFile("../mcp-registry/allowlist.json")
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Servers []struct {
			Name    string   `json:"name"`
			Tools   []string `json:"tools"`
			Remotes []struct {
				URL string `json:"url"`
			} `json:"remotes"`
		} `json:"servers"`
	}
	if err = json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	routes, err := routing.Load("pilot-routes.prod.yaml")
	if err != nil {
		t.Fatal(err)
	}
	byPath := map[string][]string{}
	for _, r := range routes {
		if !r.Disabled() {
			byPath[r.Route] = r.Tools
		}
	}
	matched := 0
	for _, s := range doc.Servers {
		for _, r := range s.Remotes {
			u, err := url.Parse(strings.ReplaceAll(r.URL, "{{domain_internal}}", "internal.invalid"))
			if err != nil || !strings.HasPrefix(u.Hostname(), gatewayHostPrefix) {
				continue
			}
			want, ok := byPath[u.Path]
			if !ok {
				continue // TestRegistryMatchesProdRoutes reports it
			}
			matched++
			have := append([]string(nil), s.Tools...)
			want = append([]string(nil), want...)
			sort.Strings(have)
			sort.Strings(want)
			if strings.Join(have, ",") != strings.Join(want, ",") {
				t.Errorf("%s: registry tools %v differ from route %s tools %v", s.Name, have, u.Path, want)
			}
		}
	}
	if matched == 0 {
		t.Fatal("no registry entry matched a prod route; the test checks nothing")
	}
}
