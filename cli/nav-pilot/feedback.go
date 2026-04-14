package main

import (
	"fmt"
	"net/url"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const feedbackRepo = "navikt/copilot"

// openBrowserFn is the function used to open a URL in the browser.
// It is a variable so tests can override it.
var openBrowserFn = openBrowser

func cmdFeedback(targetDir string, featureRequest bool) error {
	diag := collectDiagnostics(targetDir)

	kind := "bug report"
	if featureRequest {
		kind = "feature request"
	}

	issueURL := buildFeedbackURL(featureRequest, diag)

	fmt.Printf("Opening %s in browser...\n\n", kind)
	fmt.Println(dim("Diagnostics (included automatically):"))
	for _, line := range strings.Split(diag, "\n") {
		if line != "" {
			fmt.Printf("  %s\n", line)
		}
	}
	fmt.Println()

	if err := openBrowserFn(issueURL); err != nil {
		fmt.Println(dim("Could not open browser. Open this URL manually:"))
		fmt.Println()
		fmt.Println(issueURL)
	}
	return nil
}

// collectDiagnostics gathers system and installation info for bug reports.
func collectDiagnostics(targetDir string) string {
	var b strings.Builder

	fmt.Fprintf(&b, "nav-pilot  %s (%s)\n", version, commit)
	fmt.Fprintf(&b, "OS         %s/%s\n", runtime.GOOS, runtime.GOARCH)

	if isBrewManaged() {
		fmt.Fprintf(&b, "Install    homebrew\n")
	} else {
		fmt.Fprintf(&b, "Install    binary\n")
	}

	state, err := readState(targetDir)
	if err == nil && state != nil {
		// Count file integrity
		ok, modified, missing, _ := countFileIntegrity(targetDir, state)
		fmt.Fprintf(&b, "Collection %s (%s, %s)\n", state.Collection, state.Version, shortSHA(state.SourceSHA))
		fmt.Fprintf(&b, "Files      %d ok, %d modified, %d missing\n", ok, modified, missing)
	} else {
		fmt.Fprintf(&b, "Collection (none installed)\n")
	}

	return strings.TrimRight(b.String(), "\n")
}

// countFileIntegrity checks installed files and returns ok/modified/missing counts
// plus the relative paths of any modified files.
func countFileIntegrity(rootDir string, state *StateFile) (ok, modified, missing int, modifiedPaths []string) {
	for _, f := range state.Files {
		path := filepath.Join(rootDir, f.Path)
		var currentHash string
		var hashErr error
		if strings.HasSuffix(f.Path, "/") {
			currentHash, hashErr = dirHash(path)
		} else {
			currentHash, hashErr = fileHash(path)
		}
		if hashErr != nil {
			missing++
			continue
		}
		if currentHash != f.Hash {
			modified++
			modifiedPaths = append(modifiedPaths, f.Path)
		} else {
			ok++
		}
	}
	return
}

// shortSHA returns the first 7 characters of a SHA, or the full string if shorter.
func shortSHA(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}

// buildFeedbackURL constructs a GitHub issue URL with pre-filled template and diagnostics.
func buildFeedbackURL(featureRequest bool, diagnostics string) string {
	var template, labels string
	if featureRequest {
		template = "nav-pilot-feature.yml"
		labels = "nav-pilot,enhancement"
	} else {
		template = "nav-pilot-bug.yml"
		labels = "nav-pilot"
	}

	params := url.Values{}
	params.Set("template", template)
	params.Set("labels", labels)
	if diagnostics != "" {
		params.Set("diagnostics", diagnostics)
	}

	return fmt.Sprintf("https://github.com/%s/issues/new?%s", feedbackRepo, params.Encode())
}

// openBrowser opens a URL in the system default browser.
func openBrowser(rawURL string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", rawURL).Start()
	case "linux":
		return exec.Command("xdg-open", rawURL).Start()
	default:
		return fmt.Errorf("unsupported platform %s", runtime.GOOS)
	}
}
