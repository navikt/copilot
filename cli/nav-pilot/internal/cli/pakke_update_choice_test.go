package cli

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// #781: brukeren velger én gang hva som skjer når pakka publiserer en ny stabil
// release — automatisk, spør først, eller behold revisjonen. Testene under
// dekker de fire påstandene: valget står i staten og overlever at pinnen
// flyttes, «behold» stopper både oppslaget og spørsmålet uten å skjule
// releasen, valget og en rollback svarer med én og samme regel, og ingenting av
// dette muterer noe uten terminal.

// settValg skriver det varige valget rett i staten, slik en tidligere økt ville
// gjort det.
func settValg(t *testing.T, scope *InstallScope, c updateChoice) {
	t.Helper()
	state, err := readScopedState(scope)
	if err != nil || state == nil {
		t.Fatalf("readScopedState = %+v, %v", state, err)
	}
	state.UpdateChoice = string(c)
	if err := writeScopedState(scope, state); err != nil {
		t.Fatal(err)
	}
}

// flyttPinnen er en annen prosess som flytter pinnen: en eksplisitt pinning av
// sha, uten release-oppslag.
func flyttPinnen(t *testing.T, scope *InstallScope, sha string) {
	t.Helper()
	if _, err := pinRevision(scope, tier2PinSource(t, sha), nil, true, true); err != nil {
		t.Fatalf("pinRevision(%s) = %v", shortSHA(sha), err)
	}
}

func lesValg(t *testing.T, scope *InstallScope) updateChoice {
	t.Helper()
	state, _ := readScopedState(scope)
	return pakkeUpdateChoice(state)
}

// TestValgetBeholdRevisjonenSpørIkkeOgSlårIkkeOpp: «behold revisjonen» er et
// svar om alle framtidige releases, så oppstarten har ingenting å slå opp og
// ingenting å spørre om. En launch under valget rører ikke nettet.
func TestValgetBeholdRevisjonenSpørIkkeOgSlårIkkeOpp(t *testing.T) {
	e := newPromptEnv(t)
	settValg(t, e.scope, updateKeep)
	utenNett(t) // etter pinEnv/newPromptEnv: ethvert oppslag feiler testen nå

	e.launch(t)
	if len(e.asked) != 0 {
		t.Errorf("oppstarten spurte likevel: %v", e.asked)
	}
	e.assertLaunchedFrom(t, shaC)
	assertPin(t, e.scope, shaC, "", false)
}

// TestValgetAutomatiskOppdatererUtenÅSpørre er selve automatikken: en ny stabil
// release tas i bruk ved oppstart, uten spørsmål, og klienten starter fra den
// nye revisjonen.
func TestValgetAutomatiskOppdatererUtenÅSpørre(t *testing.T) {
	e := newPromptEnv(t)
	settValg(t, e.scope, updateAuto)
	stubRelease(t, releaseCandidate, release041, nil)

	e.launch(t)
	if len(e.asked) != 0 {
		t.Errorf("automatisk oppdatering spurte: %v", e.asked)
	}
	assertPin(t, e.scope, shaA, "0.4.1", true)
	e.assertLaunchedFrom(t, shaA)
	if got := lesValg(t, e.scope); got != updateAuto {
		t.Errorf("valget etter oppdateringen = %q, vil ha %q", got, updateAuto)
	}
}

// TestValgetAutomatiskTarIkkeOvergangen: overgangsspørsmålet (#782) er den ene
// kandidaten som kan være en nedgradering og et nytt abonnement på én gang.
// «Automatisk» betyr nyeste release, ikke samtykke til å gå bakover.
func TestValgetAutomatiskTarIkkeOvergangen(t *testing.T) {
	e := newPromptEnv(t)
	settValg(t, e.scope, updateAuto)
	stubRelease(t, releaseNotOffered, release041, nil)

	e.launch(t)
	if len(e.asked) != 1 {
		t.Fatalf("overgangen ble ikke spurt om: %v", e.asked)
	}
	if !strings.Contains(e.affirms[0], "follow stable releases") {
		t.Errorf("spørsmålet tilbød %q som første svar", e.affirms[0])
	}
}

