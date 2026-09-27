package cli

import (
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

// A -h after -p is the question, not a request for help.
func TestWantsHelp(t *testing.T) {
	for _, c := range []struct {
		args []string
		want bool
	}{
		{nil, false},
		{[]string{"--help"}, true},
		{[]string{"--model", "x", "-h"}, true},
		{[]string{"-p", "-h"}, false},
		{[]string{"--prompt", "--help"}, false},
	} {
		if got := wantsHelp(c.args); got != c.want {
			t.Errorf("wantsHelp(%q) = %v, want %v", c.args, got, c.want)
		}
	}
}
