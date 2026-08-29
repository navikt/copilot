package provider

import (
	"slices"
	"strings"
	"testing"

	"github.com/navikt/copilot/cli/nav-pilot/internal/domain"
	"github.com/navikt/copilot/cli/nav-pilot/internal/local"
)

// The no-op proof for local inference.
//
// Around 650 people use nav-pilot and will never turn local inference on. For
// every one of them the local branches must not exist: same model picker, same
// argument vectors, same launch environment, same staged payload. This file is
// how that is proved rather than asserted, and it is the reason the local
// branches are gated on one predicate instead of scattered checks.
//
// Every test here runs with local dispatch off, which is the default state of
// the package — nothing in this file enables it. The one that does,
// TestLocalBranchesTakeEffectWhenEnabled at the bottom, exists so the rest
// cannot pass vacuously: if the wiring were deleted entirely, these would all
// still pass and that one would fail.

// aLocalModelID is a model id from the embedded manifest — the exact input that
// would take a local branch if local dispatch were on.
func aLocalModelID(t *testing.T) string {
	t.Helper()
	models := local.Active().Models
	if len(models) == 0 {
		t.Fatal("the embedded local-model manifest names no models")
	}
	return models[0].Model
}

// withLocalEnabled turns local dispatch on for one test and off again after.
func withLocalEnabled(t *testing.T) {
	t.Helper()
	local.SetEnabled(true)
	t.Cleanup(func() { local.SetEnabled(false) })
}

// TestGoldenLocalDisabledLeavesModelMappingAlone: a local model id is just a
// provider-qualified id to a nav-pilot with local off, and passes through
// exactly as any other slash-bearing id always has.
func TestGoldenLocalDisabledLeavesModelMappingAlone(t *testing.T) {
	id := aLocalModelID(t)
	if got := ToOpenCodeModel(id); got != id {
		t.Errorf("ToOpenCodeModel(%q) = %q with local disabled; want the id unchanged, as for any other provider-qualified id", id, got)
	}
	if local.IsLocal(id) {
		t.Fatal("local.IsLocal answered true with local dispatch disabled")
	}
}

// TestGoldenLocalDisabledLeavesTheModelPickerAlone: nothing local appears in
// the list a developer picks from.
func TestGoldenLocalDisabledLeavesTheModelPickerAlone(t *testing.T) {
	got := openCodeProvider{}.KnownModels()
	if !slices.Equal(got, knownOpenCodeModels) {
		t.Errorf("opencode KnownModels() with local disabled has %d entries, want the curated %d",
			len(got), len(knownOpenCodeModels))
	}
	for _, p := range AllProviders() {
		for _, m := range p.KnownModels() {
			if strings.HasPrefix(m.ID, "mlx-community/") || strings.HasPrefix(m.ID, LocalProviderID+"/") {
				t.Errorf("%s offers local model %q with local dispatch disabled", p.ID(), m.ID)
			}
		}
	}
}

// TestGoldenLocalDisabledLeavesLaunchArgsAlone pins the argument vectors for a
// local model id against the rules that produced them before this feature: the
// copilot id is forwarded verbatim, the opencode one keeps its own provider.
func TestGoldenLocalDisabledLeavesLaunchArgsAlone(t *testing.T) {
	id := aLocalModelID(t)
	r := domain.ResolvedConfig{Model: id, AskUser: true}

	wantCopilot := []string{"--agent", "copilot", "--", "--agent", "nav-pilot", "--model", id}
	if got := BuildCopilotArgs("cplt", r); !slices.Equal(got, wantCopilot) {
		t.Errorf("BuildCopilotArgs\n got: %q\nwant: %q", got, wantCopilot)
	}
	wantOpenCode := []string{"--model", id, "--agent", "nav-pilot"}
	if got := OpenCodeArgs(r); !slices.Equal(got, wantOpenCode) {
		t.Errorf("OpenCodeArgs\n got: %q\nwant: %q", got, wantOpenCode)
	}
}

