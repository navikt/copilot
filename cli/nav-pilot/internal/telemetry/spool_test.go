package telemetry

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	colmetricpb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	metricpb "go.opentelemetry.io/proto/otlp/metrics/v1"
	"google.golang.org/protobuf/proto"
)

// At exit the export goes to the spool without the network, even with one
// stuck in flight, which fails so its counts go with the export at exit; the
// next run sends the spool and removes it.
func TestSpoolRoundTrip(t *testing.T) {
	dir := t.TempDir()
	stuck := make(chan struct{})
	hole := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.ReadAll(r.Body) // so the server sees the client hang up
		select {
		case <-stuck:
		case <-r.Context().Done():
		}
	}))
	defer hole.Close()
	defer close(stuck)

	s := newSpoolTransport(http.DefaultTransport, dir)
	client := &http.Client{Transport: s}
	inFlight := make(chan error)
	go func() {
		resp, err := client.Post(hole.URL, "application/x-protobuf", bytes.NewReader([]byte("early")))
		if err == nil {
			resp.Body.Close()
		}
		inFlight <- err
	}()
	time.Sleep(50 * time.Millisecond)

	start := time.Now()
	s.exit()
	if err := <-inFlight; err == nil {
		t.Fatalf("export in flight at exit reported success; its counts would be lost")
	}
	resp, err := client.Post(hole.URL, "application/x-protobuf", bytes.NewReader([]byte("last")))
	if err != nil || resp.StatusCode != 200 {
		t.Fatalf("export at exit: %v %v", resp, err)
	}
	if took := time.Since(start); took > 200*time.Millisecond {
		t.Errorf("exit took %s", took)
	}
	if files, _ := filepath.Glob(filepath.Join(dir, "*.pb")); len(files) != 1 {
		t.Fatalf("spool = %v, want the export at exit only", files)
	} else if b, _ := os.ReadFile(files[0]); string(b) != "last" {
		t.Fatalf("spool = %q, want the export at exit", b)
	}
	// Each export holds only what is new, so a second one after exit is a
	// file of its own, not a replacement.
	s.write([]byte("later"))
	if files, _ := filepath.Glob(filepath.Join(dir, "*.pb")); len(files) != 2 {
		t.Fatalf("spool = %v, want both exports", files)
	}

	old := filepath.Join(dir, "1-1.pb")
	os.WriteFile(old, []byte("old"), 0o600)
	week := time.Now().Add(-8 * 24 * time.Hour)
	os.Chtimes(old, week, week)

	var got []string
	collector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		got = append(got, string(b))
	}))
	defer collector.Close()
	sendSpool(t.Context(), dir, collector.URL, collector.Client())
	if len(got) != 2 || got[0] != "last" || got[1] != "later" {
		t.Errorf("sent %q, want the two exports after exit, in order", got)
	}
	if left, _ := os.ReadDir(dir); len(left) != 0 {
		t.Errorf("spool not emptied: %v", left)
	}
}

// Turned off means nothing waiting in the spool is sent either.
func TestSpoolRemovedWhenOff(t *testing.T) {
	t.Setenv("NAV_PILOT_CONFIG", filepath.Join(t.TempDir(), "config.toml"))
	d := spoolDir()
	os.MkdirAll(d, 0o700)
	os.WriteFile(filepath.Join(d, "1-1.pb"), []byte("x"), 0o600)
	t.Setenv("DO_NOT_TRACK", "1")
	if _, err := InitTelemetry(t.Context(), "dev", "false"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(d); !os.IsNotExist(err) {
		t.Errorf("spool still there with DO_NOT_TRACK: %v", err)
	}
}

// A spooled export is sent with its points at the time of sending, or Mimir
// drops it as out of order; a sender that holds the lock keeps others off
// until the holder closes its descriptor.
func TestSpoolRestampAndLock(t *testing.T) {
	dir := t.TempDir()
	hourAgo := uint64(time.Now().Add(-time.Hour).UnixNano())
	body, err := proto.Marshal(&colmetricpb.ExportMetricsServiceRequest{ResourceMetrics: []*metricpb.ResourceMetrics{{
		ScopeMetrics: []*metricpb.ScopeMetrics{{Metrics: []*metricpb.Metric{{
			Name: "c",
			Data: &metricpb.Metric_Sum{Sum: &metricpb.Sum{DataPoints: []*metricpb.NumberDataPoint{{
				StartTimeUnixNano: hourAgo, TimeUnixNano: hourAgo, Value: &metricpb.NumberDataPoint_AsInt{AsInt: 3},
			}}}},
		}}}},
	}}})
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(dir, "1-1.pb"), body, 0o600)

	var got [][]byte
	collector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		got = append(got, b)
	}))
	defer collector.Close()

	held, err := os.OpenFile(filepath.Join(dir, ".send.lock"), os.O_CREATE|os.O_RDONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if err := syscall.Flock(int(held.Fd()), syscall.LOCK_EX); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 300*time.Millisecond)
	sendSpoolLocked(ctx, dir, collector.URL, collector.Client())
	cancel()
	if len(got) != 0 {
		t.Fatalf("sent while another sender held the lock")
	}
	// The holder exits (its descriptor closes): the waiting sender goes on.
	time.AfterFunc(200*time.Millisecond, func() { held.Close() })
	before := uint64(time.Now().UnixNano())
	sendSpoolLocked(t.Context(), dir, collector.URL, collector.Client())
	if len(got) != 1 {
		t.Fatalf("sent %d, want 1 once the lock was released", len(got))
	}
	var req colmetricpb.ExportMetricsServiceRequest
	if err := proto.Unmarshal(got[0], &req); err != nil {
		t.Fatal(err)
	}
	p := req.ResourceMetrics[0].ScopeMetrics[0].Metrics[0].GetSum().DataPoints[0]
	if p.TimeUnixNano < before || p.StartTimeUnixNano != hourAgo || p.GetAsInt() != 3 {
		t.Errorf("point = %v, want time re-stamped to now and the rest kept", p)
	}
	if left, _ := filepath.Glob(filepath.Join(dir, "*.pb")); len(left) != 0 {
		t.Errorf("spool left behind: %v", left)
	}
}

