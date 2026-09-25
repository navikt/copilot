package hook

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// The hooks' telemetry goes through a spool file rather than straight to the
// collector. A hook is a fresh process per tool call, and an OTLP export from
// one is a new TCP and TLS connection: about 80 ms warm and 1.7 s cold, measured
// against the collector, on a call the CLI waits for. So a hook appends a line
// to its session's directory, the same directory the loop guard keeps its state
// in and cplt lets the session write, and the nav-pilot launch that started
// the session records them as it exits and exports anyway (a session started
// without nav-pilot waits for the next launch).
//
// A line is a metric and enum values only ("loop_guard same_result",
// "redact secret 2"). The recorder checks every value against its enum again.

const spoolName = "nav-pilot-hook-events"

// Spool appends lines to the session's spool file. Nothing to spool, or no
// session id, is a no-op.
func Spool(root, sessionID string, lines []string) error {
	id := unsafeID.ReplaceAllString(sessionID, "")
	if id == "" || len(lines) == 0 {
		return nil
	}
	dir := filepath.Join(root, id)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(dir, spoolName), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	_, err = f.WriteString(strings.Join(lines, "\n") + "\n")
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	return err
}

// DrainSpool hands every spooled line under root to fn, split into fields, and
// removes the files.
//
// ponytail: a hook appending while a drain renames loses at most that one line
// to the renamed file, which is then read. Glob over every session directory
// is a stat each, a few ms for a thousand sessions, paid once per launch.
func DrainSpool(root string, fn func(fields []string)) {
	paths, _ := filepath.Glob(filepath.Join(root, "*", spoolName))
	for _, p := range paths {
		taken := p + ".draining"
		if os.Rename(p, taken) != nil {
			continue
		}
		if f, err := os.Open(taken); err == nil {
			sc := bufio.NewScanner(f)
			for sc.Scan() {
				if fields := strings.Fields(sc.Text()); len(fields) > 0 {
					fn(fields)
				}
			}
			f.Close()
		}
		os.Remove(taken)
	}
}
