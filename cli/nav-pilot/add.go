package main

import (
	"fmt"
	"os"
	"path/filepath"
)

// cmdAdd installs a single agent, skill, instruction, or prompt from the source repo.
// It appends to the existing state file if one exists.
func cmdAdd(itemType, name, targetDir, ref string, dryRun, force bool) error {
	// Validate type
	switch itemType {
	case "agent", "skill", "instruction", "prompt":
		// ok
	default:
		return fmt.Errorf("unknown type %q. Valid types: agent, skill, instruction, prompt", itemType)
	}

	if err := validateName(name); err != nil {
		return fmt.Errorf("invalid name: %w", err)
	}

	if !dryRun {
		if _, err := os.Stat(filepath.Join(targetDir, ".git")); os.IsNotExist(err) {
			return fmt.Errorf("target %q does not appear to be a git repository (no .git directory)", targetDir)
		}
	}

	fmt.Println(dim("Resolving source..."))
	src, err := resolveSource(ref, "")
	if err != nil {
		return err
	}
	defer src.Cleanup()

	result := &installResult{}

	fmt.Println()
	if dryRun {
		fmt.Println(bold(fmt.Sprintf("Dry run: add %s %s", itemType, name)))
	} else {
		fmt.Println(bold(fmt.Sprintf("Adding %s: %s", itemType, name)))
	}
	fmt.Printf("%s %s\n", dim("Source:"), dim(fmt.Sprintf("navikt/copilot@%s", src.SHA)))
	fmt.Println()

	// Dispatch to the appropriate installer
	var installErr error
	switch itemType {
	case "agent":
		installErr = installAgent(src.Dir, targetDir, name, dryRun, force, result)
	case "skill":
		installErr = installSkill(src.Dir, targetDir, name, dryRun, force, result)
	case "instruction":
		installErr = installInstruction(src.Dir, targetDir, name, dryRun, force, result)
	case "prompt":
		installErr = installPrompt(src.Dir, targetDir, name, dryRun, force, result)
	}
	if installErr != nil {
		return installErr
	}

	if result.Conflicts > 0 {
		fmt.Printf("\n%s File already exists and differs. Use %s to overwrite.\n",
			yellow("⚠"), bold("--force"))
	}

	if dryRun || result.Installed == 0 {
		return nil
	}

	// Append to state file if one exists, otherwise create a minimal one
	state, err := readState(targetDir)
	if err != nil {
		state = nil
	}
	if state == nil {
		state = &StateFile{
			Collection:  "(à la carte)",
			SourceSHA:   src.SHA,
			InstalledAt: timeNow().UTC().Format("2006-01-02T15:04:05Z07:00"),
		}
	}
	state.SourceSHA = src.SHA

	// Merge new files into state, avoiding duplicates
	existing := make(map[string]bool)
	for _, f := range state.Files {
		existing[f.Path] = true
	}
	for _, f := range result.Files {
		if !existing[f.Path] {
			state.Files = append(state.Files, f)
		} else {
			// Update hash for existing entry
			for i, sf := range state.Files {
				if sf.Path == f.Path {
					state.Files[i].Hash = f.Hash
					break
				}
			}
		}
	}
	if err := writeState(targetDir, state); err != nil {
		fmt.Fprintf(os.Stderr, "%s Could not write state file: %v\n", yellow("⚠"), err)
	}

	fmt.Printf("\n%s Added %s %q.\n", green("✓"), itemType, name)
	return nil
}
