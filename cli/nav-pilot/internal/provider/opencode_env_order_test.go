package provider

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/navikt/copilot/cli/nav-pilot/internal/domain"
	"github.com/navikt/copilot/cli/nav-pilot/internal/testhome"
)

// Both OpenCode launch paths, Tier 1 (LaunchOpenCode) and Tier 2
// (LaunchOpenCodeStaged), run to a fake cplt with inputs that contradict
// nav-pilot's policy: the user's own OPENCODE_CONFIG_CONTENT, a user MCP
// server the registry does not list, and on Tier 1 openCodeLaunchConfig (what
// a writer could not put in a commented opencode.json). Whatever the inputs
// say, what reaches opencode must hold the policy keys, the MCP disable, the
// own-dirs rules, the hooks plugin, the provider allowlist and the user's
// model on the primary agent.
func TestOpenCodeSessionEnvPolicyWinsOnBothPaths(t *testing.T) {
	for _, c := range []struct{ name, opencode string }{
		{"v1", "1.18.32\n"},
		{"v2", "opencode v2.0.24\n"},
	} {
		for _, path := range []string{"tier1", "tier2"} {
			t.Run(c.name+"/"+path, func(t *testing.T) {
				content, argv := launchOpenCodeForOrder(t, path, c.opencode)
				t.Logf("OPENCODE_CONFIG_CONTENT=%s", content)
				t.Logf("cplt argv=%q", argv)
				assertOpenCodeSessionPolicy(t, path, c.name == "v2", content)
			})
		}
	}
}

