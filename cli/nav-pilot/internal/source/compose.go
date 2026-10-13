package source

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/navikt/copilot/cli/nav-pilot/internal/agentpakke"
	"github.com/navikt/copilot/cli/nav-pilot/internal/domain"
)

// ComposedBases contains every resolved base, nearest first. The caller owns
// these sources and must defer Cleanup until it has finished reading the resolver.
type ComposedBases []*Source

// Cleanup removes all temporary base checkouts. Nil chains and local sources
// require no cleanup.
func (b ComposedBases) Cleanup() {
	for _, src := range b {
		src.Cleanup()
	}
}

// Nearest returns the source reused directly, or nil when there is no base.
func (b ComposedBases) Nearest() *Source {
	if len(b) == 0 {
		return nil
	}
	return b[0]
}

// ReuseManifestError reports an unusable manifest in a resolved base. It wraps
// the load error so callers can distinguish a manifest failure from an offline
// or missing source, and the CLI can retain attachPakke's formatting and sentinel.
type ReuseManifestError struct {
	Source string
	Err    error
}

func (e *ReuseManifestError) Error() string {
	return fmt.Sprintf("%s ships an agentpakke manifest (%s) that this nav-pilot cannot use:\n  %v\n\nNothing was installed. Fix the manifest in the source repo, or run %s to list every finding",
		e.Source, agentpakke.ManifestPath, e.Err, "nav-pilot validate --source "+e.Source)
}

func (e *ReuseManifestError) Unwrap() error { return e.Err }

// ComposeResolver places the source's reused pakke behind resolver: the nearest
// source wins each collision. Bases may themselves reuse other sources. Cycles,
// unpinned repository bases and payload-only bases are refused before returning
// content. Failed composition cleans every checkout it resolved.
//
// The caller loads src.Pakke and builds resolver for its layout. Sources without
// a manifest and payload-only sources return resolver unchanged: only Tier 1
// sources compose at read time. Each base manifest is loaded here.
func ComposeResolver(resolver *SourceResolver, src *Source) (*SourceResolver, ComposedBases, error) {
	if src.Pakke == nil || src.Pakke.PayloadOnly() {
		return resolver, nil, nil
	}
	return composeResolverSeen(resolver, src, []string{compositionSourceLabel(src)})
}

// composeResolverSeen builds the chain depth-first, carrying repository identities
// rather than comparing their spelling. A self-declaration is a consumer repo's
// declaration, not composition; a reference to an earlier ancestor is a cycle.
func composeResolverSeen(resolver *SourceResolver, src *Source, seen []string) (*SourceResolver, ComposedBases, error) {
	decl, err := agentpakke.LoadDeclaration(src.Dir)
	if err != nil {
		if errors.Is(err, agentpakke.ErrNoDeclaration) {
			return resolver, nil, nil
		}
		return nil, nil, fmt.Errorf("reading the reuse declaration of %s: %w", compositionSourceLabel(src), err)
	}
	if decl.Source == "" || sameCompositionSource(decl.Source, compositionSourceLabel(src)) {
		return resolver, nil, nil
	}
	for _, s := range seen {
		if sameCompositionSource(s, decl.Source) {
			return nil, nil, fmt.Errorf("agentpakke %s reuses %q, which already reuses it back.\nA reuse cycle has no order to resolve in, so nothing was installed.\nBreak the cycle in %s in one of the two repos",
				compositionSourceLabel(src), decl.Source, agentpakke.DeclarationPath)
		}
	}
	seen = append(seen, decl.Source)

	// A local path is a working tree with no revision to pin. Repository reuse
	// must pin a revision rather than resolve whatever main holds at read time.
	if !filepath.IsAbs(decl.Source) && decl.SHA == "" {
		return nil, nil, fmt.Errorf("agentpakke %s reuses %q without a revision.\nReuse resolves at install and sync, so an unpinned base would compose whatever that repo's main branch holds at the time.\nNothing was installed. Add the 40-character sha of the revision to reuse to %s and commit it",
			compositionSourceLabel(src), decl.Source, agentpakke.DeclarationPath)
	}
	baseSrc, err := ResolveSource(decl.SHA, decl.Source, "")
	if err != nil {
		return nil, nil, fmt.Errorf("%s reuses agentpakke %q, which could not be resolved at %s: %w\n\nNothing was installed",
			compositionSourceLabel(src), decl.Source, domain.ShortSHA(decl.SHA), err)
	}
	m, err := agentpakke.Load(baseSrc.Dir)
	if err != nil && !errors.Is(err, agentpakke.ErrNoManifest) {
		baseSrc.Cleanup()
		return nil, nil, &ReuseManifestError{Source: compositionSourceLabel(baseSrc), Err: err}
	}
	baseSrc.Pakke = m
	// A digest-bound payload tree cannot contribute layout files to this chain.
	if baseSrc.Pakke.PayloadOnly() {
		baseSrc.Cleanup()
		return nil, nil, fmt.Errorf(
			"agentpakke %s reuses %q, which ships pre-built payloads (Tier 2).\n"+
				"A payload tree is staged and digest-verified as a whole, so there are no files to inherit from it.\n\n"+
				"Nothing was installed. Remove the reuse from %s, or ask %s for a layout-shaped pakke",
			compositionSourceLabel(src), decl.Source, agentpakke.DeclarationPath, decl.Source)
	}
	baseResolver := NewSourceResolver(baseSrc.Dir)
	if baseSrc.Pakke != nil {
		baseResolver = NewSourceResolverForLayout(baseSrc.Dir, baseSrc.Pakke.Layout)
	}
	// Preserve nested reuse, including declarations carried by legacy bases.
	baseResolver, deeper, err := composeResolverSeen(baseResolver, baseSrc, seen)
	if err != nil {
		// The deeper frame cleaned its sources; this one owns the current base.
		baseSrc.Cleanup()
		return nil, nil, err
	}
	return resolver.WithBase(baseResolver), append(ComposedBases{baseSrc}, deeper...), nil
}

func compositionSourceLabel(src *Source) string {
	if src == nil || src.Repo == "" {
		return DefaultRepo
	}
	return src.Repo
}

// Mirror the CLI's sameSourceRepo identity rules: GitHub names ignore case;
// absolute paths are cleaned and resolved through symlinks when possible.
func sameCompositionSource(a, b string) bool {
	if a == b {
		return true
	}
	if filepath.IsAbs(a) || filepath.IsAbs(b) {
		return compositionSourcePath(a) == compositionSourcePath(b)
	}
	return strings.EqualFold(a, b)
}

func compositionSourcePath(p string) string {
	if filepath.IsAbs(p) {
		if resolved, err := filepath.EvalSymlinks(p); err == nil {
			return resolved
		}
	}
	return filepath.Clean(p)
}
