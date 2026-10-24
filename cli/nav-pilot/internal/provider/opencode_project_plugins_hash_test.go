package provider

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

// launchTwice answers yes, edits, and launches again: the edit must re-ask.
func launchTwice(t *testing.T, proj string, asked *int, edit func()) {
	t.Helper()
	applyOpenCodeProjectPlugins(nil, proj)
	applyOpenCodeProjectPlugins(nil, proj)
	if *asked != 1 {
		t.Fatalf("control: asked %d times before the edit, want 1", *asked)
	}
	edit()
	applyOpenCodeProjectPlugins(nil, proj)
	if *asked != 2 {
		t.Fatalf("asked %d times after the edit, want 2", *asked)
	}
}

func TestOpenCodeProjectPluginsConfigDirEntryCodeIsHashed(t *testing.T) {
	yes := true
	proj, asked := pluginTrustSetup(t, true, &yes)
	writeFile(t, filepath.Join(proj, "opencode.json"), `{"plugins":["./tools/p"]}`)
	writeFile(t, filepath.Join(proj, "tools/p/index.ts"), "one")
	launchTwice(t, proj, asked, func() { writeFile(t, filepath.Join(proj, "tools/p/index.ts"), "two") })
}

func TestOpenCodeProjectPluginsFileURLEntryCodeIsHashed(t *testing.T) {
	yes := true
	proj, asked := pluginTrustSetup(t, true, &yes)
	writeFile(t, filepath.Join(proj, "opencode.json"), `{"plugins":["file://`+filepath.Join(proj, "tools/p")+`"]}`)
	writeFile(t, filepath.Join(proj, "tools/p/index.ts"), "one")
	launchTwice(t, proj, asked, func() { writeFile(t, filepath.Join(proj, "tools/p/index.ts"), "two") })
}

// A {file:} entry cannot be pinned down: changing its target asks again.
func TestOpenCodeProjectPluginsFileSubstitutionAsksAgain(t *testing.T) {
	yes := true
	proj, asked := pluginTrustSetup(t, true, &yes)
	writeFile(t, filepath.Join(proj, "opencode.json"), `{"plugins":["{file:./which}"]}`)
	writeFile(t, filepath.Join(proj, "which"), "./a")
	applyOpenCodeProjectPlugins(nil, proj)
	writeFile(t, filepath.Join(proj, "which"), "./b")
	applyOpenCodeProjectPlugins(nil, proj)
	if *asked != 2 {
		t.Fatalf("asked %d times, want 2", *asked)
	}
}

// An unquoted {env:} nulls the list in nav-pilot's parse, not opencode's.
func TestOpenCodeProjectPluginsUnquotedEnvIsNotSkipped(t *testing.T) {
	yes := true
	proj, asked := pluginTrustSetup(t, false, &yes)
	writeFile(t, filepath.Join(proj, "opencode.json"), `{"plugins": {env:PLUGINS}}`)
	if env := applyOpenCodeProjectPlugins([]string{`PLUGINS=["./x"]`}, proj); !disabled(env) || *asked != 0 {
		t.Fatalf("asked=%d env=%v", *asked, env)
	}
}

func TestOpenCodeProjectPluginsSymlinkTargetIsHashed(t *testing.T) {
	yes := true
	proj, asked := pluginTrustSetup(t, true, &yes)
	writeFile(t, filepath.Join(proj, "tools/p/index.ts"), "one")
	if err := os.MkdirAll(filepath.Join(proj, ".opencode/plugins"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(proj, "tools/p"), filepath.Join(proj, ".opencode/plugins/p")); err != nil {
		t.Fatal(err)
	}
	launchTwice(t, proj, asked, func() { writeFile(t, filepath.Join(proj, "tools/p/index.ts"), "two") })
}

// A FIFO is never opened (that would hang); it is unknown and asks each time.
func TestOpenCodeProjectPluginsFIFOIsUnknown(t *testing.T) {
	yes := true
	proj, asked := pluginTrustSetup(t, true, &yes)
	if err := os.MkdirAll(filepath.Join(proj, ".opencode/plugins"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(filepath.Join(proj, ".opencode/plugins/p.ts"), 0o644); err != nil {
		t.Fatal(err)
	}
	applyOpenCodeProjectPlugins(nil, proj)
	applyOpenCodeProjectPlugins(nil, proj)
	if *asked != 2 {
		t.Fatalf("asked %d times, want 2", *asked)
	}
}

// A subdirectory's .opencode loads only when launched there: its own answer.
func TestOpenCodeProjectPluginsSubdirIsItsOwnAnswer(t *testing.T) {
	yes := true
	proj, asked := pluginTrustSetup(t, true, &yes)
	writePlugin(t, proj, "p.ts", "x")
	sub := filepath.Join(proj, "sub")
	writePlugin(t, sub, "q.ts", "y")
	applyOpenCodeProjectPlugins(nil, proj)
	applyOpenCodeProjectPlugins(nil, sub)
	applyOpenCodeProjectPlugins(nil, proj)
	if *asked != 2 {
		t.Fatalf("asked %d times, want 2", *asked)
	}
}

func TestOpenCodeProjectPluginsAliasSkips(t *testing.T) {
	no := false
	proj, asked := pluginTrustSetup(t, true, &no)
	writePlugin(t, proj, "p.ts", "x")
	if env := applyOpenCodeProjectPlugins([]string{"OPENCODE_CONFIG_PROJECT_DISABLE=1"}, proj); *asked != 0 || disabled(env) {
		t.Fatalf("asked=%d env=%v", *asked, env)
	}
}
