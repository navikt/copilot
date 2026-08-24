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
		return fmt.Errorf("%s ships an agentpakke manifest (%s) that this nav-pilot cannot use:\n  %v\n\nNothing was installed. Fix the manifest in the source repo, or run %s to list every finding",
			sourceLabelFor(src), agentpakke.ManifestPath, err,
			bold("nav-pilot validate --source "+sourceLabelFor(src)))
	}
	src.Pakke = m
	return nil
}

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

// ─── B3: cross-source guard ──────────────────────────────────────────────────

// guardScopeSource refuses to install content from a source other than the one
// a scope was installed from (B3). The guard is per scope, so different scopes
// may track different agentpakker (B4).
//
// It only fires for a source that came from the persisted config key: an
// explicit --source is the user saying "yes, this source, into this scope", and
// a scope with no install yet is free to pick any source.
func guardScopeSource(scope *InstallScope, flagSource string) error {
	configured, state, err := crossSourceCheck(scope, flagSource)
	if err != nil || state == nil || state.SourceRepo == "" {
		return err
	}
	if sameSourceRepo(configured, state.SourceRepo) {
		return nil
	}
	return fmt.Errorf(
		"the %s scope was installed from %s, but your configured source is %s.\n"+
			"nav-pilot will not silently mix content from two agentpakker into one install.\n\n"+
			"  Keep this scope on its current source:  %s\n"+
			"  Switch this scope to the new source:    %s\n"+
			"  Or clear the persisted source:          %s",
		scope.Name, bold(state.SourceRepo), bold(configured),
		bold("nav-pilot install --source "+state.SourceRepo+" <name>"),
		bold("nav-pilot install --source "+configured+" <name>"),
		bold(`nav-pilot config set source ""`))
}

// guardScopeSyncSource is the sync-side half of B3. Sync already prefers the
// source recorded in the scope's state over the configured one, so a scope that
// knows where it came from can never be cross-synced. What it cannot decide is
// an install whose state predates source tracking: syncing that against a
// configured agentpakke would overwrite files from an unknown origin, so it
// refuses and asks for an explicit answer.
func guardScopeSyncSource(scope *InstallScope, flagSource string) error {
	configured, state, err := crossSourceCheck(scope, flagSource)
	if err != nil || state == nil || state.SourceRepo != "" {
		return err
	}
	return fmt.Errorf(
		"the %s scope records no source (it was installed before nav-pilot tracked one), "+
			"and your configured source is %s.\n"+
			"Syncing would pull content from an agentpakke this install may not come from.\n\n"+
			"  Sync from a named source:  %s\n"+
			"  Or reinstall the scope:    %s",
		scope.Name, bold(configured),
		bold("nav-pilot sync --source "+defaultSourceRepo),
		bold("nav-pilot install --source "+configured+" <name>"))
}

// crossSourceCheck returns the configured source and the scope's state for the
// B3 guards, or a nil state when there is nothing to guard: an explicit
// --source, no persisted source, or no install in this scope.
func crossSourceCheck(scope *InstallScope, flagSource string) (string, *StateFile, error) {
	if flagSource != "" {
		return "", nil, nil
	}
	configured, err := configuredSourceRepo()
	if err != nil || configured == "" {
		return "", nil, err
	}
	state, err := readScopedState(scope)
	if err != nil {
		// A broken state file is the command's error to report, not the guard's.
		return "", nil, nil //nolint:nilerr // deliberate: guard stays silent
	}
	return configured, state, nil
}

// sameSourceRepo compares two source values. Repo ids are case-insensitive on
// GitHub; local paths are compared as cleaned paths.
func sameSourceRepo(a, b string) bool {
	if a == b {
		return true
	}
	if filepath.IsAbs(a) || filepath.IsAbs(b) {
		return filepath.Clean(a) == filepath.Clean(b)
	}
	return strings.EqualFold(a, b)
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
