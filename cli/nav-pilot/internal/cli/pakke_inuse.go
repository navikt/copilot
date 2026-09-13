package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"

	"github.com/navikt/copilot/cli/nav-pilot/internal/local"
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
// client as a child and waits for it, so the process holding a revision open is
// a nav-pilot process that exists for exactly as long as the session: a marker
// naming its pid is the honest answer to "is this in use", and it is the only
// one available to a prune running in a *different* nav-pilot process, which is
// every prune there is.
//
// The identity is pid plus the kernel's start time for that pid, the same pair
// [local.EnsureOwnServer] proves a recorded server with. A pid alone would let
// a recycled number pin a tree that nothing is reading, forever.

// revisionHoldDir sits beside the revisions of one source and holds one marker
// per session reading them. It is bookkeeping, not a revision: [isRevisionName]
// is what keeps every reader of the source directory from treating it as one.
const revisionHoldDir = ".i-bruk"

// revisionHold is one session's claim on one revision.
type revisionHold struct {
	SHA    string `json:"sha"`
	PID    int    `json:"pid"`
	Lstart string `json:"lstart"`
}

// isRevisionName reports whether an entry under a source directory is a
// published revision, as opposed to nav-pilot's own bookkeeping beside them: a
// staging tree, or the hold directory.
func isRevisionName(name string) bool {
	return name != revisionHoldDir && !hasRevisionTmpPrefix(name)
}

// holdRevision records that this process is reading repo's revision sha, and
// returns the release. Call it where the revision is handed to the client and
// defer the release, so the marker lives exactly as long as the session does.
//
// Best effort, like the prune it talks to: a marker that cannot be written must
// not fail a launch that is otherwise ready to start. The cost of failing to
// write one is the behaviour before #784, not a worse one.
func holdRevision(repo, sha string) func() {
	dir := filepath.Join(pakkeSourceDir(repo), revisionHoldDir)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return func() {}
	}
	pid := os.Getpid()
	data, err := json.Marshal(revisionHold{SHA: sha, PID: pid, Lstart: local.ProcessStart(pid)})
	if err != nil {
		return func() {}
	}
	path := filepath.Join(dir, strconv.Itoa(pid)+".json")
	// Published with a rename so a prune reading the directory sees either no
	// marker or a whole one. Writing in place would let it read a truncated
	// file, and it cannot tell that apart from a corrupt one — which it
	// deletes.
	tmp, err := os.CreateTemp(dir, revisionTmpPrefix)
	if err != nil {
		return func() {}
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return func() {}
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return func() {}
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		os.Remove(tmp.Name())
		return func() {}
	}
	return func() { os.Remove(path) }
}

// heldRevisions names the revisions of repo that a live session is reading, and
// drops the markers of sessions that are not.
//
// A session that dies without releasing — a crash, a kill -9, a reboot — leaves
// a marker behind, and the next prune is what clears it: the recorded process
// is gone, so the marker keeps nothing and is removed on the spot. Nothing
// expires on age, and nothing needs a nav-pilot to be running to be cleaned up.
func heldRevisions(repo string) []string {
	dir := filepath.Join(pakkeSourceDir(repo), revisionHoldDir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var held []string
	for _, e := range entries {
		path := filepath.Join(dir, e.Name())
		if hasRevisionTmpPrefix(e.Name()) {
			// A marker mid-publish. It is not readable as a hold yet, and the
			// rename that is about to land is what makes it one. Leaving it is
			// safe: its session is younger than this prune, so the revision it
			// is about to claim was materialized after this prune read the
			// directory. A process killed between the write and the rename
			// leaks one of these — the same bounded leak a killed
			// materialization leaks a .tmp-* staging tree, at a hundred bytes
			// instead of a tree, and swept by the same nothing.
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			// Could not read it, which is not the same as knowing it is dead.
			// Leaving it costs one tree until the next prune manages to read
			// it; removing it on a permission blip costs a running session.
			continue
		}
		var h revisionHold
		if err := json.Unmarshal(data, &h); err != nil || !local.PIDHolds(h.PID, h.Lstart) {
			_ = os.Remove(path)
			continue
		}
		held = append(held, h.SHA)
	}
	return held
}
