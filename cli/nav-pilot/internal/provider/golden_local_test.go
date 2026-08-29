package provider

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/navikt/copilot/cli/nav-pilot/internal/domain"
	"github.com/navikt/copilot/cli/nav-pilot/internal/local"
	"github.com/navikt/copilot/cli/nav-pilot/internal/telemetry"
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

// TestGoldenLaunchEnvIsUnchangedWithoutLocal is the environment half: with
// local dispatch off, a launch adds nothing to the developer's own environment
// beyond the OTel level it has always added.
//
// It is a snapshot of the delta over os.Environ(), not a comparison of the
// builder against itself. The version before this compared CopilotEnv(otel)
// with CopilotEnv(otel) — two identical calls to a function that takes no model
// — so "byte-identical between a local and a hosted launch" was proved by
// nothing at all, and what remained was two substring greps.
//
// The delta is the honest quantity: the environment carries the developer's own
// variables and whatever OTel resolves to, and none of that is nav-pilot's to
// pin. What a launch *adds* is, and with local off it is a written-down list
// that a new variable — local or otherwise — would break.
func TestGoldenLaunchEnvIsUnchangedWithoutLocal(t *testing.T) {
	// A home with no ~/.copilot customizations and no OTel endpoint, so the
	// golden below is the launch's own doing and not the machine's.
	t.Setenv("HOME", t.TempDir())
	for _, key := range []string{
		"COPILOT_CUSTOM_INSTRUCTIONS_DIRS", "OTEL_LOG_LEVEL",
		"OTEL_EXPORTER_OTLP_ENDPOINT", "COPILOT_OTEL_ENDPOINT",
	} {
		unsetForTest(t, key)
	}

	// Keys, not values: the delta's values carry the device id and the repo
	// this test happens to run in, and neither is nav-pilot's to pin. The keys
	// are the whole claim — a local-inference variable would be one more.
	otelKeys := []string{
		"COPILOT_OTEL_ENABLED",
		"OTEL_EXPORTER_OTLP_ENDPOINT",
		"OTEL_LOGS_EXPORTER",
		"OTEL_RESOURCE_ATTRIBUTES",
	}
	golden := map[string][]string{
		"":      otelKeys,
		"none":  append(slices.Clone(otelKeys), "OTEL_LOG_LEVEL"),
		"debug": append(slices.Clone(otelKeys), "OTEL_LOG_LEVEL"),
	}
	for otel, want := range golden {
		env := CopilotEnv(otel)
		slices.Sort(want)
		if got := envDelta(os.Environ(), env); !slices.Equal(got, want) {
			t.Errorf("CopilotEnv(%q) changes %q in os.Environ(); want exactly %q", otel, got, want)
		}
		// The one value that is nav-pilot's, and the only one this argument
		// decides.
		if otel != "" {
			if got := telemetry.LookupEnvValue(env, "OTEL_LOG_LEVEL"); got != otel {
				t.Errorf("CopilotEnv(%q) set OTEL_LOG_LEVEL=%q", otel, got)
			}
		}
	}

	// And the launch refuses nothing: a local id with local off is an ordinary
	// unrecognised model id, which the server rejects, not nav-pilot.
	if local.IsLocal(aLocalModelID(t)) {
		t.Error("local.IsLocal answered true with local dispatch disabled")
	}
}

// envDelta names the variables want adds to or changes from base, sorted. A
// variable base holds and want drops is named with a leading "-", because a
// launch that takes a variable out of the developer's environment is as much a
// change as one that puts a variable in.
func envDelta(base, want []string) []string {
	had := make(map[string]string, len(base))
	for _, kv := range base {
		k, v, _ := strings.Cut(kv, "=")
		had[k] = v
	}
	var delta []string
	seen := make(map[string]bool, len(want))
	for _, kv := range want {
		k, v, _ := strings.Cut(kv, "=")
		seen[k] = true
		if old, ok := had[k]; !ok || old != v {
			delta = append(delta, k)
		}
	}
	for k := range had {
		if !seen[k] {
			delta = append(delta, "-"+k)
		}
	}
	slices.Sort(delta)
	return delta
}

