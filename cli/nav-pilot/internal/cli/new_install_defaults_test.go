package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

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
	startedFresh = noNavPilotState()
	t.Cleanup(func() { firstRunWrote, startedFresh = false, false })
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
	firstRunWrote = true // the first run just wrote a new install's config
	t.Cleanup(func() { firstRunWrote = false })
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

// An existing user without config.toml (here: the client marker) keeps
// today's path: the wizard, no silent client, auto_update or scope.
func TestMarkerWithoutConfigIsNotANewInstall(t *testing.T) {
	cfg := isolatedConfig(t)
	forceInteractive(t)
	if err := os.WriteFile(filepath.Join(filepath.Dir(cfg), "seen-client-recorded"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	stubInstalled(t, map[string]bool{"opencode": true, "copilot": true})
	wizard := 0
	orig := runConfigSetupFn
	runConfigSetupFn = func(string) error { wizard++; return nil }
	t.Cleanup(func() { runConfigSetupFn = orig })
	if err := maybeRunFirstRunSetup(""); err != nil {
		t.Fatal(err)
	}
	if wizard != 1 {
		t.Errorf("wizard ran %d times, want 1", wizard)
	}
	if _, err := os.Stat(cfg); !os.IsNotExist(err) {
		t.Error("config written without the wizard")
	}
	if freshInstall() {
		t.Error("marker present but treated as a new install")
	}
}

// A user-scope state file in ~/.copilot also means nav-pilot was here.
func TestUserStateFileIsNotANewInstall(t *testing.T) {
	isolatedConfig(t)
	home, _ := os.UserHomeDir()
	if !noNavPilotState() {
		t.Fatal("empty test home counted as state")
	}
	_ = os.MkdirAll(filepath.Join(home, ".copilot"), 0o700)
	if err := os.WriteFile(filepath.Join(home, ".copilot", ".nav-pilot-state.json"), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if noNavPilotState() {
		t.Error("user-scope state file ignored")
	}
}

// OpenCodeLoggedIn asks opencode (a fake here): a github-copilot entry with a
// connection is logged in; anything else, or a failing binary, is not.
func TestOpenCodeLoggedInAsksOpenCode(t *testing.T) {
	for name, tc := range map[string]struct {
		script string
		want   bool
	}{
		"logged in":     {`echo '[{"id":"github-copilot","connections":[{"type":"credential"}]}]'`, true},
		"empty":         {`echo '[]'`, false},
		"other only":    {`echo '[{"id":"openai","connections":[{}]}]'`, false},
		"no connection": {`echo '[{"id":"github-copilot","connections":[]}]'`, false},
		"garbage":       {`echo 'No authenticated integrations'`, false},
		"fails":         {`exit 1`, false},
	} {
		t.Run(name, func(t *testing.T) {
			bin := t.TempDir()
			if err := os.WriteFile(filepath.Join(bin, "opencode"), []byte("#!/bin/sh\n"+tc.script+"\n"), 0o755); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", bin)
			if got := providerpkg.OpenCodeLoggedIn(); got != tc.want {
				t.Errorf("OpenCodeLoggedIn() = %v, want %v", got, tc.want)
			}
		})
	}
}

// The nudge waits for a terminal and respects an earlier No to "Upgrade now?".
func TestAutoUpdateNudgeQuietWhenNotInteractiveOrDeclined(t *testing.T) {
	isolatedConfig(t)
	var out bytes.Buffer
	autoUpdateNudge(&out, &Config{}) // not interactive
	forceInteractive(t)
	if err := os.WriteFile(updateDeclinedPath(), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	autoUpdateNudge(&out, &Config{})
	if out.Len() != 0 {
		t.Errorf("nudged anyway: %s", out.String())
	}
	_ = os.Remove(updateDeclinedPath())
	autoUpdateNudge(&out, &Config{})
	if !strings.Contains(out.String(), "auto_update true") {
		t.Error("nudge not shown once the decline is gone; a non-interactive run used it up")
	}
}

// Any one piece of earlier nav-pilot state means not a new install (#2781).
func TestPartialStateIsNotANewInstall(t *testing.T) {
	cases := map[string]func(t *testing.T, home, cfgDir string){
		"cache.json only": func(t *testing.T, _, cfgDir string) {
			touchFile(t, filepath.Join(cfgDir, "cache.json"))
		},
		"device-id only": func(t *testing.T, _, cfgDir string) {
			touchFile(t, filepath.Join(cfgDir, "device-id"))
		},
		"relocated config, old ~/.nav-pilot": func(t *testing.T, home, _ string) {
			touchFile(t, filepath.Join(home, ".nav-pilot", "config.toml"))
		},
		"unreadable config dir": func(t *testing.T, _, cfgDir string) {
			if err := os.Chmod(cfgDir, 0); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.Chmod(cfgDir, 0o700) })
		},
	}
	for name, setup := range cases {
		t.Run(name, func(t *testing.T) {
			cfgDir := t.TempDir()
			t.Setenv("NAV_PILOT_CONFIG", filepath.Join(cfgDir, "config.toml"))
			home := t.TempDir()
			t.Setenv("HOME", home)
			if !noNavPilotState() {
				t.Fatal("empty state counted as earlier use")
			}
			setup(t, home, cfgDir)
			if noNavPilotState() {
				t.Error("earlier state ignored")
			}
		})
	}
}

func touchFile(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
}

// opencode auth list runs outside the caller's directory and without
// OPENCODE_* variables, so a repo's .opencode/plugin/ never loads on the host.
func TestOpenCodeLoggedInRunsOutsideTheRepo(t *testing.T) {
	bin, rec := t.TempDir(), filepath.Join(t.TempDir(), "rec")
	script := "#!/bin/sh\n/bin/pwd > " + rec + "\n/usr/bin/env >> " + rec + "\necho '[]'\n"
	if err := os.WriteFile(filepath.Join(bin, "opencode"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	t.Setenv("OPENCODE_CONFIG_DIR", "/should/not/pass")
	repo := t.TempDir()
	t.Chdir(repo)
	providerpkg.OpenCodeLoggedIn()
	data, err := os.ReadFile(rec)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.SplitN(string(data), "\n", 2)
	if real, _ := filepath.EvalSymlinks(repo); lines[0] == repo || lines[0] == real {
		t.Errorf("ran in the caller's directory %s", lines[0])
	}
	if strings.Contains(string(data), "OPENCODE_CONFIG_DIR") {
		t.Error("OPENCODE_* variable passed to opencode")
	}
}

// A child that keeps stdout open must not stretch the 5 s limit.
func TestOpenCodeLoggedInTimeoutHoldsWithAChild(t *testing.T) {
	bin := t.TempDir()
	script := "#!/bin/sh\n/bin/sleep 30 &\n/bin/sleep 30\n"
	if err := os.WriteFile(filepath.Join(bin, "opencode"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	start := time.Now()
	if providerpkg.OpenCodeLoggedIn() {
		t.Error("a hung opencode counted as logged in")
	}
	if d := time.Since(start); d > 8*time.Second {
		t.Errorf("took %s, want about 6 s at most", d)
	}
}

// existingUser isolates the config and writes one, so the run is not a new
// install.
func existingUser(t *testing.T) string {
	t.Helper()
	cfg := isolatedConfig(t)
	if err := os.WriteFile(cfg, []byte("version = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return cfg
}

// #2785 review: OpenCode installed but not ready, and no Copilot CLI, still
// saves Copilot without a question; the launch then says how to install it.
func TestNewInstallOpenCodeNotReadyNoCopilotGivesCopilot(t *testing.T) {
	cfg := isolatedConfig(t)
	forceInteractive(t)
	asked := stubInstalled(t, map[string]bool{"opencode": true})
	openCodeLoggedIn = func() bool { return false }
	captureRun(t, func() {
		if err := maybeRunFirstRunSetup(""); err != nil {
			t.Fatal(err)
		}
	})
	if data, _ := os.ReadFile(cfg); *asked != 0 || !strings.Contains(string(data), `client = "copilot"`) {
		t.Errorf("asked %d, config:\n%s", *asked, data)
	}
}

// hookPakkeSource is pakkeSource with one hook.
func hookPakkeSource(t *testing.T, repo string) *Source {
	t.Helper()
	dir := pakkeSourceTree(t, `{"contractVersion":"1","name":"grillmester","description":"g",
"clients":{"copilot":{"primaryAgents":["grillmester"]}},
"layout":{"agents":"plugin/agents","skills":"plugin/skills","hooks":"hooks"}}`)
	mustWrite(t, filepath.Join(dir, "hooks", "vakt.hook.json"), `{"matcher": "bash", "timeoutSec": 5}`)
	mustWrite(t, filepath.Join(dir, "hooks", "vakt.py"), "print('{}')\n")
	src := &Source{Dir: dir, SHA: "def5678" + strings.Repeat("0", 33), Version: "dev", Repo: repo}
	if err := attachPakke(src); err != nil {
		t.Fatalf("attachPakke: %v", err)
	}
	return src
}

// A new install that starts with `nav-pilot install` gets the first-run
// defaults too: no scope question, no picker, user scope, a config with
// auto_update on. Nav's hooks are not asked about; another source's are,
// unless --yes said so (#2785 review).
func TestRunInstallOnNewInstall(t *testing.T) {
	for name, tc := range map[string]struct {
		repo      string
		args      []string
		wantHooks int
	}{
		"nav source":           {defaultSourceRepo, []string{"install"}, 0},
		"third-party source":   {"navikt/grillmester", []string{"install"}, 1},
		"third-party, --yes":   {"navikt/grillmester", []string{"install", "--yes"}, 0},
		"named, no scope flag": {defaultSourceRepo, []string{"install", "grillmester"}, 0},
	} {
		t.Run(name, func(t *testing.T) {
			cfg := isolatedConfig(t)
			home := t.TempDir()
			t.Setenv("HOME", home)
			forceInteractive(t)
			stubInstalled(t, map[string]bool{"copilot": true})
			stubResolveSource(t, hookPakkeSource(t, tc.repo))
			scopeAsked := stubScopePrompt(t, nil)
			hooks := 0
			origHooks := askHooks
			askHooks = func(string, string) (bool, error) { hooks++; return true, nil }
			t.Cleanup(func() { askHooks = origHooks })
			t.Chdir(repoTarget(t))

			captureRun(t, func() {
				if err := run(tc.args); err != nil {
					t.Fatalf("run %v: %v", tc.args, err)
				}
			})
			if *scopeAsked {
				t.Error("a new install was asked where to install")
			}
			if hooks != tc.wantHooks {
				t.Errorf("hook question asked %d times, want %d", hooks, tc.wantHooks)
			}
			if _, err := os.Stat(filepath.Join(home, ".copilot", "agents", "grillmester.agent.md")); err != nil {
				t.Errorf("not installed to user scope: %v", err)
			}
			if data, _ := os.ReadFile(cfg); !strings.Contains(string(data), "auto_update = true") {
				t.Errorf("first-run config not written:\n%s", data)
			}
		})
	}
}

// Every stateMarker name nav-pilot writes counts as earlier use, and so do
// the caches and consent files the review named.
func TestStateNamesCoverStateMarkers(t *testing.T) {
	names := []string{"tier-cache.json", "pakke-consent.json", "pakker", "cache.json", "device-id"}
	src, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	re := regexp.MustCompile(`stateMarker\("([^"]+)"\)`)
	for _, f := range src {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		data, _ := os.ReadFile(f)
		for _, m := range re.FindAllStringSubmatch(string(data), -1) {
			names = append(names, m[1])
		}
	}
	for _, name := range names {
		matched := false
		for _, p := range navPilotStateNames {
			if ok, _ := filepath.Match(p, name); ok {
				matched = true
			}
		}
		if !matched {
			t.Errorf("%s is nav-pilot state but not in navPilotStateNames", name)
		}
	}
}

// A first launch that self-updates re-executes after telemetry wrote its
// device id; the new process is still a new install, but only for the state
// the parent listed and only inside our own re-exec (#2785 review).
func TestFreshnessSurvivesSelfUpdateReexec(t *testing.T) {
	cfg := isolatedConfig(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	if !noNavPilotState() {
		t.Fatal("empty home counted as state")
	}
	// What the first process wrote before its update re-executed it.
	touchFile(t, filepath.Join(filepath.Dir(cfg), "device-id"))
	touchFile(t, filepath.Join(home, ".nav-pilot", "seen-telemetry-notice"))
	found, _ := navPilotState()
	listed := strings.Join(found, "\n")

	t.Setenv(reexecGuardEnv, "1")
	t.Setenv(freshStateEnv, listed)
	if !freshAfterReexec() {
		t.Error("the re-executed first launch lost its new-install status")
	}
	if _, set := os.LookupEnv(freshStateEnv); set {
		t.Error("the variable is passed on to later children")
	}

	// Not our re-exec: the variable is ignored.
	t.Setenv(reexecGuardEnv, "")
	t.Setenv(freshStateEnv, listed)
	if freshAfterReexec() {
		t.Error("the variable counted outside a re-exec")
	}

	// State the parent did not list means an existing user.
	t.Setenv(reexecGuardEnv, "1")
	t.Setenv(freshStateEnv, listed)
	touchFile(t, filepath.Join(home, ".nav-pilot", "cache.json"))
	if freshAfterReexec() {
		t.Error("unlisted state excused by the variable")
	}

	// A config always means an existing user.
	t.Setenv(freshStateEnv, listed+"\n"+filepath.Join(home, ".nav-pilot", "cache.json"))
	touchFile(t, cfg)
	if freshAfterReexec() {
		t.Error("a config excused by the variable")
	}
}

// Glob metacharacters in the config directory do not hide its state.
func TestStateFoundUnderMetacharacterDir(t *testing.T) {
	isolatedConfig(t)
	t.Setenv("HOME", t.TempDir())
	dir := filepath.Join(t.TempDir(), "config[work]*?")
	t.Setenv("NAV_PILOT_CONFIG", filepath.Join(dir, "config.toml"))
	if !noNavPilotState() {
		t.Fatal("empty dir counted as state")
	}
	touchFile(t, filepath.Join(dir, "device-id"))
	if noNavPilotState() {
		t.Error("device-id under a metacharacter dir not found")
	}
}

// A malformed install writes no config and bootstraps nothing; the usage
// error comes first (#2785 review).
func TestRunInstallBadArgsOnNewInstallWritesNothing(t *testing.T) {
	cfg := isolatedConfig(t)
	t.Setenv("HOME", t.TempDir())
	forceInteractive(t)
	stubInstalled(t, map[string]bool{"copilot": true})
	ran := 0
	orig := firstRunFn
	firstRunFn = func(string) error { ran++; return nil }
	t.Cleanup(func() { firstRunFn = orig })
	var err error
	captureRun(t, func() { err = run([]string{"install", "one", "two"}) })
	if err == nil || !strings.Contains(err.Error(), "install takes one name") {
		t.Errorf("err = %v, want the usage error", err)
	}
	if ran != 0 {
		t.Error("first-run setup ran before the usage error")
	}
	if _, statErr := os.Stat(cfg); !os.IsNotExist(statErr) {
		t.Error("config written")
	}
}

// With no client installed, an install that said yes up front saves the
// defaults without offering to install opencode (#2785 review).
func TestRunInstallYesOnNewInstallAsksNoClientQuestion(t *testing.T) {
	cfg := isolatedConfig(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	forceInteractive(t)
	origGOOS := installGOOS
	installGOOS = "darwin"
	t.Cleanup(func() { installGOOS = origGOOS })
	asked := stubInstalled(t, nil)
	stubResolveSource(t, pakkeSource(t, defaultSourceRepo))
	t.Chdir(repoTarget(t))
	captureRun(t, func() {
		if err := run([]string{"install", "--user", "--all", "--yes"}); err != nil {
			t.Fatal(err)
		}
	})
	if *asked != 0 {
		t.Errorf("opencode install offered %d times", *asked)
	}
	if data, _ := os.ReadFile(cfg); !strings.Contains(string(data), `client = "copilot"`) {
		t.Errorf("defaults not saved:\n%s", data)
	}
}

// The login probe tells opencode to skip project config, and is skipped
// when its temp dir would sit inside the checkout (#2785 review).
func TestOpenCodeLoggedInProbeStaysOutOfTheCheckout(t *testing.T) {
	bin, rec := t.TempDir(), filepath.Join(t.TempDir(), "rec")
	script := "#!/bin/sh\n/usr/bin/env > " + rec + "\necho '[{\"id\":\"github-copilot\",\"connections\":[{}]}]'\n"
	if err := os.WriteFile(filepath.Join(bin, "opencode"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	repo := repoTarget(t)
	t.Chdir(repo)
	if !providerpkg.OpenCodeLoggedIn() {
		t.Fatal("probe outside the checkout failed")
	}
	if data, _ := os.ReadFile(rec); !strings.Contains(string(data), "OPENCODE_DISABLE_PROJECT_CONFIG=1") {
		t.Error("project config not disabled for the probe")
	}

	_ = os.Remove(rec)
	sub := filepath.Join(repo, "tmp")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TMPDIR", sub)
	t.Chdir(filepath.Join(repo, ".git")) // cwd below the root: the git root still counts
	if providerpkg.OpenCodeLoggedIn() {
		t.Error("probe ran with its temp dir inside the checkout")
	}
	if _, err := os.Stat(rec); err == nil {
		t.Error("opencode ran")
	}
}
