package local

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestShellEdits(t *testing.T) {
	for _, tc := range []struct {
		cmd      string
		scripted bool
		files    []string
	}{
		// Probe 4, rung 4: the whole job in one call.
		{`grep -rl "configuredJacksonMapper(" src | while read f; do name=$(basename "$f" .kt); sed -i '' "s/configuredJacksonMapper(/configuredJacksonMapper(\"$name\", /g" "$f"; done`, true, nil},
		{`for f in a.kt b.kt; do sed -i 's/x/y/' $f; done`, true, nil},
		{"sed -i \"s/x/`date`/\" a.kt", false, []string{"a.kt"}},
		// One literal file: counted.
		{`sed -i '' 's/foo(/foo("A", /' src/A.kt`, false, []string{"src/A.kt"}},
		{`sed -i.bak -e 's/a/b/' -e 's/c/d/' src/B.kt && echo ok`, false, []string{"src/B.kt"}},
		{`perl -pi -e 's/a/b/' C.kt`, false, []string{"C.kt"}},
		{`cd src && sed -i 's/a/b/' A.kt; sed -i 's/c/d/' B.kt`, false, []string{"A.kt", "B.kt"}},
		// A $ inside single quotes is a regex anchor, not a variable.
		{`sed -i 's/foo$/bar/' A.kt`, false, []string{"A.kt"}},
		// A computed value on one literal file is one edit of that file.
		{`sed -i "s/version/$NEW/" package.json`, false, []string{"package.json"}},
		{`sed -i "s/foo$/bar/" A.kt`, false, []string{"A.kt"}},
		// A computed file is not.
		{`sed -i 's/a/b/' "$REPO/A.kt"`, true, nil},
		{`sed -i "s/x/\"y\"/" A.kt`, false, []string{"A.kt"}},
		{`perl -Ilib -Mstrict -e 'print 1' A.kt`, false, nil},
		// One search-and-replace over many files: not counted.
		{`sed -i 's/a/b/' A.kt B.kt C.kt`, false, nil},
		{`grep -rl foo src | xargs sed -i 's/foo/bar/g'`, false, nil},
		{`find src -name '*.kt' -exec sed -i 's/a/b/' {} \;`, false, nil},
		// Not in place.
		{`sed -n '1,5p' A.kt`, false, nil},
		{`sed 's/a/b/' A.kt > B.kt`, false, nil},
		{`git commit -m "fix for x"`, false, nil},
		{`perl -ne 'print' A.kt`, false, nil},
	} {
		scripted, files := ShellEdits(tc.cmd)
		if scripted != tc.scripted || !slices.Equal(files, tc.files) {
			t.Errorf("ShellEdits(%q) = %v, %q; want %v, %q", tc.cmd, scripted, files, tc.scripted, tc.files)
		}
	}
}

