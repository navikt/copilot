package api

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
		{"copilot", "team-a", "IO.GitHub.Navikt/Team-A", true},
		{"copilot", "team-a", "io.github.navikt/team-a-svc", false},
		{"copilot", "foo", "io.github.navikt/foo-bar", false},
		{"copilot", "foo", "no.nav.foo-bar/svc", false},
		{"copilot", "foo-bar", "no.nav.foo-bar/svc", true},
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
		routes, allowlist := readSeedFiles(t, env)
		servers, err := seedServers(env+"-gcp", routes, allowlist)
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
	e.want(200, "POST", nsBase+"team-a/mcpservers/svc/approval", person("C222222"), approval(1, m.Status.Pending.ContentHash))
	// The platform team may take a vendor name, and a case variant collides at publish.
	m = decodeServer(t, e.want(201, "POST", nsBase+"copilot/mcpservers", person("A111111"), registryServerJSON("copilot", "playwright-mcp", "com.microsoft/playwright-mcp")))
	e.want(200, "POST", nsBase+"copilot/mcpservers/playwright-mcp/approval", person("C222222"), approval(1, m.Status.Pending.ContentHash))
	m = decodeServer(t, e.want(201, "POST", nsBase+"copilot/mcpservers", person("A111111"), registryServerJSON("copilot", "svc", "No.Nav.Team-A/svc")))
	e.wantReason(409, ReasonRegistryNameTaken, "POST", nsBase+"copilot/mcpservers/svc/approval", person("C222222"), approval(1, m.Status.Pending.ContentHash))
}

// A generation published before #2853 under a foreign name is not republished
// by rollback, and startup lists it.
func TestRealPostgresRollbackRechecksRegistryName(t *testing.T) {
	e := newApisEnv(t)
	e.srv.operatorTeam = "copilot"
	ctx := context.Background()
	m := decodeServer(t, e.want(201, "POST", nsBase+"team-a/mcpservers", person("A111111"), registryServerJSON("team-a", "svc", "no.nav.team-a/svc")))
	e.want(200, "POST", nsBase+"team-a/mcpservers/svc/approval", person("C222222"), approval(1, m.Status.Pending.ContentHash))
	// As an image from before the rule would have stored it.
	if _, err := e.db.pool.Exec(ctx, `UPDATE server_versions SET registry = jsonb_set(registry, '{name}', '"com.microsoft/svc"') WHERE namespace = 'team-a' AND name = 'svc'`); err != nil {
		t.Fatal(err)
	}
	if _, err := e.db.pool.Exec(ctx, `UPDATE servers SET registry_name = 'com.microsoft/svc' WHERE namespace = 'team-a' AND name = 'svc'`); err != nil {
		t.Fatal(err)
	}
	if got, err := e.db.ForeignRegistryNames(ctx, "copilot"); err != nil || len(got) != 1 || got[0] != "team-a/svc=com.microsoft/svc" {
		t.Fatalf("foreign names: %v %v", got, err)
	}
	m = decodeServer(t, e.want(200, "GET", nsBase+"team-a/mcpservers/svc", person("A111111"), ""))
	m = decodeServer(t, e.want(200, "PUT", nsBase+"team-a/mcpservers/svc", person("A111111"), strings.Replace(strings.Replace(registryServerJSON("team-a", "svc", "no.nav.team-a/svc"), `"1.0.0"`, `"1.0.1"`, 1), `"namespace"`, `"resourceVersion":"`+m.Metadata.ResourceVersion+`","namespace"`, 1)))
	// A registry version bump adds no access: published at once (#2634).
	if m.Status.PublishedGeneration != 2 || m.Status.Pending != nil {
		t.Fatalf("version bump must auto-publish: %+v", m.Status)
	}
	if code, out := e.do("POST", nsBase+"team-a/mcpservers/svc/rollback", person("A111111"), `{"fromGeneration":2,"reason":"test"}`); code != 422 || !strings.Contains(out, "spec.registry.name") {
		t.Fatalf("rollback to a foreign name: want 422, got %d %s", code, out)
	}
}

// A draft submitted before #2853 under a foreign name cannot be approved.
func TestRealPostgresApproveRechecksRegistryName(t *testing.T) {
	e := newApisEnv(t)
	e.srv.operatorTeam = "copilot"
	m := decodeServer(t, e.want(201, "POST", nsBase+"team-a/mcpservers", person("A111111"), registryServerJSON("team-a", "svc", "no.nav.team-a/svc")))
	if _, err := e.db.pool.Exec(context.Background(), `UPDATE server_versions SET registry = jsonb_set(registry, '{name}', '"com.microsoft/svc"') WHERE namespace = 'team-a' AND name = 'svc'`); err != nil {
		t.Fatal(err)
	}
	if code, out := e.do("POST", nsBase+"team-a/mcpservers/svc/approval", person("C222222"), approval(1, m.Status.Pending.ContentHash)); code != 422 || !strings.Contains(out, "spec.registry.name") {
		t.Fatalf("approve a foreign name: want 422, got %d %s", code, out)
	}
}