// TestSvaretBeholdRevisjonenHuskes: svaret ved oppstart er der valget faktisk
// tas. Neste oppstart spør ikke igjen, heller ikke om en nyere versjon.
func TestSvaretBeholdRevisjonenHuskes(t *testing.T) {
	e := newPromptEnv(t)
	keep := answerKeep
	e.pick = &keep
	stubRelease(t, releaseCandidate, release041, nil)

	e.launch(t)
	if len(e.asked) != 1 {
		t.Fatalf("spørsmålene = %v, vil ha ett", e.asked)
	}
	if got := lesValg(t, e.scope); got != updateKeep {
		t.Fatalf("valget = %q, vil ha %q", got, updateKeep)
	}
	assertPin(t, e.scope, shaC, "", false)

	// En nyere versjon spørres det ikke om heller: valget gjaldt alle.
	stubRelease(t, releaseCandidate, release042, nil)
	ageReleaseCache(t, pakkeReleaseCheckInterval+time.Hour)
	e.asked = nil
	e.launch(t)
	if len(e.asked) != 0 {
		t.Errorf("oppstarten spurte om en nyere versjon likevel: %v", e.asked)
	}
	e.assertLaunchedFrom(t, shaC)
}

// TestSvaretAlltidOppdatererOgHuskes: «alltid» svarer på denne releasen og på
// alle etterpå i samme svar.
func TestSvaretAlltidOppdatererOgHuskes(t *testing.T) {
	e := newPromptEnv(t)
	auto := answerAuto
	e.pick = &auto
	stubRelease(t, releaseCandidate, release041, nil)

	e.launch(t)
	assertPin(t, e.scope, shaA, "0.4.1", true)
	if got := lesValg(t, e.scope); got != updateAuto {
		t.Errorf("valget = %q, vil ha %q", got, updateAuto)
	}
}

// TestValgetOverleverAtPinnenFlyttes: valget er om pakka, ikke om revisjonen
// det ble tatt på. Ellers ville hver oppdatering glemt det, og «automatisk»
// vart i nøyaktig én release.
func TestValgetOverleverAtPinnenFlyttes(t *testing.T) {
	scope := pinEnv(t)
	installPin(t, scope, tier2PinSource(t, shaC))
	settValg(t, scope, updateAuto)

	releaseSyncSource(t, shaB)
	stubRelease(t, releaseCandidate, release041, nil)
	var err error
	out := captureStdoutFor(t, func() { err = cmdSync(scope, "", "", true, false) })
	if err != nil {
		t.Fatalf("sync --apply = %v. Utskrift:\n%s", err, out)
	}
	assertPin(t, scope, shaA, "0.4.1", true)
	if got := lesValg(t, scope); got != updateAuto {
		t.Errorf("valget etter sync --apply = %q, vil ha %q", got, updateAuto)
	}

	// Og over en eksplisitt --ref, som er et valg om én revisjon og ikke et
	// ombestemt svar om den neste releasen.
	out = captureStdoutFor(t, func() { err = cmdSync(scope, shaB, "", true, false) })
	if err != nil {
		t.Fatalf("sync --apply --ref = %v. Utskrift:\n%s", err, out)
	}
	if got := lesValg(t, scope); got != updateAuto {
		t.Errorf("valget etter --ref = %q, vil ha %q", got, updateAuto)
	}
}

