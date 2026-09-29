package telemetry

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptrace"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
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
// An export holds only what changed since the last one that went through
// (newOnlyExporter), so every export that reaches the spool gets its own
// file, and one cut short by exit reports failure: the export at exit then
// carries its counts too.
//
// Each file is claimed before it is sent (claim), so no two senders send the
// same file, even where flock fails. A sender that dies mid-send leaves its
// .sending file for the age prune: those counts are lost rather than counted
// twice; a run's own send of the spool is ended by exit and gives its claim
// back, so the child sends the file.
//
// A live export that was sent but got no answer, a timeout above all, may
// have been taken anyway. It is kept and sent again unchanged, same points and
// same timestamps, before the next export, so if the collector did take it,
// Mimir drops the second copy as a duplicate (spoolTransport.unsent). Not
// fixed at exit: one still unanswered there goes to the spool and is
// re-stamped when sent, so a copy the collector took counts twice. It takes a
// response lost after ingestion right before exit; the counts are usage
// signals, not billing.
const (
	spoolMaxAge   = 7 * 24 * time.Hour
	spoolMaxFiles = 50
	spoolMaxBytes = 1 << 20 // one export is a few kB

	// spoolSendTimeout bounds one sender, the child or the next run's
	// goroutine, waiting for the lock included.
	spoolSendTimeout = 15 * time.Second

	// unsentMax bounds the unanswered exports a run keeps to send again.
	// Beyond it an unanswered export fails as before, and its counts go
	// with the next one.
	unsentMax = 5
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
	dir     string
	exiting context.Context
	cancel  context.CancelFunc
	wrote   atomic.Bool // an export went to the spool

	mu sync.Mutex // one live send at a time, and unsent
	// unsent holds exports that were sent but not answered, oldest first.
	// Each goes again as it is before the next export, and to the spool at
	// exit.
	unsent []*http.Request
	// ready, when set, holds live sends back until the spool an earlier run
	// left has been sent: those points are re-stamped to now, and a series
	// cannot take a point older than the one it already has.
	ready <-chan struct{}
}

func newSpoolTransport(next http.RoundTripper, dir string) *spoolTransport {
	ctx, cancel := context.WithCancel(context.Background())
	return &spoolTransport{next: next, dir: dir, exiting: ctx, cancel: cancel}
}

// exit sends every export from now on to the spool, ends the one in flight,
// and writes the unanswered ones to the spool.
func (s *spoolTransport) exit() {
	s.cancel()
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, r := range s.unsent {
		s.write(bodyOf(r))
	}
	s.unsent = nil
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
	if s.ready != nil {
		select {
		case <-s.ready:
		case <-s.exiting.Done():
			return s.RoundTrip(req) // exit came first: to the spool
		case <-req.Context().Done():
			return nil, req.Context().Err()
		}
	}
	body, err := io.ReadAll(req.Body)
	req.Body.Close()
	if err != nil {
		return nil, err
	}
	req = withBody(req.Clone(req.Context()), body)

	s.mu.Lock()
	defer s.mu.Unlock()
	// The unanswered ones first, unchanged, so a copy the collector already
	// took is a duplicate Mimir drops. While one still fails, this export
	// fails too, and its counts go with the next one.
	for len(s.unsent) > 0 {
		r := withBody(s.unsent[0].Clone(req.Context()), bodyOf(s.unsent[0]))
		resp, _, err := s.send(r)
		if err != nil {
			return nil, err
		}
		resp.Body.Close()
		if resp.StatusCode >= 500 || resp.StatusCode == http.StatusTooManyRequests {
			return nil, fmt.Errorf("resending an unanswered export: %s", resp.Status)
		}
		// A 4xx other than 429 is not taken on a second try, most likely
		// points past Mimir's out-of-order window after a sleep. Its counts
		// are already counted as sent, so the spool re-stamps and sends it
		// rather than drop it.
		if resp.StatusCode >= 300 {
			s.write(bodyOf(s.unsent[0]))
		}
		s.unsent = s.unsent[1:]
	}
	resp, sent, err := s.send(req)
	if err != nil {
		// Sent and not answered: the collector may have it, so it goes again
		// as it is instead of being folded into the next export. Not sent,
		// or cut short by exit: it failed, and its counts go with the next
		// export, which at exit is the spool.
		if sent && s.exiting.Err() == nil && len(s.unsent) < unsentMax {
			// A new request, not a Clone: http.Client sets the old Cancel
			// channel for its Timeout on a transport it does not know, and a
			// clone would carry it into every resend.
			kept, _ := http.NewRequestWithContext(context.Background(), req.Method, req.URL.String(), nil)
			kept.Header = req.Header.Clone()
			s.unsent = append(s.unsent, withBody(kept, bodyOf(req)))
			return accepted(req), nil
		}
		return nil, err
	}
	return resp, nil
}

