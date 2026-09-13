package cli

import (
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
)

// A revision in use.
//
// The prune keeps the pinned revision and the one it replaced, which lets a
// session survive one update and not two (#784): a launch hands the client a
// directory it reads from for the whole session — OPENCODE_CONFIG_DIR, cplt's
// --allow-read, copilot's --plugin-dir — and the second update removes that
// directory under it.
//
// Keeping a third revision would only move the same failure one update further
// out, so what is kept is decided by evidence instead. nav-pilot launches the
// client as a child and waits for it, so the process holding a revision open
// is a nav-pilot process that exists for exactly as long as the session.
//
// The evidence is a file that process holds an exclusive [syscall.Flock] on.
// The kernel drops that lock when the process does — on exit, on a kill -9, on
// a reboot — so a marker a prune can lock is, by construction, one whose
// session is over. Nothing has to be believed about a recorded pid: a recycled
// process number cannot inherit a lock, and no `ps` has to answer for the
// evidence to be readable, so there is no "cannot tell" case to resolve in one
// direction or the other.
//
// Two locks, never nested the other way round: a session holds its own marker
// for the whole session, and both publishing a marker and pruning take the
// source directory's lock for the moment they need the set of markers to hold
// still. A prune therefore never reads a set of markers that a publication is
// in the middle of changing. Marker locks are only ever taken non-blocking, so
// the two cannot deadlock.
//
// ponytail: flock, which is advisory and local. A home directory on NFS gets
// no guarantee from it; ~/.nav-pilot is local for every user nav-pilot
// supports, and the upgrade path if that stops being true is a lock protocol
// that does not rely on the kernel — O_EXCL plus a liveness probe, which is
// exactly the pid record this replaced.

const (
	// revisionHoldDir sits beside the revisions of one source and holds one
	// marker per session reading them. It is bookkeeping, not a revision:
	// [isRevisionName] is what keeps every reader of the source directory from
	// treating it as one.
	revisionHoldDir = ".i-bruk"
	// holdSuffix names a marker file. Anything else in the directory is
	// ignored rather than trusted.
	holdSuffix = ".hold"
)

// currentHold is this process's claim, if it has one. A process launches one
// client from one revision, so there is exactly one — and a launch that
// re-reads the pin after an update replaces it rather than adding to it, so a
// session cannot hold two trees at once.
var currentHold struct {
	mu   sync.Mutex
	file *os.File // holding the flock; closing it is what releases the claim
	path string
}

// lockSource takes the lock that orders marker publication against pruning,
// and reports whether it got it.
//
// It locks the source directory itself, so there is no lock file to create,
// exclude from the revision listing, or clean up. Blocking: the critical
// sections are a directory listing plus a small write, or a listing plus the
// removals, and a launch waiting a moment on a prune is better than a launch
// racing one.
//
// A caller that does not get the lock must not delete anything. Publishing
// without it is still worth doing — an unsynchronized marker is what this
// guard had before the lock, not something worse.
func lockSource(repo string) (unlock func(), ok bool) {
	f, err := os.Open(pakkeSourceDir(repo))
	if err != nil {
		return func() {}, false
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		f.Close()
		return func() {}, false
	}
	return func() {
		_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		f.Close()
	}, true
}

// isRevisionName reports whether an entry under a source directory is a
// published revision, as opposed to nav-pilot's own bookkeeping beside them: a
// staging tree, or the hold directory.
func isRevisionName(name string) bool {
	return name != revisionHoldDir && !hasRevisionTmpPrefix(name)
}

// holdRevision publishes this process's claim on one revision directory of one
// source, replacing whatever it claimed before.
//
// revision is the directory's own name, not the SHA: for a local source those
// are two different things, because its tree is rebuilt from the working tree
// on every launch and two launches at the same HEAD are two different trees
// ([materializeRevision]). The name is what the prune deletes by, so it is the
// only identity a claim can be made in.
//
// Call it the moment a revision is named — not when the client starts. A prune
// in another process only has to run between the choice and the marker to take
// the tree this launch just chose, and everything between those two points
// (verifying the tree, the release prompt, the client's own startup) is time
// this launch does not control.
//
// Best effort, like the prune it talks to: a marker that cannot be written
// must not fail a launch that is otherwise ready to start.
func holdRevision(repo, revision string) {
	dir := filepath.Join(pakkeSourceDir(repo), revisionHoldDir)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return
	}
	unlock, _ := lockSource(repo)
	defer unlock()

	currentHold.mu.Lock()
	defer currentHold.mu.Unlock()
	releaseHeld()

	path := filepath.Join(dir, strconv.Itoa(os.Getpid())+holdSuffix)
	// O_TRUNC because a process number is reused: what may be at this path is
	// the marker of a long-dead session that happened to have this pid.
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0o600)
	if err != nil {
		return
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		// Something else holds this exact path, which can only be another
		// live process with this pid — impossible — or a lock this process
		// already holds on a second descriptor. Either way, do not claim it.
		f.Close()
		return
	}
	if _, err := f.WriteString(revision); err != nil {
		f.Close()
		os.Remove(path)
		return
	}
	currentHold.file, currentHold.path = f, path
}

// releaseRevision drops this process's claim. A session that never reaches it
// — a crash, a kill -9, a reboot — leaves the file behind, and the kernel
// drops the lock on it anyway; the next prune finds it unlocked and removes it.
func releaseRevision() {
	currentHold.mu.Lock()
	defer currentHold.mu.Unlock()
	releaseHeld()
}

// releaseHeld is releaseRevision's body, for callers already holding the mutex.
func releaseHeld() {
	if currentHold.file == nil {
		return
	}
	os.Remove(currentHold.path)
	currentHold.file.Close() // releases the flock
	currentHold.file, currentHold.path = nil, ""
}

// heldRevisions names the revision directories of repo that a live session is
// reading, and drops the markers of sessions that are not.
//
// The bool is whether that set is known at all. False means something in the
// hold directory could not be read, and a caller that deletes revisions must
// then delete none of them: "no markers could be read" and "no sessions are
// running" look the same from here and mean opposite things. The directory
// simply not existing is not that case — it is a source nothing has ever held.
//
// The caller is expected to hold [lockSource], so that the set this returns is
// still the set when it acts on it.
func heldRevisions(repo string) ([]string, bool) {
	dir := filepath.Join(pakkeSourceDir(repo), revisionHoldDir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, os.IsNotExist(err)
	}
	var held []string
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), holdSuffix) {
			continue
		}
		path := filepath.Join(dir, e.Name())
		f, err := os.OpenFile(path, os.O_RDWR, 0)
		if err != nil {
			if os.IsNotExist(err) {
				continue // released while this loop ran
			}
			return nil, false
		}
		if syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB) == nil {
			// Nothing holds it, so no session does: the kernel would not have
			// let go otherwise.
			_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
			f.Close()
			os.Remove(path)
			continue
		}
		data, err := io.ReadAll(f)
		f.Close()
		name := strings.TrimSpace(string(data))
		if err != nil || name == "" {
			// Held by a live session naming a revision this cannot read. The
			// tree it is reading is unknown, so nothing may be deleted.
			return nil, false
		}
		held = append(held, name)
	}
	return held, true
}
