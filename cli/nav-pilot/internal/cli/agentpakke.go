package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/navikt/copilot/cli/nav-pilot/internal/agentpakke"
)

// This file is the CLI's agentpakke seam. Source resolution funnels through
// attachPakke, and everything downstream of it reads content through a
// [Manifest]-derived resolver — the single-currency rule from the agentpakke
// package doc. A source that ships no manifest gets one from
// agentpakke.SynthesizeLegacy at the top of the pipeline (pakkeFor), so no
// install or sync code below this point branches on "legacy or agentpakke".
//
// The seam stops at the CLI's install/sync source resolution: persona, model,
// and launch call sites (internal/provider, internal/source/frontmatter.go)
// still read their hardcoded Nav defaults and are migrated in M2.

// attachPakke loads the agentpakke manifest of a resolved checkout onto the
// source, fail-closed (A3): a source that ships a manifest must have a
// conforming one before nav-pilot installs, syncs, or launches anything from
// it. A source with no manifest is not an error — it is the legacy case, and
// src.Pakke stays nil for pakkeFor to adapt.
func attachPakke(src *Source) error {
	m, err := agentpakke.Load(src.Dir)
	if err != nil {
		if errors.Is(err, agentpakke.ErrNoManifest) {
			src.Pakke = nil
			return nil
		}
		return &unusableManifestError{msg: fmt.Sprintf("%s ships an agentpakke manifest (%s) that this nav-pilot cannot use:\n  %v\n\nNothing was installed. Fix the manifest in the source repo, or run %s to list every finding",
			sourceLabelFor(src), agentpakke.ManifestPath, err,
			bold("nav-pilot validate --source "+sourceLabelFor(src)))}
	}
	src.Pakke = m
	return nil
}

// errUnusableManifest is the sentinel behind every attachPakke validation
// failure — a non-conforming manifest, an unsupported contractVersion, a
// nav-pilot too old for the pakke's minimum version.
//
// It exists so callers that degrade gracefully on a *resolve* failure (being
// offline, a repo name that no longer exists) can still fail closed on a
// manifest failure, which arrives through the same error return. Without it
// the day a pakke adopts contract major 2, every older nav-pilot would quietly
// drop to the built-in default instead of saying "upgrade".
//
// This mirrors agentpakke.ErrNoManifest, the package's other errors.Is
// sentinel: manifest present but broken, versus manifest absent.
var errUnusableManifest = errors.New("unusable agentpakke manifest")

// unusableManifestError carries attachPakke's user-facing message — which
// already says everything — while errors.Is can see errUnusableManifest behind
// it. Wrapping with %w would prefix the sentinel's text onto that message.
type unusableManifestError struct{ msg string }

func (e *unusableManifestError) Error() string { return e.msg }
func (e *unusableManifestError) Unwrap() error { return errUnusableManifest }

// pakkeFor returns the manifest that governs an install from this source: the
// one the source ships, or the legacy adapter naming the collection being
// installed. Past this call nothing needs to know which of the two it got.
func pakkeFor(src *Source, collection string) *agentpakke.Manifest {
	if src != nil && src.Pakke != nil {
		return src.Pakke
	}
	return agentpakke.SynthesizeLegacy(collection)
}

// resolverFor builds the content resolver for a manifest. The legacy adapter
// declares the canonical agents//skills/ layout, so manifest-less sources
// resolve exactly as they did before this seam existed.
func resolverFor(sourceDir string, pakke *agentpakke.Manifest) *SourceResolver {
	if pakke == nil {
		return NewSourceResolver(sourceDir)
	}
	return NewSourceResolverForLayout(sourceDir, pakke.Layout)
}

// resolverForState builds the resolver used by sync for an existing install.
// The source may or may not ship a manifest; the collection recorded in state
// names the legacy adapter when it does not.
func resolverForState(src *Source, state *StateFile) *SourceResolver {
	collection := ""
	if state != nil {
		collection = state.Collection
	}
	return resolverFor(src.Dir, pakkeFor(src, collection))
}

