package provider

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
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
	got := LocalDispatchPolicy(m, local.DispatchBalanced, 3, 5)

	if !strings.Contains(got, m.Model) {
		t.Errorf("the dispatch policy does not name the model %q:\n%s", m.Model, got)
	}
	if !strings.Contains(got, " 3 identical calls in a row that got the same result back") {
		t.Errorf("the dispatch policy does not name the same-result threshold 3:\n%s", got)
	}
	if !strings.Contains(got, " 5 identical calls whatever they return") {
		t.Errorf("the dispatch policy does not name the configured backstop 5:\n%s", got)
	}
	if strings.Contains(got, strconv.Itoa(local.DefaultLoopGuardRepeat)+" identical calls") {
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
	first := LocalDispatchPolicy(m, local.DispatchBalanced, 4, 8)
	for i := range 20 {
		if got := LocalDispatchPolicy(m, local.DispatchBalanced, 4, 8); got != first {
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
// without this block a cloud main agent dispatching to `local-worker` spends
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
	got := LocalDispatchPolicy(m, local.DispatchBalanced, 3, 5)
	want := fmt.Sprintf("%d minutes", chunkTimeoutMS(m)/60000)
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

// policyModel is a fixed entry, so the golden text below cannot move when the
// embedded manifest does.
func policyModel(c *local.Capabilities) local.Model {
	return local.Model{
		Model:        "mlx-community/Some-Model-4bit",
		Role:         "Sub-agent worker",
		Expect:       "Returns in seconds.",
		Params:       map[string]string{"MLX_OPENCODE_CHUNK_TIMEOUT": "600000"},
		Capabilities: c,
	}
}

// TestLocalDispatchPolicyWithoutCapabilitiesIsUnchanged: a manifest without
// the capabilities block must produce exactly the text earlier releases wrote,
// so nothing changes for anyone until a manifest with the block ships. The
// golden file was generated by the code before the block existed.
func TestLocalDispatchPolicyWithoutCapabilitiesIsUnchanged(t *testing.T) {
	want, err := os.ReadFile(filepath.Join("testdata", "dispatch_policy_no_capabilities.golden"))
	if err != nil {
		t.Fatal(err)
	}
	if got := LocalDispatchPolicy(policyModel(nil), local.DispatchBalanced, 3, 5); got != string(want) {
		t.Errorf("the no-capabilities policy moved:\n%s\n--- want ---\n%s", got, want)
	}
}

// TestLocalDispatchPolicyFromCapabilities: with only mechanical multi-file
// edits trusted for delegation, that is the only thing the worker is sent, and
// every other class is named as work to keep. The loop-guard lines stay.
func TestLocalDispatchPolicyFromCapabilities(t *testing.T) {
	c := &local.Capabilities{Classes: map[string]local.ClassVerdict{
		"read-qa":               {Delegate: local.VerdictCloud, Local: local.VerdictNotYet},
		"edit-single":           {Delegate: local.VerdictCloud, Local: local.VerdictNotYet},
		"edit-multi-mechanical": {Delegate: local.VerdictTrusted, Local: local.VerdictCloud},
		"create-file":           {Delegate: local.VerdictCloud, Local: local.VerdictCloud},
		"debug":                 {Delegate: local.VerdictCloud, Local: local.VerdictCloud},
	}}
	got := LocalDispatchPolicy(policyModel(c), local.DispatchBalanced, 3, 5)

	// "Large" and no rename: Sonnet 5 read "several files" as a floor it then
	// argued 2 files were under, and a rename is one sed that costs the same
	// sent (pending-tasks §8.8, dispatch probe 2).
	send := "Send it: large mechanical changes that follow one pattern across many files or call sites, such as a parameter threaded through its call sites.\n"
	keep := "Do not send it: questions about the code and explanations of it; comments, log lines and other single-file edits; new files, tests included; debugging; changes needing a judgement per file; a task it would need several exchanges with you to finish; changes where a wrong edit is expensive.\n"
	// splitMulti is there because edit-multi-mechanical is trusted: the worker
	// is reliable when told each place, so large jobs go out one file at a time.
	for _, want := range []string{send, sendTrusted, splitMulti, keep,
		"5 or more files or 10 or more call sites", "one task per file",
		" 3 identical calls in a row that got the same result back",
		" 5 identical calls whatever they return",
		"It often says no and changes nothing."} {
		if !strings.Contains(got, want) {
			t.Errorf("the generated policy lacks %q:\n%s", want, got)
		}
	}
	for _, gone := range []string{"lookups in the code", "a single test file",
		// The threshold and the doubt clauses kept Sonnet 5 from ever dispatching.
		"cheaper to make yourself", "if you doubt", "judge this correctly",
		// Sending one-step jobs cost the same credits and two to three times the time.
		"even when you could do them in one or two steps",
		// A per-file split is many rounds by design; the keep line must not veto it.
		"tasks needing many rounds"} {
		if strings.Contains(got, gone) {
			t.Errorf("the generated policy still sends %q, which no verdict trusts:\n%s", gone, got)
		}
	}
	if got != LocalDispatchPolicy(policyModel(c), local.DispatchBalanced, 3, 5) {
		t.Error("the generated policy is not byte-stable across calls")
	}
}

// TestLocalDispatchPolicyIgnoresWhatItDoesNotKnow: a newer generator may add
// a class or a verdict. Neither may send anything to the worker: an unknown
// class is not named at all, and an unknown or mis-cased verdict is not trust.
func TestLocalDispatchPolicyIgnoresWhatItDoesNotKnow(t *testing.T) {
	c := &local.Capabilities{Classes: map[string]local.ClassVerdict{
		"refactor-large": {Delegate: local.VerdictTrusted},
		"read-qa":        {Delegate: "trusted-soon"},
		"debug":          {Delegate: "TRUSTED"},
	}}
	got := LocalDispatchPolicy(policyModel(c), local.DispatchBalanced, 3, 5)
	if !strings.Contains(got, "Send it nothing for now") {
		t.Errorf("an unknown class or verdict was treated as trusted:\n%s", got)
	}
	if strings.Contains(got, "refactor-large") || strings.Contains(got, sendTrusted) || strings.Contains(got, splitMulti) {
		t.Errorf("the policy names an unknown class or tells the agent how to send work:\n%s", got)
	}
	if !strings.Contains(got, "questions about the code and explanations of it") || !strings.Contains(got, "; debugging;") {
		t.Errorf("classes with unknown verdicts are not kept on the cloud:\n%s", got)
	}
}

// TestLocalDispatchPolicySplitsOnlyTrustedMultiFileEdits: the size rule and
// the per-file split describe mechanical multi-file edits. A worker trusted
// with something else gets neither, so a one-file job of a trusted class is
// not kept for being small, and nothing is sent that no verdict trusts.
func TestLocalDispatchPolicySplitsOnlyTrustedMultiFileEdits(t *testing.T) {
	c := &local.Capabilities{Classes: map[string]local.ClassVerdict{
		"edit-single":           {Delegate: local.VerdictTrusted},
		"edit-multi-mechanical": {Delegate: local.VerdictNotYet},
	}}
	got := LocalDispatchPolicy(policyModel(c), local.DispatchBalanced, 3, 5)
	if !strings.Contains(got, "Send it: a fully specified edit to one file, such as a comment or a log line.\n") || !strings.Contains(got, sendTrusted) {
		t.Errorf("a trusted edit-single is not sent:\n%s", got)
	}
	if strings.Contains(got, splitMulti) || strings.Contains(got, "call sites") || strings.Contains(got, "Do smaller ones yourself") {
		t.Errorf("the policy sizes or sends multi-file edits no verdict trusts:\n%s", got)
	}
}

// TestLocalDispatchPolicyWithCapabilitiesGolden pins the whole text for the
// shipped verdicts (only edit-multi-mechanical trusted), byte for byte: it is
// the system-prompt prefix a session caches, and substring checks miss a
// stray byte. Regenerate with UPDATE_GOLDEN=1 when the text changes on purpose,
// and say why in the commit.
func TestLocalDispatchPolicyWithCapabilitiesGolden(t *testing.T) {
	c := &local.Capabilities{Classes: map[string]local.ClassVerdict{
		"edit-multi-mechanical": {Delegate: local.VerdictTrusted},
	}}
	got := LocalDispatchPolicy(policyModel(c), local.DispatchBalanced, 3, 5)
	path := filepath.Join("testdata", "dispatch_policy_edit_multi_trusted.golden")
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got != string(want) {
		t.Errorf("the capabilities policy moved:\n%s\n--- want ---\n%s", got, want)
	}
}

// TestLocalDispatchPolicyPerLevel pins what local_dispatch changes in the
// text. balanced and aggressive keep the sizes and describe the gate that
// enforces them, and say the persona's tiers do not decide who writes files;
// aggressive also describes the create-file rule when that class is trusted.
// conservative doubles the sizes and hands the decision to the orchestrator.
// No level mentions a rule for a class the manifest does not trust.
func TestLocalDispatchPolicyPerLevel(t *testing.T) {
	both := policyModel(&local.Capabilities{Classes: map[string]local.ClassVerdict{
		"edit-multi-mechanical": {Delegate: local.VerdictTrusted},
		"create-file":           {Delegate: local.VerdictTrusted},
	}})
	for _, level := range []string{local.DispatchConservative, local.DispatchBalanced, local.DispatchAggressive} {
		got := LocalDispatchPolicy(both, level, 3, 5)
		path := filepath.Join("testdata", "dispatch_policy_"+level+".golden")
		if os.Getenv("UPDATE_GOLDEN") == "1" {
			if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		want, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if got != string(want) {
			t.Errorf("the %s policy moved:\n%s\n--- want ---\n%s", level, got, want)
		}
	}

	balanced := LocalDispatchPolicy(both, local.DispatchBalanced, 3, 5)
	aggressive := LocalDispatchPolicy(both, local.DispatchAggressive, 3, 5)
	for level, text := range map[string]string{"balanced": balanced, "aggressive": aggressive} {
		if !strings.Contains(text, splitMulti) {
			t.Errorf("%s does not carry the 5-file / 10-call-site sizes", level)
		}
		if !strings.Contains(text, "nav-pilot enforces this in every tier") || !strings.Contains(text, "Trivial and Compressed tiers") {
			t.Errorf("%s must say the gate enforces the split in every tier", level)
		}
	}
	if strings.Contains(balanced, "A new file you would write yourself") {
		t.Error("balanced describes the create-file rule, which only aggressive runs")
	}
	if !strings.Contains(aggressive, "A new file you would write yourself") {
		t.Error("aggressive with create-file trusted does not describe the create-file rule")
	}
	conservative := LocalDispatchPolicy(both, local.DispatchConservative, 3, 5)
	if strings.Contains(conservative, "enforces") || strings.Contains(conservative, splitMulti) || !strings.Contains(conservative, "10 or more files") {
		t.Error("conservative must send only at 10 files or 20 call sites, unenforced")
	}
	if !strings.Contains(conservative, "If you doubt the worker can do a task, do it yourself") {
		t.Error("conservative must leave the decision with the orchestrator")
	}

	single := policyModel(&local.Capabilities{Classes: map[string]local.ClassVerdict{
		"edit-single": {Delegate: local.VerdictTrusted},
	}})
	for _, level := range []string{local.DispatchBalanced, local.DispatchAggressive} {
		if got := LocalDispatchPolicy(single, level, 3, 5); strings.Contains(got, "enforces") {
			t.Errorf("%s mentions the gate with no class it has a rule for trusted", level)
		}
	}
	if LocalDispatchPolicy(policyModel(nil), local.DispatchAggressive, 3, 5) != LocalDispatchPolicy(policyModel(nil), local.DispatchBalanced, 3, 5) {
		t.Error("a manifest without capabilities must get the same text at every level")
	}
}

// withDispatch sets local_dispatch for one test, and a manifest whose model is
// trusted with mechanical multi-file edits, which is what the gate enforces.
func withDispatch(t *testing.T, level string) {
	t.Helper()
	prev := local.DispatchLevel()
	local.SetDispatchLevel(level)
	t.Cleanup(func() { local.SetDispatchLevel(prev) })
	orig := local.Active()
	m := *orig
	m.Models = slices.Clone(orig.Models)
	for i := range m.Models {
		m.Models[i].Capabilities = &local.Capabilities{Classes: map[string]local.ClassVerdict{
			"edit-multi-mechanical": {Delegate: local.VerdictTrusted},
		}}
	}
	local.SetActive(&m)
	t.Cleanup(func() { local.SetActive(orig) })
}

// TestDispatchGatePerLevel: the gate and its plugin per level. The plugin
// file is written with the policy at every level (it is inert without the
// variable), and balanced and aggressive turn the gate on for a cloud session.
func TestDispatchGatePerLevel(t *testing.T) {
	for _, tc := range []struct {
		level string
		gate  bool
	}{
		{local.DispatchConservative, false},
		{local.DispatchBalanced, true},
		{local.DispatchAggressive, true},
	} {
		t.Run(tc.level, func(t *testing.T) {
			withOpenCodeConfig(t)
			withLocalEnabled(t)
			withDispatch(t, tc.level)
			m := withOwnServer(t)

			guard, err := startLocalDispatch("github-copilot/claude-sonnet-5")
			if err != nil || guard == nil {
				t.Fatalf("startLocalDispatch: %v, %v", guard, err)
			}
			defer guard.Close()
			if got := guard.GateURL() != ""; got != tc.gate {
				t.Errorf("gate on = %v at %s, want %v", got, tc.level, tc.gate)
			}
			if _, err := os.Stat(dispatchGatePluginPath()); err != nil {
				t.Errorf("the gate plugin was not written at %s: %v", tc.level, err)
			}

			// A local session has no one to dispatch to: never a gate.
			local2, err := startLocalDispatch(m.Model)
			if err != nil {
				t.Fatal(err)
			}
			defer local2.Close()
			if local2.GateURL() != "" {
				t.Error("the gate is on for a session running on the local model itself")
			}
		})
	}
}

// TestDispatchOffOffersNoWorker: off takes the policy, the plugin and the
// binding out for a cloud session, and starts no guard.
func TestDispatchOffOffersNoWorker(t *testing.T) {
	withOpenCodeConfig(t)
	withLocalEnabled(t)
	withDispatch(t, local.DispatchBalanced)
	m := withOwnServer(t)
	if g, err := startLocalDispatch("github-copilot/claude-sonnet-5"); err != nil || g == nil {
		t.Fatalf("setup: %v %v", g, err)
	} else {
		g.Close()
	}

	local.SetDispatchLevel(local.DispatchOff)
	guard, err := startLocalDispatch("github-copilot/claude-sonnet-5")
	if err != nil {
		t.Fatal(err)
	}
	if guard != nil {
		guard.Close()
		t.Error("a guard was started at local_dispatch = off")
	}
	if _, err := os.Stat(localPolicyPath()); !os.IsNotExist(err) {
		t.Error("the dispatch policy survived local_dispatch = off")
	}
	if workerModel(t) != "" {
		t.Errorf("the worker is still bound to %q at local_dispatch = off", workerModel(t))
	}
	if local.WorkerOffered() {
		t.Error("WorkerOffered at local_dispatch = off")
	}
	// A session on the local model itself still runs, and is offered no
	// worker: no policy, no binding.
	g, err := startLocalDispatch(m.Model)
	if err != nil || g == nil {
		t.Fatalf("a local session was refused at local_dispatch = off: %v", err)
	}
	g.Close()
	if _, err := os.Stat(localPolicyPath()); !os.IsNotExist(err) {
		t.Error("a local session at local_dispatch = off got the dispatch policy")
	}
	if workerModel(t) != "" {
		t.Errorf("a local session at local_dispatch = off bound the worker to %q", workerModel(t))
	}
}
