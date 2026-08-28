package agentpakke

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"golang.org/x/sys/unix"
)

// Payload staging (G2).
//
// A verified source payload cannot be handed to a client as it stands. Two
// things are wrong with it: the source may be a temp clone that is deleted the
// moment resolution finishes, and a git checkout carries the machine's umask,
// not the modes the payload manifest declares. Staging fixes both by building a
// second tree that nav-pilot owns outright — copied file by file from the
// manifest, chmod'd to the declared modes, and verified again, exactly, before
// anything is pointed at it.
//
// The verify → copy → re-verify shape is grillmester's, but from its *local*
// mode (scripts/grillmester_local.py::_materialize_opencode_config at
// 3573b93cc8b7568516117263562d073cae9ee7fc), not its cloud launcher: the cloud
// launcher points clients at the payload in place inside an immutable Homebrew
// bundle and never stages at all. Do not go looking for this in
// build_launch_command.
//
// Two deliberate departures from the reference:
//
//   - The reference copies with shutil.copytree(symlinks=True) — it walks the
//     source. This copies from the manifest's files map instead. The manifest,
//     not the source directory, is the authority on what the payload contains,
//     so a file smuggled into the source between verification and staging is
//     simply never copied rather than copied and then rejected.
//   - The reference re-reads the destination to verify it. This additionally
//     re-hashes every file *while* copying it, which closes the window between
//     the source-side verification and the read that copies the bytes.
//
// What the staged tree cannot contain, by construction rather than by check:
// symlinks, hardlinks to anything outside it, directories that are not on a
// manifested file's path, device nodes, fifos, and sockets. Nothing here ever
// creates a node of any other kind — every entry is either a directory created
// by [os.MkdirAll] or a regular file created by [os.OpenFile] with O_CREATE and
// O_EXCL — so there is no path by which one could appear. That is a stronger
// guarantee than rejecting them after the fact, and it is why the exact
// re-verification is a check on our own work rather than the trust boundary.

// stagedDirMode is the mode of every directory [StagePayload] creates. The
// payload manifest declares modes for files only, so nav-pilot picks this one:
// owner-only, because a staged tree is nav-pilot's own content under its own
// root and no other user has business traversing it. That is as true of a tree
// that lives for the duration of one launch as of one that is kept until the
// next revision replaces it. Consequently [VerifyPayloadExact] does not check
// directory modes — there is nothing to check them against.
//
// A umask can only clear bits, so under any sane umask these directories come
// out at 0700; under a umask that clears the owner bits the very next create
// inside them fails and staging aborts, which is the correct outcome for a
// machine configured that way.
const stagedDirMode fs.FileMode = 0o700

// stageCopyHook is a test seam: when non-nil it runs immediately before each
// file is copied, in the order [StagePayload] copies them.
//
// It exists because the mid-copy failure path is unreachable from any input a
// test can construct: every file the copy touches was proven to exist, be
// regular and hash correctly moments earlier, and every directory it creates is
// on the path of such a file, so nothing malformed can make the copy fail. The
// hook lets a test mutate the source between the verification and the copy —
// the real TOCTOU case — and so gives both the copy-time re-hash and the
// fail-closed cleanup something that can actually fail.
var stageCopyHook func(rel string)

// StagePayload verifies the payload at payloadDir against the payload manifest
// at manifestPath, copies it into destDir with the manifest's exact modes, and
// verifies that copy again, exactly. It returns destDir.
//
// destDir is created if absent, and belongs to this call alone: both
// fail-closed paths below remove it wholesale, so it must not be a directory
// anything else is using. The caller names it, which is what lets an install
// stage several payloads into one tree and publish the lot with a single
// rename.
//
// A fixed per-pakke directory would be wrong — wiping and rewriting it would
// pull the config directory out from under a session already running against
// it. A directory named after the revision's content is right for exactly that
// reason: a new revision gets a new name, so nothing is ever rewritten in
// place and a running session keeps the tree it was handed.
//
// It fails closed. Any error at any step leaves no staged tree behind and no
// usable result: there is no partially-staged tree and no fallback to an
// unverified one. A caller that gets an error must abort the launch.
func StagePayload(payloadDir, manifestPath, destDir string) (stagedDir string, err error) {
	// Read and parse the manifest once. The bytes verified below are the same
	// bytes written into the staged tree and re-verified against it, so no
	// second read can slip a different manifest in between.
	data, err := readPayloadManifestFile(manifestPath)
	if err != nil {
		return "", err
	}
	m, err := ParsePayloadManifest(data)
	if err != nil {
		return "", fmt.Errorf("%s: %w", manifestPath, err)
	}
	// Step 1: the source-side check, subset modes and all. Everything after
	// this point trusts that the source matches the manifest.
	if err := m.verifyTree(payloadDir, manifestPath, false); err != nil {
		return "", err
	}
	// The manifest is written into the staged root under its conventional name,
	// so a payload that ships a file of that name would collide with it. That
	// can only happen when the manifest lives outside the payload (the client
	// entry's `manifest` override), because a manifest inside the payload is
	// skipped by verification and would then be reported as a missing
	// manifested file. Rejected here, before anything is created, rather than
	// left to the O_EXCL below where the error would not explain itself.
	if _, ok := m.Files[PayloadManifestFile]; ok {
		return "", fmt.Errorf(
			"payload manifest lists a file named %q, which is the name nav-pilot writes the manifest itself to in the staged tree; a payload may not ship its own %s",
			PayloadManifestFile, PayloadManifestFile)
	}

	if err := os.MkdirAll(destDir, stagedDirMode); err != nil {
		return "", fmt.Errorf("creating the staged payload directory %s: %v", destDir, err)
	}
	stagedDir = destDir

	if err := stageTree(m, data, payloadDir, stagedDir); err != nil {
		os.RemoveAll(stagedDir)
		return "", err
	}
	// Step 4: verify our own work, this time with exact modes — which is
	// enforceable here precisely because we chose them. It catches a chmod that
	// did not take, a short write, and a source entry swapped after the first
	// verification (the reference's own stated reason for re-verifying).
	if err := VerifyPayloadExact(stagedDir, filepath.Join(stagedDir, PayloadManifestFile)); err != nil {
		os.RemoveAll(stagedDir)
		return "", fmt.Errorf("the staged payload does not match its manifest after staging: %w", err)
	}
	return stagedDir, nil
}

