package cli

import (
	"context"
	"os"
	"testing"
)

// TestDiscoverPakkeReleaseLive crosses the real boundary the httptest suite
// stubs: the GitHub API's actual release JSON. Opt-in, since it needs the
// network:
//
//	NAV_PILOT_LIVE_GITHUB=1 go test ./internal/cli/ -run TestDiscoverPakkeReleaseLive -v
func TestDiscoverPakkeReleaseLive(t *testing.T) {
	if os.Getenv("NAV_PILOT_LIVE_GITHUB") == "" {
		t.Skip("set NAV_PILOT_LIVE_GITHUB=1 to query api.github.com")
	}
	names := map[releaseOutcome]string{
		releaseNoMetadata: "no metadata", releaseUpToDate: "up to date",
		releaseCandidate: "candidate", releaseNotOffered: "not offered",
	}
	for _, c := range []struct{ repo, name string }{
		{"navikt/grillmester", "grillmester"}, // releases, none with the asset yet
		{"navikt/.github", ".github"},         // no releases at all
	} {
		outcome, rel, err := discoverPakkeReleaseHTTP(context.Background(), c.repo, c.name, "")
		t.Logf("%s: outcome=%s release=%+v err=%v", c.repo, names[outcome], rel, err)
		if err != nil {
			t.Errorf("%s: %v", c.repo, err)
		}
	}

	// The reachability check against real commits: Grillmester's source commit
	// is on main, and the v0.4.0 catalog commit shares no history with it.
	ctx := context.Background()
	src := "20d634fe960eb6ac981a4d7b88bd1fb078e48f4b"
	status, err := compareStatus(ctx, "navikt/grillmester", src, "main")
	t.Logf("compare %s...main: status=%q err=%v", src[:7], status, err)
	if err != nil || (status != "ahead" && status != "identical") {
		t.Errorf("the source commit is not reported on main")
	}
	status, err = compareStatus(ctx, "navikt/grillmester", src, "fe686e203b38250f4242cd95a9d228f424e3ff35")
	t.Logf("compare %s...fe686e2: status=%q err=%v", src[:7], status, err)
	if err == nil {
		t.Errorf("unrelated histories compared without error")
	}
}
