package agentpakke

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

// refPayloadFiles is the payload tree that
// testdata/reference-payload-manifest.json declares — the shape a real
// grillmester payload has (a top-level licence, an agent under agents/, an
// executable script), trimmed to three files.
var refPayloadFiles = []payloadFile{
	{rel: "LICENSE", content: "MIT\n", mode: 0o644},
	{rel: "agents/barista.agent.md", content: "---\nname: barista\n---\n\nKaffe.\n", mode: 0o644},
	{rel: "scripts/hook.sh", content: "#!/bin/sh\nexit 0\n", mode: 0o755},
}

// --- ParsePayloadManifest ---

func TestParsePayloadManifest(t *testing.T) {
	m, err := ParsePayloadManifest(payloadManifestBytes(t, payloadManifestDoc(refPayloadFiles)))
	if err != nil {
		t.Fatalf("ParsePayloadManifest = %v, want nil", err)
	}
	if m.SchemaVersion != PayloadSchemaVersion {
		t.Errorf("SchemaVersion = %d, want %d", m.SchemaVersion, PayloadSchemaVersion)
	}
	if len(m.Files) != len(refPayloadFiles) {
		t.Errorf("Files has %d records, want %d", len(m.Files), len(refPayloadFiles))
	}
	if got := m.Files["scripts/hook.sh"].Mode; got != "0755" {
		t.Errorf("Files[scripts/hook.sh].Mode = %q, want 0755", got)
	}
}

func TestParsePayloadManifestIgnoresUnknownFields(t *testing.T) {
	// The reference generator emits generator/counts/agents/skills alongside
	// files; they are descriptive and must not make the manifest unreadable.
	doc := payloadManifestDoc(refPayloadFiles)
	doc["generator"] = map[string]any{"path": "scripts/generate_copilot_manifest.py", "version": 1}
	doc["counts"] = map[string]any{"agents": 1, "skills": 0}
	doc["agents"] = []any{"barista"}
	doc["futureField"] = true
	doc["files"].(map[string]any)["LICENSE"].(map[string]any)["size"] = 4

	if _, err := ParsePayloadManifest(payloadManifestBytes(t, doc)); err != nil {
		t.Fatalf("ParsePayloadManifest = %v, want nil (unknown fields must be ignored, not rejected)", err)
	}
}

