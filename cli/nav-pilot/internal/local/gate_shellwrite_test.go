package local

import (
	"fmt"
	"slices"
	"testing"
)

func TestRedirectFiles(t *testing.T) {
	for _, tc := range []struct {
		cmd   string
		files []string
	}{
		{`go test ./... 2>&1 | tail`, nil},
		{`make >/dev/null 2>&1`, nil},
		{`echo oops > /dev/stderr`, nil},
		{`go build 2> build.log`, nil},
		{`diff <(sort a) <(sort b) >(cat)`, nil},
		{`echo hi >&2`, nil},
		{`grep -n "a > b" src/A.kt`, nil},
		{`echo x > a.txt`, []string{"a.txt"}},
		{`echo x >>a.txt; echo y 1> b.txt`, []string{"a.txt", "b.txt"}},
		{`go test &> out.txt`, []string{"out.txt"}},
		{`echo x | tee -a c.txt d.txt > /dev/null`, []string{"c.txt", "d.txt"}},
		{"cat > src/New.kt <<'EOF'\nval a = b > c\nEOF\necho done > e.txt", []string{"src/New.kt", "e.txt"}},
		{"cat <<EOF > f.txt\nx >> g.txt\nEOF", []string{"f.txt"}},
		// An apostrophe in a heredoc body does not hide what follows it.
		{"cat > a.txt <<EOF\ndon't\nEOF\necho x > b.txt", []string{"a.txt", "b.txt"}},
		// A redirect glued to the word before it.
		{`echo x>src/F5.kt`, []string{"src/F5.kt"}},
		{`echo x>>a.txt 2>&1`, []string{"a.txt"}},
		{`echo "x>y" $((a>b))`, nil},
		// tee keeps its files around a redirection.
		{"tee <<'EOF' src/F5.kt\nx\nEOF", []string{"src/F5.kt"}},
		{`tee < in.txt src/F5.kt`, []string{"src/F5.kt"}},
		{`tee src/A.kt <in.txt src/B.kt`, []string{"src/A.kt", "src/B.kt"}},
		// Comparisons are not redirects.
		{`[[ $a > $b ]]`, nil},
		{`(( a > b ))`, nil},
		{`(( $(nproc) > 4 ))`, nil},
		{`[ "$a" \> "$b" ]`, nil},
		{`if [[ "$x" > "y" ]]; then echo ok; fi`, nil},
		// A > inside a quoted word or a flag.
		{`grep -rn "x >= 1" src`, nil},
		{`curl -s "https://x/a?b>c"`, nil},
		{`git log --format='>%s'`, nil},
		{`awk '{print > f}'`, nil},
		{`jq '. > 1'`, nil},
	} {
		if got := RedirectFiles(tc.cmd); !slices.Equal(got, tc.files) {
			t.Errorf("RedirectFiles(%q) = %q, want %q", tc.cmd, got, tc.files)
		}
	}
}

func TestInterpreterWrites(t *testing.T) {
	for _, tc := range []struct {
		cmd  string
		want bool
	}{
		{`python3 -c 'import json; print(json.load(open("a.json")))'`, false},
		{`node -e 'console.log(require("fs").readFileSync("a").length)'`, false},
		{`python3 script.py`, false},
		{`grep -rn "open(f, 'w')" src`, false},
		{`python3 -c 'open("a.kt", "w").write("x")'`, true},
		{"python3 - <<'EOF'\nfrom pathlib import Path\nPath('a.kt').write_text('x')\nEOF", true},
		{"python <<EOF\nwith open('a', mode='a') as f: f.write('x')\nEOF", true},
		{`node -e 'require("fs").writeFileSync("a.ts", "x")'`, true},
		{`ruby -e 'File.write("a.rb", "x")'`, true},
		{`perl -e 'open(my $fh, ">", "a.pl"); print $fh "x"'`, true},
		{`deno eval 'await Deno.writeTextFile("a.ts", "x")'`, true},
		{`bun -e 'await Bun.write("a.ts", "x")'`, true},
		// Only the interpreter's own words and heredoc bodies are read.
		{`python3 -c 'print(1)' && grep -rn "open(f, 'w')" src`, false},
		{`python3 -c 'import sys; print(sys.version)'; grep -rn 'writeFileSync(' src/`, false},
		{`node -e 'console.log(1)' # see fs.writeFile( docs`, false},
		{`python3 -c 'import json; d = json.load(open("a")); print(d["mode"])' | grep mode='w'`, false},
		{`python3 -c "open(\"a.kt\", \"w\").write(\"x\")"`, true},
		// Exact interpreter names.
		{`bundle exec rspec -e 'File.write'`, false},
		// Glued options and nested parentheses.
		{`python3 -c'open("a", "w")'`, true},
		{`node --eval='require("fs").writeFileSync("a", "x")'`, true},
		{`python3 -c 'from pathlib import Path; open(Path("a.kt"), "w").write("x")'`, true},
		// A heredoc body with an apostrophe, then a write in a later heredoc.
		{"cat > notes.txt <<EOF\ndon't\nEOF\npython3 - <<EOF\nopen('a', 'w')\nEOF", true},
	} {
		if got := InterpreterWrites(tc.cmd); got != tc.want {
			t.Errorf("InterpreterWrites(%q) = %v, want %v", tc.cmd, got, tc.want)
		}
	}
}

