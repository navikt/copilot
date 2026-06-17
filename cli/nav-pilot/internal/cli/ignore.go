package cli

import (
	"encoding/json"
	"fmt"
	"os"
)

// ignoreResult is the JSON output for --json mode.
type ignoreResult struct {
	Type   string `json:"type"`
	Name   string `json:"name"`
	Path   string `json:"path"`
	Status string `json:"status"` // "ignored" or "already_ignored"
}

// cmdIgnore marks a named item as ignored in the state file so it no longer
// appears in new-item reminders. Only meaningful for user-scope (all) installs.
func cmdIgnore(itemType, name string, scope *InstallScope, jsonOutput bool) error {
	kind, ok := kindByName[itemType]
	if !ok || kind == KindPrompt {
		return fmt.Errorf("unknown type %q. Valid types: agent, skill, instruction", itemType)
	}

	if !scope.IsUser() {
		return fmt.Errorf("ignore only applies to user-scope installs (--user)")
	}

	if err := validateName(name); err != nil {
		return fmt.Errorf("invalid name: %w", err)
	}

	state, err := readScopedState(scope)
	if err != nil {
		return fmt.Errorf("reading state: %w", err)
	}
	if state == nil {
		return fmt.Errorf("no installation found — run 'nav-pilot install --user' first")
	}

	// Compute the path as stored in the state file (matches detectNewItems logic).
	fileName := name + kind.Suffix
	if kind.IsDir {
		fileName = name
	}
	relPath := scope.RelPath(kind.Dir, fileName)
	if kind.IsDir {
		relPath += "/"
	}

	// Check existing state entries.
	for i, f := range state.Files {
		if f.Path == relPath {
			if f.Status == fileStatusIgnored {
				if jsonOutput {
					return printIgnoreJSON(ignoreResult{Type: itemType, Name: name, Path: relPath, Status: "already_ignored"})
				}
				fmt.Printf("%s %s %q is already ignored.\n", dim("ℹ"), itemType, name)
				return nil
			}
			// Active entry — mark as ignored.
			state.Files[i].Status = fileStatusIgnored
			if err := writeScopedState(scope, state); err != nil {
				return fmt.Errorf("writing state: %w", err)
			}
			if jsonOutput {
				return printIgnoreJSON(ignoreResult{Type: itemType, Name: name, Path: relPath, Status: "ignored"})
			}
			fmt.Printf("%s Ignored %s %q.\n", green("✓"), itemType, name)
			return nil
		}
	}

	// Not in state — add a new ignored entry.
	state.Files = append(state.Files, InstalledFile{
		Path:   relPath,
		Hash:   "",
		Status: fileStatusIgnored,
	})
	if err := writeScopedState(scope, state); err != nil {
		return fmt.Errorf("writing state: %w", err)
	}

	if jsonOutput {
		return printIgnoreJSON(ignoreResult{Type: itemType, Name: name, Path: relPath, Status: "ignored"})
	}
	fmt.Printf("%s Ignored %s %q.\n", green("✓"), itemType, name)
	fmt.Printf("  %s %s will no longer appear in new-item reminders.\n", dim("→"), name)
	return nil
}

func printIgnoreJSON(r ignoreResult) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(r)
}
