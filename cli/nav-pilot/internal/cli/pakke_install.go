package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/navikt/copilot/cli/nav-pilot/internal/agentpakke"
)

// Tier 2 revision pinning (WP4′).
//
// A payload-only agentpakke ships no Tier 1 content, so "installing" it is not
// a matter of copying agents and skills into a scope. What it ships is payload
// trees, and what an install of one produces is a *pinned revision*: every
// declared context of every payload-bearing client, verified and materialized
// once, under a directory named after the source SHA.
//
//	~/.nav-pilot/pakker/<owner>-<repo>/<sha>/
//	├── .nav-pilot/agentpakke.json   the pinned agentpakke manifest
//	├── copilot/full/                a verified payload tree per client × context
//	└── copilot/focused/
//
// Content-addressed naming is what makes the tree safe to keep: a new revision
// is a new directory, so nothing is ever rewritten under a session already
// reading one. It is also why per-launch staging is gone — the launch verifies
// the pinned tree and points the client at it, without cloning or copying
// anything.
//
// The state entry that records the pin carries no files at all: SourceRepo and
// SourceSHA are the whole install. Everything downstream reads that fine —
// `nav-pilot list --installed` prints "Files: 0", and uninstall removes nothing
// before removing the state file.

// pakkerRoot is where pinned revisions live: ~/.nav-pilot/pakker, a sibling of
// config.toml. It follows configPath(), so NAV_PILOT_CONFIG relocates it too.
func pakkerRoot() string {
	return filepath.Join(filepath.Dir(configPath()), "pakker")
}

// pakkeSourceDir holds every kept revision of one source.
//
// The repo id is flattened with "/" → "-", which means a/b-c and a-b/c share a
// directory. Accepted: a collision additionally requires the same SHA, and the
// state entry records the repo the pin was made from, so a launch configured
// for the other source does not read it.
//
// It is also lower-cased, because [sameSourceRepo] folds case for repo ids —
// they are case-insensitive on GitHub — and every guard in the pin path is
// built on that. Naming the directory byte-exact instead makes the two
// disagree: re-casing the configured source id leaves the identity checks
// saying "same install" while the install materializes under a second
// directory, and the first one is then named by nothing — not by the launch or
// uninstall (which key off the state's now re-cased repo), not by the prune
// (which reads the new directory). A local path is left alone: sameSourceRepo
// compares those as the checkout they resolve to, not by folding.
func pakkeSourceDir(repo string) string {
	name := strings.ReplaceAll(repo, "/", "-")
	if !filepath.IsAbs(repo) {
		name = strings.ToLower(name)
	}
	return filepath.Join(pakkerRoot(), name)
}

// pakkeRevisionDir names one immutable revision of one source.
func pakkeRevisionDir(repo, sha string) string {
	return filepath.Join(pakkeSourceDir(repo), sha)
}

// revisionTmpPrefix names an unpublished revision. It is a prefix rather than a
// suffix so the staging tree sorts away from the SHA directories beside it, and
// it is checked in two places: [os.MkdirTemp] writes it, and the prune leaves
// anything wearing it alone.
const revisionTmpPrefix = ".tmp-"

// pinnable reports whether a source can be pinned at all. Only repo-shaped
// sources can.
//
// An absolute path is the agentpakke author's own working tree: it resolves
// with a stat and no clone, and for a non-git directory its "SHA" is the
// literal "unknown". Pinning that would freeze the tree at whatever it held the
// first time it was launched — every later launch would read the stale
// revision, and sync would compare "unknown" against "unknown" and report it up
// to date forever. So a local source is re-materialized every launch instead,
// which is the loop it had before revisions were pinned.
//
// This mirrors tierCacheable's carve-out, for the same reason: a developer
// editing their own checkout must see the change on the next launch.
func pinnable(sourceRepo string) bool {
	return sourceRepo != "" && !filepath.IsAbs(sourceRepo)
}