// stateCollection returns what an install records in StateFile.Collection.
//
// A manifest-bearing source supersedes the collection model, so its identity is
// the agentpakke name (A1/Q2). A manifest-less source keeps recording the legacy
// collection label verbatim — including the synthetic "(all)" and "(à la carte)"
// labels, which are not contract identifiers and must round-trip unchanged for
// existing installs.
func stateCollection(src *Source, collection string) string {
	if src != nil && src.Pakke != nil {
		return src.Pakke.Name
	}
	return collection
}

// payloadOnly reports whether a source ships an agentpakke that declares
// pre-built payloads and no layout — Tier 2 only.
//
// Tier is derived from shape: such a manifest has no Tier 1 content for this
// binary to materialize into a scope, so the install paths route it to
// [installPakkePin] instead of resolving the canonical agents//skills/
// directories in a repo that ships neither.
//
// It is a predicate, not a policy: it says what the source is, and the caller
// decides what to do about it. [guardPakkeScope] holds the one policy left.
func payloadOnly(src *Source) bool {
	return src != nil && src.Pakke != nil && src.Pakke.Layout == nil &&
		src.Pakke.HasTier(agentpakke.TierPayload)
}

// guardPakkeScope refuses a Tier 2 install into any scope but the user's.
//
// A pin lives in the scope's state file, and every launch reads the pin from
// user scope only. A repo-scope Tier 2 install would therefore write a
// zero-file state under .github/ that nothing ever reads back — an install that
// reports success and changes nothing.
func guardPakkeScope(scope *InstallScope, src *Source) error {
	if scope == nil || scope.IsUser() {
		return nil
	}
	return fmt.Errorf(
		"agentpakke %q in %s ships pre-built payloads (Tier 2), which nav-pilot pins per user rather than per repository.\n"+
			"Nothing was installed — a pin recorded in the %s scope is one no launch would ever read.\n\n"+
			"  Install it for your user:  %s",
		src.Pakke.Name, sourceLabelFor(src), scope.Name,
		bold("nav-pilot install --user "+src.Pakke.Name))
}

// validatePakkeSource runs the on-disk conformance checks before any file is
// written (A3: no partial installs). It is a no-op for manifest-less sources,
// which have no contract to violate.
func validatePakkeSource(src *Source) error {
	if src == nil || src.Pakke == nil {
		return nil
	}
	errs := agentpakke.ValidateSource(src.Dir)
	if len(errs) == 0 {
		return nil
	}
	var b strings.Builder
	fmt.Fprintf(&b, "agentpakke %q in %s does not conform to its manifest:", src.Pakke.Name, sourceLabelFor(src))
	for _, e := range errs {
		fmt.Fprintf(&b, "\n  - %v", e)
	}
	fmt.Fprintf(&b, "\n\nNothing was installed. Ask the agentpakke to fix its content, or run %s to re-check",
		bold("nav-pilot validate --source "+sourceLabelFor(src)))
	return errors.New(b.String())
}

// sourceLabelFor names a resolved source in user-facing messages.
func sourceLabelFor(src *Source) string {
	if src == nil || src.Repo == "" {
		return defaultSourceRepo
	}
	return src.Repo
}

// pakkeInstallName is the single installable name a manifest-bearing source
// offers: its agentpakke identity, which supersedes collections/<name>.
func pakkeInstallName(src *Source) string {
	if src == nil || src.Pakke == nil {
		return ""
	}
	return src.Pakke.Name
}

// pakkeInstallTarget names what an install command would be given for a source:
// the agentpakke's name when it ships a manifest, and a placeholder when it
// does not.
//
// A manifest-less checkout has no single installable name — its names are the
// collections it ships — and the refusals that print an install command reach a
// source before anything has established that it ships a manifest at all.
// Reading src.Pakke.Name there is a nil dereference, so this is what they use.
func pakkeInstallTarget(src *Source) string {
	if name := pakkeInstallName(src); name != "" {
		return name
	}
	return "<name>"
}

// ─── B3: cross-source guard ──────────────────────────────────────────────────

