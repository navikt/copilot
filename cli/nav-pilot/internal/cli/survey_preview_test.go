package cli

import (
	"strings"
	"testing"

	"github.com/navikt/copilot/cli/nav-pilot/internal/agentpakke"
)

// Only `survey` and `survey preview <id>` run; anything else is a usage error,
// so a typo can never open the live survey.
func TestSurveyRejectsMalformedSubcommands(t *testing.T) {
	for _, args := range [][]string{
		{"survey", "preview"},
		{"survey", "previev", "q4-2026"},
		{"survey", "preview", "q4-2026", "extra"},
		{"survey", "preview", "q4-2026", "--json"},
		{"survey", "q4-2026"},
	} {
		err := run(args)
		if err == nil || !strings.Contains(err.Error(), "usage: nav-pilot survey") {
			t.Errorf("%v: err = %v, want usage error", args, err)
		}
	}
	// Well-formed reaches the preview, which checks the id before anything else.
	err := run([]string{"survey", "preview", "Q4 2026"})
	if err == nil || !strings.Contains(err.Error(), "ugyldig id") {
		t.Errorf("preview with bad id: err = %v, want ugyldig id", err)
	}
}

func TestSurveyPreviewHonoursMinCLIVersion(t *testing.T) {
	orig := agentpakke.RunningVersion()
	t.Cleanup(func() { agentpakke.SetVersion(orig) })
	for _, tc := range []struct {
		running, floor string
		want           bool
	}{
		{"2026.09.24-120000-abc1234", "2026.09.29-072249", true},
		{"2026.09.29-072249-abc1234", "2026.09.29-072249", false},
		{"2026.09.29-072249-abc1234", "2026.09.29", false},
		{"2026.09.28-235959-abc1234", "2026.09.29", true},
		{"2026.09.24-120000-abc1234", "", false},
		{"dev", "2026.09.29-072249", false},
		{"2026.09.24-120000-abc1234", "nonsense", true},
	} {
		agentpakke.SetVersion(tc.running)
		if got := (surveyDef{MinCLIVersion: tc.floor}).needsNewerCLI(); got != tc.want {
			t.Errorf("running %s, floor %q: needsNewerCLI = %v, want %v", tc.running, tc.floor, got, tc.want)
		}
	}
}
