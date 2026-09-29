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
// not on PATH. That hook denies every matching tool call when it cannot start
// rtk (#915). It says so on stderr: a PATH trimmed by an IDE can hide an rtk
// someone installed themselves.
func removeUnusableRtkHook(client string) error {
	if client != "copilot" || isRtkInstalled() {
		return nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("could not determine home directory: %w", err)
	}
	hook := rtkCopilotHook(home)
	err = os.Remove(hook)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("could not remove unusable hook %s: %w", hook, err)
	}
	fmt.Fprintf(os.Stderr, "Removed ~/.copilot/hooks/rtk-rewrite.json: rtk is not on PATH and the hook would deny every Copilot tool call.\n")
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
	plugin := filepath.Join(dir, "plugins", "rtk.ts")
	cfg := filepath.Join(dir, "opencode.json")
	data, _ := os.ReadFile(cfg)
	listed := bytes.Contains(data, []byte("plugins/rtk.ts"))
	switch {
	case exists(plugin):
		found = true
		fmt.Printf("    %s rtk: opencode plugin found (%s)\n", yellow("⚠"), tilde(plugin))
		fmt.Printf("        nav-pilot no longer sets up rtk. The plugin rewrites commands before they run and can change what they return.\n")
		fmt.Printf("        %s If you did not install rtk yourself, remove it with %s\n", yellow("Solution:"), bold("rm -- "+tilde(plugin)))
		if listed {
			fmt.Printf("        and delete its entry from \"plugin\" in %s.\n", tilde(cfg))
		}
	case listed:
		found = true
		fmt.Printf("    %s rtk: %s still lists plugins/rtk.ts, which is gone\n", yellow("⚠"), tilde(cfg))
		fmt.Printf("        %s Delete its entry from \"plugin\" in %s.\n", yellow("Solution:"), tilde(cfg))
	}
	return found
}
