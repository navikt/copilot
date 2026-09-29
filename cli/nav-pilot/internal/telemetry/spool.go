package telemetry

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	colmetricpb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	metricpb "go.opentelemetry.io/proto/otlp/metrics/v1"
	"google.golang.org/protobuf/proto"
)

// No command waits for telemetry. At exit the last export is written to a
// file in ~/.nav-pilot/telemetry-spool instead of sent. A detached child,
// `nav-pilot __telemetry-send`, sends it right after exit (cli's
// spawnTelemetrySender), and the next nav-pilot sends whatever is left from a
// goroutine that exit does not wait for. A file that is not sent in time
// stays for the run after that.
//
// A file is sent with its points re-stamped to the time of sending: Mimir
// drops samples older than its out-of-order window (30-60 min), so a file
// sent the next morning with its own timestamps would be lost.
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

	// spoolSendTimeout bounds one sender, the child or the next run's
	// goroutine; spoolLockStale is when its lock counts as left by a sender
	// that died.
	spoolSendTimeout = 15 * time.Second
	spoolLockStale   = 60 * time.Second
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
	wrote   atomic.Bool // an export went to the spool
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
		return
	}
	s.wrote.Store(true)
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

// SendSpool is `nav-pilot __telemetry-send`: it sends the spool once, within
// spoolSendTimeout, and returns. What it cannot send stays for the next run.
func SendSpool() {
	if !TelemetryEnabled() {
		return
	}
	if dir := spoolDir(); dir != "" {
		ctx, cancel := context.WithTimeout(context.Background(), spoolSendTimeout)
		defer cancel()
		sendSpoolLocked(ctx, dir, telemetryEndpoint(), &http.Client{Transport: &http.Transport{Proxy: http.ProxyFromEnvironment}})
	}
}

// sendSpoolLocked is sendSpool under the spool's lock, so the child and the
// next run do not send the same files at once. It does nothing while another
// sender holds the lock, unless that lock is older than spoolLockStale.
func sendSpoolLocked(ctx context.Context, dir, endpoint string, client *http.Client) {
	lock := filepath.Join(dir, ".send.lock")
	for range 2 {
		f, err := os.OpenFile(lock, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err == nil {
			f.Close()
			defer os.Remove(lock)
			sendSpool(ctx, dir, endpoint, client)
			return
		}
		info, serr := os.Stat(lock)
		if !errors.Is(err, fs.ErrExist) || serr != nil || time.Since(info.ModTime()) < spoolLockStale {
			return
		}
		os.Remove(lock)
	}
}

// sendSpool sends what earlier runs left in dir, oldest first, and removes
// each file the collector took. It stops at the first network error: the
// rest waits for the next run. Files past spoolMaxAge, and the oldest beyond
// spoolMaxFiles, are removed unsent.
func sendSpool(ctx context.Context, dir, endpoint string, client *http.Client) {
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
		body = restamp(body, uint64(time.Now().UnixNano()))
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
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

// restamp sets every point in an OTLP metrics export to now. The start times
// stay, so a cumulative series still reads from where its process began. A
// body that does not parse is sent as it is.
func restamp(body []byte, now uint64) []byte {
	var req colmetricpb.ExportMetricsServiceRequest
	if proto.Unmarshal(body, &req) != nil {
		return body
	}
	for _, rm := range req.ResourceMetrics {
		for _, sm := range rm.ScopeMetrics {
			for _, m := range sm.Metrics {
				switch d := m.Data.(type) {
				case *metricpb.Metric_Sum:
					for _, p := range d.Sum.DataPoints {
						p.TimeUnixNano = now
					}
				case *metricpb.Metric_Gauge:
					for _, p := range d.Gauge.DataPoints {
						p.TimeUnixNano = now
					}
				case *metricpb.Metric_Histogram:
					for _, p := range d.Histogram.DataPoints {
						p.TimeUnixNano = now
					}
				case *metricpb.Metric_ExponentialHistogram:
					for _, p := range d.ExponentialHistogram.DataPoints {
						p.TimeUnixNano = now
					}
				case *metricpb.Metric_Summary:
					for _, p := range d.Summary.DataPoints {
						p.TimeUnixNano = now
					}
				}
			}
		}
	}
	out, err := proto.Marshal(&req)
	if err != nil {
		return body
	}
	return out
}
