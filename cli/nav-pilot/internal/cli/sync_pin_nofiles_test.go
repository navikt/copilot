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

// En erklæring kan navngi en kilde uten å pinne noe: det er formen en
// utvikler skriver for hånd før første sync. sync --apply skal fylle inn
// pinnen, ellers blir erklæringa stående uten sha for alltid, og poenget med
// å committe den, at teamet installerer samme revisjon, faller bort.
//
// Testen måler utfallet, ikke hvilket kall som gir det: pinnen fylles fra
// flere steder i sync, så å fjerne ett av dem endrer ingenting her.
func TestSyncApplyFillsInAMissingPin(t *testing.T) {
	isolatedConfig(t)
	repo, first, _ := gitAgentpakke(t)
	localAgentpakkeRemote(t, repo)

	scope := ScopeRepo(repoTarget(t))
	writeDeclaration(t, scope,
		`{"contractVersion":"1","source":"navikt/grillmester","sha":"`+first+`"}`)
	captureStdoutFor(t, func() {
		if err := cmdInstallAuto("grillmester", "", scope, "", "", false, false, false); err != nil {
			t.Fatalf("install: %v", err)
		}
	})

	// Ta bort pinnen, men behold kilden: filene er fortsatt oppdaterte.
	writeDeclaration(t, scope, `{"contractVersion":"1","source":"navikt/grillmester"}`)
	if got := readDeclaration(t, scope).SHA; got != "" {
		t.Fatalf("erklæringa har fortsatt en pinne %q, da tester dette noe annet", got)
	}

	captureStdoutFor(t, func() {
		if err := cmdSync(scope, "", "", true, false); err != nil {
			t.Fatalf("sync --apply: %v", err)
		}
	})
	if got := readDeclaration(t, scope).SHA; got == "" {
		t.Error("sync --apply fylte ikke inn den manglende pinnen")
	}
}

// sync --apply skal la staten stå på revisjonen den faktisk synket til.
//
// Navnet sier ikke «når alt er oppdatert», og det er med vilje: fixturet
// endrer filer mellom de to revisjonene, så denne kjøringa går
// oppdateringsstien. Grenen som frisker opp source_sha når ingenting har
// endret seg er dermed ikke dekket her. Den lar seg ikke nå med dette
// fixturet, som må ha to revisjoner med identisk innhold for å prøves, og
// det er notert framfor å bli påstått dekket.
func TestSyncApplyLeavesStateOnTheSyncedRevision(t *testing.T) {
	isolatedConfig(t)
	repo, first, second := gitAgentpakke(t)
	localAgentpakkeRemote(t, repo)

	scope := ScopeRepo(repoTarget(t))
	writeDeclaration(t, scope,
		`{"contractVersion":"1","source":"navikt/grillmester","sha":"`+first+`"}`)
	captureStdoutFor(t, func() {
		if err := cmdInstallAuto("grillmester", "", scope, "", "", false, false, false); err != nil {
			t.Fatalf("install: %v", err)
		}
	})
	captureStdoutFor(t, func() {
		if err := cmdSync(scope, "", "", true, false); err != nil {
			t.Fatalf("sync --apply: %v", err)
		}
	})

	state, err := readScopedState(scope)
	if err != nil || state == nil {
		t.Fatalf("leste ikke staten: %v", err)
	}
	if state.SourceSHA == first {
		t.Errorf("staten står igjen på den gamle revisjonen %q", shortSHA(first))
	}
	if state.SourceSHA != second {
		t.Errorf("staten har source_sha %q, ventet %q", state.SourceSHA, second)
	}
}
