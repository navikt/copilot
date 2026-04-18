package main

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

	if !jsonOutput {
		fmt.Println(dim("Resolving source..."))
	}
	src, err := resolveSource(ref, sourceRepo)
	if err != nil {
		return err
	}
	defer src.Cleanup()

	sourceLabel := "navikt/copilot"
	if sourceRepo != "" {
		sourceLabel = sourceRepo
	}

	result := &installResult{}

	if !jsonOutput {
		fmt.Println()
		if dryRun {
			fmt.Println(bold(fmt.Sprintf("Dry run: add %s %s", itemType, name)))
		} else {
			fmt.Println(bold(fmt.Sprintf("Adding %s: %s", itemType, name)))
		}
		fmt.Printf("%s %s\n", dim("Source:"), dim(fmt.Sprintf("%s@%s", sourceLabel, src.SHA)))
		fmt.Printf("%s %s\n", dim("Target:"), dim(scope.Label()))
		fmt.Println()
	}

	// Dispatch to the appropriate installer
	kind := kindByName[itemType]
	resolver := NewSourceResolver(src.Dir)
	installErr := installArtifact(resolver, scope, kind, name, dryRun, force, result)
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
			// Update hash and clear ignored status for existing entry
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

	fmt.Printf("\n%s Added %s %q.\n", green("✓"), itemType, name)
	return nil
}