func TestParsePayloadManifestFailsClosed(t *testing.T) {
	tests := []struct {
		name     string
		patch    func(doc map[string]any)
		raw      string
		wantErrs []string
	}{
		{
			name:     "not JSON",
			raw:      `not json at all`,
			wantErrs: []string{"payload manifest is not a JSON object"},
		},
		{
			name:     "files is not an object",
			raw:      `{"schemaVersion": 1, "files": "everything"}`,
			wantErrs: []string{"payload manifest is not a JSON object"},
		},
		{
			name:     "no files map",
			raw:      `{"schemaVersion": 1, "target": "copilot-full-v1"}`,
			wantErrs: []string{"no \"files\" map", "exact contents"},
		},
		{
			name:     "unsupported schemaVersion",
			patch:    func(doc map[string]any) { doc["schemaVersion"] = 2 },
			wantErrs: []string{"schemaVersion 2", "nav-pilot update"},
		},
		{
			name:     "missing schemaVersion",
			raw:      `{"files": {}}`,
			wantErrs: []string{"schemaVersion 0"},
		},
		{
			name: "record is not an object",
			patch: func(doc map[string]any) {
				doc["files"].(map[string]any)["LICENSE"] = 7
			},
			wantErrs: []string{`record for "LICENSE" is malformed`},
		},
		{
			name: "sha256 is not a digest",
			patch: func(doc map[string]any) {
				doc["files"].(map[string]any)["LICENSE"].(map[string]any)["sha256"] = "deadbeef"
			},
			wantErrs: []string{`record for "LICENSE"`, "64 lowercase hex"},
		},
		{
			name: "sha256 in uppercase",
			patch: func(doc map[string]any) {
				rec := doc["files"].(map[string]any)["LICENSE"].(map[string]any)
				rec["sha256"] = strings.ToUpper(rec["sha256"].(string))
			},
			wantErrs: []string{"64 lowercase hex"},
		},
		{
			name: "mode outside the allowed set",
			patch: func(doc map[string]any) {
				doc["files"].(map[string]any)["scripts/hook.sh"].(map[string]any)["mode"] = "0777"
			},
			wantErrs: []string{`record for "scripts/hook.sh"`, `mode "0777"`},
		},
		{
			name: "key escapes the payload",
			patch: func(doc map[string]any) {
				doc["files"].(map[string]any)["../../etc/passwd"] = fileRecordFor("x")
			},
			wantErrs: []string{`"../../etc/passwd"`, "escapes"},
		},
		{
			name: "absolute key",
			patch: func(doc map[string]any) {
				doc["files"].(map[string]any)["/etc/passwd"] = fileRecordFor("x")
			},
			wantErrs: []string{`"/etc/passwd"`, "absolute"},
		},
		{
			name: "key with a backslash separator",
			patch: func(doc map[string]any) {
				doc["files"].(map[string]any)[`agents\barista.agent.md`] = fileRecordFor("x")
			},
			wantErrs: []string{"forward slashes"},
		},
		{
			name: "non-normalized key that stays inside",
			patch: func(doc map[string]any) {
				doc["files"].(map[string]any)["agents/../LICENSE.txt"] = fileRecordFor("x")
			},
			wantErrs: []string{"not a normalized payload-relative path"},
		},
		{
			name: "empty key",
			patch: func(doc map[string]any) {
				doc["files"].(map[string]any)[""] = fileRecordFor("x")
			},
			wantErrs: []string{"must not be empty"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := []byte(tt.raw)
			if tt.raw == "" {
				doc := payloadManifestDoc(refPayloadFiles)
				tt.patch(doc)
				data = payloadManifestBytes(t, doc)
			}
			_, err := ParsePayloadManifest(data)
			if err == nil {
				t.Fatal("ParsePayloadManifest = nil, want a fail-closed error")
			}
			assertErrMentions(t, err, tt.wantErrs)
		})
	}
}

// --- VerifyPayload ---

