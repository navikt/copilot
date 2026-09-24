package cli

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

type fakeTok struct {
	tok string
	p   float64
}

// fakeDecideServer points decide at an httptest server that answers every
// completion with answer(prompt) as the top logprobs, and returns the last
// request body it saw.
func fakeDecideServer(t *testing.T, answer func(prompt string) []fakeTok) func() map[string]any {
	t.Helper()
	localTestHome(t)
	var mu sync.Mutex
	var last map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		mu.Lock()
		last = body
		mu.Unlock()
		prompt := body["messages"].([]any)[0].(map[string]any)["content"].(string)
		var top []map[string]any
		for _, tk := range answer(prompt) {
			top = append(top, map[string]any{"token": tk.tok, "logprob": math.Log(tk.p)})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []any{map[string]any{"logprobs": map[string]any{"content": []any{map[string]any{"top_logprobs": top}}}}},
			"usage":   map[string]any{"prompt_tokens": 50, "completion_tokens": 1},
		})
	}))
	t.Cleanup(srv.Close)
	orig := decideServer
	decideServer = func(context.Context) (string, string, func(), error) {
		return srv.URL, "fake-model", func() {}, nil
	}
	t.Cleanup(func() { decideServer = orig })
	return func() map[string]any { mu.Lock(); defer mu.Unlock(); return last }
}

func yesMostly(string) []fakeTok {
	return []fakeTok{{"A", 0.5}, {"ĠA", 0.2}, {"B", 0.1}, {"Hello", 0.1}}
}

// runDecide runs the command as main would and returns stdout, stderr and the
// exit code.
func runDecide(t *testing.T, args ...string) (string, string, int) {
	t.Helper()
	var code int
	out, errOut := captureRun(t, func() {
		code = exitCodeFor(run(append([]string{"alpha", "decide"}, args...)))
	})
	return out, errOut, code
}

func TestDecideNormalisesOverOptions(t *testing.T) {
	last := fakeDecideServer(t, yesMostly)
	d, err := decide(context.Background(), "Is it?", []string{"yes", "no"}, "", false)
	if err != nil {
		t.Fatal(err)
	}
	// "Hello" is not a letter and drops out; A and ĠA add up.
	if d.Choice != "yes" || math.Abs(d.P["yes"]-0.875) > 1e-9 || math.Abs(d.P["no"]-0.125) > 1e-9 {
		t.Errorf("got %+v, want yes 0.875 / no 0.125", d)
	}
	body := last()
	if body["max_tokens"] != 1.0 || body["logprobs"] != true || body["top_logprobs"] != float64(decideTopLogprobs) || body["temperature"] != 0.0 {
		t.Errorf("request = %v", body)
	}
	if body["chat_template_kwargs"].(map[string]any)["enable_thinking"] != false {
		t.Errorf("thinking not off: %v", body["chat_template_kwargs"])
	}
	prompt := body["messages"].([]any)[0].(map[string]any)["content"].(string)
	if !strings.Contains(prompt, "A: yes\nB: no\n") || strings.Contains(prompt, "EVIDENCE") {
		t.Errorf("prompt = %q", prompt)
	}
}

func TestLetterIndex(t *testing.T) {
	for tok, want := range map[string]int{"A": 0, " B": 1, "ĠC": 2, "▁D": 3, "Z": 25, "a": -1, "AB": -1, "": -1, "Ġ": -1, "1": -1} {
		if got := letterIndex(tok); got != want {
			t.Errorf("letterIndex(%q) = %d, want %d", tok, got, want)
		}
	}
}

func TestDecideMissingOptionsGetZero(t *testing.T) {
	fakeDecideServer(t, func(string) []fakeTok { return []fakeTok{{"A", 0.3}, {"B", 0.1}, {"E", 0.5}} })
	d, err := decide(context.Background(), "Which?", []string{"a", "b", "c"}, "", false)
	if err != nil {
		t.Fatal(err)
	}
	if p, ok := d.P["c"]; !ok || p != 0 {
		t.Errorf("p[c] = %v (present %v), want 0 and present", p, ok)
	}
	if math.Abs(d.P["a"]-0.75) > 1e-9 {
		t.Errorf("p[a] = %v, want 0.75 (E is not an option and must not count)", d.P["a"])
	}
}

