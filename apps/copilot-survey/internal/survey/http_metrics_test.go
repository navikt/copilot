package survey

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestHTTPMetricsRecordsPatternAndCode(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /things/{id}", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	})
	h := httpMetrics(mux)

	before := testutil.ToFloat64(httpRequestsTotal.WithLabelValues("GET", "/things/{id}", "418"))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/things/abc123", nil))
	if got := testutil.ToFloat64(httpRequestsTotal.WithLabelValues("GET", "/things/{id}", "418")) - before; got != 1 {
		t.Fatalf("pattern series grew by %v, want 1", got)
	}
	if n := testutil.CollectAndCount(httpRequestsTotal); n == 0 {
		t.Fatal("no series")
	}
	if got := testutil.ToFloat64(httpRequestsTotal.WithLabelValues("GET", "/things/abc123", "418")); got != 0 {
		t.Fatalf("raw path recorded: %v", got)
	}
}
