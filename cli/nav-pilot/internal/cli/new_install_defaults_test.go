package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	providerpkg "github.com/navikt/copilot/cli/nav-pilot/internal/provider"
)

// installedProvider is a client that is (or is not) installed and seeds nothing.
type installedProvider struct{ failingProvider }

func (installedProvider) Bootstrap(ResolvedConfig) (string, error) { return "", nil }

func stubInstalled(t *testing.T, installed map[string]bool) *int {
	t.Helper()
	orig, origAsk := providerFor, askInstallOpenCode
	t.Cleanup(func() { providerFor, askInstallOpenCode = orig, origAsk })
	providerFor = func(id string) (Provider, error) {
		return installedProvider{failingProvider{unavailable: !installed[id]}}, nil
	}
	origTested, origLogged := openCodeTested, openCodeLoggedIn
	t.Cleanup(func() { openCodeTested, openCodeLoggedIn = origTested, origLogged })
	openCodeTested = func() bool { return true }
	openCodeLoggedIn = func() bool { return true }
	asked := 0
	askInstallOpenCode = func(string) (bool, error) { asked++; return false, nil }
	t.Cleanup(func() { firstRunWrote = false })
	return &asked
}

// #2781: with OpenCode and Copilot installed, a new install asks nothing,
// picks OpenCode and turns auto-update on.
func TestNewInstallBothClientsAsksNothing(t *testing.T) {
	cfg := isolatedConfig(t)
	forceInteractive(t)
	asked := stubInstalled(t, map[string]bool{"opencode": true, "copilot": true})

	captureRun(t, func() {
		if err := maybeRunFirstRunSetup(""); err != nil {
			t.Fatal(err)
		}
	})
	data, _ := os.ReadFile(cfg)
	if *asked != 0 || !strings.Contains(string(data), `client = "opencode"`) || !strings.Contains(string(data), "auto_update = true") {
		t.Errorf("asked %d, config:\n%s", *asked, data)
	}
	// The scope question is gone too: user scope, no prompt (one would fail
	// on /dev/null).
	repo := t.TempDir()
	scope, err := promptInstallScope(repo)
	if err != nil || scope == nil || !scope.IsUser() {
		t.Errorf("scope = %v, %v; want user scope without a question", scope, err)
	}
}

// With no client installed, the OpenCode question is the only one.
func TestNewInstallNoClientAsksOnlyOpenCode(t *testing.T) {
	cfg := isolatedConfig(t)
	forceInteractive(t)
	origGOOS := installGOOS
	installGOOS = "darwin"
	t.Cleanup(func() { installGOOS = origGOOS })
	asked := stubInstalled(t, nil)

	captureRun(t, func() {
		if err := maybeRunFirstRunSetup(""); err != nil {
			t.Fatal(err)
		}
	})
	if opencodeInstallCommand() != "" && *asked != 1 {
		t.Errorf("OpenCode question asked %d times, want 1", *asked)
	}
	if data, _ := os.ReadFile(cfg); !strings.Contains(string(data), `client = "copilot"`) {
		t.Errorf("declined OpenCode should save copilot:\n%s", data)
	}
}

// An existing config is left alone, keeps the scope question's path, and the
// auto-update nudge shows once.
func TestExistingConfigUnchangedAndNudgedOnce(t *testing.T) {
	cfg := isolatedConfig(t)
	forceInteractive(t)
	stubInstalled(t, map[string]bool{"opencode": true, "copilot": true})
	orig := "version = 1\nclient = \"copilot\"\n"
	if err := os.WriteFile(cfg, []byte(orig), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := maybeRunFirstRunSetup(""); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(cfg); string(data) != orig {
		t.Errorf("config changed:\n%s", data)
	}
	if freshInstall() {
		t.Error("an existing config counted as a new install")
	}
	file, _ := readConfig()
	var out bytes.Buffer
	autoUpdateNudge(&out, file)
	autoUpdateNudge(&out, file)
	if n := strings.Count(out.String(), "nav-pilot config set auto_update true"); n != 1 {
		t.Errorf("nudge shown %d times, want 1:\n%s", n, out.String())
	}
	// Someone who set it either way hears nothing.
	out.Reset()
	off := false
	autoUpdateNudge(&out, &Config{AutoUpdate: &off})
	if out.Len() != 0 {
		t.Errorf("nudged a user who set auto_update: %s", out.String())
	}
}

// A repo lock file still picks repo scope on a new install.
func TestNewInstallLockFileGivesRepoScope(t *testing.T) {
	isolatedConfig(t)
	forceInteractive(t)
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".nav-pilot"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".nav-pilot", "agentpakke.lock.json"), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	scope, err := promptInstallScope(repo)
	if err != nil || scope == nil || scope.IsUser() {
		t.Errorf("scope = %v, %v; want repo scope", scope, err)
	}
}

// #1022 step 3: OpenCode that is not logged in, or outside the tested range,
// gives Copilot CLI when that is installed, without a question.
func TestNewInstallOpenCodeNotReadyGivesCopilot(t *testing.T) {
	for name, tested := range map[string]bool{"not logged in": true, "out of range": false} {
		t.Run(name, func(t *testing.T) {
			cfg := isolatedConfig(t)
			forceInteractive(t)
			asked := stubInstalled(t, map[string]bool{"opencode": true, "copilot": true})
			openCodeTested = func() bool { return tested }
			openCodeLoggedIn = func() bool { return !tested }
			captureRun(t, func() {
				if err := maybeRunFirstRunSetup(""); err != nil {
					t.Fatal(err)
				}
			})
			if data, _ := os.ReadFile(cfg); *asked != 0 || !strings.Contains(string(data), `client = "copilot"`) {
				t.Errorf("asked %d, config:\n%s", *asked, data)
			}
		})
	}
}

// OpenCodeLoggedIn reads opencode's auth.json for a github-copilot entry.
func TestOpenCodeLoggedInReadsAuthJSON(t *testing.T) {
	data := t.TempDir()
	t.Setenv("XDG_DATA_HOME", data)
	if providerpkg.OpenCodeLoggedIn() {
		t.Fatal("logged in without auth.json")
	}
	_ = os.MkdirAll(filepath.Join(data, "opencode"), 0o700)
	_ = os.WriteFile(filepath.Join(data, "opencode", "auth.json"), []byte(`{"github-copilot":{"type":"oauth"}}`), 0o600)
	if !providerpkg.OpenCodeLoggedIn() {
		t.Error("auth.json with github-copilot not read as logged in")
	}
}
