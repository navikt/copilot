package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	providerpkg "github.com/navikt/copilot/cli/nav-pilot/internal/provider"
	"github.com/navikt/copilot/cli/nav-pilot/internal/source"
	telemetrypkg "github.com/navikt/copilot/cli/nav-pilot/internal/telemetry"
)

// cpltTrust is `cplt trust show --json`, version 1 (navikt/cplt#644): the
// launch gate's own verdict on the repository's .cplt.toml.
type cpltTrust struct {
	Version     int     `json:"version"`
	State       string  `json:"state"` // none approved pending changed foreign outlived uncommitted invalid
	ContentHash *string `json:"content_hash"`
	Message     string  `json:"message"`
	Proposed    []struct {
		Key      string `json:"key"`
		Detail   string `json:"detail"`
		Effect   string `json:"effect"`
		Approved bool   `json:"approved"`
	} `json:"proposed"`
	// Command is set only when `cplt trust accept` would succeed.
	Command *string `json:"command"`
}

// parseCpltTrust reads the output of `cplt trust show --json`. ok is false
// for anything but a version 1 object: an older cplt without the flag, or a
// newer format nav-pilot does not know. Callers then fall back.
func parseCpltTrust(out []byte, err error) (t cpltTrust, ok bool) {
	if err != nil || json.Unmarshal(out, &t) != nil || t.Version != 1 || t.State == "" {
		return cpltTrust{}, false
	}
	return t, true
}

// readCpltTrust runs `cplt trust show --json` in dir, bounded.
func readCpltTrust(cpltPath, dir string) (cpltTrust, bool) {
	return parseCpltTrust(runBoundedIn(dir, cpltCommandTimeout, false, cpltPath, "trust", "show", "--json"))
}

// unapprovedKeys is the proposed keys the launch will not grant.
func (t cpltTrust) unapprovedKeys() []string {
	var keys []string
	for _, p := range t.Proposed {
		if !p.Approved {
			keys = append(keys, safe(p.Key, 64))
		}
	}
	return keys
}

// reportCpltTrust is doctor's .cplt.toml line from cplt's own verdict.
// Reports whether it found a problem.
func reportCpltTrust(t cpltTrust) bool {
	msg := safe(t.Message, 300)
	switch t.State {
	case "approved":
		fmt.Printf("    %s .cplt.toml rules are trusted\n", green("✓"))
		return false
	case "none":
		if t.ContentHash == nil { // no .cplt.toml at all
			return reportNoRepoConfig()
		}
		fmt.Printf("    • %s\n", msg)
		return false
	case "outlived":
		// The proposal is gone; a stale approval grants nothing.
		fmt.Printf("    %s %s\n", dim("-"), msg)
		printTrustCommand(t, dim("Tip:"))
		return false
	case "pending", "changed", "foreign":
		fmt.Printf("    %s %s\n", red("[✗]"), msg)
		for _, p := range t.Proposed {
			if !p.Approved {
				fmt.Printf("        ○ %s = %s: %s\n", safe(p.Key, 64), safe(p.Detail, 120), safe(p.Effect, 200))
			}
		}
		printTrustCommand(t, red("Solution:"))
		return true
	default: // uncommitted, invalid, and any state added later
		fmt.Printf("    %s %s\n", yellow("⚠"), msg)
		printTrustCommand(t, yellow("Solution:"))
		return true
	}
}

// printTrustCommand prints cplt's command, when it has one. Without one the
// message already said what is wrong.
func printTrustCommand(t cpltTrust, label string) {
	if t.Command != nil {
		fmt.Printf("        %s Run %s in this repository.\n", label, bold(safe(*t.Command, 80)))
	}
}

// trustSeenFile records, per repository root, the sha256 of the .cplt.toml
// the launch last looked at. An unchanged file costs a read and a hash, never
// a cplt spawn.
func trustSeenFile() (string, error) {
	dir, err := telemetrypkg.GetConfigDir()
	return filepath.Join(dir, "cplt-trust-seen.json"), err
}

func readTrustSeen() map[string]string {
	seen := map[string]string{}
	if p, err := trustSeenFile(); err == nil {
		if data, err := os.ReadFile(p); err == nil {
			_ = json.Unmarshal(data, &seen)
		}
	}
	return seen
}

func recordTrustSeen(seen map[string]string, root, sum string) {
	seen[root] = sum
	p, err := trustSeenFile()
	if err != nil {
		return
	}
	data, _ := json.Marshal(seen)
	_ = os.MkdirAll(filepath.Dir(p), 0o700)
	_ = os.WriteFile(p, data, 0o600)
}

// askTrustReview puts the question on stderr and reads one line. Only y or
// yes is a yes. A var so tests answer it.
var askTrustReview = func(prompt string) bool {
	fmt.Fprint(os.Stderr, prompt)
	// One byte at a time: a buffered reader would keep typed-ahead input
	// from cplt trust accept and the client.
	var line []byte
	b := make([]byte, 1)
	for {
		n, err := os.Stdin.Read(b)
		if n == 0 || err != nil || b[0] == '\n' {
			break
		}
		line = append(line, b[0])
	}
	a := strings.ToLower(strings.TrimSpace(string(line)))
	return a == "y" || a == "yes"
}

// runTrustAccept hands the terminal to plain `cplt trust accept` in root. It
// never passes --all, never pipes input, and shows no diff of its own: the
// review is cplt's. A var so tests see the call.
var runTrustAccept = func(cpltPath, root string) error {
	cmd := exec.Command(cpltPath, "trust", "accept")
	cmd.Dir = root
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return cmd.Run()
}

// maybeTrustNudge offers the review of a .cplt.toml that proposes sandbox
// changes, once per (repository, file hash), in a terminal and under cplt.
// Launch path: an unchanged file spawns nothing; cplt is asked only when the
// bytes differ from the ones recorded. Without a terminal it says nothing:
// cplt's own launch output names what it did not grant.
func maybeTrustNudge(projectDir string) {
	if !isInteractive() {
		return
	}
	if projectDir == "" {
		projectDir = "."
	}
	root := source.FindGitRoot(projectDir)
	if root == "" {
		return
	}
	data, err := os.ReadFile(filepath.Join(root, ".cplt.toml"))
	if err != nil {
		return
	}
	h := sha256.Sum256(data)
	sum := hex.EncodeToString(h[:])
	seen := readTrustSeen()
	if seen[root] == sum {
		return
	}
	if !cpltInstalled() {
		return
	}
	cpltPath, _ := providerpkg.FindCopilotCLI()
	t, ok := readCpltTrust(cpltPath, root)
	// An uncommitted file is asked about again once committed, with the
	// same bytes. ponytail: an older cplt or a timeout records the hash too,
	// so a slow cplt is not re-spawned every launch; doctor still reports.
	if ok && t.State == "uncommitted" {
		return
	}
	if ok && (t.State == "pending" || t.State == "changed") && t.Command != nil {
		sessionPrompted = true
		prompt := fmt.Sprintf("This repo's .cplt.toml proposes sandbox changes (%s). Review now? [y/N] ", strings.Join(t.unapprovedKeys(), ", "))
		if askTrustReview(prompt) {
			if err := runTrustAccept(cpltPath, root); err != nil {
				fmt.Fprintf(os.Stderr, "%s cplt trust accept: %v\n", yellow("⚠"), err)
			}
		}
	}
	recordTrustSeen(seen, root, sum)
}
