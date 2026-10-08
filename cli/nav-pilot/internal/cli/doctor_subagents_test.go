package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/navikt/copilot/cli/nav-pilot/internal/source"
)

func TestReportSubagentOverrides(t *testing.T) {
	// The file is security-champion.agent.md, the frontmatter name is
	// "security-champion-agent": Copilot keys overrides by the name.
	pins := []pinnedModel{{Agent: "security-champion", Name: "security-champion-agent", Label: "Claude Opus 5.5", ID: "claude-opus-5.5"}}
	catalogue := []string{"claude-opus-5.5", "gpt-5.6-luna"}

	tests := []struct {
		name      string
		settings  string // "" means no file
		catalogue []string
		want      []string // substrings; nil means the clean line
	}{
		{"no file", "", catalogue, nil},
		{"no subagents key", `{"model":"gpt-5.6-luna"}`, catalogue, nil},
		{"inherit", `{"subagents":{"agents":{"security-champion-agent":{"model":"inherit"}}}}`, catalogue,
			[]string{"@security-champion is pinned to Claude Opus 5.5", "overrides it to inherit", "parent model", "Remove subagents.agents.security-champion-agent.model"}},
		{"other model", `{"subagents":{"agents":{"security-champion-agent":{"model":"gpt-5.6-luna"}}}}`, catalogue,
			[]string{"overrides it to gpt-5.6-luna", "delegated runs use gpt-5.6-luna"}},
		{"same model by label", `{"subagents":{"agents":{"security-champion-agent":{"model":"Claude Opus 5.5"}}}}`, catalogue, nil},
		{"same model by id", `{"subagents":{"agents":{"security-champion-agent":{"model":"claude-opus-5.5"}}}}`, catalogue, nil},
		{"unknown model", `{"subagents":{"agents":{"security-champion-agent":{"model":"gpt-5.4-nano"}}}}`, catalogue,
			[]string{`"gpt-5.4-nano", which Copilot does not know`, "silently runs it on the parent model"}},
		{"unknown model, no catalogue", `{"subagents":{"agents":{"anything":{"model":"fantasimodell-9"}}}}`, nil,
			[]string{"@anything", "was not checked"}},
		{"bom", "\xef\xbb\xbf" + `{"subagents":{"agents":{"security-champion-agent":{"model":"inherit"}}}}`, catalogue,
			[]string{"overrides it to inherit"}},
		{"filename key has no effect", `{"subagents":{"agents":{"security-champion":{"model":"inherit"}}}}`, catalogue, nil},
		{"jsonc", `{
  // personal tweaks
  "subagents": {
    /* block */ "agents": {
      "security-champion-agent": {"model": "inherit", "note": "a // not a comment"},
    },
  },
}`, catalogue, []string{"overrides it to inherit"}},
		{"unparsable", `{"subagents": nope}`, catalogue, []string{"Could not read ~/.copilot/settings.json"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "settings.json")
			if tt.settings != "" {
				if err := os.WriteFile(path, []byte(tt.settings), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			var buf bytes.Buffer
			reportSubagentOverrides(&buf, path, pins, tt.catalogue)
			got := buf.String()
			if tt.want == nil {
				if !strings.Contains(got, "No subagent override") {
					t.Errorf("want the clean line, got:\n%s", got)
				}
				return
			}
			if strings.Contains(got, "does not know") && tt.catalogue == nil {
				t.Errorf("no catalogue must never claim a model is unknown:\n%s", got)
			}
			for _, w := range tt.want {
				if !strings.Contains(got, w) {
					t.Errorf("output lacks %q:\n%s", w, got)
				}
			}
		})
	}
}

func TestInstalledModelPinsReadsName(t *testing.T) {
	root := t.TempDir()
	scope := &InstallScope{Name: "user", RootDir: root}
	dir := scope.DstPath(source.KindAgent.Dir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"named" + source.KindAgent.Suffix:   "---\nname: named-agent\nmodel: Claude Opus 5.5\n---\n",
		"unnamed" + source.KindAgent.Suffix: "---\nmodel: Claude Opus 5.5\n---\n",
	}
	for f, body := range files {
		if err := os.WriteFile(filepath.Join(dir, f), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	pins := installedModelPins(scope)
	if len(pins) != 2 || pins[0].Name != "named-agent" || pins[1].Name != "unnamed" {
		t.Errorf("pins = %+v", pins)
	}
}
