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
	// Migration marks a candidate the downgrade guard did not offer, held out
	// as the move onto stable releases rather than as an update (#782).
	Migration bool `json:"migration,omitempty"`
	Failed    bool `json:"failed,omitempty"`
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
//
// ponytail: read-modify-write without a lock, so the last writer wins. Two
// launches at once, for different packages, can drop the other's entry: that
// costs one extra lookup, or asks once more about a version answered "No".
// Merge under a file lock if that ever matters.
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

// pakkeAnswer is what the person answered at startup. The zero value is the
// answer that moves nothing, so a prompt that fails moves nothing.
type pakkeAnswer int

const (
	answerLater pakkeAnswer = iota
	answerNow
	answerAuto
	answerKeep
)

// askPakkeRelease asks the question. A var so tests answer it. affirm labels the
// first answer: the migration question (#782) can offer a revision older than
// the pin, where "Update now" would not be true.
//
// The four answers carry two decisions in one list, which is what #781 asks for:
// what to do about this release, and what to do about every release after it. A
// follow-up question would arrive at the least welcome moment — right after
// someone answered the first one.
var askPakkeRelease = func(title, affirm string) (pakkeAnswer, error) {
	answer := answerLater
	err := huh.NewSelect[pakkeAnswer]().Title(title).Options(
		huh.NewOption(affirm, answerNow),
		huh.NewOption("Always, without asking from now on", answerAuto),
		huh.NewOption("Later", answerLater),
		huh.NewOption("Keep this revision and stop asking", answerKeep),
	).Value(&answer).WithTheme(navTheme()).Run()
	return answer, err
}

// recordUpdateChoice writes the durable choice a startup answer made (#781).
// Best effort, and deliberately so: it runs between a person answering and the
// client starting, and a state that cannot be written is not a reason to refuse
// the launch. The cost of losing it is the same question next time.
func recordUpdateChoice(scope *InstallScope, choice updateChoice) {
	state, err := readScopedState(scope)
	if err != nil || state == nil {
		return
	}
	state.UpdateChoice = string(choice)
	_ = writeScopedState(scope, state)
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
// Which release is offered is #780's rule: its candidate, and — for a pin that
// does not follow releases yet — the release it refused to offer as a downgrade
// (#782). Whether it is offered at all, asked about or taken outright is the
// scope's durable update choice (#781). A failed lookup, a failed or aborted
// prompt and a failed update all launch rev, which tryPakkeLaunch then verifies
// as always.
func offerPakkeRelease(resolved ResolvedConfig, rev *Source) *Source {
	scope, err := ScopeUser()
	if err != nil {
		return rev
	}
	state, err := readScopedState(scope)
	if err != nil || state == nil {
		return rev
	}
	// "Keep the revision" (#781) is answered here and not further down: it is an
	// answer about every future release, so there is nothing to look up, nothing
	// to cache and nothing to ask. A launch under it costs no network at all.
	// The news is not suppressed with the question — `nav-pilot sync` and
	// `status` still name a newer release and the command that takes it — but a
	// launch is not where someone who said "stop asking" is told again.
	choice := pakkeUpdateChoice(state)
	if choice == updateKeep {
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
		case outcome == releaseNotOffered && !follows:
			// The pin is ahead of or diverged from the newest stable release,
			// so the downgrade guard never offers it and the pin never starts
			// following on its own (#782). A pin installed from the default
			// branch before the package published releases sits here for good,
			// on development content nobody chose. Only a person can weigh a
			// possible downgrade against a subscription, so it is offered as
			// that question, once per version, and never taken automatically.
			entry.Candidate, entry.Migration = &rel, true
		}
		cache[key] = entry
		writePakkeReleaseCache(cache)
	}

	rel := entry.Candidate
	version, follows := releaseClaim(state)
	// A cache entry is evidence about the lookup it recorded, never about the
	// state now, and the migration was decided for a pin that did not follow
	// releases. The pin can start following without moving — a release cut from
	// the very revision it sits on does exactly that — and the entry stays
	// fresh, because freshness is keyed on the SHA. So the claim is read again
	// here: a following pin keeps the downgrade guard, which is its whole
	// protection (#782).
	// A revision this scope was rolled back off is not offered again (#783), and
	// not as a question either: a "yes" would pin the very revision the user
	// rejected. Any other candidate is offered as usual, so the fix arrives the
	// ordinary way. That refusal and "keep the revision" are one predicate
	// ([pakkeUpdateHold]), so a scope carrying both cannot get two answers.
	//
	// A dismissed version is a "Later", which is an answer about this version in
	// ask mode only: under "auto" nobody is asking, and skipping a release
	// because of a question answered before the mode was chosen would be the
	// automatic mode quietly not being automatic.
	dismissed := rel != nil && rel.Version == entry.Dismissed && choice != updateAuto
	if rel == nil || dismissed || (entry.Migration && follows) || pakkeUpdateHold(state, rel.SHA) != holdNone {
		return rev
	}
	installed := shortSHA(state.SourceSHA)
	if version != "" {
		installed = version
	}
	title := fmt.Sprintf("%s %s is available (you have %s). Update now?", name, rel.Version, installed)
	affirm := "Update now"
	if entry.Migration {
		// The candidate can be older than the pin — that is why it was not
		// offered as an update — and a yes also subscribes. Both revisions and
		// both consequences belong in the question.
		if version != "" {
			installed = version + " (" + shortSHA(state.SourceSHA) + ")"
		}
		title = fmt.Sprintf("%s is pinned at %s, which is not a stable release. The newest is %s, which may be older than what you have. Pin it and follow stable releases?",
			name, installed, rel.label(rel.SHA))
		affirm = "Pin it and follow stable releases"
	}

	// "auto" takes the release without asking — but never the migration, which
	// is the one candidate that can be a downgrade and a new subscription at
	// once. #782 leaves that to a person, and a mode that means "keep me on the
	// newest release" is not consent to go backwards.
	if choice == updateAuto && !entry.Migration {
		fmt.Printf("%s %s %s is available, and this scope takes new releases automatically.\n%s %s\n",
			dim("ℹ"), bold(name), rel.Version,
			dim("Be asked first:"), bold("nav-pilot sync --user --updates ask"))
	} else {
		answer, err := askPakkeRelease(title, affirm)
		if err != nil {
			return rev // Ctrl-C is not an answer: the question comes back next launch
		}
		switch answer {
		case answerLater:
			entry.Dismissed = rel.Version
			cache[key] = entry
			writePakkeReleaseCache(cache)
			return rev
		case answerKeep:
			// The pin stays where it is, and nothing is dismissed: the choice
			// covers this version and every later one, and a version-sized
			// memory of it would only be a second record saying less.
			recordUpdateChoice(scope, updateKeep)
			fmt.Printf("%s %s keeps revision %s. Change it with %s.\n",
				green("✓"), bold(name), shortSHA(state.SourceSHA), bold("nav-pilot sync --user --updates ask"))
			return rev
		case answerAuto:
			// Recorded before the update, not after: the update can fail, and
			// the answer to "what about the next release" was still given.
			recordUpdateChoice(scope, updateAuto)
		}
	}
	updated, err := activatePakkeRelease(resolved, scope, state, name, *rel, entry.Migration)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s Could not update %s: %v\nLaunching the pinned revision %s.\n", yellow("⚠"), name, err, shortSHA(rev.SHA))
		if errors.Is(err, errReleaseUnusable) {
			// Asking again would fail again on every launch.
			entry.Dismissed = rel.Version
			fmt.Fprintf(os.Stderr, "%s %s will not be offered again.\n", name, rel.Version)
		} else {
			// The cached answer may be what was wrong: look again next launch.
			entry.CheckedAt = time.Time{}
		}
		cache[key] = entry
		writePakkeReleaseCache(cache)
		return rev
	}
	return updated
}

