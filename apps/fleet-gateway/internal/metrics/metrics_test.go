package metrics //nolint:revive // the gateway never imports runtime/metrics; name per docs/architecture.md

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/navikt/copilot/fleet-gateway/internal/routing"
)

func TestMetricsRegisteredAndServed(t *testing.T) {
	ObserveMCPRequest(routing.Route{Route: "/mcp/ns/test"}, "upstream_token_error", "", "", "", "", time.Second)
	ObserveHTTPRequest(http.StatusBadGateway, "")
	RoutesLoaded.Set(3)

	if got := testutil.ToFloat64(mcpRequests.WithLabelValues("/mcp/ns/test", "upstream_token_error", "", "", "")); got < 1 {
		t.Fatalf("mcp counter = %v, want >= 1", got)
	}

	srv := httptest.NewServer(Server("").Handler)
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/metrics")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	for _, name := range []string{
		"fleet_gateway_http_requests_total",
		"fleet_gateway_mcp_requests_total",
		"fleet_gateway_mcp_request_duration_seconds_bucket",
		"fleet_gateway_routes_loaded 3",
	} {
		if !strings.Contains(string(body), name) {
			t.Errorf("/metrics lacks %q", name)
		}
	}
}

// The token-error alert uses increase(), which needs a zero sample before the
// first failure.
func TestInitMetricsPrecreatesAlertSeries(t *testing.T) {
	Init([]routing.Route{{Route: "/mcp/ns/precreated"}})
	srv := httptest.NewServer(Server("").Handler)
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/metrics")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	for _, want := range []string{
		`fleet_gateway_mcp_requests_total{decision="allow",outcome="upstream_token_error",reason="",route="/mcp/ns/precreated",tool=""} 0`,
		`fleet_gateway_http_requests_total{code="502",reason=""}`,
	} {
		if !strings.Contains(string(body), want) {
			t.Errorf("/metrics lacks %s", want)
		}
	}
}

func TestObserveMCPRequestBoundsReason(t *testing.T) {
	r := routing.Route{Route: "/mcp/ns/bound", AllowedTools: map[string]bool{"a": true}}
	ObserveMCPRequest(r, "tool_denied", "deny", "made-up", "zzz", "zzz", time.Millisecond)
	if got := testutil.ToFloat64(mcpRequests.WithLabelValues(r.Route, "tool_denied", "deny", "", "other")); got != 1 {
		t.Fatalf("unknown reason/tool not collapsed: %v", got)
	}
}

func TestInitMetricsPrecreatesDenySeries(t *testing.T) {
	Init([]routing.Route{{Route: "/mcp/ns/deny"}})
	body := scrapeMetrics(t)
	for _, reason := range denyReasons {
		if !strings.Contains(body, `decision="deny",outcome=`) || !strings.Contains(body, `reason="`+reason+`"`) {
			t.Errorf("no pre-created deny series for reason %q", reason)
		}
	}
	for _, ep := range RateLimitedEndpoints {
		if !strings.Contains(body, `fleet_gateway_rate_limited_total{oauth_endpoint="`+ep+`"} `) {
			t.Errorf("no pre-created rate limit series for %q", ep)
		}
	}
	if strings.Contains(body, `fleet_gateway_rate_limited_total{endpoint=`) {
		t.Errorf("old endpoint label still exported")
	}
}

// The never-synced alert selects source="fleet-control"; a git pod must not carry it (#2949).
func TestSetRoutesSource(t *testing.T) {
	for _, src := range []string{"git", "fleet-control"} {
		SetRoutesSource(src)
		if got := testutil.CollectAndCount(routesSource); got != 1 {
			t.Fatalf("source %s: %d series, want 1", src, got)
		}
		if got := testutil.ToFloat64(routesSource.WithLabelValues(src)); got != 1 {
			t.Fatalf("source %s: gauge %v, want 1", src, got)
		}
	}
}

func scrapeMetrics(t *testing.T) string {
	t.Helper()
	srv := httptest.NewServer(Server("").Handler)
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/metrics")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	b, _ := io.ReadAll(resp.Body)
	return string(b)
}
