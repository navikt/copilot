package provider

import (
	"strings"
	"testing"

	"github.com/navikt/copilot/cli/nav-pilot/internal/local"
)

// With local_endpoint the worker is the endpoint's model, read from the
// config rather than a recorded server, and the dispatch policy is the text
// for a model without capability verdicts.
func TestLocalWorkerOnAnEndpoint(t *testing.T) {
	local.SetEndpoint("http://127.0.0.1:1", "qwen3.6:35b")
	local.SetActive(local.EndpointManifest("qwen3.6:35b"))
	local.SetEnabled(true)
	orig := ensureOwnServer
	ensureOwnServer = func() error { return nil }
	t.Cleanup(func() {
		ensureOwnServer = orig
		local.SetEndpoint("", "")
		local.SetActive(nil)
		local.SetEnabled(false)
	})
	t.Setenv("HOME", t.TempDir()) // no recorded server to fall back on

	m, err := localWorker()
	if err != nil || m.Model != "qwen3.6:35b" {
		t.Fatalf("localWorker = %+v, %v", m, err)
	}
	policy := LocalDispatchPolicy(m, 4, 8)
	if !strings.Contains(policy, "Unsupported and unmeasured") || !strings.Contains(policy, "Send it: lookups in the code") {
		t.Errorf("policy for an endpoint model:\n%s", policy)
	}
}
