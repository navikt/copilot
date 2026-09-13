package cli

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// #783: går en ny release i stykker, skal brukeren kunne gå tilbake til forrige
// verifiserte revisjon uten nett, og bli stående der. Testene under dekker de
// fire påstandene den funksjonen består av: rollback uten nett, rollback uten
// noe å gå tilbake til, at rollbacken står seg ved neste sync, og at den ikke
// rører treet en annen økt leser fra.

// utenNett lar enhver vei ut på nettet feile testen. Rollbacken leser bare
// disk, så både release-oppslaget og kildeoppløsningen skal stå urørt — og
// #830 er nettopp historien om et oppslag ingen la merke til.
func utenNett(t *testing.T) {
	t.Helper()
	origRelease := discoverPakkeRelease
	origResolve := resolveSourceForSync
	t.Cleanup(func() {
		discoverPakkeRelease = origRelease
		resolveSourceForSync = origResolve
	})
	discoverPakkeRelease = func(context.Context, string, string, string) (releaseOutcome, pakkeRelease, error) {
		t.Error("rollback slo opp releases; den skal ikke røre nettet")
		return releaseNoMetadata, pakkeRelease{}, nil
	}
	resolveSourceForSync = func(string, string) (*Source, error) {
		t.Error("rollback resolvet kilden; den skal ikke røre nettet")
		return nil, nil
	}
}

// toRevisjoner pinner shaA og deretter shaB, slik oppbevaringsregelen gir to
// revisjoner på disk. shaA aldres bakover, så previousRevision avgjøres av
// rekkefølgen og ikke av at to materialiseringer landet i samme mtime-tikk.
func toRevisjoner(t *testing.T) *InstallScope {
	t.Helper()
	scope := pinEnv(t)
	installPin(t, scope, tier2PinSource(t, shaA))
	eldre := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(pakkeRevisionDir("navikt/grillmester", shaA), eldre, eldre); err != nil {
		t.Fatal(err)
	}
	installPin(t, scope, tier2PinSource(t, shaB))
	return scope
}

func rollbackKjør(t *testing.T, jsonOutput bool) (string, error) {
	t.Helper()
	var err error
	out := captureStdoutFor(t, func() { err = cmdRollback(jsonOutput) })
	return out, err
}

// TestRollbackUtenNettLeserForrigeRevisjon er selve behovet: releasen som
// ligger pinnet er ødelagt, maskinen er uten nett, og forrige revisjon ligger
// der fortsatt. Etter rollbacken er det den launchen leser.
func TestRollbackUtenNettLeserForrigeRevisjon(t *testing.T) {
	scope := toRevisjoner(t)
	utenNett(t)

	out, err := rollbackKjør(t, false)
	if err != nil {
		t.Fatalf("rollback = %v. Utskrift:\n%s", err, out)
	}
	if !strings.Contains(out, shortSHA(shaA)) {
		t.Errorf("rollbacken navnga ikke revisjonen den gikk til. Utskrift:\n%s", out)
	}

	state, _ := readScopedState(scope)
	if state == nil || !sameSHA(state.SourceSHA, shaA) {
		t.Fatalf("state = %+v, vil ha pinne på %s", state, shaA)
	}
	if !sameSHA(state.RolledBackFrom, shaB) {
		t.Errorf("rolled_back_from = %q, vil ha %s", state.RolledBackFrom, shaB)
	}

	// Det launchen faktisk leser, ikke bare det staten sier.
	rev, err := pinnedRevision("navikt/grillmester")
	if err != nil || rev == nil {
		t.Fatalf("pinnedRevision = %v, %v", rev, err)
	}
	if rev.Dir != pakkeRevisionDir("navikt/grillmester", shaA) {
		t.Errorf("launchen leser %s, vil ha revisjonen for %s", rev.Dir, shortSHA(shaA))
	}
	assertRevisionVerifies(t, rev.Dir)
}

