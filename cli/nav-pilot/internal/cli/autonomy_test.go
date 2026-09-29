package cli

import (
	"bytes"
	"os"
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
	return *cfg.Autonomy
}

// New users get autonomy = sandbox; everyone with a config from before keeps
// what they had, and nothing moves them.
func TestAutonomyDefaultsNewVersusExisting(t *testing.T) {
	if got := resolve(nil, CLIOverrides{}).Autonomy; got != "sandbox" {
		t.Errorf("no config.toml: autonomy = %q, want sandbox", got)
	}
	if got := resolve(&Config{}, CLIOverrides{}).Autonomy; got != "conservative" {
		t.Errorf("config.toml without the key: autonomy = %q, want conservative", got)
	}
	v := "sandbox"
	if got := resolve(&Config{Autonomy: &v}, CLIOverrides{}).Autonomy; got != "sandbox" {
		t.Errorf("autonomy = sandbox in the file: got %q", got)
	}
	bad := "yolo"
	if problems := validateConfigProblems(&Config{Autonomy: &bad}); len(problems) == 0 {
		t.Error("autonomy = yolo passed validation")
	}
}

// The site's first step is `nav-pilot config set client copilot`, which
// creates config.toml. That must not turn a new user into an existing one.
func TestConfigSetOnANewFileWritesSandbox(t *testing.T) {
	isolatedConfig(t)
	if err := updateConfigKey("client", `"copilot"`); err != nil {
		t.Fatal(err)
	}
	if got := autonomyOf(t); got != "sandbox" {
		t.Errorf("autonomy = %s, want sandbox", got)
	}
}

func TestConfigSetOnAnExistingFileLeavesAutonomyAlone(t *testing.T) {
	path := isolatedConfig(t)
	if err := os.WriteFile(path, []byte("version = 1\nclient = \"copilot\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := updateConfigKey("model", `"gpt-5.5"`); err != nil {
		t.Fatal(err)
	}
	if got := autonomyOf(t); got != "(unset)" {
		t.Errorf("autonomy = %s, want it left unset", got)
	}
}

func TestConfigInitWritesSandbox(t *testing.T) {
	isolatedConfig(t)
	if err := cmdConfigInit(); err != nil {
		t.Fatal(err)
	}
	if got := autonomyOf(t); got != "sandbox" {
		t.Errorf("autonomy = %s, want sandbox", got)
	}
}

func TestSetupWizardWritesAutonomy(t *testing.T) {
	for _, tc := range []struct{ answer, want string }{
		{"", "sandbox"}, // a client the question is not asked for
		{"sandbox", "sandbox"},
		{"conservative", "conservative"},
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

// One line, only for a config from before the key.
func TestAutonomyNudge(t *testing.T) {
	var buf bytes.Buffer
	printAutonomyNudge(&buf, &Config{}, "")
	if out := buf.String(); strings.Count(out, "\n") != 1 || !strings.Contains(out, "nav-pilot config setup") {
		t.Errorf("nudge = %q, want one line naming nav-pilot config setup", out)
	}
	v := "conservative"
	for _, cfg := range []*Config{nil, {Autonomy: &v}} {
		buf.Reset()
		printAutonomyNudge(&buf, cfg, "")
		if buf.Len() != 0 {
			t.Errorf("nudged %+v: %q", cfg, buf.String())
		}
	}
}
