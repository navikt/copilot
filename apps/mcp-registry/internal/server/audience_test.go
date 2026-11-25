package server

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestAudience_ServedUnchanged checks both sources serve audience as given
// under _meta."io.github.navikt/registry", and that entries without one have
// no audience key.
func TestAudience_ServedUnchanged(t *testing.T) {
	path := filepath.Join(t.TempDir(), "allowlist.json")
	static := `{"servers":[
		{"name":"io.github.navikt/aud-static","description":"Static with audience","version":"1","audience":{"teams":["team-b","team-a"]},"remotes":[{"type":"streamable-http","url":"https://mcp.example.com/mcp"}]},
		{"name":"io.github.navikt/plain-static","description":"Static without audience","version":"1","tools":["x"],"remotes":[{"type":"streamable-http","url":"https://mcp.example.com/mcp"}]}]}`
	if err := os.WriteFile(path, []byte(static), 0o600); err != nil {
		t.Fatal(err)
	}
	loader := func(context.Context) ([]DBEntry, error) {
		return []DBEntry{
			{Name: "io.github.navikt/aud-db", Entry: []byte(`{"name":"io.github.navikt/aud-db","description":"DB with audience","version":"1","owner":"team-a","audience":{"teams":["team-c"]},"remotes":[{"type":"sse","url":"https://mcp.example.com"}]}`)},
			{Name: "io.github.navikt/plain-db", Entry: subJSON("io.github.navikt/plain-db", "DB without audience", "1")},
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
		"io.github.navikt/aud-static":   `{"teams":["team-b","team-a"]}`,
		"io.github.navikt/aud-db":       `{"teams":["team-c"]}`,
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
		if aud := string(got.Meta["io.github.navikt/registry"]["audience"]); aud != w {
			t.Errorf("%s: served audience %q, want %q", s.Server.Name, aud, w)
		}
	}
	if len(want) != 0 {
		t.Errorf("servers missing from response: %v", want)
	}
}
