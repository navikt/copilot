package main

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestRegistryNameAllowed(t *testing.T) {
	for _, c := range []struct {
		platform, ns, name string
		ok                 bool
	}{
		{"copilot", "team-a", "no.nav.team-a/svc", true},
		{"copilot", "team-a", "io.github.navikt/team-a", true},
		{"copilot", "team-a", "IO.GitHub.Navikt/Team-A-svc", true},
		{"copilot", "team-a", "no.nav.team-b/svc", false},
		{"copilot", "team-a", "io.github.navikt/team-b-svc", false},
		{"copilot", "team-a", "io.github.navikt/team-ab", false},
		{"copilot", "team-a", "io.github.navikt/dolly", false},
		{"copilot", "team-a", "com.microsoft/playwright-mcp", false},
		{"copilot", "copilot", "com.microsoft/playwright-mcp", true},
		{"copilot", "copilot", "no.nav.team-a/svc", true},
		{"", "", "com.microsoft/playwright-mcp", false},
	} {
		if got := registryNameAllowed(c.platform, c.ns, c.name); got != c.ok {
			t.Errorf("%s in %s (platform %q): got %v, want %v", c.name, c.ns, c.platform, got, c.ok)
		}
	}
}

// Every seeded name must stay publishable by its namespace under the prod config.
func TestSeededRegistryNamesStayValid(t *testing.T) {
	for _, env := range []string{"dev", "prod"} {
		servers, err := seedServers(readSeedFiles(t, env))
		if err != nil {
			t.Fatal(err)
		}
		for _, s := range servers {
			if s.RegistryName != "" && !registryNameAllowed("copilot", s.Namespace, s.RegistryName) {
				t.Errorf("%s: seeded %s in %s is no longer allowed", env, s.RegistryName, s.Namespace)
			}
		}
	}
}

func registryServerJSON(ns, name, regName string) string {
	raw, _ := json.Marshal(map[string]any{
		"apiVersion": apiGroupVersion, "kind": "MCPServer", "metadata": map[string]any{"name": name, "namespace": ns},
		"spec": map[string]any{"dataCategory": "Intern", "registry": map[string]any{"name": regName, "description": "d", "version": "1.0.0"}},
	})
	return string(raw)
}

func TestRealPostgresRegistryNamePrefix(t *testing.T) {
	e := newApisEnv(t)
	e.srv.operatorTeam = "copilot"
	for _, n := range []string{"no.nav.team-b/svc", "io.github.navikt/svc", "com.microsoft/svc"} {
		if code, out := e.do("POST", nsBase+"team-a/mcpservers", person("A111111"), registryServerJSON("team-a", "svc", n)); code != 422 || !strings.Contains(out, "spec.registry.name") {
			t.Errorf("team-a takes %s: want 422, got %d %s", n, code, out)
		}
	}
	m := decodeServer(t, e.want(201, "POST", nsBase+"team-a/mcpservers", person("A111111"), registryServerJSON("team-a", "svc", "no.nav.team-a/svc")))
	e.want(200, "POST", nsBase+"team-a/mcpservers/svc/approval", person("A222222"), approval(1, m.Status.Pending.ContentHash))
	// The platform team may take a vendor name, and a case variant collides at publish.
	m = decodeServer(t, e.want(201, "POST", nsBase+"copilot/mcpservers", person("A111111"), registryServerJSON("copilot", "playwright-mcp", "com.microsoft/playwright-mcp")))
	e.want(200, "POST", nsBase+"copilot/mcpservers/playwright-mcp/approval", person("C222222"), approval(1, m.Status.Pending.ContentHash))
	m = decodeServer(t, e.want(201, "POST", nsBase+"copilot/mcpservers", person("A111111"), registryServerJSON("copilot", "svc", "No.Nav.Team-A/svc")))
	e.want(409, "POST", nsBase+"copilot/mcpservers/svc/approval", person("C222222"), approval(1, m.Status.Pending.ContentHash))
}

func TestRealPostgresMigration008RefusesCaseCollisions(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	if err := db.migrateTo(ctx, "007_agentjob_orderer.sql"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.pool.Exec(ctx, `INSERT INTO servers (namespace, name, registry_name, status) VALUES ('a', 'x', 'no.nav.a/x', 'draft'), ('b', 'x', 'No.Nav.A/x', 'draft')`); err != nil {
		t.Fatal(err)
	}
	if err := db.Migrate(ctx); err == nil || !strings.Contains(err.Error(), "no.nav.a/x") {
		t.Fatalf("want a clear refusal naming the name, got %v", err)
	}
}
