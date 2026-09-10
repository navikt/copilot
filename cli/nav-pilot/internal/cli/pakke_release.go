package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/navikt/copilot/cli/nav-pilot/internal/agentpakke"
)

// Release discovery for a pinned Tier 2 agentpakke (#779). The publishing
// contract it reads is documented in docs/README.agentpakke.md, "Stabile
// releases"; why it is Releases plus an asset and not a tag, a branch or a
// catalog is in docs/agentpakke-beslutninger.md.

const (
	// pakkeReleaseAsset is the release asset a package owner publishes.
	pakkeReleaseAsset = agentpakke.ReleaseAssetName
	// pakkeReleaseAssetMax caps the asset download. Four short fields fit in
	// well under a kilobyte; anything near this is not the contract.
	pakkeReleaseAssetMax = 64 << 10
	// pakkeReleaseTimeout bounds the whole lookup, every request included.
	pakkeReleaseTimeout = 10 * time.Second
)

// githubAPIBase is the GitHub REST API root. Overridable in tests.
var githubAPIBase = "https://api.github.com"

// releaseOutcome is what a lookup found. An error is the fifth outcome, and
// never collapses into any of these.
type releaseOutcome int

const (
	// releaseNoMetadata: no stable, immutable release carries metadata for this
	// package. The source is not release-backed.
	releaseNoMetadata releaseOutcome = iota
	// releaseUpToDate: the installed SHA is the newest stable release's.
	releaseUpToDate
	// releaseCandidate: a newer stable release is offered.
	releaseCandidate
	// releaseNotOffered: the installed SHA is ahead of or diverged from the
	// newest stable release, so moving to it would be a downgrade or a jump
	// sideways.
	releaseNotOffered
)

// pakkeRelease is one checked stable release of a package.
type pakkeRelease struct {
	Version string `json:"version"`
	SHA     string `json:"sha"`
	Tag     string `json:"tag"`
}

// version is the release version, or "" for no release.
func (r *pakkeRelease) version() string {
	if r == nil {
		return ""
	}
	return r.Version
}

// label names a revision for output: "0.4.1 (abc1234)", or just the short SHA
// when it is not a release.
func (r *pakkeRelease) label(sha string) string {
	if r == nil {
		return shortSHA(sha)
	}
	return r.Version + " (" + shortSHA(sha) + ")"
}

// releaseClaim is what a state says about releases, trusted only for the SHA it
// was recorded for. An older nav-pilot that re-pins keeps the keys as unknown
// ones while moving the pin, and a claim about another revision is no claim
// about this one.
func releaseClaim(state *StateFile) (version string, follows bool) {
	if state == nil || state.PakkeVersionSHA == "" || !sameSHA(state.PakkeVersionSHA, state.SourceSHA) {
		return "", false
	}
	return state.PakkeVersion, state.FollowsReleases
}

// errGitHubNotFound marks a 404 from the GitHub API.
var errGitHubNotFound = errors.New("not found")

// errReleasesNotFound marks a 404 for the releases list itself: a repo that is
// gone, or a private one that GITHUB_TOKEN does not reach.
var errReleasesNotFound = errors.New("GitHub answered 404 for the releases list (a private repo needs GITHUB_TOKEN)")

// releasesNotVisible is the warning for a source whose releases list is a 404,
// shared by sync and a new pin.
func releasesNotVisible(repo string) string {
	return fmt.Sprintf("releases for %s are not visible (GitHub answered 404); set GITHUB_TOKEN if the repo is private", repo)
}

// discoverPakkeRelease is the lookup sync uses. A var so sync tests can stub it
// without a network call.
var discoverPakkeRelease = discoverPakkeReleaseHTTP

type ghPakkeRelease struct {
	TagName    string `json:"tag_name"`
	Draft      bool   `json:"draft"`
	Prerelease bool   `json:"prerelease"`
	Immutable  bool   `json:"immutable"`
	Assets     []struct {
		Name string `json:"name"`
		URL  string `json:"url"`
	} `json:"assets"`
}

