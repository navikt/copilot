package cli

import (
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// Every command and every alpha local subcommand has a page of its own, and
// --help, -h and help <cmd> print it on stdout with exit 0 (#1040). They used
// to print the global help, or for alpha local the alpha overview, and
// `alpha local setup --help` failed with "unknown flag".
func TestEveryCommandHasItsOwnHelp(t *testing.T) {
	isolatedRun(t)
	type tc struct {
		args []string
		want string
	}
	var cases []tc
	for _, cmd := range dispatchedCommands {
		switch cmd {
		case "help", "alpha":
			continue // the global page and the alpha overview are their pages
		case "update":
			cmd = "upgrade" // the deprecated name gets the upgrade page
		}
		want := "Usage: nav-pilot " + cmd
		if _, ok := commandHelp[cmd]; !ok {
			t.Errorf("%s has no page in commandHelp", cmd)
		}
		cases = append(cases,
			tc{[]string{cmd, "--help"}, want}, tc{[]string{cmd, "-h"}, want}, tc{[]string{"help", cmd}, want})
	}
	for _, sub := range localCommands {
		want := "Usage: nav-pilot alpha local " + sub
		if _, ok := localHelp[sub]; !ok {
			t.Errorf("alpha local %s has no page in localHelp", sub)
		}
		cases = append(cases,
			tc{[]string{"alpha", "local", sub, "--help"}, want},
			tc{[]string{"alpha", "local", sub, "-h"}, want},
			tc{[]string{"help", "alpha", "local", sub}, want})
	}
	for _, c := range cases {
		var err error
		out, errOut := captureRun(t, func() { err = run(c.args) })
		if err != nil {
			t.Errorf("%v: %v", c.args, err)
			continue
		}
		if !strings.HasPrefix(out, c.want) || errOut != "" {
			t.Errorf("%v: want %q on stdout only\nstdout:\n%s\nstderr:\n%s", c.args, c.want, out, errOut)
		}
	}
}

// For ask the words are the question: only a leading -h is help, and a -h
// after -p is the question.
func TestWantsHelp(t *testing.T) {
	for _, c := range []struct {
		sub  string
		args []string
		want bool
	}{
		{"setup", nil, false},
		{"setup", []string{"--help"}, true},
		{"setup", []string{"--model", "x", "-h"}, true},
		{"ask", []string{"--help"}, true},
		{"ask", []string{"-h"}, true},
		{"ask", []string{"-p", "-h"}, false},
		{"ask", []string{"--prompt", "--help"}, false},
		{"ask", []string{"what", "does", "-h", "mean"}, false},
		{"ask", []string{"what", "--help"}, false},
	} {
		if got := wantsHelp(c.sub, c.args); got != c.want {
			t.Errorf("wantsHelp(%q, %q) = %v, want %v", c.sub, c.args, got, c.want)
		}
	}
}

// switchCases returns the string cases of the one-tab-indented switch that
// starts at marker in file, up to its default.
func switchCases(t *testing.T, file, marker string) []string {
	t.Helper()
	src, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	_, block, ok := strings.Cut(string(src), marker)
	block, _, ok2 := strings.Cut(block, "\n\tdefault:")
	if !ok || !ok2 {
		t.Fatalf("cannot find %q in %s; update this test's markers", marker, file)
	}
	var names []string
	for _, line := range regexp.MustCompile(`(?m)^\tcase (.+):$`).FindAllStringSubmatch(block, -1) {
		for _, q := range regexp.MustCompile(`"([^"]*)"`).FindAllStringSubmatch(line[1], -1) {
			names = append(names, q[1])
		}
	}
	if len(names) < 5 {
		t.Fatalf("found only %d cases after %q in %s; the parse is broken", len(names), marker, file)
	}
	return names
}

// The lists the help test walks are the ones dispatch uses: a command added
// to a dispatch switch and not to the list fails here, so it cannot skip
// the help test.
func TestDispatchListsMatchTheSwitches(t *testing.T) {
	for _, name := range switchCases(t, "cli.go", "\n\tswitch command {\n") {
		if !strings.HasPrefix(name, "-") && !slices.Contains(dispatchedCommands, name) {
			t.Errorf("run dispatches %q, which is not in dispatchedCommands", name)
		}
	}
	for _, name := range switchCases(t, "alpha_local.go", "\n\tswitch sub {\n") {
		if name != "" && name != "help" && !slices.Contains(localCommands, name) {
			t.Errorf("cmdAlpha dispatches alpha local %q, which is not in localCommands", name)
		}
	}
}
