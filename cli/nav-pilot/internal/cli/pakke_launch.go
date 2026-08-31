package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/navikt/copilot/cli/nav-pilot/internal/agentpakke"
	providerpkg "github.com/navikt/copilot/cli/nav-pilot/internal/provider"
)

// tryPakkeLaunch runs the Tier 2 (payload) launch path when — and only when —
// the agentpakke governing the resolved source declares pre-built payloads for
// the client being launched. It reports whether it handled the launch; (false,
// nil) means the caller should launch the legacy way.
//
// The first check is the no-regression gate: with no source selected (the
// default for every user who has never run `config set source` or passed
// --source) it returns immediately, having resolved nothing, read nothing, and
// left the active agentpakke on the built-in default.
//
// Past that gate the launch is a lookup. A pinned revision — written once by
// `nav-pilot install`, or by the first launch of an un-installed payload-only
// source — is verified where it lies and handed to the client. Nothing is
// cloned, copied or staged per launch, and no launch ever reads a moving
// default branch: a new revision arrives through `nav-pilot sync`, never
// through launching again.
func tryPakkeLaunch(resolved ResolvedConfig) (bool, error) {
	if resolved.Source == "" {
		return false, payloadContextUnsupported(resolved, defaultSourceRepo)
	}

	// The pin answers before anything is resolved. An error here is
	// fail-closed rather than a fall-through: a revision whose manifest this
	// binary cannot use must say "upgrade", never quietly re-clone the moving
	// branch the pin exists to get away from.
	rev, err := pinnedRevision(resolved.Source)
	if err != nil {
		return true, err
	}

	if rev != nil {
		if err := mixedPakkeRefusal(rev.Pakke, resolved.Client); err != nil {
			return true, err
		}
		if rev.Pakke.Tier(resolved.Client) != agentpakke.TierPayload {
			// The pinned agentpakke declares no payload for this client, so
			// there is nothing pinned to launch from: exactly today's legacy
			// path, and reached without resolving anything.
			return false, payloadContextUnsupported(resolved, resolved.Source)
		}
	} else {
		var handled bool
		rev, handled, err = resolveAndPin(resolved)
		if rev == nil {
			return handled, err
		}
	}

	pakke := rev.Pakke
	context := resolved.PayloadContext
	if context == "" {
		context = pakke.DefaultContext(resolved.Client)
	}
	if _, ok := pakke.Payload(resolved.Client, context); !ok {
		return true, fmt.Errorf("agentpakke %q declares no %q payload for %s (declared contexts: %s)",
			pakke.Name, context, resolved.Client, strings.Join(declaredContexts(pakke, resolved.Client), ", "))
	}

	// The tree has been sitting in ~/.nav-pilot/pakker since the pin was made,
	// so it is verified again here rather than trusted: the exact hash walk
	// checks every manifested file's digest and permission bits, rejects
	// unmanifested extras, and refuses symlinks and non-regular files anywhere
	// in it. Fail-closed — a Tier 2 launch never degrades to the legacy path.
	dir := filepath.Join(rev.Dir, resolved.Client, context)
	if err := agentpakke.VerifyPayloadExact(dir, filepath.Join(dir, agentpakke.PayloadManifestFile)); err != nil {
		return true, fmt.Errorf("the pinned %q payload of agentpakke %q for %s does not match its manifest: %w",
			context, pakke.Name, resolved.Client, err)
	}

	// The only SetActivePakke call site: past the tier gate the manifest is
	// known to declare this client, which is the invariant provider.PrimaryAgent
	// relies on. It is the manifest pinned with the payloads, so persona, model
	// and the compatibility gate all read the revision that is about to run.
	providerpkg.SetActivePakke(pakke)
	fmt.Println(dim(fmt.Sprintf("Using agentpakke %s@%s (%s payload).", pakke.Name, rev.SHA, context)))

	launch, ok := stagedLaunchers[resolved.Client]
	if !ok {
		return true, fmt.Errorf("agentpakke %q declares payloads for %s, but this nav-pilot cannot launch staged payloads for that client",
			pakke.Name, resolved.Client)
	}
	// After the handover gate: the notice announces a session that is about to
	// start, and this is the last point that can still refuse to start one.
	printModelNotice(resolved)
	return true, launch(resolved, providerpkg.StagedLaunch{Dir: dir, PakkeName: pakke.Name, Context: context})
}