// TestValgetOgRollbackenErÉnRegel er det skarpe punktet: `rolled_back_from`
// betyr «ikke denne revisjonen», og «behold revisjonen» betyr «ingen nyere
// revisjon». Et scope som bærer begge må få ett svar, og riktig begrunnelse for
// hver kandidat — ikke to sjekker som kan si hver sin ting.
func TestValgetOgRollbackenErÉnRegel(t *testing.T) {
	scope := toRevisjoner(t) // pinnet på shaB, shaA ligger under
	utenNett(t)
	if _, err := rollbackKjør(t, false); err != nil {
		t.Fatalf("rollback = %v", err)
	}
	settValg(t, scope, updateKeep)

	// Kandidaten er nøyaktig revisjonen rollbacken forlot.
	refs := releaseSyncSource(t, shaB)
	stubRelease(t, releaseCandidate, pakkeRelease{Version: "0.5.0", SHA: shaB, Tag: "v0.5.0"}, nil)
	var err error
	out := captureStdoutFor(t, func() { err = cmdSync(scope, "", "", true, false) })
	if err != nil {
		t.Fatalf("sync --apply = %v. Utskrift:\n%s", err, out)
	}
	if !strings.Contains(out, "rolled back") {
		t.Errorf("sync begrunnet ikke med rollbacken. Utskrift:\n%s", out)
	}
	assertPin(t, scope, shaA, "", false)

	// Og en kandidat scopet aldri har forkastet: samme utfall, annen grunn.
	stubRelease(t, releaseCandidate, pakkeRelease{Version: "0.6.0", SHA: shaC, Tag: "v0.6.0"}, nil)
	out = captureStdoutFor(t, func() { err = cmdSync(scope, "", "", true, false) })
	if err != nil {
		t.Fatalf("sync --apply = %v. Utskrift:\n%s", err, out)
	}
	if !strings.Contains(out, "keeps the revision") {
		t.Errorf("sync begrunnet ikke med valget. Utskrift:\n%s", out)
	}
	if strings.Contains(out, "rolled back") {
		t.Errorf("sync begrunnet en ny kandidat med rollbacken. Utskrift:\n%s", out)
	}
	assertPin(t, scope, shaA, "", false)
	if len(*refs) == 0 {
		t.Error("sync hentet aldri kandidaten den holdt tilbake")
	}

	// Veien videre står fortsatt åpen, og den flytter ikke valget.
	out = captureStdoutFor(t, func() { err = cmdSync(scope, shaC, "", true, false) })
	if err != nil {
		t.Fatalf("sync --apply --ref = %v. Utskrift:\n%s", err, out)
	}
	state, _ := readScopedState(scope)
	if state == nil || !sameSHA(state.SourceSHA, shaC) {
		t.Fatalf("--ref flyttet ikke pinnen: %+v", state)
	}
	if pakkeUpdateChoice(state) != updateKeep {
		t.Errorf("valget etter --ref = %q, vil ha %q", state.UpdateChoice, updateKeep)
	}
}

// TestBeholdRevisjonenSkjulerIkkeReleasen: valget stopper flyttingen, ikke
// nyheten. Kommer det en sikkerhetsrettelse, skal både sync og status navngi
// den og si hva som skal til for å ta den.
func TestBeholdRevisjonenSkjulerIkkeReleasen(t *testing.T) {
	scope := pinEnv(t)
	installPin(t, scope, tier2PinSource(t, shaC))
	settValg(t, scope, updateKeep)
	releaseSyncSource(t, shaB)
	stubRelease(t, releaseCandidate, release041, nil)

	var err error
	out := captureStdoutFor(t, func() { err = cmdSync(scope, "", "", false, true) })
	if err != nil {
		t.Fatalf("sync --json = %v. Utskrift:\n%s", err, out)
	}
	var doc syncResult
	if jerr := json.Unmarshal([]byte(out), &doc); jerr != nil {
		t.Fatalf("utskriften er ikke ett JSON-dokument: %v\n%s", jerr, out)
	}
	if doc.UpdateChoice != string(updateKeep) {
		t.Errorf("update_choice = %q, vil ha %q", doc.UpdateChoice, updateKeep)
	}
	if !strings.Contains(doc.Warning, "0.4.1") {
		t.Errorf("warning navngir ikke releasen som holdes tilbake: %q", doc.Warning)
	}

	state, _ := readScopedState(scope)
	st := pakkeStatus(scope, state)
	if st == nil || st.PendingRelease == nil {
		t.Fatalf("status skjulte releasen: %+v", st)
	}
	if st.UpdateChoice != string(updateKeep) {
		t.Errorf("status update_choice = %q, vil ha %q", st.UpdateChoice, updateKeep)
	}
	statusUt := captureStdoutFor(t, func() { printPakkeStatus(st) })
	if !strings.Contains(statusUt, "keeps the revision") || !strings.Contains(statusUt, "--ref "+shaA) {
		t.Errorf("status sa ikke hva som skal til for å ta releasen. Utskrift:\n%s", statusUt)
	}
}

