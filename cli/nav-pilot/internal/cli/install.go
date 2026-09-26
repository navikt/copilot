package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/x/term"

	"github.com/charmbracelet/huh"

	"github.com/navikt/copilot/cli/nav-pilot/internal/agentpakke"
	"github.com/navikt/copilot/cli/nav-pilot/internal/source"
)

type installResult struct {
	Installed int
	// Missing names every artifact the manifest listed and the source does not
	// ship. installArtifact warns and moves on, so the count is the third way
	// an install lands incomplete — beside a conflict and an unsupported kind.
	Missing   []string
	Conflicts int
	// Existing names the paths skipped because a file nav-pilot did not
	// install was already there. They are not recorded in state.
	Existing    []string
	Unsupported []string
	Files       []InstalledFile
}

// installItems installs every artifact a collection manifest names, reading
// content through the resolver the active agentpakke manifest produced.
func installItems(resolver *SourceResolver, scope *InstallScope, manifest *Manifest, dryRun, force bool) (*installResult, error) {
	result := &installResult{}
	stateHashes := scopeStateHashes(scope)

	// Derived from AllKinds rather than listed here, the lesson of #649, #650
	// and #708: a kind added to the resolver and forgotten in a table like this
	// installs nothing, silently.
	for _, kind := range AllKinds {
		names, ok := manifest.NamesByKind(kind)
		if !ok {
			continue
		}
		group := struct {
			label string
			names []string
			kind  *ArtifactKind
		}{kindLabel(kind), names, kind}
		if len(group.names) == 0 {
			continue
		}
		if !scope.SupportsType(group.kind.Name) {
			result.Unsupported = append(result.Unsupported, fmt.Sprintf("%d %s", len(group.names), group.label))
			continue
		}
		fmt.Println(bold(fmt.Sprintf("%s (%d):", group.label, len(group.names))))
		for _, name := range group.names {
			if err := installArtifact(resolver, scope, stateHashes, group.kind, name, dryRun, force, result); err != nil {
				return result, err
			}
		}
		fmt.Println()
	}

	if !dryRun {
		warnRepoHooksNeedTrust(scope)
	}

	return result, nil
}

// pakkeContents lists everything an agentpakke's layout ships, as the content
// manifest the installer already knows how to walk. It is the manifest-bearing
// counterpart to loadManifest: the agentpakke manifest supersedes
// collections/<name>/manifest.json rather than declaring entries in it.
// manifestItemCount is how many artifacts a manifest names, derived from
// AllKinds rather than summed by hand. The hand-written sums it replaces each
// missed extensions after #739, so a pakke shipping only extensions counted as
// shipping nothing.
func manifestItemCount(m *Manifest) int {
	total := 0
	for _, kind := range AllKinds {
		names, ok := m.NamesByKind(kind)
		if !ok {
			continue
		}
		total += len(names)
	}
	return total
}

func pakkeContents(resolver *SourceResolver, src *Source) (*Manifest, error) {
	pakke := src.Pakke
	manifest, err := collectAllItemsWith(resolver)
	if err != nil {
		return nil, err
	}
	for _, p := range resolver.List(KindPrompt) {
		manifest.Prompts = append(manifest.Prompts, p.Name)
	}
	manifest.Name = pakke.Name
	manifest.Description = pakke.Description
	if err := validateManifest(manifest); err != nil {
		return nil, fmt.Errorf("agentpakke %q: %w", pakke.Name, err)
	}
	// A Tier 2-only agentpakke declares no layout, so shipping nothing at these
	// paths is what it is *supposed* to look like: what it ships is payload
	// trees, which the pin materializes. The zero-item manifest it returns here
	// carries the name and description `nav-pilot list` prints. Only a manifest
	// that declares a layout and then ships nothing at it is an error, and the
	// message says exactly that.
	total := manifestItemCount(manifest)
	if total == 0 && pakke.Layout != nil {
		return nil, fmt.Errorf("agentpakke %q declares a layout but ships nothing at it.\n"+
			"Check the layout paths in %s", pakke.Name, agentpakke.ManifestPath)
	}
	return manifest, nil
}

// scopeStateHashes maps every tracked path to the hash nav-pilot recorded when
// it last wrote that file. It is what makes "conflict" mean "the user changed
// it" rather than "upstream moved": a file whose hash still matches the record
// is nav-pilot's own and may be overwritten.
//
// Conflicted entries are deliberately left out, exactly as
// [artifacts.SyncOpenCodeArtifacts] does: their recorded hash is the user's own
// copy, so comparing against it would call an edited file untouched. Left out,
// the path is treated as untracked and the byte comparison keeps the conflict.
// An entry with no hash (a file recorded by name only) is left out for the same
// reason.
//
// So is an ignored one, and for the same reason again: `ignore` keeps the
// recorded hash, so an untouched-but-ignored file would hash equal, be
// overwritten as an ordinary update, and come back with an empty status —
// silently un-ignored and back under sync's management.
//
// Both are still present, with an empty hash: the path is nav-pilot's, and
// install keeps a differing file there and records it. Left out entirely it
// would read as a file nav-pilot never installed, be skipped, and then be
// removed as an orphan.
func scopeStateHashes(scope *InstallScope) map[string]string {
	hashes := map[string]string{}
	state, err := readScopedState(scope)
	if err != nil || state == nil {
		return hashes
	}
	for _, f := range state.Files {
		switch {
		case f.Status == fileStatusConflict || f.Status == fileStatusIgnored:
			hashes[f.Path] = ""
		case f.Hash != "":
			hashes[f.Path] = f.Hash
		}
	}
	return hashes
}

// mergeStateFiles overlays the files an install just wrote onto the prior
// entries that survived [removeOrphans], keyed by path. Replacing the list
// instead would drop every path the narrower install did not name, and sync
// only ever walks state.Files — so a dropped artifact becomes invisible and no
// command can update or remove it.
//
// The index is built as it goes, so a prior list that already held the same
// path twice collapses to one entry instead of leaving the stale copy behind
// for conflictStatePaths and resolveSyncFiles to disagree over.
//
// A fresh entry with no Source inherits the prior one's: a scope install over a
// path that `add --source` tagged must not drop the foreign-source marker
// (#571).
func mergeStateFiles(prior, files []InstalledFile) []InstalledFile {
	merged := make([]InstalledFile, 0, len(prior)+len(files))
	index := make(map[string]int, cap(merged))
	for _, f := range append(append([]InstalledFile(nil), prior...), files...) {
		i, ok := index[f.Path]
		if !ok {
			index[f.Path] = len(merged)
			merged = append(merged, f)
			continue
		}
		if f.Source == "" {
			f.Source = merged[i].Source
		}
		merged[i] = f
	}
	return merged
}

// installArtifact handles the install for any artifact type.
// Resolution, copy, hash logic are driven by the ArtifactKind.
func installArtifact(resolver *SourceResolver, scope *InstallScope, stateHashes map[string]string, kind *ArtifactKind, name string, dryRun, force bool, result *installResult) error {
	if err := validateName(name); err != nil {
		return fmt.Errorf("invalid %s name: %w", kind.Name, err)
	}

	art, found := resolver.Get(kind, name)
	if !found {
		// Warnings go to stderr: a caller reading stdout as a JSON document
		// still needs to be told what was not installed.
		fmt.Fprintf(os.Stderr, "  %s %s not found: %s\n", yellow("⚠"), titleCase(kind.Name), name)
		result.Missing = append(result.Missing, name)
		return nil
	}

	dst := scope.DstPath(kind.Dir, art.FileName())
	relPath := kind.RelPathForName(scope, art.Name)
	if art.IsDir && !strings.HasSuffix(relPath, "/") {
		relPath = scope.RelPath(kind.Dir, art.Name) + "/"
	}

	// A tracked file that still hashes to what nav-pilot recorded is nav-pilot's
	// own and untouched, so a differing source is an update, not a conflict.
	// Only an untracked path, or one recorded as a conflict, falls back to the
	// byte comparison.
	conflicted, foreign := false, false
	if storedHash, tracked := stateHashes[relPath]; tracked && storedHash != "" {
		// An absent or unreadable file is not the user's edit either, so it is
		// not a conflict — it is simply reinstalled.
		current, hashErr := rawArtifactHash(dst, art.IsDir)
		conflicted = hashErr == nil && current != storedHash
	} else {
		c, err := checkConflict(dst, art.AbsPath, art.IsDir)
		if err != nil {
			return err
		}
		conflicted = tracked && c != nil
		foreign = !tracked && c != nil
	}

	// A file nav-pilot did not install is the team's, whatever it is called.
	// Recording it as nav-pilot's is what let a later sync --apply overwrite
	// it, so it is left untracked and never written. Only the --force typed on
	// this command line takes it over; the force a re-install implies for
	// nav-pilot's own files does not.
	if foreign && !(force && installForce) {
		printSkippedExisting(scope, kind, name, relPath)
		result.Existing = append(result.Existing, relPath)
		return nil
	}

	if conflicted && !force {
		// The file exists and its content differs from the hash nav-pilot
		// recorded. Skip the overwrite but track as conflict so it is not lost
		// from state or reported as "new".
		//
		// "differs from what nav-pilot installed" and not "locally modified"
		// (#692): the hash comparison cannot tell an edit from a file that was
		// installed by an older revision of the source, and saying "you changed
		// this" to someone who did not is how a reader learns to ignore the
		// warning.
		fmt.Fprintf(os.Stderr, "  %s %s (differs from what nav-pilot installed, kept; %s takes the source's version and saves yours as .orig)\n",
			yellow("⚠"), name, bold("nav-pilot sync --apply"))
		existingHash, hashErr := rawArtifactHash(dst, art.IsDir)
		if hashErr == nil {
			result.Files = append(result.Files, InstalledFile{Path: relPath, Hash: existingHash, Status: fileStatusConflict})
		}
		result.Conflicts++
		return nil
	}

	if dryRun {
		extra := ""
		if kind.IsDir {
			refCount := countDirFiles(filepath.Join(art.AbsPath, "references"))
			if refCount > 0 {
				extra = dim(fmt.Sprintf(" (%d reference file(s))", refCount))
			}
		}
		// Every path it would write, the way the user types it (#9).
		fmt.Printf("  %s %s%s\n", dim("→"), scopePath(scope, dst)+dirSlash(art.IsDir), extra)
		if kind == KindHook {
			fmt.Printf("  %s %s\n", dim("→"), hookRegistrationPath(scope, art.Name))
		}
		result.Installed++
		return nil
	}

	if conflicted || foreign {
		saved, err := saveOrig(dst, art.AbsPath, scope.RootDir, art.IsDir)
		if err != nil {
			return fmt.Errorf("saving your copy of %s before replacing it: %w", relPath, err)
		}
		warnReplacedLocalEdits(scope, relPath, saved)
	}
	if err := copyArtifact(art.AbsPath, dst, scope.RootDir, art.IsDir); err != nil {
		// Every kind, not just hooks: cplt's deny list covers the skill
		// directories too since cplt#508, so an install inside a sandbox now
		// fails here on a skill before it ever reaches a hook (#862).
		return fmt.Errorf("copying %s %s: %w", kind.Name, name, explainSandboxedWrite(err, kind, filepath.Dir(dst)))
	}
	hash, err := rawArtifactHash(dst, art.IsDir)
	if err != nil {
		return fmt.Errorf("hashing installed %s %s: %w", kind.Name, name, err)
	}
	result.Files = append(result.Files, InstalledFile{Path: relPath, Hash: hash})

	// A hook is the one kind whose file on disk does nothing on its own: the
	// CLI runs it only once an entry points at it. Activation sits here rather
	// than in installItems because cmdAdd and cmdAddFromSource install one
	// artifact through this same function, and a gate that activates only on
	// the collection path is the half-built install #569 is about.
	if kind == KindHook {
		if err := activateHook(scope, art, result); err != nil {
			return fmt.Errorf("activating hook %s: %w", name, err)
		}
	}

	fmt.Printf("  %s %s\n", green("✓"), name)
	result.Installed++

	return nil
}

