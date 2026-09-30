package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/navikt/copilot/cli/nav-pilot/internal/agentpakke"
	"github.com/navikt/copilot/cli/nav-pilot/internal/source"
)

// cmdPakkeBumpBase moves the base that root's lock file pins onto what sync
// would take for that repo: its newest stable release, or its default branch
// when it publishes none (#1368). It is the pakke owner's command, and the
// work behind .github/workflows/agentpakke-base-bump.yaml, which only commits
// the result and opens the pull request.
//
// stdout is a Markdown summary for that pull request: the agents that changed
// and every frontmatter model pin that moved. A pin beats --model, so a model
// change is the line a reviewer most needs to see. Progress goes to stderr.
// Nothing is written when the pin is current, so the lock file's diff is the
// answer to "did it move".
func cmdPakkeBumpBase(root string) error {
	d, err := agentpakke.LoadDeclaration(root)
	if errors.Is(err, agentpakke.ErrNoDeclaration) {
		return fmt.Errorf("%s has no %s, so it reuses no agentpakke and there is nothing to bump", root, agentpakke.DeclarationPath)
	}
	if err != nil {
		return err
	}
	if !pinnable(d.Source) || d.SHA == "" {
		return fmt.Errorf("%s reuses %q without a pinned revision, so there is nothing to bump", agentpakke.DeclarationPath, d.Source)
	}

	old, err := resolveSourceForSync(d.SHA, d.Source)
	if err != nil {
		return fmt.Errorf("fetching the pinned %s@%s: %w", d.Source, shortSHA(d.SHA), err)
	}
	defer old.Cleanup()
	name := ""
	if old.Pakke != nil {
		name = old.Pakke.Name
	}

	// The same rule sync applies to a source of its own. A failed lookup is an
	// error, not a reason to take the default branch: a pin moved onto HEAD on
	// a guess lands ahead of every release (#13).
	outcome, rel, err := discoverPakkeRelease(context.Background(), d.Source, name, d.SHA)
	if err != nil {
		return fmt.Errorf("looking up stable releases of %s: %w\nThe pin is unchanged", d.Source, err)
	}
	var next *Source
	target := "its default branch"
	switch outcome {
	case releaseCandidate:
		next, err = fetchPakkeRelease(d.Source, name, rel)
		target = "release " + rel.Version
	case releaseNoMetadata:
		next, err = resolveSourceForSync("", d.Source)
	default:
		fmt.Fprintf(os.Stderr, "%s %s@%s is at or ahead of its newest stable release. Nothing to bump.\n", green("✓"), d.Source, shortSHA(d.SHA))
		return nil
	}
	if err != nil {
		return err
	}
	defer next.Cleanup()
	if sameSHA(next.SHA, d.SHA) {
		fmt.Fprintf(os.Stderr, "%s %s@%s is current. Nothing to bump.\n", green("✓"), d.Source, shortSHA(d.SHA))
		return nil
	}
	// compose refuses a payload-only base, so a pin onto one would break every
	// install of this pakke.
	if payloadOnly(next) {
		return fmt.Errorf("%s at %s ships pre-built payloads only, and a pakke can only reuse a layout. The pin is unchanged", d.Source, shortSHA(next.SHA))
	}

	from := d.SHA
	d.SHA = next.SHA
	d.MinNavPilotVersion = minNavPilotVersionOf(next)
	if err := agentpakke.WriteDeclaration(root, d); err != nil {
		return err
	}
	fmt.Print(baseBumpSummary(d.Source, from, next.SHA, target, agentChanges(old, next)))
	fmt.Fprintf(os.Stderr, "%s Bumped %s: %s %s → %s (%s).\n", green("✓"), agentpakke.DeclarationPath, d.Source, shortSHA(from), shortSHA(next.SHA), target)
	return nil
}

// agentChanges lists, as Markdown bullets, every agent added, removed or
// changed between two checkouts of a pakke, naming the model pin when it moved.
func agentChanges(old, next *Source) []string {
	agents := func(s *Source) map[string][]byte {
		m := map[string][]byte{}
		for _, r := range resolverFor(s.Dir, s.Pakke).List(KindAgent) {
			if r.IsDir {
				continue
			}
			data := r.Data
			if data == nil {
				data, _ = os.ReadFile(r.AbsPath)
			}
			m[r.Name] = data
		}
		return m
	}
	model := func(data []byte) string {
		fm, _, ok := source.SplitFrontmatter(data)
		if !ok {
			return ""
		}
		v, _ := source.ExtractFrontmatterValue(fm, "model")
		return v
	}
	orNone := func(s string) string {
		if s == "" {
			return "none"
		}
		return "`" + s + "`"
	}

	before, after := agents(old), agents(next)
	names := map[string]bool{}
	for n := range before {
		names[n] = true
	}
	for n := range after {
		names[n] = true
	}
	var lines []string
	for n := range names {
		a, inA := before[n]
		b, inB := after[n]
		switch {
		case !inA:
			lines = append(lines, fmt.Sprintf("- `%s`: added, model %s", n, orNone(model(b))))
		case !inB:
			lines = append(lines, fmt.Sprintf("- `%s`: removed", n))
		case model(a) != model(b):
			lines = append(lines, fmt.Sprintf("- `%s`: model %s → %s", n, orNone(model(a)), orNone(model(b))))
		case string(a) != string(b):
			lines = append(lines, fmt.Sprintf("- `%s`: changed", n))
		}
	}
	sort.Strings(lines)
	return lines
}

// baseBumpSummary is the pull request body for a bump.
func baseBumpSummary(repo, from, to, target string, agents []string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Bumps `%s` in `%s` from `%s` to `%s`, %s.\n\n", repo, agentpakke.DeclarationPath, shortSHA(from), shortSHA(to), target)
	fmt.Fprintf(&b, "Changes: https://github.com/%s/compare/%s...%s\n\n", repo, from, to)
	b.WriteString("### Agents\n\n")
	if len(agents) == 0 {
		b.WriteString("No agent changed.\n")
	} else {
		b.WriteString(strings.Join(agents, "\n") + "\n\n")
		b.WriteString("A model pin in an agent's frontmatter decides which model it runs on and beats `--model`, so check the model lines before merging.\n")
	}
	return b.String()
}
