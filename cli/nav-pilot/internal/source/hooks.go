package source

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// A hook is the fifth artifact kind, and the only one that is executable code
// rather than text the model reads. Two things follow from that, and both live
// in this file.
//
// First, the config it has to end up in is not nav-pilot's own file. Repo scope
// shares one conventional file, .github/hooks/copilot-hooks.json — the name
// apps/mcp-onboarding/mcp.go probes for and internal/readiness recommends — and
// a user may already have their own entries in it. Installing therefore merges;
// it never overwrites. User scope needs no merge: ~/.copilot/hooks/ is a
// directory of separate files, one per hook (confirmed against rtk-rewrite.json,
// written there by the unrelated rtk binary).
//
// Second, nav-pilot must be able to find its own entries again without touching
// anyone else's. Every entry it writes carries HookMarker, and both the merge
// and the uninstall key off it: an unmarked entry is the user's and is never
// added to, updated, or removed.
//
// The two files speak different dialects, and the dialect is read off the two
// working examples rather than chosen:
//
//   - repo: {"hooks": {"preToolUse": [{… "timeoutSec": 5}]}} — the shape in this
//     repo's own .github/hooks/copilot-hooks.json, measured working in #557
//     (uu3 went from 0/5 to 9/10 with a non-empty NAV_PILOT_HOOK_DEBUG log).
//   - user: {"hooks": {"PreToolUse": [{… "timeout": 5}]}} — the shape rtk wrote
//     to ~/.copilot/hooks/rtk-rewrite.json.
//
// The hook scripts themselves read both payload dialects, so the casing of the
// event name changes only what the payload keys are called, never what the gate
// decides (see the contract note in hooks/ask-first-aria.py).

const (
	// HookMarker is the field that identifies an entry as nav-pilot's own. Its
	// value is the hook name, so an update can find the one entry it owns even
	// when several are installed.
	HookMarker = "navPilot"

	// RepoHooksConfig is the shared, conventional file repo-scope hooks merge into.
	RepoHooksConfig = "copilot-hooks.json"

	hookEventRepo = "preToolUse"
	hookEventUser = "PreToolUse"
)

// HookMeta is the sidecar a hook ships alongside its script: hooks/<name>.py is
// the code, hooks/<name>.hook.json says which tool calls it should see. It is
// separate from the script because the matcher is config the installer has to
// read, and parsing it out of Python would be worse.
type HookMeta struct {
	Matcher    string `json:"matcher"`
	TimeoutSec int    `json:"timeoutSec"`
}

// HookMetaSuffix is the sidecar's extension.
const HookMetaSuffix = ".hook.json"

// LoadHookMeta reads the sidecar for a resolved hook. A missing or unreadable
// sidecar is not fatal: the hook still installs, with no matcher (every tool
// call) and the default timeout. Refusing to install would turn a missing config
// file into a missing gate, which is the failure #569 exists to end.
func LoadHookMeta(scriptPath string) HookMeta {
	meta := HookMeta{TimeoutSec: 5}
	base := strings.TrimSuffix(scriptPath, filepath.Ext(scriptPath))
	data, err := os.ReadFile(base + HookMetaSuffix)
	if err != nil {
		return meta
	}
	var got HookMeta
	if err := json.Unmarshal(data, &got); err != nil {
		return meta
	}
	if got.TimeoutSec <= 0 {
		got.TimeoutSec = meta.TimeoutSec
	}
	return got
}

// HookCommand is the shell command a hook entry runs. The `command -v python3`
// guard makes a machine without python3 allow the call instead of denying every
// one of them: a gate that fails closed is worse than no gate.
func HookCommand(scriptPath string) string {
	return fmt.Sprintf("command -v python3 >/dev/null 2>&1 && python3 %s || exit 0", scriptPath)
}

// hooksFile is a Copilot hooks config, with every entry kept as raw JSON.
//
// Foreign entries survive a merge byte-for-byte because they are never decoded:
// a struct round-trip would drop any field this type does not know about, which
// is exactly what "merging must not damage the user's own hooks" forbids.
type hooksFile struct {
	Version int                          `json:"version"`
	Hooks   map[string][]json.RawMessage `json:"hooks,omitempty"`

	// rest preserves top-level keys the config may carry that are none of
	// nav-pilot's business.
	rest map[string]json.RawMessage
}

func readHooksFile(path string) (*hooksFile, error) {
	f := &hooksFile{Version: 1, Hooks: map[string][]json.RawMessage{}, rest: map[string]json.RawMessage{}}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return f, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, &f.rest); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	if raw, ok := f.rest["version"]; ok {
		_ = json.Unmarshal(raw, &f.Version)
		delete(f.rest, "version")
	}
	if raw, ok := f.rest["hooks"]; ok {
		if err := json.Unmarshal(raw, &f.Hooks); err != nil {
			return nil, fmt.Errorf("parsing hooks in %s: %w", path, err)
		}
		delete(f.rest, "hooks")
	}
	if f.Hooks == nil {
		f.Hooks = map[string][]json.RawMessage{}
	}
	return f, nil
}

