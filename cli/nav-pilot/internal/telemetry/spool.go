package telemetry

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// No command waits for telemetry. At exit the last export is written to a
// file in ~/.nav-pilot/telemetry-spool instead of sent, and the next nav-pilot
// sends it from a goroutine that exit does not wait for. A file that is not
// sent in time stays for the run after that.
//
// Every instrument is cumulative (temporalityFor), so the last export of a
// process holds all it recorded: one file per process, overwritten, is
// enough, and an export cut short at exit loses nothing. It also makes a file
// sent twice (two runs at once, or an exit in the middle of a send) harmless:
// the same points again.
const (
	spoolMaxAge   = 7 * 24 * time.Hour
	spoolMaxFiles = 50
	spoolMaxBytes = 1 << 20 // one export is a few kB
)

// spoolDir is where exports wait to be sent; "" when there is no home.
func spoolDir() string {
	d, err := GetConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(d, "telemetry-spool")
}

// spoolTransport sends the exports while nav-pilot runs. Once exit is called
// it writes them to the spool instead, and ends the one in flight.
type spoolTransport struct {
	next    http.RoundTripper
	file    string
	exiting context.Context
	exit    context.CancelFunc
}

func newSpoolTransport(next http.RoundTripper, dir string) *spoolTransport {
	ctx, cancel := context.WithCancel(context.Background())
	file := ""
	if dir != "" {
		file = filepath.Join(dir, fmt.Sprintf("%d-%d.pb", time.Now().UnixNano(), os.Getpid()))
	}
	return &spoolTransport{next: next, file: file, exiting: ctx, exit: cancel}
}

func (s *spoolTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if s.exiting.Err() != nil {
		body, err := io.ReadAll(req.Body)
		req.Body.Close()
		if err == nil {
			s.write(body)
		}
		return accepted(req), nil
	}
	ctx, cancel := context.WithCancel(req.Context())
	stop := context.AfterFunc(s.exiting, cancel)
	resp, err := s.next.RoundTrip(req.WithContext(ctx))
	if err != nil {
		stop()
		cancel()
		if s.exiting.Err() != nil {
			// Cut short by exit. The export at exit, which goes to the
			// spool, holds everything this one did.
			return accepted(req), nil
		}
		return nil, err
	}
	resp.Body = &closeFunc{resp.Body, func() { stop(); cancel() }}
	return resp, nil
}

// write replaces this process's spool file, atomically.
func (s *spoolTransport) write(body []byte) {
	if s.file == "" || len(body) == 0 || len(body) > spoolMaxBytes {
		return
	}
	dir := filepath.Dir(s.file)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return
	}
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return
	}
	_, werr := tmp.Write(body)
	if cerr := tmp.Close(); werr != nil || cerr != nil || os.Rename(tmp.Name(), s.file) != nil {
		os.Remove(tmp.Name())
	}
}

func accepted(req *http.Request) *http.Response {
	return &http.Response{
		Status: "200 OK", StatusCode: http.StatusOK,
		Proto: "HTTP/1.1", ProtoMajor: 1, ProtoMinor: 1,
		Header: http.Header{}, Body: http.NoBody, Request: req,
	}
}

type closeFunc struct {
	io.ReadCloser
	done func()
}

func (c *closeFunc) Close() error {
	c.done()
	return c.ReadCloser.Close()
}

// sendSpool sends what earlier runs left in dir, oldest first, and removes
// each file the collector took. It stops at the first network error: the
// rest waits for the next run. Files past spoolMaxAge, and the oldest beyond
// spoolMaxFiles, are removed unsent.
func sendSpool(dir, endpoint string, client *http.Client) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	pb := 0
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".pb") {
			pb++
		}
	}
	for _, e := range entries {
		p := filepath.Join(dir, e.Name())
		info, err := e.Info()
		if err != nil {
			continue
		}
		isPB := strings.HasSuffix(e.Name(), ".pb")
		if time.Since(info.ModTime()) > spoolMaxAge || (isPB && pb > spoolMaxFiles) {
			os.Remove(p)
			if isPB {
				pb--
			}
			continue
		}
		if !isPB {
			continue
		}
		body, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
		if err != nil {
			return
		}
		// ponytail: body only, no headers, so OTEL_EXPORTER_OTLP_HEADERS or
		// _COMPRESSION would not be repeated; nav-pilot sets neither.
		req.Header.Set("Content-Type", "application/x-protobuf")
		resp, err := client.Do(req)
		if err != nil {
			return
		}
		resp.Body.Close()
		// A 4xx other than 429 will not be taken on a second try either.
		if resp.StatusCode < 300 || (resp.StatusCode < 500 && resp.StatusCode != http.StatusTooManyRequests) {
			os.Remove(p)
		}
	}
}
