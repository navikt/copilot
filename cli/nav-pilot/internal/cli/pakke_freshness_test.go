package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func agentFile(model string) string {
	return "---\nname: a\nmodel: " + model + "\n---\nbody\n"
}

// doctor names an agent whose model the base has moved since the pakke's pin,
// and nothing else: not the pakke's own override, not an agent the base does
// not ship, not a lookup that failed.
func TestDoctorReportsPinDrift(t *testing.T) {
	scope := ScopeRepo(repoTarget(t))
	installed := map[string]string{
		"security-champion": "gpt-6-sol", // base moved it: warn
		"override":          "own-model", // the pakke chose its own: silent
		"own":               "gpt-6-sol", // the base does not ship it: silent
		"same":              "claude-x",  // unchanged: silent
		"broken":            "gpt-6-sol", // lookup fails: silent
	}
	var files []InstalledFile
	for name, model := range installed {
		p := filepath.Join(".github", "agents", name+".agent.md")
		mustWrite(t, filepath.Join(scope.RootDir, p), agentFile(model))
		files = append(files, InstalledFile{Path: p, Hash: "h"})
	}
	orig := githubFile
	t.Cleanup(func() { githubFile = orig })
	githubFile = func(ctx context.Context, repo, path, ref string) ([]byte, error) {
		if repo != "navikt/copilot" {
			t.Errorf("read %s from %s", path, repo)
		}
		// Each request gets its own deadline: a shared one let slow early
		// answers drop every agent after them, unreported.
		if d, ok := ctx.Deadline(); !ok || time.Until(d) < pinDriftRequestTimeout-time.Second {
			t.Errorf("%s@%s: deadline %v is not this request's own", path, ref, d)
		}
		head := map[string]string{"security-champion": "claude-opus-5.5", "override": "claude-opus-5.5", "same": "claude-x"}
		pin := map[string]string{"security-champion": "gpt-6-sol", "override": "gpt-6-sol"}
		name := strings.TrimSuffix(strings.TrimPrefix(path, "agents/"), ".agent.md")
		m := head
		if ref == basePin {
			m = pin
		}
		if model, ok := m[name]; ok {
			return []byte(agentFile(model)), nil
		}
		if name == "broken" {
			return nil, context.DeadlineExceeded
		}
		return nil, fmt.Errorf("%s: %w", path, errGitHubNotFound)
	}
	out := captureStdoutFor(t, func() {
		reportPinDrift(scope, &StateFile{SourceRepo: "nais/pilot", Files: files}, "navikt/copilot", basePin)
	})
	want := "      ⚠ @security-champion runs gpt-6-sol; navikt/copilot now pins claude-opus-5.5.\n" +
		"      When the owners of nais/pilot have updated, run nav-pilot sync --apply.\n" +
		"      Could not check 1 agent(s) against navikt/copilot (network or rate limit).\n"
	if out != want {
		t.Errorf("want:\n%s\ngot:\n%s", want, out)
	}
}

// doctor says it could not tell, rather than nothing, when the lookup fails;
// a pakke without a lock (a 404) reuses nothing and gets no line.
func TestDoctorSaysWhenItCouldNotCheck(t *testing.T) {
	orig := githubFileJSON
	t.Cleanup(func() { githubFileJSON = orig })
	state := &StateFile{SourceRepo: "nais/pilot", SourceSHA: strings.Repeat("a", 40),
		Files: []InstalledFile{{Path: ".github/agents/x.agent.md", Hash: "h"}}}
	for _, tt := range []struct {
		err  error
		want string
	}{
		{errOfflineForTests, "Could not check whether nais/pilot is up to date (network or rate limit)."},
		{fmt.Errorf("x: %w", errGitHubNotFound), ""},
	} {
		githubFileJSON = func(context.Context, string, string, string, any) error { return tt.err }
		out := captureStdoutFor(t, func() { reportScopeBaseLag(ScopeRepo(repoTarget(t)), state) })
		if strings.TrimSpace(out) != tt.want {
			t.Errorf("%v: got %q, want %q", tt.err, out, tt.want)
		}
	}
}

// A content install from another pakke is offered its own newer revision at
// launch, once a day, and offline or declined it says nothing and waits for
// nothing.
func TestPakkeScopeUpdate(t *testing.T) {
	state := &StateFile{SourceRepo: "nais/pilot", SourceSHA: strings.Repeat("a", 40), Collection: "pilot",
		Files: []InstalledFile{{Path: ".github/agents/x.agent.md", Hash: "h"}}}
	stub := func(t *testing.T, newer string, err error) *int {
		calls := new(int)
		orig := lookupPakkeUpdate
		t.Cleanup(func() { lookupPakkeUpdate = orig })
		lookupPakkeUpdate = func(ctx context.Context, repo, name, installed string) (*pakkeRelease, error) {
			*calls++
			if _, ok := ctx.Deadline(); !ok {
				t.Error("the launch lookup has no deadline")
			}
			if newer == "" {
				return nil, err
			}
			return &pakkeRelease{Version: newer, SHA: strings.Repeat("b", 40)}, err
		}
		return calls
	}
	t.Run("newer, then cached", func(t *testing.T) {
		isolatedConfig(t)
		calls := stub(t, "0.5.0", nil)
		if got := scopeStaleness(ScopeRepo(repoTarget(t)), state); got != "0.5.0" {
			t.Errorf("got %q", got)
		}
		if got := pakkeScopeUpdate(state); got != "0.5.0" || *calls != 1 {
			t.Errorf("second launch: %q after %d lookups, want the cached answer", got, *calls)
		}
	})
	t.Run("offline", func(t *testing.T) {
		isolatedConfig(t)
		calls := stub(t, "", errOfflineForTests)
		if got := pakkeScopeUpdate(state); got != "" {
			t.Errorf("offline offered %q", got)
		}
		if pakkeScopeUpdate(state); *calls != 1 {
			t.Errorf("an offline answer was not cached: %d lookups", *calls)
		}
	})
	t.Run("declined", func(t *testing.T) {
		isolatedConfig(t)
		calls := stub(t, "0.5.0", nil)
		p := syncDeclinedPath()
		mustWrite(t, p, "x\n")
		if got := pakkeScopeUpdate(state); got != "" || *calls != 0 {
			t.Errorf("declined: %q after %d lookups", got, *calls)
		}
		_ = os.Remove(p)
	})
	t.Run("a release sync would refuse is not offered", func(t *testing.T) {
		isolatedConfig(t)
		stub(t, "0.5.0", nil)
		held := *state
		held.RolledBackFrom = strings.Repeat("b", 40)
		if got := pakkeScopeUpdate(&held); got != "" {
			t.Errorf("offered %q, a release this scope was rolled back off", got)
		}
	})
	t.Run("pin is left to the pin's own prompt", func(t *testing.T) {
		isolatedConfig(t)
		calls := stub(t, "0.5.0", nil)
		pin := *state
		pin.Files = nil
		if got := pakkeScopeUpdate(&pin); got != "" || *calls != 0 {
			t.Errorf("pin: %q after %d lookups", got, *calls)
		}
	})
}
