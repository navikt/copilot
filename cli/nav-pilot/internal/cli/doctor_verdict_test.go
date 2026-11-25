package cli

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	providerpkg "github.com/navikt/copilot/cli/nav-pilot/internal/provider"
)

// A run that printed ⚠ or ✗ rows never ends "healthy"; the control, a run
// without any, does.
func TestDoctorVerdictCountsFixRows(t *testing.T) {
	var n int
	out := captureStdout(func() {
		n = countFixRows(func() {
			fmt.Println("    ✓ git: OK")
			fmt.Println("      ⚠ cplt 1 is out of date")
			fmt.Println("          Solution: Run brew upgrade navikt/tap/cplt")
			fmt.Println("      [✗] Binary not found on PATH")
		})
	})
	if n != 2 || !strings.Contains(out, "out of date") {
		t.Fatalf("counted %d rows (want 2), passed through %q", n, out)
	}
	if v := doctorVerdict(n, false); !strings.Contains(v, "2 things to fix; see the lines above") || strings.Contains(v, "healthy") {
		t.Errorf("verdict with rows: %q", v)
	}
	if v := doctorVerdict(1, false); !strings.Contains(v, "1 thing to fix") {
		t.Errorf("verdict with one row: %q", v)
	}
	if v := doctorVerdict(0, false); !strings.Contains(v, "All systems healthy") {
		t.Errorf("control: verdict with no rows: %q", v)
	}
}

// The recommended notice keeps its reasons and drops only its upgrade command
// when the version row gave one; control: not offered keeps it whole.
func TestRecommendedRowKeepsReasons(t *testing.T) {
	n := "cplt X or newer is recommended: it starts quietly. Update cplt: brew upgrade navikt/tap/cplt"
	if got := recommendedRow(n, true); got != "cplt X or newer is recommended: it starts quietly." {
		t.Errorf("offered: %q", got)
	}
	if got := recommendedRow(n, false); got != n {
		t.Errorf("control: %q", got)
	}
}

// doctor's version row reports whether it already offered the upgrade, so the
// recommended-version notice does not add a second upgrade line. Control: a
// current cplt offers none.
func TestCpltVersionRowReportsOfferedUpgrade(t *testing.T) {
	isolatedConfig(t)
	fakeCpltHosts(t)
	var behind, current bool
	captureStdout(func() {
		behind = reportCpltVersionWith("/usr/local/bin/cplt", "cplt 2026.09.01-000000-abcdef0", "2026.10.01-000000-abcdef0", nil)
		current = reportCpltVersionWith("/usr/local/bin/cplt", "cplt 2026.10.01-000000-abcdef0", "2026.10.01-000000-abcdef0", nil)
	})
	if !behind || current {
		t.Errorf("offered upgrade: behind=%v current=%v, want true false", behind, current)
	}
}

// A section header shows the worst state among its lines (#2905). Control: a
// section with only info lines keeps [i].
func TestDoctorSectionHeaderShowsWorstState(t *testing.T) {
	out := captureStdout(func() {
		countFixRows(func() {
			fmt.Println("[i] Clients")
			fmt.Println("    ✓ found")
			fmt.Println("    ⚠ out of date")
			fmt.Println("[i] Pins")
			fmt.Println("    ✓ none")
			fmt.Println("[i] Hooks")
			fmt.Println("    • none installed")
			fmt.Println("[i] Security")
			fmt.Println("    ⚠ waiver")
			fmt.Println("    [✗] not enforcing")
		})
	})
	for _, want := range []string{"[⚠] Clients\n", "[✓] Pins\n", "[i] Hooks\n", "[✗] Security\n"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

// A keyring login over HTTPS is one line; with an SSH origin the push warning
// still prints, and --verbose shows the full block.
func TestDoctorGHOneLineOnlyWithoutSSH(t *testing.T) {
	orig := originUsesSSH
	t.Cleanup(func() { originUsesSSH = orig; providerpkg.Verbose = false })
	for _, ssh := range []bool{false, true} {
		originUsesSSH = func() bool { return ssh }
		var b bytes.Buffer
		reportDoctorGH(&b, "copilot", "auto", ghAuthKeyring, "")
		out := stripANSI(b.String())
		oneLine := strings.Contains(out, "(push and PRs): ✓ gh is signed in")
		warned := strings.Contains(out, "origin uses SSH, which cplt blocks")
		if oneLine == ssh || warned != ssh {
			t.Errorf("ssh=%v: one-liner=%v warning=%v\n%s", ssh, oneLine, warned, out)
		}
	}
	originUsesSSH = func() bool { return false }
	providerpkg.Verbose = true
	var b bytes.Buffer
	reportDoctorGH(&b, "copilot", "auto", ghAuthKeyring, "")
	if strings.Contains(b.String(), "(push and PRs): ") || !strings.Contains(b.String(), "Sandbox authentication remains unverified") {
		t.Errorf("verbose: %s", b.String())
	}
}