// stagedLaunchers is the one list of clients this binary can hand a pinned
// payload tree to.
//
// It is a map rather than a switch because it has a second reader: `nav-pilot
// list` annotates the payload clients it cannot launch, and that annotation has
// to mean the same thing as the refusal above. Keying the listing on
// agentpakke.IsKnownClient instead is what let a `pi` payload be listed without
// a warning and then refuse at launch — IsKnownClient is the wider set of
// clients nav-pilot knows at all, and the clients the warning exists for are
// precisely the ones in the gap. A client gaining a launcher must change one
// list, here.
var stagedLaunchers = map[string]func(ResolvedConfig, providerpkg.StagedLaunch) error{
	"copilot":  providerpkg.LaunchCopilotStaged,
	"opencode": providerpkg.LaunchOpenCodeStaged,
}

// pinnedRevision returns the revision this user has pinned for a source, or nil
// when there is none. Nil is the ordinary answer for a Tier 1 install, a
// manifest-less source, and a payload-only source nobody has installed yet.
//
// It reads user scope only, which is where every Tier 2 install records its pin
// (guardPakkeScope refuses the others). The recorded repo must match the
// configured one — otherwise a scope installed from one agentpakke would hand
// its revision to a launch configured for another — and the revision directory
// must actually exist, which is what makes an uninstalled or hand-deleted pin
// fall through to a normal resolve rather than failing.
//
// A manifest that will not load is the one hard failure: [attachPakke]'s
// fail-closed rule applies to the pinned manifest exactly as it does to a fresh
// checkout.
func pinnedRevision(sourceRepo string) (*Source, error) {
	if !pinnable(sourceRepo) {
		// A local path is the author's own working tree and is never pinned, so
		// anything left under its placeholder SHA by an older binary is stale
		// by construction: adopting it would hand back whatever the tree held
		// the first time it was launched.
		return nil, nil
	}
	scope, err := ScopeUser()
	if err != nil {
		return nil, nil //nolint:nilerr // no user scope means no pin, not a launch failure
	}
	state, err := readScopedState(scope)
	if err != nil || state == nil {
		return nil, nil //nolint:nilerr // an unreadable state file is the install commands' error to report
	}
	if state.SourceSHA == "" || !sameSourceRepo(state.SourceRepo, sourceRepo) {
		return nil, nil
	}
	dir := pakkeRevisionDir(state.SourceRepo, state.SourceSHA)
	if _, err := os.Stat(dir); err != nil {
		return nil, nil //nolint:nilerr // nothing materialized under this SHA: resolve as usual
	}

	src := &Source{Dir: dir, SHA: state.SourceSHA, Repo: state.SourceRepo}
	if err := attachPakke(src); err != nil {
		return nil, err
	}
	if src.Pakke == nil {
		return nil, nil // a revision directory without a manifest is not a pin
	}
	return src, nil
}

