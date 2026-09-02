package agentpakke

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
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
var DeclaredItemTypes = []string{"agent", "skill", "instruction", "prompt"}

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
	if err := checkContractVersion(d.ContractVersion); err != nil {
		return fmt.Errorf("%s: %w", DeclarationPath, err)
	}
	if strings.TrimSpace(d.Source) == "" {
		return fmt.Errorf("%s must name a source", DeclarationPath)
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
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("creating %s: %w", filepath.Dir(DeclarationPath), err)
	}
	data, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding %s: %w", DeclarationPath, err)
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}