// guardScopeSource refuses to install content from a source other than the one
// a scope was installed from (B3). The guard is per scope, so different scopes
// may track different agentpakker (B4).
//
// It only fires for a source that came from the persisted config key: an
// explicit --source is the user saying "yes, this source, into this scope", and
// a scope with no install yet is free to pick any source.
func guardScopeSource(scope *InstallScope, flagSource string) error {
	configured, state, origin, err := crossSourceCheck(scope, flagSource)
	if err != nil || state == nil || state.SourceRepo == "" {
		return err
	}
	if sameSourceRepo(configured, state.SourceRepo) {
		return nil
	}
	// Which of the two holds the conflicting value decides both how to name it
	// and how to clear it: telling someone with an empty config to run
	// `config set source ""` is advice that does nothing.
	held, clear := "your configured source", bold(`nav-pilot config set source ""`)
	if origin == agentpakke.DeclarationPath {
		held = "this repo declares"
		clear = "edit the " + bold("source") + " in " + bold(agentpakke.DeclarationPath)
	}
	return fmt.Errorf(
		"the %s scope was installed from %s, but %s %s.\n"+
			"nav-pilot will not silently mix content from two agentpakker into one install.\n\n"+
			"  Keep this scope on its current source:  %s\n"+
			"  Switch this scope to the new source:    %s\n"+
			"  Or clear the conflicting value:         %s",
		scope.Name, bold(state.SourceRepo), held, bold(configured),
		bold("nav-pilot install --source "+state.SourceRepo+" <name>"),
		bold("nav-pilot install --source "+configured+" <name>"),
		clear)
}

// adoptSyncSource is the sync-side half of B3. Sync already prefers the source
// recorded in the scope's state over the configured one, so a scope that knows
// where it came from can never be cross-synced — [guardScopeSource] covers the
// install side and this returns nothing for it.
//
// The case it answers is an install whose state predates source tracking. That
// scope has no recorded origin to guard against, and refusing to sync it would
// strand every pre-tracking install behind a flag. Instead sync proceeds from
// the effective source and adopts it: the returned value is what the caller
// records in state after a successful sync, so from the next run on the scope
// has an origin and the normal guard applies. An empty return means there is
// nothing to adopt (explicit --source, no configured source, no install, or a
// scope that already records one).
func adoptSyncSource(scope *InstallScope, flagSource string) (string, error) {
	configured, state, _, err := crossSourceCheck(scope, flagSource)
	if err != nil || state == nil || state.SourceRepo != "" {
		return "", err
	}
	return configured, nil
}

// noteAdoptedSource says which source a pre-tracking scope is being synced
// from, and that nav-pilot is about to remember it. Adopting silently is how
// someone ends up with a scope pinned to a source they never chose.
func noteAdoptedSource(scope *InstallScope, sourceRepo string) {
	fmt.Printf("%s The %s scope predates source tracking; syncing from %s and recording it as this scope's source.\n\n",
		dim("ℹ"), scope.Name, bold(sourceRepo))
}

// recordAdoptedSource writes the adopted source into the scope's state, after
// the sync it describes actually succeeded. It never fails the sync: the files
// are already correct, and a scope that stays sourceless just gets asked again
// next time.
func recordAdoptedSource(scope *InstallScope, sourceRepo string) {
	state, err := readScopedState(scope)
	if err != nil || state == nil || state.SourceRepo != "" {
		return
	}
	state.SourceRepo = sourceRepo
	if err := writeScopedState(scope, state); err != nil {
		fmt.Fprintf(os.Stderr, "%s Could not record %s as the %s scope's source: %v\n",
			yellow("⚠"), sourceRepo, scope.Name, err)
	}
}

// crossSourceCheck returns the configured source and the scope's state for the
// B3 guards, or a nil state when there is nothing to guard: an explicit
// --source, no persisted source, or no install in this scope.
// The third return says *where* the conflicting value lives, so a refusal can
// tell the user which file to edit.
func crossSourceCheck(scope *InstallScope, flagSource string) (string, *StateFile, string, error) {
	if flagSource != "" {
		return "", nil, "", nil
	}
	// The repo's committed declaration outranks the machine-wide config key
	// (see declaration.go for the full ladder), so the guard has to judge the
	// scope against the same source an install would actually reach for.
	// Reading configuredSourceRepo() here instead would let a repo whose
	// declaration names one agentpakke install over a scope recorded against
	// another without a word — the exact mixing B3 exists to refuse.
	configured, origin := declaredSourceRepo(scope), agentpakke.DeclarationPath
	if configured == "" {
		var err error
		origin = "config"
		configured, err = configuredSourceRepo()
		if err != nil || configured == "" {
			return "", nil, "", err
		}
	}
	state, err := readScopedState(scope)
	if err != nil {
		// A broken state file is the command's error to report, not the guard's.
		return "", nil, "", nil //nolint:nilerr // deliberate: guard stays silent
	}
	return configured, state, origin, nil
}

