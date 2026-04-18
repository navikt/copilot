package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// syncResult holds the outcome of a sync check for machine-readable output.
type syncResult struct {
	UpToDate  bool         `json:"up_to_date"`
	Source    string       `json:"source"`
	Updates   []syncUpdate `json:"updates,omitempty"`
	Errors    []string     `json:"errors,omitempty"`
	Overrides []string     `json:"overrides,omitempty"`
	Ignored   []string     `json:"ignored,omitempty"`
}

type syncUpdate struct {
	Path        string `json:"path"`
	SourcePath  string `json:"-"` // resolved source path, not serialized
	CurrentHash string `json:"current_hash"`
	SourceHash  string `json:"source_hash"`
}

// errUpdatesAvailable is returned when sync finds updates but --apply is not set.
// main() maps this to exit code 1 for CI use.
var errUpdatesAvailable = fmt.Errorf("updates available")

// errSyncFailed is returned when sync encounters errors checking files.
// main() maps this to exit code 2 to distinguish from "updates available".
var errSyncFailed = fmt.Errorf("sync failed")

// cmdSync checks installed files against source and optionally applies updates.
//
// Modes:
//   - check (default): report which files differ, exit 1 if updates available
//   - apply: update differing files in place
//
// Works with both state-based repos (nav-pilot install) and auto-detected repos.
func cmdSync(scope *InstallScope, ref, sourceRepo string, apply, jsonOutput bool) error {
	src, err := resolveSource(ref, sourceRepo)
	if err != nil {
		return err
	}
	defer src.Cleanup()

	// Determine which files to check
	files, _, err := resolveSyncFiles(scope, src.Dir)
	if err != nil {
		return err
	}

	if len(files) == 0 {
		if jsonOutput {
			return outputJSON(syncResult{UpToDate: true, Source: src.SHA})
		}
		fmt.Println("No customization files found to sync.")
		return nil
	}

	// Read sync config and filter out overridden files
	cfg, err := readSyncConfig(scope.RootDir)
	if err != nil {
		return fmt.Errorf("reading %s: %w", syncConfigPath, err)
	}
	overrides := overrideSet(cfg)
	var filtered []syncFile
	var overriddenPaths []string
	for _, sf := range files {
		key := filepath.ToSlash(filepath.Clean(sf.localPath))
		if overrides[key] {
			overriddenPaths = append(overriddenPaths, sf.localPath)
			continue
		}
		filtered = append(filtered, sf)
	}
	files = filtered

	if !jsonOutput && len(overriddenPaths) > 0 {
		for _, p := range overriddenPaths {
			fmt.Printf("  %s %s (override)\n", dim("⊘"), p)
		}
		fmt.Println()
	}

	// Compare each file against source.
	// Files that are in state but missing on disk are treated as intentionally
	// deleted — they get marked "ignored" in the state file so future syncs skip them.
	var updates []syncUpdate
	var syncErrors []string
	var ignoredPaths []string
	for _, sf := range files {
		// Check if local file exists; if missing, treat as intentional deletion
		localFull := filepath.Join(scope.RootDir, sf.localPath)
		if _, statErr := os.Stat(localFull); os.IsNotExist(statErr) {
			ignoredPaths = append(ignoredPaths, sf.localPath)
			continue
		}

		u, err := checkSyncFile(scope.RootDir, src.Dir, sf)
		if err != nil {
			if !jsonOutput {
				fmt.Fprintf(os.Stderr, "%s %s: %v\n", yellow("⚠"), sf.localPath, err)
			}
			syncErrors = append(syncErrors, fmt.Sprintf("%s: %v", sf.localPath, err))
			continue
		}
		if u != nil {
			updates = append(updates, *u)
		}
	}

	// Mark missing files as ignored in state
	if len(ignoredPaths) > 0 {
		if err := markFilesIgnored(scope, ignoredPaths); err != nil {
			if !jsonOutput {
				fmt.Fprintf(os.Stderr, "%s Could not update state for deleted files: %v\n", yellow("⚠"), err)
			}
		}
		if !jsonOutput {
			for _, p := range ignoredPaths {
				fmt.Printf("  %s %s (deleted — marked ignored)\n", dim("⊘"), p)
			}
			fmt.Println()
		}
	}

	result := syncResult{
		UpToDate:  len(updates) == 0 && len(syncErrors) == 0,
		Source:    src.SHA,
		Updates:   updates,
		Errors:    syncErrors,
		Overrides: overriddenPaths,
		Ignored:   ignoredPaths,
	}

	if jsonOutput {
		if err := outputJSON(result); err != nil {
			return err
		}
		// Exit 2 if any errors occurred (even with updates)
		if len(syncErrors) > 0 {
			return errSyncFailed
		}
		if !result.UpToDate {
			return errUpdatesAvailable
		}
		return nil
	}

	if result.UpToDate {
		fmt.Printf("%s All %d files up to date (source: %s)\n",
			green("✓"), len(files), src.SHA)
		reportNewItems(scope, src.Dir)
		return nil
	}

	// Report updates
	fmt.Printf("%s %d of %d files have updates available (source: %s)\n\n",
		yellow("⚠"), len(updates), len(files), src.SHA)
	for _, u := range updates {
		fmt.Printf("  %s %s\n", yellow("~"), u.Path)
	}
	fmt.Println()

	if !apply {
		fmt.Printf("Run %s to apply updates.\n", bold("nav-pilot sync --apply"))
		return errUpdatesAvailable
	}

	// Apply updates
	applied := 0
	var appliedUpdates []syncUpdate
	var applyErrors int
	for _, u := range updates {
		if err := applySyncUpdate(scope, src.Dir, u); err != nil {
			fmt.Fprintf(os.Stderr, "%s Could not update %s: %v\n", yellow("⚠"), u.Path, err)
			applyErrors++
			continue
		}
		fmt.Printf("  %s %s\n", green("✓"), u.Path)
		applied++
		appliedUpdates = append(appliedUpdates, u)
	}
	fmt.Printf("\n%s Updated %d file(s).\n", green("✓"), applied)

	// Update state with new hashes
	if err := updateScopedStateHashes(scope, appliedUpdates); err != nil {
		fmt.Fprintf(os.Stderr, "%s Could not update state file: %v\n", yellow("⚠"), err)
	}

	// Only bump source SHA if ALL updates were applied successfully
	if state, err := readScopedState(scope); err == nil && state != nil {
		if applyErrors == 0 {
			state.SourceSHA = src.SHA
		}
		// Use the binary's release version directly.
		// "dev" means local/unreleased build — checkStaleness() skips it.
		if src.Version != "" {
			state.Version = src.Version
		}
		if err := writeScopedState(scope, state); err != nil {
			fmt.Fprintf(os.Stderr, "%s Could not update state: %v\n", yellow("⚠"), err)
		}
	}

	if applyErrors > 0 {
		return errSyncFailed
	}

	reportNewItems(scope, src.Dir)
	return nil
}

