package provider

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
	"io"
	"net/url"
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

// OpenCodePluginsCommand is the command that answers again.
const OpenCodePluginsCommand = "nav-pilot plugins"

// openCodePluginSet is what a launch would load from the project.
type openCodePluginSet struct {
	key   string // root plus the launch directory: the answer's key
	root  string
	names []string // short, for the prompt
	sum   string   // "" when there are none
}

// openCodeProjectPlugins finds the plugin files and directories, and plugins
// entries in project config documents with the code they point at. sum covers
// paths (relative to root), file contents and the entries, so any change asks
// again. The key adds the launch directory, since opencode reads a
// subdirectory's .opencode only when launched there. A set that cannot be
// pinned down (a non-regular file, an entry built from {env:} or {file:}) gets
// a sum that never matches, so it is asked about every time.
//
// ponytail: a bundled plugin can import any file in the repo, so the hash
// covers the plugin's own files, not everything it may load.
func openCodeProjectPlugins(projectDir string, env []string) openCodePluginSet {
	getenv := openCodeEnvGetter(env)
	for _, k := range []string{openCodeDisableProjectConfigEnv, "OPENCODE_CONFIG_PROJECT_DISABLE"} {
		if v := strings.ToLower(getenv(k)); v == "1" || v == "true" {
			return openCodePluginSet{}
		}
	}
	if projectDir == "" {
		projectDir, _ = os.Getwd()
	}
	projectDir, _ = filepath.Abs(projectDir)
	set := openCodePluginSet{root: source.FindGitRoot(projectDir)}
	if set.root == "" {
		set.root = projectDir
	}
	rel, _ := filepath.Rel(set.root, projectDir)
	set.key = set.root + "::" + filepath.ToSlash(rel)
	home, _ := os.UserHomeDir()
	// opencode 2 walks every ancestor; ~/.opencode is the user's own.
	var dirs []string
	for d := projectDir; !isOpenCodeWalkStopDir(d, home) && (home == "" || d != filepath.Clean(home)); d = filepath.Dir(d) {
		dirs = append(dirs, d, filepath.Join(d, ".opencode"))
		if d == filepath.Dir(d) {
			break
		}
	}
	w := pluginHasher{h: sha256.New(), root: set.root, seen: map[string]bool{}}
	for _, d := range dirs {
		if filepath.Base(d) != ".opencode" {
			continue
		}
		for _, sub := range []string{"plugin", "plugins"} {
			entries, _ := os.ReadDir(filepath.Join(d, sub))
			for _, e := range entries {
				p := filepath.Join(d, sub, e.Name())
				if st, err := os.Stat(p); err == nil && !st.IsDir() && !strings.HasSuffix(p, ".ts") && !strings.HasSuffix(p, ".js") {
					continue
				}
				set.names = append(set.names, e.Name())
				w.add(p)
			}
		}
	}
	for _, doc := range openCodeConfigDocuments(projectDir, env, false) {
		if !slices.Contains(dirs, filepath.Dir(doc.path)) {
			continue
		}
		var cfg map[string]json.RawMessage
		if json.Unmarshal(stripJSONC(doc.content), &cfg) != nil {
			continue
		}
		for _, k := range []string{"plugins", "plugin"} {
			raw, ok := cfg[k]
			if !ok {
				continue
			}
			var list []json.RawMessage
			if json.Unmarshal(raw, &list) != nil || list == nil {
				// An unquoted {env:} nulls it here; opencode still sees a list.
				set.names = append(set.names, k+" in "+filepath.Base(doc.path))
				w.unknown = true
				continue
			}
			for _, e := range list {
				w.entry(doc.path, e, &set.names)
			}
		}
	}
	switch {
	case len(set.names) == 0:
	case w.unknown:
		set.sum = "unknown:" + rand.Text()
	default:
		set.sum = hex.EncodeToString(w.h.Sum(nil))
	}
	return set
}

// pluginMaxRead caps what is read per file before the user has said yes.
const pluginMaxRead = 1 << 20

type pluginHasher struct {
	h       hash.Hash
	root    string
	seen    map[string]bool
	unknown bool
}

func (w *pluginHasher) rel(p string) string {
	if r, err := filepath.Rel(w.root, p); err == nil && !strings.HasPrefix(r, "..") {
		return filepath.ToSlash(r)
	}
	return p
}

// entry hashes one plugins entry and, for a local one, the code it names,
// resolved as opencode's core/src/config/plugin/source.ts does.
func (w *pluginHasher) entry(docPath string, e json.RawMessage, names *[]string) {
	var s string
	if json.Unmarshal(e, &s) != nil {
		var o struct {
			Package string `json:"package"`
		}
		_ = json.Unmarshal(e, &o)
		s = o.Package
	}
	if s == "" || strings.HasPrefix(s, "-") {
		return
	}
	if strings.Contains(string(e), "{env:") || strings.Contains(string(e), "{file:") {
		w.unknown = true
	}
	target := s
	switch {
	case strings.HasPrefix(s, "file://"):
		if u, err := url.Parse(s); err == nil {
			target = u.Path
		}
	case strings.HasPrefix(s, "./"), strings.HasPrefix(s, "../"):
		target = filepath.Join(filepath.Dir(docPath), s)
	}
	*names = append(*names, filepath.Base(target))
	fmt.Fprintf(w.h, "entry %s %s\n", w.rel(docPath), e)
	if filepath.IsAbs(target) {
		w.add(target)
	}
}

