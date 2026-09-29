package cli

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/navikt/copilot/cli/nav-pilot/internal/artifacts"
)

// nav-pilot no longer installs rtk or wires it into a client (#1321). Earlier
// versions left a Copilot hook and an opencode plugin behind; this file only
// finds them. Someone who installed rtk themselves keeps it.

func isRtkInstalled() bool {
	_, err := exec.LookPath("rtk")
	return err == nil
}

func rtkCopilotHook(home string) string {
	return filepath.Join(home, ".copilot", "hooks", "rtk-rewrite.json")
}

// removeUnusableRtkHook removes the Copilot rtk hook when the rtk binary is
// gone. That hook denies every matching tool call when it cannot start rtk, so
// it is broken, not a choice (#915).
func removeUnusableRtkHook(client string) error {
	if client != "copilot" || isRtkInstalled() {
		return nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("could not determine home directory: %w", err)
	}
	hook := rtkCopilotHook(home)
	if err := os.Remove(hook); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("could not remove unusable hook %s: %w", hook, err)
	}
	return nil
}

// reportRtkLeftovers warns about an rtk Copilot hook or opencode plugin and
// names the command that removes it. It deletes nothing. It returns whether it
// printed anything.
func reportRtkLeftovers() bool {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return false
	}
	tilde := func(p string) string {
		if rest, ok := strings.CutPrefix(p, home+string(filepath.Separator)); ok {
			return "~/" + filepath.ToSlash(rest)
		}
		return p
	}
	exists := func(p string) bool {
		st, err := os.Stat(p)
		return err == nil && st.Mode().IsRegular()
	}
	found := false

	if hook := rtkCopilotHook(home); exists(hook) {
		found = true
		rm := "rm -- " + tilde(hook)
		if !isRtkInstalled() {
			fmt.Printf("    %s rtk: hook installed, but binary not found on PATH\n", red("[✗]"))
			fmt.Printf("        The hook denies every matching Copilot tool call when it cannot start rtk.\n")
			fmt.Printf("        %s Remove it with %s\n", red("Solution:"), bold(rm))
		} else {
			fmt.Printf("    %s rtk: Copilot hook found (%s)\n", yellow("⚠"), tilde(hook))
			fmt.Printf("        nav-pilot no longer sets up rtk. The hook rewrites commands before they run and can change what they return.\n")
			fmt.Printf("        %s If you did not install rtk yourself, remove it with %s\n", yellow("Solution:"), bold(rm))
		}
	}

	dir := artifacts.OpenCodeConfigDir()
	if plugin := filepath.Join(dir, "plugins", "rtk.ts"); exists(plugin) {
		found = true
		fmt.Printf("    %s rtk: opencode plugin found (%s)\n", yellow("⚠"), tilde(plugin))
		fmt.Printf("        nav-pilot no longer sets up rtk. The plugin rewrites commands before they run and can change what they return.\n")
		fmt.Printf("        %s If you did not install rtk yourself, remove it with %s\n", yellow("Solution:"), bold("rm -- "+tilde(plugin)))
		cfg := filepath.Join(dir, "opencode.json")
		if data, err := os.ReadFile(cfg); err == nil && bytes.Contains(data, []byte("plugins/rtk.ts")) {
			fmt.Printf("        and delete its entry from \"plugin\" in %s.\n", tilde(cfg))
		}
	}
	return found
}
