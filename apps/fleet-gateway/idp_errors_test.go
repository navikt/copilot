package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/navikt/copilot/fleet-gateway/internal/httpx"
	"github.com/navikt/copilot/fleet-gateway/internal/metrics"
)

// Only Entra/Texas faults count; user-caused 4xx must not page (#2893).
func TestCallbackCountsIDPErrors(t *testing.T) {
	stub := func(t *testing.T, status int, body map[string]any) string {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			httpx.WriteJSON(w, status, body)
		}))
		t.Cleanup(srv.Close)
		return srv.URL
	}
	closed := httptest.NewServer(http.NotFoundHandler())
	closed.Close()

	cases := []struct {
		name  string
		setup func(t *testing.T, te *testEnv)
		step  string
		want  float64
	}{
		{"400 invalid_grant", func(t *testing.T, te *testEnv) {
			te.gw.entra.TokenEndpoint = stub(t, http.StatusBadRequest, map[string]any{"error": "invalid_grant"})
		}, "code_redeem", 0},
		{"503", func(t *testing.T, te *testEnv) {
			te.gw.entra.TokenEndpoint = stub(t, http.StatusServiceUnavailable, map[string]any{"error": "temporarily_unavailable"})
		}, "code_redeem", 1},
		{"transport error", func(_ *testing.T, te *testEnv) {
			te.gw.entra.TokenEndpoint = closed.URL
		}, "code_redeem", 1},
		{"introspection failure", func(_ *testing.T, te *testEnv) {
			te.gw.cfg.IdentitySource = identitySourceIntrospection
			te.mu.Lock()
			te.introspectStatus = http.StatusInternalServerError
			te.mu.Unlock()
		}, "introspection", 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			te := newTestEnv(t)
			tc.setup(t, te)
			c := metrics.IDPErrors.WithLabelValues(tc.step)
			before := testutil.ToFloat64(c)
			entraState, stateCookie := te.startLogin("verifier-0123456789012345678901234567890123456789")
			resp := te.callback(entraState, stateCookie)
			_ = resp.Body.Close()
			if got := testutil.ToFloat64(c) - before; got != tc.want {
				t.Fatalf("%s: got %v, want %v", tc.step, got, tc.want)
			}
		})
	}
}
