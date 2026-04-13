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
	fmt.Fprintf(os.Stderr, `nav-pilot — manage Nav's Copilot toolkit

Usage:
  nav-pilot <command> [flags]

Commands:
  install <collection>    Install a curated collection into the current repo
  add <type> <name>       Install a single agent, skill, instruction, or prompt
  sync                    Check for updates and optionally apply them
  list                    List available collections and items
  status                  Show what's currently installed
  uninstall               Remove installed collection files
  version                 Show version information

Flags:
  -n, --dry-run           Show what would happen without making changes
  -f, --force             Overwrite files that differ from source
  -t, --target <dir>      Target repository (default: current directory)
  -r, --ref <ref>         Git branch or tag to install from
  -s, --source <repo>     Source repository (default: navikt/copilot)
  --apply                 Apply available updates (sync only)
  --json                  Output results as JSON (sync only)

Examples:
  nav-pilot install kotlin-backend
  nav-pilot add agent security-champion
  nav-pilot add skill postgresql-review
  nav-pilot sync --apply
  nav-pilot list --items
`)
}

// run parses args and dispatches to the appropriate command.
// It returns an error instead of calling os.Exit, making it testable.
func run(args []string) error {
	if len(args) < 1 {
		usage()
		return fmt.Errorf("no command specified")
	}

	command := args[0]
	rest := args[1:]

	var dryRun, force, apply, jsonOutput, listItems bool
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
		case "-t", "--target":
			if i+1 >= len(rest) {
				return fmt.Errorf("--target requires a value")
			}
			i++
			targetDir = rest[i]
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

	if abs, err := filepath.Abs(targetDir); err == nil {
		targetDir = abs
	}

	switch command {
	case "install":
		if len(positional) == 0 {
			return fmt.Errorf("install requires a collection name. Run 'nav-pilot list' to see available collections")
		}
		return cmdInstall(positional[0], targetDir, ref, dryRun, force)
	case "add":
		if len(positional) < 2 {
			return fmt.Errorf("add requires a type and name.\n\nUsage: nav-pilot add <type> <name>\n\nTypes: agent, skill, instruction, prompt\n\nExamples:\n  nav-pilot add agent security-champion\n  nav-pilot add skill postgresql-review")
		}
		return cmdAdd(positional[0], positional[1], targetDir, ref, dryRun, force)
	case "sync":
		return cmdSync(targetDir, ref, sourceRepo, apply, jsonOutput)
	case "list":
		return cmdList(ref, listItems)
	case "status":
		return cmdStatus(targetDir)
	case "uninstall":
		return cmdUninstall(targetDir, dryRun)
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