// TestRollbackUtenForrigeRevisjonSierDet: oppbevaringsregelen er to
// revisjoner, så rollback er ikke alltid mulig. Da skal kommandoen si det, ikke
// feile med noe som ligner en ødelagt installasjon.
func TestRollbackUtenForrigeRevisjonSierDet(t *testing.T) {
	scope := pinEnv(t)
	installPin(t, scope, tier2PinSource(t, shaA))
	utenNett(t)

	out, err := rollbackKjør(t, false)
	if err == nil {
		t.Fatalf("rollback uten forrige revisjon lyktes. Utskrift:\n%s", out)
	}
	if !strings.Contains(err.Error(), "no older revision") {
		t.Errorf("feilen sier ikke at det ikke finnes en eldre revisjon: %v", err)
	}
	state, _ := readScopedState(scope)
	if state == nil || !sameSHA(state.SourceSHA, shaA) {
		t.Errorf("pinnen flyttet seg likevel: %+v", state)
	}
}

// TestRollbackToGangerNekter: etter én rollback er den nyeste andre revisjonen
// nettopp den rollbacken forlot. Å «gå tilbake» til den ville vært å gå fram
// igjen til revisjonen brukeren nettopp forkastet.
func TestRollbackToGangerNekter(t *testing.T) {
	toRevisjoner(t)
	utenNett(t)

	if _, err := rollbackKjør(t, false); err != nil {
		t.Fatalf("første rollback = %v", err)
	}
	out, err := rollbackKjør(t, false)
	if err == nil {
		t.Fatalf("andre rollback lyktes og gikk fram igjen til %s. Utskrift:\n%s", shortSHA(shaB), out)
	}
	if !strings.Contains(err.Error(), "no older revision") {
		t.Errorf("feilen sier ikke at det ikke finnes en eldre revisjon: %v", err)
	}
}

// TestRollbackStårSegVedNesteSync er spørsmålet som avgjør om funksjonen er
// verdt noe: uten markøren finner neste sync den forkastede revisjonen nyere og
// setter pinnen rett tilbake på den.
func TestRollbackStårSegVedNesteSync(t *testing.T) {
	scope := toRevisjoner(t)
	utenNett(t)
	if _, err := rollbackKjør(t, false); err != nil {
		t.Fatalf("rollback = %v", err)
	}

	// Nettet er tilbake, og oppslaget tilbyr nøyaktig revisjonen som ble
	// forkastet.
	releaseSyncSource(t, shaB)
	stubRelease(t, releaseCandidate, pakkeRelease{Version: "0.5.0", SHA: shaB, Tag: "v0.5.0"}, nil)

	var err error
	out := captureStdoutFor(t, func() { err = cmdSync(scope, "", "", false, false) })
	if err != nil {
		t.Fatalf("sync = %v, vil ha ingenting å gjøre. Utskrift:\n%s", err, out)
	}
	if !strings.Contains(out, "rolled back") {
		t.Errorf("sync sa ikke fra om at scopet er rullet tilbake. Utskrift:\n%s", out)
	}

	out = captureStdoutFor(t, func() { err = cmdSync(scope, "", "", true, false) })
	if err != nil {
		t.Fatalf("sync --apply = %v. Utskrift:\n%s", err, out)
	}
	state, _ := readScopedState(scope)
	if state == nil || !sameSHA(state.SourceSHA, shaA) {
		t.Fatalf("sync --apply flyttet pinnen tilbake til den forkastede revisjonen: %+v", state)
	}
}

// TestSyncMedRefGårTilbakeTilRevisjonenSomBleForkastet: markøren stenger ikke
// veien tilbake, den fjerner bare det automatiske valget. En eksplisitt --ref
// er et pinningvalg og vinner, og pinnen den skriver bærer ingen markør.
func TestSyncMedRefGårTilbakeTilRevisjonenSomBleForkastet(t *testing.T) {
	scope := toRevisjoner(t)
	utenNett(t)
	if _, err := rollbackKjør(t, false); err != nil {
		t.Fatalf("rollback = %v", err)
	}

	releaseSyncSource(t, shaB)
	stubRelease(t, releaseCandidate, pakkeRelease{Version: "0.5.0", SHA: shaB, Tag: "v0.5.0"}, nil)

	var err error
	out := captureStdoutFor(t, func() { err = cmdSync(scope, shaB, "", true, false) })
	if err != nil {
		t.Fatalf("sync --apply --ref = %v. Utskrift:\n%s", err, out)
	}
	state, _ := readScopedState(scope)
	if state == nil || !sameSHA(state.SourceSHA, shaB) {
		t.Fatalf("--ref flyttet ikke pinnen tilbake: %+v", state)
	}
	if state.RolledBackFrom != "" {
		t.Errorf("rolled_back_from = %q, vil ha tom: pinnen har svart på spørsmålet markøren stiller", state.RolledBackFrom)
	}
}

