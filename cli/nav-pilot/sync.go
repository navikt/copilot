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
	UpToDate bool         `json:"up_to_date"`
	Source   string       `json:"source"`
	Updates  []syncUpdate `json:"updates,omitempty"`
	Errors   []string     `json:"errors,omitempty"`
}

type syncUpdate struct {
	Path        string `json:"path"`
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
func cmdSync(targetDir, ref, sourceRepo string, apply, jsonOutput bool) error {
	src, err := resolveSource(ref, sourceRepo)
	if err != nil {
		return err
	}
	defer src.Cleanup()

	// Determine which files to check
	files, collection, err := resolveSyncFiles(targetDir, src.Dir)
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

	// Compare each file against source
	var updates []syncUpdate
	var syncErrors []string
	for _, sf := range files {
		u, err := checkSyncFile(targetDir, src.Dir, sf)
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

	result := syncResult{
		UpToDate: len(updates) == 0 && len(syncErrors) == 0,
		Source:   src.SHA,
		Updates:  updates,
		Errors:   syncErrors,
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
	for _, u := range updates {
		if err := applySyncUpdate(targetDir, src.Dir, u); err != nil {
			fmt.Fprintf(os.Stderr, "%s Could not update %s: %v\n", yellow("⚠"), u.Path, err)
			continue
		}
		fmt.Printf("  %s %s\n", green("✓"), u.Path)
		applied++
	}
	fmt.Printf("\n%s Updated %d file(s).\n", green("✓"), applied)

	// Update state file if it exists
	if err := updateStateHashes(targetDir, updates); err != nil {
		fmt.Fprintf(os.Stderr, "%s Could not update state file: %v\n", yellow("⚠"), err)
	}

	// If state-based, also update the source SHA
	if state, err := readState(targetDir); err == nil && state != nil {
		state.SourceSHA = src.SHA
		if collection != "" {
			// Reload manifest to get latest version
			if m, err := loadManifest(src.Dir, collection); err == nil {
				state.Version = m.Version
			}
		}
		if err := writeState(targetDir, state); err != nil {
			fmt.Fprintf(os.Stderr, "%s Could not update state: %v\n", yellow("⚠"), err)
		}
	}

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
func resolveSyncFiles(targetDir, sourceDir string) ([]syncFile, string, error) {
	state, err := readState(targetDir)
	if err != nil {
		return nil, "", fmt.Errorf("reading state: %w", err)
	}

	if state != nil {
		// State-based: check all installed files
		var files []syncFile
		for _, f := range state.Files {
			files = append(files, syncFile{
				localPath:  f.Path,
				sourcePath: f.Path,
				isDir:      strings.HasSuffix(f.Path, "/"),
			})
		}
		return files, state.Collection, nil
	}

	// Auto-detect: scan for customization files that also exist in source
	return autoDetectSyncFiles(targetDir, sourceDir)
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

	// Check skill directories
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
			sourceSkill := filepath.Join(sourceDir, ".github", "skills", e.Name())
			if _, err := os.Stat(sourceSkill); os.IsNotExist(err) {
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
		return &syncUpdate{Path: sf.localPath, CurrentHash: localHash, SourceHash: sourceHash}, nil
	}

	localHash, err := fileHash(localFull)
	if err != nil {
		return nil, fmt.Errorf("hashing local: %w", err)
	}
	sourceHash, err := fileHash(sourceFull)
	if err != nil {
		return nil, fmt.Errorf("hashing source: %w", err)
	}
	if localHash == sourceHash {
		return nil, nil
	}
	return &syncUpdate{Path: sf.localPath, CurrentHash: localHash, SourceHash: sourceHash}, nil
}

// applySyncUpdate copies a single file/dir from source to target.
func applySyncUpdate(targetDir, sourceDir string, u syncUpdate) error {
	sourceFull := filepath.Join(sourceDir, u.Path)
	targetFull := filepath.Join(targetDir, u.Path)

	if strings.HasSuffix(u.Path, "/") {
		return copyDir(sourceFull, targetFull)
	}
	return copyFile(sourceFull, targetFull)
}

// updateStateHashes updates the state file with new hashes after applying updates.
func updateStateHashes(targetDir string, updates []syncUpdate) error {
	state, err := readState(targetDir)
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
		path := filepath.Join(targetDir, f.Path)
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

	return writeState(targetDir, state)
}

func outputJSON(v interface{}) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