// printConflictHint says what a conflict is and how it ends. "Skipped" on its
// own reads as permanent, and --force reads as "throw my edits away": neither
// says that a sync --apply takes the source's version of exactly these files.
//
// It says "kept" and not "kept as you edited them" (#692): a file installed
// from an older revision of the source differs without anyone having touched
// it, and the hash comparison behind this cannot tell the two apart.
func printConflictHint(n int) {
	fmt.Printf("%s %d file(s) kept, differing from what nav-pilot installed.\n", yellow("⚠"), n)
	fmt.Printf("  %s takes the source's version and saves yours as <file>.orig.\n",
		bold("nav-pilot sync --apply"))
}

// ─── Commands ───────────────────────────────────────────────────────────────

// finishInstall closes out an install command.
//
// B2: an explicit --source is persisted only after the install it was given to
// actually succeeded (and validated, which resolveSource does). A cancelled
// prompt installed nothing, so it persists nothing — and it is still a clean
// exit, not an error the user has to read.
//
// Only a scope-defining install persists. `install <name> --type <t> --source X`
// pulls one artifact out of another agentpakke; it does not make X the scope's
// agentpakke, and writing it to the config would refuse every later plain add
// (B3) and let sync adopt X for a pre-tracking scope. The file is stamped with
// its origin instead, which is what keeps it current.
func finishInstall(err error, flagSource string, dryRun, scopeDefining bool) error {
	if errors.Is(err, errInstallCancelled) {
		return nil
	}
	if err != nil {
		return err
	}
	if scopeDefining {
		persistInstalledSource(flagSource, dryRun)
	}
	return nil
}

// cmdInstallAuto resolves whether <name> is a collection or an individual artifact,
// then dispatches to the appropriate installer. If --type is provided, it skips
// collection lookup and installs a specific artifact type.
func cmdInstallAuto(name, itemType string, scope *InstallScope, ref, sourceRepo string, dryRun, force bool, jsonOutput bool) error {
	// If explicit --type given, go straight to single-artifact install
	if itemType != "" {
		// run() already refuses --frozen --type as a usage error, so this is
		// unreachable from the CLI. It is here so that refusal is a
		// convenience rather than the only thing standing between --frozen and
		// an install with no precheck, no pin match and no completeness check.
		//
		// A plain error, not frozenf: contradictory flags are a usage mistake
		// and exit 1 from either door. Exit 3 means the pin was not honoured,
		// which is a different thing for CI to branch on, and one mistake must
		// not have two answers depending on which door it came through — the
		// same reason --user answers 1 in both places.
		if installFrozen {
			return fmt.Errorf("%s installs the agentpakke %s names, and %s takes one item regardless of what it says",
				bold("--frozen"), agentpakke.DeclarationPath, bold("--type "+itemType))
		}
		if _, ok := kindByName[itemType]; !ok {
			return fmt.Errorf("unknown type %q. Valid types: %s", itemType, strings.Join(kindNames(), ", "))
		}
		return cmdAdd(itemType, name, scope, ref, sourceRepo, dryRun, force, jsonOutput)
	}

	if !dryRun && !scope.IsUser() {
		if _, err := os.Stat(filepath.Join(scope.RootDir, ".git")); os.IsNotExist(err) {
			return fmt.Errorf("target %q does not appear to be a git repository (no .git directory)", scope.RootDir)
		}
	}

	if err := guardScopeSource(scope, sourceRepo); err != nil {
		return err
	}

	// --frozen refuses every repo state it cannot be frozen against before a
	// single byte is fetched or written.
	if err := frozenPrecheck(scope); err != nil {
		return err
	}

	if !jsonOutput {
		fmt.Println(dim("Resolving source..."))
	}
	src, err := resolveDeclaredSource(scope, ref, sourceRepo)
	if err != nil {
		return err
	}
	defer src.Cleanup()

	if err := frozenPinMatches(scope, src); err != nil {
		return err
	}

	// A source that ships an agentpakke manifest supersedes the collection
	// model: its single installable name is the agentpakke identity, and that
	// name wins over an artifact of the same name (agentpakker commonly ship an
	// agent named after themselves). The artifact stays installable with --type.
	pakkeName := pakkeInstallName(src)
	if pakkeName != "" && name == pakkeName {
		return cmdInstallFromSource(name, src, scope, dryRun, force, jsonOutput)
	}

	// Check if name matches a collection
	isCollection := false
	var collections []string
	if pakkeName != "" {
		collections = []string{pakkeName}
	} else {
		collections, _ = listCollectionDirs(src.Dir) // ignore error: missing dir = no collections
		for _, c := range collections {
			if c == name {
				isCollection = true
				break
			}
		}
	}

	// Check if name matches any artifact. Composed: an artifact this pakke
	// inherits is one it installs, so `nav-pilot install <navn>` has to be able
	// to find it by name here too, and the "did you mean" candidates below have
	// to know about it (#844).
	//
	// Not narrowed by the declaration's item list: this is a name lookup that
	// ends in cmdAdd, and naming one artifact on the command line is not the
	// pakke install the list governs. See the same note there (#869).
	resolver, bases, err := composedResolverFor(src, name)
	if err != nil {
		return err
	}
	defer bases.cleanup()
	var matchedKinds []*ArtifactKind
	for _, kind := range AllKinds {
		if _, ok := resolver.Get(kind, name); ok {
			matchedKinds = append(matchedKinds, kind)
		}
	}

	// Resolve ambiguity
	if isCollection && len(matchedKinds) > 0 {
		kindNames := make([]string, len(matchedKinds))
		for i, k := range matchedKinds {
			kindNames[i] = k.Name
		}
		return fmt.Errorf("%q matches both a collection and %s %s.\n  Install the collection: nav-pilot install %s\n  Install the %s: nav-pilot install %s --type %s",
			name, articleFor(matchedKinds[0].Name), strings.Join(kindNames, ", "),
			name, matchedKinds[0].Name, name, matchedKinds[0].Name)
	}

	if !isCollection && len(matchedKinds) > 1 {
		kindNames := make([]string, len(matchedKinds))
		for i, k := range matchedKinds {
			kindNames[i] = k.Name
		}
		return fmt.Errorf("%q matches multiple artifact types: %s.\n  Use --type to specify: nav-pilot install %s --type <%s>",
			name, strings.Join(kindNames, ", "), name, strings.Join(kindNames, "|"))
	}

	if isCollection {
		return cmdInstallFromSource(name, src, scope, dryRun, force, jsonOutput)
	}

	if len(matchedKinds) == 1 {
		// A single artifact is an a-la-carte install: it writes no declaration
		// (a documented limitation) and installs one name regardless of what
		// the declaration lists, so there is nothing for --frozen to hold.
		if installFrozen {
			return frozenf("%q is a single %s, not the agentpakke %s declares.\n"+
				"--frozen installs what the declaration names; drop the flag to take one item",
				name, matchedKinds[0].Name, agentpakke.DeclarationPath)
		}
		return cmdAddFromSource(resolver, matchedKinds[0].Name, name, src, scope, sourceRepo, dryRun, force, jsonOutput)
	}

	// The five retired collections get the fold, not a "not found": the name
	// still lives in docs, muscle memory, and committed state files (#468).
	if pakkeName == agentpakke.DefaultName && agentpakke.IsLegacyCollection(name) {
		return fmt.Errorf("%q is retired, and %q replaces it (navikt/copilot#468).\n"+
			"It installs everything those names covered; deselect in the interactive picker if you want less.\n\n"+
			"  Install it:  %s",
			name, pakkeName, bold("nav-pilot install "+pakkeName))
	}

	// Not found — suggest closest match
	var candidates []string
	candidates = append(candidates, collections...)
	for _, kind := range AllKinds {
		for _, art := range resolver.List(kind) {
			candidates = append(candidates, art.Name)
		}
	}
	if s := suggest(name, candidates); s != "" {
		return fmt.Errorf("%q not found. Did you mean %q?\n\nRun 'nav-pilot list' to see what is available", name, s)
	}
	return fmt.Errorf("%q not found. Run 'nav-pilot list' to see what is available", name)
}

// articleFor returns "a" or "an" for an artifact kind name.
func articleFor(kind string) string {
	switch kind[0] {
	case 'a', 'e', 'i', 'o', 'u':
		return "an"
	}
	return "a"
}