func TestDecideNoOptionLetterIsAnError(t *testing.T) {
	fakeDecideServer(t, func(string) []fakeTok { return []fakeTok{{"The", 0.9}} })
	if _, _, code := runDecide(t, "Is it?", "--options", "yes,no"); code != 2 {
		t.Errorf("exit = %d, want 2", code)
	}
}

func TestDecideEvidenceFlagAndWarning(t *testing.T) {
	last := fakeDecideServer(t, yesMostly)

	out, errOut, code := runDecide(t, "Is it?", "--options", "yes,no")
	var d decision
	if err := json.Unmarshal([]byte(out), &d); err != nil || code != 0 {
		t.Fatalf("stdout %q, exit %d: %v", out, code, err)
	}
	if d.Evidence || !strings.Contains(errOut, "without evidence") {
		t.Errorf("no evidence: evidence=%v stderr=%q", d.Evidence, errOut)
	}

	ev := filepath.Join(t.TempDir(), "msg.txt")
	if err := os.WriteFile(ev, []byte("feat: add decide"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, errOut, _ = runDecide(t, "Is it?", "--options", "yes,no", "--evidence", ev)
	_ = json.Unmarshal([]byte(out), &d)
	if !d.Evidence || strings.Contains(errOut, "without evidence") {
		t.Errorf("with evidence: evidence=%v stderr=%q", d.Evidence, errOut)
	}
	prompt := last()["messages"].([]any)[0].(map[string]any)["content"].(string)
	if !strings.Contains(prompt, "<<<EVIDENCE\nfeat: add decide\nEVIDENCE>>>") {
		t.Errorf("prompt = %q", prompt)
	}
}

func TestDecideEvidenceIsCapped(t *testing.T) {
	got := truncateEvidence(strings.Repeat("å", decideEvidenceCap)) // 2 bytes each
	if !strings.HasSuffix(got, "[evidence truncated at 32 KiB]") || len(got) > decideEvidenceCap+40 {
		t.Errorf("len %d, suffix %q", len(got), got[len(got)-40:])
	}
	if strings.ContainsRune(got, '�') {
		t.Error("truncation split a rune")
	}
}

func TestDecideThresholdExitCodes(t *testing.T) {
	fakeDecideServer(t, yesMostly) // p(yes) = 0.875
	for _, tc := range []struct {
		threshold string
		want      int
	}{{"0.8", 0}, {"0.875", 0}, {"0.9", 1}} {
		if _, _, code := runDecide(t, "Is it?", "--options", "yes,no", "--threshold", tc.threshold, "--expect", "yes"); code != tc.want {
			t.Errorf("threshold %s: exit %d, want %d", tc.threshold, code, tc.want)
		}
	}
	for _, args := range [][]string{
		{"--threshold", "0.5"},                  // no --expect
		{"--threshold", "0.5", "--expect", "x"}, // not an option
		{"--threshold", "2", "--expect", "yes"},
		{"--threshold", "-0.1"}, // was silently ignored
		{"--threshold", "-0.1", "--expect", "yes"},
	} {
		if _, _, code := runDecide(t, append([]string{"Is it?", "--options", "yes,no"}, args...)...); code != 2 {
			t.Errorf("%v: exit %d, want 2", args, code)
		}
	}
}

func TestDecideValidatesOptions(t *testing.T) {
	fakeDecideServer(t, yesMostly)
	for _, opts := range []string{"yes", "yes,yes", "yes,,no", strings.Repeat("x,", 26) + "y"} {
		if _, _, code := runDecide(t, "Is it?", "--options", opts); code != 2 {
			t.Errorf("--options %q: exit %d, want 2", opts, code)
		}
	}
}

func TestDecideTimeout(t *testing.T) {
	localTestHome(t)
	busy := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select { // a server busy with someone else's session
		case <-r.Context().Done():
		case <-busy:
		}
	}))
	t.Cleanup(srv.Close)
	t.Cleanup(func() { close(busy) }) // runs before srv.Close, which waits for handlers
	orig := decideServer
	decideServer = func(context.Context) (string, string, func(), error) { return srv.URL, "m", func() {}, nil }
	t.Cleanup(func() { decideServer = orig })

	started := time.Now()
	_, errOut, code := runDecide(t, "Is it?", "--options", "yes,no", "--timeout", "100ms")
	if code != 2 || !strings.Contains(errOut, "no decision within 100ms") {
		t.Errorf("exit %d, stderr %q", code, errOut)
	}
	if time.Since(started) > 5*time.Second {
		t.Errorf("took %s, the timeout was not honoured", time.Since(started))
	}
}

