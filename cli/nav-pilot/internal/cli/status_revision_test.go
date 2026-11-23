package cli

import (
	"strings"
	"testing"
)

// #2749: status names the revision stampRevision recorded, one line per scope,
// and says nothing when none is recorded.
func TestStatusShowsRecordedRevision(t *testing.T) {
	isolatedConfig(t)
	a, b := strings.Repeat("a", 40), strings.Repeat("b", 40)
	status := func(files ...InstalledFile) string {
		return captureStdoutFor(t, func() {
			printStatusBlock(ScopeRepo(t.TempDir()), &StateFile{Collection: "x", Files: files})
		})
	}

	if out := status(InstalledFile{Path: "a", Revision: a}, InstalledFile{Path: "b", Revision: a}); strings.Count(out, "Installert fra "+shortSHA(a)+"\n") != 1 {
		t.Errorf("present: want one line naming %s, got:\n%s", shortSHA(a), out)
	}
	if out := status(InstalledFile{Path: "a"}); strings.Contains(out, "Installert fra") {
		t.Errorf("absent: want no line, got:\n%s", out)
	}
	// Two scopes, two revisions: each block carries its own line.
	repo, user := status(InstalledFile{Path: "a", Revision: a}), status(InstalledFile{Path: "a", Revision: b})
	if !strings.Contains(repo, shortSHA(a)) || strings.Contains(repo, shortSHA(b)) || !strings.Contains(user, shortSHA(b)) {
		t.Errorf("multiple scopes: repo=%q user=%q", repo, user)
	}
}
