package local

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
)

// Benchmarking a model from a publisher outside [allowedPublishers] used to
// need a patched nav-pilot. These two variables let a bench run do it with the
// shipped binary, and only a bench run:
//
//	NAV_PILOT_BENCH_MANIFEST=/path/models.json   read this file as the manifest:
//	                                             no network, no cache, no
//	                                             embedded fallback
//	NAV_PILOT_BENCH_ALLOW_ORGS=Accio-Lab[,Org2]  also accept these publishers,
//	                                             in that file only
//
// The org list is honoured only for the file named by the first variable. The
// served manifest, the cache and the embedded copy are always held to
// [allowedPublishers], so neither variable widens what a developer's machine
// accepts from the network, and a leftover NAV_PILOT_BENCH_ALLOW_ORGS in a
// shell profile does nothing but say so. Every process that accepts an unvetted
// publisher says that on stderr too.
//
// It is not a trust boundary of its own: whoever sets a process's environment
// can already point it at other weights. It exists so the boundary above stays
// a code change for everyone else.
const (
	BenchManifestEnv  = "NAV_PILOT_BENCH_MANIFEST"
	BenchAllowOrgsEnv = "NAV_PILOT_BENCH_ALLOW_ORGS"
)

// SourceBench is the manifest named by NAV_PILOT_BENCH_MANIFEST.
const SourceBench Source = "bench"

// benchWarnings is where the override announces itself. A variable for tests.
var benchWarnings io.Writer = os.Stderr

var (
	benchWarnMu   sync.Mutex
	benchWarnSeen = map[string]bool{}
)

// benchWarn prints msg on stderr once per process.
func benchWarn(msg string) {
	benchWarnMu.Lock()
	defer benchWarnMu.Unlock()
	if benchWarnSeen[msg] {
		return
	}
	benchWarnSeen[msg] = true
	fmt.Fprintln(benchWarnings, msg)
}

// benchOrgs parses NAV_PILOT_BENCH_ALLOW_ORGS.
func benchOrgs() []string {
	var orgs []string
	for _, o := range strings.Split(os.Getenv(BenchAllowOrgsEnv), ",") {
		if o = strings.TrimSpace(o); o != "" {
			orgs = append(orgs, o)
		}
	}
	return orgs
}

// benchManifest is [Resolve] and [Cached] when NAV_PILOT_BENCH_MANIFEST is
// set: that file or nothing. ok is false when the variable is unset. A file
// that is missing or refused yields a nil manifest and the reason, never a
// fallback: a bench that silently ran the default model would measure the
// wrong thing.
func benchManifest() (m *Manifest, ok bool, err error) {
	path := os.Getenv(BenchManifestEnv)
	if path == "" {
		if os.Getenv(BenchAllowOrgsEnv) != "" {
			benchWarn(fmt.Sprintf("bench override: %s is ignored without %s", BenchAllowOrgsEnv, BenchManifestEnv))
		}
		return nil, false, nil
	}
	if abs, aerr := filepath.Abs(path); aerr == nil {
		path = abs
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, true, fmt.Errorf("%s: %w", BenchManifestEnv, err)
	}
	defer f.Close()
	// The same cap as the network path (fetchManifest).
	data, err := io.ReadAll(io.LimitReader(f, 1<<20))
	if err != nil {
		return nil, true, fmt.Errorf("%s=%s: %w", BenchManifestEnv, path, err)
	}
	m, err = parse(data, benchOrgs())
	if err != nil {
		return nil, true, fmt.Errorf("%s=%s: %w", BenchManifestEnv, path, err)
	}
	return m, true, nil
}

// publisherAllowed is the publisher rule for this manifest: the allow-list,
// plus the bench orgs when this is the bench manifest.
func (m *Manifest) publisherAllowed(publisher string) bool {
	if slices.Contains(allowedPublishers, publisher) {
		return true
	}
	if slices.Contains(m.benchOrgs, publisher) {
		benchWarn("bench override: allowing unvetted publisher " + publisher)
		return true
	}
	return false
}
