package telemetry

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	colmetricpb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	"google.golang.org/protobuf/proto"
)

// A live export the collector took but never answered is sent again byte for
// byte, same points and timestamps, so Mimir drops the copy. Folded into the
// next export instead, its counts would arrive twice (#1270).
func TestUnansweredExportResentUnchanged(t *testing.T) {
	var mu sync.Mutex
	var got [][]byte
	collector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		mu.Lock()
		got = append(got, b)
		first := len(got) == 1
		mu.Unlock()
		if first {
			<-r.Context().Done() // taken, and the answer is lost
		}
	}))
	defer collector.Close()

	s := newSpoolTransport(http.DefaultTransport, t.TempDir())
	exp, err := otlpmetrichttp.New(t.Context(),
		otlpmetrichttp.WithEndpointURL(collector.URL),
		otlpmetrichttp.WithHTTPClient(&http.Client{Transport: s}),
		otlpmetrichttp.WithRetry(otlpmetrichttp.RetryConfig{Enabled: false}),
		otlpmetrichttp.WithTemporalitySelector(temporalityFor))
	if err != nil {
		t.Fatal(err)
	}
	e := &newOnlyExporter{Exporter: exp}
	reader := sdkmetric.NewManualReader(sdkmetric.WithTemporalitySelector(temporalityFor))
	c, _ := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader)).Meter("t").Int64Counter("c")
	export := func() error {
		var rm metricdata.ResourceMetrics
		if err := reader.Collect(t.Context(), &rm); err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(t.Context(), 300*time.Millisecond)
		defer cancel()
		return e.Export(ctx, &rm)
	}

	c.Add(t.Context(), 3)
	if err := export(); err != nil {
		t.Fatalf("unanswered export failed (%v): its counts would be folded into the next", err)
	}
	c.Add(t.Context(), 2)
	if err := export(); err != nil {
		t.Fatal(err)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(got) != 3 {
		t.Fatalf("collector got %d exports, want the unanswered one, it again, and the next", len(got))
	}
	if !bytes.Equal(got[0], got[1]) {
		t.Error("unanswered export not resent unchanged")
	}
	for i, want := range []int64{3, 3, 2} {
		if v := counterValue(t, got[i]); v != want {
			t.Errorf("export %d carries %d, want %d", i, v, want)
		}
	}
}

// An export that never went out is not kept: the collector cannot have it, so
// it fails and its counts go with the next export. One kept unanswered goes
// to the spool at exit.
func TestUnsentAndExit(t *testing.T) {
	dir := t.TempDir()
	s := newSpoolTransport(http.DefaultTransport, dir)
	client := &http.Client{Transport: s}

	gone := httptest.NewServer(http.NotFoundHandler())
	gone.Close()
	if resp, err := client.Post(gone.URL, "application/x-protobuf", bytes.NewReader([]byte("never"))); err == nil {
		resp.Body.Close()
		t.Fatal("export to a closed port reported success; its counts would be lost")
	}

	hole := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.ReadAll(r.Body)
		<-r.Context().Done()
	}))
	defer hole.Close()
	ctx, cancel := context.WithTimeout(t.Context(), 200*time.Millisecond)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, hole.URL, bytes.NewReader([]byte("maybe")))
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("unanswered export failed: %v", err)
	}
	resp.Body.Close()
	if len(s.unsent) != 1 {
		t.Fatalf("kept %d exports, want only the unanswered one", len(s.unsent))
	}

	s.exit()
	files, _ := filepath.Glob(filepath.Join(dir, "*.pb"))
	if len(files) != 1 {
		t.Fatalf("spool = %v, want the unanswered export", files)
	}
	if b, _ := os.ReadFile(files[0]); string(b) != "maybe" {
		t.Errorf("spool = %q, want the unanswered export", b)
	}
}

func counterValue(t *testing.T, body []byte) (v int64) {
	t.Helper()
	var req colmetricpb.ExportMetricsServiceRequest
	if err := proto.Unmarshal(body, &req); err != nil {
		t.Fatal(err)
	}
	for _, rm := range req.ResourceMetrics {
		for _, sm := range rm.ScopeMetrics {
			for _, m := range sm.Metrics {
				for _, p := range m.GetSum().GetDataPoints() {
					v += p.GetAsInt()
				}
			}
		}
	}
	return v
}
