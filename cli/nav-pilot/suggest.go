package main

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
				curr[j-1]+1,   // insert
				prev[j]+1,     // delete
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
	"--json",
	"--items",
	"-F", "--feature",
	"-u", "--user",
	"-t", "--target",
	"-r", "--ref",
	"-s", "--source",
	"-h", "--help",
}
