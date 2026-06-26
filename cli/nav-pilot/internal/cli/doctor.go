package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
)

// cmdDoctor runs system health checks and outputs actionable diagnostics.
func cmdDoctor() error {
	fmt.Printf("%s\n\n", bold("nav-pilot doctor"))
	hasErrors := false

	// 1. Configuration
	configPath := filepath.Join(os.Getenv("HOME"), ".nav-pilot", "config.toml")
	fmt.Printf("[i] Configuration (%s)\n", configPath)
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Printf("    • File not found (using default values)\n")
			fmt.Printf("      %s To create configuration, run: %s\n\n", yellow("Solution:"), bold("nav-pilot config init"))
		} else {
			hasErrors = true
			fmt.Printf("    %s Error reading config: %v\n\n", red("[✗]"), err)
		}
	} else {
		var cfg Config
		_, parseErr := toml.Decode(string(data), &cfg)
		if parseErr != nil {
			hasErrors = true
			fmt.Printf("    %s TOML parse error: %v\n", red("[✗]"), parseErr)
			fmt.Printf("      %s Fix syntax in %s or run %s\n\n", red("Solution:"), configPath, bold("nav-pilot config validate"))
		} else {
			fmt.Printf("    %s Valid syntax and known keys\n\n", green("✓"))
		}
	}

	// 2. Context Installation
	fmt.Printf("[i] Context Installation\n")
	userScope, userErr := ScopeUser()
	var userState *StateFile
	if userErr != nil {
		hasErrors = true
		fmt.Printf("    • User scope (~/.copilot): %s Failed to determine user home: %v\n", red("[✗]"), userErr)
	} else {
		userState, _ = readScopedState(userScope)
		if userState != nil {
			ok, modified, missing, _, _ := countFileIntegrity(userScope.RootDir, userState)
			if missing > 0 || modified > 0 {
				hasErrors = true
				fmt.Printf("    • User scope (~/.copilot): %q collection\n", userState.Collection)
				fmt.Printf("      %s %d missing files, %d modified\n", red("[✗]"), missing, modified)
				fmt.Printf("          %s Run %s to restore missing files.\n", red("Solution:"), bold("nav-pilot sync"))
			} else {
				fmt.Printf("    • User scope (~/.copilot): %q collection\n", userState.Collection)
				fmt.Printf("      %s %d files OK\n", green("✓"), ok)
			}
		} else {
			fmt.Printf("    • User scope (~/.copilot): Not installed\n")
		}
	}

	repoDir, err := os.Getwd()
	var repoState *StateFile
	if err != nil {
		hasErrors = true
		fmt.Printf("    • Repo scope (.github): %s Failed to determine current directory: %v\n", red("[✗]"), err)
	} else {
		repoScope := ScopeRepo(repoDir)
		repoState, _ = readScopedState(repoScope)
		if repoState != nil {
			ok, modified, missing, _, _ := countFileIntegrity(repoScope.RootDir, repoState)
			if missing > 0 || modified > 0 {
				hasErrors = true
				fmt.Printf("    • Repo scope (.github): %q collection\n", repoState.Collection)
				fmt.Printf("      %s %d missing files, %d modified\n", red("[✗]"), missing, modified)
				fmt.Printf("          %s Run %s to restore missing files.\n", red("Solution:"), bold("nav-pilot sync"))
			} else {
				fmt.Printf("    • Repo scope (.github): %q collection\n", repoState.Collection)
				fmt.Printf("      %s %d files OK\n", green("✓"), ok)
			}
		} else {
			fmt.Printf("    • Repo scope (.github): Not installed\n")
		}
	}
	if userState == nil && repoState == nil {
		fmt.Printf("      %s Run %s to install a collection.\n", yellow("Solution:"), bold("nav-pilot install <collection>"))
	}
	fmt.Println()

	// 3. Client Agents
	fmt.Printf("[i] Client Agents\n")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// copilot (cplt)
	fmt.Printf("    • copilot (cplt)\n")
	cpltPath, _ := exec.LookPath("cplt")
	if cpltPath == "" {
		hasErrors = true
		fmt.Printf("      %s Binary not found on PATH\n", red("[✗]"))
		fmt.Printf("          %s Install cplt via Homebrew: %s\n", red("Solution:"), bold("brew install navikt/tap/cplt"))
	} else {
		versionOut, err := exec.CommandContext(ctx, cpltPath, "--version").Output()
		version := strings.TrimSpace(string(versionOut))
		if err != nil {
			version = "unknown"
		}
		fmt.Printf("      %s Binary found: %s (%s)\n", green("✓"), cpltPath, version)

		// Check pinning
		cfgOut, _ := exec.CommandContext(ctx, cpltPath, "config", "show").CombinedOutput()
		if strings.Contains(string(cfgOut), "nav-pilot") {
			fmt.Printf("      %s Agent properly pinned to nav-pilot\n", green("✓"))
		} else {
			hasErrors = true
			fmt.Printf("      %s Agent not pinned to nav-pilot\n", red("[✗]"))
			fmt.Printf("          %s Set agent alias via %s\n", red("Solution:"), bold("cplt config set copilot.agent_name nav-pilot"))
		}
	}

	// opencode
	fmt.Printf("    • opencode\n")
	ocPath, _ := exec.LookPath("opencode")
	if ocPath == "" {
		fmt.Printf("      [i] Binary not found on PATH (optional)\n")
	} else {
		fmt.Printf("      %s Binary found: %s\n", green("✓"), ocPath)
		// Check opencode context
		configDir, err := os.UserConfigDir()
		if err != nil {
			configDir = filepath.Join(os.Getenv("HOME"), ".config")
		}
		ocDir := filepath.Join(configDir, "opencode")
		ocScope := &InstallScope{Name: "opencode", RootDir: ocDir, StateFile: ".nav-pilot-state.json"}
		ocState, _ := readScopedState(ocScope)
		if ocState != nil {
			ok, _, missing, _, _ := countFileIntegrity(ocDir, ocState)
			if missing > 0 {
				hasErrors = true
				fmt.Printf("      %s Context is missing %d files\n", red("[✗]"), missing)
				fmt.Printf("          %s Run %s to fix.\n", red("Solution:"), bold("nav-pilot sync"))
			} else {
				fmt.Printf("      %s Context securely materialized (%d files OK)\n", green("✓"), ok)
			}
		} else {
			fmt.Printf("      [i] Context not initialized yet\n")
		}
	}

	// pi
	fmt.Printf("    • pi\n")
	piPath, _ := exec.LookPath("pi")
	if piPath == "" {
		fmt.Printf("      [i] Binary not found on PATH (optional)\n")
	} else {
		fmt.Printf("      %s Binary found: %s\n", green("✓"), piPath)
	}
	fmt.Println()

	// 4. Project Security
	fmt.Printf("[i] Project Security (.cplt.toml)\n")
	if cpltPath != "" {
		cfgOut, _ := exec.CommandContext(ctx, cpltPath, "config", "show").CombinedOutput()
		if strings.Contains(string(cfgOut), "pending approval") || strings.Contains(string(cfgOut), "pending") {
			hasErrors = true
			fmt.Printf("    %s Pending permissions detected!\n", red("[✗]"))
			fmt.Printf("        %s Run %s in this directory to approve new sandbox rules.\n", red("Solution:"), bold("cplt trust"))
		} else {
			if _, err := os.Stat(".cplt.toml"); err == nil {
				fmt.Printf("    %s .cplt.toml rules are trusted\n", green("✓"))
			} else {
				fmt.Printf("    • No .cplt.toml found in current directory\n")
			}
		}
	} else {
		fmt.Printf("    • Skipped (cplt not installed)\n")
	}
	fmt.Println()

	// 5. Dependencies
	fmt.Printf("[i] Dependencies\n")
	checkDep := func(name string) {
		p, _ := exec.LookPath(name)
		if p == "" {
			hasErrors = true
			fmt.Printf("    %s %s: Not found on PATH\n", red("[✗]"), name)
			fmt.Printf("        %s Install %s to use nav-pilot fully.\n", red("Solution:"), name)
		} else {
			fmt.Printf("    %s %s: OK\n", green("✓"), name)
		}
	}
	checkDep("rtk")
	checkDep("git")
	fmt.Println()

	if hasErrors {
		fmt.Printf("%s Health check complete with warnings. See solutions above.\n", yellow("⚠"))
	} else {
		fmt.Printf("%s All systems healthy! 🚀\n", green("✓"))
	}

	return nil
}
