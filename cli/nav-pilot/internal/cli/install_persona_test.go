package cli

import (
	"testing"

	"github.com/navikt/copilot/cli/nav-pilot/internal/agentpakke"
)

// TestInstalledPrimaryAgent covers the last places an install named Nav's own
// persona to a team installing someone else's pakke. Found by installing a
// synthetic third-party agentpakke end to end, where the closing line read "Use
// @nav-pilot in Copilot to start planning" over a pakke whose only agent is
// called kokken.
//
// Both call sites print it: the one that installs a named pakke and the one
// that installs everything a source ships. Review caught that the first fix
// only reached one of them.
func TestInstalledPrimaryAgent(t *testing.T) {
	tests := []struct {
		name string
		src  *Source
		want string
	}{
		{"no source at all", nil, "nav-pilot"},
		{"manifest-less source", &Source{Repo: "navikt/other"}, "nav-pilot"},
		{
			"a pakke that declares its own roster",
			&Source{Repo: "navikt/annet-team", Pakke: &agentpakke.Manifest{
				Name:    "annet-team",
				Clients: map[string]agentpakke.ClientEntry{"copilot": {PrimaryAgents: []string{"kokken", "baker"}}},
			}},
			"kokken",
		},
		{
			// A pakke that declares no agents for this client falls back rather
			// than printing an empty handle.
			"a pakke with no copilot roster",
			&Source{Repo: "navikt/annet-team", Pakke: &agentpakke.Manifest{
				Name:    "annet-team",
				Clients: map[string]agentpakke.ClientEntry{"opencode": {PrimaryAgents: []string{"kokken"}}},
			}},
			"nav-pilot",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := installedPrimaryAgent(tt.src); got != tt.want {
				t.Errorf("installedPrimaryAgent = %q, want %q", got, tt.want)
			}
		})
	}
}