func TestVerifyPayload(t *testing.T) {
	tests := []struct {
		name string
		// mutate runs after the conforming tree is written and before the
		// manifest is written, so a case can change the tree, the manifest, or
		// both.
		mutate   func(t *testing.T, dir string, doc map[string]any)
		wantErrs []string
	}{
		{
			name: "conforming payload",
		},
		{
			name: "restrictive umask on a 0644 file is tolerated",
			// Deliberate deviation from the reference's exact-IMODE check: a
			// clone made under umask 0077 is content-identical and must verify.
			mutate: func(t *testing.T, dir string, _ map[string]any) {
				chmod(t, filepath.Join(dir, "LICENSE"), 0o600)
			},
		},
		{
			name: "restrictive umask on a 0755 file is tolerated",
			mutate: func(t *testing.T, dir string, _ map[string]any) {
				chmod(t, filepath.Join(dir, "scripts", "hook.sh"), 0o700)
			},
		},
		{
			name: "symlinked file in the tree",
			mutate: func(t *testing.T, dir string, _ map[string]any) {
				rm(t, filepath.Join(dir, "LICENSE"))
				symlink(t, "/etc/passwd", filepath.Join(dir, "LICENSE"))
			},
			wantErrs: []string{`symlink "LICENSE"`, "real files and directories"},
		},
		{
			name: "symlinked directory in the tree",
			mutate: func(t *testing.T, dir string, _ map[string]any) {
				symlink(t, t.TempDir(), filepath.Join(dir, "elsewhere"))
			},
			wantErrs: []string{`symlink "elsewhere"`},
		},
		{
			name: "symlink pointing at a conforming file",
			// Content-equal but still refused: what a link resolves to can
			// change between verification and staging.
			mutate: func(t *testing.T, dir string, doc map[string]any) {
				symlink(t, filepath.Join(dir, "LICENSE"), filepath.Join(dir, "COPYING"))
				doc["files"].(map[string]any)["COPYING"] = fileRecordFor("MIT\n")
			},
			wantErrs: []string{`symlink "COPYING"`},
		},
		{
			name: "file on disk the manifest does not list",
			mutate: func(t *testing.T, dir string, _ map[string]any) {
				writeFile(t, filepath.Join(dir, "agents", "smuggled.agent.md"), "---\nname: smuggled\n---\n")
			},
			wantErrs: []string{`"agents/smuggled.agent.md"`, "does not list", "unmanifested"},
		},
		{
			name: "manifested file missing from disk",
			mutate: func(t *testing.T, dir string, _ map[string]any) {
				rm(t, filepath.Join(dir, "agents", "barista.agent.md"))
			},
			wantErrs: []string{"missing file(s)", "agents/barista.agent.md"},
		},
		{
			name: "digest mismatch",
			mutate: func(t *testing.T, dir string, _ map[string]any) {
				writeFile(t, filepath.Join(dir, "agents", "barista.agent.md"),
					"---\nname: barista\n---\n\nKaffe. Og en instruks til.\n")
			},
			wantErrs: []string{`"agents/barista.agent.md"`, "sha256", "does not match its manifest"},
		},
		{
			name: "empty file where content is manifested",
			mutate: func(t *testing.T, dir string, _ map[string]any) {
				writeFile(t, filepath.Join(dir, "LICENSE"), "")
			},
			wantErrs: []string{`"LICENSE"`, "sha256"},
		},
		{
			name: "exec bit added to a 0644 file",
			mutate: func(t *testing.T, dir string, _ map[string]any) {
				chmod(t, filepath.Join(dir, "LICENSE"), 0o755)
			},
			wantErrs: []string{`"LICENSE"`, "executable bit does not match", "0644"},
		},
		{
			name: "exec bit removed from a 0755 file",
			mutate: func(t *testing.T, dir string, _ map[string]any) {
				chmod(t, filepath.Join(dir, "scripts", "hook.sh"), 0o644)
			},
			wantErrs: []string{`"scripts/hook.sh"`, "executable bit does not match", "0755"},
		},
		{
			name: "setuid added to a 0755 file",
			// FileMode.Perm() masks to 0777, so setuid is invisible to a
			// permission comparison and needs its own check.
			mutate: func(t *testing.T, dir string, _ map[string]any) {
				chmodSpecial(t, filepath.Join(dir, "scripts", "hook.sh"), os.ModeSetuid|0o755)
			},
			wantErrs: []string{`"scripts/hook.sh"`, "setuid/setgid/sticky"},
		},
		{
			name: "setgid added to a 0644 file",
			mutate: func(t *testing.T, dir string, _ map[string]any) {
				chmodSpecial(t, filepath.Join(dir, "LICENSE"), os.ModeSetgid|0o644)
			},
			wantErrs: []string{`"LICENSE"`, "setuid/setgid/sticky"},
		},
		{
			name: "world-writable where the manifest declares 0644",
			// A umask only clears bits, so extra bits are never a umask artifact.
			mutate: func(t *testing.T, dir string, _ map[string]any) {
				chmod(t, filepath.Join(dir, "LICENSE"), 0o666)
			},
			wantErrs: []string{`"LICENSE"`, "0666", "0644", "does not"},
		},
		{
			name: "world-writable where the manifest declares 0755",
			mutate: func(t *testing.T, dir string, _ map[string]any) {
				chmod(t, filepath.Join(dir, "scripts", "hook.sh"), 0o777)
			},
			wantErrs: []string{`"scripts/hook.sh"`, "0777", "0755", "does not"},
		},
		{
			name: "malformed record",
			mutate: func(_ *testing.T, _ string, doc map[string]any) {
				doc["files"].(map[string]any)["LICENSE"] = "adc37366"
			},
			wantErrs: []string{`record for "LICENSE" is malformed`},
		},
		{
			name: "manifest lists a path outside the payload",
			mutate: func(_ *testing.T, _ string, doc map[string]any) {
				doc["files"].(map[string]any)["../secrets.env"] = fileRecordFor("x")
			},
			wantErrs: []string{`"../secrets.env"`, "escapes"},
		},
		{
			name: "manifest lists itself",
			mutate: func(_ *testing.T, _ string, doc map[string]any) {
				doc["files"].(map[string]any)[PayloadManifestFile] = fileRecordFor("{}")
			},
			wantErrs: []string{"missing file(s)", PayloadManifestFile},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := writePayloadTree(t, refPayloadFiles)
			doc := payloadManifestDoc(refPayloadFiles)
			if tt.mutate != nil {
				tt.mutate(t, dir, doc)
			}
			manifestPath := filepath.Join(dir, PayloadManifestFile)
			writeFile(t, manifestPath, string(payloadManifestBytes(t, doc)))

			err := VerifyPayload(dir, manifestPath)
			if len(tt.wantErrs) == 0 {
				if err != nil {
					t.Fatalf("VerifyPayload = %v, want nil", err)
				}
				return
			}
			if err == nil {
				t.Fatal("VerifyPayload = nil, want a fail-closed error")
			}
			assertErrMentions(t, err, tt.wantErrs)
		})
	}
}

