package main

import (
	"encoding/json"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func TestLocalCommandValidation(t *testing.T) {
	for _, tt := range []struct {
		name    string
		local   LocalCommand
		remotes []Transport
		wantErr bool
	}{
		{name: "native", local: LocalCommand{Command: "nais", Args: []string{"alpha", "mcp", "serve"}}},
		{name: "no args", local: LocalCommand{Command: "nais"}},
		{name: "empty arg", local: LocalCommand{Command: "nais", Args: []string{""}}},
		{name: "empty command", wantErr: true},
		{name: "shell string", local: LocalCommand{Command: "nais alpha mcp serve"}, wantErr: true},
		{name: "NUL command", local: LocalCommand{Command: "nais\x00"}, wantErr: true},
		{name: "NUL arg", local: LocalCommand{Command: "nais", Args: []string{"\x00"}}, wantErr: true},
		{name: "ambiguous remote", local: LocalCommand{Command: "nais"}, remotes: []Transport{{Type: TransportTypeSSE, URL: "https://example.com/sse"}}, wantErr: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			e := StaticServerData{Name: "io.github.nais/nais-mcp", Description: "Nais MCP", Version: "6.5.0", Local: &tt.local, Remotes: tt.remotes}
			if err := validateServerEntry(&e, 0, map[string]bool{}); (err != nil) != tt.wantErr {
				t.Fatalf("validation = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
	e := StaticServerData{Name: "io.github.nais/nais-mcp", Description: "Nais MCP", Version: "6.5.0", Local: &LocalCommand{Command: "nais"}, Packages: []Package{{}}}
	if err := validateServerEntry(&e, 0, map[string]bool{}); err == nil {
		t.Fatal("local and packages must not be combined")
	}
}

func TestNaisRecipeReturnedByListAndVersion(t *testing.T) {
	config := &Config{DomainInternal: "intern.nav.no", DomainExternal: "nav.no"}
	for _, path := range []string{"/v0.1/servers", "/v0.1/servers/io.github.nais%2Fnais-mcp/versions/latest"} {
		t.Run(path, func(t *testing.T) {
			w := httptest.NewRecorder()
			r := httptest.NewRequest("GET", path, nil)
			var response ServerResponse
			if path == "/v0.1/servers" {
				serversListHandler(w, r, config)
				var list ServerListResponse
				if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil {
					t.Fatal(err)
				}
				for _, s := range list.Servers {
					if s.Server.Name == "io.github.nais/nais-mcp" {
						response = s
					}
				}
			} else {
				serverVersionHandler(w, r, config)
				if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
					t.Fatal(err)
				}
			}
			if w.Code != 200 || response.Meta.NavRegistry == nil {
				t.Fatalf("response = %d %s", w.Code, w.Body.String())
			}
			meta := response.Meta.NavRegistry
			if response.Meta.Official.Status != StatusDeprecated {
				t.Fatal("Nais must remain inactive until ROS conditions and client rollout are resolved")
			}
			if !reflect.DeepEqual(meta.Local, &LocalCommand{Command: "nais", Args: []string{"alpha", "mcp", "serve"}}) {
				t.Fatalf("native recipe = %+v", meta.Local)
			}
			if len(response.Server.Packages)+len(response.Server.Remotes) != 0 {
				t.Fatal("native recipe was published as a package or remote")
			}
			if !reflect.DeepEqual(meta.SandboxHosts, []string{"console.nav.cloud.nais.io"}) || len(meta.Tools) != 12 {
				t.Fatalf("hosts/tools = %+v", meta)
			}
			if len(meta.SetupInstructions) != 2 || meta.SetupInstructions[0].Commands[0] != "nais login -n" {
				t.Fatalf("setup = %+v", meta.SetupInstructions)
			}
		})
	}
	meta := navRegistryMeta(StaticServerData{Local: &LocalCommand{Command: "nais"}})
	if meta == nil || meta.Local == nil {
		t.Fatal("local-only metadata must be published")
	}
	data, err := json.Marshal(ServerJSON{Name: "io.github.nais/nais-mcp"})
	if err != nil || strings.Contains(string(data), `"local"`) {
		t.Fatalf("native extension leaked into upstream server: %s, %v", data, err)
	}
}
