package provider

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/navikt/copilot/cli/nav-pilot/internal/domain"
	"github.com/navikt/copilot/cli/nav-pilot/internal/source"
	"github.com/navikt/copilot/cli/nav-pilot/internal/telemetry"
)

// opencode 2 loads a repository's own plugins (.opencode/plugin[s]/ and the
// plugins entries in the project's opencode.json) with no question asked, and
// they run in the same process as nav-pilot's hooks bridge. nav-pilot asks
// once per project and plugin set before letting them load. The user's own
// global plugins are theirs and are not asked about.

// openCodeProjectPlugins is what a launch in projectDir would load from the
// project: plugin files and directories, and plugins entries in project config
// documents. Names are short, for the prompt; sum covers paths, file contents
// and the entries, so any change asks again. root keys the answer.
func openCodeProjectPlugins(projectDir string, env []string) (root string, names []string, sum string) {
	getenv := openCodeEnvGetter(env)
	if v := strings.ToLower(getenv(openCodeDisableProjectConfigEnv)); v == "1" || v == "true" {
		return "", nil, ""
	}
	if projectDir == "" {
		projectDir, _ = os.Getwd()
	}
	projectDir, _ = filepath.Abs(projectDir)
	root = source.FindGitRoot(projectDir)
	if root == "" {
		root = projectDir
	}
	home, _ := os.UserHomeDir()
	// opencode 2 walks every ancestor; ~/.opencode is the user's own.
	var dirs []string
	for d := projectDir; !isOpenCodeWalkStopDir(d, home) && (home == "" || d != filepath.Clean(home)); d = filepath.Dir(d) {
		dirs = append(dirs, d, filepath.Join(d, ".opencode"))
		if d == filepath.Dir(d) {
			break
		}
	}
	h := sha256.New()
	for _, d := range dirs {
		if filepath.Base(d) != ".opencode" {
			continue
		}
		for _, sub := range []string{"plugin", "plugins"} {
			entries, _ := os.ReadDir(filepath.Join(d, sub))
			for _, e := range entries {
				p := filepath.Join(d, sub, e.Name())
				st, err := os.Stat(p)
				if err != nil || (!st.IsDir() && !strings.HasSuffix(p, ".ts") && !strings.HasSuffix(p, ".js")) {
					continue
				}
				names = append(names, e.Name())
				// ponytail: hashes every file under a plugin directory except node_modules; fine for plugin-sized trees.
				_ = filepath.WalkDir(p, func(f string, de fs.DirEntry, err error) error {
					if err != nil {
						return nil
					}
					if de.IsDir() && de.Name() == "node_modules" {
						return filepath.SkipDir
					}
					if b, err := os.ReadFile(f); err == nil {
						fmt.Fprintf(h, "file %s %d\n", f, len(b))
						h.Write(b)
					}
					return nil
				})
			}
		}
	}
	for _, doc := range openCodeConfigDocuments(projectDir, env, false) {
		if !slices.Contains(dirs, filepath.Dir(doc.path)) {
			continue
		}
		var cfg struct {
			Plugins []json.RawMessage `json:"plugins"`
			Plugin  []json.RawMessage `json:"plugin"`
		}
		if json.Unmarshal(stripJSONC(doc.content), &cfg) != nil {
			continue
		}
		for _, e := range append(cfg.Plugins, cfg.Plugin...) {
			var s string
			if json.Unmarshal(e, &s) != nil {
				var o struct {
					Package string `json:"package"`
				}
				_ = json.Unmarshal(e, &o)
				s = o.Package
			}
			if s == "" || strings.HasPrefix(s, "-") {
				continue
			}
			names = append(names, filepath.Base(strings.TrimPrefix(s, "file://")))
			fmt.Fprintf(h, "entry %s %s\n", doc.path, e)
		}
	}
	if len(names) == 0 {
		return root, nil, ""
	}
	return root, names, hex.EncodeToString(h.Sum(nil))
}

type openCodePluginAnswer struct {
	Hash  string `json:"hash"`
	Allow bool   `json:"allow"`
}

// The answers live next to cplt-trust-seen.json, keyed by project root: like
// that file they record what this user said about a repo's own files, by hash.
// The proposal consent store is about pakke proposals and their scopes.
func openCodePluginAnswersFile() string {
	dir, err := telemetry.GetConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "opencode-project-plugins.json")
}

func readOpenCodePluginAnswers() map[string]openCodePluginAnswer {
	m := map[string]openCodePluginAnswer{}
	if p := openCodePluginAnswersFile(); p != "" {
		if b, err := os.ReadFile(p); err == nil {
			_ = json.Unmarshal(b, &m)
		}
	}
	return m
}

