package cli

import (
	"bytes"
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// Every flag the launch pre-scan in run parses is listed in --help, and in
// launchFlags for did-you-mean (#1049). --local-dispatch, --log-level,
// --otel-log-level and --payload-context all worked and none was in --help.
// The flags are read from the pre-scan's source, so a new case there fails
// this test until the help names it.
func TestEveryLaunchFlagIsInHelp(t *testing.T) {
	src, err := os.ReadFile("cli.go")
	if err != nil {
		t.Fatal(err)
	}
	_, block, ok := strings.Cut(string(src), "// Pre-scan: extract launch-override flags")
	block, _, ok2 := strings.Cut(block, "args = cleanArgs")
	if !ok || !ok2 {
		t.Fatal("cannot find the launch pre-scan in cli.go; update this test's markers")
	}
	var help bytes.Buffer
	usage(&help)
	flags := regexp.MustCompile(`"(--[a-z-]+)"`).FindAllStringSubmatch(block, -1)
	if len(flags) < 10 {
		t.Fatalf("found only %d flags in the pre-scan; the parse is broken", len(flags))
	}
	for _, m := range flags {
		flag := m[1]
		if flag == "--agent" || flag == "--no-sandbox" {
			continue // removed: parsed only to say what replaced it
		}
		if !regexp.MustCompile(`(^|[\s(])` + regexp.QuoteMeta(flag) + `\b`).MatchString(help.String()) {
			t.Errorf("launch flag %s is parsed but not in nav-pilot --help", flag)
		}
		if !slices.Contains(launchFlags, flag) {
			t.Errorf("launch flag %s is parsed but not in launchFlags", flag)
		}
	}
}
