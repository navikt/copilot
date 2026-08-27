package agentpakke

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/navikt/copilot/cli/nav-pilot/internal/domain"
)

// ErrNoManifest reports that a source checkout ships no agentpakke manifest.
// It is not a failure: callers substitute [SynthesizeLegacy] and install the
// source under the legacy collection conventions (see the package doc's
// migration path).
var ErrNoManifest = errors.New("no agentpakke manifest")

// Load reads and validates the agentpakke manifest of a resolved source
// checkout. sourceRoot is the checkout's root directory; the manifest is read
// from <sourceRoot>/.nav-pilot/agentpakke.json.
//
// A missing manifest returns an error matching [ErrNoManifest] via errors.Is.
// Every other failure is fail-closed and actionable: the manifest exists but
// does not conform, so nothing should be installed, synced, or launched from it.
func Load(sourceRoot string) (*Manifest, error) {
	file := filepath.Join(sourceRoot, ManifestDir, ManifestFile)
	data, err := os.ReadFile(file)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("%s: %w", file, ErrNoManifest)
		}
		return nil, fmt.Errorf("reading %s: %w", file, err)
	}
	m, err := Parse(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", file, err)
	}
	return m, nil
}

// Parse validates raw manifest bytes and returns the manifest. Validation runs
// schema-first (the published contract), then the semantic rules the schema
// cannot express.
func Parse(data []byte) (*Manifest, error) {
	return parse(data, cliVersion)
}

// Validate checks raw manifest bytes without returning the manifest — the entry
// point for `nav-pilot validate` and for agentpakke CI that only wants a verdict.
func Validate(data []byte) error {
	_, err := Parse(data)
	return err
}

// parse is Parse with the running version injected, so version gating is
// testable without mutating package state.
func parse(data []byte, runningVersion string) (*Manifest, error) {
	if err := validateSchema(data); err != nil {
		return nil, err
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		// Unreachable in practice: the schema pass already parsed the JSON, so
		// only a type the schema permits but Go cannot decode lands here.
		return nil, fmt.Errorf("decoding manifest: %w", err)
	}
	if err := m.checkSemantics(runningVersion); err != nil {
		return nil, err
	}
	return &m, nil
}

// checkSemantics runs the fail-closed rules that JSON Schema cannot express:
// contract-version support, minimum binary version, Tier 1's dependence on
// layout, and path containment for every repo-relative path in the manifest.
func (m *Manifest) checkSemantics(runningVersion string) error {
	if err := m.checkContractVersion(); err != nil {
		return err
	}
	if err := m.checkMinVersion(runningVersion); err != nil {
		return err
	}
	if err := m.checkTiers(); err != nil {
		return err
	}
	return m.checkPaths()
}

// checkContractVersion rejects a contract major this binary does not implement,
// naming what it does support and how to proceed (A3/A4).
func (m *Manifest) checkContractVersion() error {
	major, _, _ := strings.Cut(m.ContractVersion, ".")
	for _, supported := range SupportedContractMajors {
		if major == supported {
			return nil
		}
	}
	return fmt.Errorf(
		"contractVersion %q is not supported by this nav-pilot; supported contract versions: %s. "+
			"Upgrade nav-pilot (nav-pilot update) or ask the agentpakke to publish a manifest on a supported contract version",
		m.ContractVersion, strings.Join(SupportedContractMajors, ", "))
}

// checkMinVersion enforces minNavPilotVersion against the running binary.
// Development and unset builds are exempt: they carry no comparable version,
// and gating them would block local work on an agentpakke.
func (m *Manifest) checkMinVersion(runningVersion string) error {
	// The raw value is checked, not a trimmed copy: padding is a malformed
	// declaration of a known field, and checkRelPath rejects it for paths too.
	required := m.MinNavPilotVersion
	if strings.TrimSpace(required) == "" {
		return nil
	}
	if !isReleaseVersionFormat(required) {
		return fmt.Errorf(
			"minNavPilotVersion %q is not a nav-pilot release version. "+
				"Use the YYYY.MM.DD-HHMMSS-sha7 format (for example 2026.09.01-120000-a1b2c3d); "+
				"a value nav-pilot cannot compare would silently disable the minimum-version gate instead of enforcing it",
			m.MinNavPilotVersion)
	}
	if !isReleaseVersion(runningVersion) {
		return nil
	}
	if versionOlder(runningVersion, required) {
		return fmt.Errorf(
			"agentpakke %q requires nav-pilot %s or newer, but this binary is %s. "+
				"Run `nav-pilot update` (or reinstall via Homebrew) and try again",
			m.Name, required, runningVersion)
	}
	return nil
}

