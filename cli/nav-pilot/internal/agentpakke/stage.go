package agentpakke

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

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
const (
	// stagedDirPrefix names every staged tree so a leftover one found in the
	// staged root is obviously nav-pilot's and obviously disposable. It is also
	// what [GCStaged] and [CleanupStaged] match on, so neither can be talked
	// into deleting a directory nav-pilot did not create.
	stagedDirPrefix = "nav-pilot-staged-"

	// stagedDirMode is the mode of the staged root, every staged tree and every
	// directory inside one. The payload manifest declares modes for files only,
	// so nav-pilot picks this one: owner-only, because a staged tree is a
	// private projection for one process and no other user has business
	// traversing it. Consequently [VerifyPayloadExact] does not check directory
	// modes — there is nothing to check them against.
	//
	// A umask can only clear bits, so under any sane umask these directories
	// come out at 0700; under a umask that clears the owner bits the very next
	// create inside them fails and staging aborts, which is the correct outcome
	// for a machine configured that way.
	stagedDirMode fs.FileMode = 0o700
)

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
// at manifestPath, copies it into a fresh private directory under stagedRoot
// with the manifest's exact modes, and verifies that copy again, exactly. It
// returns the staged directory, which the caller owns and must remove with
// [CleanupStaged] once the client it was staged for has exited.
//
// stagedRoot is created if absent. Each call stages into its own [os.MkdirTemp]
// directory: two concurrent calls, even for the same source, produce two
// independent trees and share nothing but the root they sit in. A fixed
// per-pakke directory would be wrong here — wiping and rewriting it would pull
// the config directory out from under a session already running against it.
//
// It fails closed. Any error at any step leaves no staged tree behind and no
// usable result: there is no partially-staged tree and no fallback to an
// unverified one. A caller that gets an error must abort the launch.
func StagePayload(payloadDir, manifestPath, stagedRoot string) (stagedDir string, err error) {
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

	if err := os.MkdirAll(stagedRoot, stagedDirMode); err != nil {
		return "", fmt.Errorf("creating the staged-payload root %s: %v", stagedRoot, err)
	}
	stagedDir, err = os.MkdirTemp(stagedRoot, stagedDirPrefix)
	if err != nil {
		return "", fmt.Errorf("creating a staged payload directory under %s: %v", stagedRoot, err)
	}

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
	// moments ago and no one else knows the name of: nothing can pre-exist
	// there and nothing can have planted a link there. The flags are what make
	// that an enforced property rather than an assumed one — if the assumption
	// ever stops holding, staging fails instead of writing through whatever is
	// in the way.
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

// CleanupStaged removes a staged tree. It is what the caller defers around the
// client process: the tree is a disposable projection of the pakke repo, so
// there is nothing to keep once the client that was reading it has exited.
//
// A tree that is already gone is not an error — the caller should be able to
// defer this unconditionally. A path that is not a staged tree is: the argument
// is fed to [os.RemoveAll], and a caller that passes an empty or wrong path
// deserves an error rather than a recursive delete of whatever it named.
func CleanupStaged(stagedDir string) error {
	if !isStagedDir(stagedDir) {
		return fmt.Errorf(
			"refusing to remove %q: a staged payload directory is named %s* and nav-pilot will not recursively delete anything else",
			stagedDir, stagedDirPrefix)
	}
	if err := os.RemoveAll(stagedDir); err != nil {
		return fmt.Errorf("removing the staged payload %s: %v", stagedDir, err)
	}
	return nil
}

// GCStaged removes staged trees under stagedRoot that have not been modified
// for maxAge, and is how a tree survives a crash by at most that long: the
// happy path is [CleanupStaged] when the client exits, and this is the sweep
// for the times there was no happy path.
//
// The age rule is deliberately naive. A tree's mtime is set when staging writes
// the last file into it and nothing touches it afterwards, so "older than
// maxAge" means "staged more than maxAge ago" — not "unused". With maxAge at
// 24h that guarantees a leaked tree is collected within a day of the crash that
// leaked it, and it guarantees nothing at all about a session still running
// after maxAge: that session's tree is swept out from under it, and its client
// starts failing to read its own config. It is a visible failure rather than an
// unsafe one, and a day-long interactive session is not a case worth carrying
// pid-liveness machinery for.
//
// ponytail: mtime heuristic; swap in an owner-pid file per tree if sessions
// outliving maxAge turn out to be real.
//
// A missing stagedRoot is not an error — nothing has been staged yet. Trees
// that cannot be removed are reported together rather than aborting the sweep,
// so one stuck tree does not shield the rest.
func GCStaged(stagedRoot string, maxAge time.Duration) error {
	entries, err := os.ReadDir(stagedRoot)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("reading the staged-payload root %s: %v", stagedRoot, err)
	}
	cutoff := time.Now().Add(-maxAge)
	var errs []error
	for _, e := range entries {
		// Only our own trees: the staged root is nav-pilot's, but a sweep that
		// deletes by age alone is one misconfigured path away from deleting a
		// user's directory.
		if !isStagedDir(e.Name()) {
			continue
		}
		info, err := e.Info()
		if errors.Is(err, fs.ErrNotExist) {
			continue // another process swept it first
		}
		if err != nil {
			errs = append(errs, fmt.Errorf("inspecting the staged payload %s: %v", e.Name(), err))
			continue
		}
		if !info.ModTime().Before(cutoff) {
			continue
		}
		if err := os.RemoveAll(filepath.Join(stagedRoot, e.Name())); err != nil {
			errs = append(errs, fmt.Errorf("removing the stale staged payload %s: %v", e.Name(), err))
		}
	}
	return errors.Join(errs...)
}

// isStagedDir reports whether p names a directory [StagePayload] created.
func isStagedDir(p string) bool {
	base := filepath.Base(p)
	return strings.HasPrefix(base, stagedDirPrefix) && len(base) > len(stagedDirPrefix)
}
