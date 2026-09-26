package cli

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/navikt/copilot/cli/nav-pilot/internal/agentpakke"
)

// syncResult holds the outcome of a sync check for machine-readable output.
type syncResult struct {
	UpToDate bool         `json:"up_to_date"`
	Source   string       `json:"source"`
	Updates  []syncUpdate `json:"updates,omitempty"`
	// Added names artifacts the source ships that this scope does not have
	// yet, which `--apply` installs for a scope that tracks the whole pakke
	// (#878). Paths, like every other list here.
	Added     []string `json:"added,omitempty"`
	Deletions []string `json:"deletions,omitempty"`
	Errors    []string `json:"errors,omitempty"`
	Overrides []string `json:"overrides,omitempty"`
	Ignored   []string `json:"ignored,omitempty"`
	Foreign   []string `json:"foreign,omitempty"`
	Conflicts []string `json:"conflicts,omitempty"`
	// Kept names files the source deleted that sync left on disk because they
	// differ from what nav-pilot installed (#729). They are not deletions: a
	// workflow reading this document must not report them as removed.
	Kept []string `json:"kept,omitempty"`
	// Retired names artifacts the source has withdrawn that are still
	// installed, and whose bytes nav-pilot published (#716).
	Retired []string     `json:"retired,omitempty"`
	PinBump *syncPinBump `json:"pin_bump,omitempty"`
	// Version is a pinned agentpakke's release version, when it is known (#779).
	Version string `json:"version,omitempty"`
	// UpdateChoice is the scope's durable update choice, written where it is
	// what decided the outcome (#781): a sync that held a release back says so
	// rather than leaving a caller to read "up to date" and wonder.
	UpdateChoice string `json:"update_choice,omitempty"`
	// Warning is a problem sync stepped around without changing anything.
	Warning string `json:"warning,omitempty"`
	// Skipped says sync did not check for an update at all; Warning says why.
	Skipped bool `json:"skipped,omitempty"`
}

// syncPinBump is the committed pin moving, reported as its own unit of work.
//
// It is not a file update: no installed file's content has to differ for the
// agentpakke to have moved forward. Before #606 that made it invisible to
// anything reading sync's exit code — the check run exited 0, --apply never
// ran, and the pin the whole file exists to keep current stayed where it was.
type syncPinBump struct {
	Path string `json:"path"`
	From string `json:"from"`
	To   string `json:"to"`
}

type syncUpdate struct {
	Path        string `json:"path"`
	SourcePath  string `json:"-"` // resolved source path, not serialized
	SourceRoot  string `json:"-"` // the checkout SourcePath is relative to
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
	err = syncScope(scope, ref, sourceRepo, adopted, apply, jsonOutput)
	// errUpdatesAvailable is a successful check, not a failure: the source was
	// fetched and read. Recording only on nil left every pre-tracking scope
	// with a pending update sourceless forever, and so unable to adopt the
	// pakke identity — "do exactly what the tool says, nothing happens" (#877).
	if adopted != "" && (err == nil || errors.Is(err, errUpdatesAvailable)) {
		recordAdoptedSource(scope, adopted)
	}
	return err
}

// refuseSourceSwitch stops a sync that names a different source than the one
// the scope is recorded against (#691).
//
// sync moves the pin within one source; changing source is an install. That is
// what [syncPakkePin] already says for a pakke scope, and this is the same rule
// for a content scope, which never reached that guard: it lives past the
// installsContent gate.
//
// Two things went wrong without it, and the second is the damaging one:
//
//  1. Only SourceSHA and Version are written back (see the state block at the
//     end of this file). SourceRepo is written by install alone. So
//     `sync --source X --apply` left the scope recorded against the old source
//     with the new source's revision, and the next plain sync read the old
//     source again and offered to roll every file back.
//  2. The file diff is computed against the new source, but it is not an
//     install: a file the new source does not ship is reported as "deleted in
//     source" and --apply removes it, while a file only the new source ships is
//     never added. The result is neither source, which is worse than either.
//
// An empty recorded source is the adoption path (B3) and passes: that scope has
// no source to switch away from.
func refuseSourceSwitch(scope *InstallScope, sourceRepo string) error {
	if sourceRepo == "" {
		return nil
	}
	state, err := readScopedState(scope)
	if err != nil || state == nil || state.SourceRepo == "" {
		return nil
	}
	if sameSourceRepo(state.SourceRepo, sourceRepo) {
		return nil
	}
	return fmt.Errorf(
		"the %s scope is installed from %s, and %s names %s.\n"+
			"sync updates the source a scope already has; switching sources is an install.\n\n"+
			"  Sync the recorded source:  %s\n"+
			"  Switch this scope over:    %s",
		scope.Name, bold(state.SourceRepo), bold("--source"), bold(sourceRepo),
		bold("nav-pilot sync"),
		bold("nav-pilot install --"+scope.Name+" --source "+sourceRepo))
}