// The age prune leaves the lock file alone, however old it is.
func TestSpoolPruneKeepsLock(t *testing.T) {
	dir := t.TempDir()
	lock := filepath.Join(dir, ".send.lock")
	os.WriteFile(lock, nil, 0o600)
	week := time.Now().Add(-8 * 24 * time.Hour)
	os.Chtimes(lock, week, week)
	sendSpool(t.Context(), dir, "http://127.0.0.1:0", http.DefaultClient)
	if _, err := os.Stat(lock); err != nil {
		t.Errorf("lock file pruned: %v", err)
	}
}

// A filesystem where flock fails outright sends at once, unlocked, instead
// of waiting out the context as if the lock were held.
func TestSpoolSendsWhenFlockUnsupported(t *testing.T) {
	orig := flock
	flock = func(int, int) error { return syscall.ENOLCK }
	defer func() { flock = orig }()

	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "1-1.pb"), []byte("x"), 0o600)
	sent := 0
	collector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { sent++ }))
	defer collector.Close()

	start := time.Now()
	sendSpoolLocked(t.Context(), dir, collector.URL, collector.Client())
	if took := time.Since(start); took > time.Second {
		t.Errorf("took %s, want no wait", took)
	}
	if sent != 1 {
		t.Errorf("sent %d, want 1", sent)
	}
}

// A file another sender has claimed is not sent again, and a claim is given
// back when the send fails, so the next run still has the file.
func TestSpoolClaim(t *testing.T) {
	dir := t.TempDir()
	mine := filepath.Join(dir, "1-1.pb")
	theirs := filepath.Join(dir, "2-1.pb")
	os.WriteFile(mine, []byte("mine"), 0o600)
	os.WriteFile(theirs, []byte("theirs"), 0o600)
	if _, ok := claim(theirs); !ok {
		t.Fatal("claim failed")
	}
	if _, ok := claim(theirs); ok {
		t.Fatal("second claim of the same file succeeded")
	}

	var got []string
	collector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		got = append(got, string(b))
	}))
	defer collector.Close()
	sendSpool(t.Context(), dir, collector.URL, collector.Client())
	if len(got) != 1 || got[0] != "mine" {
		t.Errorf("sent %q, want only the unclaimed file", got)
	}
	if _, err := os.Stat(theirs + ".sending"); err != nil {
		t.Errorf("another sender's claim disturbed: %v", err)
	}

	os.WriteFile(mine, []byte("mine"), 0o600)
	failing := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer failing.Close()
	sendSpool(t.Context(), dir, failing.URL, failing.Client())
	if _, err := os.Stat(mine); err != nil {
		t.Errorf("claim not given back after a 503: %v", err)
	}
	failing.Close()
	sendSpool(t.Context(), dir, failing.URL, failing.Client())
	if _, err := os.Stat(mine); err != nil {
		t.Errorf("claim not given back after a network error: %v", err)
	}
}

// Exit ends the run's own send of an earlier spool and gives its claim back,
// so the child sends the file instead of it being left claimed, unsent.
func TestShutdownGivesClaimBack(t *testing.T) {
	t.Setenv("NAV_PILOT_CONFIG", filepath.Join(t.TempDir(), "config.toml"))
	t.Setenv("DO_NOT_TRACK", "")
	d := spoolDir()
	os.MkdirAll(d, 0o700)
	f := filepath.Join(d, "1-1.pb")
	os.WriteFile(f, []byte("x"), 0o600)
	arrived := make(chan struct{}, 1)
	hole := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.ReadAll(r.Body) // so the server sees the client hang up
		select {
		case arrived <- struct{}{}:
		default:
		}
		<-r.Context().Done()
	}))
	defer hole.Close()
	t.Setenv("NAV_PILOT_TELEMETRY_ENDPOINT", hole.URL)

	r, err := InitTelemetry(t.Context(), "dev", "false")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := r.(*otelTelemetry); !ok {
		t.Fatalf("telemetry off in test: %T", r)
	}
	// Bounded: a hang here lasts until go test's timeout kills the binary,
	// and a killed binary leaves its t.TempDir in $TMPDIR.
	select {
	case <-arrived:
	case <-time.After(10 * time.Second):
		t.Fatal("the earlier spool was never sent")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	start := time.Now()
	r.Shutdown(ctx)
	if took := time.Since(start); took > time.Second {
		t.Errorf("Shutdown took %s", took)
	}
	if _, err := os.Stat(f); err != nil {
		t.Errorf("claim not given back at exit: %v", err)
	}
}
