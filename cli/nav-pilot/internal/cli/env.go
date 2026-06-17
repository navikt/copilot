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

	copilotDir := filepath.Join(home, ".copilot")
	instrDir := filepath.Join(copilotDir, ".github", "instructions")

	// Check if instructions are actually installed
	matches, _ := filepath.Glob(filepath.Join(instrDir, "*.instructions.md"))
	if len(matches) == 0 {
		fmt.Fprintf(os.Stderr, "# No user-scope instructions installed.\n")
		fmt.Fprintf(os.Stderr, "# Run 'nav-pilot install --user' to install agents, skills, and instructions.\n")
		return nil
	}

	fmt.Fprintf(os.Stderr, "# nav-pilot env — Copilot CLI integration (%d instructions)\n", len(matches))
	fmt.Fprintf(os.Stderr, "# Add to your shell profile: eval \"$(nav-pilot env)\"\n")

	// Merge with existing COPILOT_CUSTOM_INSTRUCTIONS_DIRS if set
	value := copilotDir
	if existing := os.Getenv("COPILOT_CUSTOM_INSTRUCTIONS_DIRS"); existing != "" {
		alreadyPresent := false
		for _, p := range strings.Split(existing, ",") {
			if strings.TrimSpace(p) == copilotDir {
				alreadyPresent = true
				break
			}
		}
		if !alreadyPresent {
			value = existing + "," + copilotDir
		} else {
			value = existing
		}
	}

	// Print the export to stdout (so eval captures it)
	fmt.Printf("export COPILOT_CUSTOM_INSTRUCTIONS_DIRS=\"%s\"\n", value)
	return nil
}