// pakkeReleaseMetadata is agentpakke-release.json once the published schema,
// schemaVersion included, has accepted it.
type pakkeReleaseMetadata struct {
	Name      string `json:"name"`
	Version   string `json:"version"`
	SourceSHA string `json:"sourceSha"`
}

// strictSemver orders release tags before any asset is downloaded. The asset's
// own version is held to the same shape by the published schema.
var strictSemver = regexp.MustCompile(`^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)$`)

// parseStrictSemver accepts MAJOR.MINOR.PATCH and nothing else: no "v", no
// prerelease, no build metadata.
//
// ponytail: components are Go ints (64-bit on every release target), so a tag
// with a component above 9223372036854775807 is skipped as not a version. That
// fails safe: a skipped tag is never pinned, and the downgrade guard compares
// commits, not numbers. Parse with math/big if a package ever needs one.
func parseStrictSemver(s string) (agentpakke.Semver3, bool) {
	m := strictSemver.FindStringSubmatch(s)
	if m == nil {
		return agentpakke.Semver3{}, false
	}
	var v agentpakke.Semver3
	for i := range v {
		n, err := strconv.Atoi(m[i+1])
		if err != nil {
			return agentpakke.Semver3{}, false
		}
		v[i] = n
	}
	return v, true
}

// tagVersion is what a release tag must equal the metadata version as: the tag
// with an optional "<prefix>/" and an optional "v" stripped.
func tagVersion(tag string) string {
	if i := strings.LastIndex(tag, "/"); i >= 0 {
		tag = tag[i+1:]
	}
	return strings.TrimPrefix(tag, "v")
}

