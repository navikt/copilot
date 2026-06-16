package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	openCodeStateFileName = ".nav-pilot-state.json"
	// openCodeCollection is the collection name recorded in opencode state files.
	openCodeCollection = "opencode-export"
	// openCodeScopeName identifies opencode state files; distinct from "repo"/"user".
	openCodeScopeName = "opencode"
)

// openCodeStateFilePath returns the nav-pilot state file path inside the opencode output dir.
func openCodeStateFilePath(outputDir string) string {
	return filepath.Join(outputDir, openCodeStateFileName)
}

// readOpenCodeState reads the nav-pilot state from the opencode output directory.
// Uses readStateRaw (no InstallScope validation) because opencode paths follow
// different conventions (skills/, commands/, agents/, AGENTS.md) than .github/.
func readOpenCodeState(outputDir string) (*StateFile, error) {
	s, err := readStateRaw(openCodeStateFilePath(outputDir))
	if err != nil || s == nil {
		return s, err
	}
	if s.Scope != openCodeScopeName {
		return nil, fmt.Errorf("state file scope mismatch: expected %q, got %q", openCodeScopeName, s.Scope)
	}
	for _, f := range s.Files {
		if err := validateOpenCodeStatePath(f.Path); err != nil {
			return nil, fmt.Errorf("unsafe opencode state file: %w", err)
		}
	}
	return s, nil
}

// writeOpenCodeState writes the nav-pilot state to the opencode output directory.
// Uses writeStateAt (atomic write + symlink guard) with outputDir as the boundary.
func writeOpenCodeState(outputDir string, state *StateFile) error {
	return writeStateAt(openCodeStateFilePath(outputDir), outputDir, state)
}