// cmdInstallFromSource installs a collection from an already-resolved source.
//
// The active agentpakke manifest decides what "a collection" means here: a
// manifest-bearing source supersedes collections/<name>/manifest.json and
// installs everything its layout declares, while a manifest-less source keeps
// reading the collection manifest exactly as before.
func cmdInstallFromSource(collection string, src *Source, scope *InstallScope, dryRun, force bool, jsonOutput bool) error {
	defer suppressHumanOutput(jsonOutput)()

	// A Tier 1 agentpakke that publishes stable releases installs the newest
	// release, not the default branch this source resolved (#794). Before
	// anything reads src: the manifest, the resolver and the item list all
	// belong to the revision being installed.
	relSrc, release, err := tier1Release(scope, src, installRef != "" || pinnedByDeclaration(scope, src))
	if err != nil && !fallBackToHead(err) {
		return err
	}
	if relSrc != src {
		defer relSrc.Cleanup()
		src = relSrc
	}

	// Before the tier split, not inside it: a mixed pakke — layout plus
	// payloads — takes the Tier 1 route below, so a check on the payload-only
	// branch would let it install its layout half and report green.
	if err := frozenTier2(src); err != nil {
		return err
	}

	// A payload-only agentpakke has no Tier 1 content to materialize into this
	// scope; installing it means pinning a revision of its payloads.
	if payloadOnly(src) {
		return installPakkePin(scope, src, dryRun, jsonOutput)
	}

	// Before a byte is written: a scope that changes agentpakke loses the
	// previous one's content, and that is the last moment the user can say no
	// without ending up holding both. After the Tier 2 return, which gates
	// itself: every pin entry point reaches [installPakkePin], not all of
	// them come through here.
	if !confirmSourceSwitch(scope, src, dryRun, jsonOutput) {
		return errInstallCancelled
	}

	// Fail closed before touching the filesystem (A3): a non-conforming
	// agentpakke must not leave a partial install behind.
	if err := validatePakkeSource(src); err != nil {
		return err
	}

	// A pakke that reuses another resolves it here, before its contents are
	// collected: pakkeContents lists through the resolver, so the reused
	// artifacts are part of the manifest without a second merge step. The
	// repo's committed item list narrows the result, and does so before
	// anything is written: a list naming something the agentpakke does not ship
	// refuses the whole install rather than half of it.
	resolver, bases, manifest, err := composedContentsFor(scope, src, collection)
	if err != nil {
		return err
	}
	defer bases.cleanup()
	reused := bases.nearest()

	if err := confirmInstallWrites(scope, resolver, manifest, dryRun, jsonOutput); err != nil {
		return err
	}
	// Past every refusal, so nothing is consented to for an install that then
	// does not happen (#858). The Tier 2 route asks from installPakkePin, after
	// its own guards.
	noteProposalConsent(scope, src, dryRun, jsonOutput)

	sourceLabel := sourceLabelFor(src)

	if !jsonOutput {
		fmt.Println()
		if dryRun {
			fmt.Println(bold(fmt.Sprintf("Dry run: %s", collection)))
		} else {
			fmt.Println(bold(fmt.Sprintf("Installing: %s", collection)))
		}
		fmt.Printf("%s %s\n", dim("Source:"), dim(fmt.Sprintf("%s@%s", sourceLabel, shortSHA(src.SHA))))
		if reused != nil {
			fmt.Printf("%s %s\n", dim("Reuses:"), dim(fmt.Sprintf("%s@%s", sourceLabelFor(reused), shortSHA(reused.SHA))))
		}
		fmt.Printf("%s %s\n", dim("Target:"), dim(scope.Label()))
		printManifestContents(manifest)
		fmt.Println()
	}

	result, err := installItems(resolver, scope, manifest, dryRun, force)
	if err != nil {
		return err
	}
	if !dryRun {
		telemetry.RecordInstallItems(scope.Name, telemetryMode(), int64(result.Installed))
	}

	// Before the JSON output, so the machine-readable path cannot report a
	// partial install as a success either.
	if err := frozenComplete(result, scope); err != nil {
		return err
	}

	// Deferred until after the state file and the declaration are written.
	// Emitting here would have returned before both, so `install --json`
	// reported a success that left no state and no lock — a CI job could not
	// tell an install apart from a no-op, and the next sync had nothing to
	// reconcile against (#797).
	// Filled in by the state rebuild below; the closure reads them when it
	// runs, which is after it. The per-file list the human output collapses on
	// a switch lives here instead.
	var removedOrphans []string
	var switchedFrom string

	emitJSON := func() error {
		doc := withPakkeName(map[string]interface{}{
			"command":     "install",
			"scope":       scope.Name,
			"source_sha":  src.SHA,
			"version":     src.Version,
			"installed":   result.Installed,
			"skipped":     len(result.Missing),
			"conflicts":   result.Conflicts,
			"unsupported": result.Unsupported,
			"dry_run":     dryRun,
		}, stateCollection(src, collection))
		if len(removedOrphans) > 0 {
			doc["removed"] = removedOrphans
		}
		if len(result.Existing) > 0 {
			doc["skipped_existing"] = result.Existing
		}
		if switchedFrom != "" {
			doc["switched_from"] = switchedFrom
		}
		// The same two keys a Tier 2 pin reports (#794): version above is
		// nav-pilot's own, and cannot carry the agentpakke's.
		if release != nil {
			doc["pakke_version"], doc["follows_releases"] = release.Version, true
		}
		return outputJSON(doc)
	}

	if !jsonOutput {
		if result.Conflicts > 0 {
			printConflictHint(result.Conflicts)
		}

		if len(result.Unsupported) > 0 {
			fmt.Printf("%s Skipped (not supported in %s scope): %s\n",
				yellow("⚠"), scope.Name, strings.Join(result.Unsupported, ", "))
		}
	}

	if dryRun {
		if jsonOutput {
			return emitJSON()
		}
		printDryRunRecords(scope, !installFrozen)
		fmt.Printf("%s Would install %d items from %q.\n",
			dim("→"), result.Installed, collection)
		return nil
	}

	stateVersion := src.Version

	state := &StateFile{
		Collection:  stateCollection(src, collection),
		Version:     stateVersion,
		Scope:       scope.Name,
		SourceRepo:  src.Repo,
		SourceSHA:   src.SHA,
		InstalledAt: timeNow().UTC().Format("2006-01-02T15:04:05Z07:00"),
		Files:       stampRevision(result.Files, src.SHA, inheritedPaths(resolver, scope, reused)),
	}
	// A re-install over a checked-in state file must not throw away the keys a
	// newer nav-pilot put there; the fresh struct has none of them (#588). Nor
	// the paths it does not name: a narrower install must not make the rest of
	// the scope invisible to sync.
	//
	// The two must be decided together. removeOrphans deletes the prior paths
	// this install did not write and nav-pilot still owns; merging every prior
	// entry back would then commit a state that lists a file the tool has just
	// deleted. So only what removeOrphans kept is merged.
	prior, _ := readScopedState(scope)
	if prior != nil {
		state.PreserveUnknownFrom(prior)
		var kept []InstalledFile
		kept, removedOrphans = removeOrphans(scope, prior, result.Files, src.Repo)
		switchedFrom = sourceSwitch(prior, src.Repo)
		state.Files = mergeStateFiles(kept, state.Files)
	}
	recordRelease(state, prior, src, release, installRef != "")
	if err := writeScopedState(scope, state); err != nil {
		fmt.Fprintf(os.Stderr, "%s Could not write state file: %v\n", yellow("⚠"), err)
	}
	// --frozen never moves the pin, not even to rewrite it as the value it
	// already holds: a CI job must not produce a diff in a file it was only
	// meant to obey.
	if !installFrozen {
		recordDeclaration(scope, src, jsonOutput)
	}

	if jsonOutput {
		return emitJSON()
	}

	fmt.Printf("%s Installed %d items from %q (v%s, %s).\n",
		green("✓"), result.Installed, collection, stateVersion, shortSHA(src.SHA))
	fmt.Println()
	// The pakke's own persona, not ours. A team that installs their agentpakke
	// and is then told to use @nav-pilot has been handed the wrong name for
	// what they just installed, which is the same mistake the launch path made
	// before #728.
	agent := installedPrimaryAgent(src)
	if scope.IsUser() {
		fmt.Println(dim("Agents and skills are now available across all your repos."))
		fmt.Println(dim(fmt.Sprintf("Use @%s in Copilot Chat, or start it in the sandbox with nav-pilot (or cplt -- --agent %s)", agent, agent)))
	} else {
		fmt.Println(dim("Next steps:"))
		fmt.Println(dim("  1. Review the installed files in .github/"))
		fmt.Println(dim("  2. Commit and push to enable Copilot customization"))
		fmt.Println(dim(fmt.Sprintf("  3. Use @%s in Copilot to start planning", agent)))
	}
	printMCPServerNotice(src)

	return nil
}

// cmdAddFromSource installs a single artifact from an already-resolved source.
// It preserves the à-la-carte state semantics from cmdAdd.
func cmdAddFromSource(resolver *SourceResolver, itemType, name string, src *Source, scope *InstallScope, explicitSource string, dryRun, force bool, jsonOutput bool) error {
	if !scope.SupportsType(itemType) {
		return fmt.Errorf("type %q is not supported in user scope. Only %s can be installed to ~/.copilot", itemType, strings.Join(scope.SupportedTypes, ", "))
	}

	sourceLabel := sourceLabelFor(src)

	result := &installResult{}

	if !jsonOutput {
		fmt.Println()
		if dryRun {
			fmt.Println(bold(fmt.Sprintf("Dry run: install %s %s", itemType, name)))
		} else {
			fmt.Println(bold(fmt.Sprintf("Installing %s: %s", itemType, name)))
		}
		fmt.Printf("%s %s\n", dim("Source:"), dim(fmt.Sprintf("%s@%s", sourceLabel, shortSHA(src.SHA))))
		fmt.Printf("%s %s\n", dim("Target:"), dim(scope.Label()))
		fmt.Println()
	}

	kind := kindByName[itemType]
	installErr := installArtifact(resolver, scope, scopeStateHashes(scope), kind, name, dryRun, force, result)
	if installErr != nil {
		return installErr
	}
	if !dryRun {
		telemetry.RecordInstallItems(scope.Name, telemetryMode(), int64(result.Installed))
	}

	if jsonOutput {
		return outputJSON(map[string]interface{}{
			"command":    "install",
			"type":       itemType,
			"name":       name,
			"scope":      scope.Name,
			"source_sha": src.SHA,
			"installed":  result.Installed,
			"conflicts":  result.Conflicts,
			"dry_run":    dryRun,
		})
	}

	if result.Conflicts > 0 {
		fmt.Println()
		printConflictHint(result.Conflicts)
	}

	if dryRun || result.Installed == 0 {
		return nil
	}

	foreign, err := recordAddedFiles(scope, src, result, explicitSource)
	if err != nil {
		return err
	}

	fmt.Printf("\n%s Installed %s %q.\n", green("✓"), itemType, name)
	noteForeignSource(scope, foreign, fmt.Sprintf("nav-pilot install %s --type %s --source %s --force", name, itemType, foreign))
	return nil
}