// checkTiers verifies that every Tier 1 client has content to materialize:
// tier is derived from shape, so a client entry without payloads is a promise
// that the manifest's layout exists (D1).
//
// Only clients this binary knows are checked. A future client key is ignored
// per A3/A4 — it is unavailable here, and rejecting the manifest over content
// this binary would never materialize is exactly the breakage ignore-unknown
// exists to prevent.
func (m *Manifest) checkTiers() error {
	if m.Layout != nil {
		return nil
	}
	var tier1 []string
	for _, id := range m.AvailableClients() {
		if m.Tier(id) == TierLayout {
			tier1 = append(tier1, id)
		}
	}
	if len(tier1) == 0 {
		return nil
	}
	return fmt.Errorf(
		"client(s) %s declare no payloads, which makes them Tier 1, but the manifest has no \"layout\". "+
			"Add a layout with agents and skills paths, or declare payloads to make them Tier 2",
		strings.Join(tier1, ", "))
}

// checkPaths rejects any repo-relative path that is absolute or escapes the
// agentpakke checkout. Applied to every path the manifest can point at, not
// only payloads: all of them are resolved against a cloned repo at install and
// launch time.
func (m *Manifest) checkPaths() error {
	type ref struct{ field, value string }
	var refs []ref

	for _, client := range m.ClientIDs() {
		entry := m.Clients[client]
		contexts := make([]string, 0, len(entry.Payloads))
		for ctx := range entry.Payloads {
			contexts = append(contexts, ctx)
		}
		sort.Strings(contexts)
		for _, ctx := range contexts {
			p := entry.Payloads[ctx]
			base := fmt.Sprintf("clients.%s.payloads.%s", client, ctx)
			refs = append(refs, ref{base + ".path", p.Path})
			if p.Manifest != "" {
				refs = append(refs, ref{base + ".manifest", p.Manifest})
			}
		}
	}
	if m.Layout != nil {
		refs = append(refs,
			ref{"layout.agents", m.Layout.Agents},
			ref{"layout.skills", m.Layout.Skills},
			ref{"layout.instructions", m.Layout.Instructions},
			ref{"layout.prompts", m.Layout.Prompts},
		)
	}
	if m.Policies != nil {
		refs = append(refs, ref{"policies.opencodePermissions", m.Policies.OpenCodePermissions})
	}
	if m.Profiles != nil {
		refs = append(refs, ref{"profiles.dir", m.Profiles.Dir})
	}

	for _, r := range refs {
		if r.value == "" {
			continue
		}
		if err := checkRelPath(r.value); err != nil {
			return fmt.Errorf("%s: %w", r.field, err)
		}
	}
	return nil
}

