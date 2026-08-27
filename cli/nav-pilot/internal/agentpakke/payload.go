package agentpakke

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

// Payload verification (G1).
//
// A Tier 2 payload is a pre-built tree that nav-pilot never authors and never
// edits — it only stages it into a client's config location. The payload
// manifest's files map is therefore the whole trust boundary: it declares
// exactly which files the tree may contain, their sha256, and their mode.
// Verification is fail-closed and exact in both directions — a file the
// manifest does not list is as fatal as a file the manifest lists and the tree
// does not have — so a tampered or partially-updated checkout cannot be staged.
//
// The semantics mirror grillmester's _verify_manifested_payload
// (scripts/grillmester_local.py at 3573b93cc8b7568516117263562d073cae9ee7fc),
// with two deliberate deviations documented at the checks that make them:
// no target-name pin (see [PayloadManifest.Target]) and an exec-bit rather
// than exact-mode comparison on the source tree (see [FileRecord.Mode]).

const (
	// PayloadSchemaVersion is the payload-manifest schemaVersion this binary
	// verifies. A payload on any other version is refused rather than guessed
	// at: the files map is a security contract, not a best effort.
	PayloadSchemaVersion = 1

	// maxPayloadManifestBytes caps the payload manifest read. The largest
	// manifest in the reference agentpakke is ~30 KB; anything past this is a
	// malformed or hostile input, not a payload manifest.
	maxPayloadManifestBytes = 2 << 20
)

// PayloadManifest is a parsed Tier 2 payload manifest — the file at
// <payload>/manifest.json, or wherever the client entry's `manifest` field
// points (see [Payload.ManifestPath]).
//
// Unknown fields are tolerated, matching the contract's ignore-unknown rule:
// the reference generator also emits `generator`, `counts`, `agents` and
// `skills`, which are descriptive and not part of the digest contract.
type PayloadManifest struct {
	// SchemaVersion is the payload-manifest contract version. Only
	// [PayloadSchemaVersion] is verifiable by this binary.
	SchemaVersion int `json:"schemaVersion"`

	// Target is the payload's own build-target name ("copilot-full-v1", …).
	//
	// Deviation from the reference: grillmester pins target against a
	// hard-coded map of its own target names. The agentpakke contract carries
	// no target names — an agentpakke names its payloads by client×context, not
	// by build target — so Target is parsed and exposed but not asserted
	// against. The digest chain, not the label, is what binds the tree.
	Target string `json:"target,omitempty"`

	// Files maps every payload-relative file path to its expected content and
	// mode. The map is exhaustive: the tree may contain nothing else (bar the
	// payload manifest itself) and must contain all of it.
	Files map[string]FileRecord `json:"files"`
}

// FileRecord is one payload manifest entry: what a file must hash to and which
// mode it must carry.
type FileRecord struct {
	// SHA256 is the file's content digest as 64 lowercase hex characters.
	SHA256 string `json:"sha256"`

	// Mode is the file's permission bits as an octal string, either "0644" or
	// "0755" — the only two modes the contract allows a payload to declare.
	Mode string `json:"mode"`
}

// perm returns the permission bits Mode declares, or an error naming the
// offending value.
func (r FileRecord) perm() (fs.FileMode, error) {
	switch r.Mode {
	case "0644":
		return 0o644, nil
	case "0755":
		return 0o755, nil
	}
	return 0, fmt.Errorf("mode %q is not one of \"0644\" or \"0755\"", r.Mode)
}

