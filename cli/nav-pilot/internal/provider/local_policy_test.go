package provider

import (
	"encoding/json"
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