// TestIkkeInteraktivOppstartSpørIkkeOgOppdatererIkke: et planlagt sync eller en
// CI-kjøring kan ikke svare på et spørsmål. «Spør først» rapporterer da, og
// «automatisk» muterer ingenting uten terminal.
func TestIkkeInteraktivOppstartSpørIkkeOgOppdatererIkke(t *testing.T) {
	for _, valg := range []updateChoice{updateAsk, updateAuto} {
		t.Run(string(valg), func(t *testing.T) {
			e := newPromptEnv(t)
			settValg(t, e.scope, valg)
			forceNonInteractive = true
			t.Cleanup(func() { forceNonInteractive = false })
			calls := stubRelease(t, releaseCandidate, release041, nil)

			e.launch(t)
			if *calls != 0 || len(e.asked) != 0 {
				t.Errorf("oppstart uten terminal slo opp %d gang(er) og spurte %v", *calls, e.asked)
			}
			assertPin(t, e.scope, shaC, "", false)
			e.assertLaunchedFrom(t, shaC)

			// Og et sync i samme skall rapporterer oppdateringen framfor å
			// spørre om den: exit 1, pinnen urørt.
			releaseSyncSource(t, shaB)
			var err error
			out := captureStdoutFor(t, func() { err = cmdSync(e.scope, "", "", false, false) })
			if err == nil {
				t.Fatalf("sync meldte ingenting å gjøre. Utskrift:\n%s", out)
			}
			if len(e.asked) != 0 {
				t.Errorf("sync spurte: %v", e.asked)
			}
			assertPin(t, e.scope, shaC, "", false)
		})
	}
}

// TestUpdatesSetterValgetOgNekterTull: flagget er veien tilbake for et scope som
// har valgt «behold» — der spør ingenting lenger — så det må virke, og et
// mistastet modusnavn må ikke lese som en modus som gjør mindre.
func TestUpdatesSetterValgetOgNekterTull(t *testing.T) {
	scope := pinEnv(t)
	installPin(t, scope, tier2PinSource(t, shaC))

	out := captureStdoutFor(t, func() {
		if err := cmdUpdateChoice("keep", false); err != nil {
			t.Fatalf("--updates keep = %v", err)
		}
	})
	if !strings.Contains(out, shortSHA(shaC)) {
		t.Errorf("bekreftelsen navnga ikke revisjonen. Utskrift:\n%s", out)
	}
	if got := lesValg(t, scope); got != updateKeep {
		t.Fatalf("valget = %q, vil ha %q", got, updateKeep)
	}

	err := cmdUpdateChoice("aldri", false)
	if err == nil {
		t.Fatal("--updates aldri ble godtatt")
	}
	if !strings.Contains(err.Error(), "auto, ask, keep") {
		t.Errorf("feilen lister ikke modusene: %v", err)
	}
	if got := lesValg(t, scope); got != updateKeep {
		t.Errorf("et ugyldig valg endret staten til %q", got)
	}

	captureStdoutFor(t, func() {
		if err := cmdUpdateChoice("ask", false); err != nil {
			t.Fatalf("--updates ask = %v", err)
		}
	})
	if got := lesValg(t, scope); got != updateAsk {
		t.Errorf("valget = %q, vil ha %q", got, updateAsk)
	}
}

