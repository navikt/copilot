package provider

import (
	"os"
	"strings"
	"testing"

	"github.com/navikt/copilot/cli/nav-pilot/internal/domain"
)

func TestCheckCpltFlagFloor(t *testing.T) {
	args := []string{"--agent", "copilot", "--allow-socket", "/tmp/m.sock", "--", "--allow-socket"}
	for _, tc := range []struct {
		name, version string
		owner         domain.PkgManager
		wantHint      string
	}{
		{"older than #108 blocks, brew", "cplt 2026.07.03-185915-aaaaaaa", domain.PkgBrew, "brew upgrade navikt/tap/cplt"},
		{"older than #108 blocks, apt", "cplt 2026.06.01-000000-aaaaaaa", domain.PkgApt, "sudo apt update && sudo apt install --only-upgrade cplt"},
		{"floor launches", "cplt 2026.07.03-205915-2798b06", domain.PkgBrew, ""},
		{"older than #108 blocks, bare binary", "cplt 2026.06.01-000000-aaaaaaa", domain.PkgNone, "cplt update"},
		{"newer launches", "cplt 2026.10.29-130225-8a38a48", domain.PkgBrew, ""},
		{"dev build launches", "cplt dev", domain.PkgBrew, ""},
		{"unknown launches", "cplt unknown", domain.PkgBrew, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Cleanup(StubCpltVersion(tc.version))
			orig := domain.PkgOwner
			domain.PkgOwner = func(string) domain.PkgManager { return tc.owner }
			t.Cleanup(func() { domain.PkgOwner = orig })

			err := checkCpltFlagFloor(args)
			if tc.wantHint == "" {
				if err != nil {
					t.Fatalf("want launch, got %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("want a block, got launch")
			}
			msg := err.Error()
			if strings.Contains(msg, "\n") || !strings.Contains(msg, "--allow-socket") || !strings.Contains(msg, tc.wantHint) {
				t.Fatalf("message %q: want one line naming --allow-socket and %q", msg, tc.wantHint)
			}
		})
	}
}

// --allow-clipboard (navikt/cplt#337) has its own, later floor.
func TestCheckCpltFlagFloorClipboard(t *testing.T) {
	args := []string{"--agent", "copilot", "--allow-clipboard", "--"}
	t.Cleanup(StubCpltVersion("cplt 2026.09.04-194300-aaaaaaa\n"))
	if err := checkCpltFlagFloor(args); err == nil || !strings.Contains(err.Error(), "--allow-clipboard") {
		t.Fatalf("one second before #337: want a block naming --allow-clipboard, got %v", err)
	}
	t.Cleanup(StubCpltVersion("cplt 2026.09.04-194301-d59230a\n"))
	if err := checkCpltFlagFloor(args); err != nil {
		t.Fatalf("the #337 release: want launch, got %v", err)
	}
}

// A blocked launch says exactly one thing: the returned error. It runs no cplt
// and prints nothing first, not even the recommended-cplt notice, which an old
// cplt would otherwise trigger.
func TestLaunchViaCpltBlockedPrintsOneLine(t *testing.T) {
	isolateHome(t)
	t.Chdir(t.TempDir())
	out := fakeCpltArgs(t)
	t.Cleanup(StubCpltVersion("cplt 2026.06.01-000000-aaaaaaa\n"))
	var err error
	stderr := stderrOf(t, func() {
		err = launchViaCplt(cpltLaunch{agent: "copilot", displayName: "Copilot", sessionFlags: []string{"--allow-clipboard"}})
	})
	if err == nil || strings.Contains(err.Error(), "\n") {
		t.Fatalf("want a one-line error, got %v", err)
	}
	if stderr != "" {
		t.Errorf("printed before the error: %q", stderr)
	}
	if _, statErr := os.Stat(out); statErr == nil {
		t.Error("cplt ran")
	}
}

// A launch without a floored flag never probes cplt, and agent args after
// "--" do not count.
func TestCheckCpltFlagFloorSkipsWithoutFlag(t *testing.T) {
	prev := probeCpltVersion
	t.Cleanup(func() { probeCpltVersion = prev })
	probeCpltVersion = func() (string, error) { t.Fatal("probed"); return "", nil }
	if err := checkCpltFlagFloor([]string{"--agent", "copilot", "--", "--allow-socket"}); err != nil {
		t.Fatal(err)
	}
}
