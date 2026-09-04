package cli

import (
	"os"
	"strings"
	"testing"
)

// ─── sync round-trips a SHA through real git ─────────────────────────────────

// TestSyncApplyWritesAFetchablePin closes the seam every other sync test left
// open. All six of them replace resolveSourceForSync with a stub that hands
// back a fabricated SHA, so nothing on the sync path has ever round-tripped a
// revision id back through git — which is exactly how #597 shipped an install
// pin that git refused to fetch, behind ten green tests.
//
// Here the resolver is not stubbed. `sync --apply` clones the source for real
// (RemoteURLFn points the real plumbing at a repository on disk), and the SHA
// it writes into the repo's declaration is then proved fetchable the only way
// that means anything: a second, fresh scope installs from it and gets the
// content that revision holds.
func TestSyncApplyWritesAFetchablePin(t *testing.T) {
	isolatedConfig(t)
	repo, first, second := gitAgentpakke(t)
	localAgentpakkeRemote(t, repo)

	scope := ScopeRepo(repoTarget(t))
	writeDeclaration(t, scope,
		`{"contractVersion":"1","source":"navikt/grillmester","sha":"`+first+`"}`)
	captureStdoutFor(t, func() {
		if err := cmdInstallAuto("grillmester", "", scope, "", "", false, false, false); err != nil {
			t.Fatalf("install from the declared pin: %v", err)
		}
	})
	if !strings.Contains(installedAgentBody(t, scope), "REVISION ONE") {
		t.Fatal("the fixture install did not get the pinned revision")
	}

	// The resolver is left alone on purpose: this is the whole point of the test.
	captureStdoutFor(t, func() {
		if err := cmdSync(scope, "", "", true, false); err != nil {
			t.Fatalf("sync --apply through the real resolver: %v", err)
		}
	})
	if !strings.Contains(installedAgentBody(t, scope), "REVISION TWO") {
		t.Error("sync --apply did not bring the scope onto the newer revision")
	}

	// The assertion the stubs could never make, and the one that comes first
	// because it is the one that matters: what sync wrote is fetchable.
	pinned := readDeclaration(t, scope).SHA
	fresh := ScopeRepo(repoTarget(t))
	writeDeclaration(t, fresh,
		`{"contractVersion":"1","source":"navikt/grillmester","sha":"`+pinned+`"}`)
	captureStdoutFor(t, func() {
		if err := cmdInstallAuto("grillmester", "", fresh, "", "", false, false, false); err != nil {
			t.Fatalf("the revision sync wrote cannot be fetched back: %v", err)
		}
	})
	if !strings.Contains(installedAgentBody(t, fresh), "REVISION TWO") {
		t.Error("installing the pin sync wrote did not produce the revision it names")
	}
	if pinned != second {
		t.Errorf("sync pinned %q, want the revision git resolved to, %q", pinned, second)
	}
}

// ─── #605: a pin recorded before #597 is seven characters ────────────────────

// A Tier 2 pin written by an older binary holds an abbreviated SHA, and the
// source now resolves to all forty characters of the same commit. Sync must
// see one revision, not two: the old comparison reported a newer revision that
// does not exist — printing the same seven characters on both sides of the
// sentence — and --apply then re-fetched and re-materialized the revision
// already pinned into a second directory under pakkerRoot().
func TestSyncPinToleratesAnAbbreviatedRecordedSHA(t *testing.T) {
	scope := pinEnv(t)
	tree := tier2PinSourceTree(t)
	const full = "abc1234def567890abcdef1234567890abcdef12"
	short := full[:7]

	old := &Source{Dir: tree, SHA: short, Version: "dev", Repo: "navikt/grillmester"}
	if err := attachPakke(old); err != nil {
		t.Fatal(err)
	}
	installPin(t, scope, old)

	pinnedSyncSource(t, tree, full)

	var err error
	out := captureStdoutFor(t, func() { err = cmdSync(scope, "", "", false, false) })
	if err != nil {
		t.Fatalf("sync over a pre-#597 pin = %v, want up to date. Output:\n%s", err, out)
	}
	if strings.Contains(out, "newer revision") {
		t.Errorf("sync announced an update to the revision already pinned:\n%s", out)
	}
	if !strings.Contains(out, "up to date") {
		t.Errorf("sync did not report the pin as up to date:\n%s", out)
	}

	// --apply must not re-materialize the same commit under its long name.
	captureStdoutFor(t, func() { err = cmdSync(scope, "", "", true, false) })
	if err != nil {
		t.Fatalf("sync --apply over a pre-#597 pin = %v, want nil", err)
	}
	if _, statErr := os.Stat(pakkeRevisionDir("navikt/grillmester", full)); !os.IsNotExist(statErr) {
		t.Errorf("sync --apply materialized a second directory for the revision already pinned (stat err %v)", statErr)
	}
	if state, _ := readScopedState(scope); state == nil || state.SourceSHA != short {
		t.Errorf("state after --apply = %+v, want the pin left at %q", state, short)
	}
}

func TestSameSHA(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"abc1234", "abc1234def567890abcdef1234567890abcdef12", true},
		{"abc1234def567890abcdef1234567890abcdef12", "abc1234", true},
		{"abc1234", "abc1235def567890abcdef1234567890abcdef12", false},
		{"abc1234", "abc1234", true},
		{"", "abc1234", false}, // "" is nobody's prefix
		{"abc1234", "", false},
		{"", "", true},
	}
	for _, c := range cases {
		if got := sameSHA(c.a, c.b); got != c.want {
			t.Errorf("sameSHA(%q, %q) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}
