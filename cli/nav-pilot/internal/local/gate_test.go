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
	"syscall"
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
	if got := post(`{"session":"s","turn":1,"agent":"nav-pilot","tool":"task","subagent":"local-worker","phase":"after"}`); !strings.Contains(got, `"append"`) {
		t.Errorf("the worker's return was answered %q", got)
	}
	if got := post(`{"session":"s","turn":1,"agent":"nav-pilot","phase":"text"}`); !strings.Contains(got, `"nudge"`) {
		t.Errorf("text after the worker's return was answered %q", got)
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

func TestSubstitution(t *testing.T) {
	for _, tc := range []struct {
		script string
		ere    bool
		line   string
		n      int // matches in line; -1 for no pattern
		global bool
	}{
		// Probe 6, r6: BRE, where ( ) are literal.
		{`s/\.generate()/.generate("T")/g`, false, `a.generate() + b.generate()`, 2, true},
		{`s/foo()/foo("A")/`, false, `foo() foo()`, 2, false},
		{`s/foo\(bar\)/x/g`, false, `foobar`, 1, true},
		{`s|a/b|c|g`, false, `a/b a/b`, 2, true},
		{`s/a\/b/c/g`, false, `a/b`, 1, true},
		{`s/\<id\>/key/g`, false, `id idx id`, 2, true},
		// ERE and perl: ( ) group.
		{`s/foo(\(\))/x/g`, true, `foo()`, 1, true},
		{`s/toP\(\)/toP("R")/g`, true, `toP() toP()`, 2, true},
		// Not a plain substitution: no count.
		{`55s/a/b/`, false, ``, -1, false},
		{`s/a/b/g; s/c/d/g`, false, ``, -1, false},
		{`s/(a)\1/b/`, true, ``, -1, false},
		{`y/abc/xyz/`, false, ``, -1, false},
	} {
		re, global := substitution(tc.script, tc.ere)
		if tc.n < 0 {
			if re != nil {
				t.Errorf("substitution(%q) = %v, want none", tc.script, re)
			}
			continue
		}
		if re == nil || global != tc.global || len(re.FindAllStringIndex(tc.line, -1)) != tc.n {
			t.Errorf("substitution(%q) = %v, %v on %q; want %d matches, global %v", tc.script, re, global, tc.line, tc.n, tc.global)
		}
	}
}

// writeCalls writes a file of n lines that each call x.foo().
func writeCalls(t *testing.T, path string, n int) {
	t.Helper()
	if err := os.WriteFile(path, []byte(strings.Repeat("    x.foo()\n", n)), 0o644); err != nil {
		t.Fatal(err)
	}
}

func bash(turn int, cmd string) GateRequest {
	return GateRequest{Session: "s", Turn: turn, Agent: "nav-pilot", Tool: "bash", Command: cmd}
}

// Probe 6, cell r6: 60 call sites in 3 files, as the orchestrator did it
// there. Two edits of two files, then one sed with /g over the test file.
func TestGateCountsTheSitesOfAReplacement(t *testing.T) {
	for _, rules := range []GateRules{multi, {Multi: true, Checkpoint: true}} {
		g := testGate(t, true, rules)
		writeCalls(t, "src/BitTest.kt", 58)
		g.decide(edit(1, "src/F1.kt"))
		g.decide(edit(1, "src/F2.kt"))
		sed := bash(1, `cd src && sed -i '' 's/\.foo()/.foo("BitTest")/g' BitTest.kt`)
		deny, outcome := g.decide(sed)
		if deny == "" || outcome != "deny_sites" {
			t.Fatalf("checkpoint %v: 58 call sites in a 3rd file got %q", rules.Checkpoint, outcome)
		}
		if rules.Checkpoint {
			if deny, _ := g.decide(sed); deny != "" {
				t.Fatal("the retry at a checkpoint was denied")
			}
			continue
		}
		g.decide(GateRequest{Session: "s", Turn: 1, Agent: "nav-pilot", Tool: "task", Subagent: WorkerAgent, Prompt: "In src/BitTest.kt …"})
		if deny, _ := g.decide(sed); deny != "" {
			t.Fatal("the sed on a file sent to the worker was denied")
		}
	}
}

// Probe 6's small cell: 6 call sites in 3 files, under every size. The same
// kind of sed must not trip the site rule.
func TestGateSmallReplacementPasses(t *testing.T) {
	g := testGate(t, true, multi)
	writeCalls(t, "src/Repo.kt", 4)
	for _, r := range []GateRequest{
		edit(1, "src/Repo.kt"),
		bash(1, `sed -i '' 's/foo()/foo("Repo")/g' src/Repo.kt`),
		edit(1, "src/F1.kt"),
		edit(1, "src/F2.kt"),
		bash(1, `perl -pi -e 's/foo\(\)/foo("F3")/g' src/F3.kt`),
	} {
		if deny, outcome := g.decide(r); deny != "" {
			t.Fatalf("%s %s%s was denied (%s)", r.Tool, r.Path, r.Command, outcome)
		}
	}
}

func TestGateSitesAbsoluteCd(t *testing.T) {
	g := testGate(t, true, multi)
	writeCalls(t, "src/BitTest.kt", 12)
	g.decide(edit(1, "src/F1.kt"))
	g.decide(edit(1, "src/F2.kt"))
	if _, outcome := g.decide(bash(1, "cd "+root+` && sed -E -i '' 's/\.foo\(\)/.foo("B")/g' src/BitTest.kt`)); outcome != "deny_sites" {
		t.Fatalf("an ERE sed after an absolute cd got %q", outcome)
	}
}

func TestGateSitesSkipsAFifo(t *testing.T) {
	g := testGate(t, true, multi)
	if err := syscall.Mkfifo("src/Pipe.kt", 0o644); err != nil {
		t.Skip(err)
	}
	g.decide(edit(1, "src/F1.kt"))
	g.decide(edit(1, "src/F2.kt"))
	if deny, _ := g.decide(bash(1, `sed -i 's/foo/bar/g' src/Pipe.kt`)); deny != "" {
		t.Fatal("a FIFO was counted")
	}
}

func TestGateCountsAReplaceAllEdit(t *testing.T) {
	g := testGate(t, true, multi)
	writeCalls(t, "src/Big.kt", 12)
	g.decide(edit(1, "src/F1.kt"))
	g.decide(edit(1, "src/F2.kt"))
	one := edit(1, "src/Big.kt")
	one.Old = "x.foo()"
	if deny, _ := g.decide(one); deny != "" {
		t.Fatal("an edit of one match was counted as all of them")
	}
	all := edit(1, "src/Big.kt")
	all.Old, all.ReplaceAll = "x.foo()", true
	if deny, outcome := g.decide(all); outcome != "deny_sites" {
		t.Fatalf("a replaceAll edit of 12 matches got %q %q", deny, outcome)
	}
}

func TestGateSitesStayInTheProject(t *testing.T) {
	outside := t.TempDir()
	writeCalls(t, filepath.Join(outside, "Big.kt"), 50)
	g := testGate(t, true, multi)
	g.decide(edit(1, "src/F1.kt"))
	g.decide(edit(1, "src/F2.kt"))
	// A temp dir outside the project is refused on its own (deny_tmp), not counted.
	if _, outcome := g.decide(bash(1, "cd "+outside+` && sed -i 's/foo/bar/g' Big.kt`)); outcome != "deny_tmp" {
		t.Fatalf("the gate counted the sites of a file outside the project: %q", outcome)
	}
	if err := os.Symlink(filepath.Join(outside, "Big.kt"), "src/Link.kt"); err != nil {
		t.Fatal(err)
	}
	if deny, _ := g.decide(bash(1, `sed -i 's/foo/bar/g' src/Link.kt`)); deny != "" {
		t.Fatal("the gate followed a link out of the project")
	}
}

func TestVerifies(t *testing.T) {
	for cmd, want := range map[string]bool{
		`cd /p && ./gradlew compileKotlin compileTestKotlin -q 2>&1 | tail -60`: true,
		`JAVA_HOME=/jdk ./gradlew test --tests 'FooTest'`:                       true,
		`timeout 300 mvn -q test`:                                               true,
		`go test ./...`:                                                         true,
		`npm run typecheck`:                                                     true,
		`pnpm test`:                                                             true,
		`npx tsc --noEmit`:                                                      true,
		`uv run pytest -q`:                                                      true,
		`cargo check`:                                                           true,
		`grep -rn "foo(" src`:                                                   false,
		`git diff --stat`:                                                       false,
		`echo "run the gradle build"`:                                           false,
		`go doc fmt`:                                                            false,
		`npm install`:                                                           false,
	} {
		if got := Verifies(cmd); got != want {
			t.Errorf("Verifies(%q) = %v, want %v", cmd, got, want)
		}
	}
}

func TestGateVerifiesTheWorkersResult(t *testing.T) {
	g := testGate(t, true, multi)
	text := GateRequest{Session: "s", Turn: 1, Agent: "nav-pilot", Phase: "text"}
	if key, _ := g.verify(text); key != "" {
		t.Fatal("a reminder before anything was sent to the worker")
	}
	returned := GateRequest{Session: "s", Turn: 1, Agent: "nav-pilot", Tool: "task", Subagent: WorkerAgent, Phase: "after"}
	if key, got := g.verify(returned); key != "append" || got != GateVerifyText {
		t.Fatalf("the worker's return got %q %q", key, got)
	}
	// A grep is not a check.
	g.decide(bash(1, `grep -rn "foo(" src`))
	if key, got := g.verify(text); key != "nudge" || got != GateNudgeText {
		t.Fatalf("text after an unchecked return got %q", key)
	}
	if key, _ := g.verify(text); key != "" {
		t.Fatal("the reminder came twice in one turn")
	}
	if n := g.snapshot()["verify_nudge"]; n != 1 {
		t.Errorf("verify_nudge = %d, want 1", n)
	}
	// A build after the return settles it, for the next return too.
	g.verify(GateRequest{Session: "s", Turn: 2, Agent: "nav-pilot", Tool: "task", Subagent: WorkerAgent, Phase: "after"})
	g.decide(bash(2, "./gradlew build"))
	if key, _ := g.verify(GateRequest{Session: "s", Turn: 2, Agent: "nav-pilot", Phase: "text"}); key != "" {
		t.Fatal("a reminder after the build had run")
	}
	// The worker's own session and a task to another agent are not its business.
	for _, r := range []GateRequest{
		{Session: "s", Turn: 3, Agent: WorkerAgent, Tool: "task", Subagent: WorkerAgent, Phase: "after"},
		{Session: "s", Turn: 3, Agent: "nav-pilot", Tool: "task", Subagent: "explore", Phase: "after"},
	} {
		if key, _ := g.verify(r); key != "" {
			t.Errorf("%+v answered %q", r, key)
		}
	}
	if key, _ := g.verify(GateRequest{Session: "s", Turn: 3, Agent: "nav-pilot", Phase: "text"}); key != "" {
		t.Error("a reminder with nothing returned from the worker")
	}
}

// A check that fails after the worker created a file sends it back to the
// worker once, with the check's output; a second failure is the
// orchestrator's to fix (mlx-workspace #126, bench-frontier's retry2).
func TestGateGivesACreatedFileOneRetry(t *testing.T) {
	g := testGate(t, true, multi)
	exit := func(n int) *int { return &n }
	created := filepath.Join(root, "src", "NewThing.kt")
	// The worker creates a file in its own session.
	g.verify(GateRequest{Session: "w1", Agent: WorkerAgent, Tool: "write", Create: true, Path: created, Phase: "worker"})
	g.verify(GateRequest{Session: "s", Turn: 1, Agent: "nav-pilot", Tool: "task", Subagent: WorkerAgent, Worker: "w1", Phase: "after"})
	g.decide(bash(1, "./gradlew test"))
	failed := GateRequest{Session: "s", Turn: 1, Agent: "nav-pilot", Tool: "bash", Command: "./gradlew test", Exit: exit(1), Phase: "after"}
	if key, got := g.verify(failed); key != "append" || got != GateRetryText {
		t.Fatalf("first failure got %q %q", key, got)
	}
	// The retry: the same task again, and the same check.
	g.verify(GateRequest{Session: "s", Turn: 1, Agent: "nav-pilot", Tool: "task", Subagent: WorkerAgent, Worker: "w1", Phase: "after"})
	g.decide(bash(1, "./gradlew test"))
	if key, got := g.verify(failed); key != "append" || got != GateRetryDoneText {
		t.Fatalf("failure after the retry got %q %q", key, got)
	}
	// Bounded: no third attempt is offered.
	g.verify(GateRequest{Session: "s", Turn: 1, Agent: "nav-pilot", Tool: "task", Subagent: WorkerAgent, Phase: "after"})
	g.decide(bash(1, "./gradlew test"))
	if _, got := g.verify(failed); got == GateRetryText {
		t.Fatal("a second retry was offered")
	}
	c := g.snapshot()
	if c["create_retry"] != 1 || c["create_retry_failed"] != 1 {
		t.Errorf("counts = %v", c)
	}

	// Next turn: the retry passes.
	g.verify(GateRequest{Session: "w2", Agent: WorkerAgent, Tool: "write", Create: true, Path: created, Phase: "worker"})
	g.verify(GateRequest{Session: "s", Turn: 2, Agent: "nav-pilot", Tool: "task", Subagent: WorkerAgent, Worker: "w2", Phase: "after"})
	g.decide(bash(2, "npm test"))
	g.verify(GateRequest{Session: "s", Turn: 2, Agent: "nav-pilot", Tool: "bash", Command: "npm test", Exit: exit(1), Phase: "after"})
	g.verify(GateRequest{Session: "s", Turn: 2, Agent: "nav-pilot", Tool: "task", Subagent: WorkerAgent, Worker: "w2", Phase: "after"})
	g.decide(bash(2, "npm test"))
	if key, _ := g.verify(GateRequest{Session: "s", Turn: 2, Agent: "nav-pilot", Tool: "bash", Command: "npm test", Exit: exit(0), Phase: "after"}); key != "" {
		t.Errorf("a passing check got %q", key)
	}
	if n := g.snapshot()["create_retry_passed"]; n != 1 {
		t.Errorf("create_retry_passed = %d, want 1", n)
	}

	// A worker that only edited gets no retry offer: that is not create-file.
	g.verify(GateRequest{Session: "s", Turn: 3, Agent: "nav-pilot", Tool: "task", Subagent: WorkerAgent, Worker: "w3", Phase: "after"})
	g.decide(bash(3, "npm test"))
	if key, _ := g.verify(GateRequest{Session: "s", Turn: 3, Agent: "nav-pilot", Tool: "bash", Command: "npm test", Exit: exit(1), Phase: "after"}); key != "" {
		t.Errorf("an edit-only return got a retry offer: %q", key)
	}
}

// Re-probe 7 (#1237): a backup or draft in /tmp asks for external_directory,
// and `opencode run` ends the session on it. The gate refuses it instead, at
// any budget, and leaves the project, the worker and relative paths alone.
func TestGateRefusesTempOutsideTheProject(t *testing.T) {
	g := testGate(t, true, GateRules{Create: true})
	for _, r := range []GateRequest{
		bash(1, "cp src/main/kotlin/DateUtil.kt /tmp/DateUtil.kt.bak"),
		bash(1, "mkdir -p /tmp/navpilot_tests"),
		bash(1, "./gradlew test > /tmp/out.txt 2>&1"),
		bash(1, "cd /tmp && ls"),
		bash(1, "go test -coverprofile=/private/tmp/c.out ./..."),
		{Session: "s", Turn: 1, Agent: "nav-pilot", Tool: "write", Path: "/tmp/FooTest.kt", Create: true},
		{Session: "s", Turn: 1, Agent: "nav-pilot", Tool: "edit", Path: filepath.Join(os.TempDir(), "x", "Foo.kt")},
	} {
		if deny, outcome := g.decide(r); deny != GateTmpText || outcome != "deny_tmp" {
			t.Errorf("%s %q%s was not refused: %q", r.Tool, r.Command, r.Path, outcome)
		}
	}
	for _, r := range []GateRequest{
		bash(1, "cp src/Foo.kt src/Foo.kt.bak"),
		bash(1, "cp "+abs("src/Foo.kt")+" "+abs("build/Foo.kt.bak")),
		bash(1, "./gradlew test"),
		bash(1, "cd "+root+" && ./gradlew test"),
		edit(1, "src/Foo.kt"),
		{Session: "w", Turn: 1, Agent: WorkerAgent, Tool: "bash", Command: "mkdir -p /tmp/x"},
	} {
		if deny, _ := g.decide(r); deny != "" {
			t.Errorf("%s %q%s was refused", r.Tool, r.Command, r.Path)
		}
	}
	if n := g.snapshot()["deny_tmp"]; n != 7 {
		t.Errorf("deny_tmp = %d, want 7", n)
	}
}

// TestGateRefusesOtherPathsOutsideTheProject: #1273. /var/tmp and $TMPDIR are
// temp dirs like /tmp; home is refused for the commands opencode checks, not
// for a read the build needs.
func TestGateRefusesOtherPathsOutsideTheProject(t *testing.T) {
	g := testGate(t, true, GateRules{Create: true})
	t.Setenv("HOME", "/home/np-uat")
	t.Setenv("TMPDIR", "/scratch/np-uat-tmp")
	for _, tc := range []struct {
		req  GateRequest
		deny bool
	}{
		{bash(1, "cp src/F1.kt /var/tmp/F1.bak"), true},
		{bash(1, "cp src/F1.kt /private/var/tmp/F1.bak"), true},
		{bash(1, "cp src/F1.kt $TMPDIR/F1.bak"), true},
		{bash(1, "cp src/F1.kt ${TMPDIR}/F1.bak"), true},
		{bash(1, "cp src/F1.kt /scratch/np-uat-tmp/F1.bak"), true},
		{bash(1, "cat $TMPDIR/out.txt"), true},
		{bash(1, "cp src/F1.kt ~/F1.bak"), true},
		{bash(1, "cp src/F1.kt $HOME/F1.bak"), true},
		{bash(1, "mkdir -p ~/navpilot_tests"), true},
		{bash(1, "./gradlew test > ~/out.txt"), true},
		{bash(1, "./gradlew test >~/out.txt"), true},
		{bash(1, "cd ~ && ls"), true},
		{GateRequest{Session: "s", Turn: 1, Agent: "nav-pilot", Tool: "write", Path: "/home/np-uat/Draft.kt", Create: true}, true},
		{bash(1, "cat ~/.gradle/gradle.properties"), true},
		{bash(1, "rm ~/F1.bak"), true},
		{bash(1, "MODE=backup cp src/F1.kt ~/F1.bak"), true},
		{bash(1, "env LC_ALL=C timeout 5 cp src/F1.kt ~/F1.bak"), true},
		{bash(1, "grep org.gradle ~/.gradle/gradle.properties"), false},
		{bash(1, "JAVA_HOME=~/.sdkman/candidates/java/21 ./gradlew test"), false},
		{bash(1, "ls /home/np-uat/.m2/repository"), false},
		{bash(1, "cp ~user/F1.kt src/"), false},
		{bash(1, "cp src/F1.kt src/F1.bak"), false},
		{GateRequest{Session: "w", Turn: 1, Agent: WorkerAgent, Tool: "bash", Command: "cp a ~/b"}, false},
	} {
		deny, _ := g.decide(tc.req)
		if got := deny == GateTmpText; got != tc.deny {
			t.Errorf("%s %q%s: refused = %v, want %v", tc.req.Tool, tc.req.Command, tc.req.Path, got, tc.deny)
		}
	}
}

// TestResolvedFormsFollowsSymlinks: a $TMPDIR behind a symlink, as on macOS
// (/var/folders -> /private/var/folders), is recognised in either form.
func TestResolvedFormsFollowsSymlinks(t *testing.T) {
	real, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(t.TempDir(), "tmp")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	forms := resolvedForms(link)
	for _, p := range []string{filepath.Join(link, "F1.bak"), filepath.Join(real, "F1.bak"), real} {
		if !within(forms, p) {
			t.Errorf("within(%v, %q) = false, want true", forms, p)
		}
	}
	if within(forms, "/elsewhere/F1.bak") {
		t.Errorf("within(%v, /elsewhere/F1.bak) = true", forms)
	}
}