// discoverPakkeReleaseHTTP finds the newest stable release of package name in
// repo (owner/name) and decides whether it is offered over installedSHA (empty
// for none).
//
// Only a published, non-draft, non-prerelease, immutable release qualifies.
// Its metadata must name this package, carry a strict version its tag binds to
// and a full source SHA, and that SHA must be on the repo's default branch:
// `git fetch origin <sha>` also fetches a commit that only exists in a fork,
// served through the parent. Invalid metadata is skipped so one broken
// immutable release cannot block every later one, but a repo whose metadata is
// all invalid is an error, never "no metadata": nothing here falls back to the
// default branch.
func discoverPakkeReleaseHTTP(ctx context.Context, repo, name, installedSHA string) (releaseOutcome, pakkeRelease, error) {
	ctx, cancel := context.WithTimeout(ctx, pakkeReleaseTimeout)
	defer cancel()

	var releases []ghPakkeRelease
	// ponytail: one page of the 100 newest releases. A package whose newest
	// stable release is older than that is reported as not release-backed;
	// follow the Link header if a repo ever gets there.
	if err := githubJSON(ctx, fmt.Sprintf("%s/repos/%s/releases?per_page=100", githubAPIBase, repo), &releases); err != nil {
		if errors.Is(err, errGitHubNotFound) {
			err = errReleasesNotFound
		}
		return 0, pakkeRelease{}, fmt.Errorf("listing releases of %s: %w", repo, err)
	}

	type candidate struct {
		tag      string
		assetURL string
		version  agentpakke.Semver3
	}
	// The token goes with the asset download, so the URL must be this repo's
	// own asset endpoint on the API, whatever the listing says.
	assetPrefix := githubAPIBase + "/repos/" + repo + "/releases/assets/"
	var cands []candidate
	var invalid []string
	for _, r := range releases {
		if r.Draft || r.Prerelease || !r.Immutable {
			continue
		}
		// A "<prefix>/" tag belongs to the package it names. Downloading every
		// other package's metadata in a multi-package repo only spends the
		// timeout.
		if i := strings.LastIndex(r.TagName, "/"); i >= 0 && r.TagName[:i] != name {
			continue
		}
		v, ok := parseStrictSemver(tagVersion(r.TagName))
		if !ok {
			continue // a tag that is not a version cannot bind one
		}
		for _, a := range r.Assets {
			if a.Name != pakkeReleaseAsset {
				continue
			}
			if len(a.URL) < len(assetPrefix) || !strings.EqualFold(a.URL[:len(assetPrefix)], assetPrefix) {
				invalid = append(invalid, fmt.Sprintf("%s: asset URL %q is not an asset of %s", r.TagName, a.URL, repo))
				continue
			}
			cands = append(cands, candidate{tag: r.TagName, assetURL: a.URL, version: v})
		}
	}
	// Newest version first, so the lookup downloads only until it has a winner.
	sort.SliceStable(cands, func(i, j int) bool { return cands[i].version.Compare(cands[j].version) > 0 })

	var chosen *pakkeRelease
	var chosenVersion agentpakke.Semver3
	for _, c := range cands {
		if chosen != nil && c.version.Compare(chosenVersion) < 0 {
			break
		}
		meta, err := readPakkeReleaseMetadata(ctx, c.assetURL)
		if err != nil {
			if ctx.Err() != nil {
				return 0, pakkeRelease{}, fmt.Errorf("reading %s from %s: %w", pakkeReleaseAsset, c.tag, err)
			}
			invalid = append(invalid, fmt.Sprintf("%s: %v", c.tag, err))
			continue
		}
		if meta.Name != name {
			continue // another package published from the same repo
		}
		if tagVersion(c.tag) != meta.Version {
			invalid = append(invalid, fmt.Sprintf("%s: tag does not bind to version %s", c.tag, meta.Version))
			continue
		}
		v, _ := parseStrictSemver(meta.Version) // the schema held it to MAJOR.MINOR.PATCH
		if chosen != nil {
			if meta.SourceSHA != chosen.SHA {
				return 0, pakkeRelease{}, fmt.Errorf("%s version %s is published twice with different source SHAs (%s in %s, %s in %s)",
					name, meta.Version, chosen.SHA, chosen.Tag, meta.SourceSHA, c.tag)
			}
			continue
		}
		chosen = &pakkeRelease{Version: meta.Version, SHA: meta.SourceSHA, Tag: c.tag}
		chosenVersion = v
	}

	if chosen == nil {
		if len(invalid) > 0 {
			return 0, pakkeRelease{}, fmt.Errorf("%s publishes %s, but none is valid for %s (nav-pilot does not fall back to the default branch):\n  %s",
				repo, pakkeReleaseAsset, name, strings.Join(invalid, "\n  "))
		}
		return releaseNoMetadata, pakkeRelease{}, nil
	}

	var info struct {
		DefaultBranch string `json:"default_branch"`
	}
	if err := githubJSON(ctx, fmt.Sprintf("%s/repos/%s", githubAPIBase, repo), &info); err != nil {
		return 0, *chosen, fmt.Errorf("reading %s's default branch: %w", repo, err)
	}
	if info.DefaultBranch == "" {
		return 0, *chosen, fmt.Errorf("GitHub reported no default branch for %s", repo)
	}
	onBranch, err := compareStatus(ctx, repo, chosen.SHA, info.DefaultBranch)
	if err != nil {
		return 0, *chosen, fmt.Errorf("checking that %s (%s) is on %s: %w", chosen.SHA, chosen.Tag, info.DefaultBranch, err)
	}
	if onBranch != "ahead" && onBranch != "identical" {
		return 0, *chosen, fmt.Errorf("%s %s names source %s, which is not on %s's default branch %s (compare: %s)",
			name, chosen.Tag, chosen.SHA, repo, info.DefaultBranch, onBranch)
	}

	if installedSHA == "" {
		return releaseCandidate, *chosen, nil
	}
	if sameSHA(installedSHA, chosen.SHA) {
		return releaseUpToDate, *chosen, nil
	}
	moved, err := compareStatus(ctx, repo, installedSHA, chosen.SHA)
	if errors.Is(err, errGitHubNotFound) {
		return 0, *chosen, fmt.Errorf(
			"GitHub does not know the installed revision %s in %s, so nav-pilot cannot tell whether %s %s is newer.\n\n"+
				"  Move the pin deliberately:  nav-pilot sync --user --apply --ref %s",
			shortSHA(installedSHA), repo, name, chosen.Version, chosen.SHA)
	}
	if err != nil {
		return 0, *chosen, fmt.Errorf("comparing installed %s with %s: %w", shortSHA(installedSHA), chosen.Tag, err)
	}
	switch moved {
	case "ahead":
		return releaseCandidate, *chosen, nil
	case "identical":
		return releaseUpToDate, *chosen, nil
	default: // behind, diverged
		return releaseNotOffered, *chosen, nil
	}
}

