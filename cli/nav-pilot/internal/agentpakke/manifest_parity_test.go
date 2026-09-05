package agentpakke

import (
	"path/filepath"
	"reflect"
	"testing"
)

// TestCommittedManifestMatchesLegacyAdapter holds the repo's committed
// .nav-pilot/agentpakke.json byte-for-value equal to [SynthesizeLegacy]("").
//
// The two describe the same thing from opposite ends of the deprecation
// window: the manifest is what a current binary installs from, the adapter is
// what an older binary (or a pinned pre-manifest ref) synthesizes for the same
// source. Until the adapter retires with the collection mechanism, an edit to
// either without the other splits what users get by binary version — exactly
// the drift the adapter's doc comment warns about. Change both, or neither.
func TestCommittedManifestMatchesLegacyAdapter(t *testing.T) {
	repoRoot := filepath.Join("..", "..", "..", "..")
	committed, err := Load(repoRoot)
	if err != nil {
		t.Fatalf("loading the repo's own %s: %v", ManifestPath, err)
	}
	if want := SynthesizeLegacy(""); !reflect.DeepEqual(committed, want) {
		t.Errorf("committed %s = %+v,\nwant SynthesizeLegacy(\"\") = %+v\n"+
			"(update legacy.go and the committed manifest together)", ManifestPath, committed, want)
	}
}