// TestUpdatesUtenPinneSierDet: brukerscopet trenger ikke ha en pinnet
// agentpakke, og da er det ingenting valget kan gjelde.
func TestUpdatesUtenPinneSierDet(t *testing.T) {
	pinEnv(t)
	err := cmdUpdateChoice("auto", false)
	if err == nil {
		t.Fatal("--updates uten pinne lyktes")
	}
	if !strings.Contains(err.Error(), "pins none") {
		t.Errorf("feilen sier ikke at scopet ikke pinner noe: %v", err)
	}
}

// TestValgetSkriverIkkeTilbakeEnPinneSomFlyttetSeg: valget skrives på staten som
// ligger der nå, ikke på det kommandoen leste før den skrev. Flytter en annen
// prosess pinnen i vinduet mellom de to, ville en skriving av det gamle
// øyeblikksbildet satt den forrige revisjonen tilbake i staten — og med den
// filoppføringene som hørte til.
func TestValgetSkriverIkkeTilbakeEnPinneSomFlyttetSeg(t *testing.T) {
	scope := pinEnv(t)
	installPin(t, scope, tier2PinSource(t, shaC))

	// Rekkefølgen drives eksplisitt: kroken kjører inni vinduet, én gang.
	orig := stateWriteHook
	t.Cleanup(func() { stateWriteHook = orig })
	stateWriteHook = func() {
		stateWriteHook = nil
		flyttPinnen(t, scope, shaA)
	}

	out := captureStdoutFor(t, func() {
		if err := cmdUpdateChoice("keep", false); err != nil {
			t.Fatalf("--updates keep = %v", err)
		}
	})
	state, _ := readScopedState(scope)
	if state == nil || !sameSHA(state.SourceSHA, shaA) {
		t.Fatalf("valget skrev tilbake pinnen som ble flyttet: %+v", state)
	}
	if pakkeUpdateChoice(state) != updateKeep {
		t.Errorf("valget = %q, vil ha %q", state.UpdateChoice, updateKeep)
	}
	if !strings.Contains(out, shortSHA(shaA)) {
		t.Errorf("bekreftelsen navnga en annen revisjon enn den som er pinnet. Utskrift:\n%s", out)
	}
}

// TestSvaretVedOppstartSkriverIkkeTilbakeEnPinneSomFlyttetSeg: svaret ved
// oppstart går gjennom den samme skrivingen, og vinduet er det bredeste av dem —
// et menneske ser på spørsmålet så lenge det vil.
func TestSvaretVedOppstartSkriverIkkeTilbakeEnPinneSomFlyttetSeg(t *testing.T) {
	e := newPromptEnv(t)
	keep := answerKeep
	e.pick = &keep
	stubRelease(t, releaseCandidate, release041, nil)

	orig := stateWriteHook
	t.Cleanup(func() { stateWriteHook = orig })
	stateWriteHook = func() {
		stateWriteHook = nil
		flyttPinnen(t, e.scope, shaB)
	}

	e.launch(t)
	state, _ := readScopedState(e.scope)
	if state == nil || !sameSHA(state.SourceSHA, shaB) {
		t.Fatalf("oppstartssvaret skrev tilbake pinnen som ble flyttet: %+v", state)
	}
	if pakkeUpdateChoice(state) != updateKeep {
		t.Errorf("valget = %q, vil ha %q", state.UpdateChoice, updateKeep)
	}
}