// TestRollbackRørerIkkeRevisjonenEnAnnenØktHolder: revisjonen rollbacken
// forlater kan være treet en levende økt leser som OPENCODE_CONFIG_DIR. Den
// slettes ikke — hverken av rollbacken eller som en bieffekt av den.
func TestRollbackRørerIkkeRevisjonenEnAnnenØktHolder(t *testing.T) {
	scope := toRevisjoner(t)
	// Økta leser revisjonen som er pinnet nå, og lever videre gjennom
	// rollbacken.
	annenØkt(t, "navikt/grillmester", shaB)
	utenNett(t)

	if _, err := rollbackKjør(t, false); err != nil {
		t.Fatalf("rollback = %v", err)
	}

	revB := pakkeRevisionDir("navikt/grillmester", shaB)
	if _, err := os.Stat(revB); err != nil {
		t.Fatalf("rollbacken tok revisjonen en økt leser fra: %v", err)
	}
	assertRevisionVerifies(t, revB)
	if held := heldNames(t, "navikt/grillmester"); len(held) != 1 || held[0] != shaB {
		t.Errorf("markørene etter rollbacken = %v, vil ha [%s]", held, shaB)
	}
	// Og staten er flyttet, så rollbacken gjorde jobben sin uten å røre treet.
	state, _ := readScopedState(scope)
	if state == nil || !sameSHA(state.SourceSHA, shaA) {
		t.Fatalf("state = %+v, vil ha pinne på %s", state, shaA)
	}
}

// TestRollbackJSONSierHvaSomSkjedde: rollback er en gjenopprettingskommando, og
// den kjøres av skript like ofte som av mennesker.
func TestRollbackJSONSierHvaSomSkjedde(t *testing.T) {
	toRevisjoner(t)
	utenNett(t)

	out, err := rollbackKjør(t, true)
	if err != nil {
		t.Fatalf("rollback --json = %v. Utskrift:\n%s", err, out)
	}
	var doc struct {
		Command        string `json:"command"`
		SourceSHA      string `json:"source_sha"`
		RolledBackFrom string `json:"rolled_back_from"`
	}
	if jerr := json.Unmarshal([]byte(out), &doc); jerr != nil {
		t.Fatalf("utskriften er ikke ett JSON-dokument: %v\n%s", jerr, out)
	}
	if doc.Command != "rollback" || !sameSHA(doc.SourceSHA, shaA) || !sameSHA(doc.RolledBackFrom, shaB) {
		t.Errorf("dokumentet = %+v", doc)
	}
}

