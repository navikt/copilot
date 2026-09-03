package artifacts

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/navikt/copilot/cli/nav-pilot/internal/domain"
	"github.com/navikt/copilot/cli/nav-pilot/internal/source"
)

const (
	openCodeStateFileName = ".nav-pilot-state.json"
	OpenCodeCollection    = "opencode-export"
	OpenCodeScopeName     = "opencode"
)

func openCodeStateFilePath(outputDir string) string {
	return filepath.Join(outputDir, openCodeStateFileName)
}

// ReadOpenCodeState reads the nav-pilot state from the opencode output directory.
// Uses ReadStateRaw (no InstallScope validation) because opencode paths follow
// different conventions (skills/, commands/, agents/, AGENTS.md) than .github/.
func ReadOpenCodeState(outputDir string) (*domain.StateFile, error) {
	s, err := ReadStateRaw(openCodeStateFilePath(outputDir))
	if err != nil || s == nil {
		return s, err
	}
	if s.Scope != OpenCodeScopeName {
		return nil, fmt.Errorf("state file scope mismatch: expected %q, got %q", OpenCodeScopeName, s.Scope)
	}
	for _, f := range s.Files {
		if err := ValidateOpenCodeStatePath(f.Path); err != nil {
			return nil, fmt.Errorf("unsafe opencode state file: %w", err)
		}
	}
	return s, nil
}

// WriteOpenCodeState writes the nav-pilot state to the opencode output directory.
// Uses WriteStateAt (atomic write + symlink guard) with outputDir as the boundary.
func WriteOpenCodeState(outputDir string, state *domain.StateFile) error {
	return WriteStateAt(openCodeStateFilePath(outputDir), outputDir, state)
}

// ValidateOpenCodeStatePath checks that a path in the opencode state file is safe.
// OpenCode artifacts live outside .github/, so different prefix rules apply.
func ValidateOpenCodeStatePath(p string) error {
	if filepath.IsAbs(p) {
		return fmt.Errorf("absolute path not allowed: %s", p)
	}
	if strings.Contains(p, "..") {
		return fmt.Errorf("path traversal not allowed: %s", p)
	}
	normalized := filepath.ToSlash(p)
	if normalized == "AGENTS.md" {
		return nil
	}
	for _, prefix := range []string{"skills/", "commands/", "agents/", "instructions/"} {
		if strings.HasPrefix(normalized, prefix) {
			return nil
		}
	}
	return fmt.Errorf("path outside allowed opencode directories: %s", p)
}

// NavPilotOwns reports whether a tracked file is still byte-for-byte what
// nav-pilot wrote, and is therefore nav-pilot's to remove. A conflicted entry
// never is: its recorded hash is the user's own copy, so a hash comparison
// would call it untouched.
//
// Note that the write path disagrees about a conflicted entry: stateHashes in
// [SyncOpenCodeArtifacts] leaves it out, so the sync after a conflict is
// reported treats the path as untracked and overwrites it. This function treats
// it as the user's for good. The write path is the older half and the wrong
// one, but it is a separate bug with its own issue, not something to change
// under a fix for what the scope materializes.
func NavPilotOwns(outputDir string, f domain.InstalledFile) bool {
	if f.Status == domain.FileStatusConflict {
		return false
	}
	rel := filepath.Join(outputDir, f.Path)
	current, err := source.RawArtifactHash(rel, strings.HasSuffix(f.Path, "/"))
	if os.IsNotExist(err) {
		return true // already gone; removing it is a no-op
	}
	if err != nil {
		return false // unreadable is not permission to delete
	}
	return current == f.Hash
}

// withScopeExtras appends the artifacts of a kind that the installed scope has
// and the source checkout does not. Names already present in entries are left
// alone, so the source stays authoritative and the order of the source entries
// is unchanged.
func withScopeExtras(entries []source.Resolved, scopeDir string, kind *source.ArtifactKind) []source.Resolved {
	if scopeDir == "" {
		return entries
	}
	seen := make(map[string]bool, len(entries))
	for _, e := range entries {
		seen[e.Name] = true
	}
	for _, e := range source.NewSourceResolver(scopeDir).List(kind) {
		if !seen[e.Name] {
			entries = append(entries, e)
		}
	}
	return entries
}