func TestVerifyPayloadManifestFile(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(t *testing.T, dir string) string
		wantErrs []string
	}{
		{
			name: "missing manifest",
			setup: func(_ *testing.T, dir string) string {
				return filepath.Join(dir, PayloadManifestFile)
			},
			wantErrs: []string{"payload manifest", "unavailable"},
		},
		{
			name: "symlinked manifest",
			setup: func(t *testing.T, dir string) string {
				real := filepath.Join(t.TempDir(), "elsewhere.json")
				writeFile(t, real, string(payloadManifestBytes(t, payloadManifestDoc(refPayloadFiles))))
				link := filepath.Join(dir, PayloadManifestFile)
				symlink(t, real, link)
				return link
			},
			wantErrs: []string{"real, regular file"},
		},
		{
			name: "manifest is a directory",
			setup: func(t *testing.T, dir string) string {
				p := filepath.Join(dir, PayloadManifestFile)
				mkdirAll(t, p)
				return p
			},
			wantErrs: []string{"real, regular file"},
		},
		{
			name: "manifest outside the payload directory",
			setup: func(t *testing.T, dir string) string {
				p := filepath.Join(filepath.Dir(dir), "payload-manifest.json")
				writeFile(t, p, string(payloadManifestBytes(t, payloadManifestDoc(refPayloadFiles))))
				return p
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := writePayloadTree(t, refPayloadFiles)
			err := VerifyPayload(dir, tt.setup(t, dir))
			if len(tt.wantErrs) == 0 {
				if err != nil {
					t.Fatalf("VerifyPayload = %v, want nil (a manifest outside the tree is legal, per the `manifest` override)", err)
				}
				return
			}
			if err == nil {
				t.Fatal("VerifyPayload = nil, want a fail-closed error")
			}
			assertErrMentions(t, err, tt.wantErrs)
		})
	}
}

func TestVerifyPayloadRejectsSymlinkedPayloadDir(t *testing.T) {
	dir := writePayloadTree(t, refPayloadFiles)
	manifestPath := filepath.Join(dir, PayloadManifestFile)
	writeFile(t, manifestPath, string(payloadManifestBytes(t, payloadManifestDoc(refPayloadFiles))))

	link := filepath.Join(t.TempDir(), "payload")
	symlink(t, dir, link)

	err := VerifyPayload(link, filepath.Join(link, PayloadManifestFile))
	if err == nil || !strings.Contains(err.Error(), "real directory") {
		t.Fatalf("VerifyPayload(symlinked payload dir) = %v, want a real-directory error", err)
	}
}

