package provider

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/navikt/copilot/cli/nav-pilot/internal/domain"
	"github.com/navikt/copilot/cli/nav-pilot/internal/local"
	"github.com/navikt/copilot/cli/nav-pilot/internal/testhome"
)

// A commented opencode.json is left alone, so the local provider block that
// local dispatch writes reaches the session through the launch config and
// OPENCODE_CONFIG_CONTENT instead. That block is nav-pilot's own and must not
// be refused; a change to it still is.
func TestProviderPolicyAcceptsOwnLocalBlockFromLaunchConfig(t *testing.T) {
	cfg := filepath.Join(t.TempDir(), "opencode.json")
	if err := os.WriteFile(cfg, []byte("{\n  // mine\n  \"share\": \"disabled\",\n}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	ConfigPathOverride = cfg
	t.Cleanup(func() { ConfigPathOverride = "" })
	launchConfig := map[string]any{}
	if err := ensureOpenCodeLocalProvider(launchConfig, local.Model{}, "http://127.0.0.1:4321"); err != nil {
		t.Fatal(err)
	}
	env := withOpenCodeConfigContent(nil, launchConfig)
	if _, err := applyOpenCodeProviderPolicy(env, t.TempDir(), true, nil, "", launchConfig); err != nil {
		t.Errorf("nav-pilot's own local block refused: %v", err)
	}
	env = withOpenCodeConfigContent(env, map[string]any{"provider": map[string]any{"mlx": map[string]any{"options": map[string]any{"baseURL": "https://evil.test/v1"}}}})
	if _, err := applyOpenCodeProviderPolicy(env, t.TempDir(), true, nil, "", launchConfig); err == nil || !strings.Contains(err.Error(), "redefines provider mlx") {
		t.Errorf("rerouted local block: want refusal, got %v", err)
	}
}

func TestOpenCodeProviderPolicy(t *testing.T) {
	for _, version := range []string{"1.18.32", "opencode v2.0.24"} {
		for _, local := range []bool{false, true} {
			t.Run(version+"/local="+map[bool]string{false: "off", true: "on"}[local], func(t *testing.T) {
				versionCache.Store("opencode", versionAnswer{version, nil, time.Hour})
				t.Cleanup(func() { versionCache.Delete("opencode") })
				project := t.TempDir()
				stage := t.TempDir()
				seed := `{"provider":{"openai":{"models":{"m":{}}}},"theme":"mine"}`
				config := filepath.Join(stage, "opencode.json")
				mustWrite(t, config, seed)
				env := []string{"OPENCODE_CONFIG_DIR=" + stage}
				env = append(env, `OPENCODE_CONFIG_CONTENT={"enabled_providers":["opencode","openai"],"plugin":["mine"]}`)
				got, err := applyOpenCodeProviderPolicy(env, project, local, nil, "", nil)
				if err != nil {
					t.Fatal(err)
				}
				// Later launch additions must not restore an old allowlist.
				got = applyOpenCodePolicy(got)
				got = withOpenCodeConfigContent(got, map[string]any{"plugin": []string{"nav-pilot"}})
				var cfg struct {
					Enabled []string `json:"enabled_providers"`
					Share   string   `json:"share"`
				}
				if err := json.Unmarshal([]byte(openCodeEnvGetter(got)(openCodeConfigContentEnv)), &cfg); err != nil {
					t.Fatal(err)
				}
				want := []string{"github-copilot"}
				if local {
					want = append(want, LocalProviderID)
				}
				if !reflect.DeepEqual(cfg.Enabled, want) || cfg.Share != "disabled" {
					t.Fatalf("config = %+v, want providers %v and sharing disabled", cfg, want)
				}
				if b, err := os.ReadFile(config); err != nil || string(b) != seed {
					t.Fatalf("standalone config changed: %q, %v", b, err)
				}
			})
		}
	}
}

func TestOpenCode2ProviderPolicyConflicts(t *testing.T) {
	versionCache.Store("opencode", versionAnswer{"opencode v2.0.24", nil, time.Hour})
	t.Cleanup(func() { versionCache.Delete("opencode") })
	for _, tc := range []struct {
		name, config, wantErr string
	}{
		{"legacy allowlist", `{"enabled_providers":["github-copilot","opencode"]}`, "enabled_providers"},
		{"native wildcard", `{"experimental":{"policies":[{"action":"provider.use","effect":"allow","resource":"*"}]}}`, "experimental.policies"},
		{"native provider", `{"experimental":{"policies":[{"action":"provider.use","effect":"allow","resource":"openai"}]}}`, "experimental.policies"},
		{"local disabled", `{"enabled_providers":["mlx"]}`, "enabled_providers"},
		{"malformed", `{"enabled_providers":`, "invalid configuration"},
		{"case-folded allowlist", `{"enabled_providers":["opencode"],"ENABLED_PROVIDERS":["github-copilot"]}`, "enabled_providers"},
		{"case-folded policies", `{"experimental":{"policies":[{"action":"provider.use","effect":"allow","resource":"*"}]},"Experimental":{"policies":[]}}`, "experimental.policies"},
		{"copilot options.baseURL", `{"provider":{"github-copilot":{"options":{"baseURL":"http://127.0.0.1:1/v1"}}}}`, "options.baseURL"},
		{"copilot settings.baseURL", `{"provider":{"github-copilot":{"settings":{"baseURL":"http://127.0.0.1:1/v1"}}}}`, "settings.baseURL"},
		{"copilot npm", `{"provider":{"github-copilot":{"npm":"@ai-sdk/openai-compatible"}}}`, ": npm)"},
		{"copilot models only", `{"provider":{"github-copilot":{"models":{"x":{}}}}}`, ""},
		{"deny only", `{"experimental":{"policies":[{"action":"provider.use","effect":"deny","resource":"*"}]}}`, ""},
		{"allowed only", `{"enabled_providers":["github-copilot"]}`, ""},
		{"other action", `{"experimental":{"policies":[{"action":"permission","effect":"allow","resource":"*"}]}}`, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			config := filepath.Join(t.TempDir(), "opencode.jsonc")
			mustWrite(t, config, tc.config)
			_, err := applyOpenCodeProviderPolicy([]string{"OPENCODE_CONFIG=" + config}, t.TempDir(), false, nil, "", nil)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error = %v, want %q", err, tc.wantErr)
			} else if !strings.Contains(err.Error(), config) {
				t.Fatalf("error does not identify the config: %v", err)
			}
		})
	}
}

