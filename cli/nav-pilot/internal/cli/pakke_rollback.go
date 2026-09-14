package cli

import (
	"fmt"
)

// Local rollback of a pinned Tier 2 agentpakke (#783).
//
// A release that breaks has to be escapable without waiting for the publisher
// and without a network. The revision that was pinned before is usually still
// on disk — retention keeps the pin and the one it replaced — so rolling back
// is a local operation: verify that older tree where it lies, and move the pin
// onto it.
//
// Three things decide whether that is useful, and all three are choices:
//
//  1. "Verified" means verified now, not verified once. The tree has been
//     sitting under ~/.nav-pilot/pakker since it was staged, and drift — a
//     restore, an antivirus quarantine, a half-synced home directory — is the
//     expected failure for a tree that old. So [verifyRevision] runs again, the
//     same walk the launch and an adopting install run, and a tree that does not
//     hold up is refused rather than pinned. Nothing is re-downloaded to repair
//     it: repairing needs the source, and the whole point is that the source may
//     be out of reach.
//
//  2. The pin moves and the release claim is trimmed to what disk can prove.
//     The subscription survives — a pin that followed stable releases goes on
//     following, so the fix ships as the next release and arrives as an ordinary
//     update — but the version does not: the revision directory carries the
//     agentpakke manifest, never the release metadata, so the older revision's
//     own version is not knowable offline and is recorded as unknown rather than
//     guessed.
//
//  3. A rolled-back scope stays rolled back. Moving the pin alone would not do
//     that: the very next `sync --apply` would find the rejected revision newer
//     and put it straight back, which is the whole feature undone. So the
//     rollback records the revision it left ([StateFile.RolledBackFrom]), and
//     sync and the startup prompt do not offer that one again. They offer every
//     other one, which is what makes the subscription worth keeping.
//
// It deletes nothing. The revision being left is what a `--ref` goes back to,
// and it may be the tree a live session is reading — pruning it here would be
// the one thing #834 closed, reached from a new command. It is removed by the
// ordinary retention rule, on the next pin that replaces it.

// cmdRollback moves this user's pin to the previous revision on disk.
func cmdRollback(jsonOutput bool) error {
	scope, err := ScopeUser()
	if err != nil {
		return fmt.Errorf("locating your user scope: %w", err)
	}
	state, err := readScopedState(scope)
	if err != nil {
		return fmt.Errorf("reading state: %w", err)
	}
	// pinnedState, not pinnedRevisionOnDisk: a pin whose own revision directory
	// is gone is exactly a scope that wants an older one back.
	if !pinnedState(state) || !pinnable(state.SourceRepo) {
		return fmt.Errorf(
			"rollback moves a pinned agentpakke back to the previous revision on this machine, and your user scope pins none.\n\n"+
				"  Pin one:  %s",
			bold("nav-pilot install --user <name>"))
	}

	// previousRevision names the newest revision that is not the current one,
	// which after one rollback is the revision that rollback left. Rolling
	// "back" onto it would be rolling forward onto the revision just rejected,
	// so it is refused: with retention at two there is nothing older to reach.
	previous := previousRevision(state.SourceRepo, state.SourceSHA)
	if previous == "" || sameSHA(previous, state.RolledBackFrom) {
		return fmt.Errorf(
			"%s is pinned at %s, and no older revision of %s is on this machine.\n"+
				"nav-pilot keeps the pinned revision and the one it replaced, so a rollback is possible until the pin has moved twice.\n\n"+
				"  Pin a revision deliberately (needs the network):  %s",
			bold(state.Collection), shortSHA(state.SourceSHA), bold(sourceLabelForRepo(state.SourceRepo)),
			bold("nav-pilot sync --user --apply --ref <branch|sha>"))
	}

	// Claimed before it is verified, for [holdRevision]'s own reason: everything
	// between naming a revision and acting on it is time another process can
	// prune in. Released when the command ends — a rollback is not a session.
	holdRevision(state.SourceRepo, previous)
	defer releaseRevision()

	revDir := pakkeRevisionDir(state.SourceRepo, previous)
	src := &Source{Dir: revDir, SHA: previous, Repo: state.SourceRepo, Version: state.Version}
	if err := attachPakke(src); err != nil {
		return err
	}
	if src.Pakke == nil {
		return fmt.Errorf("%s carries no agentpakke manifest, so it is not a revision nav-pilot can pin", revDir)
	}
	if err := verifyRevision(src, revDir); err != nil {
		return fmt.Errorf("the previous revision %s does not verify, so nothing was rolled back: %w\n\n"+
			"  Pin a revision deliberately (needs the network):  %s",
			shortSHA(previous), err, bold("nav-pilot sync --user --apply --ref <branch|sha>"))
	}

	// Lost update, the guard [pinRevision] has: verifying a whole revision takes
	// long enough for another install, sync or launch to move this scope's pin.
	current, err := readScopedState(scope)
	if err != nil {
		return fmt.Errorf("reading state: %w", err)
	}
	if pinMoved(state, current) {
		return fmt.Errorf("the %s scope's pin changed to %s while the rollback was being prepared; nothing was recorded. Run the command again",
			scope.Name, pinLabel(current))
	}

	_, follows := releaseClaim(state)
	rolled := &StateFile{
		Collection:    src.Pakke.Name,
		Version:       state.Version,
		Scope:         scope.Name,
		SourceRepo:    state.SourceRepo,
		SourceSHA:     previous,
		PinnedClients: payloadClients(src.Pakke),
		InstalledAt:   timeNow().UTC().Format("2006-01-02T15:04:05Z07:00"),
		// PakkeVersion stays empty: the release version of this revision is not
		// on disk. The subscription is, and it is what brings the fix.
		FollowsReleases: follows,
		RolledBackFrom:  state.SourceSHA,
		// The durable update choice is about the package and survives the
		// rollback: going back a revision is not an answer to what should
		// happen when the next release ships (#781).
		UpdateChoice: state.UpdateChoice,
	}
	if follows {
		rolled.PakkeVersionSHA = previous
	}
	rolled.PreserveUnknownFrom(state)
	if err := writeScopedState(scope, rolled); err != nil {
		return fmt.Errorf("writing state: %w", err)
	}

	if jsonOutput {
		return outputJSON(withPakkeName(map[string]interface{}{
			"command":          "rollback",
			"scope":            scope.Name,
			"source_sha":       previous,
			"rolled_back_from": state.SourceSHA,
			"follows_releases": follows,
		}, rolled.Collection))
	}
	fmt.Printf("%s Rolled %s back to revision %s.\n", green("✓"), bold(rolled.Collection), shortSHA(previous))
	fmt.Println()
	fmt.Printf("%s\n", dim(fmt.Sprintf("%s is left on disk and is not offered again; a newer revision is.", shortSHA(state.SourceSHA))))
	fmt.Printf("%s %s\n", dim("Go back to it deliberately:"), bold("nav-pilot sync --user --apply --ref "+state.SourceSHA))
	return nil
}