var openCodePluginAnswersMu sync.Mutex

func recordOpenCodePluginAnswer(root string, a openCodePluginAnswer) {
	openCodePluginAnswersMu.Lock()
	defer openCodePluginAnswersMu.Unlock()
	p := openCodePluginAnswersFile()
	if p == "" {
		return
	}
	m := readOpenCodePluginAnswers()
	m[root] = a
	b, _ := json.MarshalIndent(m, "", "  ")
	_ = os.MkdirAll(filepath.Dir(p), 0o700)
	_ = writeConfigAtomically(p, b)
}

// Test seams.
var (
	openCodePluginsTTY = func() bool { return IsTerminal(os.Stdin) }
	askOpenCodePlugins = func(prompt string) bool {
		fmt.Fprint(os.Stderr, prompt)
		// One byte at a time, so typed-ahead input stays for the client.
		var line []byte
		b := make([]byte, 1)
		for {
			n, err := os.Stdin.Read(b)
			if n == 0 || err != nil || b[0] == '\n' {
				break
			}
			line = append(line, b[0])
		}
		a := strings.ToLower(strings.TrimSpace(string(line)))
		return a == "y" || a == "yes" || a == "j" || a == "ja"
	}
)

// OpenCodePluginsCommand is the command that answers again.
const OpenCodePluginsCommand = "nav-pilot pakke plugins"

func askOpenCodePluginSet(root string, names []string, sum string) bool {
	short := names
	if len(short) > 3 {
		short = append(slices.Clone(short[:3]), "…")
	}
	allow := askOpenCodePlugins(fmt.Sprintf("This repo has its own OpenCode plugins (%d: %s). Load them? They run with the same access as the agent. [y/N] ", len(names), strings.Join(short, ", ")))
	recordOpenCodePluginAnswer(root, openCodePluginAnswer{Hash: sum, Allow: allow})
	return allow
}

// applyOpenCodeProjectPlugins asks before an opencode 2 launch loads the
// repo's own plugins, and turns the project config off when the answer is no.
// A stored answer holds with or without a terminal. A yes is silent; a no says
// one dim line each launch, since the repo's AGENTS.md and agents are off too.
func applyOpenCodeProjectPlugins(env []string, projectDir string) []string {
	if openCodeMajor() < 2 {
		return env
	}
	root, names, sum := openCodeProjectPlugins(projectDir, env)
	if sum == "" {
		return env
	}
	a, ok := readOpenCodePluginAnswers()[root]
	allow := a.Allow
	switch {
	case ok && a.Hash == sum:
	case !openCodePluginsTTY():
		// Headless: never block, never load an unanswered set, record nothing.
		env, _ = telemetry.SetEnvValue(env, openCodeDisableProjectConfigEnv, "1")
		fmt.Fprintf(os.Stderr, "%s This repo's OpenCode config is off: its plugins are not approved yet. To approve, run %s in a terminal.\n", domain.Yellow("⚠"), OpenCodePluginsCommand)
		return env
	default:
		allow = askOpenCodePluginSet(root, names, sum)
	}
	if allow {
		return env
	}
	env, _ = telemetry.SetEnvValue(env, openCodeDisableProjectConfigEnv, "1")
	fmt.Fprintf(os.Stderr, "%s This repo's OpenCode config is off (its plugins aren't trusted). Change: %s\n", domain.Dim("-"), OpenCodePluginsCommand)
	return env
}

// RunOpenCodePlugins shows the repo's own OpenCode plugins and the stored
// answer, and asks again on a terminal.
func RunOpenCodePlugins(projectDir string) error {
	root, names, sum := openCodeProjectPlugins(projectDir, nil)
	if sum == "" {
		fmt.Println("This repo has no OpenCode plugins of its own.")
		return nil
	}
	fmt.Printf("OpenCode plugins in %s:\n", root)
	for _, n := range names {
		fmt.Printf("  %s\n", n)
	}
	a, ok := readOpenCodePluginAnswers()[root]
	switch {
	case !ok || a.Hash != sum:
		fmt.Println("Answer: not asked (off until approved)")
	case a.Allow:
		fmt.Println("Answer: load them")
	default:
		fmt.Println("Answer: do not load (the repo's OpenCode config is off)")
	}
	if !openCodePluginsTTY() {
		return fmt.Errorf("no terminal to ask on: run %s in a terminal", OpenCodePluginsCommand)
	}
	askOpenCodePluginSet(root, names, sum)
	return nil
}
