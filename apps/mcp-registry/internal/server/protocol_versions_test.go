package server

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestProtocolVersions_ServedUnchanged checks both sources serve the list as
// given under _meta."io.github.navikt/registry".
func TestProtocolVersions_ServedUnchanged(t *testing.T) {
	path := filepath.Join(t.TempDir(), "allowlist.json")
	static := `{"servers":[
		{"name":"io.github.navikt/pv-static","description":"Static with versions","version":"1","protocolVersions":["2026-07-28","2025-11-25"],"remotes":[{"type":"streamable-http","url":"https://mcp.example.com/mcp"}]},
		{"name":"io.github.navikt/plain-static","description":"Static without","version":"1","tools":["x"],"remotes":[{"type":"streamable-http","url":"https://mcp.example.com/mcp"}]}]}`
	if err := os.WriteFile(path, []byte(static), 0o600); err != nil {
		t.Fatal(err)
	}
	loader := func(context.Context) ([]DBEntry, error) {
		return []DBEntry{
			{Name: "io.github.navikt/pv-db", Entry: []byte(`{"name":"io.github.navikt/pv-db","description":"DB with versions","version":"1","owner":"team-a","protocolVersions":["2026-07-28"],"remotes":[{"type":"sse","url":"https://mcp.example.com"}]}`)},
			{Name: "io.github.navikt/plain-db", Entry: subJSON("io.github.navikt/plain-db", "DB without", "1")},
		}, nil
	}
	store := NewStore(testConfig(), path, loader)
	if err := store.Reload(t.Context()); err != nil {
		t.Fatal(err)
	}
	servers, err := store.GetServers()
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"io.github.navikt/pv-static":    `["2026-07-28","2025-11-25"]`,
		"io.github.navikt/pv-db":        `["2026-07-28"]`,
		"io.github.navikt/plain-static": ``,
		"io.github.navikt/plain-db":     ``,
	}
	for _, s := range servers {
		w, ok := want[s.Server.Name]
		if !ok {
			continue
		}
		delete(want, s.Server.Name)
		raw, err := json.Marshal(s)
		if err != nil {
			t.Fatal(err)
		}
		var got struct {
			Meta map[string]map[string]json.RawMessage `json:"_meta"`
		}
		if err := json.Unmarshal(raw, &got); err != nil {
			t.Fatal(err)
		}
		if pv := string(got.Meta["io.github.navikt/registry"]["protocolVersions"]); pv != w {
			t.Errorf("%s: served protocolVersions %q, want %q", s.Server.Name, pv, w)
		}
	}
	if len(want) != 0 {
		t.Errorf("servers missing from response: %v", want)
	}
}