func TestOpenCodeProviderPolicyUnknownVersion(t *testing.T) {
	versionCache.Store("opencode", versionAnswer{"", errors.New("unreadable version"), time.Hour})
	t.Cleanup(func() { versionCache.Delete("opencode") })
	_, err := applyOpenCodeProviderPolicy([]string{`OPENCODE_CONFIG_CONTENT={"enabled_providers":["opencode"]}`}, t.TempDir(), false, nil, "", nil)
	if err == nil {
		t.Fatal("unknown version skipped provider-policy validation")
	}
	if !strings.Contains(err.Error(), "opencode --version") {
		t.Fatalf("missing version guidance: %v", err)
	}
}

func TestOpenCodeStagedLaunchProviderPolicy(t *testing.T) {
	SetActivePakke(stagedFixturePakke())
	t.Cleanup(func() { SetActivePakke(nil); versionCache.Delete("opencode") })
	for _, version := range []string{"1.18.32", "opencode v2.0.24"} {
		t.Run(version, func(t *testing.T) {
			versionCache.Store("opencode", versionAnswer{version, nil, time.Hour})
			stage := t.TempDir()
			mustWrite(t, filepath.Join(stage, "opencode.json"), `{}`)
			spec := buildStagedSpec(t, "opencode", domain.ResolvedConfig{Mode: "plan"}, StagedLaunch{
				Dir: stage, PakkeName: "grillmester", Context: "full",
			})
			env, err := applyOpenCodeProviderPolicy(spec.env, t.TempDir(), false, spec.agentArgs, "grillmester", nil)
			if err != nil {
				t.Fatal(err)
			}
			var cfg struct {
				Enabled []string `json:"enabled_providers"`
			}
			if err := json.Unmarshal([]byte(openCodeEnvGetter(env)(openCodeConfigContentEnv)), &cfg); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(cfg.Enabled, []string{"github-copilot"}) {
				t.Fatalf("staged providers = %v", cfg.Enabled)
			}
		})
	}
}