// materializeRevision builds the revision for src, if a usable one is not
// already there, and returns its directory.
//
// It stages into a sibling temporary directory and publishes with a single
// [os.Rename], so a revision directory either holds every context, verified, or
// does not exist. A crash between the rename and the file data reaching disk is
// the gap that leaves one that exists but is broken, which is why an existing
// revision is verified before it is adopted and rebuilt when it does not hold
// up. Without that a single bad publish wedges every launch of that SHA
// forever: sync reports "up to date", a re-install of the same SHA hands back
// the same broken tree, and only uninstall recovers.
//
// Two processes materializing the same SHA at once is safe: both build their
// own tmp tree, one rename wins, and the loser finds the winner's revision in
// place and adopts it. Both then verify it independently before use.
func materializeRevision(src *Source) (string, error) {
	revDir := pakkeRevisionDir(src.Repo, src.SHA)
	_, statErr := os.Stat(revDir)
	// A local source is never pinned, so an existing revision under its
	// placeholder SHA says nothing about the working tree that produced it and
	// is always replaced. A repo-shaped one is kept only if it still verifies.
	replace := statErr == nil && (!pinnable(src.Repo) || verifyRevision(src, revDir) != nil)
	if statErr == nil && !replace {
		return revDir, nil
	}

	sourceDir := pakkeSourceDir(src.Repo)
	if err := os.MkdirAll(sourceDir, 0o700); err != nil {
		return "", fmt.Errorf("creating the agentpakke revision root %s: %v", sourceDir, err)
	}
	tmp, err := os.MkdirTemp(sourceDir, revisionTmpPrefix)
	if err != nil {
		return "", fmt.Errorf("creating a staging directory under %s: %v", sourceDir, err)
	}

	if err := stageRevision(src, tmp); err != nil {
		os.RemoveAll(tmp)
		return "", err
	}
	// Only a directory this call already found unusable is removed, so the
	// ordinary "not there yet" case cannot delete a revision another process
	// published while this one was staging.
	if replace {
		_ = os.RemoveAll(revDir)
	}
	if err := os.Rename(tmp, revDir); err != nil {
		os.RemoveAll(tmp)
		if verifyRevision(src, revDir) == nil {
			return revDir, nil // another process published it first
		}
		return "", fmt.Errorf("publishing agentpakke revision %s: %v", src.SHA, err)
	}
	return revDir, nil
}

// verifyRevision reports whether a revision directory holds everything the
// manifest declares, verified: the pinned agentpakke manifest, and every
// declared context of every payload-bearing client passing the exact hash walk.
//
// It is the same check the launch runs on the one context it is about to hand
// over, applied to the whole revision — which is what lets an install adopt an
// existing revision without lying about having repaired it.
func verifyRevision(src *Source, dir string) error {
	if _, err := os.Stat(filepath.Join(dir, agentpakke.ManifestDir, agentpakke.ManifestFile)); err != nil {
		return fmt.Errorf("the revision carries no pinned agentpakke manifest: %v", err)
	}
	pakke := src.Pakke
	for _, client := range pakke.ClientIDs() {
		if pakke.Tier(client) != agentpakke.TierPayload {
			continue
		}
		for _, context := range declaredContexts(pakke, client) {
			d := filepath.Join(dir, client, context)
			if err := agentpakke.VerifyPayloadExact(d, filepath.Join(d, agentpakke.PayloadManifestFile)); err != nil {
				return fmt.Errorf("the %q payload for %s does not match its manifest: %w", context, client, err)
			}
		}
	}
	return nil
}

