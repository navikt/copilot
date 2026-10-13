package artifacts

import (
	"github.com/navikt/copilot/cli/nav-pilot/internal/source"
)

// composedContent gives every layout materialization the same pinned reuse
// chain as installation. The caller owns the base checkouts until writing ends.
// The top-level manifest remains the authority for the primary-agent roster.
func composedContent(sourceDir, sourceRepo string) (*source.SourceResolver, source.ComposedBases, error) {
	m, err := pakkeManifest(sourceDir)
	if err != nil {
		return nil, nil, err
	}
	src := &source.Source{Dir: sourceDir, Repo: sourceRepo, Pakke: m}
	if src.Repo == "" {
		src.Repo = sourceDir
	}
	var resolver *source.SourceResolver
	if m == nil {
		resolver = source.NewSourceResolver(sourceDir)
	} else {
		resolver = source.NewSourceResolverForLayout(sourceDir, m.Layout)
	}
	return source.ComposeResolver(resolver, src)
}