func (f *hooksFile) write(path string) error {
	out := map[string]json.RawMessage{}
	for k, v := range f.rest {
		out[k] = v
	}
	version, err := marshalIndent(f.Version, "")
	if err != nil {
		return err
	}
	out["version"] = version
	for event, entries := range f.Hooks {
		if len(entries) == 0 {
			delete(f.Hooks, event)
		}
	}
	if len(f.Hooks) > 0 {
		hooks, err := marshalIndent(f.Hooks, "")
		if err != nil {
			return err
		}
		out["hooks"] = hooks
	}
	data, err := marshalIndent(out, "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

// isEmpty reports whether the config carries nothing but its version — the one
// state in which uninstall may delete the file rather than leave it behind.
func (f *hooksFile) isEmpty() bool {
	if len(f.rest) > 0 {
		return false
	}
	for _, entries := range f.Hooks {
		if len(entries) > 0 {
			return false
		}
	}
	return true
}

// markerOf returns the nav-pilot marker on a raw entry, or "" for an entry that
// is not nav-pilot's.
func markerOf(raw json.RawMessage) string {
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(raw, &probe); err != nil {
		return ""
	}
	var name string
	if err := json.Unmarshal(probe[HookMarker], &name); err != nil {
		return ""
	}
	return name
}

// HookEntry is one hook nav-pilot installs.
type HookEntry struct {
	Name    string
	Matcher string
	Command string
	Timeout int
}

// wireEntry is the on-disk shape of an entry nav-pilot writes. It exists as a
// struct rather than a map so the fields land in a readable order: a reviewer
// asking "what does installing this run on my machine" should read type,
// matcher, command in that order, not alphabetically.
type wireEntry struct {
	Type       string `json:"type"`
	Matcher    string `json:"matcher,omitempty"`
	Command    string `json:"command"`
	TimeoutSec int    `json:"timeoutSec,omitempty"`
	Timeout    int    `json:"timeout,omitempty"`
	NavPilot   string `json:"navPilot"`
}

func (h HookEntry) marshal(event string) (json.RawMessage, error) {
	entry := wireEntry{Type: "command", Matcher: h.Matcher, Command: h.Command, NavPilot: h.Name}
	if event == hookEventRepo {
		entry.TimeoutSec = h.Timeout
	} else {
		entry.Timeout = h.Timeout
	}
	return marshalIndent(entry, "")
}

// marshalIndent is json.MarshalIndent with HTML escaping off. The commands
// these entries carry are shell (`>/dev/null 2>&1 && …`), and \u003e in a file
// a human is expected to read before trusting it is noise at best.
func marshalIndent(v any, indent string) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if indent != "" {
		enc.SetIndent("", indent)
	}
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}

// MergeRepoHooks adds or updates nav-pilot's entries in the shared repo config
// at <hooksDir>/copilot-hooks.json, leaving every entry it does not own exactly
// as it found it. Running it twice with the same entries changes nothing.
func MergeRepoHooks(hooksDir string, entries []HookEntry) error {
	path := filepath.Join(hooksDir, RepoHooksConfig)
	f, err := readHooksFile(path)
	if err != nil {
		return err
	}
	list := f.Hooks[hookEventRepo]
	for _, e := range entries {
		raw, err := e.marshal(hookEventRepo)
		if err != nil {
			return err
		}
		replaced := false
		for i, existing := range list {
			if markerOf(existing) == e.Name {
				list[i] = raw
				replaced = true
				break
			}
		}
		if !replaced {
			list = append(list, raw)
		}
	}
	f.Hooks[hookEventRepo] = list
	return f.write(path)
}

// RemoveRepoHooks drops every entry nav-pilot marked as its own from the shared
// repo config, and removes the file only once nothing at all is left in it.
// Entries the user wrote are never touched, and a config that still holds one
// stays a valid config.
func RemoveRepoHooks(hooksDir string) (removed int, err error) {
	path := filepath.Join(hooksDir, RepoHooksConfig)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return 0, nil
	}
	f, err := readHooksFile(path)
	if err != nil {
		return 0, err
	}
	for event, list := range f.Hooks {
		kept := list[:0]
		for _, raw := range list {
			if markerOf(raw) != "" {
				removed++
				continue
			}
			kept = append(kept, raw)
		}
		f.Hooks[event] = kept
	}
	if f.isEmpty() {
		return removed, os.Remove(path)
	}
	return removed, f.write(path)
}

// WriteUserHook writes one hook's own config file into a user-scope hooks
// directory. No merge: each hook is its own file there, so an install can only
// ever overwrite the file it wrote itself.
func WriteUserHook(hooksDir string, entry HookEntry) error {
	raw, err := entry.marshal(hookEventUser)
	if err != nil {
		return err
	}
	f := &hooksFile{
		Version: 1,
		Hooks:   map[string][]json.RawMessage{hookEventUser: {raw}},
		rest:    map[string]json.RawMessage{},
	}
	return f.write(filepath.Join(hooksDir, entry.Name+".json"))
}

// UserHookConfigName is the file WriteUserHook produces for a hook, relative to
// the user-scope hooks directory.
func UserHookConfigName(name string) string { return name + ".json" }

// HookNamesIn lists the hook names a config file has nav-pilot entries for,
// sorted. Used by tests and by status output.
func HookNamesIn(path string) []string {
	f, err := readHooksFile(path)
	if err != nil {
		return nil
	}
	var names []string
	for _, list := range f.Hooks {
		for _, raw := range list {
			if name := markerOf(raw); name != "" {
				names = append(names, name)
			}
		}
	}
	sort.Strings(names)
	return names
}
