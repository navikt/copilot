package cli

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
	Conflicts []string     `json:"conflicts,omitempty"`
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

// cmdSyncFn is overridable in tests.
var cmdSyncFn = cmdSync

// cmdSync checks installed files against source and optionally applies updates.
//
// Modes:
//   - check (default): report which files differ, exit 1 if updates available
//   - apply: update differing files in place
//
// Works with both state-based repos (nav-pilot install) and auto-detected repos.
func cmdSync(scope *InstallScope, ref, sourceRepo string, apply, jsonOutput bool) error {
	src, err := resolveSourceForSync(ref, sourceRepo)
	if err != nil {
		return err
	}
	defer src.Cleanup()

	// Determine which files to check
	files, _, err := resolveSyncFiles(scope, src.Dir, apply)
	if err != nil {
		return err
	}

	conflictPaths := conflictStatePaths(scope)
	if err := clearResolvedConflicts(scope, src.Dir, conflictPaths); err != nil {
		if !jsonOutput {
			fmt.Fprintf(os.Stderr, "%s Could not clear resolved conflicts: %v\n", yellow("⚠"), err)
		}
	}
	// Re-fetch conflictPaths in case any were resolved
	conflictPaths = conflictStatePaths(scope)

	if len(files) == 0 {
		if len(conflictPaths) > 0 && !apply {
			telemetry.RecordSyncConflicts(scope.Name, telemetryMode(), int64(len(conflictPaths)))
			result := syncResult{
				UpToDate:  false,
				Source:    src.SHA,
				Conflicts: conflictPaths,
			}
			if jsonOutput {
				if err := outputJSON(result); err != nil {
					return err
				}
				return errUpdatesAvailable
			}
			fmt.Printf("%s %d file(s) are in conflict state and were skipped (source: %s)\n\n",
				yellow("⚠"), len(conflictPaths), src.SHA)
			for _, p := range conflictPaths {
				fmt.Printf("  %s %s\n", dim("⊘"), p)
			}
			fmt.Println()
			fmt.Printf("Run %s to apply updates.\n", bold("nav-pilot sync --apply"))
			return errUpdatesAvailable
		}
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
		UpToDate:  len(updates) == 0 && len(syncErrors) == 0 && (apply || len(conflictPaths) == 0),
		Source:    src.SHA,
		Updates:   updates,
		Errors:    syncErrors,
		Overrides: overriddenPaths,
		Ignored:   ignoredPaths,
		Conflicts: conflictPaths,
	}
	tMode := telemetryMode()
	if !apply {
		tMode += "_dry_run"
	}
	telemetry.RecordSyncUpdates(scope.Name, tMode, int64(len(result.Updates)))
	telemetry.RecordSyncConflicts(scope.Name, tMode, int64(len(result.Conflicts)))

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
		// Bump state version so staleness check won't re-trigger for this release
		if src.Version != "" {
			if state, err := readScopedState(scope); err == nil && state != nil {
				if state.Version != src.Version || state.SourceSHA != src.SHA {
					state.Version = src.Version
					state.SourceSHA = src.SHA
					if err := writeScopedState(scope, state); err != nil {
						fmt.Fprintf(os.Stderr, "%s Could not update state: %v\n", yellow("⚠"), err)
					}
				}
			}
		}
		reportNewItems(scope, src.Dir)
		return nil
	}

	// Report updates
	if len(updates) > 0 {
		fmt.Printf("%s %d of %d files have updates available (source: %s)\n\n",
			yellow("⚠"), len(updates), len(files), src.SHA)
		for _, u := range updates {
			fmt.Printf("  %s %s\n", yellow("~"), u.Path)
		}
		fmt.Println()
	} else if len(conflictPaths) > 0 && !apply {
		fmt.Printf("%s %d file(s) are in conflict state and were skipped (source: %s)\n\n",
			yellow("⚠"), len(conflictPaths), src.SHA)
		for _, p := range conflictPaths {
			fmt.Printf("  %s %s\n", dim("⊘"), p)
		}
		fmt.Println()
	}

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

	// Only bump source SHA and version if ALL updates were applied successfully
	if state, err := readScopedState(scope); err == nil && state != nil {
		if applyErrors == 0 {
			state.SourceSHA = src.SHA
			// Use the binary's release version directly.
			// "dev" means local/unreleased build — checkStaleness() skips it.
			if src.Version != "" {
				state.Version = src.Version
			}
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

// cmdSyncAuto syncs all detected scopes (repo + user) when the user didn't
// explicitly pick one with --user or --target. Mirrors how the interactive
// flow and `list --installed` handle scope discovery.
func cmdSyncAuto(repoDir, ref, sourceRepo string, apply, jsonOutput bool) error {
	repoScope := ScopeRepo(repoDir)
	repoState, _ := readScopedState(repoScope)

	userScope, userErr := ScopeUser()
	var userState *StateFile
	if userErr == nil {
		userState, _ = readScopedState(userScope)
	}

	if repoState == nil && userState == nil {
		if jsonOutput {
			return outputJSON(map[string]interface{}{"installed": false})
		}
		fmt.Println("No nav-pilot collection installed (repo or user scope).")
		fmt.Printf("Install with: %s\n", bold("nav-pilot install <collection>"))
		return nil
	}

	var firstErr error

	if repoState != nil {
		if !jsonOutput {
			fmt.Printf("%s Syncing %s scope...\n", dim("→"), bold("repo"))
		}
		if err := cmdSyncFn(repoScope, ref, sourceRepo, apply, jsonOutput); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			if !jsonOutput {
				if err == errUpdatesAvailable {
					fmt.Printf("%s Repo scope has updates available.\n", yellow("⚠"))
				} else {
					fmt.Printf("%s Repo scope sync failed.\n", yellow("⚠"))
				}
			}
		} else if !jsonOutput {
			fmt.Printf("%s Repo scope synced.\n", green("✓"))
		}
	}

	if userState != nil {
		if !jsonOutput {
			if repoState != nil {
				fmt.Println()
			}
			fmt.Printf("%s Syncing %s scope...\n", dim("→"), bold("user"))
		}
		if err := cmdSyncFn(userScope, ref, sourceRepo, apply, jsonOutput); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			if !jsonOutput {
				if err == errUpdatesAvailable {
					fmt.Printf("%s User scope has updates available.\n", yellow("⚠"))
				} else {
					fmt.Printf("%s User scope sync failed.\n", yellow("⚠"))
				}
			}
		} else if !jsonOutput {
			fmt.Printf("%s User scope synced.\n", green("✓"))
		}
	}

	// Sync provider-specific context artifacts (e.g. opencode Nav context).
	// Each provider checks its own state and skips silently if not managed.
	hasPrevOutput := repoState != nil || userState != nil
	for _, p := range allProviders() {
		res := p.SyncContext(ref, sourceRepo, jsonOutput, hasPrevOutput)
		if res.Managed {
			hasPrevOutput = true
		}
		if res.Err != nil && firstErr == nil {
			firstErr = errSyncFailed
		}
	}

	return firstErr
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
func resolveSyncFiles(scope *InstallScope, sourceDir string, includeConflicts bool) ([]syncFile, string, error) {
	state, err := readScopedState(scope)
	if err != nil {
		return nil, "", fmt.Errorf("reading state: %w", err)
	}

	if state != nil {
		// State-based: check all installed files, skip ignored and conflicted ones
		resolver := NewSourceResolver(sourceDir)
		var files []syncFile
		for _, f := range state.Files {
			if f.Status == fileStatusIgnored {
				continue
			}
			if f.Status == fileStatusConflict && !includeConflicts {
				continue
			}
			sp := resolver.MapLocalPath(f.Path, scope.IsUser())
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

func conflictStatePaths(scope *InstallScope) []string {
	state, err := readScopedState(scope)
	if err != nil || state == nil {
		return nil
	}
	var conflicts []string
	for _, f := range state.Files {
		if f.Status == fileStatusConflict {
			conflicts = append(conflicts, f.Path)
		}
	}
	return conflicts
}

// detectNewItems checks if the source has agents/skills/instructions not in the state file.
// Only relevant for "(all)" user-scope installs where new items may appear.
func detectNewItems(scope *InstallScope, sourceDir string) []string {
	state, err := readScopedState(scope)
	if err != nil || state == nil || state.Collection != CollectionAll || !scope.IsUser() {
		return nil
	}

	resolver := NewSourceResolver(sourceDir)

	installed := make(map[string]bool)
	for _, f := range state.Files {
		installed[f.Path] = true
	}

	var newItems []string
	for _, kind := range []*ArtifactKind{KindAgent, KindSkill, KindInstruction} {
		for _, art := range resolver.List(kind) {
			relPath := kind.RelPathForName(scope, art.Name)
			if !installed[relPath] {
				newItems = append(newItems, kind.Name+": "+art.Name)
			}
		}
	}
	return newItems
}

// autoDetectSyncFiles finds customization files in the target that also exist in source.
// Target files are always under .github/. Source may be at root or .github/.
func autoDetectSyncFiles(targetDir, sourceDir string) ([]syncFile, string, error) {
	resolver := NewSourceResolver(sourceDir)

	// Build file scan patterns from artifact kind definitions.
	type scanPattern struct {
		glob    string
		typeDir string
		suffix  string
	}
	var patterns []scanPattern
	for _, kind := range AllKinds {
		if kind.Suffix != "" {
			patterns = append(patterns, scanPattern{
				glob:    ".github/" + kind.Dir + "/*" + kind.Suffix,
				typeDir: kind.Dir,
				suffix:  kind.Suffix,
			})
		}
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
			// Resolve source: check root-level first, then .github/
			fileName := filepath.Base(m)
			_, srcRel, ok := resolver.GetFile(p.typeDir, fileName)
			if !ok {
				continue
			}
			seen[rel] = true
			files = append(files, syncFile{localPath: rel, sourcePath: srcRel, isDir: false})
		}
	}

	// Check directory-based artifacts (skills and prompt dirs).
	for _, kind := range AllKinds {
		if !kind.IsDir && !kind.CanBeDir {
			continue
		}
		dir := filepath.Join(targetDir, ".github", kind.Dir)
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			rel := filepath.Join(".github", kind.Dir, e.Name()) + "/"
			if seen[rel] {
				continue
			}
			art, ok := resolver.Get(kind, e.Name())
			if !ok || !art.IsDir {
				continue
			}
			seen[rel] = true
			files = append(files, syncFile{localPath: rel, sourcePath: art.RelPath + "/", isDir: true})
		}
	}

	return files, "", nil
}

// checkSyncFile compares a single file/dir between target and source.
func checkSyncFile(targetDir, sourceDir string, sf syncFile) (*syncUpdate, error) {
	localFull := filepath.Join(targetDir, sf.localPath)
	sourceFull := filepath.Join(sourceDir, sf.sourcePath)

	localHash, err := comparableArtifactHash(localFull, sf.isDir)
	if err != nil {
		return nil, fmt.Errorf("hashing local: %w", err)
	}
	sourceHash, err := comparableArtifactHash(sourceFull, sf.isDir)
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
	return copyArtifact(sourceFull, targetFull, scope.RootDir, strings.HasSuffix(u.Path, "/"))
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
		hash, err := rawArtifactHash(path, strings.HasSuffix(f.Path, "/"))
		if err != nil {
			continue
		}
		state.Files[i].Hash = hash
		state.Files[i].Status = ""
	}

	return writeScopedState(scope, state)
}

// clearResolvedConflicts clears conflict status for files that currently match source.
func clearResolvedConflicts(scope *InstallScope, sourceDir string, conflictPaths []string) error {
	if len(conflictPaths) == 0 {
		return nil
	}

	state, err := readScopedState(scope)
	if err != nil || state == nil {
		return nil
	}

	conflictSet := make(map[string]bool, len(conflictPaths))
	for _, p := range conflictPaths {
		conflictSet[p] = true
	}

	resolver := NewSourceResolver(sourceDir)
	changed := false
	for i, f := range state.Files {
		if !conflictSet[f.Path] {
			continue
		}

		localFull := filepath.Join(scope.RootDir, f.Path)
		sourcePath := resolver.MapLocalPath(f.Path, scope.IsUser())
		sourceFull := filepath.Join(sourceDir, sourcePath)
		isDir := strings.HasSuffix(f.Path, "/")

		localHash, localErr := comparableArtifactHash(localFull, isDir)
		sourceHash, sourceErr := comparableArtifactHash(sourceFull, isDir)
		if localErr != nil || sourceErr != nil {
			continue
		}
		if localHash == sourceHash && state.Files[i].Status != "" {
			state.Files[i].Status = ""
			changed = true

			// Update the stored raw hash so we have the correct baseline
			rawHash, _ := rawArtifactHash(localFull, isDir)
			if rawHash != "" {
				state.Files[i].Hash = rawHash
			}
		}
	}

	if changed {
		return writeScopedState(scope, state)
	}
	return nil
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
