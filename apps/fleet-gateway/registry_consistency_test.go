package main

import (
	"encoding/json"
	"net/url"
	"os"
	"sort"
	"strings"
	"testing"
)

// Registry entries that point at the gateway must match the enabled routes in
// pilot-routes.prod.yaml, and the other way round (#2336). The path comes from
// the route definition, so it survives a change of route scheme.
// Routes carry no owner or tool list today; extend here when they do.

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
	routes, err := loadPilotRoutes("pilot-routes.prod.yaml")
	if err != nil {
		t.Fatalf("load prod routes: %v", err)
	}
	set := map[string]bool{}
	for _, r := range routes {
		if !r.isDisabled() {
			set[r.Route] = true
		}
	}
	return set
}

func diffPaths(have, want map[string]string, wantRoutes map[string]bool) (missingEntry, missingRoute []string) {
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
	missingEntry, missingRoute := diffPaths(registryGatewayPaths(t), nil, enabledProdRoutes(t))
	for _, r := range missingEntry {
		t.Errorf("enabled prod route %s has no registry entry in apps/mcp-registry/allowlist.json", r)
	}
	for _, p := range missingRoute {
		t.Errorf("registry gateway path %s has no enabled route in pilot-routes.prod.yaml", p)
	}
}

// Control: the comparison reports drift in both directions.
func TestRegistryRouteDiffDetectsDrift(t *testing.T) {
	entry, route := diffPaths(map[string]string{"/a": "x", "/b": "y"}, nil, map[string]bool{"/a": true, "/c": true})
	if len(entry) != 1 || entry[0] != "/c" || len(route) != 1 || route[0] != "/b" {
		t.Fatalf("got %v %v", entry, route)
	}
}