// stageRevision fills an unpublished revision directory: the agentpakke
// manifest, then every declared context of every payload-bearing client.
//
// The manifest is pinned with the payloads, at the conventional path, so the
// revision is self-describing once the checkout is gone — attachPakke loads it
// straight off the revision directory, and the launch reads persona, model and
// the compatibility gate from the manifest the payloads were built with rather
// than from whatever the default branch says today.
//
// Every payload-bearing client is materialized, including ones this binary
// cannot launch. Filtering to the launchable ones would be a second client list
// to keep in step with the launch switch, which already errors for a client it
// cannot stage.
func stageRevision(src *Source, dest string) error {
	pakke := src.Pakke
	manifestData, err := os.ReadFile(filepath.Join(src.Dir, agentpakke.ManifestDir, agentpakke.ManifestFile))
	if err != nil {
		return fmt.Errorf("reading the agentpakke manifest to pin it: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dest, agentpakke.ManifestDir), 0o700); err != nil {
		return fmt.Errorf("creating %s in the revision: %v", agentpakke.ManifestDir, err)
	}
	if err := os.WriteFile(filepath.Join(dest, agentpakke.ManifestDir, agentpakke.ManifestFile), manifestData, 0o644); err != nil {
		return fmt.Errorf("pinning the agentpakke manifest: %v", err)
	}

	// ponytail: contexts are materialized independently, so shared bytes are
	// stored once per context; hardlink by digest if the size ever matters.
	for _, client := range pakke.ClientIDs() {
		if pakke.Tier(client) != agentpakke.TierPayload {
			continue
		}
		for _, context := range declaredContexts(pakke, client) {
			payload, ok := pakke.Payload(client, context)
			if !ok {
				continue // unreachable: declaredContexts lists the payload keys
			}
			if _, err := agentpakke.StagePayload(
				filepath.Join(src.Dir, filepath.FromSlash(payload.Path)),
				filepath.Join(src.Dir, filepath.FromSlash(payload.ManifestPath())),
				filepath.Join(dest, client, context),
			); err != nil {
				return fmt.Errorf("staging the %q payload of agentpakke %q for %s: %w",
					context, pakke.Name, client, err)
			}
		}
	}
	return nil
}

// prunePakkeRevisions removes every revision of a source except the named ones,
// which is how at most two survive: the current pin and the one it replaced. A
// session running the old revision therefore survives one update; a second
// update pulls the tree out from under a very old session, which is accepted
// and documented rather than defended with liveness tracking.
//
// It never touches a .tmp-* directory. Those are the staging trees of
// materializations happening right now — a launch racing `sync --apply`, or two
// first launches at different SHAs — and deleting one mid-write either fails
// the other process outright or, if the timing lands between staging and the
// rename, publishes a half-deleted tree as a revision. [materializeRevision]
// removes its own staging tree on every error path, so only a hard kill leaks
// one, and a bounded leak of trees nothing will ever name is strictly cheaper
// than that. Sweeping them on an age rule instead is the TTL this design
// deliberately does not have.
//
// Best effort: pruning is housekeeping, and a revision that will not delete
// must not fail an install that has already succeeded.
func prunePakkeRevisions(repo string, keep ...string) {
	dir := pakkeSourceDir(repo)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), revisionTmpPrefix) || slices.Contains(keep, e.Name()) {
			continue
		}
		_ = os.RemoveAll(filepath.Join(dir, e.Name()))
	}
}

// releasePin removes the revisions behind a scope's pin at the one moment they
// stop being reachable: a state that is a pin is about to be replaced by one
// recording installed Tier 1 content.
//
// A pin tracks no files, so its state entry is the only record that the trees
// under ~/.nav-pilot/pakker exist at all. An ordinary Tier 1 install writes
// over it through paths that know nothing about pins, and uninstall — the one
// thing that would otherwise remove them — is gated on the state still being a
// pin. The leak is reached by following nav-pilot's own advice: every Tier 2
// refusal that cannot be resolved automatically names `nav-pilot install
// --user <name>`, and that command takes the ordinary Tier 1 path.
//
// An ignore marker is deliberately not installed content, which is the same
// answer [pinnedState] gives: `nav-pilot ignore` appends a zero-hash entry, and
// a pin carrying one is still a pin to every command that maintains it. Nothing
// has been installed over it, so there is nothing to release — and deleting the
// payload trees a working install launches from would be far worse than leaving
// them, since the launch would then refuse and auto-pin cannot rebuild over a
// state that tracks files at all (see [autoPin], which is stricter here on
// purpose).
func releasePin(scope *InstallScope, next *StateFile) {
	if scope == nil || !scope.IsUser() || !installsContent(next) {
		return
	}
	existing, err := readScopedState(scope)
	if err != nil || !pinnedState(existing) {
		return
	}
	// pinnedState guarantees a non-empty SourceRepo, so this is never
	// pakkeSourceDir(""), the root holding every source's revisions.
	_ = os.RemoveAll(pakkeSourceDir(existing.SourceRepo))
}