// ParsePayloadManifest validates raw payload-manifest bytes and returns the
// manifest. Every record is checked here, before a single file is hashed, so a
// malformed manifest is reported as such rather than as a digest mismatch.
func ParsePayloadManifest(data []byte) (*PayloadManifest, error) {
	// Files is decoded per record below so a malformed record can name the path
	// it belongs to; encoding/json's own error for a map value does not.
	var doc struct {
		SchemaVersion int                        `json:"schemaVersion"`
		Target        string                     `json:"target"`
		Files         map[string]json.RawMessage `json:"files"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("payload manifest is not a JSON object with schemaVersion and a files map: %w", err)
	}
	if doc.SchemaVersion != PayloadSchemaVersion {
		return nil, fmt.Errorf(
			"payload manifest declares schemaVersion %d; this nav-pilot verifies payload schemaVersion %d. "+
				"Upgrade nav-pilot (nav-pilot update) or ask the agentpakke to publish a payload manifest on a supported version",
			doc.SchemaVersion, PayloadSchemaVersion)
	}
	if doc.Files == nil {
		return nil, fmt.Errorf(
			"payload manifest has no \"files\" map; nav-pilot refuses to verify a payload that does not declare its exact contents")
	}

	m := &PayloadManifest{
		SchemaVersion: doc.SchemaVersion,
		Target:        doc.Target,
		Files:         make(map[string]FileRecord, len(doc.Files)),
	}
	// Sorted so a manifest with several bad records always reports the same one.
	rels := make([]string, 0, len(doc.Files))
	for rel := range doc.Files {
		rels = append(rels, rel)
	}
	sort.Strings(rels)
	for _, rel := range rels {
		rec, err := parseFileRecord(rel, doc.Files[rel])
		if err != nil {
			return nil, err
		}
		m.Files[rel] = rec
	}
	return m, nil
}

// parseFileRecord validates one files-map entry: the key as a contained,
// normalized payload-relative path, and the record as a well-formed digest and
// mode.
func parseFileRecord(rel string, raw json.RawMessage) (FileRecord, error) {
	// checkRelPath rejects the empty, padded, absolute, backslashed and
	// escaping forms; path.Clean additionally rejects the forms that stay
	// inside the tree but are not normalized ("./a", "a/../b", "a//b", "a/").
	if err := checkRelPath(rel); err != nil {
		return FileRecord{}, fmt.Errorf("payload manifest lists %q: %w", rel, err)
	}
	if path.Clean(rel) != rel {
		return FileRecord{}, fmt.Errorf(
			"payload manifest lists %q, which is not a normalized payload-relative path; write it without \".\", \"..\", duplicate or trailing slashes",
			rel)
	}
	var rec FileRecord
	if err := json.Unmarshal(raw, &rec); err != nil {
		return FileRecord{}, fmt.Errorf(
			"payload manifest record for %q is malformed; expected {\"sha256\": …, \"mode\": …}: %w", rel, err)
	}
	if !isSHA256Hex(rec.SHA256) {
		return FileRecord{}, fmt.Errorf(
			"payload manifest record for %q has sha256 %q, want 64 lowercase hex characters", rel, rec.SHA256)
	}
	if _, err := rec.perm(); err != nil {
		return FileRecord{}, fmt.Errorf("payload manifest record for %q: %w", rel, err)
	}
	return rec, nil
}

// isSHA256Hex reports whether s is a 64-character lowercase hex digest.
func isSHA256Hex(s string) bool {
	if len(s) != 64 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

// VerifyPayload checks a payload tree against its payload manifest and returns
// the first violation it finds, or nil when the tree matches the manifest
// exactly. Both paths are absolute (or relative to the process working
// directory); manifestPath may sit inside payloadDir or outside it.
//
// It fails closed on every one of: a symlink or non-regular file anywhere in
// the tree, a file the manifest does not list, a manifested file missing from
// disk, a content digest that does not match, a mode that does not match, and a
// malformed manifest or record.
//
// Unlike the rest of validation, this returns a single error rather than every
// violation: past the first mismatch the payload is untrustworthy, and there is
// nothing an agentpakke author gains from a full list of what else is wrong
// with a tree that will not be staged either way.
func VerifyPayload(payloadDir, manifestPath string) error {
	data, err := readPayloadManifestFile(manifestPath)
	if err != nil {
		return err
	}
	m, err := ParsePayloadManifest(data)
	if err != nil {
		return fmt.Errorf("%s: %w", manifestPath, err)
	}
	return m.verifyTree(payloadDir, manifestPath)
}

// readPayloadManifestFile reads the payload manifest itself under the same
// distrust as the tree it describes: a symlinked or non-regular manifest could
// point anywhere, and an oversized one is not a payload manifest.
func readPayloadManifestFile(manifestPath string) ([]byte, error) {
	info, err := os.Lstat(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("payload manifest %s is unavailable: %v", manifestPath, err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return nil, fmt.Errorf("payload manifest %s must be a real, regular file, not a symlink or a directory", manifestPath)
	}
	if info.Size() > maxPayloadManifestBytes {
		return nil, fmt.Errorf("payload manifest %s is %d bytes, past the %d-byte limit", manifestPath, info.Size(), maxPayloadManifestBytes)
	}
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("reading payload manifest %s: %v", manifestPath, err)
	}
	return data, nil
}

// verifyTree walks the payload directory and compares it against the manifest
// in both directions.
func (m *PayloadManifest) verifyTree(payloadDir, manifestPath string) error {
	info, err := os.Lstat(payloadDir)
	if err != nil {
		return fmt.Errorf("payload directory %s is unavailable: %v", payloadDir, err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return fmt.Errorf("payload %s must be a real directory, not a symlink or a file", payloadDir)
	}

	// The payload manifest describes the tree and is not part of it, so it is
	// skipped when it lives inside. A manifest that also lists itself in files
	// is then reported as a missing manifested file, which is what it is.
	skip := payloadRelative(payloadDir, manifestPath)

	seen := make(map[string]bool, len(m.Files))
	walkErr := filepath.WalkDir(payloadDir, func(abs string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("reading payload entry %s: %v", abs, err)
		}
		if abs == payloadDir {
			return nil
		}
		rel := payloadRelative(payloadDir, abs)
		if rel == "" {
			return fmt.Errorf("payload entry %s resolves outside the payload directory %s", abs, payloadDir)
		}
		// WalkDir lstats, so a symlink is reported as a symlink and never
		// descended into — a symlinked directory is caught here too.
		if d.Type()&fs.ModeSymlink != 0 {
			return fmt.Errorf(
				"payload contains the symlink %q; payload content must be real files and directories, and nav-pilot will not stage what a link points at", rel)
		}
		if d.IsDir() {
			return nil
		}
		if !d.Type().IsRegular() {
			return fmt.Errorf("payload contains %q, which is not a regular file", rel)
		}
		if rel == skip {
			return nil
		}
		rec, ok := m.Files[rel]
		if !ok {
			return fmt.Errorf(
				"payload contains %q, which the payload manifest does not list; nav-pilot refuses to stage an unmanifested file", rel)
		}
		seen[rel] = true
		return verifyPayloadFile(abs, rel, rec)
	})
	if walkErr != nil {
		return walkErr
	}

	var missing []string
	for rel := range m.Files {
		if !seen[rel] {
			missing = append(missing, rel)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return fmt.Errorf(
			"payload is missing file(s) the payload manifest declares: %s", strings.Join(missing, ", "))
	}
	return nil
}

// verifyPayloadFile compares one file's content digest and mode against its
// manifest record.
func verifyPayloadFile(abs, rel string, rec FileRecord) error {
	f, err := os.Open(abs)
	if err != nil {
		return fmt.Errorf("reading payload file %q: %v", rel, err)
	}
	defer f.Close()

	digest := sha256.New()
	if _, err := io.Copy(digest, f); err != nil {
		return fmt.Errorf("reading payload file %q: %v", rel, err)
	}
	if got := hex.EncodeToString(digest.Sum(nil)); got != rec.SHA256 {
		return fmt.Errorf(
			"payload file %q has sha256 %s but the payload manifest declares %s; the payload does not match its manifest", rel, got, rec.SHA256)
	}

	// Stat the open descriptor rather than the path: the file was already
	// lstat'd as a regular file during the walk, and this cannot be redirected
	// by a link swapped in between.
	info, err := f.Stat()
	if err != nil {
		return fmt.Errorf("inspecting payload file %q: %v", rel, err)
	}
	want, err := rec.perm()
	if err != nil {
		// Unreachable: ParsePayloadManifest already rejected every other mode.
		return fmt.Errorf("payload file %q: %w", rel, err)
	}
	// Deviation from the reference, which compares S_IMODE exactly. On the
	// source side only the exec bit is compared: a checkout is created under
	// the user's umask, so a restrictive umask (0077) yields 0600/0700 for
	// files the manifest declares 0644/0755 — content-identical, and failing
	// verification over it would reject every conforming payload on those
	// machines. The exec bit is the part that carries meaning here, and the
	// digest binds the content. Exact modes are enforced where they matter:
	// the staging package chmods the staged tree to the manifest's modes and
	// re-runs the full exact check on it before any client is pointed at it.
	if (info.Mode().Perm()&0o111 != 0) != (want&0o111 != 0) {
		return fmt.Errorf(
			"payload file %q is mode %04o but the payload manifest declares %s; the executable bit does not match",
			rel, info.Mode().Perm(), rec.Mode)
	}
	return nil
}

// payloadRelative returns p as a slash-separated path relative to payloadDir,
// or "" when p is not inside payloadDir.
func payloadRelative(payloadDir, p string) string {
	rel, err := filepath.Rel(payloadDir, p)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return ""
	}
	return filepath.ToSlash(rel)
}
