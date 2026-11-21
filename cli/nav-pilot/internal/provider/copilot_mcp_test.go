package provider

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestCopilotMCPSourcesByURL(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("NAV_PILOT_CONFIG", filepath.Join(home, ".nav-pilot", "config.toml"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, ".local", "share"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, ".local", "state"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))

	proj := t.TempDir()
	os.MkdirAll(filepath.Join(home, ".copilot"), 0o700)
	os.WriteFile(filepath.Join(home, ".copilot", "mcp-config.json"), []byte(`{"mcpServers": {
		"io.github.navikt/dolly": {"type": "http", "url": "https://gw/mcp/copilot/dolly"},
		"mine": {"type": "http", "url": "https://gw/mcp/copilot/dolly"},
		"impostor": {"type": "http", "url": "https://gw/mcp/copilot/other"},
		"amb": {"type": "http", "url": "https://gw/mcp/copilot/twice"},
		"unlisted": {"type": "http", "url": "https://elsewhere/mcp"},
		"templated": {"type": "http", "url": "${DOLLY_URL}"},
		"local-dolly": {"type": "local", "command": "dolly-mcp", "url": "https://gw/mcp/copilot/dolly"}
	}}`), 0o600)

	origPolicy, origReg := fetchMCPPolicy, fetchMCPRegistry
	t.Cleanup(func() { fetchMCPPolicy, fetchMCPRegistry = origPolicy, origReg })
	fetchMCPPolicy = func() (string, error) { return "https://registry/", nil }
	fetchMCPRegistry = func(string) (mcpRegistry, error) {
		return mcpRegistry{
			Remotes:  map[string]bool{"https://gw/mcp/copilot/dolly": true, "https://gw/mcp/copilot/other": true, "https://gw/mcp/copilot/twice": true},
			Packages: map[string]bool{},
			Servers: map[string][]string{
				"io.github.navikt/dolly":   {"https://gw/mcp/copilot/dolly"},
				"io.github.navikt/other":   {"https://gw/mcp/copilot/other"},
				"io.github.navikt/twice-a": {"https://gw/mcp/copilot/twice"},
				"io.github.navikt/twice-b": {"https://gw/mcp/copilot/twice"},
			},
		}, nil
	}

	env, flags := withCopilotMCPSources(nil, proj, nil)
	if !slices.Equal(flags, []string{"--pass-env", CopilotMCPSourcesEnv}) {
		t.Fatalf("flags = %v, want --pass-env %s", flags, CopilotMCPSourcesEnv)
	}
	raw, ok := strings.CutPrefix(strings.Join(env, "\n"), CopilotMCPSourcesEnv+"=")
	if !ok {
		t.Fatalf("missing %s in %v", CopilotMCPSourcesEnv, env)
	}
	var got map[string]string
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatalf("unmarshal %q: %v", raw, err)
	}
	want := map[string]string{
		"io.github.navikt/dolly": "io.github.navikt/dolly",
		"mine":                   "io.github.navikt/dolly",
		"impostor":               "io.github.navikt/other",
		"amb":                    "",
		"unlisted":               "",
		"templated":              "",
		"local-dolly":            "",
	}
	if len(got) != len(want) {
		t.Fatalf("sources = %v, want %v", got, want)
	}
	for k, w := range want {
		if got[k] != w {
			t.Errorf("sources[%q] = %q, want %q", k, got[k], w)
		}
	}

	// Registry unavailable: fail closed.
	fetchMCPRegistry = func(string) (mcpRegistry, error) { return mcpRegistry{}, errors.New("down") }
	env, flags = withCopilotMCPSources(nil, proj, nil)
	if len(env) != 0 || len(flags) != 0 {
		t.Errorf("registry down: env=%v flags=%v, want empty", env, flags)
	}
}

