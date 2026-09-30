package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func autonomyOf(t *testing.T) string {
	t.Helper()
	cfg, err := readConfig()
	if err != nil || cfg == nil {
		t.Fatalf("readConfig: %v, %v", cfg, err)
	}
	if cfg.Autonomy == nil {
		return "(unset)"
	}
	if autonomyChosenConservative(cfg) {
		return "conservative (chosen)"
	}
	return *cfg.Autonomy
}

// markUsedBefore leaves one of the signs of an earlier run.
var markUsedBefore = map[string]func(t *testing.T, cfgPath string){
	"cplt hint marker": func(t *testing.T, cfgPath string) {
		if err := os.WriteFile(filepath.Join(filepath.Dir(cfgPath), "seen-cplt-hint"), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	},
	"user state file": func(t *testing.T, _ string) {
		dir := filepath.Join(os.Getenv("HOME"), ".copilot")
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, ".nav-pilot-state.json"), []byte("{}"), 0o600); err != nil {
			t.Fatal(err)
		}
	},
}

// Sandbox for everyone, except a conservative the user chose. An
// autonomy = "conservative" without autonomy_chosen was written by #1349's
// config init or the #1350 wizard's preselected answer, and counts as not
// chosen. The notice goes to everyone who was on conservative before.
func TestResolveAutonomy(t *testing.T) {
	isolatedConfig(t)
	s, c, yes := "sandbox", "conservative", true
	for _, tc := range []struct {
		name       string
		file       *Config
		want       string
		wantNotice bool
	}{
		{"no config.toml, new user", nil, "sandbox", false},
		{"config.toml without the key", &Config{}, "sandbox", true},
		{"auto-written conservative", &Config{Autonomy: &c}, "sandbox", true},
		{"chosen conservative", &Config{Autonomy: &c, AutonomyChosen: &yes}, "conservative", false},
		{"sandbox", &Config{Autonomy: &s}, "sandbox", false},
		{"chosen sandbox", &Config{Autonomy: &s, AutonomyChosen: &yes}, "sandbox", false},
		{"chosen without a value", &Config{AutonomyChosen: &yes}, "sandbox", true},
	} {
		r := resolve(tc.file, CLIOverrides{})
		if r.Autonomy != tc.want || r.AutonomyNotice != tc.wantNotice {
			t.Errorf("%s: autonomy = %q, notice = %v; want %q, %v", tc.name, r.Autonomy, r.AutonomyNotice, tc.want, tc.wantNotice)
		}
	}
	bad := "yolo"
	if problems := validateConfigProblems(&Config{Autonomy: &bad}); len(problems) == 0 {
		t.Error("autonomy = yolo passed validation")
	}
}

// Someone who ran nav-pilot before without a config.toml was on
// conservative: they get sandbox now, and the notice.
func TestUsedBeforeWithoutAFileGetsSandboxAndNotice(t *testing.T) {
	for name, mark := range markUsedBefore {
		t.Run(name, func(t *testing.T) {
			path := isolatedConfig(t)
			mark(t, path)
			r := resolve(nil, CLIOverrides{})
			if r.Autonomy != "sandbox" || !r.AutonomyNotice {
				t.Errorf("autonomy = %q, notice = %v; want sandbox with the notice", r.Autonomy, r.AutonomyNotice)
			}
			// config set and config init write no autonomy of their own.
			if err := updateConfigKey("model", `"gpt-5.5"`); err != nil {
				t.Fatal(err)
			}
			if got := autonomyOf(t); got != "(unset)" {
				t.Errorf("config set wrote autonomy = %s", got)
			}
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			if err := cmdConfigInit(); err != nil {
				t.Fatal(err)
			}
			if got := autonomyOf(t); got != "(unset)" {
				t.Errorf("config init wrote autonomy = %s", got)
			}
		})
	}
}

// A new user's first `config set` and `config init` write sandbox, so the
// file is never read as one from before the split.
func TestNewUserFileSaysSandbox(t *testing.T) {
	path := isolatedConfig(t)
	if err := updateConfigKey("client", `"copilot"`); err != nil {
		t.Fatal(err)
	}
	if got := autonomyOf(t); got != "sandbox" {
		t.Errorf("config set: autonomy = %s, want sandbox", got)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := cmdConfigInit(); err != nil {
		t.Fatal(err)
	}
	if got := autonomyOf(t); got != "sandbox" {
		t.Errorf("config init: autonomy = %s, want sandbox", got)
	}
	cfg, _ := readConfig()
	if resolve(cfg, CLIOverrides{}).AutonomyNotice {
		t.Error("a new user's file gets the notice")
	}
}

// Every explicit write marks the choice; removing the key removes the mark.
func TestConfigSetAutonomyMarksTheChoice(t *testing.T) {
	path := isolatedConfig(t)
	if err := os.WriteFile(path, []byte("version = 1\nautonomy = \"conservative\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := autonomyOf(t); got != "conservative" {
		t.Fatalf("setup: %s", got)
	}
	if _, err := writeConfigKey("autonomy", "conservative"); err != nil {
		t.Fatal(err)
	}
	if got := autonomyOf(t); got != "conservative (chosen)" {
		t.Errorf("after config set: %s", got)
	}
	cfg, _ := readConfig()
	if got := resolve(cfg, CLIOverrides{}).Autonomy; got != "conservative" {
		t.Errorf("resolved %q after config set conservative", got)
	}
	if err := cmdConfigUnset("autonomy"); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if strings.Contains(string(data), "autonomy") {
		t.Errorf("unset left %q", data)
	}
}

func TestSetupWizardWritesAutonomy(t *testing.T) {
	for _, tc := range []struct{ answer, want string }{
		{"", "(unset)"}, // a client the question is not asked for
		{"sandbox", "sandbox"},
		{"conservative", "conservative (chosen)"},
	} {
		isolatedConfig(t)
		if err := writeSetupConfig(setupAnswers{Client: "copilot", Autonomy: tc.answer}); err != nil {
			t.Fatal(err)
		}
		if got := autonomyOf(t); got != tc.want {
			t.Errorf("answer %q: autonomy = %s, want %s", tc.answer, got, tc.want)
		}
	}
}

// One line, only for an unchosen conservative in the file.
func TestAutonomyNudge(t *testing.T) {
	isolatedConfig(t)
	c, s, yes := "conservative", "sandbox", true
	var buf bytes.Buffer
	printAutonomyNudge(&buf, &Config{Autonomy: &c}, "")
	if out := buf.String(); strings.Count(out, "\n") != 1 || !strings.Contains(out, "nav-pilot config set autonomy conservative") {
		t.Errorf("nudge = %q, want one line naming the command", out)
	}
	for _, cfg := range []*Config{nil, {}, {Autonomy: &s}, {Autonomy: &c, AutonomyChosen: &yes}} {
		buf.Reset()
		printAutonomyNudge(&buf, cfg, "")
		if buf.Len() != 0 {
			t.Errorf("nudged %+v: %q", cfg, buf.String())
		}
	}
}
