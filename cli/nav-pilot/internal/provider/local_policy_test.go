package provider

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/navikt/copilot/cli/nav-pilot/internal/local"
)

// withOpenCodeConfig points the opencode config — and with it the dispatch
// policy beside it — at a directory this test owns.
func withOpenCodeConfig(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	ConfigPathOverride = filepath.Join(dir, "opencode.json")
	t.Cleanup(func() { ConfigPathOverride = "" })
	return dir
}

// aLocalModel is the manifest entry a launch would resolve to.
func aLocalModel(t *testing.T) local.Model {
	t.Helper()
	m, found := local.Lookup(aLocalModelID(t))
	if !found {
		t.Fatal("the embedded manifest does not contain its own model")
	}
	return m
}

// TestLocalDispatchPolicyNamesTheModelAndTheThreshold: the fragment is
// generated so it can be exact, and the two things it has to be exact about
// are which model is behind the endpoint and how many repeated calls end the
// turn. "The local model" and "a few calls" would carry neither.
func TestLocalDispatchPolicyNamesTheModelAndTheThreshold(t *testing.T) {
	m := aLocalModel(t)
	got := LocalDispatchPolicy(m, 5)

	if !strings.Contains(got, m.Model) {
		t.Errorf("the dispatch policy does not name the model %q:\n%s", m.Model, got)
	}
	if !strings.Contains(got, " 5 like kall") {
		t.Errorf("the dispatch policy does not name the configured threshold 5:\n%s", got)
	}
	if strings.Contains(got, strconv.Itoa(local.DefaultLoopGuardRepeat)+" like kall") {
		t.Errorf("the dispatch policy names the built-in default instead of the configured threshold:\n%s", got)
	}
	if m.Role != "" && !strings.Contains(got, m.Role) {
		t.Errorf("the dispatch policy drops the manifest's role:\n%s", got)
	}
	if m.Expect != "" && !strings.Contains(got, m.Expect) {
		t.Errorf("the dispatch policy drops the manifest's expect:\n%s", got)
	}
	// A policy long enough to be skimmed past defeats its own purpose.
	if lines := strings.Count(strings.TrimSpace(got), "\n") + 1; lines > 16 {
		t.Errorf("the dispatch policy is %d lines; it is meant to be short enough to read", lines)
	}
}

// TestLocalDispatchPolicyIsByteIdenticalAcrossGenerations: opencode reads the
// file into the system prompt, and prompt-cache reuse holds only while that
// prefix does not move. A timestamp, a map iteration or a "generated at" line
// in here would cost a full prefill on every tool call of every turn.
func TestLocalDispatchPolicyIsByteIdenticalAcrossGenerations(t *testing.T) {
	m := aLocalModel(t)
	first := LocalDispatchPolicy(m, 8)
	for i := range 20 {
		if got := LocalDispatchPolicy(m, 8); got != first {
			t.Fatalf("generation %d of the dispatch policy differs from the first:\n%s\n---\n%s", i, first, got)
		}
	}
}

