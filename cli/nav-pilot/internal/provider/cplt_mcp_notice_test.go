package provider

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCpltMCPNotice(t *testing.T) {
	t.Setenv("NAV_PILOT_CONFIG", filepath.Join(t.TempDir(), "config.yaml"))
	t.Cleanup(func() { versionCache.Delete("opencode") })
	prev := probeCpltVersion
	t.Cleanup(func() { probeCpltVersion = prev })
	launch := func(opencode, cplt string) string {
		t.Helper()
		versionCache.Store("opencode", versionAnswer{opencode, nil, time.Hour})
		probeCpltVersion = func() (string, error) { return cplt, nil }
		r, w, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		orig := os.Stderr
		os.Stderr = w
		warnOpenCode1()
		os.Stderr = orig
		w.Close()
		out, _ := io.ReadAll(r)
		return string(out)
	}
	const v2 = "opencode v2.1.2\n"
	old := "cplt " + openCode2CpltFloor + "-b37a8ab\n"

	// Control: the same old cplt under opencode 1 says nothing about MCP.
	if out := launch("1.18.32\n", old); strings.Contains(out, "github-mcp") {
		t.Errorf("opencode 1: %q, want no MCP notice", out)
	}
	first := launch(v2, old)
	for _, want := range []string{"io.github.navikt/github-mcp", openCode2CpltRecommended, "navikt/cplt#807", cpltUpgradeHint()} {
		if !strings.Contains(first, want) {
			t.Errorf("notice %q lacks %q", first, want)
		}
	}
	if again := launch(v2, old); again != "" {
		t.Errorf("same cplt again: %q, want nothing", again)
	}
	if next := launch(v2, "cplt 2026.10.23-000000-abcdef0\n"); !strings.Contains(next, "github-mcp") {
		t.Errorf("new cplt below recommended: %q, want notice", next)
	}
	for _, c := range []string{"cplt " + openCode2CpltRecommended + "-abcdef0\n", "cplt 2026.11.01-000000-abcdef0\n"} {
		if out := launch(v2, c); out != "" {
			t.Errorf("cplt %q: %q, want nothing", c, out)
		}
	}
}
