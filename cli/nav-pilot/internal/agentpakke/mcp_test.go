package agentpakke

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// mcpManifest is a minimal Tier 1 manifest with the given extra field appended,
// so each case differs only in what it says about MCP servers.
func mcpManifest(extra string) []byte {
	return []byte(`{"contractVersion":"1","name":"p","description":"d",` +
		`"clients":{"copilot":{"primaryAgents":["a"]}},` +
		`"layout":{"agents":"agents","skills":"skills"}` + extra + `}`)
}

// stubRegistry points the registry endpoint at a local server answering with
// the given names, in the shape the real service uses. No test may reach the
// real registry: a suite that needs the network is a suite that fails in CI.
func stubRegistry(t *testing.T, names ...string) {
	t.Helper()
	type server struct {
		Name string `json:"name"`
	}
	var doc struct {
		Servers []struct {
			Server server `json:"server"`
		} `json:"servers"`
	}
	for _, n := range names {
		doc.Servers = append(doc.Servers, struct {
			Server server `json:"server"`
		}{server{Name: n}})
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(doc)
	}))
	t.Cleanup(srv.Close)
	previous := mcpRegistryAPI
	mcpRegistryAPI = srv.URL
	t.Cleanup(func() { mcpRegistryAPI = previous })
}

// TestKnownMCPServerValidates: a name the registry publishes is accepted, and
// reaches the parsed manifest, which is what install reads to name it.
func TestKnownMCPServerValidates(t *testing.T) {
	stubRegistry(t, "io.github.navikt/github-mcp", "com.figma/figma-mcp")

	m, err := Parse(mcpManifest(`,"mcpServers":["io.github.navikt/github-mcp"]`))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(m.MCPServers) != 1 || m.MCPServers[0] != "io.github.navikt/github-mcp" {
		t.Errorf("MCPServers = %v, want one registry name", m.MCPServers)
	}
	findings, warning := m.MCPServerFindings()
	if len(findings) != 0 || warning != "" {
		t.Errorf("a published server gave findings %v / warning %q", findings, warning)
	}
}

// TestUnknownMCPServerIsRejected: the registry is the only place a server can
// be defined, so a name it does not publish is a violation — named, so an
// author who declares several can tell which one is wrong.
func TestUnknownMCPServerIsRejected(t *testing.T) {
	stubRegistry(t, "io.github.navikt/github-mcp")

	m, err := Parse(mcpManifest(`,"mcpServers":["io.github.navikt/github-mcp","no.nav/hjemmesnekret"]`))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	findings, warning := m.MCPServerFindings()
	if warning != "" {
		t.Errorf("the registry answered, so there is nothing to warn about: %q", warning)
	}
	if len(findings) != 1 {
		t.Fatalf("findings = %v, want exactly the unpublished name", findings)
	}
	if !strings.Contains(findings[0].Error(), "no.nav/hjemmesnekret") ||
		!strings.Contains(findings[0].Error(), MCPRegistryURL) {
		t.Errorf("the finding must name the value and the registry, got: %s", findings[0])
	}
}

// TestMCPRegistryUnreachableWarns is the fail-open rule: no network in CI, a
// sandbox, or an outage says nothing about whether a name is right, so it
// warns and passes. The alternative is a gate that fails for reasons unrelated
// to the manifest, which is a gate people learn to skip.
func TestMCPRegistryUnreachableWarns(t *testing.T) {
	m, err := Parse(mcpManifest(`,"mcpServers":["no.nav/hjemmesnekret"]`))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	// nil is the "registry did not answer" value every failure mode collapses to.
	findings, warning := m.mcpFindings(nil)
	if len(findings) != 0 {
		t.Errorf("an unanswered question must not fail the manifest, got %v", findings)
	}
	if !strings.Contains(warning, "not checked") {
		t.Errorf("the warning must say membership was not checked, got %q", warning)
	}

	// An endpoint that is not there is one such failure mode, end to end.
	unreachable := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
	url := unreachable.URL
	unreachable.Close()
	previous := mcpRegistryAPI
	mcpRegistryAPI = url
	defer func() { mcpRegistryAPI = previous }()

	if findings, warning := m.MCPServerFindings(); len(findings) != 0 || warning == "" {
		t.Errorf("a dead registry gave findings %v / warning %q, want a warning only", findings, warning)
	}
}

// TestMCPRegistryEmptyAnswerWarns: a registry that lists nothing is a deploy
// that went wrong, not a registry that publishes nothing. Rejecting every
// declared name over it is the one outcome worth avoiding.
func TestMCPRegistryEmptyAnswerWarns(t *testing.T) {
	stubRegistry(t)

	m, err := Parse(mcpManifest(`,"mcpServers":["io.github.navikt/github-mcp"]`))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if findings, warning := m.MCPServerFindings(); len(findings) != 0 || warning == "" {
		t.Errorf("an empty registry gave findings %v / warning %q, want a warning only", findings, warning)
	}
}

// TestManifestWithoutMCPServersUnchanged: the field is optional, a manifest
// that never mentions it parses exactly as before, and nothing asks the
// registry anything.
func TestManifestWithoutMCPServersUnchanged(t *testing.T) {
	previous := mcpRegistryAPI
	mcpRegistryAPI = "http://127.0.0.1:0/must-not-be-called"
	defer func() { mcpRegistryAPI = previous }()

	m, err := Parse(mcpManifest(""))
	if err != nil {
		t.Fatalf("Parse without mcpServers: %v", err)
	}
	if len(m.MCPServers) != 0 {
		t.Errorf("MCPServers = %v, want none", m.MCPServers)
	}
	if findings, warning := m.MCPServerFindings(); len(findings) != 0 || warning != "" {
		t.Errorf("a pakke with no servers reported %v / %q, want silence", findings, warning)
	}
}

// TestMCPServerNameFormat: the schema constrains the shape only, and the shape
// it accepts has to be the one the registry actually uses — every name it
// publishes today parses, and a value that is not a registry name does not.
//
// It reads apps/mcp-registry/allowlist.json, which is the registry's own
// source, so the pattern cannot drift into rejecting real servers. This is the
// whole of what the schema can promise now that membership is a live question.
func TestMCPServerNameFormat(t *testing.T) {
	stubRegistry(t) // never consulted: every case here fails or passes on shape

	data, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "apps", "mcp-registry", "allowlist.json"))
	if err != nil {
		t.Fatalf("reading the registry allowlist: %v", err)
	}
	var allowlist struct {
		Servers []struct {
			Name string `json:"name"`
		} `json:"servers"`
	}
	if err := json.Unmarshal(data, &allowlist); err != nil {
		t.Fatalf("parsing the registry allowlist: %v", err)
	}
	if len(allowlist.Servers) == 0 {
		t.Fatal("the registry allowlist lists no servers")
	}
	for _, s := range allowlist.Servers {
		if err := Validate(mcpManifest(`,"mcpServers":["` + s.Name + `"]`)); err != nil {
			t.Errorf("the schema rejects %q, a name the registry publishes: %v", s.Name, err)
		}
	}

	for _, bad := range []string{"github-mcp", "io.github.navikt/github mcp", "io.github.navikt/a/b", "/github-mcp"} {
		err := Validate(mcpManifest(`,"mcpServers":["` + bad + `"]`))
		if err == nil {
			t.Errorf("the schema accepts %q, which is not a registry name", bad)
			continue
		}
		if !strings.Contains(err.Error(), bad) {
			t.Errorf("the error must name the offending value %q, got:\n%s", bad, err)
		}
	}
}
