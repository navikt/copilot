package provider

import (
	"slices"
	"strings"
	"testing"

	"github.com/navikt/copilot/cli/nav-pilot/internal/domain"
	"github.com/navikt/copilot/cli/nav-pilot/internal/local"
)

// The launch hands the action check its server and lets it through cplt:
// the variable, and the port when the server is on loopback (#1165).
func TestWithActionCheckServer(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Cleanup(func() { local.SetEndpoint("", "") })
	on := domain.ResolvedConfig{LocalEnabled: true, HookActionCheck: "log"}
	tests := []struct {
		name, endpoint string
		r              domain.ResolvedConfig
		env            string
		flags          []string
	}{
		{"loopback", "http://127.0.0.1:8081", on, "http://127.0.0.1:8081 m endpoint", []string{"--pass-env", ActionCheckServerEnv, "--allow-localhost", "8081"}},
		{"private address", "http://10.0.0.5:8081", on, "http://10.0.0.5:8081 m endpoint", []string{"--pass-env", ActionCheckServerEnv}},
		{"check off", "http://127.0.0.1:8081", domain.ResolvedConfig{LocalEnabled: true, HookActionCheck: "off"}, "", nil},
		{"no local model", "http://127.0.0.1:8081", domain.ResolvedConfig{HookActionCheck: "log"}, "", nil},
		{"no server running", "", on, "", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			local.SetEndpoint(tt.endpoint, "m")
			env, flags := withActionCheckServer(tt.r, []string{"PATH=/bin"})
			got := ""
			for _, e := range env {
				if v, ok := strings.CutPrefix(e, ActionCheckServerEnv+"="); ok {
					got = v
				}
			}
			if got != tt.env || !slices.Equal(flags, tt.flags) {
				t.Errorf("env %q flags %v, want %q %v", got, flags, tt.env, tt.flags)
			}
		})
	}
}
