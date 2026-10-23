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
