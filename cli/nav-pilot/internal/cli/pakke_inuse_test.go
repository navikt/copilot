package cli

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

// #784: pruningen beholdt de to navngitte revisjonene og ingenting mer, så en
// økt som levde gjennom to oppdateringer fikk katalogen sin slettet under seg.
// Testene her kjører den rapporterte sekvensen gjennom autoPin, som er den ene
// pruningsstien en test kan kjøre uten nett.

// lokalPakkeUnderTest legger en minimal, kontraktsmessig Tier 1-pakke i et
// arbeidstre. En absolutt sti pinnes aldri, så autoPin materialiserer og pruner
// på hver launch, som er nøyaktig sekvensen under test.
func lokalPakkeUnderTest(t *testing.T) *Source {
	t.Helper()
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

// launchAt kjører én launch på en revisjon og aldrer katalogen bakover, slik at
// previousRevision avgjøres av rekkefølgen launchene skjedde i og ikke av at to
// materialiseringer landet i samme mtime-tikk.
func launchAt(t *testing.T, src *Source, sha string, age time.Duration) {
	t.Helper()
	src.SHA = sha
	if _, err := autoPin(src, "copilot"); err != nil {
		t.Fatalf("autoPin på %s: %v", sha, err)
	}
	when := time.Now().Add(-age)
	if err := os.Chtimes(pakkeRevisionDir(src.Repo, sha), when, when); err != nil {
		t.Fatal(err)
	}
}

// TestPruneKeepsTheRevisionAnOpenSessionHolds er rapporten i #784: en økt på A,
// så en oppdatering til B og en til C. Uten markøren beholder pruningen C og B,
// og A — treet økta har oppe som OPENCODE_CONFIG_DIR og som cplt-ens
// --allow-read — blir borte midt i økta.
func TestPruneKeepsTheRevisionAnOpenSessionHolds(t *testing.T) {
	isolatedConfig(t)
	src := lokalPakkeUnderTest(t)

	launchAt(t, src, "a", 2*time.Hour)
	// Økta lever fra her: markøren bærer denne prosessens pid og starttid.
	release := holdRevision(src.Repo, "a")

	launchAt(t, src, "b", time.Hour)
	launchAt(t, src, "c", 0)

	got := revisionNames(t, src.Repo)
	if want := []string{"a", "b", "c"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("revisjoner etter to oppdateringer = %v, vil ha %v: en levende økt leser a", got, want)
	}

	// Og når økta slutter, er a ikke lenger noens: neste pruning tar den.
	release()
	prunePakkeRevisions(src.Repo, "c", "b")
	if got, want := revisionNames(t, src.Repo), []string{"b", "c"}; !reflect.DeepEqual(got, want) {
		t.Errorf("revisjoner etter at økta tok slutt = %v, vil ha %v", got, want)
	}
}

// Den andre halvdelen: uten noe som kjører skal pruningen fortsatt hente inn
// plassen. Ellers er #784 «løst» ved å slutte å slette.
func TestPruneReclaimsWhenNoSessionHoldsAnything(t *testing.T) {
	isolatedConfig(t)
	src := lokalPakkeUnderTest(t)

	launchAt(t, src, "a", 2*time.Hour)
	launchAt(t, src, "b", time.Hour)
	launchAt(t, src, "c", 0)

	if got, want := revisionNames(t, src.Repo), []string{"b", "c"}; !reflect.DeepEqual(got, want) {
		t.Errorf("revisjoner = %v, vil ha %v: uten en levende økt gjelder oppbevaringsregelen på to", got, want)
	}
}

// dødPid gir et prosessnummer som helt sikkert ikke er i live: en prosess som
// har kjørt ferdig og er ryddet opp.
func dødPid(t *testing.T) int {
	t.Helper()
	cmd := exec.Command("sleep", "0")
	if err := cmd.Run(); err != nil {
		t.Fatalf("kunne ikke kjøre sleep: %v", err)
	}
	return cmd.Process.Pid
}

func skrivMarkør(t *testing.T, repo string, h revisionHold) string {
	t.Helper()
	dir := filepath.Join(pakkeSourceDir(repo), revisionHoldDir)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(h)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "markor.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// En økt som dør uten å rydde etter seg, skal ikke pinne treet sitt for alltid.
// Markøren er bevis om nåtiden — pid-en og kjernens starttid for den — og en
// markør som ikke lenger navngir en levende prosess, holder ingenting.
func TestPruneDropsACrashedSessionsMarker(t *testing.T) {
	isolatedConfig(t)
	repo := t.TempDir()
	for _, sha := range []string{"a", "b"} {
		if err := os.MkdirAll(pakkeRevisionDir(repo, sha), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	// Bogus starttid i tillegg til den døde pid-en: blir nummeret gjenbrukt
	// mellom kjøringene, er markøren fortsatt ikke om den prosessen.
	path := skrivMarkør(t, repo, revisionHold{SHA: "a", PID: dødPid(t), Lstart: "Thu Jan  1 00:00:00 1970"})

	prunePakkeRevisions(repo, "b")

	if got, want := revisionNames(t, repo), []string{"b"}; !reflect.DeepEqual(got, want) {
		t.Errorf("revisjoner = %v, vil ha %v: en krasjet økt holder ingenting", got, want)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("markøren etter den krasjede økta ligger igjen på %s (stat-feil %v)", path, err)
	}
}

// Samme sak fra den andre kanten: pid-en er i live, men det er ikke prosessen
// markøren ble skrevet om. Det er et gjenbrukt prosessnummer, og det er grunnen
// til at markøren bærer starttid og ikke bare et tall.
func TestPruneDropsAMarkerWhosePIDWasRecycled(t *testing.T) {
	isolatedConfig(t)
	repo := t.TempDir()
	if err := os.MkdirAll(pakkeRevisionDir(repo, "a"), 0o700); err != nil {
		t.Fatal(err)
	}
	path := skrivMarkør(t, repo, revisionHold{SHA: "a", PID: os.Getpid(), Lstart: "en annen prosess"})

	prunePakkeRevisions(repo)

	if got := revisionNames(t, repo); len(got) != 0 {
		t.Errorf("revisjoner = %v, vil ha ingen", got)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("markøren med gjenbrukt pid ligger igjen på %s (stat-feil %v)", path, err)
	}
}
