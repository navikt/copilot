package provider

import (
	"fmt"
	"net"
	"slices"
	"strings"
	"testing"

	"github.com/navikt/copilot/cli/nav-pilot/internal/domain"
	"github.com/navikt/copilot/cli/nav-pilot/internal/local"
	"github.com/navikt/copilot/cli/nav-pilot/internal/telemetry"
)

// Local inference on the Copilot CLI.
//
// Nothing here calls LaunchCopilotResolved: on a machine that does have a local
// server recorded and running — which is every machine this feature is for —
// that call launches a real client. What it can be split into is tested
// instead: which environment a local session gets, and which launches are
// refused.

// guardPortIsFree reports whether the loop guard's port was left alone, which is
// how "nothing local happened" is checked rather than asserted.
func guardPortIsFree(t *testing.T) bool {
	t.Helper()
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", local.GuardPort))
	if err != nil {
		return false
	}
	ln.Close()
	return true
}

// TestCopilotLocalSessionRunsOnTheGuardedLocalServer is the premise this file
// was written for: the Copilot CLI takes a custom provider from the
// environment, so a local session is a matter of setting it — and setting it to
// the loop guard, never to the server.
func TestCopilotLocalSessionRunsOnTheGuardedLocalServer(t *testing.T) {
	withLocalEnabled(t)
	m := withOwnServer(t)

	worker, guard, err := copilotLocalWorker(m.Model)
	if err != nil {
		t.Fatalf("copilotLocalWorker for the running local model: %v", err)
	}
	if guard == nil {
		t.Fatal("no loop guard for a local Copilot CLI session; every completion would reach the server unguarded")
	}
	defer guard.Close()
	if worker.Model != m.Model {
		t.Errorf("copilotLocalWorker returned %q, want the running model %q", worker.Model, m.Model)
	}

	base := CopilotEnv("")
	env := copilotLocalEnv(base, worker)

	want := []string{
		"COPILOT_MODEL",
		"COPILOT_PROVIDER_API_KEY",
		"COPILOT_PROVIDER_BASE_URL",
		"COPILOT_PROVIDER_MAX_OUTPUT_TOKENS",
		"COPILOT_PROVIDER_MAX_PROMPT_TOKENS",
		"COPILOT_PROVIDER_TYPE",
		"COPILOT_PROVIDER_WIRE_API",
	}
	if got := envDelta(base, env); !slices.Equal(got, want) {
		t.Errorf("a local Copilot CLI session changes %q over a hosted one; want exactly %q", got, want)
	}

	// The one that decides where the prompt goes. The server's own address
	// here would mean a session with no loop guard in front of it.
	if got, wantURL := telemetry.LookupEnvValue(env, "COPILOT_PROVIDER_BASE_URL"), local.GuardURL()+"/v1"; got != wantURL {
		t.Errorf("COPILOT_PROVIDER_BASE_URL = %q, want the loop guard at %q", got, wantURL)
	}
	for _, kv := range env {
		if strings.Contains(kv, local.ServerURL()) {
			t.Errorf("the launch environment names the local server directly (%q); everything has to pass the loop guard", kv)
		}
	}
	// The wire API the guard can read. Left to the client's default, an
	// exported "responses" would send a shape the guard forwards blind.
	if got := telemetry.LookupEnvValue(env, "COPILOT_PROVIDER_WIRE_API"); got != "completions" {
		t.Errorf("COPILOT_PROVIDER_WIRE_API = %q, want the chat-completions shape the loop guard reads", got)
	}
	if got, wantTokens := telemetry.LookupEnvValue(env, "COPILOT_PROVIDER_MAX_PROMPT_TOKENS"), m.Params["MLX_OPENCODE_CONTEXT"]; got != wantTokens {
		t.Errorf("COPILOT_PROVIDER_MAX_PROMPT_TOKENS = %q, want the manifest's measured context %q", got, wantTokens)
	}
	if got := telemetry.LookupEnvValue(env, "COPILOT_MODEL"); got != m.Model {
		t.Errorf("COPILOT_MODEL = %q, want the running model %q", got, m.Model)
	}
}

