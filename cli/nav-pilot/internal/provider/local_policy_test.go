package provider

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

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
	id := aLocalModelID(t)

	// Something of the developer's own, in the same key.
	mine := filepath.Join(dir, "min-egen.md")
	if err := os.WriteFile(ConfigPathOverride, []byte(`{"instructions":["`+mine+`"],"theme":"tokyonight"}`), 0o600); err != nil {
		t.Fatal(err)
	}

	for range 3 {
		if err := EnsureOpenCodeLocalPolicy(id); err != nil {
			t.Fatalf("EnsureOpenCodeLocalPolicy: %v", err)
		}
	}

	policy := filepath.Join(dir, localPolicyFileName)
	body, err := os.ReadFile(policy)
	if err != nil {
		t.Fatalf("the dispatch policy was not written: %v", err)
	}
	if !strings.Contains(string(body), id) {
		t.Errorf("the written dispatch policy does not name %q:\n%s", id, body)
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

// TestLocalDisabledWritesNoDispatchPolicy is the no-op proof: with dispatch off
// there is no file, no entry, and nothing in the config that was not already
// there. It is gated on the same predicate as the loop guard, so this cannot
// pass while the launch quietly writes one anyway.
func TestLocalDisabledWritesNoDispatchPolicy(t *testing.T) {
	dir := withOpenCodeConfig(t)

	if err := EnsureOpenCodeLocalPolicy(aLocalModelID(t)); err != nil {
		t.Fatalf("EnsureOpenCodeLocalPolicy with local dispatch off: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, localPolicyFileName)); !os.IsNotExist(err) {
		t.Error("a dispatch policy was written with local dispatch off")
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
	if err := EnsureOpenCodeLocalPolicy(aLocalModelID(t)); err != nil {
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

	if err := EnsureOpenCodeLocalPolicy(aLocalModelID(t)); err != nil {
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
// the premium requests the dispatch policy tells it are free.
func TestLocalWorkerIsBoundToTheLocalModel(t *testing.T) {
	withOpenCodeConfig(t)
	withLocalEnabled(t)
	m := aLocalModel(t)

	// Something of the developer's own, in the same file and the same key.
	if err := os.WriteFile(ConfigPathOverride,
		[]byte(`{"theme":"tokyonight","agent":{"min-egen":{"model":"github-copilot/gpt-5"}}}`), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := EnsureOpenCodeLocalProvider(m); err != nil {
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
// costs no premium requests, and only the binding makes that true. Registering
// one without the other is the sentence surviving into a session where it is
// false, so they are one write.
func TestDispatchPolicyAndBindingArriveTogether(t *testing.T) {
	withOpenCodeConfig(t)
	withLocalEnabled(t)
	m := aLocalModel(t)

	if err := EnsureOpenCodeLocalPolicy(m.Model); err != nil {
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

// TestHostedLaunchUnregistersTheDispatchPolicy: the entry outlives the session
// that wrote it. A developer who launches local once and hosted afterwards
// would otherwise keep reading "koster ingen premium-forespørsler" about a
// worker that now bills every dispatch to their session model.
func TestHostedLaunchUnregistersTheDispatchPolicy(t *testing.T) {
	dir := withOpenCodeConfig(t)
	withLocalEnabled(t)

	if err := EnsureOpenCodeLocalPolicy(aLocalModelID(t)); err != nil {
		t.Fatal(err)
	}

	// The same launch path, with a model that is not served locally.
	if err := EnsureOpenCodeLocalPolicy("gpt-5"); err != nil {
		t.Fatalf("EnsureOpenCodeLocalPolicy on a hosted launch: %v", err)
	}

	if _, found := readOpenCodeConfig(t)["instructions"]; found {
		t.Error("the dispatch policy is still registered after a hosted launch")
	}
	if _, err := os.Stat(filepath.Join(dir, localPolicyFileName)); !os.IsNotExist(err) {
		t.Error("the dispatch policy file survived a hosted launch")
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