// TestVerifyPayloadRejectsFifo pins the walk's non-regular-file check. A fifo
// is the case that makes the check load-bearing rather than tidy: without it
// verification does not merely mis-verify the payload, it blocks forever in
// open(2). The test therefore fails on a hang as well as on a nil error.
func TestVerifyPayloadRejectsFifo(t *testing.T) {
	dir := writePayloadTree(t, refPayloadFiles)
	if err := unix.Mkfifo(filepath.Join(dir, "pipe"), 0o644); err != nil {
		t.Skipf("mkfifo unsupported here: %v", err)
	}
	// The manifest lists the fifo, so the unmanifested-file check cannot shadow
	// the non-regular-file check: with the latter removed, verification really
	// does reach open(2) on the fifo and block there.
	doc := payloadManifestDoc(refPayloadFiles)
	doc["files"].(map[string]any)["pipe"] = fileRecordFor("")
	manifestPath := filepath.Join(dir, PayloadManifestFile)
	writeFile(t, manifestPath, string(payloadManifestBytes(t, doc)))

	done := make(chan error, 1)
	go func() { done <- VerifyPayload(dir, manifestPath) }()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("VerifyPayload = nil with a fifo in the tree, want a non-regular-file error")
		}
		// The walk's own wording, not the descriptor check's: this test is what
		// gives the walk check teeth, and the two layers are worded apart so
		// removing either one is visible here or in the direct test below.
		assertErrMentions(t, err, []string{`payload contains "pipe", which is not a regular file`})
	case <-time.After(30 * time.Second):
		// Deliberately not t.Fatal from this goroutine's perspective: the
		// verifier is stuck in open(2) and will never return.
		t.Fatal("VerifyPayload hung on a fifo; the non-regular-file check is what prevents this")
	}
}

// TestVerifyPayloadFileRefusesToFollowSymlink pins the O_NOFOLLOW on the open
// in verifyPayloadFile. The walk rejects symlinks it sees, but the walk's lstat
// and the open are two syscalls apart, so a link swapped in between would be
// followed and — if its target's content matched the digest — would verify.
// This calls verifyPayloadFile directly, which is the same state the race would
// leave it in, without having to win a race to observe it.
func TestVerifyPayloadFileRefusesToFollowSymlink(t *testing.T) {
	dir := t.TempDir()
	real := filepath.Join(dir, "real")
	writeFile(t, real, "MIT\n")
	link := filepath.Join(dir, "LICENSE")
	symlink(t, real, link)

	// The record matches what the link resolves to, so only O_NOFOLLOW can
	// reject this.
	rec := FileRecord{SHA256: sha256Hex("MIT\n"), Mode: "0644"}
	err := verifyPayloadFile(link, "LICENSE", rec)
	if err == nil {
		t.Fatal("verifyPayloadFile followed a symlink to a digest-matching target, want an open failure")
	}
	assertErrMentions(t, err, []string{`"LICENSE"`})
}

// TestVerifyPayloadFileRejectsNonRegularDescriptor pins the regular-file check
// on the fstat, which the walk's check would otherwise mask. It calls
// verifyPayloadFile directly — the state a fifo swapped in between the walk's
// lstat and the open would leave it in — and proves two things at once: the
// open returns rather than blocking (O_NONBLOCK), and what it opened is
// refused before a byte is read from it.
func TestVerifyPayloadFileRejectsNonRegularDescriptor(t *testing.T) {
	dir := t.TempDir()
	pipe := filepath.Join(dir, "pipe")
	if err := unix.Mkfifo(pipe, 0o644); err != nil {
		t.Skipf("mkfifo unsupported here: %v", err)
	}
	// An empty-content record: without the type checks a reader-less fifo hashes
	// as the empty file and verifies clean, so the record must not be what
	// rejects it.
	rec := FileRecord{SHA256: sha256Hex(""), Mode: "0644"}

	done := make(chan error, 1)
	go func() { done <- verifyPayloadFile(pipe, "pipe", rec) }()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("verifyPayloadFile = nil for a fifo, want a non-regular-file error")
		}
		assertErrMentions(t, err, []string{`"pipe"`, "changed type between the payload walk and the open"})
	case <-time.After(30 * time.Second):
		t.Fatal("verifyPayloadFile hung on a fifo; O_NONBLOCK is what prevents this")
	}
}