// installsContent reports whether a state records at least one file that was
// actually installed, as opposed to an ignore marker.
func installsContent(state *StateFile) bool {
	if state == nil {
		return false
	}
	for _, f := range state.Files {
		if f.Status != fileStatusIgnored {
			return true
		}
	}
	return false
}

// removeStateFiles removes every file a scope's state records, printing one
// line each, and returns how many it removed (or would remove, on a dry run).
//
// This is cmdUninstall's removal loop, extracted so the Tier 1 → Tier 2 source
// switch can remove the outgoing install's orphaned files through exactly the
// path uninstall uses.
//
// quiet is for a caller writing a JSON document to stdout: the per-file lines
// are suppressed, and a removal that failed goes to stderr rather than being
// lost — it is the one thing here the user cannot afford to miss, and stderr is
// not the stream being parsed.
func removeStateFiles(scope *InstallScope, state *StateFile, dryRun, quiet bool) int {
	removed := 0
	warn := func(path string, err error) {
		out := os.Stdout
		if quiet {
			out = os.Stderr
		}
		fmt.Fprintf(out, "  %s Could not remove %s: %v\n", yellow("⚠"), path, err)
	}
	for _, f := range state.Files {
		path := filepath.Join(scope.RootDir, f.Path)

		if dryRun {
			if !quiet {
				fmt.Printf("  %s %s\n", dim("×"), f.Path)
			}
			removed++
			continue
		}

		if strings.HasSuffix(f.Path, "/") {
			if err := os.RemoveAll(path); err != nil && !os.IsNotExist(err) {
				warn(f.Path, err)
				continue
			}
		} else {
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				warn(f.Path, err)
				continue
			}
		}
		if !quiet {
			fmt.Printf("  %s %s\n", red("×"), f.Path)
		}
		removed++
	}
	return removed
}

// pinRevision is the whole of a Tier 2 install, without any of its output: it
// validates the source, materializes the revision, records the pin in the
// scope's state and prunes what the pin replaced. It returns the revision
// directory.
//
// The order is load-bearing. State is written only after the rename has
// published a complete revision, so a crash mid-install leaves the previous pin
// intact; pruning runs only after the state names the new pin, so nothing is
// removed while it is still the recorded one.
//
// [installPakkePin] and the launch path's auto-pin both go through here — there
// is exactly one place payload bytes are written.
//
// It prints one thing of its own — the files an outgoing install leaves behind
// — and takes jsonOutput for it, because two of its three callers can be asked
// for a JSON document on stdout.
func pinRevision(scope *InstallScope, src *Source, jsonOutput bool) (string, error) {
	if err := checkPakkeInstallable(scope, src); err != nil {
		return "", err
	}

	existing, err := readScopedState(scope)
	if err != nil {
		return "", fmt.Errorf("reading state: %w", err)
	}

	revDir, err := materializeRevision(src)
	if err != nil {
		return "", err
	}

	previousPin := ""
	if existing != nil {
		// The state this pin replaces is a zero-item entry, so any files it
		// tracks are about to lose their only record. That is true whatever
		// source they came from: the same repo dropping its layout and going
		// payload-only orphans them exactly as a switch to another repo does,
		// and a shape change like that must be picked up without a migration.
		// An explicit install (or `sync --apply`) is the consent gesture for
		// removing them; the launch path refuses instead, see [autoPin].
		if len(existing.Files) > 0 {
			// jsonOutput is not cosmetic here: `install --json` and `sync
			// --apply --json` write one JSON document to stdout, and these
			// lines land in front of it.
			if !jsonOutput {
				fmt.Printf("\n%s Removing the files installed from %s:\n", dim("ℹ"), bold(sourceLabelForRepo(existing.SourceRepo)))
			}
			removeStateFiles(scope, existing, false, jsonOutput)
			if !jsonOutput {
				fmt.Println()
			}
		}
		if sameSourceRepo(existing.SourceRepo, src.Repo) {
			previousPin = existing.SourceSHA
		} else if outgoing := pakkeSourceDir(existing.SourceRepo); existing.SourceRepo != "" && outgoing != pakkeSourceDir(src.Repo) {
			// Switching this scope to a different source: the outgoing
			// revisions go too. A Tier 2 install tracks no files at all, so
			// whole materialized payload trees are what it leaves behind, and
			// neither prune (which only reads the incoming source's directory)
			// nor uninstall (whose state now names the incoming source) would
			// ever reach them again.
			//
			// The empty check is load-bearing: an install predating source
			// tracking records no repo, and pakkeSourceDir("") is the root
			// holding every source's revisions, the new one included. So is the
			// directory comparison: under the accepted "/" → "-" flattening
			// (a/b-c and a-b/c) two different repos share one directory, and
			// removing the outgoing one there would delete the revision this
			// call just materialized.
			_ = os.RemoveAll(outgoing)
		}
	}

	state := &StateFile{
		Collection:  src.Pakke.Name,
		Version:     src.Version,
		Scope:       scope.Name,
		SourceRepo:  src.Repo,
		SourceSHA:   src.SHA,
		InstalledAt: timeNow().UTC().Format("2006-01-02T15:04:05Z07:00"),
	}
	if err := writeScopedState(scope, state); err != nil {
		return "", fmt.Errorf("writing state: %w", err)
	}

	prunePakkeRevisions(src.Repo, src.SHA, previousPin)
	return revDir, nil
}