// cmdList lists what a source ships. It takes the scope so it reads the same
// declaration an install would: listing the default pakke's items in a repo
// pinned elsewhere is how the unknown-item refusal ends up sending a user to
// output that cannot contain the name they mistyped.
// hintSource is " --source <repo>" while `list --source` prints the commands
// to run next, and empty otherwise. --source is not remembered unless the
// install says --save-source, so a hint without it would install from
// somewhere else.
var hintSource string

func cmdList(scope *InstallScope, ref, sourceRepo string, showItems bool, jsonOutput bool) error {
	if sourceRepo != "" {
		hintSource = " --source " + sourceRepo
		defer func() { hintSource = "" }()
	}
	if !jsonOutput {
		fmt.Println(dim("Resolving source..."))
	}
	src, err := resolveDeclaredSource(scope, ref, sourceRepo)
	if err != nil {
		return err
	}
	defer src.Cleanup()

	// Composed, for the same reason the installs are: `list` that leaves out
	// what the pakke inherits shows a set that matches the broken install and
	// hides it (#844). It is also what the unknown-item refusal sends the user
	// to read.
	resolver, bases, err := composedResolverFor(src, "")
	if err != nil {
		return err
	}
	defer bases.cleanup()

	var collections []collectionInfo
	add := func(m *Manifest) {
		collections = append(collections, collectionInfo{
			Name:        m.Name,
			Description: m.Description,
			Items:       manifestItemCount(m),
			agents:      m.Agents,
		})
	}

	if src.Pakke != nil {
		m, err := pakkeContents(resolver, src)
		if err != nil {
			return err
		}
		add(m)
	} else {
		names, err := listCollectionDirs(src.Dir)
		if err != nil {
			return err
		}
		for _, name := range names {
			m, err := loadManifest(src.Dir, name)
			if err != nil {
				continue
			}
			m.Name = name
			add(m)
		}
	}

	if jsonOutput {
		result := map[string]interface{}{"collections": collections, "source": sourceLabelFor(src)}
		if showItems {
			result["items"] = collectAvailableItems(resolver)
		}
		return outputJSON(result)
	}

	fmt.Println()
	if src.Pakke != nil && len(collections) == 1 {
		printPakkeListing(os.Stdout, sourceLabelFor(src), collections[0], payloadLines(src), showItems, termWidth())
	} else {
		fmt.Println(bold("Available collections:"))
		fmt.Println()
		for _, c := range collections {
			fmt.Printf("  %-20s %s %s\n", bold(c.Name), c.Description, dim(fmt.Sprintf("(%d items)", c.Items)))
			if len(c.agents) > 0 {
				fmt.Printf("  %-20s %s\n", "", dim("agents: "+strings.Join(c.agents, ", ")))
			}
		}
		fmt.Println()
		fmt.Printf("Install with: %s\n", bold("nav-pilot install <name>"))
		fmt.Printf("Install everything to user home: %s\n", bold("nav-pilot install --user --all"))
		if !showItems {
			fmt.Printf("Show individual items: %s\n", bold("nav-pilot list --items"))
		}
	}

	if showItems {
		fmt.Println()
		if err := listAvailableItems(resolver); err != nil {
			return err
		}
	}
	return nil
}

// collectionInfo is one listable name. A source that ships an agentpakke
// manifest offers exactly one — the agentpakke itself — because the manifest
// supersedes the collections/<name> model. The json tags are the --json
// contract.
type collectionInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Items       int    `json:"items"`
	agents      []string
}

// payloadLines describes the payload contexts of a Tier 2 agentpakke, which
// lists 0 items because it materializes no user-visible files.
//
// Every payload-bearing client is listed, launchable by this binary or not,
// because an install materializes every one of them (a filter would be a second
// client list to keep in step with the launch switch). A listing that hid the
// ones this binary cannot launch would hide content that is on disk; saying so
// is the useful half.
//
// What "launchable" means comes from [stagedLaunchers], the map the launch
// itself dispatches on. It is not the same set as agentpakke.IsKnownClient,
// which is every client id nav-pilot knows — a known client with no staged
// launcher is exactly the case this annotation is for.
func payloadLines(src *Source) []payloadLine {
	if !payloadOnly(src) {
		return nil
	}
	var lines []payloadLine
	for _, client := range src.Pakke.ClientIDs() {
		ctxs := declaredContexts(src.Pakke, client)
		if len(ctxs) == 0 {
			continue
		}
		line := payloadLine{contexts: client + " payloads: " + strings.Join(ctxs, ", ")}
		if _, ok := stagedLaunchers[client]; !ok {
			line.note = "(materialized on install; this nav-pilot cannot launch " + client + ")"
		}
		lines = append(lines, line)
	}
	return lines
}

// payloadLine is one client's payload contexts and, when this binary cannot
// launch that client, the annotation saying so. They are kept apart because the
// context ids come from the manifest and are not length-bounded — the list has
// to wrap — while the annotation has to stay on one line to be greppable.
type payloadLine struct {
	contexts string
	note     string
}

// printPakkeListing renders the one-agentpakke listing. The collections table
// it replaces pads every line to a 20-column gutter and prints the description
// unwrapped, which for a single pakke is a 22-space gap and a sentence running
// off the right edge of the terminal.
func printPakkeListing(out io.Writer, source string, c collectionInfo, payloads []payloadLine, showItems bool, width int) {
	body := width - 2 // the two-space indent
	fmt.Fprintln(out, bold(fmt.Sprintf("Agentpakke in %s:", source)))
	fmt.Fprintln(out)
	fmt.Fprintf(out, "  %s %s\n", bold(c.Name), dim(fmt.Sprintf("(%d items)", c.Items)))
	if len(c.agents) > 0 {
		printWrapped(out, dim, "agents: "+strings.Join(c.agents, ", "), body)
	}
	for _, p := range payloads {
		printWrapped(out, dim, p.contexts, body)
		if p.note != "" {
			fmt.Fprintf(out, "  %s\n", dim(p.note))
		}
	}
	if c.Description != "" {
		fmt.Fprintln(out)
		printWrapped(out, func(s string) string { return s }, c.Description, body)
	}
	fmt.Fprintln(out)
	fmt.Fprintf(out, "Install: %s\n", bold("nav-pilot install "+c.Name+hintSource))
	if !showItems {
		fmt.Fprintf(out, "Items:   %s\n", bold("nav-pilot list --items"+hintSource))
	}
}

// printWrapped writes text at a two-space indent, wrapped to width and coloured
// line by line so the escape codes never land inside a wrap point.
func printWrapped(out io.Writer, color func(string) string, text string, width int) {
	for _, line := range wrapWords(text, width) {
		fmt.Fprintf(out, "  %s\n", color(line))
	}
}

// wrapWords breaks text into lines of at most width columns on space
// boundaries. A word longer than width gets a line of its own rather than
// being cut.
func wrapWords(text string, width int) []string {
	var lines []string
	line := ""
	for _, word := range strings.Fields(text) {
		switch {
		case line == "":
			line = word
		case utf8.RuneCountInString(line)+1+utf8.RuneCountInString(word) <= width:
			line += " " + word
		default:
			lines = append(lines, line)
			line = word
		}
	}
	if line != "" {
		lines = append(lines, line)
	}
	return lines
}

// termWidth is the column budget for human output: the terminal's width, or 80
// when stdout is not a terminal (piped, redirected, CI).
func termWidth() int {
	if w, _, err := term.GetSize(os.Stdout.Fd()); err == nil && w > 20 {
		return w
	}
	return 80
}

// listAvailableItems prints all agents, skills, instructions, and prompts in the source.
func listAvailableItems(resolver *SourceResolver) error {
	for _, kind := range AllKinds {
		items := resolver.List(kind)
		if len(items) == 0 {
			continue
		}
		fmt.Println(bold(fmt.Sprintf("Available %s:", kind.Dir)))
		for _, item := range items {
			fmt.Printf("  %-30s %s\n", item.Name, dim("nav-pilot install "+item.Name+hintSource))
		}
		fmt.Println()
	}
	return nil
}

// collectAvailableItems returns all available items as a structured map for JSON output.
func collectAvailableItems(resolver *SourceResolver) map[string][]string {
	result := make(map[string][]string)
	for _, kind := range AllKinds {
		for _, art := range resolver.List(kind) {
			result[kind.Dir] = append(result[kind.Dir], art.Name)
		}
	}
	return result
}

// cmdInstallInteractive handles `nav-pilot install` with no arguments in an interactive terminal.
// Reuses the same scope picker and collection/item pickers as the root `nav-pilot` command.
//
// scope is non-nil when the command line already named one (--user, --repo,
// --target). Then the scope question is settled, and asking it anyway is how
// `install --repo` opened the picker its own help says it skips (#820). It is
// nil only for bare `nav-pilot install`, which still gets the picker.
func cmdInstallInteractive(scope *InstallScope, targetDir, ref, sourceRepo string, force bool) error {
	// The scope is settled before the source is resolved, because the repo's
	// committed declaration is *part of* how the source is chosen: resolving
	// first would install whatever the config key happened to name and then
	// overwrite the declaration with it — for the one command, `nav-pilot
	// install` with no arguments, the declaration exists to serve.
	if scope == nil {
		var err error
		scope, err = ScopeUser()
		if err != nil {
			return err
		}
		if targetDir != "" {
			// In a git repo: ask where to install
			scope, err = promptInstallScopeFn(targetDir)
			if err != nil {
				return err
			}
			if scope == nil {
				fmt.Println(dim("Cancelled."))
				return errInstallCancelled
			}
		}
	}

	fmt.Println(dim("Resolving source..."))
	src, err := resolveDeclaredSource(scope, ref, sourceRepo)
	if err != nil {
		return err
	}
	defer src.Cleanup()

	if scope.IsUser() {
		return interactiveUserInstallFromSource(scope, src, sourceRepo)
	}

	// Repo scope: pick a collection
	return interactiveRepoInstall(src, scope, sourceRepo, force)
}

// cmdInstallAll installs all agents and skills to a scope by scanning the source.
// Used when `nav-pilot install --user` is run without a collection name, and
// when --all is passed with an explicit scope.
//
// explicitAll is true when the command line said --all. Then there is nothing
// left to ask: the scope flag named the target and --all named the selection,
// so the picker is skipped.
func cmdInstallAll(scope *InstallScope, ref, sourceRepo string, dryRun, force bool, jsonOutput bool, explicitAll bool) error {
	if err := guardScopeSource(scope, sourceRepo); err != nil {
		return err
	}
	if !jsonOutput {
		fmt.Println(dim("Resolving source..."))
	}
	src, err := resolveDeclaredSource(scope, ref, sourceRepo)
	if err != nil {
		return err
	}
	defer src.Cleanup()

	// Interactive mode: offer the picker (same UX as `nav-pilot` root command),
	// but only when the command line left the question open. An explicit --all
	// has answered it, and asking anyway is how `install --user --all` failed
	// wherever nothing could answer the prompt (#802 review).
	if isInteractive() && !dryRun && !jsonOutput {
		if !explicitAll {
			return interactiveUserInstallFn(scope, src, sourceRepo)
		}
		// The picker's flow force-updates managed files on a re-install, and
		// that is what someone re-running `install --user --all` is asking for.
		// Taking only the --force flag here would quietly stop refreshing them.
		state, _ := readScopedState(scope)
		force = force || hasManagedFiles(state)
	}

	return installAllFromSource(scope, src, nil, dryRun, force, jsonOutput)
}