func launchOpenCodeForOrder(t *testing.T, path, version string) (content string, argv []string) {
	t.Helper()
	mcpEnv(t, "", testRegistry(), nil)
	home, _ := os.UserHomeDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	NavContextDirOverride = t.TempDir()
	t.Cleanup(func() { NavContextDirOverride = "" })

	versionCache.Store("opencode", versionAnswer{version, nil, time.Hour})
	versionCache.Store("cplt", versionAnswer{"cplt 2026.10.22-172931-b37a8ab\n", nil, time.Hour})
	t.Cleanup(func() { versionCache.Delete("opencode"); versionCache.Delete("cplt") })
	prevBridge := OpenCodeHookBridge
	OpenCodeHookBridge = func(domain.ResolvedConfig) HookBridge {
		return HookBridge{Post: []BridgeHook{{Name: "r", Argv: []string{"/bin/true"}}}}
	}
	t.Cleanup(func() { OpenCodeHookBridge = prevBridge })

	out := t.TempDir()
	bin := t.TempDir()
	if err := testhome.WriteExec(filepath.Join(bin, "opencode"), "#!/bin/sh\necho '"+strings.TrimSpace(version)+"'\n"); err != nil {
		t.Fatal(err)
	}
	cplt := "#!/bin/sh\nfor a in \"$@\"; do last=$a; done\n" +
		"[ \"$last\" = --version ] && { echo 'cplt 2026.10.22-172931-b37a8ab'; exit 0; }\n" +
		"printf '%s' \"$OPENCODE_CONFIG_CONTENT\" > " + filepath.Join(out, "content") + "\n" +
		"for a in \"$@\"; do printf '%s\\n' \"$a\"; done > " + filepath.Join(out, "argv") + "\n"
	if err := testhome.WriteExec(filepath.Join(bin, "cplt"), cplt); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+"/bin:/usr/bin")

	// The user's opencode.json turns on a server the registry does not list.
	mustWrite(t, filepath.Join(home, ".config", "opencode", "opencode.json"),
		`{"share":"disabled","mcp":{"rogue":{"type":"local","command":["rogue-mcp"],"enabled":true}}}`)
	primary := "nav-pilot"
	if path == "tier2" {
		primary = "grillmester"
	}
	// The user's own OPENCODE_CONFIG_CONTENT says the opposite of the policy.
	user := map[string]any{
		"share":      "auto",
		"autoupdate": true,
		"mcp":        map[string]any{"rogue": map[string]any{"type": "local", "command": []string{"rogue-mcp"}, "enabled": true}},
		"permission": map[string]any{"external_directory": "allow"},
		"agent":      map[string]any{primary: map[string]any{"model": "github-copilot/pinned"}},
		// The allowlist replaces it, last.
		"enabled_providers": []string{"opencode", "github-copilot"},
	}
	b, _ := json.Marshal(user)
	t.Setenv(openCodeConfigContentEnv, string(b))

	r := domain.ResolvedConfig{Client: "opencode", Mode: "default", Model: "gpt-6.1-sol", AskUser: true, Source: t.TempDir()}
	var err error
	if path == "tier1" {
		// What a writer left out of a commented opencode.json reaches the
		// session through this map. Here it contradicts the policy too.
		prev := openCodeLaunchConfig
		openCodeLaunchConfig = map[string]any{
			"share":      "auto",
			"autoupdate": "always",
			"mcp":        map[string]any{"rogue": map[string]any{"enabled": true}},
			"permission": map[string]any{"external_directory": "allow"},
			"agent":      map[string]any{primary: map[string]any{"model": "github-copilot/pinned"}},
		}
		t.Cleanup(func() { openCodeLaunchConfig = prev })
		err = LaunchOpenCode(r)
	} else {
		SetActivePakke(stagedFixturePakke())
		t.Cleanup(func() { SetActivePakke(nil) })
		dir := t.TempDir()
		mustWrite(t, filepath.Join(dir, "agents", "grillmester.md"), "---\ndescription: g\nmode: primary\nmodel: github-copilot/gpt-6-sol\n---\nGrill.\n")
		err = LaunchOpenCodeStaged(r, StagedLaunch{Dir: dir, PakkeName: "grillmester", Context: "full"})
	}
	if err != nil {
		t.Fatalf("launch: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(out, "content"))
	if err != nil {
		t.Fatalf("cplt never ran: %v", err)
	}
	a, _ := os.ReadFile(filepath.Join(out, "argv"))
	return string(got), strings.Split(strings.TrimSpace(string(a)), "\n")
}

func assertOpenCodeSessionPolicy(t *testing.T, path string, v2 bool, content string) {
	t.Helper()
	var cfg map[string]any
	if err := json.Unmarshal([]byte(content), &cfg); err != nil {
		t.Fatalf("content: %v", err)
	}
	if cfg["share"] != "disabled" {
		t.Errorf("share = %v, want disabled", cfg["share"])
	}
	if cfg["autoupdate"] != "notify" {
		t.Errorf("autoupdate = %v, want notify", cfg["autoupdate"])
	}
	mcp, _ := cfg["mcp"].(map[string]any)
	rogue, _ := mcp["rogue"].(map[string]any)
	if rogue["enabled"] != false {
		t.Errorf("mcp.rogue = %v, want enabled false", rogue)
	}
	perm, _ := cfg["permission"].(map[string]any)
	ext, _ := perm["external_directory"].(map[string]any)
	if ext[filepath.ToSlash(filepath.Join(openCodeConfigDir(), "agents"))+"/*"] != "allow" {
		t.Errorf("permission.external_directory = %v, want nav-pilot's own-dirs rules", perm["external_directory"])
	}
	plugin := "plugin"
	if v2 {
		plugin = "plugins"
	}
	if !strings.Contains(content, `"`+plugin+`":[`) || !strings.Contains(content, "opencode-plugin") {
		t.Errorf("hooks plugin not registered under %q", plugin)
	}
	if p, _ := json.Marshal(cfg["enabled_providers"]); string(p) != `["github-copilot"]` {
		t.Errorf("enabled_providers = %s, want [\"github-copilot\"]", p)
	}
	primary := "nav-pilot"
	if path == "tier2" {
		primary = "grillmester"
	}
	// opencode 2 takes a native (agents) entry whole over a legacy one.
	key := "agent"
	if native, _ := cfg["agents"].(map[string]any); native[primary] != nil {
		key = "agents"
	}
	agents, _ := cfg[key].(map[string]any)
	if e, _ := agents[primary].(map[string]any); e["model"] != "github-copilot/gpt-6.1-sol" {
		t.Errorf("%s.%s.model = %v, want the user's github-copilot/gpt-6.1-sol", key, primary, e["model"])
	}
}
