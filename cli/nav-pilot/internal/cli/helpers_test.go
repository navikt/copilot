package cli

import (
	"encoding/json"
	"os"
	"testing"
)

// --- installedItemType ---

func TestInstalledItemType(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{".github/agents/nav-pilot.agent.md", "agent"},
		{".github/skills/security.skill.md", "skill"},
		{".github/instructions/foo.instructions.md", "instruction"},
		{".github/prompts/bar.prompt.md", "prompt"},
		{".github/copilot-instructions.md", "unknown"},
		{"agents/nav-pilot.agent.md", "agent"},
		{"skills/foo.skill.md", "skill"},
		{"instructions/foo.instructions.md", "instruction"},
	}
	for _, tt := range tests {
		got := installedItemType(tt.path)
		if got != tt.want {
			t.Errorf("installedItemType(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

// --- installedItemStatus ---

func TestInstalledItemStatus(t *testing.T) {
	tests := []struct {
		status string
		want   string
	}{
		{fileStatusIgnored, "ignored"},
		{fileStatusConflict, "conflict"},
		{"", "active"},
		{"other", "active"},
	}
	for _, tt := range tests {
		got := installedItemStatus(tt.status)
		if got != tt.want {
			t.Errorf("installedItemStatus(%q) = %q, want %q", tt.status, got, tt.want)
		}
	}
}

// --- normalizeCollectionLabel ---

func TestNormalizeCollectionLabel(t *testing.T) {
	tests := []struct {
		collection string
		want       string
	}{
		{CollectionAll, "all"},
		{"fullstack", "fullstack"},
		{"kotlin-backend", "kotlin-backend"},
		{"frontend", "frontend"},
		{"nextjs-frontend", "nextjs-frontend"},
		{"platform", "platform"},
		{"my-custom-collection", "other"},
		{"", "other"},
		{"  ", "other"},
	}
	for _, tt := range tests {
		got := normalizeCollectionLabel(tt.collection)
		if got != tt.want {
			t.Errorf("normalizeCollectionLabel(%q) = %q, want %q", tt.collection, got, tt.want)
		}
	}
}

// --- articleFor ---

func TestArticleFor(t *testing.T) {
	tests := []struct {
		kind string
		want string
	}{
		{"agent", "an"},
		{"instruction", "an"},
		{"skill", "a"},
		{"prompt", "a"},
		{"example", "an"},
		{"item", "an"},
		{"operator", "an"},
		{"unknown", "an"},
		{"build", "a"},
		{"collection", "a"},
	}
	for _, tt := range tests {
		got := articleFor(tt.kind)
		if got != tt.want {
			t.Errorf("articleFor(%q) = %q, want %q", tt.kind, got, tt.want)
		}
	}
}

// --- titleCase ---

func TestTitleCase(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"hello world", "Hello World"},
		{"nav-pilot", "Nav-pilot"},
		{"", ""},
		{"already Title", "Already Title"},
		{"one", "One"},
	}
	for _, tt := range tests {
		got := titleCase(tt.input)
		if got != tt.want {
			t.Errorf("titleCase(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// --- countInstalledItemsByTypeAndStatus ---

func TestCountInstalledItemsByTypeAndStatus(t *testing.T) {
	files := []InstalledFile{
		{Path: ".github/agents/nav-pilot.agent.md", Status: ""},
		{Path: ".github/agents/auth.agent.md", Status: ""},
		{Path: ".github/skills/security.skill.md", Status: fileStatusIgnored},
		{Path: ".github/instructions/golang.instructions.md", Status: fileStatusConflict},
		{Path: ".github/prompts/aksel.prompt.md", Status: ""},
	}
	counts := countInstalledItemsByTypeAndStatus(files)

	m := make(map[string]int64)
	for _, c := range counts {
		m[c.itemType+"|"+c.status] = c.count
	}
	if m["agent|active"] != 2 {
		t.Errorf("agent|active = %d, want 2", m["agent|active"])
	}
	if m["skill|ignored"] != 1 {
		t.Errorf("skill|ignored = %d, want 1", m["skill|ignored"])
	}
	if m["instruction|conflict"] != 1 {
		t.Errorf("instruction|conflict = %d, want 1", m["instruction|conflict"])
	}
	if m["prompt|active"] != 1 {
		t.Errorf("prompt|active = %d, want 1", m["prompt|active"])
	}
}

// --- formatTOMLValue ---

func TestFormatTOMLValue(t *testing.T) {
	strKey := &configKeyDef{name: "client", kind: keyKindString}
	intKey := &configKeyDef{name: "version", kind: keyKindInt}
	boolKey := &configKeyDef{name: "allow_all_tools", kind: keyKindBool}
	unknownKey := &configKeyDef{name: "x", kind: keyKind(99)}

	tests := []struct {
		kd      *configKeyDef
		value   string
		want    string
		wantErr bool
	}{
		{strKey, "opencode", `"opencode"`, false},
		{strKey, "", `""`, false},
		{intKey, "42", "42", false},
		{intKey, "notanint", "", true},
		{boolKey, "true", "true", false},
		{boolKey, "1", "true", false},
		{boolKey, "yes", "true", false},
		{boolKey, "false", "false", false},
		{boolKey, "0", "false", false},
		{boolKey, "no", "false", false},
		{boolKey, "maybe", "", true},
		{unknownKey, "x", "", true},
	}
	for _, tt := range tests {
		got, err := formatTOMLValue(tt.kd, tt.value)
		if tt.wantErr {
			if err == nil {
				t.Errorf("formatTOMLValue(%q, %q) = nil error, want error", tt.kd.name, tt.value)
			}
			continue
		}
		if err != nil {
			t.Errorf("formatTOMLValue(%q, %q) = %v, want nil", tt.kd.name, tt.value, err)
			continue
		}
		if got != tt.want {
			t.Errorf("formatTOMLValue(%q, %q) = %q, want %q", tt.kd.name, tt.value, got, tt.want)
		}
	}
}

// --- outputJSON (sync.go) ---

func TestOutputJSON(t *testing.T) {
	// Capture stdout
	origStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w

	type payload struct {
		Name string `json:"name"`
	}
	if err := outputJSON(payload{Name: "test"}); err != nil {
		w.Close()
		os.Stdout = origStdout
		t.Fatalf("outputJSON = %v", err)
	}
	w.Close()
	os.Stdout = origStdout

	buf := make([]byte, 256)
	n, _ := r.Read(buf)
	r.Close()

	var got payload
	if err := json.Unmarshal(buf[:n], &got); err != nil {
		t.Fatalf("unmarshal outputJSON output: %v", err)
	}
	if got.Name != "test" {
		t.Errorf("outputJSON Name = %q, want test", got.Name)
	}
}

// --- cmdEnv ---

func TestCmdEnv_WithInstructions(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	instrDir := home + "/.copilot/.github/instructions"
	if err := os.MkdirAll(instrDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(instrDir+"/golang.instructions.md", []byte("# Go"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Capture stdout
	origStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	t.Setenv("COPILOT_CUSTOM_INSTRUCTIONS_DIRS", "")

	cmdErr := cmdEnv()
	w.Close()
	os.Stdout = origStdout

	buf := make([]byte, 1024)
	n, _ := r.Read(buf)
	r.Close()
	out := string(buf[:n])

	if cmdErr != nil {
		t.Errorf("cmdEnv() with instructions = %v, want nil", cmdErr)
	}
	if len(out) == 0 {
		t.Error("cmdEnv() produced no stdout output")
	}
}

// --- collectAvailableItems ---

func TestCollectAvailableItems_Empty(t *testing.T) {
	tmp := t.TempDir()
	result := collectAvailableItems(tmp)
	// Empty source dir should return empty map (no panics)
	if result == nil {
		t.Error("collectAvailableItems returned nil, want empty map")
	}
}

func TestCollectAvailableItems_WithAgents(t *testing.T) {
	tmp := t.TempDir()
	agentsDir := tmp + "/agents"
	if err := os.MkdirAll(agentsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := `---
name: nav-pilot
description: Test
---
Body.
`
	if err := os.WriteFile(agentsDir+"/nav-pilot.agent.md", []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	result := collectAvailableItems(tmp)
	if len(result["agents"]) == 0 {
		t.Error("expected nav-pilot agent in result")
	}
}
