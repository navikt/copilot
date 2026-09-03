package agentpakke

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/navikt/copilot/cli/nav-pilot/internal/domain"
)

// This file is the consumer half of the agentpakke contract. [Manifest] is what
// an agentpakke repo says about itself; [Declaration] is what a repo that *uses*
// one says about that use, in a file its developers commit and review.
//
// The two are counterparts by design: same directory ([ManifestDir]), same
// contractVersion gate, same ignore-unknown rule. A repo can hold both — an
// agentpakke that consumes another agentpakke is a normal thing to be.

const (
	// DeclarationFile is the consumer-side file name inside [ManifestDir].
	//
	// It is named a lock file because its load-bearing field is a pinned
	// revision, and because that is the word the ecosystem already uses for
	// "committed, reviewed in the diff, and bumped by a command" — which is
	// exactly this file's life. It is nonetheless hand-editable: `source` and
	// `items` are written by people, `sha` by nav-pilot.
	DeclarationFile = "agentpakke.lock.json"

	// DeclarationPath is the repo-relative path to a repo's declaration.
	DeclarationPath = ManifestDir + "/" + DeclarationFile
)

// ErrNoDeclaration is returned by [LoadDeclaration] when the repo declares
// nothing. It is not a failure: the vast majority of repos have no declaration
// and resolve their source from the config key exactly as before.
var ErrNoDeclaration = errors.New("no agentpakke declaration")

// DeclaredItemTypes are the artifact types an entry in [Declaration.Items] may
// name. It mirrors the CLI's artifact kinds; the list lives here because the
// declaration is parsed before any resolver exists.
var DeclaredItemTypes = []string{"agent", "skill", "instruction", "prompt", "hook"}

// Declaration is a repo's committed statement of which agentpakke it uses and
// at which revision.
//
// The shape is deliberately flat and timestamp-free. Two branches that each
// add an item, or that each bump a different field, merge without a conflict;
// a file carrying an "installedAt" would conflict on every parallel change,
// which is the failure mode `.github/.nav-pilot-state.json` already has.
type Declaration struct {
	// ContractVersion gates the file the same way a manifest's does.
	ContractVersion string `json:"contractVersion"`

	// Source is the agentpakke this repo uses: "owner/name" or an absolute
	// path, the same value space as --source and the config `source` key.
	Source string `json:"source"`

	// SHA is the pinned revision. nav-pilot writes it on install and bumps it
	// on `sync --apply`, so an update to the agentpakke arrives as a one-line
	// diff in a pull request rather than as invisible local state.
	SHA string `json:"sha,omitempty"`

	// MinNavPilotVersion is copied from the pinned agentpakke's manifest at
	// install time: the compatibility statement a reviewer needs, which is
	// "which nav-pilot versions can use this pin". Advisory in this file — the
	// manifest at the pinned revision is what actually enforces it.
	MinNavPilotVersion string `json:"minNavPilotVersion,omitempty"`

	// Items optionally restricts the install to named artifacts, mapping an
	// artifact name to its type. Absent or empty means "everything the pakke
	// ships", which is what nav-pilot writes: auto-listing all twelve agents
	// of a platform pakke would make every upstream addition a merge conflict
	// in every consumer. A team taking four of them writes those four by hand.
	Items map[string]string `json:"items,omitempty"`

	// Unknown carries every key this binary does not understand, so a bump
	// written by an older nav-pilot does not silently delete what a newer one
	// put there (#588). It rides the same mechanism as the state file's.
	Unknown map[string]json.RawMessage `json:"-"`
}

// declarationFields is [Declaration] without its custom marshalling, so the
// methods below can encode the known half without recursing.
type declarationFields Declaration

var declarationKnownKeys = domain.KnownJSONKeys(Declaration{})

func (d *Declaration) UnmarshalJSON(b []byte) error {
	var known declarationFields
	if err := json.Unmarshal(b, &known); err != nil {
		return err
	}
	unknown, err := domain.UnknownJSONKeys(b, declarationKnownKeys)
	if err != nil {
		return err
	}
	*d = Declaration(known)
	d.Unknown = unknown
	return nil
}