// validateOpenCodeStatePath checks that a path in the opencode state file is safe.
// OpenCode artifacts live outside .github/, so different prefix rules apply.
func validateOpenCodeStatePath(p string) error {
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

// syncOpenCodeArtifacts materializes Nav context into outputDir with conflict detection
// and state tracking. It is the state-aware counterpart to materializeOpenCode:
//
//   - Files recorded in state that the user has locally modified are NOT overwritten
//     (conflict); their paths are returned in the conflicts slice.
//   - All other managed files are updated from sourceDir.
//   - State (version, SHA, per-file hashes) is written after every successful run,
//     enabling staleness detection on subsequent invocations.
//
// Non-destructive: unmanaged files in outputDir are never touched.
func syncOpenCodeArtifacts(sourceDir, outputDir, sourceVersion, sourceSHA string) (skills, commands, agents, instructions int, conflicts []string, err error) {
	// Read existing state for conflict detection.
	existingState, _ := readOpenCodeState(outputDir)
	stateHashes := map[string]string{}
	if existingState != nil {
		for _, f := range existingState.Files {
			if f.Status != fileStatusConflict {
				stateHashes[f.Path] = f.Hash
			}
		}
	}

	// isConflict returns true when a file is managed and the user has modified it
	// since nav-pilot last wrote it (stored hash ≠ current disk hash).
	isConflict := func(relPath, dstPath string, isDir bool) bool {
		storedHash, inState := stateHashes[relPath]
		if !inState {
			return false // not yet managed → write freely
		}
		if _, statErr := os.Stat(dstPath); os.IsNotExist(statErr) {
			return false // missing → write freely
		}
		currentHash, hashErr := rawArtifactHash(dstPath, isDir)
		if hashErr != nil {
			return false // unreadable → write anyway
		}
		return currentHash != storedHash
	}

	var files []InstalledFile
	resolver := NewSourceResolver(sourceDir)

	// Skills (1:1 directory copy)
	for _, skill := range resolver.List(KindSkill) {
		relPath := "skills/" + skill.Name + "/"
		dstDir := filepath.Join(outputDir, "skills", skill.Name)
		if isConflict(relPath, dstDir, true) {
			h, _ := rawArtifactHash(dstDir, true)
			files = append(files, InstalledFile{Path: relPath, Hash: h, Status: fileStatusConflict})
			conflicts = append(conflicts, relPath)
			continue
		}
		if mkErr := os.MkdirAll(filepath.Dir(dstDir), 0o755); mkErr != nil {
			return skills, commands, agents, instructions, conflicts, mkErr
		}
		if cpErr := copyDirSimple(skill.AbsPath, dstDir); cpErr != nil {
			return skills, commands, agents, instructions, conflicts,
				fmt.Errorf("skill %s: %w", skill.Name, cpErr)
		}
		h, _ := rawArtifactHash(dstDir, true)
		files = append(files, InstalledFile{Path: relPath, Hash: h})
		skills++
	}

	// Prompts → Commands
	for _, entry := range resolver.List(KindPrompt) {
		if entry.IsDir {
			continue
		}
		relPath := "commands/" + entry.Name + ".md"
		dstPath := filepath.Join(outputDir, "commands", entry.Name+".md")
		if isConflict(relPath, dstPath, false) {
			h, _ := rawArtifactHash(dstPath, false)
			files = append(files, InstalledFile{Path: relPath, Hash: h, Status: fileStatusConflict})
			conflicts = append(conflicts, relPath)
			continue
		}
		data, readErr := os.ReadFile(entry.AbsPath)
		if readErr != nil {
			return skills, commands, agents, instructions, conflicts,
				fmt.Errorf("prompt %s: %w", entry.Name, readErr)
		}
		if wErr := writeFile(dstPath, transformPrompt(data)); wErr != nil {
			return skills, commands, agents, instructions, conflicts,
				fmt.Errorf("command %s: %w", entry.Name, wErr)
		}
		h, _ := rawArtifactHash(dstPath, false)
		files = append(files, InstalledFile{Path: relPath, Hash: h})
		commands++
	}

	// Agents
	for _, entry := range resolver.List(KindAgent) {
		relPath := "agents/" + entry.Name + ".md"
		dstPath := filepath.Join(outputDir, "agents", entry.Name+".md")
		if isConflict(relPath, dstPath, false) {
			h, _ := rawArtifactHash(dstPath, false)
			files = append(files, InstalledFile{Path: relPath, Hash: h, Status: fileStatusConflict})
			conflicts = append(conflicts, relPath)
			continue
		}
		data, readErr := os.ReadFile(entry.AbsPath)
		if readErr != nil {
			return skills, commands, agents, instructions, conflicts,
				fmt.Errorf("agent %s: %w", entry.Name, readErr)
		}
		if wErr := writeFile(dstPath, transformAgent(data)); wErr != nil {
			return skills, commands, agents, instructions, conflicts,
				fmt.Errorf("agent %s: %w", entry.Name, wErr)
		}
		h, _ := rawArtifactHash(dstPath, false)
		files = append(files, InstalledFile{Path: relPath, Hash: h})
		agents++
	}

	// Instructions → AGENTS.md + individual scoped files
	globalSections, scopedRefs, collErr := collectInstructionData(sourceDir)
	if collErr != nil {
		return skills, commands, agents, instructions, conflicts, collErr
	}
	if len(globalSections) > 0 || len(scopedRefs) > 0 {
		for _, ref := range scopedRefs {
			relPath := "instructions/" + ref.name + ".md"
			dstPath := filepath.Join(outputDir, "instructions", ref.name+".md")
			if isConflict(relPath, dstPath, false) {
				h, _ := rawArtifactHash(dstPath, false)
				files = append(files, InstalledFile{Path: relPath, Hash: h, Status: fileStatusConflict})
				conflicts = append(conflicts, relPath)
				continue
			}
			if wErr := writeFile(dstPath, ref.body); wErr != nil {
				return skills, commands, agents, instructions, conflicts,
					fmt.Errorf("instruction %s: %w", ref.name, wErr)
			}
			h, _ := rawArtifactHash(dstPath, false)
			files = append(files, InstalledFile{Path: relPath, Hash: h})
		}

		// AGENTS.md — the central context file
		agentsMDPath := filepath.Join(outputDir, "AGENTS.md")
		if isConflict("AGENTS.md", agentsMDPath, false) {
			h, _ := rawArtifactHash(agentsMDPath, false)
			files = append(files, InstalledFile{Path: "AGENTS.md", Hash: h, Status: fileStatusConflict})
			conflicts = append(conflicts, "AGENTS.md")
		} else {
			agentsMD := buildLeanAGENTSmd(globalSections, scopedRefs)
			if wErr := writeFile(agentsMDPath, agentsMD); wErr != nil {
				return skills, commands, agents, instructions, conflicts,
					fmt.Errorf("AGENTS.md: %w", wErr)
			}
			h, _ := rawArtifactHash(agentsMDPath, false)
			files = append(files, InstalledFile{Path: "AGENTS.md", Hash: h})
		}
		// Count instructions regardless of AGENTS.md conflict — the file still exists.
		instructions = len(globalSections) + len(scopedRefs)
	}

	// Persist state so the next run can detect staleness and conflicts.
	newState := &StateFile{
		Collection:  openCodeCollection,
		Version:     sourceVersion,
		Scope:       openCodeScopeName,
		SourceSHA:   sourceSHA,
		InstalledAt: timeNow().UTC().Format("2006-01-02T15:04:05Z07:00"),
		Files:       files,
	}
	if wErr := writeOpenCodeState(outputDir, newState); wErr != nil {
		fmt.Fprintf(os.Stderr, "%s could not write opencode state: %v\n", yellow("⚠"), wErr)
	}

	return skills, commands, agents, instructions, conflicts, nil
}

// printOpenCodeStatusBlock prints the integrity status of nav-pilot-managed opencode files.
// Reuses countFileIntegrity so the display is consistent with printStatusBlock.
func printOpenCodeStatusBlock(outputDir string, state *StateFile) {
	// countFileIntegrity uses fileHash (not normalizedFileHash) for consistency
	// with how hashes are stored in state.
	ok, modified, missing, _, modifiedPaths := countFileIntegrity(outputDir, state)

	// Conflicts stored in state (files the user modified before nav-pilot tracked them)
	var conflictPaths []string
	for _, f := range state.Files {
		if f.Status == fileStatusConflict {
			conflictPaths = append(conflictPaths, f.Path)
		}
	}

	fmt.Println(bold("nav-pilot opencode context status"))
	fmt.Println()
	fmt.Printf("  Collection:  %s\n", bold(state.Collection))
	fmt.Printf("  Version:     %s\n", state.Version)
	fmt.Printf("  Scope:       %s\n", state.Scope)
	fmt.Printf("  Source:      %s\n", state.SourceSHA)
	fmt.Printf("  Location:    %s\n", dim(outputDir))
	fmt.Printf("  Files:       %d\n", len(state.Files))
	fmt.Println()

	for _, p := range modifiedPaths {
		fmt.Printf("  %s %s (modified locally)\n", yellow("~"), p)
	}
	for _, p := range conflictPaths {
		fmt.Printf("  %s %s (conflict — nav-pilot will not overwrite)\n", yellow("⊘"), p)
	}

	statusLine := fmt.Sprintf("\n  %s %d ok, %s %d modified, %s %d missing",
		green("✓"), ok, yellow("~"), modified, red("✗"), missing)
	if len(conflictPaths) > 0 {
		statusLine += fmt.Sprintf(", %s %d conflict(s)", yellow("⊘"), len(conflictPaths))
	}
	fmt.Println(statusLine)
}
