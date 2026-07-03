package cli

import (
	"strings"
	"testing"
)

func TestFormatNorwegianNumber(t *testing.T) {
	cases := map[int]string{
		0:       "0",
		42:      "42",
		999:     "999",
		1000:    "1 000",
		151354:  "151 354",
		-1234:   "-1 234",
		1000000: "1 000 000",
	}
	for in, want := range cases {
		if got := formatNorwegianNumber(in); got != want {
			t.Errorf("formatNorwegianNumber(%d) = %q, want %q", in, got, want)
		}
	}
}

func TestProgressBar(t *testing.T) {
	if got := progressBar(0, 10); got != strings.Repeat("░", 10) {
		t.Errorf("progressBar(0,10) = %q", got)
	}
	if got := progressBar(100, 10); got != strings.Repeat("█", 10) {
		t.Errorf("progressBar(100,10) = %q", got)
	}
	if got := progressBar(50, 10); got != strings.Repeat("█", 5)+strings.Repeat("░", 5) {
		t.Errorf("progressBar(50,10) = %q", got)
	}
	// Out-of-range values should clamp rather than panic.
	if got := progressBar(150, 10); got != strings.Repeat("█", 10) {
		t.Errorf("progressBar(150,10) should clamp: got %q", got)
	}
	if got := progressBar(-10, 10); got != strings.Repeat("░", 10) {
		t.Errorf("progressBar(-10,10) should clamp: got %q", got)
	}
}

func TestCapitalize(t *testing.T) {
	if got := capitalize(""); got != "" {
		t.Errorf("capitalize(\"\") = %q", got)
	}
	if got := capitalize("active"); got != "Active" {
		t.Errorf("capitalize(active) = %q", got)
	}
}

func TestFormatUsageTerminalDoesNotPanic(t *testing.T) {
	u := &usageResponse{Period: "November 2025"}
	u.Credits.Used = 4500
	u.Credits.Limit = 10000
	u.Credits.Percentage = 45
	u.Interactions.Total = 320
	u.Interactions.Accepted = 210
	u.Interactions.AcceptanceRate = 65.6
	u.ActiveDays = 18
	u.Subscription.Status = "active"

	out := formatUsageTerminal(u)
	if !strings.Contains(out, "4 500") || !strings.Contains(out, "10 000") {
		t.Errorf("expected formatted credit numbers in output: %s", out)
	}
	if !strings.Contains(out, "45%") {
		t.Errorf("expected percentage in output: %s", out)
	}
}

func TestFormatUsageTmux(t *testing.T) {
	u := &usageResponse{}
	u.Credits.Percentage = 72
	if got := formatUsageTmux(u); got != "Copilot 72%" {
		t.Errorf("formatUsageTmux = %q", got)
	}
}
