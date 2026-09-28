package telemetry

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// At exit the export goes to the spool without the network, even with one
// stuck in flight; the next run sends it and removes it.
func TestSpoolRoundTrip(t *testing.T) {
	dir := t.TempDir()
	stuck := make(chan struct{})
	hole := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	if err := <-inFlight; err != nil {
		t.Fatalf("export in flight at exit: %v", err)
	}
	resp, err := client.Post(hole.URL, "application/x-protobuf", bytes.NewReader([]byte("last")))
	if err != nil || resp.StatusCode != 200 {
		t.Fatalf("export at exit: %v %v", resp, err)
	}
	if took := time.Since(start); took > 200*time.Millisecond {
		t.Errorf("exit took %s", took)
	}
	if b, _ := os.ReadFile(s.file); string(b) != "last" {
		t.Fatalf("spool = %q, want the export at exit", b)
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
	sendSpool(dir, collector.URL, collector.Client())
	if len(got) != 1 || got[0] != "last" {
		t.Errorf("sent %q, want only the export at exit", got)
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
