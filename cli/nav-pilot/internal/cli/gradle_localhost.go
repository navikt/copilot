package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"

	providerpkg "github.com/navikt/copilot/cli/nav-pilot/internal/provider"
	"github.com/navikt/copilot/cli/nav-pilot/internal/source"
)

// cpltLocalFloor is the first cplt release checked to have
// `cplt config set --local` and `sandbox.allow_localhost_any`
// (2026.11.20-214214-54df560). Older cplt is not asked.
const cpltLocalFloor = "2026.11.20-214214"

const gradleLocalhostPrompt = "Gradle needs localhost ports that cplt blocks by default. Allow localhost for this repo? The agent can then reach local databases, the Docker API and dev servers. [y/N] "

// gradleMarkers at the repository root make it a Gradle repo.
var gradleMarkers = []string{"build.gradle", "build.gradle.kts", "settings.gradle", "settings.gradle.kts", "gradlew"}

func isGradleRepo(root string) bool {
	for _, m := range gradleMarkers {
		if _, err := os.Stat(filepath.Join(root, m)); err == nil {
			return true
		}
	}
	return false
}

// proposesLocalhost reports whether the repo's .cplt.toml has
// [propose] allow_localhost_any. That goes through `cplt trust`; nav-pilot
// never applies it.
func proposesLocalhost(root string) bool {
	var f struct {
		Propose map[string]any `toml:"propose"`
	}
	if _, err := toml.DecodeFile(filepath.Join(root, ".cplt.toml"), &f); err != nil {
		return false
	}
	_, ok := f.Propose["allow_localhost_any"]
	return ok
}

// runCpltConfig runs `cplt config ...` in dir and returns combined output.
// A var so tests never run the real cplt.
var runCpltConfig = func(dir string, args ...string) (string, error) {
	cpltPath, err := findCplt()
	if err != nil {
		return "", err
	}
	out, err := runBoundedIn(dir, cpltCommandTimeout, true, cpltPath, append([]string{"config"}, args...)...)
	return string(out), err
}

var cpltStampForGradle = providerpkg.CpltStamp

// maybeGradleLocalhost asks once per repo, in a terminal and under cplt,
// whether a Gradle repo may reach any localhost port (navikt/copilot#2847).
// The answer, yes or no, is stored with `cplt config set --local`; cplt wants
// an explicit false so its own Gradle hint stays quiet.
func maybeGradleLocalhost(projectDir string) {
	if !isInteractive() || sessionPrompted || os.Getenv("__CPLT_TRUST_LOCKED") != "" {
		return
	}
	if projectDir == "" {
		projectDir = "."
	}
	root := source.FindGitRoot(projectDir)
	if root == "" || !isGradleRepo(root) || proposesLocalhost(root) || !cpltInstalled() {
		return
	}
	if cpltStampForGradle() < cpltLocalFloor {
		return
	}
	if !localhostUndecided(root) {
		return
	}
	if !claimSessionPrompt() {
		return
	}
	value := "false"
	if askTrustReview(gradleLocalhostPrompt) {
		value = "true"
	}
	if _, err := runCpltConfig(root, "set", "--local", "sandbox.allow_localhost_any", value); err != nil {
		fmt.Fprintf(os.Stderr, "%s cplt config set: %v\n", yellow("⚠"), err)
	}
}

// localhostUndecided reports whether no layer has decided
// sandbox.allow_localhost_any. It asks `cplt config get --json` first
// (navikt/copilot#2874). Any error, unparseable output, other schema or
// mismatched key falls back to the text check older cplt needs.
func localhostUndecided(root string) bool {
	const key = "sandbox.allow_localhost_any"
	if out, err := runCpltConfig(root, "get", "--json", key); err == nil {
		var r struct {
			Schema int    `json:"schema"`
			Key    string `json:"key"`
			Set    *bool  `json:"set"`
		}
		if json.Unmarshal([]byte(strings.TrimSpace(out)), &r) == nil && r.Schema == 1 && r.Key == key && r.Set != nil {
			return !*r.Set
		}
	}
	out, err := runCpltConfig(root, "get", key)
	return err == nil && strings.Contains(out, "(default")
}