// syncFile represents a file to check during sync.
type syncFile struct {
	localPath  string // relative path in target repo (e.g. ".github/agents/nais.agent.md")
	sourcePath string // relative path in source repo (same unless remapped)
	isDir      bool
}

// resolveSyncFiles determines which files to sync.
// If a state file exists, uses the installed file list.
// Otherwise, auto-detects customization files in the target repo.
func resolveSyncFiles(scope *InstallScope, sourceDir string) ([]syncFile, string, error) {
	state, err := readScopedState(scope)
	if err != nil {
		return nil, "", fmt.Errorf("reading state: %w", err)
	}

	if state != nil {
		// State-based: check all installed files, skip ignored ones
		var files []syncFile
		for _, f := range state.Files {
			if f.Status == fileStatusIgnored {
				continue
			}
			sp := resolveSourcePath(sourceDir, f.Path, scope.IsUser())
			files = append(files, syncFile{
				localPath:  f.Path,
				sourcePath: sp,
				isDir:      strings.HasSuffix(f.Path, "/"),
			})
		}
		return files, state.Collection, nil
	}

	if scope.IsUser() {
		// No auto-detect for user scope without state
		return nil, "", nil
	}

	// Auto-detect: scan for customization files that also exist in source
	return autoDetectSyncFiles(scope.RootDir, sourceDir)
}

