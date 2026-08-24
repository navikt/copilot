package source

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/navikt/copilot/cli/nav-pilot/internal/agentpakke"
)

// ─── Get ────────────────────────────────────────────────────────────────────

func TestResolverGet_Agent_RootWins22(t *testing.T) {
	tmp := t.TempDir()
	mkFile(t, tmp, "agents", "nais.agent.md")

	r := NewSourceResolver(tmp)
	art, ok := r.Get(KindAgent, "nais")
	if !ok {
		t.Fatal("expected to find agent")
	}
	if art.RelPath != filepath.Join("agents", "nais.agent.md") {
		t.Errorf("RelPath = %q, want root", art.RelPath)
	}
	if art.IsDir {
		t.Error("agent should not be dir")
	}
	if art.FileName() != "nais.agent.md" {
		t.Errorf("FileName() = %q", art.FileName())
	}
}

func TestResolverGet_Agent_NotFound(t *testing.T) {
	tmp := t.TempDir()
	r := NewSourceResolver(tmp)
	_, ok := r.Get(KindAgent, "nais")
	if ok {
		t.Error("expected not found")
	}
}

func TestResolverGet_Skill_RootWithMarker(t *testing.T) {
	tmp := t.TempDir()
	mkFile(t, tmp, "skills", "api-design", "SKILL.md")

	r := NewSourceResolver(tmp)
	art, ok := r.Get(KindSkill, "api-design")
	if !ok {
		t.Fatal("expected to find skill")
	}
	if art.RelPath != filepath.Join("skills", "api-design") {
		t.Errorf("RelPath = %q", art.RelPath)
	}
	if !art.IsDir {
		t.Error("skill should be dir")
	}
	if art.FileName() != "api-design" {
		t.Errorf("FileName() = %q", art.FileName())
	}
}

func TestResolverGet_Skill_MissingMarker(t *testing.T) {
	tmp := t.TempDir()
	// Directory exists but no SKILL.md
	mkDir(t, tmp, "skills", "broken")

	r := NewSourceResolver(tmp)
	_, ok := r.Get(KindSkill, "broken")
	if ok {
		t.Error("expected not found — missing SKILL.md")
	}
}

func TestResolverGet_Skill_RootWins(t *testing.T) {
	tmp := t.TempDir()
	mkFile(t, tmp, "skills", "api-design", "SKILL.md")
	mkFile(t, tmp, "skills", "api-design", "SKILL.md")

	r := NewSourceResolver(tmp)
	art, ok := r.Get(KindSkill, "api-design")
	if !ok {
		t.Fatal("expected to find skill")
	}
	if art.RelPath != filepath.Join("skills", "api-design") {
		t.Errorf("RelPath = %q, want root", art.RelPath)
	}
}

func TestResolverGet_Instruction_Root(t *testing.T) {
	tmp := t.TempDir()
	mkFile(t, tmp, "instructions", "testing.instructions.md")

	r := NewSourceResolver(tmp)
	art, ok := r.Get(KindInstruction, "testing")
	if !ok {
		t.Fatal("expected to find instruction")
	}
	if art.RelPath != filepath.Join("instructions", "testing.instructions.md") {
		t.Errorf("RelPath = %q", art.RelPath)
	}
}

func TestResolverGet_Prompt_RootDir(t *testing.T) {
	tmp := t.TempDir()
	mkDir(t, tmp, "prompts", "review")

	r := NewSourceResolver(tmp)
	art, ok := r.Get(KindPrompt, "review")
	if !ok {
		t.Fatal("expected to find prompt")
	}
	if !art.IsDir {
		t.Error("expected dir (root dir wins over legacy file)")
	}
	if art.RelPath != filepath.Join("prompts", "review") {
		t.Errorf("RelPath = %q", art.RelPath)
	}
}

func TestResolverGet_Prompt_RootFile(t *testing.T) {
	tmp := t.TempDir()
	mkFile(t, tmp, "prompts", "review.prompt.md")

	r := NewSourceResolver(tmp)
	art, ok := r.Get(KindPrompt, "review")
	if !ok {
		t.Fatal("expected to find prompt")
	}
	if art.IsDir {
		t.Error("expected file")
	}
	if art.RelPath != filepath.Join("prompts", "review.prompt.md") {
		t.Errorf("RelPath = %q, want root", art.RelPath)
	}
}

