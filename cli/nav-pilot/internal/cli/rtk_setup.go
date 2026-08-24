package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/charmbracelet/huh"
)

// maybePromptRtkSetup coordinates the interactive prompt and installation of RTK.
// It is the main entry point called from the interactive launch flow.
func maybePromptRtkSetup(cfg ResolvedConfig) {
	if !shouldPromptRtk(cfg) {
		return
	}

	if err := promptAndInstallRtk(cfg); err != nil {
		// Log warning but don't fail the launch
		fmt.Fprintf(os.Stderr, "%s RTK Setup Warning: %v\n", yellow("⚠"), err)
	}
}

// shouldPromptRtk determines if the user needs to be prompted.
func shouldPromptRtk(cfg ResolvedConfig) bool {
	if !isInteractive() {
		return false
	}
	promptedClients := strings.Split(cfg.RtkPromptedClient, ",")
	for _, pc := range promptedClients {
		if pc == cfg.Client {
			return false // already prompted for this client
		}
	}
	return true
}

// promptAndInstallRtk handles the actual menu, state tracking, and installation execution.
func promptAndInstallRtk(cfg ResolvedConfig) error {
	hasRtk := isRtkInstalled()

	fmt.Println()
	fmt.Printf("%s Terminal output filter (rtk)\n", bold("🔧"))
	fmt.Println(dim("  rtk filters and compresses terminal output before it reaches the model's context."))
	fmt.Println(dim("  It runs in the background and won't change how your commands work."))
	fmt.Println(dim("  Note: public controlled measurement has not reproduced the vendor's savings claim —"))
	fmt.Println(dim("  https://blog.jetbrains.com/ai/2026/07/rtk-claude-code-token-savings/"))
	fmt.Println()

	var choice string
	err := huh.NewSelect[string]().
		Title(fmt.Sprintf("Install the terminal output filter (rtk) for %s?", cfg.Client)).
		Options(
			huh.NewOption("Yes, set it up", "yes"),
			huh.NewOption("No thanks", "no"),
		).
		Value(&choice).
		WithTheme(navTheme()).
		Run()

	// Handle graceful abort or state saving
	if err == nil && (choice == "yes" || choice == "no") {
		savePromptState(cfg)
	}

	if err != nil {
		telemetry.RecordRtkSetup(cfg.Client, "aborted", "success")
		return nil // User aborted
	}
	if choice != "yes" {
		telemetry.RecordRtkSetup(cfg.Client, "no", "success")
		return nil // User said no
	}

	fmt.Println()
	var rtkPath string
	if !hasRtk {
		p, res, installErr := installRtk()
		if installErr != nil {
			telemetry.RecordRtkSetup(cfg.Client, "yes", res)
			return fmt.Errorf("installation failed: %w", installErr)
		}
		rtkPath = p
	} else {
		p, _ := exec.LookPath("rtk")
		rtkPath = p
	}

	if initErr := initRtkHooks(cfg.Client, rtkPath); initErr != nil {
		telemetry.RecordRtkSetup(cfg.Client, "yes", "init_failed")
		return initErr
	}

	if hasRtk {
		telemetry.RecordRtkSetup(cfg.Client, "yes", "already_installed")
	} else {
		telemetry.RecordRtkSetup(cfg.Client, "yes", "success")
	}

	return nil
}

func savePromptState(cfg ResolvedConfig) {
	newClients := cfg.Client
	if cfg.RtkPromptedClient != "" {
		newClients = cfg.RtkPromptedClient + "," + cfg.Client
	}
	if setErr := cmdConfigSet("rtk_prompted_client", newClients); setErr != nil {
		fmt.Fprintf(os.Stderr, "%s Warning: Could not save rtk config: %v\n", yellow("⚠"), setErr)
	}
	if setErr := cmdConfigSet("rtk_prompted_at", time.Now().Format(time.RFC3339)); setErr != nil {
		fmt.Fprintf(os.Stderr, "%s Warning: Could not save rtk timestamp: %v\n", yellow("⚠"), setErr)
	}
}

func isRtkInstalled() bool {
	_, err := exec.LookPath("rtk")
	return err == nil
}

// installRtk installs rtk through Homebrew only.
//
// Why there is no scripted fallback: the previous implementation ran
// `brew install navikt/tap/rtk`, but navikt/homebrew-tap has no rtk formula
// (only cplt and nav-pilot), so the brew step always failed and every user
// silently fell through to `curl … refs/heads/master/install.sh | sh` — an
// unpinned pipe-to-shell from a moving upstream branch. rtk ships in
// homebrew-core, so `brew install rtk` gets us a checksum-verified bottle and
// Homebrew's own supply-chain handling. If Homebrew is unavailable we print
// manual instructions rather than reintroducing an unverified install script.
func installRtk() (string, string, error) {
	if _, err := exec.LookPath("brew"); err != nil {
		printManualRtkInstructions()
		return "", "brew_missing", fmt.Errorf("homebrew not found; rtk must be installed manually")
	}
	p, res, err := installRtkViaBrew()
	if err != nil {
		printManualRtkInstructions()
		return "", res, err
	}
	return p, res, nil
}