// resolveAndPin is the un-pinned half of the launch: it resolves the source the
// way every launch did before the pin existed, and — when what comes back is a
// payload-only agentpakke declaring this client — pins it before running, so
// this is the last launch that clones anything.
//
// A nil revision means this launch produced none. The bool that comes with it
// is tryPakkeLaunch's "handled": false hands the launch to the legacy path
// (with or without an error of its own), true means the Tier 2 path took it and
// failed closed.
func resolveAndPin(resolved ResolvedConfig) (*Source, bool, error) {
	// Resolving a repo-shaped source clones it. A Tier 1 or manifest-less
	// source has nothing to pin, so that launch never touched the source at
	// all before Tier 2 existed; without a memory it would now clone on every
	// launch to re-learn an answer it already had. A remembered non-payload
	// tier skips the clone and takes exactly the legacy path.
	//
	// --payload-context is exempt on purpose: it asks about payloads the
	// manifest declares, which only the manifest can answer.
	if resolved.PayloadContext == "" {
		if tier, ok := cachedTier(resolved.Source, resolved.Client); ok && tier != agentpakke.TierPayload {
			return nil, false, nil
		}
	}

	// The CLI's source funnel: applies source precedence and attaches the
	// schema-validated manifest.
	//
	// A *fetch* failure lands before the tier gate, so nothing yet says this
	// launch is Tier 2 — the source may well declare no payloads at all. Being
	// offline, or having a stale repo name in config, must therefore not block
	// a launch that worked before Tier 2 staging existed: warn and take the
	// legacy path, as EnsureOpenCodeNavContext has always done.
	//
	// A manifest-validation failure arrives through the same error return but
	// is the opposite case: the source was fetched, and what it ships says this
	// nav-pilot must not run it (attachPakke's fail-closed rule, A3). Degrading
	// there would inject the user's own ~/.copilot into a launch the pakke
	// author never sanctioned, behind a stderr warning the client TUI wipes off
	// the screen — and would silently strand every older binary the day a pakke
	// adopts contract major 2. errUnusableManifest separates the two.
	//
	// The cache decides between them for the third case: a fetch failure over a
	// source an earlier launch already resolved to TierPayload. "Nothing yet
	// says this launch is Tier 2" is true of a source nav-pilot has never seen;
	// it is not true here, because rememberTier wrote TierPayload for exactly
	// this source and client. Warning and taking the legacy path there is the
	// same silent downgrade mixedPakkeRefusal and errUnusableManifest exist to
	// refuse, so it fails closed instead.
	src, err := resolveSource("", resolved.Source)
	if err != nil {
		if errors.Is(err, errUnusableManifest) {
			return nil, true, err
		}
		if tier, ok := cachedTier(resolved.Source, resolved.Client); ok && tier == agentpakke.TierPayload {
			return nil, true, unresolvablePayloadRefusal(resolved, err)
		}
		fmt.Fprintf(os.Stderr, "%s Could not resolve source %s: %v — launching without agentpakke context.\n",
			yellow("⚠"), resolved.Source, err)
		return nil, false, payloadContextUnsupported(resolved, resolved.Source)
	}
	defer src.Cleanup()

	tier := agentpakke.TierUnknown
	if src.Pakke != nil {
		tier = src.Pakke.Tier(resolved.Client)
	}
	rememberTier(resolved.Source, resolved.Client, tier)
	if err := mixedPakkeRefusal(src.Pakke, resolved.Client); err != nil {
		return nil, true, err
	}
	if tier != agentpakke.TierPayload {
		// Manifest-less, Tier 1, or Tier 2 for a client this pakke does not
		// declare: exactly today's path, built-in default agentpakke still
		// active.
		return nil, false, payloadContextUnsupported(resolved, resolved.Source)
	}

	// Past the tier gate the launch is Tier 2 and fails closed: it never
	// degrades to the legacy path with the user's own ~/.copilot injected.
	rev, err := autoPin(src)
	return rev, true, err
}