func testGate(t *testing.T, up bool, rules GateRules) *dispatchGate {
	// Edits count only files that exist; the gate stats the path, which the
	// plugin makes absolute, and only under the project root.
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	root = dir
	rules.Root = dir
	g := newDispatchGate("", rules)
	g.serverUp = func() bool { return up }
	if err := os.MkdirAll("src", 0o755); err != nil {
		t.Fatal(err)
	}
	for _, d := range []string{"F", "G"} {
		for i := 0; i <= 12; i++ {
			for _, p := range []string{fmt.Sprintf("src/%s%d.kt", d, i), fmt.Sprintf("%s%d.kt", d, i)} {
				if err := os.WriteFile(p, nil, 0o644); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	return g
}

var (
	multi = GateRules{Multi: true}
	root  string
)

func abs(path string) string { return filepath.Join(root, path) }

func edit(turn int, path string) GateRequest {
	return GateRequest{Session: "s", Turn: turn, Agent: "nav-pilot", Tool: "edit", Path: abs(path)}
}

func TestGateDeniesTheFifthFileUntilItIsSent(t *testing.T) {
	g := testGate(t, true, multi)
	for i := 1; i <= 4; i++ {
		if deny, _ := g.decide(edit(1, fmt.Sprintf("src/F%d.kt", i))); deny != "" {
			t.Fatalf("file %d denied; the policy keeps changes under 5 files with the orchestrator", i)
		}
	}
	// Editing a file already counted is not a new file.
	if deny, _ := g.decide(edit(1, "src/F1.kt")); deny != "" {
		t.Fatal("a second edit to a counted file was denied")
	}
	deny, outcome := g.decide(edit(1, "src/F5.kt"))
	if deny != GateDenyText || outcome != "deny_files" {
		t.Fatalf("the 5th file was not denied: %q %q", deny, outcome)
	}
	// A plain retry does not pass: that would make the gate a nudge.
	if deny, _ := g.decide(edit(1, "src/F5.kt")); deny == "" {
		t.Fatal("a plain retry of the denied edit passed")
	}
	// The budget is spent: the third deny would be one too many.
	if deny, _ := g.decide(edit(1, "src/F6.kt")); deny != "" {
		t.Fatal("the gate denied past its budget of 2 per turn")
	}
	// A new turn starts clean.
	for i := 1; i <= 4; i++ {
		g.decide(edit(2, fmt.Sprintf("src/G%d.kt", i)))
	}
	if deny, _ := g.decide(edit(2, "src/G5.kt")); deny == "" {
		t.Fatal("a new turn did not reset the gate")
	}
}

func TestGatePassesAFileSentToTheWorker(t *testing.T) {
	g := testGate(t, true, multi)
	for i := 1; i <= 4; i++ {
		g.decide(edit(1, fmt.Sprintf("src/F%d.kt", i)))
	}
	if deny, _ := g.decide(edit(1, "src/F5.kt")); deny == "" {
		t.Fatal("the 5th file was not denied")
	}
	_, outcome := g.decide(GateRequest{Session: "s", Turn: 1, Agent: "nav-pilot", Tool: "task", Subagent: WorkerAgent,
		Prompt: "In src/F5.kt change every configuredJacksonMapper( to configuredJacksonMapper(\"F5\", "})
	if outcome != "dispatched_after_deny" {
		t.Errorf("a dispatch after a deny was recorded as %q", outcome)
	}
	if deny, _ := g.decide(edit(1, "src/F5.kt")); deny != "" {
		t.Fatal("the orchestrator's follow-up on a file it sent to the worker was denied")
	}
	if n := g.snapshot()["dispatched_after_deny"]; n != 1 {
		t.Errorf("dispatched_after_deny = %d, want 1", n)
	}
}

func TestGateScriptedEditPassesAfterADispatch(t *testing.T) {
	g := testGate(t, true, multi)
	loop := GateRequest{Session: "s", Turn: 1, Agent: "nav-pilot", Tool: "bash",
		Command: `for f in *.kt; do sed -i "s/a/$f/" "$f"; done`}
	if deny, outcome := g.decide(loop); deny == "" || outcome != "deny_scripted" {
		t.Fatalf("a scripted per-file edit was not denied: %q", outcome)
	}
	g.decide(GateRequest{Session: "s", Turn: 1, Agent: "nav-pilot", Tool: "task", Subagent: WorkerAgent, Prompt: "x"})
	if deny, _ := g.decide(loop); deny != "" {
		t.Fatal("a scripted edit after a worker dispatch was denied")
	}
}

func TestGateFailsOpen(t *testing.T) {
	down := testGate(t, false, multi)
	for i := 1; i <= 5; i++ {
		if deny, _ := down.decide(edit(1, fmt.Sprintf("F%d.kt", i))); deny != "" {
			t.Fatal("the gate denied with the local server down")
		}
	}
	g := testGate(t, true, multi)
	for _, r := range []GateRequest{
		{Session: "s", Turn: 1, Agent: "nav-pilot", Tool: "edit", Path: "F.kt"}, // relative: not statable, allowed
		{Session: "s", Turn: 1, Agent: WorkerAgent, Tool: "edit", Path: "F.kt"},
		{Session: "s", Turn: 1, Agent: "", Tool: "edit", Path: "F.kt"},
		{Session: "", Turn: 1, Agent: "nav-pilot", Tool: "edit", Path: "F.kt"},
	} {
		for i := 0; i < 12; i++ {
			r.Path = fmt.Sprintf("F%d.kt", i)
			if deny, _ := g.decide(r); deny != "" {
				t.Fatalf("denied %+v", r)
			}
		}
	}
}

func TestGateRoute(t *testing.T) {
	g := testGate(t, true, multi)
	srv := httptest.NewServer(http.HandlerFunc(g.serve))
	defer srv.Close()
	post := func(body string) string {
		res, err := http.Post(srv.URL, "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		b, _ := io.ReadAll(res.Body)
		return strings.TrimSpace(string(b))
	}
	if got := post("not json"); got != "{}" {
		t.Errorf("malformed request answered %q, want {}", got)
	}
	for i := 1; i <= 4; i++ {
		post(fmt.Sprintf(`{"session":"s","turn":1,"agent":"nav-pilot","tool":"write","path":%q}`, abs(fmt.Sprintf("F%d.kt", i))))
	}
	if got := post(fmt.Sprintf(`{"session":"s","turn":1,"agent":"nav-pilot","tool":"write","path":%q}`, abs("F5.kt"))); !strings.Contains(got, `"deny"`) {
		t.Errorf("the 5th file was answered %q", got)
	}
}

func TestGateOnlyCountsFilesThatExist(t *testing.T) {
	g := testGate(t, true, multi)
	for i := 1; i <= 6; i++ {
		if deny, _ := g.decide(GateRequest{Session: "s", Turn: 1, Agent: "nav-pilot", Tool: "write", Path: abs(fmt.Sprintf("new/N%d.kt", i)), Create: true}); deny != "" {
			t.Fatal("a new file counted as a mechanical edit with create-file not trusted")
		}
	}
	// One file named two ways is one file.
	for _, p := range []string{"src/F1.kt", "F1.kt", "src/F2.kt", "src/F3.kt", "src/F4.kt"} {
		g.decide(GateRequest{Session: "s", Turn: 1, Agent: "nav-pilot", Tool: "bash", Command: "sed -i 's/a/b/' " + p})
	}
	if deny, _ := g.decide(edit(1, "src/F5.kt")); deny == "" {
		t.Fatal("the 5th file was not denied")
	}
}

func TestGateCreateRule(t *testing.T) {
	g := testGate(t, true, GateRules{Create: true})
	newTest := GateRequest{Session: "s", Turn: 1, Agent: "nav-pilot", Tool: "write", Path: abs("src/test/FooTest.kt"), Create: true}
	// An edit of a path that is not there is a mistake, not a new file.
	if deny, _ := g.decide(GateRequest{Session: "s", Turn: 1, Agent: "nav-pilot", Tool: "edit", Path: abs("src/Typo.kt")}); deny != "" {
		t.Fatal("an edit of a missing path was treated as a new file")
	}
	if deny, outcome := g.decide(newTest); deny != GateCreateText || outcome != "deny_create" {
		t.Fatalf("a new test file was not denied: %q", outcome)
	}
	g.decide(GateRequest{Session: "s", Turn: 1, Agent: "nav-pilot", Tool: "task", Subagent: WorkerAgent, Prompt: "Write src/test/FooTest.kt with …"})
	if deny, _ := g.decide(newTest); deny != "" {
		t.Fatal("a new file sent to the worker was still denied")
	}
	// Create alone does not gate edits of existing files.
	for i := 1; i <= 12; i++ {
		if deny, _ := g.decide(edit(1, fmt.Sprintf("src/F%d.kt", i))); deny != "" {
			t.Fatal("the create rule gated an edit of an existing file")
		}
	}
}

func TestDispatchGateRules(t *testing.T) {
	c := &Capabilities{Classes: map[string]ClassVerdict{
		"edit-multi-mechanical": {Delegate: VerdictTrusted},
		"create-file":           {Delegate: VerdictTrusted},
	}}
	for level, want := range map[string]GateRules{
		DispatchOff:          {},
		DispatchConservative: {},
		DispatchBalanced:     {Multi: true, Checkpoint: true},
		DispatchAggressive:   {Multi: true, Create: true},
	} {
		if got := DispatchGateRules(level, c); got != want {
			t.Errorf("%s: %+v, want %+v", level, got, want)
		}
	}
	if got := DispatchGateRules(DispatchAggressive, nil); got.Any() {
		t.Errorf("a manifest trusting nothing got rules %+v", got)
	}
	untrusted := &Capabilities{Classes: map[string]ClassVerdict{"create-file": {Delegate: VerdictCloud}}}
	if got := DispatchGateRules(DispatchAggressive, untrusted); got.Any() {
		t.Errorf("an untrusted class got a rule: %+v", got)
	}
}

func TestGateExemptionMatchesWholeNames(t *testing.T) {
	st := &gateTurn{sent: []string{"In src/BarFoo.kt change x", "Edit Baz.kt: …"}}
	for path, want := range map[string]bool{
		"/abs/src/Foo.kt": false, // only BarFoo.kt was sent
		"/abs/BarFoo.kt":  true,
		"Baz.kt":          true,
		"Baz.kts":         false,
	} {
		if got := st.exempt(path); got != want {
			t.Errorf("exempt(%q) = %v, want %v", path, got, want)
		}
	}
}

func TestGateTenEditsOfOneFileIsNotMechanical(t *testing.T) {
	g := testGate(t, true, multi)
	for i := 0; i < 15; i++ {
		if deny, _ := g.decide(edit(1, "src/F1.kt")); deny != "" {
			t.Fatalf("edit %d of one file was denied", i+1)
		}
	}
}

func TestGateCheckpointLetsARetryThrough(t *testing.T) {
	g := testGate(t, true, GateRules{Multi: true, Checkpoint: true})
	for i := 1; i <= 4; i++ {
		g.decide(edit(1, fmt.Sprintf("src/F%d.kt", i)))
	}
	if deny, _ := g.decide(edit(1, "src/F5.kt")); deny != GateCheckpointText {
		t.Fatalf("the 5th file at a checkpoint got %q", deny)
	}
	if deny, _ := g.decide(edit(1, "src/F5.kt")); deny != "" {
		t.Fatal("the retry at a checkpoint was denied")
	}
	// Budget 1: nothing more this turn.
	if deny, _ := g.decide(edit(1, "src/F6.kt")); deny != "" {
		t.Fatal("a checkpoint denied twice in one turn")
	}
}

func TestGateStatsOnlyUnderTheProject(t *testing.T) {
	for path, ok := range map[string]bool{
		"/proj/src/A.kt":     true,
		"/proj/../etc/hosts": false,
		"/projx/A.kt":        false,
		"/etc/hosts":         false,
		"src/A.kt":           false,
	} {
		if _, got := under("/proj", path); got != ok {
			t.Errorf("under(/proj, %q) = %v, want %v", path, got, ok)
		}
	}
	if _, ok := under("", "/proj/A.kt"); ok {
		t.Error("no root must stat nothing")
	}
}
