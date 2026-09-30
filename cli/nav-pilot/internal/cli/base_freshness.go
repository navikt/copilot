package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/navikt/copilot/cli/nav-pilot/internal/agentpakke"
	"github.com/navikt/copilot/cli/nav-pilot/internal/artifacts"
)

// Base freshness (#1368): a pakke that reuses another pins the base in its own
// lock file, and compose resolves the base at exactly that revision. Nothing
// moved the pin and nothing said it was old, so nais/pilot's users ran agents
// whose frontmatter model pins navikt/copilot had replaced weeks before.
//
// sync and doctor now say so in one line, addressed to the pakke owner, since
// only the owner can move the pin. It is a warning and nothing else: the pin
// is never moved from here, doctor does not fail over it, and a lookup that
// cannot finish (offline, rate limited, a private base) says nothing at all.
// Launch never asks: see TestBaseFreshnessIsOffTheLaunchPath.
//
// "Behind" means behind what sync itself would take for that repo: its newest
// stable release when it publishes release metadata, and its default branch
// when it does not, which is navikt/copilot's case.
//
// ponytail: no cache. Both callers are commands a person runs on purpose, and
// sync already makes the same uncached release lookup for its own source. A
// check costs two requests (the releases list and a compare) plus two small
// file reads in doctor. Cache it next to pakke-releases.json if it ever
// reaches the anonymous rate limit.

// baseLag is how far a reused base's pin trails what sync would take for it.
type baseLag struct {
	Commits int
	// Days since the oldest commit the pin lacks, which is how long the pin
	// has been behind.
	Days int
	// Target is what the pin is behind: "its default branch" or a release.
	Target string
}

// lookupBaseLag reports how far pin trails in repo, or nil when it does not.
// A var so tests of other commands can keep it off the network.
var lookupBaseLag = lookupBaseLagHTTP

func lookupBaseLagHTTP(ctx context.Context, repo, name, pin string) (*baseLag, error) {
	outcome, rel, err := discoverPakkeRelease(ctx, repo, name, pin)
	if err != nil {
		return nil, err
	}
	head, target := "HEAD", "its default branch"
	switch outcome {
	case releaseCandidate:
		head, target = rel.SHA, "release "+rel.Version
	case releaseNoMetadata:
	default:
		return nil, nil // at the newest release, or ahead of it
	}
	// per_page=1 returns the oldest commit the pin lacks; ahead_by counts all.
	var cmp struct {
		Status  string `json:"status"`
		AheadBy int    `json:"ahead_by"`
		Commits []struct {
			Commit struct {
				Committer struct {
					Date time.Time `json:"date"`
				} `json:"committer"`
			} `json:"commit"`
		} `json:"commits"`
	}
	if err := githubJSON(ctx, fmt.Sprintf("%s/repos/%s/compare/%s...%s?per_page=1", githubAPIBase, repo, pin, head), &cmp); err != nil {
		return nil, err
	}
	// identical, behind or diverged: nothing a bump to it would fix cleanly.
	if cmp.Status != "ahead" || cmp.AheadBy == 0 {
		return nil, nil
	}
	lag := &baseLag{Commits: cmp.AheadBy, Target: target}
	if len(cmp.Commits) > 0 {
		lag.Days = int(time.Since(cmp.Commits[0].Commit.Committer.Date).Hours() / 24)
	}
	return lag, nil
}

// warnBaseLag checks the base that pakkeRepo's lock pins and prints one line
// to w when it trails. It also records the answer as freshness telemetry.
func warnBaseLag(w io.Writer, indent, scopeName, pakkeRepo, baseRepo, baseName, pin string) {
	if !pinnable(baseRepo) || pin == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), pakkeReleaseTimeout)
	defer cancel()
	lag, err := lookupBaseLag(ctx, baseRepo, baseName, pin)
	a := artifacts.StalenessAssessment{Result: "up_to_date", LatestVersion: "latest", UpToDate: true}
	switch {
	case err != nil:
		a = artifacts.StalenessAssessment{Result: "lookup_failed"}
	case lag != nil:
		a = artifacts.StalenessAssessment{Result: "stale", LatestVersion: lag.Target, SkewDays: int64(lag.Days), HasSkew: true}
	}
	recordFreshness("agentpakke-base", scopeName, a)
	if lag == nil {
		return
	}
	fmt.Fprintf(w, "%s%s %s pins %s at %s in %s, %d commit(s) and %d day(s) behind %s. The owners of %s should bump it.\n",
		indent, yellow("⚠"), pakkeRepo, baseRepo, shortSHA(pin), agentpakke.DeclarationPath,
		lag.Commits, lag.Days, lag.Target, pakkeRepo)
}

// reportScopeBaseLag is doctor's half. doctor has no checkout of the scope's
// source, only the repo and revision its state records, so it reads the lock
// and the base's name from GitHub at those revisions. A scope on the default
// source is skipped without a request: navikt/copilot reuses nothing.
func reportScopeBaseLag(scope *InstallScope, state *StateFile) {
	if state == nil || tracksDefaultSource(state) || !pinnable(state.SourceRepo) || state.SourceSHA == "" || pinnedState(state) {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), pakkeReleaseTimeout)
	defer cancel()
	var lock agentpakke.Declaration
	if githubFileJSON(ctx, state.SourceRepo, agentpakke.DeclarationPath, state.SourceSHA, &lock) != nil ||
		!pinnable(lock.Source) || lock.SHA == "" {
		return
	}
	var base struct {
		Name string `json:"name"`
	}
	if githubFileJSON(ctx, lock.Source, agentpakke.ManifestPath, lock.SHA, &base) != nil {
		return
	}
	warnBaseLag(os.Stdout, "      ", scope.Name, state.SourceRepo, lock.Source, base.Name, lock.SHA)
}

// githubFileJSON decodes one file of repo at ref, read through the contents
// API. A var, like lookupBaseLag, so the test binary starts with both offline.
var githubFileJSON = githubFileJSONHTTP

func githubFileJSONHTTP(ctx context.Context, repo, path, ref string, v any) error {
	resp, err := githubGet(ctx, fmt.Sprintf("%s/repos/%s/contents/%s?ref=%s", githubAPIBase, repo, path, url.QueryEscape(ref)), "application/vnd.github.raw+json")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GitHub API returned %d for %s", resp.StatusCode, path)
	}
	return json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(v)
}
