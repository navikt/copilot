package artifacts

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/navikt/copilot/cli/nav-pilot/internal/domain"
	"github.com/navikt/copilot/cli/nav-pilot/internal/source"
)

// The consent record for an agentpakke's sandbox proposal (#858, step 2).
//
// One record per scope and pakke, holding the content hash that was decided on.
// A revision that changes the block changes the hash, which makes the record
// stop matching — so the previous answer is void and the question comes back
// (invariant 2). Keeping the hash rather than a list of approved blocks is what
// makes that automatic: there is no second place for an old approval to survive.
//
// It lives under nav-pilot's own state directory and nowhere near cplt's
// (invariant 4): nav-pilot writes nothing into ~/.config/cplt, its local or
// trust subdirectories, or any .cplt.toml, on a pakke's behalf. Invariant 3 —
// that this file cannot be written from inside a cplt session — is a cplt
// kernel deny, not a check here; until cplt ships it, the launch side refuses
// to apply any waiver at all (see provider.withCpltAllowPrivateDomains).

// ProposalConsent is one scope's answer about one pakke's proposal.
type ProposalConsent struct {
	// Scope and Root are the scope that answered: "user" or "repo", and its
	// root directory. Root separates two repositories that install the same
	// pakke, which the scope name alone cannot.
	Scope string `json:"scope"`
	Root  string `json:"root"`

	Pakke string `json:"pakke"`

	// Hash is [agentpakke.CpltProposal.Hash] of the block that was decided on.
	Hash string `json:"hash"`

	// Approved is the answer. A decline is recorded too, so the same hash is
	// not asked about again — declining a pakke is an answer, not a deferral.
	Approved bool `json:"approved"`

	// Hosts is what the approval covers, recorded so the launch flags are
	// derived from the record rather than recomputed from a manifest that may
	// since have moved.
	Hosts []string `json:"hosts,omitempty"`

	At time.Time `json:"at"`
}

type consentFile struct {
	Records []ProposalConsent `json:"records"`
}

// ProposalConsentPath is <nav-pilot state dir>/pakke-consent.json. Its own
// file: deleting it forgets every answer and nothing else.
func ProposalConsentPath() string {
	cache := CacheFilePath()
	if cache == "" {
		return ""
	}
	return filepath.Join(filepath.Dir(cache), "pakke-consent.json")
}

// readConsent returns every record, empty when the file is missing or corrupt.
// Fail-soft on read is the safe direction here: an unreadable file means no
// approval matches, so nothing is applied.
func readConsent() []ProposalConsent {
	path := ProposalConsentPath()
	if path == "" {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var file consentFile
	if json.Unmarshal(data, &file) != nil {
		return nil
	}
	return file.Records
}

// writeConsent replaces the file by rename, so two writers leave one whole file.
func writeConsent(records []ProposalConsent) error {
	path := ProposalConsentPath()
	if path == "" {
		return nil
	}
	sort.Slice(records, func(i, j int) bool {
		if records[i].Root != records[j].Root {
			return records[i].Root < records[j].Root
		}
		return records[i].Pakke < records[j].Pakke
	})
	data, err := json.MarshalIndent(consentFile{Records: records}, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".pakke-consent-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) // no-op once the rename succeeds
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// ReadProposalConsent returns this scope's answer about this pakke, or nil.
func ReadProposalConsent(scope *domain.InstallScope, pakke string) *ProposalConsent {
	if scope == nil || pakke == "" {
		return nil
	}
	for _, rec := range readConsent() {
		if rec.Scope == scope.Name && rec.Root == scope.RootDir && rec.Pakke == pakke {
			return &rec
		}
	}
	return nil
}

// WriteProposalConsent records an answer, replacing any earlier one for the
// same scope and pakke. There is one answer per scope and pakke at a time: a
// superseded hash is not history worth keeping, it is an approval that must
// not be reachable.
func WriteProposalConsent(rec ProposalConsent) error {
	records := readConsent()
	out := records[:0:0]
	for _, existing := range records {
		if existing.Scope == rec.Scope && existing.Root == rec.Root && existing.Pakke == rec.Pakke {
			continue
		}
		out = append(out, existing)
	}
	return writeConsent(append(out, rec))
}

// RemoveProposalConsent deletes this scope's record for a pakke. Called by
// uninstall: no waiver from a pakke outlives it (invariant 6).
func RemoveProposalConsent(scope *domain.InstallScope, pakke string) error {
	if scope == nil || pakke == "" {
		return nil
	}
	records := readConsent()
	out := records[:0:0]
	for _, existing := range records {
		if existing.Scope == scope.Name && existing.Root == scope.RootDir && existing.Pakke == pakke {
			continue
		}
		out = append(out, existing)
	}
	if len(out) == len(records) {
		return nil
	}
	return writeConsent(out)
}

// ApprovedPrivateDomains returns the hosts a launch may name to cplt for the
// pakke it is about to run: the hosts of an approved record whose hash is
// exactly this revision's, held by a scope this launch reads from.
//
// "A scope this launch reads from" is the user scope always, and the current
// repository's scope when the working directory is in one — which is what
// limits a repo-scope approval to launches from that repository.
//
// Everything else returns nothing: a proposal with no record, a record that
// declined, and a record whose hash belongs to an earlier revision of the
// block (invariants 1 and 2).
func ApprovedPrivateDomains(pakke, hash string) []string {
	if pakke == "" || hash == "" {
		return nil
	}
	roots := map[string]bool{}
	if user, err := domain.ScopeUser(); err == nil {
		roots[user.RootDir] = true
	}
	if wd, err := os.Getwd(); err == nil {
		if root := source.FindGitRoot(wd); root != "" {
			roots[domain.ScopeRepo(root).RootDir] = true
		}
	}
	for _, rec := range readConsent() {
		if rec.Pakke != pakke || rec.Hash != hash || !rec.Approved || !roots[rec.Root] {
			continue
		}
		return append([]string(nil), rec.Hosts...)
	}
	return nil
}
