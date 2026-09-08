package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/navikt/copilot/cli/nav-pilot/internal/agentpakke"
)

// writePakke lays down a minimal conforming agentpakke with the named agents.
func writePakke(t *testing.T, dir, name string, agents ...string) {
	t.Helper()
	for _, sub := range []string{".nav-pilot", "agents", "skills"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	manifest := `{"contractVersion":"1","name":"` + name + `","description":"` + name +
		`","layout":{"agents":"agents","skills":"skills"},"clients":{"copilot":{"primaryAgents":["` + agents[0] + `"]}}}`
	if err := os.WriteFile(filepath.Join(dir, ".nav-pilot", "agentpakke.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, a := range agents {
		body := "---\nname: " + a + "\ndescription: fra " + name + "\n---\n" + name + "\n"
		if err := os.WriteFile(filepath.Join(dir, "agents", a+".agent.md"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func declareReuse(t *testing.T, dir, base string) {
	t.Helper()
	body := `{"contractVersion":"1","source":"` + base + `"}`
	if err := os.WriteFile(filepath.Join(dir, ".nav-pilot", "agentpakke.lock.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func loadSource(t *testing.T, dir string) *Source {
	t.Helper()
	src := &Source{Dir: dir, Repo: dir}
	if err := attachPakke(src); err != nil {
		t.Fatal(err)
	}
	return src
}

// En pakke som gjenbruker en annen installerer begges innhold, og sitt eget
// vinner der navnene kolliderer (#572).
func TestComposedInstallInheritsAndShadows(t *testing.T) {
	baseDir, ownDir := t.TempDir(), t.TempDir()
	writePakke(t, baseDir, "basepakke", "felles", "grillmester")
	writePakke(t, ownDir, "egenpakke", "eget", "grillmester")
	declareReuse(t, ownDir, baseDir)

	src := loadSource(t, ownDir)
	resolver, reused, err := composeResolver(resolverFor(src.Dir, src.Pakke), src)
	if err != nil {
		t.Fatal(err)
	}
	if reused == nil || reused.Dir != baseDir {
		t.Fatalf("gjenbrukt kilde = %v, ventet %s", reused, baseDir)
	}
	manifest, err := pakkeContents(resolver, src)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(manifest.Agents, ","); got != "eget,felles,grillmester" {
		t.Errorf("agenter = %q, ventet eget,felles,grillmester", got)
	}
	got, ok := resolver.Get(KindAgent, "grillmester")
	if !ok {
		t.Fatal("grillmester ble ikke løst")
	}
	if !strings.HasPrefix(got.AbsPath, ownDir) {
		t.Errorf("grillmester løste til %s, ventet pakkens eget under %s", got.AbsPath, ownDir)
	}
}

// To pakker som gjenbruker hverandre har ingen rekkefølge å løses i. Uten
// vakta henter composeResolver dem vekselvis til stacken tar slutt.
func TestReuseCycleIsRefused(t *testing.T) {
	aDir, bDir := t.TempDir(), t.TempDir()
	writePakke(t, aDir, "a", "en")
	writePakke(t, bDir, "b", "to")
	declareReuse(t, aDir, bDir)
	declareReuse(t, bDir, aDir)

	src := loadSource(t, aDir)
	_, _, err := composeResolver(resolverFor(src.Dir, src.Pakke), src)
	if err == nil {
		t.Fatal("en gjenbrukssyklus ble godtatt")
	}
	if !strings.Contains(err.Error(), "cycle") {
		t.Errorf("feilmeldinga nevner ikke syklusen: %v", err)
	}
}

// En pakke uten erklæring skal løses nøyaktig som før kjedinga fantes.
func TestPakkeWithoutDeclarationIsUnchanged(t *testing.T) {
	dir := t.TempDir()
	writePakke(t, dir, "alene", "en")
	src := loadSource(t, dir)
	own := resolverFor(src.Dir, src.Pakke)
	composed, reused, err := composeResolver(own, src)
	if err != nil {
		t.Fatal(err)
	}
	if reused != nil {
		t.Errorf("fant en gjenbrukt kilde der ingen er erklært: %v", reused)
	}
	if composed.Base() != nil {
		t.Error("resolveren fikk en base uten at noe er erklært")
	}
	if _, err := agentpakke.LoadDeclaration(dir); err == nil {
		t.Error("testpakka har en erklæring, da tester dette noe annet enn det står")
	}
}

// En gjenbruk av et repo uten sha ville klonet det main tilfeldigvis holder,
// så to installasjoner en uke fra hverandre komponerte ulikt innhold mens
// begge rapporterte samme erklæring.
func TestUnpinnedRepoReuseIsRefused(t *testing.T) {
	dir := t.TempDir()
	writePakke(t, dir, "egen", "eget")
	declareReuse(t, dir, "navikt/basepakke")

	src := loadSource(t, dir)
	_, _, err := composeResolver(resolverFor(src.Dir, src.Pakke), src)
	if err == nil {
		t.Fatal("en gjenbruk uten revisjon ble godtatt")
	}
	if !strings.Contains(err.Error(), "without a revision") {
		t.Errorf("feilmeldinga sier ikke hva som mangler: %v", err)
	}
}

// Kontroll: en lokal sti har ingen revisjon å pinne, og skal fortsatt virke.
// Uten denne ville testen over passert selv om vakta nektet alt.
func TestUnpinnedLocalPathReuseIsAllowed(t *testing.T) {
	baseDir, ownDir := t.TempDir(), t.TempDir()
	writePakke(t, baseDir, "basepakke", "felles")
	writePakke(t, ownDir, "egenpakke", "eget")
	declareReuse(t, ownDir, baseDir)

	src := loadSource(t, ownDir)
	_, reused, err := composeResolver(resolverFor(src.Dir, src.Pakke), src)
	if err != nil {
		t.Fatalf("en lokal sti uten sha ble nektet: %v", err)
	}
	if reused == nil {
		t.Fatal("den lokale basen ble ikke gjenbrukt")
	}
}

// Sync-komposisjonen er dekket i e2e/golden_path_test.go, ikke her.
//
// Testen som sto på dette stedet kalte composeResolver selv i stedet for å
// kjøre sync, altså gjentok den det sync gjør. Den passerte med kallet fjernet
// fra sync.go, som er nøyaktig defekten den ble skrevet for å fange. En test
// som ikke kan feile er verre enn ingen, fordi den ser ut som dekning.

// En kilde uten manifest komponerer ikke, selv om den skulle ha en erklæring
// liggende. Ellers ville en collection-kilde hentet et fremmed repo midt i en
// installasjon som aldri ba om det.
func TestLegacySourceDoesNotCompose(t *testing.T) {
	baseDir, ownDir := t.TempDir(), t.TempDir()
	writePakke(t, baseDir, "basepakke", "felles")
	if err := os.MkdirAll(filepath.Join(ownDir, ".nav-pilot"), 0o755); err != nil {
		t.Fatal(err)
	}
	declareReuse(t, ownDir, baseDir)

	src := &Source{Dir: ownDir, Repo: ownDir}
	if err := attachPakke(src); err != nil {
		t.Fatal(err)
	}
	if src.Pakke != nil {
		t.Fatal("kilden fikk et manifest, da tester dette noe annet enn det står")
	}
	_, reused, err := composeResolver(resolverFor(src.Dir, src.Pakke), src)
	if err != nil {
		t.Fatal(err)
	}
	if reused != nil {
		t.Errorf("en manifestløs kilde komponerte %s", reused.Dir)
	}
}
