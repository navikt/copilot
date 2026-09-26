package cli

import (
	"slices"
	"strings"
)

// suggest returns the closest match from candidates if the edit distance is <= 2.
// Returns "" if no close match is found.
func suggest(input string, candidates []string) string {
	best := ""
	bestDist := 3 // threshold: only suggest if dist <= 2
	for _, c := range candidates {
		d := levenshtein(input, c)
		if d < bestDist {
			bestDist = d
			best = c
		}
	}
	return best
}

// levenshtein computes the edit distance between two strings.
func levenshtein(a, b string) int {
	if len(a) == 0 {
		return len(b)
	}
	if len(b) == 0 {
		return len(a)
	}

	prev := make([]int, len(b)+1)
	curr := make([]int, len(b)+1)

	for j := range prev {
		prev[j] = j
	}

	for i := 1; i <= len(a); i++ {
		curr[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			curr[j] = min(
				curr[j-1]+1,    // insert
				prev[j]+1,      // delete
				prev[j-1]+cost, // substitute
			)
		}
		prev, curr = curr, prev
	}

	return prev[len(b)]
}

func min(a, b, c int) int {
	if b < a {
		a = b
	}
	if c < a {
		a = c
	}
	return a
}

// Known commands and flags for did-you-mean suggestions.
var knownCommands = []string{
	"install", "add", "ignore", "export", "sync", "list", "status",
	"uninstall", "update", "env", "feedback", "version", "help",
}

var knownFlags = []string{
	"-n", "--dry-run",
	"-f", "--force",
	"--apply",
	"--updates",
	"--json",
	"--items",
	"-F", "--feature",
	"-u", "--user",
	"--repo",
	"--frozen",
	"-t", "--target",
	"-r", "--ref",
	"-s", "--source",
	"-h", "--help",
	"--installed", "--all", "--type",
	"--yes", "--save-source",
}

// launchFlags are the flags nav-pilot takes with no command, when it launches
// the client. Suggestions for a typo there come from these.
var launchFlags = []string{
	"--client", "--source", "--project-dir", "--persona", "--model", "--mode",
	"--effort", "--context", "--payload-context", "--log-level", "--otel-log-level",
	"--allow-all-tools", "--no-allow-all-tools", "--ask-user", "--no-ask-user",
	"--auto-launch", "--no-auto-launch", "--no-sandbox", "--sync",
	"--version", "-v", "--help", "-h",
}

// valueFlags take a value, so --flag=value can be split into --flag value.
var valueFlags = []string{
	"--client", "--source", "--project-dir", "--persona", "--model", "--mode",
	"--effort", "--context", "--payload-context", "--log-level", "--otel-log-level",
	"--target", "--ref", "--type", "--updates",
}

// splitFlagValues rewrites --flag=value as --flag value for the flags in
// valueFlags, up to a "--" separator: what follows it belongs to the client.
func splitFlagValues(args []string) []string {
	out := make([]string, 0, len(args))
	for i, a := range args {
		if a == "--" {
			return append(out, args[i:]...)
		}
		if name, value, ok := strings.Cut(a, "="); ok && slices.Contains(valueFlags, name) {
			out = append(out, name, value)
			continue
		}
		out = append(out, a)
	}
	return out
}
