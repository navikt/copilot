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
	Deletions []string     `json:"deletions,omitempty"`
	Errors    []string     `json:"errors,omitempty"`
	Overrides []string     `json:"overrides,omitempty"`
	Ignored   []string     `json:"ignored,omitempty"`
	Foreign   []string     `json:"foreign,omitempty"`
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
//
// A scope whose state predates source tracking adopts the source it syncs from
// (B3): the sync says so, runs, and records the source only once it succeeded,
// so a failed sync leaves the scope exactly as sourceless as it found it.
func cmdSync(scope *InstallScope, ref, sourceRepo string, apply, jsonOutput bool) error {
	adopted, err := adoptSyncSource(scope, sourceRepo)
	if err != nil {
		return err
	}
	if adopted != "" && !jsonOutput {
		noteAdoptedSource(scope, adopted)
	}
	if err := syncScope(scope, ref, sourceRepo, apply, jsonOutput); err != nil {
		return err
	}
	if adopted != "" {
		recordAdoptedSource(scope, adopted)
	}
	return nil
}

// syncScope is the sync itself, once the source question is settled.
func syncScope(scope *InstallScope, ref, sourceRepo string, apply, jsonOutput bool) error {
	// The source a scope was installed from wins over the persisted default:
	// selection is per scope (B4), so syncing one scope never drags another
	// scope's agentpakke into it.
	if sourceRepo == "" {
		if state, err := readScopedState(scope); err == nil && state != nil && state.SourceRepo != "" {
			sourceRepo = state.SourceRepo
			if !jsonOutput {
				noteRecordedSourceWins(state.SourceRepo)
			}
		}
	}
	// A scope with no recorded source falls back to the repo's declaration
	// before the config key — the same rung the install side reads it on. The
	// declaration's pinned SHA is deliberately *not* used as the ref: sync's
	// job is to find out what moved, and resolving the revision the repo is
	// already pinned to would make `sync` always report "up to date".
	if sourceRepo == "" {
		sourceRepo = declaredSourceRepo(scope)
	}
	src, err := resolveSourceForSync(ref, sourceRepo)
	if err != nil {
		return err
	}
	defer src.Cleanup()
	if !jsonOutput {
		noteDeclarationDisagreement(scope, src)
	}

	// One resolver for the whole sync, built from the agentpakke manifest that
	// governs this source (the legacy adapter when it ships none).
	syncState, _ := readScopedState(scope)

	// A pinned Tier 2 install has no files to diff — its update unit is the
	// revision — so it leaves the file sync before the resolver is built.
	if pinnedSync(syncState, src) {
		return syncPakkePin(scope, src, syncState, apply, jsonOutput)
	}

	resolver := resolverForState(src, syncState)

	// Determine which files to check
	files, _, err := resolveSyncFiles(scope, resolver, apply)
	if err != nil {
		return err
	}

	conflictPaths := conflictStatePaths(scope)
	if err := clearResolvedConflicts(scope, resolver, conflictPaths); err != nil {
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
				yellow("⚠"), len(conflictPaths), shortSHA(src.SHA))
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
	var deletedPaths []string
	var syncErrors []string
	var ignoredPaths []string
	var foreignPaths []string
	for _, sf := range files {
		// Check if local file exists; if missing, treat as intentional deletion
		localFull := filepath.Join(scope.RootDir, sf.localPath)
		if _, statErr := os.Stat(localFull); os.IsNotExist(statErr) {
			ignoredPaths = append(ignoredPaths, sf.localPath)
			continue
		}

		// A file from another agentpakke is not this source's to judge. It is
		// absent here because it was never here, and reading that as "deleted
		// upstream" is what removed files that `add --source` had just
		// installed (#571). Such a file is updated by adding it again from its
		// own source, which never has to be reachable from this run — being
		// offline must not delete anything.
		if sf.source != "" && !sameSourceRepo(sf.source, sourceLabelFor(src)) {
			foreignPaths = append(foreignPaths, sf.localPath)
			continue
		}

		// Check if it exists in the source
		sourceFull := filepath.Join(src.Dir, sf.sourcePath)
		if _, statErr := os.Stat(sourceFull); os.IsNotExist(statErr) {
			deletedPaths = append(deletedPaths, sf.localPath)
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

	if !jsonOutput && len(foreignPaths) > 0 {
		for _, p := range foreignPaths {
			fmt.Printf("  %s %s (from another agentpakke — not synced from here)\n", dim("⊘"), p)
		}
		fmt.Println()
	}

	// Counts in the summary describe what this source was asked about. A file
	// from another agentpakke was skipped above without being compared, so
	// counting it as "up to date" claims a check that never happened.
	checked := len(files) - len(foreignPaths)

	result := syncResult{
		UpToDate:  len(updates) == 0 && len(deletedPaths) == 0 && len(syncErrors) == 0 && (apply || len(conflictPaths) == 0),
		Source:    src.SHA,
		Updates:   updates,
		Deletions: deletedPaths,
		Errors:    syncErrors,
		Overrides: overriddenPaths,
		Ignored:   ignoredPaths,
		Foreign:   foreignPaths,
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
		// Exit 2 if any errors occurred (even with updates/deletions)
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
			green("✓"), checked, shortSHA(src.SHA))
		// A sync that changed no file can still have landed on a new revision
		// upstream. --apply is what makes that a bump: a check-only run must
		// never dirty a file the developer would have to commit.
		if apply {
			bumpDeclarationSHA(scope, src)
		}
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
		reportNewItems(scope, resolver, src)
		return nil
	}

	// Report updates
	if len(updates) > 0 {
		fmt.Printf("%s %d of %d files have updates available (source: %s)\n\n",
			yellow("⚠"), len(updates), checked, shortSHA(src.SHA))
		for _, u := range updates {
			fmt.Printf("  %s %s\n", yellow("~"), u.Path)
		}
		fmt.Println()
	}

	// Report deletions
	if len(deletedPaths) > 0 {
		fmt.Printf("%s %d file(s) deleted in source and will be removed (source: %s)\n\n",
			yellow("⚠"), len(deletedPaths), shortSHA(src.SHA))
		for _, p := range deletedPaths {
			fmt.Printf("  %s %s\n", red("-"), p)
		}
		fmt.Println()
	}

	if len(conflictPaths) > 0 && !apply {
		fmt.Printf("%s %d file(s) are in conflict state and were skipped (source: %s)\n\n",
			yellow("⚠"), len(conflictPaths), shortSHA(src.SHA))
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

	// Apply deletions
	deleted := 0
	var deletedSuccessPaths []string
	for _, p := range deletedPaths {
		localFull := filepath.Join(scope.RootDir, p)
		var rmErr error
		if strings.HasSuffix(p, "/") {
			rmErr = os.RemoveAll(localFull)
		} else {
			rmErr = os.Remove(localFull)
		}
		if rmErr != nil && !os.IsNotExist(rmErr) {
			fmt.Fprintf(os.Stderr, "%s Could not remove %s: %v\n", yellow("⚠"), p, rmErr)
			applyErrors++
			continue
		}
		fmt.Printf("  %s %s (deleted)\n", red("×"), p)
		deleted++
		deletedSuccessPaths = append(deletedSuccessPaths, p)
	}

	if len(updates) > 0 {
		fmt.Printf("\n%s Updated %d file(s).\n", green("✓"), applied)
	}
	if len(deletedPaths) > 0 {
		fmt.Printf("%s Removed %d file(s).\n", green("✓"), deleted)
	}

	// Update state with new hashes
	if err := updateScopedStateHashes(scope, appliedUpdates); err != nil {
		fmt.Fprintf(os.Stderr, "%s Could not update state file: %v\n", yellow("⚠"), err)
	}

	if len(deletedSuccessPaths) > 0 {
		if err := removeFilesFromState(scope, deletedSuccessPaths); err != nil {
			fmt.Fprintf(os.Stderr, "%s Could not remove files from state: %v\n", yellow("⚠"), err)
		}
		scope.CleanupDirs()
	}

	// Only bump source SHA and version if ALL updates/deletions were applied successfully
	if state, err := readScopedState(scope); err == nil && state != nil {
		if applyErrors == 0 {
			bumpDeclarationSHA(scope, src)
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

	reportNewItems(scope, resolver, src)
	return nil
}

// pinnedState reports whether a scope's state has the shape [pinRevision]
// writes: a source, a SHA, and no tracked files at all.
//
// The shape alone does not make it a pin — a state predating source tracking
// wears the same one, which is why callers pair it with something that only a
// pin can be true of.
//
// "No tracked files" means no *installed* files, not an empty list. `nav-pilot
// ignore <item> --user` appends a zero-hash marker to whatever state is there,
// pin included, and counting that as content splits the three places that ask
// whether something is a pin: the launch would go on reading it (pinnedRevision
// never looks at Files), while sync fell into a file diff with nothing to diff
// — "No customization files found to sync.", exit 0, the pin frozen for good —
// and uninstall removed the state file without the revisions behind it. One
// question, [installsContent], asked in all three.
func pinnedState(state *StateFile) bool {
	return state != nil && state.SourceRepo != "" && state.SourceSHA != "" && !installsContent(state)
}

// pinnedRevisionOnDisk reports whether this state's pin was actually
// materialized: a revision directory exists for its source and SHA.
//
// This is the unambiguous signal, and the one the launch path already keys on.
// A revision directory exists only because something pinned it, so unlike the
// state's field pattern (a pre-tracking install shares it) or the source's
// current tier (upstream can change it under a pin that is still what every
// launch reads), it cannot be true of anything else.
func pinnedRevisionOnDisk(state *StateFile) bool {
	if !pinnedState(state) {
		return false
	}
	_, err := os.Stat(pakkeRevisionDir(state.SourceRepo, state.SourceSHA))
	return err == nil
}

// pinnedSync reports whether a sync is over a pinned Tier 2 install.
//
// The revision on disk is the unambiguous signal, and neither the pakke's
// display name nor the source's current shape can stand in for it: a pakke that
// renames itself upstream is the same install, and one that grows a layout
// upstream is still pinned to the payload-only revision every launch reads.
//
// But the disk check alone is not the whole answer either. A pin whose revision
// directory is gone — ~/.nav-pilot/pakker wiped, a revision hand-deleted — is
// still the pin recorded in state and still what the next launch acts on, and
// dropping it into a file sync that tracks no files prints "No customization
// files found to sync." and returns success. That is the same frozen-success
// this branch exists to close, reached through a missing directory instead of a
// missing branch. So a pin-shaped state is taken too when the source it names
// still ships payloads only: the state's shape alone is ambiguous (a Tier 1
// install can track no files), and the source's shape alone is ambiguous (see
// above) — together they are not.
func pinnedSync(state *StateFile, src *Source) bool {
	return pinnedRevisionOnDisk(state) || (pinnedState(state) && payloadOnly(src))
}

// syncPakkePin updates a pinned Tier 2 install: it compares the pinned SHA to
// the one the source resolved to and, with --apply, pins the new revision.
//
// Without this branch a zero-item pin state falls all the way through
// resolveSyncFiles to the "No customization files found to sync." dead end and
// returns nil — sync reporting success over an install that can never advance.
//
// --apply goes through [pinRevision], not [installPakkePin]: the work is
// identical — validate, materialize, re-record the pin, prune — but the output
// belongs to sync, which reports a revision rather than announcing an install.
// That re-materialization is deliberate: an update re-verifies the payloads
// rather than only moving the recorded SHA.
func syncPakkePin(scope *InstallScope, src *Source, state *StateFile, apply, jsonOutput bool) error {
	// Sync updates the source a scope is pinned to; it does not switch to
	// another one. An explicit --source bypasses the B3 guard (it is the
	// consent gesture for an *install*), so without this a sync would compare
	// this scope's pinned SHA against an unrelated repo's HEAD as though they
	// were two revisions of one thing, and --apply would perform the switch.
	//
	// This runs before anything has looked at what --source ships, so it is the
	// one refusal here that a manifest-less source reaches: hence
	// [pakkeInstallTarget] rather than src.Pakke.Name. Every refusal below it
	// is past the payloadOnly gate, which is false for a nil manifest.
	if !sameSourceRepo(state.SourceRepo, src.Repo) {
		return fmt.Errorf(
			"the %s scope is pinned to %s at %s, and %s is a different agentpakke.\n"+
				"sync updates the pinned source; switching sources is an install.\n\n"+
				"  Update the pinned agentpakke:  %s\n"+
				"  Switch this scope over:        %s",
			scope.Name, bold(state.SourceRepo), shortSHA(state.SourceSHA), bold(src.Repo),
			bold("nav-pilot sync --apply"),
			bold("nav-pilot install --user --source "+src.Repo+" "+pakkeInstallTarget(src)))
	}

	// The pinned source stopped shipping payloads only. There is no revision
	// bump to make: this release pins neither mixed pakker nor Tier 1 content,
	// so --apply has nothing valid to materialize, and every launch would go on
	// reading the payload-only revision already pinned. Reporting "up to date"
	// or offering an update that can never apply is how an install stays frozen
	// while every command says it is fine.
	if !payloadOnly(src) {
		return fmt.Errorf(
			"%s is pinned at %s, a revision that ships pre-built payloads only, and %s no longer does.\n"+
				"nav-pilot does not update a pin across that change, and launches keep reading the pinned revision.\n\n"+
				"  Reinstall it:  %s",
			bold(state.Collection), shortSHA(state.SourceSHA), bold(state.SourceRepo),
			bold("nav-pilot install --user "+state.Collection))
	}

	// The recorded pin has no revision behind it any more. It is still the pin
	// — the next launch re-materializes it — but nothing here can be reported
	// as up to date, and the SHA comparison below would do exactly that
	// whenever the source has not moved. --apply rebuilds it; a plain sync says
	// what is wrong, which is the half the user needs.
	if !pinnedRevisionOnDisk(state) {
		if !apply {
			if jsonOutput {
				if err := outputJSON(syncResult{UpToDate: false, Source: src.SHA}); err != nil {
					return err
				}
				return errUpdatesAvailable
			}
			fmt.Printf("%s %s is pinned at %s, but that revision is no longer under %s.\n\n",
				yellow("⚠"), bold(state.Collection), shortSHA(state.SourceSHA), bold(pakkerRoot()))
			fmt.Printf("Run %s to materialize it again (from %s, what the source resolves to now).\n",
				bold("nav-pilot sync --apply"), shortSHA(src.SHA))
			return errUpdatesAvailable
		}
		if _, err := pinRevision(scope, src, jsonOutput); err != nil {
			return err
		}
		if jsonOutput {
			return outputJSON(syncResult{UpToDate: true, Source: src.SHA})
		}
		fmt.Printf("%s Restored %s at revision %s.\n", green("✓"), bold(src.Pakke.Name), shortSHA(src.SHA))
		return nil
	}

	if src.SHA == state.SourceSHA {
		if jsonOutput {
			return outputJSON(syncResult{UpToDate: true, Source: src.SHA})
		}
		fmt.Printf("%s %s is up to date (pinned at %s).\n", green("✓"), bold(src.Pakke.Name), shortSHA(src.SHA))
		return nil
	}

	if !apply {
		if jsonOutput {
			if err := outputJSON(syncResult{UpToDate: false, Source: src.SHA}); err != nil {
				return err
			}
			return errUpdatesAvailable
		}
		fmt.Printf("%s A newer revision of %s is available (pinned %s, source %s).\n\n",
			yellow("⚠"), bold(src.Pakke.Name), shortSHA(state.SourceSHA), shortSHA(src.SHA))
		fmt.Printf("Run %s to update.\n", bold("nav-pilot sync --apply"))
		return errUpdatesAvailable
	}

	if _, err := pinRevision(scope, src, jsonOutput); err != nil {
		return err
	}
	if jsonOutput {
		return outputJSON(syncResult{UpToDate: true, Source: src.SHA})
	}
	fmt.Printf("%s Updated %s to revision %s.\n", green("✓"), bold(src.Pakke.Name), shortSHA(src.SHA))
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
	source     string // agentpakke this file came from; empty means the scope's own
}

// resolveSyncFiles determines which files to sync.
// If a state file exists, uses the installed file list.
// Otherwise, auto-detects customization files in the target repo.
func resolveSyncFiles(scope *InstallScope, resolver *SourceResolver, includeConflicts bool) ([]syncFile, string, error) {
	state, err := readScopedState(scope)
	if err != nil {
		return nil, "", fmt.Errorf("reading state: %w", err)
	}

	if state != nil {
		// State-based: check all installed files, skip ignored and conflicted ones
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
				source:     f.Source,
			})
		}
		return files, state.Collection, nil
	}

	if scope.IsUser() {
		// No auto-detect for user scope without state
		return nil, "", nil
	}

	// Auto-detect: scan for customization files that also exist in source
	return autoDetectSyncFiles(scope.RootDir, resolver)
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

// detectNewItems checks if the source has agents/skills/instructions not in the
// state file. Only relevant for installs that are meant to hold everything
// their source ships — see [scopeTracksEverything].
func detectNewItems(scope *InstallScope, resolver *SourceResolver, src *Source) []string {
	state, err := readScopedState(scope)
	if err != nil || state == nil || !scopeTracksEverything(scope, state, src) {
		return nil
	}

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
func autoDetectSyncFiles(targetDir string, resolver *SourceResolver) ([]syncFile, string, error) {
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
func clearResolvedConflicts(scope *InstallScope, resolver *SourceResolver, conflictPaths []string) error {
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

	changed := false
	for i, f := range state.Files {
		if !conflictSet[f.Path] {
			continue
		}

		localFull := filepath.Join(scope.RootDir, f.Path)
		sourcePath := resolver.MapLocalPath(f.Path, scope.IsUser())
		sourceFull := filepath.Join(resolver.SourceDir(), sourcePath)
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

// removeFilesFromState removes the specified paths from the state file.
func removeFilesFromState(scope *InstallScope, paths []string) error {
	state, err := readScopedState(scope)
	if err != nil || state == nil {
		return nil
	}

	pathSet := make(map[string]bool, len(paths))
	for _, p := range paths {
		pathSet[p] = true
	}

	var keptFiles []InstalledFile
	for _, f := range state.Files {
		if !pathSet[f.Path] {
			keptFiles = append(keptFiles, f)
		}
	}
	state.Files = keptFiles

	if len(state.Files) == 0 {
		return os.Remove(scope.StatePath())
	}

	return writeScopedState(scope, state)
}

func outputJSON(v interface{}) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// scopeTracksEverything reports whether a scope's install is meant to hold all
// of its source's content, which is what makes "the source grew an item" worth
// reporting rather than noise about content the user never asked for.
//
// Legacy: the "(all)" user-scope install, the only collection that means
// everything. Agentpakke: any pakke install — a pakke is installed whole, in
// either scope, so anything its layout grows belongs to this scope too. Items
// the user deselected in the picker are recorded as ignored and stay excluded
// in both cases.
func scopeTracksEverything(scope *InstallScope, state *StateFile, src *Source) bool {
	if src != nil && src.Pakke != nil {
		return state.Collection == src.Pakke.Name
	}
	return state.Collection == CollectionAll && scope.IsUser()
}

// reportNewItems prints a notice if the source has new items not yet installed.
func reportNewItems(scope *InstallScope, resolver *SourceResolver, src *Source) {
	newItems := detectNewItems(scope, resolver, src)
	if len(newItems) == 0 {
		return
	}
	fmt.Println()
	fmt.Printf("%s %d new item(s) in source not yet installed:\n", dim("ℹ"), len(newItems))
	for _, item := range newItems {
		fmt.Printf("    %s\n", item)
	}
	fmt.Printf("  Run %s to add them.\n", bold(installCommandFor(scope, src)))
}

// installCommandFor names the command that would pull new source items into
// this scope: an agentpakke installs by name into a repo, everything else is
// the user-scope install-all.
func installCommandFor(scope *InstallScope, src *Source) string {
	if src != nil && src.Pakke != nil && !scope.IsUser() {
		return "nav-pilot install " + src.Pakke.Name
	}
	return "nav-pilot install --user"
}
