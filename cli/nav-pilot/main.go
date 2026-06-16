// nav-pilot manages Nav's Copilot toolkit — agents, skills, instructions, and prompts.
// It installs curated collections or individual items from navikt/copilot
// and tracks installed state for safe updates, sync, and uninstall.
//
// Usage:
//
//	nav-pilot install <name>               # install a collection or single artifact
//	nav-pilot sync                         # check for updates
//	nav-pilot list                         # list available collections and items
//	nav-pilot list --installed             # show installed state
//	nav-pilot uninstall                    # remove installed files
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// ─── Version (injected at build time) ───────────────────────────────────────

var (
	version   = "dev"
	commit    = "unknown"
	buildDate = "unknown"
)

// timeNow is a variable so tests can override it.
var timeNow = time.Now
var telemetry telemetryRecorder = noopTelemetry{}

// ─── CLI ────────────────────────────────────────────────────────────────────

// commandAliases maps short aliases to their canonical command names.
var commandAliases = map[string]string{
	"i":  "install",
	"ls": "list",
	"s":  "sync",
	"up": "upgrade",
	"rm": "uninstall",
}

func isKnownCommand(arg string) bool {
	if _, ok := commandAliases[arg]; ok {
		return true
	}
	switch arg {
	case "install", "init", "export", "add", "ignore", "sync", "list", "status",
		"uninstall", "upgrade", "update", "config", "env", "feedback", "version",
		"--version", "-v", "-h", "--help", "help":
		return true
	default:
		return false
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, `nav-pilot — Nav's Copilot toolkit

CLI tool that installs agents, skills, and instructions for GitHub Copilot.
Once installed, use @nav-pilot in Copilot Chat to plan and build Nav apps.

Usage:
  nav-pilot <command> [flags]

Commands:
  install (i) <name>      Install a collection or individual agent/skill/instruction/prompt
  install --user --all    Install all agents, skills & instructions to ~/.copilot (user-wide)
  init                    Scaffold repo-local Copilot config files (AGENTS.md, instructions)
  sync (s)                Check for updates and optionally apply them
  list (ls)               List available collections and items
  list --installed        Show what's currently installed
  upgrade (up)            Update nav-pilot CLI to the latest version
  uninstall (rm)          Remove installed collection files
  export <format>         Export Nav customizations to another tool's format
  config <subcommand>     Manage user-specific nav-pilot configuration (init, setup, show, get, set, validate)
  env                     Print shell exports for Copilot CLI integration
  ignore <type> <name>    Suppress new-item reminders for a specific item (--user)
  feedback                Report a bug or request a feature
  version                 Show version information

Flags:
  -n, --dry-run           Show what would happen without making changes
  -f, --force             Overwrite files that differ from source
  -t, --target <dir>      Target repository (default: current directory)
  -r, --ref <ref>         Git branch or tag to install from
  -s, --source <repo>     Source repository (default: navikt/copilot)
  -u, --user              Install to ~/.copilot — works across all repos (agents, skills & instructions only)
  --type <type>           Artifact type for install (agent, skill, instruction, prompt)
  --all                   Install everything (use with --user)
  --apply                 Apply available updates (sync only)
  --sync                  Sync all scopes and launch Copilot (non-interactive)
  --json                  Output results as JSON
  -F, --feature           Submit a feature request (feedback only)

Get started:
  nav-pilot                              # Interactive: install, upgrade, or launch Copilot
  nav-pilot list                         # See available collections and items
  nav-pilot install kotlin-backend       # Install a collection to .github/
  nav-pilot install --user --all         # Install everything to ~/.copilot (all repos)
  nav-pilot install security-champion    # Install a single agent
  nav-pilot sync                         # Check for updates
  nav-pilot export opencode              # Export for OpenCode/oh-my-openagent

After installing, use @nav-pilot in GitHub Copilot Chat.
`)
}