// add hashes p, following symlinks, and reads only regular files.
func (w *pluginHasher) add(p string) {
	real, err := filepath.EvalSymlinks(p)
	if err != nil {
		if _, lerr := os.Lstat(p); lerr != nil {
			fmt.Fprintf(w.h, "missing %s\n", w.rel(p))
		} else {
			w.unknown = true // a dangling or looping link
		}
		return
	}
	if w.seen[real] {
		return
	}
	w.seen[real] = true
	st, err := os.Lstat(real)
	switch {
	case err != nil:
		w.unknown = true
	case st.IsDir():
		fmt.Fprintf(w.h, "dir %s\n", w.rel(p))
		entries, err := os.ReadDir(real)
		if err != nil {
			w.unknown = true
			return
		}
		for _, e := range entries {
			if e.Name() == "node_modules" && e.IsDir() {
				continue
			}
			w.add(filepath.Join(p, e.Name()))
		}
	case st.Mode().IsRegular():
		fmt.Fprintf(w.h, "file %s %d\n", w.rel(p), st.Size())
		if st.Size() > pluginMaxRead {
			return // name and size only
		}
		f, err := os.Open(real)
		if err != nil {
			w.unknown = true
			return
		}
		_, _ = io.Copy(w.h, io.LimitReader(f, pluginMaxRead))
		f.Close()
	default:
		w.unknown = true // a FIFO, device or socket
	}
}

type openCodePluginAnswer struct {
	Hash  string `json:"hash"`
	Allow bool   `json:"allow"`
}

// The answers live next to cplt-trust-seen.json: like that file they record
// what this user said about a repo's own files, by hash. The proposal consent
// store is about pakke proposals and their scopes.
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

func recordOpenCodePluginAnswer(key string, a openCodePluginAnswer) {
	openCodePluginAnswersMu.Lock()
	defer openCodePluginAnswersMu.Unlock()
	p := openCodePluginAnswersFile()
	if p == "" {
		return
	}
	m := readOpenCodePluginAnswers()
	m[key] = a
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

func askOpenCodePluginSet(set openCodePluginSet) bool {
	short := set.names
	if len(short) > 3 {
		short = append(slices.Clone(short[:3]), "…")
	}
	allow := askOpenCodePlugins(fmt.Sprintf("This repo has its own OpenCode plugins (%d: %s). Load them? They run with the same access as the agent. A no also turns off this repo's AGENTS.md, agents and skills. [y/N] ", len(set.names), strings.Join(short, ", ")))
	recordOpenCodePluginAnswer(set.key, openCodePluginAnswer{Hash: set.sum, Allow: allow})
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
	set := openCodeProjectPlugins(projectDir, env)
	if set.sum == "" {
		return env
	}
	a, ok := readOpenCodePluginAnswers()[set.key]
	allow := a.Allow
	switch {
	case ok && a.Hash == set.sum:
	case !openCodePluginsTTY():
		// Headless: never block, never load an unanswered set, record nothing.
		env, _ = telemetry.SetEnvValue(env, openCodeDisableProjectConfigEnv, "1")
		fmt.Fprintf(os.Stderr, "%s This repo's OpenCode config (plugins, agents, AGENTS.md) is off until you answer. Run %s in a terminal.\n", domain.Yellow("⚠"), OpenCodePluginsCommand)
		return env
	default:
		allow = askOpenCodePluginSet(set)
	}
	if allow {
		return env
	}
	env, _ = telemetry.SetEnvValue(env, openCodeDisableProjectConfigEnv, "1")
	fmt.Fprintf(os.Stderr, "%s This repo's OpenCode config is off (you declined its plugins). Change: %s\n", domain.Dim("-"), OpenCodePluginsCommand)
	// A repo-scope install lives in the same .opencode/ and goes off with it.
	if _, err := os.Stat(filepath.Join(set.root, ".opencode", ".nav-pilot-state.json")); err == nil {
		fmt.Fprintf(os.Stderr, "%s nav-pilot's agents installed in this repo are off too. nav-pilot install --user puts them where they stay on.\n", domain.Dim("-"))
	}
	return env
}

// RunOpenCodePlugins shows the repo's own OpenCode plugins and the stored
// answer, and asks again on a terminal.
func RunOpenCodePlugins(projectDir string) error {
	set := openCodeProjectPlugins(projectDir, nil)
	if set.sum == "" {
		fmt.Println("This repo has no OpenCode plugins of its own.")
		return nil
	}
	fmt.Printf("OpenCode plugins in %s:\n", set.root)
	for _, n := range set.names {
		fmt.Printf("  %s\n", n)
	}
	a, ok := readOpenCodePluginAnswers()[set.key]
	switch {
	case !ok || a.Hash != set.sum:
		fmt.Println("Answer: not asked (off until approved)")
	case a.Allow:
		fmt.Println("Answer: load them")
	default:
		fmt.Println("Answer: do not load (the repo's OpenCode config is off)")
	}
	if !openCodePluginsTTY() {
		if ok && a.Hash == set.sum {
			return nil
		}
		return fmt.Errorf("no terminal to ask on: run %s in a terminal", OpenCodePluginsCommand)
	}
	askOpenCodePluginSet(set)
	return nil
}
