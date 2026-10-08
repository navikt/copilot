package cli

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
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
	// Ref is the revision Target names: a release SHA, or "HEAD".
	Ref string
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
	lag := &baseLag{Commits: cmp.AheadBy, Target: target, Ref: head}
	if len(cmp.Commits) > 0 {
		lag.Days = int(time.Since(cmp.Commits[0].Commit.Committer.Date).Hours() / 24)
	}
	return lag, nil
}

// warnBaseLag checks the base that pakkeRepo's lock pins and prints two lines
// to w when it trails. It also records the answer as freshness telemetry, and
// returns the lookup's error so doctor can say it could not tell.
// ctx bounds the whole check; the caller sets the deadline.
func warnBaseLag(ctx context.Context, w io.Writer, indent, scopeName, pakkeRepo, baseRepo, baseName, pin string) (*baseLag, error) {
	if !pinnable(baseRepo) || pin == "" {
		return nil, nil
	}
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
		return nil, err
	}
	// Addressed to the person running the command, who cannot move the pin:
	// what it means for them, and who can.
	fmt.Fprintf(w, "%s%s %s pins %s at %s, %d commits (%d days) behind %s. You miss what changed since, such as new model choices.\n"+
		"%s  Ask the owners of %s to update (%s), then run %s.\n",
		indent, yellow("⚠"), pakkeRepo, baseRepo, shortSHA(pin), lag.Commits, lag.Days, lag.Target,
		indent, pakkeRepo, bold("nav-pilot pakke bump-base"), bold("nav-pilot sync --apply"))
	return lag, nil
}

// couldNotCheck is doctor's line for a check that did not finish, so a user
// can tell "current" from "unchecked".
func couldNotCheck(what string) {
	fmt.Printf("      %s\n", dim("Could not check "+what+" (network or rate limit)."))
}

// reportScopeBaseLag is doctor's half. doctor has no checkout of the scope's
// source, only the repo and revision its state records, so it reads the lock
// and the base's name from GitHub at those revisions. A scope on the default
// source is skipped without a request: navikt/copilot reuses nothing, and
// neither does a pakke without a lock (a 404, not a failure).
func reportScopeBaseLag(scope *InstallScope, state *StateFile) {
	if state == nil || tracksDefaultSource(state) || !pinnable(state.SourceRepo) || state.SourceSHA == "" || pinnedRevisionOnDisk(state) {
		return
	}
	// One deadline for these requests, so doctor waits at most this long.
	ctx, cancel := context.WithTimeout(context.Background(), pakkeReleaseTimeout)
	defer cancel()
	var lock agentpakke.Declaration
	if err := githubFileJSON(ctx, state.SourceRepo, agentpakke.DeclarationPath, state.SourceSHA, &lock); err != nil {
		if !errors.Is(err, errGitHubNotFound) {
			couldNotCheck("whether " + state.SourceRepo + " is up to date")
		}
		return
	}
	if !pinnable(lock.Source) || lock.SHA == "" {
		return
	}
	var base struct {
		Name string `json:"name"`
	}
	if githubFileJSON(ctx, lock.Source, agentpakke.ManifestPath, lock.SHA, &base) != nil {
		couldNotCheck("whether " + state.SourceRepo + " is up to date")
		return
	}
	lag, err := warnBaseLag(ctx, os.Stdout, "      ", scope.Name, state.SourceRepo, lock.Source, base.Name, lock.SHA)
	if err != nil {
		couldNotCheck("whether " + state.SourceRepo + " is up to date")
		return
	}
	// Drift is measured against what bump-base would move the pin to, so a
	// pin at the newest release is not told about unreleased changes.
	if lag != nil {
		reportPinDrift(scope, state, lock.Source, lock.SHA, cmp.Or(lag.Ref, "HEAD"))
	}
}

// pinDriftRequestTimeout bounds each request of the drift check on its own,
// so one slow answer costs that agent and not every agent after it.
const pinDriftRequestTimeout = 5 * time.Second

// reportPinDrift warns, per installed agent, when the base has since changed
// the agent's frontmatter model and the pakke still ships the old one. An
// agent whose installed model differs from the base's at the pin is the
// pakke's own choice (or the user's edit) and is left alone, as is an agent
// the base does not ship (a 404). Only files nav-pilot installed are read: the
// state tracks those and nothing the user wrote. It assumes the base keeps its
// agents at agents/<file>, as navikt/copilot does. Agents it could not check
// are counted in one line.
//
// ponytail: one contents request per installed agent, plus one for each whose
// model moved, each with its own deadline. About three doctor runs an hour
// anonymously; GITHUB_TOKEN lifts that. Batch through the trees API if it
// ever matters.
func reportPinDrift(scope *InstallScope, state *StateFile, baseRepo, pin, target string) {
	get := func(path, ref string) ([]byte, error) {
		ctx, cancel := context.WithTimeout(context.Background(), pinDriftRequestTimeout)
		defer cancel()
		return githubFile(ctx, baseRepo, path, ref)
	}
	drifted, unchecked := 0, 0
	for _, f := range state.Files {
		name := filepath.Base(f.Path)
		// Ignored and conflict entries are files the user kept as theirs.
		if f.Status != "" || !strings.HasSuffix(name, ".agent.md") {
			continue
		}
		local, err := os.ReadFile(filepath.Join(scope.RootDir, f.Path))
		installed := frontmatterModel(local)
		if err != nil || installed == "" {
			continue
		}
		path := "agents/" + name
		head, err := get(path, target)
		if err != nil {
			if !errors.Is(err, errGitHubNotFound) {
				unchecked++
			}
			continue
		}
		current := frontmatterModel(head)
		if current == "" || current == installed {
			continue
		}
		atPin, err := get(path, pin)
		if err != nil {
			if !errors.Is(err, errGitHubNotFound) {
				unchecked++
			}
			continue
		}
		if frontmatterModel(atPin) != installed {
			continue
		}
		drifted++
		fmt.Printf("      %s @%s runs %s; %s now pins %s.\n",
			yellow("⚠"), strings.TrimSuffix(name, ".agent.md"), installed, baseRepo, current)
	}
	if drifted > 0 {
		fmt.Printf("      When the owners of %s have updated, run %s.\n", state.SourceRepo, bold("nav-pilot sync --apply"))
	}
	if unchecked > 0 {
		couldNotCheck(fmt.Sprintf("%d agent(s) against %s", unchecked, baseRepo))
	}
}

// githubFile is one file of repo at ref, raw, read through the contents API.
// A 404 wraps errGitHubNotFound. A var so the test binary starts offline.
var githubFile = githubFileHTTP

func githubFileHTTP(ctx context.Context, repo, path, ref string) ([]byte, error) {
	resp, err := githubGet(ctx, fmt.Sprintf("%s/repos/%s/contents/%s?ref=%s", githubAPIBase, repo, path, url.QueryEscape(ref)), "application/vnd.github.raw+json")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("%s: %w", path, errGitHubNotFound)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned %d for %s", resp.StatusCode, path)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 1<<20))
}

// githubFileJSON decodes one file of repo at ref, read through the contents
// API. A var, like lookupBaseLag, so the test binary starts with both offline.
var githubFileJSON = githubFileJSONHTTP

func githubFileJSONHTTP(ctx context.Context, repo, path, ref string, v any) error {
	data, err := githubFileHTTP(ctx, repo, path, ref)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}