// errReleaseUnusable marks a refusal about the release itself, which no retry
// changes. Its text keeps the messages as they were.
var errReleaseUnusable = errors.New("the pin is unchanged")

// activatePakkeRelease moves the pin to exactly rel the way sync --apply does:
// the release's own SHA, never a re-resolved branch, through pinRevision's
// verification and lost-update guard.
func activatePakkeRelease(resolved ResolvedConfig, scope *InstallScope, state *StateFile, name string, rel pakkeRelease, migration bool) (*Source, error) {
	// The offer can be a day old. A release deleted, demoted or superseded
	// since then is not pinned as one. The person already said yes, so this
	// lookup gets the full timeout, not the launch's.
	outcome, current, err := lookUpPakkeRelease(state.SourceRepo, name, state.SourceSHA, pakkeReleaseTimeout)
	if err != nil {
		return nil, fmt.Errorf("checking %s %s again: %w", name, rel.Version, err)
	}
	// A migration was offered over the downgrade guard's refusal (#782), so
	// that refusal is what the recheck is expected to answer again. Anything
	// else means there is no release to pin — releaseNoMetadata included,
	// which arrives with a nil error and is an answer, not a success.
	offered := outcome == releaseCandidate || (migration && outcome == releaseNotOffered)
	if !offered || !sameSHA(current.SHA, rel.SHA) {
		return nil, fmt.Errorf("%s %s is no longer the stable release on offer; the pin is unchanged", name, rel.Version)
	}
	rel = current // same SHA; version and tag as the release says now

	relSrc, err := fetchPakkeRelease(state.SourceRepo, name, rel)
	if errors.Is(err, errReleaseNotThisPackage) {
		return nil, fmt.Errorf("%w; %w", err, errReleaseUnusable)
	}
	if err != nil {
		return nil, err // a clone or network failure: worth asking again
	}
	defer relSrc.Cleanup()
	if !payloadOnly(relSrc) {
		return nil, fmt.Errorf("%s %s does not ship pre-built payloads only; %w", name, rel.Version, errReleaseUnusable)
	}
	payloadCtx := resolved.PayloadContext
	if payloadCtx == "" {
		payloadCtx = relSrc.Pakke.DefaultContext(resolved.Client)
	}
	if _, ok := relSrc.Pakke.Payload(resolved.Client, payloadCtx); !ok {
		return nil, fmt.Errorf("%s %s declares no %q payload for %s; %w", name, rel.Version, payloadCtx, resolved.Client, errReleaseUnusable)
	}
	// The prompt waited on a person, far longer than pinRevision's own window.
	latest, err := readScopedState(scope)
	if err != nil {
		return nil, fmt.Errorf("reading state: %w", err)
	}
	if pinMoved(state, latest) {
		return nil, fmt.Errorf("the pin changed to %s while you were asked; the pin is unchanged", pinLabel(latest))
	}
	revDir, err := pinRevision(scope, relSrc, &rel, false, false)
	if err != nil {
		return nil, err
	}
	fmt.Printf("%s Updated %s to %s.\n", green("✓"), bold(name), rel.label(relSrc.SHA))
	return &Source{Dir: revDir, SHA: relSrc.SHA, Repo: relSrc.Repo, Pakke: relSrc.Pakke}, nil
}