// syncScope is the sync itself, once the source question is settled.
//
// adopted is the source a pre-tracking scope is adopting in this same run. It
// is not recorded in state yet — that happens after the sync — so the identity
// adoption has to be told about it, or it waits for a second sync that nothing
// asked the user to run (#877).
func syncScope(scope *InstallScope, ref, sourceRepo, adopted string, apply, jsonOutput bool) error {
	if err := refuseSourceSwitch(scope, sourceRepo); err != nil {
		return err
	}
	// The source a scope was installed from wins over the persisted default:
	// selection is per scope (B4), so syncing one scope never drags another
	// scope's agentpakke into it.
	if sourceRepo == "" {
		if state, err := readScopedState(scope); err == nil && state != nil && state.SourceRepo != "" {
			// A recorded source that is no longer there resolves to nothing,
			// and nav-pilot will not pick another one on the scope's behalf:
			// a source switch is an install (#691). Say so, rather than
			// failing further down with a git error about a path.
			if err := refuseGoneSource(scope, state); err != nil {
				return err
			}
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
		// The erroring form, not declaredSourceRepo: a scope with no recorded
		// source is exactly where a broken declaration does damage. Swallowing
		// the error would sync from the default agentpakke instead, and
		// adoptSyncSource would then write that into the scope's state — so a
		// repo that committed a pin ends up recorded against a pakke nobody
		// chose. The guards may swallow, because a guard only advises; the
		// command that resolves content may not.
		declared, _, declErr := declaredPin(scope, "", ref)
		if declErr != nil {
			return declErr
		}
		sourceRepo = declared
	}
	src, err := resolveSourceForSync(ref, sourceRepo)
	if err != nil {
		return err
	}
	defer src.Cleanup()

	// A Tier 1 agentpakke that publishes stable releases syncs from the newest
	// release, not from the default branch this resolved (#794). Everything
	// below — the retired scan, the pin bump, the file diff — is about one
	// revision, so the swap happens before any of them reads src.
	//
	// The declaration's own SHA is deliberately not consulted here: sync exists
	// to find out what moved, and bumping the declaration onto the release is
	// how a release reaches a repo at all.
	relSrc, release, err := tier1Release(scope, src, ref != "")
	if err != nil {
		return err
	}
	if relSrc != src {
		defer relSrc.Cleanup()
		src = relSrc
	}

	if !jsonOutput {
		noteDeclarationDisagreement(scope, src)
	}

	// A revision that changed what the pakke proposes voids the previous
	// answer, so this is where a sync asks again (#858). Before the tier split
	// below, so a Tier 2 pin is covered too. A sync that changes nothing about
	// the block asks nothing: the record is keyed on its content hash.
	noteProposalConsent(scope, src, !apply, jsonOutput)

	// Scanned here, before any of the early returns below, and consulted by all
	// of them. A scope whose only problem is a retired leftover has nothing in
	// updates or deletions and may have no tracked files at all, so a scan
	// placed later is a scan the two "nothing to do" paths jump over: sync
	// printed "All N files up to date" over three orphans and --apply removed
	// none of them. That is the state the machine which found #716 was in.
	retired := findRetiredOrphans(scope, src.Dir, src.Pakke)

	// What the committed pin would become. Computed before the file diff
	// because it is a change in its own right: a revision the repo tracks can
	// move without any installed file changing (#606).
	pinBump := pendingPinBump(scope, src)

	// One resolver for the whole sync, built from the agentpakke manifest that
	// governs this source (the legacy adapter when it ships none).
	syncState, _ := readScopedState(scope)

	// A pinned Tier 2 install has no files to diff — its update unit is the
	// revision — so it leaves the file sync before the resolver is built.
	if pinnedSync(syncState, src) {
		return syncPakkePin(scope, src, syncState, ref, apply, jsonOutput)
	}

	resolver := resolverForState(src, syncState)

	// The same reuse install resolved. Without it, every artifact inherited
	// from a reused pakke stops resolving the moment the install is over: sync
	// sees a tracked file its source no longer ships, which is the shape of a
	// retired artifact, and offers to delete what install just put there.
	resolver, bases, err := composeResolver(resolver, src)
	if err != nil {
		return err
	}
	defer bases.cleanup()
	reusedInSync := bases.nearest()
	// A reused pakke retires artifacts too, and its record is its own file.
	// Reading only the top source's meant an artifact the base withdrew stayed
	// installed forever in every consumer of a pakke that reuses it, which is
	// the whole thing the retired record exists to prevent (#716).
	if reusedInSync != nil {
		retired = mergeRetired(retired, findRetiredOrphans(scope, reusedInSync.Dir, reusedInSync.Pakke))
	}
	if reusedInSync != nil && !jsonOutput {
		fmt.Printf("%s %s\n", dim("Reuses:"), dim(fmt.Sprintf("%s@%s", sourceLabelFor(reusedInSync), shortSHA(reusedInSync.SHA))))
	}

	// A collection-era scope meets its source's manifest here first: rewrite
	// it onto the pakke identity before the diff, so this sync already runs —
	// and reports new items — as the pakke install it now is.
	if err := adoptPakkeIdentity(scope, src, syncState, resolver, adopted, apply, jsonOutput); err != nil {
		return err
	}

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
			printConflictSummary(scope, conflictPaths, src.SHA)
			return errUpdatesAvailable
		}
		if len(retired) > 0 {
			if jsonOutput {
				if err := outputJSON(syncResult{Source: src.SHA, Retired: retiredPaths(retired)}); err != nil {
					return err
				}
				if !apply {
					return errUpdatesAvailable
				}
			} else {
				reportRetired(retired, apply)
			}
			if apply {
				removeRetiredOrphans(scope, retired, jsonOutput)
				return nil
			}
			return errUpdatesAvailable
		}
		// A scope can declare a pin without having any files to sync: it may
		// have ignored everything, or committed the declaration before the
		// first install. The pin still moves, and reporting "nothing to do"
		// with exit 0 let it rot in exactly the repos a scheduled workflow was
		// supposed to keep fresh, because that workflow reads the exit code.
		if pinBump != nil {
			// The write comes first, so the document can report what actually
			// happened rather than what was about to.
			if apply {
				bumpDeclarationSHA(scope, src, jsonOutput)
			}
			if jsonOutput {
				if err := outputJSON(syncResult{UpToDate: apply, Source: src.SHA, PinBump: pinBump}); err != nil {
					return err
				}
			} else if !apply {
				fmt.Printf("%s %s pins %s and the source is at %s (source: %s)\n\n",
					yellow("⚠"), bold(agentpakke.DeclarationPath),
					shortSHA(pinBump.From), shortSHA(pinBump.To), shortSHA(src.SHA))
			}
			if !apply {
				return errUpdatesAvailable
			}
			return nil
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
	var keptPaths []string
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

		// Check if it exists in the source, or in a pakke this one reuses. A
		// file inherited from a reused pakke is not under src.Dir, and reading
		// that absence as "deleted upstream" deleted everything the base
		// supplied on the first sync after install.
		sourceRoot, found := resolver.SourceRootFor(sf.sourcePath)
		if !found && isUserHookConfig(scope, resolver, sf.localPath) {
			// Generated, not copied: --apply rebuilds it from the current
			// .hook.json, so a new matcher or timeout reaches it too.
			if apply {
				if err := refreshUserHookConfig(scope, resolver, sf.localPath); err != nil {
					syncErrors = append(syncErrors, fmt.Sprintf("%s: %v", sf.localPath, err))
				}
			}
			continue
		}
		if !found {
			// Deleted upstream — but only nav-pilot's own untouched copy is
			// nav-pilot's to remove. A file whose bytes have changed since it
			// was installed is the user's work, and a silent delete is the one
			// outcome it can never be recovered from. Same predicate
			// removeOrphans has always used (#729); it guarded that path alone.
			if sf.tracked != nil && !safeToRemove(scope.RootDir, *sf.tracked) {
				keptPaths = append(keptPaths, sf.localPath)
				continue
			}
			deletedPaths = append(deletedPaths, sf.localPath)
			continue
		}

		u, err := checkSyncFile(scope.RootDir, sourceRoot, sf)
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

	// Artifacts the source ships that this scope has not got. A scope that
	// tracks the whole pakke is meant to hold them, so they are pending work
	// like any update rather than a note pointing at another command (#878).
	// syncState is the state the adoption just rewrote in memory; on a run
	// without --apply that rewrite is not on disk yet.
	added := detectNewItems(scope, syncState, resolver, src)
	var addedPaths []string
	for _, a := range added {
		addedPaths = append(addedPaths, a.path)
	}

	// Counts in the summary describe what this source was asked about. A file
	// from another agentpakke was skipped above without being compared, so
	// counting it as "up to date" claims a check that never happened.
	checked := len(files) - len(foreignPaths)

	result := syncResult{
		UpToDate:  len(updates) == 0 && len(added) == 0 && len(deletedPaths) == 0 && len(syncErrors) == 0 && pinBump == nil && len(retired) == 0 && (apply || len(conflictPaths) == 0),
		Source:    src.SHA,
		Updates:   updates,
		Added:     addedPaths,
		Deletions: deletedPaths,
		Errors:    syncErrors,
		Overrides: overriddenPaths,
		Ignored:   ignoredPaths,
		Foreign:   foreignPaths,
		Conflicts: conflictPaths,
		Kept:      keptPaths,
		PinBump:   pinBump,
		Retired:   retiredPaths(retired),
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

	// Files the source deleted that sync will not. Printed ahead of the
	// up-to-date branch, and not counted as work: there is nothing for --apply
	// to do about a kept file, so making it a reason to return
	// errUpdatesAvailable would have the scheduled workflow open an empty PR
	// every week for as long as the file exists. The user still has to be told,
	// every run, because nothing about it changes until they act.
	if len(keptPaths) > 0 {
		fmt.Printf("%s %d file(s) deleted in source "+conflictWording+" and were kept (source: %s)\n\n",
			yellow("⚠"), len(keptPaths), shortSHA(src.SHA))
		for _, p := range keptPaths {
			fmt.Printf("  %s %s\n", dim("⊘"), p)
		}
		fmt.Printf("Delete them yourself if you no longer want them, or list them under %s in %s to stop sync mentioning them.\n\n",
			bold("overrides"), bold(syncConfigPath))
	}

	if result.UpToDate {
		fmt.Printf("%s All %d files up to date (source: %s)\n",
			green("✓"), checked, shortSHA(src.SHA))
		// Nothing here is a bump the check step could have seen — pendingPinBump
		// said so — but a declaration that names a source and pins nothing still
		// gets its pin filled in, and only --apply may write it.
		if apply {
			bumpDeclarationSHA(scope, src, jsonOutput)
		}
		// Bump state version so staleness check won't re-trigger for this
		// release, and record the stable release this sync read (#794): a new
		// release whose content this scope already holds changes no file, and
		// without this the claim would go stale the moment the SHA moved.
		if state, err := readScopedState(scope); err == nil && state != nil {
			before := *state
			recordRelease(state, &before, src, release, ref != "")
			changed := state.PakkeVersion != before.PakkeVersion ||
				state.FollowsReleases != before.FollowsReleases ||
				state.PakkeVersionSHA != before.PakkeVersionSHA
			if src.Version != "" && (state.Version != src.Version || state.SourceSHA != src.SHA) {
				state.Version, state.SourceSHA, changed = src.Version, src.SHA, true
			}
			if changed {
				if err := writeScopedState(scope, state); err != nil {
					fmt.Fprintf(os.Stderr, "%s Could not update state: %v\n", yellow("⚠"), err)
				}
			}
		}
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

	if len(added) > 0 {
		fmt.Printf("%s %d artifact(s) the source ships are not installed here (source: %s)\n\n",
			yellow("⚠"), len(added), shortSHA(src.SHA))
		for _, a := range added {
			fmt.Printf("  %s %s\n", green("+"), a.path)
		}
		fmt.Println()
	}

	// Artifacts the source has retired. Separate from deletedPaths, which covers
	// files the state file tracks: these are untracked leftovers the ordinary
	// delete path cannot see, and the only reason they can be removed at all is
	// that their bytes match a revision the source published (#716).
	if len(retired) > 0 {
		reportRetired(retired, apply)
	}

	// Reported, not acted on: an ignored artifact is one nav-pilot was told to
	// leave alone, and sync must keep leaving it alone. Saying so is the whole
	// fix (#724), since silence is what let one sit on a withdrawn model.
	if stale := ignoredButInstalled(scope); len(stale) > 0 {
		reportIgnoredButInstalled(stale)
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

	// A revision bump with no file change: the only thing to report, and
	// without it a check-only run would print nothing but "Run nav-pilot sync
	// --apply" over a repo whose files are all up to date.
	if pinBump != nil && len(updates) == 0 && len(deletedPaths) == 0 {
		fmt.Printf("%s %d files up to date, but %s pins %s and the source is at %s (source: %s)\n\n",
			yellow("⚠"), checked, bold(agentpakke.DeclarationPath),
			shortSHA(pinBump.From), shortSHA(pinBump.To), shortSHA(src.SHA))
	}

	if len(conflictPaths) > 0 && !apply {
		printConflictSummary(scope, conflictPaths, src.SHA)
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
		// The root the comparison used, not the top source: an inherited file
		// lives in the reused pakke, and copying it from src.Dir would read a
		// path that is not there.
		root := u.SourceRoot
		if root == "" {
			root = src.Dir
		}
		if err := applySyncUpdate(scope, root, u); err != nil {
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
		afterArtifactRemoved(scope, localFull, jsonOutput)
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
	if err := updateScopedStateHashes(scope, appliedUpdates, src.SHA); err != nil {
		fmt.Fprintf(os.Stderr, "%s Could not update state file: %v\n", yellow("⚠"), err)
	}

	if len(deletedSuccessPaths) > 0 {
		if err := removeFilesFromState(scope, deletedSuccessPaths); err != nil {
			fmt.Fprintf(os.Stderr, "%s Could not remove files from state: %v\n", yellow("⚠"), err)
		}
		scope.CleanupDirs()
	}

	// Only bump source SHA and version if ALL updates/deletions were applied
	// successfully. The declaration is bumped outside the state block: a repo
	// can carry a committed pin without a state file, and that pin is exactly
	// the one --apply exists to move.
	if len(retired) > 0 {
		fmt.Printf("%s Removing %d retired artifact(s)\n", dim("→"), len(retired))
		removeRetiredOrphans(scope, retired, jsonOutput)
	}
	if len(added) > 0 {
		fmt.Printf("%s Installing %d artifact(s) this scope did not have\n", dim("→"), len(added))
		if err := installPending(scope, resolver, added); err != nil {
			fmt.Fprintf(os.Stderr, "%s Could not install them: %v\n", yellow("⚠"), err)
			applyErrors++
		}
	}
	if applyErrors == 0 {
		bumpDeclarationSHA(scope, src, jsonOutput)
	}
	if state, err := readScopedState(scope); err == nil && state != nil {
		if applyErrors == 0 {
			// Before SourceSHA moves: the claim being carried forward is a
			// claim about the revision the state still records (#794).
			recordRelease(state, state, src, release, ref != "")
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
// isUserHookConfig reports whether localPath is the ~/.copilot/hooks/<name>.json
// entry activateHook generated for a hook the source still ships. It has no
// file of its own in the source (it is made from hooks/<name>.py and its
// .hook.json), so reading it as "deleted in source" removed the entry of every
// installed hook on the next sync and left the scripts behind.
func isUserHookConfig(scope *InstallScope, resolver *SourceResolver, localPath string) bool {
	dir, file := filepath.Split(filepath.ToSlash(localPath))
	if !scope.IsUser() || dir != KindHook.Dir+"/" || !strings.HasSuffix(file, ".json") {
		return false
	}
	name := strings.TrimSuffix(file, ".json")
	_, _, ok := resolver.GetFile(KindHook.Dir, name+KindHook.Suffix)
	return ok
}

// refreshUserHookConfig rewrites a ~/.copilot/hooks/<name>.json entry from
// the source's hook and records its new hash, so uninstall still knows the
// file as nav-pilot's own.
func refreshUserHookConfig(scope *InstallScope, resolver *SourceResolver, localPath string) error {
	name := strings.TrimSuffix(filepath.Base(localPath), ".json")
	art, ok := resolver.Get(KindHook, name)
	if !ok {
		return nil
	}
	var res installResult
	if err := activateHook(scope, art, &res); err != nil {
		return err
	}
	state, err := readScopedState(scope)
	if err != nil || state == nil || len(res.Files) == 0 {
		return err
	}
	for i := range state.Files {
		if state.Files[i].Path == res.Files[0].Path && state.Files[i].Hash != res.Files[0].Hash {
			state.Files[i].Hash = res.Files[0].Hash
			return writeScopedState(scope, state)
		}
	}
	return nil
}

// resolveSyncFiles to the "No customization files found to sync." dead end and
// returns nil — sync reporting success over an install that can never advance.
//
// --apply goes through [pinRevision], not [installPakkePin]: the work is
// identical — validate, materialize, re-record the pin, prune — but the output
// belongs to sync, which reports a revision rather than announcing an install.
// That re-materialization is deliberate: an update re-verifies the payloads
// rather than only moving the recorded SHA.
//
// Without an explicit ref it follows stable releases first (#779): a source that
// publishes release metadata moves only to the newest stable release's exact
// SHA, and a pin that follows releases never falls back to the default branch.
// An explicit --ref is a pinning choice and wins, and the pin it writes stops
// following.
func syncPakkePin(scope *InstallScope, src *Source, state *StateFile, ref string, apply, jsonOutput bool) error {
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

	var release *pakkeRelease
	var warning string // a lookup problem this sync stepped around
	version, follows := releaseClaim(state)
	if ref != "" {
		version = "" // an explicit --ref makes no release claim
	}
	if ref == "" {
		name := state.Collection
		if src.Pakke != nil {
			name = src.Pakke.Name
		}
		outcome, rel, err := discoverPakkeRelease(context.Background(), src.Repo, name, state.SourceSHA)
		if errors.Is(err, errReleasesNotFound) && !follows {
			// A private repo without GITHUB_TOKEN answers 404 here, while git
			// clones it with the user's own credentials. That is how such a pin
			// synced before releases existed, so it still does. A following pin
			// fails closed below.
			//
			// It says so. A private repo that does publish releases would
			// otherwise sync to its default branch without a hint, and land
			// ahead of its releases, where the downgrade guard keeps it once a
			// token is set.
			outcome, err = releaseNoMetadata, nil
			warning = releasesNotVisible(src.Repo)
			if !jsonOutput {
				fmt.Printf("%s %s\n", yellow("⚠"), warning)
			}
		}
		// A pin whose revision directory is gone is restored at its own SHA
		// when the lookup gives nothing to move to. "Up to date" or "not
		// offered" over a missing revision is the frozen success the
		// wiped-revision branch below exists to close.
		restore := false
		switch {
		case err != nil && follows:
			return fmt.Errorf("looking up stable releases of %s: %w\n\nThe pin is unchanged at %s", src.Repo, err, shortSHA(state.SourceSHA))
		case err != nil:
			// Not following, so whether this source is release-backed is the
			// very thing the lookup could not answer. Nothing is pinned on a
			// guess: no release is confirmed, and the default branch would land
			// ahead of every release, where the downgrade guard keeps it. It
			// exits the way a sync with nothing to change does.
			warning = fmt.Sprintf("could not look up stable releases of %s, so no update was checked; the pin stays at %s: %v",
				src.Repo, shortSHA(state.SourceSHA), err)
			if pinnedRevisionOnDisk(state) {
				if jsonOutput {
					return outputJSON(syncResult{UpToDate: true, Skipped: true, Source: state.SourceSHA, Version: version, Warning: warning})
				}
				fmt.Printf("%s %s\n", yellow("⚠"), warning)
				return nil
			}
			if !jsonOutput {
				fmt.Printf("%s %s\n", yellow("⚠"), warning)
			}
			restore = true
		case outcome == releaseNoMetadata && follows:
			return fmt.Errorf(
				"%s follows stable releases, and %s has no stable release with %s for it.\n"+
					"The pin is unchanged at %s; nav-pilot does not fall back to the default branch.\n\n"+
					"  Pin a revision deliberately:  %s",
				bold(state.Collection), bold(src.Repo), pakkeReleaseAsset, shortSHA(state.SourceSHA),
				bold("nav-pilot sync --user --apply --ref <branch|sha>"))
		case outcome == releaseNoMetadata:
			// Not release-backed: the default branch, as before.
		case outcome == releaseNotOffered && !pinnedRevisionOnDisk(state):
			restore = true
		case outcome == releaseNotOffered:
			if jsonOutput {
				return outputJSON(syncResult{UpToDate: true, Source: state.SourceSHA, Version: version})
			}
			fmt.Printf("%s %s is pinned at %s, which is newer than or diverged from the latest stable release %s (%s). It is not offered.\n",
				green("✓"), bold(state.Collection), shortSHA(state.SourceSHA), rel.Version, shortSHA(rel.SHA))
			return nil
		case outcome == releaseUpToDate && pinnedRevisionOnDisk(state):
			if jsonOutput {
				return outputJSON(syncResult{UpToDate: true, Source: state.SourceSHA, Version: rel.Version})
			}
			fmt.Printf("%s %s is up to date (%s, pinned at %s).\n", green("✓"), bold(state.Collection), rel.Version, shortSHA(state.SourceSHA))
			return nil
		case outcome == releaseUpToDate && !follows:
			// Its revision is gone (on disk returned above). Restoring the pin
			// is not a choice to follow the release it happens to be.
			restore = true
		default: // a candidate, or a following pin up to date with its revision gone
			relSrc, err := fetchPakkeRelease(src.Repo, name, rel)
			if err != nil {
				return err
			}
			defer relSrc.Cleanup()
			src, release = relSrc, &rel
		}
		if restore {
			pinned, err := resolveSourceForSync(state.SourceSHA, src.Repo)
			if err != nil {
				return fmt.Errorf("fetching the pinned revision %s: %w", shortSHA(state.SourceSHA), err)
			}
			defer pinned.Cleanup()
			if !sameSHA(pinned.SHA, state.SourceSHA) {
				return fmt.Errorf("the pinned revision %s resolved to %s; the pin is unchanged", state.SourceSHA, pinned.SHA)
			}
			src = pinned
		}
	}

	// Whether this scope's pin may move onto this revision on nav-pilot's own
	// initiative is one question, and [pakkeUpdateHold] is its one answer: the
	// revision a rollback rejected (#783), and every revision newer than the pin
	// when the durable update choice is "keep" (#781). Two records, one
	// predicate, so a scope carrying both cannot be told two different things by
	// two checks. Neither names the source, so `sync` goes on reporting what is
	// out there; an explicit --ref is the way onto it, and moves the pin without
	// changing what the scope chose about the next release.
	//
	// ponytail: checked after the release has been resolved, so a held scope
	// still pays one fetch per sync. One check site instead of two in the switch
	// above; move it up if that cost shows up.
	if hold := pakkeUpdateHold(state, src.SHA); ref == "" && hold != holdNone {
		// The pin's own revision is gone, and the only revision the source
		// offers is one this scope will not take. Reporting "up to date" here
		// would be the frozen success the wiped-revision branch below exists to
		// close, reached through a hold instead of a missing release. There is
		// nothing sync can rebuild without being told what.
		if !pinnedRevisionOnDisk(state) {
			return fmt.Errorf(
				"%s is pinned at %s, that revision is no longer under %s, and the only revision %s offers is %s — %s.\n"+
					"Nothing was changed.\n\n"+
					"  Rebuild a revision deliberately:  %s",
				bold(state.Collection), shortSHA(state.SourceSHA), bold(pakkerRoot()), bold(src.Repo), shortSHA(src.SHA), hold.why(state),
				bold("nav-pilot sync --user --apply --ref <branch|sha>"))
		}
		// What is being held back is not always a release. releaseNoMetadata
		// arrives with a nil error and leaves release nil while src is the
		// resolved default branch, which is exactly the source a non-following
		// pin syncs from — so a "keep" on such a pin lands here too, holding a
		// branch revision. [pakkeRelease.label] is nil-safe and prints the short
		// SHA for that case, and the line says "revision" rather than calling a
		// branch commit a release.
		held, heldKind := release.label(src.SHA), "revision"
		if release != nil {
			heldKind = "release"
		}
		if hold == holdKeep {
			// The choice stops the move, not the news: a release nobody hears
			// about is how a security fix sits unshipped on a machine whose
			// owner would have taken it. `warning` is a lookup problem sync
			// stepped around, and that is the more urgent of the two.
			warning = cmp.Or(warning, fmt.Sprintf("%s %s %s is available; this scope's update choice keeps revision %s",
				state.Collection, heldKind, held, shortSHA(state.SourceSHA)))
		}
		if jsonOutput {
			return outputJSON(syncResult{UpToDate: true, Source: state.SourceSHA, Version: version,
				UpdateChoice: string(pakkeUpdateChoice(state)), Warning: warning})
		}
		if hold == holdKeep {
			fmt.Printf("%s %s is pinned at %s. The newest %s is %s, and this scope keeps the revision.\n",
				green("✓"), bold(state.Collection), shortSHA(state.SourceSHA), heldKind, held)
			fmt.Printf("%s %s\n", dim("Take this one:"), bold("nav-pilot sync --user --apply --ref "+src.SHA))
			fmt.Printf("%s %s\n", dim("Be asked again:"), bold("nav-pilot sync --user --updates ask"))
			return nil
		}
		fmt.Printf("%s %s is pinned at %s, rolled back from %s, which is not offered again.\n",
			green("✓"), bold(state.Collection), shortSHA(state.SourceSHA), shortSHA(src.SHA))
		fmt.Printf("%s %s\n", dim("Go back to it deliberately:"), bold("nav-pilot sync --user --apply --ref "+state.RolledBackFrom))
		return nil
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
	// The command that applies what a plain sync reports, --ref included.
	next := "nav-pilot sync --apply"
	if ref != "" {
		next += " --ref " + ref
	}
	if !pinnedRevisionOnDisk(state) {
		if !apply {
			if jsonOutput {
				if err := outputJSON(syncResult{UpToDate: false, Source: src.SHA, Version: cmp.Or(release.version(), version), Warning: warning}); err != nil {
					return err
				}
				return errUpdatesAvailable
			}
			fmt.Printf("%s %s is pinned at %s, but that revision is no longer under %s.\n\n",
				yellow("⚠"), bold(state.Collection), shortSHA(state.SourceSHA), bold(pakkerRoot()))
			if ref != "" && follows {
				fmt.Printf("It follows stable releases. With --apply, this --ref pins %s and stops following.\n\n", shortSHA(src.SHA))
			}
			fmt.Printf("Run %s to materialize it again (from %s, what the source resolves to now).\n",
				bold(next), shortSHA(src.SHA))
			return errUpdatesAvailable
		}
		if _, err := pinRevision(scope, src, release, ref != "", jsonOutput); err != nil {
			return err
		}
		if jsonOutput {
			return outputJSON(syncResult{UpToDate: true, Source: src.SHA, Version: cmp.Or(release.version(), version), Warning: warning})
		}
		fmt.Printf("%s Restored %s at revision %s.\n", green("✓"), bold(src.Pakke.Name), shortSHA(src.SHA))
		return nil
	}

	// [sameSHA], not ==: a state file written before #597 holds seven characters and
	// this one holds forty. Comparing those byte for byte reported a newer
	// revision that does not exist, in a sentence that printed the same seven
	// characters on both sides, and --apply then re-fetched and re-materialized
	// the revision already pinned into a second directory (#605). The recorded
	// SHA is only ever a directory name here — nothing fetches it — so leaving
	// it short is harmless, and the next real update writes it out in full.
	//
	// An explicit --ref to the revision a following pin is already at still has
	// something to write: the pin stops following.
	if sameSHA(src.SHA, state.SourceSHA) && (ref == "" || !follows) {
		if jsonOutput {
			return outputJSON(syncResult{UpToDate: true, Source: src.SHA, Version: release.version(), Warning: warning})
		}
		fmt.Printf("%s %s is up to date (pinned at %s).\n", green("✓"), bold(src.Pakke.Name), shortSHA(src.SHA))
		return nil
	}

	if !apply {
		if jsonOutput {
			if err := outputJSON(syncResult{UpToDate: false, Source: src.SHA, Version: release.version(), Warning: warning}); err != nil {
				return err
			}
			return errUpdatesAvailable
		}
		if sameSHA(src.SHA, state.SourceSHA) {
			fmt.Printf("%s %s is pinned at %s and follows stable releases. With --apply, this --ref keeps the revision and stops following.\n\n",
				yellow("⚠"), bold(src.Pakke.Name), shortSHA(src.SHA))
		} else {
			fmt.Printf("%s A newer revision of %s is available (pinned %s, source %s).\n\n",
				yellow("⚠"), bold(src.Pakke.Name), shortSHA(state.SourceSHA), release.label(src.SHA))
		}
		fmt.Printf("Run %s to update.\n", bold(next))
		return errUpdatesAvailable
	}

	if _, err := pinRevision(scope, src, release, ref != "", jsonOutput); err != nil {
		return err
	}
	if jsonOutput {
		return outputJSON(syncResult{UpToDate: true, Source: src.SHA, Version: release.version(), Warning: warning})
	}
	fmt.Printf("%s Updated %s to revision %s.\n", green("✓"), bold(src.Pakke.Name), release.label(src.SHA))
	return nil
}

// cmdSyncAuto syncs all detected scopes (repo + user) when the user didn't
// explicitly pick one with --user or --target. Mirrors how the interactive
// flow and `list --installed` handle scope discovery.
//
// repoDir may be any directory in the repository: the repo scope is its git
// root, so `nav-pilot --sync` from a subfolder finds the install.
func cmdSyncAuto(repoDir, ref, sourceRepo string, apply, jsonOutput bool) error {
	if root := findGitRoot(repoDir); root != "" {
		repoDir = root
	}
	repoScope := ScopeRepo(repoDir)
	repoState, repoErr := readScopedState(repoScope)

	userScope, userErr := ScopeUser()
	var userState *StateFile
	var userStateErr error
	if userErr == nil {
		userState, userStateErr = readScopedState(userScope)
	}

	// Recorded here as well as from the interactive startup. Emitted only
	// there, nav_pilot_install_present answered "who starts nav-pilot
	// interactively": a user who only runs sync, and every CI job, reported
	// nothing at all.
	recordInstallState(repoScope.Name, repoState, repoErr)
	if userErr == nil {
		recordInstallState(userScope.Name, userState, userStateErr)
	}

	if repoState == nil && userState == nil {
		if jsonOutput {
			return outputJSON(map[string]interface{}{"installed": false})
		}
		fmt.Println("nav-pilot is not installed (repo or user scope).")
		fmt.Printf("Install with: %s\n", bold(installCommandFor(nil, nil)))
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
	//
	// The effective source, not the bare --source flag: the flag is empty unless
	// someone typed it, so a provider was left recovering the source from its own
	// state file and, failing that, from the built-in default (#813). The scoped
	// syncs above keep the flag, because refuseSourceSwitch reads it as "the user
	// asked to switch" and a configured source is no such request.
	providerSource, srcErr := sourceRepoFor(sourceRepo)
	if srcErr != nil && !jsonOutput {
		fmt.Fprintf(os.Stderr, "%s Could not read the configured source: %v\n", yellow("⚠"), srcErr)
	}
	hasPrevOutput := repoState != nil || userState != nil
	for _, p := range allProviders() {
		res := p.SyncContext(ref, providerSource, jsonOutput, hasPrevOutput)
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
	// tracked is the state entry this file came from, or nil when the scope has
	// no state and the file was auto-detected. The delete path needs the
	// recorded hash to tell nav-pilot's own copy from one the user has edited.
	tracked *InstalledFile
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
			entry := f
			files = append(files, syncFile{
				localPath:  f.Path,
				sourcePath: sp,
				isDir:      strings.HasSuffix(f.Path, "/"),
				source:     f.Source,
				tracked:    &entry,
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

// printConflictSummary reports the files a plain sync leaves alone.
//
// One function for both branches, because there were two and they drifted: the
// path where there is nothing else to sync kept the wording #623 objected to
// ("in conflict state and were skipped", then "Run sync --apply to apply
// updates") long after the main path was rewritten (#651). A message written
// twice is a message that will say two things.
//
// "differs from what nav-pilot installed" and not "your own edits" (#692). The
// status says content changed since the recorded hash; it says nothing about
// who changed it. A reader who knows they edited nothing rightly rejects the
// sentence, and the only remedy offered is --apply, which then takes the
// source's version whether that is newer or older than what is on disk.
func printConflictSummary(scope *InstallScope, conflictPaths []string, srcSHA string) {
	fmt.Printf("%s %d file(s) "+conflictWording+" and are left alone by a plain sync (source: %s)\n\n",
		yellow("⚠"), len(conflictPaths), shortSHA(srcSHA))
	// Per-file provenance turns a bare path into a fact the reader can act on
	// (#729): a file installed from an older revision than the one the scope is
	// on is behind, not edited, and those are different problems with the same
	// hash symptom. A file with no recorded revision predates provenance and
	// says nothing extra rather than guessing.
	revisions := installedRevisions(scope)
	for _, p := range conflictPaths {
		line := fmt.Sprintf("  %s %s", dim("⊘"), p)
		if rev := revisions[p]; rev != "" && !sameRevision(rev, srcSHA) {
			line += dim(fmt.Sprintf("  (installed from %s)", shortSHA(rev)))
		}
		fmt.Println(line)
	}
	fmt.Printf("%s to take the source's version of these too.\n\n", bold("nav-pilot sync --apply"))
}

// conflictWording is the one phrase nav-pilot has for "the bytes on disk are
// not the bytes nav-pilot wrote". Written once because it is written in two
// reports — the conflict summary and the files sync declines to delete — and
// two copies of a sentence are two sentences waiting to drift (#651). #692
// settled what it may claim: content differs, and nothing about who changed it.
const conflictWording = "differ from what nav-pilot installed"

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

// pendingArtifact is an artifact the source ships that a scope does not have
// yet, held as the kind and name an install needs rather than the line a
// report prints. Before #878 the list was only ever printed.
type pendingArtifact struct {
	kind *ArtifactKind
	name string
	path string
}

func (p pendingArtifact) String() string { return p.kind.Name + ": " + p.name }

// detectNewItems checks if the source has agents/skills/instructions not in the
// state file. Only relevant for installs that are meant to hold everything
// their source ships — see [scopeTracksEverything].
//
// The state is passed in rather than read: a sync that has not been given
// --apply holds the adopted pakke identity in memory only (#878), and reading
// the file back would ask the question against the identity the scope is
// leaving.
func detectNewItems(scope *InstallScope, state *StateFile, resolver *SourceResolver, src *Source) []pendingArtifact {
	if state == nil || !scopeTracksEverything(scope, state, src) {
		return nil
	}

	installed := make(map[string]bool)
	for _, f := range state.Files {
		installed[f.Path] = true
	}

	var newItems []pendingArtifact
	// Every kind except prompts. A hook or an extension that shipped after the
	// last install is executable code the user has not got yet, which is the
	// thing #569 exists to surface; a prompt is offered, never enforced.
	for _, kind := range AllKinds {
		if kind == KindPrompt || !scope.SupportsType(kind.Name) {
			continue
		}
		for _, art := range resolver.List(kind) {
			relPath := kind.RelPathForName(scope, art.Name)
			if !installed[relPath] {
				newItems = append(newItems, pendingArtifact{kind: kind, name: art.Name, path: relPath})
			}
		}
	}
	return newItems
}

// installPending installs the artifacts a scope that tracks the whole pakke
// does not have yet. It is the second half of the adoption (#878): the
// collection's subset stopped being an ignore list, so the rest of the pakke
// has to arrive on its own rather than through a command the user is told to
// run next.
//
// Nothing is overwritten. An artifact already on disk under an untracked path
// goes through the same conflict check every install does, so a file someone
// edited is kept and reported, not replaced.
func installPending(scope *InstallScope, resolver *SourceResolver, pending []pendingArtifact) error {
	result := &installResult{}
	hashes := scopeStateHashes(scope)
	for _, p := range pending {
		if err := installArtifact(resolver, scope, hashes, p.kind, p.name, false, false, result); err != nil {
			return err
		}
	}
	if len(result.Files) == 0 {
		return nil
	}
	telemetry.RecordInstallItems(scope.Name, telemetryMode(), int64(result.Installed))
	state, err := readScopedState(scope)
	if err != nil {
		return fmt.Errorf("reading state to record %d added artifact(s): %w", len(result.Files), err)
	}
	if state == nil {
		return fmt.Errorf("no state file to record %d added artifact(s) in", len(result.Files))
	}
	state.Files = mergeStateFiles(state.Files, result.Files)
	return writeScopedState(scope, state)
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
	return &syncUpdate{Path: sf.localPath, SourcePath: sf.sourcePath, SourceRoot: sourceDir, CurrentHash: localHash, SourceHash: sourceHash}, nil
}

// applySyncUpdate copies a single file/dir from source to target.
func applySyncUpdate(scope *InstallScope, sourceDir string, u syncUpdate) error {
	sourceFull := filepath.Join(sourceDir, u.SourcePath)
	targetFull := filepath.Join(scope.RootDir, u.Path)
	return copyArtifact(sourceFull, targetFull, scope.RootDir, strings.HasSuffix(u.Path, "/"))
}

// updateScopedStateHashes updates the state file with new hashes after applying updates.
func updateScopedStateHashes(scope *InstallScope, updates []syncUpdate, revision string) error {
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
		// The revision this content came from, which is what a later sync needs
		// to tell an edit from an older install (#729).
		state.Files[i].Revision = revision
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
func updateStateHashes(targetDir string, updates []syncUpdate, revision string) error {
	return updateScopedStateHashes(ScopeRepo(targetDir), updates, revision)
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

// installCommandFor names the command that would pull new source items into
// this scope: an agentpakke installs by name into a repo, everything else is
// the user-scope install-all.
//
// A nil scope is the question asked before anything is installed, which is
// where the four first-run commands ask it (#876). They have no resolved
// source either, and reaching for one would put a network fetch behind a
// hint — so they get the install-all, which needs no name and no git repo.
// The one thing it must never be is a placeholder: `nav-pilot install
// <collection>` was the first instruction a new user got, and it fails.
func installCommandFor(scope *InstallScope, src *Source) string {
	if src != nil && src.Pakke != nil && !scope.IsUser() {
		return "nav-pilot install " + src.Pakke.Name
	}
	return "nav-pilot install --user"
}