func TestResolverGet_Prompt_Precedence(t *testing.T) {
	// Full 4-way precedence: root dir > root file > legacy dir > legacy file
	tmp := t.TempDir()
	mkDir(t, tmp, "prompts", "review")
	mkFile(t, tmp, "prompts", "review.prompt.md")

	r := NewSourceResolver(tmp)
	art, ok := r.Get(KindPrompt, "review")
	if !ok {
		t.Fatal("expected to find prompt")
	}
	if !art.IsDir {
		t.Error("root dir should win")
	}
	if art.RelPath != filepath.Join("prompts", "review") {
		t.Errorf("RelPath = %q", art.RelPath)
	}
}

// ─── GetFile ────────────────────────────────────────────────────────────────

func TestResolverGetFile_Root(t *testing.T) {
	tmp := t.TempDir()
	mkFile(t, tmp, "agents", "nais.metadata.json")

	r := NewSourceResolver(tmp)
	abs, rel, ok := r.GetFile("agents", "nais.metadata.json")
	if !ok {
		t.Fatal("expected to find file")
	}
	if rel != filepath.Join("agents", "nais.metadata.json") {
		t.Errorf("rel = %q", rel)
	}
	if abs != filepath.Join(tmp, "agents", "nais.metadata.json") {
		t.Errorf("abs = %q", abs)
	}
}

func TestResolverGetFile_NotFound(t *testing.T) {
	tmp := t.TempDir()
	r := NewSourceResolver(tmp)
	_, _, ok := r.GetFile("agents", "nais.metadata.json")
	if ok {
		t.Error("expected not found")
	}
}

// ─── List ───────────────────────────────────────────────────────────────────

func TestResolverList_Agents_RootWinsOnCollision(t *testing.T) {
	tmp := t.TempDir()
	mkFile(t, tmp, "agents", "nais.agent.md")

	r := NewSourceResolver(tmp)
	agents := r.List(KindAgent)
	if len(agents) != 1 {
		t.Fatalf("len = %d, want 1 (dedup)", len(agents))
	}
	if agents[0].RelPath != filepath.Join("agents", "nais.agent.md") {
		t.Errorf("RelPath = %q, want root", agents[0].RelPath)
	}
}

func TestResolverList_Skills_RootOnly(t *testing.T) {
	tmp := t.TempDir()
	mkFile(t, tmp, "skills", "api-design", "SKILL.md")
	mkFile(t, tmp, "skills", "security", "SKILL.md")

	r := NewSourceResolver(tmp)
	skills := r.List(KindSkill)
	if len(skills) != 2 {
		t.Fatalf("len = %d, want 2", len(skills))
	}
}

func TestResolverList_Prompts_DirsAndFiles(t *testing.T) {
	tmp := t.TempDir()
	mkDir(t, tmp, "prompts", "dir-prompt")
	mkFile(t, tmp, "prompts", "file-prompt.prompt.md")

	r := NewSourceResolver(tmp)
	prompts := r.List(KindPrompt)
	if len(prompts) != 2 {
		t.Fatalf("len = %d, want 2", len(prompts))
	}
	// Sorted: dir-prompt, file-prompt
	if prompts[0].Name != "dir-prompt" || !prompts[0].IsDir {
		t.Errorf("first = %+v", prompts[0])
	}
	if prompts[1].Name != "file-prompt" || prompts[1].IsDir {
		t.Errorf("second = %+v", prompts[1])
	}
}

func TestResolverList_Prompts_RootWinsOnCollision(t *testing.T) {
	tmp := t.TempDir()
	mkDir(t, tmp, "prompts", "review")

	r := NewSourceResolver(tmp)
	prompts := r.List(KindPrompt)
	if len(prompts) != 1 {
		t.Fatalf("len = %d, want 1", len(prompts))
	}
	if !prompts[0].IsDir {
		t.Error("root dir should win over legacy file")
	}
}

func TestResolverList_Empty(t *testing.T) {
	tmp := t.TempDir()
	r := NewSourceResolver(tmp)
	if len(r.List(KindAgent)) != 0 {
		t.Error("expected empty list")
	}
}

// ─── MapLocalPath ───────────────────────────────────────────────────────────

// ─── Helpers ────────────────────────────────────────────────────────────────

func mkFile(t *testing.T, parts ...string) {
	t.Helper()
	path := filepath.Join(parts...)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func mkDir(t *testing.T, parts ...string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(parts...), 0o755); err != nil {
		t.Fatal(err)
	}
}

// --- layout-aware resolution (agentpakke Tier 1) ---