// TestVerifyPayloadRejectsOversizedManifest pins the manifest size cap. The
// oversized manifest is otherwise well-formed, so only the cap can reject it.
func TestVerifyPayloadRejectsOversizedManifest(t *testing.T) {
	dir := writePayloadTree(t, refPayloadFiles)
	doc := payloadManifestDoc(refPayloadFiles)
	// Padding in an ignored-unknown field keeps the manifest valid JSON that
	// would parse and verify fine were it not for its size.
	doc["padding"] = strings.Repeat("x", maxPayloadManifestBytes)
	manifestPath := filepath.Join(dir, PayloadManifestFile)
	writeFile(t, manifestPath, string(payloadManifestBytes(t, doc)))

	err := VerifyPayload(dir, manifestPath)
	if err == nil {
		t.Fatal("VerifyPayload = nil for an oversized manifest, want a size-cap error")
	}
	assertErrMentions(t, err, []string{"payload manifest", "past the", "limit"})
}

// TestVerifyPayloadReferenceFixture runs the verifier against a vendored
// payload manifest in the reference generator's own shape
// (grillmester plugin/manifest.json at 3573b93cc8b7568516117263562d073cae9ee7fc,
// trimmed to three files), rather than one this test built.
func TestVerifyPayloadReferenceFixture(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "reference-payload-manifest.json"))
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}
	m, err := ParsePayloadManifest(data)
	if err != nil {
		t.Fatalf("ParsePayloadManifest(fixture) = %v, want nil", err)
	}
	if m.Target != "copilot-full-v1" {
		t.Errorf("Target = %q, want copilot-full-v1 (parsed, though not asserted against — see the deviation note)", m.Target)
	}

	dir := writePayloadTree(t, refPayloadFiles)
	manifestPath := filepath.Join(dir, PayloadManifestFile)
	writeFile(t, manifestPath, string(data))
	if err := VerifyPayload(dir, manifestPath); err != nil {
		t.Fatalf("VerifyPayload against the fixture manifest = %v, want nil", err)
	}

	// And the same fixture rejects a flipped byte.
	writeFile(t, filepath.Join(dir, "LICENSE"), "MIT?\n")
	if err := VerifyPayload(dir, manifestPath); err == nil {
		t.Fatal("VerifyPayload = nil after tampering with a fixture-manifested file, want a digest error")
	}
}

// --- validate wiring ---

// tier2Manifest is a minimal Tier 2 agentpakke manifest: one client, one
// payload, no layout.
const tier2Manifest = `{
  "contractVersion": "1",
  "name": "grillmester",
  "description": "Tier 2 agentpakke med én payload",
  "clients": {
    "copilot": {
      "primaryAgents": ["grillmester"],
      "payloads": { "full": { "path": "plugin" } }
    }
  }
}`

func TestValidateSourceVerifiesPayloadDigests(t *testing.T) {
	tests := []struct {
		name     string
		tamper   func(t *testing.T, payloadDir string)
		wantErrs []string
	}{
		{
			name: "verified payload",
		},
		{
			name: "digest mismatch is reported by validate",
			tamper: func(t *testing.T, payloadDir string) {
				writeFile(t, filepath.Join(payloadDir, "LICENSE"), "Apache-2.0\n")
			},
			wantErrs: []string{"clients.copilot.payloads.full", `"LICENSE"`, "sha256"},
		},
		{
			name: "unmanifested file is reported by validate",
			tamper: func(t *testing.T, payloadDir string) {
				writeFile(t, filepath.Join(payloadDir, "extra.md"), "hei\n")
			},
			wantErrs: []string{"clients.copilot.payloads.full", "unmanifested"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			writeManifest(t, root, tier2Manifest)
			payloadDir := filepath.Join(root, "plugin")
			mkdirAll(t, payloadDir)
			for _, f := range refPayloadFiles {
				mkdirAll(t, filepath.Dir(filepath.Join(payloadDir, filepath.FromSlash(f.rel))))
				writeFile(t, filepath.Join(payloadDir, filepath.FromSlash(f.rel)), f.content)
				chmod(t, filepath.Join(payloadDir, filepath.FromSlash(f.rel)), f.mode)
			}
			writeFile(t, filepath.Join(payloadDir, PayloadManifestFile),
				string(payloadManifestBytes(t, payloadManifestDoc(refPayloadFiles))))
			if tt.tamper != nil {
				tt.tamper(t, payloadDir)
			}

			errs := ValidateSource(root)
			if len(tt.wantErrs) == 0 {
				if len(errs) != 0 {
					t.Fatalf("ValidateSource = %v, want no violations", errs)
				}
				return
			}
			if len(errs) == 0 {
				t.Fatal("ValidateSource = no violations, want the payload verification to fail closed")
			}
			joined := joinErrs(errs)
			for _, want := range tt.wantErrs {
				if !strings.Contains(joined, want) {
					t.Errorf("violations %q do not mention %q", joined, want)
				}
			}
		})
	}
}