// detectNewItems checks if the source has agents/skills/instructions not in the state file.
// Only relevant for "(all)" user-scope installs where new items may appear.
func detectNewItems(scope *InstallScope, sourceDir string) []string {
	state, err := readScopedState(scope)
	if err != nil || state == nil || state.Collection != CollectionAll || !scope.IsUser() {
		return nil
	}

	allItems, err := collectAllItems(sourceDir)
	if err != nil {
		return nil
	}

	installed := make(map[string]bool)
	for _, f := range state.Files {
		installed[f.Path] = true
	}

	var newItems []string
	for _, agent := range allItems.Agents {
		path := "agents/" + agent + ".agent.md"
		if !installed[path] {
			newItems = append(newItems, "agent: "+agent)
		}
	}
	for _, skill := range allItems.Skills {
		path := "skills/" + skill + "/"
		if !installed[path] {
			newItems = append(newItems, "skill: "+skill)
		}
	}
	for _, instr := range allItems.Instructions {
		path := ".github/instructions/" + instr + ".instructions.md"
		if !installed[path] {
			newItems = append(newItems, "instruction: "+instr)
		}
	}
	return newItems
}

// autoDetectSyncFiles finds customization files in the target that also exist in source.
func autoDetectSyncFiles(targetDir, sourceDir string) ([]syncFile, string, error) {
	patterns := []struct {
		glob  string
		isDir bool
	}{
		{".github/agents/*.agent.md", false},
		{".github/agents/*.metadata.json", false},
		{".github/instructions/*.instructions.md", false},
		{".github/prompts/*.prompt.md", false},
	}

	var files []syncFile
	seen := make(map[string]bool)

	for _, p := range patterns {
		matches, err := filepath.Glob(filepath.Join(targetDir, p.glob))
		if err != nil {
			continue
		}
		for _, m := range matches {
			rel, _ := filepath.Rel(targetDir, m)
			if seen[rel] {
				continue
			}
			// Only include if source also has this file
			sourcePath := filepath.Join(sourceDir, rel)
			if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
				continue
			}
			seen[rel] = true
			files = append(files, syncFile{localPath: rel, sourcePath: rel, isDir: p.isDir})
		}
	}

	// Check skill directories — source may be at root skills/ or .github/skills/
	skillsDir := filepath.Join(targetDir, ".github", "skills")
	if entries, err := os.ReadDir(skillsDir); err == nil {
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			rel := filepath.Join(".github", "skills", e.Name()) + "/"
			if seen[rel] {
				continue
			}
			// Resolve source: try root-level first, then legacy
			srcRel := filepath.Join(".github", "skills", e.Name()) + "/"
			rootSkill := filepath.Join(sourceDir, "skills", e.Name())
			legacySkill := filepath.Join(sourceDir, ".github", "skills", e.Name())
			if _, err := os.Stat(rootSkill); err == nil {
				srcRel = filepath.Join("skills", e.Name()) + "/"
			} else if _, err := os.Stat(legacySkill); os.IsNotExist(err) {
				continue
			}
			seen[rel] = true
			files = append(files, syncFile{localPath: rel, sourcePath: srcRel, isDir: true})
		}
	}

	// Check prompt directories
	promptsDir := filepath.Join(targetDir, ".github", "prompts")
	if entries, err := os.ReadDir(promptsDir); err == nil {
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			rel := filepath.Join(".github", "prompts", e.Name()) + "/"
			if seen[rel] {
				continue
			}
			sourcePrompt := filepath.Join(sourceDir, ".github", "prompts", e.Name())
			if _, err := os.Stat(sourcePrompt); os.IsNotExist(err) {
				continue
			}
			seen[rel] = true
			files = append(files, syncFile{localPath: rel, sourcePath: rel, isDir: true})
		}
	}

	return files, "", nil
}

