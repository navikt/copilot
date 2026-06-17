package source

import (
	"os"
	"path/filepath"
	"testing"
)

// ─── Get ────────────────────────────────────────────────────────────────────

func TestResolverGet_Agent_RootWins(t *testing.T) {
	tmp := t.TempDir()
	mkFile(t, tmp, "agents", "nais.agent.md")
	mkFile(t, tmp, ".github", "agents", "nais.agent.md")

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

func TestResolverGet_Agent_LegacyFallback(t *testing.T) {
	tmp := t.TempDir()
	mkFile(t, tmp, ".github", "agents", "nais.agent.md")

	r := NewSourceResolver(tmp)
	art, ok := r.Get(KindAgent, "nais")
	if !ok {
		t.Fatal("expected to find agent")
	}
	if art.RelPath != filepath.Join(".github", "agents", "nais.agent.md") {
		t.Errorf("RelPath = %q, want legacy", art.RelPath)
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

func TestResolverGet_Skill_LegacyWithMarker(t *testing.T) {
	tmp := t.TempDir()
	mkFile(t, tmp, ".github", "skills", "api-design", "SKILL.md")

	r := NewSourceResolver(tmp)
	art, ok := r.Get(KindSkill, "api-design")
	if !ok {
		t.Fatal("expected to find skill")
	}
	if art.RelPath != filepath.Join(".github", "skills", "api-design") {
		t.Errorf("RelPath = %q, want legacy", art.RelPath)
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
	mkFile(t, tmp, ".github", "skills", "api-design", "SKILL.md")

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

func TestResolverGet_Instruction_Legacy(t *testing.T) {
	tmp := t.TempDir()
	mkFile(t, tmp, ".github", "instructions", "testing.instructions.md")

	r := NewSourceResolver(tmp)
	art, ok := r.Get(KindInstruction, "testing")
	if !ok {
		t.Fatal("expected to find instruction")
	}
	if art.RelPath != filepath.Join(".github", "instructions", "testing.instructions.md") {
		t.Errorf("RelPath = %q", art.RelPath)
	}
}

func TestResolverGet_Prompt_RootDirWinsOverLegacyFile(t *testing.T) {
	tmp := t.TempDir()
	mkDir(t, tmp, "prompts", "review")
	mkFile(t, tmp, ".github", "prompts", "review.prompt.md")

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

func TestResolverGet_Prompt_RootFileOverLegacy(t *testing.T) {
	tmp := t.TempDir()
	mkFile(t, tmp, "prompts", "review.prompt.md")
	mkFile(t, tmp, ".github", "prompts", "review.prompt.md")

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

func TestResolverGet_Prompt_LegacyDir(t *testing.T) {
	tmp := t.TempDir()
	mkDir(t, tmp, ".github", "prompts", "review")

	r := NewSourceResolver(tmp)
	art, ok := r.Get(KindPrompt, "review")
	if !ok {
		t.Fatal("expected to find prompt")
	}
	if !art.IsDir {
		t.Error("expected dir")
	}
	if art.RelPath != filepath.Join(".github", "prompts", "review") {
		t.Errorf("RelPath = %q", art.RelPath)
	}
}

func TestResolverGet_Prompt_LegacyFile(t *testing.T) {
	tmp := t.TempDir()
	mkFile(t, tmp, ".github", "prompts", "review.prompt.md")

	r := NewSourceResolver(tmp)
	art, ok := r.Get(KindPrompt, "review")
	if !ok {
		t.Fatal("expected to find prompt")
	}
	if art.IsDir {
		t.Error("expected file")
	}
	if art.RelPath != filepath.Join(".github", "prompts", "review.prompt.md") {
		t.Errorf("RelPath = %q", art.RelPath)
	}
}

func TestResolverGet_Prompt_Precedence_RootDir_RootFile_LegacyDir_LegacyFile(t *testing.T) {
	// Full 4-way precedence: root dir > root file > legacy dir > legacy file
	tmp := t.TempDir()
	mkDir(t, tmp, "prompts", "review")
	mkFile(t, tmp, "prompts", "review.prompt.md")
	mkDir(t, tmp, ".github", "prompts", "review")
	mkFile(t, tmp, ".github", "prompts", "review.prompt.md")

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

func TestResolverGetFile_LegacyFallback(t *testing.T) {
	tmp := t.TempDir()
	mkFile(t, tmp, ".github", "agents", "nais.metadata.json")

	r := NewSourceResolver(tmp)
	_, rel, ok := r.GetFile("agents", "nais.metadata.json")
	if !ok {
		t.Fatal("expected to find file")
	}
	if rel != filepath.Join(".github", "agents", "nais.metadata.json") {
		t.Errorf("rel = %q", rel)
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

func TestResolverList_Agents_MergesRootAndLegacy(t *testing.T) {
	tmp := t.TempDir()
	mkFile(t, tmp, "agents", "alpha.agent.md")
	mkFile(t, tmp, ".github", "agents", "beta.agent.md")

	r := NewSourceResolver(tmp)
	agents := r.List(KindAgent)
	if len(agents) != 2 {
		t.Fatalf("len = %d, want 2", len(agents))
	}
	if agents[0].Name != "alpha" || agents[1].Name != "beta" {
		t.Errorf("names = %v", []string{agents[0].Name, agents[1].Name})
	}
}

func TestResolverList_Agents_RootWinsOnCollision(t *testing.T) {
	tmp := t.TempDir()
	mkFile(t, tmp, "agents", "nais.agent.md")
	mkFile(t, tmp, ".github", "agents", "nais.agent.md")

	r := NewSourceResolver(tmp)
	agents := r.List(KindAgent)
	if len(agents) != 1 {
		t.Fatalf("len = %d, want 1 (dedup)", len(agents))
	}
	if agents[0].RelPath != filepath.Join("agents", "nais.agent.md") {
		t.Errorf("RelPath = %q, want root", agents[0].RelPath)
	}
}

func TestResolverList_Skills_ValidatesMarker(t *testing.T) {
	tmp := t.TempDir()
	mkFile(t, tmp, "skills", "valid", "SKILL.md")
	mkDir(t, tmp, "skills", "invalid") // no SKILL.md
	mkFile(t, tmp, ".github", "skills", "legacy", "SKILL.md")

	r := NewSourceResolver(tmp)
	skills := r.List(KindSkill)
	if len(skills) != 2 {
		t.Fatalf("len = %d, want 2 (valid + legacy)", len(skills))
	}
	names := []string{skills[0].Name, skills[1].Name}
	if names[0] != "legacy" || names[1] != "valid" {
		t.Errorf("names = %v", names)
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
	mkFile(t, tmp, ".github", "prompts", "review.prompt.md")

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

func TestResolverList_Sorted(t *testing.T) {
	tmp := t.TempDir()
	mkFile(t, tmp, "agents", "zebra.agent.md")
	mkFile(t, tmp, "agents", "alpha.agent.md")
	mkFile(t, tmp, ".github", "agents", "middle.agent.md")

	r := NewSourceResolver(tmp)
	agents := r.List(KindAgent)
	if len(agents) != 3 {
		t.Fatalf("len = %d", len(agents))
	}
	if agents[0].Name != "alpha" || agents[1].Name != "middle" || agents[2].Name != "zebra" {
		t.Errorf("not sorted: %v", []string{agents[0].Name, agents[1].Name, agents[2].Name})
	}
}

func TestResolverList_LegacyOnly(t *testing.T) {
	tmp := t.TempDir()
	mkFile(t, tmp, ".github", "agents", "nais.agent.md")
	mkFile(t, tmp, ".github", "agents", "auth.agent.md")

	r := NewSourceResolver(tmp)
	agents := r.List(KindAgent)
	if len(agents) != 2 {
		t.Fatalf("len = %d, want 2", len(agents))
	}
}

// ─── MapLocalPath ───────────────────────────────────────────────────────────

func TestResolverMapLocalPath(t *testing.T) {
	tmp := t.TempDir()
	// Root-level artifacts
	mkFile(t, tmp, "agents", "nais.agent.md")
	mkFile(t, tmp, "agents", "nais.metadata.json")
	mkFile(t, tmp, "instructions", "testing.instructions.md")
	mkFile(t, tmp, "skills", "api-design", "SKILL.md")
	mkDir(t, tmp, "prompts", "review")
	mkFile(t, tmp, "prompts", "quick.prompt.md")
	// Legacy-only
	mkFile(t, tmp, ".github", "agents", "legacy.agent.md")
	mkFile(t, tmp, ".github", "skills", "old-skill", "SKILL.md")
	mkFile(t, tmp, ".github", "instructions", "old.instructions.md")

	r := NewSourceResolver(tmp)

	tests := []struct {
		name        string
		localPath   string
		isUserScope bool
		want        string
	}{
		// Repo scope: .github/ prefix, root-level exists → strip .github/
		{
			"repo agent root-level",
			".github/agents/nais.agent.md",
			false,
			filepath.Join("agents", "nais.agent.md"),
		},
		{
			"repo agent metadata root-level",
			".github/agents/nais.metadata.json",
			false,
			filepath.Join("agents", "nais.metadata.json"),
		},
		{
			"repo instruction root-level",
			".github/instructions/testing.instructions.md",
			false,
			filepath.Join("instructions", "testing.instructions.md"),
		},
		{
			"repo skill root-level",
			".github/skills/api-design/",
			false,
			filepath.Join("skills", "api-design") + "/",
		},
		{
			"repo prompt dir root-level",
			".github/prompts/review/",
			false,
			filepath.Join("prompts", "review") + "/",
		},
		{
			"repo prompt file root-level",
			".github/prompts/quick.prompt.md",
			false,
			filepath.Join("prompts", "quick.prompt.md"),
		},

		// Repo scope: .github/ prefix, only legacy exists → keep .github/
		{
			"repo agent legacy-only",
			".github/agents/legacy.agent.md",
			false,
			filepath.Join(".github", "agents", "legacy.agent.md"),
		},
		{
			"repo skill legacy-only",
			".github/skills/old-skill/",
			false,
			filepath.Join(".github", "skills", "old-skill") + "/",
		},
		{
			"repo instruction legacy-only",
			".github/instructions/old.instructions.md",
			false,
			filepath.Join(".github", "instructions", "old.instructions.md"),
		},

		// User scope: no .github/ prefix, root-level exists
		{
			"user agent root-level",
			"agents/nais.agent.md",
			true,
			filepath.Join("agents", "nais.agent.md"),
		},
		{
			"user skill root-level",
			"skills/api-design/",
			true,
			filepath.Join("skills", "api-design") + "/",
		},

		// User scope: .github/instructions/ prefix
		{
			"user instruction root-level",
			".github/instructions/testing.instructions.md",
			true,
			filepath.Join("instructions", "testing.instructions.md"),
		},
		{
			"user instruction legacy-only",
			".github/instructions/old.instructions.md",
			true,
			filepath.Join(".github", "instructions", "old.instructions.md"),
		},

		// User scope: no .github/ prefix, not found → prepend .github/
		{
			"user agent not-in-source",
			"agents/missing.agent.md",
			true,
			filepath.Join(".github", "agents", "missing.agent.md"),
		},
		{
			"user skill not-in-source",
			"skills/missing/",
			true,
			filepath.Join(".github", "skills", "missing") + "/",
		},

		// Edge case: repo scope without .github/ prefix → return as-is
		{
			"repo no prefix",
			"agents/foo.agent.md",
			false,
			"agents/foo.agent.md",
		},

		// Edge case: source removed → return original path unchanged
		{
			"repo removed artifact",
			".github/agents/deleted.agent.md",
			false,
			".github/agents/deleted.agent.md",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := r.MapLocalPath(tt.localPath, tt.isUserScope)
			if got != tt.want {
				t.Errorf("MapLocalPath(%q, %v) = %q, want %q",
					tt.localPath, tt.isUserScope, got, tt.want)
			}
		})
	}
}

func TestResolverMapLocalPath_SkillWithoutMarker(t *testing.T) {
	tmp := t.TempDir()
	// Directory exists at root but has no SKILL.md → falls back to legacy
	mkDir(t, tmp, "skills", "broken")
	mkFile(t, tmp, ".github", "skills", "broken", "SKILL.md")

	r := NewSourceResolver(tmp)
	got := r.MapLocalPath(".github/skills/broken/", false)
	want := filepath.Join(".github", "skills", "broken") + "/"
	if got != want {
		t.Errorf("got %q, want %q (legacy because root has no SKILL.md)", got, want)
	}
}

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
