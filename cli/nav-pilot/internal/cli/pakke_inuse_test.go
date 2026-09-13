package cli

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"syscall"
	"testing"
	"time"
)

// #784: pruningen beholdt de to navngitte revisjonene og ingenting mer, så en
// økt som levde gjennom to oppdateringer fikk katalogen sin slettet under seg.
//
// Markøren er en fil en levende økt holder en flock på, så en test kan spille
// begge roller uten å starte prosesser: testprosessen selv tar en markør, og en
// markørfil uten lås er nøyaktig det en krasjet økt etterlater seg.

func heldNames(t *testing.T, repo string) []string {
	t.Helper()
	names, known := heldRevisions(repo)
	if !known {
		t.Fatalf("heldRevisions(%s) kunne ikke avgjøres", repo)
	}
	return names
}

// krasjetØkt skriver markøren til en økt som ikke finnes lenger: fila ligger
// der, men ingen holder låsen — som etter et kill -9 eller en omstart.
func krasjetØkt(t *testing.T, repo, revision string) string {
	t.Helper()
	path := filepath.Join(markørDir(t, repo), "krasjet"+holdSuffix)
	if err := os.WriteFile(path, []byte(revision), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// annenØkt er en økt i en annen prosess som fortsatt lever: markøren er låst,
// og flock henger på fildeskriptoren, så låsen utelukker også denne prosessen.
func annenØkt(t *testing.T, repo, revision string) {
	t.Helper()
	path := filepath.Join(markørDir(t, repo), "annen"+holdSuffix)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		t.Fatalf("kunne ikke låse markøren til den andre økta: %v", err)
	}
	if _, err := f.WriteString(revision); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		f.Close()
		os.Remove(path)
	})
}

func markørDir(t *testing.T, repo string) string {
	t.Helper()
	dir := filepath.Join(pakkeSourceDir(repo), revisionHoldDir)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	return dir
}

// lokalPakkeUnderTest legger en minimal, kontraktsmessig Tier 1-pakke i et
// arbeidstre. En absolutt sti pinnes aldri, så autoPin materialiserer og pruner
// på hver launch, som er nøyaktig sekvensen under test.
func lokalPakkeUnderTest(t *testing.T) *Source {
	t.Helper()
	t.Cleanup(releaseRevision)
	work := t.TempDir()
	manifest := `{"contractVersion":"1","name":"lokalpakke","description":"l",` +
		`"layout":{"agents":"agents","skills":"skills"},` +
		`"clients":{"copilot":{"primaryAgents":["en"]}}}`
	mustWrite(t, filepath.Join(work, ".nav-pilot", "agentpakke.json"), manifest)
	mustWrite(t, filepath.Join(work, "agents", "en.agent.md"), "---\nname: en\ndescription: x\n---\nx\n")
	mustWrite(t, filepath.Join(work, "skills", ".keep"), "")

	src := &Source{Dir: work, Repo: work, SHA: "a"}
	if err := attachPakke(src); err != nil {
		t.Fatal(err)
	}
	return src
}

// launchAt kjører én launch og aldrer katalogen bakover, slik at
// previousRevision avgjøres av rekkefølgen launchene skjedde i og ikke av at to
// materialiseringer landet i samme mtime-tikk. Den returnerer katalognavnet,
// som er den eneste identiteten en lokal revisjon har.
func launchAt(t *testing.T, src *Source, sha string, age time.Duration) string {
	t.Helper()
	src.SHA = sha
	rev, err := autoPin(src, "copilot")
	if err != nil {
		t.Fatalf("autoPin på %s: %v", sha, err)
	}
	when := time.Now().Add(-age)
	if err := os.Chtimes(rev.Dir, when, when); err != nil {
		t.Fatal(err)
	}
	return filepath.Base(rev.Dir)
}

// TestPruneKeepsTheRevisionAnOpenSessionHolds er rapporten i #784: en økt på A,
// så en oppdatering til B og en til C. Uten markøren beholder pruningen C og B,
// og A — treet økta har oppe som OPENCODE_CONFIG_DIR og som cplt-ens
// --allow-read — blir borte midt i økta.
func TestPruneKeepsTheRevisionAnOpenSessionHolds(t *testing.T) {
	isolatedConfig(t)
	src := lokalPakkeUnderTest(t)

	a := launchAt(t, src, "a", 2*time.Hour)
	// Økta på a lever videre i sin egen prosess mens de to neste launchene
	// skjer her.
	annenØkt(t, src.Repo, a)

	b := launchAt(t, src, "b", time.Hour)
	c := launchAt(t, src, "c", 0)

	want := []string{a, b, c}
	sort.Strings(want)
	if got := revisionNames(t, src.Repo); !reflect.DeepEqual(got, want) {
		t.Fatalf("revisjoner etter to oppdateringer = %v, vil ha %v: en levende økt leser %s", got, want, a)
	}
}