// fetchPakkeRelease resolves exactly rel's source SHA in repo, and refuses a
// revision that is not the package the release names. The caller cleans up the
// source.
func fetchPakkeRelease(repo, name string, rel pakkeRelease) (*Source, error) {
	relSrc, err := resolveSourceForSync(rel.SHA, repo)
	if err != nil {
		return nil, fmt.Errorf("fetching %s %s (%s): %w", name, rel.Version, shortSHA(rel.SHA), err)
	}
	if !sameSHA(relSrc.SHA, rel.SHA) || relSrc.Pakke == nil || relSrc.Pakke.Name != name {
		relSrc.Cleanup()
		return nil, fmt.Errorf("%s names %s at %s, but that revision resolved to %s shipping %q; nothing was pinned from it",
			rel.Tag, name, rel.SHA, relSrc.SHA, pakkeInstallTarget(relSrc))
	}
	return relSrc, nil
}

// releaseStart is where a new pin of the payload-only source src starts (#779):
// the newest stable release when the source publishes one, src itself when it
// publishes no metadata. Default-branch HEAD is usually ahead of the newest
// release, and a pin that starts there is never offered a release afterwards.
//
// A failed lookup is an error. There is no pin to keep, and pinning HEAD on a
// guess strands the install ahead of every release. The one exception is a 404
// for the releases list, which is what a private repo without GITHUB_TOKEN
// answers while git clones it with the user's own credentials: that source
// installed from its default branch before releases existed, and still does,
// with a warning on stderr (stdout may be an install's JSON document). The
// caller cleans up a returned source that is not src.
//
// follows says the pin being replaced follows this source's releases. Then the
// 404 falls back to nothing: a re-install would otherwise leave releases
// without a word, where sync fails closed. --ref is the explicit way out.
func releaseStart(src *Source, follows bool) (*Source, *pakkeRelease, error) {
	name := src.Pakke.Name
	outcome, rel, err := discoverPakkeRelease(context.Background(), src.Repo, name, "")
	if errors.Is(err, errReleasesNotFound) && follows {
		return nil, nil, fmt.Errorf("%s follows stable releases, and %s.\n"+
			"Nothing was pinned; the pin is unchanged.\n\n"+
			"  Leave stable releases deliberately:  %s",
			bold(name), releasesNotVisible(src.Repo), bold("nav-pilot install --user --ref <branch|sha> "+name))
	}
	if errors.Is(err, errReleasesNotFound) {
		fmt.Fprintf(os.Stderr, "%s %s\n", yellow("⚠"), releasesNotVisible(src.Repo))
		return src, nil, nil
	}
	if err != nil {
		return nil, nil, fmt.Errorf("looking up stable releases of %s: %w\n\n"+
			"Nothing was pinned; nav-pilot does not start %s on the default branch when it cannot tell whether a stable release exists.\n\n"+
			"  Pin a revision deliberately:  %s",
			src.Repo, err, name, bold("nav-pilot install --user --ref <branch|sha> "+name))
	}
	if outcome != releaseCandidate { // no metadata: the only other outcome with nothing installed
		return src, nil, nil
	}
	relSrc, err := fetchPakkeRelease(src.Repo, name, rel)
	if err != nil {
		return nil, nil, err
	}
	if !payloadOnly(relSrc) {
		relSrc.Cleanup()
		return nil, nil, fmt.Errorf("%s %s (%s) does not ship pre-built payloads only, so this nav-pilot cannot pin it; nothing was pinned",
			name, rel.Version, rel.Tag)
	}
	return relSrc, &rel, nil
}

