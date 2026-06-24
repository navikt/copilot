package cli

import (
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func requireShellScripts(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("requires POSIX shell scripts")
	}
}

func writeFakeRTK(t *testing.T, dir string, stdout string) string {
	t.Helper()
	argvFile := filepath.Join(dir, "argv.txt")
	script := "#!/bin/sh\nprintf '%s' \"$*\" > \"" + argvFile + "\"\n"
	if stdout != "" {
		script += "printf '%s' '" + stdout + "'\n"
	}
	if err := os.WriteFile(filepath.Join(dir, "rtk"), []byte(script), 0o755); err != nil {
		t.Fatalf("write fake rtk: %v", err)
	}
	return argvFile
}

func captureStdoutFromCmd(t *testing.T, f func() error) (string, error) {
	t.Helper()
	origStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdout = w
	defer func() {
		os.Stdout = origStdout
		_ = w.Close()
		_ = r.Close()
	}()
	runErr := f()
	_ = w.Close()
	os.Stdout = origStdout
	data, _ := io.ReadAll(r)
	return string(data), runErr
}

func TestCmdStats_RTKMissing(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	err := cmdStats(false, false)
	if err == nil || !strings.Contains(err.Error(), "rtk is not installed") {
		t.Fatalf("expected missing-rtk error, got %v", err)
	}
}

func TestCmdStats_Gain(t *testing.T) {
	requireShellScripts(t)
	dir := t.TempDir()
	argvFile := writeFakeRTK(t, dir, "")
	t.Setenv("PATH", dir)

	if _, err := captureStdoutFromCmd(t, func() error { return cmdStats(false, false) }); err != nil {
		t.Fatalf("cmdStats(gain) error: %v", err)
	}
	got, err := os.ReadFile(argvFile)
	if err != nil {
		t.Fatalf("read argv: %v", err)
	}
	if string(got) != "gain" {
		t.Fatalf("argv = %q, want %q", string(got), "gain")
	}
}

func TestCmdStats_Discover(t *testing.T) {
	requireShellScripts(t)
	dir := t.TempDir()
	argvFile := writeFakeRTK(t, dir, "")
	t.Setenv("PATH", dir)

	if _, err := captureStdoutFromCmd(t, func() error { return cmdStats(true, false) }); err != nil {
		t.Fatalf("cmdStats(discover) error: %v", err)
	}
	got, err := os.ReadFile(argvFile)
	if err != nil {
		t.Fatalf("read argv: %v", err)
	}
	if string(got) != "discover" {
		t.Fatalf("argv = %q, want %q", string(got), "discover")
	}
}

func TestCmdStats_JSON(t *testing.T) {
	requireShellScripts(t)
	dir := t.TempDir()
	argvFile := writeFakeRTK(t, dir, "{}")
	t.Setenv("PATH", dir)

	out, err := captureStdoutFromCmd(t, func() error { return cmdStats(false, true) })
	if err != nil {
		t.Fatalf("cmdStats(json) error: %v", err)
	}
	if strings.Contains(out, "nav-pilot stats") {
		t.Fatalf("json output should not contain nav-pilot header, got %q", out)
	}
	got, err := os.ReadFile(argvFile)
	if err != nil {
		t.Fatalf("read argv: %v", err)
	}
	if string(got) != "gain --all --format json" {
		t.Fatalf("argv = %q, want %q", string(got), "gain --all --format json")
	}
}

func TestCmdStats_HumanOutputHasHeaderAndTips(t *testing.T) {
	requireShellScripts(t)
	dir := t.TempDir()
	writeFakeRTK(t, dir, "rtk gain output\n")
	t.Setenv("PATH", dir)

	out, err := captureStdoutFromCmd(t, func() error { return cmdStats(false, false) })
	if err != nil {
		t.Fatalf("cmdStats(human) error: %v", err)
	}
	for _, want := range []string{"🧭 nav-pilot stats", "→ Fetching RTK savings...", "rtk gain output", "nav-pilot stats --discover", "nav-pilot stats --json"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected output to contain %q, got %q", want, out)
		}
	}
}

func TestCmdStats_DiscoverHasHeaderWithoutTips(t *testing.T) {
	requireShellScripts(t)
	dir := t.TempDir()
	writeFakeRTK(t, dir, "discover output\n")
	t.Setenv("PATH", dir)

	out, err := captureStdoutFromCmd(t, func() error { return cmdStats(true, false) })
	if err != nil {
		t.Fatalf("cmdStats(discover human) error: %v", err)
	}
	if !strings.Contains(out, "→ Looking for new RTK savings opportunities...") {
		t.Fatalf("expected discover header, got %q", out)
	}
	if strings.Contains(out, "nav-pilot stats --json") {
		t.Fatalf("discover output should not contain tips, got %q", out)
	}
}

func TestCmdStats_DiscoverWithJSONRejected(t *testing.T) {
	err := cmdStats(true, true)
	if err == nil || !strings.Contains(err.Error(), "--json is not supported together with --discover") {
		t.Fatalf("expected discover/json error, got %v", err)
	}
}

func TestRun_StatsDiscoverFlagOnlyForStats(t *testing.T) {
	err := run([]string{"list", "--discover"})
	if err == nil || !strings.Contains(err.Error(), "--discover is only supported for the stats command") {
		t.Fatalf("expected stats-only discover error, got %v", err)
	}
}