// SyncOpenCodeArtifacts materializes Nav context into outputDir with conflict detection
// and state tracking. It is the state-aware counterpart to MaterializeOpenCode.
//
// scopeDir is the installed scope the session runs against (a repo's .github/),
// or "" when there is none. Skills, prompts and agents that live in the scope
// but not in the source checkout are materialized too: a team that adds a skill
// by hand under .github/skills/ gets it in Copilot because Copilot reads the
// scope directly, and used to lose it in opencode because only the source was
// read here. The source still wins on a name collision, so a scope with nothing
// hand-added produces exactly what it did before.
//
// Instructions are deliberately not merged from the scope. outputDir is the
// user's global opencode config, and AGENTS.md is always-on context: a repo's
// instructions would be in every prompt in every other repo until the next sync.
// A skill or an agent from another repo is visible there too, by name and
// description in the tool listing and the agent picker, but its body is only
// read when it is invoked. That is a smaller price than always-on prose, which
// is why the two are treated differently. [ExportOpenCode] writes project-local
// files instead, and merges instructions for that reason.
func SyncOpenCodeArtifacts(sourceDir, scopeDir, outputDir, sourceVersion, sourceSHA, sourceRepo string) (skills, commands, agents, instructions int, conflicts []string, err error) {
	existingState, _ := ReadOpenCodeState(outputDir)
	stateHashes := map[string]string{}
	if existingState != nil {
		for _, f := range existingState.Files {
			if f.Status != domain.FileStatusConflict {
				stateHashes[f.Path] = f.Hash
			}
		}
	}

	isConflict := func(relPath, dstPath string, isDir bool) bool {
		storedHash, inState := stateHashes[relPath]
		if !inState {
			return false
		}
		if _, statErr := os.Stat(dstPath); os.IsNotExist(statErr) {
			return false
		}
		currentHash, hashErr := source.RawArtifactHash(dstPath, isDir)
		if hashErr != nil {
			return false
		}
		return currentHash != storedHash
	}

	var files []domain.InstalledFile
	resolver := source.NewSourceResolver(sourceDir)

	// OpenCode has no tool-deny mechanism, so a preToolUse gate has nothing to
	// attach to there. Said out loud rather than skipped in silence: a user who
	// installed an enforcement hook and then exported to opencode would
	// otherwise believe the gate came along. ValidateOpenCodeStatePath keeps
	// refusing hooks/ regardless, so nothing can slip in through state either.
	if skipped := resolver.List(source.KindHook); len(skipped) > 0 {
		names := make([]string, len(skipped))
		for i, h := range skipped {
			names[i] = h.Name
		}
		fmt.Printf("  %s %d hook(s) not exported: %s. OpenCode has no tool-deny mechanism, so the gate cannot run there.\n",
			domain.Yellow("⚠"), len(names), strings.Join(names, ", "))
	}

	for _, skill := range withScopeExtras(resolver.List(source.KindSkill), scopeDir, source.KindSkill) {
		relPath := "skills/" + skill.Name + "/"
		dstDir := filepath.Join(outputDir, "skills", skill.Name)
		if isConflict(relPath, dstDir, true) {
			h, _ := source.RawArtifactHash(dstDir, true)
			files = append(files, domain.InstalledFile{Path: relPath, Hash: h, Status: domain.FileStatusConflict})
			conflicts = append(conflicts, relPath)
			continue
		}
		if err := source.CheckSymlink(dstDir, outputDir); err != nil {
			return skills, commands, agents, instructions, conflicts, fmt.Errorf("skill %s: %w", skill.Name, err)
		}
		if mkErr := os.MkdirAll(filepath.Dir(dstDir), 0o755); mkErr != nil {
			return skills, commands, agents, instructions, conflicts, mkErr
		}
		if cpErr := copyDirSimple(skill.AbsPath, dstDir); cpErr != nil {
			return skills, commands, agents, instructions, conflicts, fmt.Errorf("skill %s: %w", skill.Name, cpErr)
		}
		h, _ := source.RawArtifactHash(dstDir, true)
		files = append(files, domain.InstalledFile{Path: relPath, Hash: h})
		skills++
	}

	for _, entry := range withScopeExtras(resolver.List(source.KindPrompt), scopeDir, source.KindPrompt) {
		if entry.IsDir {
			continue
		}
		relPath := "commands/" + entry.Name + ".md"
		dstPath := filepath.Join(outputDir, "commands", entry.Name+".md")
		if isConflict(relPath, dstPath, false) {
			h, _ := source.RawArtifactHash(dstPath, false)
			files = append(files, domain.InstalledFile{Path: relPath, Hash: h, Status: domain.FileStatusConflict})
			conflicts = append(conflicts, relPath)
			continue
		}
		data, readErr := os.ReadFile(entry.AbsPath)
		if readErr != nil {
			return skills, commands, agents, instructions, conflicts, fmt.Errorf("prompt %s: %w", entry.Name, readErr)
		}
		if err := source.CheckSymlink(dstPath, outputDir); err != nil {
			return skills, commands, agents, instructions, conflicts, fmt.Errorf("command %s: %w", entry.Name, err)
		}
		if wErr := writeFile(dstPath, transformPrompt(data)); wErr != nil {
			return skills, commands, agents, instructions, conflicts, fmt.Errorf("command %s: %w", entry.Name, wErr)
		}
		h, _ := source.RawArtifactHash(dstPath, false)
		files = append(files, domain.InstalledFile{Path: relPath, Hash: h})
		commands++
	}

	for _, entry := range withScopeExtras(agentEntries(sourceDir), scopeDir, source.KindAgent) {
		relPath := "agents/" + entry.Name + ".md"
		dstPath := filepath.Join(outputDir, "agents", entry.Name+".md")
		if isConflict(relPath, dstPath, false) {
			h, _ := source.RawArtifactHash(dstPath, false)
			files = append(files, domain.InstalledFile{Path: relPath, Hash: h, Status: domain.FileStatusConflict})
			conflicts = append(conflicts, relPath)
			continue
		}
		data, readErr := os.ReadFile(entry.AbsPath)
		if readErr != nil {
			return skills, commands, agents, instructions, conflicts, fmt.Errorf("agent %s: %w", entry.Name, readErr)
		}
		if err := source.CheckSymlink(dstPath, outputDir); err != nil {
			return skills, commands, agents, instructions, conflicts, fmt.Errorf("agent %s: %w", entry.Name, err)
		}
		if wErr := writeFile(dstPath, transformAgent(data, entry.Name)); wErr != nil {
			return skills, commands, agents, instructions, conflicts, fmt.Errorf("agent %s: %w", entry.Name, wErr)
		}
		h, _ := source.RawArtifactHash(dstPath, false)
		files = append(files, domain.InstalledFile{Path: relPath, Hash: h})
		agents++
	}

	globalSections, scopedRefs, collErr := collectInstructionData(sourceDir)
	if collErr != nil {
		return skills, commands, agents, instructions, conflicts, collErr
	}
	if len(globalSections) > 0 || len(scopedRefs) > 0 {
		for _, ref := range scopedRefs {
			relPath := "instructions/" + ref.Name + ".md"
			dstPath := filepath.Join(outputDir, "instructions", ref.Name+".md")
			if isConflict(relPath, dstPath, false) {
				h, _ := source.RawArtifactHash(dstPath, false)
				files = append(files, domain.InstalledFile{Path: relPath, Hash: h, Status: domain.FileStatusConflict})
				conflicts = append(conflicts, relPath)
				continue
			}
			if err := source.CheckSymlink(dstPath, outputDir); err != nil {
				return skills, commands, agents, instructions, conflicts, fmt.Errorf("instruction %s: %w", ref.Name, err)
			}
			if wErr := writeFile(dstPath, ref.Body); wErr != nil {
				return skills, commands, agents, instructions, conflicts, fmt.Errorf("instruction %s: %w", ref.Name, wErr)
			}
			h, _ := source.RawArtifactHash(dstPath, false)
			files = append(files, domain.InstalledFile{Path: relPath, Hash: h})
		}

		agentsMDPath := filepath.Join(outputDir, "AGENTS.md")
		if isConflict("AGENTS.md", agentsMDPath, false) {
			h, _ := source.RawArtifactHash(agentsMDPath, false)
			files = append(files, domain.InstalledFile{Path: "AGENTS.md", Hash: h, Status: domain.FileStatusConflict})
			conflicts = append(conflicts, "AGENTS.md")
		} else {
			agentsMD := buildLeanAGENTSmd(globalSections, scopedRefs)
			if err := source.CheckSymlink(agentsMDPath, outputDir); err != nil {
				return skills, commands, agents, instructions, conflicts, fmt.Errorf("AGENTS.md: %w", err)
			}
			if wErr := writeFile(agentsMDPath, agentsMD); wErr != nil {
				return skills, commands, agents, instructions, conflicts, fmt.Errorf("AGENTS.md: %w", wErr)
			}
			h, _ := source.RawArtifactHash(agentsMDPath, false)
			files = append(files, domain.InstalledFile{Path: "AGENTS.md", Hash: h})
		}
		instructions = len(globalSections) + len(scopedRefs)
	}

	// Build map of new/updated files
	newFilesMap := make(map[string]bool)
	for _, f := range files {
		newFilesMap[f.Path] = true
	}

	// Delete old files that are not in the new file set, but only the ones
	// nav-pilot still owns: an entry marked conflict, or one whose bytes have
	// changed since nav-pilot wrote them, belongs to the user now. The set
	// shrinks for two reasons, and only one of them is a removal upstream. The
	// other is a scope that is no longer there, which happens on every switch to
	// another repo, so deleting on hash-blind absence would throw away a locally
	// edited file for no better reason than the working directory.
	//
	// A file that survives stays in the state, with the hash and status it had.
	// Dropping it would leave an untracked orphan that the next sync from the
	// scope it came from overwrites without a word, since isConflict only knows
	// paths the state names. Kept in state, that same sync sees the hash differ
	// and reports a conflict instead. The state therefore grows only by files
	// the user has edited, and `nav-pilot status` can show them.
	if existingState != nil {
		for _, f := range existingState.Files {
			if newFilesMap[f.Path] {
				continue
			}
			if !NavPilotOwns(outputDir, f) {
				files = append(files, f)
				continue
			}
			dst := filepath.Join(outputDir, f.Path)
			if strings.HasSuffix(f.Path, "/") {
				os.RemoveAll(dst)
			} else {
				os.Remove(dst)
			}
		}
	}

	newState := &domain.StateFile{
		Collection:  OpenCodeCollection,
		Version:     sourceVersion,
		Scope:       OpenCodeScopeName,
		SourceRepo:  sourceRepo,
		SourceSHA:   sourceSHA,
		InstalledAt: time.Now().UTC().Format("2006-01-02T15:04:05Z07:00"),
		Files:       files,
	}
	// Entries carried over untouched above keep their own unknown keys; the
	// ones this sync rebuilt do not, and neither does the top level (#588).
	newState.PreserveUnknownFrom(existingState)
	if wErr := WriteOpenCodeState(outputDir, newState); wErr != nil {
		fmt.Fprintf(os.Stderr, "%s could not write opencode state: %v\n", domain.Yellow("⚠"), wErr)
	}

	return skills, commands, agents, instructions, conflicts, nil
}