func TestNewSourceResolverForLayout(t *testing.T) {
	tmp := t.TempDir()
	mkFile(t, tmp, "plugin", "agents", "chef.agent.md")
	mkFile(t, tmp, "plugin", "skills", "grilling", "SKILL.md")
	mkFile(t, tmp, "agents", "canonical.agent.md")

	tests := []struct {
		name       string
		layout     *agentpakke.Layout
		kind       *ArtifactKind
		item       string
		wantFound  bool
		wantRelDir string
	}{
		{
			name:       "nil layout reads the canonical directories",
			layout:     nil,
			kind:       KindAgent,
			item:       "canonical",
			wantFound:  true,
			wantRelDir: "agents",
		},
		{
			name:       "declared layout reads the manifest paths",
			layout:     &agentpakke.Layout{Agents: "plugin/agents", Skills: "plugin/skills"},
			kind:       KindAgent,
			item:       "chef",
			wantFound:  true,
			wantRelDir: filepath.Join("plugin", "agents"),
		},
		{
			name:      "declared layout hides the canonical directories",
			layout:    &agentpakke.Layout{Agents: "plugin/agents", Skills: "plugin/skills"},
			kind:      KindAgent,
			item:      "canonical",
			wantFound: false,
		},
		{
			name:       "directory artifacts follow the layout too",
			layout:     &agentpakke.Layout{Agents: "plugin/agents", Skills: "plugin/skills"},
			kind:       KindSkill,
			item:       "grilling",
			wantFound:  true,
			wantRelDir: filepath.Join("plugin", "skills"),
		},
		{
			name:       "layout repeating the canonical name is a no-op",
			layout:     &agentpakke.Layout{Agents: "agents", Skills: "skills"},
			kind:       KindAgent,
			item:       "canonical",
			wantFound:  true,
			wantRelDir: "agents",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewSourceResolverForLayout(tmp, tt.layout)
			art, ok := r.Get(tt.kind, tt.item)
			if ok != tt.wantFound {
				t.Fatalf("Get(%s, %q) found = %v, want %v", tt.kind.Name, tt.item, ok, tt.wantFound)
			}
			if !tt.wantFound {
				return
			}
			if got := filepath.Dir(art.RelPath); got != tt.wantRelDir {
				t.Errorf("RelPath dir = %q, want %q", got, tt.wantRelDir)
			}
			if r.SourceDir() != tmp {
				t.Errorf("SourceDir() = %q, want %q", r.SourceDir(), tmp)
			}
		})
	}
}

func TestCollectAllItemsWithLayout(t *testing.T) {
	tmp := t.TempDir()
	mkFile(t, tmp, "plugin", "agents", "chef.agent.md")

	m, err := CollectAllItemsWith(NewSourceResolverForLayout(tmp, &agentpakke.Layout{Agents: "plugin/agents", Skills: "plugin/skills"}))
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Agents) != 1 || m.Agents[0] != "chef" {
		t.Errorf("agents = %v, want [chef]", m.Agents)
	}
}

// ─── containment ────────────────────────────────────────────────────────────

// TestResolverRefusesSymlinkedContentDir covers the canonical directories as
// well as manifest layout paths: an intermediate symlink out of the checkout
// must not become a read path, even though Lstat of the final component sees an
// ordinary file.
func TestResolverRefusesSymlinkedContentDir(t *testing.T) {
	outside := t.TempDir()
	mkFile(t, outside, "elsewhere.agent.md")
	mkFile(t, outside, "leaked", "SKILL.md")

	tmp := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(tmp, "agents")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := os.Symlink(outside, filepath.Join(tmp, "skills")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	r := NewSourceResolver(tmp)
	if art, ok := r.Get(KindAgent, "elsewhere"); ok {
		t.Errorf("resolved %q through a symlinked agents/ directory", art.AbsPath)
	}
	if art, ok := r.Get(KindSkill, "leaked"); ok {
		t.Errorf("resolved %q through a symlinked skills/ directory", art.AbsPath)
	}
	if _, _, ok := r.GetFile("agents", "elsewhere.agent.md"); ok {
		t.Error("GetFile resolved a file through a symlinked agents/ directory")
	}
	if items := r.List(KindAgent); len(items) != 0 {
		t.Errorf("List returned %d agent(s) from outside the checkout", len(items))
	}
}

// TestResolverAllowsSymlinkedDirInsideCheckout keeps the containment check from
// overreaching: a link that stays inside the source is still readable, which is
// how a repo may lay its content out.
func TestResolverAllowsSymlinkedDirInsideCheckout(t *testing.T) {
	tmp := t.TempDir()
	mkFile(t, tmp, "content", "agents", "nais.agent.md")
	if err := os.Symlink(filepath.Join(tmp, "content", "agents"), filepath.Join(tmp, "agents")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	r := NewSourceResolver(tmp)
	if _, ok := r.Get(KindAgent, "nais"); !ok {
		t.Error("agents/ symlinked to a directory inside the checkout should still resolve")
	}
}