func TestGateShellWrites(t *testing.T) {
	g := testGate(t, true, multi)
	bash := func(cmd string) GateRequest {
		return GateRequest{Session: "s", Turn: 1, Agent: "nav-pilot", Tool: "bash", Command: cmd}
	}
	// Controls: ordinary commands, a redirect to /dev/null, a read-only one-liner.
	for _, cmd := range []string{"go test ./... 2>&1 | tail -20", "make lint > /dev/null", `python3 -c 'print(open("a").read())'`} {
		if deny, _ := g.decide(bash(cmd)); deny != "" {
			t.Fatalf("%q was denied", cmd)
		}
	}
	for i := 1; i <= 4; i++ {
		if deny, _ := g.decide(bash(fmt.Sprintf("echo x > src/F%d.kt", i))); deny != "" {
			t.Fatalf("redirect %d denied", i)
		}
	}
	if deny, outcome := g.decide(bash("cat > src/F5.kt <<EOF\nx\nEOF")); deny != GateDenyText || outcome != "deny_files" {
		t.Fatalf("the 5th file written by a redirect was not denied: %q", outcome)
	}
	write := bash("python3 - <<'EOF'\nopen('src/F6.kt', 'w').write('x')\nEOF")
	if deny, outcome := g.decide(write); deny != GateInterpreterText || outcome != "deny_interpreter" {
		t.Fatalf("an interpreter write was not denied: %q", outcome)
	}
	// The worker is not gated.
	w := write
	w.Agent = WorkerAgent
	if deny, _ := g.decide(w); deny != "" {
		t.Fatal("the worker's interpreter write was denied")
	}
	if deny, _ := g.decide(GateRequest{Session: "s", Turn: 1, Agent: WorkerAgent, Tool: "bash", Command: "echo x > src/F7.kt"}); deny != "" {
		t.Fatal("the worker's redirect was denied")
	}
}

func TestGateInterpreterWriteControls(t *testing.T) {
	write := GateRequest{Session: "s", Turn: 1, Agent: "nav-pilot", Tool: "bash", Command: `python3 -c 'open("a.kt", "w").write("x")'`}
	// Off: the multi rule is not set.
	if deny, _ := testGate(t, true, GateRules{Create: true}).decide(write); deny != "" {
		t.Fatal("an interpreter write was denied without the multi rule")
	}
	// A checkpoint: the same command again passes.
	g := testGate(t, true, GateRules{Multi: true, Checkpoint: true})
	if deny, _ := g.decide(write); deny != GateInterpreterText {
		t.Fatalf("the interpreter write got %q", deny)
	}
	if deny, _ := g.decide(write); deny != "" {
		t.Fatal("the retry at a checkpoint was denied")
	}
	// After a dispatch to the worker: passes, as a scripted edit does.
	g = testGate(t, true, multi)
	g.decide(GateRequest{Session: "s", Turn: 1, Agent: "nav-pilot", Tool: "task", Subagent: WorkerAgent, Prompt: "x"})
	if deny, _ := g.decide(write); deny != "" {
		t.Fatal("an interpreter write after a dispatch was denied")
	}
	// The budget: two denies, then it passes.
	g = testGate(t, true, multi)
	g.decide(write)
	g.decide(write)
	if deny, _ := g.decide(write); deny != "" {
		t.Fatal("the gate denied past its budget")
	}
}
