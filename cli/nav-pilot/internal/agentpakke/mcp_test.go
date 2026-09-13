package agentpakke

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
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

// schemaRegistryServers reads the registry names out of the schema bytes the
// binary validates with, which is where the allowlist copy lives.
func schemaRegistryServers(t *testing.T) []string {
	t.Helper()
	var doc struct {
		Defs struct {
			MCPServerName struct {
				Enum []string `json:"enum"`
			} `json:"mcpServerName"`
		} `json:"$defs"`
		Properties map[string]json.RawMessage `json:"properties"`
	}
	if err := json.Unmarshal(SchemaJSON(), &doc); err != nil {
		t.Fatalf("parsing the embedded schema: %v", err)
	}
	if _, ok := doc.Properties["mcpServers"]; !ok {
		t.Fatal("the embedded schema declares no mcpServers property")
	}
	if len(doc.Defs.MCPServerName.Enum) == 0 {
		t.Fatal("the embedded schema publishes no registry server names")
	}
	return doc.Defs.MCPServerName.Enum
}

// TestKnownMCPServerValidates: a name the registry publishes is accepted and
// reaches the parsed manifest, which is what install reads to name it.
//
// Every name in the schema, not one sample: the enum is maintained by hand
// against the allowlist, and a typo in it would pass a one-name test.
func TestKnownMCPServerValidates(t *testing.T) {
	for _, name := range schemaRegistryServers(t) {
		m, err := Parse(mcpManifest(`,"mcpServers":["` + name + `"]`))
		if err != nil {
			t.Fatalf("Parse with registry server %q: %v", name, err)
		}
		if !reflect.DeepEqual(m.MCPServers, []string{name}) {
			t.Errorf("MCPServers = %v, want [%s]", m.MCPServers, name)
		}
	}
}

// TestUnknownMCPServerIsRejected: the registry is the only place a server can
// be defined, so a name it does not publish fails validation — and the message
// has to name the offending value, or an author with several declared servers
// cannot tell which one is wrong.
func TestUnknownMCPServerIsRejected(t *testing.T) {
	err := Validate(mcpManifest(`,"mcpServers":["io.github.navikt/github-mcp","no.nav/hjemmesnekret"]`))
	if err == nil {
		t.Fatal("a server outside the registry allowlist must fail validation")
	}
	if !strings.Contains(err.Error(), "no.nav/hjemmesnekret") {
		t.Errorf("the error must name the offending value, got:\n%s", err)
	}
	if !strings.Contains(err.Error(), MCPRegistryURL) {
		t.Errorf("the error must point at the registry, got:\n%s", err)
	}
}

// TestManifestWithoutMCPServersUnchanged: the field is optional, and a manifest
// that never mentions it parses exactly as before.
func TestManifestWithoutMCPServersUnchanged(t *testing.T) {
	m, err := Parse(mcpManifest(""))
	if err != nil {
		t.Fatalf("Parse without mcpServers: %v", err)
	}
	if len(m.MCPServers) != 0 {
		t.Errorf("MCPServers = %v, want none", m.MCPServers)
	}
}

// TestRegistryServersMatchAllowlist is the drift guard for the copy the binary
// carries: the schema's enum against apps/mcp-registry/allowlist.json, which is
// the registry's own source of truth and lives in this repo. A server added
// there without being added here would make nav-pilot reject a manifest the
// registry says is fine.
func TestRegistryServersMatchAllowlist(t *testing.T) {
	path := filepath.Join("..", "..", "..", "..", "apps", "mcp-registry", "allowlist.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading the registry allowlist: %v", err)
	}
	var allowlist struct {
		Servers []struct {
			Name string `json:"name"`
		} `json:"servers"`
	}
	if err := json.Unmarshal(data, &allowlist); err != nil {
		t.Fatalf("parsing %s: %v", path, err)
	}
	want := make([]string, 0, len(allowlist.Servers))
	for _, s := range allowlist.Servers {
		want = append(want, s.Name)
	}
	sort.Strings(want)
	got := append([]string(nil), schemaRegistryServers(t)...)
	sort.Strings(got)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("the schema's mcpServerName enum = %v,\nwant the registry allowlist = %v\n"+
			"(update the enum in cli/nav-pilot/schemas/agentpakke-v1.json to match apps/mcp-registry/allowlist.json)",
			got, want)
	}
}
