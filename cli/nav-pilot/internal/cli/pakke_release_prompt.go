package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/huh"
)

// The startup prompt for a newer stable release of a pinned agentpakke (#779).
// A launch in a terminal asks once per release whether to move the pin; nothing
// else about the launch changes, and a launch without a terminal never looks.

const (
	// pakkeReleaseCheckInterval is how long a lookup's answer is trusted.
	pakkeReleaseCheckInterval = 24 * time.Hour
	// pakkeReleaseRetryInterval replaces it after a failed lookup, so a user
	// offline for a day is not charged the timeout on every launch.
	pakkeReleaseRetryInterval = time.Hour
	// pakkeLaunchLookupTimeout bounds what a launch waits for GitHub.
	pakkeLaunchLookupTimeout = 3 * time.Second
)

// pakkeReleaseCachePath is ~/.nav-pilot/pakke-releases.json. It is its own
// file: `nav-pilot update` rewrites cache.json whole. Deleting it forgets
// lookups and dismissed versions, never a pin.
func pakkeReleaseCachePath() string {
	return filepath.Join(filepath.Dir(configPath()), "pakke-releases.json")
}

// pakkeReleaseCacheEntry is one repo and package's last lookup.
type pakkeReleaseCacheEntry struct {
	CheckedAt time.Time `json:"checked_at"`
	// PinnedSHA is the pin the lookup compared against. The downgrade guard's
	// answer is about that pin only, so a moved pin makes the entry stale.
	PinnedSHA string        `json:"pinned_sha"`
	Candidate *pakkeRelease `json:"candidate,omitempty"`
	Failed    bool          `json:"failed,omitempty"`
	// Dismissed is the version the user answered "No" to. Survives lookups.
	Dismissed string `json:"dismissed,omitempty"`
}

func pakkeReleaseCacheKey(repo, name string) string { return strings.ToLower(repo) + " " + name }

// fresh reports whether the entry still answers for pinnedSHA at now.
func (e pakkeReleaseCacheEntry) fresh(pinnedSHA string, now time.Time) bool {
	interval := pakkeReleaseCheckInterval
	if e.Failed {
		interval = pakkeReleaseRetryInterval
	}
	age := now.Sub(e.CheckedAt)
	return sameSHA(e.PinnedSHA, pinnedSHA) && age >= 0 && age < interval
}

// readPakkeReleaseCache returns the cache, empty when missing or corrupt.
func readPakkeReleaseCache() map[string]pakkeReleaseCacheEntry {
	var cache map[string]pakkeReleaseCacheEntry
	data, _ := os.ReadFile(pakkeReleaseCachePath())
	if json.Unmarshal(data, &cache) != nil || cache == nil {
		return map[string]pakkeReleaseCacheEntry{}
	}
	return cache
}

// writePakkeReleaseCache is best effort: a cache that cannot be written costs
// a lookup next launch. It replaces the file by rename, so two launches
// writing at once leave one whole file, never a torn one.
func writePakkeReleaseCache(cache map[string]pakkeReleaseCacheEntry) {
	path := pakkeReleaseCachePath()
	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil || os.MkdirAll(filepath.Dir(path), 0o755) != nil {
		return
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".pakke-releases-*")
	if err != nil {
		return
	}
	defer os.Remove(tmp.Name()) // no-op once the rename succeeds
	_, err = tmp.Write(data)
	if closeErr := tmp.Close(); err != nil || closeErr != nil {
		return
	}
	_ = os.Rename(tmp.Name(), path)
}

// confirmPakkeRelease asks the question. A var so tests answer it.
var confirmPakkeRelease = func(title string) (bool, error) {
	var update bool
	err := huh.NewConfirm().Title(title).Value(&update).WithTheme(navTheme()).Run()
	return update, err
}

// lookUpPakkeRelease runs #780's discovery within timeout.
func lookUpPakkeRelease(repo, name, pinnedSHA string, timeout time.Duration) (releaseOutcome, pakkeRelease, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return discoverPakkeRelease(ctx, repo, name, pinnedSHA)
}

