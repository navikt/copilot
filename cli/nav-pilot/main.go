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
	"fmt"
	"os"
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

// ─── CLI ────────────────────────────────────────────────────────────────────

func usage() {
	fmt.Fprintf(os.Stderr, `nav-pilot — Nav's Copilot toolkit for developers

Installs curated agents, skills, instructions, and prompts that teach
GitHub Copilot Nav's platform, patterns, and conventions.

Usage:
  nav-pilot <command> [flags]

Commands:
  install <name>          Install a collection or individual agent/skill/instruction/prompt
  install --user --all    Install all agents, skills & instructions to ~/.copilot (user-wide)
  init                    Scaffold repo-local Copilot config files (AGENTS.md, instructions)
  sync                    Check for updates and optionally apply them
  list                    List available collections and items
  list --installed        Show what's currently installed
  upgrade                 Update nav-pilot CLI to the latest version
  uninstall               Remove installed collection files
  export <format>         Export Nav customizations to another tool's format
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
  -u, --user              Install to ~/.copilot (user-wide, all agents, skills & instructions)
  --type <type>           Artifact type for install (agent, skill, instruction, prompt)
  --all                   Install everything (use with --user)
  --apply                 Apply available updates (sync only)
  --json                  Output results as JSON
  -F, --feature           Submit a feature request (feedback only)

Get started:
  nav-pilot                              # Install, upgrade, or launch Copilot sandbox (cplt)
  nav-pilot init                         # Scaffold AGENTS.md and Copilot instructions
  nav-pilot list                         # See available collections and items
  nav-pilot install kotlin-backend       # Install a collection
  nav-pilot install security-champion    # Install a single agent
  nav-pilot install --dry-run fullstack  # Preview before installing
  nav-pilot export opencode              # Export for OpenCode/oh-my-openagent

After installing, use @nav-pilot in GitHub Copilot Chat.
`)
}

// run parses args and dispatches to the appropriate command.
// It returns an error instead of calling os.Exit, making it testable.
func run(args []string) error {
	// Self-check: warn if nav-pilot binary is outdated (fast, cached)
	if version != "dev" {
		if latest := checkStaleness(version); latest != "" {
			fmt.Fprintf(os.Stderr, "%s nav-pilot %s available (current: %s) — run %s to upgrade\n",
				yellow("⚠"), latest, version, bold("nav-pilot upgrade"))
		}
	}

	if len(args) < 1 {
		if isInteractive() {
			return cmdInteractive()
		}
		usage()
		return nil
	}

	command := args[0]
	rest := args[1:]

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
		if userScope && (len(positional) == 0 || installAll) {
			if len(positional) == 0 && !installAll {
				fmt.Fprintf(os.Stderr, "%s In a future version, %s alone will require %s.\n  Run: %s\n\n",
					yellow("⚠"), bold("install --user"), bold("--all"), bold("nav-pilot install --user --all"))
			}
			return cmdInstallAll(scope, ref, sourceRepo, dryRun, force, jsonOutput)
		}
		if len(positional) == 0 {
			return fmt.Errorf("install requires a name. Run 'nav-pilot list' to see available collections and items")
		}
		if len(positional) > 1 {
			return fmt.Errorf("install takes one name. Did you mean: nav-pilot install %s --type %s", positional[1], positional[0])
		}
		return cmdInstallAuto(positional[0], installType, scope, ref, sourceRepo, dryRun, force, jsonOutput)
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
		return cmdSync(scope, ref, sourceRepo, apply, jsonOutput)
	case "list":
		if listInstalled {
			if userScope || targetProvided {
				return cmdStatusScoped(scope, false, jsonOutput)
			}
			return cmdStatusAuto(targetDir, jsonOutput)
		}
		return cmdList(ref, sourceRepo, listItems, jsonOutput)
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
		return cmdUpdate()
	case "update":
		// Deprecated: hidden alias for backward compatibility
		if !jsonOutput {
			fmt.Fprintf(os.Stderr, "%s %s is deprecated. Use: %s\n\n",
				yellow("⚠"), bold("nav-pilot update"), bold("nav-pilot upgrade"))
		}
		return cmdUpdate()
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
	if err := run(os.Args[1:]); err != nil {
		if err == errUpdatesAvailable {
			os.Exit(1)
		}
		if err == errSyncFailed {
			os.Exit(2)
		}
		fmt.Fprintf(os.Stderr, "\n%s %v\n", red("Error:"), err)
		os.Exit(1)
	}
}
