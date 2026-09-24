package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// cmdEnv prints shell export statements for Copilot CLI integration.
// Users can add `eval "$(nav-pilot env)"` to their shell profile.
func cmdEnv() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("cannot determine home directory: %w", err)
	}

	instrDir := filepath.Join(home, ".copilot", ".github", "instructions")

	// Check if instructions are actually installed
	matches, _ := filepath.Glob(filepath.Join(instrDir, "*.instructions.md"))
	if len(matches) == 0 {
		fmt.Fprintf(os.Stderr, "# No user-scope instructions installed.\n")
		fmt.Fprintf(os.Stderr, "# Run 'nav-pilot install --user' to install agents, skills, and instructions.\n")
		return nil
	}

	fmt.Fprintf(os.Stderr, "# nav-pilot env — Copilot CLI integration (%d instructions)\n", len(matches))
	fmt.Fprintf(os.Stderr, "# Add to your shell profile: eval \"$(nav-pilot env)\"\n")

	// The instructions directory itself, not ~/.copilot: Copilot searches it
	// recursively, and ~/.copilot/session-state is full of other sessions'
	// worktrees and their instructions. Merged with any existing value.
	value := instrDir
	if existing := os.Getenv("COPILOT_CUSTOM_INSTRUCTIONS_DIRS"); existing != "" {
		alreadyPresent := false
		for _, p := range strings.Split(existing, ",") {
			if strings.TrimSpace(p) == instrDir {
				alreadyPresent = true
				break
			}
		}
		if !alreadyPresent {
			value = existing + "," + instrDir
		} else {
			value = existing
		}
	}

	// Print the export to stdout (so eval captures it)
	fmt.Printf("export COPILOT_CUSTOM_INSTRUCTIONS_DIRS=\"%s\"\n", value)
	return nil
}