// installAllFromSource installs all agents+skills from source.
// If manifest is nil, it scans the source directory to discover items.
// extraStateFiles are appended to the state file after install (e.g. ignored items from picker).
// Extracted so both cmdInstallAll and the interactive flow can share this.
func installAllFromSource(scope *InstallScope, src *Source, manifest *Manifest, dryRun, force bool, jsonOutput bool, extraStateFiles ...InstalledFile) error {
	defer suppressHumanOutput(jsonOutput)()

	// The newest stable release, not the default branch, for a Tier 1
	// agentpakke that publishes one (#794). See [cmdInstallFromSource].
	//
	// ponytail: the interactive picker builds its manifest from the default
	// branch and hands it in here, so an item the release does not ship is
	// reported as missing rather than never offered. Move the swap into
	// interactiveUserInstallFromSource if that is ever more than theoretical.
	relSrc, release, err := tier1Release(scope, src, installRef != "" || pinnedByDeclaration(scope, src))
	if err != nil && !fallBackToHead(err) {
		return err
	}
	if relSrc != src {
		defer relSrc.Cleanup()
		src = relSrc
	}

	if payloadOnly(src) {
		return installPakkePin(scope, src, dryRun, jsonOutput)
	}

	// Same gate as cmdInstallFromSource, for the same reason: both paths end in
	// removeOrphans, and both can take another agentpakke's content with them.
	if !confirmSourceSwitch(scope, src, dryRun, jsonOutput) {
		return errInstallCancelled
	}

	// Fail closed before touching the filesystem (A3).
	if err := validatePakkeSource(src); err != nil {
		return err
	}

	// `--all` means everything the pakke installs, which includes what it
	// reuses and excludes what the repo's item list leaves out. Built here and
	// not before the Tier 2 return above: a payload-only pakke has no layout
	// for a base to contribute to (#844).
	resolver, bases, declared, err := composedContentsFor(scope, src, CollectionAll)
	if err != nil {
		return err
	}
	defer bases.cleanup()
	reused := bases.nearest()

	// A manifest handed in comes from the picker, which offered the declared
	// set and got a choice out of it. Narrowing that choice a second time would
	// refuse the install over every item the user deselected.
	if manifest == nil {
		manifest = declared
	}

	total := len(manifest.Agents) + len(manifest.Skills) + len(manifest.Instructions)
	if total == 0 {
		return fmt.Errorf("no agents, skills, or instructions found in source")
	}
	if err := confirmInstallWrites(scope, resolver, manifest, dryRun, jsonOutput); err != nil {
		return err
	}
	// Same placement as cmdInstallFromSource: past every refusal (#858).
	noteProposalConsent(scope, src, dryRun, jsonOutput)
	// Without a terminal nobody saw a question naming them, so say it here:
	// hooks run outside the sandbox on every matching tool call.
	if !isInteractive() && !jsonOutput && len(manifest.Hooks) > 0 {
		fmt.Fprintf(os.Stderr, "%s Installing %d hooks, which run outside the sandbox on every matching tool call: %s\n",
			yellow("ℹ"), len(manifest.Hooks), strings.Join(manifest.Hooks, ", "))
	}

	sourceLabel := sourceLabelFor(src)

	if !jsonOutput {
		fmt.Println()
		if dryRun {
			fmt.Println(bold(fmt.Sprintf("Dry run: all agents, skills & instructions (%d items)", total)))
		} else {
			fmt.Println(bold(fmt.Sprintf("Installing: all agents, skills & instructions (%d items)", total)))
		}
		fmt.Printf("%s %s\n", dim("Source:"), dim(fmt.Sprintf("%s@%s", sourceLabel, shortSHA(src.SHA))))
		if reused != nil {
			fmt.Printf("%s %s\n", dim("Reuses:"), dim(fmt.Sprintf("%s@%s", sourceLabelFor(reused), shortSHA(reused.SHA))))
		}
		fmt.Printf("%s %s\n", dim("Target:"), dim(scope.Label()))
		fmt.Println()
	}

	result, err := installItems(resolver, scope, manifest, dryRun, force)
	if err != nil {
		return err
	}
	if !dryRun {
		telemetry.RecordInstallItems(scope.Name, telemetryMode(), int64(result.Installed))
	}

	if !jsonOutput && result.Conflicts > 0 {
		printConflictHint(result.Conflicts)
	}

	// Deferred past the state write, same reason as cmdInstallFromSource: this
	// returned before writeScopedState, so `install --all --json` reported a
	// success that left no state behind (#797).
	var removedOrphans []string
	var switchedFrom string

	emitJSON := func() error {
		doc := withPakkeName(map[string]interface{}{
			"command":    "install",
			"scope":      scope.Name,
			"source_sha": src.SHA,
			"version":    src.Version,
			"installed":  result.Installed,
			"conflicts":  result.Conflicts,
			"dry_run":    dryRun,
		}, stateCollection(src, CollectionAll))
		if len(removedOrphans) > 0 {
			doc["removed"] = removedOrphans
		}
		if len(result.Existing) > 0 {
			doc["skipped_existing"] = result.Existing
		}
		if switchedFrom != "" {
			doc["switched_from"] = switchedFrom
		}
		if release != nil {
			doc["pakke_version"], doc["follows_releases"] = release.Version, true
		}
		return outputJSON(doc)
	}

	if dryRun {
		if jsonOutput {
			return emitJSON()
		}
		printDryRunRecords(scope, false)
		fmt.Printf("%s Would install %d items.\n", dim("→"), result.Installed)
		return nil
	}

	stateVersion := src.Version

	state := &StateFile{
		Collection:  stateCollection(src, CollectionAll),
		Version:     stateVersion,
		Scope:       scope.Name,
		SourceRepo:  src.Repo,
		SourceSHA:   src.SHA,
		InstalledAt: timeNow().UTC().Format("2006-01-02T15:04:05Z07:00"),
		Files:       stampRevision(result.Files, src.SHA, nil),
	}

	// Append items the user explicitly deselected in the picker as ignored.
	if len(extraStateFiles) > 0 {
		state.Files = append(state.Files, extraStateFiles...)
	}

	// Same as cmdInstallFromSource: a rebuild over an existing state keeps the
	// keys this binary does not understand (#588), retires the artifacts the
	// source has stopped shipping (#615), and keeps the state of the paths this
	// install did not name. The two install paths must agree about all three.
	prior, _ := readScopedState(scope)
	if prior != nil {
		state.PreserveUnknownFrom(prior)
		// state.Files, not result.Files: the picker's deselected items were
		// appended above, and they are still on disk on purpose. Passing the
		// narrower set makes removeOrphans read a deselection as an artifact
		// the source stopped shipping, and delete a file the user chose to keep.
		var kept []InstalledFile
		kept, removedOrphans = removeOrphans(scope, prior, state.Files, src.Repo)
		switchedFrom = sourceSwitch(prior, src.Repo)
		state.Files = mergeStateFiles(kept, state.Files)
	}
	recordRelease(state, prior, src, release, installRef != "")

	if err := writeScopedState(scope, state); err != nil {
		fmt.Fprintf(os.Stderr, "%s Could not write state file: %v\n", yellow("⚠"), err)
	}

	if jsonOutput {
		return emitJSON()
	}

	fmt.Printf("%s Installed %d items to %s (v%s, %s).\n",
		green("✓"), result.Installed, scope.Label(), stateVersion, shortSHA(src.SHA))
	fmt.Println()
	reach := "in this repository"
	if scope.IsUser() {
		reach = "across all your repos"
	}
	fmt.Println(dim(fmt.Sprintf("Agents and skills are now available %s.", reach)))
	agent := installedPrimaryAgent(src)
	fmt.Println(dim(fmt.Sprintf("Use @%s in Copilot Chat, or start it in the sandbox with nav-pilot (or cplt -- --agent %s)", agent, agent)))

	if len(manifest.Instructions) > 0 && scope.IsUser() {
		fmt.Println()
		fmt.Println(dim("Instructions are available when launching cplt via nav-pilot."))
		fmt.Println(dim("For direct cplt usage, add to your shell profile:"))
		fmt.Printf("  %s\n", dim("eval \"$(nav-pilot env)\""))
	}

	printMCPServerNotice(src)

	// Hint about repo-local config if cwd is a git repo missing files
	if scope.IsUser() {
		if cwd, err := os.Getwd(); err == nil {
			hintInitIfMissing(cwd)
		}
	}

	return nil
}

