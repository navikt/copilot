package provider

import (
	"strings"
	"testing"

	"github.com/navikt/copilot/cli/nav-pilot/internal/local"
)

// With local_endpoint the worker is the endpoint's model, read from the
// config rather than a recorded server, and its empty capabilities block
// sends it nothing until it is measured.
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
	for _, level := range []string{local.DispatchConservative, local.DispatchBalanced, local.DispatchAggressive} {
		policy := LocalDispatchPolicy(m, level, 4, 8)
		if !strings.Contains(policy, "Unsupported and unmeasured") || !strings.Contains(policy, "Send it nothing for now") ||
			strings.Contains(policy, "Send it: ") {
			t.Errorf("%s policy for an endpoint model (#1021):\n%s", level, policy)
		}
		if r := local.DispatchGateRules(level, m.Capabilities); r.Multi || r.Create {
			t.Errorf("%s: an endpoint model got gate rules %+v", level, r)
		}
	}
	// Control: the same model without the block gets the broad text, so the
	// check above can fail.
	m.Capabilities = nil
	if p := LocalDispatchPolicy(m, local.DispatchBalanced, 4, 8); !strings.Contains(p, "Send it: lookups in the code") {
		t.Errorf("control: a nil block did not get the broad text:\n%s", p)
	}
}
