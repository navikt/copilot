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
}
