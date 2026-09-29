package cli

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func boolp(b bool) *bool { return &b }

// Each preset writes exactly the keys it needs, from each starting point, and
// nothing that is already right.
func TestPresetWritesExactlyTheRightKeys(t *testing.T) {
	std := cpltGitState{Preset: "standard"}
	strict := cpltGitState{Preset: "strict"}
	stdPinned := cpltGitState{Preset: "standard", Protect: boolp(true)} // a file that sets the key, like many do
	for _, tc := range []struct {
		name   string
		preset string
		from   cpltGitState
		want   []cpltChange
	}{
		{"sandbox from default", presetSandbox, std, nil},
		{"ask from default", presetAsk, std, nil},
		{"locked from default", presetLocked, std, []cpltChange{{"sandbox.preset", "standard", "strict"}}},
		{"locked with the guard pinned open", presetLocked, stdPinned, []cpltChange{
			{"sandbox.preset", "standard", "strict"},
			{"git_guard.protect_default_branch_only", "true", "false"},
		}},
		{"sandbox from strict", presetSandbox, strict, []cpltChange{{"sandbox.preset", "strict", "standard"}}},
		{"sandbox from a pushes-off standard", presetSandbox, cpltGitState{Preset: "standard", Protect: boolp(false)},
			[]cpltChange{{"git_guard.protect_default_branch_only", "false", "true"}}},
		{"locked from locked", presetLocked, strict, nil},
		{"sandbox from permissive", presetSandbox, cpltGitState{Preset: "permissive"}, []cpltChange{{"sandbox.preset", "permissive", "standard"}}},
		{"cplt said nothing", presetSandbox, cpltGitState{}, []cpltChange{{"sandbox.preset", "", "standard"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := cpltChanges(tc.from, presetChoices[tc.preset]); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

// The mapping itself, pinned so a swapped value fails here.
func TestPresetMapping(t *testing.T) {
	want := map[string]autonomyChoice{
		presetSandbox: {"sandbox", "standard", true},
		presetAsk:     {"conservative", "standard", true},
		presetLocked:  {"conservative", "strict", false},
	}
	if !reflect.DeepEqual(presetChoices, want) {
		t.Errorf("presetChoices = %v, want %v", presetChoices, want)
	}
}

// Custom combinations the presets do not cover.
func TestCustomChoiceKeys(t *testing.T) {
	std := cpltGitState{Preset: "standard"}
	if got := cpltChanges(std, autonomyChoice{Preset: "standard", Push: false}); !reflect.DeepEqual(got,
		[]cpltChange{{"git_guard.protect_default_branch_only", "", "false"}}) {
		t.Errorf("commit only: %v", got)
	}
	if got := cpltChanges(std, autonomyChoice{Preset: "strict", Push: true}); !reflect.DeepEqual(got, []cpltChange{
		{"sandbox.preset", "standard", "strict"},
		{"git_guard.protect_default_branch_only", "", "true"},
	}) {
		t.Errorf("allowlist with pushes: %v", got)
	}
	// Keeping permissive writes nothing: its git guard is off anyway.
	if got := cpltChanges(cpltGitState{Preset: "permissive"}, autonomyChoice{Preset: "permissive", Push: true}); got != nil {
		t.Errorf("keep permissive: %v", got)
	}
}

// Rerunning setup preselects what the machine has today.
func TestCurrentPresetIsTheDefault(t *testing.T) {
	for _, tc := range []struct {
		autonomy string
		copilot  bool
		s        cpltGitState
		want     string
	}{
		{"sandbox", true, cpltGitState{Preset: "standard"}, presetSandbox},
		{"conservative", true, cpltGitState{Preset: "standard"}, presetAsk},
		{"conservative", true, cpltGitState{Preset: "strict"}, presetLocked},
		{"sandbox", true, cpltGitState{Preset: "strict"}, presetCustom},
		{"conservative", true, cpltGitState{Preset: "standard", Protect: boolp(false)}, presetCustom},
		{"conservative", true, cpltGitState{Preset: "strict", Protect: boolp(true)}, presetCustom},
		{"conservative", true, cpltGitState{Preset: "permissive"}, presetCustom},
		{"sandbox", true, cpltGitState{}, presetSandbox}, // cplt could not say: its default
		{"conservative", false, cpltGitState{Preset: "standard"}, presetSandbox},
		{"sandbox", false, cpltGitState{Preset: "strict"}, presetLocked},
	} {
		if got := currentPreset(tc.autonomy, tc.copilot, tc.s); got != tc.want {
			t.Errorf("currentPreset(%q, copilot=%v, %+v) = %q, want %q", tc.autonomy, tc.copilot, tc.s, got, tc.want)
		}
	}
}

// Through a real process: cplt's "(default" marker means the file does not
// set the key, and the writes land in that order.
func TestCpltStateAndWritesThroughCplt(t *testing.T) {
	isolatedConfig(t)
	log := fakeCplt(t, map[string]string{
		"sandbox.preset":                        "standard",
		"git_guard.protect_default_branch_only": "true\n[cplt] (default, not set in config file)",
	})
	cliPath, err := findCplt()
	if err != nil {
		t.Fatal(err)
	}
	s := readCpltGitState(cliPath)
	if s.Preset != "standard" || s.Protect != nil {
		t.Fatalf("state = %+v, want standard with the key unset", s)
	}
	if err := applyCpltChanges(cliPath, cpltChanges(s, presetChoices[presetLocked]), "", ""); err != nil {
		t.Fatal(err)
	}
	sets := configSets(t, log)
	if sets["sandbox.preset"] != "strict" || sets["proxy.allowed_domains"] == "" || len(sets) != 2 {
		t.Errorf("locked down wrote %v, want sandbox.preset and the seeded allowlist only", sets)
	}
}

func TestExplicitProtectKeyIsRead(t *testing.T) {
	isolatedConfig(t)
	fakeCplt(t, map[string]string{"sandbox.preset": "strict", "git_guard.protect_default_branch_only": "true"})
	cliPath, _ := findCplt()
	s := readCpltGitState(cliPath)
	if s.Protect == nil || !*s.Protect || !s.effectivePush() {
		t.Errorf("state = %+v, want an explicit true that allows pushes", s)
	}
}

func TestGHAuthFrom(t *testing.T) {
	exit := func(code string) error {
		return exec.Command("sh", "-c", "exit "+code).Run()
	}
	for _, tc := range []struct {
		name string
		out  string
		err  error
		want ghAuth
	}{
		{"file token", "✓ Logged in to github.com account x (/home/x/.config/gh/hosts.yml)", nil, ghAuthOK},
		{"keyring", "✓ Logged in to github.com account x (keyring)", nil, ghAuthKeyring},
		{"not signed in", "You are not logged into any GitHub hosts.", exit("1"), ghAuthNone},
		{"killed at the deadline", "", exit("137"), ghAuthUnknown},
		{"did not start", "", errors.New("boom"), ghAuthUnknown},
	} {
		if got, _ := ghAuthFrom([]byte(tc.out), tc.err); got != tc.want {
			t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestReportGHAuth(t *testing.T) {
	stubSSH(t, false)
	out := func(client string, st ghAuth) string {
		var b bytes.Buffer
		reportGHAuth(&b, "", client, st, "timeout")
		return b.String()
	}
	if s := out("copilot", ghAuthNone); !strings.Contains(s, "gh auth login") || strings.Contains(s, "insecure") {
		t.Errorf("copilot, not signed in: %q", s)
	}
	if s := out("copilot", ghAuthKeyring); !strings.Contains(s, "signed in") {
		t.Errorf("copilot, keyring: %q", s)
	}
	if s := out("copilot", ghAuthMissing); !strings.Contains(s, "not installed") {
		t.Errorf("missing: %q", s)
	}
	if s := out("copilot", ghAuthUnknown); !strings.Contains(s, "timeout") {
		t.Errorf("unknown: %q", s)
	}
	s := out("opencode", ghAuthKeyring)
	if runtime.GOOS == "darwin" {
		if !strings.Contains(s, "gh auth login --insecure-storage") {
			t.Errorf("opencode on macOS with a Keychain token: %q", s)
		}
	} else if !strings.Contains(s, "signed in") {
		t.Errorf("opencode, keyring off macOS: %q", s)
	}
}

// The "(default" line is what tells an unset key from a set one.
func TestProtectKeyWithoutDefaultMarkerIsExplicit(t *testing.T) {
	isolatedConfig(t)
	fakeCplt(t, map[string]string{"git_guard.protect_default_branch_only": "false"})
	cliPath, _ := findCplt()
	if s := readCpltGitState(cliPath); s.Protect == nil || *s.Protect {
		t.Errorf("state = %+v, want an explicit false", s)
	}
}

func stubSSH(t *testing.T, ssh bool) {
	t.Helper()
	orig := originUsesSSH
	originUsesSSH = func() bool { return ssh }
	t.Cleanup(func() { originUsesSSH = orig })
}

// gh covers HTTPS pushes only; over an SSH origin the line must not promise
// a push, and must name the insteadOf rewrite.
func TestReportGHAuthSSHOrigin(t *testing.T) {
	stubSSH(t, true)
	var b bytes.Buffer
	reportGHAuth(&b, "", "copilot", ghAuthOK, "")
	if s := b.String(); strings.Contains(s, "push branches") || !strings.Contains(s, "insteadOf") {
		t.Errorf("SSH origin: %q", s)
	}
}

// Under permissive the guards are off: the summary says so and does not
// claim the agent cannot push.
func TestSummaryPermissiveAndKeptAllowlist(t *testing.T) {
	isolatedConfig(t)
	s := autonomySummary(autonomyChoice{Autonomy: "sandbox", Preset: "permissive"}, nil, true, "")
	if !strings.Contains(s, "guards are off") || strings.Contains(s, "cannot push") {
		t.Errorf("permissive summary: %q", s)
	}
	s = autonomySummary(presetChoices[presetSandbox], nil, true, navAllowedDomainsPath())
	if !strings.Contains(s, "allowed_domains stays set") || !strings.Contains(s, "nav-pilot's list") {
		t.Errorf("kept nav-pilot allowlist: %q", s)
	}
	s = autonomySummary(presetChoices[presetSandbox], nil, true, "/home/me/hosts.txt")
	if !strings.Contains(s, "stays set") || strings.Contains(s, "nav-pilot's list") {
		t.Errorf("kept user allowlist: %q", s)
	}
}

// Only nav-pilot's own list, and only when leaving strict, is offered for
// removal; a list the user chose stays.
func TestLeavingStrictAllowlist(t *testing.T) {
	isolatedConfig(t)
	nav := navAllowedDomainsPath()
	for _, tc := range []struct {
		from, to, allowlist string
		want                bool
	}{
		{"strict", "standard", nav, true},
		{"strict", "permissive", nav, true},
		{"strict", "standard", "/home/me/hosts.txt", false},
		{"strict", "standard", "", false},
		{"standard", "standard", nav, false},
		{"strict", "strict", nav, false},
	} {
		got := leavingStrictAllowlist(tc.from, tc.to, tc.allowlist)
		if (got != nil) != tc.want {
			t.Errorf("leavingStrictAllowlist(%q, %q, %q) = %v, want offer=%v", tc.from, tc.to, tc.allowlist, got, tc.want)
		}
		if got != nil && (got.Key != "proxy.allowed_domains" || got.To != "") {
			t.Errorf("drop change = %+v", *got)
		}
	}
}

// Through a real process: leaving strict with nav-pilot's list and saying yes
// unsets the key in the global config. It must never write an empty value,
// which leaves an allowlist that blocks every host.
func TestLeavingStrictUnsetsNavAllowlist(t *testing.T) {
	isolatedConfig(t)
	log := fakeCplt(t, map[string]string{
		"sandbox.preset":                        "strict",
		"git_guard.protect_default_branch_only": "false\n[cplt] (default, not set in config file)",
		"proxy.allowed_domains":                 navAllowedDomainsPath(),
	})
	cliPath, err := findCplt()
	if err != nil {
		t.Fatal(err)
	}
	s := readCpltGitState(cliPath)
	choice := presetChoices[presetSandbox]
	changes := cpltChanges(s, choice)
	drop := leavingStrictAllowlist(s.Preset, choice.Preset, cpltConfigGet(cliPath, "proxy.allowed_domains"))
	if drop == nil {
		t.Fatal("no offer to drop nav-pilot's allowlist")
	}
	changes = append(changes, *drop)
	if sum := autonomySummary(choice, changes, true, ""); !strings.Contains(sum, "proxy.allowed_domains: "+navAllowedDomainsPath()+" → unset") {
		t.Errorf("summary does not show the removal: %q", sum)
	}
	if err := applyCpltChanges(cliPath, changes, "", ""); err != nil {
		t.Fatal(err)
	}
	args, err := os.ReadFile(log + ".args")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(args), "config set proxy.allowed_domains --unset --global\n") {
		t.Errorf("cplt calls:\n%s\nwant config set proxy.allowed_domains --unset --global", args)
	}
	if sets := configSets(t, log); sets["sandbox.preset"] != "standard" || sets["proxy.allowed_domains"] != "--unset" || len(sets) != 2 {
		t.Errorf("wrote %v, want sandbox.preset = standard and the allowlist unset", sets)
	}
}