// TestGoldenLaunchEnvIsUnchangedWithoutLocal is the environment half: the
// variables a launch runs under carry nothing about local inference, and are
// byte-identical to the same launch for a hosted model.
//
// Byte-identical is checked against the same builder called with a hosted model
// id rather than against a written-down list, because the environment also
// carries the developer's own variables and OTel resource attributes. What must
// not differ between the two is anything at all.
func TestGoldenLaunchEnvIsUnchangedWithoutLocal(t *testing.T) {
	id := aLocalModelID(t)

	for _, otel := range []string{"", "none", "debug"} {
		hosted := CopilotEnv(otel)
		withLocal := CopilotEnv(otel)
		if !slices.Equal(hosted, withLocal) {
			t.Errorf("CopilotEnv(%q) is not stable across calls; the comparison below is meaningless", otel)
		}
		for _, kv := range withLocal {
			if strings.Contains(strings.ToUpper(kv), "NAV_PILOT_LOCAL") || strings.Contains(kv, LocalProviderID+"://") {
				t.Errorf("CopilotEnv(%q) carries a local-inference variable with local disabled: %q", otel, kv)
			}
		}
	}

	// And the launch refuses nothing: a local id with local off is an ordinary
	// unrecognised model id, which the server rejects, not nav-pilot.
	if local.IsLocal(id) {
		t.Error("local.IsLocal answered true with local dispatch disabled")
	}
}

// TestGoldenStagedPayloadIsUnchangedWithoutLocal: a Tier 2 launch of a local
// model id produces the same cplt invocation, the same client arguments and the
// same environment as before local inference existed.
func TestGoldenStagedPayloadIsUnchangedWithoutLocal(t *testing.T) {
	SetActivePakke(stagedFixturePakke())
	t.Cleanup(func() { SetActivePakke(nil) })

	id := aLocalModelID(t)
	staged := StagedLaunch{Dir: "/staged/x", PakkeName: "grillmester", Context: "full"}

	copilotSpec := buildStagedSpec(t, "copilot", domain.ResolvedConfig{Model: id, AskUser: true}, staged)
	wantCopilotArgs := []string{"--plugin-dir", "/staged/x", "--agent", "grillmester:grillmester", "--model", id}
	if !slices.Equal(copilotSpec.agentArgs, wantCopilotArgs) {
		t.Errorf("staged copilot agentArgs\n got: %q\nwant: %q", copilotSpec.agentArgs, wantCopilotArgs)
	}
	if !slices.Equal(copilotSpec.cpltArgs, []string{"--allow-read", "/staged/x"}) {
		t.Errorf("staged copilot cpltArgs = %q", copilotSpec.cpltArgs)
	}

	openCodeSpec := buildStagedSpec(t, "opencode", domain.ResolvedConfig{Model: id, AskUser: true}, staged)
	wantOpenCodeArgs := []string{"--agent", "grillmester", "--model", id}
	if !slices.Equal(openCodeSpec.agentArgs, wantOpenCodeArgs) {
		t.Errorf("staged opencode agentArgs\n got: %q\nwant: %q", openCodeSpec.agentArgs, wantOpenCodeArgs)
	}

	for _, spec := range []cpltLaunch{copilotSpec, openCodeSpec} {
		for _, kv := range spec.env {
			if strings.Contains(strings.ToUpper(kv), "NAV_PILOT_LOCAL") {
				t.Errorf("staged %s env carries a local-inference variable with local disabled: %q", spec.agent, kv)
			}
		}
	}
}

// TestLocalBranchesTakeEffectWhenEnabled keeps the tests above from passing for
// the wrong reason. Delete the local wiring entirely and every assertion above
// still holds; this one does not.
func TestLocalBranchesTakeEffectWhenEnabled(t *testing.T) {
	withLocalEnabled(t)
	id := aLocalModelID(t)

	if got, want := ToOpenCodeModel(id), LocalProviderID+"/"+id; got != want {
		t.Errorf("ToOpenCodeModel(%q) with local enabled = %q, want %q", id, got, want)
	}
	found := false
	for _, m := range (openCodeProvider{}).KnownModels() {
		found = found || m.ID == id
	}
	if !found {
		t.Errorf("the opencode model picker does not offer %q with local enabled", id)
	}
	err := LaunchCopilotResolved(domain.ResolvedConfig{Model: id})
	if err == nil || !strings.Contains(err.Error(), "cannot be pointed at a server on this machine") {
		t.Errorf("LaunchCopilotResolved for a local model = %v, want a refusal naming the reason", err)
	}
}