// TestPinnenBeholderEtValgTattMensRevisjonenBleMaterialisert er den andre
// retningen av det samme kappløpet: pinnen leser staten, bruker lang tid på å
// materialisere en revisjon, og skriver til slutt. Tas valget i mellomtiden, må
// pinnen bære det videre — `pinMoved` sammenligner det ikke, så et valg lest før
// materialiseringen ville blitt skrevet tilbake til det det var.
func TestPinnenBeholderEtValgTattMensRevisjonenBleMaterialisert(t *testing.T) {
	scope := pinEnv(t)
	installPin(t, scope, tier2PinSource(t, shaC))

	orig := pinWriteHook
	t.Cleanup(func() { pinWriteHook = orig })
	pinWriteHook = func() {
		pinWriteHook = nil
		if err := setUpdateChoice(scope, updateKeep); err != nil {
			t.Errorf("setUpdateChoice i vinduet = %v", err)
		}
	}

	releaseSyncSource(t, shaB)
	stubRelease(t, releaseCandidate, release041, nil)
	var err error
	out := captureStdoutFor(t, func() { err = cmdSync(scope, "", "", true, false) })
	if err != nil {
		t.Fatalf("sync --apply = %v. Utskrift:\n%s", err, out)
	}
	state, _ := readScopedState(scope)
	if state == nil || !sameSHA(state.SourceSHA, shaA) {
		t.Fatalf("pinnen flyttet seg ikke: %+v", state)
	}
	if pakkeUpdateChoice(state) != updateKeep {
		t.Errorf("pinnen skrev over valget som ble tatt underveis: %q", state.UpdateChoice)
	}
}

// TestUpdatesNekterEtAnnetScope: `cmdUpdateChoice` skriver alltid brukerscopet,
// så enhver annen måte å navngi et scope på er en forespørsel den ikke kan
// oppfylle. `--target` er den som ikke ble fanget av en sjekk på `--repo` alene:
// den byttet brukerens valg og synket deretter et annet sted.
func TestUpdatesNekterEtAnnetScope(t *testing.T) {
	scope := pinEnv(t)
	installPin(t, scope, tier2PinSource(t, shaC))
	settValg(t, scope, updateAsk)

	for _, args := range [][]string{
		{"sync", "--updates", "keep", "--target", t.TempDir()},
		{"sync", "--updates", "keep", "--repo"},
	} {
		err := run(args)
		if err == nil {
			t.Fatalf("%v lyktes", args)
		}
		if !strings.Contains(err.Error(), "user scope") {
			t.Errorf("%v ga feilen %v, som ikke navngir brukerscopet", args, err)
		}
		if got := lesValg(t, scope); got != updateAsk {
			t.Fatalf("%v endret valget til %q", args, got)
		}
	}
}

// TestBeholdRevisjonenHolderOgsåEnKildeUtenReleases: `releaseNoMetadata` kommer
// med en nil-feil og lar `release` være nil mens kilden er standardgrenen. En
// pinne som ikke følger releases synker nettopp derfra, så et «behold» lander
// her også — og linja skal navngi en revisjon, ikke kalle en grencommit en
// release.
func TestBeholdRevisjonenHolderOgsåEnKildeUtenReleases(t *testing.T) {
	scope := pinEnv(t)
	installPin(t, scope, tier2PinSource(t, shaC))
	settValg(t, scope, updateKeep)
	releaseSyncSource(t, shaB) // standardgrenen har flyttet seg til shaB
	stubRelease(t, releaseNoMetadata, pakkeRelease{}, nil)

	var err error
	out := captureStdoutFor(t, func() { err = cmdSync(scope, "", "", true, false) })
	if err != nil {
		t.Fatalf("sync --apply = %v. Utskrift:\n%s", err, out)
	}
	assertPin(t, scope, shaC, "", false)
	if !strings.Contains(out, "newest revision is "+shortSHA(shaB)) {
		t.Errorf("sync navnga ikke revisjonen som holdes tilbake. Utskrift:\n%s", out)
	}
	if strings.Contains(out, "release") {
		t.Errorf("sync kalte en grencommit en release. Utskrift:\n%s", out)
	}

	out = captureStdoutFor(t, func() { err = cmdSync(scope, "", "", false, true) })
	if err != nil {
		t.Fatalf("sync --json = %v. Utskrift:\n%s", err, out)
	}
	var doc syncResult
	if jerr := json.Unmarshal([]byte(out), &doc); jerr != nil {
		t.Fatalf("utskriften er ikke ett JSON-dokument: %v\n%s", jerr, out)
	}
	if doc.UpdateChoice != string(updateKeep) || !strings.Contains(doc.Warning, shortSHA(shaB)) {
		t.Errorf("dokumentet = %+v", doc)
	}
}
