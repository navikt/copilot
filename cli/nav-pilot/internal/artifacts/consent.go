package artifacts

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"syscall"
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
// kernel deny, and the launch side refuses to apply a waiver on a cplt that
// does not have it (see provider.cpltProtectsNavPilotState).
//
// # Fail closed, in both directions
//
// A file that cannot be read or parsed is not "no records". Treating it as one
// made a removal report success having removed nothing, and would let a write
// clobber records it never saw (#861 review). So every read returns its error,
// every caller propagates it, and the one place that turns an error into a
// verdict — the launch lookup — turns it into "not approved".

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

	// Block is the canonical JSON of the proposal this answer was about, kept
	// so the next revision's question can show which field changed rather than
	// only which hosts did (#861 review).
	Block string `json:"block,omitempty"`

	// CpltStamp is the cplt release in force when the answer was recorded, or
	// "" when it could not be read.
	//
	// The launch gate checks the cplt running now, which says nothing about the
	// one that was running when the record was written (#861 review). An answer
	// recorded under a cplt that could not protect ~/.nav-pilot/ may have been
	// written by the agent that benefits from it, and upgrading cplt afterwards
	// does not make it trustworthy. So the stamp travels with the record, and
	// the launch refuses a record made below the protecting release.
	CpltStamp string `json:"cplt_stamp,omitempty"`

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

// errNoConsentPath reports that nav-pilot has no state directory to keep a
// record in, which is not the same as having no record.
var errNoConsentPath = errors.New("nav-pilot has no state directory to record consent in")

// readConsentLocked returns every record. A missing file is no records; an
// unreadable or malformed one is an error, because a caller that cannot see
// what is there must not decide anything about it.
func readConsentLocked() ([]ProposalConsent, error) {
	path := ProposalConsentPath()
	if path == "" {
		return nil, errNoConsentPath
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	var file consentFile
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf(
			"%s is not readable as a consent record: %w.\n  Delete it to be asked the questions again; nothing else reads it",
			path, err)
	}
	return file.Records, nil
}

// writeConsentLocked replaces the file by rename, so a reader never sees a torn
// one. Only ever called with the directory lock held, which is what stops two
// answers written at once from losing one of the two (see withConsent).
func writeConsentLocked(records []ProposalConsent) error {
	path := ProposalConsentPath()
	if path == "" {
		return errNoConsentPath
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

// withConsent runs fn over the records under an exclusive lock and writes back
// whatever it returns. Returning nil records writes nothing.
//
// Reading and writing outside a lock is what let two installs at once lose one
// of the two answers: atomic replacement keeps the file whole and still lets
// the second rename drop what the first wrote (#861 review).
//
// The lock is an flock on nav-pilot's state directory, the mechanism #835
// already uses to order agentpakke hold markers against pruning
// (cli/pakke_inuse.go, lockSource): locking the directory itself means there is
// no lock file to create, exclude from listings or clean up.
//
// Blocking, and fatal if it cannot be taken. That is the opposite of #835's
// choice, deliberately: publishing a hold marker without the lock is no worse
// than what that guard had before it existed, while writing an answer without
// the lock is exactly the lost update this exists to prevent.
//
// ponytail: flock, advisory and local, like #835's. A home directory on NFS
// gets no guarantee from it, and the upgrade path if that stops being true is
// the same one — O_EXCL plus a liveness probe.
func withConsent(fn func([]ProposalConsent) ([]ProposalConsent, error)) error {
	path := ProposalConsentPath()
	if path == "" {
		return errNoConsentPath
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	lock, err := os.Open(dir)
	if err != nil {
		return fmt.Errorf("opening %s to lock it: %w", dir, err)
	}
	// Closing is what releases the lock whatever it reports, so there is
	// nothing here a caller could act on.
	defer lock.Close() //nolint:errcheck
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX); err != nil {
		return fmt.Errorf("locking %s: %w", dir, err)
	}

	records, err := readConsentLocked()
	if err != nil {
		return err
	}
	next, err := fn(records)
	if err != nil || next == nil {
		return err
	}
	return writeConsentLocked(next)
}

// ReadProposalConsent returns this scope's answer about this pakke, or nil when
// there is none. An unreadable record file is an error, never "no answer".
func ReadProposalConsent(scope *domain.InstallScope, pakke string) (*ProposalConsent, error) {
	if scope == nil || pakke == "" {
		return nil, nil
	}
	records, err := readConsentLocked()
	if err != nil {
		return nil, err
	}
	for _, rec := range records {
		if rec.Scope == scope.Name && rec.Root == scope.RootDir && rec.Pakke == pakke {
			return &rec, nil
		}
	}
	return nil, nil
}

// WriteProposalConsent records an answer, replacing any earlier one for the
// same scope and pakke. There is one answer per scope and pakke at a time: a
// superseded hash is not history worth keeping, it is an approval that must
// not be reachable.
func WriteProposalConsent(rec ProposalConsent) error {
	return withConsent(func(records []ProposalConsent) ([]ProposalConsent, error) {
		out := records[:0:0]
		for _, existing := range records {
			if existing.Scope == rec.Scope && existing.Root == rec.Root && existing.Pakke == rec.Pakke {
				continue
			}
			out = append(out, existing)
		}
		return append(out, rec), nil
	})
}

// RemoveProposalConsentsIn deletes every record held by a scope and reports how
// many it removed.
//
// By scope rather than by pakke name, because an uninstall cannot always name
// the pakke: an install records the answer before it writes the scope's state
// file, so a failed state write leaves a record the next `uninstall` sees no
// state for and would never clean up (#861 review). Uninstalling a scope means
// nothing installed there keeps a waiver, whatever the state file survived to
// say.
func RemoveProposalConsentsIn(scope *domain.InstallScope) (int, error) {
	if scope == nil {
		return 0, nil
	}
	var removed int
	err := withConsent(func(records []ProposalConsent) ([]ProposalConsent, error) {
		out := records[:0:0]
		for _, existing := range records {
			if existing.Scope == scope.Name && existing.Root == scope.RootDir {
				removed++
				continue
			}
			out = append(out, existing)
		}
		if removed == 0 {
			return nil, nil // nothing to write
		}
		return out, nil
	})
	if err != nil {
		return 0, err
	}
	return removed, nil
}

// ApprovedProposal returns the record a launch may act on for this pakke: an
// approval whose hash is exactly this revision's, held by a scope the launch
// reads from.
//
// "A scope this launch reads from" is the user scope always, and the current
// repository's scope when the working directory is in one — which is what
// limits a repo-scope approval to launches from that repository.
//
// nil for a proposal with no record, a record that declined, and a record whose
// hash belongs to an earlier revision of the block (invariants 1 and 2). An
// unreadable record file is an error and the caller applies nothing: what
// cannot be read is not an approval. Whether the record is old enough to be
// trusted is the launch gate's question, not this lookup's — see
// [ProposalConsent.CpltStamp].
func ApprovedProposal(pakke, hash string) (*ProposalConsent, error) {
	if pakke == "" || hash == "" {
		return nil, nil
	}
	records, err := readConsentLocked()
	if err != nil {
		return nil, err
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
	for _, rec := range records {
		if rec.Pakke != pakke || rec.Hash != hash || !rec.Approved || !roots[rec.Root] {
			continue
		}
		return &rec, nil
	}
	return nil, nil
}
