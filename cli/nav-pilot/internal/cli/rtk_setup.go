package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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
	fmt.Printf("%s Terminal Token Optimizer (rtk)\n", bold("🚀"))
	fmt.Println(dim("  Safely filters terminal noise before it reaches the AI, saving 60-90% on token costs."))
	fmt.Println(dim("  It runs entirely in the background and won't change how your commands work."))
	fmt.Println()

	var choice string
	err := huh.NewSelect[string]().
		Title(fmt.Sprintf("Install Terminal Token Optimizer for %s?", cfg.Client)).
		Options(
			huh.NewOption("Yes, set it up (Highly Recommended)", "yes"),
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
		telemetry.RecordRTKSetup(cfg.Client, "aborted", "success")
		return nil // User aborted
	}
	if choice != "yes" {
		telemetry.RecordRTKSetup(cfg.Client, "no", "success")
		return nil // User said no
	}

	fmt.Println()
	var rtkPath string
	if !hasRtk {
		p, installErr := installRtk()
		if installErr != nil {
			telemetry.RecordRTKSetup(cfg.Client, "yes", "error")
			return fmt.Errorf("installation failed: %w", installErr)
		}
		rtkPath = p
	} else {
		p, _ := exec.LookPath("rtk")
		rtkPath = p
	}

	if initErr := initRtkHooks(cfg.Client, rtkPath); initErr != nil {
		telemetry.RecordRTKSetup(cfg.Client, "yes", "init_failed")
		return initErr
	}

	if hasRtk {
		telemetry.RecordRTKSetup(cfg.Client, "yes", "already_installed")
	} else {
		telemetry.RecordRTKSetup(cfg.Client, "yes", "success")
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

func installRtk() (string, error) {
	if _, err := exec.LookPath("brew"); err == nil {
		return installRtkViaBrew()
	}
	return installRtkViaCurl()
}

func installRtkViaBrew() (string, error) {
	fmt.Printf("%s Installing rtk via brew...\n", dim("→"))
	cmd := exec.Command("brew", "install", "navikt/tap/rtk")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", err
	}

	// Resolve correct path after install
	if p, err := exec.LookPath("rtk"); err == nil {
		return p, nil
	}

	// Fallback to brew prefix if LookPath fails
	if out, err := exec.Command("brew", "--prefix").Output(); err == nil {
		return filepath.Join(strings.TrimSpace(string(out)), "bin", "rtk"), nil
	}

	return "rtk", nil
}

func installRtkViaCurl() (string, error) {
	fmt.Printf("%s Installing rtk via curl script...\n", dim("→"))
	cmd := exec.Command("sh", "-c", "curl -fsSL https://raw.githubusercontent.com/rtk-ai/rtk/refs/heads/master/install.sh | sh")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", err
	}

	// The install script might drop it somewhere not immediately in PATH
	if p, err := exec.LookPath("rtk"); err == nil {
		return p, nil
	}

	// Check common cargo bin if the script is rust-based or uses cargo
	home, _ := os.UserHomeDir()
	cargoBin := filepath.Join(home, ".cargo", "bin", "rtk")
	if _, err := os.Stat(cargoBin); err == nil {
		return cargoBin, nil
	}

	// Just return "rtk" and let the exec fail if it still can't find it
	return "rtk", nil
}

func initRtkHooks(client string, rtkPath string) error {
	fmt.Printf("%s Initializing rtk hooks...\n", dim("→"))
	args := []string{"init", "--global"}

	switch client {
	case "copilot":
		args = append(args, "--copilot")
	case "opencode":
		args = append(args, "--opencode")
		home, err := os.UserHomeDir()
		if err == nil {
			opencodePath := filepath.Join(home, ".config", "opencode", "opencode.json")
			if patchErr := patchOpenCodeConfig(opencodePath); patchErr != nil {
				fmt.Fprintf(os.Stderr, "%s Warning: Could not auto-patch opencode.json: %v\n", yellow("⚠"), patchErr)
			}
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
		return fmt.Errorf("failed to init hooks: %w", err)
	}

	fmt.Printf("%s rtk is now set up!\n\n", green("✓"))
	return nil
}

// patchOpenCodeConfig ensures the given opencode config file has the rtk plugin configured.
func patchOpenCodeConfig(opencodePath string) error {
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

	pluginsRaw, exists := config["plugin"]
	if !exists {
		config["plugin"] = []string{"~/.config/opencode/plugins/rtk.ts"}
	} else {
		// Handle the case where 'plugin' is a string instead of an array
		if singleStr, ok := pluginsRaw.(string); ok {
			config["plugin"] = []string{singleStr, "~/.config/opencode/plugins/rtk.ts"}
		} else if plugins, ok := pluginsRaw.([]interface{}); ok {
			hasPlugin := false
			for _, p := range plugins {
				if str, ok := p.(string); ok && str == "~/.config/opencode/plugins/rtk.ts" {
					hasPlugin = true
					break
				}
			}

			if !hasPlugin {
				config["plugin"] = append(plugins, "~/.config/opencode/plugins/rtk.ts")
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