// TestEnsureOpenCodeLocalPolicyRegistersItselfOnce: opencode's instructions
// array is the additive hook, so nav-pilot adds one entry to it and touches
// nothing else in the developer's file — and adds it once no matter how many
// launches run.
func TestEnsureOpenCodeLocalPolicyRegistersItselfOnce(t *testing.T) {
	dir := withOpenCodeConfig(t)
	withLocalEnabled(t)
	m := aLocalModel(t)

	// Something of the developer's own, in the same key.
	mine := filepath.Join(dir, "min-egen.md")
	if err := os.WriteFile(ConfigPathOverride, []byte(`{"instructions":["`+mine+`"],"theme":"tokyonight"}`), 0o600); err != nil {
		t.Fatal(err)
	}

	for range 3 {
		if err := EnsureOpenCodeLocalPolicy(m); err != nil {
			t.Fatalf("EnsureOpenCodeLocalPolicy: %v", err)
		}
	}

	policy := filepath.Join(dir, localPolicyFileName)
	body, err := os.ReadFile(policy)
	if err != nil {
		t.Fatalf("the dispatch policy was not written: %v", err)
	}
	if !strings.Contains(string(body), m.Model) {
		t.Errorf("the written dispatch policy does not name %q:\n%s", m.Model, body)
	}

	cfg := readOpenCodeConfig(t)
	if cfg["theme"] != "tokyonight" {
		t.Errorf("registering the dispatch policy lost the developer's own keys: %v", cfg)
	}
	entries, _ := cfg["instructions"].([]any)
	want := []any{mine, policy}
	if len(entries) != len(want) {
		t.Fatalf("instructions = %v after three launches, want exactly %v", entries, want)
	}
	for i, e := range want {
		if entries[i] != e {
			t.Errorf("instructions[%d] = %v, want %v", i, entries[i], e)
		}
	}
}

// TestLocalDisabledWritesNothingForACloudSession is the no-op proof, and the
// half of the gap that must not move: with dispatch off, a launch on a hosted
// model writes no policy file, starts no guard, and leaves the developer's
// config byte-for-byte as it found it. This is every launch for the ~650
// developers who never turn the alpha on.
func TestLocalDisabledWritesNothingForACloudSession(t *testing.T) {
	dir := withOpenCodeConfig(t)

	// A config of the developer's own, so "untouched" is something this can
	// actually compare rather than an absence.
	before := []byte(`{"theme":"tokyonight","instructions":["` + filepath.Join(dir, "min-egen.md") + `"]}`)
	if err := os.WriteFile(ConfigPathOverride, before, 0o600); err != nil {
		t.Fatal(err)
	}

	guard, err := startLocalDispatch("github-copilot/claude-opus-5")
	if err != nil {
		t.Fatalf("startLocalDispatch with local dispatch off: %v", err)
	}
	if guard != nil {
		guard.Close()
		t.Error("a loop guard was started with local dispatch off")
	}

	if _, err := os.Stat(filepath.Join(dir, localPolicyFileName)); !os.IsNotExist(err) {
		t.Error("a dispatch policy was written with local dispatch off")
	}
	after, err := os.ReadFile(ConfigPathOverride)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Errorf("local dispatch off rewrote the developer's opencode config:\n got: %s\nwant: %s", after, before)
	}

	// And on a machine that has no opencode config at all, it does not create
	// one.
	ConfigPathOverride = filepath.Join(t.TempDir(), "opencode.json")
	if _, err := startLocalDispatch("github-copilot/claude-opus-5"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(ConfigPathOverride); !os.IsNotExist(err) {
		t.Error("an opencode config was created with local dispatch off")
	}
}