// run parses args and dispatches to the appropriate command.
// It returns an error instead of calling os.Exit, making it testable.
func run(args []string) error {
	// Self-check: warn if nav-pilot binary is outdated (fast, cached)
	assessment := assessStaleness(version)
	recordFreshness("cli", "none", assessment)
	if version != "dev" && assessment.LatestVersion != "" && versionNewer(assessment.LatestVersion, version) {
		fmt.Fprintf(os.Stderr, "%s nav-pilot %s available (current: %s) — run %s to upgrade\n",
			yellow("⚠"), assessment.LatestVersion, version, bold("nav-pilot upgrade"))
	}

	// Pre-scan: extract launch-override flags before command dispatch.
	// These apply to the interactive flow and --sync launch, not to subcommands.
	var cliOverrides CLIOverrides
	if len(args) == 0 || args[0] == "--sync" || !isKnownCommand(args[0]) {
		var cleanArgs []string
		for i := 0; i < len(args); i++ {
			switch args[i] {
			case "--client":
				if i+1 >= len(args) {
					return fmt.Errorf("--client requires a value")
				}
				i++
				cliOverrides.Client = args[i]
			case "--agent":
				return fmt.Errorf("--agent is no longer a nav-pilot flag; use --client to choose the coding-agent CLI (copilot, opencode, pi) — the downstream copilot --agent persona is unaffected")
			case "--model":
				if i+1 >= len(args) {
					return fmt.Errorf("--model requires a value")
				}
				i++
				cliOverrides.Model = args[i]
				if err := validateModelValue(cliOverrides.Model); err != nil {
					return err
				}
			case "--mode":
				if i+1 >= len(args) {
					return fmt.Errorf("--mode requires a value")
				}
				i++
				v := args[i]
				if !containsStr(validModes, v) {
					return fmt.Errorf("--mode %q is not valid (allowed: %s)", v, strings.Join(validModes, ", "))
				}
				cliOverrides.Mode = v
			case "--effort":
				if i+1 >= len(args) {
					return fmt.Errorf("--effort requires a value")
				}
				i++
				v := args[i]
				if !containsStr(validReasoningEffort, v) {
					return fmt.Errorf("--effort %q is not valid (allowed: %s)", v, strings.Join(validReasoningEffort, ", "))
				}
				cliOverrides.ReasoningEffort = v
			case "--context":
				if i+1 >= len(args) {
					return fmt.Errorf("--context requires a value")
				}
				i++
				v := args[i]
				if !containsStr(validContextTiers, v) {
					return fmt.Errorf("--context %q is not valid (allowed: %s)", v, strings.Join(validContextTiers, ", "))
				}
				cliOverrides.ContextTier = v
			case "--log-level":
				if i+1 >= len(args) {
					return fmt.Errorf("--log-level requires a value")
				}
				i++
				v := args[i]
				if !containsStr(validLogLevels, v) {
					return fmt.Errorf("--log-level %q is not valid (allowed: %s)", v, strings.Join(validLogLevels, ", "))
				}
				cliOverrides.LogLevel = v
			case "--otel-log-level":
				if i+1 >= len(args) {
					return fmt.Errorf("--otel-log-level requires a value")
				}
				i++
				v := args[i]
				if !containsStr(validOtelLogLevels, v) {
					return fmt.Errorf("--otel-log-level %q is not valid (allowed: %s)", v, strings.Join(validOtelLogLevels, ", "))
				}
				cliOverrides.OtelLogLevel = v
			case "--allow-all-tools":
				t := true
				cliOverrides.AllowAllTools = &t
			case "--no-allow-all-tools":
				f := false
				cliOverrides.AllowAllTools = &f
			case "--ask-user":
				t := true
				cliOverrides.AskUser = &t
			case "--no-ask-user":
				f := false
				cliOverrides.AskUser = &f
			default:
				cleanArgs = append(cleanArgs, args[i])
			}
		}
		args = cleanArgs
	}

	if cliOverrides.Client != "" && !containsStr(validClients, cliOverrides.Client) {
		return fmt.Errorf("--client %q is not valid (allowed: %s)", cliOverrides.Client, strings.Join(validClients, ", "))
	}

	if len(args) < 1 {
		if isInteractive() {
			return runWithCommandTelemetry("startup", telemetryMode(), "auto", func() error {
				return cmdInteractive(cliOverrides)
			})
		}
		usage()
		return nil
	}

	// Handle --sync flag: non-interactive sync-all + launch
	if args[0] == "--sync" {
		if isInteractive() {
			if err := maybeRunFirstRunSetup(); err != nil {
				fmt.Fprintf(os.Stderr, "%s Config setup failed: %v\n", yellow("⚠"), err)
			}
		}
		if err := runWithCommandTelemetry("sync", "non_interactive", "auto", func() error {
			return cmdSyncAuto(".", "", "", true, false)
		}); err != nil && err != errUpdatesAvailable {
			fmt.Fprintf(os.Stderr, "%s Sync failed: %v\n", yellow("⚠"), err)
		}
		resolved, cfgErr := loadConfigForLaunch(cliOverrides)
		if cfgErr != nil {
			return cfgErr
		}
		launchErr := runWithCommandTelemetry("launch", "non_interactive", "none", func() error {
			return launchClient(resolved)
		})
		return launchErr
	}

	command := args[0]
	rest := args[1:]

	// Resolve short aliases to canonical command names.
	if canonical, ok := commandAliases[command]; ok {
		command = canonical
	}

	var dryRun, force, apply, jsonOutput, listItems, featureRequest, userScope, targetProvided, installAll, listInstalled bool
	var targetDir, ref, sourceRepo, installType string
	var positional []string

	targetDir = "."

	for i := 0; i < len(rest); i++ {
		switch rest[i] {
		case "-n", "--dry-run":
			dryRun = true
		case "-f", "--force":
			force = true
		case "--apply":
			apply = true
		case "--json":
			jsonOutput = true
		case "--items":
			listItems = true
		case "--installed":
			listInstalled = true
		case "--all":
			installAll = true
		case "-F", "--feature":
			featureRequest = true
		case "-u", "--user":
			userScope = true
		case "-t", "--target":
			if i+1 >= len(rest) {
				return fmt.Errorf("--target requires a value")
			}
			i++
			targetDir = rest[i]
			targetProvided = true
		case "-r", "--ref":
			if i+1 >= len(rest) {
				return fmt.Errorf("--ref requires a value")
			}
			i++
			ref = rest[i]
		case "-s", "--source":
			if i+1 >= len(rest) {
				return fmt.Errorf("--source requires a value")
			}
			i++
			sourceRepo = rest[i]
		case "--type":
			if i+1 >= len(rest) {
				return fmt.Errorf("--type requires a value")
			}
			i++
			installType = rest[i]
		case "-h", "--help":
			usage()
			return nil
		default:
			if strings.HasPrefix(rest[i], "-") {
				return fmt.Errorf("unknown flag: %s", rest[i])
			}
			positional = append(positional, rest[i])
		}
	}

	if userScope && targetProvided {
		return fmt.Errorf("--user and --target are mutually exclusive")
	}

	if abs, err := filepath.Abs(targetDir); err == nil {
		targetDir = abs
	}

	// Build scope
	var scope *InstallScope
	if userScope {
		var err error
		scope, err = ScopeUser()
		if err != nil {
			return fmt.Errorf("resolving user home: %w", err)
		}
	} else {
		// For repo scope without explicit --target, resolve to git root
		// so commands work from any subdirectory.
		if !targetProvided {
			if root := findGitRoot(targetDir); root != "" {
				targetDir = root
			}
		}
		scope = ScopeRepo(targetDir)
	}

	// Reject --user for commands that don't support scoped installs
	if userScope {
		switch command {
		case "install", "add", "ignore", "sync", "status", "uninstall", "export", "list":
			// These commands support --user
		default:
			return fmt.Errorf("--user is not supported for %q", command)
		}
	}

	// Validate --type is only used with install (or hidden add alias)
	if installType != "" && command != "install" && command != "add" {
		return fmt.Errorf("--type is only supported for the install command")
	}

	switch command {
	case "install":
		return runWithCommandTelemetry("install", telemetryMode(), scope.Name, func() error {
			if userScope && (len(positional) == 0 || installAll) {
				return cmdInstallAll(scope, ref, sourceRepo, dryRun, force, jsonOutput)
			}
			if len(positional) == 0 {
				// No args: launch interactive flow if in a terminal
				if isInteractive() && !jsonOutput {
					return cmdInstallInteractive(targetDir, ref, sourceRepo)
				}
				return fmt.Errorf("install requires a name. Run 'nav-pilot list' to see available collections and items")
			}
			if len(positional) > 1 {
				return fmt.Errorf("install takes one name. Did you mean: nav-pilot install %s --type %s", positional[1], positional[0])
			}
			return cmdInstallAuto(positional[0], installType, scope, ref, sourceRepo, dryRun, force, jsonOutput)
		})
	case "init":
		return cmdInit(targetDir, dryRun, force)
	case "export":
		if len(positional) == 0 {
			return fmt.Errorf("export requires a format.\n\nUsage: nav-pilot export <format>\n\nFormats: opencode")
		}
		return cmdExport(positional[0], scope, ref, sourceRepo, dryRun, force, jsonOutput)
	case "add":
		// Deprecated: hidden alias for backward compatibility
		if !jsonOutput {
			if len(positional) >= 2 {
				fmt.Fprintf(os.Stderr, "%s %s is deprecated. Use: %s\n\n",
					yellow("⚠"), bold("nav-pilot add"), bold(fmt.Sprintf("nav-pilot install %s --type %s", positional[1], positional[0])))
			} else {
				fmt.Fprintf(os.Stderr, "%s %s is deprecated. Use: %s\n\n",
					yellow("⚠"), bold("nav-pilot add"), bold("nav-pilot install <name>"))
			}
		}
		if len(positional) < 2 {
			return fmt.Errorf("add requires a type and name.\n\nUsage: nav-pilot add <type> <name>\n\nTypes: agent, skill, instruction, prompt\n\nExamples:\n  nav-pilot add agent security-champion\n  nav-pilot add skill postgresql-review")
		}
		return cmdAdd(positional[0], positional[1], scope, ref, sourceRepo, dryRun, force, jsonOutput)
	case "ignore":
		if len(positional) < 2 {
			return fmt.Errorf("ignore requires a type and name.\n\nUsage: nav-pilot ignore <type> <name> --user\n\nTypes: agent, skill, instruction\n\nExamples:\n  nav-pilot ignore instruction nextjs-aksel --user\n  nav-pilot ignore agent security-champion --user")
		}
		return cmdIgnore(positional[0], positional[1], scope, jsonOutput)
	case "sync":
		syncScope := "auto"
		if userScope || targetProvided {
			syncScope = scope.Name
		}
		return runWithCommandTelemetry("sync", telemetryMode(), syncScope, func() error {
			if userScope || targetProvided {
				return cmdSync(scope, ref, sourceRepo, apply, jsonOutput)
			}
			return cmdSyncAuto(targetDir, ref, sourceRepo, apply, jsonOutput)
		})
	case "list":
		listScope := "none"
		if userScope || targetProvided {
			listScope = scope.Name
		}
		return runWithCommandTelemetry("list", telemetryMode(), listScope, func() error {
			if listInstalled {
				if userScope || targetProvided {
					return cmdStatusScoped(scope, false, jsonOutput)
				}
				return cmdStatusAuto(targetDir, jsonOutput)
			}
			return cmdList(ref, sourceRepo, listItems, jsonOutput)
		})
	case "status":
		// Deprecated: hidden alias for backward compatibility
		if !jsonOutput {
			fmt.Fprintf(os.Stderr, "%s %s is deprecated. Use: %s\n\n",
				yellow("⚠"), bold("nav-pilot status"), bold("nav-pilot list --installed"))
		}
		if userScope || targetProvided {
			return cmdStatusScoped(scope, false, jsonOutput)
		}
		return cmdStatusAuto(targetDir, jsonOutput)
	case "uninstall":
		return cmdUninstall(scope, dryRun)
	case "upgrade":
		return runWithCommandTelemetry("upgrade", telemetryMode(), "none", cmdUpdate)
	case "update":
		// Deprecated: hidden alias for backward compatibility
		if !jsonOutput {
			fmt.Fprintf(os.Stderr, "%s %s is deprecated. Use: %s\n\n",
				yellow("⚠"), bold("nav-pilot update"), bold("nav-pilot upgrade"))
		}
		return cmdUpdate()
	case "config":
		return cmdConfig(positional, jsonOutput)
	case "env":
		return cmdEnv()
	case "feedback":
		return cmdFeedback(targetDir, featureRequest)
	case "version", "--version", "-v":
		fmt.Printf("nav-pilot %s (commit: %s, built: %s)\n", version, commit, buildDate)
		return nil
	case "-h", "--help", "help":
		usage()
		return nil
	default:
		return fmt.Errorf("unknown command: %s. Run with --help for usage", command)
	}
}

func main() {
	tel, err := initTelemetry(context.Background(), version)
	if err != nil {
		debugLog("telemetry disabled: %v", err)
	}
	telemetry = tel

	exitCode := 0
	if err := run(os.Args[1:]); err != nil {
		exitCode = exitCodeFor(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = telemetry.Shutdown(ctx)
	if exitCode != 0 {
		os.Exit(exitCode)
	}
}

// exitCodeFor maps a run error to an exit code.
// ExitErrors from child processes are propagated transparently (no extra printing).
// Known sentinel errors use fixed codes. Everything else prints a red error line.
func exitCodeFor(err error) int {
	if err == nil {
		return 0
	}
	if err == errUpdatesAvailable {
		return 1
	}
	if err == errSyncFailed {
		return 2
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}
	fmt.Fprintf(os.Stderr, "\n%s %v\n", red("Error:"), err)
	return 1
}
