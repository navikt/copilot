package provider

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestOpenCode1Notice: the opencode 1 nudge is said once a week and once per
// version, never on
// opencode 2, and names the date, the one-way migration, the new login,
// bubblewrap and the install command.
func TestOpenCode1Notice(t *testing.T) {
	t.Setenv("NAV_PILOT_CONFIG", filepath.Join(t.TempDir(), "config.yaml"))
	t.Cleanup(func() { versionCache.Delete("opencode") })
	prevProbe := probeCpltVersion
	t.Cleanup(func() { probeCpltVersion = prevProbe })
	probeCpltVersion = func() (string, error) { return "cplt " + cpltRecommended + "-abcdef0\n", nil }
	launch := func(version string) string {
		t.Helper()
		versionCache.Store("opencode", versionAnswer{version, nil, time.Hour})
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

	prevOS := hostOS
	t.Cleanup(func() { hostOS = prevOS })
	hostOS = "linux"
	first := launch("1.18.32\n")
	for _, want := range []string{OpenCode1EndOfSupport, "one-way", "converts the opencode 1 database", "log in again", "auth.json", "bubblewrap", OpenCode2InstallHint(), "keeps you on opencode 1"} {
		if !strings.Contains(first, want) {
			t.Errorf("notice %q lacks %q", first, want)
		}
	}
	hostOS = "darwin"
	if n := OpenCode1Notice(); n == "" || strings.Contains(n, "bubblewrap") {
		t.Errorf("macOS notice %q: want one without bubblewrap", n)
	}
	if again := launch("1.18.32\n"); again != "" {
		t.Errorf("same version again: %q, want nothing", again)
	}
	prevNow := openCode1Now
	t.Cleanup(func() { openCode1Now = prevNow })
	openCode1Now = func() time.Time { return prevNow().AddDate(0, 0, 7) }
	if nextWeek := launch("1.18.32\n"); nextWeek == "" {
		t.Error("same version a week later: no notice")
	}
	if next := launch("1.19.4\n"); next == "" {
		t.Error("new opencode 1 version: no notice")
	}
	if v2 := launch("opencode v2.1.2\n"); v2 != "" {
		t.Errorf("opencode 2: %q, want nothing", v2)
	}
	if OpenCode1Notice() != "" {
		t.Error("OpenCode1Notice on opencode 2 is not empty")
	}

	// The untested-range warning stays out of an opencode 1 launch; the
	// control is opencode 2 outside its range, which still warns.
	untested := func(version string) string {
		versionCache.Store("opencode", versionAnswer{version, nil, time.Hour})
		r, w, _ := os.Pipe()
		orig := os.Stderr
		os.Stderr = w
		warnUntestedOpenCode()
		os.Stderr = orig
		w.Close()
		out, _ := io.ReadAll(r)
		return string(out)
	}
	if out := untested("1.0.0\n"); out != "" {
		t.Errorf("opencode 1 launch warned about the tested range: %q", out)
	}
	if out := untested("opencode v9.0.0\n"); !strings.Contains(out, "outside the tested range") {
		t.Errorf("control: opencode 9, outside the opencode 2 range: %q", out)
	}
}
