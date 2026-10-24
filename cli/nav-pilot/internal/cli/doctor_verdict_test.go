package cli

import (
	"fmt"
	"strings"
	"testing"
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
	if v := doctorVerdict(n, false); !strings.Contains(v, "2 things to fix; see the Solution lines") || strings.Contains(v, "healthy") {
		t.Errorf("verdict with rows: %q", v)
	}
	if v := doctorVerdict(1, false); !strings.Contains(v, "1 thing to fix") {
		t.Errorf("verdict with one row: %q", v)
	}
	if v := doctorVerdict(0, false); !strings.Contains(v, "All systems healthy") {
		t.Errorf("control: verdict with no rows: %q", v)
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