// autoPin materializes and records a pin for a payload-only source that has
// never been installed, so the launch it is part of is the last one to resolve.
//
// Materializing without recording the pin is not an option: the next launch
// would clone again and the pin would never take. Which is exactly why an
// install already in this scope is refused rather than overwritten — writing
// the pin replaces that state, and [pinRevision] removes the files it tracked
// and the revisions it left. An explicit install is the consent for that; a
// launch has no consent gesture at all, so it stops and names the command that
// does.
func autoPin(src *Source) (*Source, error) {
	scope, err := ScopeUser()
	if err != nil {
		return nil, fmt.Errorf("locating your user scope to pin agentpakke %q: %w", src.Pakke.Name, err)
	}

	// A local path is never pinned: it is re-materialized every launch, so an
	// edit to the working tree shows up on the next one. Nothing is recorded,
	// so nothing in this scope is replaced and the guard below does not apply.
	//
	// The prune is not optional here. A git-backed working tree resolves to its
	// short HEAD, so every commit its author launches from lands under a new
	// revision directory — and nothing else would ever remove them: the prune's
	// other caller is [pinRevision], which a local source never reaches, and
	// uninstall keys on a state entry a local source never writes. Only the
	// revision this launch is about to hand over survives.
	//
	// A running session can be reading the revision this one is about to
	// rebuild — a local source is rebuilt in place on every launch, and its
	// payload directory is a live session's OPENCODE_CONFIG_DIR — which is why
	// [materializeRevision] renames the old tree aside instead of removing it.
	// The prune below removes the SHA directories this launch left behind, not
	// the tree a session already has open.
	if !pinnable(src.Repo) {
		revDir, err := materializeRevision(src)
		if err != nil {
			return nil, err
		}
		prunePakkeRevisions(src.Repo, src.SHA)
		return &Source{Dir: revDir, SHA: src.SHA, Repo: src.Repo, Pakke: src.Pakke}, nil
	}

	if state, err := readScopedState(scope); err == nil && state != nil {
		// Files would be orphaned by the zero-item pin that replaces them, and
		// a recorded pin on another source has whole materialized revision
		// trees behind it that pinRevision removes. Either way a launch would
		// be deleting content it was never asked to delete.
		//
		// This is len(Files) and not [installsContent], and the difference is
		// deliberate: the two are answering different questions. pinnedState
		// asks whether a state *is* a pin, and an ignore marker leaves it one.
		// This asks whether writing a pin here would destroy something the user
		// chose, and an ignore marker is exactly that — a deliberate choice,
		// recorded nowhere else. The sequence is reachable: pin, `nav-pilot
		// ignore <item> --user`, then the revision directory goes (hand-deleted,
		// or a pakker/ wipe). The next launch finds no pin and lands here, and
		// under the looser test it would materialize, write a fresh zero-item
		// state and drop the markers without a word. So it refuses and names
		// `install`, which discards them too — but only because the user asked.
		foreign := !sameSourceRepo(state.SourceRepo, src.Repo)
		if len(state.Files) > 0 || (foreign && state.SourceSHA != "") {
			return nil, fmt.Errorf(
				"launching from agentpakke %q means pinning it for your user, but %s is already installed from %s.\n"+
					"nav-pilot will not remove another agentpakke's files or pinned revisions as a side effect of a launch.\n\n"+
					"  Switch to it deliberately:  %s",
				src.Pakke.Name, bold(state.Collection), bold(sourceLabelForRepo(state.SourceRepo)),
				bold("nav-pilot install --user "+src.Pakke.Name))
		}
	}

	// A launch has no JSON mode: everything it prints is for a person.
	revDir, err := pinRevision(scope, src, false)
	if err != nil {
		return nil, err
	}
	fmt.Printf("%s Pinned %s at %s — future launches use the local copy; %s updates it.\n",
		green("✓"), bold(src.Pakke.Name), src.SHA, bold("nav-pilot sync"))
	return &Source{Dir: revDir, SHA: src.SHA, Repo: src.Repo, Pakke: src.Pakke}, nil
}

// unresolvablePayloadRefusal refuses a launch of a source a previous launch
// already learned declares pre-built payloads for this client, when this launch
// cannot reach it and has no pinned revision to fall back on — offline with the
// revision directory gone, or a repo name that has moved.
//
// The legacy path is not an option here for the same reason it is not one past
// the tier gate: it would materialize the user's own ~/.copilot into a launch
// the manifest reserves for a verified payload tree, and say so only in a
// stderr warning the client TUI wipes off the screen. The remembered tier is
// what makes this knowable before the resolve succeeds.
//
// Nothing recovers this offline, so both commands named are for once the source
// is reachable again: sync rebuilds the missing revision behind a pin that is
// still recorded, and clearing the source is the deliberate way back to the
// built-in default.
func unresolvablePayloadRefusal(resolved ResolvedConfig, cause error) error {
	return fmt.Errorf(
		"source %s declares pre-built payloads for %s, and nav-pilot could not reach it: %v.\n"+
			"Nothing was launched — running it as before instead would materialize your own ~/.copilot into a launch the agentpakke reserves for a verified payload tree.\n\n"+
			"  Once the source is reachable, rebuild the pinned revision:  %s\n"+
			"  Or go back to the built-in agentpakke:                      %s",
		bold(resolved.Source), resolved.Client, cause,
		bold("nav-pilot sync --apply"),
		bold(`nav-pilot config set source ""`))
}

// sourceLabelForRepo names a recorded source repo in user-facing messages,
// falling back to the default source for an install that predates source
// tracking — which is where it must have come from.
func sourceLabelForRepo(repo string) string {
	if repo == "" {
		return defaultSourceRepo
	}
	return repo
}

