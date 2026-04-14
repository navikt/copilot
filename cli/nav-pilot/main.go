// nav-pilot manages Nav's Copilot toolkit — agents, skills, instructions, and prompts.
// It installs curated collections or individual items from navikt/copilot
// and tracks installed state for safe updates, sync, and uninstall.
//
// Usage:
//
//	nav-pilot install <collection>         # install a collection
//	nav-pilot add agent <name>             # install a single agent
//	nav-pilot sync                         # check for updates
//	nav-pilot list                         # list available collections and items
//	nav-pilot status                       # show installed state
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
  install <collection>    Install a curated collection into the current repo
  add <type> <name>       Install a single agent, skill, instruction, or prompt
  sync                    Check for updates and optionally apply them
  list                    List available collections and items
  status                  Show what's currently installed
  uninstall               Remove installed collection files
  update                  Update nav-pilot CLI to the latest version
  feedback                Report a bug or request a feature
  version                 Show version information

Flags:
  -n, --dry-run           Show what would happen without making changes
  -f, --force             Overwrite files that differ from source
  -t, --target <dir>      Target repository (default: current directory)
  -r, --ref <ref>         Git branch or tag to install from
  -s, --source <repo>     Source repository (default: navikt/copilot)
  -u, --user              Install to ~/.copilot (user-wide, agents and skills only)
  --apply                 Apply available updates (sync only)
  --json                  Output results as JSON (sync only)
  -F, --feature           Submit a feature request (feedback only)

Get started:
  nav-pilot                              # Install, upgrade, or launch Copilot sandbox (cplt)
  nav-pilot list                         # See available collections
  nav-pilot install kotlin-backend       # Install a collection
  nav-pilot install --dry-run fullstack  # Preview before installing

After installing, use @nav-pilot in GitHub Copilot Chat.
`)
}

// run parses args and dispatches to the appropriate command.
// It returns an error instead of calling os.Exit, making it testable.
func run(args []string) error {
	if len(args) < 1 {
		if isInteractive() {
			// Allow interactive mode if in a git repo or if user-scope install exists
			hasGitRepo := findGitRoot(".") != ""
			hasUserInstall := false
			if s, err := ScopeUser(); err == nil {
				// Check file existence (not parse success) so corrupted state
				// still reaches cmdInteractive() which can show a warning.
				if _, statErr := os.Stat(s.StatePath()); statErr == nil {
					hasUserInstall = true
				}
			}
			if hasGitRepo || hasUserInstall {
				return cmdInteractive()
			}
		}
		usage()
		return nil
	}

	command := args[0]
	rest := args[1:]

	var dryRun, force, apply, jsonOutput, listItems, featureRequest, userScope, targetProvided bool
	var targetDir, ref, sourceRepo string
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
		case "install", "add", "sync", "status", "uninstall":
			// These commands support --user
		default:
			return fmt.Errorf("--user is not supported for %q", command)
		}
	}

	switch command {
	case "install":
		if len(positional) == 0 {
			return fmt.Errorf("install requires a collection name. Run 'nav-pilot list' to see available collections")
		}
		return cmdInstall(positional[0], scope, ref, sourceRepo, dryRun, force)
	case "add":
		if len(positional) < 2 {
			return fmt.Errorf("add requires a type and name.\n\nUsage: nav-pilot add <type> <name>\n\nTypes: agent, skill, instruction, prompt\n\nExamples:\n  nav-pilot add agent security-champion\n  nav-pilot add skill postgresql-review")
		}
		return cmdAdd(positional[0], positional[1], scope, ref, sourceRepo, dryRun, force)
	case "sync":
		return cmdSync(scope, ref, sourceRepo, apply, jsonOutput)
	case "list":
		return cmdList(ref, sourceRepo, listItems)
	case "status":
		return cmdStatus(scope)
	case "uninstall":
		return cmdUninstall(scope, dryRun)
	case "update":
		return cmdUpdate()
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

