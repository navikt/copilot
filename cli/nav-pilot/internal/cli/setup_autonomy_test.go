package cli

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/navikt/copilot/cli/nav-pilot/internal/testhome"
)

func boolp(b bool) *bool { return &b }

// Each git answer, and each network answer of --advanced, writes exactly the
// keys it needs from each starting point, and nothing that is already right.
// The default flow keeps the preset, so only the guard key can change there.
func TestGitAnswerWritesExactlyTheRightKeys(t *testing.T) {
	std := cpltGitState{Preset: "standard"}
	strict := cpltGitState{Preset: "strict"}
	stdPinned := cpltGitState{Preset: "standard", Protect: boolp(true)} // a file that sets the key, like many do
	stdOff := cpltGitState{Preset: "standard", Protect: boolp(false)}
	push, commitOnly := true, false
	for _, tc := range []struct {
		name   string
		from   cpltGitState
		preset string // "" keeps from's preset: the default flow
		push   bool
		want   []cpltChange
	}{
		{"push from default", std, "", push, nil},
		{"commit only from default", std, "", commitOnly, []cpltChange{{"git_guard.protect_default_branch_only", "", "false"}}},
		{"push from pinned true", stdPinned, "", push, nil},
		{"commit only from pinned true", stdPinned, "", commitOnly, []cpltChange{{"git_guard.protect_default_branch_only", "true", "false"}}},
		{"push from pushes off", stdOff, "", push, []cpltChange{{"git_guard.protect_default_branch_only", "false", "true"}}},
		{"commit only from pushes off", stdOff, "", commitOnly, nil},
		{"commit only on strict", strict, "", commitOnly, nil},
		{"push on strict", strict, "", push, []cpltChange{{"git_guard.protect_default_branch_only", "", "true"}}},
		{"permissive: guard off, nothing to write", cpltGitState{Preset: "permissive"}, "", push, nil},
		{"advanced: to strict, commit only", std, "strict", commitOnly, []cpltChange{{"sandbox.preset", "standard", "strict"}}},
		{"advanced: to strict with the guard pinned open", stdPinned, "strict", commitOnly, []cpltChange{
			{"sandbox.preset", "standard", "strict"},
			{"git_guard.protect_default_branch_only", "true", "false"},
		}},
		{"advanced: strict to standard, push", strict, "standard", push, []cpltChange{{"sandbox.preset", "strict", "standard"}}},
		{"advanced: permissive to standard, push", cpltGitState{Preset: "permissive"}, "standard", push, []cpltChange{{"sandbox.preset", "permissive", "standard"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			preset := tc.preset
			if preset == "" {
				preset = tc.from.Preset
			}
			if got := cpltChanges(tc.from, autonomyChoice{Preset: preset, Push: tc.push}); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
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
	if err := applyCpltChanges(cliPath, cpltChanges(s, autonomyChoice{Preset: "strict", Push: false}), "", ""); err != nil {
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
	s = autonomySummary(autonomyChoice{Autonomy: "sandbox", Preset: "standard", Push: true}, nil, true, navAllowedDomainsPath())
	if !strings.Contains(s, "allowed_domains stays set") || !strings.Contains(s, "nav-pilot's list") {
		t.Errorf("kept nav-pilot allowlist: %q", s)
	}
	s = autonomySummary(autonomyChoice{Autonomy: "sandbox", Preset: "standard", Push: true}, nil, true, "/home/me/hosts.txt")
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

// statefulCplt is a cplt whose config get reads back what config set wrote,
// with the preset on strict and nav-pilot's allowlist set. A set of
// failKey exits 1. It returns the log of every config set, in order.
func statefulCplt(t *testing.T, failKey string) string {
	t.Helper()
	dir := t.TempDir()
	for k, v := range map[string]string{"sandbox.preset": "strict", "proxy.allowed_domains": navAllowedDomainsPath()} {
		if err := os.WriteFile(filepath.Join(dir, k), []byte(v+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	log := filepath.Join(dir, "set.log")
	script := fmt.Sprintf(`#!/bin/sh
cd %q
case "$1 $2" in
  "config set") printf '%%s\n' "$*" >> set.log
    [ "$3" = %q ] && exit 1
    if [ "$4" = --unset ]; then rm -f "$3"; else echo "$4" > "$3"; fi ;;
  "config get") [ -f "$3" ] && cat "$3" ;;
  *) exit 1 ;;
esac
`, dir, failKey)
	if err := testhome.WriteExec(filepath.Join(dir, "cplt"), script); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return log
}

// Leaving strict with nav-pilot's list and saying yes: the preset is written
// first, then the key is unset (never set to "", which blocks every host).
func TestLeavingStrictUnsetsNavAllowlist(t *testing.T) {
	isolatedConfig(t)
	log := statefulCplt(t, "")
	cliPath, err := findCplt()
	if err != nil {
		t.Fatal(err)
	}
	choice := autonomyChoice{Autonomy: "sandbox", Preset: "standard", Push: true}
	drop := leavingStrictAllowlist(cpltConfigGet(cliPath, "sandbox.preset"), choice.Preset, cpltConfigGet(cliPath, "proxy.allowed_domains"))
	if drop == nil {
		t.Fatal("no offer to drop nav-pilot's allowlist")
	}
	changes := []cpltChange{*drop, {"sandbox.preset", "strict", "standard"}}
	if sum := autonomySummary(choice, changes, true, ""); !strings.Contains(sum, "proxy.allowed_domains: "+navAllowedDomainsPath()+" → unset") {
		t.Errorf("summary does not show the removal: %q", sum)
	}
	if err := applyCpltChanges(cliPath, changes, "", ""); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(log)
	want := "config set sandbox.preset standard\nconfig set proxy.allowed_domains --unset --global\n"
	if string(got) != want {
		t.Errorf("cplt calls:\n%s\nwant:\n%s", got, want)
	}
}

// If the preset write fails, the allowlist stays and the error says so: the
// user is still on strict and needs the Nav hosts.
func TestLeavingStrictKeepsAllowlistWhenPresetFails(t *testing.T) {
	isolatedConfig(t)
	log := statefulCplt(t, "sandbox.preset")
	cliPath, err := findCplt()
	if err != nil {
		t.Fatal(err)
	}
	changes := []cpltChange{{"proxy.allowed_domains", navAllowedDomainsPath(), ""}, {"sandbox.preset", "strict", "standard"}}
	err = applyCpltChanges(cliPath, changes, "", "")
	if err == nil || !strings.Contains(err.Error(), "proxy.allowed_domains is left in place") {
		t.Errorf("err = %v, want it to say the allowlist is left in place", err)
	}
	if got, _ := os.ReadFile(log); strings.Contains(string(got), "--unset") {
		t.Errorf("unset after a failed preset write:\n%s", got)
	}
	if got := cpltConfigGet(cliPath, "proxy.allowed_domains"); got != navAllowedDomainsPath() {
		t.Errorf("allowlist = %q, want it kept", got)
	}
}
