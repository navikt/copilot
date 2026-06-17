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

// SyncOpenCodeArtifacts materializes Nav context into outputDir with conflict detection
// and state tracking. It is the state-aware counterpart to MaterializeOpenCode.
func SyncOpenCodeArtifacts(sourceDir, outputDir, sourceVersion, sourceSHA string) (skills, commands, agents, instructions int, conflicts []string, err error) {
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

	for _, skill := range resolver.List(source.KindSkill) {
		relPath := "skills/" + skill.Name + "/"
		dstDir := filepath.Join(outputDir, "skills", skill.Name)
		if isConflict(relPath, dstDir, true) {
			h, _ := source.RawArtifactHash(dstDir, true)
			files = append(files, domain.InstalledFile{Path: relPath, Hash: h, Status: domain.FileStatusConflict})
			conflicts = append(conflicts, relPath)
			continue
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

	for _, entry := range resolver.List(source.KindPrompt) {
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
		if wErr := writeFile(dstPath, transformPrompt(data)); wErr != nil {
			return skills, commands, agents, instructions, conflicts, fmt.Errorf("command %s: %w", entry.Name, wErr)
		}
		h, _ := source.RawArtifactHash(dstPath, false)
		files = append(files, domain.InstalledFile{Path: relPath, Hash: h})
		commands++
	}

	for _, entry := range resolver.List(source.KindAgent) {
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
		if wErr := writeFile(dstPath, transformAgent(data)); wErr != nil {
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
			if wErr := writeFile(agentsMDPath, agentsMD); wErr != nil {
				return skills, commands, agents, instructions, conflicts, fmt.Errorf("AGENTS.md: %w", wErr)
			}
			h, _ := source.RawArtifactHash(agentsMDPath, false)
			files = append(files, domain.InstalledFile{Path: "AGENTS.md", Hash: h})
		}
		instructions = len(globalSections) + len(scopedRefs)
	}

	newState := &domain.StateFile{
		Collection:  OpenCodeCollection,
		Version:     sourceVersion,
		Scope:       OpenCodeScopeName,
		SourceSHA:   sourceSHA,
		InstalledAt: time.Now().UTC().Format("2006-01-02T15:04:05Z07:00"),
		Files:       files,
	}
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
	fmt.Printf("  Source:      %s\n", state.SourceSHA)
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