// checkRelPath validates a manifest path as repo-relative and contained.
// Manifest paths always use forward slashes, so this reasons in slash space and
// additionally rejects backslashes, which would otherwise pass on Linux and
// resolve as separators on Windows.
func checkRelPath(p string) error {
	if strings.TrimSpace(p) != p || p == "" {
		return fmt.Errorf("path %q must not be empty or padded with whitespace", p)
	}
	if strings.ContainsAny(p, `\`) {
		return fmt.Errorf("path %q must use forward slashes and be relative to the agentpakke repo root", p)
	}
	if path.IsAbs(p) || filepath.IsAbs(p) || strings.HasPrefix(p, "~") {
		return fmt.Errorf("path %q is absolute; agentpakke paths must be relative to the repo root", p)
	}
	clean := path.Clean(p)
	if clean == ".." || strings.HasPrefix(clean, "../") {
		return fmt.Errorf("path %q escapes the agentpakke repo root; use a path inside the repo", p)
	}
	return nil
}

// ValidateSource runs the deeper conformance checks that need the agentpakke
// checkout on disk, and returns every violation it finds rather than the first
// — an agentpakke author fixing CI wants the whole list.
//
// It is the check `nav-pilot validate` performs on top of [Load]: referenced
// layout directories exist, payload trees exist, and every Tier 2 payload
// carries a payload manifest (A7).
//
// A nil/empty result means the checkout conforms. A missing manifest is
// reported as a single error matching [ErrNoManifest].
func ValidateSource(sourceRoot string) []error {
	m, err := Load(sourceRoot)
	if err != nil {
		return []error{err}
	}
	return m.validateContent(sourceRoot)
}

// validateContent checks a validated manifest against the files on disk.
func (m *Manifest) validateContent(sourceRoot string) []error {
	var errs []error

	if m.Layout != nil && m.HasTier(TierLayout) {
		for _, d := range []struct{ field, value string }{
			{"layout.agents", m.Layout.Agents},
			{"layout.skills", m.Layout.Skills},
			{"layout.instructions", m.Layout.Instructions},
			{"layout.prompts", m.Layout.Prompts},
		} {
			if d.value == "" {
				continue
			}
			if err := requireDir(sourceRoot, d.field, d.value); err != nil {
				errs = append(errs, err)
				continue
			}
			if d.field == "layout.agents" {
				errs = append(errs, checkAgentFiles(filepath.Join(sourceRoot, filepath.FromSlash(d.value)), d.field)...)
			}
		}
	}

	for _, client := range m.ClientIDs() {
		entry := m.Clients[client]
		contexts := make([]string, 0, len(entry.Payloads))
		for ctx := range entry.Payloads {
			contexts = append(contexts, ctx)
		}
		sort.Strings(contexts)
		for _, ctx := range contexts {
			p := entry.Payloads[ctx]
			field := fmt.Sprintf("clients.%s.payloads.%s", client, ctx)
			if err := requireDir(sourceRoot, field+".path", p.Path); err != nil {
				errs = append(errs, err)
				continue
			}
			manifestRel := p.ManifestPath()
			if err := requireFile(sourceRoot, field, manifestRel); err != nil {
				errs = append(errs, fmt.Errorf(
					"%w; every Tier 2 payload must ship a payload manifest (A7) — nav-pilot refuses to stage an unmanifested payload",
					err))
				continue
			}
			// The payload manifest exists, so its files map can be verified
			// against the tree it describes (G1). Only Tier 2 entries reach
			// here: this loop iterates declared payloads, and a client without
			// payloads is Tier 1 by definition.
			if err := VerifyPayload(
				filepath.Join(sourceRoot, filepath.FromSlash(p.Path)),
				filepath.Join(sourceRoot, filepath.FromSlash(manifestRel)),
			); err != nil {
				errs = append(errs, fmt.Errorf("%s: %w", field, err))
			}
		}
	}

	if m.Policies != nil && m.Policies.OpenCodePermissions != "" {
		if err := requireFile(sourceRoot, "policies.opencodePermissions", m.Policies.OpenCodePermissions); err != nil {
			errs = append(errs, err)
		}
	}
	if m.Profiles != nil && m.Profiles.Dir != "" {
		if err := requireDir(sourceRoot, "profiles.dir", m.Profiles.Dir); err != nil {
			errs = append(errs, err)
		} else if m.Profiles.Default != "" {
			if err := requireFile(sourceRoot, "profiles.default",
				path.Join(m.Profiles.Dir, m.Profiles.Default+".json")); err != nil {
				errs = append(errs, err)
			}
		}
	}

	return errs
}

// requireDir reports a manifest-referenced directory that is missing, is not a
// directory, or is not really inside the repo.
func requireDir(sourceRoot, field, rel string) error {
	abs := filepath.Join(sourceRoot, filepath.FromSlash(rel))
	if err := requireContained(sourceRoot, field, rel); err != nil {
		return err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return fmt.Errorf("%s references %q, which does not exist in the agentpakke repo", field, rel)
	}
	if !info.IsDir() {
		return fmt.Errorf("%s references %q, which is not a directory", field, rel)
	}
	return nil
}

// requireFile reports a manifest-referenced file that is missing, is not a
// regular file, or is not really inside the repo.
func requireFile(sourceRoot, field, rel string) error {
	abs := filepath.Join(sourceRoot, filepath.FromSlash(rel))
	if err := requireContained(sourceRoot, field, rel); err != nil {
		return err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return fmt.Errorf("%s expects the file %q, which does not exist in the agentpakke repo", field, rel)
	}
	if info.IsDir() {
		return fmt.Errorf("%s expects the file %q, but it is a directory", field, rel)
	}
	return nil
}

// requireContained is checkRelPath's on-disk counterpart. checkRelPath rules out
// paths that are textually outside the repo; this rules out paths that are
// textually inside it and physically elsewhere — a symlinked layout directory,
// or a link somewhere along the way to one. nav-pilot reads and copies from
// these paths, so what they resolve to is what matters, and os.Stat alone would
// follow the link and call it conforming.
func requireContained(sourceRoot, field, rel string) error {
	abs := filepath.Join(sourceRoot, filepath.FromSlash(rel))
	if info, err := os.Lstat(abs); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf(
			"%s references %q, which is a symlink; agentpakke content must be real files and directories inside the repo",
			field, rel)
	}
	if !domain.PathWithinRoot(sourceRoot, abs) {
		return fmt.Errorf(
			"%s references %q, which resolves outside the agentpakke repo through a symlinked parent directory",
			field, rel)
	}
	return nil
}

// checkAgentFiles verifies that the Tier 1 agents directory holds agent files
// and that each opens with YAML frontmatter, which the sync layer needs to read
// name/description/model from.
//
// The check stays shallow on purpose: the frontmatter parser lives in
// internal/source, which will itself consume this package's Manifest for the
// Tier 1 layout resolver — importing it here would close that cycle. Deep
// frontmatter conformance therefore belongs in the sync layer, where the parser
// already runs.
func checkAgentFiles(agentsDir, field string) []error {
	entries, err := os.ReadDir(agentsDir)
	if err != nil {
		return []error{fmt.Errorf("%s: reading %q: %v", field, agentsDir, err)}
	}
	var errs []error
	var found int
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), agentFileSuffix) {
			continue
		}
		found++
		// A symlinked agent file reads whatever it points at — including files
		// outside the checkout — so it is refused rather than parsed.
		if e.Type()&os.ModeSymlink != 0 {
			errs = append(errs, fmt.Errorf(
				"%s: %q is a symlink; agent files must be real files inside the agentpakke repo", field, e.Name()))
			continue
		}
		data, err := os.ReadFile(filepath.Join(agentsDir, e.Name()))
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: reading %q: %v", field, e.Name(), err))
			continue
		}
		if !hasFrontmatter(data) {
			errs = append(errs, fmt.Errorf(
				"%s: %q has no YAML frontmatter; agent files must open with a --- delimited block declaring at least name and description",
				field, e.Name()))
		}
	}
	if found == 0 {
		errs = append(errs, fmt.Errorf("%s: %q contains no *%s files", field, agentsDir, agentFileSuffix))
	}
	return errs
}

// agentFileSuffix is the agent file extension in nav-pilot's content layout.
const agentFileSuffix = ".agent.md"

// hasFrontmatter reports whether a markdown file opens with a --- delimited
// block. It mirrors the opening condition of internal/source.SplitFrontmatter
// without importing it (see checkAgentFiles).
func hasFrontmatter(data []byte) bool {
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	text = strings.TrimLeft(text, " \t\n")
	if !strings.HasPrefix(text, "---") {
		return false
	}
	rest := text[len("---"):]
	nl := strings.IndexByte(rest, '\n')
	if nl < 0 {
		return false
	}
	for _, line := range strings.Split(rest[nl+1:], "\n") {
		if strings.TrimRight(line, " \t") == "---" {
			return true
		}
	}
	return false
}
