package cli

import (
	"fmt"
	"os"
	"path/filepath"
)

// cmdAdd installs a single agent, skill, instruction, or prompt from the source repo.
// It appends to the existing state file if one exists.
func cmdAdd(itemType, name string, scope *InstallScope, ref, sourceRepo string, dryRun, force bool, jsonOutput bool) error {
	// Validate type
	switch itemType {
	case "agent", "skill", "instruction", "prompt":
		// ok
	default:
		return fmt.Errorf("unknown type %q. Valid types: agent, skill, instruction, prompt", itemType)
	}

	if !scope.SupportsType(itemType) {
		return fmt.Errorf("type %q is not supported in user scope. Only agents, skills, and instructions can be installed to ~/.copilot", itemType)
	}

	if err := validateName(name); err != nil {
		return fmt.Errorf("invalid name: %w", err)
	}

	if !dryRun && !scope.IsUser() {
		if _, err := os.Stat(filepath.Join(scope.RootDir, ".git")); os.IsNotExist(err) {
			return fmt.Errorf("target %q does not appear to be a git repository (no .git directory)", scope.RootDir)
		}
	}

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

	// Fail closed before touching the filesystem (A3).
	if err := validatePakkeSource(src); err != nil {
		return err
	}

	sourceLabel := sourceLabelFor(src)

	result := &installResult{}

	if !jsonOutput {
		fmt.Println()
		if dryRun {
			fmt.Println(bold(fmt.Sprintf("Dry run: add %s %s", itemType, name)))
		} else {
			fmt.Println(bold(fmt.Sprintf("Adding %s: %s", itemType, name)))
		}
		fmt.Printf("%s %s\n", dim("Source:"), dim(fmt.Sprintf("%s@%s", sourceLabel, shortSHA(src.SHA))))
		fmt.Printf("%s %s\n", dim("Target:"), dim(scope.Label()))
		fmt.Println()
	}

	// Dispatch to the appropriate installer
	kind := kindByName[itemType]
	resolver := resolverFor(src.Dir, pakkeFor(src, name))
	installErr := installArtifact(resolver, scope, scopeStateHashes(scope), kind, name, dryRun, force, result)
	if installErr != nil {
		return installErr
	}

	if jsonOutput {
		return outputJSON(map[string]interface{}{
			"command":    "add",
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

	foreign, err := recordAddedFiles(scope, src, result, sourceRepo)
	if err != nil {
		return err
	}

	fmt.Printf("\n%s Added %s %q.\n", green("✓"), itemType, name)
	noteForeignSource(scope, foreign, fmt.Sprintf("nav-pilot install %s --type %s --source %s --force", name, itemType, foreign))
	return nil
}

// recordAddedFiles merges freshly installed files into the scope's state, and
// stamps each one with where it came from when that is not the scope's own
// source.
//
// Without the stamp the scope's SourceRepo is the only origin sync knows: the
// file is absent from it, sync reads that as "deleted upstream", and the next
// `sync --apply` removes what `add --source` just installed (#571).
//
// An auto-detected local checkout is not foreign. Running from a checkout gives
// the source an absolute path as its label, which never equals the scope's
// repo id, and stamping it would make sync skip the file forever — in a scope
// whose own source it is. Only a path the user asked for by name can be foreign
// that way, so an absolute label counts as foreign only with an explicit
// --source. Every other differing label still stamps: guardScopeSource does not
// fire when the config source is unset, so a plain add into a scope installed
// from elsewhere does resolve the default source, and that file must be stamped
// or the next sync deletes it.
//
// A foreign add also leaves the scope's SourceSHA alone: that SHA describes the
// scope's own source, not this file's.
//
// A scope with no state yet takes this source as its own, so its files need no
// stamp. A scope whose state predates source tracking keeps its unset
// SourceRepo — sync's adoption path owns that question — and its files are
// compared against the default source, which is where they must have come from.
func recordAddedFiles(scope *InstallScope, src *Source, result *installResult, explicitSource string) (string, error) {
	state, err := readScopedState(scope)
	if err != nil {
		return "", fmt.Errorf("reading existing state: %w", err)
	}
	foreign := ""
	if state == nil {
		state = &StateFile{
			Collection:  "(à la carte)",
			Scope:       scope.Name,
			Version:     src.Version,
			SourceRepo:  sourceLabelFor(src),
			SourceSHA:   src.SHA,
			InstalledAt: timeNow().UTC().Format("2006-01-02T15:04:05Z07:00"),
		}
	} else if label := sourceLabelFor(src); !sameSourceRepo(scopeSourceRepo(state), label) &&
		(explicitSource != "" || !filepath.IsAbs(label)) {
		foreign = label
	}
	if foreign == "" {
		state.SourceSHA = src.SHA
	}
	if state.Version == "" {
		state.Version = src.Version
	}

	index := make(map[string]int, len(state.Files))
	for i, f := range state.Files {
		index[f.Path] = i
	}
	for _, f := range result.Files {
		f.Source = foreign
		if i, ok := index[f.Path]; ok {
			state.Files[i] = f
			continue
		}
		index[f.Path] = len(state.Files)
		state.Files = append(state.Files, f)
	}
	if err := writeScopedState(scope, state); err != nil {
		fmt.Fprintf(os.Stderr, "%s Could not write state file: %v\n", yellow("⚠"), err)
	}
	return foreign, nil
}

// noteForeignSource says that the file just added does not belong to the scope's
// agentpakke, and how to keep it current. Adding it silently is what made
// `add --source` a trap: it looked like every other file until a sync
// disagreed.
//
// The update gesture is the add again, not a sync from the other source: a
// `sync --source` re-points the whole scope, which is how the scope's own files
// would go the way this file used to.
func noteForeignSource(scope *InstallScope, foreign, updateCmd string) {
	if foreign == "" {
		return
	}
	state, _ := readScopedState(scope)
	fmt.Printf("\n%s It comes from %s, not this scope's %s. `nav-pilot sync` leaves it alone —\n  re-run %s to update it.\n",
		dim("ℹ"), bold(foreign), bold(scopeSourceRepo(state)), bold(updateCmd))
}