// PrintOpenCodeStatusBlock prints the integrity status of nav-pilot-managed opencode files.
func PrintOpenCodeStatusBlock(outputDir string, state *domain.StateFile) {
	ok, modified, missing, _, modifiedPaths := countFileIntegrity(outputDir, state)

	var conflictPaths []string
	for _, f := range state.Files {
		if f.Status == domain.FileStatusConflict {
			conflictPaths = append(conflictPaths, f.Path)
		}
	}

	fmt.Println(domain.Bold("nav-pilot opencode context status"))
	fmt.Println()
	fmt.Printf("  Collection:  %s\n", domain.Bold(state.Collection))
	fmt.Printf("  Version:     %s\n", state.Version)
	fmt.Printf("  Scope:       %s\n", state.Scope)
	fmt.Printf("  Source:      %s\n", domain.ShortSHA(state.SourceSHA))
	fmt.Printf("  Location:    %s\n", domain.Dim(outputDir))
	fmt.Printf("  Files:       %d\n", len(state.Files))
	fmt.Println()

	for _, p := range modifiedPaths {
		fmt.Printf("  %s %s (modified locally)\n", domain.Yellow("~"), p)
	}
	for _, p := range conflictPaths {
		fmt.Printf("  %s %s (conflict — nav-pilot will not overwrite)\n", domain.Yellow("⊘"), p)
	}

	statusLine := fmt.Sprintf("\n  %s %d ok, %s %d modified, %s %d missing",
		domain.Green("✓"), ok, domain.Yellow("~"), modified, domain.Red("✗"), missing)
	if len(conflictPaths) > 0 {
		statusLine += fmt.Sprintf(", %s %d conflict(s)", domain.Yellow("⊘"), len(conflictPaths))
	}
	fmt.Println(statusLine)
}

func countFileIntegrity(rootDir string, state *domain.StateFile) (ok, modified, missing, ignored int, modifiedPaths []string) {
	for _, f := range state.Files {
		if f.Status == domain.FileStatusIgnored {
			ignored++
			continue
		}
		path := filepath.Join(rootDir, f.Path)
		var currentHash string
		var hashErr error
		if strings.HasSuffix(f.Path, "/") {
			currentHash, hashErr = source.DirHash(path)
		} else {
			currentHash, hashErr = source.FileHash(path)
		}
		if hashErr != nil {
			missing++
			continue
		}
		if currentHash != f.Hash {
			modified++
			modifiedPaths = append(modifiedPaths, f.Path)
		} else {
			ok++
		}
	}
	return
}
