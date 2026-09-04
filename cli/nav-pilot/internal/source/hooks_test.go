package source

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// foreign is an entry a user wrote by hand: no nav-pilot marker, and a field
// nav-pilot's own entry type does not have. Both have to survive every merge.
const foreign = `{
  "version": 1,
  "hooks": {
    "preToolUse": [
      {
        "type": "command",
        "matcher": "shell",
        "command": "./scripts/min-egen-port.sh",
        "timeoutSec": 9,
        "minEgenNokkel": true
      }
    ]
  }
}`

func hook(name, matcher, command string) HookEntry {
	return HookEntry{Name: name, Matcher: matcher, Command: command, Timeout: 5}
}

// entriesIn returns the preToolUse entries of a config as decoded maps.
func entriesIn(t *testing.T, path string) []map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	var got struct {
		Hooks map[string][]map[string]any `json:"hooks"`
	}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("parsing %s: %v\n%s", path, err, data)
	}
	return got.Hooks["preToolUse"]
}

func TestMergeRepoHooks(t *testing.T) {
	tests := []struct {
		name     string
		existing string
		merges   [][]HookEntry
		want     []map[string]any
	}{
		{
			name:   "writes a config where none existed",
			merges: [][]HookEntry{{hook("aria", "edit", "python3 a.py")}},
			want: []map[string]any{
				{"type": "command", "matcher": "edit", "command": "python3 a.py", "timeoutSec": 5.0, "navPilot": "aria"},
			},
		},
		{
			name:     "keeps a foreign entry, unknown fields and all",
			existing: foreign,
			merges:   [][]HookEntry{{hook("aria", "edit", "python3 a.py")}},
			want: []map[string]any{
				{"type": "command", "matcher": "shell", "command": "./scripts/min-egen-port.sh", "timeoutSec": 9.0, "minEgenNokkel": true},
				{"type": "command", "matcher": "edit", "command": "python3 a.py", "timeoutSec": 5.0, "navPilot": "aria"},
			},
		},
		{
			name:     "is idempotent",
			existing: foreign,
			merges: [][]HookEntry{
				{hook("aria", "edit", "python3 a.py")},
				{hook("aria", "edit", "python3 a.py")},
				{hook("aria", "edit", "python3 a.py")},
			},
			want: []map[string]any{
				{"type": "command", "matcher": "shell", "command": "./scripts/min-egen-port.sh", "timeoutSec": 9.0, "minEgenNokkel": true},
				{"type": "command", "matcher": "edit", "command": "python3 a.py", "timeoutSec": 5.0, "navPilot": "aria"},
			},
		},
		{
			name:     "updates the entry it already owns instead of adding a second",
			existing: foreign,
			merges: [][]HookEntry{
				{hook("aria", "edit", "python3 gammel.py")},
				{hook("aria", "edit|create", "python3 ny.py")},
			},
			want: []map[string]any{
				{"type": "command", "matcher": "shell", "command": "./scripts/min-egen-port.sh", "timeoutSec": 9.0, "minEgenNokkel": true},
				{"type": "command", "matcher": "edit|create", "command": "python3 ny.py", "timeoutSec": 5.0, "navPilot": "aria"},
			},
		},
		{
			name:     "adds two hooks in the order given",
			existing: foreign,
			merges: [][]HookEntry{
				{hook("aria", "edit", "python3 a.py")},
				{hook("klarsprak", "shell|execute|bash", "python3 k.py")},
			},
			want: []map[string]any{
				{"type": "command", "matcher": "shell", "command": "./scripts/min-egen-port.sh", "timeoutSec": 9.0, "minEgenNokkel": true},
				{"type": "command", "matcher": "edit", "command": "python3 a.py", "timeoutSec": 5.0, "navPilot": "aria"},
				{"type": "command", "matcher": "shell|execute|bash", "command": "python3 k.py", "timeoutSec": 5.0, "navPilot": "klarsprak"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, RepoHooksConfig)
			if tt.existing != "" {
				if err := os.WriteFile(path, []byte(tt.existing), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			for _, entries := range tt.merges {
				if err := MergeRepoHooks(dir, entries); err != nil {
					t.Fatalf("MergeRepoHooks: %v", err)
				}
			}
			got := entriesIn(t, path)
			if len(got) != len(tt.want) {
				t.Fatalf("got %d entries, want %d: %#v", len(got), len(tt.want), got)
			}
			for i := range got {
				for k, want := range tt.want[i] {
					if got[i][k] != want {
						t.Errorf("entry %d field %q = %#v, want %#v", i, k, got[i][k], want)
					}
				}
				if len(got[i]) != len(tt.want[i]) {
					t.Errorf("entry %d has fields %#v, want exactly %#v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestRemoveRepoHooks(t *testing.T) {
	tests := []struct {
		name        string
		existing    string
		install     []HookEntry
		wantRemoved int
		wantFile    bool // config still on disk afterwards
		wantLeft    int  // entries left in it
	}{
		{
			name:        "removes only the marked entries and leaves the foreign one",
			existing:    foreign,
			install:     []HookEntry{hook("aria", "edit", "python3 a.py"), hook("klarsprak", "shell", "python3 k.py")},
			wantRemoved: 2,
			wantFile:    true,
			wantLeft:    1,
		},
		{
			name:        "removes the file when nothing at all is left",
			install:     []HookEntry{hook("aria", "edit", "python3 a.py")},
			wantRemoved: 1,
			wantFile:    false,
		},
		{
			name:        "leaves a config that only ever held foreign entries alone",
			existing:    foreign,
			wantRemoved: 0,
			wantFile:    true,
			wantLeft:    1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, RepoHooksConfig)
			if tt.existing != "" {
				if err := os.WriteFile(path, []byte(tt.existing), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if len(tt.install) > 0 {
				if err := MergeRepoHooks(dir, tt.install); err != nil {
					t.Fatal(err)
				}
			}
			removed, err := RemoveRepoHooks(dir)
			if err != nil {
				t.Fatalf("RemoveRepoHooks: %v", err)
			}
			if removed != tt.wantRemoved {
				t.Errorf("removed %d, want %d", removed, tt.wantRemoved)
			}
			_, statErr := os.Stat(path)
			if tt.wantFile != (statErr == nil) {
				t.Fatalf("file present = %v, want %v", statErr == nil, tt.wantFile)
			}
			if !tt.wantFile {
				return
			}
			got := entriesIn(t, path)
			if len(got) != tt.wantLeft {
				t.Fatalf("%d entries left, want %d: %#v", len(got), tt.wantLeft, got)
			}
			for _, e := range got {
				if _, marked := e[HookMarker]; marked {
					t.Errorf("a marked entry survived: %#v", e)
				}
			}
		})
	}
}

// The user scope has its own dialect, read off ~/.copilot/hooks/rtk-rewrite.json:
// PascalCase event name and `timeout`, where the repo config uses camelCase and
// `timeoutSec`. Getting this backwards writes a file the CLI ignores.
func TestWriteUserHook(t *testing.T) {
	dir := t.TempDir()
	if err := WriteUserHook(dir, hook("klarsprak-gate", "shell|execute|bash", "python3 /home/u/.copilot/hooks/klarsprak-gate.py")); err != nil {
		t.Fatalf("WriteUserHook: %v", err)
	}
	path := filepath.Join(dir, "klarsprak-gate.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("expected %s: %v", path, err)
	}
	var got struct {
		Version int                         `json:"version"`
		Hooks   map[string][]map[string]any `json:"hooks"`
	}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got.Version != 1 {
		t.Errorf("version = %d, want 1", got.Version)
	}
	if _, ok := got.Hooks["preToolUse"]; ok {
		t.Errorf("user config used the repo dialect (camelCase event name):\n%s", data)
	}
	entries := got.Hooks["PreToolUse"]
	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1:\n%s", len(entries), data)
	}
	e := entries[0]
	if e["timeout"] != 5.0 {
		t.Errorf("timeout = %#v, want 5", e["timeout"])
	}
	if _, ok := e["timeoutSec"]; ok {
		t.Errorf("user entry carries timeoutSec, which is the repo dialect: %#v", e)
	}
	if e[HookMarker] != "klarsprak-gate" {
		t.Errorf("%s = %#v, want the hook name", HookMarker, e[HookMarker])
	}
	if !strings.Contains(e["command"].(string), "klarsprak-gate.py") {
		t.Errorf("command does not point at the script: %#v", e["command"])
	}
}

func TestLoadHookMeta(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "port.py")
	if err := os.WriteFile(script, []byte("#\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	// No sidecar: the hook still installs, with no matcher and a default timeout.
	// Refusing here would turn a missing config file into a missing gate.
	if got := LoadHookMeta(script); got.Matcher != "" || got.TimeoutSec != 5 {
		t.Errorf("without sidecar: %#v, want {Matcher:\"\" TimeoutSec:5}", got)
	}

	if err := os.WriteFile(filepath.Join(dir, "port"+HookMetaSuffix),
		[]byte(`{"matcher":"shell|execute|bash","timeoutSec":9}`), 0o644); err != nil {
		t.Fatal(err)
	}
	got := LoadHookMeta(script)
	if got.Matcher != "shell|execute|bash" || got.TimeoutSec != 9 {
		t.Errorf("with sidecar: %#v", got)
	}
}
