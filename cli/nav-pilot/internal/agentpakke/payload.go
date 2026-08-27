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

	"golang.org/x/sys/unix"
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
// no target-name pin (see [PayloadManifest.Target]) and a subset-plus-exec-bit
// rather than exact-mode comparison on the source tree (see [FileRecord.Mode]).

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
	return verifyPayload(payloadDir, manifestPath, false)
}

// VerifyPayloadExact is [VerifyPayload] with the reference's exact mode
// comparison — every file's permission bits must equal what the manifest
// declares, not merely be a subset of them.
//
// It is for a tree nav-pilot created itself and therefore controls: the staging
// package copies a source payload file by file and chmods each file to its
// declared mode, so the subset tolerance that a foreign checkout needs (see the
// deviation note in [verifyPayloadFile]) is not only unnecessary there but
// actively unwanted — on a tree we chmod'd, a mode that is merely a subset
// means the chmod did not take, and that is a bug worth failing on.
//
// Do not point this at a source checkout: a clone made under umask 0077 is
// content-identical and would be rejected.
func VerifyPayloadExact(payloadDir, manifestPath string) error {
	return verifyPayload(payloadDir, manifestPath, true)
}

func verifyPayload(payloadDir, manifestPath string, exact bool) error {
	data, err := readPayloadManifestFile(manifestPath)
	if err != nil {
		return err
	}
	m, err := ParsePayloadManifest(data)
	if err != nil {
		return fmt.Errorf("%s: %w", manifestPath, err)
	}
	return m.verifyTree(payloadDir, manifestPath, exact)
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
func (m *PayloadManifest) verifyTree(payloadDir, manifestPath string, exact bool) error {
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
		return verifyPayloadFile(abs, rel, rec, exact)
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

// openPayloadFile opens one payload file for hashing and returns it together
// with the stat of the descriptor itself.
//
// The caller lstat'd this path as a regular file, but that lstat and this open
// are two syscalls apart, so both flags below defend against what can be
// swapped in during that window:
//
//	O_NOFOLLOW — without it a symlink swapped in is followed, and a link
//	  whose target happens to match the digest verifies clean. The open
//	  fails with ELOOP instead.
//	O_NONBLOCK — without it a fifo swapped in blocks this goroutine inside
//	  open(2) forever, hanging `nav-pilot validate` on a hostile or merely
//	  broken source. POSIX gives O_NONBLOCK no effect on regular files, so
//	  it costs nothing on the path that actually matters.
//
// The reference implementation (which lstats, then read_bytes) has neither.
//
// Staging reads its source files through here too, so the same window is closed
// on the copy as on the verification that precedes it.
func openPayloadFile(abs, rel string) (*os.File, fs.FileInfo, error) {
	f, err := os.OpenFile(abs, os.O_RDONLY|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0)
	if err != nil {
		return nil, nil, fmt.Errorf("reading payload file %q: %v", rel, err)
	}

	// Stat the open descriptor rather than the path, so what is checked belongs
	// to the same inode whose bytes are hashed by the caller — a path-based stat
	// could be redirected between the read and the stat.
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, nil, fmt.Errorf("inspecting payload file %q: %v", rel, err)
	}
	// Re-check regularity on the descriptor, not just on the walk's lstat: that
	// is what closes the swapped-in-fifo race rather than merely surviving it.
	// O_NONBLOCK got the open to return; this rejects the thing it opened
	// before a single byte is read from it.
	// Distinct wording from the walk's check on purpose: reaching this one means
	// the entry changed type after the walk saw it, which is a different event
	// from a payload that simply ships a fifo.
	if !info.Mode().IsRegular() {
		f.Close()
		return nil, nil, fmt.Errorf(
			"payload file %q is not a regular file; it changed type between the payload walk and the open", rel)
	}
	return f, info, nil
}

// verifyPayloadFile compares one file's content digest and mode against its
// manifest record. exact selects the staged-tree mode rule (see
// [VerifyPayloadExact]) over the source-tree one.
func verifyPayloadFile(abs, rel string, rec FileRecord, exact bool) error {
	f, info, err := openPayloadFile(abs, rel)
	if err != nil {
		return err
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

	want, err := rec.perm()
	if err != nil {
		// Unreachable: ParsePayloadManifest already rejected every other mode.
		return fmt.Errorf("payload file %q: %w", rel, err)
	}
	// Deviation from the reference, which compares S_IMODE exactly. On the
	// source side the on-disk permissions must be a *subset* of the declared
	// ones, and the exec bit must match. That is narrower than "compare only
	// the exec bit", which the previous revision used: a umask only ever
	// *clears* permission bits, so it can justify tolerating fewer bits than
	// declared and never more. A 0644 file found world-writable at 0666, or a
	// 0755 file found 0777, is not a umask artifact — it is a tree someone
	// widened, and it is refused.
	//
	// The umask case the deviation exists for is the clearing direction only:
	// a checkout under umask 0077 yields 0600/0700 for files declared
	// 0644/0755 — content-identical, and rejecting it would fail every
	// conforming payload on those machines. Note that 0700 is still
	// executable: clearing the exec bit off 0755 needs a umask containing
	// 0111, which would also make every directory the user creates un-enterable
	// and is not a configuration that occurs in practice. So a declared-0755
	// file found without any exec bit is a real mismatch, not a umask artifact,
	// and stays fatal.
	//
	// Setuid, setgid and sticky are rejected separately and explicitly: Go's
	// FileMode.Perm() masks to 0777, so those bits are invisible to the subset
	// comparison and a setuid 04755 file would otherwise verify clean.
	//
	// Exact modes are still enforced where they matter: [StagePayload] chmods
	// the staged tree to the manifest's modes and re-runs this check through
	// [VerifyPayloadExact] before any client is pointed at it.
	if special := info.Mode() & (os.ModeSetuid | os.ModeSetgid | os.ModeSticky); special != 0 {
		return fmt.Errorf(
			"payload file %q carries setuid/setgid/sticky bits (mode %v); nav-pilot refuses to stage a privileged file",
			rel, info.Mode())
	}
	perm := info.Mode().Perm()
	if exact {
		// The staged tree was written and chmod'd by this process, so anything
		// but an exact match means the chmod did not take (a filesystem that
		// drops modes, a umask applied where we thought we had overridden it) —
		// and a payload staged at modes nav-pilot did not choose is exactly what
		// the manifest's mode field exists to prevent.
		if perm != want {
			return fmt.Errorf(
				"staged payload file %q is mode %04o but the payload manifest declares %s; the staged tree must match the manifest's modes exactly",
				rel, perm, rec.Mode)
		}
		return nil
	}
	if (perm&0o111 != 0) != (want&0o111 != 0) {
		return fmt.Errorf(
			"payload file %q is mode %04o but the payload manifest declares %s; the executable bit does not match",
			rel, perm, rec.Mode)
	}
	if perm&^want != 0 {
		return fmt.Errorf(
			"payload file %q is mode %04o but the payload manifest declares %s; it grants permissions the payload manifest does not, and a umask can only take permissions away",
			rel, perm, rec.Mode)
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