// TestValidateSourceSkipsPayloadVerificationWithoutManifest pins the fail-closed
// order: a payload without a payload manifest is reported as unmanifested, and
// verification (which would report a second, confusing error for the same
// payload) does not run.
func TestValidateSourceSkipsPayloadVerificationWithoutManifest(t *testing.T) {
	root := t.TempDir()
	writeManifest(t, root, tier2Manifest)
	mkdirAll(t, filepath.Join(root, "plugin"))

	errs := ValidateSource(root)
	if len(errs) != 1 {
		t.Fatalf("ValidateSource returned %d violations, want exactly the missing-payload-manifest one: %v", len(errs), errs)
	}
	if joined := joinErrs(errs); !strings.Contains(joined, "payload manifest") {
		t.Errorf("violation %q should name the missing payload manifest", joined)
	}
}

// --- helpers ---

// payloadFile is one file in a synthetic payload tree.
type payloadFile struct {
	rel     string
	content string
	mode    os.FileMode
}

// writePayloadTree materializes files into a fresh temp directory and returns it.
func writePayloadTree(t *testing.T, files []payloadFile) string {
	t.Helper()
	dir := t.TempDir()
	for _, f := range files {
		abs := filepath.Join(dir, filepath.FromSlash(f.rel))
		mkdirAll(t, filepath.Dir(abs))
		writeFile(t, abs, f.content)
		chmod(t, abs, f.mode)
	}
	return dir
}

// payloadManifestDoc builds the manifest that describes files exactly, as a
// mutable document so each test case states only what it changes.
func payloadManifestDoc(files []payloadFile) map[string]any {
	records := make(map[string]any, len(files))
	for _, f := range files {
		mode := "0644"
		if f.mode&0o111 != 0 {
			mode = "0755"
		}
		records[f.rel] = map[string]any{"sha256": sha256Hex(f.content), "mode": mode}
	}
	return map[string]any{
		"schemaVersion": PayloadSchemaVersion,
		"target":        "test-v1",
		"files":         records,
	}
}

// fileRecordFor builds a well-formed record for content, so a case that adds a
// files entry fails on the rule it is testing and not on record validation.
func fileRecordFor(content string) map[string]any {
	return map[string]any{"sha256": sha256Hex(content), "mode": "0644"}
}

func payloadManifestBytes(t *testing.T, doc map[string]any) []byte {
	t.Helper()
	data, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("encoding payload manifest: %v", err)
	}
	return data
}

func sha256Hex(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}

func chmod(t *testing.T, path string, mode os.FileMode) {
	t.Helper()
	if err := os.Chmod(path, mode); err != nil {
		t.Fatalf("chmod %s: %v", path, err)
	}
}

// chmodSpecial chmods and then verifies the bits actually stuck: setuid and
// setgid are silently dropped on some filesystems, and a test that asserts a
// rejection must not pass because the bit was never there.
func chmodSpecial(t *testing.T, path string, mode os.FileMode) {
	t.Helper()
	chmod(t, path, mode)
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatalf("lstat %s: %v", path, err)
	}
	if want := mode &^ os.ModePerm; info.Mode()&want != want {
		t.Skipf("filesystem dropped %v on %s (mode is %v)", want, path, info.Mode())
	}
}

func rm(t *testing.T, path string) {
	t.Helper()
	if err := os.Remove(path); err != nil {
		t.Fatalf("remove %s: %v", path, err)
	}
}

func symlink(t *testing.T, target, link string) {
	t.Helper()
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("symlink %s -> %s: %v", link, target, err)
	}
}

func assertErrMentions(t *testing.T, err error, want []string) {
	t.Helper()
	for _, w := range want {
		if !strings.Contains(err.Error(), w) {
			t.Errorf("error %q does not mention %q", err, w)
		}
	}
}