// mixedPakkeRefusal refuses a launch of a payload-bearing client belonging to a
// pakke that also ships a layout, and returns nil for every other shape.
//
// Pinning does not cover mixed pakker this release: a pin replaces the scope's
// state with a zero-item entry, which would throw away the record of the Tier 1
// files that same pakke's install wrote. The alternative — quietly taking the
// legacy path — is worse than refusing: the client's manifest declares a
// verified payload, and materializing Tier 1 content into its config instead is
// a downgrade the user never sees. So this fails closed, like every other Tier
// 2 refusal.
func mixedPakkeRefusal(pakke *agentpakke.Manifest, client string) error {
	if pakke == nil || pakke.Layout == nil || pakke.Tier(client) != agentpakke.TierPayload {
		return nil
	}
	return fmt.Errorf(
		"agentpakke %q declares pre-built payloads for %s and a Tier 1 layout, and nav-pilot cannot pin a revision of a pakke that ships both.\n"+
			"Nothing was launched — running it as Tier 1 instead would silently materialize layout content into %s's config in place of the payload the manifest declares.\n\n"+
			"  Launch a client the pakke declares no payload for, which uses the layout:  %s\n"+
			"  Or ask the agentpakke to split the payloads out, and tell us you need this: %s",
		pakke.Name, client, client,
		bold("nav-pilot --client <other>"),
		bold("https://github.com/navikt/copilot/issues"))
}

// payloadContextUnsupported turns an explicit --payload-context into an error
// when the launch resolves to the legacy path. Ignoring the flag silently would
// mask what the user asked for, the same policy unknown config keys get.
func payloadContextUnsupported(resolved ResolvedConfig, label string) error {
	if resolved.PayloadContext == "" {
		return nil
	}
	return fmt.Errorf("--payload-context requires an agentpakke with pre-built payloads; source %s declares none for client %s",
		label, resolved.Client)
}

