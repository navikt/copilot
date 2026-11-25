package main

import (
	"encoding/json"
	"net/url"
	"os"
	"sort"
	"strings"
	"testing"

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

func TestRegistryMatchesProdRoutes(t *testing.T) {
	missingEntry, missingRoute := diffPaths(registryGatewayPaths(t), enabledProdRoutes(t))
	for _, r := range missingEntry {
		t.Errorf("enabled prod route %s has no registry entry in apps/mcp-registry/allowlist.json", r)
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
