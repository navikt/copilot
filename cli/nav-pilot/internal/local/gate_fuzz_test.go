package local

import (
	"slices"
	"strings"
	"testing"
)

// Fuzzers for the dispatch gate's hand-written parsers. They read model-written
// text, so the contract is: never panic, never return an empty or NUL path,
// and give the same answer twice. Seeds are drawn from the table tests.

func checkPaths(t *testing.T, in string, paths ...[]string) {
	t.Helper()
	for _, list := range paths {
		for _, p := range list {
			if p == "" || strings.ContainsRune(p, 0) {
				t.Fatalf("bad path %q from %q", p, in)
			}
		}
	}
}

func FuzzRedirectFiles(f *testing.F) {
	for _, s := range []string{
		`go test ./... 2>&1 | tail`,
		`diff <(sort a) <(sort b) >(cat)`,
		`grep -n "a > b" src/A.kt`,
		`echo x >>a.txt; echo y 1> b.txt`,
		`go test &> out.txt`,
		`echo x | tee -a c.txt d.txt > /dev/null`,
		"cat > src/New.kt <<'EOF'\nval a = b > c\nEOF\necho done > e.txt",
		"cat <<EOF > f.txt\nx >> g.txt\nEOF",
		"cat > a.txt <<EOF\ndon't\nEOF\necho x > b.txt",
		`echo "x>y" $((a>b))`,
		"tee <<'EOF' src/F5.kt\nx\nEOF",
		`tee src/A.kt <in.txt src/B.kt`,
		`[[ $a > $b ]]`,
		`(( $(nproc) > 4 ))`,
		`[ "$a" \> "$b" ]`,
		`awk '{print > f}'`,
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, cmd string) {
		got := RedirectFiles(cmd)
		checkPaths(t, cmd, got)
		if again := RedirectFiles(cmd); !slices.Equal(got, again) {
			t.Fatalf("unstable for %q: %q then %q", cmd, got, again)
		}
	})
}

func FuzzPatchFiles(f *testing.F) {
	for _, s := range []string{
		"*** Begin Patch\n*** Add File: src/New.kt\n+class New\n*** Update File: src/F1.kt\n@@\n-a\n+b\n*** Delete File: src/F2.kt\n*** Update File: src/F3.kt\n*** Move to: src/F3b.kt\n@@\n-x\n+y\n*** End Patch",
		"*** Begin Patch\r\n*** Update File: a.kt \r\n*** End Patch\r\n",
		"*** Begin Patch\n*** Update File: a.kt\n@@\n--- note\n+++ b/y.kt\n*** End Patch",
		"",
		"*** Begin Patch\n  *** Add File: x.kt\n+x\n\t*** Update File: .tool-versions\n*** End Patch",
		"*** Update File: a.kt\n*** Update File: a.kt",
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, text string) {
		added, changed := PatchFiles(text)
		checkPaths(t, text, added, changed)
		a2, c2 := PatchFiles(text)
		if !slices.Equal(added, a2) || !slices.Equal(changed, c2) {
			t.Fatalf("unstable for %q", text)
		}
	})
}

func FuzzInterpreterWrites(f *testing.F) {
	for _, s := range []string{
		`python3 -c 'import json; print(json.load(open("a.json")))'`,
		`python3 -c 'open("a.kt", "w").write("x")'`,
		"python3 - <<'EOF'\nfrom pathlib import Path\nPath('a.kt').write_text('x')\nEOF",
		"python <<EOF\nwith open('a', mode='a') as f: f.write('x')\nEOF",
		`node -e 'require("fs").writeFileSync("a.ts", "x")'`,
		`perl -e 'open(my $fh, ">", "a.pl"); print $fh "x"'`,
		`python3 -c 'print(1)' && grep -rn "open(f, 'w')" src`,
		`python3 -c"open(\"a.kt\", \"w\").write(\"x\")"`,
		`node --eval='require("fs").writeFileSync("a", "x")'`,
		"cat > notes.txt <<EOF\ndon't\nEOF\npython3 - <<EOF\nopen('a', 'w')\nEOF",
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, cmd string) {
		if InterpreterWrites(cmd) != InterpreterWrites(cmd) {
			t.Fatalf("unstable for %q", cmd)
		}
	})
}

func FuzzSplitHeredocs(f *testing.F) {
	for _, s := range []string{
		"cat > src/New.kt <<'EOF'\nval a = b > c\nEOF\necho done > e.txt",
		"cat <<-EOF\n\tx\n\tEOF",
		"cat <<EOF <<END\na\nEOF\nb\nEND",
		"cat <<<here",
		"cat <<EOF\nunterminated",
		"",
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, cmd string) {
		rest, bodies := splitHeredocs(cmd)
		if strings.Count(rest, "\n") > strings.Count(cmd, "\n") {
			t.Fatalf("rest gained lines for %q", cmd)
		}
		r2, b2 := splitHeredocs(cmd)
		if rest != r2 || !slices.Equal(bodies, b2) {
			t.Fatalf("unstable for %q", cmd)
		}
	})
}
