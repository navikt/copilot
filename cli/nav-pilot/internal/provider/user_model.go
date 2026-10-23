package provider

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

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
	case !SafePersona(persona):
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

// BeforeClientStart runs on a Tier 1 launch after the client's context is
// written and before the client starts; what it returns is launched. The CLI
// sets it for the model notice and the recommendation prompt.
var BeforeClientStart = func(r domain.ResolvedConfig) domain.ResolvedConfig { return r }

// RecommendedModel is the model the agentpakke recommends for a launch: the
// primary agent's pin as a Copilot id, else the pakke's defaultModel, without
// a github-copilot/ prefix. "" when neither names one.
func RecommendedModel(client, agentFile string) string {
	// No agent file to read: whether the agent pins a model is unknown, and
	// the pakke's defaultModel may not be what it runs on.
	// pi has no agent files, so no pin can disagree with the pakke (#1931).
	if agentFile == "" {
		if client == "pi" {
			m, _ := CopilotModelID(pakkeDeclaredModel(client))
			return m
		}
		return ""
	}
	m := AgentModelPin(agentFile)
	if id := domain.CopilotModelIDForLabel(m); id != "" {
		m = id
	} else if m != "" && domain.ValidateModelValue(m) != nil {
		// A display name the catalog does not know yet: nothing to name.
		return ""
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
	// A staged launch on opencode 2 restates the payload's agents here,
	// native ones under "agents" (withOpenCode2UserConfig), and opencode takes
	// a native entry whole over a legacy one: set the model where the agent is.
	key := "agent"
	for _, e := range env {
		if v, ok := strings.CutPrefix(e, openCodeConfigContentEnv+"="); ok {
			if n, err := jsonNode([]byte(v)); err == nil && nodeGet(nodeGet(n, "agents"), agent) != nil {
				key = "agents"
			}
		}
	}
	return withOpenCodeConfigContent(env, map[string]any{
		key: map[string]any{agent: map[string]any{"model": ToOpenCodeModel(r.Model)}},
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
// Each nav-pilot process gets its own directory, named by its pid, so two
// sessions never share one. nav-pilot waits for the client, so the directory
// lives as long as the session; directories of processes that are gone are
// removed at the next launch.
func copilotUserModelAgent(r domain.ResolvedConfig, persona, agentFile string) (agent, pluginDir string, err error) {
	pin := AgentModelPin(agentFile)
	if !SafePersona(persona) || !UserSetModel("copilot", r) || pin == "" {
		return "", "", nil
	}
	// The user's model is the pin already: the agent runs as it is.
	if id := domain.CopilotModelIDForLabel(pin); id != "" {
		pin = id
	}
	pin, _ = CopilotModelID(pin)
	if user, _ := CopilotModelID(strings.TrimSpace(r.Model)); strings.EqualFold(user, pin) {
		return "", "", nil
	}
	// From here a failure stops the launch: the pinned agent would run on
	// its pin, not on the model the user chose.
	fail := func(path string, err error) (string, string, error) {
		return "", "", fmt.Errorf("preparing %s so your model %s wins over the agent's %s: %w", path, r.Model, pin, err)
	}
	home, herr := os.UserHomeDir()
	root := copilotSkillsRoot()
	if herr != nil || root == "" {
		return fail("~/.copilot", errors.New("no home directory"))
	}
	data, err := os.ReadFile(agentFile)
	if err != nil {
		return fail(agentFile, err)
	}
	fm, body, _ := source.SplitFrontmatter(data)
	root = filepath.Join(root, copilotSessionPlugin)
	dir := filepath.Join(root, strconv.Itoa(os.Getpid()))
	manifest, copyFile := filepath.Join(dir, "plugin.json"), filepath.Join(dir, "agents", persona+".agent.md")
	// No symlink on the way down from the home directory before anything is
	// pruned or written, as in source.CopyFile.
	for _, p := range []string{root, manifest, copyFile} {
		if err := source.CheckSymlink(p, home); err != nil {
			return fail(p, err)
		}
	}
	pruneSessionPlugins(root)
	if err := os.MkdirAll(filepath.Dir(copyFile), 0o755); err != nil {
		return fail(dir, err)
	}
	if err := os.WriteFile(manifest, []byte(`{"name":"`+copilotSessionPlugin+`","version":"0.0.0","agents":"agents/"}`+"\n"), 0o644); err != nil {
		return fail(manifest, err)
	}
	if err := os.WriteFile(copyFile, source.Reassemble(source.StripFrontmatterKeys(fm, []string{"model"}), body), 0o644); err != nil {
		return fail(copyFile, err)
	}
	return copilotSessionPlugin + ":" + persona, dir, nil
}

// SafePersona reports whether persona is a plain file name, safe to join
// into a path.
func SafePersona(persona string) bool {
	return persona != "" && persona == filepath.Base(persona) && !strings.HasPrefix(persona, ".")
}

// pruneSessionPlugins removes the session plugin directories of nav-pilot
// processes that no longer run. A reused pid, or one the kernel will not
// answer for (EPERM), keeps its directory: it leaks rather than deleting a
// live session's agent.
func pruneSessionPlugins(root string) {
	entries, _ := os.ReadDir(root)
	for _, e := range entries {
		pid, err := strconv.Atoi(e.Name())
		if err == nil && pid != os.Getpid() && syscall.Kill(pid, 0) == syscall.ESRCH {
			_ = os.RemoveAll(filepath.Join(root, e.Name()))
		}
	}
}