// sameSourceRepo compares two source values. Repo ids are case-insensitive on
// GitHub; local paths are compared as the checkout they resolve to.
func sameSourceRepo(a, b string) bool {
	if a == b {
		return true
	}
	if filepath.IsAbs(a) || filepath.IsAbs(b) {
		return resolvedSourcePath(a) == resolvedSourcePath(b)
	}
	return strings.EqualFold(a, b)
}

// resolvedSourcePath reduces a path-form source to the directory it actually
// names, so a symlink and the checkout behind it are one source rather than two
// — otherwise the B3 guard refuses a sync over what is the same content.
//
// A path that cannot be resolved (it no longer exists, or is not a path at all)
// falls back to the cleaned string, which is the comparison this had before and
// the only one still available.
func resolvedSourcePath(p string) string {
	if !filepath.IsAbs(p) {
		return filepath.Clean(p)
	}
	resolved, err := filepath.EvalSymlinks(p)
	if err != nil {
		return filepath.Clean(p)
	}
	return resolved
}

// noteRecordedSourceWins says which source a sync actually reads from when the
// user has configured a different one.
//
// The precedence is deliberate — selection is per scope (B4), and the scope's
// recorded source is the only one that can be synced into it without mixing
// agentpakker — but applying it silently is how someone ends up wondering why
// `nav-pilot sync` never picks up the agentpakke they configured. It stays
// quiet when nothing is configured: the config key is what the user asked for,
// and an absent key asks for nothing.
func noteRecordedSourceWins(stateRepo string) {
	configured, err := configuredSourceRepo()
	if err != nil || configured == "" || sameSourceRepo(configured, stateRepo) {
		return
	}
	fmt.Printf("%s Syncing from %s (recorded for this scope); configured source %s is not used here — reinstall with %s to switch.\n\n",
		dim("ℹ"), bold(stateRepo), bold(configured),
		bold("nav-pilot install --source "+configured+" <name>"))
}

// scopeSourceRepo is the source a scope's files belong to unless a file says
// otherwise. A state that predates source tracking records nothing and came
// from the default source — the same reading as [tracksDefaultSource].
func scopeSourceRepo(state *StateFile) string {
	if state == nil || state.SourceRepo == "" {
		return defaultSourceRepo
	}
	return state.SourceRepo
}

// tracksDefaultSource reports whether a scope's install came from the default
// source, which is the only source nav-pilot's own release feed describes.
//
// An install that predates source tracking records nothing; it is treated as
// coming from the default, which is where it must have come from — so
// manifest-less default-source users see no behaviour change.
func tracksDefaultSource(state *StateFile) bool {
	if state == nil || state.SourceRepo == "" {
		return true
	}
	return sameSourceRepo(state.SourceRepo, defaultSourceRepo)
}

// ─── B2: persisting the selected source ──────────────────────────────────────

// persistInstalledSource stores an explicitly selected source in the user
// config after a successful install, so later invocations do not need --source
// (B2). It is a no-op without an explicit --source and for dry runs, and it
// never fails the install: the content is already on disk.
func persistInstalledSource(flagSource string, dryRun bool) {
	if flagSource == "" || dryRun {
		return
	}
	if err := validateSourceValue(flagSource); err != nil {
		return
	}
	current, err := configuredSourceRepo()
	if err == nil && sameSourceRepo(current, flagSource) {
		return
	}
	if _, err := writeConfigKey("source", flagSource); err != nil {
		fmt.Fprintf(os.Stderr, "%s Could not save source to %s: %v\n", yellow("⚠"), configPath(), err)
		return
	}
	fmt.Printf("\n%s Saved %s as your source in %s.\n", green("✓"), bold(flagSource), configPath())
	fmt.Printf("  %s runs use it without %s; clear it with %s.\n",
		dim("Future"), bold("--source"), bold(`nav-pilot config set source ""`))
}
