package domain

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// One spelling of the cplt install and upgrade commands: the tap name the
// formula lives under (brew info navikt/tap/cplt), on every package manager.
func TestCpltHints(t *testing.T) {
	for _, tc := range []struct{ got, want string }{
		{CpltInstallHint(PkgBrew), "brew install navikt/tap/cplt"},
		{CpltInstallHint(PkgApt), "sudo apt install cplt"},
		{CpltUpgradeHint(PkgBrew), "brew upgrade navikt/tap/cplt"},
		{CpltUpgradeHint(PkgNone), "cplt update"},
		{CpltUpgradeHint(PkgApt), "sudo apt update && sudo apt install --only-upgrade cplt"},
	} {
		if tc.got != tc.want {
			t.Errorf("got %q, want %q", tc.got, tc.want)
		}
	}
}

// No other source file spells a cplt install or upgrade command itself. The
// control: the pattern finds the spellings in pkgmanager.go.
func TestCpltHintsSpelledOnce(t *testing.T) {
	re := regexp.MustCompile(`(brew (install|upgrade) (navikt/tap/)?cplt|sudo apt (update && sudo apt )?install( --only-upgrade)? cplt)`)
	var hits []string
	err := filepath.WalkDir("..", func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if re.Match(data) {
			hits = append(hits, filepath.ToSlash(path))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 || hits[0] != "../domain/pkgmanager.go" {
		t.Errorf("cplt install/upgrade spelled in %v, want only ../domain/pkgmanager.go", hits)
	}
}