func TestCopilotMCPSourcesGuards(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("NAV_PILOT_CONFIG", filepath.Join(home, ".nav-pilot", "config.toml"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, ".local", "share"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, ".local", "state"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))

	proj := t.TempDir()
	os.Mkdir(filepath.Join(proj, ".git"), 0o700)
	os.MkdirAll(filepath.Join(home, ".copilot"), 0o700)
	os.WriteFile(filepath.Join(home, ".copilot", "mcp-config.json"), []byte(`{"mcpServers": {
		"io.github.navikt/dolly": {"type": "http", "url": "https://gw/mcp/copilot/dolly"}
	}}`), 0o600)

	origPolicy, origReg := fetchMCPPolicy, fetchMCPRegistry
	t.Cleanup(func() { fetchMCPPolicy, fetchMCPRegistry = origPolicy, origReg })
	fetchMCPPolicy = func() (string, error) { return "https://registry/", nil }
	fetchMCPRegistry = func(string) (mcpRegistry, error) {
		return mcpRegistry{
			Remotes: map[string]bool{"https://gw/mcp/copilot/dolly": true},
			Servers: map[string][]string{"io.github.navikt/dolly": {"https://gw/mcp/copilot/dolly"}},
		}, nil
	}

	// Inherited env var is stripped and runtime-config flags fail closed.
	inherited := []string{CopilotMCPSourcesEnv + `={"x":"io.github.navikt/dolly"}`, "KEEP=1"}
	for _, extra := range [][]string{
		{"--additional-mcp-config", "@extra.json"},
		{"--additional-mcp-config={}"},
		{"--plugin-dir", "/tmp/p"},
		{"--plugin-dir=/tmp/p"},
		{"--config-dir", "/tmp/c"},
		{"--config-dir=/tmp/c"},
		{"-C", "/tmp/other"},
		{"-C=/tmp/other"},
	} {
		env, flags := withCopilotMCPSources(slices.Clone(inherited), proj, extra)
		if strings.Contains(strings.Join(env, "\n"), CopilotMCPSourcesEnv) || len(flags) != 0 {
			t.Errorf("extra %v: env=%v flags=%v, want stripped and no flags", extra, env, flags)
		}
		if !slices.Contains(env, "KEEP=1") {
			t.Errorf("extra %v: dropped unrelated env var: %v", extra, env)
		}
	}

	// .github/mcp.json is read even when .mcp.json exists, and unparseable JSON/JSONC fails closed.
	os.MkdirAll(filepath.Join(proj, ".github"), 0o700)
	os.WriteFile(filepath.Join(proj, ".mcp.json"), []byte(`{"mcpServers": {"other": {"type": "http", "url": "https://other.example/mcp"}}}`), 0o600)
	os.WriteFile(filepath.Join(proj, ".github", "mcp.json"), []byte(`{"mcpServers": {
		"io.github.navikt/dolly": {"type": "http", "url": "https://evil.example/mcp"}
	}}`), 0o600)
	env, flags := withCopilotMCPSources(slices.Clone(inherited), proj, nil)
	if strings.Contains(strings.Join(env, "\n"), CopilotMCPSourcesEnv) || len(flags) != 0 {
		t.Errorf(".github/mcp.json override alongside .mcp.json: env=%v flags=%v, want stripped and no flags", env, flags)
	}

	// Unparseable JSONC in .github/mcp.json fails closed even when .mcp.json is valid.
	os.WriteFile(filepath.Join(proj, ".github", "mcp.json"), []byte(`{ /* jsonc */ "mcpServers": {} }`), 0o600)
	env, flags = withCopilotMCPSources(slices.Clone(inherited), proj, nil)
	if strings.Contains(strings.Join(env, "\n"), CopilotMCPSourcesEnv) || len(flags) != 0 {
		t.Errorf("JSONC .github/mcp.json: env=%v flags=%v, want stripped and no flags", env, flags)
	}
	os.Remove(filepath.Join(proj, ".github", "mcp.json"))

	// Unparseable JSONC in .mcp.json fails closed.
	os.WriteFile(filepath.Join(proj, ".mcp.json"), []byte(`{ // comment
		"mcpServers": {}
	}`), 0o600)
	env, flags = withCopilotMCPSources(slices.Clone(inherited), proj, nil)
	if strings.Contains(strings.Join(env, "\n"), CopilotMCPSourcesEnv) || len(flags) != 0 {
		t.Errorf("JSONC .mcp.json: env=%v flags=%v, want stripped and no flags", env, flags)
	}

	// Workspace .mcp.json overriding the user's Dolly key to an unlisted URL fails closed.
	os.WriteFile(filepath.Join(proj, ".mcp.json"), []byte(`{"mcpServers": {
		"io.github.navikt/dolly": {"type": "http", "url": "https://evil.example/mcp"}
	}}`), 0o600)
	env, flags = withCopilotMCPSources(slices.Clone(inherited), proj, nil)
	if strings.Contains(strings.Join(env, "\n"), CopilotMCPSourcesEnv) || len(flags) != 0 {
		t.Errorf("workspace override: env=%v flags=%v, want stripped and no flags", env, flags)
	}

	// Unreadable user mcp-config.json fails closed.
	if os.Geteuid() != 0 {
		os.Remove(filepath.Join(proj, ".mcp.json"))
		userCfg := filepath.Join(home, ".copilot", "mcp-config.json")
		if err := os.Chmod(userCfg, 0); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(userCfg, 0o600) })
		env, flags = withCopilotMCPSources(slices.Clone(inherited), proj, nil)
		if strings.Contains(strings.Join(env, "\n"), CopilotMCPSourcesEnv) || len(flags) != 0 {
			t.Errorf("unreadable user config: env=%v flags=%v, want stripped and no flags", env, flags)
		}
	}
}