// stageTree copies every manifested file into stagedDir and writes the manifest
// beside them. Files are copied in sorted order so a failure is reproducible.
func stageTree(m *PayloadManifest, manifestData []byte, payloadDir, stagedDir string) error {
	rels := make([]string, 0, len(m.Files))
	for rel := range m.Files {
		rels = append(rels, rel)
	}
	sort.Strings(rels)

	for _, rel := range rels {
		rec := m.Files[rel]
		perm, err := rec.perm()
		if err != nil {
			// Unreachable: ParsePayloadManifest rejected every other mode.
			return fmt.Errorf("payload file %q: %w", rel, err)
		}
		dst := filepath.Join(stagedDir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(dst), stagedDirMode); err != nil {
			return fmt.Errorf("creating the staged directory for %q: %v", rel, err)
		}
		if stageCopyHook != nil {
			stageCopyHook(rel)
		}
		if err := stageFile(filepath.Join(payloadDir, filepath.FromSlash(rel)), dst, rel, rec, perm); err != nil {
			return err
		}
	}

	// The staged tree carries its own manifest so it is self-describing once
	// the source is gone — the source may be a temp clone that resolution
	// deletes as soon as staging returns — and so the exact re-verification has
	// an input that does not depend on the source surviving either.
	p := filepath.Join(stagedDir, PayloadManifestFile)
	f, err := os.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_EXCL|unix.O_NOFOLLOW, 0o644)
	if err != nil {
		return fmt.Errorf("creating the staged payload manifest %s: %v", p, err)
	}
	defer f.Close()
	if _, err := f.Write(manifestData); err != nil {
		return fmt.Errorf("writing the staged payload manifest %s: %v", p, err)
	}
	if err := f.Chmod(0o644); err != nil {
		return fmt.Errorf("setting the mode of the staged payload manifest %s: %v", p, err)
	}
	return f.Close()
}

// stageFile copies one manifested file, re-hashing it as it goes, and puts the
// copy at the mode the manifest declares.
func stageFile(srcPath, dstPath, rel string, rec FileRecord, perm fs.FileMode) error {
	src, _, err := openPayloadFile(srcPath, rel)
	if err != nil {
		return err
	}
	defer src.Close()

	// O_EXCL and O_NOFOLLOW on a path inside a directory this process created
	// moments ago: nothing should pre-exist there and nothing should have
	// planted a link there. The flags are what make that an enforced property
	// rather than an assumed one — if the assumption ever stops holding,
	// staging fails instead of writing through whatever is in the way.
	dst, err := os.OpenFile(dstPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL|unix.O_NOFOLLOW, perm)
	if err != nil {
		return fmt.Errorf("creating the staged file %q: %v", rel, err)
	}
	defer dst.Close()

	// Hash what is written, not what was read earlier: this is the only thing
	// standing between a source mutated after verification and a staged tree
	// built from it. The exact re-verification afterwards would catch it too,
	// but only after the bad bytes had been written to disk.
	digest := sha256.New()
	if _, err := io.Copy(io.MultiWriter(dst, digest), src); err != nil {
		return fmt.Errorf("copying payload file %q: %v", rel, err)
	}
	if got := hex.EncodeToString(digest.Sum(nil)); got != rec.SHA256 {
		return fmt.Errorf(
			"payload file %q has sha256 %s but the payload manifest declares %s; it changed between verification and staging",
			rel, got, rec.SHA256)
	}

	// os.OpenFile's mode argument is masked by the process umask, so under the
	// umask 0077 that plenty of Nav machines run, the file just created at
	// "0644" is actually 0600 — and the exact re-verification would reject the
	// tree. Chmod is not subject to the umask, so this is what makes the
	// declared mode the mode. It goes through the descriptor rather than the
	// path so it cannot land on something else.
	if err := dst.Chmod(perm); err != nil {
		return fmt.Errorf("setting the mode of the staged file %q: %v", rel, err)
	}
	return dst.Close()
}
