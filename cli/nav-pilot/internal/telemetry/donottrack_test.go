package telemetry

import "testing"

// TestDoNotTrackWins: a developer who set DO_NOT_TRACK has already answered this
// question, for every tool on the machine, and should not have to answer it again
// per CLI. It takes precedence over nav-pilot's own variable in both directions:
// setting it disables telemetry even when ours says enabled.
func TestDoNotTrackWins(t *testing.T) {
	for _, tc := range []struct {
		name, dnt, ours string
		want            bool
	}{
		{name: "nothing set", want: true},
		{name: "DO_NOT_TRACK=1", dnt: "1", want: false},
		{name: "DO_NOT_TRACK=true", dnt: "true", want: false},
		{name: "DO_NOT_TRACK=0 is not opting out", dnt: "0", want: true},
		{name: "DO_NOT_TRACK=false is not opting out", dnt: "false", want: true},
		{name: "DO_NOT_TRACK beats our own enable", dnt: "1", ours: "true", want: false},
		{name: "ours alone still disables", ours: "false", want: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("DO_NOT_TRACK", tc.dnt)
			t.Setenv("NAV_PILOT_TELEMETRY_ENABLED", tc.ours)
			if got := TelemetryEnabled(); got != tc.want {
				t.Errorf("TelemetryEnabled() = %v, want %v", got, tc.want)
			}
		})
	}
}