// unsetForTest removes a variable for the length of a test and puts it back,
// which t.Setenv cannot do — an empty value is still a variable, and the
// builders under test treat the two differently.
func unsetForTest(t *testing.T, key string) {
	t.Helper()
	old, had := os.LookupEnv(key)
	if !had {
		return
	}
	if err := os.Unsetenv(key); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Setenv(key, old) })
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
	// The Copilot CLI is pointed at the loop guard rather than refused. Not
	// LaunchCopilotResolved: on a machine that does have a local server
	// recorded and running, that call launches a real client.
	if got := telemetry.LookupEnvValue(copilotLocalEnv(nil, aLocalModel(t)), "COPILOT_PROVIDER_BASE_URL"); got != local.GuardURL()+"/v1" {
		t.Errorf("copilotLocalEnv points the Copilot CLI at %q, want the loop guard at %q", got, local.GuardURL()+"/v1")
	}
}

// TestHostedLaunchStartsNoLoopGuard is finding 7: the disabled path of
// LaunchOpenCode had nothing covering it, so moving StartGuard out from behind
// its IsLocal gate failed no test at all.
//
// A hosted launch must start no listener and write no opencode config. Both are
// checked here rather than asserted, because both are what a developer who
// never opted in would notice: a port taken on their machine, and a provider
// block appearing in a file they own.
func TestHostedLaunchStartsNoLoopGuard(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cfg := filepath.Join(t.TempDir(), "opencode.json")
	ConfigPathOverride = cfg
	t.Cleanup(func() { ConfigPathOverride = "" })

	guard, err := startLocalDispatch("claude-opus-5")
	if err != nil || guard != nil {
		t.Fatalf("startLocalDispatch for a hosted model = (%v, %v), want (nil, nil)", guard, err)
	}
	if ln, lerr := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", local.GuardPort)); lerr != nil {
		t.Errorf("a hosted launch left something listening on the loop guard's port %d", local.GuardPort)
	} else {
		ln.Close()
	}
	if _, serr := os.Stat(cfg); !os.IsNotExist(serr) {
		t.Errorf("a hosted launch wrote %s; a developer who never opted in owns that file", cfg)
	}

	// A local model id with dispatch off is refused, and says how to turn it on.
	// It used to be treated as a hosted launch, which sent a Hugging Face model
	// id to the cloud provider and got back "model not available" — true of
	// GitHub's catalogue and useless as advice. A reboot that emptied the config
	// was enough to reach this, and it cost an evening.
	guard, err = startLocalDispatch(aLocalModelID(t))
	if err == nil {
		if guard != nil {
			guard.Close()
		}
		t.Fatal("a local model id with dispatch off was launched as a hosted session")
	}
	if !strings.Contains(err.Error(), "alpha local init") {
		t.Errorf("refusal does not say how to turn local inference on: %v", err)
	}
	if guard != nil {
		guard.Close()
		t.Error("a refused launch started a loop guard")
	}
}

// TestLocalLaunchRefusesAServerNavPilotDidNotStart is the launch half of the
// ownership rule Server.Start keeps at the other end of the day.
//
// The guard proxies to a fixed 127.0.0.1:8080. Before this, the launch started
// it without checking that the server nav-pilot recorded was still the process
// holding that port — so a server that crashed and any other tool that then
// bound 8080 meant every prompt of the session went to a stranger, silently.
// Refusing is the answer, not adopting: a stranger reporting the right model id
// is still a stranger.
func TestLocalLaunchRefusesAServerNavPilotDidNotStart(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	withLocalEnabled(t)
	id := aLocalModelID(t)

	// Nothing recorded. Reachable on its own: the opencode provider block that
	// `start` wrote is still in the developer's config the next morning, so the
	// model is still selectable after the server is gone.
	if _, err := startLocalDispatch(id); err == nil {
		t.Fatal("startLocalDispatch with no recorded server succeeded; the guard would forward to whatever holds the port")
	}

	// Recorded, alive, and the process it says it is — but not the one holding
	// the port. This test process is all three.
	if err := local.SaveState(local.State{PID: os.Getpid(), Model: id, Started: time.Now()}); err != nil {
		t.Fatal(err)
	}
	_, err := startLocalDispatch(id)
	if err == nil || !strings.Contains(err.Error(), "not what is listening") {
		t.Errorf("startLocalDispatch for a recorded server that does not hold the port = %v, want a refusal naming the port", err)
	}

	if ln, lerr := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", local.GuardPort)); lerr != nil {
		t.Errorf("a refused launch left the loop guard listening on %d", local.GuardPort)
	} else {
		ln.Close()
	}
}
