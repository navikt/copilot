package cli

import (
	"strings"
	"testing"

	"github.com/navikt/copilot/cli/nav-pilot/internal/agentpakke"
)

// TestMCPServerNotice: install names the servers a pakke declares and points at
// the registry, and says nothing at all for a pakke that declares none — the
// case every install today is in.
func TestMCPServerNotice(t *testing.T) {
	tests := []struct {
		name string
		src  *Source
		want []string
	}{
		{"no source at all", nil, nil},
		{"manifest-less source", &Source{Repo: "navikt/other"}, nil},
		{
			"a pakke that declares no servers",
			&Source{Repo: "navikt/annet-team", Pakke: &agentpakke.Manifest{Name: "annet-team"}},
			nil,
		},
		{
			"a pakke that declares two",
			&Source{Repo: "navikt/annet-team", Pakke: &agentpakke.Manifest{
				Name:       "annet-team",
				MCPServers: []string{"io.github.navikt/github-mcp", "com.figma/figma-mcp"},
			}},
			[]string{"io.github.navikt/github-mcp", "com.figma/figma-mcp", agentpakke.MCPRegistryURL},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := strings.Join(mcpServerNotice(tt.src), "\n")
			if len(tt.want) == 0 {
				if got != "" {
					t.Fatalf("mcpServerNotice printed %q, want nothing", got)
				}
				return
			}
			for _, want := range tt.want {
				if !strings.Contains(got, want) {
					t.Errorf("mcpServerNotice output must name %q, got:\n%s", want, got)
				}
			}
		})
	}
}
