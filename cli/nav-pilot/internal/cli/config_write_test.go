package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEditTopLevelKey(t *testing.T) {
	tests := []struct {
		name, in, key, val, replaces, want string
	}{
		{"replace keeps inline comment", "version = 1\nmode = \"plan\"   # team default\n", "mode", `"autopilot"`, "",
			"version = 1\nmode = \"autopilot\"   # team default\n"},
		{"hash inside string is not a comment", "model = \"a#b\" # c\n", "model", `"x"`, "",
			"model = \"x\" # c\n"},
		{"quoted key is the same key", "version = 1\n\"model\" = \"gpt-5.5\"\n", "model", `"gpt-5.4"`, "",
			"version = 1\nmodel = \"gpt-5.4\"\n"},
		{"indented key keeps indent", "  ask_user=false\n", "ask_user", "true", "",
			"  ask_user = true\n"},
		{"new key goes above the first table", "# notes\n\nmodel = \"m\"\n\n[extra]\ntheme = \"dark\"\n", "version", "1", "",
			"# notes\n\nmodel = \"m\"\nversion = 1\n\n[extra]\ntheme = \"dark\"\n"},
		{"new key with only a table", "[extra]\ntheme = \"dark\"\n", "version", "1", "",
			"version = 1\n\n[extra]\ntheme = \"dark\"\n"},
		{"table key of the same name is left alone", "version = 1\n[extra]\nmode = \"x\"\n", "mode", `"plan"`, "",
			"version = 1\nmode = \"plan\"\n\n[extra]\nmode = \"x\"\n"},
		{"commented template line is filled in", "version = 1\n\n# mode = \"default\"\n", "mode", `"plan"`, "",
			"version = 1\n\nmode = \"plan\"\n"},
		{"bracket inside a multi-line array is no header", "a = [\n[1],\n]\n", "mode", `"plan"`, "",
			"a = [\n[1],\n]\nmode = \"plan\"\n"},
		{"multi-line string value is replaced whole", "model = \"\"\"\nx\n\"\"\"\nmode = \"plan\"\n", "model", `"m"`, "",
			"model = \"m\"\nmode = \"plan\"\n"},
		{"retired key's line is taken over", "agent = \"opencode\"   # we use opencode\nmodel = \"m\"\n", "client", `"opencode"`, "agent",
			"client = \"opencode\"   # we use opencode\nmodel = \"m\"\n"},
		{"retired key is dropped when both are set", "client = \"pi\"\nagent = \"opencode\"\n", "client", `"copilot"`, "agent",
			"client = \"copilot\"\n"},
		{"retired key before the new one", "agent = \"opencode\"\nclient = \"pi\"\n", "client", `"copilot"`, "agent",
			"client = \"copilot\"\n"},
		{"unset removes the line", "version = 1\nmode = \"plan\" # x\nmodel = \"m\"\n", "mode", "", "",
			"version = 1\nmodel = \"m\"\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := editTopLevelKey(tt.in, tt.key, tt.val, tt.replaces)
			if err != nil {
				t.Fatalf("editTopLevelKey: %v", err)
			}
			if got != tt.want {
				t.Errorf("got\n%s\nwant\n%s", got, tt.want)
			}
		})
	}
}

func TestEditTopLevelKeyRefusesUnparseable(t *testing.T) {
	if _, err := editTopLevelKey("model = \"a\"\nmodel = \"b\"\n", "mode", `"plan"`, ""); err == nil {
		t.Fatal("edited a file that does not parse")
	}
}

func TestUpdateConfigKeyAtomicWithBackup(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	t.Setenv("NAV_PILOT_CONFIG", path)
	if err := os.WriteFile(path, []byte("version = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := updateConfigKey("mode", `"plan"`); err != nil {
		t.Fatal(err)
	}
	bak, _ := os.ReadFile(path + ".bak")
	if string(bak) != "version = 1\n" {
		t.Errorf("backup = %q", bak)
	}
	fi, _ := os.Stat(path)
	if fi.Mode().Perm() != 0o644 {
		t.Errorf("mode = %o, want 0644", fi.Mode().Perm())
	}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".tmp") {
			t.Errorf("temp file left behind: %s", e.Name())
		}
	}

	// A broken file is refused and left byte for byte.
	broken := "model = \"a\"\nmodel = \"b\"\n"
	_ = os.WriteFile(path, []byte(broken), 0o644)
	err := updateConfigKey("mode", `"plan"`)
	if err == nil || !strings.Contains(err.Error(), "nothing was written") {
		t.Fatalf("err = %v", err)
	}
	if got, _ := os.ReadFile(path); string(got) != broken {
		t.Errorf("broken file changed: %q", got)
	}
}

func TestUpdateConfigKeyWritesThroughSymlink(t *testing.T) {
	dir := t.TempDir()
	real := filepath.Join(dir, "dotfiles.toml")
	link := filepath.Join(dir, "config.toml")
	_ = os.WriteFile(real, []byte("version = 1\n"), 0o600)
	if err := os.Symlink(real, link); err != nil {
		t.Skip(err)
	}
	t.Setenv("NAV_PILOT_CONFIG", link)
	if err := updateConfigKey("mode", `"plan"`); err != nil {
		t.Fatal(err)
	}
	if fi, _ := os.Lstat(link); fi.Mode()&os.ModeSymlink == 0 {
		t.Error("symlink replaced by a file")
	}
	if got, _ := os.ReadFile(real); !strings.Contains(string(got), `mode = "plan"`) {
		t.Errorf("target not updated: %q", got)
	}
}

func TestTomlString(t *testing.T) {
	for _, s := range []string{"plain", `a"b`, "/Users/x/my repo", "tab\there", "ctl\x01", "ø"} {
		m, ok := decodesTo("k = " + tomlString(s))
		if !ok || m["k"] != s {
			t.Errorf("tomlString(%q) = %s does not round-trip", s, tomlString(s))
		}
	}
}
