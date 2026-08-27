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

// stagedMaxAge is how long a staged payload tree may survive its launch before
// the next staged launch sweeps it. A tree is normally removed when the client
// exits; anything older than this was left behind by a crash.
//
// ponytail: naive age heuristic. A session running longer than a day would have
// its config swept out from under it — visible at the next config read, not
// unsafe. Upgrade to owner-pid files only if that turns out to happen.
const stagedMaxAge = 24 * time.Hour

// stagedRoot is where verified Tier 2 payloads are staged: ~/.nav-pilot/staged,
// a sibling of config.toml. It follows configPath(), so NAV_PILOT_CONFIG
// relocates it too.
func stagedRoot() string {
	return filepath.Join(filepath.Dir(configPath()), "staged")
}

// tryPakkeLaunch runs the Tier 2 (payload) launch path when — and only when —
// the resolved source ships an agentpakke manifest that declares pre-built
// payloads for the client being launched. It reports whether it handled the
// launch; (false, nil) means the caller should launch the legacy way.
//
// The first check is the no-regression gate: with no source selected (the
// default for every user who has never run `config set source` or passed
// --source) it returns immediately, having resolved nothing, read nothing, and
// left the active agentpakke on the built-in default.
func tryPakkeLaunch(resolved ResolvedConfig) (bool, error) {
	if resolved.Source == "" {
		return false, payloadContextUnsupported(resolved, defaultSourceRepo)
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
	// Resolving a repo-shaped source clones it. A Tier 1 or manifest-less
	// source has nothing to stage, so before this PR that launch never touched
	// the source at all; without a memory it would now clone on every launch to
	// re-learn an answer it already had. A remembered non-payload tier skips
	// the clone and takes exactly the legacy path.
	//
	// --payload-context is exempt on purpose: it asks about payloads the
	// manifest declares, which only the manifest can answer.
	if resolved.PayloadContext == "" {
		if tier, ok := cachedTier(resolved.Source, resolved.Client); ok && tier != agentpakke.TierPayload {
			return false, nil
		}
	}

	src, err := resolveSource("", resolved.Source)
	if err != nil {
		if errors.Is(err, errUnusableManifest) {
			return true, err
		}
		fmt.Fprintf(os.Stderr, "%s Could not resolve source %s: %v — launching without agentpakke context.\n",
			yellow("⚠"), resolved.Source, err)
		return false, payloadContextUnsupported(resolved, resolved.Source)
	}

	tier := agentpakke.TierUnknown
	if src.Pakke != nil {
		tier = src.Pakke.Tier(resolved.Client)
	}
	rememberTier(resolved.Source, resolved.Client, tier)
	if tier != agentpakke.TierPayload {
		// Manifest-less or Tier 1: exactly today's path, built-in default
		// agentpakke still active.
		src.Cleanup()
		return false, payloadContextUnsupported(resolved, resolved.Source)
	}

	pakke := src.Pakke
	context := resolved.PayloadContext
	if context == "" {
		context = pakke.DefaultContext(resolved.Client)
	}
	payload, ok := pakke.Payload(resolved.Client, context)
	if !ok {
		src.Cleanup()
		return true, fmt.Errorf("agentpakke %q declares no %q payload for %s (declared contexts: %s)",
			pakke.Name, context, resolved.Client, strings.Join(declaredContexts(pakke, resolved.Client), ", "))
	}

	root := stagedRoot()
	// Best-effort sweep of trees a crashed session left behind. It costs one
	// directory read on a path that is about to do far more I/O than that.
	_ = agentpakke.GCStaged(root, stagedMaxAge)

	stagedDir, stageErr := agentpakke.StagePayload(
		filepath.Join(src.Dir, filepath.FromSlash(payload.Path)),
		filepath.Join(src.Dir, filepath.FromSlash(payload.ManifestPath())),
		root)
	// The staged tree carries its own manifest, so the checkout — which may be
	// a temp clone — is no longer needed either way.
	src.Cleanup()
	if stageErr != nil {
		// Fail-closed (G2): a Tier 2 launch never falls back to the legacy path.
		return true, fmt.Errorf("staging the %q payload of agentpakke %q for %s: %w",
			context, pakke.Name, resolved.Client, stageErr)
	}
	defer func() {
		// A tree that survives is verified config left on disk; say so, and
		// let the 24h sweep pick it up.
		if err := agentpakke.CleanupStaged(stagedDir); err != nil {
			fmt.Fprintf(os.Stderr, "%s %v\n", yellow("⚠"), err)
		}
	}()

	// The only SetActivePakke call site: past the tier gate the manifest is
	// known to declare this client, which is the invariant provider.PrimaryAgent
	// relies on.
	providerpkg.SetActivePakke(pakke)

	staged := providerpkg.StagedLaunch{Dir: stagedDir, PakkeName: pakke.Name, Context: context}
	switch resolved.Client {
	case "opencode":
		return true, providerpkg.LaunchOpenCodeStaged(resolved, staged)
	case "copilot":
		return true, providerpkg.LaunchCopilotStaged(resolved, staged)
	default:
		return true, fmt.Errorf("agentpakke %q declares payloads for %s, but this nav-pilot cannot launch staged payloads for that client",
			pakke.Name, resolved.Client)
	}
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

// tierCacheTTL bounds how long a remembered tier is trusted. A pakke that
// changes tier — Tier 1 adopting payloads, or dropping them — must be picked up
// without the user knowing a cache exists, and six hours means at worst one
// working day's delay while a burst of launches costs a single clone.
//
// ponytail: like stagedMaxAge, an arbitrary-but-bounded number, not a measured
// one. Nothing was timed to pick it; it is short enough that a stale answer
// self-corrects the same day and long enough that a normal day's launches do
// not re-clone. Shorten it if tier changes turn out to need faster pickup.
const tierCacheTTL = 6 * time.Hour

// tierCachePath is ~/.nav-pilot/tier-cache.json, a sibling of config.toml and
// of stagedRoot(). It is nav-pilot's own state, not an OS cache directory that
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
