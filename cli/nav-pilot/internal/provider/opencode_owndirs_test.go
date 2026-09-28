package provider

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A session may read what nav-pilot installed in opencode's config dir
// without an external_directory request, which `opencode run` rejects
// (#1120). The user's own choice for every other directory stays.
func TestApplyOpenCodeOwnDirs(t *testing.T) {
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	t.Setenv("HOME", t.TempDir())
	project := t.TempDir()
	cfgDir := filepath.Join(xdg, "opencode")
	if err := os.MkdirAll(cfgDir, 0o700); err != nil {
		t.Fatal(err)
	}
	content := func(env []string) string {
		for _, e := range env {
			if v, ok := strings.CutPrefix(e, "OPENCODE_CONFIG_CONTENT="); ok {
				return v
			}
		}
		return ""
	}
	instr := filepath.ToSlash(filepath.Join(cfgDir, "instructions")) + `/*":"allow"`

	for _, tc := range []struct {
		name, global string
		env          []string
		want, not    []string
	}{
		{name: "no user rule", want: []string{instr, "/agents/*", "/skills/*"}, not: []string{`"*"`}},
		{name: "user asks", global: `{"permission": {"external_directory": "ask"}} // jsonc`, want: []string{`"external_directory":{"*":"ask","`, instr}},
		{name: "user denies", global: `{"permission": {"external_directory": "deny"}}`, not: []string{"external_directory"}},
		{name: "user allows", global: `{"permission": {"external_directory": "allow"}}`, not: []string{"external_directory"}},
		{name: "whole block one string", global: `{"permission": "ask"}`, not: []string{"permission"}},
		{name: "user patterns", global: `{"permission": {"external_directory": {"/data/*": "allow"}}}`, want: []string{instr}, not: []string{`"*"`}},
		{name: "staged dir", env: []string{"OPENCODE_CONFIG_DIR=/stage/pakke"}, want: []string{instr, `"/stage/pakke/instructions/*":"allow"`}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(cfgDir, "opencode.json")
			os.Remove(path)
			if tc.global != "" {
				if err := os.WriteFile(path, []byte(tc.global), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			got := content(applyOpenCodeOwnDirs(tc.env, project))
			for _, w := range tc.want {
				if !strings.Contains(got, w) {
					t.Errorf("OPENCODE_CONFIG_CONTENT %s lacks %s", got, w)
				}
			}
			for _, n := range tc.not {
				if strings.Contains(got, n) {
					t.Errorf("OPENCODE_CONFIG_CONTENT %s has %s", got, n)
				}
			}
		})
	}
}