// TestCopilotLocalEnvOverridesAnExportedProvider: these variables decide where
// the prompt goes, so a leftover export in the developer's shell must not win.
func TestCopilotLocalEnvOverridesAnExportedProvider(t *testing.T) {
	withLocalEnabled(t)
	m := aLocalModel(t)
	base := []string{"COPILOT_PROVIDER_BASE_URL=https://somewhere.example.com/v1", "PATH=/usr/bin"}

	env := copilotLocalEnv(base, m)
	if got, want := telemetry.LookupEnvValue(env, "COPILOT_PROVIDER_BASE_URL"), local.GuardURL()+"/v1"; got != want {
		t.Errorf("COPILOT_PROVIDER_BASE_URL = %q with one already exported, want %q", got, want)
	}
	if n := strings.Count(strings.Join(env, "\n"), "COPILOT_PROVIDER_BASE_URL="); n != 1 {
		t.Errorf("the launch environment carries %d COPILOT_PROVIDER_BASE_URL entries, want 1", n)
	}
}

// TestCopilotHostedSessionIsUntouchedWithLocalEnabled is the difference from the
// opencode path, and the reason this gates on the session model.
//
// opencode picks a backend per agent, so a cloud main agent and a local worker
// run side by side and the opencode launch sets both up whenever dispatch is
// on. The Copilot CLI has one provider for the whole session: setting it for a
// developer who asked for a hosted model would not add a worker, it would move
// their entire session onto the local model without saying so.
func TestCopilotHostedSessionIsUntouchedWithLocalEnabled(t *testing.T) {
	withLocalEnabled(t)
	withOwnServer(t)

	worker, guard, err := copilotLocalWorker("github-copilot/claude-opus-5")
	if err != nil || guard != nil || worker.Model != "" {
		if guard != nil {
			guard.Close()
		}
		t.Fatalf("copilotLocalWorker for a hosted model = (%q, %v, %v), want nothing local", worker.Model, guard, err)
	}
	if !guardPortIsFree(t) {
		t.Errorf("a hosted Copilot CLI launch left something listening on the loop guard's port %d", local.GuardPort)
	}
}

// TestCopilotLocalDisabledRefusesWithAdvice: with dispatch off a manifest model
// id is refused, naming the command that turns local inference on.
//
// It used to be treated as an ordinary unrecognised id and passed through to
// GitHub, which answers "model not available" — a true statement about its
// catalogue and no help at all to someone whose config lost `local_enabled`.
//
// The ~650 developers who never opt in are unaffected: their session model is a
// GitHub model, which is not in the manifest, so this predicate is false for
// them. TestCopilotHostedModelStaysHosted is the other half of that.
func TestCopilotLocalDisabledRefusesWithAdvice(t *testing.T) {
	worker, guard, err := copilotLocalWorker(aLocalModelID(t))
	if guard != nil {
		guard.Close()
		t.Error("a refused launch started a loop guard")
	}
	if err == nil {
		t.Fatalf("copilotLocalWorker with local dispatch off = (%q, nil), want a refusal", worker.Model)
	}
	if !strings.Contains(err.Error(), "alpha local init") {
		t.Errorf("refusal does not say how to turn local inference on: %v", err)
	}
}

// TestCopilotHostedModelStaysHosted: a model that is not in the manifest is
// untouched whether or not local dispatch is enabled. This is every launch for
// everyone who never opted in, and the reason the refusal above is safe.
func TestCopilotHostedModelStaysHosted(t *testing.T) {
	worker, guard, err := copilotLocalWorker("claude-opus-5")
	if err != nil || guard != nil || worker.Model != "" {
		if guard != nil {
			guard.Close()
		}
		t.Fatalf("copilotLocalWorker for a hosted model = (%q, %v, %v), want nothing local", worker.Model, guard, err)
	}
}

// TestCopilotLocalRefusesAServerNavPilotDidNotStart is the launch half of the
// ownership rule, on the Copilot CLI side. The guard forwards to a fixed port,
// so a server that died and left that port to whatever bound it next would have
// the whole session proxied to a stranger.
func TestCopilotLocalRefusesAServerNavPilotDidNotStart(t *testing.T) {
	withLocalEnabled(t)
	t.Setenv("HOME", t.TempDir()) // nothing recorded, and no server to record

	worker, guard, err := copilotLocalWorker(aLocalModelID(t))
	if err == nil {
		if guard != nil {
			guard.Close()
		}
		t.Fatal("a local Copilot CLI session was allowed with no server of nav-pilot's own behind it")
	}
	if guard != nil {
		guard.Close()
		t.Error("a refused launch left the loop guard listening")
	}
	if worker.Model != "" {
		t.Errorf("a refused launch still returned a worker (%q)", worker.Model)
	}
}