func TestDecideNoServer(t *testing.T) {
	localTestHome(t) // nothing recorded in this HOME; decideServer is the real one
	_, errOut, code := runDecide(t, "Is it?", "--options", "yes,no")
	if code != 2 || !strings.Contains(errOut, "no local server is recorded as running") || !strings.Contains(errOut, "does not start one") {
		t.Errorf("exit %d, stderr %q", code, errOut)
	}
}

func TestDecideEval(t *testing.T) {
	// The model says "yes" (A) whenever the evidence starts like a conventional
	// commit, fairly sure; otherwise "no", less sure.
	fakeDecideServer(t, func(prompt string) []fakeTok {
		if strings.Contains(prompt, "EVIDENCE\nfeat") || strings.Contains(prompt, "EVIDENCE\nfix") {
			return []fakeTok{{"A", 0.9}, {"B", 0.1}}
		}
		return []fakeTok{{"A", 0.4}, {"B", 0.6}}
	})
	cases := strings.Join([]string{
		`{"question":"Conventional commit?","evidence":"feat: x","expect":"yes"}`,
		`{"question":"Conventional commit?","evidence":"fix: y","expect":"yes"}`,
		`{"question":"Conventional commit?","evidence":"Update stuff","expect":"no"}`,
		`{"question":"Conventional commit?","evidence":"chore: z","expect":"yes"}`, // the model gets this one wrong
		``,
		`{"question":"Conventional commit?","options":["no","yes"],"expect":"no"}`, // own options, no evidence: A=no
	}, "\n")
	path := filepath.Join(t.TempDir(), "cases.jsonl")
	if err := os.WriteFile(path, []byte(cases), 0o644); err != nil {
		t.Fatal(err)
	}
	out, errOut, code := runDecide(t, "--eval", path, "--options", "yes,no", "--json")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	var r evalReport
	if err := json.Unmarshal([]byte(out), &r); err != nil {
		t.Fatalf("%q: %v", out, err)
	}
	// Case 5 has options no,yes and no evidence: the model answers B = yes, wrong.
	if r.Cases != 5 || r.Correct != 3 || math.Abs(r.Accuracy-0.6) > 1e-9 {
		t.Errorf("cases/correct/accuracy = %d/%d/%v, want 5/3/0.6", r.Cases, r.Correct, r.Accuracy)
	}
	if r.Confusion["yes"]["yes"] != 2 || r.Confusion["yes"]["no"] != 1 || r.Confusion["no"]["no"] != 1 || r.Confusion["no"]["yes"] != 1 {
		t.Errorf("confusion = %v", r.Confusion)
	}
	if math.Abs(r.MeanPCorrect-0.8) > 1e-9 || math.Abs(r.MeanPWrong-0.6) > 1e-9 {
		t.Errorf("mean p correct/wrong = %v/%v, want 0.8/0.6", r.MeanPCorrect, r.MeanPWrong)
	}
	if r.WithoutEvidence != 1 {
		t.Errorf("without evidence = %d, want 1", r.WithoutEvidence)
	}

	if _, _, code := runDecide(t, "--eval", path); code != 2 {
		t.Errorf("cases without options and no --options: exit %d, want 2", code)
	}
}

func TestEvalLatencyPercentiles(t *testing.T) {
	var cases []evalCase
	var ds []decision
	for i := 1; i <= 20; i++ {
		cases = append(cases, evalCase{Expect: "a"})
		ds = append(ds, decision{Choice: "a", P: map[string]float64{"a": 1}, MS: int64(i * 10)})
	}
	r := evalMetrics(cases, ds)
	if r.P50MS != 100 || r.P95MS != 190 {
		t.Errorf("p50/p95 = %d/%d, want 100/190", r.P50MS, r.P95MS)
	}
}