// pakkeReleaseStatus is what status reports about a pinned agentpakke (#779).
type pakkeReleaseStatus struct {
	// Version is empty unless the state recorded it for the pinned SHA.
	Version         string `json:"version,omitempty"`
	PinnedSHA       string `json:"pinned_sha"`
	FollowsReleases bool   `json:"follows_releases"`
	// PendingRelease is a newer stable release the lookup offers. Status does
	// not fetch it: sync still checks that the revision is this package and
	// payload-only before it pins, and refuses otherwise.
	PendingRelease    *pakkeRelease `json:"pending_release,omitempty"`
	ReleaseCheckError string        `json:"release_check_error,omitempty"`
}

// pakkeStatus reports a user-scope pin, looking the releases up live. It is nil
// for anything that is not a pin of a repo. A failed lookup is reported in the
// result and never fails the caller.
//
// ponytail: no cache, a live lookup per status call bounded by
// pakkeReleaseTimeout. A lookup is at least four API requests (the releases
// list, one metadata asset, the repo, a compare) and five when a release is
// offered, plus one per further asset it has to try, so anonymous use (60
// requests/hour) tops out at about 12 runs an hour, fewer when newer assets are
// invalid. The startup prompt's cache (pakke-releases.json) is not read here;
// read it if status ever runs into the limit.
func pakkeStatus(scope *InstallScope, state *StateFile) *pakkeReleaseStatus {
	if scope == nil || !scope.IsUser() || !pinnedState(state) || !pinnable(state.SourceRepo) {
		return nil
	}
	version, follows := releaseClaim(state)
	st := &pakkeReleaseStatus{Version: version, PinnedSHA: state.SourceSHA, FollowsReleases: follows}
	outcome, rel, err := discoverPakkeRelease(context.Background(), state.SourceRepo, state.Collection, state.SourceSHA)
	switch {
	case err != nil:
		st.ReleaseCheckError = err.Error()
	case outcome == releaseCandidate:
		st.PendingRelease = &rel
	}
	return st
}

// readPakkeReleaseMetadata downloads and strictly decodes one metadata asset.
func readPakkeReleaseMetadata(ctx context.Context, assetURL string) (pakkeReleaseMetadata, error) {
	var meta pakkeReleaseMetadata
	resp, err := githubGet(ctx, assetURL, "application/octet-stream")
	if err != nil {
		return meta, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return meta, fmt.Errorf("download returned %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, pakkeReleaseAssetMax+1))
	if err != nil {
		return meta, err
	}
	if len(body) > pakkeReleaseAssetMax {
		return meta, fmt.Errorf("larger than %d bytes", pakkeReleaseAssetMax)
	}

	// The published schema, not a second copy of its rules here: the binary and
	// the file a package owner lints with cannot disagree.
	if err := agentpakke.ValidateRelease(body); err != nil {
		return meta, err
	}
	return meta, json.Unmarshal(body, &meta)
}

// compareStatus is GitHub's compare status of head relative to base: ahead,
// behind, identical or diverged.
func compareStatus(ctx context.Context, repo, base, head string) (string, error) {
	var cmp struct {
		Status string `json:"status"`
	}
	if err := githubJSON(ctx, fmt.Sprintf("%s/repos/%s/compare/%s...%s?per_page=1", githubAPIBase, repo, base, head), &cmp); err != nil {
		return "", err
	}
	if cmp.Status == "" {
		return "", errors.New("GitHub returned no compare status")
	}
	return cmp.Status, nil
}

// githubJSON GETs a GitHub API URL and decodes a 200 response into v.
func githubJSON(ctx context.Context, url string, v any) error {
	resp, err := githubGet(ctx, url, "")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("GitHub API returned 404: %w", errGitHubNotFound)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(v)
}