// declaredContexts lists a client's payload context ids, sorted.
func declaredContexts(m *agentpakke.Manifest, client string) []string {
	entry, ok := m.Client(client)
	if !ok {
		return nil
	}
	ids := make([]string, 0, len(entry.Payloads))
	for id := range entry.Payloads {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// ─── tier memory ─────────────────────────────────────────────────────────────
//
// The launch path caches a *decision* — which tier a source declares for a
// client, and when that was learned — never content. A miss, a stale entry, an
// unreadable or corrupt file all mean "resolve the source as usual"; none of
// them may be read as "assume legacy".

// Since the pin landed, the path this cache serves on the happy day is a custom
// source that resolves to Tier 1 or ships no manifest. tryPakkeLaunch still
// runs for it — the gate above is an empty-source check, not a tier check —
// purely to learn a tier it will not act on, and without the cache that is a
// `git clone --depth 1` on every launch. Everything payload-shaped is normally
// answered by the pin before the cache is consulted, and a first launch pins
// rather than caching an answer it will not need again.
//
// The remembered TierPayload answer is not dead weight, though. It is the only
// thing left saying "this is Tier 2" on the one launch where the pin is gone
// and the source cannot be reached, and [unresolvablePayloadRefusal] reads it
// there to refuse instead of degrading to legacy.
//
// There are therefore two read sites, and they ask opposite questions: the gate
// in resolveAndPin skips the resolve on a remembered non-payload answer, and
// the fetch-failure branch below it refuses on a remembered payload answer.
//
// Deletion trigger: when Tier 1 installs pin too (the default agentpakke
// becoming navikt/copilot), the last caller goes and this file goes with it.
//
// tierCacheTTL bounds how long a remembered tier is trusted. A pakke that
// changes tier — Tier 1 adopting payloads, or dropping them — must be picked up
// without the user knowing a cache exists, and six hours means at worst one
// working day's delay while a burst of launches costs a single clone.
//
// ponytail: an arbitrary-but-bounded number, not a measured one. Nothing was
// timed to pick it; it is short enough that a stale answer self-corrects the
// same day and long enough that a normal day's launches do not re-clone.
// Shorten it if tier changes turn out to need faster pickup.
const tierCacheTTL = 6 * time.Hour

// tierCachePath is ~/.nav-pilot/tier-cache.json, a sibling of config.toml and
// of pakkerRoot(). It is nav-pilot's own state, not an OS cache directory that
// may be swept between launches; NAV_PILOT_CONFIG relocates it too.
func tierCachePath() string {
	return filepath.Join(filepath.Dir(configPath()), "tier-cache.json")
}

type tierCacheEntry struct {
	Tier      int       `json:"tier"`
	LearnedAt time.Time `json:"learnedAt"`
}

// tierCacheKey names the source as well as the client, so changing `source`
// asks a question this cache has never been asked before rather than inheriting
// the old source's answer.
func tierCacheKey(source, client string) string { return source + "\x00" + client }

// tierCacheable reports whether a source is worth remembering. Only repo-shaped
// sources are: an absolute path resolves with a stat and no clone, so there is
// nothing to save, and a developer editing the manifest in their own checkout
// must see the change on the next launch.
func tierCacheable(source string) bool {
	return source != "" && !filepath.IsAbs(source)
}

// cachedTier returns a remembered tier and whether it may be trusted.
func cachedTier(source, client string) (int, bool) {
	if !tierCacheable(source) {
		return agentpakke.TierUnknown, false
	}
	data, err := os.ReadFile(tierCachePath())
	if err != nil {
		return agentpakke.TierUnknown, false
	}
	var cache map[string]tierCacheEntry
	if err := json.Unmarshal(data, &cache); err != nil {
		return agentpakke.TierUnknown, false
	}
	entry, ok := cache[tierCacheKey(source, client)]
	if !ok {
		return agentpakke.TierUnknown, false
	}
	// Well-formed JSON is not a trustworthy answer. A tier no nav-pilot writes
	// (a hand-edited file, or a future binary with other tier semantics) would
	// be neither TierPayload nor a known legacy tier, and the launch would skip
	// the resolve and run legacy for a source that declares a payload. A
	// learnedAt in the future — clock stepped back, or skew when it was written
	// — makes the age negative, so the entry would outlive the TTL until
	// wall-clock catches up. Both resolve normally, exactly like a corrupt file.
	switch entry.Tier {
	case agentpakke.TierUnknown, agentpakke.TierLayout, agentpakke.TierPayload:
	default:
		return agentpakke.TierUnknown, false
	}
	if age := time.Since(entry.LearnedAt); age < 0 || age > tierCacheTTL {
		return agentpakke.TierUnknown, false
	}
	return entry.Tier, true
}

// rememberTier records what a resolve just learned. Every failure is silent: a
// cache that cannot be written costs a clone next launch and nothing else.
func rememberTier(source, client string, tier int) {
	if !tierCacheable(source) {
		return
	}
	cache := map[string]tierCacheEntry{}
	if data, err := os.ReadFile(tierCachePath()); err == nil {
		if err := json.Unmarshal(data, &cache); err != nil {
			cache = map[string]tierCacheEntry{}
		}
	}
	cache[tierCacheKey(source, client)] = tierCacheEntry{Tier: tier, LearnedAt: time.Now()}
	data, err := json.Marshal(cache)
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(tierCachePath()), 0o755); err != nil {
		return
	}
	_ = os.WriteFile(tierCachePath(), data, 0o644)
}

// printModelNotice names the model this launch resolved to and where it came
// from. A user could otherwise start a session with no idea which model was
// about to bill them, since the model can come from a config file, a flag or an
// agentpakke declaration and nothing said so.
//
// Two call sites rather than one, because the funnel has two exits: Tier 2
// launches from inside tryPakkeLaunch, and its notice has to come after
// SetActivePakke or it would read the wrong pakke's declaration, and after the
// handover gate or it would announce a launch that is then refused. Tier 1
// prints from launchClientConfirming, just before the client starts.
//
// One line, on stderr, and only with a terminal, so scripted and piped runs are
// byte-identical to what they were.
func printModelNotice(resolved ResolvedConfig) {
	if !isInteractive() {
		return
	}
	if notice := providerpkg.ResolvedModelNotice(resolved.Client, resolved); notice != "" {
		fmt.Fprintln(os.Stderr, dim(notice))
	}
}
