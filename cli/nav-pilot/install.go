package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type installResult struct {
	Installed   int
	Skipped     int
	Conflicts   int
	Unsupported []string
	Files       []InstalledFile
}

func installItems(sourceDir string, scope *InstallScope, manifest *Manifest, dryRun, force bool) (*installResult, error) {
	resolver := NewSourceResolver(sourceDir)
	result := &installResult{}

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
			if err := installArtifact(resolver, scope, group.kind, name, dryRun, force, result); err != nil {
				return result, err
			}
		}
		fmt.Println()
	}

	return result, nil
}

// installArtifact handles the install for any artifact type.
// Resolution, copy, hash logic are driven by the ArtifactKind.
func installArtifact(resolver *SourceResolver, scope *InstallScope, kind *ArtifactKind, name string, dryRun, force bool, result *installResult) error {
	if err := validateName(name); err != nil {
		return fmt.Errorf("invalid %s name: %w", kind.Name, err)
	}

	art, found := resolver.Get(kind, name)
	if !found {
		fmt.Printf("  %s %s not found: %s\n", yellow("⚠"), titleCase(kind.Name), name)
		result.Skipped++
		return nil
	}

	dst := scope.DstPath(kind.Dir, art.FileName())
	relPath := kind.RelPathForName(scope, art.Name)

	if c, err := checkConflict(dst, art.AbsPath, art.IsDir); err != nil {
		return err
	} else if c != nil && !force {
		// File exists and differs but we're not forcing — skip the overwrite
		// but track as conflict so it's not lost from state or reported as "new".
		fmt.Printf("  %s %s (exists, differs — use --force to overwrite)\n", yellow("⚠"), name)
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

// ─── Commands ───────────────────────────────────────────────────────────────

// cmdInstallAuto resolves whether <name> is a collection or an individual artifact,
// then dispatches to the appropriate installer. If --type is provided, it skips
// collection lookup and installs a specific artifact type.
func cmdInstallAuto(name, itemType string, scope *InstallScope, ref, sourceRepo string, dryRun, force bool, jsonOutput bool) error {
	// If explicit --type given, go straight to single-artifact install
	if itemType != "" {
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

	if !jsonOutput {
		fmt.Println(dim("Resolving source..."))
	}
	src, err := resolveSource(ref, sourceRepo)
	if err != nil {
		return err
	}
	defer src.Cleanup()

	// Check if name matches a collection
	isCollection := false
	collections, _ := listCollectionDirs(src.Dir) // ignore error: missing dir = no collections
	for _, c := range collections {
		if c == name {
			isCollection = true
			break
		}
	}

	// Check if name matches any artifact
	resolver := NewSourceResolver(src.Dir)
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
		return cmdAddFromSource(matchedKinds[0].Name, name, src, scope, dryRun, force, jsonOutput)
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
func cmdInstallFromSource(collection string, src *Source, scope *InstallScope, dryRun, force bool, jsonOutput bool) error {
	manifest, err := loadManifest(src.Dir, collection)
	if err != nil {
		return err
	}

	sourceLabel := "navikt/copilot"

	if !jsonOutput {
		fmt.Println()
		if dryRun {
			fmt.Println(bold(fmt.Sprintf("Dry run: %s", collection)))
		} else {
			fmt.Println(bold(fmt.Sprintf("Installing: %s", collection)))
		}
		fmt.Printf("%s %s\n", dim("Source:"), dim(fmt.Sprintf("%s@%s", sourceLabel, src.SHA)))
		fmt.Printf("%s %s\n", dim("Target:"), dim(scope.Label()))
		printManifestContents(manifest)
		fmt.Println()
	}

	result, err := installItems(src.Dir, scope, manifest, dryRun, force)
	if err != nil {
		return err
	}
	if !dryRun {
		telemetry.RecordInstallItems(scope.Name, telemetryMode(), int64(result.Installed))
	}

	if jsonOutput {
		return outputJSON(map[string]interface{}{
			"command":     "install",
			"collection":  collection,
			"scope":       scope.Name,
			"source_sha":  src.SHA,
			"version":     src.Version,
			"installed":   result.Installed,
			"conflicts":   result.Conflicts,
			"unsupported": result.Unsupported,
			"dry_run":     dryRun,
		})
	}

	if result.Conflicts > 0 {
		fmt.Printf("%s %d file(s) skipped due to conflicts. Use %s to overwrite.\n",
			yellow("⚠"), result.Conflicts, bold("--force"))
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
		Collection:  collection,
		Version:     stateVersion,
		Scope:       scope.Name,
		SourceSHA:   src.SHA,
		InstalledAt: timeNow().UTC().Format("2006-01-02T15:04:05Z07:00"),
		Files:       result.Files,
	}
	if err := writeScopedState(scope, state); err != nil {
		fmt.Fprintf(os.Stderr, "%s Could not write state file: %v\n", yellow("⚠"), err)
	}

	fmt.Printf("%s Installed %d items from %q (v%s, %s).\n",
		green("✓"), result.Installed, collection, stateVersion, src.SHA)
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
func cmdAddFromSource(itemType, name string, src *Source, scope *InstallScope, dryRun, force bool, jsonOutput bool) error {
	if !scope.SupportsType(itemType) {
		return fmt.Errorf("type %q is not supported in user scope. Only agents, skills, and instructions can be installed to ~/.copilot", itemType)
	}

	sourceLabel := "navikt/copilot"

	result := &installResult{}

	if !jsonOutput {
		fmt.Println()
		if dryRun {
			fmt.Println(bold(fmt.Sprintf("Dry run: install %s %s", itemType, name)))
		} else {
			fmt.Println(bold(fmt.Sprintf("Installing %s: %s", itemType, name)))
		}
		fmt.Printf("%s %s\n", dim("Source:"), dim(fmt.Sprintf("%s@%s", sourceLabel, src.SHA)))
		fmt.Printf("%s %s\n", dim("Target:"), dim(scope.Label()))
		fmt.Println()
	}

	kind := kindByName[itemType]
	resolver := NewSourceResolver(src.Dir)
	installErr := installArtifact(resolver, scope, kind, name, dryRun, force, result)
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
		fmt.Printf("\n%s File already exists and differs. Use %s to overwrite.\n",
			yellow("⚠"), bold("--force"))
	}

	if dryRun || result.Installed == 0 {
		return nil
	}

	// Append to state file if one exists, otherwise create a minimal one
	state, err := readScopedState(scope)
	if err != nil {
		return fmt.Errorf("reading existing state: %w", err)
	}
	if state == nil {
		state = &StateFile{
			Collection:  "(à la carte)",
			Scope:       scope.Name,
			Version:     src.Version,
			SourceSHA:   src.SHA,
			InstalledAt: timeNow().UTC().Format("2006-01-02T15:04:05Z07:00"),
		}
	}
	state.SourceSHA = src.SHA
	if state.Version == "" {
		state.Version = src.Version
	}

	// Merge new files into state, avoiding duplicates
	existing := make(map[string]bool)
	for _, f := range state.Files {
		existing[f.Path] = true
	}
	for _, f := range result.Files {
		if !existing[f.Path] {
			state.Files = append(state.Files, f)
		} else {
			for i, sf := range state.Files {
				if sf.Path == f.Path {
					state.Files[i].Hash = f.Hash
					state.Files[i].Status = ""
					break
				}
			}
		}
	}
	if err := writeScopedState(scope, state); err != nil {
		fmt.Fprintf(os.Stderr, "%s Could not write state file: %v\n", yellow("⚠"), err)
	}

	fmt.Printf("\n%s Installed %s %q.\n", green("✓"), itemType, name)
	return nil
}

func cmdList(ref, sourceRepo string, showItems bool, jsonOutput bool) error {
	if !jsonOutput {
		fmt.Println(dim("Resolving source..."))
	}
	src, err := resolveSource(ref, sourceRepo)
	if err != nil {
		return err
	}
	defer src.Cleanup()

	names, err := listCollectionDirs(src.Dir)
	if err != nil {
		return err
	}

	if jsonOutput {
		type collectionInfo struct {
			Name        string `json:"name"`
			Description string `json:"description"`
			Items       int    `json:"items"`
		}
		var collections []collectionInfo
		for _, name := range names {
			m, err := loadManifest(src.Dir, name)
			if err != nil {
				continue
			}
			total := len(m.Agents) + len(m.Skills) + len(m.Instructions) + len(m.Prompts)
			collections = append(collections, collectionInfo{Name: name, Description: m.Description, Items: total})
		}
		result := map[string]interface{}{"collections": collections}
		if showItems {
			result["items"] = collectAvailableItems(src.Dir)
		}
		return outputJSON(result)
	}

	fmt.Println()
	fmt.Println(bold("Available collections:"))
	fmt.Println()
	for _, name := range names {
		m, err := loadManifest(src.Dir, name)
		if err != nil {
			continue
		}
		total := len(m.Agents) + len(m.Skills) + len(m.Instructions) + len(m.Prompts)
		fmt.Printf("  %-20s %s %s\n", bold(name), m.Description, dim(fmt.Sprintf("(%d items)", total)))
		if len(m.Agents) > 0 {
			fmt.Printf("  %-20s %s\n", "", dim("agents: "+strings.Join(m.Agents, ", ")))
		}
	}
	fmt.Println()
	fmt.Printf("Install with: %s\n", bold("nav-pilot install <name>"))
	fmt.Printf("Install everything to user home: %s\n", bold("nav-pilot install --user --all"))

	if showItems {
		fmt.Println()
		if err := listAvailableItems(src.Dir); err != nil {
			return err
		}
	} else {
		fmt.Printf("Show individual items: %s\n", bold("nav-pilot list --items"))
	}
	return nil
}

// listAvailableItems prints all agents, skills, instructions, and prompts in the source.
func listAvailableItems(sourceDir string) error {
	resolver := NewSourceResolver(sourceDir)
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
func collectAvailableItems(sourceDir string) map[string][]string {
	resolver := NewSourceResolver(sourceDir)
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
	fmt.Println(dim("Resolving source..."))

	src, err := resolveSource(ref, sourceRepo)
	if err != nil {
		return err
	}
	defer src.Cleanup()

	if targetDir == "" {
		// Not in a git repo — only user-home scope is possible
		scope, scopeErr := ScopeUser()
		if scopeErr != nil {
			return scopeErr
		}
		return interactiveUserInstallFromSource(scope, src)
	}

	// In a git repo: ask where to install
	scope, err := promptInstallScope(targetDir)
	if err != nil {
		return err
	}
	if scope == nil {
		fmt.Println(dim("Cancelled."))
		return nil
	}

	if scope.IsUser() {
		return interactiveUserInstallFromSource(scope, src)
	}

	// Repo scope: pick a collection
	return interactiveRepoInstall(src, scope)
}

// cmdInstallAll installs all agents and skills to user scope by scanning the source.
// Used when `nav-pilot install --user` is run without a collection name.
// When interactive, offers the same picker as the root `nav-pilot` command.
func cmdInstallAll(scope *InstallScope, ref, sourceRepo string, dryRun, force bool, jsonOutput bool) error {
	if !jsonOutput {
		fmt.Println(dim("Resolving source..."))
	}
	src, err := resolveSource(ref, sourceRepo)
	if err != nil {
		return err
	}
	defer src.Cleanup()

	// Interactive mode: offer the picker (same UX as `nav-pilot` root command)
	if isInteractive() && !dryRun && !jsonOutput {
		return interactiveUserInstallFromSource(scope, src)
	}

	return installAllFromSource(scope, src, nil, dryRun, force, jsonOutput)
}

// installAllFromSource installs all agents+skills from source.
// If manifest is nil, it scans the source directory to discover items.
// extraStateFiles are appended to the state file after install (e.g. ignored items from picker).
// Extracted so both cmdInstallAll and the interactive flow can share this.
func installAllFromSource(scope *InstallScope, src *Source, manifest *Manifest, dryRun, force bool, jsonOutput bool, extraStateFiles ...InstalledFile) error {
	if manifest == nil {
		var err error
		manifest, err = collectAllItems(src.Dir)
		if err != nil {
			return err
		}
	}

	total := len(manifest.Agents) + len(manifest.Skills) + len(manifest.Instructions)
	if total == 0 {
		return fmt.Errorf("no agents, skills, or instructions found in source")
	}

	sourceLabel := "navikt/copilot"

	if !jsonOutput {
		fmt.Println()
		if dryRun {
			fmt.Println(bold(fmt.Sprintf("Dry run: all agents, skills & instructions (%d items)", total)))
		} else {
			fmt.Println(bold(fmt.Sprintf("Installing: all agents, skills & instructions (%d items)", total)))
		}
		fmt.Printf("%s %s\n", dim("Source:"), dim(fmt.Sprintf("%s@%s", sourceLabel, src.SHA)))
		fmt.Printf("%s %s\n", dim("Target:"), dim(scope.Label()))
		fmt.Println()
	}

	result, err := installItems(src.Dir, scope, manifest, dryRun, force)
	if err != nil {
		return err
	}
	if !dryRun {
		telemetry.RecordInstallItems(scope.Name, telemetryMode(), int64(result.Installed))
	}

	if !jsonOutput && result.Conflicts > 0 {
		fmt.Printf("%s %d file(s) skipped due to conflicts. Use %s to overwrite.\n",
			yellow("⚠"), result.Conflicts, bold("--force"))
	}

	if jsonOutput {
		return outputJSON(map[string]interface{}{
			"command":    "install",
			"collection": CollectionAll,
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
		Collection:  CollectionAll,
		Version:     stateVersion,
		Scope:       scope.Name,
		SourceSHA:   src.SHA,
		InstalledAt: timeNow().UTC().Format("2006-01-02T15:04:05Z07:00"),
		Files:       result.Files,
	}

	// Append items the user explicitly deselected in the picker as ignored.
	if len(extraStateFiles) > 0 {
		state.Files = append(state.Files, extraStateFiles...)
	}

	if err := writeScopedState(scope, state); err != nil {
		fmt.Fprintf(os.Stderr, "%s Could not write state file: %v\n", yellow("⚠"), err)
	}

	fmt.Printf("%s Installed %d items to %s (v%s, %s).\n",
		green("✓"), result.Installed, scope.Label(), stateVersion, src.SHA)
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

// cmdInstall installs a named collection. Used by the old direct dispatch path
// and re-resolves source (unlike cmdInstallFromSource which reuses an existing source).
func cmdStatus(scope *InstallScope, jsonOutput bool) error {
	return cmdStatusScoped(scope, false, jsonOutput)
}

// cmdStatusAuto shows status for all detected scopes (repo + user) when the
// user didn't explicitly pick one with --user or --target.
func cmdStatusAuto(repoDir string, jsonOutput bool) error {
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
			ok, modified, missing, _, _ := countFileIntegrity(cs.OutputDir, cs.State)
			scopes = append(scopes, map[string]interface{}{
				"scope": cs.ScopeName, "collection": cs.State.Collection,
				"version": cs.State.Version, "source_sha": cs.State.SourceSHA,
				"installed_at": cs.State.InstalledAt, "files": len(cs.State.Files),
				"ok": ok, "modified": modified, "missing": missing,
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

func cmdStatusScoped(scope *InstallScope, _ bool, jsonOutput bool) error {
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
	fmt.Printf("  Source:      %s\n", state.SourceSHA)
	fmt.Printf("  Installed:   %s\n", state.InstalledAt)
	fmt.Printf("  Files:       %d\n", len(state.Files))
	fmt.Println()

	for _, p := range modifiedPaths {
		fmt.Printf("  %s %s (modified locally)\n", yellow("~"), p)
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

	removed := 0
	for _, f := range state.Files {
		path := filepath.Join(scope.RootDir, f.Path)

		if dryRun {
			fmt.Printf("  %s %s\n", dim("×"), f.Path)
			removed++
			continue
		}

		if strings.HasSuffix(f.Path, "/") {
			if err := os.RemoveAll(path); err != nil && !os.IsNotExist(err) {
				fmt.Printf("  %s Could not remove %s: %v\n", yellow("⚠"), f.Path, err)
				continue
			}
		} else {
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				fmt.Printf("  %s Could not remove %s: %v\n", yellow("⚠"), f.Path, err)
				continue
			}
		}
		fmt.Printf("  %s %s\n", red("×"), f.Path)
		removed++
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