// send is one live send, ended by exit. sent reports whether the request
// went out in full, so the collector may have it even without an answer.
func (s *spoolTransport) send(req *http.Request) (resp *http.Response, sent bool, err error) {
	ctx, cancel := context.WithCancel(req.Context())
	stop := context.AfterFunc(s.exiting, cancel)
	var wrote atomic.Bool
	ctx = httptrace.WithClientTrace(ctx, &httptrace.ClientTrace{
		WroteRequest: func(i httptrace.WroteRequestInfo) { wrote.Store(i.Err == nil) },
	})
	defer stop()
	defer cancel()
	resp, err = s.next.RoundTrip(req.WithContext(ctx))
	if err != nil {
		return nil, wrote.Load(), err
	}
	// The answer is read here, within the same deadline: a body that stalls
	// after a 2xx would otherwise fail the export in the exporter, and fold
	// counts the collector took into the next export. A 2xx is taken, body
	// or not; any other answer that breaks off counts as no answer.
	b, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil && resp.StatusCode/100 != 2 {
		return nil, true, err
	}
	resp.Body = io.NopCloser(bytes.NewReader(b))
	return resp, true, nil
}

// withBody is req reading body, which it can read again (GetBody) for a
// redirect or a retry.
func withBody(req *http.Request, body []byte) *http.Request {
	req.Body = io.NopCloser(bytes.NewReader(body))
	req.GetBody = func() (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(body)), nil }
	req.ContentLength = int64(len(body))
	return req
}

// bodyOf is the body of a request made by withBody.
func bodyOf(req *http.Request) []byte {
	rc, err := req.GetBody()
	if err != nil {
		return nil
	}
	b, _ := io.ReadAll(rc)
	return b
}

// spoolName is where a temporary file lands: time first, so the spool sends
// oldest first, and the temporary file's random suffix, so two exports in the
// same nanosecond do not replace each other.
func spoolName(dir, tmp string) string {
	return filepath.Join(dir, fmt.Sprintf("%d-%d-%s.pb", time.Now().UnixNano(), os.Getpid(), strings.TrimPrefix(filepath.Base(tmp), ".tmp-")))
}

// write adds body to the spool as a file of its own, atomically.
func (s *spoolTransport) write(body []byte) {
	if s.dir == "" || len(body) == 0 || len(body) > spoolMaxBytes {
		return
	}
	dir := s.dir
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return
	}
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return
	}
	_, werr := tmp.Write(body)
	if cerr := tmp.Close(); werr != nil || cerr != nil || os.Rename(tmp.Name(), spoolName(dir, tmp.Name())) != nil {
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
// next run do not send the same files at once. It waits for a sender that
// holds the lock, within ctx, and then reads the directory afresh: a file
// written while the other one sent is not missed.
//
// The lock is flock(2) on .send.lock, held by the kernel for the open file:
// a sender that exits or is killed mid-send releases it, so there is no
// stale lock to detect and no takeover to race.
// flock is syscall.Flock, a variable so a test can fail it the way NFS does.
var flock = syscall.Flock

func sendSpoolLocked(ctx context.Context, dir, endpoint string, client *http.Client) {
	f, err := os.OpenFile(filepath.Join(dir, ".send.lock"), os.O_CREATE|os.O_RDONLY, 0o600)
	if err != nil {
		return // no spool directory: nothing to send
	}
	defer f.Close()
	for {
		err := flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		// Only a lock someone holds is worth waiting for. Any other error
		// (ENOLCK on NFS without lockd, EOPNOTSUPP on some SMB and FUSE
		// mounts) means no lock can be had here at all: send without it
		// rather than wait out ctx on every run.
		if err == nil || !(errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EINTR)) {
			break
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(100 * time.Millisecond):
		}
	}
	sendSpool(ctx, dir, endpoint, client)
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
		// The lock file's mtime never moves (flock and O_CREATE leave it),
		// so the age prune would take it from under a holder.
		if e.Name() == ".send.lock" {
			continue
		}
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
		claimed, ok := claim(p)
		if !ok {
			continue // another sender has it
		}
		body, err := os.ReadFile(claimed)
		if err != nil {
			os.Rename(claimed, p)
			continue
		}
		body = restamp(body, uint64(time.Now().UnixNano()))
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
		if err != nil {
			os.Rename(claimed, p)
			return
		}
		// ponytail: body only, no headers, so OTEL_EXPORTER_OTLP_HEADERS or
		// _COMPRESSION would not be repeated; nav-pilot sets neither.
		req.Header.Set("Content-Type", "application/x-protobuf")
		resp, err := client.Do(req)
		if err != nil {
			os.Rename(claimed, p)
			return
		}
		resp.Body.Close()
		// A 4xx other than 429 will not be taken on a second try either.
		if resp.StatusCode < 300 || (resp.StatusCode < 500 && resp.StatusCode != http.StatusTooManyRequests) {
			os.Remove(claimed)
		} else {
			os.Rename(claimed, p)
		}
	}
}

// claim renames p to p.sending, which only one sender can do: the other
// finds p gone. The .sending file no longer ends in .pb, so no sender picks
// it up again; one left by a sender that died goes with the age prune.
func claim(p string) (string, bool) {
	claimed := p + ".sending"
	return claimed, os.Rename(p, claimed) == nil
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