// cmdListInstalledAuto shows list for all detected scopes (repo + user) when the
// scope isn't explicitly requested.
func cmdListInstalledAuto(repoDir string, jsonOutput bool) error {
	repoScope := ScopeRepo(repoDir)
	repoState, _ := readScopedState(repoScope)

	userScope, userErr := ScopeUser()
	var userState *StateFile
	if userErr == nil {
		userState, _ = readScopedState(userScope)
	}

	hasProviderCtx := false
	for _, p := range allProviders() {
		if p.ContextStatus() != nil {
			hasProviderCtx = true
			break
		}
	}

	if repoState == nil && userState == nil && !hasProviderCtx {
		if jsonOutput {
			return outputJSON(map[string]interface{}{"installed": false})
		}
		fmt.Printf("nav-pilot is not installed (repo or user scope).\n")
		fmt.Printf("Install with: %s\n", bold(installCommandFor(nil, nil)))

		return nil
	}

	if jsonOutput {
		scopes := []map[string]interface{}{}
		if repoState != nil {
			ok, modified, missing, ignored, _ := countFileIntegrity(repoScope.RootDir, repoState)
			scopes = append(scopes, withPakkeName(map[string]interface{}{
				"scope": "repo", "version": repoState.Version, "source_sha": repoState.SourceSHA,
				"installed_at": repoState.InstalledAt, "files": len(repoState.Files),
				"ok": ok, "modified": modified, "missing": missing, "ignored": ignored,
			}, repoState.Collection))
		}
		if userState != nil {
			ok, modified, missing, ignored, _ := countFileIntegrity(userScope.RootDir, userState)
			doc := withPakkeName(map[string]interface{}{
				"scope": "user", "version": userState.Version, "source_sha": userState.SourceSHA,
				"installed_at": userState.InstalledAt, "files": len(userState.Files),
				"ok": ok, "modified": modified, "missing": missing, "ignored": ignored,
			}, userState.Collection)
			if st := pakkeStatus(userScope, userState); st != nil {
				doc["pakke"] = st
			}
			scopes = append(scopes, doc)
		}
		for _, p := range allProviders() {
			cs := p.ContextStatus()
			if cs == nil {
				continue
			}
			ok, modified, missing, ignored, _ := countFileIntegrity(cs.OutputDir, cs.State)
			scopes = append(scopes, withPakkeName(map[string]interface{}{
				"scope": cs.ScopeName, "version": cs.State.Version, "source_sha": cs.State.SourceSHA,
				"installed_at": cs.State.InstalledAt, "files": len(cs.State.Files),
				"ok": ok, "modified": modified, "missing": missing, "ignored": ignored,
			}, cs.State.Collection))
		}
		return outputJSON(map[string]interface{}{"installed": true, "scopes": scopes})
	}

	if repoState != nil {
		printStatusBlock(repoScope, repoState)
	}
	if userState != nil {
		if repoState != nil {
			fmt.Println()
		}
		printStatusBlock(userScope, userState)
	}

	// Show provider-specific context status (e.g. opencode Nav context).
	for _, p := range allProviders() {
		if p.ContextStatus() == nil {
			continue
		}
		if repoState != nil || userState != nil {
			fmt.Println()
		}
		p.PrintContextStatus()
	}

	return nil
}

func cmdListInstalledScoped(scope *InstallScope, _ bool, jsonOutput bool) error {
	state, err := readScopedState(scope)
	if err != nil {
		return fmt.Errorf("reading state: %w", err)
	}
	if state == nil {
		if jsonOutput {
			return outputJSON(map[string]interface{}{"installed": false})
		}
		if scope.IsUser() {
			fmt.Println("nav-pilot is not installed in user home (~/.copilot).")
		} else {
			fmt.Println("nav-pilot is not installed.")
		}
		fmt.Printf("Install with: %s\n", bold(installCommandFor(nil, nil)))
		return nil
	}

	if jsonOutput {
		ok, modified, missing, ignored, _ := countFileIntegrity(scope.RootDir, state)
		doc := withPakkeName(map[string]interface{}{
			"installed":    true,
			"version":      state.Version,
			"scope":        scope.Name,
			"source_sha":   state.SourceSHA,
			"installed_at": state.InstalledAt,
			"files":        len(state.Files),
			"ok":           ok,
			"modified":     modified,
			"missing":      missing,
			"ignored":      ignored,
		}, state.Collection)
		if st := pakkeStatus(scope, state); st != nil {
			doc["pakke"] = st
		}
		return outputJSON(doc)
	}

	printStatusBlock(scope, state)
	return nil
}

// foreignFileCounts counts a scope's files per agentpakke they came from,
// leaving out the scope's own (which record no source). A file that sync
// deliberately skips should say so somewhere the user looks before wondering
// why it never updates.
func foreignFileCounts(state *StateFile) ([]string, map[string]int) {
	counts := map[string]int{}
	var sources []string
	for _, f := range state.Files {
		if f.Source == "" {
			continue
		}
		if counts[f.Source] == 0 {
			sources = append(sources, f.Source)
		}
		counts[f.Source]++
	}
	sort.Strings(sources)
	return sources, counts
}

// printPakkeStatus prints the release lines of a pinned agentpakke (#779).
func printPakkeStatus(st *pakkeReleaseStatus) {
	version := st.Version
	if version == "" {
		version = dim("unknown")
	}
	follows := "no"
	if st.FollowsReleases {
		follows = "yes"
	}
	fmt.Printf("  Package:     %s (pinned at %s)\n", version, shortSHA(st.PinnedSHA))
	fmt.Printf("  Releases:    follows stable releases: %s\n", follows)
	choice := updateChoice(st.UpdateChoice)
	fmt.Printf("  Updates:     %s\n", choice.label())
	if st.RolledBackFrom != "" {
		fmt.Printf("  Rolled back: from %s, which is not offered again\n", shortSHA(st.RolledBackFrom))
	}
	switch {
	case st.ReleaseCheckError != "":
		fmt.Printf("  %s release check failed: %s\n", yellow("⚠"), st.ReleaseCheckError)
	case st.PendingRelease != nil && choice == updateKeep:
		// Held back, not hidden: the user chose to sit still, and is still told
		// what they are sitting on and what it would take to move (#781).
		fmt.Printf("  %s Release %s is available, and this scope keeps the revision. Take this one with %s.\n",
			yellow("⚠"), st.PendingRelease.label(st.PendingRelease.SHA),
			bold("nav-pilot sync --user --apply --ref "+st.PendingRelease.SHA))
	case st.PendingRelease != nil:
		fmt.Printf("  %s Release %s is available. Run %s to update; sync checks the revision before it pins it.\n",
			yellow("⚠"), st.PendingRelease.label(st.PendingRelease.SHA), bold("nav-pilot sync --user --apply"))
	}
}

func printStatusBlock(scope *InstallScope, state *StateFile) {
	ok, modified, missing, ignored, modifiedPaths := countFileIntegrity(scope.RootDir, state)

	// Count explicitly excluded items (added via "nav-pilot ignore", have empty hash).
	excluded := 0
	for _, f := range state.Files {
		if f.Status == fileStatusIgnored && f.Hash == "" {
			excluded++
		}
	}
	autoIgnored := ignored - excluded

	fmt.Println(bold(fmt.Sprintf("nav-pilot install status (%s)", scope.Name)))
	fmt.Println()
	fmt.Printf("  Name:        %s\n", bold(state.Collection))
	fmt.Printf("  Version:     %s\n", state.Version)
	fmt.Printf("  Scope:       %s\n", scope.Name)
	fmt.Printf("  Source:      %s\n", shortSHA(state.SourceSHA))
	if st := pakkeStatus(scope, state); st != nil {
		printPakkeStatus(st)
	} else if version, follows := releaseClaim(state); version != "" && follows {
		// A Tier 1 install that follows stable releases (#794). Version above
		// is nav-pilot's own, which is why it reads "dev" for a custom source.
		//
		// ponytail: from the state, with no lookup. pakkeStatus is a live call
		// per status run, and a pending-release line here would put one on
		// every `list --installed`. Add it if someone asks for it.
		fmt.Printf("  Package:     %s (follows stable releases)\n", version)
	}
	fmt.Printf("  Installed:   %s\n", state.InstalledAt)
	fmt.Printf("  Files:       %d\n", len(state.Files))
	fmt.Println()

	for _, p := range modifiedPaths {
		fmt.Printf("  %s %s (modified locally)\n", yellow("~"), p)
	}

	foreignSources, foreignCounts := foreignFileCounts(state)
	for _, fs := range foreignSources {
		fmt.Printf("  %s %d file(s) from %s (`sync` leaves these alone — re-add to update)\n",
			dim("↗"), foreignCounts[fs], fs)
	}

	statusLine := fmt.Sprintf("\n  %s %d ok, %s %d modified, %s %d missing",
		green("✓"), ok, yellow("~"), modified, red("✗"), missing)
	if autoIgnored > 0 {
		statusLine += fmt.Sprintf(", %s %d ignored", dim("⊘"), autoIgnored)
	}
	if excluded > 0 {
		statusLine += fmt.Sprintf(", %s %d excluded", dim("⊘"), excluded)
	}
	fmt.Println(statusLine)
	if excluded > 0 {
		fmt.Printf("  %s Use %s to manage excluded items\n", dim("→"), bold("nav-pilot ignore <type> <name> --user"))
	}
}

// removePinnedRevisions removes every materialized revision of a pinned
// install, printing one line each, and returns how many it removed — or, on a
// dry run, how many it would remove.
//
// It reports rather than deletes silently because a pin tracks no files: these
// trees are the whole of what uninstall removes, and a dry run that lists an
// empty file loop and stops describes a command that does nothing.
func removePinnedRevisions(repo string, dryRun bool) int {
	dir := pakkeSourceDir(repo)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	mark := red("×")
	if dryRun {
		mark = dim("×")
	}
	// The whole directory goes either way, but only revisions are listed and
	// counted: nav-pilot's own bookkeeping beside them is not something a user
	// asked to have removed, and reporting it as a revision would make the
	// count disagree with the list above it.
	var revisions int
	for _, e := range entries {
		if !isRevisionName(e.Name()) {
			continue
		}
		revisions++
		fmt.Printf("  %s %s\n", mark, filepath.Join(dir, e.Name()))
	}
	if !dryRun {
		if err := os.RemoveAll(dir); err != nil {
			fmt.Printf("  %s Could not remove %s: %v\n", yellow("⚠"), dir, err)
		}
	}
	return revisions
}

// cmdUninstall removes an installed collection. force removes files that differ
// from what nav-pilot installed too; without it those are left in place and
// named, because uninstall must not be how a developer loses an edit (#729).
// uninstallAsk is set by run() for an uninstall command line without --yes.
// Functions called directly, as the tests do, are not asked.
var uninstallAsk bool

// askUninstall is the question uninstall asks in a terminal. Overridable in
// tests.
var askUninstall = func(title string) (bool, error) {
	ok := false
	err := huh.NewConfirm().Title(title).Affirmative("Remove").Negative("Cancel").
		Value(&ok).WithTheme(navTheme()).Run()
	return ok, err
}