// TestCopilotLocalRefusesAModelTheServerIsNotServing.
//
// The session model and the loaded model are two separate pieces of state: one
// is config the developer can change at any time, the other is what `alpha
// local start` loaded. The server answers on whatever it has loaded regardless
// of what the request names, so without this the session would quietly run on a
// model nobody chose. It needs a manifest with two models to be reachable at
// all, which is what the served one will look like as soon as a second model
// ships.
func TestCopilotLocalRefusesAModelTheServerIsNotServing(t *testing.T) {
	withLocalEnabled(t)
	running := withOwnServer(t)

	other := "mlx-community/Some-Other-Model-4bit"
	withManifestAlso(t, running, other)

	worker, guard, err := copilotLocalWorker(other)
	if err == nil {
		if guard != nil {
			guard.Close()
		}
		t.Fatal("a session configured for a model the local server is not serving was allowed")
	}
	if guard != nil {
		guard.Close()
		t.Error("a refused launch left the loop guard listening")
	}
	if worker.Model != "" {
		t.Errorf("a refused launch still returned a worker (%q)", worker.Model)
	}
	for _, name := range []string{running.Model, other} {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("the refusal does not name %q, so the developer cannot see which two models disagree:\n%v", name, err)
		}
	}
}

// withManifestAlso installs a manifest holding the running model plus one more
// id, and restores the embedded one afterwards. [local.Lookup] answers from the
// active manifest, so this is the only way to have two local ids at once while
// the served manifest names one.
func withManifestAlso(t *testing.T, running local.Model, extra string) {
	t.Helper()
	raw := fmt.Sprintf(`{"schema_version":1,"channel":"alpha","models":[
	  {"key":"running","name":"Running","model":%q,"backend":"mlx-lm","default":true,"params":{"MLX_OPENCODE_CONTEXT":"65536"}},
	  {"key":"other","name":"Other","model":%q,"backend":"mlx-lm","params":{"MLX_OPENCODE_CONTEXT":"32768"}}
	]}`, running.Model, extra)
	m, err := local.Parse([]byte(raw))
	if err != nil {
		t.Fatalf("the two-model manifest this test needs is not valid: %v", err)
	}
	local.SetActive(m)
	t.Cleanup(func() { local.SetActive(nil) })
}

// TestStagedCopilotRefusesALocalModel keeps the refusal where it is still true.
//
// A Tier 2 launch runs a digest-verified payload, built and tested against the
// model its manifest declares. Redirecting that session to a 4-bit model on a
// laptop is not the contract anyone reviewed — and it is the case the old
// refusal never covered, so a staged launch on a local model id went to GitHub.
func TestStagedCopilotRefusesALocalModel(t *testing.T) {
	SetActivePakke(stagedFixturePakke())
	t.Cleanup(func() { SetActivePakke(nil) })
	withLocalEnabled(t)

	id := aLocalModelID(t)
	staged := StagedLaunch{Dir: "/staged/x", PakkeName: "grillmester", Context: "full"}

	spec, err := buildStagedCopilotSpec(domain.ResolvedConfig{Model: id, AskUser: true}, staged)
	if err == nil {
		t.Fatalf("a staged Tier 2 launch on local model %q was built rather than refused: %q", id, spec.agentArgs)
	}
	if !strings.Contains(err.Error(), id) || !strings.Contains(err.Error(), "grillmester") {
		t.Errorf("the refusal names neither the model nor the agentpakke:\n%v", err)
	}
	// And the environment it would have run under is never built.
	for _, kv := range spec.env {
		if strings.HasPrefix(kv, "COPILOT_PROVIDER_BASE_URL=") {
			t.Errorf("a refused staged launch still produced a provider environment: %q", kv)
		}
	}
}

// TestCopilotModelPickerOffersTheLocalModel: the Copilot CLI can now reach the
// local server, so the picker has to say so. Left out, the only way to select
// the model would be to know its Hugging Face id and type it.
func TestCopilotModelPickerOffersTheLocalModel(t *testing.T) {
	withLocalEnabled(t)
	id := aLocalModelID(t)

	found := false
	for _, m := range (copilotProvider{}).KnownModels() {
		found = found || m.ID == id
	}
	if !found {
		t.Errorf("the copilot model picker does not offer %q with local dispatch on", id)
	}
	// And it is not warned about as an id the server may reject: with BYOK
	// there is no GitHub catalogue in the session to reject it.
	if advice := (copilotProvider{}).ModelAdvisory(id); advice != "" {
		t.Errorf("selecting the local model warns about it: %q", advice)
	}
}