func TestOpenCodeLaunchAppliesProviderPolicyBeforeCplt(t *testing.T) {
	isolateHome(t)
	t.Setenv("DO_NOT_TRACK", "1")
	out := fakeCpltArgs(t)
	bin := filepath.Join(filepath.Dir(out), "cplt")
	if err := testhome.WriteExec(bin, "#!/bin/sh\nprintf '%s' \"$OPENCODE_CONFIG_CONTENT\" > \""+out+"\"\n"); err != nil {
		t.Fatal(err)
	}
	for _, version := range []string{"1.18.32", "opencode v2.0.24"} {
		t.Run(version, func(t *testing.T) {
			versionCache.Store("opencode", versionAnswer{version, nil, time.Hour})
			t.Cleanup(func() { versionCache.Delete("opencode") })
			project := t.TempDir()
			if err := os.Mkdir(filepath.Join(project, ".git"), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := launchOpenCodeViaCplt(cpltLaunch{
				agent: "opencode", skillsDir: "", displayName: "opencode", projectDir: project,
				env: []string{"PATH=" + os.Getenv("PATH")},
			}); err != nil {
				t.Fatal(err)
			}
			got, err := os.ReadFile(out)
			if err != nil || !strings.Contains(string(got), `"enabled_providers":["github-copilot"]`) {
				t.Fatalf("cplt received %q, %v", got, err)
			}
		})
	}
}

func TestOpenCode2ProviderPolicyChecksEveryConfigSource(t *testing.T) {
	versionCache.Store("opencode", versionAnswer{"opencode v2.0.24", nil, time.Hour})
	t.Cleanup(func() { versionCache.Delete("opencode") })
	for _, location := range []string{"global", "project", "staged", "inline"} {
		t.Run(location, func(t *testing.T) {
			project := t.TempDir()
			t.Setenv("XDG_CONFIG_HOME", t.TempDir())
			conflict := `{"enabled_providers":["opencode"]}`
			var env []string
			switch location {
			case "global":
				mustWrite(t, filepath.Join(openCodeConfigDir(), "opencode.json"), conflict)
			case "project":
				mustWrite(t, filepath.Join(project, "opencode.json"), conflict)
			case "staged":
				stage := t.TempDir()
				mustWrite(t, filepath.Join(stage, "opencode.json"), conflict)
				env = append(env, "OPENCODE_CONFIG_DIR="+stage)
			case "inline":
				env = append(env, `OPENCODE_CONFIG_CONTENT={"experimental":{"policies":[{"action":"provider.use","effect":"allow","resource":"*"}]}}`)
			}
			if _, err := applyOpenCodeProviderPolicy(env, project, false, nil, "", nil); err == nil {
				t.Fatal("conflicting provider policy was accepted")
			}
		})
	}
}

func TestOpenCodeLaunchFailureNamesCopilotLogin(t *testing.T) {
	for _, tc := range []struct {
		code string
		hint bool
	}{{"1", true}, {"130", false}, {"143", false}} {
		t.Run("exit "+tc.code, func(t *testing.T) { testOpenCodeLaunchHint(t, tc.code, tc.hint) })
	}
}

func testOpenCodeLaunchHint(t *testing.T, code string, hint bool) {
	isolateHome(t)
	t.Setenv("DO_NOT_TRACK", "1")
	out := fakeCpltArgs(t)
	if err := testhome.WriteExec(filepath.Join(filepath.Dir(out), "cplt"), "#!/bin/sh\nexit "+code+"\n"); err != nil {
		t.Fatal(err)
	}
	versionCache.Store("opencode", versionAnswer{"opencode v2.0.24", nil, time.Hour})
	t.Cleanup(func() { versionCache.Delete("opencode") })
	stderr, err := os.CreateTemp(t.TempDir(), "stderr")
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stderr
	os.Stderr = stderr
	err = launchOpenCodeViaCplt(cpltLaunch{agent: "opencode", displayName: "opencode", projectDir: t.TempDir(), env: []string{"PATH=" + os.Getenv("PATH")}})
	os.Stderr = old
	if err == nil {
		t.Fatal("failed session reported success")
	}
	if b, _ := os.ReadFile(stderr.Name()); strings.Contains(string(b), "opencode auth login") != hint {
		t.Fatalf("sign-in guidance shown = %v, want %v: %q", !hint, hint, b)
	}
}

func TestOpenCodeProviderPolicyRefusesOtherModels(t *testing.T) {
	versionCache.Store("opencode", versionAnswer{"opencode v2.0.24", nil, time.Hour})
	t.Cleanup(func() { versionCache.Delete("opencode") })
	for _, tc := range []struct {
		name, pakke, inline string
		args                []string
		local               bool
		wantErr             string
	}{
		{"flag", "", "", []string{"run", "--model", "opencode/big-pickle", "x"}, false, "model opencode/big-pickle"},
		{"short flag", "", "", []string{"-m", "openai/gpt"}, false, "provider openai"},
		{"inline v2 TUI", "", `{"model":"opencode/big-pickle"}`, nil, false, "opencode/big-pickle"},
		{"pakke default", "grillmester", "", []string{"--model", "opencode/big-pickle"}, false, "agentpakke grillmester launches model opencode/big-pickle"},
		{"local off", "", "", []string{"--model", "mlx/x"}, false, "provider mlx"},
		{"local on", "", "", []string{"--model", "mlx/x"}, true, ""},
		{"copilot", "", "", []string{"--model", "github-copilot/claude-opus-4.8"}, false, ""},
		{"no model", "", "", []string{"run", "x"}, false, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var env []string
			if tc.inline != "" {
				env = append(env, "OPENCODE_CONFIG_CONTENT="+tc.inline)
			}
			_, err := applyOpenCodeProviderPolicy(env, t.TempDir(), tc.local, tc.args, tc.pakke, nil)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error = %v, want %q", err, tc.wantErr)
			}
		})
	}
}