func cmdUninstall(scope *InstallScope, dryRun, force bool) error {
	state, err := readScopedState(scope)
	if err != nil {
		return fmt.Errorf("reading state: %w", err)
	}

	// Before anything else is removed, and before the "nothing installed" exit
	// (#858, invariant 6; #861 review). Two reasons for both halves of that:
	//
	//   - First, so uninstall cannot report success with a waiver still on
	//     disk. It used to run last and only warn, so a failed write left an
	//     approved record behind, the retry found no state to uninstall, and a
	//     later reinstall of the same revision picked the old approval up again.
	//   - Before the exit, so an orphaned record is reachable at all. An install
	//     records the answer before it writes the scope's state file, so a
	//     failed state write leaves a record this command would otherwise never
	//     look for.
	//
	// The launch derives its flags from the record alone, so removing it is the
	// whole of the removal — there is no cplt configuration to undo.
	// In a terminal it lists everything first and asks (#10). The list is the
	// dry run, so what is asked about is exactly what is then removed.
	if !dryRun && uninstallAsk && isInteractive() && state != nil {
		fmt.Println(bold(fmt.Sprintf("Uninstall %s from %s", state.Collection, uninstallScopeLabel(scope))))
		fmt.Println()
		n, kept := uninstallItems(scope, state, true, false, force)
		fmt.Println()
		if kept > 0 {
			fmt.Printf("%s %d file(s) changed since nav-pilot installed them and stay. %s removes them too.\n\n",
				yellow("⚠"), kept, bold("nav-pilot uninstall --force"))
		}
		ok, err := askUninstall(fmt.Sprintf("Remove these %d items?", n))
		if err != nil && !errors.Is(err, huh.ErrUserAborted) {
			return fmt.Errorf("could not ask: %w\n\n  Uninstall without asking:  %s", err, bold("nav-pilot uninstall --yes"))
		}
		if err != nil || !ok {
			fmt.Println(dim("Cancelled. Nothing was removed."))
			return nil
		}
		uninstallQuiet = true
		defer func() { uninstallQuiet = false }()
	}

	if !dryRun {
		removedConsent, err := forgetProposalConsent(scope)
		if err != nil {
			return fmt.Errorf("removing the sandbox consent record for the %s scope: %w\n\nNothing was uninstalled: a waiver that outlives its agentpakke is exactly what this record must not do", scope.Name, err)
		}
		if removedConsent > 0 {
			fmt.Printf("%s Removed %d sandbox consent record(s).\n", green("✓"), removedConsent)
		}
	}

	if state == nil {
		fmt.Println("nav-pilot is not installed. Nothing else to uninstall.")
		return nil
	}

	if !uninstallQuiet {
		if dryRun {
			fmt.Println(bold(fmt.Sprintf("Dry run: would uninstall %s from %s", state.Collection, uninstallScopeLabel(scope))))
		} else {
			fmt.Println(bold(fmt.Sprintf("Uninstalling %s from %s", state.Collection, uninstallScopeLabel(scope))))
		}
		fmt.Println()
	}

	removed, kept := uninstallItems(scope, state, dryRun, uninstallQuiet, force)

	if !dryRun {
		os.Remove(scope.StatePath())
		removeDeclarationFor(scope, state)
		scope.CleanupDirs()
	}

	fmt.Println()
	if dryRun {
		fmt.Printf("%s Would remove %d items.\n", dim("→"), removed)
	} else {
		fmt.Printf("%s Removed %d items.\n", green("✓"), removed)
	}
	// Before the removal, --force is the way to take these too. After it the
	// state that named them is gone, and `uninstall --force` would only say
	// nothing is installed, so the hint there is to delete them by hand.
	if kept > 0 && dryRun {
		fmt.Printf("%s %d file(s) differ from what nav-pilot installed and would be left in place. %s removes them too.\n",
			yellow("⚠"), kept, bold("nav-pilot uninstall --force"))
	} else if kept > 0 {
		fmt.Printf("%s %d file(s) differ from what nav-pilot installed and were left in place (marked ⊘ above). They are yours now: delete them yourself if you do not want them.\n",
			yellow("⚠"), kept)
	}
	return nil
}

// uninstallQuiet is set once uninstall has listed and been confirmed, so the
// removal does not print the same list a second time.
var uninstallQuiet bool

// uninstallItems removes (or with dryRun lists) everything an uninstall takes:
// the tracked files, nav-pilot's hook entries, a pin's revisions, and the two
// records, state file and lock file, which were removed without being named.
func uninstallItems(scope *InstallScope, state *StateFile, dryRun, quiet, force bool) (removed, kept int) {
	removed, kept = removeStateFiles(scope, state, dryRun, quiet, force)
	removed += deactivateRepoHooks(scope, dryRun, quiet)

	// A pinned Tier 2 install keeps everything it materialized outside the
	// scope, so the file loop above removed nothing and the revisions are the
	// only thing this command actually deletes — which is exactly why the dry
	// run has to name them too.
	//
	// Only a user-scope pin has revisions: [pinRevision] writes nowhere else,
	// and the state shape it writes is what [pinnedState] recognizes. A
	// repo-scope or Tier 1 state that happens to track no files is not a pin
	// and must not take the user's revisions with it.
	if scope.IsUser() && pinnedState(state) {
		removed += removePinnedRevisions(state.SourceRepo, dryRun)
	}
	if !quiet {
		mark := red("×")
		if dryRun {
			mark = dim("×")
		}
		fmt.Printf("  %s %s\n", mark, scopePath(scope, scope.StatePath()))
		if declarationGoesWith(scope, state) {
			fmt.Printf("  %s %s\n", mark, agentpakke.DeclarationPath)
		}
	}
	// Counted as listed, so the question and the summary give the number
	// the list shows.
	removed++
	if declarationGoesWith(scope, state) {
		removed++
	}
	return removed, kept
}

// uninstallScopeLabel says which scope an uninstall covers, and where the
// other one is: the two are separate, and uninstall in a repo left the user
// scope's hooks running without a word (#10).
func uninstallScopeLabel(scope *InstallScope) string {
	if scope.IsUser() {
		return "~/.copilot (user scope; a repository's install is removed in that repository)"
	}
	return scope.RootDir + " (this repository only; ~/.copilot is nav-pilot uninstall --user)"
}

// safeToRemove is the one predicate every deletion path asks before removing a
// tracked file: nav-pilot may take back what it wrote, and nothing else.
//
// It wraps [navPilotOwns] with the case that predicate has no answer for. An
// entry with no recorded hash predates hashing, so there is nothing to compare
// and the state's word that nav-pilot installed the file is all the evidence
// there is or ever will be. Refusing those would make uninstall a no-op in
// every repo that has not reinstalled since, which is a worse failure than the
// one the guard exists to prevent: the user asked for the removal, and no edit
// is being claimed. A recorded conflict is different. That status is positive
// evidence the file is the user's, so it is honoured whether or not a hash came
// with it.
//
// Until the review of #729 this predicate guarded [removeOrphans] alone. Sync's
// delete path and [removeStateFiles] removed whatever the state named, edited
// or not.
//
// [removeOrphans] stays on the stricter [navPilotOwns]: it runs inside an
// install, which is not a removal anyone asked for, so a hashless entry there
// keeps the benefit of the doubt it has always had.
func safeToRemove(rootDir string, f InstalledFile) bool {
	if f.Hash == "" && f.Status != fileStatusConflict {
		return true
	}
	return navPilotOwns(rootDir, f)
}

// sourceSwitch names the agentpakke a scope is installed from when this install
// replaces it with a different one, and is empty otherwise: a first install, a
// state written before source tracking, or the same agentpakke again.
//
// The two cases must not share a message. A file that goes because the scope
// changed agentpakke was not retired upstream — it is still shipped, by the
// pakke this scope no longer installs from — and saying "no longer in the
// collection" for it is how someone loses another team's agents and only
// works out why afterwards.
func sourceSwitch(prior *StateFile, newRepo string) string {
	if prior == nil || prior.SourceRepo == "" || newRepo == "" || sameSourceRepo(prior.SourceRepo, newRepo) {
		return ""
	}
	return prior.SourceRepo
}

// reinstallCommand is the command that puts a scope's previous agentpakke back,
// with the source filled in. Printed after a switch, because the user has just
// been told their content was removed and the way back is not obvious.
//
// An à-la-carte state records no installable name, only a label, so it gets
// the placeholder the other refusals print rather than a command that fails.
func reinstallCommand(scope *InstallScope, prior *StateFile) string {
	target := prior.Collection
	switch {
	case target == "" || target == CollectionAll:
		target = "--all"
	case !agentpakke.IsIdentifier(target):
		target = "<name>"
	}
	flag := "--repo"
	if scope.IsUser() {
		flag = "--user"
	}
	return fmt.Sprintf("nav-pilot install %s --source %s %s", target, prior.SourceRepo, flag)
}

// askSwitch puts the switch question to the user. A var so a test can answer
// it, like the other prompts the install path owns.
//
// The bool it is handed is the default, and it is true: the user named the new
// source on the command line, so Enter means "yes, go ahead".
var askSwitch = func(title string, proceed *bool) error {
	return huh.NewConfirm().
		Title(title).
		Value(proceed).
		WithTheme(navTheme()).
		Run()
}

// installForce carries install's --force for one invocation, the way
// installFrozen and installRef do. The `force` the install functions take is
// not the flag: every picker path and `install --all` set it themselves on a
// re-install so managed files get refreshed (#814, #820), which is exactly
// when a scope has something to lose. Only the flag the user typed may skip
// the switch prompt.
var installForce bool

// confirmSourceSwitch says what an install from a different agentpakke is about
// to remove, and asks first when there is a terminal to ask. It reports whether
// the install should go ahead.
//
// It runs before anything is written: answering "no" after the new content has
// landed would leave the scope holding both agentpakker, which is exactly the
// mix [guardScopeSource] exists to refuse.
//
// Without a terminal it announces and proceeds. Scripts, CI and our own
// harnesses install this way, and a refusal path there would break them; what
// they get instead is the announcement on stdout.
func confirmSourceSwitch(scope *InstallScope, src *Source, dryRun, jsonOutput bool) bool {
	if dryRun || jsonOutput || src == nil {
		return true
	}
	prior, _ := readScopedState(scope)
	from := sourceSwitch(prior, src.Repo)
	if from == "" {
		return true
	}
	// What removeOrphans would remove: a file nav-pilot no longer owns stays
	// whatever happens, and an entry with nothing on disk (an ignored item, a
	// file already gone) is not a removal. "Up to", because a path the new
	// agentpakke also ships is overwritten rather than removed; the summary
	// after the install prints the number that actually went.
	owned := 0
	for _, f := range prior.Files {
		if onDisk(scope, f) && navPilotOwns(scope.RootDir, f) {
			owned++
		}
	}
	fmt.Printf("%s The %s scope is installed from %s. This install switches it to %s.\n",
		yellow("⚠"), scope.Name, bold(from), bold(src.Repo))
	if owned > 0 {
		fmt.Printf("  Up to %d file(s) from %s will be removed.\n", owned, from)
	}
	if installForce || !isInteractive() {
		return true
	}
	proceed := true
	err := askSwitch(fmt.Sprintf("Switch the %s scope to %s?", scope.Name, src.Repo), &proceed)
	if err == nil && proceed {
		return true
	}
	// Ctrl-C, or a terminal that could not show the question. Either way no
	// one said yes to losing files, so nothing is lost.
	if err != nil && !errors.Is(err, huh.ErrUserAborted) {
		fmt.Printf("%s Could not ask: %v. Pass %s to switch without asking.\n", yellow("⚠"), err, bold("--force"))
	}
	fmt.Println(dim("Cancelled."))
	return false
}