// offerPakkeRelease is the launch's release check for the pinned revision rev.
// It returns the revision to launch: rev, or the release the user chose to
// update to. The caller only calls it in a terminal.
//
// Which release is offered is #780's rule, discoverPakkeRelease's candidate,
// and nothing else. A failed lookup, a failed or aborted prompt and a failed
// update all launch rev, which tryPakkeLaunch then verifies as always.
func offerPakkeRelease(resolved ResolvedConfig, rev *Source) *Source {
	scope, err := ScopeUser()
	if err != nil {
		return rev
	}
	state, err := readScopedState(scope)
	if err != nil || state == nil {
		return rev
	}
	name := rev.Pakke.Name
	key := pakkeReleaseCacheKey(rev.Repo, name)
	cache := readPakkeReleaseCache()
	entry := cache[key]
	if !entry.fresh(state.SourceSHA, time.Now()) {
		outcome, rel, err := lookUpPakkeRelease(rev.Repo, name, state.SourceSHA, pakkeLaunchLookupTimeout)
		entry = pakkeReleaseCacheEntry{CheckedAt: time.Now(), PinnedSHA: state.SourceSHA, Dismissed: entry.Dismissed}
		_, follows := releaseClaim(state)
		switch {
		case errors.Is(err, errReleasesNotFound) && !follows:
			// A private repo without GITHUB_TOKEN. Sync and install read this
			// as no metadata for a pin that does not follow releases, and so
			// does the launch, without a line on every launch.
		case err != nil:
			entry.Failed = true
			msg, _, _ := strings.Cut(err.Error(), "\n")
			fmt.Fprintf(os.Stderr, "%s release check for %s failed: %s\n", yellow("⚠"), name, msg)
		case outcome == releaseCandidate:
			entry.Candidate = &rel
		}
		cache[key] = entry
		writePakkeReleaseCache(cache)
	}

	rel := entry.Candidate
	if rel == nil || rel.Version == entry.Dismissed {
		return rev
	}
	installed := shortSHA(state.SourceSHA)
	if version, _ := releaseClaim(state); version != "" {
		installed = version
	}
	update, err := confirmPakkeRelease(fmt.Sprintf("%s %s is available (you have %s). Update now?", name, rel.Version, installed))
	if err != nil {
		return rev // Ctrl-C is not "No": the question comes back next launch
	}
	if !update {
		entry.Dismissed = rel.Version
		cache[key] = entry
		writePakkeReleaseCache(cache)
		return rev
	}
	updated, err := activatePakkeRelease(resolved, scope, state, name, *rel)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s Could not update %s: %v\nLaunching the pinned revision %s.\n", yellow("⚠"), name, err, shortSHA(rev.SHA))
		// The cached answer may be what was wrong: look again next launch.
		entry.CheckedAt = time.Time{}
		cache[key] = entry
		writePakkeReleaseCache(cache)
		return rev
	}
	return updated
}

// activatePakkeRelease moves the pin to exactly rel the way sync --apply does:
// the release's own SHA, never a re-resolved branch, through pinRevision's
// verification and lost-update guard.
func activatePakkeRelease(resolved ResolvedConfig, scope *InstallScope, state *StateFile, name string, rel pakkeRelease) (*Source, error) {
	// The offer can be a day old. A release deleted, demoted or superseded
	// since then is not pinned as one. The person already said yes, so this
	// lookup gets the full timeout, not the launch's.
	outcome, current, err := lookUpPakkeRelease(state.SourceRepo, name, state.SourceSHA, pakkeReleaseTimeout)
	if err != nil {
		return nil, fmt.Errorf("checking %s %s again: %w", name, rel.Version, err)
	}
	if outcome != releaseCandidate || !sameSHA(current.SHA, rel.SHA) {
		return nil, fmt.Errorf("%s %s is no longer the stable release on offer; the pin is unchanged", name, rel.Version)
	}
	rel = current // same SHA; version and tag as the release says now

	relSrc, err := fetchPakkeRelease(state.SourceRepo, name, rel)
	if err != nil {
		return nil, err
	}
	defer relSrc.Cleanup()
	if !payloadOnly(relSrc) {
		return nil, fmt.Errorf("%s %s does not ship pre-built payloads only; the pin is unchanged", name, rel.Version)
	}
	payloadCtx := resolved.PayloadContext
	if payloadCtx == "" {
		payloadCtx = relSrc.Pakke.DefaultContext(resolved.Client)
	}
	if _, ok := relSrc.Pakke.Payload(resolved.Client, payloadCtx); !ok {
		return nil, fmt.Errorf("%s %s declares no %q payload for %s; the pin is unchanged", name, rel.Version, payloadCtx, resolved.Client)
	}
	// The prompt waited on a person, far longer than pinRevision's own window.
	latest, err := readScopedState(scope)
	if err != nil {
		return nil, fmt.Errorf("reading state: %w", err)
	}
	if pinMoved(state, latest) {
		return nil, fmt.Errorf("the pin changed to %s while you were asked; the pin is unchanged", pinLabel(latest))
	}
	revDir, err := pinRevision(scope, relSrc, &rel, false)
	if err != nil {
		return nil, err
	}
	fmt.Printf("%s Updated %s to %s.\n", green("✓"), bold(name), rel.label(relSrc.SHA))
	return &Source{Dir: revDir, SHA: relSrc.SHA, Repo: relSrc.Repo, Pakke: relSrc.Pakke}, nil
}