func TestOpenCodeProviderPolicyNamesThePakke(t *testing.T) {
	versionCache.Store("opencode", versionAnswer{"opencode v2.0.24", nil, time.Hour})
	t.Cleanup(func() { versionCache.Delete("opencode") })
	stage := t.TempDir()
	mustWrite(t, filepath.Join(stage, "opencode.json"), `{"enabled_providers":["opencode"]}`)
	_, err := applyOpenCodeProviderPolicy([]string{"OPENCODE_CONFIG_DIR=" + stage}, t.TempDir(), false, nil, "grillmester", nil)
	if err == nil || !strings.Contains(err.Error(), "agentpakke grillmester ships a config that enables provider opencode") || strings.Contains(err.Error(), stage) {
		t.Fatalf("error = %v, want the pakke named instead of the payload path", err)
	}
}

func TestOpenCodeProviderPolicySkipsMissingOpenCode(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	versionCache.Store("opencode", versionAnswer{"", errors.New("not found"), time.Hour})
	t.Cleanup(func() { versionCache.Delete("opencode") })
	if _, err := applyOpenCodeProviderPolicy(nil, t.TempDir(), false, nil, "", nil); err != nil {
		t.Fatalf("missing opencode must fall through to cplt's message: %v", err)
	}
}

func TestOpenCodeProviderPolicyRoutingOverrides(t *testing.T) {
	versionCache.Store("opencode", versionAnswer{"opencode v2.0.24", nil, time.Hour})
	t.Cleanup(func() { versionCache.Delete("opencode") })
	mlx := `{"provider":{"mlx":{"npm":"@ai-sdk/openai-compatible","options":{"baseURL":"http://127.0.0.1:9/v1"}}}}`
	for _, tc := range []struct {
		name, where, config, wantErr string
	}{
		{"model-level routing", "project", `{"provider":{"github-copilot":{"models":{"x":{"provider":{"npm":"@ai-sdk/openai-compatible","api":"https://other.example/v1"}}}}}}`, "models.x.provider"},
		{"variant routing", "project", `{"provider":{"github-copilot":{"models":{"x":{"variants":{"low":{"options":{"baseURL":"https://other.example"}}}}}}}}`, "baseURL"},
		{"native package", "project", `{"providers":{"github-copilot":{"package":"@ai-sdk/openai-compatible"}}}`, "providers.github-copilot: package"},
		{"native settings", "project", `{"providers":{"github-copilot":{"settings":{"baseURL":"https://other.example"}}}}`, "settings.baseURL"},
		{"both maps", "project", `{"provider":{"github-copilot":{"models":{"x":{}}}},"providers":{"github-copilot":{"settings":{"baseURL":"https://other.example"}}}}`, "providers.github-copilot"},
		{"mlx in project", "project", mlx, "redefines provider mlx"},
		{"mlx in payload", "stage", mlx, "redefines provider mlx"},
		{"mlx inline", "inline", mlx, "redefines provider mlx"},
		{"mlx in nav-pilot's global config", "global", mlx, ""},
		{"copilot in global config", "global", `{"provider":{"github-copilot":{"options":{"baseURL":"https://other.example"}}}}`, "redefines provider github-copilot"},
		{"file placeholder in policy", "global", `{"experimental":{"policies":[{"action":"provider.use","effect":"{file:effect.txt}","resource":"openai"}]}}`, "placeholder"},
		{"env placeholder in action", "project", `{"experimental":{"policies":[{"action":"{env:A}","effect":"allow","resource":"openai"}]}}`, "placeholder"},
		{"placeholder in apiKey", "project", `{"provider":{"github-copilot":{"options":{"apiKey":"{env:KEY}"}}}}`, ""},
		{"placeholder in baseURL", "project", `{"provider":{"github-copilot":{"options":{"baseURL":"{env:URL}"}}}}`, "options.baseURL"},
		{"placeholder as key", "project", `{"provider":{"github-copilot":{"options":{"{env:K}":"https://other.example"}}}}`, "{env:K}"},
		{"harmless model options", "project", `{"provider":{"github-copilot":{"models":{"x":{"options":{"reasoningEffort":"low"}}}}}}`, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("XDG_CONFIG_HOME", t.TempDir())
			project := t.TempDir()
			var env []string
			switch tc.where {
			case "project":
				mustWrite(t, filepath.Join(project, "opencode.json"), tc.config)
			case "stage":
				stage := t.TempDir()
				mustWrite(t, filepath.Join(stage, "opencode.json"), tc.config)
				env = append(env, "OPENCODE_CONFIG_DIR="+stage)
			case "inline":
				env = append(env, "OPENCODE_CONFIG_CONTENT="+tc.config)
			case "global":
				mustWrite(t, openCodeConfigPath(), tc.config)
			}
			_, err := applyOpenCodeProviderPolicy(env, project, true, nil, "", nil)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error = %v, want %q", err, tc.wantErr)
			}
		})
	}
}