// onDisk reports whether a state entry has anything behind it to remove.
func onDisk(scope *InstallScope, f InstalledFile) bool {
	_, err := os.Stat(filepath.Join(scope.RootDir, f.Path))
	return err == nil
}

// removeOrphans deletes files the previous install put on disk that this one
// did not write.
//
// State is rebuilt from what the install produced, so a file the source has
// stopped shipping simply falls out of it. Nothing then deletes it, and nothing
// can: sync walks the state, so a path it no longer names is invisible to every
// later run. The file stays installed for the life of the machine.
//
// Reported after the local worker agent was renamed from lokal-arbeider to
// local-worker. Both were listed in Copilot CLI's agent picker for three days,
// and the stale one showed "inherit (default behavior)" — a worker agent that
// would have run on the cloud model, which is the one thing it exists not to do.
//
// A file is removed only when it is byte-for-byte what nav-pilot wrote.
// Anything the developer edited is theirs and stays, which is the same rule the
// opencode scope has applied since it was written.
//
// It returns the prior entries it did not delete. Those are the ones whose state
// must survive the install — deleting a file and keeping its state entry would
// have status report it missing and the next sync report a deletion nav-pilot
// performed itself.
func removeOrphans(scope *InstallScope, prior *StateFile, installed []InstalledFile, newRepo string) ([]InstalledFile, []string) {
	if prior == nil {
		return nil, nil
	}
	switched := sourceSwitch(prior, newRepo)
	var removed []string
	written := make(map[string]bool, len(installed))
	for _, f := range installed {
		written[f.Path] = true
	}
	kept := make([]InstalledFile, 0, len(prior.Files))
	for _, f := range prior.Files {
		if written[f.Path] || !navPilotOwns(scope.RootDir, f) {
			kept = append(kept, f)
			continue
		}
		full := filepath.Join(scope.RootDir, f.Path)
		var err error
		if strings.HasSuffix(f.Path, "/") {
			err = removeAllButOrig(full)
		} else {
			err = os.Remove(full)
		}
		if os.IsNotExist(err) {
			// An ignored item, or a file already gone: nothing was removed,
			// and an entry for it belongs to the install being replaced.
			continue
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s Could not remove %s, which is no longer part of the install: %v\n",
				yellow("⚠"), f.Path, err)
			kept = append(kept, f)
			continue
		}
		// A hook's script went, so what registers it goes too; a skill's
		// directory, likewise. The same cleanup as every other removal path.
		afterArtifactRemoved(scope, full, switched != "")
		removed = append(removed, f.Path)
		if switched == "" {
			fmt.Printf("  %s %s %s\n", red("×"), f.Path, dim("(no longer part of the install)"))
		}
	}
	// One line for a switch, not one per file: sixty-three of them said the
	// same wrong thing sixty-three times. The full list is in --json.
	if switched != "" && len(removed) > 0 {
		fmt.Printf("  %s Removed %d file(s) from %s.\n", red("×"), len(removed), bold(switched))
		fmt.Printf("  %s Put %s back with: %s\n", dim("→"), switched, bold(reinstallCommand(scope, prior)))
	}
	return kept, removed
}

// stampRevision records which source revision wrote each file this install
// produced (#729).
//
// Only files with content: an ignored entry has no bytes and therefore no
// provenance, and stamping one would claim a revision put something on disk
// that it never did.
func stampRevision(files []InstalledFile, revision string, inherited func(string) bool) []InstalledFile {
	if revision == "" {
		return files
	}
	for i := range files {
		if files[i].Hash == "" {
			continue
		}
		// A file that came from a reused pakke did not come from this
		// revision, and stamping it anyway made sync's "installed from <sha>"
		// hint name a revision the file was never in. Leaving it empty says
		// nothing extra, which is what the field is documented to do when it
		// does not know (#729).
		if inherited != nil && inherited(files[i].Path) {
			continue
		}
		files[i].Revision = revision
	}
	return files
}

// inheritedPaths reports which installed paths came from a reused pakke rather
// than from this source. Nil when nothing is reused, which is nearly always.
func inheritedPaths(resolver *SourceResolver, scope *InstallScope, reused *Source) func(string) bool {
	if reused == nil || resolver == nil {
		return nil
	}
	return func(localPath string) bool {
		rel := resolver.MapLocalPath(localPath, scope.IsUser())
		root, ok := resolver.SourceRootFor(rel)
		return ok && root != resolver.SourceDir()
	}
}

// installedPrimaryAgent names the persona a user should reach for after an
// install: the first primaryAgents entry the source's manifest declares for
// Copilot, or nav-pilot when the source declares none.
//
// A manifest-less source, and Nav's own agentpakke, both answer nav-pilot, so
// the ordinary message is unchanged.
func installedPrimaryAgent(src *Source) string {
	const fallback = "nav-pilot"
	if src == nil || src.Pakke == nil {
		return fallback
	}
	agents := src.Pakke.PrimaryAgents("copilot")
	if len(agents) == 0 {
		return fallback
	}
	return agents[0]
}

// mcpServerNotice is what an install says about the MCP servers a pakke
// declares: which ones its content expects, and where to enable them.
//
// nav-pilot writes no MCP configuration for any client, so this is the honest
// whole of what it can do — the pakke names governed servers, and turning one
// on stays the user's action in their own client config. A pakke that declares
// none gets no lines, so nothing new appears for the installs that exist today.
func mcpServerNotice(src *Source) []string {
	if src == nil || src.Pakke == nil || len(src.Pakke.MCPServers) == 0 {
		return nil
	}
	lines := []string{"This agentpakke expects these MCP servers:"}
	for _, name := range src.Pakke.MCPServers {
		lines = append(lines, "  "+name)
	}
	return append(lines,
		"nav-pilot does not configure MCP. Enable them in your client: "+agentpakke.MCPRegistryURL)
}

// printMCPServerNotice prints [mcpServerNotice] after an install, or nothing.
func printMCPServerNotice(src *Source) {
	lines := mcpServerNotice(src)
	if len(lines) == 0 {
		return
	}
	fmt.Println()
	for _, line := range lines {
		fmt.Println(dim(line))
	}
}

// kindLabel is the plural heading an install prints for a kind: "Agents",
// "Skills", "Extensions".
func kindLabel(kind *ArtifactKind) string {
	if kind.Dir == "" {
		return kind.Name
	}
	return strings.ToUpper(kind.Dir[:1]) + kind.Dir[1:]
}

// installConsentRequired is set by run() for an install command line that has
// not already said yes: no --yes, no --all with a scope flag, no --frozen.
// Functions called directly, as the tests and the pickers do, are not asked.
var installConsentRequired bool

// askHooks is the question a user-scope install asks before it writes hooks.
// Overridable in tests.
var askHooks = func(title, description string) (bool, error) {
	ok := true
	err := huh.NewConfirm().Title(title).Description(description).
		Affirmative("Install").Negative("Cancel").Value(&ok).WithTheme(navTheme()).Run()
	return ok, err
}

// confirmInstallWrites is the consent an install needs before it writes
// anything, asked once for every way into it.
//
// Without a terminal nobody can be asked, and "install nav-pilot" in a script
// wrote 152 files and three hooks that run outside the sandbox on every
// matching tool call. So it refuses, the way `alpha local init` does, and says
// what it would have done. In a terminal, a user-scope install that brings
// hooks asks about them, whichever command got there: the picker behind
// `install --user` always did, the scope picker behind `install <name>` did
// not.
func confirmInstallWrites(scope *InstallScope, resolver *SourceResolver, manifest *Manifest, dryRun, jsonOutput bool) error {
	if dryRun || !installConsentRequired {
		return nil
	}
	files := 0
	for _, kind := range AllKinds {
		names, _ := manifest.NamesByKind(kind)
		if !scope.SupportsType(kind.Name) {
			continue
		}
		for _, name := range names {
			if art, ok := resolver.Get(kind, name); ok && art.IsDir {
				files += countDirFiles(art.AbsPath)
			} else if ok {
				files++
			}
		}
	}
	hooks := len(manifest.Hooks)
	if !isInteractive() || jsonOutput {
		what := fmt.Sprintf("%d files to %s", files, scope.Label())
		if hooks > 0 {
			what += fmt.Sprintf(", including %d hooks that run outside the sandbox", hooks)
		}
		return &exitCode{code: 2, err: fmt.Errorf("install would write %s. Run it in a terminal, or pass --yes", what)}
	}
	if !scope.IsUser() || hooks == 0 {
		return nil
	}
	ok, err := askHooks(
		fmt.Sprintf("Install %d files and %d hooks to ~/.copilot?", files, hooks),
		fmt.Sprintf("Hooks run outside the sandbox on every matching tool call: %s", strings.Join(manifest.Hooks, ", ")))
	if err != nil && !errors.Is(err, huh.ErrUserAborted) {
		return fmt.Errorf("could not ask about the hooks: %w\n\n  Install without asking:  %s", err, bold("nav-pilot install <name> --user --yes"))
	}
	if err != nil || !ok {
		fmt.Println(dim("Cancelled."))
		return errInstallCancelled
	}
	return nil
}

// dirSlash is the trailing slash that marks a directory in a listed path.
func dirSlash(isDir bool) string {
	if isDir {
		return "/"
	}
	return ""
}

// hookRegistrationPath names what makes a hook run, as a dry run lists it:
// the shared repo config with the hook's entry in it, or the hook's own file
// in ~/.copilot/hooks.
func hookRegistrationPath(scope *InstallScope, name string) string {
	if scope.IsUser() {
		return scopePath(scope, scope.DstPath(KindHook.Dir, source.UserHookConfigName(name)))
	}
	return scopePath(scope, scope.DstPath(KindHook.Dir, source.RepoHooksConfig)) + " (entry for " + name + ")"
}

// printDryRunRecords lists the bookkeeping an install writes beside the
// artifacts: the state file always, and in a repo the lock file that pins the
// revision (not under --frozen, which never writes it).
func printDryRunRecords(scope *InstallScope, lock bool) {
	fmt.Println(bold("Records:"))
	fmt.Printf("  %s %s\n", dim("→"), scopePath(scope, scope.StatePath()))
	if lock && !scope.IsUser() {
		fmt.Printf("  %s %s\n", dim("→"), agentpakke.DeclarationPath)
	}
	fmt.Println()
}
