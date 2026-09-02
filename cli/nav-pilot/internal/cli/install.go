package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/navikt/copilot/cli/nav-pilot/internal/agentpakke"
)

type installResult struct {
	Installed int
	// Missing names every artifact the manifest listed and the source does not
	// ship. installArtifact warns and moves on, so the count is the third way
	// an install lands incomplete — beside a conflict and an unsupported kind.
	Missing     []string
	Conflicts   int
	Unsupported []string
	Files       []InstalledFile
}

// installItems installs every artifact a collection manifest names, reading
// content through the resolver the active agentpakke manifest produced.
func installItems(resolver *SourceResolver, scope *InstallScope, manifest *Manifest, dryRun, force bool) (*installResult, error) {
	result := &installResult{}
	stateHashes := scopeStateHashes(scope)

	for _, group := range []struct {
		label string
		names []string
		kind  *ArtifactKind
	}{
		{"Agents", manifest.Agents, KindAgent},
		{"Skills", manifest.Skills, KindSkill},
		{"Instructions", manifest.Instructions, KindInstruction},
		{"Prompts", manifest.Prompts, KindPrompt},
	} {
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

	return result, nil
}

// pakkeContents lists everything an agentpakke's layout ships, as the content
// manifest the installer already knows how to walk. It is the manifest-bearing
// counterpart to loadManifest: the agentpakke manifest supersedes
// collections/<name>/manifest.json rather than declaring entries in it.
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
	total := len(manifest.Agents) + len(manifest.Skills) + len(manifest.Instructions) + len(manifest.Prompts)
	if total == 0 && pakke.Layout != nil {
		return nil, fmt.Errorf("agentpakke %q declares a layout but ships no agents, skills, instructions, or prompts.\n"+
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
func scopeStateHashes(scope *InstallScope) map[string]string {
	hashes := map[string]string{}
	state, err := readScopedState(scope)
	if err != nil || state == nil {
		return hashes
	}
	for _, f := range state.Files {
		if f.Status != fileStatusConflict && f.Status != fileStatusIgnored && f.Hash != "" {
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
		fmt.Printf("  %s %s not found: %s\n", yellow("⚠"), titleCase(kind.Name), name)
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
	// Only an untracked path falls back to the byte comparison, where a
	// hand-placed or foreign file genuinely is one.
	conflicted := false
	if storedHash, tracked := stateHashes[relPath]; tracked {
		// An absent or unreadable file is not the user's edit either, so it is
		// not a conflict — it is simply reinstalled.
		current, hashErr := rawArtifactHash(dst, art.IsDir)
		conflicted = hashErr == nil && current != storedHash
	} else {
		c, err := checkConflict(dst, art.AbsPath, art.IsDir)
		if err != nil {
			return err
		}
		conflicted = c != nil
	}

	if conflicted && !force {
		// The file exists and the user changed it — skip the overwrite but
		// track as conflict so it's not lost from state or reported as "new".
		fmt.Printf("  %s %s (locally modified — kept; %s takes the upstream version)\n",
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
		fmt.Printf("  %s %s%s\n", dim("→"), relPath, extra)
		result.Installed++
		return nil
	}

	if err := copyArtifact(art.AbsPath, dst, scope.RootDir, art.IsDir); err != nil {
		return fmt.Errorf("copying %s %s: %w", kind.Name, name, err)
	}
	hash, err := rawArtifactHash(dst, art.IsDir)
	if err != nil {
		return fmt.Errorf("hashing installed %s %s: %w", kind.Name, name, err)
	}
	result.Files = append(result.Files, InstalledFile{Path: relPath, Hash: hash})

	fmt.Printf("  %s %s\n", green("✓"), name)
	result.Installed++

	return nil
}

// printConflictHint says what a conflict is and how it ends. "Skipped" on its
// own reads as permanent, and --force reads as "throw my edits away": neither
// says that a sync --apply takes the upstream version of exactly these files.
func printConflictHint(n int) {
	fmt.Printf("%s %d file(s) kept as you edited them.\n", yellow("⚠"), n)
	fmt.Printf("  %s to take the upstream version, or %s to overwrite on the next install.\n",
		bold("nav-pilot sync --apply"), bold("--force"))
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
			return fmt.Errorf("unknown type %q. Valid types: agent, skill, instruction, prompt", itemType)
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

	// Check if name matches any artifact
	resolver := resolverFor(src.Dir, pakkeFor(src, name))
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
		return cmdAddFromSource(matchedKinds[0].Name, name, src, scope, sourceRepo, dryRun, force, jsonOutput)
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
		return fmt.Errorf("%q not found. Did you mean %q?\n\nRun 'nav-pilot list' to see available collections and items", name, s)
	}
	return fmt.Errorf("%q not found. Run 'nav-pilot list' to see available collections and items", name)
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
	pakke := pakkeFor(src, collection)
	resolver := resolverFor(src.Dir, pakke)

	// The repo's committed declaration may narrow the install to named items.
	// It is read before anything is written, so a list naming something the
	// agentpakke does not ship refuses the whole install rather than half of it.
	decl, err := scopeDeclaration(scope)
	if err != nil {
		return err
	}
	var declaredItems map[string]string
	if decl != nil {
		declaredItems = decl.Items
	}
	if err := guardDeclaredItems(src, declaredItems); err != nil {
		return err
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

	// Fail closed before touching the filesystem (A3): a non-conforming
	// agentpakke must not leave a partial install behind.
	if err := validatePakkeSource(src); err != nil {
		return err
	}

	var manifest *Manifest
	if src.Pakke != nil {
		manifest, err = pakkeContents(resolver, src)
	} else {
		manifest, err = loadManifest(src.Dir, collection)
	}
	if err != nil {
		return err
	}
	if manifest, err = applyDeclaredItems(manifest, declaredItems); err != nil {
		return err
	}

	sourceLabel := sourceLabelFor(src)

	if !jsonOutput {
		fmt.Println()
		if dryRun {
			fmt.Println(bold(fmt.Sprintf("Dry run: %s", collection)))
		} else {
			fmt.Println(bold(fmt.Sprintf("Installing: %s", collection)))
		}
		fmt.Printf("%s %s\n", dim("Source:"), dim(fmt.Sprintf("%s@%s", sourceLabel, shortSHA(src.SHA))))
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

	if jsonOutput {
		return outputJSON(map[string]interface{}{
			"command":     "install",
			"collection":  stateCollection(src, collection),
			"scope":       scope.Name,
			"source_sha":  src.SHA,
			"version":     src.Version,
			"installed":   result.Installed,
			"skipped":     len(result.Missing),
			"conflicts":   result.Conflicts,
			"unsupported": result.Unsupported,
			"dry_run":     dryRun,
		})
	}

	if result.Conflicts > 0 {
		printConflictHint(result.Conflicts)
	}

	if len(result.Unsupported) > 0 {
		fmt.Printf("%s Skipped (not supported in %s scope): %s\n",
			yellow("⚠"), scope.Name, strings.Join(result.Unsupported, ", "))
	}

	if dryRun {
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
		Files:       result.Files,
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
	if prior, err := readScopedState(scope); err == nil && prior != nil {
		state.PreserveUnknownFrom(prior)
		state.Files = mergeStateFiles(removeOrphans(scope, prior, result.Files), state.Files)
	}
	if err := writeScopedState(scope, state); err != nil {
		fmt.Fprintf(os.Stderr, "%s Could not write state file: %v\n", yellow("⚠"), err)
	}
	// --frozen never moves the pin, not even to rewrite it as the value it
	// already holds: a CI job must not produce a diff in a file it was only
	// meant to obey.
	if !installFrozen {
		recordDeclaration(scope, src)
	}

	fmt.Printf("%s Installed %d items from %q (v%s, %s).\n",
		green("✓"), result.Installed, collection, stateVersion, shortSHA(src.SHA))
	fmt.Println()
	if scope.IsUser() {
		fmt.Println(dim("Agents and skills are now available across all your repos."))
		fmt.Println(dim("Use @nav-pilot in Copilot Chat or copilot --agent nav-pilot"))
	} else {
		fmt.Println(dim("Next steps:"))
		fmt.Println(dim("  1. Review the installed files in .github/"))
		fmt.Println(dim("  2. Commit and push to enable Copilot customization"))
		fmt.Println(dim("  3. Use @nav-pilot in Copilot to start planning"))
	}

	return nil
}

// cmdAddFromSource installs a single artifact from an already-resolved source.
// It preserves the à-la-carte state semantics from cmdAdd.
func cmdAddFromSource(itemType, name string, src *Source, scope *InstallScope, explicitSource string, dryRun, force bool, jsonOutput bool) error {
	if !scope.SupportsType(itemType) {
		return fmt.Errorf("type %q is not supported in user scope. Only agents, skills, and instructions can be installed to ~/.copilot", itemType)
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
	resolver := resolverFor(src.Dir, pakkeFor(src, name))
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
func cmdList(scope *InstallScope, ref, sourceRepo string, showItems bool, jsonOutput bool) error {
	if !jsonOutput {
		fmt.Println(dim("Resolving source..."))
	}
	src, err := resolveDeclaredSource(scope, ref, sourceRepo)
	if err != nil {
		return err
	}
	defer src.Cleanup()

	resolver := resolverFor(src.Dir, pakkeFor(src, ""))

	// A source that ships an agentpakke manifest offers exactly one installable
	// name — the agentpakke itself — because the manifest supersedes the
	// collections/<name> model.
	type collectionInfo struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Items       int    `json:"items"`
		agents      []string
	}
	var collections []collectionInfo
	add := func(m *Manifest) {
		collections = append(collections, collectionInfo{
			Name:        m.Name,
			Description: m.Description,
			Items:       len(m.Agents) + len(m.Skills) + len(m.Instructions) + len(m.Prompts),
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
	if src.Pakke != nil {
		fmt.Println(bold(fmt.Sprintf("Agentpakke in %s:", sourceLabelFor(src))))
	} else {
		fmt.Println(bold("Available collections:"))
	}
	fmt.Println()
	for _, c := range collections {
		fmt.Printf("  %-20s %s %s\n", bold(c.Name), c.Description, dim(fmt.Sprintf("(%d items)", c.Items)))
		if len(c.agents) > 0 {
			fmt.Printf("  %-20s %s\n", "", dim("agents: "+strings.Join(c.agents, ", ")))
		}
		// A Tier 2 agentpakke lists 0 items because it materializes no
		// user-visible files; what it ships is payload contexts.
		//
		// Every payload-bearing client is listed, launchable by this binary or
		// not, because an install materializes every one of them (a filter
		// would be a second client list to keep in step with the launch
		// switch). A listing that hid the ones this binary cannot launch would
		// hide content that is on disk; saying so is the useful half.
		//
		// What "launchable" means comes from [stagedLaunchers], the map the
		// launch itself dispatches on. It is not the same set as
		// agentpakke.IsKnownClient, which is every client id nav-pilot knows —
		// a known client with no staged launcher is exactly the case this
		// annotation is for.
		if payloadOnly(src) {
			for _, client := range src.Pakke.ClientIDs() {
				ctxs := declaredContexts(src.Pakke, client)
				if len(ctxs) == 0 {
					continue
				}
				line := client + " payloads: " + strings.Join(ctxs, ", ")
				if _, ok := stagedLaunchers[client]; !ok {
					line += " (materialized on install; this nav-pilot cannot launch " + client + ")"
				}
				fmt.Printf("  %-20s %s\n", "", dim(line))
			}
		}
	}
	fmt.Println()
	fmt.Printf("Install with: %s\n", bold("nav-pilot install <name>"))
	fmt.Printf("Install everything to user home: %s\n", bold("nav-pilot install --user --all"))

	if showItems {
		fmt.Println()
		if err := listAvailableItems(resolver); err != nil {
			return err
		}
	} else {
		fmt.Printf("Show individual items: %s\n", bold("nav-pilot list --items"))
	}
	return nil
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
			fmt.Printf("  %-30s %s\n", item.Name, dim("nav-pilot install "+item.Name))
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
func cmdInstallInteractive(targetDir, ref, sourceRepo string) error {
	// The scope is settled before the source is resolved, because the repo's
	// committed declaration is *part of* how the source is chosen: resolving
	// first would install whatever the config key happened to name and then
	// overwrite the declaration with it — for the one command, `nav-pilot
	// install` with no arguments, the declaration exists to serve.
	scope, err := ScopeUser()
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
	return interactiveRepoInstall(src, scope, sourceRepo)
}

// cmdInstallAll installs all agents and skills to user scope by scanning the source.
// Used when `nav-pilot install --user` is run without a collection name.
// When interactive, offers the same picker as the root `nav-pilot` command.
func cmdInstallAll(scope *InstallScope, ref, sourceRepo string, dryRun, force bool, jsonOutput bool) error {
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

	// Interactive mode: offer the picker (same UX as `nav-pilot` root command)
	if isInteractive() && !dryRun && !jsonOutput {
		return interactiveUserInstallFromSource(scope, src, sourceRepo)
	}

	return installAllFromSource(scope, src, nil, dryRun, force, jsonOutput)
}

// installAllFromSource installs all agents+skills from source.
// If manifest is nil, it scans the source directory to discover items.
// extraStateFiles are appended to the state file after install (e.g. ignored items from picker).
// Extracted so both cmdInstallAll and the interactive flow can share this.
func installAllFromSource(scope *InstallScope, src *Source, manifest *Manifest, dryRun, force bool, jsonOutput bool, extraStateFiles ...InstalledFile) error {
	pakke := pakkeFor(src, CollectionAll)
	resolver := resolverFor(src.Dir, pakke)

	if payloadOnly(src) {
		return installPakkePin(scope, src, dryRun, jsonOutput)
	}

	// Fail closed before touching the filesystem (A3).
	if err := validatePakkeSource(src); err != nil {
		return err
	}

	if manifest == nil {
		var err error
		manifest, err = collectAllItemsWith(resolver)
		if err != nil {
			return err
		}
	}

	total := len(manifest.Agents) + len(manifest.Skills) + len(manifest.Instructions)
	if total == 0 {
		return fmt.Errorf("no agents, skills, or instructions found in source")
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

	if jsonOutput {
		return outputJSON(map[string]interface{}{
			"command":    "install",
			"collection": stateCollection(src, CollectionAll),
			"scope":      scope.Name,
			"source_sha": src.SHA,
			"version":    src.Version,
			"installed":  result.Installed,
			"conflicts":  result.Conflicts,
			"dry_run":    dryRun,
		})
	}

	if dryRun {
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
		Files:       result.Files,
	}

	// Append items the user explicitly deselected in the picker as ignored.
	if len(extraStateFiles) > 0 {
		state.Files = append(state.Files, extraStateFiles...)
	}

	// Same as cmdInstallFromSource: a rebuild over an existing state keeps the
	// keys this binary does not understand (#588), retires the artifacts the
	// source has stopped shipping (#615), and keeps the state of the paths this
	// install did not name. The two install paths must agree about all three.
	if prior, err := readScopedState(scope); err == nil && prior != nil {
		state.PreserveUnknownFrom(prior)
		state.Files = mergeStateFiles(removeOrphans(scope, prior, result.Files), state.Files)
	}

	if err := writeScopedState(scope, state); err != nil {
		fmt.Fprintf(os.Stderr, "%s Could not write state file: %v\n", yellow("⚠"), err)
	}

	fmt.Printf("%s Installed %d items to %s (v%s, %s).\n",
		green("✓"), result.Installed, scope.Label(), stateVersion, shortSHA(src.SHA))
	fmt.Println()
	fmt.Println(dim("Agents and skills are now available across all your repos."))
	fmt.Println(dim("Use @nav-pilot in Copilot Chat or copilot --agent nav-pilot"))

	if len(manifest.Instructions) > 0 && scope.IsUser() {
		fmt.Println()
		fmt.Println(dim("Instructions are available when launching cplt via nav-pilot."))
		fmt.Println(dim("For direct cplt usage, add to your shell profile:"))
		fmt.Printf("  %s\n", dim("eval \"$(nav-pilot env)\""))
	}

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
		fmt.Printf("No nav-pilot collection installed (repo or user scope).\n")
		fmt.Printf("Install with: %s\n", bold("nav-pilot install <collection>"))

		return nil
	}

	if jsonOutput {
		scopes := []map[string]interface{}{}
		if repoState != nil {
			ok, modified, missing, ignored, _ := countFileIntegrity(repoScope.RootDir, repoState)
			scopes = append(scopes, map[string]interface{}{
				"scope": "repo", "collection": repoState.Collection,
				"version": repoState.Version, "source_sha": repoState.SourceSHA,
				"installed_at": repoState.InstalledAt, "files": len(repoState.Files),
				"ok": ok, "modified": modified, "missing": missing, "ignored": ignored,
			})
		}
		if userState != nil {
			ok, modified, missing, ignored, _ := countFileIntegrity(userScope.RootDir, userState)
			scopes = append(scopes, map[string]interface{}{
				"scope": "user", "collection": userState.Collection,
				"version": userState.Version, "source_sha": userState.SourceSHA,
				"installed_at": userState.InstalledAt, "files": len(userState.Files),
				"ok": ok, "modified": modified, "missing": missing, "ignored": ignored,
			})
		}
		for _, p := range allProviders() {
			cs := p.ContextStatus()
			if cs == nil {
				continue
			}
			ok, modified, missing, ignored, _ := countFileIntegrity(cs.OutputDir, cs.State)
			scopes = append(scopes, map[string]interface{}{
				"scope": cs.ScopeName, "collection": cs.State.Collection,
				"version": cs.State.Version, "source_sha": cs.State.SourceSHA,
				"installed_at": cs.State.InstalledAt, "files": len(cs.State.Files),
				"ok": ok, "modified": modified, "missing": missing, "ignored": ignored,
			})
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
			fmt.Println("No nav-pilot collection installed in user home (~/.copilot).")
		} else {
			fmt.Println("No nav-pilot collection installed.")
		}
		fmt.Printf("Install with: %s\n", bold("nav-pilot install <collection>"))
		return nil
	}

	if jsonOutput {
		ok, modified, missing, ignored, _ := countFileIntegrity(scope.RootDir, state)
		return outputJSON(map[string]interface{}{
			"installed":    true,
			"collection":   state.Collection,
			"version":      state.Version,
			"scope":        scope.Name,
			"source_sha":   state.SourceSHA,
			"installed_at": state.InstalledAt,
			"files":        len(state.Files),
			"ok":           ok,
			"modified":     modified,
			"missing":      missing,
			"ignored":      ignored,
		})
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
	fmt.Printf("  Collection:  %s\n", bold(state.Collection))
	fmt.Printf("  Version:     %s\n", state.Version)
	fmt.Printf("  Scope:       %s\n", scope.Name)
	fmt.Printf("  Source:      %s\n", shortSHA(state.SourceSHA))
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
	for _, e := range entries {
		fmt.Printf("  %s %s\n", mark, filepath.Join(dir, e.Name()))
	}
	if !dryRun {
		if err := os.RemoveAll(dir); err != nil {
			fmt.Printf("  %s Could not remove %s: %v\n", yellow("⚠"), dir, err)
		}
	}
	return len(entries)
}

func cmdUninstall(scope *InstallScope, dryRun bool) error {
	state, err := readScopedState(scope)
	if err != nil {
		return fmt.Errorf("reading state: %w", err)
	}
	if state == nil {
		fmt.Println("No nav-pilot collection installed. Nothing to uninstall.")
		return nil
	}

	if dryRun {
		fmt.Println(bold("Dry run: would uninstall"))
	} else {
		fmt.Println(bold(fmt.Sprintf("Uninstalling: %s", state.Collection)))
	}
	fmt.Println()

	removed := removeStateFiles(scope, state, dryRun, false)

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

	if !dryRun {
		os.Remove(scope.StatePath())
		scope.CleanupDirs()
	}

	fmt.Println()
	if dryRun {
		fmt.Printf("%s Would remove %d items.\n", dim("→"), removed)
	} else {
		fmt.Printf("%s Removed %d items.\n", green("✓"), removed)
	}
	return nil
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
func removeOrphans(scope *InstallScope, prior *StateFile, installed []InstalledFile) []InstalledFile {
	if prior == nil {
		return nil
	}
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
			err = os.RemoveAll(full)
		} else {
			err = os.Remove(full)
		}
		if err != nil && !os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "%s Could not remove %s, which is no longer part of the collection: %v\n",
				yellow("⚠"), f.Path, err)
			kept = append(kept, f)
			continue
		}
		fmt.Printf("  %s %s %s\n", red("×"), f.Path, dim("(no longer in the collection)"))
	}
	return kept
}