// Den andre halvdelen: uten noe som kjører skal pruningen fortsatt hente inn
// plassen. Ellers er #784 «løst» ved å slutte å slette.
func TestPruneReclaimsWhenNoSessionHoldsAnything(t *testing.T) {
	isolatedConfig(t)
	src := lokalPakkeUnderTest(t)

	launchAt(t, src, "a", 2*time.Hour)
	b := launchAt(t, src, "b", time.Hour)
	c := launchAt(t, src, "c", 0)

	want := []string{b, c}
	sort.Strings(want)
	if got := revisionNames(t, src.Repo); !reflect.DeepEqual(got, want) {
		t.Errorf("revisjoner = %v, vil ha %v: uten en levende økt gjelder oppbevaringsregelen på to", got, want)
	}
}

// En økt som dør uten å rydde, skal ikke pinne treet sitt for alltid. Kjernen
// slipper flock-en når prosessen dør, så en markørfil pruningen selv får låse,
// er per definisjon en markør uten økt bak seg.
func TestPruneDropsACrashedSessionsMarker(t *testing.T) {
	isolatedConfig(t)
	repo := t.TempDir()
	for _, sha := range []string{"a", "b"} {
		if err := os.MkdirAll(pakkeRevisionDir(repo, sha), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	path := krasjetØkt(t, repo, "a")

	prunePakkeRevisions(repo, "b")

	if got, want := revisionNames(t, repo), []string{"b"}; !reflect.DeepEqual(got, want) {
		t.Errorf("revisjoner = %v, vil ha %v: en krasjet økt holder ingenting", got, want)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("markøren etter den krasjede økta ligger igjen på %s (stat-feil %v)", path, err)
	}
}

// ─── funn 1: scan og sletting må være atomisk mot publisering ────────────────

// Låsen er hele beviset: uten den kan en launch publisere markøren sin mellom
// pruningens scan og dens RemoveAll, og miste treet den nettopp valgte. Testen
// prøver å ta låsen ikke-blokkerende nøyaktig i det vinduet. Får den den, er
// vinduet åpent.
func TestPruneHoldsTheSourceLockAcrossScanAndDelete(t *testing.T) {
	isolatedConfig(t)
	repo := t.TempDir()
	if err := os.MkdirAll(pakkeRevisionDir(repo, "a"), 0o700); err != nil {
		t.Fatal(err)
	}

	var slippInn bool
	pruneScannedHook = func() { slippInn = kanLåseKilden(t, repo) }
	t.Cleanup(func() { pruneScannedHook = nil })

	prunePakkeRevisions(repo, "a")

	if slippInn {
		t.Error("kilden kunne låses mellom pruningens scan og slettingen: en markør publisert der blir ikke sett, og treet blir slettet likevel")
	}
}

// Og primitivet begge sider bruker: to uavhengige åpninger av samme katalog
// utelukker hverandre. flock henger på fildeskriptoren, ikke på prosessen, så
// dette gjelder også to nav-pilot-prosesser.
func TestSourceLockExcludes(t *testing.T) {
	isolatedConfig(t)
	repo := t.TempDir()
	if err := os.MkdirAll(pakkeSourceDir(repo), 0o700); err != nil {
		t.Fatal(err)
	}
	unlock, ok := lockSource(repo)
	if !ok {
		t.Fatal("lockSource fikk ikke låsen på en katalog ingen andre rører")
	}
	if kanLåseKilden(t, repo) {
		t.Error("kilden lot seg låse to ganger samtidig: låsen utelukker ingenting")
	}
	unlock()
	if !kanLåseKilden(t, repo) {
		t.Error("kilden lot seg ikke låse etter at låsen ble sluppet")
	}
}

// kanLåseKilden prøver låsen ikke-blokkerende og slipper den igjen.
func kanLåseKilden(t *testing.T, repo string) bool {
	t.Helper()
	f, err := os.Open(pakkeSourceDir(repo))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return false
	}
	_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
	return true
}

// ─── funn 2: kravet tas før pruningsvinduet, ikke etter ──────────────────────

// autoPin pruner selv, og andre prosesser pruner samtidig. Blir kravet tatt
// først etter at autoPin har returnert, er revisjonen ubeskyttet gjennom
// nøyaktig den pruningen.
func TestAutoPinHoldsTheRevisionBeforeItPrunes(t *testing.T) {
	isolatedConfig(t)
	src := lokalPakkeUnderTest(t)

	var seenUnderPrune []string
	pruneScannedHook = func() { seenUnderPrune = heldNames(t, src.Repo) }
	t.Cleanup(func() { pruneScannedHook = nil })

	rev, err := autoPin(src, "copilot")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Base(rev.Dir)
	if !reflect.DeepEqual(seenUnderPrune, []string{want}) {
		t.Errorf("holdt mens autoPin pruner = %v, vil ha [%s]: kravet tas etter pruningsvinduet", seenUnderPrune, want)
	}
}

// Samme regel på pin-stien: revisjonen er krevd før noe annet enn oppslaget av
// den har skjedd — før manifestet lastes, før release-spørsmålet, før hash-
// vandringen.
func TestPinnedRevisionHoldsWhatItReturns(t *testing.T) {
	scope := pinEnv(t)
	t.Cleanup(releaseRevision)
	src := tier2PinSource(t, "sha-en")
	installPin(t, scope, src)

	rev, err := pinnedRevision(src.Repo)
	if err != nil || rev == nil {
		t.Fatalf("pinnedRevision = %v, %v", rev, err)
	}
	if got, want := heldNames(t, src.Repo), []string{filepath.Base(rev.Dir)}; !reflect.DeepEqual(got, want) {
		t.Errorf("holdt etter pinnedRevision = %v, vil ha %v", got, want)
	}
}

// ─── funn 3: en lokal kilde sin identitet er katalogen, ikke SHA-en ──────────

// En lokal kilde stages på nytt ved hver launch. Publiseres begge trærne på
// samme navn, flytter materializeRevision det gamle til side og sletter det, og
// en markør som navngir SHA-en beskytter da et annet tre enn det økta leser.
func TestLocalLaunchesAtTheSameSHAAreDifferentTrees(t *testing.T) {
	isolatedConfig(t)
	src := lokalPakkeUnderTest(t)
	src.SHA = "unknown" // en ikke-git-katalog: alle launcher deler denne «SHA-en»

	first, err := autoPin(src, "copilot")
	if err != nil {
		t.Fatal(err)
	}
	manifest := filepath.Join(first.Dir, ".nav-pilot", "agentpakke.json")
	if _, err := os.Stat(manifest); err != nil {
		t.Fatalf("første launch materialiserte ikke et helt tre: %v", err)
	}

	second, err := autoPin(src, "copilot")
	if err != nil {
		t.Fatal(err)
	}
	if first.Dir == second.Dir {
		t.Fatalf("to launcher på samme SHA delte katalog %s: da er katalognavnet ingen identitet, og en markør på den navngir feil tre", first.Dir)
	}
	if _, err := os.Stat(manifest); err != nil {
		t.Errorf("treet den første økta leser er borte etter neste launch: %v", err)
	}
}

// ─── funn 4: ukjent holdsett må stoppe slettingen ────────────────────────────

// Kan ikke .i-bruk leses, er «ingen markører» og «ingen økter» det samme synet
// og motsatte ting. Da slettes ingenting.
func TestPruneDeletesNothingWhenTheHoldSetIsUnknown(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root leser en katalog uten lesebit likevel")
	}
	isolatedConfig(t)
	repo := t.TempDir()
	if err := os.MkdirAll(pakkeRevisionDir(repo, "a"), 0o700); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(pakkeSourceDir(repo), revisionHoldDir)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })

	prunePakkeRevisions(repo, "b")

	if got, want := revisionNames(t, repo), []string{"a"}; !reflect.DeepEqual(got, want) {
		t.Errorf("revisjoner = %v, vil ha %v: et uleselig holdsett er ikke det samme som ingen økter", got, want)
	}
}

// ─── funn 5: beviset er ikke avhengig av at `ps` finnes ──────────────────────

// Kravet lå før i en pid pluss kjernens starttid for den, lest med `ps`. På en
// maskin uten brukbar `ps` ble en gyldig markør lest som ubevist og slettet —
// altså feil vei for en sletting. Låsen kjernen holder, trenger ingen `ps`.
func TestAHoldSurvivesOnAMachineWithoutPs(t *testing.T) {
	isolatedConfig(t)
	repo := t.TempDir()
	t.Cleanup(releaseRevision)
	if err := os.MkdirAll(pakkeRevisionDir(repo, "a"), 0o700); err != nil {
		t.Fatal(err)
	}
	holdRevision(repo, "a")

	t.Setenv("PATH", t.TempDir()) // ingen ps, ingen noe

	prunePakkeRevisions(repo, "b")

	if got, want := revisionNames(t, repo), []string{"a"}; !reflect.DeepEqual(got, want) {
		t.Errorf("revisjoner = %v, vil ha %v: kravet skal ikke avhenge av at en binærfil finnes", got, want)
	}
}
