package local

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// What the local model has actually done on this machine, kept locally and
// sent nowhere.
//
// The telemetry counts dispatches per session because that is what tells us
// whether the feature earns its place across 650 developers. It is no use to
// the developer in front of the machine, who wants to know what their own
// model has been doing: how many requests it has taken, how many tokens it
// generated for free, and how long it has spent doing it.
//
// One line per completion, appended. Append-only because two nav-pilot
// sessions share one server and therefore write here concurrently: an
// O_APPEND write of a line this short lands whole, where a read-modify-write
// of a counter file would lose one of the two.

type statLine struct {
	At      time.Time `json:"at"`
	In      int64     `json:"in"`
	Out     int64     `json:"out"`
	Seconds float64   `json:"s"`
}

// Stats is the sum of everything recorded, for `alpha local status`.
type Stats struct {
	Requests int64
	TokensIn int64
	// Tokens the model generated. This is the number that would have been
	// billed had the work gone to the cloud, which is the point of it.
	TokensOut int64
	Seconds   float64
	// Requests whose response carried no usage block. Counted separately so a
	// small token total is never mistaken for a small amount of work: a
	// streaming client that does not ask for usage reports none.
	WithoutUsage int64
	Since        time.Time
}

func statsPath() string { return filepath.Join(filepath.Dir(statePath()), "stats.jsonl") }

// RecordCompletion appends one completion. Errors are dropped on purpose: a
// full disk or a read-only home must not fail the developer's actual work for
// the sake of a counter.
func RecordCompletion(in, out int64, seconds float64) {
	line, err := json.Marshal(statLine{At: time.Now(), In: in, Out: out, Seconds: seconds})
	if err != nil {
		return
	}
	f, err := openAppend(statsPath())
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.Write(append(line, '\n'))
}

// ReadStats sums the file. A malformed line is skipped rather than fatal,
// because a half-written line from a killed process should cost one
// completion in the total and nothing else.
func ReadStats() (Stats, error) {
	var s Stats
	f, err := os.Open(statsPath())
	if os.IsNotExist(err) {
		return s, nil
	}
	if err != nil {
		return s, fmt.Errorf("reading local stats: %w", err)
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		var l statLine
		if json.Unmarshal(sc.Bytes(), &l) != nil {
			continue
		}
		s.Requests++
		s.TokensIn += l.In
		s.TokensOut += l.Out
		s.Seconds += l.Seconds
		if l.In == 0 && l.Out == 0 {
			s.WithoutUsage++
		}
		if s.Since.IsZero() || l.At.Before(s.Since) {
			s.Since = l.At
		}
	}
	return s, sc.Err()
}

// ResetStats drops the file, for `purge` and for a developer who wants the
// count to start again.
func ResetStats() error {
	if err := os.Remove(statsPath()); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing local stats: %w", err)
	}
	return nil
}

// openAppend is the one write mode this file may ever be opened in. O_APPEND is
// what makes a concurrent write from a second session safe: the kernel places
// each short line at the current end of file, where a seek-then-write would let
// two sessions land on the same offset and lose one.
func openAppend(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
}