// checkSyncFile compares a single file/dir between target and source.
func checkSyncFile(targetDir, sourceDir string, sf syncFile) (*syncUpdate, error) {
	localFull := filepath.Join(targetDir, sf.localPath)
	sourceFull := filepath.Join(sourceDir, sf.sourcePath)

	if sf.isDir {
		localHash, err := dirHash(localFull)
		if err != nil {
			return nil, fmt.Errorf("hashing local: %w", err)
		}
		sourceHash, err := dirHash(sourceFull)
		if err != nil {
			return nil, fmt.Errorf("hashing source: %w", err)
		}
		if localHash == sourceHash {
			return nil, nil
		}
		return &syncUpdate{Path: sf.localPath, SourcePath: sf.sourcePath, CurrentHash: localHash, SourceHash: sourceHash}, nil
	}

	localHash, err := normalizedFileHash(localFull)
	if err != nil {
		return nil, fmt.Errorf("hashing local: %w", err)
	}
	sourceHash, err := normalizedFileHash(sourceFull)
	if err != nil {
		return nil, fmt.Errorf("hashing source: %w", err)
	}
	if localHash == sourceHash {
		return nil, nil
	}
	return &syncUpdate{Path: sf.localPath, SourcePath: sf.sourcePath, CurrentHash: localHash, SourceHash: sourceHash}, nil
}

// applySyncUpdate copies a single file/dir from source to target.
func applySyncUpdate(scope *InstallScope, sourceDir string, u syncUpdate) error {
	sourceFull := filepath.Join(sourceDir, u.SourcePath)
	targetFull := filepath.Join(scope.RootDir, u.Path)

	if strings.HasSuffix(u.Path, "/") {
		return copyDir(sourceFull, targetFull, scope.RootDir)
	}
	return copyFile(sourceFull, targetFull, scope.RootDir)
}

// updateScopedStateHashes updates the state file with new hashes after applying updates.
func updateScopedStateHashes(scope *InstallScope, updates []syncUpdate) error {
	state, err := readScopedState(scope)
	if err != nil || state == nil {
		return nil // no state file, nothing to update
	}

	updateMap := make(map[string]bool)
	for _, u := range updates {
		updateMap[u.Path] = true
	}

	for i, f := range state.Files {
		if !updateMap[f.Path] {
			continue
		}
		path := filepath.Join(scope.RootDir, f.Path)
		var hash string
		if strings.HasSuffix(f.Path, "/") {
			hash, err = dirHash(path)
		} else {
			hash, err = fileHash(path)
		}
		if err != nil {
			continue
		}
		state.Files[i].Hash = hash
	}

	return writeScopedState(scope, state)
}

// updateStateHashes is a backward-compatible wrapper for repo scope.
func updateStateHashes(targetDir string, updates []syncUpdate) error {
	return updateScopedStateHashes(ScopeRepo(targetDir), updates)
}

// markFilesIgnored updates the state file to mark the given paths as "ignored".
// This prevents future syncs from re-adding files that were intentionally deleted.
func markFilesIgnored(scope *InstallScope, paths []string) error {
	state, err := readScopedState(scope)
	if err != nil || state == nil {
		return nil
	}

	pathSet := make(map[string]bool, len(paths))
	for _, p := range paths {
		pathSet[p] = true
	}

	for i, f := range state.Files {
		if pathSet[f.Path] {
			state.Files[i].Status = fileStatusIgnored
		}
	}

	return writeScopedState(scope, state)
}

func outputJSON(v interface{}) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// reportNewItems prints a notice if the source has new items not yet installed.
func reportNewItems(scope *InstallScope, sourceDir string) {
	newItems := detectNewItems(scope, sourceDir)
	if len(newItems) == 0 {
		return
	}
	fmt.Println()
	fmt.Printf("%s %d new item(s) in source not yet installed:\n", dim("ℹ"), len(newItems))
	for _, item := range newItems {
		fmt.Printf("    %s\n", item)
	}
	fmt.Printf("  Run %s to add them.\n", bold("nav-pilot install --user"))
}
