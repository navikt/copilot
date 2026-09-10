package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
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
	Version string
	SHA     string
	Tag     string
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

// pakkeReleaseMetadata is schemaVersion 1 of agentpakke-release.json.
type pakkeReleaseMetadata struct {
	SchemaVersion int    `json:"schemaVersion"`
	Name          string `json:"name"`
	Version       string `json:"version"`
	SourceSHA     string `json:"sourceSha"`
}

// strictSemver orders release tags before any asset is downloaded. The asset's
// own version is held to the same shape by the published schema.
var strictSemver = regexp.MustCompile(`^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)$`)

// parseStrictSemver accepts MAJOR.MINOR.PATCH and nothing else: no "v", no
// prerelease, no build metadata.
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
		return 0, pakkeRelease{}, fmt.Errorf("listing releases of %s: %w", repo, err)
	}

	type candidate struct {
		tag      string
		assetURL string
		version  agentpakke.Semver3
		ok       bool // the tag parses as a version at all
	}
	var cands []candidate
	for _, r := range releases {
		if r.Draft || r.Prerelease || !r.Immutable {
			continue
		}
		for _, a := range r.Assets {
			if a.Name == pakkeReleaseAsset {
				v, ok := parseStrictSemver(tagVersion(r.TagName))
				cands = append(cands, candidate{tag: r.TagName, assetURL: a.URL, version: v, ok: ok})
			}
		}
	}
	// Newest tag first, so the lookup downloads only until it has a winner.
	// A tag that is not a version cannot bind, but is still downloaded when
	// nothing else is valid: it may be another package's release, and only its
	// name says so.
	sort.SliceStable(cands, func(i, j int) bool {
		if cands[i].ok != cands[j].ok {
			return cands[i].ok
		}
		return cands[i].version.Compare(cands[j].version) > 0
	})

	var chosen *pakkeRelease
	var chosenVersion agentpakke.Semver3
	var invalid []string
	for _, c := range cands {
		if chosen != nil && (!c.ok || c.version.Compare(chosenVersion) < 0) {
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

	// schemaVersion first, so a newer format reads as unsupported rather than
	// as a pile of unknown fields.
	var probe struct {
		SchemaVersion int `json:"schemaVersion"`
	}
	if err := json.Unmarshal(body, &probe); err != nil {
		return meta, fmt.Errorf("not valid JSON: %w", err)
	}
	if probe.SchemaVersion != 1 {
		return meta, fmt.Errorf("unsupported schemaVersion %d (this nav-pilot reads 1)", probe.SchemaVersion)
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
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(v)
}