func (d Declaration) MarshalJSON() ([]byte, error) {
	b, err := json.Marshal(declarationFields(d))
	if err != nil {
		return nil, err
	}
	return domain.AppendUnknownKeys(b, d.Unknown), nil
}

// DeclarationFilePath returns the declaration path inside a repo root.
func DeclarationFilePath(root string) string {
	return filepath.Join(root, filepath.FromSlash(DeclarationPath))
}

// LoadDeclaration reads and validates a repo's declaration. A repo without one
// yields [ErrNoDeclaration].
//
// Validation is fail-closed for the same reason the manifest's is: a
// declaration nav-pilot cannot read is a pin nobody is honouring, and silently
// falling back to the config key would install content the pull request never
// showed. Source *syntax* is not checked here — the CLI owns that value space —
// only that a source is present.
func LoadDeclaration(root string) (*Declaration, error) {
	path := DeclarationFilePath(root)
	// A symlinked .nav-pilot/ is a file the pull request never showed. The same
	// guard the state file gets, on both ends of this one.
	if err := domain.CheckSymlink(path, root); err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrNoDeclaration
		}
		return nil, fmt.Errorf("reading %s: %w", DeclarationPath, err)
	}
	var d Declaration
	if err := json.Unmarshal(data, &d); err != nil {
		return nil, fmt.Errorf("%s is not valid JSON: %w", DeclarationPath, err)
	}
	if err := d.validate(); err != nil {
		return nil, err
	}
	return &d, nil
}

func (d *Declaration) validate() error {
	if err := checkContractVersionFor(d.ContractVersion,
		"set a supported contractVersion in "+DeclarationPath); err != nil {
		return fmt.Errorf("%s: %w", DeclarationPath, err)
	}
	source := strings.TrimSpace(d.Source)
	if source == "" {
		return fmt.Errorf("%s must name a source", DeclarationPath)
	}
	// A path source is a working tree, not a revision anyone can fetch: there
	// is nothing for a pin to name, and writing one anyway is how "unknown"
	// ends up committed as a sha. Say so instead.
	if filepath.IsAbs(source) && strings.TrimSpace(d.SHA) != "" {
		return fmt.Errorf("%s pins sha %q against the path source %s; a local checkout has no revision to fetch, so drop the sha and let the working tree decide",
			DeclarationPath, d.SHA, source)
	}
	names := make([]string, 0, len(d.Items))
	for name := range d.Items {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if !isDeclaredItemType(d.Items[name]) {
			return fmt.Errorf("%s: item %q has type %q; valid types are %s",
				DeclarationPath, name, d.Items[name], strings.Join(DeclaredItemTypes, ", "))
		}
	}
	return nil
}

func isDeclaredItemType(t string) bool {
	for _, valid := range DeclaredItemTypes {
		if t == valid {
			return true
		}
	}
	return false
}

// WriteDeclaration writes a declaration to a repo root, creating [ManifestDir]
// if needed. Output is deterministic: encoding/json sorts object keys, the
// struct field order is fixed, and nothing dated goes in — so a rewrite that
// changes only the SHA produces exactly a one-line diff.
func WriteDeclaration(root string, d *Declaration) error {
	path := DeclarationFilePath(root)
	if err := domain.CheckSymlink(path, root); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("creating %s: %w", filepath.Dir(DeclarationPath), err)
	}
	data, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding %s: %w", DeclarationPath, err)
	}
	// Temp file plus rename, the same way the state file beside it is written.
	// A truncating write that dies halfway — Ctrl-C, a full disk — leaves a
	// half-written declaration, and LoadDeclaration is deliberately fail-closed:
	// install, add, export, list and sync would all then refuse the repo until
	// someone hand-repaired the file.
	tmp, err := os.CreateTemp(filepath.Dir(path), ".agentpakke-lock-*")
	if err != nil {
		return fmt.Errorf("writing %s: %w", DeclarationPath, err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := tmp.Write(append(data, '\n')); err != nil {
		tmp.Close()
		return fmt.Errorf("writing %s: %w", DeclarationPath, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("writing %s: %w", DeclarationPath, err)
	}
	if err := os.Chmod(tmpPath, 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", DeclarationPath, err)
	}
	return os.Rename(tmpPath, path)
}