// checkPakkeInstallable is everything that must hold before a Tier 2 install
// writes anything: the scope can hold a pin, and the source conforms to its own
// manifest. validatePakkeSource digest-verifies every declared payload, which
// is why nothing downstream needs verification code of its own.
//
// It writes nothing, so a dry run runs it too — a dry run that answers "would
// install" for a source the real install refuses is worse than no dry run.
func checkPakkeInstallable(scope *InstallScope, src *Source) error {
	if err := guardPakkeScope(scope, src); err != nil {
		return err
	}
	if !pinnable(src.Repo) {
		return fmt.Errorf(
			"agentpakke %q is read from the local path %s, which has no immutable revision to pin.\n"+
				"Nothing was installed — a launch re-materializes a local source every time, so your edits are picked up without an install.\n\n"+
				"  Pin it by installing from the repo it is published to:  %s",
			src.Pakke.Name, sourceLabelFor(src),
			bold("nav-pilot install --user --source <owner>/<repo> "+src.Pakke.Name))
	}
	return validatePakkeSource(src)
}

// installPakkePin installs a payload-only agentpakke: it pins a revision, and
// says so. It is what every install entry point routes a Tier 2 source to.
func installPakkePin(scope *InstallScope, src *Source, dryRun, jsonOutput bool) error {
	pakke := src.Pakke

	if dryRun {
		if err := checkPakkeInstallable(scope, src); err != nil {
			return err
		}
		if jsonOutput {
			return outputJSON(map[string]interface{}{
				"command":    "install",
				"collection": pakke.Name,
				"scope":      scope.Name,
				"source_sha": src.SHA,
				"version":    src.Version,
				"installed":  0,
				"dry_run":    true,
			})
		}
		fmt.Printf("%s Would install %s, pinned at %s.\n", dim("→"), bold(pakke.Name), src.SHA)
		return nil
	}

	if _, err := pinRevision(scope, src, jsonOutput); err != nil {
		return err
	}

	if jsonOutput {
		return outputJSON(map[string]interface{}{
			"command":    "install",
			"collection": pakke.Name,
			"scope":      scope.Name,
			"source_sha": src.SHA,
			"version":    src.Version,
			"installed":  0,
			"dry_run":    false,
		})
	}
	fmt.Printf("%s Installed %s, pinned at %s.\n", green("✓"), bold(pakke.Name), src.SHA)
	fmt.Println()
	fmt.Println(dim("It ships pre-built payloads rather than files, so nothing was written to ~/.copilot."))
	fmt.Printf("%s %s %s\n", dim("Launches read the pinned revision;"), bold("nav-pilot sync"), dim("updates it."))
	return nil
}