// printManualRtkInstructions tells the user how to install rtk themselves.
func printManualRtkInstructions() {
	fmt.Fprint(os.Stderr, "\n  Install rtk manually, then run nav-pilot again:\n")
	fmt.Fprint(os.Stderr, "    brew install rtk    (homebrew-core, checksum-verified)\n")
	fmt.Fprint(os.Stderr, "    or download a signed release archive from https://github.com/rtk-ai/rtk/releases\n\n")
}

func installRtkViaBrew() (string, string, error) {
	fmt.Printf("%s Installing rtk via brew...\n", dim("→"))
	// homebrew-core formula (https://formulae.brew.sh/formula/rtk), not a Nav tap.
	cmd := exec.Command("brew", "install", "rtk")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", "brew_failed", err
	}

	// Resolve correct path after install
	if p, err := exec.LookPath("rtk"); err == nil {
		return p, "success", nil
	}

	// Fallback to brew prefix if LookPath fails
	if out, err := exec.Command("brew", "--prefix").Output(); err == nil {
		return filepath.Join(strings.TrimSpace(string(out)), "bin", "rtk"), "success", nil
	}

	return "rtk", "success", nil
}

func getOpenCodeConfigDir() string {
	if runtime.GOOS == "windows" {
		if cfg, err := os.UserConfigDir(); err == nil {
			return filepath.Join(cfg, "opencode")
		}
	}
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "opencode")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "opencode")
}

func initRtkHooks(client string, rtkPath string) error {
	fmt.Printf("%s Initializing rtk hooks...\n", dim("→"))
	args := []string{"init", "--global"}

	switch client {
	case "copilot":
		args = append(args, "--copilot")
	case "opencode":
		args = append(args, "--opencode")
		ocDir := getOpenCodeConfigDir()
		opencodePath := filepath.Join(ocDir, "opencode.json")
		if patchErr := patchOpenCodeConfig(opencodePath, ocDir); patchErr != nil {
			fmt.Fprintf(os.Stderr, "%s Warning: Could not auto-patch opencode.json: %v\n", yellow("⚠"), patchErr)
		}
	case "pi":
		args = append(args, "--agent", "pi")
	default:
		args = append(args, "--agent", "claude")
	}

	cmd := exec.Command(rtkPath, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		cmdStr := fmt.Sprintf("%s %s", rtkPath, strings.Join(args, " "))
		fmt.Fprintf(os.Stderr, "\n  Suggestions:\n  1. Try running the command manually: %s\n  2. Ensure you have the necessary write permissions.\n", cmdStr)
		return fmt.Errorf("failed to init hooks: %w", err)
	}

	fmt.Printf("%s rtk is now set up!\n\n", green("✓"))
	return nil
}

// patchOpenCodeConfig ensures the given opencode config file has the rtk plugin configured.
func patchOpenCodeConfig(opencodePath string, ocDir string) error {
	// Resolve symlinks to avoid overwriting the symlink itself with a regular file during atomic rename
	realPath, err := filepath.EvalSymlinks(opencodePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // File doesn't exist, nothing to do
		}
		return fmt.Errorf("failed to evaluate symlink for opencode config: %w", err)
	}

	info, err := os.Stat(realPath)
	if err != nil {
		return fmt.Errorf("failed to stat opencode config: %w", err)
	}

	data, err := os.ReadFile(realPath)
	if err != nil {
		return fmt.Errorf("failed to read opencode config: %w", err)
	}

	var config map[string]interface{}
	if err := json.Unmarshal(data, &config); err != nil {
		// Might be JSONC or invalid JSON. We abort safely.
		return fmt.Errorf("failed to unmarshal opencode config: %w", err)
	}

	pluginStr := filepath.ToSlash(filepath.Join(ocDir, "plugins", "rtk.ts"))

	pluginsRaw, exists := config["plugin"]
	if !exists {
		config["plugin"] = []string{pluginStr}
	} else {
		// Handle the case where 'plugin' is a string instead of an array
		if singleStr, ok := pluginsRaw.(string); ok {
			if singleStr == pluginStr {
				return nil
			}
			config["plugin"] = []string{singleStr, pluginStr}
		} else if plugins, ok := pluginsRaw.([]interface{}); ok {
			hasPlugin := false
			for _, p := range plugins {
				if str, ok := p.(string); ok && str == pluginStr {
					hasPlugin = true
					break
				}
			}

			if !hasPlugin {
				config["plugin"] = append(plugins, pluginStr)
			} else {
				return nil // already patched
			}
		} else {
			return fmt.Errorf("in file %s: 'plugin' field has unexpected type %T, expected string or array", realPath, pluginsRaw)
		}
	}

	patchedData, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal patched config: %w", err)
	}

	// Atomic write: write to temp file then rename
	tmpPath := realPath + ".tmp"
	if err := os.WriteFile(tmpPath, patchedData, info.Mode()); err != nil {
		return fmt.Errorf("failed to write temporary config file: %w", err)
	}
	// Explicitly apply the original permissions, as os.WriteFile is affected by umask
	if err := os.Chmod(tmpPath, info.Mode()); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("failed to apply original permissions to temp file: %w", err)
	}
	if err := os.Rename(tmpPath, realPath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("failed to commit patched config file: %w", err)
	}
	return nil
}
