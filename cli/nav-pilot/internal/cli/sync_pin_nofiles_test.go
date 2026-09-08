package cli

import (
	"errors"
	"strings"
	"testing"
)

// Et scope kan erklære en pinne uten å ha filer å synke: det har ignorert alt,
// eller committet erklæringa før første install. Pinnen flytter seg likevel.
//
// Grenen for «ingen filer» leste aldri pendingPinBump, så sync meldte
// «No customization files found to sync» og gikk ut med 0. En planlagt
// workflow avgjør på exit-koden om den skal kjøre --apply, så pinnen råtnet
// nettopp i de repoene workflowen fantes for å holde ferske.
func TestSyncReportsAStalePinWithNoFilesInstalled(t *testing.T) {
	isolatedConfig(t)
	repo, first, second := gitAgentpakke(t)
	localAgentpakkeRemote(t, repo)

	scope := ScopeRepo(repoTarget(t))
	writeDeclaration(t, scope,
		`{"contractVersion":"1","source":"navikt/grillmester","sha":"`+first+`"}`)

	// Ingen install: erklæringa står alene, slik den gjør i et repo som har
	// committet pinnen men ikke installert noe ennå.
	out := captureStdoutFor(t, func() {
		err := cmdSync(scope, "", "", false, false)
		if !errors.Is(err, errUpdatesAvailable) {
			t.Errorf("sync ga %v, ventet errUpdatesAvailable for en foreldet pinne", err)
		}
	})
	if strings.Contains(out, "No customization files found") {
		t.Errorf("sync meldte at det ikke var noe å gjøre:\n%s", out)
	}

	// --apply skal flytte pinnen, ikke bare rapportere den.
	captureStdoutFor(t, func() {
		if err := cmdSync(scope, "", "", true, false); err != nil {
			t.Fatalf("sync --apply: %v", err)
		}
	})
	if got := readDeclaration(t, scope).SHA; got != second {
		t.Errorf("pinnen står på %q, ventet %q", got, second)
	}
}