// TestRollbackNekterEnRevisjonSomIkkeVerifiserer: treet har ligget på disk
// siden det ble stagd, og drift er den forventede feilen for et så gammelt tre.
// Rollbacken verifiserer det på nytt framfor å stole på at det var verifisert
// den gangen det ble lagt der.
func TestRollbackNekterEnRevisjonSomIkkeVerifiserer(t *testing.T) {
	scope := toRevisjoner(t)
	utenNett(t)

	fil := filepath.Join(pakkeRevisionDir("navikt/grillmester", shaA), "copilot", "full", "agents", "grillmester.agent.md")
	if err := os.WriteFile(fil, []byte("noe annet\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := rollbackKjør(t, false)
	if err == nil {
		t.Fatalf("rollback til et tre som ikke verifiserer lyktes. Utskrift:\n%s", out)
	}
	if !strings.Contains(err.Error(), "does not verify") {
		t.Errorf("feilen sier ikke at revisjonen ikke verifiserer: %v", err)
	}
	state, _ := readScopedState(scope)
	if state == nil || !sameSHA(state.SourceSHA, shaB) {
		t.Errorf("pinnen flyttet seg likevel: %+v", state)
	}
}

// TestRollbackHindrerSpørsmåletVedOppstart: oppstartsspørsmålet er den andre
// veien pinnen flyttes automatisk. Et «ja» der ville pinnet nøyaktig den
// revisjonen brukeren nettopp forkastet, så spørsmålet stilles ikke.
func TestRollbackHindrerSpørsmåletVedOppstart(t *testing.T) {
	e := newPromptEnv(t) // pinnet på shaC, standardgrenen på shaB
	eldre := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(pakkeRevisionDir("navikt/grillmester", shaC), eldre, eldre); err != nil {
		t.Fatal(err)
	}
	installPin(t, e.scope, tier2PinSource(t, shaA)) // shaA pinnes, shaC er forrige
	if _, err := rollbackKjør(t, false); err != nil {
		t.Fatalf("rollback = %v", err)
	}

	// Oppslaget tilbyr nøyaktig revisjonen rollbacken forlot.
	stubRelease(t, releaseCandidate, pakkeRelease{Version: "0.5.0", SHA: shaA, Tag: "v0.5.0"}, nil)
	e.launch(t)
	if len(e.asked) != 0 {
		t.Errorf("oppstarten spurte om den forkastede revisjonen: %v", e.asked)
	}
	e.assertLaunchedFrom(t, shaC)
	assertPin(t, e.scope, shaC, "", false)
}

// TestStatusSierAtScopetErRulletTilbake: status skal ikke mase om en
// oppdatering ingenting kommer til å utføre, og skal si hvorfor.
func TestStatusSierAtScopetErRulletTilbake(t *testing.T) {
	scope := toRevisjoner(t)
	utenNett(t)
	if _, err := rollbackKjør(t, false); err != nil {
		t.Fatalf("rollback = %v", err)
	}

	stubRelease(t, releaseCandidate, pakkeRelease{Version: "0.5.0", SHA: shaB, Tag: "v0.5.0"}, nil)
	state, _ := readScopedState(scope)
	st := pakkeStatus(scope, state)
	if st == nil {
		t.Fatal("pakkeStatus = nil for en pinnet pakke")
	}
	if st.PendingRelease != nil {
		t.Errorf("status tilbyr den forkastede revisjonen: %+v", st.PendingRelease)
	}
	if !sameSHA(st.RolledBackFrom, shaB) {
		t.Errorf("rolled_back_from = %q, vil ha %s", st.RolledBackFrom, shaB)
	}
	out := captureStdoutFor(t, func() { printPakkeStatus(st) })
	if !strings.Contains(out, "Rolled back") {
		t.Errorf("status sa ikke fra om rollbacken. Utskrift:\n%s", out)
	}
}

// TestRulletTilbakeMedRevisjonenBorteMelderIkkeSuksess: er den pinnede
// revisjonen borte fra disk, og det eneste kilden tilbyr er den forkastede,
// finnes det ingenting sync kan bygge opp igjen uten å få beskjed om hva.
func TestRulletTilbakeMedRevisjonenBorteMelderIkkeSuksess(t *testing.T) {
	scope := toRevisjoner(t)
	utenNett(t)
	if _, err := rollbackKjør(t, false); err != nil {
		t.Fatalf("rollback = %v", err)
	}
	if err := os.RemoveAll(pakkeRevisionDir("navikt/grillmester", shaA)); err != nil {
		t.Fatal(err)
	}

	releaseSyncSource(t, shaB)
	stubRelease(t, releaseCandidate, pakkeRelease{Version: "0.5.0", SHA: shaB, Tag: "v0.5.0"}, nil)

	var err error
	out := captureStdoutFor(t, func() { err = cmdSync(scope, "", "", true, false) })
	if err == nil {
		t.Fatalf("sync --apply meldte suksess over en pinne uten tre. Utskrift:\n%s", out)
	}
	state, _ := readScopedState(scope)
	if state == nil || !sameSHA(state.SourceSHA, shaA) {
		t.Errorf("pinnen flyttet seg til den forkastede revisjonen: %+v", state)
	}
}