// TestRemoveOpenCodeLocalPolicyLeavesTheDeveloperTheirOwn: off takes back
// exactly what the launch put there.
func TestRemoveOpenCodeLocalPolicyLeavesTheDeveloperTheirOwn(t *testing.T) {
	dir := withOpenCodeConfig(t)
	withLocalEnabled(t)

	mine := filepath.Join(dir, "min-egen.md")
	if err := os.WriteFile(ConfigPathOverride, []byte(`{"instructions":["`+mine+`"]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := EnsureOpenCodeLocalPolicy(aLocalModel(t)); err != nil {
		t.Fatal(err)
	}
	if err := RemoveOpenCodeLocalPolicy(); err != nil {
		t.Fatalf("RemoveOpenCodeLocalPolicy: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, localPolicyFileName)); !os.IsNotExist(err) {
		t.Error("the dispatch policy file survived removal")
	}
	entries, _ := readOpenCodeConfig(t)["instructions"].([]any)
	if len(entries) != 1 || entries[0] != mine {
		t.Errorf("instructions = %v after removal, want only the developer's own %q", entries, mine)
	}

	// And again, on a machine that never had one: off must work anywhere.
	if err := RemoveOpenCodeLocalPolicy(); err != nil {
		t.Errorf("RemoveOpenCodeLocalPolicy on an already-clean config: %v", err)
	}
	ConfigPathOverride = filepath.Join(t.TempDir(), "opencode.json")
	if err := RemoveOpenCodeLocalPolicy(); err != nil {
		t.Errorf("RemoveOpenCodeLocalPolicy with no opencode config at all: %v", err)
	}
	if _, err := os.Stat(ConfigPathOverride); !os.IsNotExist(err) {
		t.Error("removal created an opencode config that was not there")
	}
}

// TestRemoveOpenCodeLocalPolicyDropsAnEmptyInstructionsKey: nav-pilot does not
// leave "instructions": [] behind in someone else's file.
func TestRemoveOpenCodeLocalPolicyDropsAnEmptyInstructionsKey(t *testing.T) {
	withOpenCodeConfig(t)
	withLocalEnabled(t)

	if err := EnsureOpenCodeLocalPolicy(aLocalModel(t)); err != nil {
		t.Fatal(err)
	}
	if err := RemoveOpenCodeLocalPolicy(); err != nil {
		t.Fatal(err)
	}
	if _, found := readOpenCodeConfig(t)["instructions"]; found {
		t.Error("an empty instructions key was left behind in the developer's config")
	}
}

func readOpenCodeConfig(t *testing.T) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(ConfigPathOverride)
	if err != nil {
		t.Fatalf("reading the opencode config back: %v", err)
	}
	var cfg map[string]any
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatalf("the opencode config is not valid JSON: %v", err)
	}
	return cfg
}

// workerModel returns the model the worker agent is bound to in the config on
// disk, or "" when nothing binds it.
func workerModel(t *testing.T) string {
	t.Helper()
	agents, _ := readOpenCodeConfig(t)["agent"].(map[string]any)
	worker, _ := agents[local.WorkerAgent].(map[string]any)
	model, _ := worker["model"].(string)
	return model
}

// TestLocalWorkerIsBoundToTheLocalModel is the alpha's cost premise, pinned. An
// opencode subagent with no model of its own runs on the session's model, so
// without this block a cloud main agent dispatching to `lokal-arbeider` spends
// the tokens the dispatch policy tells it are free.
func TestLocalWorkerIsBoundToTheLocalModel(t *testing.T) {
	withOpenCodeConfig(t)
	withLocalEnabled(t)
	m := aLocalModel(t)

	// Something of the developer's own, in the same file and the same key.
	if err := os.WriteFile(ConfigPathOverride,
		[]byte(`{"theme":"tokyonight","agent":{"min-egen":{"model":"github-copilot/gpt-5"}}}`), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := EnsureOpenCodeLocalProvider(m, testGuardURL); err != nil {
		t.Fatalf("EnsureOpenCodeLocalProvider: %v", err)
	}

	want := LocalProviderID + "/" + m.Model
	if got := workerModel(t); got != want {
		t.Errorf("the worker agent is bound to %q, want %q", got, want)
	}
	cfg := readOpenCodeConfig(t)
	if cfg["theme"] != "tokyonight" {
		t.Errorf("binding the worker lost the developer's own keys: %v", cfg)
	}
	agents, _ := cfg["agent"].(map[string]any)
	if _, found := agents["min-egen"]; !found {
		t.Errorf("binding the worker lost the developer's own agent: %v", agents)
	}

	// off takes it back out, and leaves the developer's agent where it was.
	if err := RemoveOpenCodeLocalProvider(); err != nil {
		t.Fatalf("RemoveOpenCodeLocalProvider: %v", err)
	}
	if got := workerModel(t); got != "" {
		t.Errorf("the worker agent is still bound to %q after off", got)
	}
	agents, _ = readOpenCodeConfig(t)["agent"].(map[string]any)
	if _, found := agents["min-egen"]; !found {
		t.Errorf("off removed the developer's own agent: %v", agents)
	}
}

// TestDispatchPolicyAndBindingArriveTogether: the fragment says the worker
// draws no AI credits, and only the binding makes that true. Registering
// one without the other is the sentence surviving into a session where it is
// false, so they are one write.
func TestDispatchPolicyAndBindingArriveTogether(t *testing.T) {
	withOpenCodeConfig(t)
	withLocalEnabled(t)
	m := aLocalModel(t)

	if err := EnsureOpenCodeLocalPolicy(m); err != nil {
		t.Fatalf("EnsureOpenCodeLocalPolicy: %v", err)
	}
	entries, _ := readOpenCodeConfig(t)["instructions"].([]any)
	if len(entries) != 1 {
		t.Fatalf("instructions = %v, want the dispatch policy registered", entries)
	}
	if want := LocalProviderID + "/" + m.Model; workerModel(t) != want {
		t.Errorf("the dispatch policy is registered but the worker is bound to %q, want %q", workerModel(t), want)
	}
}

// TestTurningLocalOffUnregistersTheDispatchPolicy: the entry outlives the
// session that wrote it, so a developer who turns the alpha off would otherwise
// keep reading "trekker ingen AI-credits" about a worker nothing
// dispatches to any more.
//
// What triggers it is local being off, not the session model being hosted. That
// was the defect: a hosted session model is the normal case for this feature —
// a cloud main agent dispatching to a local worker — so unregistering on it
// took the fragment back out of exactly the session it was written for.
func TestTurningLocalOffUnregistersTheDispatchPolicy(t *testing.T) {
	dir := withOpenCodeConfig(t)
	withLocalEnabled(t)
	m := withOwnServer(t)

	guard, err := startLocalDispatch("github-copilot/claude-opus-5")
	if err != nil {
		t.Fatal(err)
	}
	guard.Close()
	if _, found := readOpenCodeConfig(t)["instructions"]; !found {
		t.Fatalf("a cloud session with local dispatch on did not register the dispatch policy for %s", m.Model)
	}

	// The same launch path with the alpha turned off.
	local.SetEnabled(false)
	if _, err := startLocalDispatch("github-copilot/claude-opus-5"); err != nil {
		t.Fatalf("startLocalDispatch with local dispatch off: %v", err)
	}

	if _, found := readOpenCodeConfig(t)["instructions"]; found {
		t.Error("the dispatch policy is still registered after local dispatch was turned off")
	}
	if _, err := os.Stat(filepath.Join(dir, localPolicyFileName)); !os.IsNotExist(err) {
		t.Error("the dispatch policy file survived local dispatch being turned off")
	}
}

// TestDispatchPolicyTimingMatchesTheConfiguredTimeout: the fragment used to
// tell the main agent that a missing answer after two minutes meant failure,
// while the provider block waits ten. A dispatcher that gives up first can
// duplicate an edit that is still in flight.
func TestDispatchPolicyTimingMatchesTheConfiguredTimeout(t *testing.T) {
	m := aLocalModel(t)
	got := LocalDispatchPolicy(m, 5)
	want := fmt.Sprintf("%d minutter", chunkTimeoutMS(m)/60000)
	if !strings.Contains(got, want) {
		t.Errorf("the dispatch policy does not name the configured timeout (%q):\n%s", want, got)
	}
}

// withOwnServer stands in for a running local server and returns the model it
// serves. The ownership proof shells out to ps and lsof against a fixed port,
// which a test cannot arrange without taking that port on the machine it runs
// on — so the proof itself is held at its seam and the record behind it is
// real.
func withOwnServer(t *testing.T) local.Model {
	t.Helper()
	m := aLocalModel(t)
	t.Setenv("HOME", t.TempDir())
	if err := local.SaveState(local.State{PID: os.Getpid(), Model: m.Model, Started: time.Now()}); err != nil {
		t.Fatal(err)
	}
	orig := ensureOwnServer
	ensureOwnServer = func() error { return nil }
	t.Cleanup(func() { ensureOwnServer = orig })
	return m
}

// TestCloudSessionGetsTheLocalWorker is the gap this file was written around.
//
// The feature exists so a cloud main agent can hand focused tasks to a local
// worker, and the setup used to be gated on the *session* model being local —
// so the one session the worker is for got no provider block, no binding and no
// dispatch policy. Manual testing missed it because a single earlier launch on
// a local model left all three behind in a config file that outlives the
// session; a developer who never ran one had them at no point.
func TestCloudSessionGetsTheLocalWorker(t *testing.T) {
	dir := withOpenCodeConfig(t)
	withLocalEnabled(t)
	m := withOwnServer(t)

	guard, err := startLocalDispatch("github-copilot/claude-opus-5")
	if err != nil {
		t.Fatalf("startLocalDispatch for a cloud session with local dispatch on: %v", err)
	}
	if guard == nil {
		t.Fatal("no loop guard was started for a cloud session with local dispatch on; the worker's completions would go to the server unguarded")
	}
	defer guard.Close()

	cfg := readOpenCodeConfig(t)
	providers, _ := cfg["provider"].(map[string]any)
	if _, found := providers[LocalProviderID]; !found {
		t.Errorf("the local provider block is missing after a cloud session launch: %v", cfg)
	}
	if want := LocalProviderID + "/" + m.Model; workerModel(t) != want {
		t.Errorf("the worker agent is bound to %q after a cloud session launch, want %q", workerModel(t), want)
	}
	policy := filepath.Join(dir, localPolicyFileName)
	entries, _ := cfg["instructions"].([]any)
	if len(entries) != 1 || entries[0] != policy {
		t.Errorf("instructions = %v after a cloud session launch, want the dispatch policy %q", entries, policy)
	}
	body, err := os.ReadFile(policy)
	if err != nil {
		t.Fatalf("the dispatch policy was not written for a cloud session: %v", err)
	}
	if !strings.Contains(string(body), m.Model) {
		t.Errorf("the dispatch policy names something other than the running model %q:\n%s", m.Model, body)
	}
}

// TestNoLocalServerLeavesACloudSessionLaunchable pins the decision for dispatch
// on with nothing running.
//
// A cloud session is not refused: it only loses the worker, and a developer who
// left the alpha on and has not started a server today still has a session
// worth launching — refusing would make `alpha local off` something to remember
// before every cloud launch. What it does lose is the claim: no guard, and the
// dispatch policy comes back out, because the fragment tells the main agent the
// worker is free and there is no worker.
//
// A session running *on* the local model is refused, because there is nothing
// else for its prompts to run on.
func TestNoLocalServerLeavesACloudSessionLaunchable(t *testing.T) {
	dir := withOpenCodeConfig(t)
	withLocalEnabled(t)
	m := aLocalModel(t)
	t.Setenv("HOME", t.TempDir()) // nothing recorded, and no server to record

	// What an earlier session, with a server up, left behind.
	if err := EnsureOpenCodeLocalPolicy(m); err != nil {
		t.Fatal(err)
	}

	guard, err := startLocalDispatch("github-copilot/claude-opus-5")
	if err != nil {
		t.Fatalf("a cloud session with dispatch on and no server was refused: %v", err)
	}
	if guard != nil {
		guard.Close()
		t.Error("a loop guard was started with no server of nav-pilot's own behind it")
	}
	if _, found := readOpenCodeConfig(t)["instructions"]; found {
		t.Error("the dispatch policy is still registered with no local server running")
	}
	if _, err := os.Stat(filepath.Join(dir, localPolicyFileName)); !os.IsNotExist(err) {
		t.Error("the dispatch policy file survived a session with no local server")
	}

	if _, err := startLocalDispatch(m.Model); err == nil {
		t.Error("a launch on the local model itself was allowed with no server running; every prompt of that session would fail")
	}
}
