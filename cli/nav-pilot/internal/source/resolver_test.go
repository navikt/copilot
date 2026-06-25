package source

import (
	"os"
	"path/filepath"
	"testing"
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
