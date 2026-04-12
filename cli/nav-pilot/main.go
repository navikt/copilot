// nav-pilot installs a nav-pilot collection into the current repository.
// It copies agents, skills, instructions, and prompts from navikt/copilot
// and tracks installed state for safe updates and uninstall.
//
// Usage:
//
//	nav-pilot install <collection>     # install a collection
//	nav-pilot install -n <collection>  # dry-run
//	nav-pilot list                     # list available collections
//	nav-pilot status                   # show installed state
//	nav-pilot uninstall                # remove installed files
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
	fmt.Fprintf(os.Stderr, `nav-pilot — collection installer for Nav's Copilot toolkit

Usage:
  nav-pilot <command> [flags]

Commands:
  install <collection>    Install a collection into the current repo
  list                    List available collections
  status                  Show what's currently installed
  uninstall               Remove installed collection files
  version                 Show version information

Flags (install/uninstall):
  -n, --dry-run           Show what would happen without making changes
  -f, --force             Overwrite files that differ from source
  -t, --target <dir>      Target repository (default: current directory)
  -r, --ref <ref>         Git branch or tag to install from (default: main)

Examples:
  nav-pilot install kotlin-backend
  nav-pilot install --ref nav-pilot kotlin-backend
  nav-pilot install --dry-run fullstack
  nav-pilot install --force fullstack
  nav-pilot list
  nav-pilot status
  nav-pilot uninstall
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

	var dryRun, force bool
	var targetDir, ref string
	var positional []string

	targetDir = "."

	for i := 0; i < len(rest); i++ {
		switch rest[i] {
		case "-n", "--dry-run":
			dryRun = true
		case "-f", "--force":
			force = true
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
			return fmt.Errorf("install requires a collection name. Run 'list' to see available collections")
		}
		return cmdInstall(positional[0], targetDir, ref, dryRun, force)
	case "list":
		return cmdList(ref)
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
		fmt.Fprintf(os.Stderr, "\n%s %v\n", red("Error:"), err)
		os.Exit(1)
	}
}

