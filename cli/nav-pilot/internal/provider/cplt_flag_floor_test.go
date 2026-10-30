package provider

import (
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
