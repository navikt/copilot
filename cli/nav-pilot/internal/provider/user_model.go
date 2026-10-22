package provider

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/navikt/copilot/cli/nav-pilot/internal/domain"
	"github.com/navikt/copilot/cli/nav-pilot/internal/source"
)

// A model the user set (config `model` or --model) wins over the model the
// launched primary agent pins in its frontmatter (#1920). Subagents keep
// their own pins. With no user model nothing here changes the launch.
//
// Both clients that read an agent's `model:` let it beat --model:
//
//   - Copilot CLI: measured on 1.0.103. `--agent probe --model gpt-6-luna`
//     with probe pinning gpt-5.6-luna ran gpt-5.6-luna; the same agent
//     without the pin, loaded from a --plugin-dir, ran gpt-6-luna.
//   - opencode: the TUI picks the agent's model before the session model
//     (v2.1.3 packages/tui/src/context/local.tsx, newSessionModel; v1 alike).
//
// pi has no agents: the persona is appended as text, so --model already wins.

// UserSetModel reports whether r carries a model the user chose. opencode
// reads "auto" and the legacy alias as unset (see isOpenCodeUnsetModel); the
// Copilot CLI runs "auto" as a model of its own.
func UserSetModel(client string, r domain.ResolvedConfig) bool {
	if client == "copilot" {
		return strings.TrimSpace(r.Model) != ""
	}
	return !isOpenCodeUnsetModel(r.Model)
}

// PrimaryAgentFile returns the file of the primary agent persona a launch
// runs, or "" when there is none to read: the staged payload's agents/ when
// stagedDir is set, else where Tier 1 put it for client.
func PrimaryAgentFile(client, persona, stagedDir, projectDir string) string {
	var dirs []string
	switch {
	case persona == "":
	case stagedDir != "":
		dirs = []string{filepath.Join(stagedDir, "agents")}
	case client == "copilot":
		if projectDir == "" {
			projectDir, _ = os.Getwd()
		}
		if root := source.FindGitRoot(projectDir); root != "" {
			dirs = append(dirs, filepath.Join(root, ".github", "agents"))
		}
		if home := copilotSkillsRoot(); home != "" {
			dirs = append(dirs, filepath.Join(home, "agents"))
		}
	case client == "opencode":
		dirs = []string{filepath.Join(openCodeNavContextDir(), "agents")}
	}
	for _, d := range dirs {
		for _, name := range []string{persona + ".agent.md", persona + ".md"} {
			if f := filepath.Join(d, name); fileExists(f) {
				return f
			}
		}
	}
	return ""
}

// AgentModelPin returns the `model:` an agent file pins, as written, or "".
func AgentModelPin(file string) string {
	data, err := os.ReadFile(file)
	if err != nil {
		return ""
	}
	fm, _, ok := source.SplitFrontmatter(data)
	if !ok {
		return ""
	}
	v, _ := source.ExtractFrontmatterValue(fm, "model")
	return strings.TrimSpace(v)
}

// RecommendedModel is the model the agentpakke recommends for a launch: the
// primary agent's pin as a Copilot id, else the pakke's defaultModel, without
// a github-copilot/ prefix. "" when neither names one.
func RecommendedModel(client, agentFile string) string {
	m := AgentModelPin(agentFile)
	if id := domain.CopilotModelIDForLabel(m); id != "" {
		m = id
	}
	if m == "" {
		m = pakkeDeclaredModel(client)
	}
	m, _ = CopilotModelID(m)
	return m
}

// withUserAgentModel sets the launched opencode agent's model to the user's
// in OPENCODE_CONFIG_CONTENT. That document is merged after the agent files
// on both majors (v1.18.25 config.ts merges it after each directory's
// agents; v2.1.3 config.ts loads it after the global and project
// directories), so it outranks the agent's own `model:`. opencode 2 reads the
// v1 `agent` key too (config/normalize.ts, legacyAgents).
func withUserAgentModel(env []string, r domain.ResolvedConfig, agent string) []string {
	if agent == "" || agent == "plan" || !UserSetModel("opencode", r) {
		return env
	}
	return withOpenCodeConfigContent(env, map[string]any{
		"agent": map[string]any{agent: map[string]any{"model": ToOpenCodeModel(r.Model)}},
	})
}

// copilotSessionPlugin is the plugin a Copilot launch runs the primary agent
// from when the user's model has to win over the agent's pin. Copilot CLI
// skips a second plugin of a name it already loaded, so this cannot reuse the
// pakke's name.
const copilotSessionPlugin = "nav-pilot-session"

// copilotUserModelAgent returns the --agent value and the extra plugin dir a
// Copilot launch uses so that the user's model wins: a copy of agentFile
// without `model:` in a plugin of its own under ~/.copilot, which cplt lets
// copilot read. ("", "") when nothing needs to change: no user model, no
// agent file, or no pin in it.
//
// ponytail: one fixed directory per machine; two sessions launching agents
// of the same name from different pakker at the same moment race on it.
func copilotUserModelAgent(r domain.ResolvedConfig, persona, agentFile string) (agent, pluginDir string) {
	if !UserSetModel("copilot", r) || AgentModelPin(agentFile) == "" {
		return "", ""
	}
	home := copilotSkillsRoot()
	if home == "" {
		return "", ""
	}
	data, err := os.ReadFile(agentFile)
	if err != nil {
		return "", ""
	}
	fm, body, _ := source.SplitFrontmatter(data)
	dir := filepath.Join(home, copilotSessionPlugin)
	if os.MkdirAll(filepath.Join(dir, "agents"), 0o755) != nil ||
		os.WriteFile(filepath.Join(dir, "plugin.json"), []byte(`{"name":"`+copilotSessionPlugin+`","version":"0.0.0","agents":"agents/"}`+"\n"), 0o644) != nil ||
		os.WriteFile(filepath.Join(dir, "agents", persona+".agent.md"), source.Reassemble(source.StripFrontmatterKeys(fm, []string{"model"}), body), 0o644) != nil {
		return "", ""
	}
	return copilotSessionPlugin + ":" + persona, dir
}
